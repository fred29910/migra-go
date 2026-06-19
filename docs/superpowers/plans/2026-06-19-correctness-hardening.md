# 正确性加固实现计划

> **面向 AI 代理的工作者：** 必需子技能：使用 superpowers:subagent-driven-development（推荐）或 superpowers:executing-plans 逐任务实现此计划。步骤使用复选框（`- [ ]`）语法来跟踪进度。

**目标：** 修复深度评审标注的 4 个高优先级缺陷，同步过时的 DDL 文档，补充文件/目录 loader 的 context 响应，使 migra-go 的核心 diff/push 链路在取消、类型排序、rename 检测和 DB 内省上具备正确性保证。

**架构：** 按 pipeline 层逐层加固：introspect 改用 pg_catalog 获取真实类型 → diff 层增加 error 返回与取消传播 → diff 层调整类型排序与列类型依赖 → diff 层增加 rename 开关 → source 层补充 ctx 检查 → 文档同步。每个任务独立可测，按依赖顺序推进。

**技术栈：** Go 1.26.2、pgx v5、pgxmock v4、pg_query_go、cobra/viper、testify

**规格文档：** `docs/superpowers/specs/2026-06-19-correctness-hardening-design.md`

---

## 文件结构

### 修改的文件及职责

| 文件 | 职责 | 涉及任务 |
|------|------|---------|
| `internal/introspect/tables.go` | 表/列内省查询（改用 pg_catalog） | 任务 1 |
| `internal/introspect/introspect_test.go` | 内省的 pgxmock 测试 | 任务 1 |
| `internal/diff/differ.go` | `Engine` 接口、`Differ`、`diffNamespace` 顺序、Option | 任务 2、3、4 |
| `internal/diff/context.go` | `diffContext` 取消错误记录、noRename 字段 | 任务 2、4 |
| `internal/diff/operation_table.go` | `AddTableOp.DependsOn` 列类型依赖 | 任务 3 |
| `internal/diff/operation_column.go` | `AddColumnOp`/`AlterColumnTypeOp` 列类型依赖 | 任务 3 |
| `internal/diff/operation_test.go` | DependsOn 断言更新 | 任务 3 |
| `internal/diff/diff_tables.go` | rename 启发式 noRename 开关 + 多候冲突 | 任务 4 |
| `internal/diff/diff_tables_test.go` | rename 测试补充 | 任务 4 |
| `internal/diff/differ_test.go` | 取消传播测试、顺序测试 | 任务 2、3 |
| `internal/app/pipeline.go` | `ComputeDiff` 中断逻辑、透传 noRename | 任务 2、4 |
| `internal/app/diff_service.go` | `DiffConfig.NoRename` 字段 | 任务 4 |
| `internal/app/pipeline_test.go` | ComputeDiff 取消测试适配 | 任务 2 |
| `internal/source/dir_loader.go` | ctx.Err() 检查 | 任务 5 |
| `internal/source/sql_file_loader.go` | ctx.Err() 检查 | 任务 5 |
| `internal/source/dir_loader_test.go` | 取消测试 | 任务 5 |
| `internal/source/sql_file_loader_test.go` | 取消测试 | 任务 5 |
| `internal/util/util.go` | `IsBuiltinType` 函数 | 任务 3 |
| `internal/util/util_test.go`（新建若不存在） | `IsBuiltinType` 测试 | 任务 3 |
| `cmd/migra/diff.go` | `--no-rename` flag | 任务 4 |
| `cmd/migra/push.go` | `--no-rename` flag | 任务 4 |
| `cmd/migra/diff_runner.go` | `parseDiffConfig` 读取 no-rename | 任务 4 |
| `docs/DDL.md` | 删除限制 #6、更新类型映射、补充 --no-rename | 任务 6 |
| `README.md` | 命令行参数表补充 --no-rename | 任务 6 |

### 新增函数/类型

- `util.IsBuiltinType(dt string) bool` — 任务 3
- `diff.columnTypeDependencies(schema string, col *model.Column) []model.ObjectKey` — 任务 3
- `diff.DifferOption` / `diff.WithNoRename(v bool) DifferOption` — 任务 4
- `diffContext.cancelErr error` 字段 — 任务 2
- `diffContext.noRename bool` 字段 — 任务 4

---

## 任务 1：DB 内省改用 pg_catalog 获取真实列类型

**文件：**
- 修改：`internal/introspect/tables.go`
- 测试：`internal/introspect/introspect_test.go`

### 背景

现有 `loadTables` 用 `information_schema.columns.data_type`，enum/domain/array 列返回 `USER-DEFINED`/`ARRAY`。改用 `pg_catalog` 的 `format_type(atttypid, atttypmod)` 获取真实类型。

注意：`format_type` 对 varchar 返回 `character varying(255)`（而非 `varchar(255)`），对 timestamp 返回 `timestamp without time zone`。这些会经由 `normalize.CanonicalizeSchema` → `util.NormalizeDataType` 归一化为 `varchar(255)`/`timestamp`，故测试断言需在 normalize 后验证，或在内省断言中接受 `format_type` 的原始输出。

- [ ] **步骤 1：更新 `emptyTableRows` 的 mock 列结构**

修改 `internal/introspect/introspect_test.go:32-38`：

```go
func emptyTableRows() *pgxmock.Rows {
	return pgxmock.NewRows([]string{
		"table_name", "column_name", "data_type",
		"is_not_null", "column_default", "ordinal_position",
		"is_identity", "collation_name",
	})
}
```

- [ ] **步骤 2：更新 `TestLoadTables_SingleTable` 的 AddRow 数据**

修改 `internal/introspect/introspect_test.go:158-199`。pg_catalog 查询返回的字段顺序与含义变了：`is_nullable`（YES/NO）→ `is_not_null`（bool）；`character_maximum_length` 不再单独返回（由 `format_type` 内含）；`is_identity`（YES/NO）→ `attidentity`（''/'a'/'d'）；`identity_generation` 列移除。

