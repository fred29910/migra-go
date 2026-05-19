# RENAME COLUMN 实现计划

> **面向 AI 代理的工作者：** 必需子技能：使用 superpowers:subagent-driven-development（推荐）或 superpowers:executing-plans 逐任务实现此计划。步骤使用复选框（`- [ ]`）语法来跟踪进度。

**目标：** 在 migra-go 中实现完整的 RENAME COLUMN 支持，使得：
1. `ALTER TABLE ... RENAME COLUMN old_name TO new_name` 能被解析器正确解析为 SchemaMutation 并应用
2. Diff 引擎能启发式检测列重命名（而非输出 DROP + ADD）
3. 渲染器能输出正确的 `ALTER TABLE ... RENAME COLUMN` SQL
4. 完整测试覆盖

**关键发现：** 在 pg_query_go v6.2.2 中，`ALTER TABLE ... RENAME COLUMN` **不被解析为 AlterTableStmt**，而是被解析为 **`RenameStmt`**（`rename_type: OBJECT_COLUMN, subname: 旧列名, newname: 新列名`）。这意味着需要新增针对 `Node_RenameStmt` 的 Handler，而非在 `alter_table_handler.go` 中追加 `AT_RenameColumn` case（该 case 在 pg_query_go v6 protobuf 中不存在）。

**架构：**

```
RenameStmt (AST Node_RenameStmt)
    → RenameStmtHandler (新增，注册到 DefaultRegistry for Node_RenameStmt)
    → RenameColumnMutation (Apply: 更新 ColumnByName map + Column.Name)
    → model.Schema 正确应用

Diff 引擎 diffTableColumns
    → 启发式检测: isColumnRenameCandidate (类型+可空+默认值匹配)
    → RenameColumnOp (非破坏性, DependsOn: table)
    → Renderer.renderRenameColumn
    → SQL: ALTER TABLE "schema"."table" RENAME COLUMN "old" TO "new";
```

**技术栈：** Go 1.26+, pg_query_go v6.2.2, testify v1.11+

---

## 文件结构

| 操作 | 文件 | 变更内容 |
|------|------|----------|
| 修改 | `internal/parser/mutation.go` | 新增 `MutKindRenameColumn` 常量 |
| 创建 | `internal/parser/rename_column_mutation.go` | `RenameColumnMutation` 定义 + Apply |
| 创建 | `internal/parser/rename_stmt_handler.go` | `RenameStmtHandler`（处理 `Node_RenameStmt`） |
| 修改 | `internal/parser/registry.go` | 注册 `Node_RenameStmt` → `RenameStmtHandler` |
| 修改 | `internal/diff/operation.go` | 新增 `KindRenameColumn` 常量 |
| 创建 | `internal/diff/rename_column_op.go` | `RenameColumnOp` 定义 |
| 修改 | `internal/diff/diff_tables.go` | `diffTableColumns` 中添加重命名启发式 |
| 修改 | `internal/render/render.go` | `Render` switch 添加 `RenameColumnOp` case + `renderRenameColumn` 方法 |
| 修改 | `internal/plan/plan.go` | 将 `KindRenameColumn` 加入 deploy 阶段 |
| 创建 | `internal/parser/rename_handler_test.go` | RenameStmtHandler 单元测试 + 集成测试 |
| 创建 | `internal/parser/rename_mutation_test.go` | RenameColumnMutation Apply 测试 |
| 创建 | `internal/diff/rename_column_op_test.go` | RenameColumnOp 元数据 + DependsOn 测试 |
| 创建 | `internal/diff/diff_rename_column_test.go` | Diff 引擎重命名启发式检测测试 |
| 创建 | `internal/render/rename_render_test.go` | RENAME COLUMN SQL 渲染测试 |

---

## 任务

### 任务 1：MutKindRenameColumn 常量

**文件：**
- 修改：`internal/parser/mutation.go:12-24`

- [ ] **步骤 1：在 const 块中添加 MutKindRenameColumn**

在 `mutation.go` 的 const 块末尾（`MutKindCreateSchema` 之后）添加：

```go
	MutKindRenameColumn    MutationKind = "rename_column"
```

- [ ] **步骤 2：验证编译**

运行：`go build ./internal/parser/...`
预期：编译成功

- [ ] **步骤 3：Commit**

```bash
git add internal/parser/mutation.go
git commit -m "feat: add MutKindRenameColumn mutation kind constant"
```

---

### 任务 2：RenameColumnMutation 定义与 Apply 实现

**文件：**
- 创建：`internal/parser/rename_column_mutation.go`
- 创建：`internal/parser/rename_mutation_test.go`

- [ ] **步骤 1：编写失败的测试**

