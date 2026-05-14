# CreateIndexHandler 设计文档与代码实现比对评审报告

**评审日期：** 2026-05-08  
**评审人：** AI Code Reviewer  
**设计文档：** `docs/superpowers/specs/2026-05-08-index-handler-implementation-design.md`  
**评审范围：** 实现一致性、技术合理性、功能完整性、可维护性与鲁棒性  

---

## 评审总结

**关键发现：设计文档已批准（状态：已批准），但代码实现几乎完全缺失。**

当前 `CreateIndexHandler` 仅有一个占位符实现，返回 "not yet supported" 错误。`model.Index` 结构未扩展，`IndexElem` 结构不存在，`CreateIndexMutation` 未实现，相关测试缺失。

**建议：需要按照设计文档完成实现，或明确标注设计文档状态为"待实现"。**

---

## 1. 实现一致性评审

### 1.1 数据模型设计

#### 1.1.1 新增 `model.IndexElem` 结构

**设计文档要点：**
```go
type IndexElem struct {
    Name          string     // column name (for simple column index)
    Expr          string     // expression (for expression index)
    IndexColName  string     // name for index column
    Collation     string     // collation name
    Opclass       string     // operator class
    Ordering      string     // ASC/DESC/default
    NullsOrdering string    // FIRST/LAST/default
}
```

**对应代码实现：**
- 文件 `internal/model/index.go` **不存在**
- `internal/model/` 目录中无 `IndexElem` 结构定义

**评审结论：** ❌ **缺失**

**具体修改建议：**
创建 `internal/model/index.go` 文件，实现 `IndexElem` 结构：
```go
package model

// IndexElem represents an element in an index definition
type IndexElem struct {
    Name          string
    Expr          string
    IndexColName  string
    Collation     string
    Opclass       string
    Ordering      string
    NullsOrdering string
}
```

---

#### 1.1.2 扩展 `model.Index` 结构

**设计文档要点：**
```go
type Index struct {
    Name          string
    Table         string
    Elements      []IndexElem  // replaces Columns []string
    Unique        bool
    Method        string       // btree, hash, gin, etc.
    Primary       bool         // is primary key index
    IsConstraint  bool        // is it for a pkey/unique constraint
    WhereClause   string       // partial index predicate
    Concurrent    bool         // concurrent index build
    IfNotExists   bool         // IF NOT EXISTS
}
```

**对应代码实现：**
文件：`internal/model/table.go` (第 21-28 行)
```go
// Index represents a database index
type Index struct {
    Name    string
    Table   string
    Columns []string    // 设计文档要求用 Elements []IndexElem 替换
    Unique  bool
    Method  string
}
```

**评审结论：** ❌ **不一致**

**具体修改建议：**
按照设计文档扩展 `Index` 结构，同时保留向后兼容性：
```go
type Index struct {
    Name          string
    Table         string
    Columns       []string      // 保留以兼容现有代码，标记为 deprecated
    Elements      []IndexElem   // 新增：替代 Columns
    Unique        bool
    Method        string
    Primary       bool
    IsConstraint  bool
    WhereClause   string
    Concurrent    bool
    IfNotExists   bool
}
```

---

### 1.2 Handler 和 Mutation 实现

#### 1.2.1 `CreateIndexHandler.Handle` 实现逻辑

**设计文档要点：**
1. 类型断言检查 `node.(pg_nodes.IndexStmt)`
2. 使用 `parserutil.ParseRangeVar` 解析表名和模式名
3. 遍历 `IndexStmt.IndexParams` 解析 IndexElem
4. 解析 `WhereClause` 使用 `pg_query.Deparse()` 转为字符串
5. 返回 `CreateIndexMutation` 列表

**对应代码实现：**
文件：`internal/parser/index_handler.go` (完整内容)
```go
func (h *CreateIndexHandler) Handle(node pg_nodes.Node) ([]SchemaMutation, error) {
    _, ok := node.(pg_nodes.IndexStmt)
    if !ok {
        return nil, fmt.Errorf("CreateIndexHandler: expected pg_nodes.IndexStmt, got %T", node)
    }
    return nil, fmt.Errorf("CREATE INDEX is not yet supported (MVP scope only includes CREATE TABLE and ALTER TABLE ADD COLUMN)")
}
```

