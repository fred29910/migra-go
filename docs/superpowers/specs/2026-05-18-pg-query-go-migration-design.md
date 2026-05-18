# pg_query_go 依赖迁移设计

## 背景

当前项目使用 `github.com/lfittl/pg_query_go` v1.0.2 作为 PostgreSQL SQL 解析器。该库自 2018 年起停止维护，且存在以下问题：

1. **Windows 交叉编译困难**：依赖 CGo + MinGW，构建容器需要手动安装完整的 MinGW-w64 工具链
2. **不支持新版 PostgreSQL 语法**：解析器基于 PostgreSQL 10 时代的代码
3. **无人维护**：上游仓库不再接受 bug 修复和功能更新

目标替换为活跃维护的分支 `github.com/pganalyze/pg_query_go` v6.2.2。

## 方案选择

| 方案 | 描述 | 优点 | 缺点 |
|------|------|------|------|
| A（推荐） | 完整迁移到 `pganalyze/pg_query_go/v6` | 获得活跃维护的依赖，支持 PostgreSQL 17 语法 | 改动量大，需重写所有 AST 访问代码 |
| B | 替换为 `pganalyze` v2.x（API 兼容） | 最小改动 | 使用较旧的解析器版本 |
| C | 完整迁移 + 引入适配层 | handler 代码基本不变 | 增加中间层复杂度 |

选择方案 A，原因：项目处于活跃开发期，现在迁移成本最低；v6 是当前最新稳定版，长期维护性好。

## API 差异分析

### 解析入口

```go
// 当前（lfittl v1.0.2）
import pg "github.com/lfittl/pg_query_go"
tree, err := pg.Parse(sql)
// tree.Statements 是 []interface{}，每个元素是 pg_nodes.RawStmt

// 目标（pganalyze v6.2.2）
import pg_query "github.com/pganalyze/pg_query_go/v6"
tree, err := pg_query.Parse(sql)
// tree.Stmts 是 []*pg_query.RawStmt
```

### 节点访问方式

```go
// 当前：类型断言
rawStmt := stmt.(pg_nodes.RawStmt)
createStmt := node.(pg_nodes.CreateStmt)

// 目标：getter 方法（protobuf oneof）
rawStmt := stmt.GetRawStmt()
createStmt := node.GetCreateStmt()
```

### 列表访问

```go
// 当前：通过 .Items 字段访问
for _, item := range stmt.TableElts.Items { ... }
for _, item := range typeName.Names.Items { ... }
for _, item := range elt.Constraints.Items { ... }

// 目标：直接遍历切片
for _, item := range createStmt.TableElts { ... }
for _, item := range typeName.Names { ... }
for _, item := range colDef.Constraints { ... }
```

### 枚举常量映射

| 当前（lfittl） | 目标（pganalyze） |
|----------------|-------------------|
| `pg_nodes.CONSTR_PRIMARY` | `pg_query.ConstrType_CONSTR_PRIMARY` |
| `pg_nodes.CONSTR_NOTNULL` | `pg_query.ConstrType_CONSTR_NOTNULL` |
| `pg_nodes.CONSTR_DEFAULT` | `pg_query.ConstrType_CONSTR_DEFAULT` |
| `pg_nodes.CONSTR_FOREIGN` | `pg_query.ConstrType_CONSTR_FOREIGN` |
| `pg_nodes.AT_AddColumn` | `pg_query.AlterTableType_AT_AddColumn` |
| `pg_nodes.AT_DropColumn` | `pg_query.AlterTableType_AT_DropColumn` |
| `pg_nodes.AT_AlterColumnType` | `pg_query.AlterTableType_AT_AlterColumnType` |
| `pg_nodes.AT_SetNotNull` | `pg_query.AlterTableType_AT_SetNotNull` |
| `pg_nodes.AT_DropNotNull` | `pg_query.AlterTableType_AT_DropNotNull` |
| `pg_nodes.AT_ColumnDefault` | `pg_query.AlterTableType_AT_ColumnDefault` |
| `pg_nodes.SORTBY_DEFAULT` | `pg_query.SortByDir_SORTBY_DEFAULT` |
| `pg_nodes.SORTBY_ASC` | `pg_query.SortByDir_SORTBY_ASC` |
| `pg_nodes.SORTBY_DESC` | `pg_query.SortByDir_SORTBY_DESC` |
| `pg_nodes.SORTBY_NULLS_DEFAULT` | `pg_query.SortByNulls_SORTBY_NULLS_DEFAULT` |
| `pg_nodes.SORTBY_NULLS_FIRST` | `pg_query.SortByNulls_SORTBY_NULLS_FIRST` |
| `pg_nodes.SORTBY_NULLS_LAST` | `pg_query.SortByNulls_SORTBY_NULLS_LAST` |

### 字面量节点

```go
// 当前 → 目标
pg_nodes.String{Str: "x"}    → pg_query.String_{S: "x"}
pg_nodes.Integer{Ival: 42}   → pg_query.Integer{Ival: 42}
pg_nodes.Float{Str: "3.14"}  → pg_query.Float{Str: "3.14"}
```

