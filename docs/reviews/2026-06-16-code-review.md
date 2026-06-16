# 生产就绪代码审查报告

**评审时间**：2026-06-16
**评审分支**：当前工作区（HEAD: `d7aa1ea`）
**评审范围**：当前暂存区变更（`git diff --cached`），共 12 个文件，+141 / -2405 行
**评审方法**：静态审查 + 构建/测试验证 + 已删文件回溯比对

---

## 1. 变更文件清单

| 文件 | 状态 | 变更量 | 说明 |
|------|------|--------|------|
| `cmd/migra/diff.go` | M | -4 | 删除注释 |
| `cmd/migra/main.go` | M | -3 | 删除注释 |
| `cmd/migra/push.go` | M | +142/-0 | 重写 push 命令，委托给 `internal/app/push` |
| `cmd/migra/diff_runner_test.go` | D | -625 | 删除 diff runner 测试 |
| `cmd/migra/diff_test.go` | D | -152 | 删除 diff 命令测试 |
| `cmd/migra/push_runner.go` | D | -416 | 删除 push 执行逻辑（迁移到 internal） |
| `cmd/migra/push_runner_test.go` | D | -813 | 删除 push runner 测试 |
| `cmd/migra/push_test.go` | D | -280 | 删除 push 命令测试 |
| `docs/reviews/2026-06-03-code-review.md` | D | -106 | 删除旧评审报告 |
| `internal/app/diff_service.go` | M | +2/-2 | 修复错误包装（保留底层错误） |
| `internal/app/push/push_service.go` | M | -1 | 移除未使用的 `time` 导入 |
| `internal/parser/parser.go` | M | +1/-1 | 修复错误包装（保留底层错误） |

---

## 2. 变更意图分析

本批次变更的核心目标是 **将 `push_runner.go` 的核心逻辑从 `cmd/migra/` 下沉到 `internal/app/push/` 包**，这与 `docs/reviews/cli_architecture_review.md` 中第 2 项建议完全一致（"将 `push_runner.go` 的核心逻辑下沉到 `internal/app/` 或专门的 `internal/push/` 包中"）。

### 做得好的地方

1. **架构改进方向正确**：将 push 执行逻辑迁移到 `internal/app/push/` 包是正确的关注点分离做法，使 `cmd/migra/push.go` 仅负责参数解析和编排。
2. **错误包装修复**：`diff_service.go` 和 `parser.go` 中的 `fmt.Errorf` 从 `fmt.Errorf("load source: %w", errors.ErrLoadFailed)`（丢失底层错误）改为 `fmt.Errorf("load source: %w, %w", errors.ErrLoadFailed, err)`（Go 1.20+ 多重包装），直接回应了旧评审报告第 4 项建议。
3. **接口抽象合理**：`push_service.go` 中定义了 `sqlTx`、`dbConnector`、`connectFunc` 接口，便于测试时注入 mock。
4. **信号处理改进**：从 `os.Exit(1)` 改为 `context.WithCancel` + 返回错误，可维护性和可测试性提升。

---

## 3. 问题清单

### [必须修复] 1. `--execute` 标志完全失效：被解析但从未传递给 `pushService.ExecutePlan()`

**文件**：`cmd/migra/push.go:168-174`

**证据**：
```go
// push.go:168 — cfg.Execute 被解析
if cfg.DryRun && !cfg.Execute {
    fmt.Println("Dry-run mode. Use --execute to apply changes.")
    return nil
}

// push.go:174 — 但 Execute 标志从未传递给 pushService
return pushService.ExecutePlan(cmd.Context(), appCfg, sourceSchema, ops)
```

`ExecutePlan` 签名：
```go
func (s *PushService) ExecutePlan(ctx context.Context, cfg app.Config, sourceSchema *model.Schema, ops []diff.Operation) error
```

`app.Config` 中没有 `Execute` 字段。旧代码 `push_runner.go` 有 `autoMode := cfg.Execute` 逻辑在交互循环中跳过确认。

**影响**：`migra push --execute file.sql postgres://localhost/db` 将始终进入交互确认模式，行为与文档和 README 不一致。这是一个 **功能回归**。

**修复建议**：
```go
// 方案 A：在 app.Config 中增加 Execute 字段
type Config struct {
    // ... existing fields ...
    Execute bool
}

// 方案 B：为 push 单独定义选项
type PushOptions struct {
    Execute    bool
    NoVerify   bool
    UnsafeDrop bool  // 注意：当前是硬编码 true，见问题 3
}
```