```go
func TestLoadTables_SingleTable(t *testing.T) {
	q, mock := newMock(t)

	rows := emptyTableRows().
		AddRow("users", "id", "bigint", true, "nextval('users_id_seq'::regclass)", 1, "", "").
		AddRow("users", "name", "character varying(255)", false, nil, 2, "", "").
		AddRow("users", "email", "text", true, nil, 3, "", "en_US")
	mock.ExpectQuery("SELECT").WithArgs(pgxmock.AnyArg()).WillReturnRows(rows)

	ns := model.NewNamespace("public")
	if err := loadTables(context.Background(), q, "public", ns); err != nil {
		t.Fatalf("loadTables: %v", err)
	}

	table, ok := ns.Tables["users"]
	if !ok {
		t.Fatal("expected 'users' table")
	}
	if len(table.Columns) != 3 {
		t.Fatalf("expected 3 columns, got %d", len(table.Columns))
	}

	idCol := table.Columns[0]
	if idCol.Name != "id" || idCol.DataType != "bigint" || idCol.IsNullable {
		t.Errorf("id column: name=%q, type=%q, nullable=%v", idCol.Name, idCol.DataType, idCol.IsNullable)
	}
	if idCol.DefaultExpr == nil || *idCol.DefaultExpr != "nextval('users_id_seq'::regclass)" {
		t.Errorf("id column default: %v", idCol.DefaultExpr)
	}

	nameCol := table.Columns[1]
	if nameCol.DataType != "character varying(255)" {
		t.Errorf("name column type: got %q, want 'character varying(255)'", nameCol.DataType)
	}
	if !nameCol.IsNullable {
		t.Error("name column should be nullable")
	}

	emailCol := table.Columns[2]
	if emailCol.Collation != "en_US" {
		t.Errorf("email column collation: got %q, want 'en_US'", emailCol.Collation)
	}
}
```

- [ ] **步骤 3：新增 enum/domain/array 列类型的内省测试**

在 `internal/introspect/introspect_test.go` 末尾追加：

```go
func TestLoadTables_EnumAndArrayColumns(t *testing.T) {
	q, mock := newMock(t)

	// format_type 对 enum 返回类型名，对数组返回 "int[]"，对 domain 返回域名
	rows := emptyTableRows().
		AddRow("t", "status", "mood", false, nil, 1, "", "").
		AddRow("t", "tags", "text[]", false, nil, 2, "", "").
		AddRow("t", "score", "posint", false, nil, 3, "", "")
	mock.ExpectQuery("SELECT").WithArgs(pgxmock.AnyArg()).WillReturnRows(rows)

	ns := model.NewNamespace("public")
	if err := loadTables(context.Background(), q, "public", ns); err != nil {
		t.Fatalf("loadTables: %v", err)
	}

	table := ns.Tables["t"]
	if table.Columns[0].DataType != "mood" {
		t.Errorf("enum column: got %q, want mood", table.Columns[0].DataType)
	}
	if table.Columns[1].DataType != "text[]" {
		t.Errorf("array column: got %q, want text[]", table.Columns[1].DataType)
	}
	if table.Columns[2].DataType != "posint" {
		t.Errorf("domain column: got %q, want posint", table.Columns[2].DataType)
	}
}

func TestLoadTables_IdentityColumn(t *testing.T) {
	q, mock := newMock(t)

	// attidentity: 'a' = ALWAYS, 'd' = BY DEFAULT, '' = 非 identity
	rows := emptyTableRows().
		AddRow("t", "id", "integer", true, nil, 1, "a", "").
		AddRow("t", "seq", "integer", true, nil, 2, "d", "")
	mock.ExpectQuery("SELECT").WithArgs(pgxmock.AnyArg()).WillReturnRows(rows)

	ns := model.NewNamespace("public")
	if err := loadTables(context.Background(), q, "public", ns); err != nil {
		t.Fatalf("loadTables: %v", err)
	}

	cols := ns.Tables["t"].Columns
	if !cols[0].IsIdentity || cols[0].IdentityKind != "ALWAYS" {
		t.Errorf("id identity: IsIdentity=%v, Kind=%q", cols[0].IsIdentity, cols[0].IdentityKind)
	}
	if !cols[1].IsIdentity || cols[1].IdentityKind != "BY DEFAULT" {
		t.Errorf("seq identity: IsIdentity=%v, Kind=%q", cols[1].IsIdentity, cols[1].IdentityKind)
	}
}
```

- [ ] **步骤 4：运行测试验证失败**

运行：`rtk go test ./internal/introspect/ -run TestLoadTables -v`
预期：FAIL，`loadTables` 仍用旧查询，mock 列不匹配导致扫描错误

- [ ] **步骤 5：重写 `loadTables` 查询与扫描逻辑**

替换 `internal/introspect/tables.go:11-90` 全部内容：

```go
package introspect

import (
	"context"
	"database/sql"
	"fmt"

	"github.com/fred29910/migra-go/internal/model"
)

func loadTables(ctx context.Context, q Querier, schemaName string, ns *model.Namespace) error {
	query := `
	SELECT
		c.relname                            AS table_name,
		a.attname                            AS column_name,
		format_type(a.atttypid, a.atttypmod) AS data_type,
		a.attnotnull                         AS is_not_null,
		pg_get_expr(ad.adbin, ad.adrelid)    AS column_default,
		a.attnum                             AS ordinal_position,
		a.attidentity                        AS is_identity,
		COALESCE(coll.collname, '')          AS collation_name
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
	ORDER BY c.relname, a.attnum`

	rows, err := q.Query(ctx, query, schemaName)
	if err != nil {
		return fmt.Errorf("query tables: %w", err)
	}
	defer rows.Close()

	var (
		tableName     string
		colName       string
		dataType      string
		isNotNull     bool
		colDefault    sql.NullString
		ordinalPos    int
		isIdentity    string
		collationName string
	)

	currentTable := ""
	for rows.Next() {
		err := rows.Scan(&tableName, &colName, &dataType, &isNotNull, &colDefault, &ordinalPos, &isIdentity, &collationName)
		if err != nil {
			return fmt.Errorf("scan table row: %w", err)
		}

		if tableName != currentTable {
			if _, exists := ns.Tables[tableName]; !exists {
				ns.Tables[tableName] = model.NewTable(schemaName, tableName)
			}
			currentTable = tableName
		}

		table := ns.Tables[tableName]
		col := &model.Column{
			Name:       colName,
			DataType:   dataType,
			IsNullable: !isNotNull,
			Collation:  collationName,
		}
		if colDefault.Valid {
			defaultStr := colDefault.String
			col.DefaultExpr = &defaultStr
		}
		if isIdentity == "a" || isIdentity == "d" {
			col.IsIdentity = true
			switch isIdentity {
			case "a":
				col.IdentityKind = "ALWAYS"
			case "d":
				col.IdentityKind = "BY DEFAULT"
			}
		}
		table.AddColumn(col)
	}

	if err := rows.Err(); err != nil {
		return fmt.Errorf("iterate table rows: %w", err)
	}
	return nil
}
```