**评审结论：** ❌ **缺失（仅占位符）**

**具体修改建议：**
按照设计文档实现完整的 `Handle` 方法：
```go
func (h *CreateIndexHandler) Handle(node pg_nodes.Node) ([]SchemaMutation, error) {
    stmt, ok := node.(pg_nodes.IndexStmt)
    if !ok {
        return nil, fmt.Errorf("CreateIndexHandler: expected pg_nodes.IndexStmt, got %T", node)
    }
    
    // 1. 解析表名
    schema, table := parserutil.ParseRelation(stmt.Relation)
    
    // 2. 解析索引名（为空时生成默认名）
    indexName := ""
    if stmt.Idxname != nil {
        indexName = *stmt.Idxname
    }
    if indexName == "" {
        // 生成默认索引名
    }
    
    // 3. 遍历 IndexParams 解析 IndexElem
    elements := make([]model.IndexElem, 0)
    // ... 解析逻辑
    
    // 4. 解析 WhereClause
    whereClause := ""
    if stmt.WhereClause != nil {
        // 使用 pg_query.Deparse()
    }
    
    // 5. 构建 Index 和 Mutation
    index := model.Index{
        Name:        indexName,
        Table:       table,
        Elements:    elements,
        Unique:      stmt.Unique,
        Method:      parseAccessMethod(stmt.AccessMethod),
        Primary:     stmt.Primary,
        Concurrent:  stmt.Concurrent,
        IfNotExists: stmt.IfNotExists,
    }
    
    return []SchemaMutation{CreateIndexMutation{
        Schema: schema,
        Index:  index,
    }}, nil
}
```

---

#### 1.2.2 `CreateIndexMutation` 实现

**设计文档要点：**
```go
type CreateIndexMutation struct {
    Schema string
    Index  model.Index
}

// Kind() → MutKindCreateIndex
// Target() → 返回索引的 ObjectKey
// Apply():
//   - 如果 IfNotExists=true 且索引已存在，跳过
//   - 如果 Primary=true，同时设置 table.PrimaryKey
//   - 将索引添加到 table.Indexes map
```

**对应代码实现：**
文件：`internal/parser/mutation.go`
- 第 16 行定义了 `MutKindCreateIndex MutationKind = "create_index"`
- **但 `CreateIndexMutation` 结构未实现**

**评审结论：** ❌ **缺失**

**具体修改建议：**
在 `mutation.go` 中添加：
```go
// CreateIndexMutation describes creating an index
type CreateIndexMutation struct {
    Schema string
    Index  model.Index
}

func (m CreateIndexMutation) Kind() MutationKind { return MutKindCreateIndex }
func (m CreateIndexMutation) Target() model.ObjectKey {
    return model.NewObjectKey(m.Schema, m.Index.Name, model.KindIndex)
}
func (m CreateIndexMutation) Apply(schema *model.Schema) error {
    ns := schema.GetOrCreateNamespace(m.Schema)
    table, exists := ns.Tables[m.Index.Table]
    if !exists {
        return fmt.Errorf("table %s.%s does not exist for index %s", m.Schema, m.Index.Table, m.Index.Name)
    }
    
    // IfNotExists 检查
    if m.Index.IfNotExists {
        if _, exists := table.Indexes[m.Index.Name]; exists {
            return nil // 索引已存在，跳过
        }
    }
    
    // Primary key 设置
    if m.Index.Primary {
        table.PrimaryKey = &model.PrimaryKey{
            Name:    m.Index.Name,
            Columns: extractColumnNames(m.Index.Elements),
        }
    }
    
    table.Indexes[m.Index.Name] = &m.Index
    return nil
}
```

---

### 1.3 `parserutil` 辅助函数

**设计文档要点：**
- 添加索引解析辅助函数

**对应代码实现：**
文件：`internal/parser/parserutil/util.go`
- 现有函数：`ParseRelation`、`ParseColumnDef`、`ParseTypeName`、`ParseExpression`
- **无索引解析相关函数**

**评审结论：** ❌ **缺失**

