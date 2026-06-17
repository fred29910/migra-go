# 接口与管道优化实现计划

> **面向 AI Agent 的工作者：** 必需子技能：使用 subagent-driven-development（推荐）逐任务实现此计划。步骤使用复选框语法来跟踪进度。

**目标：** 重构核心接口和管道，解决 13 个架构性问题，提升代码可维护性、可测试性和可扩展性。

**架构：** 按 P0→P1→P2→P3 四级优先级分阶段推进，每个阶段独立提交。P0 阶段聚焦 Context 传播、Config 拆分和 ExecutePlan 重构。

**技术栈：** Go 1.26.2，pgx v5，Cobra + Viper

**分支：** `optimize/interface-and-pipeline`（基于 `refactor/cli-fixes`）

---

## 阶段 P0：基础治理（架构性缺陷修复）

### 任务 P0-1：重构 Config——拆分为 DiffConfig 和 PushConfig

**文件：**
- 修改：`internal/app/diff_service.go`
- 修改：`cmd/migra/diff_runner.go`
- 修改：`cmd/migra/push.go`
- 修改：`cmd/migra/diff.go`
- 修改：`internal/app/push/push_service.go`
- 修改：`internal/app/pipeline.go`
- 修改：`internal/app/diff_service_test.go`
- 修改：`internal/app/push/push_service_test.go`

**分析：** `app.Config` 同时承载 diff 和 push 的配置项，`Execute`/`NoVerify` 仅 push 使用。拆分为 `DiffConfig` + `PushConfig`，`PushConfig` 内嵌 `DiffConfig`。

- [ ] **步骤 1：定义新类型并替换 Config**

修改 `internal/app/diff_service.go`：

```go
// DiffConfig holds configuration for a diff operation.
type DiffConfig struct {
    Source     string
    Target     string
    Schemas    []string
    Format     string
    OutputFile string
    UnsafeDrop bool
    Strict     bool
    Timeout    time.Duration
}

// PushConfig holds configuration for a push operation.
type PushConfig struct {
    DiffConfig
    Execute  bool // skip interactive confirmation (--execute)
    NoVerify bool // skip post-execution validation (--no-verify)
}

// Keep Config as alias for backward compatibility during transition
// TODO: remove after all references updated
type Config = DiffConfig
```

- [ ] **步骤 2：更新 ComputeDiff 签名**

修改 `internal/app/pipeline.go` 中的 `ComputeDiff`，将 `cfg Config` 改为 `cfg DiffConfig`：

```go
func ComputeDiff(source, target *model.Schema, cfg DiffConfig) ([]diff.Operation, []string, error) {
```

同时更新 `Config` → `DiffConfig`：
- `FilterDestructiveOps` 的调用（传入 `cfg.UnsafeDrop` 布尔值即可，不影响签名）
- `BuildExecutionPlan` 同理

- [ ] **步骤 3：更新 diff_service.go 中的 DiffService 接口和 RunnerDeps**

```go
type DiffService interface {
    Run(ctx context.Context, cfg DiffConfig) (output string, warnings []string, err error)
}

type RunnerDeps struct {
    LoadSchema func(ctx context.Context, source string, schemas []string, strict bool) (*model.Schema, error)
    Compute    func(source, target *model.Schema, cfg DiffConfig) ([]diff.Operation, []string, error)
    Render     func(ops []diff.Operation, format string) (string, error)
}
```

- [ ] **步骤 4：更新 diff_runner.go 中的 parseDiffConfig 和 newDefaultDeps**

```go
func parseDiffConfig(cmd *cobra.Command, args []string) (app.DiffConfig, error) {
    // ... same logic but returns app.DiffConfig
    return app.DiffConfig{...}, nil
}

func newDefaultDeps() app.RunnerDeps {
    return app.RunnerDeps{
        Compute: func(source, target *model.Schema, cfg app.DiffConfig) ([]diff.Operation, []string, error) {
            return app.ComputeDiff(source, target, cfg)
        },
        // ...
    }
}
```

