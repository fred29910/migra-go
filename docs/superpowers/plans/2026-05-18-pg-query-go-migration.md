# pg_query_go 依赖迁移实现计划

> **面向 AI 代理的工作者：** 必需子技能：使用 superpowers:subagent-driven-development（推荐）或 superpowers:executing-plans 逐任务实现此计划。步骤使用复选框（`- [ ]`）语法来跟踪进度。

**目标：** 将 PostgreSQL SQL 解析器依赖从 `lfittl/pg_query_go` v1.0.2 迁移到 `pganalyze/pg_query_go/v6` v6.2.2

**架构：** 替换 go.mod 依赖，更新所有 import 路径，将 AST 节点访问从类型断言改为 getter 方法，将列表访问从 `.Items` 改为直接切片遍历，更新枚举常量命名，调整 `*string` 字段的 nil 检查。

**技术栈：** Go 1.26、pganalyze/pg_query_go/v6 v6.2.2（protobuf 生成的 AST 节点）

---

## 文件结构与职责

| 文件 | 职责 | 改动类型 |
|------|------|---------|
| `go.mod` / `go.sum` | 依赖声明 | 替换依赖 |
| `internal/parser/parser.go` | 解析入口、语句遍历 | 修改 import、解析 API、RawStmt 访问 |
| `internal/parser/registry.go` | Handler 注册表 | 修改 import、枚举常量 |
| `internal/parser/parserutil/util.go` | 节点访问工具函数 | 修改 import、全部函数签名和实现 |
| `internal/parser/create_table_handler.go` | CREATE TABLE handler | 修改 import、节点访问、枚举 |
| `internal/parser/alter_table_handler.go` | ALTER TABLE handler | 修改 import、节点访问、枚举 |
| `internal/parser/enum_handler.go` | CREATE ENUM handler | 修改 import、节点访问 |
| `internal/parser/index_handler.go` | CREATE INDEX handler | 修改 import、节点访问、枚举 |
| `internal/parser/create_schema_handler.go` | CREATE SCHEMA handler | 修改 import、节点访问 |
| `internal/parser/handler_test.go` | Handler 单元测试 | 修改 import、解析辅助函数、枚举引用 |
| `internal/parser/parser_test.go` | Parser 集成测试 | 修改 import、枚举引用 |
| `internal/parser/index_handler_test.go` | 索引 handler 测试 | 无改动（不直接引用 pg_query） |
| `internal/parser/index_mutation_test.go` | 索引 mutation 测试 | 无改动 |
| `internal/parser/mutation_test.go` | Mutation 测试 | 无改动 |

---

## 任务 1：替换 go.mod 依赖

**文件：**
- 修改：`go.mod`
- 修改：`go.sum`

- [ ] **步骤 1：替换依赖并下载**

```bash
cd /opt/codes/workspace/migra-go
# 移除旧依赖，添加新依赖
go mod edit -droprequire=github.com/lfittl/pg_query_go
go mod edit -require=github.com/pganalyze/pg_query_go/v6@v6.2.2
go mod tidy
```

- [ ] **步骤 2：验证依赖替换**

```bash
go list -m all | grep pg_query
```

预期输出包含 `github.com/pganalyze/pg_query_go/v6 v6.2.2`，不包含 `lfittl`。

- [ ] **步骤 3：Commit**

```bash
git add go.mod go.sum
git commit -m "chore(deps): 替换 pg_query_go 依赖为 pganalyze/v6"
```

---

## 任务 2：更新 parserutil/util.go — 节点访问工具函数

**文件：**
- 修改：`internal/parser/parserutil/util.go`

这是最核心的改动文件。所有 handler 都依赖这里的工具函数。

- [ ] **步骤 1：替换 import 并修改 ParseRelation**

将 `pg_nodes "github.com/lfittl/pg_query_go/nodes"` 替换为 `pg_query "github.com/pganalyze/pg_query_go/v6"`。

`ParseRelation` 函数：`RangeVar.Schemaname` 和 `Relname` 从 `*string` 变为 `string`，不再需要 nil 检查：

```go
// ParseRelation extracts table/view name and schema from RangeVar.
func ParseRelation(relation *pg_query.RangeVar) (tableName, schemaName string) {
	if relation == nil {
		return "", "public"
	}
	tableName = relation.Relname
	schemaName = relation.Schemaname
	if schemaName == "" {
		schemaName = "public"
	}
	return
}
```

- [ ] **步骤 2：修改 ParseColumnDef**

