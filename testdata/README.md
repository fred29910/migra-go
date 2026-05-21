# MIGRA-Go Testdata 指南

本目录包含 migra-go 项目的测试数据，用于单元测试、集成测试和 golden file 测试。

## 目录结构

```
testdata/
├── README.md                     ← 本文件
├── example_source.sql            ← 基础 diff 测试：源（2 表 + 索引 + 枚举）
├── example_target.sql            ← 基础 diff 测试：目标（增列 + 增表 + 枚举标签）
├── alter_operations.sql          ← 12 种 ALTER TABLE 变体（ADD/DROP/SET/ALTER COLUMN TYPE）
├── complex_ddl.sql               ← 复合约束、高级类型、自定义枚举
├── drop_scenarios.sql            ← DROP 语义测试（残留 schema 状态）
├── edge_cases.sql                ← 边界 SQL 模式（引号标识符、继承表、分区表）
│
└── diff/                         ← 目录型 diff 场景（DirectoryLoader + 全流水线测试）
├── v1/                       ← 版本 v1：users + posts + 索引 + 枚举
│   ├── 01_users.sql          ←   拆分多文件：用户表
│   ├── 02_posts.sql          ←   拆分多文件：文章表
│   ├── 03_indexes.sql        ←   拆分多文件：索引
│   └── 04_enums.sql          ←   拆分多文件：枚举
    │
├── v2/                       ← 版本 v2：v1 + age 列 + comments 表 + guest 枚举
│   └── schema.sql            ←   合并单文件
    │
    ├── v3/                       ← 版本 v3：修改/删除场景
    │   ├── schema.sql            ←   删 age 列/comments 表；增 phone 列/categories 表/UNIQUE(email)
    │   └── README.md             ←   场景说明
    │
    ├── snapshot.sql              ← v2 快照（单文件等于 v2/ 目录）
    │
    ├── nested/                   ← 嵌套子目录结构
    │   ├── 01_tables/users.sql   ←   子目录 1：用户表
    │   └── 02_tables/posts.sql   ←   子目录 2：文章表
    │
    ├── nested_target/            ← 嵌套目录的目标版本（扩展列+外键）
    │   ├── 01_tables/users.sql   ←   用户表（增加 email 列）
    │   └── 02_tables/posts.sql   ←   文章表（增加 FK 引用）
    │
    ├── multi_schema/             ← 多 schema 命名空间场景
    │   ├── v1/                   ←   版本 v1：public + auth
    │   │   ├── 01_public.sql     ←      public.users
    │   │   └── 02_auth.sql       ←      auth.roles + auth.permissions
    │   └── v2/                   ←   版本 v2：扩展版本
    │       ├── 01_public.sql     ←      public.users（增 email）+ public.profiles
    │       ├── 02_auth.sql       ←      auth.roles（增 description）
    │       └── snapshot.sql      ←      合并快照
    │
    ├── rename_example/           ← RENAME COLUMN 基础场景
    │   ├── v1/                   ←   版本 v1：users(username)
    │   │   └── schema.sql        ←      users + posts 表
    │   └── v2/                   ←   版本 v2：users(login_name)
    │       └── schema.sql        ←      username 重命名为 login_name
    │
    ├── rename_complex/           ← RENAME COLUMN 组合变更场景
    │   ├── v1/                   ←   版本 v1：含 age 列 + username 索引
    │   │   └── schema.sql        ←      users(username, age) + idx_users_username
    │   └── v2/                   ←   版本 v2：重命名 + 删列 + 新增 + 改索引
    │       └── schema.sql        ←      users(login_name, phone) + idx_users_login_name
    │
    ├── v4/                       ← 版本 v4：IDENTITY + COLLATE + FK CASCADE
    │   ├── schema.sql            ←   在 v3 基础上将 id 转为 IDENTITY，引入 COLLATE、CASCADE FK
    │   └── README.md             ←   变更说明
    │
    ├── identity_example/         ← IDENTITY 列检测专用场景
    │   ├── v1/                   ←   版本 v1：SERIAL 基线
    │   │   └── schema.sql        ←      users(id SERIAL, ...)
    │   └── v2/                   ←   版本 v2：IDENTITY 版本
    │       └── schema.sql        ←      users(id GENERATED ALWAYS AS IDENTITY, ...)
    │
    ├── collate_example/          ← COLLATE 子句检测专用场景
    │   ├── v1/                   ←   版本 v1：无 COLLATE
    │   │   └── schema.sql        ←      users(name varchar)
    │   └── v2/                   ←   版本 v2：含 COLLATE
    │       └── schema.sql        ←      users(name varchar COLLATE "en_US")
    │
    ├── objects_example/          ← VIEW/SEQUENCE/EXTENSION 检测专用场景
    │   ├── v1/                   ←   版本 v1：仅表
    │   │   └── schema.sql        ←      users + posts
    │   └── v2/                   ←   版本 v2：表 + EXTENSION + SEQUENCE + VIEW
    │       └── schema.sql        ←      + pgcrypto + seq_id + user_view
    │
    └── edge/                     ← DirectoryLoader 边界情况测试数据
        ├── tables/users.sql      ←   正常 SQL 文件（可被加载）
        ├── .hidden.sql           ←   隐藏文件（应被跳过）
        ├── .hidden_dir/          ←   隐藏目录（应被跳过）
        │   └── secret.sql
        ├── empty/                ←   空目录（应产生警告）
        ├── readme.txt            ←   非 .sql 文件（应被跳过）
        ├── empty.sql             ←   纯注释 SQL 文件（应无错误无变更）
        ├── encoding_utf8.sql     ←   UTF-8 编码 + Unicode 注释
        ├── no_extension_file     ←   无 .sql 后缀的 SQL 文件
        └── large/                ←   大目录（50 张重复表）
            └── generated_schema.sql
```

