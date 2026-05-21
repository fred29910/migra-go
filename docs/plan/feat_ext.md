# DDL 特性扩展支持分析

本文档基于 `docs/DDL.md` 与当前代码实现进行对照，评估 PostgreSQL DDL 特性在 migra-go 中的支持现状和后续扩展空间。

> 说明：仓库中实际文件为 `docs/DDL.md`，不是小写的 `docs/ddl.md`。

## 总体结论

`docs/DDL.md` 中列出的核心 schema diff 特性仍然可以在本项目中支持。当前架构已经具备清晰的扩展点：Parser 使用 `HandlerRegistry` 分发 AST handler，Diff 层以 `Operation` 表达差异，Render 层统一输出 SQL，Introspect 层负责从 `pg_catalog` / `information_schema` 读取目标数据库状态。

但文档中部分标记为「完全支持」的特性与当前实现存在差距，尤其集中在以下几类：

- SQL 文件解析路径未完整落模型。
- Diff 层能检测部分字段，但 Renderer 未完整输出对应 DDL。
- 数据库自省信息较少，导致 SQL 文件与数据库对比时可能误报或漏报。
- 一些 DDL 属于一次性运维操作，不适合直接纳入当前「目标 schema 状态收敛」模型。

## 当前基本可支持

| 特性范围 | 支持判断 | 说明 |
| --- | --- | --- |
| 表基础操作 | 可支持 | 包括 `CREATE TABLE`、diff 生成 `DROP TABLE`、新增列、删除列。 |
| 列基础属性 | 可支持 | 包括数据类型、`NOT NULL`、`DEFAULT`、`IDENTITY`、`COLLATE`。 |
| 列重命名 | 可支持 | Parser 支持 `ALTER TABLE ... RENAME COLUMN`；Diff 侧使用启发式检测。 |
| 主键与外键 | 可支持 | 支持单列、复合键、跨 schema 外键、`ON DELETE` / `ON UPDATE`。 |
| 简单索引 | 基础可支持 | 支持 `CREATE INDEX`、`CREATE UNIQUE INDEX` 和 diff 生成 `DROP INDEX`。 |
| 枚举类型 | 可支持 | 支持 `CREATE TYPE ... AS ENUM`、追加 enum label、diff 生成 drop enum。 |
| 多 schema | 可支持 | 支持 schema 限定对象与 `CREATE SCHEMA`。 |
| 执行安全机制 | 可支持 | 包括危险 DROP 过滤、执行计划、拓扑排序。 |

## 需要修正文档或补齐实现的特性

| 特性 | 当前情况 | 建议 |
| --- | --- | --- |
| `ALTER COLUMN TYPE ... USING` | 当前 `AlterColumnTypeOp` 只有 `FromType` / `ToType`，Renderer 不输出 `USING`。 | 若要支持，需要在 Parser、Operation、Renderer 中保存和输出 `USING` 表达式。 |
| `UNIQUE` / `CHECK` SQL 文件解析 | `CreateTableHandler` 主要处理 primary key 和 foreign key，未完整处理 unique/check。DB introspect 能读 `p/u/c`。 | 补齐 `CreateTableHandler` 对 `CONSTR_UNIQUE`、`CONSTR_CHECK` 的解析。 |
| `ALTER TABLE ... ADD CONSTRAINT` | `AlterTableHandler` 当前未处理 add/drop constraint 子命令。 | 增加对应 mutation，并复用现有 `AddConstraintOp` / `DropConstraintOp`。 |
| 高级索引 | Parser 和模型保存了部分字段，但 Renderer 当前只输出基础 `CREATE INDEX`。 | 补齐 `USING`、`WHERE`、opclass、排序、`CONCURRENTLY`、`IF NOT EXISTS` 的渲染。 |
| DB introspect 高级索引 | 当前只读取 index name、table、columns、unique、method。 | 改为读取 `pg_get_indexdef()` 或补齐 predicate、expression、opclass、排序等结构化字段。 |
| `DROP SCHEMA` | Renderer 有 `DropSchemaOp`，但 Diff 只输出 warning，不生成 operation。 | 若允许 schema 删除，需要在 Diff 中生成 `DropSchemaOp`，并受 `--unsafe-drop` 控制。 |
| `IDENTITY` 修改 | 创建列支持 identity，但普通列改 identity 时统一渲染 `SET GENERATED`，不覆盖所有 PostgreSQL 场景。 | 区分 `ADD GENERATED ... AS IDENTITY` 与 `SET GENERATED ...`。 |
| `character(n)` / `char(n)` | normalize 未完整处理同义类型。 | 增加 `character(n)` 到 `char(n)` 的规范化映射。 |

