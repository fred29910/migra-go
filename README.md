# MIGRA-Go

[![Go Report Card](https://goreportcard.com/badge/github.com/fred29910/migra-go)](https://goreportcard.com/report/github.com/fred29910/migra-go)
[![Test](https://github.com/fred29910/migra-go/actions/workflows/test.yml/badge.svg)](https://github.com/fred29910/migra-go/actions/workflows/test.yml)
[![Lint](https://github.com/fred29910/migra-go/actions/workflows/lint.yml/badge.svg)](https://github.com/fred29910/migra-go/actions/workflows/lint.yml)
[![License: MIT](https://img.shields.io/badge/License-MIT-blue.svg)](LICENSE)

一个用 Go 编写的 PostgreSQL Schema 差异比较工具，灵感来自 Python 版的 [migra](https://github.com/djrobstep/migra)。

## 目录

- [功能特性](#功能特性)
- [快速开始](#快速开始)
- [架构概览](#架构概览)
- [项目结构](#项目结构)
- [开发指南](#开发指南)
- [参与贡献](#参与贡献)
- [变更日志](#变更日志)
- [许可证](#许可证)

## 功能特性

- 🔍 **多源比较**：支持 SQL 文件、目录、PostgreSQL 实例任意组合的双向对比
- 🏗️ **结构化模型**：基于中间 SchemaModel 表示数据库结构，涵盖表、列、主键、索引、约束（外键/唯一/检查）、枚举类型、视图、序列和扩展
- 📋 **SQL 生成与执行**：输出可执行的迁移 SQL，并支持 `push` 命令将变更直接应用到目标数据库
- 🛡️ **交互式安全执行**：`push` 命令提供逐条确认、危险操作中断、事务保护与自动回滚
- 🛡️ **安全保护**：DROP 操作默认拦截告警，可选 `--unsafe-drop` 放行
- 🎯 **语义归一化**：同义类型名（如 `int4` → `integer`）自动映射，减少误报
- 🚀 **DAG 拓扑排序**：基于 Kahn 算法对有向无环图进行排序，保证执行顺序正确且高效
- ⚙️ **三阶段执行计划**：自动将操作分为 Pre-deploy（创建）、Deploy（修改）、Post-deploy（删除）三个阶段，覆盖全部 33 种操作
- 🔧 **可扩展解析器**：基于 HandlerRegistry + 访问者模式的 OCP 设计，已支持 9 种 DDL，新增 DDL 只需实现 Handler + Mutation 并注册
- 🧩 **丰富的差异检测**：支持表、列、类型、默认值、非空约束、索引内容、约束、视图、序列、扩展的全量对比，共 33 种差异操作
- 📊 **多格式输出**：支持 SQL 和 JSON 两种输出格式
- ⏱️ **超时控制**：支持 `--timeout` 参数控制 Schema 加载超时时间
- 📁 **目录作为 Schema 来源**：支持递归扫描目录下所有 `.sql` 文件，合并为完整 schema 参与 diff
- 🔌 **多种连接方式**：支持连接字符串、环境变量、`pg_service.conf`、`.pgpass` 等 PostgreSQL 标准连接方式
- 🏷️ **IDENTITY 列支持**：完整支持 `GENERATED ALWAYS/BY DEFAULT AS IDENTITY` 的解析、内省、Diff 和渲染
- 🔤 **排序规则支持**：支持列级 COLLATE 的解析、内省和差异检测
- 🗂️ **多 Schema 支持**：通过 `--schema` 指定多个 schema，支持跨 schema 外键引用
- 🔒 **严格模式**：`--strict` 控制在遇到不支持的语句时是直接失败还是跳过并警告

## 快速开始

### 安装

```bash
# 从源码构建
git clone https://github.com/fred29910/migra-go.git
cd migra-go
make build

# 或直接下载二进制文件（Release）
# https://github.com/fred29910/migra-go/releases
```

### 基本用法

```bash
# 比较 SQL 文件和数据库（支持 pg:// 等短连接格式）
migra diff file.sql postgres://user:pass@localhost/dbname
migra diff file.sql pg://localhost/dbname

# 比较两个数据库
migra diff postgres://localhost/db1 postgres://localhost/db2

# 比较两个 SQL 文件 / 目录
migra diff file_a.sql file_b.sql
migra diff ./schemas/v1/ ./schemas/v2/

# 指定 schema 和超时时间
migra diff --schema public --schema auth --timeout 2m file.sql postgres://localhost/db

# 输出 JSON 格式 / 启用危险 DROP 操作
migra diff --format json file_a.sql file_b.sql
migra diff --unsafe-drop file_a.sql file_b.sql

# 从配置文件读取源和目标
migra diff
migra diff file.sql  # 目标从 database.url 读取
```

### push 命令

将 Schema 变更应用到目标数据库：

```bash
migra push file.sql postgres://localhost/db
migra push postgres://localhost/db1 postgres://localhost/db2

# 预览模式 / 跳过确认 / 跳过校验
migra push --dry-run file.sql postgres://localhost/db
migra push --execute file.sql postgres://localhost/db
migra push --no-verify file.sql postgres://localhost/db

# 指定多个 Schema / 允许危险操作 / 设置超时
migra push --schema public --schema auth file.sql postgres://localhost/db
migra push --unsafe-drop file.sql postgres://localhost/db
migra push --timeout 2m file.sql postgres://localhost/db
```

### push 交互流程

`migra push` 默认进入交互模式，逐条确认后执行。

```
$ migra push --execute file.sql postgres://localhost/db

=== Diff Preview ===
1:
CREATE TABLE users (
    id SERIAL PRIMARY KEY,
    ...
);

2:
CREATE INDEX idx_users_username ON users(username);

Execute SQL #1? (y=yes, n=no, a=apply all, s=skip): y
SQL #1 executed
Execute SQL #2? (y=yes, n=no, a=apply all, s=skip): a
SQL #2 executed

All SQL executed successfully, transaction committed

=== Post-execution validation ===
Validation passed: target schema matches expected state
```

交互选项：

| 按键 | 说明 |
|------|------|
| `y` | 执行当前 SQL |
| `n` | 取消并回滚事务 |
| `a` | 自动执行剩余所有 SQL（遇到危险操作会再次询问） |
| `s` | 跳过当前 SQL，继续下一条 |

安全机制：

- **事务保护**：所有 SQL 在事务中执行，任意一条失败自动回滚
- **危险操作检测**：DROP 类操作需额外确认，除非使用 `--unsafe-drop`
- **非事务性 DDL 拦截**：`CREATE INDEX CONCURRENTLY` 等操作会被拦截并提示单独执行
- **执行后校验**：提交后自动重新 diff，确认目标库与预期一致
- **信号处理**：Ctrl+C 触发事务回滚，安全退出

### 命令行参数

| 参数 | 命令 | 说明 | 默认值 |
|------|------|------|--------|
| `-s, --schema` | diff, push | 指定要比较的 Schema 列表（可指定多个） | `public` |
| `-f, --format` | diff | 输出格式：`sql` 或 `json` | `sql` |
| `--unsafe-drop` | diff, push | 允许输出/执行危险的 DROP 操作 | `false` |
| `--strict` | diff | 遇到不支持的语句时直接失败退出（默认跳过并警告） | `false` |
| `--timeout` | diff, push | Schema 加载超时时间（如 `30s`, `2m`） | `30s` |
| `-o, --output` | diff | 输出到文件（默认输出到 stdout） | - |
| `--dry-run` | push | 显示 SQL 预览但不执行 | `false` |
| `--execute` | push | 跳过交互确认直接执行（生产环境慎用） | `false` |
| `--no-verify` | push | 跳过执行后校验 | `false` |
| `-c, --config` | 全局 | 指定配置文件路径 | `~/.migra.yaml` 或 `./migra.yaml` |
| `-v, --verbose` | 全局 | 输出详细日志 | `false` |

### 配置文件

支持多层配置，优先级从高到低：命令行参数 > 环境变量 > 配置文件 > 默认值。

```bash
# 配置文件位置（自动发现）
# ~/.migra.yaml（用户级）
# ./migra.yaml（项目级）

# 环境变量（MIGRA_ 前缀）
export MIGRA_DATABASE_URL="postgres://user:password@localhost:5432/dbname"
export MIGRA_DIFF_SCHEMAS="public,auth"
export MIGRA_DATABASE_SOURCE="postgres://localhost/db1"
export MIGRA_DATABASE_TARGET="postgres://localhost/db2"

# .env 文件（通过 godotenv 自动加载）
```

详细配置说明请参考 [docs/configuration.md](docs/configuration.md)。

### diff 命令的三种参数模式

| 参数数量 | 场景 | 说明 |
|----------|------|------|
| 2 个参数 | `migra diff A B` | 比较 A 与 B（最常见） |
| 1 个参数 | `migra diff A` | 比较 A 与配置文件中的 `database.url` |
| 0 个参数 | `migra diff` | 比较配置文件中 `database.source` 与 `database.target` |

### 数据库连接方式

支持多种 PostgreSQL 连接方式：

```bash
# 1. 连接字符串
migra diff file.sql "postgres://user:password@localhost:5432/dbname?sslmode=disable"

# 2. 标准环境变量（pgx 自动支持）
export PGHOST=localhost PGPORT=5432 PGUSER=myuser PGPASSWORD=mypassword PGDATABASE=mydb
migra diff file.sql "postgres://"

# 3. pg_service.conf 服务名
migra diff file.sql "postgres://?service=myservice"

# 4. .pgpass 密码文件（pgx 自动读取 ~/.pgpass）
migra diff file.sql "postgres://myuser@localhost/mydb"
```

## 架构概览

### 流水线架构

```mermaid
flowchart LR
    subgraph Pipeline
        A["🔍 Source<br/>SQL文件 / 目录 / 数据库"] --> B["📝 Parser<br/>pg_query_go AST → SchemaModel"]
        B --> C["🎯 Normalize<br/>语义归一化 int4→integer"]
        C --> D["⚖️ Diff<br/>Schema A vs Schema B"]
        D --> E["📋 Plan<br/>DAG 拓扑排序 + 三阶段"]
        E --> F["🖨️ Render<br/>SQL / JSON 输出"]
    end

    F --> G["🚀 Push<br/>交互执行 / 事务保护 / 自动回滚"]
```

### 三阶段执行计划

```mermaid
flowchart LR
    subgraph Stages
        P["Pre-deploy<br/>CREATE 新对象"] --> D["Deploy<br/>ALTER 现有对象"]
        D --> Q["Post-deploy<br/>DROP 废弃对象 ⚠️"]
    end
```

### 支持的差异操作（33 种）

```mermaid
mindmap
  root((Diff 操作))
    表操作
      add_table
      drop_table
    列操作
      add_column
      drop_column
      alter_column_type
      set_not_null
      drop_not_null
      set_default
      drop_default
      rename_column
      add_identity
      set_identity
      drop_identity
      alter_column_collation
    约束操作
      add_constraint
      drop_constraint
    索引操作
      add_index
      drop_index
    枚举操作
      add_enum_type
      drop_enum_type
      add_enum_label
    视图操作
      create_view
      drop_view
      replace_view
      create_materialized_view
      drop_materialized_view
    模式操作
      create_schema
      drop_schema
    序列操作
      create_sequence
      drop_sequence
      alter_sequence
    扩展操作
      create_extension
      drop_extension
      alter_extension_update
```

### 可扩展解析器架构

```mermaid
flowchart TD
    SQL["SQL DDL 语句"] --> Parser["pg_query_go<br/>解析 AST"]
    Parser --> Registry["HandlerRegistry<br/>reflect.Type 路由"]
    Registry --> H1["CreateTableHandler"]
    Registry --> H2["AlterTableHandler"]
    Registry --> H3["IndexHandler"]
    Registry --> H4["EnumHandler"]
    Registry --> H5["SchemaHandler"]
    Registry --> H6["ViewHandler"]
    Registry --> H7["SequenceHandler"]
    Registry --> H8["ExtensionHandler"]
    H1 --> M1["CreateTableMutation"]
    H2 --> M2["AddColumnMutation / DropColumnMutation / ..."]
    H3 --> M3["CreateIndexMutation"]
    H4 --> M4["CreateEnumTypeMutation"]
    H5 --> M5["CreateSchemaMutation"]
    H6 --> M6["CreateViewMutation"]
    H7 --> M7["CreateSequenceMutation"]
    H8 --> M8["CreateExtensionMutation"]
    M1 --> App["MutationApplier<br/>→ SchemaModel"]
    M2 --> App
    M3 --> App
    M4 --> App
    M5 --> App
    M6 --> App
    M7 --> App
    M8 --> App
```

## 项目结构

```mermaid
graph TD
    CMD["cmd/migra/<br/>CLI 入口 (Cobra + Viper)"]
    subgraph Internal
        APP["app/<br/>应用层服务 + 依赖注入"]
        MODEL["model/<br/>中间数据模型<br/>Schema / Table / Column / View / Sequence"]
        SOURCE["source/<br/>Schema 来源加载<br/>Loader 接口 + 策略模式"]
        PARSER["parser/<br/>SQL 解析 (pg_query_go)<br/>OCP Handler 架构"]
        INTR["introspect/<br/>数据库内省<br/>pg_catalog 全量加载"]
        NORM["normalize/<br/>语义归一化<br/>同义类型映射"]
        DIFF["diff/<br/>差异比较引擎<br/>33 种 Operation"]
        PLAN["plan/<br/>执行计划<br/>DAG 拓扑排序 + 三阶段"]
        RENDER["render/<br/>SQL / JSON 渲染器"]
    end
    TESTDATA["testdata/<br/>单元测试 + 集成测试用例"]
    DOCS["docs/<br/>架构设计 / 配置 / DDL 矩阵"]
    EXAMPLES["examples/<br/>配置示例 + .env 模板"]

    CMD --> APP
    APP --> MODEL
    APP --> SOURCE
    APP --> NORM
    APP --> DIFF
    APP --> PLAN
    APP --> RENDER
    SOURCE --> PARSER
    SOURCE --> INTR
    PARSER --> MODEL
    INTR --> MODEL
    DIFF --> MODEL
    PLAN --> DIFF
    RENDER --> PLAN
```

### 目录文件明细

```
.
├── cmd/migra/              # CLI 入口（Cobra + Viper）
│   ├── main.go             # 根命令与配置初始化
│   ├── diff.go             # diff 子命令
│   ├── diff_runner.go      # 差异计算流水线编排
│   ├── push.go             # push 子命令
│   ├── push_runner.go      # push 执行逻辑（交互确认、事务、回滚）
│   └── *_test.go           # 各层测试
├── internal/
│   ├── app/                # 应用层服务（依赖注入编排）
│   ├── model/              # 中间数据模型（Schema、Table、Column 等）
│   ├── source/             # Schema 来源加载（Loader 抽象层 + 策略模式）
│   ├── parser/             # SQL 解析（pg_query_go + OCP Handler 架构）
│   │   └── parserutil/     # 解析器辅助函数
│   ├── introspect/         # 数据库内省（pg_catalog 全量加载）
│   ├── normalize/          # 语义归一化（同义类型映射）
│   ├── diff/               # 差异比较引擎（33 种 Operation）
│   ├── plan/               # 执行计划与拓扑排序（DAG）
│   ├── render/             # SQL / JSON 渲染器
│   └── testutil/           # 测试工具集
├── testdata/               # 单元测试与集成测试用例
├── docs/                   # 架构设计、配置文档、DDL 矩阵
├── examples/               # 配置与环境变量示例
├── scripts/                # 开发环境初始化与发布脚本
├── Makefile                # 常用构建命令集合
└── .github/workflows/      # GitHub Actions CI/CD（test / lint / release）
```

## 开发指南

### 环境搭建

```bash
# 使用初始化脚本
./scripts/setup.sh

# 或手动设置
make build
make test
```

### 常用命令

```bash
make build        # 构建项目可执行文件
make test         # 运行单元与集成测试
make test-coverage # 运行测试并生成覆盖率报告
make lint         # 运行代码规范检查
make fmt          # 格式化 Go 代码
make vet          # 运行 go vet 静态检查
make ci           # 本地运行完整 CI 检查流程
```

### 技术栈

| 组件 | 技术选型 |
|------|----------|
| 语言 | Go 1.26.2 |
| CLI 框架 | [Cobra](https://github.com/spf13/cobra) + [Viper](https://github.com/spf13/viper) |
| 数据库驱动 | [pgx v5](https://github.com/jackc/pgx) |
| SQL 解析 | [pg_query_go](https://github.com/lfittl/pg_query_go) |
| 测试框架 | [testify](https://github.com/stretchr/testify) |

### 代码规范

- **格式化**：使用 `gofmt`，提交前运行 `make fmt`
- **静态检查**：启用 `go vet` 和 `golangci-lint`
- **测试覆盖**：核心逻辑要求单元测试覆盖，提交前运行 `make ci`

## 参与贡献

欢迎大家提交 Issue 和 Pull Request！

提交 Pull Request 前，请确保：

- ✅ 运行 `make ci` 且通过所有检查
- ✅ 补充了必要的单元测试或集成测试
- ✅ 更新了相关的 Markdown 文档

## 变更日志

查看 [CHANGELOG.md](CHANGELOG.md) 了解详细的版本迭代与变更历史。

## 许可证

本项目采用 [MIT License](LICENSE) 开源协议。

---

**注意**：本项目仍在积极迭代中，部分 API 和内部实现可能随时进行演进与优化。
