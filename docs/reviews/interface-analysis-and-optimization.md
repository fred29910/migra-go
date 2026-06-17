# 接口分析与重构优化方案

> 分析日期：2026-06-16
>
> 范围：全部 `internal/` 及 `cmd/migra/` 包（104 个 Go 文件，12 个接口定义，34 种 Operation，16 种 Mutation）

---

## 一、项目架构总览

```
cmd/migra (CLI - Cobra + Viper)
  ↓
internal/app (应用层编排：load → normalize → diff → plan → render)
  ├── internal/source (Loader 接口 + Registry 策略模式)
  │   ├── internal/parser (HandlerRegistry + Handler + SchemaMutation)
  │   └── internal/introspect (pg_catalog 自省)
  ├── internal/model (核心数据模型)
  ├── internal/normalize (语义归一化)
  ├── internal/diff (差异引擎：34 种 Operation)
  ├── internal/plan (DAG 拓扑排序 + 三阶段)
  └── internal/render (SQL/JSON 渲染)
```

**核心流水线：** Source → Parser/Introspect → Normalize → Diff → Plan → Render → Push

---

## 二、全部公共接口清单

项目共定义 **12 个 Go 接口（interface）**，80+ 个实现类型：

| # | 接口 | 包 | 方法数 | 实现者 | 用途 |
|---|------|-----|--------|--------|------|
| 1 | `diff.Operation` | `internal/diff` | 5 | 34 个结构体 | 数据库变更操作 |
| 2 | `diff.Engine` | `internal/diff` | 1 | `*Differ` | Diff 引擎 |
| 3 | `diff.RenderContext` | `internal/diff` | 1 | `*Renderer` | 渲染上下文 |
| 4 | `source.Loader` | `internal/source` | 2 | DBLoader / SQLFileLoader / DirectoryLoader | Schema 加载策略 |
| 5 | `app.DiffService` | `internal/app` | 1 | `*diffService` | Diff 服务 |
| 6 | `plan.Engine` | `internal/plan` | 1 | `*Planner` | 执行计划引擎 |
| 7 | `render.SQLEngine` | `internal/render` | 1 | `*Renderer` | SQL 渲染器 |
| 8 | `parser.Handler` | `internal/parser` | 1 | 10 个 Handler | AST 处理器 |
| 9 | `parser.SchemaMutation` | `internal/parser` | 3 | 16 个 Mutation | 自应用变更 |
| 10 | `introspect.Querier` | `internal/introspect` | 1 | `*pgx.Conn` / mock | DB 查询抽象 |
| 11 | `push.sqlTx` | `internal/app/push` | 3 | `pgx.Tx` / `*mockTx` | 事务接口（unexported） |
| 12 | `push.dbConnector` | `internal/app/push` | 2 | `*pgxConnAdapter` / `*mockConnector` | 连接接口（unexported） |

---

## 三、发现问题与优化方案

### P0：架构性缺陷（建议立即动手）

---

#### 问题 1：核心流水线缺乏 Context 传播

**现状：** 以下关键路径均不接受 `context.Context`：

- `diff.Engine.Diff()` — 无法取消长时间运行的 diff
- `plan.Plan()` — 无法取消
- `render.SQLEngine.RenderAll()` — 无法取消

`diff_service.go` 只在最外层设置 `context.WithTimeout`，一旦进入内部，取消信号无法传播。

**影响：** 大型 Schema 的 diff 无法超时或优雅取消；用户按 Ctrl+C 后进程可能继续运行。

**方案：** 为三个核心接口增加 `ctx context.Context` 参数：

```go
// diff/differ.go
type Engine interface {
    Diff(ctx context.Context, source, target *model.Schema) ([]Operation, []string)
}

// plan/plan.go
type Engine interface {
    Plan(ctx context.Context, ops []diff.Operation) (map[Stage][]diff.Operation, error)
}

// render/render.go
type SQLEngine interface {
    RenderAll(ctx context.Context, ops []diff.Operation) (string, error)
}
```

内部循环中定期检查 `ctx.Err()`，实现可取消的 diff/plan/render。

**涉及文件：** `diff/differ.go`、`plan/plan.go`、`render/render.go`、`app/pipeline.go`

---

#### 问题 2：Config 结构体职责混杂

**现状：** `app.Config` 同时包含 diff 和 push 两个命令的配置项：

```go
type Config struct {
    Source, Target string
    Schemas        []string
    Format, OutputFile string   // 仅 diff 使用
    UnsafeDrop, Strict bool     // diff + push 共用
    Timeout        time.Duration
    Execute        bool         // 仅 push 使用
    NoVerify       bool         // 仅 push 使用
}
```

