# COLLATE 列支持实现计划

> **面向 AI 代理的工作者：** 必需子技能：使用 superpowers:subagent-driven-development（推荐）或 superpowers:executing-plans 逐任务实现此计划。步骤使用复选框（`- [ ]`）语法来跟踪进度。

**目标：** 为 migra-go 的列模型添加 COLLATE 子句支持，包括模型字段、DDL 解析中的 collation 提取、渲染 COLLATE 输出以及列 collation 变更的差异检测。

**架构：** 在 `model.Column` 中新增 `Collation string` 字段；`parserutil.ParseColumnDef` 从 pg_query_go v6 的 `ColumnDef.CollClause` 中提取排序规则名称；`render.renderAddTable`/`renderAddColumn` 在列定义中输出 `COLLATE "name"`；`diff_columns.go` 检测 collation 变更并生成 `AlterColumnCollationOp`。

**技术栈：** Go 1.26、pganalyze/pg_query_go/v6 v6.2.2（protobuf 生成的 AST 节点）

**pg_query_go v6 API 参考：**
- `ColumnDef` 结构：
  - `CollClause *CollateClause` — COLLATE 子句（有值时非 nil）
  - `CollOid uint32` — 排序规则的 OID
- `CollateClause` 结构：
  - `Collname []*Node` — 排序规则名称各部分（每个节点是 `String_`，`.S` 为名称片段）
  - 如 `COLLATE "en_US.UTF-8"` 则 `Collname[0].GetString_().S == "en_US.UTF-8"`

---

## 文件结构与职责

| 文件 | 职责 | 改动类型 |
|------|------|---------|
| `internal/model/column.go` | Column 模型 | 新增 `Collation string` 字段 |
| `internal/parser/parserutil/util.go` | ParseColumnDef 函数 | 提取 collation 值 |
| `internal/parser/create_table_handler.go` | CREATE TABLE handler | 无改动（自动通过 ParseColumnDef 传递） |
| `internal/parser/alter_table_handler.go` | ALTER TABLE handler | 无改动（自动通过 ParseColumnDef 传递） |
| `internal/render/render.go` | SQL 渲染 | renderAddTable / renderAddColumn 输出 COLLATE 子句 |
| `internal/diff/operation.go` | 差异操作定义 | 新增 `AlterColumnCollationOp` 与 `KindAlterColumnCollation` |
| `internal/diff/diff_columns.go` | 列差异检测 | 新增 collation 变更检测逻辑 |
| `internal/introspect/tables.go` | 数据库内省 | 从 `information_schema.columns` 查询列 collation |
| `internal/parser/handler_test.go` | Handler 测试 | 新增 collation 解析测试用例 |
| `internal/render/render_test.go` | 渲染测试 | 新增 COLLATE 渲染测试用例 |
| `internal/diff/differ_test.go` | Diff 测试 | 新增 collation 变更差异检测测试用例 |

---

## 任务 1：Column 模型添加 Collation 字段

**文件：**
- 修改：`internal/model/column.go`

- [ ] **步骤 1：在 Column 结构体中添加 `Collation string` 字段**

COLLATE 子句是可选的，空字符串 `""` 表示使用默认排序规则（无 COLLATE 子句）。这与 `model.IndexElem.Collation` 的现有模式一致。

```go
// Column represents a table column
type Column struct {
	Name         string
	DataType     string
	IsNullable   bool
	DefaultExpr  *string
	IsIdentity   bool
	IdentityKind string // "ALWAYS" or "BY DEFAULT"
	Collation    string // collation name, empty = default collation
}
```

- [ ] **步骤 2：运行编译验证**

```bash
go build ./internal/model/...
```

预期：编译通过，无错误。

- [ ] **步骤 3：Commit**

```bash
git add internal/model/column.go
git commit -m "feat(model): 添加 Collation 字段到 Column 模型"
```

---

## 任务 2：ParseColumnDef 提取 collation

**文件：**
- 修改：`internal/parser/parserutil/util.go`