---

### [必须修复] 2. `--no-verify` 标志完全失效：被解析但从未使用

**文件**：`cmd/migra/push.go:74-76, 90`

**证据**：
```go
noVerify, err := cmd.Flags().GetBool("no-verify")
// ... cfg.NoVerify 被赋值

return pushService.ExecutePlan(cmd.Context(), appCfg, sourceSchema, ops)
// NoVerify 从未传递
```

旧代码 `push_runner.go` 有完整的 post-execution validation：
```go
if !cfg.NoVerify {
    fmt.Println("\n=== Post-execution validation ===")
    newTargetSchema, err := loadSchemaWithContext(ctx, cfg.Target, cfg.Schemas, false)
    // ... 重新 diff 验证
}
```

新 `push_service.go` 完全没有验证逻辑。

**影响**：`--no-verify` 是死代码；默认情况下 post-execution validation 也丢失了。这违背了 README 中 "执行后校验：提交后自动重新 diff，确认目标库与预期一致" 的安全机制声明。

**修复建议**：在 `push_service.go` 的 `ExecutePlan` 中增加验证步骤，或在 `cmd/migra/push.go` 的 `runPush` 中提交后调用验证。

---

### [必须修复] 3. `--unsafe-drop` 标志在 push 执行阶段失效：`UnsafeDrop` 被硬编码为 `true`

**文件**：`cmd/migra/push.go:127`

**证据**：
```go
appCfg := app.Config{
    // ...
    UnsafeDrop: true, // Show all ops (including destructive) in preview
    Timeout:    cfg.Timeout,
}
```

此 `appCfg` 被传递给 `pushService.ExecutePlan()`。在 `push_service.go` 中：
```go
// push_service.go:145
if isDestructive && !cfg.UnsafeDrop {
    fmt.Println("Destructive operation requires --unsafe-drop or explicit confirmation")
    continue
}
```

由于 `cfg.UnsafeDrop = true`，此保护条件 **永远不触发**。用户即使不传 `--unsafe-drop`，危险操作也会在交互确认后直接执行。

**影响**：安全保护机制降级。旧代码中 `cfg.UnsafeDrop` 正确从 flag 读取，不传 `--unsafe-drop` 时会拦截危险操作。

**修复建议**：
```go
appCfg := app.Config{
    // ...
    UnsafeDrop: cfg.UnsafeDrop, // 使用用户传入的值
}
```

---

### [必须修复] 4. 测试覆盖大幅回退：5 个测试文件（1876 行）被删除，新文件零测试

**删除的测试文件**：

| 文件 | 行数 | 覆盖内容 |
|------|------|----------|
| `diff_runner_test.go` | 625 | parseDiffConfig、依赖注入、Viper fallback、默认格式 |
| `diff_test.go` | 152 | diff 命令边界情况 |
| `push_runner_test.go` | 813 | push 配置解析、executeWithConfirmation、mock DB、事务处理 |
| `push_test.go` | 280 | isDestructiveOperation、isNonTransactionalSQL、stripSQLComments、readUserInput |

**新代码测试状态**：
```
?   \tgithub.com/fred29910/migra-go/internal/app/push\t[no test files]
```

`internal/app/push/push_service.go`（269 行）**没有任何测试文件**。

**影响**：
- `isNonTransactionalSQL`、`stripSQLComments`、`readUserInput`、`execSQL` 等函数的单元测试全部丢失
- `ExecutePlan` 的事务处理、信号处理、中断回滚等关键路径无测试覆盖
- `diff_runner.go` 仍然存在，但其对应的 625 行测试被删除
- 对于一个数据库迁移工具，事务安全和中断处理的测试是 **生产就绪的硬性要求**

**修复建议**：至少为 `internal/app/push/push_service.go` 补充以下测试：
- `TestReadUserInput`（各种输入、EOF）
- `TestIsNonTransactionalSQL`（CONCURRENTLY 模式、注释处理）
- `TestStripSQLComments`（多行注释、内联注释）
- `TestExecuteSQL`（成功、失败）
- `TestIsDestructiveOperation`（各种组合）
- 使用 mock DB 测试 `ExecutePlan` 的基本路径

---

### [重要] 5. `isDestructiveOperation` 函数已定义但从未调用（死代码）