- [ ] **步骤 5：更新 push.go 中的 PushConfig 和 runPush**

```go
type pushCliConfig struct {
    DiffConfig app.DiffConfig
    DryRun     bool
    Execute    bool
    NoVerify   bool
}

func runPush(cmd *cobra.Command, args []string) error {
    // After computing ops:
    pushService := push.NewPushService()
    appPushCfg := app.PushConfig{
        DiffConfig: appCfg,
        Execute:    cfg.Execute,
        NoVerify:   cfg.NoVerify,
    }
    if err := pushService.ExecutePlan(cmd.Context(), appPushCfg, sourceSchema, ops); err != nil {
        return err
    }
}
```

- [ ] **步骤 6：更新 push_service.go 中的 ExecutePlan 签名**

```go
func (s *PushService) ExecutePlan(ctx context.Context, cfg app.PushConfig, sourceSchema *model.Schema, ops []diff.Operation) error {
    // Use cfg.DiffConfig.Target, cfg.DiffConfig.UnsafeDrop, cfg.Execute, cfg.NoVerify
}
```

- [ ] **步骤 7：更新 diff_service_test.go 和 push_service_test.go 中的类型引用**

```go
// diff_service_test.go 中使用 app.DiffConfig{...}
// push_service_test.go 中使用 app.PushConfig{DiffConfig: app.DiffConfig{...}}
```

- [ ] **步骤 8：编译验证 + 运行测试**

```bash
rtk cargo build 2>/dev/null; rtk go build ./...
rtk go test ./internal/app/... ./internal/app/push/... -v -count=1 2>&1 | head -50
```

- [ ] **步骤 9：提交**

```bash
git add internal/app/diff_service.go internal/app/pipeline.go cmd/migra/diff_runner.go cmd/migra/diff.go cmd/migra/push.go internal/app/push/push_service.go internal/app/diff_service_test.go internal/app/push/push_service_test.go
git commit -m "refactor(config): 拆分 Config 为 DiffConfig 和 PushConfig

- DiffConfig 仅包含 diff 相关字段
- PushConfig 内嵌 DiffConfig，追加 Execute/NoVerify
- 更新 ComputeDiff/RunnerDeps 签名
- 保持向后兼容（Config = DiffConfig 别名）
"
```

---

### 任务 P0-2：为 diff.Engine / plan.Engine / render.SQLEngine 增加 Context 传播

**文件：**
- 修改：`internal/diff/differ.go`（Engine 接口 + Differ 实现）
- 修改：`internal/diff/context.go`（diffContext 方法签名）
- 修改：`internal/diff/diff_tables.go`
- 修改：`internal/diff/diff_columns.go`
- 修改：`internal/diff/*.go`（所有 diff_*.go 的接收者方法）
- 修改：`internal/plan/plan.go`（Engine 接口 + Planner）
- 修改：`internal/plan/dag.go`（TopoSort 等）
- 修改：`internal/render/render.go`（SQLEngine 接口 + Renderer）
- 修改：`internal/app/pipeline.go`（调用方）
- 修改：`internal/app/diff_runner.go`（依赖注入）
- 修改：`internal/app/push/push_service.go`（调用方）

- [ ] **步骤 1：修改 diff.Engine 接口 + Differ 实现**

```go
// internal/diff/differ.go
type Engine interface {
    Diff(ctx context.Context, source, target *model.Schema) ([]Operation, []string)
}

func (d *Differ) Diff(ctx context.Context, source, target *model.Schema) ([]Operation, []string) {
    ctx := &diffContext{ops: make([]Operation, 0, 16), warnings: make([]string, 0, 4)}
    ctx.diffSchemas(ctx, source, target)
    return ctx.ops, ctx.warnings
}
```

- [ ] **步骤 2：修改 diffContext 方法传递 Context**