```go
// internal/parser/rename_mutation_test.go
package parser

import (
	"testing"

	"github.com/fred29910/migra-go/internal/model"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestRenameColumnMutation_Kind(t *testing.T) {
	mut := RenameColumnMutation{Schema: "public", Table: "users", OldName: "username", NewName: "login_name"}
	assert.Equal(t, MutKindRenameColumn, mut.Kind())
}

func TestRenameColumnMutation_Target(t *testing.T) {
	mut := RenameColumnMutation{Schema: "public", Table: "users", OldName: "username", NewName: "login_name"}
	key := mut.Target()
	assert.Equal(t, "public", key.Schema)
	assert.Equal(t, "users.login_name", key.Name)
	assert.Equal(t, model.KindColumn, key.Kind)
}

func TestRenameColumnMutation_Apply_RenamesColumn(t *testing.T) {
	schema := model.NewSchema()
	ns := schema.GetOrCreateNamespace("public")
	table := model.NewTable("public", "users")
	table.AddColumn(&model.Column{Name: "username", DataType: "text", IsNullable: true})
	table.AddColumn(&model.Column{Name: "email", DataType: "text"})
	ns.Tables["users"] = table

	mut := RenameColumnMutation{
		Schema:  "public",
		Table:   "users",
		OldName: "username",
		NewName: "login_name",
	}
	err := mut.Apply(schema)
	require.NoError(t, err)

	col := table.ColumnByName["login_name"]
	require.NotNil(t, col, "new column name should exist in ColumnByName")
	assert.Equal(t, "login_name", col.Name)
	assert.Nil(t, table.ColumnByName["username"], "old column name should be removed from ColumnByName")
	assert.Len(t, table.Columns, 2)
	assert.Equal(t, "login_name", table.Columns[0].Name)
	assert.Equal(t, "email", table.Columns[1].Name)
	assert.Equal(t, "text", col.DataType)
	assert.True(t, col.IsNullable)
}

func TestRenameColumnMutation_Apply_TableNotFound(t *testing.T) {
	schema := model.NewSchema()
	schema.GetOrCreateNamespace("public")
	mut := RenameColumnMutation{Schema: "public", Table: "nonexistent", OldName: "x", NewName: "y"}
	err := mut.Apply(schema)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "not found")
}

func TestRenameColumnMutation_Apply_ColumnNotFound(t *testing.T) {
	schema := model.NewSchema()
	ns := schema.GetOrCreateNamespace("public")
	ns.Tables["users"] = model.NewTable("public", "users")
	mut := RenameColumnMutation{Schema: "public", Table: "users", OldName: "nope", NewName: "y"}
	err := mut.Apply(schema)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "not found")
}

func TestRenameColumnMutation_Apply_NameCollision(t *testing.T) {
	schema := model.NewSchema()
	ns := schema.GetOrCreateNamespace("public")
	table := model.NewTable("public", "users")
	table.AddColumn(&model.Column{Name: "username", DataType: "text"})
	table.AddColumn(&model.Column{Name: "email", DataType: "text"})
	ns.Tables["users"] = table

	mut := RenameColumnMutation{Schema: "public", Table: "users", OldName: "username", NewName: "email"}
	err := mut.Apply(schema)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "already exists")
}

func TestRenameColumnMutation_Apply_NilNamespace(t *testing.T) {
	schema := model.NewSchema()
	mut := RenameColumnMutation{Schema: "public", Table: "users", OldName: "x", NewName: "y"}
	err := mut.Apply(schema)
	require.Error(t, err)
}
```

运行：`go test ./internal/parser/... -run TestRenameColumnMutation -count=1`
预期：FAIL（RenameColumnMutation 未定义）

- [ ] **步骤 2：实现 RenameColumnMutation**

```go
// internal/parser/rename_column_mutation.go
package parser

import (
	"fmt"

	"github.com/fred29910/migra-go/internal/model"
)

// RenameColumnMutation describes renaming a column in a table.
type RenameColumnMutation struct {
	Schema  string
	Table   string
	OldName string
	NewName string
}

func (m RenameColumnMutation) Kind() MutationKind { return MutKindRenameColumn }
func (m RenameColumnMutation) Target() model.ObjectKey {
	return model.NewObjectKey(m.Schema, m.Table+"."+m.NewName, model.KindColumn)
}
func (m RenameColumnMutation) Apply(schema *model.Schema) error {
	ns := schema.GetNamespace(m.Schema)
	if ns == nil {
		return fmt.Errorf("schema %s not found", m.Schema)
	}
	table, exists := ns.Tables[m.Table]
	if !exists {
		return fmt.Errorf("table %s.%s not found", m.Schema, m.Table)
	}
	col := table.ColumnByName[m.OldName]
	if col == nil {
		return fmt.Errorf("column %s.%s.%s not found", m.Schema, m.Table, m.OldName)
	}
	if _, exists := table.ColumnByName[m.NewName]; exists {
		return fmt.Errorf("column %s.%s.%s already exists", m.Schema, m.Table, m.NewName)
	}
	col.Name = m.NewName
	delete(table.ColumnByName, m.OldName)
	table.ColumnByName[m.NewName] = col
	return nil
}
```

- [ ] **步骤 3：运行测试验证通过**

运行：`go test ./internal/parser/... -run TestRenameColumnMutation -count=1 -v`
预期：6 个测试全部 PASS

- [ ] **步骤 4：Commit**

```bash
git add internal/parser/rename_column_mutation.go internal/parser/rename_mutation_test.go
git commit -m "feat: implement RenameColumnMutation with Apply method"
```

---

### 任务 3：RenameStmtHandler 解析 RENAME COLUMN AST

**文件：**
- 创建：`internal/parser/rename_stmt_handler.go`
- 创建：`internal/parser/rename_handler_test.go`
- 修改：`internal/parser/registry.go`

