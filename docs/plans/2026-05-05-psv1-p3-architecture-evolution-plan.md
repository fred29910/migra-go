# PSV1 P3 架构演进 实现计划

> **面向 AI 代理的工作者：** 必需子技能：使用 superpowers:subagent-driven-development（推荐）或 superpowers:executing-plans 逐任务实现此计划。步骤使用复选框（`- [ ]`）语法来跟踪进度。

**目标：** 完成 P3（P3/P15/P16/P17）架构演进，降低内存分配、移除状态泄漏、统一渲染入口，并建立 `cmd` 与业务编排层分离。

**架构：** 以“兼容优先”的迁移策略推进：先引入新 API 和新目录，再切换调用方，最后删除旧路径。每个任务均保持 `go test ./...` 绿灯，避免一次性重写。

**技术栈：** Go 1.24、Cobra、Viper、pgx、`go test`

---

## 文件结构

**创建：**
- `internal/app/diff_service.go`：业务编排层（schema 加载、normalize、diff、plan、render）。
- `internal/app/diff_service_test.go`：编排层单测。
- `internal/diff/context.go`：无状态 diff 上下文容器（`diffContext`）。

**修改：**
- `internal/normalize/normalize.go`：改为就地标准化 API。
- `internal/diff/differ.go`：移除 `Differ.ops` 字段，改局部上下文累积。
- `internal/render/render.go`：删除 `format` / `SetFormat`，收敛渲染接口。
- `cmd/migra/diff.go`：调用 `internal/app` 服务层。
- `cmd/migra/main.go`：保持命令层薄适配。
- `cmd/migra/integration_test.go`：回归链路。

**测试：**
- `internal/normalize` 定向测试
- `internal/diff` 定向测试
- `internal/app` 单测
- `cmd/migra` 集成回归

### 任务 1：将 `normalize` 改为就地标准化（P3）

**文件：**
- 修改：`internal/normalize/normalize.go`
- 修改：`internal/model/golden_test.go`（如需）
- 测试：`internal/model/schema_test.go`

- [ ] **步骤 1：编写失败测试（原对象地址不变且值被标准化）**

```go
func TestCanonicalizeSchema_InPlace(t *testing.T) {
	s := model.NewSchema()
	ns := s.GetOrCreateNamespace("Public")
	tbl := model.NewTable("Public", "Users")
	d := "( now() )"
	tbl.AddColumn(&model.Column{Name: "Name", DataType: "INT4", DefaultExpr: &d, IsNullable: true})
	ns.Tables["Users"] = tbl

	beforePtr := tbl.Columns[0]
	if err := CanonicalizeSchema(s); err != nil {
		t.Fatal(err)
	}
	afterPtr := s.Schemas["Public"].Tables["Users"].Columns[0]
	if beforePtr != afterPtr {
		t.Fatal("expected in-place mutation without replacing column pointer")
	}
	if afterPtr.DataType != "integer" {
		t.Fatalf("expected normalized type integer, got %s", afterPtr.DataType)
	}
}
```

- [ ] **步骤 2：修改 API 为 `func CanonicalizeSchema(s *model.Schema) error`**

```go
func CanonicalizeSchema(s *model.Schema) error {
	for _, ns := range s.Schemas {
		canonicalizeNamespaceInPlace(ns)
	}
	return nil
}
```

- [ ] **步骤 3：实现 namespace/table/column/constraint/index 的就地变换**

```go
func canonicalizeColumnInPlace(col *model.Column) {
	col.Name = normalizeIdentifier(col.Name)
	col.DataType = normalizeDataType(col.DataType)
	if col.DefaultExpr != nil {
		n := normalizeDefaultExpr(*col.DefaultExpr)
		col.DefaultExpr = &n
	}
}
```

- [ ] **步骤 4：修正调用方签名并跑测试**

运行：`go test ./internal/normalize ./internal/model -v`
预期：PASS。

- [ ] **步骤 5：Commit**

```bash
git add internal/normalize/normalize.go internal/model/golden_test.go internal/model/schema_test.go
git commit -m "perf(normalize): switch to in-place canonicalization"
```

### 任务 2：将 `Differ` 改为无状态（P15）