### 字段可空性变化

| 字段 | 当前（lfittl） | 目标（pganalyze） |
|------|----------------|-------------------|
| `ColumnDef.Colname` | `*string` | `string` |
| `Constraint.Conname` | `*string` | `string` |
| `CreateSchemaStmt.Schemaname` | `*string` | `string` |
| `IndexStmt.Idxname` | `*string` | `string` |
| `AlterTableCmd.Name` | `*string` | `string` |
| `RangeVar.Schemaname` | `*string` | `string` |
| `RangeVar.Relname` | `*string` | `string` |
| `IndexElem.Name` | `*string` | `string` |
| `IndexElem.Indexcolname` | `*string` | `string` |

### Deparse 方式

```go
// 当前：节点方法（可能 panic）
result := node.Deparse()

// 目标：包级函数（返回 error）
output, err := pg_query.Deparse(tree)
```

## 文件改动范围

| 文件 | 改动内容 | 复杂度 |
|------|---------|--------|
| `go.mod` / `go.sum` | 替换依赖 | 低 |
| `internal/parser/parser.go` | 解析入口、RawStmt 访问、Deparse 调用 | 中 |
| `internal/parser/registry.go` | 枚举常量引用 | 低 |
| `internal/parser/parserutil/util.go` | 所有节点访问函数 | 高 |
| `internal/parser/create_table_handler.go` | 节点访问、枚举、字段可空性 | 中 |
| `internal/parser/alter_table_handler.go` | 节点访问、枚举、字段可空性 | 中 |
| `internal/parser/enum_handler.go` | 节点访问、字段可空性 | 中 |
| `internal/parser/index_handler.go` | 节点访问、枚举、字段可空性 | 中 |
| `internal/parser/create_schema_handler.go` | 节点访问、字段可空性 | 低 |
| `internal/parser/handler_test.go` | `mustParseFirstStmt` 辅助函数 | 中 |
| `internal/parser/*_test.go` | 枚举引用更新 | 低 |

## 实现策略

### 1. 依赖替换

```go
// go.mod
- github.com/lfittl/pg_query_go v1.0.2
+ github.com/pganalyze/pg_query_go/v6 v6.2.2
```

### 2. 解析入口（parser.go）

```go
import pg_query "github.com/pganalyze/pg_query_go/v6"

func (p *Parser) ParseSQL(sql string) (*model.Schema, error) {
    tree, err := pg_query.Parse(sql)
    if err != nil {
        return nil, fmt.Errorf("pg_query parse failed: %w", err)
    }

    for _, rawStmt := range tree.Stmts {
        stmt := rawStmt.GetRawStmt()
        if err := p.VisitNode(stmt, rawStmt.StmtLocation); err != nil {
            p.errors = append(p.errors, err)
        }
    }
    // ...
}
```

### 3. 节点访问工具函数（parserutil/util.go）

核心变化：所有从 `pg_nodes.Node` 类型断言改为 `node.GetXxx()` getter 调用。

```go
// ParseRelation 示例
func ParseRelation(relation *pg_query.RangeVar) (tableName, schemaName string) {
    if relation == nil {
        return "", "public"
    }
    tableName = relation.Relname      // 不再是 *string
    schemaName = relation.Schemaname  // 不再是 *string
    return
}

// ParseColumnDef 示例
func ParseColumnDef(node *pg_query.Node) *model.Column {
    colDef := node.GetColumnDef()
    col := &model.Column{IsNotNull: colDef.IsNotNull}
    col.Name = colDef.Colname  // 直接 string，不再需要 nil 检查
    // ...
}
```

### 4. Handler 改动模式

每个 handler 的 `Handle` 方法需要：

1. 将 `node.(pg_nodes.Xxx)` 改为 `node.GetXxx()`
2. 将枚举常量替换为 `pg_query.Xxx_YYY` 格式
3. 将 `.Items` 访问改为直接切片遍历
4. 将 `*string` 指针检查改为直接字符串空值检查

## 风险与缓解

| 风险 | 影响 | 缓解措施 |
|------|------|---------|
| 字段可空性变化导致 nil panic | 高 | 全面检查所有 `*string` 字段访问，改为空字符串检查 |
| 枚举值不匹配 | 中 | 编译期即可发现，逐个替换 |
| 解析行为差异 | 中 | 运行完整测试套件，对比解析结果 |
| Deparse 行为变化 | 低 | 更新测试期望值 |

## 测试策略

1. 运行现有全部单元测试（`go test ./internal/parser/...`）
2. 运行集成测试（`go test ./cmd/migra/...`）
3. 如有解析结果差异，更新测试期望值
4. 验证 Windows 交叉编译（`GOOS=windows GOARCH=amd64 CGO_ENABLED=1 CC=x86_64-w64-mingw32-gcc go build ./cmd/migra`）

## 兼容性说明

此变更仅影响内部 `parser` 包，不改变任何公开 API。`cmd/migra` 的 CLI 接口和行为保持不变。
