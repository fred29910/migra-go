# 配置说明

migra 支持通过配置文件、环境变量和命令行参数进行配置。

## 配置文件

### YAML 配置

支持 `~/.migra.yaml` 或 `./migra.yaml`：

```yaml
# 数据库连接
database:
  # 默认数据库连接字符串（diff 单参数模式或 push 目标使用）
  url: "postgres://user:password@localhost:5432/dbname?sslmode=disable"
  # diff 零参数模式的来源与目标
  source: "postgres://localhost/db1"
  target: "postgres://localhost/db2"

  # 连接池配置（预留，暂未实现）
  # pool:
  #   max_open_conns: 10
  #   max_idle_conns: 5
  #   conn_max_lifetime: "1h"

# 比较选项
diff:
  schemas:
    - public          # 默认值；可通过 --schema 覆盖
  unsafe_drop: false  # 是否允许破坏性 DROP 操作
  strict: false       # 遇到不支持的语句时是否失败
  format: sql          # 输出格式：sql 或 json

# 输出选项
output:
  file: ""      # 输出文件路径（空则输出到 stdout）

# 全局选项
verbose: false    # 详细输出模式（注意：此为顶层键，不是 output.verbose）

# 日志配置（预留，暂未实现）
# logging:
#   level: "info"  # debug, info, warn, error
#   format: "text" # text, json
```

示例文件：`examples/config.yaml`

### .env 文件

migra 可以通过 `source .env` 或 `direnv` 等外部工具加载 `.env` 文件中的环境变量。Viper 的 `AutomaticEnv()` 会自动读取已设置的 OS 环境变量（使用 `MIGRA_` 前缀）。

```bash
cp examples/.env.example .env
# 编辑 .env 填入实际配置
```

`.env` 文件中的变量名与下方「环境变量配置」一节中的名称一致（使用 `MIGRA_` 前缀）。

> **注意**：`diff.timeout` 配置项在代码中**未绑定到 Viper**，因此 YAML 配置文件中的 `diff.timeout` 值不会生效。超时时间只能通过 CLI 标志 `--timeout` 设置（默认 `30s`）。

### 环境变量配置

复制 `examples/.env.example` 为 `.env` 并修改。所有环境变量使用 `MIGRA_` 前缀：

```bash
# 数据库连接
MIGRA_DATABASE_URL=postgres://user:password@localhost:5432/dbname?sslmode=disable
MIGRA_DATABASE_SOURCE=postgres://localhost/db1
MIGRA_DATABASE_TARGET=postgres://localhost/db2

# 连接池配置（预留，暂未实现）
# MIGRA_DATABASE_POOL_MAX_OPEN_CONNS=10
# MIGRA_DATABASE_POOL_MAX_IDLE_CONNS=5
# MIGRA_DATABASE_POOL_CONN_MAX_LIFETIME=1h

# 比较选项
MIGRA_DIFF_SCHEMAS=public
MIGRA_DIFF_UNSAFE_DROP=false
MIGRA_DIFF_STRICT=false
MIGRA_DIFF_FORMAT=sql

# 输出选项
MIGRA_OUTPUT_FILE=

# 全局选项
MIGRA_VERBOSE=false

# 日志（预留，暂未实现）
# MIGRA_LOGGING_LEVEL=info
# MIGRA_LOGGING_FORMAT=text
```

环境变量通过 Viper 自动绑定，点分隔键转换为下划线分隔的大写形式（如 `diff.schemas` → `MIGRA_DIFF_SCHEMAS`）。

---

## CLI 标志

### 全局标志

| 参数 | 说明 | 默认值 |
|------|------|--------|
| `-c, --config` | 指定配置文件路径 | `~/.migra.yaml` 或 `./migra.yaml` |
| `-V, --verbose` | 输出详细日志 | `false` |
| `-v, --version` | 输出版本信息并退出 | `false` |

### diff 子命令

| 参数 | 说明 | 默认值 |
|------|------|--------|
| `source` `target` | 两个 schema 来源路径/URL（支持 2/1/0 参数模式） | — |
| `-s, --schema` | 指定要比较的 Schema 列表（可指定多个） | `public` |
| `-f, --format` | 输出格式：`sql` 或 `json` | `sql` |
| `--unsafe-drop` | 允许输出/执行危险的 DROP 操作 | `false` |
| `--strict` | 遇到不支持的语句时直接失败退出（默认跳过并警告） | `false` |
| `--timeout` | Schema 加载超时时间（如 `30s`, `2m`） | `30s` |
| `-o, --output` | 输出到文件（默认输出到 stdout） | — |