**关键知识：**
- `colDef.CollClause` 为 `*pg_query.CollateClause`，为 nil 时表示无 COLLATE
- `colDef.CollClause.Collname` 为 `[]*pg_query.Node`，每个节点是 `*pg_query.Node{Node: &pg_query.Node_String_{String_: &pg_query.String_{S: "name"}}}`
- 多段 collation 名称（如 `"pg_catalog"."default"`）会被拆分为多个元素，通常只需用 `.` 连接

- [ ] **步骤 1：提取 collation 名称辅助函数**

在 `util.go` 的 `MapTypeName` 函数（或文件末尾）之前添加：

```go
// ExtractCollation extracts the collation name from a ColumnDef's CollClause.
// Returns empty string if no collation is specified.
func ExtractCollation(colDef *pg_query.ColumnDef) string {
	if colDef.CollClause == nil {
		return ""
	}
	parts := make([]string, 0, len(colDef.CollClause.Collname))
	for _, item := range colDef.CollClause.Collname {
		if s := item.GetString_(); s != nil {
			parts = append(parts, s.S)
		}
	}
	return strings.Join(parts, ".")
}
```

注意：辅助函数的命名空间是 `parserutil` 包，仅在包内使用。也可保持为私有函数（小写），但为了可测试性保留为大写。

- [ ] **步骤 2：在 ParseColumnDef 中调用 ExtractCollation**

修改 `ParseColumnDef`，在设置 `DefaultExpr` 之后添加 collation 提取：

```go
// ParseColumnDef extracts a model.Column from pg_query ColumnDef node.
func ParseColumnDef(colDef *pg_query.ColumnDef) *model.Column {
	col := &model.Column{IsNullable: !colDef.IsNotNull}
	if colDef.Colname != "" {
		col.Name = colDef.Colname
	}
	if colDef.TypeName != nil {
		col.DataType = ParseTypeName(colDef.TypeName)
	}
	for _, item := range colDef.Constraints {
		if c := item.GetConstraint(); c != nil {
			switch c.Contype {
			case pg_query.ConstrType_CONSTR_NOTNULL:
				col.IsNullable = false
			case pg_query.ConstrType_CONSTR_DEFAULT:
				if c.RawExpr != nil {
					if expr, ok := ParseExpression(c.RawExpr); ok {
						col.DefaultExpr = &expr
					}
				}
			}
		}
	}
	col.Collation = ExtractCollation(colDef)
	return col
}
```

- [ ] **步骤 3：运行编译验证**

```bash
go build ./internal/parser/...
```

预期：编译通过。

- [ ] **步骤 4：Commit**

```bash
git add internal/parser/parserutil/util.go
git commit -m "feat(parser): 从 ColumnDef 提取 COLLATE 排序规则名称"
```

---

## 任务 3：Handler 测试 — 验证 COLLATE 解析

**文件：**
- 修改：`internal/parser/handler_test.go`

- [ ] **步骤 1：添加 CREATE TABLE 带 COLLATE 的解析测试**

在 `TestCreateTableHandler` 之后新增：

```go
func TestCreateTableHandler_WithCollation(t *testing.T) {
	node := mustParseFirstStmt(t, `CREATE TABLE users (name text COLLATE "en_US.UTF-8")`)
	h := &CreateTableHandler{}

	mutations, err := h.Handle(node)
	require.NoError(t, err)
	require.Len(t, mutations, 1)

	mut, ok := mutations[0].(CreateTableMutation)
	require.True(t, ok)
	require.Len(t, mut.Columns, 1)
	assert.Equal(t, "name", mut.Columns[0].Name)
	assert.Equal(t, "text", mut.Columns[0].DataType)
	assert.Equal(t, "en_US.UTF-8", mut.Columns[0].Collation)
}
```

- [ ] **步骤 2：添加无 COLLATE 和带 NOT NULL 的边界测试**