## 暂不支持但适合后续扩展

以下特性可以沿当前架构扩展，但需要同时新增 model、parser handler、diff operation、renderer 和 introspect 支持：

| 特性类别 | 具体 DDL |
| --- | --- |
| 视图 | `CREATE VIEW`、`CREATE MATERIALIZED VIEW` |
| 序列 | `CREATE SEQUENCE` |
| 触发器与规则 | `CREATE TRIGGER`、`CREATE RULE` |
| 权限与安全 | `CREATE POLICY`、行级安全（RLS） |
| 扩展与排序规则 | `CREATE EXTENSION`、`CREATE COLLATION` |
| 搜索能力 | `CREATE TEXT SEARCH ...` |
| 可编程对象 | `CREATE FUNCTION`、`CREATE PROCEDURE`、`CREATE AGGREGATE` |
| 类型系统 | range type、composite type、domain |
| 操作符系统 | `CREATE OPERATOR`、`CREATE CAST` |
| 表高级能力 | `INHERITS`、`PARTITION BY`、storage parameters、tablespace |
| 注释 | `COMMENT ON ...` |

## 不建议直接纳入 schema diff 的特性

以下 DDL 更像一次性运维操作，而不是目标 schema 状态的一部分：

- `TRUNCATE TABLE`
- `REINDEX INDEX`

如果后续确实需要支持，建议作为显式 migration operation 或手工脚本能力，而不是放入当前「比较两个 schema 并收敛目标状态」的 diff 模型。

## 建议优先级

### P0：修正文档与现有能力对齐

- 将高级索引从「完全支持」调整为「部分支持」。
- 将 `UNIQUE` / `CHECK` SQL 文件解析从「完全支持」调整为「部分支持」。
- 明确 `ALTER COLUMN TYPE ... USING` 当前未渲染。
- 明确 `DROP SCHEMA` 当前只 warning，不生成 DDL。

### P1：补齐低成本高收益能力

- 补齐 `UNIQUE` / `CHECK` 的 SQL 文件解析。
- 补齐 `ALTER TABLE ... ADD/DROP CONSTRAINT`。
- 补齐 `character(n)` / `char(n)` 规范化。
- 修正 identity 变更渲染语义。

### P2：增强索引支持

- Renderer 输出索引方法 `USING ...`。
- Renderer 输出 partial index 的 `WHERE` 子句。
- 支持表达式索引、排序规则、opclass、`NULLS FIRST/LAST`。
- DB introspect 使用 `pg_get_indexdef()` 或结构化 catalog 查询还原高级索引定义。

> 实施计划：`docs/superpowers/plans/2026-05-21-p2-p3-index-object-expansion.md`。
>
> P2 目标是完成 SQL 文件和 DB introspect 的结构化索引闭环。P3 首批对象限定为 view、sequence、extension；function、trigger、policy 保持为下一阶段扩展。

### P3：扩展新对象类型

- 优先考虑 view、sequence、extension。
- 其次考虑 function、trigger、policy。
- 每类对象都应独立设计 model 与 diff 语义，避免把 raw SQL 字符串直接塞进现有 table/type 模型。

> 实施计划：`docs/superpowers/plans/2026-05-21-p2-p3-index-object-expansion.md`。
>
> P2 目标是完成 SQL 文件和 DB introspect 的结构化索引闭环。P3 首批对象限定为 view、sequence、extension；function、trigger、policy 保持为下一阶段扩展。

## 验证记录

本次分析后运行了现有测试：

```bash
rtk go test ./...
```

结果：219 个测试通过，覆盖 13 个 Go package。