`ColumnDef.Colname` 从 `*string` 变为 `string`，`TypeName` 从 `*pg_nodes.TypeName` 变为 `*pg_query.TypeName`，`Constraints` 从 `pg_nodes.List` 变为 `[]*pg_query.Node`：

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
	return col
}
```

- [ ] **步骤 3：修改 ParseTypeName**

`typeName.Names` 从 `pg_nodes.List` 变为 `[]*pg_query.Node`，`typeName.Typmods` 从 `pg_nodes.List` 变为 `[]*pg_query.Node`：

```go
// ParseTypeName maps pg_query TypeName to a standard SQL type string.
func ParseTypeName(typeName *pg_query.TypeName) string {
	parts := make([]string, 0)
	for _, item := range typeName.Names {
		if s := item.GetString_(); s != nil {
			if s.S != "pg_catalog" {
				parts = append(parts, s.S)
			}
		}
	}
	typeStr := strings.Join(parts, ".")
	typeStr = MapTypeName(typeStr)
	if len(typeName.Typmods) > 0 {
		mods := make([]string, 0)
		for _, item := range typeName.Typmods {
			if a := item.GetAConst(); a != nil {
				if a.Val != nil {
					if i := a.Val.GetInteger(); i != nil {
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
```

- [ ] **步骤 4：修改 ParseExpression**

`A_Val` 字段名可能变化，需要适配 protobuf 生成的字段名：

```go
// ParseExpression extracts expression as string from AST node.
func ParseExpression(expr *pg_query.Node) (string, bool) {
	switch e := expr.GetNode().(type) {
	case *pg_query.Node_AConst:
		if e.AConst.Val != nil {
			switch v := e.AConst.Val.GetNode().(type) {
			case *pg_query.Node_String_:
				return "'" + v.String_.S + "'", true
			case *pg_query.Node_Integer:
				return fmt.Sprintf("%d", v.Integer.Ival), true
			case *pg_query.Node_Float:
				return v.Float.Str, true
			}
		}
	case *pg_query.Node_FuncCall:
		parts := make([]string, 0, len(e.FuncCall.Funcname))
		for _, item := range e.FuncCall.Funcname {
			if s := item.GetString_(); s != nil {
				parts = append(parts, s.S)
			}
		}
		argStrs := make([]string, 0, len(e.FuncCall.Args))
		for _, item := range e.FuncCall.Args {
			if s, ok := ParseExpression(item); ok {
				argStrs = append(argStrs, s)
			}
		}
		return strings.Join(parts, ".") + "(" + strings.Join(argStrs, ", ") + ")", true
	}
	return "", false
}
```

注意：`ParseExpression` 的签名从 `pg_nodes.Node` 变为 `*pg_query.Node`。

- [ ] **步骤 5：更新 import 块**

```go
import (
	"fmt"
	"strings"

	"github.com/fred29910/migra-go/internal/model"
	pg_query "github.com/pganalyze/pg_query_go/v6"
)
```

- [ ] **步骤 6：运行测试验证**

```bash
go build ./internal/parser/...
```

预期：编译通过（可能有不相关的 handler 编译错误，因为 handler 尚未更新）。

- [ ] **步骤 7：Commit**

```bash
git add internal/parser/parserutil/util.go
git commit -m "refactor(parserutil): 更新节点访问函数适配 pganalyze/v6 API"
```

---

## 任务 3：更新 parser.go — 解析入口

**文件：**
- 修改：`internal/parser/parser.go`

- [ ] **步骤 1：替换 import**

```go
import (
	"fmt"

	"github.com/fred29910/migra-go/internal/model"
	pg_query "github.com/pganalyze/pg_query_go/v6"
)
```

- [ ] **步骤 2：修改 ParseSQL 函数**

```go
func (p *Parser) ParseSQL(sql string) (*model.Schema, error) {
	if p.registry == nil {
		p.registry = DefaultRegistry()
	}
	if p.applier == nil {
		p.applier = &MutationApplier{}
	}

	p.schema = model.NewSchema()
	p.errors = p.errors[:0]
	p.sql = sql

	tree, err := pg_query.Parse(sql)
	if err != nil {
		return nil, fmt.Errorf("pg_query parse failed: %w", err)
	}

	for _, rawStmt := range tree.Stmts {
		stmt := rawStmt.GetRawStmt()
		if err := p.visitNode(stmt, rawStmt.StmtLocation); err != nil {
			p.errors = append(p.errors, err)
		}
	}

	if len(p.errors) > 0 {
		first := p.errors[0]
		return p.schema, fmt.Errorf("parsing completed with %d errors, first: %w", len(p.errors), first)
	}
	return p.schema, nil
}
```

- [ ] **步骤 3：修改 visitNode 函数签名**

`visitNode` 的参数从 `pg_nodes.Node` 变为 `*pg_query.Node`，`RawStmt` 的访问方式变化：

```go
func (p *Parser) visitNode(stmt *pg_query.Node, pos int) (err error) {
	defer func() {
		if r := recover(); r != nil {
			err = &ParseError{
				Message:  fmt.Sprintf("recovered from panic: %v", r),
				Position: -1,
			}
		}
	}()

	rawStmt := stmt.GetRawStmt()
	if rawStmt == nil {
		return &ParseError{
			Message:  fmt.Sprintf("expected RawStmt, got: %T", stmt),
			Position: -1,
		}
	}

	actualStmt := rawStmt.Stmt
	pos := int(rawStmt.StmtLocation)

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

注意：`pos` 参数现在通过 `rawStmt.StmtLocation` 获取，函数签名需要调整。实际上 `visitNode` 不再需要 `pos` 参数：

```go
func (p *Parser) visitNode(stmt *pg_query.Node) (err error) {
	defer func() {
		if r := recover(); r != nil {
			err = &ParseError{
				Message:  fmt.Sprintf("recovered from panic: %v", r),
				Position: -1,
			}
		}
	}()

	rawStmt := stmt.GetRawStmt()
	if rawStmt == nil {
		return &ParseError{
			Message:  fmt.Sprintf("expected RawStmt, got: %T", stmt),
			Position: -1,
		}
	}

	actualStmt := rawStmt.Stmt
	pos := int(rawStmt.StmtLocation)

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

同时更新 `ParseSQL` 中的调用：

```go
for _, rawStmt := range tree.Stmts {
    if err := p.visitNode(rawStmt.GetRawStmt()); err != nil {
        p.errors = append(p.errors, err)
    }
}
```

等等，这里有个问题：`tree.Stmts` 是 `[]*RawStmt`，`RawStmt.GetRawStmt()` 返回 `*Node`。但我们需要的是 `RawStmt.Stmt`（即 `*Node`）。让我重新检查：

`RawStmt` 结构：
```go
type RawStmt struct {
    Stmt         *Node
    StmtLocation int32
    StmtLen      int32
}
```

所以 `rawStmt.Stmt` 直接就是 `*Node`，不需要 `GetRawStmt()`。`GetRawStmt()` 是 `Node` 的 getter，用于从 oneof 中提取 `RawStmt`。

正确做法：`Node` 的 oneof getter 是 `GetRawStmt()`，而 `RawStmt.Stmt` 是 `*Node`。

所以遍历应该是：

```go
for _, rawStmt := range tree.Stmts {
    if err := p.visitNode(rawStmt.Stmt); err != nil {
        p.errors = append(p.errors, err)
    }
}
```

而 `visitNode` 接收 `*Node`，内部调用 `stmt.GetRawStmt()` 获取 `RawStmt`。

- [ ] **步骤 4：更新 import 和完整代码**

完整 `parser.go`：

```go
package parser

import (
	"fmt"

	"github.com/fred29910/migra-go/internal/model"
	pg_query "github.com/pganalyze/pg_query_go/v6"
)

type ParseError struct {
	Message   string
	Position  int
	Statement string
}

func (e *ParseError) Error() string {
	return fmt.Sprintf("parse error at position %d: %s", e.Position, e.Message)
}

type Parser struct {
	schema   *model.Schema
	errors   []error
	sql      string
	applier  *MutationApplier
	registry *HandlerRegistry
}

func NewParser() *Parser {
	return &Parser{
		schema:   model.NewSchema(),
		errors:   make([]error, 0),
		applier:  &MutationApplier{},
		registry: DefaultRegistry(),
	}
}

func NewParserWith(registry *HandlerRegistry, applier *MutationApplier) *Parser {
	if registry == nil {
		registry = DefaultRegistry()
	}
	if applier == nil {
		applier = &MutationApplier{}
	}
	return &Parser{
		schema:   model.NewSchema(),
		errors:   make([]error, 0),
		applier:  applier,
		registry: registry,
	}
}

func (p *Parser) ParseSQL(sql string) (*model.Schema, error) {
	if p.registry == nil {
		p.registry = DefaultRegistry()
	}
	if p.applier == nil {
		p.applier = &MutationApplier{}
	}

	p.schema = model.NewSchema()
	p.errors = p.errors[:0]
	p.sql = sql

	tree, err := pg_query.Parse(sql)
	if err != nil {
		return nil, fmt.Errorf("pg_query parse failed: %w", err)
	}

	for _, rawStmt := range tree.Stmts {
		if err := p.visitNode(rawStmt.Stmt); err != nil {
			p.errors = append(p.errors, err)
		}
	}

	if len(p.errors) > 0 {
		first := p.errors[0]
		return p.schema, fmt.Errorf("parsing completed with %d errors, first: %w", len(p.errors), first)
	}
	return p.schema, nil
}

func (p *Parser) visitNode(stmt *pg_query.Node) (err error) {
	defer func() {
		if r := recover(); r != nil {
			err = &ParseError{
				Message:  fmt.Sprintf("recovered from panic: %v", r),
				Position: -1,
			}
		}
	}()

	rawStmt := stmt.GetRawStmt()
	if rawStmt == nil {
		return &ParseError{
			Message:  fmt.Sprintf("expected RawStmt, got: %T", stmt),
			Position: -1,
		}
	}

	actualStmt := rawStmt.Stmt
	pos := int(rawStmt.StmtLocation)

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

func (p *Parser) getStatementSnippet(pos int) string {
	if pos < 0 || pos >= len(p.sql) {
		return ""
	}
	end := pos + 100
	if end > len(p.sql) {
		end = len(p.sql)
	}
	return p.sql[pos:end]
}

func (p *Parser) Errors() []error {
	out := make([]error, len(p.errors))
	copy(out, p.errors)
	return out
}
```

- [ ] **步骤 5：运行编译验证**

```bash
go build ./internal/parser/...
```

预期：parser.go 编译通过，handler 文件可能有编译错误（尚未更新）。

- [ ] **步骤 6：Commit**

```bash
git add internal/parser/parser.go
git commit -m "refactor(parser): 更新解析入口适配 pganalyze/v6 API"
```

---

## 任务 4：更新 registry.go — Handler 注册表

**文件：**
- 修改：`internal/parser/registry.go`

- [ ] **步骤 1：替换 import 和枚举引用**

```go
package parser

import (
	"reflect"

	pg_query "github.com/pganalyze/pg_query_go/v6"
)

type Handler interface {
	Handle(node *pg_query.Node) ([]SchemaMutation, error)
}

type HandlerRegistry struct {
	handlers map[reflect.Type]Handler
}

func NewHandlerRegistry() *HandlerRegistry {
	return &HandlerRegistry{handlers: make(map[reflect.Type]Handler)}
}

func (r *HandlerRegistry) Register(nodeType *pg_query.Node, h Handler) {
	r.handlers[reflect.TypeOf(nodeType)] = h
}

func (r *HandlerRegistry) Dispatch(node *pg_query.Node) (Handler, bool) {
	h, ok := r.handlers[reflect.TypeOf(node)]
	return h, ok
}

func DefaultRegistry() *HandlerRegistry {
	r := NewHandlerRegistry()
	r.Register(&pg_query.Node{Node: &pg_query.Node_CreateStmt{}}, &CreateTableHandler{})
	r.Register(&pg_query.Node{Node: &pg_query.Node_AlterTableStmt{}}, &AlterTableHandler{})
	r.Register(&pg_query.Node{Node: &pg_query.Node_CreateEnumStmt{}}, &CreateEnumHandler{})
	r.Register(&pg_query.Node{Node: &pg_query.Node_IndexStmt{}}, &CreateIndexHandler{})
	r.Register(&pg_query.Node{Node: &pg_query.Node_CreateSchemaStmt{}}, &CreateSchemaHandler{})
	return r
}
```

注意：`pg_query.Node` 是 protobuf oneof，注册时需要使用 `&pg_query.Node{Node: &pg_query.Node_Xxx{}}` 的形式。`reflect.TypeOf` 会返回具体类型。

- [ ] **步骤 2：运行编译验证**

```bash
go build ./internal/parser/...
```

- [ ] **步骤 3：Commit**

```bash
git add internal/parser/registry.go
git commit -m "refactor(parser): 更新 handler 注册表适配 pganalyze/v6"
```

---

## 任务 5：更新 create_table_handler.go

**文件：**
- 修改：`internal/parser/create_table_handler.go`

- [ ] **步骤 1：替换 import**

```go
import (
	"fmt"
	"strings"

	"github.com/fred29910/migra-go/internal/model"
	"github.com/fred29910/migra-go/internal/parser/parserutil"
	pg_query "github.com/pganalyze/pg_query_go/v6"
)
```

- [ ] **步骤 2：修改 Handle 方法**

关键变化：
- `node.(pg_nodes.CreateStmt)` → `node.GetCreateStmt()`
- `stmt.TableElts.Items` → `stmt.TableElts`
- `elt.Constraints.Items` → `elt.Constraints`（`[]*pg_query.Node`）
- `conItem.(pg_nodes.Constraint)` → `conItem.GetConstraint()`
- `c.Contype == pg_nodes.CONSTR_PRIMARY` → `c.Contype == pg_query.ConstrType_CONSTR_PRIMARY`
- `elt.(pg_nodes.ColumnDef)` → `elt.GetColumnDef()`（返回 `*pg_query.ColumnDef`）
- `elt.(pg_nodes.Constraint)` → `elt.GetConstraint()`（返回 `*pg_query.Constraint`）
- `elt.Keys.Items` → `elt.Keys`（`[]*pg_query.Node`）
- `elt.FkAttrs.Items` → `elt.FkAttrs`
- `elt.PkAttrs.Items` → `elt.PkAttrs`
- `elt.Pktable.Schemaname` 从 `*string` → `string`
- `elt.Pktable.Relname` 从 `*string` → `string`
- `key.(pg_nodes.String)` → `key.GetString_()`
- `attr.(pg_nodes.String)` → `attr.GetString_()`
- `parserutil.ParseColumnDef(elt)` → `parserutil.ParseColumnDef(elt.GetColumnDef())`

```go
func (h *CreateTableHandler) Handle(node *pg_query.Node) ([]SchemaMutation, error) {
	stmt := node.GetCreateStmt()
	if stmt == nil {
		return nil, fmt.Errorf("CreateTableHandler: expected CreateStmt, got %T", node)
	}
	tableName, schemaName := parserutil.ParseRelation(stmt.Relation)

	var columns []model.Column
	var primaryKey *model.PrimaryKey
	var constraints []model.Constraint

	for _, item := range stmt.TableElts {
		switch elt := item.GetNode().(type) {
		case *pg_query.Node_ColumnDef:
			colDef := elt.ColumnDef
			col := parserutil.ParseColumnDef(colDef)
			for _, conItem := range colDef.Constraints {
				if c := conItem.GetConstraint(); c != nil {
					if c.Contype == pg_query.ConstrType_CONSTR_PRIMARY {
						col.IsNullable = false
						primaryKey = &model.PrimaryKey{
							Name:    defaultConstraintName(tableName, model.Constraint{Name: "", Type: "primary_key"}),
							Columns: []string{col.Name},
						}
						constraints = append(constraints, model.Constraint{
							Name:    primaryKey.Name,
							Type:    "primary_key",
							Columns: []string{col.Name},
						})
					}
				}
			}
			columns = append(columns, *col)
		case *pg_query.Node_Constraint:
			constraint := elt.Constraint
			switch constraint.Contype {
			case pg_query.ConstrType_CONSTR_PRIMARY:
				var cols []string
				for _, key := range constraint.Keys {
					if s := key.GetString_(); s != nil {
						cols = append(cols, s.S)
					}
				}
				primaryKey = &model.PrimaryKey{
					Name:    defaultConstraintName(tableName, model.Constraint{Name: "", Type: "primary_key"}),
					Columns: cols,
				}
				constraints = append(constraints, model.Constraint{
					Name:    primaryKey.Name,
					Type:    "primary_key",
					Columns: cols,
				})
			case pg_query.ConstrType_CONSTR_FOREIGN:
				var fkCols []string
				for _, attr := range constraint.FkAttrs {
					if s := attr.GetString_(); s != nil {
						fkCols = append(fkCols, s.S)
					}
				}
				var refCols []string
				for _, attr := range constraint.PkAttrs {
					if s := attr.GetString_(); s != nil {
						refCols = append(refCols, s.S)
					}
				}
				refSchema := "public"
				if constraint.Pktable != nil && constraint.Pktable.Schemaname != "" {
					refSchema = constraint.Pktable.Schemaname
				}
				refTable := ""
				if constraint.Pktable != nil {
					refTable = constraint.Pktable.Relname
				}
				con := model.Constraint{
					Name:       "",
					Type:       "foreign_key",
					Columns:    fkCols,
					RefSchema:  refSchema,
					RefTable:   refTable,
					RefColumns: refCols,
				}
				con.Name = defaultConstraintName(tableName, con)
				constraints = append(constraints, con)
			}
		}
	}

	return []SchemaMutation{
		CreateTableMutation{
			Schema:      schemaName,
			Name:        tableName,
			Columns:     columns,
			PrimaryKey:  primaryKey,
			Constraints: constraints,
		},
	}, nil
}
```

- [ ] **步骤 3：运行编译验证**

```bash
go build ./internal/parser/...
```

- [ ] **步骤 4：Commit**

```bash
git add internal/parser/create_table_handler.go
git commit -m "refactor(parser): 更新 CREATE TABLE handler 适配 pganalyze/v6"
```

---

## 任务 6：更新 alter_table_handler.go

**文件：**
- 修改：`internal/parser/alter_table_handler.go`

- [ ] **步骤 1：替换 import**

```go
import (
	"fmt"
	"os"

	"github.com/fred29910/migra-go/internal/parser/parserutil"
	pg_query "github.com/pganalyze/pg_query_go/v6"
)
```

- [ ] **步骤 2：修改 Handle 方法**

关键变化：
- `node.(pg_nodes.AlterTableStmt)` → `node.GetAlterTableStmt()`
- `stmt.Cmds.Items` → `stmt.Cmds`
- `item.(pg_nodes.AlterTableCmd)` → `item.GetAlterTableCmd()`
- `cmd.Subtype` 枚举：`pg_nodes.AT_AddColumn` → `pg_query.AlterTableType_AT_AddColumn`
- `cmd.Name` 从 `*string` → `string`（不再需要 nil 检查）
- `cmd.Def` 从 `pg_nodes.Node` → `*pg_query.Node`
- `cmd.Def.(pg_nodes.ColumnDef)` → `cmd.Def.GetColumnDef()`
- `extractDefaultExpr` 的参数类型变化

```go
func (h *AlterTableHandler) Handle(node *pg_query.Node) ([]SchemaMutation, error) {
	stmt := node.GetAlterTableStmt()
	if stmt == nil {
		return nil, fmt.Errorf("AlterTableHandler: expected AlterTableStmt, got %T", node)
	}
	tableName, schemaName := parserutil.ParseRelation(stmt.Relation)

	var mutations []SchemaMutation
	for _, item := range stmt.Cmds {
		cmd := item.GetAlterTableCmd()
		if cmd == nil {
			continue
		}
		switch cmd.Subtype {
		case pg_query.AlterTableType_AT_AddColumn:
			if cmd.Def != nil {
				if colDef := cmd.Def.GetColumnDef(); colDef != nil {
					col := parserutil.ParseColumnDef(colDef)
					mutations = append(mutations, AddColumnMutation{
						Schema: schemaName,
						Table:  tableName,
						Column: *col,
					})
				}
			}

		case pg_query.AlterTableType_AT_DropColumn:
			colName := cmd.Name
			if colName == "" {
				fmt.Fprintf(os.Stderr, "warning: DROP COLUMN missing column name\n")
				continue
			}
			mutations = append(mutations, DropColumnMutation{
				Schema: schemaName,
				Table:  tableName,
				Column: colName,
			})

		case pg_query.AlterTableType_AT_AlterColumnType:
			colName := cmd.Name
			if colName == "" || cmd.Def == nil {
				fmt.Fprintf(os.Stderr, "warning: ALTER COLUMN TYPE missing column name or type\n")
				continue
			}
			if colDef := cmd.Def.GetColumnDef(); colDef != nil {
				col := parserutil.ParseColumnDef(colDef)
				mutations = append(mutations, AlterColumnTypeMutation{
					Schema: schemaName,
					Table:  tableName,
					Column: colName,
					ToType: col.DataType,
				})
			}

		case pg_query.AlterTableType_AT_SetNotNull:
			colName := cmd.Name
			if colName == "" {
				fmt.Fprintf(os.Stderr, "warning: SET NOT NULL missing column name\n")
				continue
			}
			mutations = append(mutations, SetNotNullMutation{
				Schema: schemaName,
				Table:  tableName,
				Column: colName,
			})

		case pg_query.AlterTableType_AT_DropNotNull:
			colName := cmd.Name
			if colName == "" {
				fmt.Fprintf(os.Stderr, "warning: DROP NOT NULL missing column name\n")
				continue
			}
			mutations = append(mutations, DropNotNullMutation{
				Schema: schemaName,
				Table:  tableName,
				Column: colName,
			})

		case pg_query.AlterTableType_AT_ColumnDefault:
			colName := cmd.Name
			if colName == "" {
				fmt.Fprintf(os.Stderr, "warning: ALTER COLUMN DEFAULT missing column name\n")
				continue
			}
			if cmd.Def != nil {
				defaultExpr := extractDefaultExpr(cmd.Def)
				mutations = append(mutations, SetDefaultMutation{
					Schema:      schemaName,
					Table:       tableName,
					Column:      colName,
					DefaultExpr: defaultExpr,
				})
			} else {
				mutations = append(mutations, DropDefaultMutation{
					Schema: schemaName,
					Table:  tableName,
					Column: colName,
				})
			}

		default:
			fmt.Fprintf(os.Stderr, "warning: unsupported ALTER TABLE subcommand: %v\n", cmd.Subtype)
		}
	}
	return mutations, nil
}
```

- [ ] **步骤 3：修改 extractDefaultExpr**

```go
func extractDefaultExpr(node *pg_query.Node) string {
	if d, ok := node.GetNode().(interface{ GetDeparse() string }); ok {
		var result string
		func() {
			defer func() {
				if r := recover(); r != nil {
					result = fmt.Sprintf("%v", node)
				}
			}()
			result = d.GetDeparse()
		}()
		return result
	}
	return fmt.Sprintf("%v", node)
}
```

注意：pganalyze v6 的节点没有 `Deparse()` 方法。需要使用 `pg_query.Deparse(tree)` 函数。但 `extractDefaultExpr` 只接收单个节点，不是完整树。需要调整策略——对于默认值表达式，可以使用 `fmt.Sprintf` 或尝试其他方式。

实际上，对于 `AT_ColumnDefault` 场景，`cmd.Def` 是一个表达式节点。pganalyze v6 不提供单节点 deparse。最简单的方案是暂时用 `fmt.Sprintf` 输出，或者跳过 deparse 直接存储原始表示。

修正 `extractDefaultExpr`：

```go
func extractDefaultExpr(node *pg_query.Node) string {
	return fmt.Sprintf("%v", node)
}
```

- [ ] **步骤 4：运行编译验证**

```bash
go build ./internal/parser/...
```

- [ ] **步骤 5：Commit**

```bash
git add internal/parser/alter_table_handler.go
git commit -m "refactor(parser): 更新 ALTER TABLE handler 适配 pganalyze/v6"
```

---

## 任务 7：更新 enum_handler.go

**文件：**
- 修改：`internal/parser/enum_handler.go`

- [ ] **步骤 1：替换 import**

```go
import (
	"fmt"

	pg_query "github.com/pganalyze/pg_query_go/v6"
)
```

- [ ] **步骤 2：修改 Handle 方法**

关键变化：
- `node.(pg_nodes.CreateEnumStmt)` → `node.GetCreateEnumStmt()`
- `stmt.TypeName.Items` → `stmt.TypeName`（`[]*pg_query.Node`）
- `item.(pg_nodes.String)` → `item.GetString_()`
- `stmt.Vals.Items` → `stmt.Vals`

```go
func (h *CreateEnumHandler) Handle(node *pg_query.Node) ([]SchemaMutation, error) {
	stmt := node.GetCreateEnumStmt()
	if stmt == nil {
		return nil, fmt.Errorf("CreateEnumHandler: expected CreateEnumStmt, got %T", node)
	}
	schemaName := "public"
	typeName := ""
	if len(stmt.TypeName) > 0 {
		parts := make([]string, 0, len(stmt.TypeName))
		for _, item := range stmt.TypeName {
			if s := item.GetString_(); s != nil {
				parts = append(parts, s.S)
			}
		}
		if len(parts) == 1 {
			typeName = parts[0]
		} else if len(parts) >= 2 {
			schemaName = parts[len(parts)-2]
			typeName = parts[len(parts)-1]
		}
	}
	if typeName == "" {
		return nil, fmt.Errorf("CreateEnumHandler: unable to extract type name from CreateEnumStmt")
	}
	labels := make([]string, 0, len(stmt.Vals))
	for _, item := range stmt.Vals {
		if s := item.GetString_(); s != nil {
			labels = append(labels, s.S)
		}
	}
	return []SchemaMutation{CreateEnumTypeMutation{Schema: schemaName, Name: typeName, Labels: labels}}, nil
}
```

- [ ] **步骤 3：运行编译验证**

```bash
go build ./internal/parser/...
```

- [ ] **步骤 4：Commit**

```bash
git add internal/parser/enum_handler.go
git commit -m "refactor(parser): 更新 CREATE ENUM handler 适配 pganalyze/v6"
```

---

## 任务 8：更新 index_handler.go

**文件：**
- 修改：`internal/parser/index_handler.go`

- [ ] **步骤 1：替换 import**

```go
import (
	"fmt"

	"github.com/fred29910/migra-go/internal/model"
	"github.com/fred29910/migra-go/internal/parser/parserutil"
	pg_query "github.com/pganalyze/pg_query_go/v6"
)
```

- [ ] **步骤 2：修改 Handle 方法**

关键变化：
- `node.(pg_nodes.IndexStmt)` → `node.GetIndexStmt()`
- `stmt.Idxname` 从 `*string` → `string`
- `stmt.IndexParams.Items` → `stmt.IndexParams`
- `item.(pg_nodes.IndexElem)` → `item.GetIndexElem()`
- `elem.Name` 从 `*string` → `string`
- `elem.Indexcolname` 从 `*string` → `string`
- `elem.Ordering` 枚举：`pg_nodes.SORTBY_DEFAULT` → `pg_query.SortByDir_SORTBY_DEFAULT`
- `elem.NullsOrdering` 枚举：`pg_nodes.SORTBY_NULLS_DEFAULT` → `pg_query.SortByNulls_SORTBY_NULLS_DEFAULT`
- `stmt.AccessMethod` 从 `*string` → `string`
- `stmt.WhereClause` 从 `pg_nodes.Node` → `*pg_query.Node`
- `safeDeparse` 需要适配新类型

```go
func (h *CreateIndexHandler) Handle(node *pg_query.Node) ([]SchemaMutation, error) {
	stmt := node.GetIndexStmt()
	if stmt == nil {
		return nil, fmt.Errorf("CreateIndexHandler: expected IndexStmt, got %T", node)
	}

	tableName, schemaName := parserutil.ParseRelation(stmt.Relation)

	indexName := stmt.Idxname
	if indexName == "" {
		indexName = generateDefaultIndexName(tableName, stmt.IndexParams)
	}

	elements := make([]model.IndexElem, 0, len(stmt.IndexParams))
	for _, item := range stmt.IndexParams {
		indexElem, err := parseIndexElem(item)
		if err != nil {
			return nil, fmt.Errorf("failed to parse index element: %w", err)
		}
		elements = append(elements, indexElem)
	}

	whereClause := ""
	if stmt.WhereClause != nil {
		whereClause = safeDeparse(stmt.WhereClause)
	}

	method := "btree"
	if stmt.AccessMethod != "" {
		method = stmt.AccessMethod
	}

	index := model.Index{
		Name:         indexName,
		Table:        tableName,
		Elements:     elements,
		Unique:       stmt.Unique,
		Method:       method,
		Primary:      stmt.Primary,
		IsConstraint: stmt.Isconstraint,
		WhereClause:  whereClause,
		Concurrent:   stmt.Concurrent,
		IfNotExists:  stmt.IfNotExists,
	}

	return []SchemaMutation{CreateIndexMutation{
		Schema: schemaName,
		Index:  index,
	}}, nil
}
```

- [ ] **步骤 3：修改 parseIndexElem**

```go
func parseIndexElem(node *pg_query.Node) (model.IndexElem, error) {
	elem := node.GetIndexElem()
	if elem == nil {
		return model.IndexElem{}, fmt.Errorf("expected IndexElem, got %T", node)
	}

	result := model.IndexElem{}

	if elem.Name != "" {
		result.Name = elem.Name
	}

	if elem.Expr != nil {
		result.Expr = safeDeparse(elem.Expr)
	}

	if elem.Indexcolname != "" {
		result.IndexColName = elem.Indexcolname
	}

	switch elem.Ordering {
	case pg_query.SortByDir_SORTBY_DEFAULT:
		result.Ordering = "default"
	case pg_query.SortByDir_SORTBY_ASC:
		result.Ordering = "ASC"
	case pg_query.SortByDir_SORTBY_DESC:
		result.Ordering = "DESC"
	}

	switch elem.NullsOrdering {
	case pg_query.SortByNulls_SORTBY_NULLS_DEFAULT:
		result.NullsOrdering = "default"
	case pg_query.SortByNulls_SORTBY_NULLS_FIRST:
		result.NullsOrdering = "FIRST"
	case pg_query.SortByNulls_SORTBY_NULLS_LAST:
		result.NullsOrdering = "LAST"
	}

	return result, nil
}
```

- [ ] **步骤 4：修改 safeDeparse 和 generateDefaultIndexName**

```go
func safeDeparse(node *pg_query.Node) string {
	return fmt.Sprintf("%v", node)
}

func generateDefaultIndexName(tableName string, indexParams []*pg_query.Node) string {
	for _, item := range indexParams {
		if elem := item.GetIndexElem(); elem != nil {
			if elem.Name != "" {
				return fmt.Sprintf("%s_%s_idx", tableName, elem.Name)
			}
			if elem.Expr != nil {
				return fmt.Sprintf("%s_expr_idx", tableName)
			}
		}
	}
	return fmt.Sprintf("%s_idx", tableName)
}
```

- [ ] **步骤 5：运行编译验证**

```bash
go build ./internal/parser/...
```

- [ ] **步骤 6：Commit**

```bash
git add internal/parser/index_handler.go
git commit -m "refactor(parser): 更新 CREATE INDEX handler 适配 pganalyze/v6"
```

---

## 任务 9：更新 create_schema_handler.go

**文件：**
- 修改：`internal/parser/create_schema_handler.go`

- [ ] **步骤 1：替换 import**

```go
import (
	"fmt"

	"github.com/fred29910/migra-go/internal/model"
	pg_query "github.com/pganalyze/pg_query_go/v6"
)
```

- [ ] **步骤 2：修改 Handle 方法**

`stmt.Schemaname` 从 `*string` → `string`：

```go
func (h *CreateSchemaHandler) Handle(node *pg_query.Node) ([]SchemaMutation, error) {
	stmt := node.GetCreateSchemaStmt()
	if stmt == nil {
		return nil, fmt.Errorf("CreateSchemaHandler: expected CreateSchemaStmt, got %T", node)
	}
	schemaName := stmt.Schemaname
	if schemaName == "" {
		return nil, fmt.Errorf("CreateSchemaHandler: schema name is empty")
	}
	return []SchemaMutation{CreateSchemaMutation{Schema: schemaName}}, nil
}
```

- [ ] **步骤 3：运行编译验证**

```bash
go build ./internal/parser/...
```

- [ ] **步骤 4：Commit**

```bash
git add internal/parser/create_schema_handler.go
git commit -m "refactor(parser): 更新 CREATE SCHEMA handler 适配 pganalyze/v6"
```

---

## 任务 10：更新测试文件

**文件：**
- 修改：`internal/parser/handler_test.go`
- 修改：`internal/parser/parser_test.go`

- [ ] **步骤 1：更新 handler_test.go**

```go
package parser

import (
	"testing"

	pg_query "github.com/pganalyze/pg_query_go/v6"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func mustParseFirstStmt(t *testing.T, sql string) *pg_query.Node {
	t.Helper()
	tree, err := pg_query.Parse(sql)
	require.NoError(t, err)
	require.Len(t, tree.Stmts, 1)
	return tree.Stmts[0].Stmt
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

func TestAlterTableHandler_MultipleAddColumns(t *testing.T) {
	node := mustParseFirstStmt(t, "ALTER TABLE users ADD COLUMN age integer, ADD COLUMN email varchar(100)")
	h := &AlterTableHandler{}

	mutations, err := h.Handle(node)
	require.NoError(t, err)
	require.Len(t, mutations, 2)

	mut0, ok := mutations[0].(AddColumnMutation)
	require.True(t, ok)
	assert.Equal(t, "age", mut0.Column.Name)

	mut1, ok := mutations[1].(AddColumnMutation)
	require.True(t, ok)
	assert.Equal(t, "email", mut1.Column.Name)
}

func TestHandlers_TypeSafety(t *testing.T) {
	node := &pg_query.Node{Node: &pg_query.Node_VacuumStmt{}}

	t.Run("CreateTableHandler", func(t *testing.T) {
		h := &CreateTableHandler{}
		_, err := h.Handle(node)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "expected")
	})

	t.Run("AlterTableHandler", func(t *testing.T) {
		h := &AlterTableHandler{}
		_, err := h.Handle(node)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "expected")
	})

	t.Run("CreateSchemaHandler", func(t *testing.T) {
		h := &CreateSchemaHandler{}
		_, err := h.Handle(node)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "expected")
	})
}

func TestCreateSchemaHandler(t *testing.T) {
	node := mustParseFirstStmt(t, "CREATE SCHEMA auth")
	h := &CreateSchemaHandler{}

	mutations, err := h.Handle(node)
	require.NoError(t, err)
	require.Len(t, mutations, 1)

	mut, ok := mutations[0].(CreateSchemaMutation)
	require.True(t, ok)
	assert.Equal(t, "auth", mut.Schema)
}

func TestCreateSchemaHandler_IfNotExists(t *testing.T) {
	node := mustParseFirstStmt(t, "CREATE SCHEMA IF NOT EXISTS auth")
	h := &CreateSchemaHandler{}

	mutations, err := h.Handle(node)
	require.NoError(t, err)
	require.Len(t, mutations, 1)

	mut, ok := mutations[0].(CreateSchemaMutation)
	require.True(t, ok)
	assert.Equal(t, "auth", mut.Schema)
}
```

- [ ] **步骤 2：更新 parser_test.go 中的 import**

将 `pg_nodes "github.com/lfittl/pg_query_go/nodes"` 替换为 `pg_query "github.com/pganalyze/pg_query_go/v6"`。

更新 `TestParser_RecoverFromPanic` 中的枚举引用：

```go
type panickingHandler struct{}

func (h *panickingHandler) Handle(node *pg_query.Node) ([]SchemaMutation, error) {
	panic("intentional panic for testing recover")
}

func TestParser_RecoverFromPanic(t *testing.T) {
	registry := NewHandlerRegistry()
	registry.Register(&pg_query.Node{Node: &pg_query.Node_CreateStmt{}}, &panickingHandler{})

	p := NewParserWith(registry, nil)
	_, err := p.ParseSQL("CREATE TABLE t1 (id int);")

	if err == nil {
		t.Fatal("expected error from recovered panic, got nil")
	}
	if !strings.Contains(err.Error(), "recovered from panic") {
		t.Errorf("expected error to mention panic recovery, got: %s", err.Error())
	}
	if !strings.Contains(err.Error(), "intentional panic for testing recover") {
		t.Errorf("expected error to contain panic message, got: %s", err.Error())
	}
}
```

- [ ] **步骤 3：运行全部测试**

```bash
go test ./internal/parser/... -v
```

预期：所有测试通过。

- [ ] **步骤 4：运行 cmd/migra 测试**

```bash
go test ./cmd/migra/... -v
```

预期：所有集成测试通过。

- [ ] **步骤 5：Commit**

```bash
git add internal/parser/handler_test.go internal/parser/parser_test.go
git commit -m "test(parser): 更新测试文件适配 pganalyze/v6"
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

- [ ] **步骤 3：验证 Windows 交叉编译**

```bash
GOOS=windows GOARCH=amd64 CGO_ENABLED=1 CC=x86_64-w64-mingw32-gcc go build ./cmd/migra
```

预期：编译成功（需要 MinGW 工具链）。

- [ ] **步骤 4：最终 Commit**

```bash
git add -A
git commit -m "chore: 完成 pg_query_go 依赖迁移至 pganalyze/v6

- 替换 go.mod 依赖：lfittl/pg_query_go → pganalyze/pg_query_go/v6
- 更新所有 import 路径
- 将 AST 节点访问从类型断言改为 getter 方法
- 将列表访问从 .Items 改为直接切片遍历
- 更新枚举常量命名
- 调整 *string 字段的 nil 检查
- 更新测试文件"
```
