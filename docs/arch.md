# MIGRA-Go 架构文档

> 最后更新：2026-05-14

## 概述

MIGRA-Go 是一个用 Go 编写的 PostgreSQL Schema 差异比较工具。它采用**五阶段流水线架构**，将 schema 加载、解析、归一化、差异比较、执行计划编排和渲染输出串联为一条清晰的数据处理管道。

核心设计原则：

- **接口驱动**：每个核心阶段都定义了 Go 接口，支持多种实现策略
- **OCP（开闭原则）**：新增 DDL 类型只需实现 Handler + Mutation 并注册，无需改动核心解析器
- **依赖注入**：通过 `RunnerDeps` 将核心依赖注入到应用服务层，便于测试
- **策略模式**：Source 加载层通过 Registry 自动匹配来源类型

---

## 1. 五阶段流水线

```
┌──────────┐   ┌──────────┐   ┌───────────┐   ┌─────────┐   ┌──────────┐
│  Source  │──▶│  Parser  │──▶│ Normalize │──▶│  Diff   │──▶│  Plan    │──▶ Render
│  (Loader)│   │ (AST →   │   │ (语义归一化)│   │ (Schema │   │ (DAG 排序│   (SQL/JSON)
│          │   │  Schema) │   │           │   │  A vs B)│   │  三阶段) │
└──────────┘   └──────────┘   └───────────┘   └─────────┘   └──────────┘
                                                              │
                                                              ▼
                                                         ┌──────────┐
                                                         │   Push   │
                                                         │ (交互执行 │
                                                         │ 事务+回滚)│
                                                         └──────────┘
```

### 阶段职责

| # | 阶段 | 包 | 输入 | 输出 |
|---|------|-----|------|------|
| 1 | Source | `internal/source/` | 路径/URL 字符串 | `*model.Schema` |
| 2 | Parser | `internal/parser/` | SQL 文本 | `*model.Schema` |
| 3 | Normalize | `internal/normalize/` | `*model.Schema` | 归一化的 `*model.Schema` |
| 4 | Diff | `internal/diff/` | `source, target *model.Schema` | `[]Operation` |
| 5 | Plan + Render | `internal/plan/` + `internal/render/` | `[]Operation` | SQL / JSON |

---

## 2. 分层架构

```
cmd/migra/          # CLI 入口（Cobra + Viper）— 参数解析、依赖注入
  └── internal/app/       # 应用层服务 — 流水线编排
        ├── source/       # Schema 来源加载 — Loader 接口 + Registry
        ├── parser/       # SQL 解析 — pg_query_go AST → SchemaMutation
        ├── model/        # 中间数据模型 — Schema / Table / Column / ...
        ├── normalize/    # 语义归一化 — 类型别名、表达式规范化
        ├── diff/         # 差异比较引擎
        ├── plan/         # 执行计划 — 三阶段 + DAG 拓扑排序
        ├── render/       # 渲染器 — SQL / JSON
        ├── introspect/   # 数据库内省 — 从 pg_catalog 读取 schema
        └── testutil/     # 测试工具
```

### 2.1 CLI 层 (`cmd/migra/`)

基于 **Cobra** + **Viper**，支持多级配置：命令行参数 > 环境变量 > 配置文件。

```
cmd/migra/
├── main.go               # 根命令、全局配置初始化
├── diff.go               # diff 子命令定义 + sourceRegistry 注册
├── diff_runner.go        # diff 参数解析 + 依赖注入工厂 (newDefaultDeps)
├── diff_test.go          # CLI 层测试
├── diff_runner_test.go   # diff 运行器测试
├── push.go               # push 子命令定义
├── push_runner.go        # push 交互执行逻辑（事务、确认、回滚、校验）
├── push_test.go          # push 命令测试
└── integration_test.go   # 端到端集成测试
```

**关键组件：**

- **sourceRegistry** (`diff.go:16-22`)：全局 Loader 注册表，注册顺序决定匹配优先级：
  ```
  DBLoader → DirectoryLoader → SQLFileLoader
  ```

- **依赖注入** (`diff_runner.go:86-98`)：`newDefaultDeps()` 返回 `app.RunnerDeps`，包含 `LoadSchema`、`Compute`、`Render` 三个函数，支持测试时替换。

- **diff 参数模式**：支持 0/1/2 个参数，通过 `parseDiffConfig` 统一处理。

