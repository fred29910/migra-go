# MIGRA-Go

一个用 Go 编写的 PostgreSQL schema 差异比较工具，灵感来自 Python 版的 [migra](https://github.com/djrobstep/migra)。

## 功能特性

- 🔍 **双向 Diff**：比较 SQL 文件、PostgreSQL 实例或两者之间的差异
- 🏗️ **结构化模型**：使用中间 SchemaModel 表示数据库结构
- 📋 **SQL 生成**：输出可执行的迁移 SQL
- 🛡️ **安全保护**：标记破坏性操作，可选跳过危险变更
- 🎯 **语义归一化**：减少因同义表达导致的误报

## 安装

```bash
git clone https://github.com/migra-go/migra-go.git
cd migra-go
go build -o schemadiff ./cmd/schemadiff
```

或者直接从 releases 下载二进制文件（待发布）。

## 使用方法

### 基本用法

```bash
# 比较 SQL 文件和数据库
schemadiff file.sql postgres://user:pass@localhost/dbname

# 比较两个数据库
schemadiff postgres://localhost/db1 postgres://localhost/db2

# 比较两个 SQL 文件
schemadiff file_a.sql file_b.sql
```

### 命令行参数

| 参数 | 说明 |
|------|------|
| `-s, --schema` | 指定要比较的 schema（可多个） |
| `-f, --format` | 输出格式：sql 或 json |
| `--unsafe-drop` | 允许输出危险的 DROP 操作 |
| `--strict` | 遇到不支持的语句时失败 |
| `-o, --output` | 输出到文件（默认：stdout） |

### 示例

1. **从 SQL 文件迁移到数据库**：
   ```bash
   schemadiff schema.sql postgres://localhost/myapp
   ```

2. **仅输出高风险操作警告**：
   ```bash
   schemadiff --unsafe-drop=false file.sql postgres://localhost/myapp
   ```

3. **输出 JSON 格式**：
   ```bash
   schemadiff -f json file.sql postgres://localhost/myapp
   ```

## 项目结构

```
.
├── cmd/schemadiff        # CLI 入口（cobra + viper）
├── internal/
│   ├── model/          # 中间数据模型（Schema, Table, Column...）
│   ├── parser/         # SQL 解析（pg_query_go）
│   ├── introspect/     # 数据库内省（pg_catalog）
│   ├── normalize/      # 语义归一化
│   ├── diff/           # 差异比较引擎
│   ├── plan/           # 执行计划（DAG 排序）
│   ├── render/         # SQL 渲染器
│   └── testutil/       # 测试工具
├── docs/               # 设计文档
└── testdata/           # 测试数据
```

## 开发进度

- ✅ **Phase 1**：introspect MVP + golden 测试框架
- ✅ **Phase 2**：diff/render + plan DAG 排序
- ✅ **Phase 3**：parser MVP + file→db 链路
- ✅ **Phase 4**：破坏性变更诊断 + CLI 完善

## 技术栈

- **语言**：Go 1.21+
- **CLI 框架**：[cobra](https://github.com/spf13/cobra) + [viper](https://github.com/spf13/viper)
- **数据库驱动**：[pgx v5](https://github.com/jackc/pgx)
- **SQL 解析**：[pg_query_go](https://github.com/lfittl/pg_query_go)
- **测试**：[testify](https://github.com/stretchr/testify)

## 贡献

欢迎提交 Issue 和 Pull Request！

## License

[待定]

---

**注意**：本项目仍在积极开发中，API 和功能可能会发生变化。
