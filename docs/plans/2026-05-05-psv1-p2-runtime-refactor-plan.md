# PSV1 P2 运行时重构 实现计划

> **面向 AI 代理的工作者：** 必需子技能：使用 superpowers:subagent-driven-development（推荐）或 superpowers:executing-plans 逐任务实现此计划。步骤使用复选框（`- [ ]`）语法来跟踪进度。

**目标：** 完成 P2（P6/P10/P11/P12）重构，使 `diff` 命令可测试、可注入、具备超时/取消语义，并消除 `Renderer` 可变状态。

**架构：** 将 `runDiff` 拆为配置解析、schema 加载、diff+plan、渲染、输出五段；通过小接口把 `cmd/migra` 与 `internal/diff|plan|render` 解耦；将数据库加载改为调用方传入 `context.Context`；将 `Renderer` 改为无状态实现，保持 SQL 输出语义不变。

**技术栈：** Go 1.24、Cobra、Viper、pgx、`go test`

---

## 文件结构

**创建：**
- `cmd/migra/diff_runner.go`：`runDiff` 的可测试编排实现。
- `cmd/migra/diff_runner_test.go`：配置解析、超时构造、依赖注入测试。

**修改：**
- `cmd/migra/diff.go`：瘦身为参数入口与依赖装配。
- `internal/diff/differ.go`：定义 `Engine` 接口并保证 `Differ` 满足接口。
- `internal/plan/plan.go`：定义 `Engine` 接口并保证 `Planner` 满足接口。
- `internal/render/render.go`：移除 `sql []string` 状态，改为局部 `strings.Builder`。
- `cmd/migra/integration_test.go`：回归主链路。

**测试：**
- `cmd/migra/diff_runner_test.go`
- `cmd/migra/integration_test.go`
- `internal/render`（新增或补充测试）

### 任务 1：拆分 `runDiff` 编排职责（P6）

**文件：**
- 创建：`cmd/migra/diff_runner.go`
- 修改：`cmd/migra/diff.go`
- 测试：`cmd/migra/diff_runner_test.go`

- [ ] **步骤 1：编写失败测试（`parseDiffConfig` 对 flag 映射正确）**

```go
func TestParseDiffConfig(t *testing.T) {
	cmd := &cobra.Command{Use: "diff"}
	cmd.Flags().StringSlice("schema", []string{"public"}, "")
	cmd.Flags().String("format", "sql", "")
	cmd.Flags().Bool("unsafe-drop", false, "")
	cmd.Flags().Bool("strict", false, "")
	cmd.Flags().String("output", "", "")
	_ = cmd.Flags().Set("schema", "public,app")
	_ = cmd.Flags().Set("format", "json")

	cfg, err := parseDiffConfig(cmd, []string{"a.sql", "b.sql"})
	if err != nil {
		t.Fatal(err)
	}
	if cfg.source != "a.sql" || cfg.target != "b.sql" || cfg.format != "json" {
		t.Fatalf("unexpected cfg: %+v", cfg)
	}
}
```

- [ ] **步骤 2：实现 `diffConfig` 和分层函数**

```go
type diffConfig struct {
	source     string
	target     string
	schemas    []string
	format     string
	outputFile string
	unsafeDrop bool
	strict     bool
	timeout    time.Duration
}

func runDiff(cmd *cobra.Command, args []string) error {
	cfg, err := parseDiffConfig(cmd, args)
	if err != nil {
		return err
	}
	return runDiffWithDeps(cmd.Context(), cfg, newDefaultDeps())
}
```

- [ ] **步骤 3：实现可测试编排入口**

```go
func runDiffWithDeps(parent context.Context, cfg diffConfig, deps runnerDeps) error {
	ctx, cancel := context.WithTimeout(parent, cfg.timeout)
	defer cancel()

	sourceSchema, err := deps.loadSchema(ctx, cfg.source, cfg.schemas, cfg.strict)
	if err != nil { return fmt.Errorf("failed to load source: %w", err) }
	targetSchema, err := deps.loadSchema(ctx, cfg.target, cfg.schemas, cfg.strict)
	if err != nil { return fmt.Errorf("failed to load target: %w", err) }

	ops, warnings, err := deps.compute(sourceSchema, targetSchema, cfg)
	if err != nil { return err }
	deps.reportWarnings(warnings)
	output, err := deps.render(ops, cfg.format)
	if err != nil { return err }
	return deps.writeOutput(output, cfg.outputFile)
}
```

