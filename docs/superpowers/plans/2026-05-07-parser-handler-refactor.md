# Parser Handler 重构实现计划

> **面向 AI 代理的工作者：** 必需子技能：使用 superpowers:subagent-driven-development（推荐）或 superpowers:executing-plans 逐任务实现此计划。步骤使用复选框（`- [ ]`）语法来跟踪进度。

**目标：** 将 `internal/parser` 从硬编码 switch 模式重构为插件式 Handler + 自应用 Mutation 模式

**架构：** pg_query AST 节点由 HandlerRegistry 路由到对应 Handler，Handler 返回 SchemaMutation 列表，MutationApplier 遍历调用每个 mutation 的 Apply 方法修改 model.Schema。所有 mutation 自带 Apply 逻辑，确保新增 DDL 类型无需修改 Parser/Applier 核心。

**技术栈：** Go 1.26.2, pg_query_go v1.0.2, testify

---

## 文件结构

### 新增文件

| 文件 | 职责 |
|---|---|
| `internal/parser/parserutil/util.go` | 无状态辅助函数：ParseRelation, ParseColumnDef, ParseTypeName, ParseExpression, mapTypeName |
| `internal/parser/mutation.go` | SchemaMutation 接口 + Kind 常量 + 各 Mutation struct 定义（CreateTableMutation, AddColumnMutation 等） |
| `internal/parser/applier.go` | MutationApplier：遍历 mutation 列表并收集错误 |
| `internal/parser/registry.go` | Handler 接口 + HandlerRegistry（reflect.Type 映射）+ DefaultRegistry |
| `internal/parser/create_table_handler.go` | CreateTableHandler：AST → []SchemaMutation |
| `internal/parser/alter_table_handler.go` | AlterTableHandler：AST → []SchemaMutation |
| `internal/parser/index_handler.go` | CreateIndexHandler 框架 |
| `internal/parser/enum_handler.go` | CreateEnumHandler 框架 |
| `internal/parser/parserutil/util_test.go` | 辅助函数单元测试 |
| `internal/parser/handler_test.go` | Handler 单元测试（CreateTableHandler, AlterTableHandler） |
| `internal/parser/mutation_test.go` | Mutation Apply 单元测试 |

### 修改文件

| 文件 | 修改内容 |
|---|---|
| `internal/parser/parser.go` | 移除 handleCreateTable、handleAlterTable、parseColumnDef、parseTypeName、parseExpression、mapTypeName、parseRelation、typeNameMapping；保留 ParseSQL、visitNode 编排逻辑；修改 Parser struct 加入 registry/applier；新增 NewParser/NewParserWith |
| `internal/parser/parser_test.go` | 更新导入路径（parserutil 函数），确保端到端测试通过 |

### 不涉及文件

- `internal/model/*` — 模型类型不变
- `internal/diff/*` — diff 引擎不变
- `internal/render/*` — render 不变
- `cmd/*` — CLI 入口不变

---

## 任务分解

### 阶段 1：提取 parserutil 包（纯机械性移动，低风险）

**目标：** 将 parser.go 中的辅助函数提取到独立包，消除后续重构的耦合。

---

#### 任务 1.1：创建 parserutil/util.go

**文件：**
- 创建：`internal/parser/parserutil/util.go`

- [ ] **步骤 1：编写 parserutil/util.go**

将 `parser.go` 中的以下函数/变量完整迁移：
- `parseRelation` → `ParseRelation`
- `parseColumnDef` → `ParseColumnDef`
- `parseTypeName` → `ParseTypeName`
- `parseExpression` → `ParseExpression`
- `typeNameMapping` → `typeNameMapping`
- `mapTypeName` → `MapTypeName`

注意：导出函数首字母大写。

