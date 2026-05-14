# Parser Handler 重构设计：SchemaMutation 列表模式

- 日期：2026-05-07
- 状态：待审查

## 目标

将 `internal/parser` 从 "硬编码 switch + 内联修改 schema" 模式，重构为 "插件式 Handler + 自应用变更描述" 模式。

**解决的问题：**
1. 每新增一个 DDL 类型需修改 `visitNode` 的 switch，违反开闭原则
2. Handler 直接修改 `p.schema`，无法独立测试
3. Parser 文件承载辅助函数 + 编排逻辑 + 业务逻辑，过大且边界不清

**预期收益：**
1. 新增 DDL 类型只需实现 **Handler + Mutation**（两处），无需修改 Parser 核心
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
                    │     Handler      │ ← 纯函数：AST → []SchemaMutation
                    └──────────────────┘
                              │
                              ▼
                    ┌──────────────────┐
                    │ MutationApplier  │ ← 遍历调用 mutation.Apply(schema)
                    └──────────────────┘
```

## 一、SchemaMutation 类型系统

Mutation 是纯数据描述，不含执行逻辑。

```go
package parser

// MutationKind identifies the type of schema mutation
type MutationKind string

const (
    MutKindCreateTable     MutationKind = "create_table"
    MutKindAddColumn       MutationKind = "add_column"
    MutKindCreateEnumType  MutationKind = "create_enum_type"
    MutKindAddEnumLabel    MutationKind = "add_enum_label"
    MutKindCreateIndex     MutationKind = "create_index"
    MutKindAlterColumnType MutationKind = "alter_column_type"
    MutKindDropColumn      MutationKind = "drop_column"
    MutKindAddConstraint   MutationKind = "add_constraint"
    // ... extensible
)

// SchemaMutation is a self-describing and self-applying schema change.
type SchemaMutation interface {
    Kind() MutationKind
    Target() model.ObjectKey
    // Apply executes this mutation on the given schema.
    // Returns error if preconditions fail (e.g., target table missing).
    Apply(schema *model.Schema) error
}
```

**核心 Mutation 类型：**

| Struct | 字段 | 描述 |
|---|---|---|
| `CreateTableMutation` | Schema, Name, []Column | 创建表 |
| `AddColumnMutation` | Schema, Table, Column | 向已有表添加列 |
| `CreateEnumTypeMutation` | Schema, Name, []string(labels) | 创建枚举类型 |
| `AddEnumLabelMutation` | Schema, Type, Label | 向枚举添加值 |
| `CreateIndexMutation` | Schema, Index | 创建索引 |
| `AddConstraintMutation` | Schema, Table, Constraint | 添加约束 |

命名规范：**Kind 常量** 以 `MutKind` 为前缀，**struct** 以 `Mutation` 为后缀。所有 mutation 为**值类型**（非指针），`Target()` 返回 `model.ObjectKey` 用于错误定位，`Apply()` 实现具体逻辑。

## 二、StmtHandler 接口与注册表

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

// HandlerRegistry routes AST nodes to their handlers using O(1) type lookup.
type HandlerRegistry struct {
    handlers map[reflect.Type]Handler
}

func NewHandlerRegistry() *HandlerRegistry {
    return &HandlerRegistry{handlers: make(map[reflect.Type]Handler)}
}

// Register associates a handler with a specific AST node type (e.g., pg_nodes.CreateStmt).
func (r *HandlerRegistry) Register(nodeType pg_nodes.Node, h Handler) {
    r.handlers[reflect.TypeOf(nodeType)] = h
}

func (r *HandlerRegistry) Dispatch(node pg_nodes.Node) (Handler, bool) {
    h, ok := r.handlers[reflect.TypeOf(node)]
    return h, ok
}

// DefaultRegistry returns a registry with all built-in handlers registered.
func DefaultRegistry() *HandlerRegistry {
    r := NewHandlerRegistry()
    r.Register(pg_nodes.CreateStmt{}, &CreateTableHandler{})
    r.Register(pg_nodes.AlterTableStmt{}, &AlterTableHandler{})
    r.Register(pg_nodes.IndexStmt{}, &CreateIndexHandler{})
    r.Register(pg_nodes.CreateEnumStmt{}, &CreateEnumHandler{})
    return r
}
```

## 三、MutationApplier

```go
package parser

type MutationApplier struct {
    errors []error
}

// Apply executes each mutation in order by delegating to mutation.Apply(schema).
// Mutations are applied even if some fail; errors are collected and returned as aggregate.
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
```

**一致性校验（由每个 mutation 的 Apply 方法自行实现）：**
- `AddColumnMutation.Apply`：目标表不存在时，**自动创建空表**（适配 `ALTER TABLE` 在 `CREATE TABLE` 之前的 SQL 顺序）。此行为写入 `AddColumnMutation.Apply` 的代码注释，**非隐式**。
- 重复创建表返回 error 但不中断后续 mutation（Applier 收集错误继续）
- `MutationError` 包装 mutation 上下文

## 四、Handler 实现示例