```go
func TestCreateTableHandler_WithCollationAndNotNull(t *testing.T) {
	node := mustParseFirstStmt(t, `CREATE TABLE t (s text COLLATE "de_DE" NOT NULL)`)
	h := &CreateTableHandler{}

	mutations, err := h.Handle(node)
	require.NoError(t, err)
	require.Len(t, mutations, 1)

	mut, ok := mutations[0].(CreateTableMutation)
	require.True(t, ok)
	require.Len(t, mut.Columns, 1)
	assert.Equal(t, "s", mut.Columns[0].Name)
	assert.Equal(t, "de_DE", mut.Columns[0].Collation)
	assert.False(t, mut.Columns[0].IsNullable)
}

func TestCreateTableHandler_WithoutCollation(t *testing.T) {
	node := mustParseFirstStmt(t, `CREATE TABLE users (name text)`)
	h := &CreateTableHandler{}

	mutations, err := h.Handle(node)
	require.NoError(t, err)
	require.Len(t, mutations, 1)

	mut, ok := mutations[0].(CreateTableMutation)
	require.True(t, ok)
	require.Len(t, mut.Columns, 1)
	assert.Equal(t, "name", mut.Columns[0].Name)
	assert.Equal(t, "", mut.Columns[0].Collation, "expected empty collation for default")
}
```

- [ ] **步骤 3：添加 ALTER TABLE ADD COLUMN 带 COLLATE 的解析测试**

```go
func TestAlterTableHandler_AddColumnWithCollation(t *testing.T) {
	node := mustParseFirstStmt(t, `ALTER TABLE users ADD COLUMN full_name text COLLATE "en_US.UTF-8"`)
	h := &AlterTableHandler{}

	mutations, err := h.Handle(node)
	require.NoError(t, err)
	require.Len(t, mutations, 1)

	mut, ok := mutations[0].(AddColumnMutation)
	require.True(t, ok)
	assert.Equal(t, "full_name", mut.Column.Name)
	assert.Equal(t, "en_US.UTF-8", mut.Column.Collation)
}

func TestAlterTableHandler_AddColumnWithoutCollation(t *testing.T) {
	node := mustParseFirstStmt(t, `ALTER TABLE users ADD COLUMN bio text`)
	h := &AlterTableHandler{}

	mutations, err := h.Handle(node)
	require.NoError(t, err)
	require.Len(t, mutations, 1)

	mut, ok := mutations[0].(AddColumnMutation)
	require.True(t, ok)
	assert.Equal(t, "", mut.Column.Collation, "expected empty collation for default")
}
```

- [ ] **步骤 4：运行测试验证**

```bash
go test ./internal/parser/... -v -run "WithCollation|WithoutCollation|AddColumn"
```

预期：所有测试通过，包括新增的 collation 测试。

- [ ] **步骤 5：Commit**

```bash
git add internal/parser/handler_test.go
git commit -m "test(parser): 增加 COLLATE 排序规则解析测试"
```

---

## 任务 4：渲染 COLLATE 子句 — renderAddTable 与 renderAddColumn

**文件：**
- 修改：`internal/render/render.go`

**渲染规则：**
- PostgreSQL 中列定义的语法顺序：`column_name type COLLATE "collation" NOT NULL DEFAULT expr`
- COLLATE 必须出现在类型之后、可选的 NOT NULL 和 DEFAULT 之前
- 仅当 `col.Collation != ""` 时才输出 COLLATE 子句

- [ ] **步骤 1：在 renderAddTable 中添加 COLLATE 输出**

修改 `renderAddTable` 中的列定义构建逻辑（当前的第 144 行附近）：

```go
for _, col := range table.Columns {
	colDef := fmt.Sprintf("    %s %s", quoteIdentifier(col.Name), col.DataType)
	if col.Collation != "" {
		colDef += fmt.Sprintf(" COLLATE %s", quoteIdentifier(col.Collation))
	}
	if !col.IsNullable {
		colDef += " NOT NULL"
	}
	if col.DefaultExpr != nil {
		colDef += fmt.Sprintf(" DEFAULT %s", *col.DefaultExpr)
	}
	lines = append(lines, colDef)
}
```

- [ ] **步骤 2：在 renderAddColumn 中添加 COLLATE 输出**

修改 `renderAddColumn`（当前的第 196-208 行附近）：