```go
package parserutil

import (
	"fmt"
	"strings"

	"github.com/fred29910/migra-go/internal/model"
	pg_nodes "github.com/lfittl/pg_query_go/nodes"
)

// ParseRelation extracts table/view name and schema from RangeVar.
func ParseRelation(relation *pg_nodes.RangeVar) (tableName, schemaName string) {
	if relation == nil {
		return "", "public"
	}
	if relation.Relname != nil {
		tableName = *relation.Relname
	}
	if relation.Schemaname != nil {
		schemaName = *relation.Schemaname
	} else {
		schemaName = "public"
	}
	return
}

// ParseColumnDef extracts a model.Column from pg_query ColumnDef node.
func ParseColumnDef(elt pg_nodes.ColumnDef) *model.Column {
	col := &model.Column{IsNullable: true}
	if elt.Colname != nil {
		col.Name = *elt.Colname
	}
	if elt.TypeName != nil {
		col.DataType = ParseTypeName(*elt.TypeName)
	}
	for _, item := range elt.Constraints.Items {
		switch c := item.(type) {
		case pg_nodes.Constraint:
			switch c.Contype {
			case pg_nodes.CONSTR_NOTNULL:
				col.IsNullable = false
			case pg_nodes.CONSTR_DEFAULT:
				if c.RawExpr != nil {
					expr, ok := ParseExpression(c.RawExpr)
					if ok {
						col.DefaultExpr = &expr
					}
				}
			}
		}
	}
	return col
}

// ParseTypeName maps pg_query TypeName to a standard SQL type string.
func ParseTypeName(typeName pg_nodes.TypeName) string {
	parts := make([]string, 0)
	for _, item := range typeName.Names.Items {
		if s, ok := item.(pg_nodes.String); ok {
			if s.Str != "pg_catalog" {
				parts = append(parts, s.Str)
			}
		}
	}
	typeStr := strings.Join(parts, ".")
	typeStr = MapTypeName(typeStr)
	if len(typeName.Typmods.Items) > 0 {
		mods := make([]string, 0)
		for _, item := range typeName.Typmods.Items {
			if a, ok := item.(pg_nodes.A_Const); ok {
				if a.Val != nil {
					if i, ok := a.Val.(pg_nodes.Integer); ok {
						mods = append(mods, fmt.Sprintf("%d", i.Ival))
					}
				}
			}
		}
		if len(mods) > 0 {
			typeStr += "(" + strings.Join(mods, ",") + ")"
		}
	}
	return strings.ToLower(typeStr)
}

// ParseExpression extracts expression as string from AST node.
func ParseExpression(expr pg_nodes.Node) (string, bool) {
	switch e := expr.(type) {
	case pg_nodes.A_Const:
		if e.Val != nil {
			switch v := e.Val.(type) {
			case pg_nodes.String:
				return "'" + v.Str + "'", true
			case pg_nodes.Integer:
				return fmt.Sprintf("%d", v.Ival), true
			case pg_nodes.Float:
				return v.Str, true
			}
		}
	}
	return "", false
}

var typeNameMapping = map[string]string{
	"int4":   "integer",
	"int8":   "bigint",
	"float4": "real",
	"float8": "double precision",
}

// MapTypeName maps PostgreSQL internal type names to standard SQL names.
func MapTypeName(name string) string {
	if mapped, ok := typeNameMapping[name]; ok {
		return mapped
	}
	return name
}
```

- [ ] **步骤 2：编译验证**

运行：`go build ./internal/parser/parserutil/...`

预期：PASS（无错误）

- [ ] **步骤 3：修改 parser.go 使用 parserutil**

修改 `internal/parser/parser.go`：将辅助函数调用替换为 `parserutil.` 前缀。

**修改点 1** — 导入 `parserutil`：

```go
import (
	"fmt"
	"strings"

	"github.com/fred29910/migra-go/internal/model"
	"github.com/fred29910/migra-go/internal/parser/parserutil"
	pg "github.com/lfittl/pg_query_go"
	pg_nodes "github.com/lfittl/pg_query_go/nodes"
)
```

**修改点 2** — `handleCreateTable` 中替换调用：

```go
// OLD:
tableName, schemaName := p.parseRelation(stmt.Relation)
// NEW:
tableName, schemaName := parserutil.ParseRelation(stmt.Relation)

// OLD:
col := p.parseColumnDef(elt)
// NEW:
col := parserutil.ParseColumnDef(elt)
```

**修改点 3** — `handleAlterTable` 中替换调用：

```go
// OLD:
tableName, schemaName := p.parseRelation(stmt.Relation)
// NEW:
tableName, schemaName := parserutil.ParseRelation(stmt.Relation)

// OLD:
col := p.parseColumnDef(colDef)
// NEW:
col := parserutil.ParseColumnDef(colDef)
```

**修改点 4** — `parseColumnDef` 方法内部替换（该方法后续会被删除，暂时保留但内部调用改前缀）：

```go
// OLD:
col.DataType = p.parseTypeName(*colDef.TypeName)
// NEW:
col.DataType = parserutil.ParseTypeName(*colDef.TypeName)

// OLD:
expr, ok := p.parseExpression(c.RawExpr)
// NEW:
expr, ok := parserutil.ParseExpression(c.RawExpr)
```

- [ ] **步骤 4：运行测试验证无回归**

运行：`go test ./internal/parser/...`

预期：PASS（所有现有测试通过）

- [ ] **步骤 5：Commit**

```bash
git add internal/parser/parserutil/
git add internal/parser/parser.go
git commit -m "refactor(parser): 提取 parserutil 辅助函数包"
```

---

### 阶段 2：定义 SchemaMutation 接口 + MutationApplier

**目标：** 建立 mutation 类型系统，但暂不实现具体 mutation 的 Apply 方法（返回 `fmt.Errorf("not implemented")`）。

---

#### 任务 2.1：创建 mutation.go

**文件：**
- 创建：`internal/parser/mutation.go`

- [ ] **步骤 1：编写 mutation.go**