`diff` 命令的 `Run()` 对 `Execute` / `NoVerify` 字段完全不感知，但字段却存在于同一个结构体中，违反了**接口隔离原则**。

**方案：** 拆分为 `DiffConfig` 和 `PushConfig`，`PushConfig` 内嵌 `DiffConfig`：

```go
type DiffConfig struct {
    Source, Target string
    Schemas        []string
    Format         string
    UnsafeDrop     bool
    Strict         bool
    Timeout        time.Duration
}

type PushConfig struct {
    DiffConfig             // 内嵌
    Execute, NoVerify bool
}
```

**涉及文件：** `app/diff_service.go`、`cmd/migra/diff_runner.go`、`cmd/migra/push.go`

---

#### 问题 3：ExecutePlan 方法过于庞大

**现状：** `push_service.go:ExecutePlan()` 单方法约 150 行，混杂了以下职责：

- 信号处理（SIGINT/SIGTERM → 取消 → 回滚）
- 事务管理（Begin → Commit / Rollback）
- 非事务性 DDL 检测（CREATE INDEX CONCURRENTLY）
- 4 种交互模式（y / n / a / s）
- 自动模式与交互模式切换
- 危险操作检测与拦截
- SQL 执行与错误处理

**方案：** 拆分为 5 个内部方法：

```go
func (s *PushService) ExecutePlan(ctx context.Context, ...) error {
    ctx = s.setupSignalHandler(ctx)
    conn, tx := s.beginTransaction(ctx, connStr)
    defer s.rollbackOnPanic(tx)

    if cfg.Execute {
        return s.executeAuto(ctx, tx, ops)
    }
    return s.executeInteractive(ctx, tx, ops)
}

func (s *PushService) executeInteractive(ctx context.Context, tx sqlTx, ops []diff.Operation) error
func (s *PushService) executeAuto(ctx context.Context, tx sqlTx, ops []diff.Operation) error
func (s *PushService) handleUserInput(prompt string, isDestructive bool) (action, error)
func (s *PushService) runPostExecutionValidation(ctx context.Context, cfg app.Config, sourceSchema *model.Schema) error
```

**涉及文件：** `app/push/push_service.go`

---

### P1：接口与设计质量（建议第一期迭代）

---

#### 问题 4：错误处理双轨制

**现状：** `internal/errors` 中同时存在两套错误系统：

```go
// 哨兵错误（旧代码使用）
var ErrNotFound = errors.New("resource not found")

// 结构化错误（新代码使用）
var ErrCodeNotFound = &Error{Code: "NOT_FOUND", Message: "resource not found"}
```

旧代码（`app/diff_service.go`、`source/registry.go`）使用哨兵错误 + `%w` 包装；新代码使用 `Error` 结构体。两者语义完全重叠，开发者在选择时容易困惑。

**方案：** 统一为结构化错误，废弃哨兵错误常量。为所有 `Error` 添加 `Is(target error) bool` 方法以实现向下兼容：

```go
func (e *Error) Is(target error) bool {
    // 兼容旧哨兵错误：ErrNotFound 应能匹配 ErrCodeNotFound
    if target == ErrNotFound && e.Code == "NOT_FOUND" {
        return true
    }
    return false
}
```

**涉及文件：** `errors/errors.go`，以及所有引用哨兵错误的文件

---

#### 问题 5：PushService 不是接口

**现状：** `push.PushService` 是具体结构体，`ExecutePlan` 是实例方法。`cmd/migra/push.go` 中直接 `push.NewPushService()` 实例化，测试时依赖真实的 `os.Stdin`：

```go
pushService := push.NewPushService()
pushService.ExecutePlan(cmd.Context(), appCfg, sourceSchema, ops)
```

**方案：** 定义 `push.Service` 接口，`NewPushService()` 返回接口：

```go
type Service interface {
    ExecutePlan(ctx context.Context, cfg app.Config, sourceSchema *model.Schema, ops []diff.Operation) error
}

func NewPushService() Service { ... }
```

**涉及文件：** `app/push/push_service.go`、`cmd/migra/push.go`

---

#### 问题 6：Parser 可变状态

**现状：** `Parser` 结构体包含 `schema`、`errors`、`warnings`、`sql` 等字段，`ParseSQL()` 调用会**重置**这些字段：

```go
func (p *Parser) ParseSQL(sql string) (*model.Schema, error) {
    p.schema = model.NewSchema()   // 重置
    p.errors = p.errors[:0]        // 重置
    p.warnings = p.warnings[:0]    // 重置
```