## 测试数据全景

### 测试数据 vs 测试用例映射

| 测试数据文件 | 被哪些测试引用 | 测试场景 |
|---|---|---|
| `example_source.sql` + `example_target.sql` | `internal/app/diff_service_test.go:TestDiffService_Run`<br>`cmd/migra/integration_test.go:TestExampleSQLFilesDiffIncludesEnumAndConstraints` | 基础 diff 流水线：增列、增表、枚举加标签 |
| `diff/v1/schema.sql` → `diff/v2/schema.sql` | `cmd/migra/integration_test.go:TestV3DirectoryDiff` | 目录 vs 目录 diff：相同变更集 |
| `diff/v3/schema.sql` | `cmd/migra/integration_test.go:TestV3UnsafeDropDiff_Safe`<br>`cmd/migra/integration_test.go:TestV3UnsafeDropDiff_Unsafe` | 删除/修改场景：删列、删表、改索引名、改枚举 |
| `diff/nested/` → `diff/nested_target/` | `cmd/migra/integration_test.go:TestNestedDirectoryDiff` | 嵌套子目录 diff |
| `diff/multi_schema/v1/` → `diff/multi_schema/v2/` | `cmd/migra/integration_test.go:TestMultiSchemaDirectoryDiffStatic` | 多 schema（public + auth）diff |
| `diff/rename_example/v1/` → `diff/rename_example/v2/` | `cmd/migra/integration_test.go:TestRenameColumnDiff` | RENAME COLUMN 基础场景 |
| `diff/rename_complex/v1/` → `diff/rename_complex/v2/` | `cmd/migra/integration_test.go:TestRenameColumnComplexDiff` | RENAME COLUMN + 删列 + 新增组合 |
| `diff/v4/` | 未直接引用（供未来集成测试用） | v3→v4 演进：IDENTITY + COLLATE + FK CASCADE |
| `diff/identity_example/v1/` → `diff/identity_example/v2/` | `cmd/migra/integration_test.go:TestIdentityColumnDiff` | IDENTITY 列检测专用 |
| `diff/collate_example/v1/` → `diff/collate_example/v2/` | `cmd/migra/integration_test.go:TestCollateClauseDiff` | COLLATE 子句检测专用 |
| `diff/objects_example/v1/` → `diff/objects_example/v2/` | `cmd/migra/integration_test.go:TestObjectsDiff` | VIEW/SEQUENCE/EXTENSION 检测专用 |
| `diff/edge/{.hidden.sql,readme.txt,empty/...}` | 未直接引用（DirLoader 测试用 `t.TempDir()` 动态创建数据） | DirectoryLoader 边界条件 |
| `alter_operations.sql` | `internal/parser/handler_test.go:TestParseAlterOperations` | ALTER TABLE 全操作集解析 |
| `complex_ddl.sql` | `internal/parser/handler_test.go:TestParseComplexDDL` | 复合约束/高级类型解析 |
| `drop_scenarios.sql` | `internal/parser/handler_test.go:TestParseDropScenarios` | DROP 语义残留状态 |
| `edge_cases.sql` | `internal/parser/handler_test.go:TestParseEdgeCases` | 边界 SQL 模式 |
| `internal/model/testdata/schema_golden.json` | `internal/model/golden_test.go:TestGoldenSchema` | Schema JSON 序列化稳定性 |

### 版本演进图谱

`diff/v1/` → `diff/v2/` → `diff/v3/` → `diff/v4/` 构成一个四阶段演进序列：

```
v1 (基线)
├── users:      id, username, email, created_at
├── posts:      id, user_id, title, content (FK→users)
├── idx_posts_user_id, idx_users_username
├── user_role:  'admin', 'user'
│
v2 (v1 + 新增)
├── users:     +age
├── comments:  新建 (FK→posts)
├── user_role: +'guest'
│
v3 (v2 - 删除 + 修改)
├── users:     -age, +phone, +UNIQUE(email)
├── posts:     不变
├── categories: 新建 (UNIQUE name)
├── comments:  删除
├── idx_users_username → idx_users_username_unique
├── user_role: -'guest', +'moderator'
│
v4 (v3 + IDENTITY + COLLATE + FK CASCADE)
├── users:     id → GENERATED ALWAYS AS IDENTITY (was SERIAL)
├── posts:     user_id +content COLLATE "en_US"
├── categories: name COLLATE "en_US"
├── categories: 新增 UNIQUE(name)
├── posts:     FK user_id 新增 ON DELETE CASCADE
```

## 测试流程

### 三层测试架构

```
┌─────────────────────────────────────────────────┐
│  第三层：集成测试 (cmd/migra/integration_test.go) │
│  全流水线：Loader → Parser → Diff → Plan → Render │
│  使用 testdata/ 中的 .sql 文件作为输入             │
├─────────────────────────────────────────────────┤
│  第二层：逻辑测试 (internal/diff/plan/render)      │
│  算法正确性：Diff 引擎、DAG 排序、执行计划分组       │
│  使用代码构造 model.Schema，不依赖 testdata 文件    │
├─────────────────────────────────────────────────┤
│  第一层：单元测试 (internal/parser/source/model)    │
│  组件隔离测试：Parser、Loader、Mutation             │
│  Parser/Loader 使用 .sql 字符串，Model 用代码构造   │
└─────────────────────────────────────────────────┘
```