```go
package parser

import (
	"fmt"

	"github.com/fred29910/migra-go/internal/model"
)

// MutationKind identifies the type of schema mutation.
type MutationKind string

const (
	MutKindCreateTable     MutationKind = "create_table"
	MutKindAddColumn       MutationKind = "add_column"
	MutKindCreateEnumType  MutationKind = "create_enum_type"
	MutKindCreateIndex     MutationKind = "create_index"
)

// SchemaMutation is a self-describing and self-applying schema change.
type SchemaMutation interface {
	Kind() MutationKind
	Target() model.ObjectKey
	Apply(schema *model.Schema) error
}

// CreateTableMutation describes creating a new table.
type CreateTableMutation struct {
	Schema  string
	Name    string
	Columns []model.Column
}

func (m CreateTableMutation) Kind() MutationKind { return MutKindCreateTable }
func (m CreateTableMutation) Target() model.ObjectKey {
	return model.NewObjectKey(m.Schema, m.Name, model.KindTable)
}
func (m CreateTableMutation) Apply(schema *model.Schema) error {
	return fmt.Errorf("CreateTableMutation.Apply not implemented yet")
}

// AddColumnMutation describes adding a column to an existing table.
type AddColumnMutation struct {
	Schema string
	Table  string
	Column model.Column
}

func (m AddColumnMutation) Kind() MutationKind { return MutKindAddColumn }
func (m AddColumnMutation) Target() model.ObjectKey {
	return model.NewObjectKey(m.Schema, m.Table+"."+m.Column.Name, model.KindColumn)
}
func (m AddColumnMutation) Apply(schema *model.Schema) error {
	return fmt.Errorf("AddColumnMutation.Apply not implemented yet")
}

// CreateEnumTypeMutation describes creating an enum type.
type CreateEnumTypeMutation struct {
	Schema string
	Name   string
	Labels []string
}

func (m CreateEnumTypeMutation) Kind() MutationKind { return MutKindCreateEnumType }
func (m CreateEnumTypeMutation) Target() model.ObjectKey {
	return model.NewObjectKey(m.Schema, m.Name, model.KindType)
}
func (m CreateEnumTypeMutation) Apply(schema *model.Schema) error {
	return fmt.Errorf("CreateEnumTypeMutation.Apply not implemented yet")
}
```

- [ ] **步骤 2：编译验证**

运行：`go build ./internal/parser/...`

预期：PASS

- [ ] **步骤 3：Commit**

```bash
git add internal/parser/mutation.go
git commit -m "feat(parser): 定义 SchemaMutation 接口和核心 mutation 类型（占位 Apply）"
```

---

#### 任务 2.2：创建 applier.go

**文件：**
- 创建：`internal/parser/applier.go`

- [ ] **步骤 1：编写 applier.go**

```go
package parser

import "fmt"

// MutationApplier applies SchemaMutation values to a Schema.
// It validates consistency and collects errors without halting on the first one.
type MutationApplier struct {
	errors []error
}

// Apply applies mutations to the given schema in order.
// Returns nil if all mutations succeeded, or an aggregate error.
func (a *MutationApplier) Apply(schema *model.Schema, mutations []SchemaMutation) error {
	a.errors = a.errors[:0]
	for _, mut := range mutations {
		if err := mut.Apply(schema); err != nil {
			a.errors = append(a.errors, &MutationError{Mutation: mut, Cause: err})
		}
	}
	if len(a.errors) > 0 {
		first := a.errors[0]
		return fmt.Errorf("applied %d mutations with %d errors, first: %w",
			len(mutations), len(a.errors), first)
	}
	return nil
}

// MutationError wraps an error with mutation context.
type MutationError struct {
	Mutation SchemaMutation
	Cause    error
}

func (e *MutationError) Error() string {
	t := e.Mutation.Target()
	return fmt.Sprintf("mutation %s on %s/%s: %v", e.Mutation.Kind(), t.Schema, t.Name, e.Cause)
}

func (e *MutationError) Unwrap() error {
	return e.Cause
}
```

- [ ] **步骤 2：修复编译 — 添加 model 导入**

上一步的 `Apply` 方法签名引用了 `*model.Schema`，需要导入 `model` 包。

```go
import (
	"fmt"
	"github.com/fred29910/migra-go/internal/model"
)
```

- [ ] **步骤 3：编译验证**

运行：`go build ./internal/parser/...`

预期：PASS

- [ ] **步骤 4：Commit**

```bash
git add internal/parser/applier.go
git commit -m "feat(parser): 添加 MutationApplier（遍历 mutation.Apply，收集错误）"
```

---

### 阶段 3：实现 CreateTableMutation.Apply + CreateTableHandler

**目标：** 用 mutation + handler 替换 `handleCreateTable`。

---

#### 任务 3.1：实现 CreateTableMutation.Apply

**文件：**
- 修改：`internal/parser/mutation.go`（修改 Apply 方法体）

- [ ] **步骤 1：修改 CreateTableMutation.Apply**

将 `CreateTableMutation.Apply` 从占位实现替换为真实逻辑：

```go
func (m CreateTableMutation) Apply(schema *model.Schema) error {
	ns := schema.GetOrCreateNamespace(m.Schema)
	if _, exists := ns.Tables[m.Name]; exists {
		return fmt.Errorf("table %s.%s already exists", m.Schema, m.Name)
	}
	table := model.NewTable(m.Schema, m.Name)
	for i := range m.Columns {
		table.AddColumn(&m.Columns[i])
	}
	ns.Tables[m.Name] = table
	return nil
}
```

- [ ] **步骤 2：编写 mutation_test.go（CreateTableMutation 测试）**

创建 `internal/parser/mutation_test.go`：