- [ ] **步骤 6：运行测试验证通过**

运行：`rtk go test ./internal/introspect/ -v`
预期：PASS

- [ ] **步骤 7：Commit**

```bash
git add internal/introspect/tables.go internal/introspect/introspect_test.go
git commit -m "fix(introspect): DB 内省改用 pg_catalog format_type 获取真实列类型

enum/domain/array 列不再返回 USER-DEFINED/ARRAY，format_type 直接返回
真实类型。identity 改读 attidentity 字段，default 改用 pg_get_expr。

Co-Authored-By: Oz <oz-agent@warp.dev>"
```

---

## 任务 2：diff.Engine 接口增加 error 返回，传播 Context 取消

**文件：**
- 修改：`internal/diff/differ.go`、`internal/diff/context.go`
- 测试：`internal/diff/differ_test.go`、`internal/app/pipeline_test.go`
- 适配：`internal/app/pipeline.go`

### 背景

`Engine.Diff` 签名无 error 返回，`checkCancelled()` 取消后各函数仅 `return`，`ComputeDiff` 继续输出半成品。本任务给接口加 error 返回，并在 `ComputeDiff` 遇取消时中断。

- [ ] **步骤 1：编写取消传播的失败测试**

在 `internal/diff/differ_test.go` 末尾追加：

```go
func TestDiffer_Diff_CancelledContext(t *testing.T) {
	d := NewDiffer()
	src := model.NewSchema()
	tgt := model.NewSchema()
	tgtNs := model.NewNamespace("public")
	tgtNs.Tables["users"] = model.NewTable("public", "users")
	tgtNs.Tables["users"].AddColumn(&model.Column{Name: "id", DataType: "integer"})
	tgt.Schemas["public"] = tgtNs

	ctx, cancel := context.WithCancel(context.Background())
	cancel() // 立即取消

	ops, warns, err := d.Diff(ctx, src, tgt)
	if err == nil {
		t.Fatal("expected error on cancelled context, got nil")
	}
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("expected context.Canceled, got %v", err)
	}
	// 取消时 ops 可能为空或部分，warns 可为空
	_ = ops
	_ = warns
}
```

注意：若 `differ_test.go` 未导入 `errors`，需在 import 块添加 `"errors"`。

- [ ] **步骤 2：运行测试验证失败**

运行：`rtk go test ./internal/diff/ -run TestDiffer_Diff_CancelledContext -v`
预期：FAIL，编译错误（`Diff` 返回 2 值，不能赋给 3 变量）

- [ ] **步骤 3：更新 `diffContext` 记录取消错误**

修改 `internal/diff/context.go`。在 `diffContext` 结构体增加 `cancelErr` 字段，`checkCancelled` 记录错误：

```go
package diff

import (
	"context"
	"fmt"
)

type diffContext struct {
	ctx       context.Context
	ops       []Operation
	warnings  []string
	cancelErr error
}

func newDiffContext(ctx context.Context) *diffContext {
	return &diffContext{
		ctx:       ctx,
		ops:       make([]Operation, 0, 16),
		warnings:  make([]string, 0, 4),
	}
}

func (c *diffContext) checkCancelled() error {
	if err := c.ctx.Err(); err != nil {
		c.cancelErr = err
		return err
	}
	return nil
}

func (c *diffContext) addOp(op Operation) {
	c.ops = append(c.ops, op)
}

func (c *diffContext) warnf(format string, args ...any) {
	c.warnings = append(c.warnings, fmt.Sprintf(format, args...))
}
```

- [ ] **步骤 4：更新 `Engine` 接口与 `Differ.Diff` 签名**

修改 `internal/diff/differ.go:11-33`：

```go
type Engine interface {
	Diff(ctx context.Context, source, target *model.Schema) ([]Operation, []string, error)
}

var _ Engine = (*Differ)(nil)

type Differ struct{}

func NewDiffer() *Differ {
	return &Differ{}
}

func (d *Differ) Diff(ctx context.Context, source, target *model.Schema) ([]Operation, []string, error) {
	c := newDiffContext(ctx)
	c.diffSchemas(source, target)
	if c.cancelErr != nil {
		return c.ops, c.warnings, c.cancelErr
	}
	return c.ops, c.warnings, nil
}
```

- [ ] **步骤 5：更新 `ComputeDiff` 处理 error**

修改 `internal/app/pipeline.go:130-131`：

```go
	differ := diff.NewDiffer()
	operations, warnings, err := differ.Diff(ctx, sourceClone, targetClone)
	if err != nil {
		return nil, warnings, fmt.Errorf("diff interrupted: %w", err)
	}
```

- [ ] **步骤 6：修复其他 `Diff` 调用点**

搜索所有调用 `.Diff(` 的位置并适配 3 返回值。已知位置：

- `internal/app/pipeline_test.go` — 若有直接调用 `differ.Diff`，改为接收 3 返回值
- `internal/diff/differ_test.go` — 现有测试中 `d.Diff(...)` 调用改为 3 返回值

运行搜索确认：`rtk grep "differ.Diff\|d\.Diff\|\.Diff(ctx" internal/`

对每个调用点，将：
```go
ops, warns := d.Diff(ctx, src, tgt)
```
改为：
```go
ops, warns, err := d.Diff(ctx, src, tgt)
if err != nil {
    t.Fatalf("Diff: %v", err)
}
```

- [ ] **步骤 7：运行测试验证通过**

运行：`rtk go test ./internal/diff/ ./internal/app/ -v`
预期：PASS

- [ ] **步骤 8：Commit**

```bash
git add internal/diff/differ.go internal/diff/context.go internal/diff/differ_test.go internal/app/pipeline.go internal/app/pipeline_test.go
git commit -m "fix(diff): Engine.Diff 增加 error 返回，传播 Context 取消

diffContext 记录取消错误，Differ.Diff 返回 error，ComputeDiff 遇取消
中断不再输出半成品迁移脚本。

Co-Authored-By: Oz <oz-agent@warp.dev>"
```

---

## 任务 3：enum 类型依赖排序