```go
func (r *Renderer) renderAddColumn(op *diff.AddColumnOp) string {
	col := op.Column
	sql := fmt.Sprintf("ALTER TABLE %s ADD COLUMN %s %s",
		quoteQualifiedIdentifier(op.Schema, op.Table), quoteIdentifier(col.Name), col.DataType)

	if col.Collation != "" {
		sql += fmt.Sprintf(" COLLATE %s", quoteIdentifier(col.Collation))
	}
	if !col.IsNullable {
		sql += " NOT NULL"
	}
	if col.DefaultExpr != nil {
		sql += fmt.Sprintf(" DEFAULT %s", *col.DefaultExpr)
	}

	return fmt.Sprintf("-- op: add_column risk:low\n%s;", sql)
}
```

- [ ] **步骤 3：运行编译验证**

```bash
go build ./internal/render/...
```

预期：编译通过。

- [ ] **步骤 4：Commit**

```bash
git add internal/render/render.go
git commit -m "feat(render): 渲染 CREATE TABLE 和 ADD COLUMN 的 COLLATE 子句"
```

---

## 任务 5：渲染测试 — 验证 COLLATE 输出

**文件：**
- 修改：`internal/render/render_test.go`

- [ ] **步骤 1：添加 renderAddTable 带 COLLATE 的测试**

```go
func TestRenderAddTable_WithCollation(t *testing.T) {
	table := model.NewTable("public", "users")
	table.AddColumn(&model.Column{Name: "id", DataType: "integer", IsNullable: false})
	table.AddColumn(&model.Column{Name: "name", DataType: "text", IsNullable: true, Collation: "en_US.UTF-8"})
	table.AddColumn(&model.Column{Name: "label", DataType: "varchar(50)", IsNullable: false, Collation: "de_DE"})
	table.PrimaryKey = &model.PrimaryKey{Name: "users_pkey", Columns: []string{"id"}}

	sql := NewRenderer().Render(diff.NewAddTableOp("public", "users", table))
	for _, want := range []string{
		`"name" text COLLATE "en_US.UTF-8"`,
		`"label" character varying(50) COLLATE "de_DE" NOT NULL`,
		`"id" integer NOT NULL`,
	} {
		if !strings.Contains(sql, want) {
			t.Errorf("expected SQL to contain %q, got:\n%s", want, sql)
		}
	}
}
```

- [ ] **步骤 2：添加 renderAddColumn 带 COLLATE 的测试**

```go
func TestRenderAddColumn_WithCollation(t *testing.T) {
	col := &model.Column{Name: "full_name", DataType: "text", IsNullable: true, Collation: "en_US.UTF-8"}
	op := diff.NewAddColumnOp("public", "users", col)

	sql := NewRenderer().Render(op)
	want := `ALTER TABLE "public"."users" ADD COLUMN "full_name" text COLLATE "en_US.UTF-8";`
	if !strings.Contains(sql, want) {
		t.Errorf("expected SQL to contain %q, got:\n%s", want, sql)
	}

	// Also test without collation (no regression)
	col2 := &model.Column{Name: "age", DataType: "integer", IsNullable: false}
	op2 := diff.NewAddColumnOp("public", "users", col2)
	sql2 := NewRenderer().Render(op2)
	if strings.Contains(sql2, "COLLATE") {
		t.Errorf("expected no COLLATE for default collation, got:\n%s", sql2)
	}
}
```

- [ ] **步骤 3：添加 COLLATE + DEFAULT + NOT NULL 的组合测试**

```go
func TestRenderAddTable_WithCollationDefaultNotNull(t *testing.T) {
	defaultExpr := "current_timestamp"
	table := model.NewTable("public", "logs")
	table.AddColumn(&model.Column{
		Name:       "ts",
		DataType:   "timestamptz",
		IsNullable: false,
		DefaultExpr: &defaultExpr,
	})
	table.AddColumn(&model.Column{
		Name:       "message",
		DataType:   "text",
		IsNullable: false,
		Collation:  "en_US.UTF-8",
	})

	sql := NewRenderer().Render(diff.NewAddTableOp("public", "logs", table))
	for _, want := range []string{
		`"message" text COLLATE "en_US.UTF-8" NOT NULL`,
		`"ts" timestamptz NOT NULL DEFAULT current_timestamp`,
	} {
		if !strings.Contains(sql, want) {
			t.Errorf("expected SQL to contain %q, got:\n%s", want, sql)
		}
	}
}
```

- [ ] **步骤 4：运行测试验证**

