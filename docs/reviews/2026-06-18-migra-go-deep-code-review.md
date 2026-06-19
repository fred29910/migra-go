# MIGRA-Go 深度代码评审报告

评审日期：2026-06-18

评审范围：围绕 MIGRA-Go 的架构设计、性能与资源利用、逻辑准确性与鲁棒性、Go 代码规范、可测试性与可靠性进行审查，重点关注数据库 Schema 对比与迁移 SQL 生成链路。

## 总体结论

项目分层比较清晰：`source -> parser/introspect -> model -> normalize -> diff -> plan -> render -> app/cmd` 的主链路符合 Go 项目实践，测试数量也不少。当前主要风险不在代码风格，而在「Schema 语义保真」和「迁移 SQL 执行顺序」两类核心准确性问题；这些会直接导致错误 diff 或生成不可执行 SQL。

已验证：

- `go test ./...` 通过，17 个包、854 个测试。
- `go test ./... -cover` 通过。
- `go test -race ./...` 通过。

覆盖率摘要：

| 包 | 覆盖率 |
| --- | ---: |
| `internal/parser` | 93.0% |
| `internal/introspect` | 89.0% |
| `internal/model` | 92.3% |
| `internal/plan` | 90.5% |
| `internal/diff` | 61.4% |
| `cmd/migra` | 20.9% |
| `internal/util` | 22.6% |

## 【高优先级/缺陷】

### 1. DB 内省会丢失 enum/domain/array 等真实列类型

[必须修复]

`internal/introspect/tables.go` 只读取 `information_schema.columns.data_type`，并用它作为 `model.Column.DataType`。PostgreSQL 中 enum、domain、自定义类型通常会返回 `USER-DEFINED`，数组返回 `ARRAY`，这会导致 DB vs SQL 文件对比时误判，甚至渲染出不可执行的 `USER-DEFINED` 类型。

影响：

- enum 列、domain 列、数组列在 DB 源上失真。
- 与 SQL 文件源对比时会产生错误 diff。
- 后续 `render` 可能输出不可执行 SQL。

建议：

- 改为基于 `pg_catalog` 读取列类型。
- 使用 `pg_attribute.atttypid` 和 `pg_attribute.atttypmod`，通过 `format_type(a.atttypid, a.atttypmod)` 获取真实类型。
- 同时补充 `attgenerated`、`pg_get_expr(ad.adbin, ad.adrelid)` 等字段，覆盖 generated column 和 default 表达式。

相关位置：

- `internal/introspect/tables.go:13`

### 2. enum 类型创建顺序可能晚于依赖它的表

[必须修复]

`internal/diff/differ.go` 在 namespace 内先执行 `diffTables`，后执行 `diffTypes`；而 `AddTableOp.DependsOn` 只声明外键表依赖，没有声明列类型依赖。新增 enum 和新增使用该 enum 的表时，可能生成 `CREATE TABLE` 早于 `CREATE TYPE`，SQL 在 PostgreSQL 上会失败。

影响：

- 迁移脚本不可执行。
- DAG 拓扑排序无法纠正该问题，因为依赖关系没有被建模。

建议：

- 将 type diff 前置，先生成 `AddEnumTypeOp`。
- 给 `AddTableOp`、`AddColumnOp`、`AlterColumnTypeOp` 增加类型依赖识别。
- 更稳妥的方案是在 `model.Column` 中保留结构化类型引用，例如 `{Schema, Name, IsArray}`，由 planner 统一排序。

相关位置：

- `internal/diff/differ.go:87`
- `internal/diff/operation_table.go:93`

### 3. Context 取消会被 diff 阶段静默吞掉

[必须修复]

`diff.Engine` 的签名是 `Diff(...) ([]Operation, []string)`，没有 error 返回。各循环里 `checkCancelled()` 发现取消后只是 `return` 当前函数，最终 `Differ.Diff` 仍返回已有 ops。`ComputeDiff` 会继续 plan/render，可能输出半成品迁移脚本。

影响：

- 超时或取消时可能生成不完整 diff。
- 自动化执行链路中存在数据变更风险。

建议：

- 将接口改为 `Diff(ctx, source, target) ([]Operation, []string, error)`。
- 任何 `ctx.Err()` 都直接向上返回。
- `ComputeDiff` 遇到取消错误时应中止，不应继续渲染。

相关位置：

- `internal/diff/differ.go:11`
- `internal/diff/differ.go:29`
- `internal/app/pipeline.go:130`

### 4. 自动列重命名启发式可能把「删除 + 新增」误判为 rename

[必须修复]

`diffTableColumns` 使用 `dataType + nullable + default + collation` 签名匹配 source-only 和 target-only 列。两个语义完全不同但结构相同的列会被自动渲染成 `ALTER TABLE ... RENAME COLUMN`，这会把旧数据保留到新字段，属于数据语义错误。

影响：