- [ ] **步骤 1：编写失败的测试**

```go
// internal/parser/rename_handler_test.go
package parser

import (
	"testing"

	pg_query "github.com/pganalyze/pg_query_go/v6"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestRenameStmtHandler_Handle_RenameColumn(t *testing.T) {
	node := mustParseFirstStmt(t, "ALTER TABLE users RENAME COLUMN sku TO item_code")
	h := &RenameStmtHandler{}
	mutations, err := h.Handle(node)
	require.NoError(t, err)
	require.Len(t, mutations, 1)

	mut, ok := mutations[0].(RenameColumnMutation)
	require.True(t, ok)
	assert.Equal(t, "users", mut.Table)
	assert.Equal(t, "public", mut.Schema)
	assert.Equal(t, "sku", mut.OldName)
	assert.Equal(t, "item_code", mut.NewName)
}

func TestRenameStmtHandler_Handle_RenameColumn_SchemaQualified(t *testing.T) {
	node := mustParseFirstStmt(t, "ALTER TABLE inventory.items RENAME COLUMN sku TO item_code")
	h := &RenameStmtHandler{}
	mutations, err := h.Handle(node)
	require.NoError(t, err)
	require.Len(t, mutations, 1)

	mut, ok := mutations[0].(RenameColumnMutation)
	require.True(t, ok)
	assert.Equal(t, "items", mut.Table)
	assert.Equal(t, "inventory", mut.Schema)
	assert.Equal(t, "sku", mut.OldName)
	assert.Equal(t, "item_code", mut.NewName)
}

func TestRenameStmtHandler_Handle_WrongNodeType(t *testing.T) {
	node := mustParseFirstStmt(t, "CREATE TABLE users (id integer)")
	h := &RenameStmtHandler{}
	_, err := h.Handle(node)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "expected RenameStmt")
}

func TestRenameStmtHandler_RenameTypeCheck(t *testing.T) {
	node := mustParseFirstStmt(t, "ALTER TABLE users RENAME COLUMN sku TO item_code")
	stmt := node.GetRenameStmt()
	require.NotNil(t, stmt)
	assert.Equal(t, pg_query.ObjectType_OBJECT_COLUMN, stmt.RenameType)
	assert.Equal(t, "sku", stmt.Subname)
	assert.Equal(t, "item_code", stmt.Newname)
}

func TestRenameStmtHandler_Integration(t *testing.T) {
	parser := NewParser()
	schema, err := parser.ParseSQL(`
		CREATE TABLE users (id integer, username text);
		ALTER TABLE users RENAME COLUMN username TO login_name;
	`)
	require.NoError(t, err)

	ns := schema.GetNamespace("public")
	require.NotNil(t, ns)
	table := ns.Tables["users"]
	require.NotNil(t, table)
	require.False(t, table.IsPlaceholder)

	assert.Nil(t, table.ColumnByName["username"])
	col := table.ColumnByName["login_name"]
	require.NotNil(t, col)
	assert.Equal(t, "login_name", col.Name)
	assert.Equal(t, "text", col.DataType)
	assert.True(t, col.IsNullable)
	assert.Len(t, table.Columns, 2)
	assert.Equal(t, "id", table.Columns[0].Name)
	assert.Equal(t, "login_name", table.Columns[1].Name)
}

func TestRenameStmtHandler_Integration_Placeholder(t *testing.T) {
	// RENAME on a non-existent table should create a placeholder
	// with the rename applied (column doesn't exist on placeholder)
	parser := NewParser()
	schema, err := parser.ParseSQL("ALTER TABLE users RENAME COLUMN old_name TO new_name;")
	require.NoError(t, err)
	ns := schema.GetNamespace("public")
	require.NotNil(t, ns)
	table := ns.Tables["users"]
	require.NotNil(t, table)
	assert.True(t, table.IsPlaceholder)
	assert.Nil(t, table.ColumnByName["old_name"])
	assert.Nil(t, table.ColumnByName["new_name"])
}
```

运行：`go test ./internal/parser/... -run TestRenameStmtHandler -count=1`
预期：FAIL（RenameStmtHandler 未定义）

- [ ] **步骤 2：实现 RenameStmtHandler**

```go
// internal/parser/rename_stmt_handler.go
package parser

import (
	"fmt"

	"github.com/fred29910/migra-go/internal/parser/parserutil"
	pg_query "github.com/pganalyze/pg_query_go/v6"
)

// RenameStmtHandler handles RENAME statements, specifically RENAME COLUMN.
// In pg_query_go v6, ALTER TABLE ... RENAME COLUMN is parsed as a RenameStmt
// (not as AlterTableStmt), with rename_type=OBJECT_COLUMN.
type RenameStmtHandler struct{}

// Handle converts a pg_query RenameStmt into schema mutations.
// Only handles RENAME COLUMN (rename_type == OBJECT_COLUMN).
func (h *RenameStmtHandler) Handle(node *pg_query.Node) ([]SchemaMutation, error) {
	stmt := node.GetRenameStmt()
	if stmt == nil {
		return nil, fmt.Errorf("RenameStmtHandler: expected RenameStmt, got %T", node)
	}
	if stmt.RenameType != pg_query.ObjectType_OBJECT_COLUMN {
		return nil, nil
	}
	tableName, schemaName := parserutil.ParseRelation(stmt.Relation)
	if stmt.Subname == "" {
		return nil, fmt.Errorf("RENAME COLUMN missing old column name")
	}
	if stmt.Newname == "" {
		return nil, fmt.Errorf("RENAME COLUMN missing new column name")
	}
	return []SchemaMutation{
		RenameColumnMutation{
			Schema:  schemaName,
			Table:   tableName,
			OldName: stmt.Subname,
			NewName: stmt.Newname,
		},
	}, nil
}
```