**文件**：`internal/app/push/push_service.go:216-219`

```go
func isDestructiveOperation(isDestructive, unsafeDrop bool) bool {
    return isDestructive && !unsafeDrop
}
```

代码中直接使用 `isDestructive && !cfg.UnsafeDrop` 而非此函数。此函数是从旧 `push_runner.go` 迁移过来的，但调用处未使用。

**修复建议**：删除此函数，或在调用处统一使用此函数。

---

### [重要] 6. README 项目结构中引用了已删除的 `push_runner.go`

**文件**：`README.md:363`

```
│   ├── push_runner.go      # push 执行逻辑（交互确认、事务、回滚）
```

此文件已被删除，逻辑已迁移到 `internal/app/push/push_service.go`。

**修复建议**：更新 README 中的项目结构描述。

---

### [重要] 7. 删除了 2026-06-03 评审报告文档

**文件**：`docs/reviews/2026-06-03-code-review.md`（已删除）

该文件记录了之前发现的 7 个问题及其修复状态，是项目审计线索的一部分。删除历史评审报告会丧失可追溯性。

**修复建议**：建议保留历史评审报告，可在文档中添加 "已修复" 或 "已过时" 标注。

---

### [建议修改] 8. `case "auto":` 标签不可从用户输入到达

**文件**：`internal/app/push/push_service.go:168-169`

```go
case "a":
    // ...
    fallthrough
case "auto":
    // Auto mode: execute without confirmation
```

`case "auto":` 只能通过 `fallthrough` 到达，用户输入 `"auto"` 会进入 `default` 分支。这在功能上没有 bug，但命名可能误导维护者认为 `"auto"` 是一个有效的用户输入选项。

**修复建议**：将标签改为更清晰的命名（如 `_auto`），或改为独立的代码块。

---

## 4. 验证结果

| 检查项 | 结果 |
|--------|------|
| `go build ./...` | ✅ 通过 |
| `go vet ./...` | ✅ 通过（无输出） |
| `go test ./...`（全量） | ✅ 通过（14 个包，0 失败） |
| `go test -count=1 ./...`（无缓存） | ✅ 通过 |
| `internal/app/push` 测试 | ⚠️ 无测试文件 |

---

## 5. 与项目意图的对齐分析

| README/设计承诺 | 当前状态 | 评估 |
|----------------|----------|------|
| 交互式安全执行（逐条确认） | ✅ `push_service.go` 实现了交互循环 | 正常 |
| DROP 操作默认拦截，`--unsafe-drop` 放行 | ❌ `UnsafeDrop` 硬编码为 `true` | **功能降级** |
| 事务保护与自动回滚 | ✅ `defer tx.Rollback` + `txActive` 标志 | 正常 |
| 非事务性 DDL 拦截 | ✅ `isNonTransactionalSQL` 检测 | 正常 |
| 执行后校验 | ❌ 代码中无实现 | **功能丢失** |
| `--execute` 跳过确认 | ❌ 代码中无传递 | **功能丢失** |
| `--no-verify` 跳过校验 | ❌ 代码中无传递 | **功能丢失** |
| 信号处理（Ctrl+C 安全退出） | ✅ 改用 context cancel | 改进 |
| 测试覆盖 | ❌ 1876 行测试被删除，新代码零测试 | **质量降级** |

---

## 6. 评审结论

### 修复后可以合并

本次变更的 **架构方向正确**（push 逻辑下沉到 internal 包），错误包装修复也是有益的改进。但存在 3 个必须修复的功能回归问题（`--execute`、`--no-verify`、`--unsafe-drop` 标志失效）和 1 个严重的测试覆盖缺失，这些问题不修复将直接影响生产安全性和可维护性。

**必须修复后才能合并的问题**：
1. `--execute` 标志传递（问题 1）
2. `--no-verify` 标志传递及 post-execution validation 恢复（问题 2）
3. `--unsafe-drop` 标志传递，解除硬编码 `true`（问题 3）
4. 为 `internal/app/push/push_service.go` 补充单元测试（问题 4）

**建议修复**（不阻塞合并但应在本轮或下轮迭代处理）：
5. 删除死代码 `isDestructiveOperation`（问题 5）
6. 更新 README 项目结构（问题 6）
7. 保留历史评审报告（问题 7）
8. 清理不可达的 `case "auto":` 标签（问题 8）