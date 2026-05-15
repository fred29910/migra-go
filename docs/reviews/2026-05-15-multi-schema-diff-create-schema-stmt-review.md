# 代码评审报告：多 Schema diff — `CreateSchemaStmt` 修复

> **评审日期:** 2026-05-15
> **评审人:** Roo (AI Code Reviewer)
> **评审范围:** `docs/bugs/2026-05-15-multi-schema-diff-create-schema-stmt.md` 所述 Bug 的修复代码
> **评审依据:** `.agents/skills/chinese-commit-conventions/SKILL.md` 规范

---

## 一、变更概览

| 文件 | 变更类型 | 说明 |
|------|---------|------|
| `internal/parser/create_schema_handler.go` | **新增** | `CreateSchemaHandler` 处理 `CREATE SCHEMA` AST 节点 |
| `internal/parser/registry.go` | 修改 | `DefaultRegistry` 注册 `CreateSchemaStmt` → `CreateSchemaHandler` |
| `internal/parser/mutation.go` | 修改 | 新增 `MutKindCreateSchema` 常量 |
| `internal/model/object_key.go` | 修改 | 新增 `KindSchema` 常量 |
| `internal/app/diff_service.go` | 修改 | 新增 `FilterNamespaces` 函数；`ComputeDiff` 中调用 |
| `internal/parser/handler_test.go` | 修改 | 新增 `TestCreateSchemaHandler` + 类型安全检查 |
| `internal/parser/mutation_test.go` | 修改 | 新增 `TestCreateSchemaMutation_Apply` + `_Idempotent` |
| `internal/app/diff_service_test.go` | 修改 | 新增 `TestFilterNamespaces_*` |
| `cmd/migra/integration_test.go` | 修改 | 新增 `TestMultiSchemaDiff` |

---

## 二、架构设计与实现评审

### 2.1 整体架构评价 ✅

修复方案遵循了项目既有的 **Handler + Mutation + Registry** 架构模式：

```
SQL → Parser → HandlerRegistry → CreateSchemaHandler → CreateSchemaMutation → model.Schema
```

这一模式与现有的 `CreateTableHandler`、`AlterTableHandler`、`CreateEnumHandler`、`CreateIndexHandler` 完全一致，**设计一致性良好**。

### 2.2 `CreateSchemaHandler` 实现评审

**文件:** [`internal/parser/create_schema_handler.go`](internal/parser/create_schema_handler.go)

**优点：**
- Handler 职责单一：仅从 AST 提取 `Schemaname`，返回 `CreateSchemaMutation`
- `CreateSchemaMutation.Apply()` 调用 `schema.GetOrCreateNamespace()`，利用了已有的幂等机制，**无需重复创建检查**
- 注释说明了 `SchemaElts` 不在此处理的原因（pg_query_go 已将其作为独立 AST 节点返回），**设计决策有文档化**

**潜在改进点：**
1. `Handle()` 方法中 `node.(pg_nodes.CreateSchemaStmt)` 类型断言失败时返回的 error message 包含了 `%T` 格式化，但在 Go 1.18+ 中建议使用 `any` 类型替代 `pg_nodes.Node` 接口以获得更好的类型安全。当前代码使用 `pg_nodes.Node` 是为了与 `Handler` 接口签名保持一致，**属于合理约束，不建议修改**。
2. `CreateSchemaMutation` 没有携带额外元数据（如 `ifNotExists` 标志），当前实现通过 `GetOrCreateNamespace` 实现了隐式幂等，**这是合理的简化**。

### 2.3 `FilterNamespaces` 实现评审

**文件:** [`internal/app/diff_service.go`](internal/app/diff_service.go:79-104)

**优点：**
- 放置在 `ComputeDiff` 的 Normalize 之后、Diff 之前，**过滤时机正确**——既避免了加载层的复杂性，又确保了 diff 计算前 schema 已精简
- `schemas` 为空时直接返回 nil，不过滤，**保持了向后兼容**
- 返回 warning 而非 error，当过滤后无 schema 匹配时给出友好提示

**潜在改进点：**
1. 当前 `FilterNamespaces` 直接修改传入的 `*model.Schema` 对象（原地删除），虽然函数名暗示了"过滤"语义，但**调用方可能不期望原始 schema 被修改**。建议在函数文档中明确标注"in-place modification"，或改为返回新的 schema 副本。
2. 缺少对 `source.Schemas` 和 `target.Schemas` 同时为空的边界情况处理——当前仅在两者都为空时返回 warning，如果仅 source 为空或仅 target 为空，不会产生 warning。这可能是**预期行为**（因为 diff 逻辑本身能处理单侧为空），但值得在文档中说明。

### 2.4 `Registry` 模式评审

**文件:** [`internal/parser/registry.go`](internal/parser/registry.go)

**优点：**
- 使用 `reflect.Type` 作为 key 的注册表模式，使得新增 DDL 类型只需 3 步：实现 Handler、定义 Mutation、注册
- `DefaultRegistry()` 集中管理所有内置 handler，**扩展点清晰**

**潜在改进点：**
1. `Register()` 使用 `reflect.TypeOf(nodeType)`，其中 `nodeType` 是值类型而非指针。对于大型 AST 节点类型，reflect 操作可能有性能开销，但在初始化阶段只执行一次，**可以接受**。
2. 当前没有提供 `Unregister()` 或 `ListRegistered()` 方法，对于调试和插件化场景可能有用，但当前非必需。

---

## 三、代码质量评审

### 3.1 命名规范 ✅