**文件：**
- 创建：`internal/diff/context.go`
- 修改：`internal/diff/differ.go`
- 修改：`internal/diff/diff_tables.go`
- 修改：`internal/diff/diff_columns.go`
- 测试：`internal/diff/differ_test.go`

- [ ] **步骤 1：编写失败测试（同实例并发 Diff 无共享状态污染）**

```go
func TestDiffer_DiffNoSharedState(t *testing.T) {
	d := NewDiffer()
	src := model.NewSchema()
	tgt := model.NewSchema()
	tgt.GetOrCreateNamespace("public")
	for i := 0; i < 20; i++ {
		ops := d.Diff(src, tgt)
		if len(ops) != 0 {
			t.Fatalf("expected 0 ops, got %d", len(ops))
		}
	}
}
```

- [ ] **步骤 2：定义局部上下文并承载 `ops/warnings`**

```go
type diffContext struct {
	ops      []Operation
	warnings []string
}

func (c *diffContext) addOp(op Operation) { c.ops = append(c.ops, op) }
func (c *diffContext) warnf(format string, args ...any) {
	c.warnings = append(c.warnings, fmt.Sprintf(format, args...))
}
```

- [ ] **步骤 3：把 `d.diff*` 递归改为 `ctx.diff*`**

```go
func (d *Differ) Diff(source, target *model.Schema) []Operation {
	ctx := &diffContext{ops: make([]Operation, 0, 16), warnings: make([]string, 0, 4)}
	ctx.diffSchemas(source, target)
	d.lastWarnings = append(d.lastWarnings[:0], ctx.warnings...)
	return ctx.ops
}

type Differ struct {
	lastWarnings []string
}
```

- [ ] **步骤 4：运行 diff 包测试**

运行：`go test ./internal/diff -v`
预期：PASS。

- [ ] **步骤 5：Commit**

```bash
git add internal/diff/context.go internal/diff/differ.go internal/diff/diff_tables.go internal/diff/diff_columns.go internal/diff/differ_test.go
git commit -m "refactor(diff): move mutable state into local diff context"
```

### 任务 3：统一渲染入口并删除死代码字段（P16）

**文件：**
- 修改：`internal/render/render.go`
- 修改：`cmd/migra/diff_runner.go`（或 `cmd/migra/diff.go`）
- 测试：`internal/render/render_test.go`

- [ ] **步骤 1：编写失败测试（统一入口支持 sql/json）**

```go
func TestRenderOutput_SupportsSQLAndJSON(t *testing.T) {
	ops := []diff.Operation{diff.NewDropIndexOp("public", "idx_a")}
	r := NewRenderer()
	sql, err := r.RenderOutput(ops, "sql")
	if err != nil || !strings.Contains(sql, "DROP INDEX") {
		t.Fatalf("unexpected sql render: %v %q", err, sql)
	}
	js, err := r.RenderOutput(ops, "json")
	if err != nil || !strings.Contains(js, `"kind"`) {
		t.Fatalf("unexpected json render: %v %q", err, js)
	}
}
```

- [ ] **步骤 2：实现 `RenderOutput`，删除 `format` 字段和 `SetFormat`**

```go
type Renderer struct {
	useIfExists bool
}

func (r *Renderer) RenderOutput(ops []diff.Operation, format string) (string, error) {
	switch format {
	case "sql":
		return r.RenderAll(ops), nil
	case "json":
		return RenderJSON(ops)
	default:
		return "", fmt.Errorf("unsupported format: %s (allowed: sql, json)", format)
	}
}
```

- [ ] **步骤 3：调用方改为单入口**

```go
renderer := render.NewRenderer()
outputText, err := renderer.RenderOutput(allOps, cfg.format)
if err != nil { return err }
```

- [ ] **步骤 4：运行 render/cmd 测试**

运行：`go test ./internal/render ./cmd/migra -v`
预期：PASS。

- [ ] **步骤 5：Commit**

```bash
git add internal/render/render.go internal/render/render_test.go cmd/migra/diff_runner.go cmd/migra/diff.go
git commit -m "refactor(render): unify format rendering entrypoint"
```

### 任务 4：引入 `internal/app` 业务编排层并瘦身 `cmd`（P17）

**文件：**
- 创建：`internal/app/diff_service.go`
- 创建：`internal/app/diff_service_test.go`
- 修改：`cmd/migra/diff.go`
- 修改：`cmd/migra/diff_runner.go`

