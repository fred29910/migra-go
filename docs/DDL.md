# PostgreSQL DDL 特性支持矩阵

> **项目**: migra-go — 基于 `pg_query_go` 的 PostgreSQL  schema diff 工具  
> **最后更新**: 2026-06-03  
> **Legend**: ✅ 完全支持 | ⚠️ 部分支持 (含多种情况: a) pg_query 可解析但下游不处理; b) 部分子特性支持; c) 能检测但不生成修复 DDL) | ❌ 暂不支持

---

## 1. 表操作 (Table Operations)

| 特性 | 解析 (Parser) | Diff 引擎 | 渲染 (Renderer) | 数据库自省 (Introspect) | 状态 |
|------|:---:|:---:|:---:|:---:|------|
| `CREATE TABLE` | ✅ | ✅ | ✅ | ✅ | ✅ |
| `CREATE TABLE IF NOT EXISTS` | ✅ | ✅ | ⚠️ (不输出 IF NOT EXISTS, 见限制 #4) | — | ✅ |
| `DROP TABLE` | — | ✅ (源端检测) | ✅ `DROP TABLE IF EXISTS` | — | ✅ |
| `DROP TABLE IF EXISTS` | — | ✅ | ✅ | — | ✅ |
| `ALTER TABLE ... ADD COLUMN` | ✅ | ✅ | ✅ | ✅ | ✅ |
| `ALTER TABLE ... DROP COLUMN` | ✅ | ✅ | ✅ `DROP COLUMN IF EXISTS` | ✅ | ✅ |
| `ALTER TABLE ... ALTER COLUMN TYPE` | ✅ (含 `USING`) | ✅ | ✅ | ✅ | ✅ |
| `ALTER TABLE ... RENAME COLUMN` | ✅ (RenameStmtHandler) | ✅ (启发式检测) | ✅ `RENAME COLUMN ... TO ...` | — | ✅ |
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
| `IDENTITY` 列 (`GENERATED ALWAYS/BY DEFAULT AS IDENTITY`) | ✅ | ✅ | ✅ | ✅ | ✅ |
| 列排序规则 (`COLLATE`) | ⚠️ pg_query 解析但未充分提取 | ✅ (AlterColumnCollationOp) | ✅ (SET DATA TYPE ... COLLATE) | ✅ (查询 collation_name) | ⚠️ |
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
| `FOREIGN KEY` `ON DELETE` / `ON UPDATE` | ✅ (含级联动作) | ✅ (`sameConstraintContent` 比较 OnDelete/OnUpdate) | ✅ (`ON DELETE CASCADE` 等) | ✅ (查询 confupdtype/confdeltype) | ✅ |
| `CHECK` 约束 (表级) | ✅ | ✅ | ✅ | ✅ | ✅ |
| `CHECK` 约束 (行内) | ✅ | ✅ | ✅ | ✅ | ✅ |
| `ALTER TABLE ... ADD CONSTRAINT` | ✅ | ✅ | ✅ | — | ✅ |
| `ALTER TABLE ... DROP CONSTRAINT` | — | ✅ (源端检测) | ✅ | — | ✅ |
| 约束命名 (`CONSTRAINT name`) | ✅ | ✅ | ✅ | ✅ | ✅ |
| 自动生成约束名 (`_pkey`, `_fkey`) | ✅ | ✅ | ✅ | ✅ | ✅ |
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
| 索引方法 (`USING btree/hash/gin/gist/brin`) | ✅ | ✅ | ✅ | ✅ | ✅ |
| 表达式索引 | ✅ | ✅ | ✅ | ✅ (pg_get_indexdef element) | ✅ |
| 部分索引 (`WHERE` 子句) | ✅ | ✅ | ✅ | ✅ (pg_get_expr) | ✅ |
| 操作符类 | ✅ | ✅ | ✅ | ✅ (pg_get_indexdef element) | ✅ |
| 排序规则 | ✅ | ✅ | ✅ | ✅ (pg_get_indexdef element) | ✅ |
| `CONCURRENTLY` | ✅ | ✅ | ✅ | ❌ | ⚠️ |
| `IF NOT EXISTS` | ✅ | ✅ | ✅ | — | ✅ |
| 索引列命名 (`INDEX col_name_idx`) | ✅ | ✅ | ✅ | ✅ | ✅ |
| `ALTER INDEX ... RENAME` | ❌ | ❌ | ❌ | — | ❌ |
| `REINDEX INDEX` | ❌ | ❌ | ❌ | — | ❌ |

> **注意**: 数据库自省 (DBLoader) 现已使用 `pg_get_indexdef(indexrelid, column_no, true)` 按索引元素返回完整定义（包含表达式、opclass、collation、排序、NULLS），结构化解析为 `model.IndexElem`。`CONCURRENTLY` 标志仍需从 `pg_index` 元组字段提取，当前仅在 SQL 文件解析路径中可用。

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
| `CREATE SCHEMA` | ✅ (CreateSchemaHandler) | ✅ (生成 CreateSchemaOp) | ✅ (IF NOT EXISTS) | — | ✅ |
| `DROP SCHEMA` | — | ✅ (生成 DropSchemaOp) | ✅ (IF EXISTS) | — | ✅ |
| `ALTER SCHEMA ... RENAME` | ❌ | ❌ | ❌ | — | ❌ |
| Schema 迁移 (跨 schema 移动对象) | ❌ | ❌ | ❌ | — | ❌ |

> **注意**: 数据库自省中，Schema 列表通过 `--schema` / `-s` 参数指定，默认仅 `public`。删除 Schema 受 `--unsafe-drop` 保护，默认不输出。

---

## 7. 高级 / 其他 DDL 特性

| 特性 | 状态 | 说明 |
|------|------|------|
| **视图** (`CREATE VIEW`) | ✅ | 支持普通 view 的 parse、diff、render、introspect。物化视图可被内省加载但 diff/render 将其视为普通 view 处理。 |
| **序列** (`CREATE SEQUENCE`) | ✅ | 完整支持序列的 parse、diff、render、introspect。支持数据类型、start/increment/min/max/cache/cycle 属性变更检测。 |
| **触发器** (`CREATE TRIGGER`) | ❌ | 不支持 |
| **规则** (`CREATE RULE`) | ❌ | 不支持 |
| **行级安全策略** (`CREATE POLICY`) | ❌ | 不支持 |
| **扩展** (`CREATE EXTENSION`) | ✅ | 完整支持扩展的 parse、diff、render、introspect。支持 create/drop 和 update to version。 |
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
| **Pre-deploy** (创建) | `CREATE SCHEMA`, `ADD TABLE`, `ADD COLUMN`, `ADD INDEX`, `ADD CONSTRAINT`, `ADD ENUM TYPE`, `CREATE VIEW`, `CREATE SEQUENCE`, `CREATE EXTENSION` | ✅ |
| **Deploy** (修改) | `ALTER COLUMN TYPE`, `SET/DROP NOT NULL`, `SET/DROP DEFAULT`, `SET/DROP IDENTITY`, `ADD IDENTITY`, `ADD ENUM LABEL`, `RENAME COLUMN`, `ALTER COLUMN COLLATION`, `REPLACE VIEW`, `ALTER SEQUENCE`, `ALTER EXTENSION UPDATE` | ✅ |
| **Post-deploy** (删除, 需 `--unsafe-drop`) | `DROP SCHEMA`, `DROP TABLE`, `DROP COLUMN`, `DROP INDEX`, `DROP CONSTRAINT`, `DROP ENUM TYPE`, `DROP IDENTITY`, `DROP VIEW`, `DROP SEQUENCE`, `DROP EXTENSION` | ✅ |

依赖排序使用 **Kahn 拓扑排序** 算法，确保:
- 外键引用的表先于引用它的表创建
- 删除操作先于创建操作执行 (同约束名)
- 列的添加先于索引创建

> **注意**: `CREATE SCHEMA`/`DROP SCHEMA` 已在 `assignStage()` 中显式分配为 Pre-deploy/Post-deploy。`alter_column_collation`, `set_identity`, `drop_identity`, `add_identity`, `replace_view`, `alter_sequence`, `alter_extension_update` 等均已在 `assignStage()` 中显式匹配到对应阶段。

---

## 10. 已知限制与注意事项

1. **`IF NOT EXISTS` on `CREATE TABLE`**: 解析器支持 `CREATE TABLE IF NOT EXISTS` 并正确提取表结构，但渲染器不输出 `IF NOT EXISTS` 子句（表创建始终直接生成 `CREATE TABLE`）。在 diff 工具语义下这是符合预期的——因为目标数据库中尚无该表，`IF NOT EXISTS` 对正确性无影响；仅当用户期望保留原始 DDL 文本时才被视为信息丢失。

2. **枚举标签删除/重命名**: Diff 引擎仅支持追加检测 (`append-only`)，非追加变更输出警告但不生成修复操作。

3. **Schema 删除**: 检测到源端存在但目标端缺失的 Schema 时生成 `DROP SCHEMA` 语句（需 `--unsafe-drop` 放行）。

4. **`UNSAFE DROP` 机制**: 默认情况下所有 `DROP` 操作被过滤并替换为警告，需显式传递 `--unsafe-drop` 标志。

5. **`IDENTITY` 列支持**: 完整支持（解析 → 内省 → Diff → 渲染）。可正确处理 `GENERATED ALWAYS AS IDENTITY` / `GENERATED BY DEFAULT AS IDENTITY` 的创建、变更和删除。注意：a) `SET GENERATED` 对非 identity 列需使用 `ADD GENERATED ... AS IDENTITY` 语法，当前统一使用 `SetIdentityOp` 未区分两种场景；b) `IDENTITY` 列不支持与 `DEFAULT` 子句共存（PostgreSQL 限制，未做校验）。