```go
// internal/diff/context.go
type diffContext struct {
    ctx      context.Context
    ops      []Operation
    warnings []string
}

func newDiffContext(ctx context.Context) *diffContext {
    return &diffContext{ctx: ctx, ops: make([]Operation, 0, 16), warnings: make([]string, 0, 4)}
}

func (c *diffContext) addOp(op Operation) {
    c.ops = append(c.ops, op)
}

func (c *diffContext) checkCancelled() error {
    return c.ctx.Err()
}
```

- [ ] **步骤 3：在 diffSchemas / diffTables 等长循环中插入取消检查**

```go
// internal/diff/differ.go
func (c *diffContext) diffSchemas(source, target *model.Schema) {
    if err := c.checkCancelled(); err != nil {
        return
    }
    // ... existing logic
}

// internal/diff/diff_tables.go
func (c *diffContext) diffTableColumns(source, target *model.Table, schemaName string) {
    if err := c.checkCancelled(); err != nil {
        return
    }
    // Phase 1: rename detection
    for _, srcCol := range source.Columns {
        if err := c.checkCancelled(); err != nil {
            return
        }
        // ...
    }
}
```

在 `diffTableColumns`、`diffTables`、`diffViews`、`diffSequences`、`diffExtensions`、`diffTypes` 的每个循环迭代中检查取消。

- [ ] **步骤 4：修改 plan.Engine 接口 + Planner 实现**

```go
// internal/plan/plan.go
type Engine interface {
    Plan(ctx context.Context, ops []diff.Operation) (map[Stage][]diff.Operation, error)
}

func (p *Planner) Plan(ctx context.Context, ops []diff.Operation) (map[Stage][]diff.Operation, error) {
    stages := make(map[Stage][]diff.Operation)
    // ...
    for _, op := range ops {
        if err := ctx.Err(); err != nil {
            return nil, err
        }
        stage := p.assignStage(op)
        // ...
    }
    return stages, nil
}

func TopoSort(ctx context.Context, ops []diff.Operation) ([]diff.Operation, error) {
    dag := BuildDAG(ops)
    return dag.GetExecutionOrder(ctx)
}
```

- [ ] **步骤 5：修改 render.SQLEngine 接口 + Renderer 实现**

```go
// internal/render/render.go
type SQLEngine interface {
    RenderAll(ctx context.Context, ops []diff.Operation) (string, error)
}

func (r *Renderer) RenderAll(ctx context.Context, ops []diff.Operation) (string, error) {
    var b strings.Builder
    for _, op := range ops {
        if err := ctx.Err(); err != nil {
            return "", err
        }
        sql := r.Render(op)
        // ...
    }
    return b.String(), nil
}
```

- [ ] **步骤 6：更新 pipeline.go 中的 BuildExecutionPlan 和 ComputeDiff**

```go
func BuildExecutionPlan(ctx context.Context, ops []diff.Operation, unsafeDrop bool) ([]diff.Operation, error) {
    planner := plan.NewPlanner(unsafeDrop)
    stages, err := planner.Plan(ctx, ops)
    if err != nil {
        return nil, err
    }
    // ...
    for _, stage := range stageOrder {
        stageOps := stages[stage]
        if len(stageOps) == 0 { continue }
        sortedStageOps, err := plan.TopoSort(ctx, stageOps)
        // ...
    }
    return allOps, nil
}

func ComputeDiff(ctx context.Context, source, target *model.Schema, cfg DiffConfig) ([]diff.Operation, []string, error) {
    // ...
    differ := diff.NewDiffer()
    operations, warnings := differ.Diff(ctx, source, target)
    // ...
    sortedOps, err := BuildExecutionPlan(ctx, filteredOps, cfg.UnsafeDrop)
    // ...
}
```

- [ ] **步骤 7：更新 RenderOutput 传递 Context**

```go
func RenderOutput(ctx context.Context, ops []diff.Operation, format string) (string, error) {
    renderer := render.NewRenderer()
    return renderer.RenderOutput(ctx, ops, format)
}
```

`Renderer.RenderOutput` 也需要 Context：