**文件：**
- 修改：`internal/diff/differ.go`、`internal/diff/operation_table.go`、`internal/diff/operation_column.go`
- 新建/修改：`internal/util/util.go`、`internal/util/util_test.go`
- 测试：`internal/diff/differ_test.go`、`internal/diff/operation_test.go`

### 背景

`diffNamespace` 中 `diffTables` 先于 `diffTypes`，且 `AddTableOp.DependsOn` 无列类型依赖。调整顺序 + 给 Op 增加列类型依赖，双重保障 `CREATE TYPE` 早于 `CREATE TABLE`。

- [ ] **步骤 1：编写 `IsBuiltinType` 失败测试**

检查 `internal/util/util_test.go` 是否存在；若不存在则创建。在文件中追加：

```go
func TestIsBuiltinType(t *testing.T) {
	builtins := []string{
		"integer", "int", "int4", "int8", "int2", "bigint", "smallint",
		"serial", "bigserial", "smallserial",
		"boolean", "bool", "text", "varchar", "character varying",
		"char", "character", "numeric", "real", "float4",
		"double precision", "float8",
		"json", "jsonb", "uuid", "inet", "cidr", "macaddr",
		"interval", "date", "time", "timetz", "timestamp", "timestamptz",
		"bytea", "money", "oid", "void", "name",
		"varchar(255)", "numeric(10,2)", "char(10)",
		"integer[]", "text[]", "varchar(100)[]",
		"timestamp without time zone", "timestamp with time zone",
		"INTEGER", "VarChar(50)",
	}
	for _, dt := range builtins {
		if !IsBuiltinType(dt) {
			t.Errorf("IsBuiltinType(%q) = false, want true", dt)
		}
	}

	customs := []string{
		"mood", "posint", "auth.mood", "my_type",
	}
	for _, dt := range customs {
		if IsBuiltinType(dt) {
			t.Errorf("IsBuiltinType(%q) = true, want false", dt)
		}
	}
}
```

- [ ] **步骤 2：运行测试验证失败**

运行：`rtk go test ./internal/util/ -run TestIsBuiltinType -v`
预期：FAIL，`IsBuiltinType` 未定义

- [ ] **步骤 3：实现 `IsBuiltinType`**

在 `internal/util/util.go` 末尾追加：

```go
var builtinTypeNames = map[string]bool{
	"int": true, "integer": true, "int4": true, "int8": true, "int2": true,
	"bigint": true, "smallint": true,
	"serial": true, "bigserial": true, "smallserial": true,
	"boolean": true, "bool": true,
	"text": true, "varchar": true, "character varying": true,
	"char": true, "character": true,
	"numeric": true, "decimal": true,
	"real": true, "float4": true, "double precision": true, "float8": true,
	"json": true, "jsonb": true,
	"uuid": true, "inet": true, "cidr": true, "macaddr": true, "macaddr8": true,
	"interval": true, "date": true,
	"time": true, "timetz": true,
	"timestamp": true, "timestamptz": true,
	"timestamp without time zone": true, "timestamp with time zone": true,
	"bytea": true, "money": true, "oid": true, "void": true, "name": true,
	"bpchar": true, "int2vector": true, "oidvector": true,
	"pg_node_tree": true, "pg_ddl_command": true, "pg_snapshot": true,
	"tsvector": true, "tsquery": true, "gtsvector": true,
	"xml": true, "point": true, "line": true, "lseg": true, "box": true,
	"path": true, "polygon": true, "circle": true,
}

// IsBuiltinType reports whether dt is a PostgreSQL built-in type.
// Handles length modifiers (varchar(N)), array suffixes (int[]),
// and case-insensitive matching.
func IsBuiltinType(dt string) bool {
	dt = strings.ToLower(strings.TrimSpace(dt))
	if dt == "" {
		return false
	}
	// Strip array suffix recursively
	if strings.HasSuffix(dt, "[]") {
		return IsBuiltinType(strings.TrimSuffix(dt, "[]"))
	}
	// Strip length modifier: varchar(255) -> varchar, numeric(10,2) -> numeric
	if idx := strings.IndexByte(dt, '('); idx > 0 {
		dt = dt[:idx]
	}
	return builtinTypeNames[dt]
}
```

- [ ] **步骤 4：运行测试验证通过**

运行：`rtk go test ./internal/util/ -run TestIsBuiltinType -v`
预期：PASS

- [ ] **步骤 5：编写 enum 排序 golden 测试**

在 `internal/diff/differ_test.go` 追加：

```go
func TestDiff_EnumBeforeTable(t *testing.T) {
	// 源为空，目标有 enum mood + 表 users(status mood)
	// 期望输出顺序：CREATE TYPE mood 在 CREATE TABLE users 之前
	src := model.NewSchema()

	tgt := model.NewSchema()
	tgtNs := model.NewNamespace("public")
	tgtNs.Types["mood"] = &model.EnumType{Name: "mood", Labels: []string{"happy", "sad"}}
	users := model.NewTable("public", "users")
	users.AddColumn(&model.Column{Name: "status", DataType: "mood", IsNullable: true})
	tgtNs.Tables["users"] = users
	tgt.Schemas["public"] = tgtNs

	d := NewDiffer()
	ops, _, err := d.Diff(context.Background(), src, tgt)
	if err != nil {
		t.Fatalf("Diff: %v", err)
	}

	// 找到 AddEnumType 和 AddTable 的位置
	enumIdx, tableIdx := -1, -1
	for i, op := range ops {
		switch op.Kind() {
		case KindAddEnumType:
			enumIdx = i
		case KindAddTable:
			tableIdx = i
		}
	}
	if enumIdx < 0 {
		t.Fatal("expected AddEnumType op")
	}
	if tableIdx < 0 {
		t.Fatal("expected AddTable op")
	}
	if enumIdx >= tableIdx {
		t.Errorf("enum (idx %d) should come before table (idx %d)", enumIdx, tableIdx)
	}
}
```

- [ ] **步骤 6：运行测试验证失败**

运行：`rtk go test ./internal/diff/ -run TestDiff_EnumBeforeTable -v`
预期：可能 FAIL（顺序不保证，取决于 diffNamespace 当前 tables 先于 types）

- [ ] **步骤 7：调整 `diffNamespace` 顺序**

修改 `internal/diff/differ.go:82-97`，将 `diffTypes` 移到 `diffTables` 之前：

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