6. **`character(n)` / `char(n)` 同义映射缺失**: PostgreSQL 中 `CHARACTER(n)` 与 `CHAR(n)` 等价，但 `normalize.go` 的类型别名映射仅覆盖 `character varying → varchar`，未包含 `character → char`。当 SQL 文件使用 `CHARACTER(10)` 而数据库自省返回 `char(10)` 时，会产生误报 diff。

7. **列排序规则 (`COLLATE`)**: Column 模型已包含 `Collation` 字段，数据库内省准确查询 `collation_name`，Diff 引擎能检测排序规则变更并生成 `AlterColumnCollationOp`，渲染器能输出 `SET DATA TYPE ... COLLATE ...` SQL。但 SQL 文件解析路径中，`CREATE TABLE` / `ALTER TABLE` 的 `COLLATE` 子句可能未被提取到 Column 中 (取决于 pg_query_go AST 的解析覆盖)，导致文件解析路径下的排序规则比较可能不完整。

8. **重命名列启发式检测的限制**: Diff 引擎通过比较 `DataType`、`IsNullable`、`DefaultExpr`、`Collation` 来推断列重命名。此启发式方法可能产生误报——例如用户删除了具有属性 X 的列并新增了具有相同属性的列（但语义不同）。未来可通过 SQL 注释声明 (`-- @rename from_col to_col`) 来显式声明重命名，消除误报。

