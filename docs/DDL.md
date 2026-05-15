# PostgreSQL DDL 特性支持矩阵

> **项目**: migra-go — 基于 `pg_query_go` 的 PostgreSQL  schema diff 工具  
> **最后更新**: 2026-05-15  
> **Legend**: ✅ 完全支持 | ⚠️ 部分支持 | ❌ 暂不支持

---

## 1. 表操作 (Table Operations)

| 特性 | 解析 (Parser) | Diff 引擎 | 渲染 (Renderer) | 数据库自省 (Introspect) | 状态 |
|------|:---:|:---:|:---:|:---:|------|
| `CREATE TABLE` | ✅ | ✅ | ✅ | ✅ | ✅ |
| `CREATE TABLE IF NOT EXISTS` | ✅ | ✅ | ✅ | — | ✅ |
| `DROP TABLE` | — | ✅ (源端检测) | ✅ `DROP TABLE IF EXISTS` | — | ✅ |
| `DROP TABLE IF EXISTS` | — | ✅ | ✅ | — | ✅ |
| `ALTER TABLE ... ADD COLUMN` | ✅ | ✅ | ✅ | ✅ | ✅ |
| `ALTER TABLE ... DROP COLUMN` | ✅ | ✅ | ✅ `DROP COLUMN IF EXISTS` | ✅ | ✅ |
| `ALTER TABLE ... ALTER COLUMN TYPE` | ✅ (含 `USING`) | ✅ | ✅ | ✅ | ✅ |
| `ALTER TABLE ... RENAME COLUMN` | ⚠️ pg_query 解析但不处理 | ❌ | ❌ | — | ⚠️ |
| `ALTER TABLE ... SET (storage_param)` | ⚠️ pg_query 解析但不处理 | ❌ | ❌ | — | ⚠️ |
| 表继承 (`INHERITS`) | ⚠️ pg_query 解析但不处理 | ❌ | ❌ | — | ⚠️ |
| 分区表 (`PARTITION BY`) | ⚠️ pg_query 解析但不处理 | ❌ | ❌ | — | ⚠️ |
| `CREATE TABLE ... AS` | ❌ | ❌ | ❌ | — | ❌ |
| `TRUNCATE TABLE` | ❌ | ❌ | ❌ | — | ❌ |
| `COMMENT ON TABLE` | ❌ | ❌ | ❌ | — | ❌ |

---

## 2. 列操作 (Column Operations)

| 特性 | 解析 (Parser) | Diff 引擎 | 渲染 (Renderer) | 数据库自省 (Introspect) | 状态 |
|------|:---:|:---:|:---:|:---:|------|
| 基本数据类型 (`integer`, `text`, `varchar`, 等) | ✅ | ✅ | ✅ | ✅ | ✅ |
| 数值精度 (`numeric(10,2)`, `varchar(100)`) | ✅ | ✅ | ✅ | ✅ | ✅ |
| `SERIAL` / `BIGSERIAL` / `SMALLSERIAL` | ✅ (映射为 integer/bigint/smallint) | ✅ | ✅ | ✅ | ✅ |
| `boolean` / `bool` | ✅ | ✅ | ✅ | ✅ | ✅ |
| `UUID` | ✅ | ✅ | ✅ | ✅ | ✅ |
| `JSON` / `JSONB` | ✅ | ✅ | ✅ | ✅ | ✅ |
| `INET` | ✅ | ✅ | ✅ | ✅ | ✅ |
| `INTERVAL` | ✅ | ✅ | ✅ | ✅ | ✅ |
| `DATE` / `TIME` / `TIMESTAMP` / `TIMESTAMPTZ` | ✅ | ✅ | ✅ | ✅ | ✅ |
| `REAL` / `DOUBLE PRECISION` / `SMALLINT` | ✅ | ✅ | ✅ | ✅ | ✅ |
| 自定义类型 (含 enum) | ✅ | ✅ | ✅ | ✅ | ✅ |
| `NOT NULL` 约束 | ✅ | ✅ | ✅ | ✅ | ✅ |
| `DEFAULT` 表达式 | ✅ (含函数调用) | ✅ | ✅ | ✅ | ✅ |
| 列级 `CHECK` 约束 | ✅ (作为表约束解析) | ✅ | ✅ | ✅ | ✅ |
| `PRIMARY KEY` (行内) | ✅ | ✅ | ✅ | ✅ | ✅ |
| `IDENTITY` 列 (`GENERATED ALWAYS/BY DEFAULT AS IDENTITY`) | ⚠️ model.Column 有字段但自省未查询 | ⚠️ | ⚠️ | ❌ | ⚠️ |
| 列排序规则 (`COLLATE`) | ⚠️ pg_query 解析但 model 未存储 | ❌ | ❌ | ❌ | ⚠️ |
| 存储参数 (`STORAGE`) | ❌ | ❌ | ❌ | ❌ | ❌ |
| `GENERATED ALWAYS AS` (计算列) | ❌ | ❌ | ❌ | ❌ | ❌ |

