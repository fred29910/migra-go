# 正确性加固设计文档

> 日期：2026-06-19
> 范围：修复 `docs/reviews/2026-06-18-migra-go-deep-code-review.md` 标注的 4 个高优先级缺陷，同步过时文档，补充 loader 的 context 响应。

## 背景与问题

深度代码评审报告标注 4 个「必须修复」缺陷，经核对在当前代码（main 分支）中仍然存在。这些缺陷会导致错误 diff、不可执行 SQL 或半成品迁移脚本，属于核心准确性问题。另外 `docs/DDL.md` 限制 #6 描述的 `character(n)` → `char(n)` 映射缺失问题实际已修复，文档未同步，会误导使用者。

### 缺陷清单

| 编号 | 缺陷 | 位置 | 影响 |
|------|------|------|------|
| #1 | DB 内省丢失 enum/domain/array 真实列类型 | `internal/introspect/tables.go:13` | DB 源失真、误报 diff、渲染不可执行 `USER-DEFINED` 类型 |
| #2 | enum 类型创建顺序可能晚于依赖它的表 | `internal/diff/differ.go:87`、`operation_table.go:93` | 生成 `CREATE TABLE` 早于 `CREATE TYPE`，SQL 不可执行 |
| #3a | Context 取消被 diff 阶段静默吞掉 | `internal/diff/differ.go:13`、`app/pipeline.go:131` | 超时/取消时输出半成品迁移脚本 |
| #3b | 文件/目录 loader 未响应 context | `internal/source/dir_loader.go:43`、`sql_file_loader.go:34` | 大目录/大文件下 CLI timeout 无法中断 |
| #4 | 列重命名启发式把「删除+新增」误判为 rename | `internal/diff/diff_tables.go:178` | 数据语义错误，旧数据保留到新字段 |

### 验证基线

- `go build ./...` 成功
- `go vet ./...` 无 issue
- `go test ./...` 通过，17 包 854 测试

## 设计决策

| 决策点 | 选定方案 | 理由 |
|--------|---------|------|
| #1 内省改造范围 | 完整改用 pg_catalog + `format_type(atttypid, atttypmod)` | 彻底解决 enum/domain/array 失真，information_schema 仅作兜底 |
| #2 enum 排序策略 | 调整 diffNamespace 顺序（types 先于 tables）+ 给 Op 增加列类型依赖 | 双重保障，入队顺序与 DAG 依赖共同保证正确性 |
| #3a 接口改造范围 | 改 `Engine.Diff` 签名为 `(...)([]Operation, []string, error)` | 从源头传播取消错误，ComputeDiff 遇取消中断不继续 render |
| #4 rename 处理策略 | 保留默认开启 + `--no-rename` 开关 + 多候冲突回退警告 | 向后兼容，风险仅在未关闭时，多候冲突时安全回退 |
| #4 配置下沉方式 | `Differ` 用 Option 模式持有 `noRename` | 接口保留 ctx 通用性，配置通过构造器注入 |

## 架构定位

pipeline 各层改造点：

```
source(SQL/Dir/DB) → normalize → diff → plan → render → push
       ↑#1改内省        ↑#4改启发式  ↑#2改依赖
       ↑#3b改loader ctx  ↑#3a改接口签名
```

改动分布：
- `internal/introspect/tables.go` — #1
- `internal/diff/differ.go` — #2 顺序、#3a 签名、#4 Option
- `internal/diff/context.go` — #3a cancelErr 记录
- `internal/diff/operation_table.go`、`operation_column.go` — #2 DependsOn
- `internal/diff/diff_tables.go` — #4 rename 开关 + 多候冲突
- `internal/app/pipeline.go`、`diff_service.go` — #3a 中断、#4 透传配置
- `internal/source/dir_loader.go`、`sql_file_loader.go` — #3b context 响应
- `internal/util/util.go` — #2 新增 `IsBuiltinType`
- `cmd/migra/diff.go`、`push.go`、`diff_runner.go` — #4 CLI flag
- `docs/DDL.md`、`README.md` — 文档同步