### 测试类型详解

#### 1. 单元测试 — 组件隔离测试

| 包 | 测试文件 | 验证目标 |
|---|---|---|
| `internal/parser` | `parser_test.go` | SQL→Schema 解析：CREATE TABLE、ALTER TABLE、ENUM、索引 |
| `internal/parser` | `handler_test.go` | Handler 类型安全和字段解析 |
| `internal/parser` | `mutation_test.go` | Mutation Apply 逻辑：建表、加列、占位表合并 |
| `internal/parser` | `index_handler_test.go` | 索引处理：普通、唯一、表达式、部分索引 |
| `internal/parser` | `index_mutation_test.go` | 索引 Mutation 应用 |
| `internal/source` | `dir_loader_test.go` | DirectoryLoader：多文件、嵌套、隐藏、重复、解析错误 |
| `internal/source` | `loader_test.go` | Loader 接口匹配逻辑 |
| `internal/model` | `schema_test.go` | Schema/Table 序列化和占位表 |
| `internal/normalize` | `normalize_test.go` | 类型同义映射、表达式规范化、索引规范化 |
| `internal/render` | `render_test.go` | SQL 生成：表、列、约束、索引、JSON 输出 |
| `internal/plan` | `plan_test.go` | 三阶段执行计划分组 |
| `internal/plan` | `dag_test.go` | DAG 拓扑排序：依赖顺序、稳定排序 |

#### 2. 逻辑测试 — Diff 引擎核心

| 测试文件 | 验证场景 |
|---|---|
| `differ_test.go` | 无共享状态、枚举加标签+设默认+删列组合、同名约束内容变更 |
| `diff_index_test.go` | 索引内容变更、内容相同无变更、表达式索引变更 |
| `diff_constraint_test.go` | 约束列填充、PK 无假阳性、nil 列处理 |
| `operation_test.go` | Operation 接口契约（DependsOn 方法） |

#### 3. 集成测试 — 全流水线

| 测试函数 | 数据源 | 验证点 |
|---|---|---|
| `TestIntegrationDiffRender` | 代码构造 Schema | diff → render SQL/JSON |
| `TestIntegrationEnumType` | 代码构造 Schema | 枚举 diff + render |
| `TestIntegrationDAGSort` | 代码构造 Operation | DAG 顺序：AddTable 先于 AddColumn |
| `TestExampleSQLFilesDiffIncludesEnumAndConstraints` | `testdata/example_source.sql` + `testdata/example_target.sql` | 增表、增列、枚举加标签、主键 |
| `TestV3DirectoryDiff` | `testdata/diff/v2/schema.sql` + `testdata/diff/v3/schema.sql` | v2→v3 差异（safe mode） |
| `TestV2ToV3SafeDiff` | `testdata/diff/v1/` + `testdata/diff/v3/` | v1 目录 vs v3 目录 diff |
| `TestV3UnsafeDropDiff_Safe` | `testdata/diff/v2/schema.sql` + `testdata/diff/v3/schema.sql` | v2→v3 安全模式（过滤 DROP） |
| `TestV3UnsafeDropDiff_Unsafe` | `testdata/diff/v2/schema.sql` + `testdata/diff/v3/schema.sql` | v2→v3 unsafe-drop 模式（含 DROP） |
| `TestNestedDirectoryDiff` | `testdata/diff/nested/` + `testdata/diff/nested_target/` | 嵌套子目录 diff |
| `TestMultiSchemaDirectoryDiffStatic` | `testdata/diff/multi_schema/v1/` + `testdata/diff/multi_schema/v2/` | 多 schema diff |
| `TestRenameColumnDiff` | `testdata/diff/rename_example/v1/` + `testdata/diff/rename_example/v2/` | RENAME COLUMN 基础 |
| `TestRenameColumnComplexDiff` | `testdata/diff/rename_complex/v1/` + `testdata/diff/rename_complex/v2/` | RENAME COLUMN 组合变更 |
| `TestIdentityColumnDiff` | `testdata/diff/identity_example/v1/` + `testdata/diff/identity_example/v2/` | SERIAL → IDENTITY 变更检测 |
| `TestCollateClauseDiff` | `testdata/diff/collate_example/v1/` + `testdata/diff/collate_example/v2/` | COLLATE 子句新增检测 |
| `TestObjectsDiff` | `testdata/diff/objects_example/v1/` + `testdata/diff/objects_example/v2/` | EXTENSION + SEQUENCE + VIEW 差异 |
| `TestV3ToV4Diff` | `testdata/diff/v3/schema.sql` + `testdata/diff/v4/schema.sql` | v3→v4 IDENTITY/COLLATE/CASCADE 演进 |
| `TestFullPipeline` | 代码构造 Schema (占位) | 流水线编排框架 |
| `TestDirectoryVsDirectory_Diff` | `t.TempDir()` 动态创建 | 目录 vs 目录 diff |
| `TestDirectoryVsFile_Diff` | `t.TempDir()` 动态创建 | 目录 vs 文件 diff |

### 数据源类型

