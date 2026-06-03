# MIGRA-Go

[![Go Report Card](https://goreportcard.com/badge/github.com/fred29910/migra-go)](https://goreportcard.com/report/github.com/fred29910/migra-go)
[![Test](https://github.com/fred29910/migra-go/actions/workflows/test.yml/badge.svg)](https://github.com/fred29910/migra-go/actions/workflows/test.yml)
[![Lint](https://github.com/fred29910/migra-go/actions/workflows/lint.yml/badge.svg)](https://github.com/fred29910/migra-go/actions/workflows/lint.yml)
[![License: MIT](https://img.shields.io/badge/License-MIT-blue.svg)](LICENSE)

一个用 Go 编写的 PostgreSQL Schema 差异比较工具，灵感来自 Python 版的 [migra](https://github.com/djrobstep/migra)。

## 功能特性

- 🔍 **多源比较**：支持 SQL 文件、目录、PostgreSQL 实例任意组合的双向对比。
- 🏗️ **结构化模型**：基于中间 SchemaModel 表示数据库结构，涵盖表、列、主键、索引、约束（外键/唯一/检查）、枚举类型、视图、序列和扩展。
- 📋 **SQL 生成与执行**：输出可执行的迁移 SQL，并支持 `push` 命令将变更直接应用到目标数据库。
- 🛡️ **交互式安全执行**：`push` 命令提供逐条确认、危险操作中断、事务保护与自动回滚。
- 🛡️ **安全保护**：DROP 操作默认拦截告警，可选 `--unsafe-drop` 放行。
- 🎯 **语义归一化**：同义类型名（如 `int4` → `integer`）自动映射，减少误报。
- 🚀 **DAG 拓扑排序**：基于 Kahn 算法对有向无环图进行排序，保证执行顺序正确且高效（外键依赖先创建、后删除）。
- ⚙️ **三阶段执行计划**：自动将操作分为 Pre-deploy（创建）、Deploy（修改）、Post-deploy（删除）三个阶段，覆盖全部 33 种操作。
- 🔧 **可扩展解析器**：基于 HandlerRegistry + 访问者模式的 OCP 设计，已支持 9 种 DDL（CREATE TABLE/ALTER TABLE/CREATE INDEX/CREATE ENUM/CREATE SCHEMA/RENAME/CREATE VIEW/CREATE SEQUENCE/CREATE EXTENSION），新增 DDL 只需实现 Handler + Mutation 并注册。
- 🧩 **丰富的差异检测**：支持表、列、类型、默认值、非空约束、索引内容、约束（主键/外键/唯一/检查）、视图、序列、扩展的全量对比，共 33 种差异操作。
- 📊 **多格式输出**：支持 SQL 和 JSON 两种输出格式。
- ⏱️ **超时控制**：支持 `--timeout` 参数控制 Schema 加载超时时间。
- 📁 **目录作为 Schema 来源**：支持递归扫描目录下所有 `.sql` 文件，合并为完整 schema 参与 diff，自动跳过隐藏文件/目录、支持嵌套子目录。
- 🔌 **多种连接方式**：支持连接字符串、环境变量、`pg_service.conf`、`.pgpass` 等 PostgreSQL 标准连接方式。

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

# 比较两个 SQL 文件
migra diff file_a.sql file_b.sql

# 比较两个目录（递归扫描 .sql 文件）
migra diff ./schemas/v1/ ./schemas/v2/

# 目录 vs 单文件
migra diff ./schemas/v1/ ./schemas/v2/snapshot_file.sql

# 目录 vs 数据库
migra diff ./schemas/v1/ postgres://localhost/db

# 指定 schema 和超时时间
migra diff --schema public --schema auth --timeout 2m file.sql postgres://localhost/db

# 输出 JSON 格式
migra diff --format json file_a.sql file_b.sql

# 启用危险 DROP 操作输出
migra diff --unsafe-drop file_a.sql file_b.sql

# 从配置文件读取源和目标（需设置 database.source / database.target）
migra diff
migra diff file.sql  # 目标从 database.url 读取
```

### push 命令

将 Schema 变更应用到目标数据库（需要两个参数）：

```bash
# 将 SQL 文件中的 Schema 变更应用到数据库
migra push file.sql postgres://localhost/db

# 比较两个数据库，将差异应用到目标库
migra push postgres://localhost/db1 postgres://localhost/db2

# 预览模式：显示 Diff SQL 但不执行
migra push --dry-run file.sql postgres://localhost/db

# 指定多个 Schema 进行推送
migra push --schema public --schema auth file.sql postgres://localhost/db

# 跳过危险操作确认（自动允许 DROP）
migra push --unsafe-drop file.sql postgres://localhost/db

# 跳过交互确认直接执行（不推荐，生产环境慎用）
migra push --execute file.sql postgres://localhost/db

# 跳过执行后校验
migra push --no-verify file.sql postgres://localhost/db

# 设置超时时间
migra push --timeout 2m file.sql postgres://localhost/db
```

### push 交互流程

`migra push` 默认进入交互模式，逐条确认后执行。使用 `--dry-run` 仅预览不执行，`--execute` 跳过确认直接执行。

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

交互选项说明：

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
| `--strict` | diff | 遇到不支持的语句时直接失败退出 | `false` |
| `--timeout` | diff, push | Schema 加载超时时间（如 `30s`, `2m`） | `30s` |
| `-o, --output` | diff | 输出到文件（默认输出到 stdout） | - |
| `--dry-run` | push | 显示 SQL 预览但不执行 | `false` |
| `--execute` | push | 跳过交互确认直接执行（不推荐） | `false` |
| `--no-verify` | push | 跳过执行后校验 | `false` |
| `-c, --config` | 全局 | 指定配置文件路径 | `~/.migra.yaml` 或 `./migra.yaml` |
| `-v, --verbose` | 全局 | 输出详细日志 | `false` |

### 配置文件

支持多层配置，优先级从高到低：命令行参数 > 环境变量 > 配置文件 > 默认值。

**配置文件位置**（自动发现）：
- `~/.migra.yaml`（用户级）
- `./migra.yaml`（项目级）

```bash
# 复制示例配置
cp examples/config.yaml ~/.migra.yaml
# 或
cp examples/config.yaml ./migra.yaml
```

**环境变量**（可选）：
使用 `MIGRA_` 前缀的环境变量（通过 viper.SetEnvPrefix 自动绑定）：

```bash
export MIGRA_DATABASE_URL="postgres://user:password@localhost:5432/dbname"
export MIGRA_DIFF_SCHEMAS="public,auth"
export MIGRA_DIFF_FORMAT="sql"
export MIGRA_DATABASE_SOURCE="postgres://localhost/db1"
export MIGRA_DATABASE_TARGET="postgres://localhost/db2"
```

详细配置说明请参考 [docs/configuration.md](docs/configuration.md)。

### diff 命令的三种参数模式

`diff` 命令支持灵活的传参方式：

| 参数数量 | 场景 | 说明 |
|----------|------|------|
| 2 个参数 | `migra diff A B` | 比较 A 与 B（最常见） |
| 1 个参数 | `migra diff A` | 比较 A 与配置文件中的 `database.url` |
| 0 个参数 | `migra diff` | 比较配置文件中 `database.source` 与 `database.target` |

### 数据库连接方式

MIGRA-Go 支持多种 PostgreSQL 连接方式：

**1. 连接字符串**
```bash
migra diff file.sql "postgres://user:password@localhost:5432/dbname?sslmode=disable"
```

**2. 标准环境变量**（pgx 自动支持）
```bash
export PGHOST=localhost
export PGPORT=5432
export PGUSER=myuser
export PGPASSWORD=mypassword
export PGDATABASE=mydb
migra diff file.sql "postgres://"
```

**3. pg_service.conf 服务名**
```bash
# ~/.pg_service.conf 中定义 [myservice]
migra diff file.sql "postgres://?service=myservice"
```

**4. .pgpass 密码文件**（pgx 自动读取 `~/.pgpass`）
```bash
# ~/.pgpass 内容：localhost:5432:mydb:myuser:mypassword
migra diff file.sql "postgres://myuser@localhost/mydb"
```

详见 [examples/](examples/) 目录和 [PostgreSQL 官方文档](https://www.postgresql.org/docs/current/libpq-envars.html)。

## 架构概览

### 流水线架构

整个工具的核心是一条清晰的五阶段流水线：

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

阶段说明：

1. **Source** — 通过 Loader 接口统一加载来源，支持三种来源类型：
   - **SQL 文件**（`SQLFileLoader`）：解析单个 `.sql` 文件
   - **目录**（`DirectoryLoader`）：递归扫描目录合并多个 `.sql` 文件
   - **数据库**（`DBLoader`）：从 PostgreSQL 实例内省 schema
   - 通过 Registry 策略模式自动匹配来源类型
2. **Parser** — 基于 pg_query_go 解析 AST，通过 Handler 访问者模式将 DDL 转换为 SchemaModel
3. **Normalize** — 语义归一化（如 `int4` → `integer`），减少同义表达导致的误报
4. **Diff** — 逐层比较两个 SchemaModel（表、列、索引、约束、枚举、视图、序列、扩展），生成 33 种 Operation 列表
5. **Plan** — 按三阶段分组（Pre-deploy/Deploy/Post-deploy），基于 Kahn 算法做 DAG 拓扑排序

### 项目结构

```
.
├── cmd/migra/              # CLI 入口（Cobra + Viper）
│   ├── main.go             # 根命令与配置初始化
│   ├── diff.go             # diff 子命令与参数解析
│   ├── diff_runner.go      # 差异计算流水线编排
│   ├── diff_runner_test.go # diff 运行器测试
│   ├── diff_test.go        # CLI 层测试
│   ├── push.go             # push 子命令定义与参数解析
│   ├── push_runner.go      # push 执行逻辑（交互确认、事务、回滚）
│   ├── push_test.go        # push 命令测试
│   ├── version_test.go     # 版本信息 CLI 测试
│   └── integration_test.go # 端到端集成测试
├── internal/
│   ├── app/                # 应用层服务（依赖注入编排）
│   │   ├── diff_service.go # 差异计算服务 + ComputeDiff 管线
│   │   └── diff_service_test.go
│   ├── model/              # 中间数据模型（Schema、Table、Column、View、Sequence、Extension）
│   │   ├── schema.go       # Schema / Namespace / EnumType
│   │   ├── table.go        # Table / PrimaryKey / Index / Constraint
│   │   ├── column.go       # Column 定义
│   │   ├── index_elem.go   # IndexElem（索引元素）
│   │   ├── object_key.go   # ObjectKey（对象统一标识 + 依赖追踪）
│   │   ├── schema_test.go  # Schema/Table 序列化测试
│   │   ├── golden_test.go  # Golden 文件测试
│   │   └── testdata/       # 模型层测试数据
│   ├── source/             # Schema 来源加载（Loader 抽象层）
│   │   ├── loader.go       # Loader 接口定义
│   │   ├── registry.go     # 来源注册表（策略模式路由）
│   │   ├── db_loader.go    # 数据库来源加载器
│   │   ├── sql_file_loader.go # SQL 文件来源加载器
│   │   ├── dir_loader.go    # 目录来源加载器（递归扫描 + schema 合并）
│   │   ├── dir_loader_test.go # 目录加载器测试
│   │   └── loader_test.go  # 加载器测试
│   ├── parser/             # SQL 解析（基于 pg_query_go，OCP 架构）
│   │   ├── parser.go       # 主解析器 + 异常恢复
│   │   ├── registry.go     # Handler 注册表（reflect.Type 路由）
│   │   ├── applier.go      # Mutation 应用器
│   │   ├── mutation.go     # SchemaMutation 接口与核心类型
│   │   ├── create_table_handler.go # CREATE TABLE 处理
│   │   ├── alter_table_handler.go  # ALTER TABLE 处理（ADD/DROP/ALTER COLUMN）
│   │   ├── index_handler.go        # CREATE/DROP INDEX 处理
│   │   ├── index_mutation.go       # 索引相关 Mutation 实现
│   │   ├── enum_handler.go         # CREATE TYPE AS ENUM 处理
│   │   ├── parserutil/             # 解析器辅助函数
│   │   │   └── util.go             # 类型映射、表达式解析、列解析
│   │   ├── handler_test.go         # Handler 单元测试
│   │   ├── index_handler_test.go   # 索引 Handler 测试
│   │   ├── index_mutation_test.go  # 索引 Mutation 测试
│   │   ├── mutation_test.go        # Mutation 测试
│   │   └── parser_test.go          # 解析器测试
│   ├── introspect/         # 数据库内省（读取 pg_catalog）
│   │   ├── introspect.go   # 主入口（表/约束/索引/枚举全量加载）
│   │   ├── tables.go       # 表与列信息加载
│   │   ├── constraints.go  # 约束加载（主键、外键、唯一、检查）
│   │   ├── indexes.go      # 索引信息加载
│   │   ├── enums.go        # 枚举类型加载
│   │   ├── tables_test.go  # 表加载测试
│   │   └── enums_test.go   # 枚举加载测试
│   ├── normalize/          # 语义归一化
│   │   ├── normalize.go    # 同义类型映射、表达式规范化
│   │   └── normalize_test.go
│   ├── diff/               # 差异比较引擎
│   │   ├── differ.go       # Differ 核心逻辑 + DiffEngine 接口
│   │   ├── context.go      # diffContext（单次 diff 的本地状态）
│   │   ├── operation.go    # 全部 Operation 类型定义（15+ 种）
│   │   ├── diff_tables.go  # 表级差异（列/索引/约束对比）
│   │   ├── diff_columns.go # 列级差异（类型/非空/默认值）
│   │   ├── differ_test.go  # 差异引擎测试
│   │   ├── diff_constraint_test.go # 约束差异测试
│   │   ├── diff_index_test.go     # 索引差异测试
│   │   └── operation_test.go      # Operation 测试
│   ├── plan/               # 执行计划与拓扑排序（DAG）
│   │   ├── plan.go         # 三阶段执行计划（PlanEngine 接口）
│   │   ├── dag.go          # Kahn 算法 DAG 构建与拓扑排序
│   │   ├── plan_test.go    # 计划测试
│   │   └── dag_test.go     # DAG 测试
│   ├── render/             # SQL / JSON 渲染器
│   │   ├── render.go       # 渲染器（SQLEngine 接口，15+ 操作渲染）
│   │   └── render_test.go  # 渲染测试
│   └── testutil/           # 测试工具集
│       └── testutil.go     # Schema 构建辅助函数
├── scripts/                # 辅助构建脚本
│   ├── setup.sh            # 开发环境初始化
│   └── release.sh          # 版本发布脚本
├── examples/               # 示例配置与环境变量
│   ├── config.yaml         # 配置文件示例
│   └── .env.example        # 环境变量示例
├── docs/                   # 使用手册、架构设计及评审纪要
│   ├── arch.md             # 架构设计文档
│   ├── configuration.md    # 配置文档
│   ├── DDL.md              # PostgreSQL DDL 功能支持矩阵
│   ├── bugs/               # 已知 Bug 跟踪
│   ├── plans/              # 实施计划文档
│   ├── reviews/            # 技术评审与代码审查报告
│   └── superpowers/        # 设计规格与评审纪要（superpowers 工作流）
├── testdata/               # 单元测试与集成测试用例
│   ├── example_source.sql  # 示例源 schema（单文件）
│   ├── example_target.sql  # 示例目标 schema（单文件）
│   ├── alter_operations.sql # ALTER TABLE 全操作集覆盖
│   ├── complex_ddl.sql     # 复合约束、高级类型、自定义枚举
│   ├── drop_scenarios.sql  # DROP 语义测试
│   ├── edge_cases.sql      # 边界 SQL 模式（继承表、分区表等）
│   └── diff/               # 目录型 diff 场景（DirectoryLoader 集成测试）
│       ├── v1/             # 源版本（users + posts + indexes + enum）
│       ├── v2/             # 目标版本（v1 + comments + age + guest）
│       ├── v3/             # 修改/删除场景（删列、删表、重命名索引）
│       ├── nested/         # 嵌套子目录结构
│       ├── nested_target/  # 嵌套目录的目标版本（扩展列+外键）
│       ├── multi_schema/   # 多 schema（public + auth）命名空间场景
│       ├── snapshot.sql    # v2 快照（单文件等价于 v2/ 目录）
│       └── edge/           # 边界情况（隐藏文件、非 SQL 文件、空目录）
├── Makefile                # 常用构建命令集合
└── .github/                # GitHub Actions CI/CD 工作流
    ├── workflows/
    │   ├── test.yml        # 单元测试（push/PR 触发）
    │   ├── lint.yml        # 代码规范检查（gofmt + vet + golangci-lint）
    │   └── release.yml     # 发版构建与 GitHub Release
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
make build    # 构建项目可执行文件
make test     # 运行单元与集成测试
make test-coverage # 运行测试并生成覆盖率报告
make lint     # 运行代码规范检查
make fmt      # 格式化 Go 代码
make vet      # 运行 go vet 静态检查
make ci       # 本地运行完整 CI 检查流程
```

### 技术栈

- **语言**：Go 1.26.2
- **CLI 框架**：[Cobra](https://github.com/spf13/cobra) + [Viper](https://github.com/spf13/viper)
- **数据库驱动**：[pgx v5](https://github.com/jackc/pgx)
- **SQL 解析**：[pg_query_go](https://github.com/lfittl/pg_query_go)
- **测试框架**：[testify](https://github.com/stretchr/testify)

## 代码规范

代码风格遵循 Go 标准实践：

- **格式化**：使用 `gofmt` 格式化，提交前运行 `make fmt`
- **静态检查**：启用 `go vet` 和 `golangci-lint`
- **测试覆盖**：核心逻辑要求单元测试覆盖，提交前运行 `make ci`

## 参与贡献

欢迎大家提交 Issue 和 Pull Request！参与前请先阅读：

- [贡献指南](./CONTRIBUTING.md)
- [行为准则](./CODE_OF_CONDUCT.md)

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