```go
package parser

import (
	"testing"

	"github.com/fred29910/migra-go/internal/model"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestCreateTableMutation_Apply(t *testing.T) {
	schema := model.NewSchema()
	mut := CreateTableMutation{
		Schema: "public",
		Name:   "users",
		Columns: []model.Column{
			{Name: "id", DataType: "integer", IsNullable: false},
			{Name: "name", DataType: "varchar(50)", IsNullable: true},
		},
	}

	err := mut.Apply(schema)
	require.NoError(t, err)

	ns := schema.GetNamespace("public")
	require.NotNil(t, ns)
	table := ns.Tables["users"]
	require.NotNil(t, table)
	assert.Equal(t, "users", table.Name)
	assert.Len(t, table.Columns, 2)
	assert.Equal(t, "id", table.Columns[0].Name)
}

func TestCreateTableMutation_Apply_Duplicate(t *testing.T) {
	schema := model.NewSchema()
	mut1 := CreateTableMutation{Schema: "public", Name: "users", Columns: []model.Column{{Name: "id"}}}
	mut2 := CreateTableMutation{Schema: "public", Name: "users", Columns: []model.Column{{Name: "id"}}}

	require.NoError(t, mut1.Apply(schema))
	err := mut2.Apply(schema)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "already exists")
}
```

- [ ] **步骤 3：运行测试**

运行：`go test ./internal/parser/... -run TestCreateTableMutation`

预期：PASS

- [ ] **步骤 4：Commit**

```bash
git add internal/parser/mutation.go
git add internal/parser/mutation_test.go
git commit -m "feat(parser): 实现 CreateTableMutation.Apply + 测试"
```

---

#### 任务 3.2：创建 CreateTableHandler

**文件：**
- 创建：`internal/parser/create_table_handler.go`

- [ ] **步骤 1：编写 create_table_handler.go**

```go
package parser

import (
	"github.com/fred29910/migra-go/internal/model"
	"github.com/fred29910/migra-go/internal/parser/parserutil"
	pg_nodes "github.com/lfittl/pg_query_go/nodes"
)

// CreateTableHandler handles CREATE TABLE statements.
type CreateTableHandler struct{}

// Handle converts a pg_query CreateStmt into schema mutations.
func (h *CreateTableHandler) Handle(node pg_nodes.Node) ([]SchemaMutation, error) {
	stmt := node.(pg_nodes.CreateStmt)
	tableName, schemaName := parserutil.ParseRelation(stmt.Relation)

	var columns []model.Column
	for _, item := range stmt.TableElts.Items {
		switch elt := item.(type) {
		case pg_nodes.ColumnDef:
			columns = append(columns, *parserutil.ParseColumnDef(elt))
		// MVP: skip constraints
		}
	}

	return []SchemaMutation{
		CreateTableMutation{
			Schema:  schemaName,
			Name:    tableName,
			Columns: columns,
		},
	}, nil
}
```

- [ ] **步骤 2：编写 handler 单元测试**

创建测试 helper `mustParseFirstStmt`，然后测试 handler。

在 `internal/parser/handler_test.go` 中：

```go
package parser

import (
	"testing"

	pg "github.com/lfittl/pg_query_go"
	pg_nodes "github.com/lfittl/pg_query_go/nodes"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func mustParseFirstStmt(t *testing.T, sql string) pg_nodes.Node {
	t.Helper()
	tree, err := pg.Parse(sql)
	require.NoError(t, err)
	require.Len(t, tree.Statements, 1)
	raw := tree.Statements[0].(pg_nodes.RawStmt)
	return raw.Stmt
}

func TestCreateTableHandler(t *testing.T) {
	node := mustParseFirstStmt(t, "CREATE TABLE users (id integer NOT NULL, name varchar(50))")
	h := &CreateTableHandler{}

	mutations, err := h.Handle(node)
	require.NoError(t, err)
	require.Len(t, mutations, 1)

	mut, ok := mutations[0].(CreateTableMutation)
	require.True(t, ok)
	assert.Equal(t, "users", mut.Name)
	assert.Equal(t, "public", mut.Schema)
	assert.Len(t, mut.Columns, 2)
	assert.Equal(t, "id", mut.Columns[0].Name)
	assert.Equal(t, "integer", mut.Columns[0].DataType)
	assert.False(t, mut.Columns[0].IsNullable)
	assert.Equal(t, "name", mut.Columns[1].Name)
}
```

- [ ] **步骤 3：运行测试**

运行：`go test ./internal/parser/... -run TestCreateTableHandler`

预期：PASS

- [ ] **步骤 4：Commit**

```bash
git add internal/parser/create_table_handler.go
git add internal/parser/handler_test.go
git commit -m "feat(parser): 添加 CreateTableHandler + 单元测试"
```

---

#### 任务 3.3：修改 parser.go 使用 CreateTableMutation 替代 handleCreateTable

**文件：**
- 修改：`internal/parser/parser.go`

- [ ] **步骤 1：在 Parser struct 中添加 applier 字段**

```go
type Parser struct {
	schema  *model.Schema
	errors  []error
	sql     string
	applier *MutationApplier  // 新增
}
```

- [ ] **步骤 2：修改 NewParser 初始化 applier**

```go
func NewParser() *Parser {
	return &Parser{
		// ... existing ...
		applier: &MutationApplier{},
	}
}
```