测试使用三种数据源策略：

```
策略一：静态 testdata 文件 (.sql)
  用途：集成测试、Service 层测试
  特点：版本可控、可审查、多人共享
  文件：testdata/example_source.sql, example_target.sql
  测试：TestExampleSQLFilesDiffIncludesEnumAndConstraints

策略二：t.TempDir() 动态创建
  用途：目录加载器测试、目录 diff 测试
  特点：隔离性好、无状态污染、并发安全
  测试：TestDirectoryLoader_Load_*, TestDirectoryVs*_Diff

策略三：代码构造 model.Schema
  用途：Diff 引擎测试、Plan 测试、Render 测试
  特点：精确控制、高覆盖率、不依赖外部文件
  测试：TestDiffer_*, TestPlanner_*, TestRender*
```

## 如何运行测试

### 本地运行

```bash
# 执行全部测试
make test
# 实际命令：go test ./... -v

# 运行测试并生成覆盖率报告
make test-coverage
# 生成 coverage.html，在浏览器中打开查看

# 运行单个包测试
go test ./internal/diff/... -v
go test ./internal/parser/... -v -run TestParseCreateTable

# 运行单个测试
go test ./... -v -run TestExampleSQLFilesDiffIncludesEnumAndConstraints
```

### CI 流程

GitHub Actions 自动在 push/PR 时执行：

```yaml
# .github/workflows/test.yml
steps:
  - uses: actions/checkout@v4
  - uses: actions/setup-go@v5
    with:
      go-version: '1.26'
  - run: make build
  - run: go test ./... -v -short    # CI 中使用 -short 跳过需数据库的测试
```

### 完整 CI 检查

```bash
make ci
# 顺序执行：fmt → vet → lint → test
```

## Golden 文件管理

### 机制

`internal/model/testdata/schema_golden.json` 是 Schema 模型的 golden file：

1. `TestGoldenSchema` 构造 Schema → JSON → 与 golden 文件逐字节比较
2. 若一致 → 测试通过；若不一致 → 测试失败，打印 diff

### 更新 Golden 文件

当 Schema 模型字段有变化（新增字段、修改 JSON tag）时：

```bash
UPDATE_GOLDEN=1 go test ./internal/model/... -run TestGoldenSchema
```

这会重新生成 `schema_golden.json`，需要一并提交到版本控制。

### 注意事项

- Golden 文件由测试自动生成，**不要手动编辑**
- 修改模型字段后必须运行 `UPDATE_GOLDEN=1` 更新 golden 文件
- 提交 PR 时 golden 文件的变更应与模型变更一起审查

## 测试基础设施

### 依赖

```
github.com/stretchr/testify v1.11.1    # 断言库 (require/assert)
github.com/lfittl/pg_query_go v1.0.2   # PostgreSQL SQL 解析器（基于 libpg_query）
github.com/jackc/pgx/v5 v5.9.2         # PostgreSQL 驱动（DBLoader 使用）
```

### 辅助工具

```go
// internal/testutil/testutil.go
func GoldenFile(t, expectedPath, actual string, updateFlag bool)
func CompareJSON(t, expected, actual string)
func LoadSchemaFromJSON(t, path) *model.Schema
func SaveSchemaToJSON(t, schema, path)
```

### 跳过数据库测试

所有需要真实 PostgreSQL 连接的测试都被标记为 `t.Skip("requires database")` 或放在单独的测试文件中。
CI 中 `go test -short` 会进一步确保不触发数据库连接。

## 测试数据设计原则

1. **自包含**：每个 .sql 文件独立可解析，不依赖其他文件
2. **渐进复杂**：v1（基线）→ v2（新增）→ v3（删除/修改）→ v4（IDENTITY/COLLATE/CASCADE）构成完整演进序列
3. **模块化场景**：新增单个 DDL 特性的专项测试数据（`identity_example/`、`collate_example/`、`objects_example/`），与演进序列互为补充
4. **边界覆盖**：隐藏文件、空目录、非 SQL 文件、大目录、UTF-8 编码
5. **多粒度**：单文件、多文件、嵌套目录、多 schema
6. **版本一致性**：`snapshot.sql` 文件与对应目录内容等效，用于测试 "目录 vs 单文件" 同语义 diff

## 如何新增测试数据

1. 在 `testdata/` 或 `testdata/diff/` 下创建新 SQL 文件
2. 遵循既有格式：大写 SQL 关键字、小写标识符、4 空格缩进
3. 如添加新版本目录，同时在 `diff/v3/` 同级提供 `snapshot.sql` 合并快照
4. 在新测试中引用时使用相对于测试文件的路径
5. 对于 DirectoryLoader 测试，优先使用 `t.TempDir()` 动态创建；静态 testdata 用于集成测试

## 常见问题排查

| 现象 | 原因 | 解决方案 |
|---|---|---|
| `TestGoldenSchema` 失败 | Schema 模型字段变更，golden 文件过期 | `UPDATE_GOLDEN=1 go test ./internal/model/...` |
| DirectoryLoader 测试读取到多余表 | 测试间状态污染 | 检查是否复用了 `Differ` 等有状态对象；每个测试应使用独立的 Loader 实例 |
| 集成测试找不到 testdata 文件 | 测试工作目录不对 | 路径相对于测试文件所在包目录（`internal/app/` 用 `testdata/`，`cmd/migra/` 用 `../../testdata/`） |