- [ ] **步骤 8：实现 `columnTypeDependencies` 辅助函数**

在 `internal/diff/operation_column.go` 顶部 import 块添加 `"github.com/fred29910/migra-go/internal/util"`（若未导入），并在文件末尾追加：

```go
// columnTypeDependencies returns ObjectKeys for custom types referenced by a column.
// Built-in types return nil. Schema-qualified types (auth.mood) are split into
// schema + name. Non-qualified custom types use the given schema.
func columnTypeDependencies(schema string, col *model.Column) []model.ObjectKey {
	dt := col.DataType
	if dt == "" || util.IsBuiltinType(dt) {
		return nil
	}
	// Strip array suffix for dependency lookup
	base := dt
	if strings.HasSuffix(base, "[]") {
		base = strings.TrimSuffix(base, "[]")
		if util.IsBuiltinType(base) {
			return nil
		}
	}
	if idx := strings.LastIndex(base, "."); idx > 0 {
		return []model.ObjectKey{model.NewObjectKey(base[:idx], base[idx+1:], model.KindType)}
	}
	return []model.ObjectKey{model.NewObjectKey(schema, base, model.KindType)}
}
```

注意：`operation_column.go` 顶部 import 需添加 `"strings"`（若未导入）。

- [ ] **步骤 9：给 `AddTableOp.DependsOn` 增加列类型依赖**

修改 `internal/diff/operation_table.go:93-102`：

```go
func (op *AddTableOp) DependsOn() []model.ObjectKey {
	var deps []model.ObjectKey
	for _, constraint := range op.Table.Constraints {
		if constraint.Type == "foreign_key" && constraint.RefTable != "" {
			deps = append(deps, model.NewObjectKey(constraint.RefSchema, constraint.RefTable, model.KindTable))
		}
	}
	for _, col := range op.Table.Columns {
		deps = append(deps, columnTypeDependencies(op.Table.Schema, col)...)
	}
	return deps
}
```

- [ ] **步骤 10：给 `AddColumnOp` 和 `AlterColumnTypeOp` 增加列类型依赖**

修改 `internal/diff/operation_column.go:36-41`（AddColumnOp.DependsOn）：

```go
func (op *AddColumnOp) DependsOn() []model.ObjectKey {
	deps := []model.ObjectKey{
		model.NewObjectKey(op.Schema, op.Table, model.KindTable),
	}
	deps = append(deps, columnTypeDependencies(op.Schema, op.Column)...)
	return deps
}
```

修改 `internal/diff/operation_column.go:199-204`（AlterColumnTypeOp.DependsOn）。`AlterColumnTypeOp` 无 `*model.Column`，只有 `ToType` 字符串，构造临时 Column 复用 `columnTypeDependencies`：

```go
func (op *AlterColumnTypeOp) DependsOn() []model.ObjectKey {
	deps := []model.ObjectKey{
		model.NewObjectKey(op.Schema, op.Table, model.KindTable),
	}
	deps = append(deps, columnTypeDependencies(op.Schema, &model.Column{DataType: op.ToType})...)
	return deps
}
```

- [ ] **步骤 11：更新 `operation_test.go` 的 DependsOn 断言**

`TestOperationInterfaceHasDependsOn`（`internal/diff/operation_test.go:50-59`）断言 `AddColumnOp`/`AlterColumnTypeOp` 等恰好 1 个依赖。现在列类型依赖使其可能 >1。但这些测试构造的列用 `DataType: "email"`（非内置、非 schema-qualified），会被判定为自定义类型依赖。

修改 `internal/diff/operation_test.go:17` 的 `AddColumnOp` 构造，用内置类型避免引入类型依赖，使断言保持 1 个依赖：

```go
		NewAddColumnOp("public", "users", &model.Column{Name: "email", DataType: "varchar"}),
```

修改 `internal/diff/operation_test.go:18` 的 `AlterColumnTypeOp` 构造，`varchar(50)`/`varchar(100)` 都是内置类型，依赖数仍为 1，无需改动断言。

但 `AddTableOp`（第 15 行）用 `&model.Table{}`（无列），`DependsOn` 返回的外键依赖为 0、列类型依赖为 0，断言为 nil（第 83-88 行 default 分支）保持成立。

- [ ] **步骤 12：运行测试验证通过**

运行：`rtk go test ./internal/diff/ ./internal/util/ -v`
预期：PASS

- [ ] **步骤 13：Commit**

```bash
git add internal/diff/differ.go internal/diff/operation_table.go internal/diff/operation_column.go internal/diff/operation_test.go internal/diff/differ_test.go internal/util/util.go internal/util/util_test.go
git commit -m "fix(diff): enum 类型创建顺序先于引用它的表

调整 diffNamespace 顺序（types 先于 tables），并给 AddTableOp/
AddColumnOp/AlterColumnTypeOp 增加列类型依赖，使 DAG 排序保证
CREATE TYPE 早于 CREATE TABLE。新增 util.IsBuiltinType 区分内置
与自定义类型。

Co-Authored-By: Oz <oz-agent@warp.dev>"
```

---

## 任务 4：rename 启发式增加 --no-rename 开关与多候冲突回退

**文件：**
- 修改：`internal/diff/differ.go`、`internal/diff/context.go`、`internal/diff/diff_tables.go`
- 测试：`internal/diff/diff_tables_test.go`
- 修改：`internal/app/diff_service.go`、`internal/app/pipeline.go`
- 修改：`cmd/migra/diff.go`、`cmd/migra/push.go`、`cmd/migra/diff_runner.go`

### 背景

启发式把「删除+新增」误判为 rename。保留默认开启，新增 `--no-rename` 关闭，多候冲突回退 drop+add 并警告。

- [ ] **步骤 1：编写 noRename=true 的失败测试**

在 `internal/diff/diff_tables_test.go` 追加：

