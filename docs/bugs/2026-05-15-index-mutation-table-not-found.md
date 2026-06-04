# Bug: `migra diff` 跨文件索引创建时报告 "table does not exist"

| 字段 | 值 |
|------|-----|
| **日期** | 2026-05-15 |
| **严重程度** | 高（功能阻塞） |
| **状态** | 已修复 |

---

## 错误现象

```bash
./migra diff testdata/diff/v1/ testdata/diff/v2/
Error: failed to load source: failed to parse testdata/diff/v1/03_indexes.sql: \
  parsing completed with 2 errors, first: parse error at position 0: \
  applied 1 mutations with 1 errors, first: \
  mutation create_index on public/idx_posts_user_id: \
  table public.posts does not exist for index creation
```

---

## 根因分析

### 问题链路

1. **目录加载器按文件逐个解析**  
   [`internal/source/dir_loader.go`](../internal/source/dir_loader.go) 的 `Load` 方法通过 `filepath.WalkDir` 收集所有 `.sql` 文件，排序后**逐个文件**调用 `SQLFileLoader.Load()`。

2. **每个文件独立解析，Schema 不跨文件共享**  
   [`internal/source/sql_file_loader.go`](../internal/source/sql_file_loader.go) 的 `Load` 方法每次都会创建一个新的 `parser.NewParser()`，而 [`internal/parser/parser.go`](../internal/parser/parser.go) 的 `ParseSQL` 方法在每次调用时都会执行：
   ```go
   p.schema = model.NewSchema()  // ← 每次重置！
   ```
   这意味着**每个 SQL 文件都在一个全新的、空的 Schema 上独立解析**。

3. **索引文件中的表尚未创建**  
   `testdata/diff/v1/03_indexes.sql` 包含：
   ```sql
   CREATE INDEX idx_posts_user_id ON posts(user_id);
   CREATE UNIQUE INDEX idx_users_username ON users(username);
   ```
   当这个文件被独立解析时，Schema 中既没有 `posts` 表也没有 `users` 表（它们定义在 `01_users.sql` 和 `02_posts.sql` 中）。

4. **索引变更 Apply 时检查表是否存在**  
   [`internal/parser/index_mutation.go`](../internal/parser/index_mutation.go) 的 `Apply` 方法：
   ```go
   func (m CreateIndexMutation) Apply(schema *model.Schema) error {
       ns := schema.GetOrCreateNamespace(m.Schema)
       table, exists := ns.Tables[m.Index.Table]
       if !exists {
           return fmt.Errorf("table %s.%s does not exist for index creation", m.Schema, m.Index.Table)
       }
       // ...
   }
   ```
   由于当前 Schema 是空的，`ns.Tables["posts"]` 不存在，因此报错。

### 核心问题

**`ParseSQL` 每次调用都重置 Schema，导致跨文件的 DDL 依赖关系丢失。**

目录加载器虽然按顺序处理文件，但每个文件解析时使用的是独立的 Parser 实例和独立的 Schema 对象。前一个文件中创建的表不会传递到后一个文件的解析过程中。

---

## 数据流图解

```
testdata/diff/v1/
├── 01_users.sql   → Parser A → Schema A (有 users 表)  → 合并到 merged
├── 02_posts.sql   → Parser B → Schema B (有 posts 表)  → 合并到 merged
└── 03_indexes.sql → Parser C → Schema C (空!)          → ❌ 找不到 posts 表
```

每个 Parser 都有自己的 Schema，互不影响。

---

## 解决方案

### 方案 A：在 Parser 级别支持增量解析（推荐）

修改 `ParseSQL` 方法，使其**可选地**接受一个已有的 Schema 而不是每次都创建新的：

```go
func (p *Parser) ParseSQL(sql string) (*model.Schema, error) {
    if p.schema == nil {
        p.schema = model.NewSchema()
    }
    // 不再每次重置 p.schema
    // ...
}
```

同时修改 `SQLFileLoader` 或 `DirectoryLoader`，在循环中复用同一个 Parser 实例。

### 方案 B：在 DirectoryLoader 中合并后再解析

将所有 SQL 文件内容合并为一个字符串，然后一次性解析。这样可以保持文件间的依赖关系：

```go
// 在 dir_loader.go 中
var allSQL strings.Builder
for _, file := range files {
    data, _ := os.ReadFile(file)
    allSQL.Write(data)
    allSQL.WriteString("\n")
}
schema, parseErr := p.parser.ParseSQL(allSQL.String())
```

**缺点**: 失去了文件级别的错误定位能力。

### 方案 C：在索引变更中创建占位表

修改 `CreateIndexMutation.Apply`，当表不存在时创建占位表（类似 `AddColumnMutation` 的做法）：

```go
func (m CreateIndexMutation) Apply(schema *model.Schema) error {
    ns := schema.GetOrCreateNamespace(m.Schema)
    table, exists := ns.Tables[m.Index.Table]
    if !exists {
        // 创建占位表
        table = model.NewTable(m.Schema, m.Index.Table)
        table.IsPlaceholder = true
        ns.Tables[m.Index.Table] = table
    }
    // ...
}
```

**优点**: 改动最小，且与现有 `AddColumnMutation` 的模式一致。  
**缺点**: 可能掩盖真正的表缺失错误。

---

## 相关文件

| 文件 | 角色 |
|------|------|
| [`internal/source/dir_loader.go`](../internal/source/dir_loader.go) | 目录加载器，逐个文件解析 |
| [`internal/source/sql_file_loader.go`](../internal/source/sql_file_loader.go) | SQL 文件加载器，每次创建新 Parser |
| [`internal/parser/parser.go`](../internal/parser/parser.go) | 解析器，`ParseSQL` 每次重置 Schema |
| [`internal/parser/index_mutation.go`](../internal/parser/index_mutation.go) | 索引变更，Apply 时检查表存在 |
| [`testdata/diff/v1/03_indexes.sql`](../../testdata/diff/v1/03_indexes.sql) | 触发错误的测试数据 |

---

## 复现步骤

```bash
# 构建项目
go build -o migra ./cmd/migra/

# 运行 diff 命令
./migra diff testdata/diff/v1/ testdata/diff/v2/

# 预期: 成功输出差异 SQL
# 实际: 报错 "table public.posts does not exist for index creation"
```

---

## 测试建议

修复后应添加以下测试场景：

1. **跨文件索引创建**: 表定义和索引定义在不同文件中
2. **跨文件外键引用**: 外键引用的表定义在另一个文件中
3. **混合顺序**: CREATE INDEX 出现在 CREATE TABLE 之前（验证是否支持乱序）
4. **snapshot 文件**: 验证 snapshot.sql 与分文件解析结果一致