```bash
go test ./internal/render/... -v -run "WithCollation"
```

预期：所有测试通过。

- [ ] **步骤 5：运行完整渲染测试确保无回归**

```bash
go test ./internal/render/... -v
```

预期：所有测试通过（包括现有测试）。

- [ ] **步骤 6：Commit**

```bash
git add internal/render/render_test.go
git commit -m "test(render): 增加 COLLATE 子句渲染测试"
```

---

## 任务 6：Diff 操作定义 — AlterColumnCollationOp

**文件：**
- 修改：`internal/diff/operation.go`

**设计说明：**
- 新增 `KindAlterColumnCollation` 操作类型
- `AlterColumnCollationOp` 包含 schema、table、column、dataType、from/to collation
- 渲染时输出 `ALTER TABLE t ALTER COLUMN c SET DATA TYPE same_type COLLATE "coll"`
- PostgreSQL 不提供单独的 `ALTER COLLATION` 语法——必须通过 `SET DATA TYPE` 变通

- [ ] **步骤 1：添加 KindAlterColumnCollation 常量**

在现有的 Kind 常量块中，`KindDropDefault` 之后添加：

```go
const (
	// ... 现有常量 ...
	KindSetDefault           Kind = "set_default"
	KindDropDefault          Kind = "drop_default"
	KindAlterColumnCollation Kind = "alter_column_collation"
)
```

- [ ] **步骤 2：添加 AlterColumnCollationOp 结构体**

在 `DropDefaultOp`（或 `DropColumnOp`）之后添加：

```go
// AlterColumnCollationOp represents changing a column's collation
type AlterColumnCollationOp struct {
	baseOperation
	Schema        string
	Table         string
	Column        string
	DataType      string // the column's data type (needed for SET DATA TYPE ... COLLATE)
	FromCollation string
	ToCollation   string
}

func NewAlterColumnCollationOp(schema, table, column, dataType, fromCollation, toCollation string) *AlterColumnCollationOp {
	return &AlterColumnCollationOp{
		baseOperation: baseOperation{
			kind:      KindAlterColumnCollation,
			objectKey: model.NewObjectKey(schema, table+"."+column, model.KindColumn),
		},
		Schema:        schema,
		Table:         table,
		Column:        column,
		DataType:      dataType,
		FromCollation: fromCollation,
		ToCollation:   toCollation,
	}
}

func (op *AlterColumnCollationOp) IsDestructive() bool {
	return false
}

func (op *AlterColumnCollationOp) DependsOn() []model.ObjectKey {
	return []model.ObjectKey{
		model.NewObjectKey(op.Schema, op.Table, model.KindTable),
	}
}
```

- [ ] **步骤 3：运行编译验证**

```bash
go build ./internal/diff/...
```

预期：编译通过。

- [ ] **步骤 4：Commit**

```bash
git add internal/diff/operation.go
git commit -m "feat(diff): 新增 AlterColumnCollationOp 操作定义"
```

---

## 任务 7：Diff 列 collation 变更检测

**文件：**
- 修改：`internal/diff/diff_columns.go`

- [ ] **步骤 1：在 diffColumn 中添加 collation 变更检测**

修改 `diffColumn` 函数，在 DEFAULT 检测之后添加 collation 比较：

```go
// diffColumn compares two columns and generates operations for differences
func (c *diffContext) diffColumn(schema, table string, source, target *model.Column) {
	// Check for data type change
	if source.DataType != target.DataType {
		c.addOp(NewAlterColumnTypeOp(schema, table, source.Name, source.DataType, target.DataType))
	}

	// Check for nullable change
	if source.IsNullable && !target.IsNullable {
		c.addOp(NewSetNotNullOp(schema, table, source.Name))
	} else if !source.IsNullable && target.IsNullable {
		c.addOp(NewDropNotNullOp(schema, table, source.Name))
	}

	// Check for default expression change
	if !sameDefault(source.DefaultExpr, target.DefaultExpr) {
		if target.DefaultExpr == nil {
			c.addOp(NewDropDefaultOp(schema, table, source.Name))
		} else {
			c.addOp(NewSetDefaultOp(schema, table, source.Name, *target.DefaultExpr))
		}
	}

	// Check for collation change
	if source.Collation != target.Collation {
		dataType := target.DataType
		if dataType == "" {
			dataType = source.DataType
		}
		c.addOp(NewAlterColumnCollationOp(schema, table, source.Name, dataType, source.Collation, target.Collation))
	}
}
```

