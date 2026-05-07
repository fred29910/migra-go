# Parser Handler 重构设计：SchemaMutation 列表模式

- 日期：2026-05-07
- 状态：待审查

## 目标

将 `internal/parser` 从 "硬编码 switch + 内联修改 schema" 模式，重构为 "插件式 Handler + 返回变更描述 + 统一应用" 模式。

**解决的问题：**
1. 每新增一个 DDL 类型需修改 `visitNode` 的 switch，违反开闭原则
2. Handler 直接修改 `p.schema`，无法独立测试
3. Parser 文件承载辅助函数 + 编排逻辑 + 业务逻辑，过大且边界不清

**预期收益：**
1. 新增 DDL 类型只需实现 Handler + Mutation + Applier 分支（三处）
2. Handler 纯函数化，可独立单元测试（输入 AST → 断言 mutation 列表）
3. 辅助函数提取到 `parserutil` 包，供 handler 与 parser 复用

## 架构概览

```
SQL → pg_query.Parse() → []pg_nodes.Node
                              │
                              ▼
                    ┌──────────────────┐
                    │ HandlerRegistry  │ ← 路由 AST 到对应 Handler
                    └──────────────────┘
                              │
                              ▼
                    ┌──────────────────┐
                    │   StmtHandler    │ ← 纯函数：AST → []SchemaMutation
                    └──────────────────┘
                              │
                              ▼
                    ┌──────────────────┐
                    │ MutationApplier  │ ← 统一应用 mutations 到 model.Schema
                    └──────────────────┘
```

## 一、SchemaMutation 类型系统

Mutation 是纯数据描述，不含执行逻辑。

```go
package parser

// SchemaMutationKind identifies the type of mutation
type SchemaMutationKind string

const (
    MutCreateTable     SchemaMutationKind = "create_table"
    MutAddColumn       SchemaMutationKind = "add_column"
    MutCreateEnumType  SchemaMutationKind = "create_enum_type"
    MutAddEnumLabel    SchemaMutationKind = "add_enum_label"
    MutCreateIndex     SchemaMutationKind = "create_index"
    MutAlterColumnType SchemaMutationKind = "alter_column_type"
    MutDropColumn      SchemaMutationKind = "drop_column"
    MutAddConstraint   SchemaMutationKind = "add_constraint"
    // ... extensible
)

type SchemaMutation interface {
    Kind() SchemaMutationKind
    Target() model.ObjectKey
}
```

**核心 Mutation 类型：**

| Mutation | 字段 | 描述 |
|---|---|---|
| `MutCreateTable` | Schema, Name, []Column | 创建表 |
| `MutAddColumn` | Schema, Table, Column | 向已有表添加列 |
| `MutCreateEnumType` | Schema, Name, []string(labels) | 创建枚举类型 |
| `MutAddEnumLabel` | Schema, Type, Label | 向枚举添加值 |
| `MutCreateIndex` | Schema, Index | 创建索引 |
| `MutAddConstraint` | Schema, Table, Constraint | 添加约束 |

所有 mutation 为**值类型**（非指针），`Target()` 返回 `model.ObjectKey` 用于错误定位。

## 二、StmtHandler 接口与注册表

```go
package parser

// StmtHandler parses a single pg_query AST node into schema mutations.
type StmtHandler interface {
    CanHandle(node pg_nodes.Node) bool
    Handle(node pg_nodes.Node) ([]SchemaMutation, error)
}

// HandlerRegistry routes AST nodes to their handlers.
type HandlerRegistry struct {
    handlers []StmtHandler
}

func DefaultRegistry() *HandlerRegistry {
    r := &HandlerRegistry{}
    r.Register(
        &CreateTableHandler{},
        &AlterTableHandler{},
        &CreateEnumHandler{},
        &CreateIndexHandler{},
    )
    return r
}

func (r *HandlerRegistry) Dispatch(node pg_nodes.Node) (StmtHandler, bool) {
    for _, h := range r.handlers {
        if h.CanHandle(node) {
            return h, true
        }
    }
    return nil, false
}
```

## 三、MutationApplier

```go
package parser

type MutationApplier struct {
    errors []error
}

func (a *MutationApplier) Apply(schema *model.Schema, mutations []SchemaMutation) error {
    for _, mut := range mutations {
        if err := a.applyOne(schema, mut); err != nil {
            a.errors = append(a.errors, &MutationError{Mutation: mut, Cause: err})
        }
    }
    // 返回聚合错误（包装首个）
}

func (a *MutationApplier) applyOne(schema *model.Schema, mut SchemaMutation) error {
    switch m := mut.(type) {
    case MutCreateTable:
        return a.applyCreateTable(schema, m)
    case MutAddColumn:
        return a.applyAddColumn(schema, m)
    // ... 其他 case
    }
}
```