```go
func (r *Renderer) RenderOutput(ctx context.Context, ops []diff.Operation, format string) (string, error) {
    switch format {
    case "sql":
        return r.RenderAll(ctx, ops)
    case "json":
        return renderJSON(ctx, ops)
    // ...
}
```

- [ ] **步骤 8：更新 diff_service.go 调用链**

```go
func (s *diffService) Run(parent context.Context, cfg DiffConfig) (string, []string, error) {
    ctx, cancel := context.WithTimeout(parent, cfg.Timeout)
    defer cancel()

    sourceSchema, err := s.deps.LoadSchema(ctx, cfg.Source, cfg.Schemas, cfg.Strict)
    // ...
    ops, warnings, err := s.deps.Compute(ctx, sourceSchema, targetSchema, cfg)
    // ^ Compute 签名增加 ctx

    output, err := s.deps.Render(ctx, ops, cfg.Format)
    // ^ Render 签名增加 ctx
    // ...
}
```

- [ ] **步骤 9：更新 RunnerDeps 类型**

```go
type RunnerDeps struct {
    LoadSchema func(ctx context.Context, source string, schemas []string, strict bool) (*model.Schema, error)
    Compute    func(ctx context.Context, source, target *model.Schema, cfg DiffConfig) ([]diff.Operation, []string, error)
    Render     func(ctx context.Context, ops []diff.Operation, format string) (string, error)
}
```

- [ ] **步骤 10：更新 diff_runner.go 中的 newDefaultDeps**

```go
func newDefaultDeps() app.RunnerDeps {
    return app.RunnerDeps{
        LoadSchema: func(ctx context.Context, source string, schemas []string, strict bool) (*model.Schema, error) {
            return loadSchemaWithContext(ctx, source, schemas, strict)
        },
        Compute: func(ctx context.Context, source, target *model.Schema, cfg app.DiffConfig) ([]diff.Operation, []string, error) {
            return app.ComputeDiff(ctx, source, target, cfg)
        },
        Render: func(ctx context.Context, ops []diff.Operation, format string) (string, error) {
            return app.RenderOutput(ctx, ops, format)
        },
    }
}
```

- [ ] **步骤 11：更新 push_service.go 和 push.go 中的调用**

```go
// push.go
ops, _, err := app.ComputeDiff(cmd.Context(), targetSchema, sourceSchema, appCfg)
// 改为
ops, _, err := app.ComputeDiff(cmd.Context(), targetSchema, sourceSchema, appCfg.DiffConfig)

// push_service.go — 无需传 Context，ComputeDiff 已在外部完成
```

- [ ] **步骤 12：编译验证 + 运行测试**

```bash
rtk go build ./...
rtk go test ./internal/diff/... ./internal/plan/... ./internal/render/... ./internal/app/... -count=1 -v 2>&1 | tail -30
```

- [ ] **步骤 13：修复编译错误并重复验证**

- [ ] **步骤 14：提交**

```bash
git add internal/diff/ internal/plan/ internal/render/ internal/app/
git commit -m "refactor(core): 为 Engine/Planner/Renderer 增加 Context 传播

- diff.Engine.Diff(ctx, source, target) 添加 ctx 参数
- plan.Engine.Plan(ctx, ops) 添加 ctx 参数
- render.SQLEngine.RenderAll(ctx, ops) 添加 ctx 参数
- diffContext 携带 ctx，长循环中检查取消信号
- 更新 ComputeDiff/BuildExecutionPlan/RenderOutput 签名
- 更新 RunnerDeps 中的函数签名
"
```

---

### 任务 P0-3：重构 ExecutePlan——拆分为小方法

**文件：**
- 修改：`internal/app/push/push_service.go`
- 修改：`internal/app/push/push_service_test.go`

- [ ] **步骤 1：提取信号设置方法**

```go
// push_service.go
func (s *PushService) setupSignalHandler(ctx context.Context) (context.Context, context.CancelFunc) {
    execCtx, execCancel := context.WithCancel(ctx)
    sigChan := make(chan os.Signal, 1)
    signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)

    go func() {
        defer signal.Stop(sigChan)
        select {
        case <-sigChan:
            fmt.Println("\nInterrupt received, cancelling...")
            execCancel()
        case <-execCtx.Done():
        }
    }()
    return execCtx, execCancel
}
```