- [ ] **步骤 2：运行编译验证**

```bash
go build ./internal/diff/...
```

预期：编译通过。

- [ ] **步骤 3：Commit**

```bash
git add internal/diff/diff_columns.go
git commit -m "feat(diff): 检测列 collation 变更并生成 AlterColumnCollationOp"
```

---

## 任务 8：渲染 AlterColumnCollationOp

**文件：**
- 修改：`internal/render/render.go`

- [ ] **步骤 1：在 Render 的 switch 中添加新 case**

在 `renderAddColumn` case 之后或 `DropDefaultOp` case 之后添加：

```go
func (r *Renderer) Render(op diff.Operation) string {
	switch v := op.(type) {
	// ... 现有 cases ...
	case *diff.DropDefaultOp:
		// existing code
	case *diff.AlterColumnCollationOp:
		return r.renderAlterColumnCollation(v)
	// ... 其余 cases ...
	}
}
```

- [ ] **步骤 2：实现 renderAlterColumnCollation 方法**

在 `renderDropDefault` 方法之后添加：

```go
func (r *Renderer) renderAlterColumnCollation(op *diff.AlterColumnCollationOp) string {
	sql := fmt.Sprintf("ALTER TABLE %s ALTER COLUMN %s SET DATA TYPE %s",
		quoteQualifiedIdentifier(op.Schema, op.Table),
		quoteIdentifier(op.Column),
		op.DataType,
	)
	if op.ToCollation != "" {
		sql += fmt.Sprintf(" COLLATE %s", quoteIdentifier(op.ToCollation))
	}
	return fmt.Sprintf("-- op: alter_column_collation risk:low\n%s;", sql)
}
```

- [ ] **步骤 3：运行编译验证**

```bash
go build ./internal/render/...
```

预期：编译通过。

- [ ] **步骤 4：添加 render 测试**

在 `render_test.go` 中新增测试：

```go
func TestRenderAlterColumnCollation(t *testing.T) {
	r := NewRenderer()

	t.Run("add collation", func(t *testing.T) {
		op := diff.NewAlterColumnCollationOp("public", "users", "name", "text", "", "en_US.UTF-8")
		sql := r.Render(op)
		want := `ALTER TABLE "public"."users" ALTER COLUMN "name" SET DATA TYPE text COLLATE "en_US.UTF-8";`
		if !strings.Contains(sql, want) {
			t.Errorf("expected SQL to contain %q, got:\n%s", want, sql)
		}
	})

	t.Run("remove collation", func(t *testing.T) {
		op := diff.NewAlterColumnCollationOp("public", "users", "name", "text", "en_US.UTF-8", "")
		sql := r.Render(op)
		if strings.Contains(sql, "COLLATE") {
			t.Errorf("expected no COLLATE when removing collation, got:\n%s", sql)
		}
		if !strings.Contains(sql, "SET DATA TYPE text") {
			t.Errorf("expected SET DATA TYPE, got:\n%s", sql)
		}
	})
}
```

- [ ] **步骤 5：运行测试验证**

```bash
go test ./internal/render/... -v -run "Collation"
```

预期：所有测试通过。

- [ ] **步骤 6：Commit**

```bash
git add internal/render/render.go internal/render/render_test.go
git commit -m "feat(render): 渲染 ALTER COLUMN COLLATE 变更"
```

---

## 任务 9：Diff 测试 — 验证 collation 变更检测

**文件：**
- 修改：`internal/diff/differ_test.go`

- [ ] **步骤 1：添加 collation 新增、删除、无变更的三个测试**