---

## 3. 约束操作 (Constraint Operations)

| 特性 | 解析 (Parser) | Diff 引擎 | 渲染 (Renderer) | 数据库自省 (Introspect) | 状态 |
|------|:---:|:---:|:---:|:---:|------|
| `PRIMARY KEY` (单列) | ✅ | ✅ | ✅ | ✅ | ✅ |
| `PRIMARY KEY` (复合键) | ✅ | ✅ | ✅ | ✅ | ✅ |
| `UNIQUE` 约束 (表级) | ✅ | ✅ | ✅ | ✅ | ✅ |
| `UNIQUE` 约束 (行内) | ✅ | ✅ | ✅ | ✅ | ✅ |
| `FOREIGN KEY` (单列) | ✅ | ✅ | ✅ | ✅ | ✅ |
| `FOREIGN KEY` (复合键) | ✅ | ✅ | ✅ | ✅ | ✅ |
| `FOREIGN KEY` 跨 schema 引用 | ✅ | ✅ | ✅ | ✅ | ✅ |
| `CHECK` 约束 (表级) | ✅ | ✅ | ✅ | ✅ | ✅ |
| `CHECK` 约束 (行内) | ✅ | ✅ | ✅ | ✅ | ✅ |
| `ALTER TABLE ... ADD CONSTRAINT` | ✅ | ✅ | ✅ | — | ✅ |
| `ALTER TABLE ... DROP CONSTRAINT` | — | ✅ (源端检测) | ✅ | — | ✅ |
| 约束命名 (`CONSTRAINT name`) | ✅ | ✅ | ✅ | ✅ | ✅ |
| 自动生成约束名 (`_pkey`, `_fkey`) | ✅ | ✅ | ✅ | ✅ | ✅ |
| `ON DELETE` / `ON UPDATE` 级联 | ⚠️ pg_query 解析但 model 未完整存储 | ❌ | ❌ | ❌ | ⚠️ |
| `DEFERRABLE` / `INITIALLY DEFERRED` | ❌ | ❌ | ❌ | ❌ | ❌ |
| `EXCLUDE` 约束 | ❌ | ❌ | ❌ | ❌ | ❌ |
| `ASSERTION` | ❌ | ❌ | ❌ | ❌ | ❌ |

---

## 4. 索引操作 (Index Operations)

| 特性 | 解析 (Parser) | Diff 引擎 | 渲染 (Renderer) | 数据库自省 (Introspect) | 状态 |
|------|:---:|:---:|:---:|:---:|------|
| `CREATE INDEX` | ✅ | ✅ | ✅ | ✅ | ✅ |
| `CREATE UNIQUE INDEX` | ✅ | ✅ | ✅ | ✅ | ✅ |
| `DROP INDEX` | — | ✅ (源端检测) | ✅ `DROP INDEX IF EXISTS` | — | ✅ |
| `DROP INDEX IF EXISTS` | — | ✅ | ✅ | — | ✅ |
| 索引方法 (`USING btree/hash/gin/gist/brin`) | ✅ | ✅ | ✅ | ✅ (btree/hash/gin) | ✅ |
| 表达式索引 (`ON tbl (lower(col))`) | ✅ | ✅ | ✅ | ❌ | ⚠️ |
| 部分索引 (`WHERE` 子句) | ✅ | ✅ | ✅ | ❌ | ⚠️ |
| 操作符类 (`text_pattern_ops`, 等) | ✅ | ✅ | ✅ | ❌ | ⚠️ |
| 排序规则 (`ASC`/`DESC`, `NULLS FIRST/LAST`) | ✅ | ✅ | ✅ | ❌ | ⚠️ |
| `CONCURRENTLY` | ✅ | ✅ | ✅ | ❌ | ⚠️ |
| `IF NOT EXISTS` | ✅ | ✅ | ✅ | — | ✅ |
| 索引列命名 (`INDEX col_name_idx`) | ✅ | ✅ | ✅ | ✅ | ✅ |
| `ALTER INDEX ... RENAME` | ❌ | ❌ | ❌ | — | ❌ |
| `REINDEX INDEX` | ❌ | ❌ | ❌ | — | ❌ |