- **push 交互流程**：
  1. Load source + target schema
  2. ComputeDiff → Diff Preview
  3. 逐条交互确认（y/n/a/s）
  4. 事务提交
  5. 执行后校验（re-diff 确认一致）

### 2.2 应用服务层 (`internal/app/`)

`diff_service.go` 是整个流水线的编排者。

```go
type DiffService interface {
    Run(ctx context.Context, cfg Config) (output string, warnings []string, err error)
}
```

**Run 方法流程：**

```
LoadSchema(source)  ──┐
                      ├──▶ NormalizeSchemas → ComputeDiff → RenderOutput
LoadSchema(target)  ──┘
```

`ComputeDiff` (行 146-163) 是核心管线：

```
NormalizeSchema → Differ.Diff → FilterDestructiveOps → BuildExecutionPlan
```

`BuildExecutionPlan` (行 119-143) 将 operations 按三阶段分组，每个阶段内部做 DAG 拓扑排序。

### 2.3 Source 层 (`internal/source/`)

采用 **Registry 策略模式**，通过 `Loader` 接口统一多种来源。

```go
type Loader interface {
    Match(source string) bool
    Load(ctx context.Context, source string, opt LoadOptions) (*model.Schema, []error, error)
}
```

**三种实现：**

| Loader | Match 条件 | Load 行为 |
|--------|-----------|-----------|
| `DBLoader` | `postgres://`, `postgresql://`, `pg://` 前缀 | 连接数据库 → `introspect.LoadFromDBWithConn` |
| `DirectoryLoader` | `os.Stat(path).IsDir()` 为 true | `filepath.WalkDir` 递归扫描 → 排序 → 合并 SQL → 一次性解析 |
| `SQLFileLoader` | `.sql` 后缀 或 `file://` 前缀 | `os.ReadFile` → `parser.ParseSQL` |

**Registry 分发流程**：

```
sourceRegistry.Load(source)
  │
  ├── DBLoader.Match(source)?        → DBLoader.Load()
  ├── DirectoryLoader.Match(source)?  → DirectoryLoader.Load()
  ├── SQLFileLoader.Match(source)?    → SQLFileLoader.Load()
  └── none match                     → error "no loader found"
```

**DirectoryLoader 合并策略**：

- 使用 `filepath.WalkDir` 递归遍历
- 跳过隐藏文件/目录（以 `.` 开头）
- 按文件路径排序后合并 SQL 文本（确定性输出）
- 合并后**一次性解析**，确保跨文件 DDL 依赖（如索引引用另一文件的表）正确解析
- 重复表名由 `CreateTableMutation.Apply` 检测并返回 `"already exists"` 错误
- 空目录返回空 schema + warning（无 fatal error）

### 2.4 模型层 (`internal/model/`)

数据模型采用**树状组合结构**：

```
Schema
 └── Namespaces map[string]*Namespace
      ├── Tables map[string]*Table
      │    ├── Columns []*Column
      │    │    ├── Name, DataType, IsNullable, DefaultExpr
      │    │    └── IsIdentity, IdentityKind
      │    ├── PrimaryKey *PrimaryKey
      │    │    └── Name, Columns []string
      │    ├── Indexes map[string]*Index
      │    │    └── Elements []IndexElem (column/expression, collation, opclass, ordering)
      │    └── Constraints map[string]*Constraint
      │         ├── Type (CHECK, FOREIGN KEY, UNIQUE, PRIMARY KEY)
      │         ├── Definition, Columns, RefSchema, RefTable, RefColumns
      │         └── HasInvalidRef, FKeyMatchType, FKeyUpdateRule, FKeyDeleteRule
      └── Types map[string]*EnumType
           └── Name, Labels []string
```

**关键类型：**

- **ObjectKey** (`object_key.go`)：统一的对象标识符，包含 `Schema`、`Name`、`Kind`(Table/Column/Index/Constraint/Type) 和 `Signature`，用于 diff 和 plan 阶段的依赖追踪。

### 2.5 解析器层 (`internal/parser/`)

基于 **pg_query_go** 生成 AST，采用 **Handler Registry 模式**实现 OCP。

**ParseSQL 流程：**

```
pg_query.Parse(sql) → AST
  │
  ├── for each statement in AST:
  │     ├── visitNode(statement)
  │     │     ├── handler = registry.Dispatch(nodeType)
  │     │     ├── mutations = handler.Handle(node)
  │     │     └── applier.Apply(mutations, schema)
  │     └── panic recovery (unsupported → collect error, continue)
  │
  └── return schema, errors
```