- [ ] **步骤 2：提取交互式执行方法**

```go
// push_service.go
func (s *PushService) executeOne(ctx context.Context, tx sqlTx, sql string, idx int) error {
    _, err := tx.Exec(ctx, sql)
    if err != nil {
        return fmt.Errorf("error executing SQL #%d: %w", idx, err)
    }
    fmt.Printf("SQL #%d executed\n", idx)
    return nil
}

func (s *PushService) handlePrompt(ctx context.Context, idx int, sql string, isDestructive bool, cfg app.PushConfig) (action, error) {
    // Returns: actionExec, actionSkip, actionCancel, actionApplyAll
}
```

- [ ] **步骤 3：提取事务安全回滚方法**

```go
type transactionState struct {
    tx       sqlTx
    active   bool
}

func (s *transactionState) rollback(ctx context.Context) {
    if s.active {
        _ = s.tx.Rollback(ctx)
        s.active = false
    }
}

func (s *transactionState) commit(ctx context.Context) error {
    if err := s.tx.Commit(ctx); err != nil {
        return err
    }
    s.active = false
    return nil
}
```

- [ ] **步骤 4：重构 ExecutePlan 为主体流程**

```go
func (s *PushService) ExecutePlan(ctx context.Context, cfg app.PushConfig, sourceSchema *model.Schema, ops []diff.Operation) error {
    execCtx, execCancel := s.setupSignalHandler(ctx)
    defer execCancel()

    conn, err := s.connectFunc(ctx, cfg.DiffConfig.Target)
    if err != nil {
        return fmt.Errorf("failed to connect to target database: %w", err)
    }
    defer func() { _ = conn.Close(ctx) }()

    tx, err := conn.Begin(execCtx)
    if err != nil {
        return fmt.Errorf("failed to begin transaction: %w", err)
    }
    ts := &transactionState{tx: tx, active: true}
    defer ts.rollback(ctx)

    renderer := render.NewRenderer()

    switch {
    case cfg.Execute:
        return s.executeAuto(execCtx, ts, renderer, ops, cfg)
    default:
        return s.executeInteractive(execCtx, ts, renderer, ops, cfg)
    }
}
```

- [ ] **步骤 5：提取 executeAuto 和 executeInteractive**

```go
func (s *PushService) executeAuto(ctx context.Context, ts *transactionState, renderer *render.Renderer, ops []diff.Operation, cfg app.PushConfig) error {
    for i, op := range ops {
        if err := ctx.Err(); err != nil {
            return s.failWithRollback(ctx, ts, fmt.Errorf("execution interrupted: %w", err))
        }
        sql := renderer.Render(op)
        if sql == "" { continue }
        if isNonTransactionalSQL(sql) {
            return s.failWithRollback(ctx, ts, fmt.Errorf("non-transactional DDL at SQL #%d", i+1))
        }
        if op.IsDestructive() && !cfg.DiffConfig.UnsafeDrop {
            fmt.Printf("Destructive operation blocked at SQL #%d (use --unsafe-drop to allow):\n  %s\n", i+1, sql)
            continue
        }
        if err := s.executeOne(ctx, ts.tx, sql, i+1); err != nil {
            return s.failWithRollback(ctx, ts, err)
        }
    }
    return s.commitTransaction(ctx, ts)
}

func (s *PushService) executeInteractive(ctx context.Context, ts *transactionState, renderer *render.Renderer, ops []diff.Operation, cfg app.PushConfig) error {
    var applyAll bool
    for i, op := range ops {
        if err := ctx.Err(); err != nil {
            return s.failWithRollback(ctx, ts, fmt.Errorf("execution interrupted: %w", err))
        }
        sql := renderer.Render(op)
        if sql == "" { continue }
        if isNonTransactionalSQL(sql) {
            fmt.Printf("\nNon-transactional DDL detected at SQL #%d...\n", i+1)
            return s.failWithRollback(ctx, ts, fmt.Errorf("non-transactional DDL at SQL #%d", i+1))
        }
        if err := s.promptAndExecute(ctx, ts, renderer, op, sql, i, &applyAll, cfg); err != nil {
            return err
        }
    }
    return s.commitTransaction(ctx, ts)
}
```

