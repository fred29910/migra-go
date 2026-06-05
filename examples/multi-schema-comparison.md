# 多 Schema 比较示例

本文档展示如何使用 MIGRA-Go 比较多个 PostgreSQL Schema 的差异。

## 场景说明

在实际项目中，数据库通常包含多个 Schema，例如：

- `public`：默认 Schema，存放业务表
- `auth`：认证相关的表（用户、角色、权限）
- `api`：API 相关的表（接口定义、限流规则）

使用 `--schema` 参数可以同时比较多个 Schema 的差异。

## 基本用法

### 比较单个 Schema

```bash
# 比较 public schema
migra diff --schema public source.sql target.sql

# 比较 auth schema
migra diff --schema auth source.sql target.sql
```

### 比较多个 Schema

```bash
# 同时比较 public 和 auth 两个 schema
migra diff --schema public --schema auth source.sql target.sql

# 比较三个 schema
migra diff --schema public --schema auth --schema api source.sql target.sql
```

### 使用配置文件

在配置文件中预定义 schema 列表，避免每次输入：

```yaml
# migra.yaml
diff:
  schemas:
    - public
    - auth
    - api
```

然后直接运行：

```bash
migra diff source.sql target.sql
```

## 实际示例

### 示例 1：比较 SQL 文件

假设有两个 SQL 文件，分别包含不同版本的 schema：

```bash
# v1/schema.sql - 包含 public 和 auth 两个 schema
# v2/schema.sql - 包含 public、auth 和 api 三个 schema

migra diff --schema public --schema auth v1/schema.sql v2/schema.sql
```

输出示例：

```sql
CREATE SCHEMA api;

CREATE TABLE api.endpoints (
    id SERIAL PRIMARY KEY,
    path TEXT NOT NULL,
    method TEXT NOT NULL,
    description TEXT
);

CREATE INDEX idx_endpoints_path ON api.endpoints(path);
```

### 示例 2：比较数据库

```bash
# 比较两个数据库的 public 和 auth schema
migra diff --schema public --schema auth \
  postgres://localhost/db1 \
  postgres://localhost/db2
```

### 示例 3：混合比较

```bash
# 比较 SQL 文件和数据库的多个 schema
migra diff --schema public --schema auth \
  schema.sql \
  postgres://localhost/db
```

### 示例 4：输出 JSON 格式

```bash
# 生成 JSON 格式的差异报告
migra diff --schema public --schema auth --format json \
  source.sql target.sql > diff.json
```

## 高级用法

### 结合超时控制

```bash
# 设置 2 分钟超时（适用于大型数据库）
migra diff --schema public --schema auth --timeout 2m \
  postgres://localhost/db1 postgres://localhost/db2
```

### 启用危险操作检测

```bash
# 包含 DROP 操作的差异
migra diff --schema public --schema auth --unsafe-drop \
  source.sql target.sql
```

### 严格模式

```bash
# 遇到不支持的语句时直接失败
migra diff --schema public --schema auth --strict \
  source.sql target.sql
```

## Push 命令中的多 Schema

### 预览变更

```bash
# 预览多 schema 的变更
migra push --dry-run --schema public --schema auth \
  source.sql postgres://localhost/db
```

### 执行变更

```bash
# 执行多 schema 的变更
migra push --schema public --schema auth \
  source.sql postgres://localhost/db
```

## 注意事项

1. **Schema 必须存在**：指定的 Schema 必须在源和目标中都存在，否则会报错
2. **跨 Schema 引用**：支持跨 Schema 的外键引用，但需要确保引用的 Schema 也在比较范围内
3. **默认 Schema**：如果不指定 `--schema`，默认只比较 `public` Schema
4. **性能考虑**：比较多个 Schema 会增加处理时间，建议使用 `--timeout` 设置合理的超时时间

## 故障排查

### 问题：Schema 不存在

```
Error: schema "xxx" does not exist
```

**解决方案**：检查 Schema 名称是否正确，确保数据库中存在该 Schema。

### 问题：跨 Schema 外键引用失败

```
Error: foreign key constraint references non-existent table
```

**解决方案**：将外键引用的 Schema 也加入比较范围。

### 问题：超时

```
Error: context deadline exceeded
```

**解决方案**：使用 `--timeout` 参数增加超时时间，例如 `--timeout 5m`。

## 相关文档

- [README.md](../README.md) - 项目概述和完整功能列表
- [configuration.md](../docs/configuration.md) - 配置文件详细说明
- [DDL.md](../docs/DDL.md) - 支持的 DDL 类型矩阵