## 二进制 CLI 验证

`migra diff` 是项目的核心命令，负责比较两个 schema 来源并输出差异 SQL。以下是完整的二进制验证流程。

### 五阶段流水线

```
┌──────────┐   ┌──────────┐   ┌───────────┐   ┌─────────┐   ┌──────────┐
│  Source  │──▶│  Parser  │──▶│ Normalize │──▶│  Diff   │──▶│  Plan    │──▶ Render
│  (Loader)│   │ (AST →   │   │ (语义归一化)│   │ (Schema │   │ (DAG 排序│   (SQL/JSON)
│          │   │  Schema) │   │           │   │  A vs B)│   │  三阶段) │
└──────────┘   └──────────┘   └───────────┘   └─────────┘   └──────────┘
```

| 阶段 | 组件 | 输入 → 输出 | 说明 |
|---|---|---|---|
| **1. Source** | Registry → Loader | `source string` → `*model.Schema` | 根据来源字符串匹配合适的 Loader（DBLoader / DirectoryLoader / SQLFileLoader） |
| **2. Parser** | Parser → Handlers → Mutations | `SQL(string)` → `[]SchemaMutation` → `Schema` | pg_query_go 解析 AST，HandlerRegistry 路由到对应 Handler，Mutation 应用到 Schema |
| **3. Normalize** | CanonicalizeSchema | `Schema` → `Schema` (in-place) | 同义类型映射（`int4`→`integer`），表达式规范化 |
| **4. Diff** | Differ.diffContext | `source,target Schema` → `[]Operation` | 逐层比较 namespace → table → column → index → constraint → type |
| **5. Plan** | Planner → DAG TopoSort | `[]Operation` → `[]Operation(sorted)` | 三阶段分组（Pre-deploy / Deploy / Post-deploy）→ Kahn 拓扑排序 |
| **6. Render** | Renderer | `[]Operation` → `SQL(string)` / `JSON(string)` | 每个 Operation 渲染为对应 SQL DDL 语句 |

### 构建二进制

```bash
# 标准构建
make build
# 结果：./migra

# 验证构建
./migra --help
./migra --version
```

构建时注入版本信息：

| 注入字段 | 来源 |
|---|---|
| `Version` | `git describe --tags --always --dirty` |
| `BuildTime` | `date -u +%Y-%m-%dT%H:%M:%SZ` |
| `GitCommit` | `git rev-parse --short HEAD` |
| `GoVersion` | `go version` 输出 |

```bash
# 示例输出
$ ./migra --version
migra v0.1.0 (commit: a1b2c3d, built: 2026-05-15T10:00:00Z, go1.26)
```

### 流水线详细流程

```
CLI 参数解析 (parseDiffConfig)
  │
  ├─ 参数模式：
  │   2 args → source=args[0], target=args[1]
  │   1 arg  → source=args[0], target=config.database.url
  │   0 args → source=config.database.source, target=config.database.target
  │
  ├─ 标志绑定 (viper):
  │   --schema, -s     []string  默认 ["public"]
  │   --format, -f     string    默认 "sql"（可选 "json"）
  │   --unsafe-drop    bool      默认 false
  │   --strict         bool      默认 false
  │   --output, -o     string    默认 ""（stdout）
  │   --timeout        duration  默认 30s
  │
  └─ Config → app.NewDiffService(deps).Run()
       │
       ├─ 1. LoadSchema(source) ────────────────────────────────
       │    ├─ sourceRegistry.Load(ctx, source, opts)
       │    │    ├─ DBLoader.Match?     ← "postgres://..."
       │    │    ├─ DirectoryLoader.Match? ← 目录路径
       │    │    └─ SQLFileLoader.Match?   ← "*.sql" / "file://..."
       │    │
       │    ├─ Loader.Load() ──────────────────────────
       │    │    ├─ SQLFileLoader:    os.ReadFile → parser.ParseSQL
       │    │    ├─ DirectoryLoader:  filepath.WalkDir → 收集 .sql → SQLFileLoader(各文件) → Merge
       │    │    └─ DBLoader:         pgx.Connect → pg_catalog queries → Schema
       │    │
       │    └─ Parser.ParseSQL(sql) ──────────────────
       │         ├─ pg_query.Parse (AST)
       │         ├─ visitNode(各AST节点)
       │         │    ├─ CreateStmt   → CreateTableHandler  → CreateTableMutation
       │         │    ├─ AlterTableStmt → AlterTableHandler  → Add/Drop/AlterColumnMutation
       │         │    ├─ CreateEnumStmt → CreateEnumHandler   → CreateEnumTypeMutation
       │         │    └─ IndexStmt      → CreateIndexHandler  → CreateIndexMutation
       │         └─ MutationApplier.Apply(Schema, mutations)
       │
       ├─ 2. LoadSchema(target) ── 同上
       │
       ├─ 3. ComputeDiff(src, tgt) ─────────────────────────────
       │    ├─ NormalizeSchemas(src, tgt)   ← normalize.CanonicalizeSchema
       │    ├─ differ.Diff(src, tgt)        ← diffContext.diffSchemas
       │    │    ├─ diffTables    → 表增删、列比较、索引比较、约束比较
       │    │    ├─ diffColumns   → 类型、非空、默认值
       │    │    ├─ diffIndexes   → 内容变更检测
       │    │    ├─ diffConstraints → 约束变更检测
       │    │    └─ diffTypes     → 枚举增删标签
       │    ├─ FilterDestructiveOps(ops, unsafeDrop)  ← 过滤 DROP 操作
       │    └─ BuildExecutionPlan(filteredOps)         ← Planner.Plan + TopoSort
       │
       └─ 4. RenderOutput(ops, format) ─────────────────────────
            ├─ "sql"  → RenderAll(ops) → 拼接 DDL
            └─ "json" → RenderJSON(ops) → JSON 数组
```