## 缺陷 #1：DB 内省真实列类型

### 问题

`tables.go` 用 `information_schema.columns.data_type`，enum/domain/array 列返回 `USER-DEFINED`/`ARRAY`，导致 DB 源失真。

### 方案

改用 `pg_catalog` 查询，`format_type(atttypid, atttypmod)` 获取真实类型。

新查询：

```sql
SELECT
  c.relname                          AS table_name,
  a.attname                          AS column_name,
  format_type(a.atttypid, a.atttypmod) AS data_type,
  a.attnotnull                       AS is_not_null,
  pg_get_expr(ad.adbin, ad.adrelid)  AS column_default,
  a.attnum                           AS ordinal_position,
  a.attidentity                      AS is_identity,
  COALESCE(coll.collname, '')        AS collation_name
FROM pg_catalog.pg_class c
JOIN pg_catalog.pg_namespace n ON n.oid = c.relnamespace
JOIN pg_catalog.pg_attribute a ON a.attrelid = c.oid
LEFT JOIN pg_catalog.pg_attrdef ad
  ON ad.adrelid = a.attrelid AND ad.adnum = a.attnum AND a.atthasdef
LEFT JOIN pg_catalog.pg_collation coll ON coll.oid = a.attcollation
WHERE n.nspname = $1
  AND c.relkind = 'r'
  AND a.attnum > 0
  AND NOT a.attisdropped
ORDER BY c.relname, a.attnum
```

字段映射：
- `format_type` 直接返回 `integer`/`text`/`mood`(enum)/`int[]`(array)/`posint`(domain) 等真实类型，无需后处理
- `is_nullable`：`attnotnull` 取反（`attnotnull = false` → nullable）
- `is_identity`：`attidentity` ∈ `{'a','d'}` 表示 identity（`a`=ALWAYS, `d`=BY DEFAULT），空串表示非 identity
- `column_default`：`pg_get_expr(adbin, adrelid)` 返回 default 表达式文本
- 表类型过滤用 `relkind='r'`（等价原 `table_type='BASE TABLE'`）

### 兜底

若 `format_type` 对极少数自定义类型返回 `USER-DEFINED`，保持现状原样输出，由 normalize 层处理——与原行为一致，不引入回归。该改动不涉及 model 层：`Column.DataType` 仍是 string，只是填充来源换了。

### 测试

更新 `introspect_test.go` 的 pgxmock 行（从 information_schema 列改为 pg_catalog 列），新增 enum/domain/array 列返回真实类型的用例。pgxmock 可模拟任意列名/行，无需真实 PostgreSQL。

## 缺陷 #2：enum 类型依赖排序

### 问题

`differ.go:87` 中 `diffTables` 先于 `diffTypes`，且 `AddTableOp.DependsOn` 只声明外键依赖，无列类型依赖。新增 enum + 引用它的表时，`CREATE TABLE` 可能早于 `CREATE TYPE`。

### 方案：双重保障

**改动 1：调整 `diffNamespace` 顺序**（`differ.go:82-97`）

```go
func (c *diffContext) diffNamespace(source, target *model.Namespace) {
	if target == nil {
		return
	}
	c.diffTypes(source, target)
	c.diffTables(source, target)
	c.diffViews(source, target)
	c.diffSequences(source, target)
	c.diffExtensions(source, target)
}
```

保证同 namespace 内 `AddEnumTypeOp` 先入队。但因 DAG 跨阶段排序（types 与 tables 同在 Pre-deploy），仅靠入队顺序不够稳——需要改动 2。

**改动 2：给 Op 增加列类型依赖**

`AddTableOp.DependsOn`（`operation_table.go:94-102`）在现有外键依赖基础上，追加列的自定义类型依赖：