```go
func TestDiffer_DetectsColumnCollationChange(t *testing.T) {
	source := model.NewSchema()
	sourceNs := source.GetOrCreateNamespace("public")
	sourceTable := model.NewTable("public", "users")
	sourceTable.AddColumn(&model.Column{Name: "name", DataType: "text", IsNullable: true, Collation: ""})
	sourceNs.Tables["users"] = sourceTable

	target := model.NewSchema()
	targetNs := target.GetOrCreateNamespace("public")
	targetTable := model.NewTable("public", "users")
	targetTable.AddColumn(&model.Column{Name: "name", DataType: "text", IsNullable: true, Collation: "en_US.UTF-8"})
	targetNs.Tables["users"] = targetTable

	ops, warnings := NewDiffer().Diff(source, target)
	if len(warnings) != 0 {
		t.Fatalf("unexpected warnings: %#v", warnings)
	}

	found := false
	for _, op := range ops {
		if op.Kind() == KindAlterColumnCollation {
			found = true
			collOp, ok := op.(*AlterColumnCollationOp)
			if !ok {
				t.Fatal("expected AlterColumnCollationOp type")
			}
			if collOp.Column != "name" {
				t.Errorf("expected column 'name', got %q", collOp.Column)
			}
			if collOp.ToCollation != "en_US.UTF-8" {
				t.Errorf("expected to collation 'en_US.UTF-8', got %q", collOp.ToCollation)
			}
		}
	}
	if !found {
		t.Fatal("expected AlterColumnCollationOp in diff output, but not found")
	}
}

func TestDiffer_DetectsColumnCollationRemoval(t *testing.T) {
	source := model.NewSchema()
	sourceNs := source.GetOrCreateNamespace("public")
	sourceTable := model.NewTable("public", "users")
	sourceTable.AddColumn(&model.Column{Name: "name", DataType: "text", IsNullable: true, Collation: "en_US.UTF-8"})
	sourceNs.Tables["users"] = sourceTable

	target := model.NewSchema()
	targetNs := target.GetOrCreateNamespace("public")
	targetTable := model.NewTable("public", "users")
	targetTable.AddColumn(&model.Column{Name: "name", DataType: "text", IsNullable: true, Collation: ""})
	targetNs.Tables["users"] = targetTable

	ops, warnings := NewDiffer().Diff(source, target)
	if len(warnings) != 0 {
		t.Fatalf("unexpected warnings: %#v", warnings)
	}

	found := false
	for _, op := range ops {
		if op.Kind() == KindAlterColumnCollation {
			found = true
			collOp := op.(*AlterColumnCollationOp)
			if collOp.FromCollation != "en_US.UTF-8" {
				t.Errorf("expected from collation 'en_US.UTF-8', got %q", collOp.FromCollation)
			}
			if collOp.ToCollation != "" {
				t.Errorf("expected empty to collation, got %q", collOp.ToCollation)
			}
		}
	}
	if !found {
		t.Fatal("expected AlterColumnCollationOp for removal, but not found")
	}
}

func TestDiffer_NoCollationDifferenceWhenSame(t *testing.T) {
	source := model.NewSchema()
	sourceNs := source.GetOrCreateNamespace("public")
	sourceTable := model.NewTable("public", "users")
	sourceTable.AddColumn(&model.Column{Name: "name", DataType: "text", IsNullable: true, Collation: "en_US.UTF-8"})
	sourceNs.Tables["users"] = sourceTable

	target := model.NewSchema()
	targetNs := target.GetOrCreateNamespace("public")
	targetTable := model.NewTable("public", "users")
	targetTable.AddColumn(&model.Column{Name: "name", DataType: "text", IsNullable: true, Collation: "en_US.UTF-8"})
	targetNs.Tables["users"] = targetTable

	ops, warnings := NewDiffer().Diff(source, target)
	if len(warnings) != 0 {
		t.Fatalf("unexpected warnings: %#v", warnings)
	}

	for _, op := range ops {
		if op.Kind() == KindAlterColumnCollation {
			t.Fatal("expected NO AlterColumnCollationOp when collation is the same")
		}
	}
}
```

- [ ] **步骤 2：运行测试验证**

```bash
go test ./internal/diff/... -v -run "Collation"
```

预期：所有测试通过。

- [ ] **步骤 3：运行完整 diff 测试确保无回归**

```bash
go test ./internal/diff/... -v
```

预期：所有测试通过。

- [ ] **步骤 4：Commit**

