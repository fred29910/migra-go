# 修复方案评审：多 Schema Diff — CreateSchemaStmt 支持 + --schema 过滤

> **评审时间**：2026-05-15
> **评审对象**：`docs/superpowers/plans/2026-05-15-multi-schema-diff-fix.md`
> **对应 Bug**：`docs/bugs/2026-05-15-multi-schema-diff-create-schema-stmt.md`

---

## 总结

整体方案思路清晰，两个根因定位准确，任务拆分合理，遵循 TDD 先红后绿的开发节奏。方案在架构层面是合理的，在实现细节上存在 **1 个建议修改的问题** 和 **3 个优化建议**，方案可以直接进入实施阶段，无需阻塞。

---

## [建议修改] 问题 1：`CreateSchemaHandler` 应提取局部变量避免直接解引用

**位置**：方案 2c 步骤 4，`create_schema_handler.go` 实现代码

**问题**：方案中在返回语句里直接解引用 `*stmt.Schemaname`：

```go
return []SchemaMutation{CreateSchemaMutation{Schema: *stmt.Schemaname}}, nil
```

虽然 nil 检查已经在前一行完成，但将解引用提取为局部变量可读性更好，也符合 Go 的惯用写法。

**建议**：

```go
if stmt.Schemaname == nil {
    return nil, fmt.Errorf("CreateSchemaHandler: schema name is nil")
}
schemaName := *stmt.Schemaname
return []SchemaMutation{CreateSchemaMutation{Schema: schemaName}}, nil
```

**另外注意**：`CreateSchemaStmt` 的 `SchemaElts` 字段（schema 子元素，如 `CREATE SCHEMA auth CREATE TABLE ...`）当前方案未处理。对于测试数据中的简单 `CREATE SCHEMA auth;` 语句，`SchemaElts` 为空，不影响功能。但建议在代码注释中记录此限制，以便后续扩展。

---

## [建议修改] 问题 2：`FilterNamespaces` 缺少对空结果的警告

**位置**：方案 3b 步骤 3，`FilterNamespaces` 函数实现

**问题**：当用户拼写错误 schema 名（如 `--schema pulbic`）时，`FilterNamespaces` 会过滤掉所有 namespace，导致 diff 结果为空，用户可能误以为没有差异。

**建议**：在 `ComputeDiff` 中，如果过滤后 source 和 target 的 Schemas 都为空，返回一个警告：

```go
if len(source.Schemas) == 0 && len(target.Schemas) == 0 {
    warnings = append(warnings, "no schemas matched the filter -- check --schema flag")
}
```

---

## [建议修改] 问题 3：集成测试需确认 `newDefaultDeps()` 存在

**位置**：方案 4 步骤 1，`TestMultiSchemaDiff` 集成测试

**问题**：方案中的测试代码使用了 `app.NewDiffService(newDefaultDeps())`。经检查，`cmd/migra/integration_test.go` 中已有 `newDefaultDeps()` 函数（在 `diff_runner.go` 中定义），现有测试 `TestExampleSQLFilesDiffIncludesEnumAndConstraints` 也在使用。**确认此函数已存在，方案无需额外修改。**

但需注意：集成测试中 `newDefaultDeps()` 的 `LoadSchema` 字段绑定的是 `loadSchemaWithContext`，它会调用 `sourceRegistry.Load()`，走真实的加载路径。这意味着集成测试会真正解析包含 `CREATE SCHEMA` 的 SQL 文件，是对修复效果的完整验证。

---

## [建议修改] 问题 4：幂等性测试可增加 namespace 数量断言

**位置**：方案 2d 步骤 6，`TestCreateSchemaMutation_Apply_Idempotent` 测试

**建议**：在幂等性测试中增加对 namespace 数量的验证，确保不会创建重复的 namespace：

```go
func TestCreateSchemaMutation_Apply_Idempotent(t *testing.T) {
    schema := model.NewSchema()
    schema.GetOrCreateNamespace("auth")

    mut := CreateSchemaMutation{Schema: "auth"}
    err := mut.Apply(schema)
    require.NoError(t, err, "CreateSchemaMutation should be idempotent")

    // 验证只有一个 auth namespace
    require.Len(t, schema.Schemas, 1)
}
```

---

## 审查清单

- [x] 每条评论都标注了优先级
- [x] 所有问题都给出了具体的修复建议或说明
- [x] 没有因为面子而跳过关键问题
- [x] 没有纠结于工具能自动处理的风格问题
- [x] 对好的代码给予了肯定
- [x] 给出了整体总结

---

## 详细评审意见

### ✅ 方案优点

1. **根因分析准确**：正确识别了两个独立问题（Parser 缺少 handler + `--schema` 过滤对文件源无效）
2. **任务拆分合理**：6 个任务按依赖关系排序，先模型常量 → handler → 过滤 → 测试 → 文档 → 验证
3. **TDD 开发节奏**：每个实现任务都先写测试（红），再实现（绿），符合项目规范
4. **向后兼容**：`FilterNamespaces` 在 `schemas` 为空时不过滤，不影响现有行为
5. **测试覆盖全面**：Handler 测试、Mutation 测试、FilterNamespaces 测试、集成测试一应俱全
6. **Commit 粒度合理**：每个任务独立 commit，便于回滚和审查

### ⚠️ 需要关注的设计决策

1. **`CreateSchemaMutation` 的 `Target()` 返回值**：方案中返回 `model.NewObjectKey(m.Schema, "", model.KindSchema)`，Name 字段为空字符串。这与其它 Mutation 的 `Target()` 不同（如 `CreateTableMutation` 返回 `NewObjectKey(schema, name, kind)`）。这是合理的，因为 schema 名就是 ObjectKey 的 Schema 字段本身。由于 `CreateSchemaMutation` 不参与 diff 比较（它只是确保 namespace 存在），`Target()` 的 Name 字段不会被 diff 引擎使用，因此空字符串是安全的。

2. **`FilterNamespaces` 放在 `NormalizeSchemas` 之后**：这个顺序是正确的。先规范化（类型映射、表达式规范化），再过滤 namespace。

3. **`--schema` 过滤在 diff 阶段而非加载阶段**：这是正确的架构决策。文件源加载时无法预知用户要比较哪些 schema，过滤应在 diff 计算阶段进行。

---

## 结论

**方案整体可行，无需阻塞即可进入实施阶段。** 4 个建议修改的问题可以在实施过程中一并处理，不阻塞方案审批。核心实现思路正确，TDD 节奏合理，测试覆盖充分。