- [ ] **步骤 1：编写失败测试（service 输入 source/target 返回文本）**

```go
func TestDiffService_Run(t *testing.T) {
	svc := NewDiffService(NewDefaultDeps())
	cfg := Config{Source: "testdata/example_source.sql", Target: "testdata/example_target.sql", Format: "sql", Schemas: []string{"public"}, Timeout: 5 * time.Second}
	out, warns, err := svc.Run(context.Background(), cfg)
	if err != nil { t.Fatal(err) }
	if len(out) == 0 { t.Fatal("expected non-empty output") }
	_ = warns
}
```

- [ ] **步骤 2：在 `internal/app` 定义配置与服务接口**

```go
type Config struct {
	Source     string
	Target     string
	Schemas    []string
	Format     string
	OutputFile string
	UnsafeDrop bool
	Strict     bool
	Timeout    time.Duration
}

type Service interface {
	Run(ctx context.Context, cfg Config) (output string, warnings []string, err error)
}
```

- [ ] **步骤 3：迁移 `loadSchema/compute/plan/render` 到 service**

```go
func (s *DiffService) Run(parent context.Context, cfg Config) (string, []string, error) {
	ctx, cancel := context.WithTimeout(parent, cfg.Timeout)
	defer cancel()
	// load -> normalize -> diff -> plan -> topo -> render
}
```

- [ ] **步骤 4：让 `cmd` 层仅做参数解析和输出写入**

```go
func runDiff(cmd *cobra.Command, args []string) error {
	cfg, err := parseDiffConfig(cmd, args)
	if err != nil { return err }
	out, warns, err := app.NewDiffService(app.NewDefaultDeps()).Run(cmd.Context(), app.Config{
		Source:     cfg.source,
		Target:     cfg.target,
		Schemas:    cfg.schemas,
		Format:     cfg.format,
		OutputFile: cfg.outputFile,
		UnsafeDrop: cfg.unsafeDrop,
		Strict:     cfg.strict,
		Timeout:    cfg.timeout,
	})
	if err != nil { return err }
	for _, w := range warns { fmt.Fprintf(os.Stderr, "Warning: %s\n", w) }
	return writeOutput(out, cfg.outputFile)
}
```

- [ ] **步骤 5：运行测试并提交**

运行：`go test ./internal/app ./cmd/migra -v`
预期：PASS。

```bash
git add internal/app/diff_service.go internal/app/diff_service_test.go cmd/migra/diff.go cmd/migra/diff_runner.go
git commit -m "refactor(app): move diff orchestration into internal app service"
```

### 任务 5：全量回归与文档回填

**文件：**
- 修改：`CHANGELOG.md`
- 修改：`docs/reviews/psv1-2026-05-05.md`

- [ ] **步骤 1：执行全量测试**

运行：`go test ./...`
预期：PASS。

- [ ] **步骤 2：更新 CHANGELOG**

```markdown
## Unreleased
- perf(normalize): switch canonicalization to in-place mutation
- refactor(diff): use local diff context, remove shared mutable ops path
- refactor(render): unify sql/json rendering entrypoint
- refactor(app): introduce internal app service and thin cmd adapter
```

- [ ] **步骤 3：在评审文档回填 P3 状态**

```markdown
- [x] P3 已修复（commit: `$(git rev-parse --short HEAD)`）
- [x] P15 已修复（commit: `$(git rev-parse --short HEAD)`）
- [x] P16 已修复（commit: `$(git rev-parse --short HEAD)`）
- [x] P17 已修复（commit: `$(git rev-parse --short HEAD)`）
```

- [ ] **步骤 4：文档提交**

```bash
git add CHANGELOG.md docs/reviews/psv1-2026-05-05.md
git commit -m "docs: record psv1 p3 architecture evolution"
```

- [ ] **步骤 5：验证提交链**

运行：`git log --oneline -n 5`
预期：包含任务 1-5 的独立提交。

## 自检

1. 规格覆盖度：P3/P15/P16/P17 均映射到任务 1-5。
2. 占位符扫描：无 “TODO/待定/后续实现” 类占位。
3. 类型一致性：`DiffService`、`Config`、`RenderOutput`、`diffContext` 在全文一致。