- [ ] **步骤 4：运行测试确认通过**

运行：`go test ./cmd/migra -run 'TestParseDiffConfig' -v`
预期：PASS。

- [ ] **步骤 5：Commit**

```bash
git add cmd/migra/diff.go cmd/migra/diff_runner.go cmd/migra/diff_runner_test.go
git commit -m "refactor(cli): split runDiff into testable pipeline"
```

### 任务 2：引入接口抽象并在命令层依赖接口（P10）

**文件：**
- 修改：`internal/diff/differ.go`
- 修改：`internal/plan/plan.go`
- 修改：`internal/render/render.go`
- 修改：`cmd/migra/diff_runner.go`
- 测试：`cmd/migra/diff_runner_test.go`

- [ ] **步骤 1：编写失败测试（注入 fake 引擎可驱动 runDiff）**

```go
func TestRunDiffWithDeps_UsesInjectedEngines(t *testing.T) {
	deps := newFakeDeps()
	cfg := diffConfig{source: "a.sql", target: "b.sql", format: "sql", timeout: time.Second}
	if err := runDiffWithDeps(context.Background(), cfg, deps); err != nil {
		t.Fatal(err)
	}
	if deps.computeCalled == 0 {
		t.Fatal("expected injected compute to be called")
	}
}
```

- [ ] **步骤 2：在 `internal/diff` 增加接口定义**

```go
type Engine interface {
	Diff(source, target *model.Schema) []Operation
	Warnings() []string
}

var _ Engine = (*Differ)(nil)
```

- [ ] **步骤 3：在 `internal/plan` 增加接口定义**

```go
type Engine interface {
	Plan(ops []diff.Operation) map[Stage][]diff.Operation
}

var _ Engine = (*Planner)(nil)
```

- [ ] **步骤 4：在 `internal/render` 增加接口定义并用于装配**

```go
type SQLEngine interface {
	RenderAll(ops []diff.Operation) string
}

var _ SQLEngine = (*Renderer)(nil)
```

- [ ] **步骤 5：运行测试并提交**

运行：`go test ./cmd/migra -run TestRunDiffWithDeps_UsesInjectedEngines -v`
预期：PASS。

```bash
git add internal/diff/differ.go internal/plan/plan.go internal/render/render.go cmd/migra/diff_runner.go cmd/migra/diff_runner_test.go
git commit -m "refactor: depend on interfaces in diff command pipeline"
```

### 任务 3：传播 `context` 与超时控制（P12）

**文件：**
- 修改：`cmd/migra/diff.go`
- 修改：`cmd/migra/diff_runner.go`
- 修改：`cmd/migra/diff_runner_test.go`

- [ ] **步骤 1：新增 `--timeout` flag 与解析测试**

```go
func TestParseDiffConfig_DefaultTimeout(t *testing.T) {
	cmd := newDiffTestCommand()
	cfg, err := parseDiffConfig(cmd, []string{"a.sql", "b.sql"})
	if err != nil { t.Fatal(err) }
	if cfg.timeout != 30*time.Second {
		t.Fatalf("expected 30s, got %s", cfg.timeout)
	}
}
```

- [ ] **步骤 2：修改 `loadFromDB` 接口为接收调用方 context**

```go
func loadFromDB(ctx context.Context, connStr string, schemas []string) (*model.Schema, error) {
	opt := introspect.LoadOptions{Schemas: schemas}
	return introspect.LoadFromDB(ctx, connStr, opt)
}
```

- [ ] **步骤 3：让 `loadSchema` 和编排路径透传 context**

```go
func loadSchema(ctx context.Context, source string, schemas []string, strict bool) (*model.Schema, error) {
	if isPostgresURL(source) { return loadFromDB(ctx, source, schemas) }
	if isSQLFile(source) { return loadFromSQLFile(source, strict) }
	return nil, fmt.Errorf("unsupported source: %s", source)
}
```