**一致性校验：**
- `MutAddColumn` 目标表不存在时，Applier 自动创建空表（适配 `ALTER TABLE` 在 `CREATE TABLE` 之前的 SQL 顺序）
- 重复创建表返回错误但不中断后续 mutation
- `MutationError` 包装 mutation 上下文

## 四、Handler 实现示例

```go
// internal/parser/handlers/create_table.go
package handlers

type CreateTableHandler struct{}

func (h *CreateTableHandler) CanHandle(node pg_nodes.Node) bool {
    _, ok := node.(pg_nodes.CreateStmt)
    return ok
}

func (h *CreateTableHandler) Handle(node pg_nodes.Node) ([]parser.SchemaMutation, error) {
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

    return []parser.SchemaMutation{
        parser.MutCreateTable{Schema: schemaName, Name: tableName, Columns: columns},
    }, nil
}
```

## 五、辅助函数提取

```go
// internal/parser/parserutil/util.go
package parserutil

func ParseRelation(relation *pg_nodes.RangeVar) (tableName, schemaName string)
func ParseColumnDef(elt pg_nodes.ColumnDef) *model.Column
func ParseTypeName(typeName pg_nodes.TypeName) string
func ParseExpression(expr pg_nodes.Node) (string, bool)
```

提取后：
- `parserutil` 为无状态工具包
- `internal/parser/handlers/` 每个 handler 独立文件
- `parser.go` 仅保留编排逻辑（ParseSQL → registry.Dispatch → handler.Handle → applier.Apply）

## 六、重构后的 Parser 核心

```go
// internal/parser/parser.go
type Parser struct {
    registry *HandlerRegistry
    applier  *MutationApplier
    schema   *model.Schema
    errors   []error
    sql      string
}

func (p *Parser) ParseSQL(sql string) (*model.Schema, error) {
    p.schema = model.NewSchema()
    p.errors = p.errors[:0]
    p.sql = sql

    tree, err := pg.Parse(sql)
    if err != nil {
        return nil, fmt.Errorf("pg_query parse failed: %w", err)
    }

    for _, stmt := range tree.Statements {
        if err := p.visitNode(stmt); err != nil {
            p.errors = append(p.errors, err)
        }
    }

    if len(p.errors) > 0 {
        return p.schema, fmt.Errorf("parsing completed with %d errors, first: %w", len(p.errors), p.errors[0])
    }
    return p.schema, nil
}

func (p *Parser) visitNode(stmt pg_nodes.Node) error {
    rawStmt, ok := stmt.(pg_nodes.RawStmt)
    if !ok {
        return &ParseError{Message: fmt.Sprintf("expected RawStmt, got: %T", stmt)}
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

## 实施范围

**本次重构涉及文件：**

1. **新增：**
   - `internal/parser/mutation.go` — SchemaMutation 类型与接口
   - `internal/parser/registry.go` — HandlerRegistry
   - `internal/parser/applier.go` — MutationApplier
   - `internal/parser/parserutil/util.go` — 辅助函数提取
   - `internal/parser/handlers/create_table.go` — CreateTableHandler
   - `internal/parser/handlers/alter_table.go` — AlterTableHandler
   - `internal/parser/handlers/enum.go` — CreateEnumHandler（框架）
   - `internal/parser/handlers/index.go` — CreateIndexHandler（框架）

2. **修改：**
   - `internal/parser/parser.go` — 移除 handler 和辅助函数，保留编排
   - `internal/parser/parser_test.go` — 更新测试适配新结构
   - 所有用到 `parser.go` 辅助函数的外部文件（如有）

**本次重构不涉及：**
- diff/render 模块
- introspect 模块
- model 类型本身（Schema/Table/Column 等不变）

## 新增 DDL 类型的流程（开闭原则验证）

以新增 `CREATE SCHEMA` 支持为例：

1. 实现 `CreateSchemaHandler`（实现 `StmtHandler`）
2. 新增 `MutCreateSchema` 类型
3. 在 `MutationApplier.applyOne` 中新增 `case MutCreateSchema:`
4. 在 `DefaultRegistry().Register()` 中注册 handler

**无需修改 `parser.go` 的 switch 或核心逻辑。**

## 测试策略

1. **Handler 单元测试**：每个 handler 独立测试（AST → mutation 断言）
2. **Applier 单元测试**：mock schema + mutation 列表 → 断言 schema 状态
3. **集成测试**：`parser_test.go` 中的端到端测试保持不变，确保行为兼容
4. **回归测试**：运行现有 `TestIntegrationParserToDiff`

## 验收标准

1. `go build ./...` 编译通过
2. `go test ./...` 全量测试通过
3. `go vet ./...` 无警告
4. `parser.go` 中无 handler 实现（仅编排）
5. 新增 handler 无需修改 `parser.go` 核心代码
6. `TestIntegrationParserToDiff` 端到端测试通过
7. `make ci` 通过