```go
func TestDiffColumns_NoRenameDisabled(t *testing.T) {
	// 源有列 a int，目标有列 b int（同签名）。noRename=true 时应输出 drop a + add b
	src := model.NewTable("public", "t")
	src.AddColumn(&model.Column{Name: "a", DataType: "integer", IsNullable: true})

	tgt := model.NewTable("public", "t")
	tgt.AddColumn(&model.Column{Name: "b", DataType: "integer", IsNullable: true})

	d := NewDiffer(WithNoRename(true))
	srcSchema := model.NewSchema()
	srcSchema.Schemas["public"] = model.NewNamespace("public")
	srcSchema.Schemas["public"].Tables["t"] = src
	tgtSchema := model.NewSchema()
	tgtSchema.Schemas["public"] = model.NewNamespace("public")
	tgtSchema.Schemas["public"].Tables["t"] = tgt

	ops, _, err := d.Diff(context.Background(), srcSchema, tgtSchema)
	if err != nil {
		t.Fatalf("Diff: %v", err)
	}

	var hasRename, hasDrop, hasAdd bool
	for _, op := range ops {
		switch op.Kind() {
		case KindRenameColumn:
			hasRename = true
		case KindDropColumn:
			hasDrop = true
		case KindAddColumn:
			hasAdd = true
		}
	}
	if hasRename {
		t.Error("expected no rename op when noRename=true")
	}
	if !hasDrop {
		t.Error("expected drop column op")
	}
	if !hasAdd {
		t.Error("expected add column op")
	}
}

func TestDiffColumns_AmbiguousSignatureFallback(t *testing.T) {
	// 源有列 a int，目标有列 b int + c int（同签名匹配 a）。
	// 多候冲突时应回退 drop+add 并输出 warning
	src := model.NewTable("public", "t")
	src.AddColumn(&model.Column{Name: "a", DataType: "integer", IsNullable: true})

	tgt := model.NewTable("public", "t")
	tgt.AddColumn(&model.Column{Name: "b", DataType: "integer", IsNullable: true})
	tgt.AddColumn(&model.Column{Name: "c", DataType: "integer", IsNullable: true})

	d := NewDiffer()
	srcSchema := model.NewSchema()
	srcSchema.Schemas["public"] = model.NewNamespace("public")
	srcSchema.Schemas["public"].Tables["t"] = src
	tgtSchema := model.NewSchema()
	tgtSchema.Schemas["public"] = model.NewNamespace("public")
	tgtSchema.Schemas["public"].Tables["t"] = tgt

	ops, warns, err := d.Diff(context.Background(), srcSchema, tgtSchema)
	if err != nil {
		t.Fatalf("Diff: %v", err)
	}

	var hasRename bool
	for _, op := range ops {
		if op.Kind() == KindRenameColumn {
			hasRename = true
		}
	}
	if hasRename {
		t.Error("expected no rename op on ambiguous signature")
	}
	// 应有 warning 提示冲突
	foundWarn := false
	for _, w := range warns {
		if strings.Contains(w, "ambiguous") {
			foundWarn = true
		}
	}
	if !foundWarn {
		t.Errorf("expected ambiguous warning, got %v", warns)
	}
}
```

注意：测试文件需导入 `"strings"`（若未导入）。

- [ ] **步骤 2：运行测试验证失败**

运行：`rtk go test ./internal/diff/ -run "TestDiffColumns_NoRenameDisabled|TestDiffColumns_AmbiguousSignatureFallback" -v`
预期：FAIL，`WithNoRename` 未定义，多候冲突未回退

- [ ] **步骤 3：给 `Differ` 增加 Option 模式与 noRename 字段**

修改 `internal/diff/differ.go` 的 `Differ` 定义与构造器（任务 2 已改为无参 `NewDiffer()`，现扩展为可变参数）：

```go
type Differ struct {
	noRename bool
}

type DifferOption func(*Differ)

// WithNoRename disables heuristic column rename detection.
func WithNoRename(v bool) DifferOption {
	return func(d *Differ) { d.noRename = v }
}

func NewDiffer(opts ...DifferOption) *Differ {
	d := &Differ{}
	for _, opt := range opts {
		opt(d)
	}
	return d
}

func (d *Differ) Diff(ctx context.Context, source, target *model.Schema) ([]Operation, []string, error) {
	c := newDiffContext(ctx, d)
	c.diffSchemas(source, target)
	if c.cancelErr != nil {
		return c.ops, c.warnings, c.cancelErr
	}
	return c.ops, c.warnings, nil
}
```

- [ ] **步骤 4：`diffContext` 增加 noRename 字段并透传**

修改 `internal/diff/context.go`：

```go
type diffContext struct {
	ctx       context.Context
	ops       []Operation
	warnings  []string
	cancelErr error
	noRename  bool
}

func newDiffContext(ctx context.Context, d *Differ) *diffContext {
	c := &diffContext{
		ctx:       ctx,
		ops:       make([]Operation, 0, 16),
		warnings:  make([]string, 0, 4),
	}
	if d != nil {
		c.noRename = d.noRename
	}
	return c
}
```

- [ ] **步骤 5：`diffTableColumns` 实现 noRename 跳过与多候冲突回退**

修改 `internal/diff/diff_tables.go:194-235`（`diffTableColumns` 函数体的 rename 检测部分）：

```go
func (c *diffContext) diffTableColumns(schema string, source, target *model.Table) {
	sourceOnlyNames := make([]string, 0, len(source.ColumnByName))
	for name := range source.ColumnByName {
		if _, exists := target.ColumnByName[name]; !exists {
			sourceOnlyNames = append(sourceOnlyNames, name)
		}
	}
	sort.Strings(sourceOnlyNames)

	targetOnlyNames := make([]string, 0, len(target.ColumnByName))
	for name := range target.ColumnByName {
		if _, exists := source.ColumnByName[name]; !exists {
			targetOnlyNames = append(targetOnlyNames, name)
		}
	}
	sort.Strings(targetOnlyNames)

	renamedSource := make(map[string]bool)
	renamedTarget := make(map[string]bool)

	// Heuristic rename detection: disabled when noRename is set.
	if !c.noRename && len(sourceOnlyNames) > 0 && len(targetOnlyNames) > 0 {
		// Build a map from column signature to target column name for O(n) lookup.
		targetBySig := make(map[string]string, len(targetOnlyNames))
		sigConflict := make(map[string]bool)
		for _, tgtName := range targetOnlyNames {
			tgtCol := target.ColumnByName[tgtName]
			sig := columnSignature(tgtCol)
			if _, found := targetBySig[sig]; found {
				sigConflict[sig] = true
				c.warnf("column rename ambiguous in %s.%s: signature %q matches multiple target columns, falling back to drop+add", schema, target.Name, sig)
			}
			targetBySig[sig] = tgtName
		}

		for _, srcName := range sourceOnlyNames {
			if err := c.checkCancelled(); err != nil {
				return
			}
			srcCol := source.ColumnByName[srcName]
			sig := columnSignature(srcCol)
			if tgtName, found := targetBySig[sig]; found && !sigConflict[sig] && !renamedTarget[tgtName] {
				c.addOp(NewRenameColumnOp(schema, target.Name, srcName, tgtName))
				renamedSource[srcName] = true
				renamedTarget[tgtName] = true
			}
		}
	}

	// Phase 2: Add columns that are truly new (not rename targets)
	addColNames := make([]string, 0, len(targetOnlyNames))
	for _, name := range targetOnlyNames {
		if !renamedTarget[name] {
			addColNames = append(addColNames, name)
		}
	}
	sort.Strings(addColNames)
	for _, name := range addColNames {
		col := target.ColumnByName[name]
		c.addOp(NewAddColumnOp(schema, target.Name, col))
	}

	// Phase 3: Drop columns that are truly removed (not rename sources)
	for name := range source.ColumnByName {
		if _, exists := target.ColumnByName[name]; !exists && !renamedSource[name] {
			c.addOp(NewDropColumnOp(schema, source.Name, name))
		}
	}

	// Phase 4: Compare columns that exist in both (unchanged)
	for _, targetCol := range target.Columns {
		if err := c.checkCancelled(); err != nil {
			return
		}
		if sourceCol, exists := source.ColumnByName[targetCol.Name]; exists {
			c.diffColumn(schema, target.Name, sourceCol, targetCol)
		}
	}
}
```

