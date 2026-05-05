# MIGRA-Go

[![Go Report Card](https://goreportcard.com/badge/github.com/fred29910/migra-go)](https://goreportcard.com/report/github.com/fred29910/migra-go)
[![CI](https://github.com/fred29910/migra-go/actions/workflows/test.yml/badge.svg)](https://github.com/fred29910/migra-go/actions/workflows/test.yml)
[![Lint](https://github.com/fred29910/migra-go/actions/workflows/lint.yml/badge.svg)](https://github.com/fred29910/migra-go/actions/workflows/lint.yml)
[![License: MIT](https://img.shields.io/badge/License-MIT-blue.svg)](LICENSE)

一个用 Go 编写的 PostgreSQL schema 差异比较工具，灵感来自 Python 版的 [migra](https://github.com/djrobstep/migra)。

## 功能特性

- 🔍 **双向 Diff**：比较 SQL 文件、PostgreSQL 实例或两者之间的差异
- 🏗️ **结构化模型**：使用中间 SchemaModel 表示数据库结构
- 📋 **SQL 生成**：输出可执行的迁移 SQL
- 🛡️ **安全保护**：标记破坏性操作，可选跳过危险变更
- 🎯 **语义归一化**：减少因同义表达导致的误报

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
# 比较 SQL 文件和数据库
migra diff file.sql postgres://user:pass@localhost/dbname

# 比较两个数据库
migra diff postgres://localhost/db1 postgres://localhost/db2

# 比较两个 SQL 文件
migra diff file_a.sql file_b.sql
```

### 命令行参数

| 参数 | 说明 |
|------|------|
| `-s, --schema` | 指定要比较的 schema（可多个） |
| `-f, --format` | 输出格式：sql 或 json |
| `--unsafe-drop` | 允许输出危险的 DROP 操作 |
| `--strict` | 遇到不支持的语句时失败 |
| `-o, --output` | 输出到文件（默认：stdout） |
| `-c, --config` | 指定配置文件路径 |
| `-v, --verbose` | 详细输出 |

### 配置文件

支持配置文件 `~/.migra.yaml` 或 `./migra.yaml`，示例：

```bash
# 复制示例配置
cp examples/config.yaml ~/.migra.yaml
# 或使用环境变量
cp examples/.env.example .env
```

### 数据库连接方式

migra 支持多种 PostgreSQL 连接方式：

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

详见 [examples/](examples/) 目录和 [PostgreSQL 文档](https://www.postgresql.org/docs/current/libpq-envars.html)。

## 项目结构

```
.
├── cmd/migra/           # CLI 入口（cobra + viper）
├── internal/
│   ├── model/          # 中间数据模型（Schema, Table, Column...）
│   ├── parser/         # SQL 解析（pg_query_go）
│   ├── introspect/     # 数据库内省（pg_catalog）
│   ├── normalize/      # 语义归一化
│   ├── diff/           # 差异比较引擎
│   ├── plan/           # 执行计划（DAG 排序）
│   ├── render/         # SQL 渲染器
│   └── testutil/       # 测试工具
├── scripts/            # 辅助脚本
├── examples/           # 示例配置
├── docs/               # 使用手册、架构设计
├── testdata/           # 测试数据
├── Makefile            # 常用命令（build/test/lint）
└── .github/            # CI/CD 配置
```

## 开发

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
make build    # 构建项目
make test     # 运行测试
make lint     # 代码检查
make fmt      # 格式化代码
make vet      # Go vet 检查
make ci       # 运行完整 CI 检查
```

### 技术栈

- **语言**：Go 1.26+
- **CLI 框架**：[cobra](https://github.com/spf13/cobra) + [viper](https://github.com/spf13/viper)
- **数据库驱动**：[pgx v5](https://github.com/jackc/pgx)
- **SQL 解析**：[pg_query_go](https://github.com/lfittl/pg_query_go)
- **测试**：[testify](https://github.com/stretchr/testify)

## 贡献

欢迎贡献！请阅读：

- [CONTRIBUTING.md](CONTRIBUTING.md) - 贡献指南
- [CODE_OF_CONDUCT.md](CODE_OF_CONDUCT.md) - 行为准则

提交 Pull Request 前请确保：
- ✅ 运行 `make ci` 通过所有检查
- ✅ 添加必要的测试
- ✅ 更新相关文档

## 变更日志

查看 [CHANGELOG.md](CHANGELOG.md) 了解版本变更。

## License

本项目采用 [MIT License](LICENSE) 开源。

---

**注意**：本项目仍在积极开发中，API 和功能可能会发生变化。
