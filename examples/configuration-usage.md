# 配置文件使用示例

本文档展示如何使用 MIGRA-Go 的配置文件功能。

## 配置优先级

MIGRA-Go 支持多层配置，优先级从高到低：

1. **命令行参数**：最高优先级
2. **环境变量**：使用 `MIGRA_` 前缀
3. **配置文件**：`~/.migra.yaml` 或 `./migra.yaml`
4. **默认值**：最低优先级

## 配置文件

### 文件位置

MIGRA-Go 会自动查找以下配置文件：

- `~/.migra.yaml`：用户级配置（全局生效）
- `./migra.yaml`：项目级配置（当前目录生效）

### 创建配置文件

```bash
# 复制示例配置
cp examples/config.yaml ~/.migra.yaml
# 或
cp examples/config.yaml ./migra.yaml
```

### 配置文件结构

```yaml
# migra.yaml

# 全局选项
verbose: false

# 数据库连接
database:
  # 默认数据库连接字符串
  url: "postgres://localhost:5432/mydb?sslmode=disable"

  # diff 零参数模式的来源与目标
  source: "postgres://localhost/db1"
  target: "postgres://localhost/db2"

# 比较选项
diff:
  # 指定要比较的 schema 列表
  schemas:
    - public
    - auth

  # 是否允许破坏性 DROP 操作
  unsafe_drop: false

  # 遇到不支持的语句时是否失败退出
  strict: false

  # 输出格式：sql 或 json
  format: sql

  # Schema 加载超时时间
  timeout: 30s

# 输出选项
output:
  # 输出文件路径（空则输出到 stdout）
  file: ""
```

## 环境变量

### 使用 `MIGRA_` 前缀

所有配置项都可以通过环境变量设置，使用 `MIGRA_` 前缀：

```bash
# 数据库连接
export MIGRA_DATABASE_URL="postgres://localhost:5432/mydb?sslmode=disable"

# diff 配置
export MIGRA_DIFF_SCHEMAS="public,auth,api"
export MIGRA_DIFF_UNSAFE_DROP="true"
export MIGRA_DIFF_STRICT="true"
export MIGRA_DIFF_FORMAT="json"
export MIGRA_DIFF_TIMEOUT="2m"

# 输出配置
export MIGRA_OUTPUT_FILE="diff.sql"
```

### 配置项映射

配置文件中的点号 `.` 对应环境变量中的下划线 `_`：

| 配置文件 | 环境变量 |
|---------|---------|
| `database.url` | `MIGRA_DATABASE_URL` |
| `diff.schemas` | `MIGRA_DIFF_SCHEMAS` |
| `diff.unsafe_drop` | `MIGRA_DIFF_UNSAFE_DROP` |
| `diff.strict` | `MIGRA_DIFF_STRICT` |
| `diff.format` | `MIGRA_DIFF_FORMAT` |
| `diff.timeout` | `MIGRA_DIFF_TIMEOUT` |

## .env 文件

### 使用 godotenv

MIGRA-Go 支持自动加载 `.env` 文件：

```bash
# 创建 .env 文件
cp examples/.env.example .env
```

### .env 文件格式

```bash
# .env

# 数据库连接
MIGRA_DATABASE_URL="postgres://localhost:5432/mydb?sslmode=disable"

# diff 配置
MIGRA_DIFF_SCHEMAS="public,auth"
MIGRA_DIFF_UNSAFE_DROP="false"
MIGRA_DIFF_STRICT="false"
MIGRA_DIFF_FORMAT="sql"
MIGRA_DIFF_TIMEOUT="30s"

# 输出配置
MIGRA_OUTPUT_FILE=""
```

## 实际示例

### 示例 1：基本配置

创建 `./migra.yaml`：

```yaml
database:
  url: "postgres://localhost:5432/mydb?sslmode=disable"

diff:
  schemas:
    - public
  format: sql
```

运行：

