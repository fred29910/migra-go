# 代码评审报告 — feat/full_ddl 分支

**日期**：2026-05-19
**分支**：`feat/full_ddl`
**提交数**：27
**变更文件数**：62（+9370 / -388）

---

## 概述

本分支新增了 5 大 DDL 特性支持：

1. **COLLATE 排序规则** — 列排序规则的解析、检测与渲染
2. **IDENTITY 列** — `GENERATED ALWAYS/BY DEFAULT AS IDENTITY` 的完整支持
3. **FK ON DELETE/ON UPDATE** — 外键级联动作的解析、内省与渲染
4. **CREATE/DROP SCHEMA** — Schema 的创建与删除
5. **RENAME COLUMN** — 列重命名（含启发式匹配）

整体架构清晰，parser → model → diff → render 的分层设计保持一致，测试覆盖较为充分。

---

## 值得肯定之处

1. **架构一致性**：每个新特性都遵循了相同的模式 — model 字段扩展 → parser 提取 → diff 检测 → render 输出 → plan 阶段分配，扩展方式非常规整。
2. **测试覆盖全面**：每个新特性都有对应的单元测试，包括正常路径和边界情况（如 `RenameColumnMutation_Apply_TableNotFound`、`Apply_NameCollision`、`Apply_NilNamespace`）。
3. **防御性编程**：`RenameColumnMutation.Apply` 对 placeholder 表、列不存在、名称碰撞都有明确处理。
4. **FK 动作码映射**：`fkActionCode` 和 `pgConstraintAction` 两个函数逻辑一致且注释清晰。

---

## 问题清单

### 1. [建议修改] FK 动作码映射函数重复定义

`fkActionCode` 在 `internal/parser/create_table_handler.go:28` 和 `pgConstraintAction` 在 `internal/introspect/constraints.go:85` 实现了完全相同的映射逻辑。

**建议**：提取到公共位置（如 `internal/model` 或 `internal/util`），避免后续维护时两处不同步。

---

### 2. [建议修改] `isColumnRenameCandidate` 启发式匹配可能产生误判

`internal/diff/diff_tables.go:162` 中 `isColumnRenameCandidate` 仅通过 `DataType`、`IsNullable`、`DefaultExpr` 三个字段判断是否为重命名。

**场景**：如果用户同时将一个 `text NOT NULL DEFAULT 'foo'` 列改为 `text NOT NULL DEFAULT 'bar'` 并新增一个 `text NOT NULL DEFAULT 'foo'` 列，算法会将后者误判为重命名目标。

**建议**：
- 增加更多匹配维度（如列位置 `ordinal_position`、collation 等）
- 或者在注释中明确说明这是启发式匹配，存在误判风险，未来可考虑让用户显式声明重命名

---

### 3. [建议修改] `AlterColumnCollationOp` 渲染使用了 `SET DATA TYPE` 而非 `COLLATE`

`internal/render/render.go:320` 中渲染 collation 变更使用了：

```sql
ALTER TABLE ... ALTER COLUMN ... SET DATA TYPE <type> COLLATE <collation>
```

PostgreSQL 中更标准的写法是：

```sql
ALTER TABLE ... ALTER COLUMN ... COLLATE <collation>
```

`SET DATA TYPE` 会触发全表重写和类型校验，而 `COLLATE` 仅修改排序规则。除非确实需要同时改数据类型，否则建议分开处理。

---

### 4. [建议修改] `alter_table_handler.go` 中 RENAME COLUMN 被静默忽略

在 `internal/parser/alter_table_handler.go:28` 的 `switch cmd.Subtype` 中，没有处理 `AT_RenameColumn` 类型。虽然 `RenameStmtHandler` 通过 `Node_RenameStmt` 处理了顶层 `ALTER TABLE ... RENAME COLUMN`，但如果 pg_query 在某些场景下将其解析为 `AlterTableCmd` 子命令而非独立 `RenameStmt`，则会被 `default` 分支静默忽略（仅打印 warning）。

**建议**：在 `default` 分支中明确记录未处理的 subtype，或者添加一个 `AT_RenameColumn` case 做防御性处理。

---

### 5. [仅供参考] `defaultConstraintName` 生成的约束名可能冲突

`internal/parser/create_table_handler.go:25` 中 `defaultConstraintName` 使用 `table + "_constraint"` 作为默认名。对于包含多个 FK 约束的表，所有 FK 都会生成相同的名称 `<table>_constraint`。

**当前影响**：由于这些约束名仅用于内部模型匹配（不直接输出到 SQL），暂时不会导致问题。但如果未来需要输出约束名到 DDL，需要改进命名策略（如 `<table>_<col>_fkey`）。

---

### 6. [仅供参考] `operation_test.go` 中 `CreateSchemaOp` 和 `DropSchemaOp` 未加入接口检查

`internal/diff/operation_test.go:35` 的 `TestOperationInterfaceHasDependsOn` 中，`CreateSchemaOp` 和 `DropSchemaOp` 没有被加入 `case` 分支进行依赖项检查。虽然它们确实实现了 `DependsOn()`（返回空切片），但测试没有覆盖到。

**建议**：在测试中补充这两个类型的接口合规检查。

---

### 7. [仅供参考] `collationName` 变量命名风格

`internal/introspect/tables.go:44` 中 `isIdentity` 是 `string` 类型（`"YES"/"NO"`）而非布尔值，与其他 `IsNullable`（bool）的命名风格略有不同。这不是问题，只是值得注意。

---

## 总结

整体实现思路清晰，代码质量良好，测试覆盖充分。5 个新特性的扩展方式高度一致，体现了良好的架构设计。

**建议处理优先级**：

1. **优先修复**：第 3 点（Collation SQL 语义问题）— 直接影响生成 SQL 的正确性
2. **本次或下次迭代**：第 1 点（代码重复）、第 2 点（启发式匹配准确性）、第 4 点（防御性处理）
3. **后续优化**：第 5、6、7 点