- [ ] **步骤 4：运行 cmd 包测试**

运行：`go test ./cmd/migra -v`
预期：PASS。

- [ ] **步骤 5：Commit**

```bash
git add cmd/migra/diff.go cmd/migra/diff_runner.go cmd/migra/diff_runner_test.go
git commit -m "feat(cli): add timeout flag and propagate context"
```

### 任务 4：将 `Renderer` 改为无状态实现（P11）

**文件：**
- 修改：`internal/render/render.go`
- 创建：`internal/render/render_test.go`

- [ ] **步骤 1：编写失败测试（同实例多次渲染结果一致）**

```go
func TestRenderer_RenderAll_IdempotentAcrossCalls(t *testing.T) {
	r := NewRenderer()
	ops := []diff.Operation{diff.NewDropIndexOp("public", "idx_a")}
	got1 := r.RenderAll(ops)
	got2 := r.RenderAll(ops)
	if got1 != got2 {
		t.Fatalf("expected stable output, got1=%q got2=%q", got1, got2)
	}
}
```

- [ ] **步骤 2：移除 `Renderer.sql`，改用局部 `strings.Builder`**

```go
type Renderer struct {
	format      string
	useIfExists bool
}

func (r *Renderer) RenderAll(ops []diff.Operation) string {
	var b strings.Builder
	for _, op := range ops {
		sql := r.Render(op)
		if sql == "" { continue }
		if b.Len() == 0 {
			b.WriteString("-- Begin Diff\n")
		}
		b.WriteString(sql)
		b.WriteByte('\n')
	}
	if b.Len() == 0 { return "-- No changes detected" }
	b.WriteString("-- End Diff")
	return b.String()
}
```

- [ ] **步骤 3：删除仅为缓存服务的 `String()` 状态依赖**

```go
// String() 若保留，仅包装 RenderAll 结果；不得读取共享可变切片。
```

- [ ] **步骤 4：运行 render 与集成测试**

运行：`go test ./internal/render ./cmd/migra -run 'TestRenderer_RenderAll_IdempotentAcrossCalls|TestIntegrationDiffRender' -v`
预期：PASS。

- [ ] **步骤 5：Commit**

```bash
git add internal/render/render.go internal/render/render_test.go cmd/migra/integration_test.go
git commit -m "refactor(render): make renderer stateless"
```

### 任务 5：全量回归与文档回填

**文件：**
- 修改：`CHANGELOG.md`
- 修改：`docs/reviews/psv1-2026-05-05.md`

- [ ] **步骤 1：执行全量测试**

运行：`go test ./...`
预期：PASS。

- [ ] **步骤 2：更新变更日志**

```markdown
## Unreleased
- refactor(cli): split diff pipeline into testable stages
- refactor: inject diff/plan/render via interfaces
- feat(cli): add timeout flag and context propagation
- refactor(render): remove renderer mutable SQL buffer
```

- [ ] **步骤 3：在评审文档回填 P2 状态**

```markdown
- [x] P6 已修复（commit: `$(git rev-parse --short HEAD)`）
- [x] P10 已修复（commit: `$(git rev-parse --short HEAD)`）
- [x] P11 已修复（commit: `$(git rev-parse --short HEAD)`）
- [x] P12 已修复（commit: `$(git rev-parse --short HEAD)`）
```

- [ ] **步骤 4：文档提交**

```bash
git add CHANGELOG.md docs/reviews/psv1-2026-05-05.md
git commit -m "docs: record psv1 p2 remediation"
```

- [ ] **步骤 5：验证提交链**

运行：`git log --oneline -n 5`
预期：包含任务 1-5 的独立提交。

## 自检

1. 规格覆盖度：P6/P10/P11/P12 均映射到任务 1-5。
2. 占位符扫描：无 “TODO/待定/后续实现” 类占位。
3. 类型一致性：`diffConfig`、`runDiffWithDeps`、`Engine` 命名在全文一致。