- [ ] **步骤 6：`DiffConfig` 增加 NoRename 字段**

修改 `internal/app/diff_service.go:14-23`：

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

- [ ] **步骤 7：`ComputeDiff` 透传 NoRename**

修改 `internal/app/pipeline.go:130`（任务 2 已改为 `diff.NewDiffer()`，现加 Option）：

```go
	differ := diff.NewDiffer(diff.WithNoRename(cfg.NoRename))
	operations, warnings, err := differ.Diff(ctx, sourceClone, targetClone)
	if err != nil {
		return nil, warnings, fmt.Errorf("diff interrupted: %w", err)
	}
```

- [ ] **步骤 8：diff 命令增加 --no-rename flag**

修改 `cmd/migra/diff.go` 的 `init()` 函数（在 `timeout` flag 之后添加）：

```go
	diffCmd.Flags().Bool("no-rename", false, "disable heuristic column rename detection")
	_ = viper.BindPFlag("diff.no_rename", diffCmd.Flags().Lookup("no-rename"))
```

修改 `cmd/migra/diff_runner.go` 的 `parseDiffConfig`，在读取 `timeout` 后添加：

```go
	noRename, err := cmd.Flags().GetBool("no-rename")
	if err != nil {
		return app.DiffConfig{}, fmt.Errorf("failed to get no-rename flag: %w", err)
	}
```

并在返回的 `app.DiffConfig{...}` 中添加字段 `NoRename: noRename,`。

- [ ] **步骤 9：push 命令增加 --no-rename flag**

修改 `cmd/migra/push.go` 的 `init()` 函数（在 `timeout` flag 之后添加）：

```go
	pushCmd.Flags().Bool("no-rename", false, "disable heuristic column rename detection")
	_ = viper.BindPFlag("diff.no_rename", pushCmd.Flags().Lookup("no-rename"))
```

修改 `cmd/migra/push.go` 的 `pushConfig` 结构体增加字段 `NoRename bool`，`parsePushConfig` 读取该 flag 并填充，`runPush` 中构造 `appCfg` 时添加 `NoRename: cfg.NoRename,`。

- [ ] **步骤 10：运行测试验证通过**

运行：`rtk go test ./internal/diff/ ./internal/app/ ./cmd/migra/ -v`
预期：PASS

- [ ] **步骤 11：Commit**

```bash
git add internal/diff/differ.go internal/diff/context.go internal/diff/diff_tables.go internal/diff/diff_tables_test.go internal/app/diff_service.go internal/app/pipeline.go cmd/migra/diff.go cmd/migra/push.go cmd/migra/diff_runner.go
git commit -m "feat(diff): rename 启发式增加 --no-rename 开关与多候冲突回退

Differ 用 Option 模式持有 noRename 配置，默认仍开启 rename 检测；
--no-rename 关闭时直接走 drop+add。多候签名冲突时回退 drop+add
并输出 warning，避免数据语义错误。

Co-Authored-By: Oz <oz-agent@warp.dev>"
```

---

## 任务 5：文件/目录 loader 增加 context 响应

**文件：**
- 修改：`internal/source/dir_loader.go`、`internal/source/sql_file_loader.go`
- 测试：`internal/source/dir_loader_test.go`、`internal/source/sql_file_loader_test.go`

### 背景

`DirectoryLoader` 的 `WalkDir`、逐文件 `ReadFile`，以及 `SQLFileLoader` 的 `os.ReadFile` 未检查 `ctx.Err()`，大目录/大文件下 CLI timeout 无法及时中断。

- [ ] **步骤 1：编写 `SQLFileLoader` 取消测试**

在 `internal/source/sql_file_loader_test.go` 追加：

```go
func TestSQLFileLoader_CancelledContext(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "s.sql")
	if err := os.WriteFile(path, []byte("CREATE TABLE t (id int);"), 0644); err != nil {
		t.Fatal(err)
	}
	l := &SQLFileLoader{}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	_, _, err := l.Load(ctx, path, LoadOptions{})
	if err == nil {
		t.Fatal("expected error on cancelled context, got nil")
	}
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("expected context.Canceled, got %v", err)
	}
}
```

注意：测试文件需导入 `"context"`、`"errors"`、`"os"`、`"path/filepath"`（检查现有 import）。

- [ ] **步骤 2：编写 `DirectoryLoader` 取消测试**

在 `internal/source/dir_loader_test.go` 追加：

```go
func TestDirectoryLoader_CancelledContext(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "a.sql"), []byte("CREATE TABLE a (id int);"), 0644); err != nil {
		t.Fatal(err)
	}
	l := &DirectoryLoader{}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	_, _, err := l.Load(ctx, dir, LoadOptions{})
	if err == nil {
		t.Fatal("expected error on cancelled context, got nil")
	}
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("expected context.Canceled, got %v", err)
	}
}
```