- [ ] **步骤 6：提取 promptAndExecute 方法**

```go
func (s *PushService) promptAndExecute(ctx context.Context, ts *transactionState, renderer *render.Renderer, op diff.Operation, sql string, i int, applyAll *bool, cfg app.PushConfig) error {
    isDestructive := op.IsDestructive()
    idx := i + 1

    if *applyAll {
        if isDestructive && !cfg.DiffConfig.UnsafeDrop {
            fmt.Printf("Destructive operation detected in auto-mode, reverting to interactive.\nSQL #%d: %s\n\n", idx, sql)
            *applyAll = false
        } else {
            return s.executeOne(ctx, ts.tx, sql, idx)
        }
    }

    prompt := fmt.Sprintf("Execute SQL #%d? (y=yes, n=no, a=apply all, s=skip): ", idx)
    if isDestructive {
        prompt = fmt.Sprintf("DANGER: Execute SQL #%d? (y=yes, n=no, a=apply all, s=skip): ", idx)
    }

    for {
        fmt.Print(prompt)
        input := strings.ToLower(strings.TrimSpace(readUserInput(s.stdinReader)))
        // ... handle y/n/a/s with proper error handling
        switch input {
        case "y":
            if isDestructive && !cfg.DiffConfig.UnsafeDrop {
                fmt.Println("Destructive operation requires --unsafe-drop or explicit confirmation")
                continue
            }
            return s.executeOne(ctx, ts.tx, sql, idx)
        case "n":
            fmt.Println("Cancelled")
            ts.rollback(ctx)
            return nil
        case "a":
            *applyAll = true
            if isDestructive && !cfg.DiffConfig.UnsafeDrop {
                fmt.Printf("Destructive operation detected...\n")
                *applyAll = false
                continue
            }
            return s.executeOne(ctx, ts.tx, sql, idx)
        case "s":
            fmt.Printf("SQL #%d skipped\n", idx)
            return nil
        default:
            fmt.Println("Invalid input. Use: y, n, a, or s")
        }
    }
}
```

- [ ] **步骤 7：提取辅助方法**

```go
func (s *PushService) failWithRollback(ctx context.Context, ts *transactionState, err error) error {
    fmt.Println("Rolling back transaction...")
    ts.rollback(ctx)
    return err
}

func (s *PushService) commitTransaction(ctx context.Context, ts *transactionState) error {
    if err := ts.tx.Commit(ctx); err != nil {
        return fmt.Errorf("failed to commit transaction: %w", err)
    }
    ts.active = false
    fmt.Println("\nAll SQL executed successfully, transaction committed")
    return nil
}
```

- [ ] **步骤 8：更新 push_service_test.go 适配新结构**

更新 mock 和测试用例以使用新的重构后方法结构。

- [ ] **步骤 9：编译验证 + 运行测试**

```bash
rtk go build ./...
rtk go test ./internal/app/push/... -v -count=1 2>&1 | tail -30
```

- [ ] **步骤 10：修复编译错误并重复验证**

- [ ] **步骤 11：提交**

```bash
git add internal/app/push/
git commit -m "refactor(push): 拆分 ExecutePlan 为小方法

- 提取 setupSignalHandler/executeAuto/executeInteractive
- 提取 promptAndExecute/failWithRollback/commitTransaction
- 引入 transactionState 管理事务安全
- 150 行单方法拆分为 6 个聚焦方法
"
```

---

## 阶段 P1：接口与设计质量（第一期迭代）

### 任务 P1-1：统一错误处理