连续调用 `ParseSQL()` 两次，第二次会覆盖第一次的结果。且文档中缺失并发安全说明。

**方案（推荐 B）：** 将 `schema`、`errors`、`warnings` 改为局部变量而非结构体字段，使 `ParseSQL()` 幂等：

```go
type Parser struct {
    registry *HandlerRegistry
    applier  *MutationApplier
    warnFn   WarningEmitter
}

func (p *Parser) ParseSQL(sql string) (*model.Schema, error) {
    schema := model.NewSchema()
    errors := make([]error, 0)
    warnings := make([]string, 0)
    // ... 解析逻辑 ...
    return schema, nil
}

func NewParser() *Parser {
    return &Parser{
        registry: DefaultRegistry(),
        applier:  &MutationApplier{},
    }
}
```

**方案 A（备选）：** 明确标注非并发安全，并添加文档警告。

**涉及文件：** `parser/parser.go`

---

#### 问题 7：plan.assignStage 硬编码操作分类

**现状：** `assignStage()` 是一个巨大的 switch-case，硬编码了 34 种 `diff.Kind` 与三阶段（Pre-deploy / Deploy / Post-deploy）的映射关系。新增一种 Operation 就需要修改此函数：

```go
func (p *Planner) assignStage(op diff.Operation) Stage {
    switch kind {
    case diff.KindCreateSchema, diff.KindAddTable, ...: return StagePreDeploy
    case diff.KindAlterColumnType, ...: return StageDeploy
    case diff.KindDropView, ...:
        // 特殊处理 IsRecreate
    }
}
```

**方案：** 将 Stage 信息编码到 Operation 自身，例如在 `Kind` 上添加方法，或在 Operation 接口上增加可选的 Stager 接口：

```go
// 方案 A：Operation 接口增加 Stage() 方法
type HasStage interface {
    Stage() Stage
}

// Planner 优先检测 Stager 接口
if stager, ok := op.(HasStage); ok {
    return stager.Stage()
}
// 否则 fallback 到 switch-case

// 方案 B：在注册 Kind 时声明 Stage（需要一个全局映射表）
var kindStageMap = map[Kind]Stage{...}
```

**涉及文件：** `plan/plan.go`

---

### P2：可扩展性与健壮性（建议第二期迭代）

---

#### 问题 8：Model 缺少深拷贝方法

**现状：** `model.Schema` 及其子结构体都是指针引用。`NormalizeSchemas()` 直接原地修改，会**污染调用者的数据**。`ComputeDiff()` 的调用者（如 `push.go`）传入的是同一个 Schema 指针，normalize 操作后原始数据被不可逆地修改。

**方案：** 为 `Schema` 添加 `Clone() *Schema` 深拷贝方法：

```go
func (s *Schema) Clone() *Schema {
    clone := NewSchema()
    for name, ns := range s.Schemas {
        clone.Schemas[name] = ns.Clone()
    }
    return clone
}

func (ns *Namespace) Clone() *Namespace { ... }
func (t *Table) Clone() *Table { ... }
```

Normalize 操作在 Schema 的副本上进行，确保调用者的原始数据不受影响。

**涉及文件：** `model/schema.go`、`model/table.go`、`model/column.go`、`app/pipeline.go`

---

#### 问题 9：Loader 无优先级

**现状：** `source.Registry` 按注册顺序遍历，先匹配者胜出：

```go
func (r *Registry) Load(ctx, source, opt) {
    for _, loader := range r.loaders {
        if loader.Match(source) { return loader.Load(...) }
    }
}
```

当前注册顺序为 `DBLoader` → `SQLFileLoader` → `DirectoryLoader`。`DirectoryLoader` 虽有防御检查，但整体缺乏显式的优先级机制。如果用户有一个名为 `postgres://` 的目录（极端情况），行为不可预期。

**方案：** 为 `Loader` 增加 `Priority() int` 方法，按优先级排序后匹配：

```go
type Loader interface {
    Match(source string) bool
    Load(ctx, source, opt) (*model.Schema, []error, error)
    Priority() int   // 高优先级优先匹配
}

// 注册时自动排序
func (r *Registry) Register(loader Loader) {
    r.loaders = append(r.loaders, loader)
    sort.Slice(r.loaders, func(i, j int) bool {
        return r.loaders[i].Priority() > r.loaders[j].Priority()
    })
}
```

**涉及文件：** `source/loader.go`、`source/registry.go`、`source/db_loader.go`、`source/sql_file_loader.go`、`source/dir_loader.go`

---

#### 问题 10：Pipeline 无 Hook 机制

**现状：** `ComputeDiff()` 是固定的流水线：