- [ ] **步骤 3：运行测试验证失败**

运行：`rtk go test ./internal/source/ -run "CancelledContext" -v`
预期：FAIL，loader 未检查 ctx

- [ ] **步骤 4：`SQLFileLoader.Load` 增加 ctx 检查**

修改 `internal/source/sql_file_loader.go:28-34`，在 `os.ReadFile` 前添加 ctx 检查：

```go
func (l *SQLFileLoader) Load(ctx context.Context, source string, opt LoadOptions) (*model.Schema, []error, error) {
	if err := ctx.Err(); err != nil {
		return nil, nil, err
	}
	path := source
	if strings.HasPrefix(strings.ToLower(source), "file://") {
		path = source[len("file://"):]
	}
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to read file %s: %w", path, err)
	}
	// ...其余不变
```

- [ ] **步骤 5：`DirectoryLoader.Load` 增加 ctx 检查**

修改 `internal/source/dir_loader.go`。在 `WalkDir` 回调开头与逐文件读取循环中添加 ctx 检查：

```go
func (l *DirectoryLoader) Load(ctx context.Context, source string, opt LoadOptions) (*model.Schema, []error, error) {
	if err := ctx.Err(); err != nil {
		return nil, nil, err
	}
	sourcePath := stripFileScheme(source)

	var files []string
	err := filepath.WalkDir(sourcePath, func(path string, d os.DirEntry, err error) error {
		if err := ctx.Err(); err != nil {
			return err
		}
		if err != nil {
			return err
		}
		if strings.HasPrefix(d.Name(), ".") {
			if d.IsDir() {
				return filepath.SkipDir
			}
			return nil
		}
		if !d.IsDir() && strings.HasSuffix(strings.ToLower(path), ".sql") {
			files = append(files, path)
		}
		return nil
	})
	if err != nil {
		return nil, nil, fmt.Errorf("failed to scan directory %s: %w", sourcePath, err)
	}

	sort.Strings(files)

	if len(files) == 0 {
		return model.NewSchema(), []error{fmt.Errorf("no .sql files found in directory: %s", sourcePath)}, nil
	}

	var combinedSQL strings.Builder
	for _, file := range files {
		if err := ctx.Err(); err != nil {
			return nil, nil, err
		}
		data, readErr := os.ReadFile(file)
		if readErr != nil {
			return nil, nil, fmt.Errorf("failed to read %s: %w", file, readErr)
		}
		if combinedSQL.Len() > 0 {
			combinedSQL.WriteString("\n")
		}
		combinedSQL.Write(data)
	}
	// ...其余不变
```

- [ ] **步骤 6：运行测试验证通过**

运行：`rtk go test ./internal/source/ -v`
预期：PASS

- [ ] **步骤 7：Commit**

```bash
git add internal/source/dir_loader.go internal/source/sql_file_loader.go internal/source/dir_loader_test.go internal/source/sql_file_loader_test.go
git commit -m "fix(source): 文件/目录 loader 增加 context 取消检查

WalkDir 回调、逐文件 ReadFile 循环、SQLFileLoader 读前均检查
ctx.Err()，使 CLI timeout 能及时中断大目录/大文件加载。

Co-Authored-By: Oz <oz-agent@warp.dev>"
```

---

## 任务 6：同步 DDL.md 与 README.md 文档

**文件：**
- 修改：`docs/DDL.md`、`README.md`

### 背景

`docs/DDL.md` 限制 #6 描述的 `character(n)` → `char(n)` 映射缺失问题已在 `util.go:37` 修复，文档未同步。同时补充本次新增的 `--no-rename` 标志说明。

- [ ] **步骤 1：删除 DDL.md 限制 #6 并更新类型映射表**

修改 `docs/DDL.md`：

1. 类型映射表第 9 行（约第 175 行），将：
```
| `character(n)` | `char(n)` / `character(n)` | ⚠️ |
```
改为：
```
| `character(n)` | `char(n)` | ✅ |
```

2. 删除限制 #6 整条（约第 209-210 行），即「**`character(n)` / `char(n)` 同义映射缺失**」整段。

3. 后续限制编号顺延：原 #7 → #6，#8 → #7，以此类推（更新所有后续编号）。

- [ ] **步骤 2：更新 DDL.md 最后更新日期**

修改 `docs/DDL.md` 第 4 行：
```
> **最后更新**: 2026-06-19
```

- [ ] **步骤 3：DDL.md 限制（原 #8，现 #7）补充 --no-rename 说明**

在 rename 启发式限制段落补充：
```
可通过 `--no-rename` 标志禁用启发式检测（回退为 drop+add）。当多列签名匹配同一源列（多候冲突）时，自动回退 drop+add 并输出警告。
```

- [ ] **步骤 4：DDL.md 命令行参数表新增 --no-rename 行**

在命令行参数表（`--strict` 行之后）新增：
```
| `--no-rename` | diff, push | 禁用列重命名启发式检测（回退为 drop+add） | `false` |
```

- [ ] **步骤 5：README.md 命令行参数表新增 --no-rename 行**

修改 `README.md` 命令行参数表（约第 153 行 `--strict` 行之后）新增：
```
| `--no-rename` | diff, push | 禁用列重命名启发式检测 | `false` |
```

- [ ] **步骤 6：Commit**

```bash
git add docs/DDL.md README.md
git commit -m "docs: 同步 DDL 矩阵与 README，删除已修复限制 #6，补充 --no-rename

character(n)→char(n) 映射已实现，移除过时限制。新增 --no-rename
标志说明。

Co-Authored-By: Oz <oz-agent@warp.dev>"
```

---

## 最终验证

- [ ] **步骤 1：全量构建**

运行：`rtk go build ./...`
预期：成功，无输出

- [ ] **步骤 2：全量静态检查**

运行：`rtk go vet ./...`
预期：No issues found

- [ ] **步骤 3：全量测试**

运行：`rtk go test ./...`
预期：全绿，测试数 ≥ 854（新增用例后应更多）

- [ ] **步骤 4：竞态检查**

运行：`go test -race ./internal/diff/ ./internal/source/ ./internal/app/`
预期：全绿

- [ ] **步骤 5：确认无遗漏的 Diff 调用点**

运行：`rtk grep "\.Diff(ctx" internal/ cmd/`
预期：所有调用均接收 3 返回值

如有未适配的调用点，修复后重新运行步骤 1-3。