- 可能将本应删除的数据错误迁移到新列。
- 对生产库执行 `push` 时风险较高。

建议：

- 默认禁用启发式 rename。
- 改为显式配置、SQL 注释 hint、迁移规则文件，或交互确认。
- 至少在多候选或签名冲突时退回 drop/add 并输出 warning。

相关位置：

- `internal/diff/diff_tables.go:215`

## 【中优先级/优化】

### 1. Dialect 抽象不足，目前基本是 PostgreSQL 专用

[建议修改]

虽然 `source.Loader`、`diff.Engine`、`render.SQLEngine` 有接口，但 `introspect`、`parser`、`model`、`render` 中大量 PostgreSQL 语义是硬编码的，例如 pgx、pg_query、`pg_catalog`、PostgreSQL DDL 渲染。若目标是支持多种数据库方言（Dialect），当前抽象还不够。

建议：

- 引入 `Dialect` 接口。
- 将 `Introspector`、`Parser`、`Normalizer`、`Renderer`、`Capabilities` 分开注册。
- PostgreSQL 作为第一个 dialect 实现，避免后续 MySQL、SQLite 等方言侵入核心包。

### 2. 大 Schema 下内存占用偏高

[建议修改]

`ComputeDiff` 每次 diff 都深拷贝 source/target，再 normalize、filter、diff。大库场景会同时持有原始对象图和两份 clone，内存峰值偏高。

建议：

- 将 normalize 设计为纯函数或只读比较器。
- 在 loader 后就 canonicalize，避免每次 diff 重复 clone。
- 如果必须保留 immutability，可支持 copy-on-write，或只 clone 被 filter/normalize 修改的字段。

相关位置：

- `internal/app/pipeline.go:120`

### 3. 文件/目录 loader 没有真正响应 context

[建议修改]

`DirectoryLoader` 的 `WalkDir`、逐文件读取，以及 `SQLFileLoader` 的 `os.ReadFile` 都没有检查 `ctx.Err()`。大目录或大 SQL 文件下，CLI timeout 不能及时中断。

建议：

- 在 walk/read 循环中检查 `ctx.Err()`。
- 解析层可增加 `ParseSQL(ctx, sql)`，至少在 statement 级别中断。
- 对超大文件可考虑 streaming 或分 statement 解析，降低峰值内存。

相关位置：

- `internal/source/dir_loader.go:43`
- `internal/source/dir_loader.go:73`
- `internal/source/sql_file_loader.go:34`

### 4. 约束和索引模型仍缺少关键 PostgreSQL 语义

[建议修改]

当前 constraint introspect 主要取 `contype`、列、`pg_get_constraintdef`；index introspect 缺少 INCLUDE columns、opclass 参数、tablespace、storage parameters、`NULLS NOT DISTINCT` 等信息。

建议：

- 补齐 `pg_constraint.condeferrable`、`pg_constraint.condeferred`、`pg_constraint.convalidated`。
- 补齐 `pg_index.indnkeyatts` 后的 included columns。
- 覆盖 `pg_index.indnullsnotdistinct`、tablespace、storage parameters。
- 将这些字段纳入 diff 和 render。

相关位置：

- `internal/introspect/constraints.go:11`
- `internal/introspect/indexes.go:13`

## 【低优先级/建议】

### 1. 建立与原始 migra 的能力矩阵

[仅供参考]

README 写明支持表、列、索引、约束、enum、视图、序列、扩展，但原始 migra 常见场景还涉及函数、触发器、规则、domain、policy、权限、comment 等。

建议：

- 维护一张「支持 / 部分支持 / 不支持」对象矩阵。
- 对不支持对象输出明确 warning。
- 在文档中说明当前版本的能力边界，避免用户误用。

### 2. 增加真实 PostgreSQL 版本矩阵集成测试

[仅供参考]

当前单测覆盖率不错，但核心风险是 pg_catalog 语义和 PostgreSQL 版本差异。

建议：

- 使用 Testcontainers 或 docker compose 跑 PostgreSQL 13/14/15/16/17。
- 覆盖 enum 列、domain、array、generated column、partition、deferrable FK、partial/expression/include index。
- 将 DB -> DB、SQL -> DB、DB -> SQL 三类来源组合纳入测试矩阵。

### 3. 给 diff/plan 增加 golden SQL 顺序测试

[仅供参考]

建议补充以下 golden case：

- `CREATE TYPE -> CREATE TABLE`
- 跨 schema FK
- view 依赖链
- drop/recreate 顺序
- rename 冲突

这些用例可以防止 planner 后续回归。

### 4. 建立性能基准阈值

[仅供参考]

已有 benchmark 文件，但建议把大 Schema 基准纳入 CI 趋势观察。

建议记录：

- `allocs/op`
- `B/op`
- `ns/op`

重点关注 clone、排序、DAG 构建和 SQL render 的回归。