> **注意**: 数据库自省 (DBLoader) 查询索引时仅获取 `index_name, table_name, column_names, is_unique, method`，不包含 `WHERE` 子句、操作符类、排序规则、并发标志等。这些高级特性仅在 SQL 文件解析路径中可用。

---

## 5. 枚举类型操作 (Enum Type Operations)

| 特性 | 解析 (Parser) | Diff 引擎 | 渲染 (Renderer) | 数据库自省 (Introspect) | 状态 |
|------|:---:|:---:|:---:|:---:|------|
| `CREATE TYPE ... AS ENUM` | ✅ | ✅ | ✅ | ✅ | ✅ |
| `ALTER TYPE ... ADD VALUE` | ✅ | ✅ | ✅ | ✅ | ✅ |
| `DROP TYPE ... ENUM` | — | ✅ (源端检测) | ✅ `DROP TYPE IF EXISTS` | — | ✅ |
| 枚举标签排序 (`enumsortorder`) | ✅ | ✅ | ✅ | ✅ | ✅ |
| 枚举标签追加检测 (append-only) | — | ✅ (仅追加, 非追加时警告) | — | — | ✅ |
| 枚举重命名标签 | ❌ | ❌ | ❌ | — | ❌ |
| 枚举删除标签 | ❌ | ❌ | ❌ | — | ❌ |
| `CREATE TYPE ... AS RANGE` | ❌ | ❌ | ❌ | — | ❌ |
| `CREATE TYPE ... AS COMPOSITE` | ❌ | ❌ | ❌ | — | ❌ |
| `CREATE DOMAIN` | ❌ | ❌ | ❌ | — | ❌ |

---

## 6. Schema / 命名空间操作 (Schema Operations)

| 特性 | 解析 (Parser) | Diff 引擎 | 渲染 (Renderer) | 数据库自省 (Introspect) | 状态 |
|------|:---:|:---:|:---:|:---:|------|
| 多 Schema 支持 (`public`, `auth`, 等) | ✅ | ✅ | ✅ | ✅ | ✅ |
| Schema 限定表引用 (`schema.table`) | ✅ | ✅ | ✅ | ✅ | ✅ |
| Schema 限定枚举引用 | ✅ | ✅ | ✅ | ✅ | ✅ |
| `CREATE SCHEMA` | ❌ | ❌ | ❌ | — | ❌ |
| `DROP SCHEMA` | ❌ | ❌ | ❌ | — | ❌ |
| `ALTER SCHEMA ... RENAME` | ❌ | ❌ | ❌ | — | ❌ |
| Schema 迁移 (跨 schema 移动对象) | ❌ | ❌ | ❌ | — | ❌ |

> **注意**: 数据库自省中，Schema 列表通过 `--schema` / `-s` 参数指定，默认仅 `public`。删除 Schema 时会输出警告 `"namespace drop is not implemented yet"`。

---

## 7. 高级 / 其他 DDL 特性

| 特性 | 状态 | 说明 |
|------|------|------|
| **视图** (`CREATE VIEW`) | ❌ | 不在解析器、Diff 引擎、渲染器中 |
| **物化视图** (`CREATE MATERIALIZED VIEW`) | ❌ | 同上 |
| **序列** (`CREATE SEQUENCE`) | ❌ | 不支持；`SERIAL` 类型已映射为 `integer` + 默认值 |
| **触发器** (`CREATE TRIGGER`) | ❌ | 不支持 |
| **规则** (`CREATE RULE`) | ❌ | 不支持 |
| **行级安全策略** (`CREATE POLICY`) | ❌ | 不支持 |
| **扩展** (`CREATE EXTENSION`) | ❌ | 不支持 |
| **排序规则** (`CREATE COLLATION`) | ❌ | 不支持 |
| **全文搜索配置** (`CREATE TEXT SEARCH`) | ❌ | 不支持 |
| **函数 / 过程** (`CREATE FUNCTION/PROCEDURE`) | ❌ | 不支持 |
| **聚合函数** (`CREATE AGGREGATE`) | ❌ | 不支持 |
| **操作符** (`CREATE OPERATOR`) | ❌ | 不支持 |
| **类型转换** (`CREATE CAST`) | ❌ | 不支持 |
| **继承表** (`INHERITS`) | ⚠️ | pg_query 解析但 diff/renderer 不处理 |
| **分区表** (`PARTITION BY`) | ⚠️ | pg_query 解析但 diff/renderer 不处理 |
| **表空间** (`TABLESPACE`) | ❌ | 不支持 |
| **行级安全** (`FORCE ROW LEVEL SECURITY`) | ❌ | 不支持 |