```go
func (op *AddTableOp) DependsOn() []model.ObjectKey {
	var deps []model.ObjectKey
	for _, constraint := range op.Table.Constraints {
		if constraint.Type == "foreign_key" && constraint.RefTable != "" {
			deps = append(deps, model.NewObjectKey(constraint.RefSchema, constraint.RefTable, model.KindTable))
		}
	}
	for _, col := range op.Table.Columns {
		for _, typeKey := range columnTypeDependencies(op.Table.Schema, col) {
			deps = append(deps, typeKey)
		}
	}
	return deps
}
```

`AddColumnOp.DependsOn`（`operation_column.go:37-41`）和 `AlterColumnTypeOp.DependsOn` 同样追加列类型依赖。

**改动 3：新增 `util.IsBuiltinType` + `columnTypeDependencies`**

`internal/util` 新增 `IsBuiltinType(dt string) bool`，内置 PostgreSQL 内置类型白名单。

白名单匹配规则：
- 先 `strings.ToLower` + `TrimSpace`，再匹配
- 支持带空格类型：`character varying`、`double precision`、`timestamp without time zone` 等
- 支持长度修饰：`varchar(N)`、`numeric(P,S)`、`char(N)` 剥离括号后匹配基础名
- 支持数组变体：以 `[]` 结尾的，剥离后缀递归判断基础类型
- 白名单基础类型：integer/int/int4/int8/int2/bigint/smallint/serial/bigserial/smallserial/boolean/bool/text/varchar/character varying/char/character/numeric/real/float4/double precision/float8/json/jsonb/uuid/inet/cidr/macaddr/interval/date/time/timetz/timestamp/timestamptz/bytea/money/oid/void/name 等

`columnTypeDependencies(schema string, col *model.Column) []model.ObjectKey`：
- 若 `IsBuiltinType(col.DataType)` 为 true，返回 nil（无类型依赖）
- 若类型含 `.`（schema-qualified，如 `auth.mood`），按最后一个 `.` 拆分为 schema + name，生成 `NewObjectKey(schema, name, KindType)`
- 否则（非限定自定义类型），用列所在表 schema 生成 `NewObjectKey(schema, col.DataType, KindType)`
- 数组类型（以 `[]` 结尾）：剥离 `[]` 后对基础类型递归判断

DAG 匹配的鲁棒性：依赖生成的 ObjectKey 与 `AddEnumTypeOp` 的 ObjectKey（`NewObjectKey(schema, name, KindType)`）需一致。`AddEnumTypeOp` 用 `target.Name` 作为 schema——列类型若是同 schema 的 enum，生成的 key 同样用表所在 schema，匹配成功。跨 schema 类型引用解析出实际 schema，也能匹配。

### 测试

- 新增 golden 顺序测试：源空、目标有 enum `mood` + 表 `users(status mood)`，断言输出顺序为 `CREATE TYPE mood → CREATE TABLE users`
- 跨 schema 类型引用测试
- 现有 DAG 测试不受影响

## 缺陷 #3a：Context 取消传播

### 问题

`Engine.Diff` 签名无 error 返回，`checkCancelled()` 取消后各函数仅 `return`，`ComputeDiff` 继续输出半成品。

### 方案

**改动 1：`Engine` 接口签名**（`differ.go:11-14`）

```go
type Engine interface {
	Diff(ctx context.Context, source, target *model.Schema) ([]Operation, []string, error)
}
```

**改动 2：`diffContext` 持有取消错误**（`context.go`）

新增 `cancelErr error` 字段。`checkCancelled()` 遇 `ctx.Err() != nil` 时记录到 `cancelErr` 并返回。各 `diff*` 函数现有模式 `if err := c.checkCancelled(); err != nil { return }` 保持不变。

**改动 3：`Differ.Diff` 返回 error**（`differ.go:29-33`）

```go
func (d *Differ) Diff(ctx context.Context, source, target *model.Schema) ([]Operation, []string, error) {
	c := newDiffContext(ctx, d)
	c.diffSchemas(source, target)
	if c.cancelErr != nil {
		return c.ops, c.warnings, c.cancelErr
	}
	return c.ops, c.warnings, nil
}
```