- [ ] **步骤 3：在 handleCreateTable 中构建并应用 mutation**

替换 `handleCreateTable` 方法体为 mutation 模式：

```go
func (p *Parser) handleCreateTable(stmt pg_nodes.CreateStmt) error {
	tableName, schemaName := parserutil.ParseRelation(stmt.Relation)

	var columns []model.Column
	for _, item := range stmt.TableElts.Items {
		switch elt := item.(type) {
		case pg_nodes.ColumnDef:
			columns = append(columns, *parserutil.ParseColumnDef(elt))
		}
	}

	mut := CreateTableMutation{
		Schema:  schemaName,
		Name:    tableName,
		Columns: columns,
	}
	return p.applier.Apply(p.schema, []SchemaMutation{mut})
}
```

- [ ] **步骤 4：运行 parser 测试确保无回归**

运行：`go test ./internal/parser/...`

预期：PASS（所有现有测试仍通过）

- [ ] **步骤 5：Commit**

```bash
git add internal/parser/parser.go
git commit -m "refactor(parser): handleCreateTable 改用 CreateTableMutation + MutationApplier"
```

---

### 阶段 4：实现 AddColumnMutation.Apply + AlterTableHandler

**目标：** 用 mutation + handler 替换 `handleAlterTable`。

---

#### 任务 4.1：实现 AddColumnMutation.Apply

**文件：**
- 修改：`internal/parser/mutation.go`

- [ ] **步骤 1：修改 AddColumnMutation.Apply**

```go
func (m AddColumnMutation) Apply(schema *model.Schema) error {
	ns := schema.GetOrCreateNamespace(m.Schema)
	table, exists := ns.Tables[m.Table]
	if !exists {
		// ALTER TABLE may appear before CREATE TABLE in SQL;
		// create an empty placeholder table.
		table = model.NewTable(m.Schema, m.Table)
		ns.Tables[m.Table] = table
	}
	table.AddColumn(&m.Column)
	return nil
}
```

- [ ] **步骤 2：在 mutation_test.go 添加测试**

```go
func TestAddColumnMutation_Apply_ToExistingTable(t *testing.T) {
	schema := model.NewSchema()
	// Pre-create table
	mutTable := CreateTableMutation{Schema: "public", Name: "users", Columns: []model.Column{{Name: "id"}}}
	require.NoError(t, mutTable.Apply(schema))

	mutCol := AddColumnMutation{Schema: "public", Table: "users", Column: model.Column{Name: "age", DataType: "integer"}}
	err := mutCol.Apply(schema)
	require.NoError(t, err)

	ns := schema.GetNamespace("public")
	require.Len(t, ns.Tables["users"].Columns, 2)
	assert.Equal(t, "age", ns.Tables["users"].Columns[1].Name)
}

func TestAddColumnMutation_Apply_CreatesPlaceholderTable(t *testing.T) {
	schema := model.NewSchema()
	mutCol := AddColumnMutation{Schema: "public", Table: "users", Column: model.Column{Name: "age", DataType: "integer"}}
	err := mutCol.Apply(schema)
	require.NoError(t, err)

	ns := schema.GetNamespace("public")
	require.NotNil(t, ns.Tables["users"])
	assert.Len(t, ns.Tables["users"].Columns, 1)
}
```

- [ ] **步骤 3：运行测试**

运行：`go test ./internal/parser/... -run TestAddColumnMutation`

预期：PASS

- [ ] **步骤 4：Commit**

```bash
git add internal/parser/mutation.go
git add internal/parser/mutation_test.go
git commit -m "feat(parser): 实现 AddColumnMutation.Apply + 测试"
```

---

#### 任务 4.2：创建 AlterTableHandler

**文件：**
- 创建：`internal/parser/alter_table_handler.go`

- [ ] **步骤 1：编写 alter_table_handler.go**

```go
package parser

import (
	"fmt"

	"github.com/fred29910/migra-go/internal/model"
	"github.com/fred29910/migra-go/internal/parser/parserutil"
	pg_nodes "github.com/lfittl/pg_query_go/nodes"
)

// AlterTableHandler handles ALTER TABLE statements.
type AlterTableHandler struct{}

func (h *AlterTableHandler) Handle(node pg_nodes.Node) ([]SchemaMutation, error) {
	stmt := node.(pg_nodes.AlterTableStmt)
	tableName, schemaName := parserutil.ParseRelation(stmt.Relation)

	var mutations []SchemaMutation
	for _, item := range stmt.Cmds.Items {
		cmd, ok := item.(pg_nodes.AlterTableCmd)
		if !ok {
			continue
		}
		switch cmd.Subtype {
		case pg_nodes.AT_AddColumn:
			if cmd.Def != nil {
				if colDef, ok := cmd.Def.(pg_nodes.ColumnDef); ok {
					col := parserutil.ParseColumnDef(colDef)
					mutations = append(mutations, AddColumnMutation{
						Schema: schemaName,
						Table:  tableName,
						Column: *col,
					})
				}
			}
		// MVP: Skip other alter subcommands
		default:
			// Collect warnings or skip silently
		}
	}

	return mutations, nil
}
```

- [ ] **步骤 2：在 handler_test.go 添加测试**