```bash
git add internal/diff/differ_test.go
git commit -m "test(diff): 增加列 collation 变更检测测试"
```

---

## 任务 10：数据库内省 — 从 pg_catalog 加载 collation

**文件：**
- 修改：`internal/introspect/tables.go`

**设计说明：**
- `information_schema.columns` 提供了 `collation_name` 列
- 对于未设置 collation 的列，该值为 NULL
- 需要使用 `COALESCE` 转换为空字符串以匹配 `model.Column.Collation` 的约定

- [ ] **步骤 1：更新 SQL 查询以包含 collation_name**

修改 `loadTables` 中的 SQL 查询：

```go
query := `
	SELECT 
		t.table_name,
		c.column_name,
		c.data_type,
		c.character_maximum_length,
		c.is_nullable,
		c.column_default,
		c.ordinal_position,
		COALESCE(c.collation_name, '') AS collation_name
	FROM information_schema.tables t
	JOIN information_schema.columns c ON t.table_name = c.table_name AND t.table_schema = c.table_schema
	WHERE t.table_schema = $1 AND t.table_type = 'BASE TABLE'
	ORDER BY t.table_name, c.ordinal_position`
```

- [ ] **步骤 2：更新 Scan 代码以读取 collation_name**

添加 `collationName string` 到 Scan 目标：

```go
var (
	tableName     string
	colName       string
	dataType      string
	charMaxLen    sql.NullInt64
	isNullable    string
	colDefault    sql.NullString
	ordinalPos    int
	collationName string
)
```

更新 `rows.Scan` 调用以包含 `collationName`：

```go
err := rows.Scan(&tableName, &colName, &dataType, &charMaxLen, &isNullable, &colDefault, &ordinalPos, &collationName)
```

更新 `model.Column` 构造：

```go
col := &model.Column{
	Name:       colName,
	DataType:   dataType,
	IsNullable: isNullable == "YES",
	Collation:  collationName,
}
```

- [ ] **步骤 3：运行编译验证**

```bash
go build ./internal/introspect/...
```

预期：编译通过。

- [ ] **步骤 4：运行集成测试（需要数据库连接）**

```bash
go test ./internal/introspect/... -v
```

预期：测试通过（若无集成测试则只检查编译）。

- [ ] **步骤 5：Commit**

```bash
git add internal/introspect/tables.go
git commit -m "feat(introspect): 从 pg_catalog 加载列 collation 信息"
```

---

## 任务 11：全量验证

- [ ] **步骤 1：运行全部测试**

```bash
go test ./... -v
```

- [ ] **步骤 2：运行 go vet**

```bash
go vet ./...
```

- [ ] **步骤 3：运行 lint**

```bash
make lint
```

- [ ] **步骤 4：最终 Commit**

```bash
git add -A
git commit -m "feat: 添加 COLLATE 列支持

- Column 模型新增 Collation 字段
- ParseColumnDef 从 ColumnDef.CollClause 提取排序规则名称
- CREATE TABLE 和 ADD COLUMN 渲染 COLLATE 子句
- 新增 AlterColumnCollationOp 差异操作与渲染
- Diff 引擎检测列 collation 变更
- 数据库内省从 information_schema 加载 collation_name
- 全链路解析、渲染、差异检测的单元测试覆盖"
```

---

## 回滚方案

若实现过程中出现问题，可以使用以下命令回滚到初始状态：

```bash
git log --oneline -10
# 找到实现前的最后一个 commit
git reset --hard <last_commit_hash>
```

## 未包含在本次计划中的内容

1. **索引元素 COLLATE**：`model.IndexElem` 已有 `Collation` 字段，但 parser/render 尚未完整支持。这是独立的工作项。
2. **ALTER COLUMN 的独立 COLLATE 语法**：PostgreSQL 不支持单独的 `ALTER COLUMN c COLLATE "x"`，必须通过 `SET DATA TYPE` 实现。本计划已正确处理。
3. **表达式级 COLLATE**：如 `WHERE name COLLATE "C" = 'foo'`。不属于列定义范畴。
4. **Collation OID 验证**：pg_query_go v6 中 `ColumnDef.CollOid` 提供 OID，但在 parser 阶段不做跨数据库的 OID 解析。