### push 子命令

| 参数 | 说明 | 默认值 |
|------|------|--------|
| `source` `target` | 两个参数：来源与目标（仅 2 参数模式） | — |
| `-s, --schema` | 指定要比较的 Schema 列表 | `public` |
| `--unsafe-drop` | 跳过危险 DROP 操作确认 | `false` |
| `--dry-run` | 显示 SQL 预览但不执行 | `false` |
| `--execute` | 跳过交互确认直接执行（不推荐生产使用） | `false` |
| `--no-verify` | 跳过执行后校验 | `false` |
| `--timeout` | Schema 加载超时时间 | `30s` |

---

## diff 命令的三种参数模式

`diff` 命令支持灵活的传参方式：

| 参数数量 | 场景 | 说明 |
|----------|------|------|
| 2 个参数 | `migra diff A B` | 比较 A 与 B（最常见） |
| 1 个参数 | `migra diff A` | 比较 A 与配置中的 `database.url` |
| 0 个参数 | `migra diff` | 比较配置中的 `database.source` 与 `database.target` |

`push` 命令固定需要 2 个参数。

---

## PostgreSQL 连接方式

### 1. 连接字符串

标准 URI 格式：

```
postgres://[user[:password]@][host][:port][/dbname][?param1=value1&...]
```

参数说明：
- `sslmode`: `disable`, `require`, `verify-full` 等
- `connect_timeout`: 连接超时（秒）
- `pool_max_conns`: 连接池最大连接数

### 2. 标准环境变量

pgx 自动支持 PostgreSQL 标准环境变量：

| 环境变量 | 说明 | 示例 |
|---------|------|------|
| `PGHOST` | 数据库主机 | `localhost` |
| `PGPORT` | 端口号 | `5432` |
| `PGUSER` | 用户名 | `myuser` |
| `PGPASSWORD` | 密码 | `mypassword` |
| `PGDATABASE` | 数据库名 | `mydb` |
| `PGSSLMODE` | SSL 模式 | `disable` |

使用方式：
```bash
export PGHOST=localhost
export PGPORT=5432
export PGUSER=myuser
export PGDATABASE=mydb
migra diff file.sql "postgres://"
```

### 3. pg_service.conf

通过服务名连接，避免暴露凭证：

**~/.pg_service.conf**:
```ini
[myservice]
host=localhost
port=5432
user=myuser
dbname=mydb
sslmode=disable
```

使用：
```bash
migra diff file.sql "postgres://?service=myservice"
```

### 4. .pgpass 密码文件

将密码存储在 `~/.pgpass` 文件中：

```
localhost:5432:mydb:myuser:mypassword
```

权限设置：
```bash
chmod 600 ~/.pgpass
```

然后使用：
```bash
migra diff file.sql "postgres://myuser@localhost/mydb"
```

---

## 配置优先级

优先级从高到低：
1. **命令行参数**（如 `--schema`, `--format`, `--timeout`）
2. **环境变量**（如 `MIGRA_DIFF_SCHEMAS`）
3. **配置文件**（`~/.migra.yaml` 或 `./migra.yaml`）
4. **默认值**

---

## Viper 配置绑定

migra 使用 [viper](https://github.com/spf13/viper) 管理配置，支持：
- 配置文件自动发现：`~/.migra.yaml` → `./migra.yaml`
- 环境变量自动绑定：`MIGRA_` 前缀映射到点分隔键
- 命令行参数绑定到对应的 Viper 键

配置初始化流程 (`cmd/migra/main.go` `initConfig()`)：
1. 设置环境变量前缀为 `MIGRA_`
2. 启用 `AutomaticEnv()` 自动绑定
3. 按优先级查找配置文件：`--config` 指定 → `~/.migra.yaml` → `./migra.yaml`
4. 使用 viper 读取 YAML 配置

---

## 安全注意事项

⚠️ **重要**：
- 不要将包含真实凭证的 `.env` 文件提交到仓库
- `.env` 已在 `.gitignore` 中忽略
- 建议使用 `pg_service.conf` 或 `.pgpass` 管理凭证
- CI/CD 中使用 GitHub Secrets 或环境变量注入凭证
