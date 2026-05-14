# CreateIndexHandler 完整实现设计

**日期：** 2026-05-08  
**状态：** 已批准  
**范围：** 完整实现 index_handler.go，支持所有 IndexStmt 和 IndexElem 选项

---

## 1. 数据模型设计

### 新增 model.IndexElem 结构

```go
package model

// IndexElem represents an element in an index definition
type IndexElem struct {
    Name          string     // column name (for simple column index)
    Expr          string     // expression (for expression index, e.g., "(col1 + col2)")
    IndexColName  string     // name for index column, empty = default
    Collation     string     // collation name, empty = default
    Opclass       string     // operator class, empty = default
    Ordering      string     // ASC/DESC/default
    NullsOrdering string    // FIRST/LAST/default
}
```

### 扩展 model.Index 结构

```go
type Index struct {
    Name          string
    Table         string
    Elements      []IndexElem  // replaces Columns []string
    Unique        bool
    Method        string       // btree, hash, gin, etc.
    Primary       bool         // is primary key index
    IsConstraint  bool        // is it for a pkey/unique constraint
    WhereClause   string       // partial index predicate (WHERE clause)
    Concurrent    bool         // concurrent index build
    IfNotExists   bool         // IF NOT EXISTS
}
```

### 向后兼容性

- `model.Table` 的 `Indexes map[string]*Index` 保持不变
- 移除 `Columns []string` 字段，改用 `Elements []IndexElem`

---

## 2. Handler 和 Mutation 实现

### CreateIndexHandler.Handle 实现逻辑

1. 类型断言检查 `node.(pg_nodes.IndexStmt)`
2. 使用 `parserutil.ParseRangeVar` 解析表名和模式名
3. 遍历 `IndexStmt.IndexParams`（List of IndexElem）：
   - 对每个 `IndexElem`，判断是列索引（`Name != nil`）还是表达式索引（`Expr != nil`）
   - 解析排序规则、操作符类、排序方向、NULL 排序等选项
4. 解析 `WhereClause`（部分索引的 WHERE 条件），使用 `pg_query.Deparse()` 转为字符串
5. 返回 `CreateIndexMutation` 列表

### CreateIndexMutation 设计

```go
type CreateIndexMutation struct {
    Schema string
    Index  model.Index
}
```

实现 `SchemaMutation` 接口：
- `Kind()` → `MutKindCreateIndex`
- `Target()` → 返回索引的 ObjectKey（KindIndex）
- `Apply(schema *model.Schema)`：
  - 如果 `IfNotExists=true` 且索引已存在，跳过
  - 如果 `Primary=true`，同时设置 `table.PrimaryKey`
  - 将索引添加到 `table.Indexes` map

---

## 3. 数据流和错误处理

### 数据流

```
SQL ──pg_query.Parse()──> AST (IndexStmt)
    │
    └──> CreateIndexHandler.Handle()
            │
            ├── 解析 IndexStmt → CreateIndexMutation
            │   ├── 解析表名 (Relation → ParseRangeVar)
            │   ├── 解析索引名、唯一性、方法等
            │   ├── 遍历 IndexParams → []model.IndexElem
            │   └── 解析 WhereClause → 部分索引条件
            │
            └──> 返回 []SchemaMutation
                    │
                    └──> MutationApplier.Apply()
                            │
                            └──> CreateIndexMutation.Apply()
                                    ├── 检查表是否存在（或创建 placeholder）
                                    ├── 检查 IfNotExists 和重复索引
                                    ├── 设置 table.PrimaryKey (if Primary)
                                    └── 添加索引到 table.Indexes
```

### 错误处理策略

1. **类型安全**：使用 `value, ok := node.(pg_nodes.IndexStmt)` 模式
2. **表和列检查**：索引引用的表必须存在（除非是 placeholder）
3. **重复索引**：检查 `table.Indexes[name]` 是否已存在，IfNotExists 时跳过
4. **表达式解析**：使用 `pg_query.Deparse()` 将表达式节点转为字符串
5. **Panic 恢复**：依赖 `visitNode` 中的 defer/recover 机制

### 边界情况处理

- `Idxname` 为空时生成默认索引名（格式：`{table}_{col}_idx` 或 `{table}_expr_idx`）
- `AccessMethod` 为空时使用默认方法（btree）
- 表达式索引中 `Name` 为空，`Expr` 不为空
- 主键索引（Primary=true）同时设置 `table.PrimaryKey`

---

## 4. 测试计划

### 单元测试

1. **TestCreateIndexHandler_Basic** - 基本 CREATE INDEX
2. **TestCreateIndexHandler_Unique** - CREATE UNIQUE INDEX
3. **TestCreateIndexHandler_WithMethod** - 指定访问方法（btree/hash/gin）
4. **TestCreateIndexHandler_ExpressionIndex** - 表达式索引
5. **TestCreateIndexHandler_PartialIndex** - 部分索引（WHERE 子句）
6. **TestCreateIndexHandler_MultiColumn** - 多列索引
7. **TestCreateIndexHandler_WithOrdering** - 指定排序方向（ASC/DESC）
8. **TestCreateIndexHandler_PrimaryKey** - 主键索引，验证 table.PrimaryKey 设置
9. **TestCreateIndexHandler_IfNotExists** - IF NOT EXISTS 重复索引跳过
10. **TestCreateIndexHandler_Complex** - 复杂组合（多列+表达式+排序+WHERE）

### 集成测试

1. **TestIntegration_IndexToDiff** - 索引创建到 diff 的完整流程
2. **TestIntegration_IndexPlaceholder** - ALTER TABLE 在 CREATE TABLE 之前（placeholder 场景）

---

## 5. 实现步骤概览

1. 扩展 `model.Index` 和新增 `model.IndexElem`
2. 添加 `MutKindCreateIndex` 常量
3. 实现 `CreateIndexMutation`
4. 实现 `CreateIndexHandler.Handle()` 完整逻辑
5. 更新 `parserutil` 添加索引解析辅助函数
6. 编写单元测试和集成测试
7. 更新文档和注释

---

## 规格自检

- [x] 无占位符或 TODO
- [x] 各章节内部一致
- [x] 范围聚焦，可用一个实现计划覆盖
- [x] 需求明确，无歧义