```go
func TestAlterTableHandler_AddColumn(t *testing.T) {
	node := mustParseFirstStmt(t, "ALTER TABLE users ADD COLUMN age integer")
	h := &AlterTableHandler{}

	mutations, err := h.Handle(node)
	require.NoError(t, err)
	require.Len(t, mutations, 1)

	mut, ok := mutations[0].(AddColumnMutation)
	require.True(t, ok)
	assert.Equal(t, "users", mut.Table)
	assert.Equal(t, "public", mut.Schema)
	assert.Equal(t, "age", mut.Column.Name)
	assert.Equal(t, "integer", mut.Column.DataType)
}
```

- [ ] **步骤 3：运行测试**

运行：`go test ./internal/parser/... -run TestAlterTableHandler`

预期：PASS

- [ ] **步骤 4：Commit**

```bash
git add internal/parser/alter_table_handler.go
git add internal/parser/handler_test.go
git commit -m "feat(parser): 添加 AlterTableHandler + 单元测试"
```

---

#### 任务 4.3：修改 parser.go 使用 AddColumnMutation 替代 handleAlterTable

**文件：**
- 修改：`internal/parser/parser.go`

- [ ] **步骤 1：替换 handleAlterTable 方法体**

```go
func (p *Parser) handleAlterTable(stmt pg_nodes.AlterTableStmt) error {
	tableName, schemaName := parserutil.ParseRelation(stmt.Relation)

	var mutations []SchemaMutation
	for _, item := range stmt.Cmds.Items {
		cmd, ok := item.(pg_nodes.AlterTableCmd)
		if !ok {
			continue
		}
		switch cmd.Subtype {
		case pg_nodes.AT_AddColumn:
			if cmd.Def != nil {
				if colDef, ok := cmd.Def.(pg_nodes.ColumnDef); ok {
					col := parserutil.ParseColumnDef(colDef)
					mutations = append(mutations, AddColumnMutation{
						Schema: schemaName,
						Table:  tableName,
						Column: *col,
					})
				}
			}
		}
	}

	return p.applier.Apply(p.schema, mutations)
}
```

- [ ] **步骤 2：运行 parser 测试**

运行：`go test ./internal/parser/...`

预期：PASS

- [ ] **步骤 3：Commit**

```bash
git add internal/parser/parser.go
git commit -m "refactor(parser): handleAlterTable 改用 AddColumnMutation + MutationApplier"
```

---

### 阶段 5：引入 HandlerRegistry，重构 visitNode

**目标：** 用注册表 + 反射类型映射替代 visitNode 中的 switch。

---

#### 任务 5.1：创建 registry.go

**文件：**
- 创建：`internal/parser/registry.go`

- [ ] **步骤 1：编写 registry.go**

```go
package parser

import (
	"reflect"

	pg_nodes "github.com/lfittl/pg_query_go/nodes"
)

// Handler parses a single pg_query AST node into schema mutations.
type Handler interface {
	Handle(node pg_nodes.Node) ([]SchemaMutation, error)
}

// HandlerRegistry routes AST nodes to their handlers using reflect.Type lookup.
type HandlerRegistry struct {
	handlers map[reflect.Type]Handler
}

// NewHandlerRegistry creates an empty registry.
func NewHandlerRegistry() *HandlerRegistry {
	return &HandlerRegistry{handlers: make(map[reflect.Type]Handler)}
}

// Register associates a handler with a specific AST node type.
func (r *HandlerRegistry) Register(nodeType pg_nodes.Node, h Handler) {
	r.handlers[reflect.TypeOf(nodeType)] = h
}

// Dispatch finds the handler for the given AST node.
func (r *HandlerRegistry) Dispatch(node pg_nodes.Node) (Handler, bool) {
	h, ok := r.handlers[reflect.TypeOf(node)]
	return h, ok
}

// DefaultRegistry returns a registry with all built-in handlers.
func DefaultRegistry() *HandlerRegistry {
	r := NewHandlerRegistry()
	r.Register(pg_nodes.CreateStmt{}, &CreateTableHandler{})
	r.Register(pg_nodes.AlterTableStmt{}, &AlterTableHandler{})
	// Index/Enum handlers added in later tasks
	return r
}
```

- [ ] **步骤 2：编译验证**

运行：`go build ./internal/parser/...`

预期：PASS

- [ ] **步骤 3：Commit**

```bash
git add internal/parser/registry.go
git commit -m "feat(parser): 添加 HandlerRegistry（reflect.Type 映射路由）"
```

---

#### 任务 5.2：重构 visitNode 使用 registry

**文件：**
- 修改：`internal/parser/parser.go`

- [ ] **步骤 1：修改 Parser struct 加入 registry**

```go
type Parser struct {
	schema   *model.Schema
	errors   []error
	sql      string
	applier  *MutationApplier
	registry *HandlerRegistry  // 新增
}
```

- [ ] **步骤 2：修改 NewParser 使用 DefaultRegistry**

