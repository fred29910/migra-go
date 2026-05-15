# Bug 报告：场景 7 多 Schema diff — `CreateSchemaStmt` 未处理导致解析失败

## 元信息

| 字段 | 值 |
|---|---|
| **发现日期** | 2026-05-15 |
| **严重程度** | 🔴 高（功能完全不可用） |
| **影响范围** | 所有包含 `CREATE SCHEMA` 语句的 SQL 文件/目录 diff |
| **触发命令** | `./migra diff --schema public --schema auth testdata/diff/multi_schema/v1/ testdata/diff/multi_schema/v2/` |
| **状态** | Fixed |

---

## 错误现象

```
Error: failed to load source: failed to parse directory contents: parsing completed with 1 errors, first: parse error at position 88: unsupported statement type: pg_query.CreateSchemaStmt
```

命令以非零状态码退出，未产生任何 diff 输出。

---

## 复现步骤

1. 构建二进制：`make build`
2. 执行多 schema diff：
   ```bash
   ./migra diff --schema public --schema auth \
     testdata/diff/multi_schema/v1/ \
     testdata/diff/multi_schema/v2/
   ```
3. 观察错误：解析 v1 源目录时立即失败。

**复现条件**：任何通过 `DirectoryLoader` 或 `SQLFileLoader` 加载的 SQL 源，只要包含 `CREATE SCHEMA <name>;` 语句，都会触发此错误。

---

## 根因分析

### 直接原因

`internal/parser/registry.go` 中的 `DefaultRegistry()` 只注册了 4 种 AST 节点类型的 handler：

```go
func DefaultRegistry() *HandlerRegistry {
    r := NewHandlerRegistry()
    r.Register(pg_nodes.CreateStmt{}, &CreateTableHandler{})       // CREATE TABLE
    r.Register(pg_nodes.AlterTableStmt{}, &AlterTableHandler{})    // ALTER TABLE
    r.Register(pg_nodes.CreateEnumStmt{}, &CreateEnumHandler{})    // CREATE TYPE ... AS ENUM
    r.Register(pg_nodes.IndexStmt{}, &CreateIndexHandler{})        // CREATE INDEX
    return r
}
```

**没有注册 `pg_nodes.CreateSchemaStmt` 的 handler。**

当 `Parser.visitNode()` 遍历 AST 节点时，遇到 `CreateSchemaStmt` 节点，`registry.Dispatch()` 返回 `found=false`，于是返回 `ParseError: unsupported statement type: pg_query.CreateSchemaStmt`。

### 调用链追踪

```
migra diff --schema public --schema auth v1/ v2/
  │
  ├─ diffService.Run()
  │    ├─ loadSchema("v1/", schemas=["public","auth"])
  │    │    └─ sourceRegistry.Load("v1/", opts)
  │    │         └─ DirectoryLoader.Match("v1/") → true
  │    │              └─ DirectoryLoader.Load("v1/", opts)
  │    │                   ├─ WalkDir → ["01_public.sql", "02_auth.sql"]
  │    │                   ├─ 合并 SQL 内容：
  │    │                   │   "CREATE TABLE public.users (...);\nCREATE SCHEMA auth;\nCREATE TABLE auth.roles (...);..."
  │    │                   └─ parser.NewParser().ParseSQL(combinedSQL)
  │    │                        ├─ pg.Parse(combinedSQL) → AST (包含 CreateSchemaStmt 节点)
  │    │                        └─ visitNode(CreateSchemaStmt)
  │    │                             └─ registry.Dispatch(CreateSchemaStmt) → NOT FOUND
  │    │                                  → ParseError: unsupported statement type
  │    │                        → ParseSQL 返回 error
  │    │                   → "failed to parse directory contents: ..."
  │    │              → 返回 error
  │    │    → "failed to load source: ..."
  │    → 返回 error → 命令失败
```

### 为什么 position 是 88？

合并后的 SQL 字符串为：
```
CREATE TABLE public.users (\n    id SERIAL PRIMARY KEY,\n    name VARCHAR(100) NOT NULL\n);\n\nCREATE SCHEMA auth;\n\nCREATE TABLE auth.roles (...
```
`CREATE SCHEMA auth;` 的起始位置恰好是合并字符串的第 88 个字符左右，与错误信息 `parse error at position 88` 吻合。

### 深层原因：`--schema` 标志与 `LoadOptions.Schemas` 未对文件源生效

`--schema public --schema auth` 参数通过 `LoadOptions.Schemas` 传递给了 Loader，但：

- `DirectoryLoader.Load()` **完全忽略** `opt.Schemas` 参数
- `SQLFileLoader.Load()` **完全忽略** `opt.Schemas` 参数
- 只有 `DBLoader.Load()` 实际使用 `opt.Schemas` 来过滤 introspection 查询

这意味着对于文件/目录源，`Schemas` 选项形同虚设。即使 parser 能处理 `CREATE SCHEMA`，`--schema` 过滤也不会在加载阶段生效。

### 受影响的数据文件

| 文件 | 包含 `CREATE SCHEMA` |
|---|---|
| `testdata/diff/multi_schema/v1/02_auth.sql` | ✅ `CREATE SCHEMA auth;` |
| `testdata/diff/multi_schema/v2/02_auth.sql` | ✅ `CREATE SCHEMA auth;` |
| `testdata/diff/multi_schema/v2/snapshot.sql` | ✅ `CREATE SCHEMA auth;` |

---

## 问题分类

这是一个**两个独立问题叠加**导致的 bug：