**文件：**
- 修改：`internal/errors/errors.go`
- 修改：所有引用哨兵错误的文件（全局搜索 `ErrNotFound`、`ErrParseFailed` 等）

- [ ] **步骤 1：保留结构化错误，添加 Is() 向下兼容**

```go
// internal/errors/errors.go
// 保留哨兵错误供外部使用（标记为 DEPRECATED）
// Deprecated: Use ErrCodeNotFound instead.
var ErrNotFound = errors.New("resource not found")

// 结构化错误
var ErrCodeNotFound = &Error{Code: "NOT_FOUND", Message: "resource not found"}
var ErrCodeParseFailed = &Error{Code: "PARSE_FAILED", Message: "parse failed"}
var ErrCodeLoadFailed = &Error{Code: "LOAD_FAILED", Message: "load failed"}
var ErrCodeDiffFailed = &Error{Code: "DIFF_FAILED", Message: "diff failed"}

// 让结构化错误可以匹配哨兵错误
func (e *Error) Is(target error) bool {
    switch target {
    case ErrNotFound:
        return e.Code == "NOT_FOUND"
    case ErrParseFailed:
        return e.Code == "PARSE_FAILED"
    case ErrLoadFailed:
        return e.Code == "LOAD_FAILED"
    case ErrDiffFailed:
        return e.Code == "DIFF_FAILED"
    }
    return false
}
```

- [ ] **步骤 2：全局搜索替换 `errors.ErrNotFound` 使用点**

注意区分：`errors.Is(err, ErrNotFound)` 可以保留，因为 `Error.Is()` 支持匹配。但 `errors.ErrNotFound` 添加到 `%w` 的包装需要检查。

- [ ] **步骤 3：编译验证**

```bash
rtk go build ./...
```

- [ ] **步骤 4：提交**

```bash
git add internal/errors/errors.go
git commit -m "refactor(errors): 统一错误处理，结构化错误兼容哨兵错误
"
```

---

### 任务 P1-2：PushService 接口化

**文件：**
- 修改：`internal/app/push/push_service.go`
- 修改：`cmd/migra/push.go`
- 修改：`internal/app/push/push_service_test.go`

- [ ] **步骤 1：定义 Service 接口**

```go
// internal/app/push/push_service.go

// Service defines the interface for pushing schema changes to a database.
type Service interface {
    ExecutePlan(ctx context.Context, cfg app.PushConfig, sourceSchema *model.Schema, ops []diff.Operation) error
}
```

- [ ] **步骤 2：让 PushService 显式实现接口**

```go
// Compile-time check
var _ Service = (*PushService)(nil)
```

- [ ] **步骤 3：更新 NewPushService 返回类型**

```go
func NewPushService() Service {
    return &PushService{
        connectFunc: defaultConnectToDB,
        stdinReader: bufio.NewReader(os.Stdin),
    }
}
```

- [ ] **步骤 4：更新 push.go 中的调用**

无需修改，`NewPushService()` 已返回 `Service` 接口。

- [ ] **步骤 5：编译验证 + 运行测试**

```bash
rtk go build ./...
rtk go test ./internal/app/push/... -v -count=1 2>&1 | tail -20
```

- [ ] **步骤 6：提交**

```bash
git add internal/app/push/push_service.go cmd/migra/push.go
git commit -m "refactor(push): PushService 接口化

- 定义 Service 接口
- NewPushService() 返回 Service
- 编译时检查 var _ Service = (*PushService)(nil)
"
```

---

### 任务 P1-3：Parser 无状态化

**文件：**
- 修改：`internal/parser/parser.go`
- 修改：`internal/parser/parser_test.go`

- [ ] **步骤 1：移除 Parser 的可变状态字段**

```go
type Parser struct {
    registry *HandlerRegistry
    applier  *MutationApplier
    warnFn   WarningEmitter
}
```

移除 `schema`、`errors`、`warnings`、`sql` 字段。

- [ ] **步骤 2：将 ParseSQL 改为使用局部变量**