9. **约束名称冲突处理**: 当同一条 ALTER TABLE 语句中同时存在 DROP CONSTRAINT 和 ADD CONSTRAINT 且名称相同时，pg_query_go 会将其拆分为独立的 AST 节点。当前处理方式先生成 Drop 再生成 Add，DAG 排序确保 Drop 先执行。但某些复杂场景（如递归生成引用自身的约束）可能导致排序问题。

10. **`ONLY` 子句 (表继承)**: `CREATE TABLE ... INHERITS (...)` 被 pg_query 解析但 diff/renderer 不处理继承关系。

11. **分区表**: `PARTITION BY RANGE/LIST/HASH` 子句被 pg_query 解析但 diff/renderer 不处理。

12. **存储参数**: `WITH (fillfactor=70)` 等存储参数被 pg_query 解析但 diff/renderer 不处理。

13. **数据库自省的索引信息有限**: DBLoader 现已通过 `pg_get_indexdef(indexrelid, column_no, true)` 按索引元素获取完整定义（表达式、opclass、collation、排序、NULLS），结构化解析为 `model.IndexElem`。`CONCURRENTLY` 标志仍需从 `pg_index` 元组字段提取，当前仅在 SQL 文件解析路径中可用。

14. **`Rename Column` 启发式检测**: 见限制 #8。仅当列属性完全匹配时才判定为重命名，否则回退为 `DROP COLUMN + ADD COLUMN`。