**Handler Registry** (`registry.go`)：

- 使用 `reflect.Type` 将 AST 节点类型路由到对应的 `Handler` 实现
- `DefaultRegistry()` 预注册了 4 种 DDL 的 Handler
- 新增 DDL 支持只需：实现 Handler → 实现 Mutation → 注册到 Registry

**已有的 Handler：**

| Handler | AST 类型 | DDL 语句 | 产生的 Mutation |
|---------|---------|----------|----------------|
| `CreateTableHandler` | `CreateStmt` | `CREATE TABLE` | `CreateTableMutation` |
| `AlterTableHandler` | `AlterTableStmt` | `ALTER TABLE ...` | `AddColumnMutation`, `DropColumnMutation`, `AlterColumnTypeMutation`, `SetNotNullMutation`, `DropNotNullMutation`, `SetDefaultMutation`, `DropDefaultMutation` |
| `IndexHandler` | `IndexStmt` | `CREATE INDEX` | `CreateIndexMutation` |
| `EnumHandler` | `CreateEnumStmt` | `CREATE TYPE ... AS ENUM` | `CreateEnumTypeMutation` |

**SchemaMutation 接口** (`mutation.go`)：

```go
type SchemaMutation interface {
    Kind() MutationKind
    Target() model.ObjectKey
    Apply(schema *model.Schema) error
}
```

**错误处理：**
- 每个 `visitNode` 有独立的 `defer recover()`，防止单个语句崩溃影响整个解析
- 错误聚合：`Applier.Apply` 收集所有 `MutationError`，不中断执行
- `Parser.Errors()` 返回所有非致命错误

### 2.6 语义归一化 (`internal/normalize/`)

归一化是**原地修改** schema 对象，减少同义表达导致的误报。

**归一化规则：**

| 规则 | 示例 | 目的 |
|------|------|------|
| 类型别名映射 | `int4` → `integer`, `bool` → `boolean` | 统一类型名 |
| 标识符规范化 | 去除引号，小写化 | 统一对象名 |
| 默认值表达式规范化 | 剥离括号/类型转换 | 统一表达式形式 |
| 约束定义规范化 | 小写化 + 合并空白 | 统一约束文本 |
| Index Elem 填充 | `Columns` → `Elements` 转换 | 统一 DB 内省与文件解析的表述 |

### 2.7 差异比较引擎 (`internal/diff/`)

**Differ** 逐层比较两个 `model.Schema`，生成 `Operation` 列表。

```go
type DiffEngine interface {
    Diff(source, target *model.Schema) ([]Operation, []string)
}
```

**比较层次：**

```
Diff
 └── diffSchemas (按 namespace 排序)
      ├── diffNamespace
      │    ├── diffTables
      │    │    ├── 检测新增/删除的表
      │    │    ├── diffTableColumns (比较列类型、可为空、默认值)
      │    │    ├── diffTableIndexes (比较索引定义、唯一性、表达式)
      │    │    └── diffTableConstraints (比较约束语义，避免 DROP+ADD 循环)
      │    └── diffTypes
      │         ├── 检测新增/删除的枚举
      │         └── diffEnumType (仅支持追加 label，不支持中间插入/删除)
      └── 删除检测（遍历 source 中不在 target 的 namespace/table/type）

```

**Operation 类型（16 种）：**

| Kind | 业务含义 | 是否破坏性 |
|------|---------|-----------|
| `add_table` | 添加表 | ❌ |
| `drop_table` | 删除表 | ✅ |
| `add_column` | 添加列 | ❌ |
| `drop_column` | 删除列 | ✅ |
| `alter_column_type` | 修改列类型 | ❌ |
| `set_not_null` / `drop_not_null` | 设置/取消非空 | ❌ |
| `set_default` / `drop_default` | 设置/取消默认值 | ❌ |
| `add_index` / `drop_index` | 添加/删除索引 | ❌ / ✅ |
| `add_constraint` / `drop_constraint` | 添加/删除约束 | ❌ / ✅ |
| `add_enum_type` / `drop_enum_type` | 添加/删除枚举类型 | ❌ / ✅ |
| `add_enum_label` | 枚举追加值 | ❌ |

**diffContext** (`context.go`) 跟踪单次 diff 的状态，收集 ops 和 warnings。

**约束比较策略**：`diffTableConstraints` 使用 `sameConstraintContent` 和 `sameConstraintSemantics` 两级比较，避免因 `pg_get_constraintdef` 输出格式差异导致的误报。