**具体修改建议：**
添加以下辅助函数：
```go
// ParseIndexStmt 解析 IndexStmt 为 model.Index
func ParseIndexStmt(stmt pg_nodes.IndexStmt) (model.Index, error) { ... }

// ParseIndexElem 解析单个 IndexElem 节点
func ParseIndexElem(elt pg_nodes.Node) (model.IndexElem, error) { ... }

// DeparseNode 将 AST 节点转为 SQL 字符串
func DeparseNode(node pg_nodes.Node) (string, error) { ... }
```

---

## 2. 技术合理性评估

### 2.1 当前占位符实现的合理性

**评审结论：** ✅ **合理（作为 MVP 阶段临时方案）**

**分析：**
- 当前 MVP 范围仅包含 CREATE TABLE 和 ALTER TABLE ADD COLUMN
- 占位符实现 + 清晰的错误提示是合理的渐进式开发策略
- `MutKindCreateIndex` 常量已定义，为后续实现预留了接口

**建议：**
- 在设计文档或代码注释中明确标注"待实现"状态和预计完成时间
- 考虑在 `HandlerRegistry` 中对未实现的处理器添加特殊标记

---

### 2.2 设计文档中的技术选型评估

| 评估维度 | 设计文档方案 | 评估 | 建议 |
|---------|-------------|------|------|
| **使用 `pg_query.Deparse()`** | 将表达式节点转为字符串 | ✅ 合理，避免重复实现 SQL 生成逻辑 | 需确认 `pg_query_go` 库是否支持 Deparse 功能 |
| **IndexElem 使用字符串存储表达式** | `Expr string` | ⚠️ 可接受但丢失结构信息 | 如果后续需要表达式级别的差异比较，考虑保留 AST |
| **Elements 替换 Columns** | `[]IndexElem` 替代 `[]string` | ✅ 更灵活，支持表达式索引 | 需注意向后兼容性 |
| **使用 reflect.Type 作为 registry key** | `map[reflect.Type]Handler` | ⚠️ 有一定性能开销 | 当前阶段可接受，见之前审查建议 |

---

## 3. 功能完整性检查

### 3.1 设计文档功能清单 vs 代码实现

| 功能点 | 设计文档要求 | 代码实现状态 | 评审结论 |
|--------|-------------|-------------|---------|
| `model.IndexElem` 结构 | 7 个字段完整定义 | ❌ 未实现 | **缺失** |
| `model.Index` 扩展 | 10 个字段（含 Elements） | ❌ 仅 5 个基础字段 | **不一致** |
| `CreateIndexHandler.Handle` | 完整解析逻辑 | ❌ 仅占位符 | **缺失** |
| `CreateIndexMutation` | 完整实现 + Apply | ❌ 未实现 | **缺失** |
| `parserutil` 辅助函数 | 索引解析函数 | ❌ 未实现 | **缺失** |
| 单元测试（10 个） | 见设计文档 4.1 | ❌ 0 个 | **缺失** |
| 集成测试（2 个） | 见设计文档 4.2 | ❌ 0 个 | **缺失** |

---

### 3.2 边界条件处理检查

**设计文档要求的边界情况：**

| 边界情况 | 设计要求 | 实现状态 | 评审结论 |
|---------|---------|---------|---------|
| `Idxname` 为空时生成默认索引名 | `{table}_{col}_idx` 或 `{table}_expr_idx` | ❌ 未实现 | **缺失** |
| `AccessMethod` 为空时使用默认方法（btree） | 默认 btree | ❌ 未实现 | **缺失** |
| 表达式索引中 `Name` 为空，`Expr` 不为空 | 正确处理 | ❌ 未实现 | **缺失** |
| 主键索引同时设置 `table.PrimaryKey` | Primary=true 时设置 | ❌ 未实现 | **缺失** |
| `IfNotExists` 重复索引跳过 | 检查并跳过 | ❌ 未实现 | **缺失** |

---

### 3.3 错误处理机制检查

**设计文档要求的错误处理：**

| 错误场景 | 设计要求 | 实现状态 | 评审结论 |
|---------|---------|---------|---------|
| 类型安全 | `value, ok := node.(pg_nodes.IndexStmt)` | ✅ 已实现 | **通过** |
| 表和列检查 | 索引引用的表必须存在 | ❌ 未实现 | **缺失** |
| 重复索引检查 | `IfNotExists` 时跳过 | ❌ 未实现 | **缺失** |
| 表达式解析 | 使用 `pg_query.Deparse()` | ❌ 未实现 | **缺失** |
| Panic 恢复 | 依赖 `visitNode` 中的 defer/recover | ✅ 已存在 | **通过** |

