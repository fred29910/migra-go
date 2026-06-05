# 目录比较示例

本文档展示如何使用 MIGRA-Go 比较目录中的 SQL 文件差异。

## 场景说明

在实际开发中，数据库 Schema 通常以 SQL 文件形式存储在版本控制系统中。使用目录比较功能可以：

- 比较不同版本的 Schema 文件
- 检查迁移脚本的差异
- 验证 Schema 变更的完整性

## 基本用法

### 比较两个目录

```bash
# 比较 v1 和 v2 目录中的所有 SQL 文件
migra diff ./schemas/v1/ ./schemas/v2/
```

### 目录 vs 单文件

```bash
# 比较目录和单个 SQL 文件
migra diff ./schemas/v1/ ./schemas/v2/snapshot.sql
```

### 目录 vs 数据库

```bash
# 比较目录和数据库
migra diff ./schemas/v1/ postgres://localhost/db
```

## 目录结构要求

### 基本结构

```
schemas/
├── v1/
│   ├── 01_create_users.sql
│   ├── 02_create_orders.sql
│   └── 03_create_indexes.sql
└── v2/
    ├── 01_create_users.sql
    ├── 02_create_orders.sql
    ├── 03_create_indexes.sql
    └── 04_create_products.sql
```

### 支持嵌套目录

```
schemas/
├── v1/
│   ├── tables/
│   │   ├── users.sql
│   │   └── orders.sql
│   └── indexes/
│       └── users_indexes.sql
└── v2/
    ├── tables/
    │   ├── users.sql
    │   ├── orders.sql
    │   └── products.sql
    └── indexes/
        ├── users_indexes.sql
        └── products_indexes.sql
```

### 自动忽略规则

MIGRA-Go 会自动忽略以下文件和目录：

- 隐藏文件（以 `.` 开头）
- 隐藏目录（以 `.` 开头）
- 非 `.sql` 文件
- 空目录

## 实际示例

### 示例 1：版本比较

假设有以下目录结构：

```bash
# v1/schema.sql
CREATE TABLE users (
    id SERIAL PRIMARY KEY,
    name TEXT NOT NULL,
    email TEXT UNIQUE
);

# v2/schema.sql
CREATE TABLE users (
    id SERIAL PRIMARY KEY,
    name TEXT NOT NULL,
    email TEXT UNIQUE,
    created_at TIMESTAMP DEFAULT NOW()
);

CREATE TABLE products (
    id SERIAL PRIMARY KEY,
    name TEXT NOT NULL,
    price DECIMAL(10,2)
);
```

运行比较：

```bash
migra diff ./v1/ ./v2/
```

输出：

```sql
ALTER TABLE users ADD COLUMN created_at TIMESTAMP DEFAULT NOW();

CREATE TABLE products (
    id SERIAL PRIMARY KEY,
    name TEXT NOT NULL,
    price DECIMAL(10,2)
);
```

### 示例 2：多 Schema 目录

```bash
# 目录结构
schemas/
├── public/
│   ├── users.sql
│   └── orders.sql
└── auth/
    ├── roles.sql
    └── permissions.sql
```

比较时指定 schema：

```bash
migra diff --schema public --schema auth ./schemas/ ./schemas_new/
```

### 示例 3：与数据库比较

```bash
# 比较目录和数据库
migra diff ./schemas/v1/ postgres://localhost/db

# 指定 schema
migra diff --schema public --schema auth \
  ./schemas/v1/ postgres://localhost/db
```

## 高级用法

### 输出 JSON 格式

```bash
# 生成 JSON 格式的差异报告
migra diff --format json ./v1/ ./v2/ > diff.json
```

### 启用危险操作检测

```bash
# 包含 DROP 操作的差异
migra diff --unsafe-drop ./v1/ ./v2/
```

### 严格模式

```bash
# 遇到不支持的语句时直接失败
migra diff --strict ./v1/ ./v2/
```

### 超时控制

```bash
# 设置超时时间
migra diff --timeout 2m ./v1/ ./v2/
```

## Push 命令中的目录比较

### 预览变更

```bash
# 预览目录变更
migra push --dry-run ./v1/ postgres://localhost/db
```

### 执行变更

```bash
# 执行目录变更
migra push ./v1/ postgres://localhost/db
```

## 文件合并规则

MIGRA-Go 会将目录中的所有 SQL 文件合并为一个完整的 Schema 进行比较：

1. **按文件名排序**：文件按字母顺序读取
2. **按目录深度排序**：嵌套目录中的文件在父目录文件之后读取
3. **合并所有内容**：所有 SQL 语句合并为一个完整的 Schema

### 示例

目录结构：

```
schema/
├── 01_tables.sql
├── 02_indexes.sql
└── sub/
    └── 03_constraints.sql
```

合并顺序：

1. `01_tables.sql`
2. `02_indexes.sql`
3. `sub/03_constraints.sql`

## 注意事项

1. **文件编码**：SQL 文件应使用 UTF-8 编码
2. **SQL 语法**：文件中的 SQL 语句必须是有效的 PostgreSQL 语法
3. **文件大小**：建议单个 SQL 文件不超过 10MB
4. **嵌套深度**：建议目录嵌套深度不超过 5 层
5. **文件数量**：建议单个目录中的 SQL 文件不超过 100 个

## 故障排查

### 问题：目录不存在

```
Error: directory does not exist: ./schemas/v1/
```

**解决方案**：检查目录路径是否正确。

### 问题：目录为空

```
Error: no SQL files found in directory
```

**解决方案**：确保目录中包含 `.sql` 文件。

### 问题：SQL 语法错误

```
Error: parse error near "xxx"
```

**解决方案**：检查 SQL 文件中的语法错误。

### 问题：文件编码错误

```
Error: invalid UTF-8 encoding
```

**解决方案**：将 SQL 文件转换为 UTF-8 编码。

## 相关文档

- [README.md](../README.md) - 项目概述和完整功能列表
- [multi-schema-comparison.md](./multi-schema-comparison.md) - 多 Schema 比较示例
- [configuration-usage.md](./configuration-usage.md) - 配置文件使用示例