```go
func (p *Parser) ParseSQL(sql string) (*model.Schema, error) {
    if p.registry == nil {
        p.registry = DefaultRegistry()
    }
    if p.applier == nil {
        p.applier = &MutationApplier{}
    }

    schema := model.NewSchema()
    var errs []error
    var warnings []string

    tree, err := pg_query.Parse(sql)
    if err != nil {
        return nil, fmt.Errorf("parse SQL: %w, %w", errors.ErrParseFailed, err)
    }

    for _, rawStmt := range tree.Stmts {
        if err := p.visitNode(schema, rawStmt.Stmt, int(rawStmt.StmtLocation)); err != nil {
            errs = append(errs, err)
        }
    }

    if len(errs) > 0 {
        return schema, fmt.Errorf("parsing completed with %d errors, first: %w", len(errs), errs[0])
    }
    return schema, nil
}
```

- [ ] **步骤 3：更新 visitNode 签名接受 schema 参数**

```go
func (p *Parser) visitNode(schema *model.Schema, stmt *pg_query.Node, pos int) (err error) {
    // 不再使用 p.schema
    // ...
    if err := p.applier.Apply(schema, mutations); err != nil {
        // ...
    }
}
```

- [ ] **步骤 4：运行测试验证**

```bash
rtk go test ./internal/parser/... -v -count=1 2>&1 | tail -30
```

- [ ] **步骤 5：提交**

```bash
git add internal/parser/parser.go
git commit -m "refactor(parser): Parser 无状态化

- 移除 schema/errors/warnings/sql 可变字段
- ParseSQL 使用局部变量，支持多次并行调用
- visitNode 通过参数传递 schema
"
```

---

### 任务 P1-4：Stage 编码到 Operation

**文件：**
- 修改：`internal/diff/operation.go`
- 修改：`internal/plan/plan.go`

- [ ] **步骤 1：添加 StageAware 接口检测**

```go
// internal/plan/plan.go
// HasStage is an optional interface that operations can implement
// to declare their execution stage directly.
type HasStage interface {
    Stage() Stage
}
```

- [ ] **步骤 2：修改 assignStage 优先检测 HasStage**

```go
func (p *Planner) assignStage(op diff.Operation) Stage {
    // 如果 Operation 实现了 HasStage，优先使用
    if stager, ok := op.(HasStage); ok {
        return stager.Stage()
    }

    kind := op.Kind()
    switch kind {
    // ... 保留现有的 fallback switch-case
    }
}
```

- [ ] **步骤 3：编译验证**

```bash
rtk go build ./...
```

- [ ] **步骤 4：提交**

```bash
git add internal/plan/plan.go
git commit -m "refactor(plan): 增加 HasStage 接口实现可扩展 stage 分配

- 优先检测 Operation 是否实现 HasStage 接口
- 未实现时 fallback 到原有 switch-case
- 新增操作可通过实现 HasStage 声明 stage，无需修改 Planner
"
```

---

## 阶段 P2：可扩展性（后续迭代）

*注意：P2 项依赖 P0+P1 完成后方可进行。以下为计划预览。*

| 任务 | 描述 | 估算 |
|------|------|------|
| P2-1 | Model 深拷贝 Clone 方法 | 1 天 |
| P2-2 | Loader 优先级排序 | 0.5 天 |
| P2-3 | Pipeline Hook 机制 | 1 天 |

---

## 阶段 P3：质量提升（持续迭代）

| 任务 | 描述 | 估算 |
|------|------|------|
| P3-1 | normalize 正则补充边缘测试 | 0.5 天 |
| P3-2 | Benchmark 套件 | 1 天 |
| P3-3 | Model 快捷方法 | 0.5 天 |

---

## 验证清单

每个阶段完成后：

- [ ] `rtk go build ./...` 编译通过
- [ ] `rtk go vet ./...` 无静态检查错误
- [ ] `rtk go test ./... -count=1` 测试全部通过
- [ ] 所有变更已提交到 `optimize/interface-and-pipeline` 分支