---

## 4. 可维护性与鲁棒性分析

### 4.1 当前占位符实现的鲁棒性

**评审结论：** ✅ **可接受**

**分析：**
- 占位符实现返回清晰的错误信息，便于调试
- 错误信息明确说明了 MVP 范围限制

```go
return nil, fmt.Errorf("CREATE INDEX is not yet supported (MVP scope only includes CREATE TABLE and ALTER TABLE ADD COLUMN)")
```

---

### 4.2 设计文档实现后的可维护性评估

**潜在问题：**

1. **[建议修改] `pg_query.Deparse()` 依赖**
   - 问题：设计文档假设 `pg_query_go` 库支持 `Deparse()` 功能
   - 建议：先验证库是否支持此功能，或准备备选方案（如手动拼接 SQL）

2. **[建议修改] `IndexElem` 表达式存储格式**
   - 问题：使用 `string` 存储表达式可能丢失类型信息
   - 建议：如果后续需要表达式级别的操作，考虑保留结构化表示

3. **[仅供参考] 测试覆盖**
   - 设计文档列出了 10 个单元测试 + 2 个集成测试
   - 建议按照测试计划实现，确保覆盖率

---

### 4.3 扩展性分析

**评审结论：** ✅ **设计良好**

**分析：**
- 使用 `HandlerRegistry` + `SchemaMutation` 接口的设计易于扩展
- 新增索引类型只需实现对应的 Handler 和 Mutation
- `MutKindCreateIndex` 已定义，接口已预留

---

## 5. 评审清单确认

- [x] 设计文档与代码实现的比对已完成
- [x] 每个不一致点都标注了具体位置
- [x] 缺失的功能点已明确列出
- [x] 给出了具体的修改建议和代码示例
- [x] 技术合理性已评估
- [x] 边界条件和错误处理已检查

---

## 6. 总结与建议

### 6.1 核心问题

**设计文档与代码实现严重脱节：**
- 设计文档状态为"已批准"，但代码实现几乎完全缺失
- 仅有占位符实现和常量定义

### 6.2 建议行动

**短期（立即执行）：**
1. 更新设计文档状态为"实现中"或"待实现"
2. 或者在代码中添加 `// TODO: 实现设计文档 docs/superpowers/specs/2026-05-08-index-handler-implementation-design.md` 注释

**中期（按照设计文档实现）：**
1. 创建 `internal/model/index.go`，实现 `IndexElem` 和扩展 `Index`
2. 实现 `CreateIndexMutation` 和相关测试
3. 实现 `CreateIndexHandler.Handle()` 完整逻辑
4. 添加 `parserutil` 辅助函数
5. 编写单元测试和集成测试

**长期（优化）：**
1. 考虑 `reflect.Type` 作为 registry key 的性能优化
2. 完善错误处理和边界条件覆盖

---

## 7. 详细偏差对照表

| 设计文档章节 | 要求 | 代码文件 | 实际状态 | 偏差原因 |
|------------|------|---------|---------|---------|
| 1. 数据模型设计 | `IndexElem` 结构 | 不存在 | ❌ 缺失 | 未开始实现 |
| 1. 数据模型设计 | 扩展 `Index` 结构 | `table.go` | ❌ 不一致 | 未开始实现 |
| 2. Handler 实现 | `Handle()` 完整逻辑 | `index_handler.go` | ❌ 缺失 | MVP 范围限制 |
| 2. Mutation 实现 | `CreateIndexMutation` | `mutation.go` | ❌ 缺失 | 未开始实现 |
| 3. 数据流 | 完整数据流 | 多个文件 | ❌ 缺失 | 未开始实现 |
| 4. 测试计划 | 12 个测试 | `*_test.go` | ❌ 缺失 | 未开始实现 |
| 5. 实现步骤 | 7 个步骤 | 多个文件 | ❌ 缺失 | 未开始实现 |

---

**评审完成时间：** 2026-05-08  
**下一步：** 请确认是否开始按照设计文档实现功能，或更新设计文档状态。