```
Normalize → FilterNamespaces → Diff → FilterDestructiveOps → Plan → Sort
```

中间没有任何插入自定义逻辑的扩展点。如果调用者需要在 diff 前做 Schema 预处理，或在 render 前过滤操作，只能自己包装 `ComputeDiff()`。

**方案：** 提供可选 Pre/Post hooks：

```go
type PipelineHooks struct {
    PreNormalize  func(source, target *model.Schema) error
    PostDiff      func(ops []diff.Operation) ([]diff.Operation, error)
    PreRender     func(ops []diff.Operation) ([]diff.Operation, error)
}
```

将 Hooks 参数加入 `ComputeDiff()` 签名（或通过 Context 传递），nil 表示不执行。

**涉及文件：** `app/pipeline.go`

---

### P3：测试与边缘覆盖（建议第三期迭代）

---

#### 问题 11：normalize 正则表达式脆弱

**现状：** `normalizeDefaultExpr()` 使用正则表达式处理类型转换：

```go
var typeCastRe = regexp.MustCompile(`::[\w\s]+$`)
var nestedTypeCastRe = regexp.MustCompile(`'([^']*)'::[\w\s]+`)
```

对于以下边缘情况可能匹配失败：

- 模式限定的类型名：`::pg_catalog.int4`
- 带参数的类型的转换：`::numeric(10,2)`
- 嵌套表达式中的类型转换
- 函数调用返回值的类型转换

**方案：**

1. 使用 `pg_query_go` 解析默认值表达式 AST，在 AST 层处理类型转换
2. 或补充以下边缘用例的测试：

```
nextval('my_seq'::regclass)
'value'::pg_catalog.text
(3 + 2)::numeric(10,2)
current_setting('server_version')::integer
ARRAY[1,2,3]::integer[]
```

**涉及文件：** `normalize/normalize.go`

---

#### 问题 12：缺少 Benchmark 测试

**现状：** 仅 `diff/benchmark_test.go` 一个 benchmark 文件，覆盖了 2 种操作。以下关键路径完全无 benchmark：

- `parser.ParseSQL()` — 对大 SQL 文件的解析性能
- `diff.Diff()` — 对大型 Schema 的 diff 性能
- `plan.TopoSort()` — 对大量操作（1000+）的排序性能
- 完整流水线 end-to-end benchmark

**方案：** 新增 benchmark 文件：

```go
// parser/parser_bench_test.go
func BenchmarkParseSQL(b *testing.B) {
    sql := loadLargeTestSQL() // 包含 100+ DDL 语句
    b.ResetTimer()
    for i := 0; i < b.N; i++ {
        p := NewParser()
        _, err := p.ParseSQL(sql)
        if err != nil { b.Fatal(err) }
    }
}

// diff/diff_bench_test.go  — 补充大型 Schema diff
func BenchmarkDiffLargeSchema(b *testing.B) { ... }

// plan/plan_bench_test.go
func BenchmarkTopoSort1000Ops(b *testing.B) { ... }
```

**涉及文件：** `diff/benchmark_test.go`，新增 `parser/parser_bench_test.go`、`plan/plan_bench_test.go`

---

#### 问题 13：Model 缺少快捷方法

**现状：** `resolveColumn()` 在 parser 中需要三级手动查找：

```go
func resolveColumn(schema *model.Schema, schemaName, table, column string) (...) {
    ns := schema.GetNamespace(schemaName)
    if ns == nil { return nil, nil, nil, fmt.Errorf("schema %s not found", schemaName) }
    tbl, exists := ns.Tables[table]
    if !exists { return nil, nil, nil, fmt.Errorf("table %s.%s not found", schemaName, table) }
    col := tbl.ColumnByName[column]
    if col == nil { return nil, nil, nil, fmt.Errorf("column %s.%s.%s not found", schemaName, table, column) }
    ...
}
```

类似的模式在 `diff` 和 `introspect` 中也重复出现。错误格式不统一，每次调用都需要重复编写 nil 检查逻辑。

**方案：** 在 `model.Schema` 级别提供快捷方法，统一错误格式：

```go
type SchemaError struct {
    Schema, Table, Column string
    Cause string
}