- [ ] **步骤 3：注册到 DefaultRegistry**

修改 `internal/parser/registry.go` 的 `DefaultRegistry()`：

```go
// 在 CreateSchemaHandler 注册之后添加：
	r.Register(&pg_query.Node{Node: &pg_query.Node_RenameStmt{}}, &RenameStmtHandler{})
```

- [ ] **步骤 4：运行测试验证通过**

运行：`go test ./internal/parser/... -run TestRenameStmtHandler -count=1 -v`
预期：6 个测试全部 PASS

- [ ] **步骤 5：运行全部解析器测试确认无回归**

运行：`go test ./internal/parser/... -count=1`
预期：全部 PASS

- [ ] **步骤 6：Commit**

```bash
git add internal/parser/rename_stmt_handler.go internal/parser/rename_handler_test.go internal/parser/registry.go
git commit -m "feat: add RenameStmtHandler for ALTER TABLE RENAME COLUMN (Node_RenameStmt)"
```

---

### 任务 4：RenameColumnOp 操作类型

**文件：**
- 修改：`internal/diff/operation.go`
- 创建：`internal/diff/rename_column_op.go`
- 创建：`internal/diff/rename_column_op_test.go`
- 修改：`internal/diff/operation_test.go`

- [ ] **步骤 1：在 operation.go 中添加 KindRenameColumn 常量**

在 `internal/diff/operation.go` 的 const 块末尾（`KindDropIdentity` 之后）添加：

```go
	KindDropIdentity         Kind = "drop_identity"
	KindRenameColumn         Kind = "rename_column"
```

- [ ] **步骤 2：编写失败的测试**

```go
// internal/diff/rename_column_op_test.go
package diff

import (
	"testing"

	"github.com/fred29910/migra-go/internal/model"
)

func TestRenameColumnOp_Metadata(t *testing.T) {
	op := NewRenameColumnOp("public", "users", "old_name", "new_name")
	if op.Kind() != KindRenameColumn {
		t.Fatalf("expected KindRenameColumn, got %s", op.Kind())
	}
	if op.IsDestructive() {
		t.Fatal("RenameColumnOp should not be destructive")
	}
	expectedKey := model.ObjectKey{Schema: "public", Kind: model.KindColumn, Name: "users.old_name"}
	if op.ObjectKey() != expectedKey {
		t.Fatalf("expected object key %v, got %v", expectedKey, op.ObjectKey())
	}
}

func TestRenameColumnOp_DependsOn(t *testing.T) {
	op := NewRenameColumnOp("public", "users", "old_name", "new_name")
	deps := op.DependsOn()
	if len(deps) != 1 {
		t.Fatalf("expected 1 dependency, got %d", len(deps))
	}
	expectedDep := model.ObjectKey{Schema: "public", Kind: model.KindTable, Name: "users"}
	if deps[0] != expectedDep {
		t.Fatalf("expected dependency %v, got %v", expectedDep, deps[0])
	}
}

func TestRenameColumnOp_Fields(t *testing.T) {
	op := NewRenameColumnOp("public", "users", "old_name", "new_name")
	if op.Schema != "public" || op.Table != "users" || op.OldName != "old_name" || op.NewName != "new_name" {
		t.Fatalf("unexpected fields: %+v", op)
	}
}
```

运行：`go test ./internal/diff/... -run TestRenameColumnOp -count=1`
预期：FAIL（RenameColumnOp 未定义）

- [ ] **步骤 3：实现 RenameColumnOp**

```go
// internal/diff/rename_column_op.go
package diff

import "github.com/fred29910/migra-go/internal/model"

// RenameColumnOp represents renaming a column.
type RenameColumnOp struct {
	baseOperation
	Schema  string
	Table   string
	OldName string
	NewName string
}

func NewRenameColumnOp(schema, table, oldName, newName string) *RenameColumnOp {
	return &RenameColumnOp{
		baseOperation: baseOperation{
			kind:      KindRenameColumn,
			objectKey: model.NewObjectKey(schema, table+"."+oldName, model.KindColumn),
		},
		Schema:  schema,
		Table:   table,
		OldName: oldName,
		NewName: newName,
	}
}

func (op *RenameColumnOp) IsDestructive() bool { return false }

func (op *RenameColumnOp) DependsOn() []model.ObjectKey {
	return []model.ObjectKey{
		model.NewObjectKey(op.Schema, op.Table, model.KindTable),
	}
}
```

- [ ] **步骤 4：更新 operation_test.go**

在 `internal/diff/operation_test.go` 中，将 `RenameColumnOp` 加入：
1. `TestOperationInterfaceHasDependsOn` 的 ops 列表
2. 对应的 switch case

```go
// ops 列表追加：
		NewRenameColumnOp("public", "users", "old_name", "new_name"),

// 第 43 行的 case 列表追加：
		*RenameColumnOp,
```

