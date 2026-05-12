# DDL 缺口补齐设计 — 评审报告

**文档**: `docs/superpowers/specs/2026-05-12-example-ddl-gap-closure-design.md`  
**评审日期**: 2026-05-12  
**项目**: migra-go

---

## 总体评价

设计定位清晰合理，采用"示例闭环优先"的务实范围控制，无范围蔓延。与代码库现状高度吻合。

---

## 一、设计与现状吻合度（优秀）

| 设计项 | 当前现状 | 评价 |
|--------|---------|------|
| `CreateEnumHandler` 修复 | `CreateEnumTypeMutation` struct 已存在但 `Apply()` 返回错误 | **精准对齐**，设计复用现有 struct |
| `CreateTableHandler` 解析 PK/FK/默认值 | 当前 handler 跳过约束 | **精准对齐** |
| `renderAddTable` 输出 PK 和约束 | 当前有 `// TODO: Add primary key, constraints` | **精准对齐** |
| `SetDefaultOp` / `DropDefaultOp` | `diffColumn` 中默认值变更仅 warn | **精准对齐** |
| `DropColumnOp` | `diffTableColumns` 中列删除仅 warn | **精准对齐** |

---

## 二、关键风险

### 风险 1：约束定义（Definition）字符串稳定性

设计将约束 `Definition` 定义为"可直接拼入 SQL 的片段"（如 `PRIMARY KEY ("id")`），但 `pg_query_go` 的 deparse 输出格式可能因版本变化。

**建议**: 约束定义输出放在 render 层重新组装，而非依赖 AST 的 deparse 字符串。在 parser 层只提取语义（columns, ref_table, ref_columns），由 render 层拼 SQL。

### 风险 2：`AddEnumLabelOp` 的 `DependsOn` 细化

设计说"enum label 依赖 enum type"，但 enum type 的 `ObjectKey` 与 table 不同。`AddEnumTypeOp` 当前无 `DependsOn`，新增的 `AddEnumLabelOp` 若依赖 type 而 type 在本轮才创建，需确保 planner 排序正确。

### 风险 3：ALTER ADD COLUMN 与 CREATE TABLE 的列默认值处理一致性

`CreateTableMutation.Apply()` 当前只写 `Columns`，不写 `DefaultExpr`（因为 parser 没提取）。需要确保两处 handler 同时推进。

### 风险 4：验收命令中的 `rtk` 前缀

验收命令使用 `rtk`（Rust Token Killer），但 `Makefile` 中不含此前缀。需确认 `rtk` 是全局可用命令。

---

## 三、设计建议

### 建议 1：约束定义拼装替代方案

当前方案：
```
Constraint.Definition = "PRIMARY KEY (\"id\")"  // 来自 AST deparse
```

建议方案：
```
Constraint.Definition = ""  // parser 不写，render 时按约束类型 + Columns + RefTable 拼装
```

优点：不受 `pg_query_go` 序列化格式影响，golden 测试稳定。

### 建议 2：`DropColumnOp` 的破坏性风险量化

设计标记 `DropColumnOp` 为 `risk:high`，但未讨论 CASCADE 策略。建议：
- 默认输出 `DROP COLUMN ...`（非 CASCADE）
- 在文档中注明 `--unsafe-drop` 不自动加 CASCADE

### 建议 3：enum 非尾部追加的 warning 格式

设计保守合理。建议 warning 格式包含具体 label 信息：
```
enum "public"."user_role": label "guest" removed or reordered, skipping (manual migration required)
```

---

## 四、TDD 要求

设计明确提出"先写失败测试，再实现"，符合 TDD 原则。建议执行顺序：

1. Parser 测试（红）→ 实现（绿）
2. Diff 测试（红）→ 实现（绿）
3. Render 测试（红）→ 实现（绿）
4. CLI 端到端测试 → 验收

---

## 五、验收条件清晰度

4 条验收命令明确可执行，验收标准具体：

- `CREATE TABLE "public"."comments"` 出现在输出 ✅
- `ALTER TABLE "public"."users" ADD COLUMN "age" integer` ✅
- `ALTER TYPE "public"."user_role" ADD VALUE 'guest'` ✅
- stderr 不含 `CREATE TYPE ENUM is not yet supported` ✅

无遗漏。

---

## 六、结论

**设计质量：良+（B+）**

- 范围控制得当，无范围蔓延 ✅
- 与现有代码结构高度吻合 ✅
- 设计文档详细程度充足 ✅
- 验收标准明确 ✅
- **主要建议**：约束 Definition 的拼装策略需重新考量（风险 1）
