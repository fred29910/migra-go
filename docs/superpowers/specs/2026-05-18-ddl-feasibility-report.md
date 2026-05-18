# PostgreSQL DDL 特性可行性评估报告

> **项目**: migra-go — PostgreSQL Schema Diff 工具  
> **日期**: 2026-05-18  
> **范围**: 基于 `docs/DDL.md` 支持矩阵，评估所有 ⚠️/❌ 特性的实施可行性  
> **方法**: 逐特性分析代码现状（模型层/解析器/Diff 引擎/渲染器/内省层），给出技术路径和工作量估算

---

## 目录

1. [T1: Quick Wins — 快速取胜](#t1-quick-wins--快速取胜)
2. [T2: 中等工程](#t2-中等工程)
3. [T3: 复杂项目](#t3-复杂项目)
4. [T4: 重大特性 — 全新 DDL 子领域](#t4-重大特性--全新-ddl-子领域)
5. [汇总矩阵](#汇总矩阵)

---

## T1: Quick Wins — 快速取胜

这些特性的共同特征：**pg_query 已完全解析**，代码中**已有同类模式可复制**，改动范围限定在 2-4 个文件以内。

---

### 1.1 RENAME COLUMN

| 维度 | 当前状态 |
|------|---------|
| **解析** | pg_query 解析为 `AlterTableType_AT_RenameColumn`，但 `alter_table_handler.go` 的 `switch cmd.Subtype` 中未处理，落入 `default: warning` |
| **模型** | 无对应 Mutation/Operation |
| **Diff** | 列存在性 check 按名称匹配，任何列名变更会被处理为 DROP + ADD（破坏性） |
| **渲染** | 无对应渲染逻辑 |

**技术路径**：
1. `internal/parser/mutation.go` — 新增 `MutKindRenameColumn` + `RenameColumnMutation`，Apply 中重命名 `table.ColumnByName` 中的 key
2. `internal/parser/alter_table_handler.go` — 新增 `case pg_query.AlterTableType_AT_RenameColumn` 分支
3. `internal/diff/operation.go` — 新增 `KindRenameColumn` + `RenameColumnOp`，`IsDestructive() = false`
4. `internal/render/render.go` — 新增渲染：`ALTER TABLE schema.table RENAME COLUMN old_name TO new_name;`

**可选增强 — 列重命名的 Diff 检测**：
当前 diff 按列名匹配，源端有 `old_name` + 目标端有 `new_name` 且类型相同 → 猜测为重命名。可在 `diff_columns.go` 中增加启发式匹配（成本低，但因猜想性质默认关闭为宜）。

**估算**: 小 (4 文件, ~80 行) — **1-2 天**

---

### 1.2 IDENTITY 列 — 内省层补全

| 维度 | 当前状态 |
|------|---------|
| **模型** | `model.Column` 已有 `IsIdentity bool` / `IdentityKind string` 字段 ✅ |
| **解析** | pg_query 已正确解析 `GENERATED ALWAYS/BY DEFAULT AS IDENTITY`，字段已设置 ✅ |
| **内省** | `introspect/tables.go` 的 SQL 查询未查询 `is_identity` / `identity_generation` |
| **渲染** | `renderAddTable` / `renderAddColumn` 都忽略 `IsIdentity` 字段 |

**技术路径**：
1. `internal/introspect/tables.go` — SQL 增加 `c.is_identity, c.identity_generation` 列（来自 `information_schema.columns`）
2. `internal/render/render.go` — `renderAddTable` 和 `renderAddColumn` 中检查 `col.IsIdentity`，追加 `GENERATED {ALWAYS|BY DEFAULT} AS IDENTITY`

**估算**: 小 (2 文件, ~40 行) — **0.5 天**

---

### 1.3 COLLATE 列支持

| 维度 | 当前状态 |
|------|---------|
| **解析** | pg_query 解析 `col.collname`，但 `parserutil.ParseColumnDef` 未提取 |
| **模型** | `model.Column` 无 `Collation` 字段 |
| **渲染** | 无渲染 |

**技术路径**：
1. `internal/model/column.go` — 新增 `Collation string` 字段
2. `internal/parser/parserutil/util.go` — `ParseColumnDef` 中提取 `colDef.Collation` 并设值
3. `internal/render/render.go` — `renderAddTable`/`renderAddColumn` 中当 `col.Collation != ""` 时追加 `COLLATE "collation_name"`

> **注意**: pg_query v6 中 `ColumnDef` 的 collation 访问路径已变更，需确认实际 API。

**估算**: 小 (3 文件, ~30 行) — **0.5 天**

---

### 1.4 高级索引字段内省 (WHERE / Opclass / Ordering / CONCURRENTLY)

| 维度 | 当前状态 |
|------|---------|
| **模型** | `model.Index` 已有 `WhereClause`, `Concurrent`, `Elements[].Opclass`, `Elements[].Ordering` 等字段 ✅ |
| **解析/渲染** | 已支持表达式索引和部分索引的解析与渲染 ✅ |
| **内省** | `introspect/indexes.go` 仅查询 `index_name, table_name, column_names, is_unique, method`，不包含 WHERE 子句、操作符类、排序规则 |

**技术路径**：
1. `internal/introspect/indexes.go` — 扩展 SQL 查询 `pg_index.indpred` 获取 WHERE 子句表达式
2. 对于操作符类和排序规则：需要解析 `pg_index.indkey` 和 `pg_opclass` 的映射关系
3. 映射到 `model.Index.WhereClause` 和 `IndexElem.Opclass` / `IndexElem.Ordering`

> **注意**: 自省 WHERE 子句相对直接（`pg_get_expr(indpred, indrelid)`），但操作符类和排序规则需要更复杂的系统表查询。

**估算**: 小-中 (1-2 文件, ~60 行) — **1 天**

---

### 1.5 CREATE SCHEMA / DROP SCHEMA

| 维度 | 当前状态 |
|------|---------|
| **解析** | pg_query 已解析 `CreateSchemaStmt`，但 `registry.go` 无对应 Handler |
| **模型** | `Schema.GetOrCreateNamespace(name)` 已存在 |
| **Diff** | 无 Schema 级别 diff |
| **渲染** | 无 |

**技术路径**：
1. `internal/parser/registry.go` — 注册 `CreateSchemaHandler`
2. `internal/parser/schema_handler.go` (新文件) — 处理 `CreateSchemaStmt` → `CreateSchemaMutation`
3. `internal/parser/mutation.go` — 新增 `MutKindCreateSchema` + `CreateSchemaMutation`，`Apply` 调用 `schema.GetOrCreateNamespace`
4. `internal/diff/operation.go` — 新增 `KindCreateSchema` + `KindDropSchema` + Operation 类型
5. `internal/render/render.go` — `CREATE SCHEMA [IF NOT EXISTS] schema_name;` / `DROP SCHEMA [IF EXISTS] schema_name;`
6. `internal/diff/differ.go` — 新增 Schema 层对比逻辑（目前 diff 起点就是 Namespace，需要提升到 Schema 层）

> **注意**: 当前 diff 入口 `diffSchema` 已经是对 Namespace 级别的，但 Schema 的创建/删除需要更上层的检测。当前代码在 `LoadOptions.Schemas` 中硬编码了目标 schema 列表。

**估算**: 中 (5-6 文件, ~150 行) — **2-3 天**

---

### 1.6 ON DELETE CASCADE / ON UPDATE CASCADE

| 维度 | 当前状态 |
|------|---------|
| **解析** | pg_query 的 `Constraint` 中有 `FkDelAction` / `FkUpdAction` 字段 |
| **模型** | `model.Constraint` 无 `OnDelete` / `OnUpdate` 字段 |
| **内省** | `pg_constraint` 有 `confupdtype` / `confdeltype` 字段（`a`=no action, `r`=restrict, `c`=cascade, `n`=set null, `d`=set default），但当前 SQL 未查询 |
| **渲染** | `renderConstraintDefinition` 中外键渲染不包含级联选项 |

**技术路径**：
1. `internal/model/table.go` — `Constraint` 新增 `OnDelete string` / `OnUpdate string`
2. `internal/parser/create_table_handler.go` — 提取 `constraint.FkDelAction` / `constraint.FkUpdAction`
3. `internal/introspect/constraints.go` — 外键 SQL 增加 `c.confupdtype, c.confdeltype`，将 pg_constraint 单字符码映射为 SQL 关键字 (`a`→`"NO ACTION"`, `c`→`"CASCADE"`, 等)
4. `internal/render/render.go` — `renderConstraintDefinition` 中外键部分追加 `ON DELETE ... ON UPDATE ...`
5. `internal/diff/diff_tables.go` — `sameConstraintContent` / `sameConstraintSemantics` 需要比较 `OnDelete` / `OnUpdate`

**估算**: 中 (4-5 文件, ~120 行) — **1-2 天**

---

## T2: 中等工程

这些特性需要新增一定的逻辑复杂度，但整体上仍可以用现有架构模式覆盖。

---

### 2.1 COMMENT ON

| 维度 | 当前状态 |
|------|---------|
| **解析** | pg_query 解析 `CommentStmt` |
| **模型** | 无 Comment 存储 |
| **Diff** | 无 |
| **渲染** | 无 |

**技术路径**：
- 新增 `Comment` 模型 (key-value: `ObjectKey → string`)
- 新建 `CommentHandler` + `SetCommentMutation` + `DropCommentMutation`
- Diff 引擎需要对比 comment 差异
- 渲染: `COMMENT ON TABLE/COLUMN/INDEX xxx IS 'text';`

**估算**: 中 (5-6 文件, ~200 行) — **2-3 天**

---

### 2.2 Enum 标签重命名 / 删除

| 维度 | 当前状态 |
|------|---------|
| **解析** | `ALTER TYPE ... RENAME VALUE` pg_query 可解析 |
| **Diff** | 当前仅支持 append-only 检测 |
| **安全** | PostgreSQL 原生不支持删除枚举值，需要重建类型 |

**技术路径**：
- 新增 `AlterEnumRenameLabelOp` / `AlterEnumDropLabelOp`
- 枚举 diff 增加标签重命名启发式检测（位置+排序稳定时的匹配）
- DROP label：警告用户需要手动迁移（因为 PG 不支持安全原地删除）

**估算**: 中 (3-4 文件, ~150 行) — **2-3 天**

---

### 2.3 Storage Parameters (WITH option)

| 维度 | 当前状态 |
|------|---------|
| **解析** | pg_query 解析 `CreateStmt.Options` 中的 `WITH (fillfactor=70)` |
| **模型** | 无存储参数模型 |
| **渲染** | 无 |

**技术路径**：
- 新增 `StorageParam` 模型 (map[string]string)
- Handler 抽取 CREATE TABLE 和 ALTER TABLE 中的 WITH 选项
- Diff 对比存储参数差异
- 渲染: `WITH (fillfactor = 70, toast_tuple_target = 128)`

**估算**: 中 (4-5 文件, ~180 行) — **2-3 天**

---

### 2.4 ALTER INDEX ... RENAME / REINDEX

**ALTER INDEX RENAME** 与 RENAME COLUMN 模式完全一致：
- pg_query 有对应的 `RenameStmt` (`RenameStmt.renameType == OBJECT_INDEX`)
- 需要新 Handler + Mutation + Operation + Render
- **估算**: 小 (3 文件, ~60 行) — **1 天**

**REINDEX** 不涉及模型变更，只需渲染输出：
- **估算**: 小 (2 文件, ~30 行) — **0.5 天**

---

## T3: 复杂项目

这些特性需要新增数据结构、对比算法和渲染逻辑，涉及流水线中 3+ 个阶段。

---

### 3.1 分区表 (PARTITION BY)

| 维度 | 当前状态 |
|------|---------|
| **解析** | pg_query 在 `CreateStmt` 中包含 `PartitionBy` 和 `PartitionBoundSpec` |
| **模型** | 需要 `PartitionKey`, `PartitionBound` 等新类型 |
| **Diff** | 需要对比分区键和分区边界 |
| **渲染** | 需要生成 `PARTITION BY RANGE/LIST/HASH (col)` + 子表定义 |

**核心挑战**：
- PostgreSQL 分区表有两种实现方式：声明式分区（`PARTITION BY`）和表继承分区
- 分区表模型复杂：分区键（1-多列）、分区类型（RANGE/LIST/HASH）、分区边界
- Diff 引擎需要处理分区定义变化（如从 RANGE 改为 LIST）
- `ALTER TABLE ... ATTACH/DETACH PARTITION` 也需要支持

**估算**: 大 (5-6 文件, ~350 行) — **4-5 天**

---

### 3.2 表继承 (INHERITS)

| 维度 | 当前状态 |
|------|---------|
| **解析** | pg_query 在 `CreateStmt` 的 `InhRelations` 中包含父表列表 |
| **模型** | 需要 `Inherits []string` 字段在 Table 上 |
| **Diff** | 需要对比继承链变化 |
| **渲染** | 需要生成 `INHERITS (parent_table)` |

> **注意**: 表继承在 PostgreSQL 生态中使用场景较窄，且与分区表有重叠。如果已有分区表支持，继承可以通过分区表机制间接覆盖。

**估算**: 大 (4-5 文件, ~250 行) — **3-4 天**

---

## T4: 重大特性 — 全新 DDL 子领域

这些特性每个都是一个**独立的 DDL 子领域**，需要新建几乎完整的流水线（Parser Handler → Model → Mutation → Diff → Render），类比现存 `CREATE TABLE` 的体量。

| 特性 | 体量估算 | 特殊挑战 |
|------|---------|---------|
| **视图 / 物化视图** | 5-7 天 | 视图定义（SELECT 语句）的存储和对比；物化视图的索引、刷新策略 |
| **序列** | 3-5 天 | 独立的序列对象模型；CACHE/INCREMENT/MINVALUE 参数对比 |
| **触发器** | 5-7 天 | 触发器函数体存储；`pg_get_triggerdef` 依赖 |
| **函数 / 存储过程** | 8-15 天 | **体量最大**的类别；函数体（PL/pgSQL）的解析与对比；重载函数签名 |
| **扩展** | 2-3 天 | 主要涉及 `CREATE EXTENSION IF NOT EXISTS` 的渲染 |
| **行级安全策略** | 3-5 天 | `CREATE POLICY` 的 USING/WITH CHECK 表达式 |
| **全文搜索配置** | 5-7 天 | `TEXT SEARCH CONFIGURATION/DICTIONARY/PARSER` 等整组对象 |
| **域名 (DOMAIN)** | 2-4 天 | `CREATE DOMAIN` 约束条件的对比 |
| **排序规则 (COLLATION)** | 2-3 天 | 系统级 vs 用户级排序规则 |
| **表空间 (TABLESPACE)** | 2-3 天 | 集群级对象，跨数据库影响 |
| **自定义类型 (复合类型)** | 3-5 天 | `CREATE TYPE AS (col1 type1, ...)` |
| **EXTENSION / 行级安全 / CAST** | 各 2-4 天 | 依赖集群环境，Diff 意义有限 |

---

## 汇总矩阵

### 按 ROI 排序

| 优先级 | 特性 | 梯队 | 工作量 | 用户价值 | 依赖前置 |
|--------|------|------|--------|---------|---------|
| P0 | IDENTITY 列内省+渲染 | T1 | 0.5 天 | ⭐⭐⭐ 消除自省/渲染 gap | 无 |
| P0 | COLLATE 列支持 | T1 | 0.5 天 | ⭐⭐ 补齐常见语法 | 无 |
| P0 | ON DELETE/UPDATE 级联 | T1 | 1-2 天 | ⭐⭐⭐ 外键完整性的关键 gap | 无 |
| P0 | RENAME COLUMN | T1 | 1-2 天 | ⭐⭐⭐ 常见迁移场景 | 无 |
| P1 | 高级索引内省 | T1 | 1 天 | ⭐⭐ 消除自省信息丢失 | 无 |
| P1 | CREATE/DROP SCHEMA | T1 | 2-3 天 | ⭐⭐⭐ 多 schema 场景必需 | 无 |
| P1 | COMMENT ON | T2 | 2-3 天 | ⭐⭐ 元数据管理 | 无 |
| P2 | 存储参数 | T2 | 2-3 天 | ⭐ 小众但填补 gap | 无 |
| P2 | 枚举重命名/删除 | T2 | 2-3 天 | ⭐⭐ 枚举生命周期管理 | T1 枚举基础 |
| P2 | ALTER INDEX RENAME | T1 | 1 天 | ⭐ 较冷门 | 无 |
| P3 | 分区表 | T3 | 4-5 天 | ⭐⭐⭐ 大表场景核心功能 | 无 |
| P3 | 表继承 | T3 | 3-4 天 | ⭐ 使用场景窄 | 无 |
| P4 | 视图/物化视图 | T4 | 5-7 天 | ⭐⭐⭐ 常见对象类型 | 无 |
| P4 | 序列 | T4 | 3-5 天 | ⭐⭐ SERIAL 的显式替代 | 无 |
| P4 | 函数/存储过程 | T4 | 8-15 天 | ⭐⭐⭐ 但体量太大 | 无 |
| P5 | 触发器/策略/全文搜索 | T4 | 各 3-7 天 | ⭐⭐ 专业场景 | 无 |

### 工作量总计

| 梯队 | 特性数 | 合计人天 |
|------|--------|---------|
| T1: Quick Wins | 7 个 | ~8-12 天 |
| T2: 中等工程 | 4 个 | ~7-11 天 |
| T3: 复杂项目 | 2 个 | ~7-9 天 |
| T4: 重大特性 | 12 个 | ~45-75 天 |
| **总计** | **25 个特性** | **~67-107 天** |

---

## 附录: 核心架构参考

### 流水线接口

```go
// 为每个新特性需要实现或扩展的组件：
//
// 1. Parser:   type Handler interface { Handle(*pg_query.Node) ([]SchemaMutation, error) }
// 2. Mutation: type SchemaMutation interface { Kind(); Target(); Apply(*model.Schema) error }
// 3. Diff:     type Operation interface { Kind(); ObjectKey(); DependsOn(); IsDestructive() }
// 4. Render:   type SQLEngine interface { RenderAll([]diff.Operation) string }
//
// Handler 注册: HandlerRegistry (reflect.Type → Handler, 在 DefaultRegistry() 中注册)
// Mutation 应用: MutationApplier.ApplyAll(schema, mutations)
// Diff 执行:   Differ.Diff(source, target) → []Operation
// 计划编排:  diff.Planner.Plan(ops) → 三阶段 (Pre-deploy / Deploy / Post-deploy)
```

### 对象标识

```go
// 所有对象通过 ObjectKey{Schema, Name, Kind} 统一标识
// 现有 Kind: table, column, index, constraint, type, view, function, schema
// 新增 DDL 类型需要按需增加 ObjectKind 常量
```