### 数据源路由规则

```
Registry.Load(source)
  │
  ├─ DBLoader.Match():
  │   HasPrefix "postgres://" | "postgresql://" | "pg://"
  │
  ├─ DirectoryLoader.Match():
  │   NOT DB URL AND os.Stat(source).IsDir() == true
  │   支持 "file://" 前缀
  │
  └─ SQLFileLoader.Match():
      HasSuffix ".sql" | HasPrefix "file://"
```

### 扫描规则（DirectoryLoader）

```
filepath.WalkDir(source)
  │
  ├─ 跳过隐藏文件/目录（HasPrefix "."）→ SkipDir
  ├─ 仅收录 .sql 后缀文件（不区分大小写）
  ├─ 排序（sort.Strings）保证确定性顺序
  └─ 合并 SQL 内容 → 一次性解析

合并策略：
  - 所有 .sql 文件的内容合并为一个字符串，一次性解析
  - 确保跨文件 DDL 依赖（索引引用另一文件的表、外键等）正确解析
  - 重复表名由 `CreateTableMutation.Apply` 检测并返回错误
  - **注意**: 目录中不应同时包含 `schema.sql`（完整快照）和分文件，否则会导致重复定义
```

### 参数模式验证

`migra diff` 支持 0/1/2 个参数：

| 参数数 | 场景 | Source 来源 | Target 来源 |
|---|---|---|---|
| 2 | 最常用 | args[0] | args[1] |
| 1 | 单参数 | args[0] | config.database.url |
| 0 | 全配置 | config.database.source | config.database.target |

```bash
# 验证各参数模式
./migra diff testdata/example_source.sql testdata/example_target.sql    # 2 参数
./migra diff testdata/example_source.sql                                # 1 参数（需配置）
./migra diff                                                             # 0 参数（需配置）
```

### 验证场景清单

以下所有场景使用 `./migra` 二进制，不需要数据库连接。

#### 场景 1：基本文件 vs 文件 diff

```bash
./migra diff testdata/example_source.sql testdata/example_target.sql
```

**预期输出**（safe mode，不带 --unsafe-drop）：
```sql
-- op: add_table risk:low
CREATE TABLE "public"."comments" (
    "id" serial,
    "post_id" integer NOT NULL,
    "content" text NOT NULL,
    "created_at" timestamp DEFAULT now(),
    CONSTRAINT "comments_pkey" PRIMARY KEY ("id"),
    CONSTRAINT "comments_post_id_fkey" FOREIGN KEY ("post_id") REFERENCES "public"."posts" ("id")
);

-- op: add_column risk:low
ALTER TABLE "public"."users" ADD COLUMN "age" integer;

-- op: add_enum_label risk:low
ALTER TYPE "public"."user_role" ADD VALUE 'guest';
```

**验证点**：
- ✅ 检测到新建 `comments` 表（含主键和外键）
- ✅ 检测到 `users.age` 新列
- ✅ 检测到 `user_role` 枚举新增 `guest` 标签
- ✅ 按拓扑排序：`comments` 表先于其外键依赖输出
- ✅ 无 DROP 操作（safe mode）
- ✅ 输出格式为可执行 SQL，每段前有操作注释

#### 场景 2：JSON 格式输出

```bash
./migra diff --format json testdata/example_source.sql testdata/example_target.sql
```

**预期输出**（JSON 数组）：
```json
[
  {
    "kind": "add_table",
    "object_key": "public.comments",
    "destructive": false,
    "sql": "CREATE TABLE \"public\".\"comments\" ..."
  },
  {
    "kind": "add_column",
    "object_key": "public.users.age",
    "destructive": false,
    "sql": "ALTER TABLE \"public\".\"users\" ADD COLUMN \"age\" integer"
  },
  {
    "kind": "add_enum_label",
    "object_key": "public.user_role.guest",
    "destructive": false,
    "sql": "ALTER TYPE \"public\".\"user_role\" ADD VALUE 'guest'"
  }
]
```

**验证点**：
- ✅ JSON 格式正确可解析
- ✅ 包含 `kind`、`object_key`、`destructive`、`sql` 字段
- ✅ 与 SQL 格式相同操作集合

#### 场景 3：目录 vs 目录 diff

```bash
# v1 → v2：目录结构 diff（单文件目录）
./migra diff testdata/diff/v1/ testdata/diff/v2/

# 验证：效果应等同于文件 vs 文件
./migra diff testdata/example_source.sql testdata/example_target.sql
```

**验证点**：
- ✅ 目录加载正确（v1 单文件 vs v1 拆分多文件结果一致）
- ✅ 检测到 `age` 列、`comments` 表、`guest` 枚举标签

#### 场景 4：目录 vs 快照文件 diff

```bash
# 目录 vs 快照文件（应无差异）
./migra diff testdata/diff/v2/ testdata/diff/snapshot.sql

# 目录 vs 另一版本的快照文件
./migra diff testdata/diff/v2/ testdata/diff/snapshot.sql
```