- [ ] **步骤 5：运行测试验证通过**

运行：`go test ./internal/diff/... -run "TestRenameColumnOp|TestOperationInterfaceHasDependsOn" -count=1 -v`
预期：PASS

- [ ] **步骤 6：Commit**

```bash
git add internal/diff/operation.go internal/diff/rename_column_op.go internal/diff/rename_column_op_test.go internal/diff/operation_test.go
git commit -m "feat: add KindRenameColumn and RenameColumnOp"
```

---

### 任务 5：SQL 渲染 RENAME COLUMN

**文件：**
- 修改：`internal/render/render.go`
- 创建：`internal/render/rename_render_test.go`

- [ ] **步骤 1：编写失败的测试**

```go
// internal/render/rename_render_test.go
package render

import (
	"strings"
	"testing"

	"github.com/fred29910/migra-go/internal/diff"
)

func TestRenderRenameColumn(t *testing.T) {
	r := NewRenderer()
	op := diff.NewRenameColumnOp("public", "users", "username", "login_name")
	sql := r.Render(op)
	expected := `ALTER TABLE "public"."users" RENAME COLUMN "username" TO "login_name";`
	if !strings.Contains(sql, expected) {
		t.Errorf("expected SQL to contain %q, got:\n%s", expected, sql)
	}
	if !strings.Contains(sql, "rename_column") {
		t.Errorf("expected operation tag rename_column, got:\n%s", sql)
	}
}

func TestRenderRenameColumn_RiskLow(t *testing.T) {
	r := NewRenderer()
	op := diff.NewRenameColumnOp("public", "items", "sku", "item_code")
	sql := r.Render(op)
	if !strings.Contains(sql, "risk:low") {
		t.Errorf("RENAME COLUMN should be risk:low, got:\n%s", sql)
	}
}

func TestRenderRenameColumn_SchemaQualified(t *testing.T) {
	r := NewRenderer()
	op := diff.NewRenameColumnOp("inventory", "items", "sku", "item_code")
	sql := r.Render(op)
	expected := `ALTER TABLE "inventory"."items" RENAME COLUMN "sku" TO "item_code";`
	if !strings.Contains(sql, expected) {
		t.Errorf("expected SQL to contain %q, got:\n%s", expected, sql)
	}
}

func TestRenderRenameColumn_ReservedWordNames(t *testing.T) {
	r := NewRenderer()
	op := diff.NewRenameColumnOp("public", "users", "order", "sort_order")
	sql := r.Render(op)
	expected := `RENAME COLUMN "order" TO "sort_order"`
	if !strings.Contains(sql, expected) {
		t.Errorf("expected quoted names, got:\n%s", sql)
	}
}
```

运行：`go test ./internal/render/... -run TestRenderRenameColumn -count=1`
预期：FAIL（走 default → "Unknown operation"）

- [ ] **步骤 2：在 Render 函数中添加 RenameColumnOp 分支**

在 `internal/render/render.go` 的 `Render` switch 中添加：

```go
	case *diff.RenameColumnOp:
		return r.renderRenameColumn(v)
```

- [ ] **步骤 3：实现 renderRenameColumn 方法**

在 `internal/render/render.go` 中添加（放在 `renderDropColumn` 之后）：

```go
func (r *Renderer) renderRenameColumn(op *diff.RenameColumnOp) string {
	return fmt.Sprintf("-- op: rename_column risk:low\nALTER TABLE %s RENAME COLUMN %s TO %s;",
		quoteQualifiedIdentifier(op.Schema, op.Table),
		quoteIdentifier(op.OldName),
		quoteIdentifier(op.NewName),
	)
}
```

- [ ] **步骤 4：运行测试验证通过**

运行：`go test ./internal/render/... -run TestRenderRenameColumn -count=1 -v`
预期：4 个测试全部 PASS

- [ ] **步骤 5：运行全部 render 测试确认无回归**

运行：`go test ./internal/render/... -count=1`
预期：PASS

- [ ] **步骤 6：Commit**

```bash
git add internal/render/render.go internal/render/rename_render_test.go
git commit -m "feat: add RENAME COLUMN SQL rendering"
```

---

### 任务 6：Diff 引擎列重命名启发式检测

**文件：**
- 修改：`internal/diff/diff_tables.go`
- 创建：`internal/diff/diff_rename_column_test.go`

**设计：**

在 `diffTableColumns` 中新增三个阶段：

```
Phase 1: Detect renames
  遍历 sourceOnly 列和 targetOnly 列的笛卡尔积
  isColumnRenameCandidate 检查条件：
    1. DataType 相同
    2. IsNullable 相同
    3. DefaultExpr 相同 (sameDefault)
  匹配 → 标记为 rename（从 drop/add 列表中排除）

Phase 2: 处理真·新增列（排除 rename targets）

Phase 3: 处理真·删除列（排除 rename sources）

Phase 4: 对比同名列（现有逻辑不变）
```

- [ ] **步骤 1：编写失败的测试**