---

## 8. 数据类型映射 (Type Mapping)

| PostgreSQL 内部类型 | 标准化输出 | 状态 |
|---------------------|-----------|------|
| `int4` | `integer` | ✅ |
| `int8` | `bigint` | ✅ |
| `int2` | `smallint` | ✅ |
| `serial` | `integer` | ✅ |
| `bigserial` | `bigint` | ✅ |
| `smallserial` | `smallint` | ✅ |
| `bool` | `boolean` | ✅ |
| `float4` | `real` | ✅ |
| `float8` | `double precision` | ✅ |
| `character varying` | `varchar` | ✅ |
| `timestamp without time zone` | `timestamp` | ✅ |
| `timestamp with time zone` | `timestamptz` | ✅ |
| `character(n)` | `char(n)` / `character(n)` | ⚠️ |
| 其他自定义类型 | 原样输出 | ✅ |

---

## 9. 执行计划与依赖排序 (Execution Plan)

| 阶段 | 包含操作 | 状态 |
|------|---------|------|
| **Pre-deploy** (创建) | `ADD TABLE`, `ADD COLUMN`, `ADD INDEX`, `ADD CONSTRAINT`, `ADD ENUM TYPE` | ✅ |
| **Deploy** (修改) | `ALTER COLUMN TYPE`, `SET/DROP NOT NULL`, `SET/DROP DEFAULT`, `ADD ENUM LABEL` | ✅ |
| **Post-deploy** (删除, 需 `--unsafe-drop`) | `DROP TABLE`, `DROP COLUMN`, `DROP INDEX`, `DROP CONSTRAINT`, `DROP ENUM TYPE` | ✅ |

依赖排序使用 **Kahn 拓扑排序** 算法，确保:
- 外键引用的表先于引用它的表创建
- 删除操作先于创建操作执行 (同约束名)
- 列的添加先于索引创建

---

## 10. 已知限制与注意事项

1. **`Rename Column` / `Rename Table`**: pg_query 解析为 `ALTER TABLE RENAME COLUMN`，但 diff 引擎和渲染器未实现对应的 Mutation/Operation。

2. **`ON DELETE CASCADE` / `ON UPDATE CASCADE`**: 外键约束的引用操作在 model.Constraint 中未存储，渲染时丢失。

3. **数据库自省的索引信息有限**: DBLoader 仅查询 `pg_index` + `pg_am` 获取索引名、表、列、唯一性和方法，不包含 `WHERE` 子句、操作符类、排序规则、并发标志。

4. **`IF NOT EXISTS` on `CREATE TABLE`**: 解析器支持但渲染器不输出 `IF NOT EXISTS` 子句 (表创建始终直接生成 `CREATE TABLE`)。

5. **枚举标签删除/重命名**: Diff 引擎仅支持追加检测 (`append-only`)，非追加变更输出警告但不生成修复操作。

6. **Schema 删除**: 检测到源端存在但目标端缺失的 Schema 时输出警告，不生成 `DROP SCHEMA` 语句。

7. **`UNSAFE DROP` 机制**: 默认情况下所有 `DROP` 操作被过滤并替换为警告，需显式传递 `--unsafe-drop` 标志。

8. **`ONLY` 子句 (表继承)**: `CREATE TABLE ... INHERITS (...)` 被 pg_query 解析但 diff/renderer 不处理继承关系。

9. **分区表**: `PARTITION BY RANGE/LIST/HASH` 子句被 pg_query 解析但 diff/renderer 不处理。

10. **存储参数**: `WITH (fillfactor=70)` 等存储参数被 pg_query 解析但 diff/renderer 不处理。