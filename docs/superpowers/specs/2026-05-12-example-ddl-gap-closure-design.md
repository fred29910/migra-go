# 示例 DDL 缺口补齐设计

## 背景

当前项目的基础 diff 流水线已经可用，但 `testdata/example_source.sql` 到 `testdata/example_target.sql` 的示例无法完整体现 README 中展示的能力。主要问题是 SQL 文件解析会对 `CREATE TYPE ... AS ENUM` 报不支持，`CREATE TABLE` 内的主键、外键和函数默认值没有完整进入模型，`CREATE TABLE` 渲染也不会输出表级约束。

本设计采用“示例闭环优先”的范围：让 `migra diff testdata/example_source.sql testdata/example_target.sql` 输出覆盖示例差异的合理迁移 SQL，同时保持现有索引解析和渲染能力不回退。

## 目标

1. SQL 文件输入不再因示例中的 enum 语句产生 unsupported warning。
2. 示例目标新增表 `comments` 的渲染包含列、默认值、非空约束和主键。
3. 示例中 `user_role` enum 从 `admin,user` 到 `admin,user,guest` 时输出 enum label 增量迁移。
4. 示例中 `users.age` 新增列仍输出 `ALTER TABLE ... ADD COLUMN`。
5. 列删除、默认值变更和约束增删具备最小可用 operation 与 render 支撑，为后续 MVP 完整化打基础。

## 非目标

1. 不实现 rename 推断。重命名仍可能表现为 drop + add。
2. 不覆盖 PostgreSQL 所有 DDL 语法，只覆盖示例和附近常见写法。
3. 不实现数据库 roundtrip 测试，也不要求连接真实 PostgreSQL。
4. 不重构现有 model 的整体形态。

## 数据模型

沿用现有 `model.Schema`、`model.Namespace`、`model.Table`、`model.Column`、`model.Constraint`、`model.EnumType` 和 `model.Index`。本轮只在必要时增加 operation 类型，不做大规模模型迁移。

约束继续存放在 `Table.Constraints`。解析阶段为主键和外键生成稳定约束名：

1. 列级或表级主键默认名为 `<table>_pkey`。
2. 外键默认名为 `<table>_<column>_fkey`。
3. 如果 AST 提供约束名，则优先使用显式名称。

约束 `Definition` 保存可直接拼入 `ALTER TABLE ... ADD CONSTRAINT` 或 `CREATE TABLE` 的 SQL 片段，例如 `PRIMARY KEY ("id")` 和 `FOREIGN KEY ("user_id") REFERENCES "public"."users" ("id")`。

## Parser 设计

`CreateEnumHandler` 从 `pg_nodes.CreateEnumStmt` 提取 schema、type name 和 labels，返回 `CreateEnumTypeMutation`。`CreateEnumTypeMutation.Apply` 写入 `Namespace.Types`，重复定义返回错误。

`CreateTableHandler` 继续解析列，同时补充以下能力：

1. 列级 `PRIMARY KEY`：列保持 `NOT NULL`，并生成表级 primary key constraint。
2. 表级 `PRIMARY KEY (...)`：生成 `table.PrimaryKey` 和 `Constraint`。
3. 表级 `FOREIGN KEY (...) REFERENCES ... (...)`：生成 `Constraint`。
4. `DEFAULT now()` 等函数表达式：通过安全 deparse 或专用表达式解析写入 `Column.DefaultExpr`。

`CreateTableMutation` 需要扩展为同时携带 `PrimaryKey` 和 `Constraints`，`Apply` 时写入 `model.Table`。这样 parser handler 只负责提取 AST 语义，mutation 继续作为唯一写模型入口。

`AlterTableHandler` 保持 `ADD COLUMN` 支持。对于本轮新增的 drop column/default 相关 operation，优先通过 diff 层模型差异产生，不要求 parser 覆盖所有 ALTER 子命令。

## Diff 设计

新增 operation：

1. `AddEnumLabelOp`：同名 enum 存在，target labels 是 source labels 的有序追加时，为每个新增 label 生成操作。
2. `SetDefaultOp`：source 默认值为空或不同，target 有默认值。
3. `DropDefaultOp`：source 有默认值，target 默认值为空。
4. `DropColumnOp`：source 存在而 target 不存在的列。

默认值变更不再只 warning。列删除标记为 destructive，默认仍受 `--unsafe-drop` 控制。

enum 变更采用保守策略：只支持尾部追加 label。若 label 顺序重排、删除或中间插入，本轮不生成危险 SQL，返回 warning，避免输出不可执行或语义错误的迁移。

约束 diff 沿用现有 `AddConstraintOp` 和 `DropConstraintOp`，按约束名比较。若同名约束定义不同，生成 drop + add；drop 受 unsafe 策略控制。

## Render 设计

`renderAddTable` 输出：

1. 列名、类型、`NOT NULL`、`DEFAULT`。
2. 表级 primary key 和 constraints。
3. schema-qualified table name。

`AddEnumLabelOp` 渲染为：

```sql
-- op: add_enum_label risk:low
ALTER TYPE "public"."user_role" ADD VALUE 'guest';
```

`SetDefaultOp` 渲染为 `ALTER TABLE ... ALTER COLUMN ... SET DEFAULT ...`。

`DropDefaultOp` 渲染为 `ALTER TABLE ... ALTER COLUMN ... DROP DEFAULT`。

`DropColumnOp` 渲染为 `ALTER TABLE ... DROP COLUMN IF EXISTS ...`，风险等级 high。

现有 index render 保持兼容，不改变 `Index.Elements` 与 `Index.Columns` 的 fallback 行为。

## Planner 与安全策略

`AddEnumLabelOp`、`SetDefaultOp` 和 `DropDefaultOp` 属于 deploy 阶段。`DropColumnOp` 属于 post-deploy 阶段，只有 `--unsafe-drop` 为 true 时输出。

新增 operation 的 `DependsOn` 要保证：

1. enum label 依赖 enum type。
2. default 变更依赖对应 table/column。
3. drop column 不需要阻塞示例主线，但必须提供稳定 object key。

## 测试计划

先写失败测试，再实现。

1. Parser 单测覆盖 enum、primary key、foreign key、`DEFAULT now()`。
2. Diff 单测覆盖 enum label 追加、默认值 set/drop、列删除和约束定义变化。
3. Render 单测覆盖带约束的新表渲染、enum label、default op、drop column。
4. CLI 端到端测试覆盖示例 source/target，断言输出包含 `CREATE TABLE "public"."comments"`、`ALTER TABLE "public"."users" ADD COLUMN "age" integer`、`ALTER TYPE "public"."user_role" ADD VALUE 'guest'`，且 stderr 不包含 `CREATE TYPE ENUM is not yet supported`。

## 验收命令

1. `rtk go test ./...`
2. `rtk go vet ./...`
3. `rtk make build`
4. `rtk ./migra diff testdata/example_source.sql testdata/example_target.sql`

## 风险与约束

1. `pg_query_go` 的 AST 节点覆盖细节可能与预期不同，parser 测试应先固定实际结构。
2. 约束定义字符串需要稳定，否则 golden 和 diff 容易抖动。
3. enum 非尾部追加不能安全自动迁移，本轮明确 warning，不伪装成功。
4. 目标是示例闭环，不代表完整 PostgreSQL DDL 支持。