```go
func NewParser() *Parser {
	return &Parser{
		// ... existing fields ...
		applier:  &MutationApplier{},
		registry: DefaultRegistry(),
	}
}

// NewParserWith creates a Parser with custom dependencies. Primarily for testing.
func NewParserWith(registry *HandlerRegistry, applier *MutationApplier) *Parser {
	return &Parser{
		schema:   model.NewSchema(),
		applier:  applier,
		registry: registry,
	}
}
```

- [ ] **步骤 3：重构 visitNode**

```go
func (p *Parser) visitNode(stmt pg_nodes.Node) error {
	rawStmt, ok := stmt.(pg_nodes.RawStmt)
	if !ok {
		return &ParseError{
			Message:  fmt.Sprintf("expected RawStmt, got: %T", stmt),
			Position: -1,
		}
	}

	actualStmt := rawStmt.Stmt
	pos := rawStmt.StmtLocation

	handler, found := p.registry.Dispatch(actualStmt)
	if !found {
		return &ParseError{
			Message:   fmt.Sprintf("unsupported statement type: %T", actualStmt),
			Position:  pos,
			Statement: p.getStatementSnippet(pos),
		}
	}

	mutations, err := handler.Handle(actualStmt)
	if err != nil {
		return &ParseError{
			Message:   err.Error(),
			Position:  pos,
			Statement: p.getStatementSnippet(pos),
		}
	}

	if err := p.applier.Apply(p.schema, mutations); err != nil {
		return &ParseError{
			Message:   err.Error(),
			Position:  pos,
			Statement: p.getStatementSnippet(pos),
		}
	}
	return nil
}
```

- [ ] **步骤 4：运行全量测试**

运行：`go test ./internal/parser/... -v`

预期：PASS（所有现有 parser 测试通过）

- [ ] **步骤 5：运行端到端集成测试**

运行：`go test ./internal/parser/... -run TestIntegrationParserToDiff`

预期：PASS

- [ ] **步骤 6：Commit**

```bash
git add internal/parser/parser.go
git add internal/parser/registry.go
git commit -m "refactor(parser): visitNode 改用 HandlerRegistry + MutationApplier"
```

---

### 阶段 6：清理死代码

**目标：** 删除旧的 handleCreateTable、handleAlterTable 及已从 parserutil 迁移的辅助方法。

---

#### 任务 6.1：删除 parser.go 中的旧方法

**文件：**
- 修改：`internal/parser/parser.go`

- [ ] **步骤 1：删除 handleCreateTable 方法**

完全删除 `func (p *Parser) handleCreateTable` 方法。

- [ ] **步骤 2：删除 handleAlterTable 方法**

完全删除 `func (p *Parser) handleAlterTable` 方法。

- [ ] **步骤 3：删除 parseColumnDef 方法**

完全删除 `func (p *Parser) parseColumnDef` 方法。

- [ ] **步骤 4：删除 parseTypeName 方法**

完全删除 `func (p *Parser) parseTypeName` 方法。

- [ ] **步骤 5：删除 parseExpression 方法**

完全删除 `func (p *Parser) parseExpression` 方法。

- [ ] **步骤 6：删除 typeNameMapping 和 mapTypeName**

完全删除 `var typeNameMapping` 和 `func mapTypeName`。

- [ ] **步骤 7：清理未使用的导入**

检查并移除 parser.go 中不再使用的导入（如 `strings` 如果已完全迁移到 parserutil）。

- [ ] **步骤 8：编译验证**

运行：`go build ./internal/parser/...`

预期：PASS

- [ ] **步骤 9：运行全量测试**

运行：`go test ./...`

预期：PASS

- [ ] **步骤 10：Commit**

```bash
git add internal/parser/parser.go
git commit -m "refactor(parser): 清理旧 handleCreateTable/handleAlterTable/辅助方法"
```

---

### 阶段 7：新增 Enum/Index Handler（扩展验证架构）

**目标：** 验证新增 DDL 类型确实无需修改 parser.go，仅新增文件 + 注册即可。

---

#### 任务 7.1：创建 CreateEnumHandler 框架

**文件：**
- 创建：`internal/parser/enum_handler.go`

- [ ] **步骤 1：编写 enum_handler.go**

```go
package parser

import (
	"fmt"

	pg_nodes "github.com/lfittl/pg_query_go/nodes"
)

// CreateEnumHandler handles CREATE TYPE ... AS ENUM statements.
type CreateEnumHandler struct{}

func (h *CreateEnumHandler) Handle(node pg_nodes.Node) ([]SchemaMutation, error) {
	stmt := node.(pg_nodes.CreateEnumStmt)

	// Extract schema and type name from TypeName
	var schemaName, typeName string
	if stmt.TypeName != nil && len(stmt.TypeName.Items) > 0 {
		// TypeName is a List of String nodes (schema.type or just type)
		// For now, return placeholder to verify architecture works
		return nil, fmt.Errorf("CREATE TYPE ENUM parsing not yet fully implemented")
	}

	return nil, fmt.Errorf("CREATE TYPE ENUM parsing not yet fully implemented")
}
```

- [ ] **步骤 2：在 DefaultRegistry 中注册**

修改 `internal/parser/registry.go`：

```go
func DefaultRegistry() *HandlerRegistry {
	r := NewHandlerRegistry()
	r.Register(pg_nodes.CreateStmt{}, &CreateTableHandler{})
	r.Register(pg_nodes.AlterTableStmt{}, &AlterTableHandler{})
	r.Register(pg_nodes.CreateEnumStmt{}, &CreateEnumHandler{})  // 新增
	return r
}
```

