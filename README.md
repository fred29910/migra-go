# MIGRA-Go

[![Go Report Card](https://goreportcard.com/badge/github.com/fred29910/migra-go)](https://goreportcard.com/report/github.com/fred29910/migra-go)
[![CI](https://github.com/fred29910/migra-go/actions/workflows/test.yml/badge.svg)](https://github.com/fred29910/migra-go/actions/workflows/test.yml)
[![Lint](https://github.com/fred29910/migra-go/actions/workflows/lint.yml/badge.svg)](https://github.com/fred29910/migra-go/actions/workflows/lint.yml)
[![License: MIT](https://img.shields.io/badge/License-MIT-blue.svg)](LICENSE)

一个用 Go 编写的 PostgreSQL Schema 差异比较工具，灵感来自 Python 版的 [migra](https://github.com/djrobstep/migra)。

## 功能特性

- 🔍 **双向 Diff**：比较 SQL 文件、PostgreSQL 实例或两者之间的差异。
- 🏗️ **结构化模型**：使用中间 SchemaModel 表示数据库结构，支持表、列、主键、索引、枚举类型和约束。
- 📋 **SQL 生成与执行**：输出可执行的迁移 SQL，并支持 `push` 命令将变更直接应用到目标数据库。
- 🛡️ **交互式安全执行**：`push` 命令提供逐条确认、危险操作中断、事务保护与自动回滚。
- 🛡️ **安全保护**：标记破坏性操作（DROP），默认抛出告警提示，可选跳过危险变更（`--unsafe-drop`）。
- 🎯 **语义归一化**：减少因同义表达导致的误报。
- 🚀 **高性能**：基于 Kahn 算法实现的 DAG（有向无环图）拓扑排序，保证生成脚本的执行顺序确定且高效。
- ⚙️ **三阶段执行计划**：自动将操作分为 pre-deploy（创建）、deploy（修改）、post-deploy（删除）三个阶段。
- 🔧 **可扩展解析器**：基于 HandlerRegistry + 访问者模式的 OCP 设计，新增 DDL 类型只需实现 Handler + Mutation 并注册。
- 📊 **多格式输出**：支持 SQL 和 JSON 两种输出格式。
- ⏱️ **超时控制**：支持 `--timeout` 参数控制 Schema 加载超时时间。

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

# 指定 schema 和超时时间
migra diff --schema public --schema auth --timeout 2m file.sql postgres://localhost/db

# 输出 JSON 格式
migra diff --format json file_a.sql file_b.sql

# 启用危险 DROP 操作输出
migra diff --unsafe-drop file_a.sql file_b.sql

# 将 schema 变更应用到目标数据库（交互式确认）
migra push file.sql postgres://localhost/db

# 比较两个数据库，将差异应用到目标库
migra push postgres://localhost/db1 postgres://localhost/db2

# 预览模式（默认）：显示 diff SQL 但不执行
migra push --dry-run file.sql postgres://localhost/db

# 指定多个 schema 进行推送
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

`migra push` 默认以 dry-run 模式运行，仅预览 SQL 变更。添加 `--execute` 后进入交互执行模式：

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
| `--dry-run` | push | 显示 SQL 预览但不执行 | `true` |
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
```

详细配置说明请参考 [docs/configuration.md](docs/configuration.md)。

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

### 数据流

```
┌─────────────┐     ┌──────────────┐     ┌──────────────┐     ┌──────────────┐
│  SQL / DB   │────▶│   Parser     │────▶│   Diff       │────▶│   Renderer   │
│  (Source &  │     │  (AST →      │     │  (Schema A   │     │  (Operations │
│   Target)   │     │   SchemaModel)│     │   vs B →     │     │   → SQL/JSON)│
└─────────────┘     └──────────────┘     │   DiffOps)   │     └──────┬───────┘
                                         └──────┬───────┘            │
                                                │           ┌────────▼────────┐
                                         ┌──────▼───────┐   │    Push        │
                                         │   Planner    │──▶│ (交互确认执行  │
                                         │ (DAG 拓扑排序│   │  事务+回滚)    │
                                         │  三阶段执行)  │   └─────────────────┘
                                         └──────────────┘
```

### 项目结构

```
.
├── cmd/migra/          # CLI 入口（Cobra + Viper）
│   ├── main.go         # 根命令与配置初始化
│   ├── diff.go         # diff 子命令与参数解析
│   ├── diff_runner.go  # 差异计算流水线编排
│   ├── diff_test.go    # CLI 层测试
│   ├── push.go         # push 子命令定义与参数解析
│   ├── push_runner.go  # push 执行逻辑（交互确认、事务、回滚）
│   └── push_test.go    # push 命令测试
├── internal/
│   ├── app/            # 应用层服务（依赖注入编排）
│   │   └── diff_service.go
│   ├── model/          # 中间数据模型（Schema、Table、Column 等）
│   │   ├── schema.go
│   │   ├── table.go
│   │   ├── column.go
│   │   ├── index_elem.go
│   │   └── object_key.go
│   ├── parser/         # SQL 解析（基于 pg_query_go，OCP 架构）
│   │   ├── parser.go           # 主解析器 + 异常恢复
│   │   ├── registry.go         # Handler 注册表（reflect.Type 路由）
│   │   ├── applier.go          # Mutation 应用器
│   │   ├── mutation.go         # SchemaMutation 接口与核心类型
│   │   ├── create_table_handler.go
│   │   ├── alter_table_handler.go
│   │   ├── index_handler.go    # 完整索引支持（IndexElem）
│   │   ├── enum_handler.go
│   │   └── parserutil/         # 辅助函数包
│   ├── introspect/     # 数据库内省（读取 pg_catalog）
│   ├── normalize/      # 语义归一化
│   ├── diff/           # 差异比较引擎
│   │   ├── differ.go          # Differ 核心逻辑
│   │   ├── operation.go       # Operation 类型定义
│   │   ├── diff_tables.go     # 表级差异
│   │   ├── diff_columns.go    # 列级差异
│   │   └── diff_index.go      # 索引差异
│   ├── plan/           # 执行计划与拓扑排序（DAG）
│   │   ├── plan.go            # 三阶段执行计划
│   │   └── dag.go             # DAG 构建与排序
│   ├── render/         # SQL / JSON 渲染器
│   └── testutil/       # 测试工具集
├── scripts/            # 辅助构建脚本
├── examples/           # 示例配置与环境变量
├── docs/               # 使用手册、架构设计及评审纪要
├── testdata/           # 单元测试与集成测试用例
├── Makefile            # 常用构建命令集合
└── .github/            # GitHub Actions CI/CD 工作流
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
make lint     # 运行代码规范检查
make fmt      # 格式化 Go 代码
make vet      # 运行 go vet 静态检查
make ci       # 本地运行完整 CI 检查流程
```

### 技术栈

- **语言**：Go 1.26+
- **CLI 框架**：[Cobra](https://github.com/spf13/cobra) + [Viper](https://github.com/spf13/viper)
- **数据库驱动**：[pgx v5](https://github.com/jackc/pgx)
- **SQL 解析**：[pg_query_go](https://github.com/lfittl/pg_query_go)
- **测试框架**：[testify](https://github.com/stretchr/testify)

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