```go
// internal/parser/create_table_handler.go
package parser

type CreateTableHandler struct{}

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
        CreateTableMutation{Schema: schemaName, Name: tableName, Columns: columns},
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
- `parserutil` 为无状态工具包（包含 `ParseRelation`, `ParseColumnDef`, `ParseTypeName`, `ParseExpression`, `typeNameMapping` 等）
- `internal/parser/` 内每个 handler 独立文件（`create_table_handler.go`, `alter_table_handler.go` 等）
- `parser.go` 仅保留编排逻辑（ParseSQL → registry.Dispatch → handler.Handle → applier.Apply）

**包结构说明：** Handler 与 Parser 处于**同一包** `internal/parser`，避免 `parser → handlers → parser` 的循环依赖。Go 同包内文件数量较多时仍优于跨包循环导入。

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

// NewParser creates a Parser with default handlers and applier.
// Zero-arg constructor for backward compatibility.
func NewParser() *Parser {
    return &Parser{
        registry: DefaultRegistry(),
        applier:  &MutationApplier{},
    }
}

// NewParserWith creates a Parser with custom registry/applier. Primarily for testing.
func NewParserWith(registry *HandlerRegistry, applier *MutationApplier) *Parser {
    return &Parser{
        registry: registry,
        applier:  applier,
    }
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
   - `internal/parser/mutation.go` — SchemaMutation 接口 + 各 Mutation struct
   - `internal/parser/registry.go` — HandlerRegistry（reflect.Type 映射）
   - `internal/parser/applier.go` — MutationApplier（仅循环调用 mut.Apply）
   - `internal/parser/parserutil/util.go` — 辅助函数提取（含 `typeNameMapping`）
   - `internal/parser/create_table_handler.go` — CreateTableHandler
   - `internal/parser/alter_table_handler.go` — AlterTableHandler
   - `internal/parser/index_handler.go` — CreateIndexHandler（框架）
   - `internal/parser/enum_handler.go` — CreateEnumHandler（框架）

2. **修改：**
   - `internal/parser/parser.go` — 移除 handler 和辅助函数，保留编排；新增 `NewParser()` 和 `NewParserWith()`
   - `internal/parser/parser_test.go` — 更新测试适配新结构；handler 单元测试用 SQL→Parse→取第一个节点构造输入
   - 所有用到 `parser.go` 辅助函数的外部文件（如有）

**本次重构不涉及：**
- diff/render 模块
- introspect 模块
- model 类型本身（Schema/Table/Column 等不变）

## 新增 DDL 类型的流程（开闭原则验证）

以新增 `CREATE SCHEMA` 支持为例：

1. 实现 `CreateSchemaHandler`（实现 `Handler` 接口）
2. 新增 `CreateSchemaMutation` 类型，实现 `SchemaMutation` 接口（含 `Apply` 方法）
3. 在 `DefaultRegistry()` 中 `r.Register(pg_nodes.CreateSchemaStmt{}, &CreateSchemaHandler{})`

**无需修改 `parser.go`、`applier.go`、或任何已有文件的核心逻辑。**

对比原设计 vs 修复后：

| 步骤 | 原设计（有缺陷） | 修复后 |
|---|---|---|
| Handler | ✅ 新增文件 | ✅ 新增文件 |
| Mutation | ✅ 新增类型 | ✅ 新增类型 |
| Applier | ❌ 修改 `applyOne` switch | ✅ 无需修改（mutation 自带 Apply） |
| Registry | ✅ 新增一行注册 | ✅ 新增一行注册 |

## 测试策略

1. **Handler 单元测试**：每个 handler 独立测试（AST → mutation 断言）

```go
// 推荐的 handler 测试模式：SQL → Parse → 取 RawStmt.Stmt
func mustParseFirstStmt(t *testing.T, sql string) pg_nodes.Node {
    tree, err := pg.Parse(sql)
    require.NoError(t, err)
    require.Len(t, tree.Statements, 1)
    raw := tree.Statements[0].(pg_nodes.RawStmt)
    return raw.Stmt
}

func TestCreateTableHandler(t *testing.T) {
    node := mustParseFirstStmt(t, "CREATE TABLE users (id integer NOT NULL)")
    h := &CreateTableHandler{}
    mutations, err := h.Handle(node)
    require.NoError(t, err)
    require.Len(t, mutations, 1)

    mut := mutations[0].(CreateTableMutation)
    assert.Equal(t, "users", mut.Name)
    assert.Len(t, mut.Columns, 1)
    assert.Equal(t, "id", mut.Columns[0].Name)
}
```

2. **Applier 单元测试**：给定 schema + mutation 列表 → 断言 schema 状态
3. **集成测试**：`parser_test.go` 中的端到端测试保持不变，确保行为兼容
4. **回归测试**：运行现有 `TestIntegrationParserToDiff`

## 分阶段实施计划

| 阶段 | 内容 | 风险 | 验证命令 |
|------|------|------|---------|
| 1 | 提取 `parserutil` 包（纯机械性移动辅助函数） | 低 | `go build ./... && go test ./internal/parser/...` |
| 2 | 定义 `SchemaMutation` 接口 + `MutationApplier`（无 Apply 实现） | 低 | `go build ./...` |
| 3 | 实现 `CreateTableMutation` 的 `Apply` + `CreateTableHandler`，替换 `handleCreateTable` | 中 | `go test ./internal/parser/...` |
| 4 | 实现 `AddColumnMutation` 的 `Apply` + `AlterTableHandler`，替换 `handleAlterTable` | 中 | `go test ./internal/parser/...` |
| 5 | 引入 `HandlerRegistry`（reflect.Type 映射），重构 `visitNode`，`NewParser()` 适配 | 中 | `go test ./...` |
| 6 | 清理旧 `handleCreateTable`、`handleAlterTable`、辅助函数等死代码 | 低 | `go test ./...` |
| 7 | 新增 Enum/Index handler（扩展验证架构） | 低 | `go test ./...` |

每个阶段结束后运行 `go test ./...` 确保不回归。

## 验收标准

1. `go build ./...` 编译通过
2. `go test ./...` 全量测试通过
3. `go vet ./...` 无警告
4. `parser.go` 中无 handler 实现（仅编排）
5. 新增 handler 无需修改 `parser.go` 核心代码
6. `TestIntegrationParserToDiff` 端到端测试通过
7. `make ci` 通过