- [ ] **步骤 3：编译验证**

运行：`go build ./internal/parser/...`

预期：PASS

- [ ] **步骤 4：Commit**

```bash
git add internal/parser/enum_handler.go
git add internal/parser/registry.go
git commit -m "feat(parser): 添加 CreateEnumHandler 框架（验证 OCP）"
```

---

#### 任务 7.2：创建 CreateIndexHandler 框架

**文件：**
- 创建：`internal/parser/index_handler.go`

- [ ] **步骤 1：编写 index_handler.go**

```go
package parser

import (
	"fmt"

	pg_nodes "github.com/lfittl/pg_query_go/nodes"
)

// CreateIndexHandler handles CREATE INDEX statements.
type CreateIndexHandler struct{}

func (h *CreateIndexHandler) Handle(node pg_nodes.Node) ([]SchemaMutation, error) {
	_ = node.(pg_nodes.IndexStmt)
	return nil, fmt.Errorf("CREATE INDEX parsing not yet fully implemented")
}
```

- [ ] **步骤 2：在 DefaultRegistry 中注册**

修改 `internal/parser/registry.go`：

```go
func DefaultRegistry() *HandlerRegistry {
	r := NewHandlerRegistry()
	r.Register(pg_nodes.CreateStmt{}, &CreateTableHandler{})
	r.Register(pg_nodes.AlterTableStmt{}, &AlterTableHandler{})
	r.Register(pg_nodes.CreateEnumStmt{}, &CreateEnumHandler{})
	r.Register(pg_nodes.IndexStmt{}, &CreateIndexHandler{})  // 新增
	return r
}
```

- [ ] **步骤 3：编译验证**

运行：`go build ./internal/parser/...`

预期：PASS

- [ ] **步骤 4：运行全量测试**

运行：`go test ./...`

预期：PASS（Index/Enum handler 的 `fmt.Errorf` 会被 `visitNode` 包装为 `ParseError`，现有跳过测试的行为不变）

- [ ] **步骤 5：Commit**

```bash
git add internal/parser/index_handler.go
git add internal/parser/registry.go
git commit -m "feat(parser): 添加 CreateIndexHandler 框架（验证 OCP）"
```

---

### 阶段 8：最终验证

---

#### 任务 8.1：全量 CI 验证

- [ ] **步骤 1：运行全量测试**

```bash
go test ./...
```

预期：PASS

- [ ] **步骤 2：运行 vet**

```bash
go vet ./...
```

预期：无警告

- [ ] **步骤 3：运行 lint（如果已配置 golangci-lint）**

```bash
make lint
```

预期：PASS 或无新增警告

- [ ] **步骤 4：运行 make ci**

```bash
make ci
```

预期：PASS

- [ ] **步骤 5：最终 Commit（如 make ci 产生额外修改）**

```bash
git add -A
git commit -m "chore(parser): handler 重构最终验证与清理"
```

---

## 自检

### 1. 规格覆盖度

| 规格需求 | 对应任务 |
|---|---|
| `parserutil` 包提取辅助函数 | 任务 1.1 |
| `SchemaMutation` 接口 + Kind 常量 | 任务 2.1 |
| `MutationApplier` 遍历 + 错误收集 | 任务 2.2 |
| `CreateTableMutation.Apply` | 任务 3.1 |
| `AddColumnMutation.Apply`（含空表占位） | 任务 4.1 |
| `CreateTableHandler` | 任务 3.2 |
| `AlterTableHandler` | 任务 4.2 |
| `HandlerRegistry` reflect.Type 映射 | 任务 5.1 |
| `DefaultRegistry()` 显式注册 | 任务 5.1, 7.1, 7.2 |
| `NewParser()` / `NewParserWith()` | 任务 5.2 |
| 清理旧方法 | 任务 6.1 |
| Enum/Index handler 框架 | 任务 7.1, 7.2 |
| 端到端测试无回归 | 贯穿所有任务，最终任务 8.1 |

**覆盖度：100%，无遗漏。**

### 2. 占位符扫描

- 无 "待定" / "TODO"（除 Enum/Index handler 框架中的 `fmt.Errorf` 为有意设计）
- 无 "后续实现"
- 每个代码步骤包含完整代码块
- 无未定义的类型引用

### 3. 类型一致性

- `MutationKind` 常量前缀 `MutKind` 全程一致
- `SchemaMutation` 接口方法 `Kind()`, `Target()`, `Apply()` 全程一致
- `Handler.Handle(node pg_nodes.Node) ([]SchemaMutation, error)` 全程一致
- `Parser` struct 字段 `applier`, `registry` 全程一致

---

## 执行交接

**计划已完成并保存到 `docs/superpowers/plans/2026-05-07-parser-handler-refactor.md`。两种执行方式：**

**1. 子代理驱动（推荐）** - 每个任务调度一个新的子代理，任务间进行审查，快速迭代

**2. 内联执行** - 在当前会话中使用 executing-plans 执行任务，批量执行并设有检查点

**选哪种方式？**