### 2.8 执行计划 (`internal/plan/`)

**三阶段模型：**

| 阶段 | 内容 | 说明 |
|------|------|------|
| `StagePreDeploy` | 创建操作 | 安全，可先执行 |
| `StageDeploy` | 修改操作 | 中间阶段 |
| `StagePostDeploy` | 删除操作 | 危险，需 `--unsafe-drop` |

**DAG 拓扑排序** (`dag.go`)：

- 基于 **Kahn 算法** 实现
- 根据 Operation.DependsOn() 构建依赖图
- 外键依赖：`AddTableOp` 通过 `DependsOn()` 声明对引用表的依赖
- 同一对象的多个操作（如 DropConstraint + AddConstraint）：优先让 Drop 先执行

```
Planner.Plan(ops)
  │
  ├── 按 kind 分配到三个阶段
  ├── 各阶段内部 TopoSort (Kahn)
  │     ├── BuildDAG: 构建有向图
  │     ├── 计算入度 → 零入度入队
  │     ├── 依次出队 → 减少依赖入度
  │     └── 检查循环依赖
  └── 合并为有序列表
```

### 2.9 渲染器 (`internal/render/`)

`Renderer` 将 `[]Operation` 转换为 SQL 或 JSON。

- **SQL 格式**：每个操作渲染为一条 SQL 语句，带 `-- op: <kind> risk:<level>` 注释
- **JSON 格式**：通过 `encoding/json` 序列化操作列表
- 列名/表名使用 `quoteIdentifier` 进行安全引用

### 2.10 数据库内省 (`internal/introspect/`)

从 PostgreSQL 实例读取 schema，映射到 `model.Schema`。

**加载流程：**

```
LoadFromDB(connStr)
  └── LoadFromDBWithConn(conn)
       └── for each schemaName:
            ├── loadTables()       → pg_catalog 查询表/列/默认值
            ├── loadConstraints()  → pg_catalog 查询主键、唯一、检查约束
            ├── loadForeignKeys()  → pg_catalog 查询外键约束
            ├── loadIndexes()      → pg_catalog 查询索引（表达式、WHERE 条件）
            └── loadEnumTypes()    → pg_catalog 查询枚举类型和 labels
```

每个加载子步骤独立按 schemaName + tableName 写入 `model.Namespace` 中的对应对象。

### 2.11 测试工具 (`internal/testutil/`)

提供 schema 构建和比较的辅助函数：

- `LoadSchemaFromJSON(t, path)` — 从 JSON 文件加载预期 schema
- `SaveSchemaToJSON(t, schema, path)` — 将 schema 输出为 JSON
- `CompareJSON(t, expected, actual)` — 忽略格式的 JSON 比较
- `GoldenFile(t, path, actual, updateFlag)` — Golden 文件测试

---

## 3. 依赖关系图

```
cmd/migra/main.go  (Cobra root + Viper config)
  │
  ├── cmd/migra/diff.go     → source.Registry (DBLoader, DirectoryLoader, SQLFileLoader)
  │                          → app.DiffService
  │
  ├── cmd/migra/push.go     → source.Registry
  │                          → app.ComputeDiff
  │                          → pgx 事务管理
  │
  └── cmd/migra/diff_runner.go → app.RunnerDeps (DI)
        │
        └── internal/app/diff_service.go
              │
              ├── internal/source/   (Registry → Loader → *model.Schema)
              │     ├── registry.go
              │     ├── db_loader.go → internal/introspect/  (→ pgx)
              │     ├── dir_loader.go → internal/source/sql_file_loader.go
              │     └── sql_file_loader.go → internal/parser/
              │
              ├── internal/normalize/  (CanonicalizeSchema → in-place normalization)
              │
              ├── internal/diff/   (Differ → []Operation)
              │     ├── differ.go
              │     ├── diff_tables.go, diff_columns.go
              │     ├── operation.go (16 kinds)
              │     └── context.go
              │
              ├── internal/plan/   (Planner → topo sort)
              │     ├── plan.go (3 stages)
              │     └── dag.go (Kahn algorithm)
              │
              └── internal/render/ (Renderer → SQL string / JSON)
```

---

## 4. 关键设计决策

### 4.1 为什么用 Registry 策略模式而不是 if-else 链？

- 新增来源类型（如未来的 `S3Loader`、`HTTPLoader`）只需实现 `Loader` 接口并注册
- CLI 层只需初始化 Registry，不关心具体 Loader 实现
- 匹配逻辑（`Match`）与加载逻辑（`Load`）解耦

