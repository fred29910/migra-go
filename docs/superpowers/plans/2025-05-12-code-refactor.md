# 代码优化实现计划

> **面向 AI 代理的工作者：** 必需子技能：使用 superpowers:subagent-driven-development（推荐）或 superpowers:executing-plans 逐任务实现此计划。步骤使用复选框（`- [ ]`）语法来跟踪进度。

**目标：** 根据 Go 设计原则评审结果，优化代码结构和可维护性

**架构：** 将 app.ComputeDiff 拆分为独立函数，提升单一职责；修复 lint 警告

**技术栈：** Go 1.26+

---

## 文件变更概览

| 文件 | 变更类型 | 职责 |
|------|----------|------|
| `internal/app/diff_service.go` | 重构 | 拆分 ComputeDiff 函数 |
| `internal/diff/differ.go` | 修复 | 移除未使用变量 |
| `internal/app/diff_service_test.go` | 扩展 | 添加新函数测试 |

---

## 任务清单

### 任务 1：修复未使用变量 lint 警告

**文件：**
- 修改：`internal/diff/differ.go:62`

- [ ] **步骤 1：查看当前代码**

```bash
cd /opt/codes/workspace/migra-go
rtk cat -n internal/diff/differ.go | sed -n '55,70p'
```

- [ ] **步骤 2：修复未使用变量**

将 `internal/diff/differ.go:62` 的：
```go
_ = source.Schemas[name]
c.warnf("namespace drop is not implemented yet (ignored): %s", name)
```

修改为：
```go
c.warnf("namespace drop is not implemented yet (ignored): %s", name)
```

- [ ] **步骤 3：运行 lint 验证**

```bash
cd /opt/codes/workspace/migra-go
make lint
```

预期：无 lint 警告

- [ ] **步骤 4：运行测试验证**

```bash
cd /opt/codes/workspace/migra-go
make test
```

预期：所有测试通过

- [ ] **步骤 5：Commit**

```bash
git add internal/diff/differ.go
git commit -m "fix: remove unused variable in differ.go"
```

---

### 任务 2：拆分 ComputeDiff 函数职责

**文件：**
- 修改：`internal/app/diff_service.go`
- 扩展：`internal/app/diff_service_test.go`

#### 2.1 拆分 NormalizeSchemas

- [ ] **步骤 1：添加 NormalizeSchemas 函数**

在 `internal/app/diff_service.go` 的 `ComputeDiff` 函数前添加：

```go
// NormalizeSchemas normalizes source and target schemas in place
func NormalizeSchemas(source, target *model.Schema) error {
    if err := normalize.CanonicalizeSchema(source); err != nil {
        return fmt.Errorf("failed to normalize source schema: %w", err)
    }
    if err := normalize.CanonicalizeSchema(target); err != nil {
        return fmt.Errorf("failed to normalize target schema: %w", err)
    }
    return nil
}
```

- [ ] **步骤 2：添加 FilterDestructiveOps 函数**

在 `NormalizeSchemas` 函数后添加：

```go
// FilterDestructiveOps filters operations based on destructive flag
func FilterDestructiveOps(ops []diff.Operation, unsafeDrop bool, cfg Config) ([]diff.Operation, []string) {
    if unsafeDrop {
        return ops, nil
    }

    filtered := make([]diff.Operation, 0, len(ops))
    warnings := make([]string, 0, 4)

    for _, op := range ops {
        if !op.IsDestructive() {
            filtered = append(filtered, op)
        }
    }

    destructiveCount := len(ops) - len(filtered)
    if destructiveCount > 0 {
        warnings = append(warnings, fmt.Sprintf("%d destructive operation(s) detected!", destructiveCount))
        for _, op := range ops {
            if op.IsDestructive() {
                warnings = append(warnings, fmt.Sprintf("  - %s: %s (destructive)", op.Kind(), op.ObjectKey()))
            }
        }
        warnings = append(warnings, "Use --unsafe-drop to include destructive DROP operations in output")
    }

    return filtered, warnings
}
```

- [ ] **步骤 3：添加 BuildExecutionPlan 函数**

在 `FilterDestructiveOps` 函数后添加：

```go
// BuildExecutionPlan creates an ordered execution plan from operations
func BuildExecutionPlan(ops []diff.Operation, unsafeDrop bool) ([]diff.Operation, error) {
    planner := plan.NewPlanner(unsafeDrop)
    stages := planner.Plan(ops)

    stageOrder := []plan.Stage{
        plan.StagePreDeploy,
        plan.StageDeploy,
        plan.StagePostDeploy,
    }

    allOps := make([]diff.Operation, 0, len(ops))
    for _, stage := range stageOrder {
        stageOps := stages[stage]
        if len(stageOps) == 0 {
            continue
        }
        sortedStageOps, err := plan.TopoSort(stageOps)
        if err != nil {
            return nil, fmt.Errorf("failed to topologically sort %s operations: %w", stage, err)
        }
        allOps = append(allOps, sortedStageOps...)
    }

    return allOps, nil
}
```

- [ ] **步骤 4：重构 ComputeDiff 函数**

将原来的 `ComputeDiff` 函数简化为调用上述三个函数：