```bash
migra diff
```

### 示例 2：多 Schema 配置

创建 `./migra.yaml`：

```yaml
database:
  source: "postgres://localhost/db1"
  target: "postgres://localhost/db2"

diff:
  schemas:
    - public
    - auth
    - api
  unsafe_drop: true
```

运行：

```bash
migra diff
```

### 示例 3：环境变量覆盖

配置文件 `./migra.yaml`：

```yaml
database:
  url: "postgres://localhost:5432/mydb?sslmode=disable"

diff:
  schemas:
    - public
  format: sql
```

使用环境变量覆盖：

```bash
# 覆盖数据库连接
export MIGRA_DATABASE_URL="postgres://localhost:5432/otherdb?sslmode=disable"

# 覆盖 schema 列表
export MIGRA_DIFF_SCHEMAS="public,auth"

# 运行
migra diff
```

### 示例 4：命令行参数覆盖

```bash
# 使用配置文件中的设置
migra diff

# 使用命令行参数覆盖
migra diff --schema public --schema auth --format json file.sql target.sql
```

## 数据库连接配置

### 连接字符串

```yaml
database:
  url: "postgres://user:password@localhost:5432/dbname?sslmode=disable"
```

### 短连接格式

```yaml
database:
  url: "pg://localhost/dbname"
```

### 环境变量配置

```bash
# 使用标准 PostgreSQL 环境变量
export PGHOST=localhost
export PGPORT=5432
export PGUSER=myuser
export PGPASSWORD=mypassword
export PGDATABASE=mydb
```

### pg_service.conf 配置

```yaml
database:
  url: "postgres://?service=myservice"
```

### .pgpass 密码文件

```yaml
database:
  url: "postgres://myuser@localhost/mydb"
```

## 高级配置

### 详细输出模式

```yaml
verbose: true
```

### 超时配置

```yaml
diff:
  timeout: 2m  # 支持格式：30s, 2m, 1h
```

### 输出到文件

```yaml
output:
  file: "diff_output.sql"
```

## 配置验证

### 检查配置文件语法

```bash
# 使用 yaml 工具检查语法
cat migra.yaml | python3 -c "import sys, yaml; yaml.safe_load(sys.stdin)"
```

### 验证数据库连接

```bash
# 测试数据库连接
migra diff --schema public source.sql postgres://localhost/db
```

## 故障排查

### 问题：配置文件未找到

```
Error: config file not found
```

**解决方案**：
- 检查配置文件路径是否正确
- 确保配置文件存在于 `~/.migra.yaml` 或 `./migra.yaml`

### 问题：配置文件语法错误

```
Error: failed to parse config file
```

**解决方案**：
- 检查 YAML 语法是否正确
- 确保缩进使用空格，不要使用 Tab

### 问题：环境变量未生效

**解决方案**：
- 检查环境变量名称是否正确（使用 `MIGRA_` 前缀）
- 确保环境变量已导出（使用 `export`）

### 问题：配置优先级问题

**解决方案**：
- 命令行参数 > 环境变量 > 配置文件 > 默认值
- 使用 `--verbose` 查看实际使用的配置

## 最佳实践

1. **使用项目级配置**：在项目根目录创建 `./migra.yaml`，方便团队共享
2. **敏感信息使用环境变量**：数据库密码等敏感信息不要写入配置文件
3. **使用 .env 文件**：在开发环境中使用 `.env` 文件存储本地配置
4. **版本控制**：将配置文件提交到版本控制系统，但排除 `.env` 文件
5. **文档化**：在 README 中说明配置文件的使用方法

## 相关文档

- [README.md](../README.md) - 项目概述和完整功能列表
- [multi-schema-comparison.md](./multi-schema-comparison.md) - 多 Schema 比较示例
- [directory-comparison.md](./directory-comparison.md) - 目录比较示例
- [configuration.md](../docs/configuration.md) - 配置文件详细说明
