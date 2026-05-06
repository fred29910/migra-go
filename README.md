# MIGRA-Go

[![Go Report Card](https://goreportcard.com/badge/github.com/fred29910/migra-go)](https://goreportcard.com/report/github.com/fred29910/migra-go)
[![CI](https://github.com/fred29910/migra-go/actions/workflows/test.yml/badge.svg)](https://github.com/fred29910/migra-go/actions/workflows/test.yml)
[![Lint](https://github.com/fred29910/migra-go/actions/workflows/lint.yml/badge.svg)](https://github.com/fred29910/migra-go/actions/workflows/lint.yml)
[![License: MIT](https://img.shields.io/badge/License-MIT-blue.svg)](LICENSE)

一个用 Go 编写的 PostgreSQL Schema 差异比较工具，灵感来自 Python 版的 [migra](https://github.com/djrobstep/migra)。

## 功能特性

- 🔍 **双向 Diff**：比较 SQL 文件、PostgreSQL 实例或两者之间的差异。
- 🏗️ **结构化模型**：使用中间 SchemaModel 表示数据库结构。
- 📋 **SQL 生成**：输出可执行的迁移 SQL，支持枚举类型、索引、数据列和约束的精确变更。
- 🛡️ **安全保护**：标记破坏性操作，默认抛出告警提示，可选跳过危险变更。
- 🎯 **语义归一化**：减少因同义表达导致的误报。
- 🚀 **高性能**：基于 Kahn 算法实现的 DAG（有向无环图）拓扑排序，保证生成脚本的执行顺序确定且高效。

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
```

### 命令行参数

| 参数 | 说明 | 默认值 |
|------|------|--------|
| `-s, --schema` | 指定要比较的 Schema 列表（可指定多个） | `public` |
| `-f, --format` | 输出格式：`sql` 或 `json` | `sql` |
| `--unsafe-drop` | 允许输出危险的 DROP 操作 | `false` |
| `--strict` | 遇到不支持的语句时直接失败退出 | `false` |
| `--timeout` | Schema 加载超时时间（如 `30s`, `2m`） | `30s` |
| `-o, --output` | 输出到文件（默认输出到 stdout） | - |
| `-c, --config` | 指定配置文件路径 | `~/.migra.yaml` 或 `./migra.yaml` |
| `-v, --verbose` | 输出详细日志 | `false` |

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
```bash
# 复制并编辑环境变量文件
cp examples/.env.example .env
# 或直接使用环境变量
export DATABASE_URL="postgres://user:password@localhost:5432/dbname"
export MIGRA_SCHEMAS="public"
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

## 项目结构

```
.
├── cmd/migra/          # CLI 入口（Cobra + Viper）
├── internal/
│   ├── model/          # 中间数据模型（Schema、Table、Column 等）
│   ├── parser/         # SQL 解析（基于 pg_query_go）
│   ├── introspect/     # 数据库内省（读取 pg_catalog）
│   ├── normalize/      # 语义归一化
│   ├── diff/           # 差异比较引擎（包含操作收集与警告反馈）
│   ├── plan/           # 执行计划与拓扑排序（DAG）
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

- **语言**：Go 1.24+
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