func (s *Schema) ResolveTable(schemaName, tableName string) (*Namespace, *Table, error)
func (s *Schema) ResolveColumn(schemaName, tableName, columnName string) (*Namespace, *Table, *Column, error)
```

**涉及文件：** `model/schema.go`、`parser/mutation.go`

---

## 四、优化优先级矩阵

| 优先级 | 问题 | 影响范围 | 工作估算 | 涉及文件 |
|--------|------|----------|----------|----------|
| **P0** | Context 传播缺失 | 大型 Schema diff 无法取消/超时 | 5 files | `diff/differ.go`、`plan/plan.go`、`render/render.go`、`app/pipeline.go` |
| **P0** | Config 职责混杂 | 新增 push 选项易破坏 diff | 2 files | `app/diff_service.go`、`cmd/migra/diff_runner.go` |
| **P0** | ExecutePlan 过大 | 维护成本高，难测试 | 1 file | `app/push/push_service.go` |
| **P1** | 错误处理双轨 | 开发者困惑 | 2 files | `errors/errors.go`、全局调用点 |
| **P1** | PushService 非接口 | 无法单元测试 | 2 files | `app/push/push_service.go`、`cmd/migra/push.go` |
| **P1** | Parser 可变状态 | 并发安全问题 | 1 file | `parser/parser.go` |
| **P1** | assignStage 硬编码 | 新增操作必须改 switch | 2 files | `plan/plan.go` |
| **P2** | 缺少深拷贝 | Normalize 污染源数据 | 5 files | `model/`、`app/pipeline.go` |
| **P2** | Loader 无优先级 | 极端场景行为异常 | 5 files | `source/` |
| **P2** | 无 Pipeline Hooks | 难以扩展 | 1 file | `app/pipeline.go` |
| **P3** | normaliz 正则脆弱 | 边缘用例误判 | 1 file | `normalize/normalize.go` |
| **P3** | 缺少 Benchmark | 性能退化无感知 | 3 files | 新增 benchmark 文件 |
| **P3** | Model 无快捷方法 | 重复代码 | 3 files | `model/schema.go`、`parser/mutation.go` |

---

## 五、迭代路线图

```
阶段一（基础治理）        阶段二（接口规范化）      阶段三（可扩展性）       阶段四（质量提升）
┌─────────────────┐    ┌─────────────────┐    ┌─────────────────┐    ┌─────────────────┐
│ Context 传播      │    │ 错误处理统一      │    │ 深拷贝 Clone      │    │ 边缘测试         │
│                  │    │                 │    │                  │    │                 │
│ Config 拆分       │──→ │ Push 接口化       │──→ │ Loader 优先级排序 │──→ │ Benchmark 套件   │
│                  │    │                 │    │                  │    │                 │
│ ExecutePlan 拆小  │    │ Parser 无状态化    │    │ Pipeline Hooks   │    │ Fuzz 测试        │
│                  │    │                 │    │                  │    │                 │
│                  │    │ Stage 编码到 Op   │    │                  │    │                  │
└─────────────────┘    └─────────────────┘    └─────────────────┘    └─────────────────┘
```

### 第一阶段（建议 1-2 天）

1. 为 `diff.Engine`、`plan.Engine`、`render.SQLEngine` 增加 `ctx context.Context` 参数
2. 拆分 `Config` 为 `DiffConfig` + `PushConfig`
3. 重构 `ExecutePlan()`，拆分为 4-5 个内部方法

### 第二阶段（建议 2-3 天）

4. 统一错误处理：废弃哨兵错误，保留结构化错误
5. `PushService` 接口化
6. Parser 状态局部变量化（方案 B）
7. Stage 信息编码到 Operation 中

### 第三阶段（建议 2-3 天）

8. 添加 `Clone()` 深拷贝方法
9. Loader 优先级排序
10. Pipeline Hook 机制

### 第四阶段（建议持续）

11. 补充 normalize 边缘用例测试
12. 添加 Benchmark 套件
13. 添加 Fuzz 测试（特别针对 parser 和 normalize）

---

## 六、开发原则

### 接口设计准则

- **单一职责**：一个接口只做一件事。`Config` 拆分即为此原则的实践。
- **依赖倒置**：依赖抽象而非具体实现。PushService 接口化即为此原则的实践。
- **接口隔离**：不应强迫调用者依赖它们不使用的方法。
- **Context 传播**：所有阻塞或耗时的操作都应接受 `context.Context` 作为第一个参数。

### 代码质量门禁

- 新增 public 接口时必须 review 签名设计
- 不允许 `as any`、`@ts-ignore`、`@ts-expect-error`（Go 中对应不允许 `interface{}` 滥用）
- 所有阻塞操作必须可取消（接受 Context）
- Config 结构体必须职责单一

### 演化方向

- 当前项目有良好的分层结构和 OCP（开闭原则）设计（Parser Handler 模式），应将此风格延续到其他模块
- 优先通过接口 + 组合扩展，避免在现有函数中加 switch-case
- 持续降低 `plan.assignStage` 等硬编码映射表的维护成本