**验证点**：
- ✅ 目录加载结果与 snapshot 合并快照语义相同 → 无变更输出
- ✅ 不同版本间 diff 正确

#### 场景 5：v3 删除/修改场景

```bash
# v2 → v3：检测删除和修改
./migra diff --unsafe-drop testdata/diff/v2/schema.sql testdata/diff/v3/schema.sql

# 不带 --unsafe-drop（应过滤 DROP）
./migra diff testdata/diff/v2/schema.sql testdata/diff/v3/schema.sql
```

**带 --unsafe-drop 预期输出包含**：
```sql
-- DROP 操作（destructive）
ALTER TABLE "public"."users" DROP COLUMN IF EXISTS "age";
DROP TABLE "public"."comments";
DROP INDEX "public"."idx_users_username";
DROP TYPE "public"."user_role";

-- 新建/修改
ALTER TABLE "public"."users" ADD COLUMN "phone" varchar(20);
ALTER TABLE "public"."users" ADD CONSTRAINT "users_email_key" UNIQUE ("email");
CREATE TABLE "public"."categories" (...);
CREATE UNIQUE INDEX "idx_users_username_unique" ON "public"."users" ("username");
ALTER TYPE "public"."user_role" ADD VALUE 'moderator';
```

**不带 --unsafe-drop**：应输出警告并跳过 DROP 操作。

#### 场景 6：嵌套目录 diff

```bash
# 原始 nested（2 表）→ nested_target（带扩展）
./migra diff testdata/diff/nested/ testdata/diff/nested_target/
```

**预期输出**：
```sql
-- op: add_column risk:low
ALTER TABLE "public"."users" ADD COLUMN "email" varchar(100) DEFAULT 'unknown';
-- op: add_constraint risk:low
ALTER TABLE "public"."posts" ADD CONSTRAINT "posts_user_id_fkey" FOREIGN KEY ("user_id") REFERENCES "public"."users" ("id");
```

**验证点**：
- ✅ 嵌套子目录被正确扫描并合并
- ✅ 新增列和外键被正确检测

#### 场景 7：多 Schema diff

```bash
./migra diff --schema public --schema auth testdata/diff/multi_schema/v1/ testdata/diff/multi_schema/v2/
```

**预期输出**：
```sql
-- op: add_table risk:low
CREATE TABLE "public"."profiles" (...);

-- op: add_column risk:low
ALTER TABLE "auth"."roles" ADD COLUMN "description" text DEFAULT '';

-- op: add_column risk:low
ALTER TABLE "public"."users" ADD COLUMN "email" varchar(255);
```

**验证点**：
- ✅ 多 schema 正确加载和比较
- ✅ 跨 schema 外键（profiles.user_id → users.id）
- ✅ `--schema` 标志筛选生效

#### 场景 8：RENAME COLUMN 基础场景

```bash
./migra diff testdata/diff/rename_example/v1/schema.sql testdata/diff/rename_example/v2/schema.sql
```

**预期输出**：
```sql
-- op: rename_column risk:low
ALTER TABLE "public"."users" RENAME COLUMN "username" TO "login_name";
```

**验证点**：
- ✅ 检测到列重命名而非 DROP + ADD
- ✅ 输出 risk:low 标签
- ✅ SQL 语法正确

#### 场景 9：RENAME COLUMN 组合变更场景

```bash
./migra diff testdata/diff/rename_complex/v1/schema.sql testdata/diff/rename_complex/v2/schema.sql
```

**预期输出**：
```sql
-- op: add_column risk:low
ALTER TABLE "public"."users" ADD COLUMN "phone" varchar(20);
-- op: add_index risk:low
CREATE UNIQUE INDEX "idx_users_login_name" ON "public"."users" ("login_name");
-- op: rename_column risk:low
ALTER TABLE "public"."users" RENAME COLUMN "username" TO "login_name";
-- op: add_enum_label risk:low
ALTER TYPE "public"."user_role" ADD VALUE 'guest';
```

**验证点**：
- ✅ 重命名与其他变更（ADD/DROP）共存时正确检测
- ✅ 索引名随之更新（idx_users_username → idx_users_login_name）
- ✅ 不会产生误报的 DROP COLUMN + ADD COLUMN

#### 场景 10：边界条件验证

```bash
# 隐藏文件/目录应被跳过（不会导致错误）
./migra diff testdata/diff/edge/tables/ testdata/diff/edge/tables/

# 空 SQL 文件（纯注释）→ 无变更
./migra diff testdata/diff/edge/empty.sql testdata/diff/edge/empty.sql

# 非 .sql 文件被扫描器自动跳过（执行不会报错）
# 注意：SQLFileLoader 只加载 .sql 后缀文件
./migra diff testdata/diff/edge/no_extension_file testdata/diff/edge/no_extension_file
# 预期：由于无 .sql 后缀，registry 将无法匹配 Loader
# 但如果传入的是目录路径，则走 DirectoryLoader
```

#### 场景 9：unsafe-drop 安全机制验证

```bash
# 无 DROP 场景：正常输出
./migra diff testdata/example_source.sql testdata/example_target.sql

# 含 DROP 场景（v2→v3 有 DROP）
./migra diff testdata/diff/v2/schema.sql testdata/diff/v3/schema.sql
# 预期：输出警告但不生成 DROP SQL

# 含 DROP 场景 + --unsafe-drop
./migra diff --unsafe-drop testdata/diff/v2/schema.sql testdata/diff/v3/schema.sql
# 预期：包含 DROP 操作在 Post-deploy 阶段
```