```go
// internal/diff/diff_rename_column_test.go
package diff

import (
	"testing"

	"github.com/fred29910/migra-go/internal/model"
)

func TestDiffer_DetectsColumnRename(t *testing.T) {
	source := model.NewSchema()
	sourceNs := source.GetOrCreateNamespace("public")
	sourceTable := model.NewTable("public", "users")
	sourceTable.AddColumn(&model.Column{Name: "id", DataType: "integer", IsNullable: false})
	sourceTable.AddColumn(&model.Column{Name: "username", DataType: "text", IsNullable: true})
	sourceTable.AddColumn(&model.Column{Name: "email", DataType: "text", IsNullable: false})
	sourceNs.Tables["users"] = sourceTable

	target := model.NewSchema()
	targetNs := target.GetOrCreateNamespace("public")
	targetTable := model.NewTable("public", "users")
	targetTable.AddColumn(&model.Column{Name: "id", DataType: "integer", IsNullable: false})
	targetTable.AddColumn(&model.Column{Name: "login_name", DataType: "text", IsNullable: true})
	targetTable.AddColumn(&model.Column{Name: "email", DataType: "text", IsNullable: false})
	targetNs.Tables["users"] = targetTable

	ops, warnings := NewDiffer().Diff(source, target)
	if len(warnings) != 0 {
		t.Fatalf("unexpected warnings: %v", warnings)
	}

	var hasRename, hasDrop, hasAdd bool
	for _, op := range ops {
		switch op.Kind() {
		case KindRenameColumn:
			hasRename = true
			rop := op.(*RenameColumnOp)
			if rop.OldName != "username" || rop.NewName != "login_name" {
				t.Fatalf("expected rename username -> login_name, got %s -> %s",
					rop.OldName, rop.NewName)
			}
		case KindDropColumn:
			hasDrop = true
		case KindAddColumn:
			hasAdd = true
		}
	}
	if !hasRename {
		t.Fatal("expected RenameColumnOp, got none")
	}
	if hasDrop || hasAdd {
		t.Fatal("expected no DropColumn/AddColumn when rename is detected")
	}
}

func TestDiffer_NoFalsePositiveOnTypeMismatch(t *testing.T) {
	source := model.NewSchema()
	sourceNs := source.GetOrCreateNamespace("public")
	st := model.NewTable("public", "users")
	st.AddColumn(&model.Column{Name: "username", DataType: "text", IsNullable: true})
	sourceNs.Tables["users"] = st

	target := model.NewSchema()
	targetNs := target.GetOrCreateNamespace("public")
	tt := model.NewTable("public", "users")
	tt.AddColumn(&model.Column{Name: "login_name", DataType: "integer", IsNullable: true})
	targetNs.Tables["users"] = tt

	ops, _ := NewDiffer().Diff(source, target)
	for _, op := range ops {
		if op.Kind() == KindRenameColumn {
			t.Fatal("should not detect rename when types differ")
		}
	}
}

func TestDiffer_RenameWithAddColumn(t *testing.T) {
	source := model.NewSchema()
	sourceNs := source.GetOrCreateNamespace("public")
	st := model.NewTable("public", "users")
	st.AddColumn(&model.Column{Name: "username", DataType: "text", IsNullable: true})
	sourceNs.Tables["users"] = st

	target := model.NewSchema()
	targetNs := target.GetOrCreateNamespace("public")
	tt := model.NewTable("public", "users")
	tt.AddColumn(&model.Column{Name: "login_name", DataType: "text", IsNullable: true})
	tt.AddColumn(&model.Column{Name: "age", DataType: "integer", IsNullable: true})
	targetNs.Tables["users"] = tt

	ops, _ := NewDiffer().Diff(source, target)
	var hasRename, hasAdd bool
	for _, op := range ops {
		switch op.Kind() {
		case KindRenameColumn:
			hasRename = true
		case KindAddColumn:
			hasAdd = true
		}
	}
	if !hasRename {
		t.Fatal("expected rename op")
	}
	if !hasAdd {
		t.Fatal("expected add op for age")
	}
}

func TestDiffer_RenameWithDropColumn(t *testing.T) {
	source := model.NewSchema()
	sourceNs := source.GetOrCreateNamespace("public")
	st := model.NewTable("public", "users")
	st.AddColumn(&model.Column{Name: "username", DataType: "text", IsNullable: true})
	st.AddColumn(&model.Column{Name: "age", DataType: "integer", IsNullable: true})
	sourceNs.Tables["users"] = st

	target := model.NewSchema()
	targetNs := target.GetOrCreateNamespace("public")
	tt := model.NewTable("public", "users")
	tt.AddColumn(&model.Column{Name: "login_name", DataType: "text", IsNullable: true})
	targetNs.Tables["users"] = tt

	ops, _ := NewDiffer().Diff(source, target)
	var hasRename, hasDrop bool
	for _, op := range ops {
		switch op.Kind() {
		case KindRenameColumn:
			hasRename = true
		case KindDropColumn:
			hasDrop = true
		}
	}
	if !hasRename {
		t.Fatal("expected rename op")
	}
	if !hasDrop {
		t.Fatal("expected drop op for age")
	}
}

func TestDiffer_RenameNullableChangeDoesNotMatch(t *testing.T) {
	source := model.NewSchema()
	sourceNs := source.GetOrCreateNamespace("public")
	st := model.NewTable("public", "users")
	st.AddColumn(&model.Column{Name: "username", DataType: "text", IsNullable: true})
	sourceNs.Tables["users"] = st

	target := model.NewSchema()
	targetNs := target.GetOrCreateNamespace("public")
	tt := model.NewTable("public", "users")
	tt.AddColumn(&model.Column{Name: "login_name", DataType: "text", IsNullable: false})
	targetNs.Tables["users"] = tt

	ops, _ := NewDiffer().Diff(source, target)
	for _, op := range ops {
		if op.Kind() == KindRenameColumn {
			t.Fatal("should not detect rename when nullable differs")
		}
	}
}

func TestDiffer_RenameWithColumnReorder(t *testing.T) {
	source := model.NewSchema()
	sourceNs := source.GetOrCreateNamespace("public")
	st := model.NewTable("public", "users")
	st.AddColumn(&model.Column{Name: "id", DataType: "integer", IsNullable: false})
	st.AddColumn(&model.Column{Name: "username", DataType: "text", IsNullable: true})
	st.AddColumn(&model.Column{Name: "email", DataType: "text", IsNullable: false})
	sourceNs.Tables["users"] = st

	target := model.NewSchema()
	targetNs := target.GetOrCreateNamespace("public")
	tt := model.NewTable("public", "users")
	tt.AddColumn(&model.Column{Name: "login_name", DataType: "text", IsNullable: true})
	tt.AddColumn(&model.Column{Name: "id", DataType: "integer", IsNullable: false})
	tt.AddColumn(&model.Column{Name: "email", DataType: "text", IsNullable: false})
	targetNs.Tables["users"] = tt

	ops, warnings := NewDiffer().Diff(source, target)
	if len(warnings) != 0 {
		t.Fatalf("unexpected warnings: %v", warnings)
	}
	var hasRename bool
	for _, op := range ops {
		if op.Kind() == KindRenameColumn {
			hasRename = true
			rop := op.(*RenameColumnOp)
			if rop.OldName != "username" || rop.NewName != "login_name" {
				t.Fatalf("expected username -> login_name, got %s -> %s", rop.OldName, rop.NewName)
			}
		}
	}
	if !hasRename {
		t.Fatal("expected rename even with column reorder")
	}
}
```

