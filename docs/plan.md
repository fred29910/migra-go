# 🧩 总体目标：用 Go 重写“SQL 文件 ↔ PostgreSQL 实际 Schema”双向 Diff 工具

你要实现的是 **Go 版 MIGRA**，但语义、结构、可扩展性都要比 Python 版更现代、可维护。

核心能力：

1. **解析 SQL 文件 → 构建虚拟 schema**
2. **从 PostgreSQL 实例读取实际 schema**
3. **进行结构化 diff（表、列、约束、索引、类型、视图、函数等）**
4. **输出可执行的 SQL migration**

---

# 🧱 整体架构设计（Golang）

```
/internal
    /parser        # SQL → AST → SchemaModel
    /introspect    # DB Metadata (pg_catalog → SchemaModel)
    /model         # 中间结构化 SchemaModel
    /diff          # ModelDiff
    /render        # Diff → SQL migration
/cmd
    schemadiff     # CLI 入口
```

---

# 🗂️ 一、SchemaModel（核心中间层）

所有 diff 逻辑都基于 **中间结构模型**：

```go
type Schema struct {
    Tables map[string]*Table
    Types  map[string]*EnumType
    Views  map[string]*View
    Funcs  map[string]*Function
}

type Table struct {
    Name       string
    Columns    map[string]*Column
    PrimaryKey *PrimaryKey
    Indexes    map[string]*Index
    Constraints map[string]*Constraint
}

type Column struct {
    Name       string
    DataType   string
    Nullable   bool
    Default    *string
}
```

原则：

* Model 必须 100% 稳定、可序列化（用于测试）
* 不依赖 postgres 本身 → 可对比“任何来源”构建的 schema

---

# 📘 二、SQL 文件解析（替代 schemainspector 的 SQL parser）

## 推荐方案：pg_query（C binding）+ sqlparser AST → SchemaModel

你已经试过 Python 的 `pg_query` —— Go 版本也有 binding，可直接用。

Go 库：

* [https://github.com/lfittl/pg_query_go](https://github.com/lfittl/pg_query_go)

解析思路：

```
SQL 文件 (create table...) 
→ 使用 pg_query 生成 AST 
→ 遍历 AST → 构建 SchemaModel
```

你需要实现：

* CreateTableStmt
* AlterTableStmt
* CreateIndexStmt
* CreateTypeStmt (ENUM)
* CreateFunctionStmt
* CreateViewStmt

用 visitor pattern 实现：

```go
type Parser struct {}

func (p *Parser) ParseSQL(sql string) (*model.Schema, error) {
    tree, _ := pg_query.Parse(sql)
    for _, stmt := range tree.Statements {
        switch stmt.(type) {
        case *pg.CreateTable:
            p.handleCreateTable(...)
        }
    }
}
```

最终目的：**SQL 文件完全还原 SchemaModel**

---

# 🏛️ 三、Postgres 实例 introspection（替代 schemainspector）

使用 pg_catalog & information_schema 构建 SchemaModel：

推荐 SQL：

### 表和列

```sql
SELECT table_name, column_name, data_type, is_nullable, column_default
FROM information_schema.columns
WHERE table_schema = 'public'
ORDER BY ordinal_position;
```

### PK、约束

```sql
SELECT
    pgc.conname,
    pgc.contype,
    pg_get_constraintdef(pgc.oid) AS definition,
    tbl.relname as table_name
FROM pg_constraint pgc
JOIN pg_class tbl ON tbl.oid = pgc.conrelid
WHERE tbl.relnamespace = 'public'::regnamespace;
```

### 枚举类型

```sql
SELECT t.typname, e.enumlabel
FROM pg_type t
JOIN pg_enum e ON t.oid = e.enumtypid;
```

### 索引

```sql
SELECT indexname, indexdef, tablename
FROM pg_indexes WHERE schemaname='public';
```

你要写：

```go
func LoadFromDB(conn *pgx.Conn) (*model.Schema, error)
```

---

# 🔍 四、Diff 引擎（核心）

类似 migra 的思想，但 **结构化 diff**：

```
SchemaModel(A) vs SchemaModel(B)
↓
[]DiffOp (AddTable, DropColumn, AlterColumnType…)
↓
SQL Renderer
```

Diff 模块：

```
/diff
    diff_tables.go
    diff_columns.go
    diff_constraints.go
    diff_indexes.go
```

一个 DiffOp：

```go
type DiffOp struct {
    Kind    OperationKind
    Obj     string
    Details map[string]string
}

type OperationKind string
const (
    AddTable OperationKind = "add_table"
    DropTable
    AddColumn
    AlterColumn
    DropColumn
    AddIndex
    DropIndex
)
```

---

# 🧾 五、SQL Renderer（生成可执行 migration）

方式：

* 每个 DiffOp 知道如何转成 SQL
* 按依赖排序（先 type，再 table，再 column，再 index）

```go
func Render(op DiffOp) string {
    switch op.Kind {
    case AddTable:
        return fmt.Sprintf("CREATE TABLE %s (...)", op.Obj)
    case AddColumn:
        return fmt.Sprintf("ALTER TABLE %s ADD COLUMN %s ...", ...)
    }
}
```

最终输出：

```sql
-- Begin Diff
ALTER TABLE users ADD COLUMN age int;
CREATE INDEX idx_users_age ON users (age);
-- End Diff
```

---

# 🧪 六、测试体系（必须做到 MIGRA 级别）

三种测试：

## 1. Golden Test（SQL → SchemaModel）

```
testdata/
    input.sql
    want.schema.json
```

## 2. DB introspection test（启动 Docker PostgreSQL）

使用 ory/dockertest：

```go
db := dockertest.StartPostgres("17")
```

## 3. Diff Roundtrip Test

```
SQL A → Model A
SQL B → Model B
Diff(A,B) → SQL
执行 SQL → 得到数据库 C
C 和 B 应一致
```

---

# 💡 七、CLI 设计（类似 migra）

```
schemadiff file.sql postgres://conn
schemadiff pg://A pg://B
schemadiff fileA.sql fileB.sql
```

---

# 🚀 八、额外功能规划（Go 版可超越 migra）

| 功能                  | 是否加入 |
| ------------------- | ---- |
| YAML/JSON schema 支持 | ✓    |
| 输出“破坏性变更”诊断         | ✓    |
| 生成 rollback SQL     | ✓    |
| 输出图形化 DOT schema    | 可选   |
| LSP 插件（VSCode 完成）   | 可选   |

---

# 📌 九、实现优先级路线图（给你真实可执行的计划）

### **Phase 1 (3–5 天)：核心 SchemaModel + Parser**

* pg_query_go 解析 CREATE TABLE
* 构建 SchemaModel

### **Phase 2 (3 天)：DB introspection**

* tables, columns, constraints, indexes

### **Phase 3 (5–7 天)：Diff 引擎**

* table/column/index
* 结构化 diff

### **Phase 4 (3–4 天)：SQL Renderer**

### **Phase 5 (2–3 天)：CLI + 文档**

---

# 你继续告诉我：

## ✔️ 你想让我先为你**生成完整项目骨架**吗？

我可以直接生成：

* 完整目录结构
* 初始代码（编译可运行）
* go.mod
* CLI 主程序
  甚至可以生成一个最小可运行的 “file.sql ↔ pg” diff demo。