取消时返回已收集的 ops + warnings + error，调用方可决定是否使用部分结果。

**改动 4：`ComputeDiff` 中断逻辑**（`pipeline.go:131`）

```go
differ := diff.NewDiffer()
operations, warnings, err := differ.Diff(ctx, sourceClone, targetClone)
if err != nil {
	return nil, warnings, fmt.Errorf("diff interrupted: %w", err)
}
```

取消时直接返回 error，不再继续 `FilterDestructiveOps`/`BuildExecutionPlan`/render。

注意：本步骤 `NewDiffer()` 暂用无参形式。`WithNoRename` Option 在缺陷 #4 阶段引入后，此处改为 `diff.NewDiffer(diff.WithNoRename(cfg.NoRename))`。执行顺序保证 #3a 先于 #4，故 #3a 阶段不引用未定义符号。

**改动 5：影响面清理**

`Engine` 接口的唯一实现是 `Differ`。需更新直接调用 `Diff` 的地方：`pipeline.go`、`pipeline_test.go`、`diff_service.go` 调用链、`diff_runner.go`、`push.go`。

## 缺陷 #3b：loader context 响应

### 问题

`DirectoryLoader` 的 `WalkDir`、逐文件 `ReadFile`，以及 `SQLFileLoader` 的 `os.ReadFile` 都没有检查 `ctx.Err()`。大目录或大 SQL 文件下，CLI timeout 不能及时中断。

### 方案

`DirectoryLoader.Load`（`dir_loader.go:43-82`）：在 `WalkDir` 回调和逐文件 `ReadFile` 循环中检查 `ctx.Err()`：

```go
err := filepath.WalkDir(sourcePath, func(path string, d os.DirEntry, err error) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	// ...原有逻辑
})
// ...
for _, file := range files {
	if err := ctx.Err(); err != nil {
		return nil, nil, err
	}
	data, readErr := os.ReadFile(file)
	// ...
}
```

`SQLFileLoader.Load`（`sql_file_loader.go:34`）：`os.ReadFile` 前检查 ctx。`os.ReadFile` 本身不支持 ctx 中断，读前检查可拦截大部分场景。超大文件 streaming 改造超出本次范围。

`DBLoader`：已透传 ctx 给 pgx，无需改动。

### 测试

- `dir_loader`/`sql_file_loader` 新增取消测试——预取消 ctx 后调用 Load，断言返回 ctx 错误
- `diff` 层新增取消测试——用 `context.WithCancel` 立即取消，断言 `Diff` 返回 `context.Canceled` error

## 缺陷 #4：rename 启发式开关

### 问题

`diff_tables.go:178-235` 的启发式把「删除+新增」误判为 rename，存在数据语义风险。

### 方案：保留默认开启 + `--no-rename` 开关 + 多候冲突回退

**改动 1：`DiffConfig` 增加字段**（`diff_service.go:14-23`）

```go
type DiffConfig struct {
	Source     string
	Target     string
	Schemas    []string
	Format     string
	OutputFile string
	UnsafeDrop bool
	Strict     bool
	Timeout    time.Duration
	NoRename   bool
}
```

**改动 2：`Differ` 用 Option 模式持有配置**（`differ.go`）

```go
type Differ struct {
	noRename bool
}

func NewDiffer(opts ...DifferOption) *Differ {
	d := &Differ{}
	for _, opt := range opts {
		opt(d)
	}
	return d
}

type DifferOption func(*Differ)
func WithNoRename(v bool) DifferOption { return func(d *Differ) { d.noRename = v } }
```

`diffContext` 增加 `noRename bool`，由 `newDiffContext(ctx, d)` 传入。`diffTableColumns` 在 `noRename=true` 时跳过 rename 检测，直接走 drop+add。