| 项目 | 评价 |
|------|------|
| `CreateSchemaHandler` | 符合 Handler 命名惯例 |
| `CreateSchemaMutation` | 符合 Mutation 命名惯例 |
| `MutKindCreateSchema` | 符合常量命名惯例 |
| `KindSchema` | 符合 ObjectKind 命名惯例 |
| `FilterNamespaces` | 动词+名词，语义清晰 |

所有命名均与项目现有风格一致，**无中英混杂问题**。

### 3.2 错误处理 ✅

- `Handle()` 对 nil `Schemaname` 进行了防御性检查
- `Apply()` 利用 `GetOrCreateNamespace` 的幂等性避免了重复创建错误
- `FilterNamespaces` 对空 schemas 列表进行了短路返回

**建议补充：** `FilterNamespaces` 中 `delete(s.Schemas, name)` 后没有检查删除后的 map 状态，如果后续代码依赖 `source.Schemas[name]` 是否存在，可能导致 nil pointer。当前 diff 逻辑通过 `diffSchemas` 中的 `exists` 检查规避了此问题，**但属于隐性依赖**。

### 3.3 测试覆盖评审 ✅

| 测试文件 | 覆盖内容 | 评价 |
|----------|---------|------|
| `handler_test.go` | `TestCreateSchemaHandler` + 类型安全检查 | ✅ 正向 + 反向用例 |
| `mutation_test.go` | `TestCreateSchemaMutation_Apply` + `_Idempotent` | ✅ 正常 + 幂等性 |
| `diff_service_test.go` | `TestFilterNamespaces_*` (4 个用例) | ✅ 空列表、过滤、不存在 schema、warning |
| `integration_test.go` | `TestMultiSchemaDiff` | ✅ 端到端集成测试 |

**测试充分性评价：** 单元测试覆盖了正常路径和边界条件，集成测试验证了端到端流程。**建议补充**：
1. `CREATE SCHEMA IF NOT EXISTS` 的解析测试（当前仅测试 `CREATE SCHEMA auth`）
2. `FilterNamespaces` 在 schemas 列表包含不存在 schema 名时的行为（已有 `TestFilterNamespaces_NonExistentSchema` 覆盖）

### 3.4 性能考量

- `FilterNamespaces` 使用 `map[string]bool` 做集合查找，时间复杂度 O(n)，**高效**
- `CreateSchemaMutation.Apply()` 调用 `GetOrCreateNamespace`，时间复杂度 O(1)，**高效**
- 整体无额外内存分配热点

---

## 四、问题与建议汇总

### 严重程度：高

| # | 问题 | 文件 | 建议 |
|---|------|------|------|
| 1 | `FilterNamespaces` 原地修改传入的 `*model.Schema`，副作用不明确 | `internal/app/diff_service.go:89-94` | 在 godoc 中明确标注 in-place，或返回新对象 |

### 严重程度：中

| # | 问题 | 文件 | 建议 |
|---|------|------|------|
| 2 | 缺少 `CREATE SCHEMA IF NOT EXISTS` 的测试用例 | `internal/parser/handler_test.go` | 补充 `TestCreateSchemaHandler_IfNotExists` |
| 3 | `FilterNamespaces` 对 source/target 单侧为空无 warning | `internal/app/diff_service.go:100-103` | 考虑增加单侧为空的边界提示 |

### 严重程度：低（优化建议）

| # | 问题 | 文件 | 建议 |
|---|------|------|------|
| 4 | `FilterNamespaces` 中 `delete` 操作后的 map 状态无验证 | `internal/app/diff_service.go:91-93` | 可添加防御性 nil 检查 |
| 5 | `Registry` 缺少 `Unregister`/`ListRegistered` 方法 | `internal/parser/registry.go` | 未来插件化场景可能需要 |

---

## 五、合规性检查

### 5.1 提交规范（chinese-commit-conventions）

根据 `.agents/skills/chinese-commit-conventions/SKILL.md` 规范检查：

- [x] **type**: `fix`（缺陷修复）
- [x] **scope**: `parser` / `diff-service`（影响模块明确）
- [x] **description**: 应使用中文动宾短语，如「修复多 Schema diff 中 CreateSchemaStmt 解析失败问题」
- [x] **body**: 需说明根因（缺少 handler + schema 过滤未生效）和修复方案
- [ ] **footer**: 涉及 schema 结构变更，建议标注 `BREAKING CHANGE`（如果 `CREATE SCHEMA` 在某些场景下改变了 diff 输出语义）

### 5.2 CHANGELOG 更新

建议在 `CHANGELOG.md` 中新增条目：

```markdown
## [未发布] - 2026-05-15

### 修复
- fix(parser): 支持 CREATE SCHEMA 语句解析
- fix(diff-service): 修复 --schema 标志对文件源无效的问题
```

---

## 六、结论

**评审结论：通过 ✅**

本次修复正确识别了两个独立的根因（Parser 缺少 handler + Schema 过滤未对文件源生效），分别给出了合理的解决方案。代码实现遵循了项目既有的架构模式，测试覆盖充分，命名规范统一。建议在合并前补充 `CREATE SCHEMA IF NOT EXISTS` 的测试用例，并在 godoc 中明确 `FilterNamespaces` 的原地修改行为。

**建议的合入顺序：**
1. 先合并 Parser 相关变更（`create_schema_handler.go` + `registry.go` + `mutation.go`）
2. 再合并 Diff Service 变更（`diff_service.go` + 测试）
3. 最后更新文档和 CHANGELOG