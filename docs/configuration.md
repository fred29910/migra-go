# 配置说明

migra 支持通过配置文件、环境变量和命令行参数进行配置。

## 配置文件

### YAML 配置

支持 `~/.migra.yaml` 或 `./migra.yaml`：

```yaml
# 数据库连接
database:
  url: "postgres://user:password@localhost:5432/dbname?sslmode=disable"
  pool:
    max_open_conns: 10
    max_idle_conns: 5
    conn_max_lifetime: "1h"

# 比较选项
diff:
  schemas:
    - public
  unsafe_drop: false
  strict: false
  format: sql

# 输出选项
output:
  file: ""
  verbose: false

# 日志配置
logging:
  level: "info"
  format: "text"
```

示例文件：`examples/config.yaml`

### 环境变量配置

复制 `examples/.env.example` 为 `.env` 并修改：

```bash
DATABASE_URL="postgres://user:password@localhost:5432/dbname?sslmode=disable"
MIGRA_SCHEMAS="public"
MIGRA_UNSAFE_DROP="false"
MIGRA_STRICT="false"
MIGRA_FORMAT="sql"
MIGRA_OUTPUT=""
MIGRA_VERBOSE="false"
LOG_LEVEL="info"
```

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

## 安全注意事项

⚠️ **重要**：
- 不要将包含真实凭证的 `.env` 文件提交到仓库
- `.env` 已在 `.gitignore` 中忽略
- 建议使用 `pg_service.conf` 或 `.pgpass` 管理凭证
- CI/CD 中使用 GitHub Secrets 或环境变量注入凭证

## 命令行参数优先级

优先级从高到低：
1. 命令行参数（如 `--schema`, `--format`）
2. 环境变量（如 `MIGRA_SCHEMAS`）
3. 配置文件（`~/.migra.yaml` 或 `./migra.yaml`）
4. 默认值

## viper 配置绑定

migra 使用 [viper](https://github.com/spf13/viper) 管理配置，支持：
- 配置文件自动发现
- 环境变量自动绑定
- 命令行参数绑定