**改动 3：多候冲突回退+警告**（`diff_tables.go:215-235`）

现有逻辑用 `targetBySig` map，同签名多列会后者覆盖前者（静默）。改为：签名冲突时记录冲突，该签名不参与 rename 匹配，相关列回退 drop+add 并输出 warning。

```go
targetBySig := make(map[string]string, len(targetOnlyNames))
sigConflict := make(map[string]bool)
for _, tgtName := range targetOnlyNames {
	tgtCol := target.ColumnByName[tgtName]
	sig := columnSignature(tgtCol)
	if existing, found := targetBySig[sig]; found {
		sigConflict[sig] = true
		c.warnf("column rename ambiguous in %s.%s: signature %q matches multiple target columns, falling back to drop+add", schema, target.Name, sig)
		_ = existing
	}
	targetBySig[sig] = tgtName
}
for _, srcName := range sourceOnlyNames {
	// ...
	if tgtName, found := targetBySig[sig]; found && !sigConflict[sig] && !renamedTarget[tgtName] {
		// rename
	}
}
```

**改动 4：CLI flag**（`diff.go`、`push.go`）

```go
diffCmd.Flags().Bool("no-rename", false, "disable heuristic column rename detection")
_ = viper.BindPFlag("diff.no_rename", diffCmd.Flags().Lookup("no-rename"))
```

`push.go:init` 同样增加 `--no-rename`。`parseDiffConfig`/`parsePushConfig` 读取并填入 `DiffConfig.NoRename`。

### 测试

- `noRename=true`：源列 `a int`、目标列 `b int`（同签名），断言输出 `DROP COLUMN a` + `ADD COLUMN b`，无 rename
- 多候冲突：目标有两列同签名匹配源一列，断言回退 drop+add + warning
- 默认行为不变：现有 rename 测试保持通过

## 文档同步

### `docs/DDL.md`

- 限制 #6（第 209 行）：删除整条（`character(n)` → `char(n)` 映射已在 `util.go:37` 实现）
- 类型映射表第 9 行（第 175 行）：`character(n)` 状态从 `⚠️` 改为 `✅`
- 限制 #8（rename 启发式）：补充 `--no-rename` 开关说明与多候冲突回退
- 命令行参数表：新增 `--no-rename` 行
- 「最后更新」日期更新为 2026-06-19

### `README.md`

- 命令行参数表（第 148-161 行）：新增 `--no-rename` 行

## 执行顺序

按依赖关系推进，避免返工：

1. 文档同步（#6/7）——独立、零风险，先做
2. #1 DB 内省——改 `introspect/tables.go` + 更新 `introspect_test.go`
3. #3a diff 接口——改 `Engine` 签名 + `diffContext` + `Differ.Diff` + `ComputeDiff` + 调用方适配
4. #2 enum 依赖排序——改 `diffNamespace` 顺序 + `IsBuiltinType` + 三个 Op 的 `DependsOn`
5. #4 rename 开关——改 `Differ` Option + `diff_tables.go` + CLI flag
6. #3b loader context——改 `dir_loader.go`/`sql_file_loader.go`

## 验证标准

每个改动后：
- `go build ./...` 通过
- `go vet ./...` 无 issue
- `go test ./...` 全绿

最终整体验证：
- `rtk go test ./...` 全绿，测试数 ≥ 854
- `rtk go vet ./...` 无 issue
- `rtk go build ./...` 成功

## 不在本次范围

- 真实 PostgreSQL 集成测试（Testcontainers）——评审报告低优先级，本次 pgxmock 验证足够
- 超大文件 streaming 解析——`SQLFileLoader` 的 `os.ReadFile` 仍是全量读取，仅加读前 ctx 检查
- parser 层 COLLATE 提取（DDL 限制 #7）——本次未涉及
- 中优先级 #1（Dialect 抽象）、#2（大 Schema 内存优化）、#4（约束索引模型补全）——非正确性阻断问题，留待后续