运行：`go test ./internal/diff/... -run TestDiffer_DetectsColumnRename -count=1`
预期：FAIL（diffTableColumns 尚未实现启发式）

- [ ] **步骤 2：在 diffTableColumns 中添加重命名检测**

替换 `internal/diff/diff_tables.go` 中的 `diffTableColumns` 方法。

**替换前**（原方法，139-186 行）：
```go
func (c *diffContext) diffTableColumns(schema string, source, target *model.Table) {
	addColNames := make([]string, 0, len(target.ColumnByName))
	for name := range target.ColumnByName {
		if _, exists := source.ColumnByName[name]; !exists {
			addColNames = append(addColNames, name)
		}
	}
	sort.Strings(addColNames)
	for _, name := range addColNames {
		col := target.ColumnByName[name]
		c.addOp(NewAddColumnOp(schema, target.Name, col))
	}

	for name := range source.ColumnByName {
		if _, exists := target.ColumnByName[name]; !exists {
			c.addOp(NewDropColumnOp(schema, source.Name, name))
		}
	}

	for _, targetCol := range target.Columns {
		if sourceCol, exists := source.ColumnByName[targetCol.Name]; exists {
			c.diffColumn(schema, target.Name, sourceCol, targetCol)
		}
	}
}
```

**替换后**：
```go
func (c *diffContext) diffTableColumns(schema string, source, target *model.Table) {
	// Phase 1: Detect column renames via heuristic matching.
	// For columns that appear "dropped" in source and "added" in target,
	// check if they match on data type, nullable, and default expression.
	// If matched, emit RenameColumnOp instead of DropColumnOp + AddColumnOp.
	sourceOnlyNames := make([]string, 0)
	for name := range source.ColumnByName {
		if _, exists := target.ColumnByName[name]; !exists {
			sourceOnlyNames = append(sourceOnlyNames, name)
		}
	}
	sort.Strings(sourceOnlyNames)

	targetOnlyNames := make([]string, 0)
	for name := range target.ColumnByName {
		if _, exists := source.ColumnByName[name]; !exists {
			targetOnlyNames = append(targetOnlyNames, name)
		}
	}
	sort.Strings(targetOnlyNames)

	renamedSource := make(map[string]bool)
	renamedTarget := make(map[string]bool)

	for _, srcName := range sourceOnlyNames {
		srcCol := source.ColumnByName[srcName]
		for _, tgtName := range targetOnlyNames {
			if renamedTarget[tgtName] {
				continue
			}
			tgtCol := target.ColumnByName[tgtName]
			if isColumnRenameCandidate(srcCol, tgtCol) {
				c.addOp(NewRenameColumnOp(schema, target.Name, srcName, tgtName))
				renamedSource[srcName] = true
				renamedTarget[tgtName] = true
				break
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
		if sourceCol, exists := source.ColumnByName[targetCol.Name]; exists {
			c.diffColumn(schema, target.Name, sourceCol, targetCol)
		}
	}
}

// isColumnRenameCandidate checks if a source column and target column match
// the heuristic for being the same column that was renamed.
// Conditions: same DataType, same IsNullable, same DefaultExpr.
func isColumnRenameCandidate(src, tgt *model.Column) bool {
	if src.DataType != tgt.DataType {
		return false
	}
	if src.IsNullable != tgt.IsNullable {
		return false
	}
	return sameDefault(src.DefaultExpr, tgt.DefaultExpr)
}
```