**安全机制行为**：
| 标志 | 有 DROP 操作时 | 无 DROP 操作时 |
|---|---|---|
| (默认) | 警告：N 个危险操作，跳过，输出中不含 DROP | 正常输出全部 |
| `--unsafe-drop` | 无警告，DROP 操作包含在输出中 | 同默认 |

#### 场景 10：输出到文件

```bash
./migra diff -o /tmp/diff_output.sql testdata/example_source.sql testdata/example_target.sql
cat /tmp/diff_output.sql
```

**验证点**：
- ✅ 输出写入指定文件而非 stdout
- ✅ 文件内容与 stdout 输出一致

#### 场景 11：strict 模式验证

```bash
# 对比含非 DDL 语句的 SQL 文件
# 默认（非 strict）模式：跳过不识别的语句，输出警告到 stderr
./migra diff testdata/edge_cases.sql testdata/edge_cases.sql
# 预期：无变更（两个相同文件对比），但可能输出解析警告

# strict 模式测试需构造含语法错误的 SQL 文件
```

#### 场景 12：push dry-run 验证（仅预览）

```bash
# push 的 dry-run 模式：显示差异 SQL 但不执行
./migra push --dry-run testdata/example_source.sql postgres://localhost/testdb
# 注意：push 需要 target 为数据库连接串
# dry-run 仅预览，不会实际连接（如果数据库不可达会报错）
```

### 验证命令速查表

```bash
# 构建
make build

# 基本 diff
./migra diff testdata/example_source.sql testdata/example_target.sql

# JSON 格式
./migra diff --format json testdata/example_source.sql testdata/example_target.sql

# 目录 diff
./migra diff testdata/diff/v1/ testdata/diff/v2/

# 目录 vs 快照
./migra diff testdata/diff/v2/ testdata/diff/snapshot.sql

# 删除场景（安全模式）
./migra diff testdata/diff/v2/schema.sql testdata/diff/v3/schema.sql

# 删除场景（含 DROP）
./migra diff --unsafe-drop testdata/diff/v2/schema.sql testdata/diff/v3/schema.sql

# 嵌套目录
./migra diff testdata/diff/nested/ testdata/diff/nested_target/

# 多 schema
./migra diff --schema public --schema auth testdata/diff/multi_schema/v1/ testdata/diff/multi_schema/v2/

# RENAME COLUMN 基础
./migra diff testdata/diff/rename_example/v1/schema.sql testdata/diff/rename_example/v2/schema.sql

# RENAME COLUMN 组合变更
./migra diff testdata/diff/rename_complex/v1/schema.sql testdata/diff/rename_complex/v2/schema.sql

# 输出到文件
./migra diff -o /tmp/result.sql testdata/example_source.sql testdata/example_target.sql

# 版本信息
./migra --version

# 帮助
./migra diff --help
```

### 预期行为总结

| 测试数据组合 | 预期输出 | 关键验证 |
|---|---|---|
| `example_source.sql` → `example_target.sql` | ADD TABLE comments, ADD COLUMN age, ADD ENUM VALUE guest | 基础流水线、三阶段排序 |
| `diff/v1/` → `diff/v2/` | 同上（目录版本） | DirectoryLoader 多文件合并后一次性解析 |
| `diff/v2/` → `diff/snapshot.sql` | 无变更 | 目录加载 ≡ 单文件加载语义等价 |
| `diff/v2/schema.sql` → `diff/v3/schema.sql` (safe) | 仅非 DROP 操作 + 警告 | unsafe-drop 安全过滤 |
| `diff/v2/schema.sql` → `diff/v3/schema.sql` (unsafe) | DROP + ADD + ALTER 全量 | DROP 操作正确输出 |
| `diff/nested/` → `diff/nested_target/` | ADD COLUMN email, ADD CONSTRAINT fk | 嵌套子目录递归扫描 |
| `diff/multi_schema/v1/` → `diff/multi_schema/v2/` | ADD TABLE profiles, ADD COLUMN email/description | 多 schema 命名空间支持 |
| `diff/rename_example/v1/` → `diff/rename_example/v2/` | RENAME COLUMN username TO login_name | RENAME COLUMN 基础检测 |
| `diff/rename_complex/v1/` → `diff/rename_complex/v2/` | RENAME COLUMN + ADD COLUMN phone + ADD INDEX + ADD ENUM | 重命名与其他变更组合 |
| `diff/v3/schema.sql` → `diff/v4/schema.sql` | IDENTITY + COLLATE + FK CASCADE 变更 | v3→v4 多特性组合演进 |
| `diff/identity_example/v1/` → `diff/identity_example/v2/` | ALTER COLUMN id SET DATA TYPE → GENERATED ALWAYS AS IDENTITY | IDENTITY 列专项检测 |
| `diff/collate_example/v1/` → `diff/collate_example/v2/` | ADD COLUMN name COLLATE "en_US" | COLLATE 子句新增检测 |
| `diff/objects_example/v1/` → `diff/objects_example/v2/` | ADD EXTENSION + ADD SEQUENCE + ADD VIEW | 非表对象新增检测 |
| `diff/edge/tables/` → `diff/edge/tables/` | 无变更 | 隐藏文件被跳过（加载正常） |