```go
// ComputeDiff runs the normalize -> diff -> plan pipeline and returns operations and warnings
func ComputeDiff(source, target *model.Schema, cfg Config) ([]diff.Operation, []string, error) {
    // Normalize schemas
    if err := NormalizeSchemas(source, target); err != nil {
        return nil, nil, err
    }

    // Diff schemas
    differ := diff.NewDiffer()
    operations, warnings := differ.Diff(source, target)

    // Filter destructive operations
    filteredOps, filterWarnings := FilterDestructiveOps(operations, cfg.UnsafeDrop, cfg)
    warnings = append(warnings, filterWarnings...)

    // Build execution plan
    sortedOps, err := BuildExecutionPlan(filteredOps, cfg.UnsafeDrop)
    if err != nil {
        return nil, nil, err
    }

    return sortedOps, warnings, nil
}
```

- [ ] **步骤 5：运行测试验证**

```bash
cd /opt/codes/workspace/migra-go
make test
```

预期：所有测试通过

- [ ] **步骤 6：Commit**

```bash
git add internal/app/diff_service.go
git commit -m "refactor: split ComputeDiff into focused functions"
```

#### 2.2 添加单元测试

- [ ] **步骤 1：查看现有测试文件**

```bash
cd /opt/codes/workspace/migra-go
cat internal/app/diff_service_test.go
```

- [ ] **步骤 2：添加 NormalizeSchemas 测试**

在 `internal/app/diff_service_test.go` 添加：

```go
func TestNormalizeSchemas(t *testing.T) {
    source := model.NewSchema()
    source.GetOrCreateNamespace("public").Tables["users"] = &model.Table{
        Name: "users",
    }

    target := model.NewSchema()
    target.GetOrCreateNamespace("public")

    err := NormalizeSchemas(source, target)
    require.NoError(t, err)
}
```

- [ ] **步骤 3：添加 FilterDestructiveOps 测试**

```go
func TestFilterDestructiveOps(t *testing.T) {
    ops := []diff.Operation{
        diff.NewAddTableOp("public", "new_table", &model.Table{Name: "new_table"}),
        diff.NewDropTableOp("public", "old_table"),
    }

    // Test with unsafeDrop=false - should filter out destructive
    filtered, warnings := FilterDestructiveOps(ops, false, app.Config{})
    require.Len(t, filtered, 1)
    require.Len(t, warnings, 2) // 1 count message + 1 detail + 1 usage hint

    // Test with unsafeDrop=true - should keep all
    filteredAll, warningsAll := FilterDestructiveOps(ops, true, app.Config{})
    require.Len(t, filteredAll, 2)
    require.Len(t, warningsAll, 0)
}
```

- [ ] **步骤 4：添加 BuildExecutionPlan 测试**

```go
func TestBuildExecutionPlan(t *testing.T) {
    ops := []diff.Operation{
        diff.NewAddTableOp("public", "users", &model.Table{Name: "users"}),
        diff.NewAddColumnOp("public", "users", &model.Column{Name: "id", DataType: "integer"}),
    }

    result, err := BuildExecutionPlan(ops, true)
    require.NoError(t, err)
    require.Len(t, result, 2)

    // Verify add_table comes before add_column (dependency)
    require.Equal(t, diff.KindAddTable, result[0].Kind())
}
```

- [ ] **步骤 5：运行测试验证**

```bash
cd /opt/codes/workspace/migra-go
make test
```

预期：所有测试通过，包括新增测试

- [ ] **步骤 6：Commit**

```bash
git add internal/app/diff_service_test.go
git commit -m "test: add unit tests for refactored functions"
```

---

### 任务 3：改进 Panic 恢复错误信息（可选）

**文件：**
- 修改：`internal/parser/parser.go:97-106`

- [ ] **步骤 1：查看当前代码**

```bash
cd /opt/codes/workspace/migra-go
rtk cat -n internal/parser/parser.go | sed -n '95,110p'
```

- [ ] **步骤 2：增强错误信息**

将：
```go
defer func() {
    if r := recover(); r != nil {
        err = &ParseError{
            Message:  fmt.Sprintf("recovered from panic: %v", r),
            Position: -1,
        }
    }
}()
```

修改为：
```go
defer func() {
    if r := recover(); r != nil {
        err = &ParseError{
            Message:  fmt.Sprintf("recovered from panic: %v", r),
            Position: -1,
        }
        // Note: In production, consider logging the stack trace
        // log.Printf("Parser panic recovery: %v\n%s", r, debug.Stack())
    }
}()
```

- [ ] **步骤 3：运行测试验证**

```bash
cd /opt/codes/workspace/migra-go
make test
```

预期：测试通过

- [ ] **步骤 4：Commit（可选）**

如果觉得改动有意义：

```bash
git add internal/parser/parser.go
git commit -m "refactor: enhance panic recovery error message"
```

---

## 执行交接

**计划已完成并保存到 `docs/superpowers/plans/2025-05-12-code-refactor.md`。两种执行方式：**

**1. 子代理驱动（推荐）** - 每个任务调度一个新的子代理，任务间进行审查，快速迭代

**2. 内联执行** - 在当前会话中使用 executing-plans 执行任务，批量执行并设有检查点

**选哪种方式？**