### 问题 1：Parser 缺少 `CreateSchemaStmt` handler（直接原因）

- **位置**：`internal/parser/registry.go:36-43`
- **性质**：功能缺失 — parser 不支持 `CREATE SCHEMA` 语句
- **影响**：任何包含 `CREATE SCHEMA` 的 SQL 文件都无法被解析

### 问题 2：`LoadOptions.Schemas` 对文件源无效（设计缺陷）

- **位置**：`internal/source/dir_loader.go:45-101`、`internal/source/sql_file_loader.go:24-41`
- **性质**：`--schema` 标志仅对数据库源生效，对文件/目录源不执行任何过滤
- **影响**：即使 parser 支持 `CREATE SCHEMA`，用户也无法通过 `--schema` 来限定只比较特定 schema

---

## 修复方向（不实施，仅记录）

### 修复问题 1：添加 `CreateSchemaStmt` handler

1. 新建 `internal/parser/create_schema_handler.go`，实现 `Handler` 接口
2. 处理 `pg_nodes.CreateSchemaStmt` 节点
3. 返回一个 `SchemaMutation`（或空 mutations 列表，因为 schema 命名空间在 `model.Schema` 中是自动创建的）
4. 在 `DefaultRegistry()` 中注册：`r.Register(pg_nodes.CreateSchemaStmt{}, &CreateSchemaHandler{})`

### 修复问题 2：`--schema` 过滤对文件源生效

**方案 A**：在 `DirectoryLoader`/`SQLFileLoader` 解析后，根据 `opt.Schemas` 过滤 `model.Schema.Schemas` map，移除不在列表中的 namespace。

**方案 B**：在 `diff_service.go` 的 `ComputeDiff` 阶段，根据 `cfg.Schemas` 过滤 schema，只保留指定的 namespace 进行比较。

方案 B 更合理，因为过滤逻辑属于 diff 语义层，而非加载层。

---

## 相关代码文件

| 文件 | 角色 |
|---|---|
| `internal/parser/registry.go` | Handler 注册中心，缺少 `CreateSchemaStmt` |
| `internal/parser/parser.go` | Parser 核心，`visitNode` 中 dispatch 失败 |
| `internal/source/dir_loader.go` | 目录加载器，合并 SQL 后一次性解析 |
| `internal/source/sql_file_loader.go` | SQL 文件加载器 |
| `internal/source/loader.go` | `LoadOptions` 定义（含 `Schemas`） |
| `internal/source/db_loader.go` | 唯一使用 `opt.Schemas` 的 Loader |
| `internal/app/diff_service.go` | Diff 服务编排，`Schemas` 未在计算阶段使用 |
| `cmd/migra/diff.go` | CLI 入口，`--schema` 标志定义和传递 |
| `testdata/diff/multi_schema/v1/02_auth.sql` | 触发 bug 的测试数据 |
| `testdata/diff/multi_schema/v2/02_auth.sql` | 触发 bug 的测试数据 |

---

## 修复记录

### 修复时间

2026-05-15

### 修改文件

| 文件 | 变更说明 |
|------|----------|
| [`internal/parser/create_schema_handler.go`](../internal/parser/create_schema_handler.go) | **新增**：`CreateSchemaHandler` 处理 `CreateSchemaStmt` AST 节点；`CreateSchemaMutation` 确保 namespace 存在 |
| [`internal/parser/registry.go`](../internal/parser/registry.go) | `DefaultRegistry` 中注册 `CreateSchemaStmt` → `CreateSchemaHandler` |
| [`internal/parser/mutation.go`](../internal/parser/mutation.go) | 新增 `MutKindCreateSchema` 常量 |
| [`internal/model/object_key.go`](../internal/model/object_key.go) | 新增 `KindSchema` 常量 |
| [`internal/app/diff_service.go`](../internal/app/diff_service.go) | 新增 `FilterNamespaces` 函数；`ComputeDiff` 中于 Normalize 后、Diff 前调用 |
| [`internal/parser/handler_test.go`](../internal/parser/handler_test.go) | 新增 `TestCreateSchemaHandler` + 类型安全检查 |
| [`internal/parser/mutation_test.go`](../internal/parser/mutation_test.go) | 新增 `TestCreateSchemaMutation_Apply` + `_Idempotent` |
| [`internal/app/diff_service_test.go`](../internal/app/diff_service_test.go) | 新增 `TestFilterNamespaces_*` |
| [`cmd/migra/integration_test.go`](../cmd/migra/integration_test.go) | 新增 `TestMultiSchemaDiff` |

### 修复要点

1. **Parser 支持 `CREATE SCHEMA`**：新增 `CreateSchemaHandler`，从 AST 提取 `Schemaname`，返回 `CreateSchemaMutation` 确保 namespace 在 model.Schema 中存在
2. **`--schema` 过滤对文件源生效**：在 `ComputeDiff` 的 Normalize 之后、Diff 之前，调用 `FilterNamespaces` 移除不在 `--schema` 列表中的 namespace。`schemas` 为空时不过滤，保持向后兼容

---

## 验证方法

修复后应满足：

1. `./migra diff --schema public --schema auth testdata/diff/multi_schema/v1/ testdata/diff/multi_schema/v2/` 成功执行
2. 输出包含：
   - `CREATE TABLE "public"."profiles"` (新增表)
   - `ALTER TABLE "auth"."roles" ADD COLUMN "description"` (新增列)
   - `ALTER TABLE "public"."users" ADD COLUMN "email"` (新增列)
3. 不输出任何 `DROP` 操作（safe mode）
4. 不输出解析错误或警告