- [ ] **步骤 3：运行测试验证通过**

运行：`go test ./internal/diff/... -run "TestDiffer_DetectsColumnRename|TestDiffer_NoFalsePositive|TestDiffer_RenameWith" -count=1 -v`
预期：6 个测试全部 PASS

- [ ] **步骤 4：运行全部 diff 测试确认无回归**

运行：`go test ./internal/diff/... -count=1`
预期：PASS

- [ ] **步骤 5：Commit**

```bash
git add internal/diff/diff_tables.go internal/diff/diff_rename_column_test.go
git commit -m "feat: add column rename heuristic detection in diff engine"
```

---

### 任务 7：DAG 排序注册（plan.go deploy 分组）

**文件：**
- 修改：`internal/plan/plan.go`

**上下文：** `plan.go` 的 `assignStage` 函数使用 `switch kind` 将操作分为三组（pre-deploy/deploy/post-deploy）。
deploy 分组（第 71 行）目前包含：`KindAlterColumnType, KindSetNotNull, KindDropNotNull, KindAddEnumLabel, KindSetDefault, KindDropDefault`。
**注意：** deploy 分组中缺失以下已有 Kind（这些被第 83 行 `return StageDeploy` 兜底处理）：
- `KindAlterColumnCollation`
- `KindSetIdentity`, `KindDropIdentity`
- `KindCreateSchema`, `KindDropSchema`

本任务只需添加 `KindRenameColumn` 到 deploy 分组。

- [ ] **步骤 1：将 KindRenameColumn 加入 deploy 分组**

在 `internal/plan/plan.go` 第 71 行的 deploy switch case 中添加：

```go
case diff.KindAlterColumnType, diff.KindSetNotNull, diff.KindDropNotNull, diff.KindAddEnumLabel, diff.KindSetDefault, diff.KindDropDefault, diff.KindRenameColumn:
```

- [ ] **步骤 2：运行 plan 测试确认无回归**

运行：`go test ./internal/plan/... -count=1`
预期：PASS

- [ ] **步骤 3：Commit**

```bash
git add internal/plan/plan.go
git commit -m "fix: register KindRenameColumn in deploy phase ordering"
```

---

### 任务 8：全量验证

**文件：**
- 无代码变更——仅运行验证

- [ ] **步骤 1：构建项目**

运行：`go build ./...`
预期：编译成功

- [ ] **步骤 2：运行全部测试**

运行：`go test ./... -count=1 2>&1 | tail -40`
预期：全部 PASS，无 regression

- [ ] **步骤 3：端到端冒烟测试（解析已有 testdata）**

```bash
cd /opt/codes/workspace/migra-go
go run ./cmd/migra/ diff testdata/alter_operations.sql testdata/alter_operations.sql
```
预期：`-- No changes detected`（源=目标，无变更）

- [ ] **步骤 4：Commit 最终**

```bash
git add -A
git commit -m "feat: complete RENAME COLUMN support across parser, diff, and render pipeline"
```

---

## 自检

### 1. 规格覆盖度
- ✅ **解析 `ALTER TABLE ... RENAME COLUMN`** → 任务 3（RenameStmtHandler 处理 Node_RenameStmt）
- ✅ **Mutation Apply 到 Schema** → 任务 2（RenameColumnMutation.Apply）
- ✅ **Mutation 常量注册** → 任务 1（MutKindRenameColumn）
- ✅ **Operation 类型定义** → 任务 4（RenameColumnOp）
- ✅ **Diff 引擎启发式检测** → 任务 6（isColumnRenameCandidate + 3 阶段排除）
- ✅ **SQL 渲染** → 任务 5（renderRenameColumn）
- ✅ **注册到 DefaultRegistry** → 任务 3 步骤 3
- ✅ **DAG 排序** → 任务 7
- ✅ **冒烟测试** → 任务 8

### 2. 占位符扫描
- ✅ 无 TODO/待定/后续实现
- ✅ 每个代码步骤包含完整 Go 代码
- ✅ 每个测试步骤包含完整测试代码
- ✅ 所有引用的类型和方法在前面的任务中定义
- ✅ 错误处理具体实现（无"添加适当的错误处理"类描述）

### 3. 类型一致性
| 符号 | 定义于 | 使用于 |
|------|--------|--------|
| `MutKindRenameColumn` | 任务 1 | 任务 2、3 |
| `RenameColumnMutation` | 任务 2 | 任务 3 |
| `RenameStmtHandler` | 任务 3 | 任务 3（registry.go） |
| `KindRenameColumn` | 任务 4 | 任务 4、5、6、7 |
| `RenameColumnOp` | 任务 4 | 任务 4、5 |
| `NewRenameColumnOp` | 任务 4 | 任务 5、6、测试 |
| `isColumnRenameCandidate` | 任务 6 | 任务 6 |
| `renderRenameColumn` | 任务 5 | 任务 5 |
| `parserutil.ParseRelation` | 已有（parserutil/util.go） | 任务 3 |