### 4.2 为什么解析器用 Handler Registry 而不是 switch？

- 新增 DDL 类型（如 `CREATE VIEW`、`CREATE SEQUENCE`）只需：
  1. 实现 `Handler` 接口
  2. 实现 `SchemaMutation`
  3. 注册到 `DefaultRegistry()`
- 无需改动 `Parser` 核心代码，符合 OCP
- 每个 Handler 职责单一，可独立测试

### 4.3 为什么 Normalize 是单独的 pipeline 阶段？

- 数据库内省与 SQL 文件解析可能产生不同的类型别名（`int4` vs `integer`）
- 在 diff 之前统一归一化，确保比较的公平性
- 归一化逻辑集中管理，不影响 parser 和 loader

### 4.4 为什么用 DAG 做操作排序？

- 数据库操作有严格的依赖关系（如创建外键前必须先有被引用的表）
- 简单的按类型排序不够精确（外键依赖需要在同一个 stage 内精确排序）
- Kahn 算法保证在存在依赖时仍然能找到合法顺序，并能检测循环依赖

### 4.5 DirectoryLoader 的 Strict 模式

- 目录加载将所有 SQL 合并后**一次性解析**，parse error 使整个解析失败
- Strict 模式由调用方传入的 `opt.Strict` 控制，默认 `false`（与 `--strict` CLI 标志绑定）
- 合并后一次性解析意味着：单个文件失败 = 整个目录加载失败（不再支持逐文件容错）

---

## 5. 数据流示例

### 示例：`migra diff ./v1/ ./v2/`

```
1. parseDiffConfig("./v1/", "./v2/")
   → Config{Source: "./v1/", Target: "./v2/", ...}

2. diffService.Run(ctx, cfg)
   │
   ├── LoadSchema("./v1/")
   │     └── sourceRegistry.Load("./v1/")
   │           └── DirectoryLoader.Match("./v1/") → true
   │                 └── WalkDir → 解析 schema.sql → 合并 → *model.Schema (source)
   │
   ├── LoadSchema("./v2/")
   │     └── sourceRegistry.Load("./v2/")
   │           └── DirectoryLoader.Match("./v2/") → true
   │                 └── WalkDir → 解析 schema.sql → 合并 → *model.Schema (target)
   │
   ├── ComputeDiff(source, target, cfg)
   │     ├── NormalizeSchemas (规范化双方 schema)
   │     ├── Differ.Diff(source, target)
   │     │     ├── diffSchemas → diffTables
   │     │     │     ├── posts → 相同，跳过
   │     │     │     ├── users → ADD COLUMN age
   │     │     │     └── comments → 新增表 → AddTableOp
   │     │     └── diffTypes → user_role → ADD VALUE 'guest'
   │     ├── FilterDestructiveOps (无破坏性操作，全部保留)
   │     └── BuildExecutionPlan
   │           └── TopoSort → 确定执行顺序
   │
   └── RenderOutput(ops, "sql")
         └── Renderer.RenderAll → SQL 字符串
```

---

## 6. 扩展指南

### 新增一个 DDL 语句的支持

1. 在 `internal/parser/` 中创建 Handler（实现 `Handler` 接口）
2. 在 `internal/parser/` 中创建 Mutation（实现 `SchemaMutation` 接口）
3. 将 Handler 注册到 `DefaultRegistry()`
4. 在 `internal/render/` 中添加对应的渲染逻辑（如需要反向生成 SQL）
5. 在 `internal/diff/operation.go` 中添加对应的 Operation Kind（如需要差异检测）
6. 编写单元测试

### 新增一个 Source 类型

1. 在 `internal/source/` 中实现 `Loader` 接口（`Match` + `Load`）
2. 在 `cmd/migra/diff.go` 的 `sourceRegistry` 中注册
3. 编写单元测试 + 集成测试

---

## 7. 项目文件索引

```
├── cmd/migra/               CLI 入口
├── internal/
│   ├── app/                 应用服务（流水线编排）
│   ├── model/               数据模型
│   ├── source/              Schema 来源加载
│   ├── parser/              SQL 解析
│   ├── normalize/         语义归一化
│   ├── diff/                差异比较
│   ├── plan/                执行计划
│   ├── render/              渲染输出
│   ├── introspect/          数据库内省
│   └── testutil/            测试工具
├── docs/                    文档
├── testdata/                测试数据
└── .github/workflows/       CI/CD
```
