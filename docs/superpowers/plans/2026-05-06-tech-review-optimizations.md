# migra-go 技术评审优化 实现计划

> **面向 AI 代理的工作者：** 必需子技能：使用 superpowers:subagent-driven-development（推荐）或 superpowers:executing-plans 逐任务实现此计划。步骤使用复选框（`- [ ]`）语法来跟踪进度。

**目标：** 根据 2026-05-05 深度技术评审报告，修复 migra-go 的性能瓶颈、保证并发安全、提升代码质量并重构依赖。

**架构：** 对 Diff 引擎和规范化逻辑引入内存优化和确定性排序；解决 Parser 和 Enum 加载的错误上下文丢失；将引擎和服务的接口重新命名使其明确；并最终重构依赖计算结构（DAG）。

**技术栈：** Go 1.21+, Postgres SQL

---

### 任务 1：修复约束/索引 diff 的随机迭代及 SQL 注入边界（Issue 1, 8）

**文件：**
- 修改：`internal/diff/diff_tables.go:69-130`
- 修改：`internal/render/render.go:171-180`
- 测试：`internal/diff/diff_tables_test.go` 和 `internal/render/render_test.go`

- [ ] **步骤 1：编写失败的测试**

```go
// internal/render/render_test.go 中补充测试
func TestRenderCreateIndex_Quoted(t *testing.T) {
	r := NewRenderer()
	op := &diff.CreateIndexOp{
		Schema: "public",
		Index: &model.Index{
			Name:    "idx_user",
			Table:   "users",
			Columns: []string{"User Name", "id"},
		},
	}
	sql := r.renderCreateIndex(op)
	expected := "-- op: add_index risk:low\nCREATE INDEX \"idx_user\" ON \"public\".\"users\" (\"User Name\", \"id\");"
	if sql != expected {
		t.Errorf("got %q, want %q", sql, expected)
	}
}
```

- [ ] **步骤 2：运行测试验证失败**

运行：`go test ./internal/render -v`
预期：FAIL，输出 `CREATE INDEX "idx_user" ON "public"."users" (User Name, id);` (缺少引号)

- [ ] **步骤 3：编写最少实现代码**

修改 `internal/diff/diff_tables.go`：
```go
func (c *diffContext) diffTableConstraints(schema string, source, target *model.Table) {
	addNames := make([]string, 0, len(target.Constraints))
	for name := range target.Constraints {
		if _, exists := source.Constraints[name]; !exists {
			addNames = append(addNames, name)
		}
	}
	sort.Strings(addNames)
	for _, name := range addNames {
		c.addOp(NewAddConstraintOp(schema, target.Name, target.Constraints[name]))
	}

	dropNames := make([]string, 0, len(source.Constraints))
	for name := range source.Constraints {
		if _, exists := target.Constraints[name]; !exists {
			dropNames = append(dropNames, name)
		}
	}
	sort.Strings(dropNames)
	for _, name := range dropNames {
		c.addOp(NewDropConstraintOp(schema, source.Name, name))
	}
}

func (c *diffContext) diffTableIndexes(schema string, source, target *model.Table) {
	addIdx := make([]string, 0, len(target.Indexes))
	for name := range target.Indexes {
		if _, exists := source.Indexes[name]; !exists {
			addIdx = append(addIdx, name)
		}
	}
	sort.Strings(addIdx)
	for _, name := range addIdx {
		c.addOp(NewCreateIndexOp(schema, target.Indexes[name]))
	}

	dropIdx := make([]string, 0, len(source.Indexes))
	for name := range source.Indexes {
		if _, exists := target.Indexes[name]; !exists {
			dropIdx = append(dropIdx, name)
		}
	}
	sort.Strings(dropIdx)
	for _, name := range dropIdx {
		c.addOp(NewDropIndexOp(schema, source.Name, name))
	}
}
```

修改 `internal/render/render.go`：
```go
func (r *Renderer) renderCreateIndex(op *diff.CreateIndexOp) string {
	idx := op.Index
	unique := ""
	if idx.Unique {
		unique = "UNIQUE "
	}
	quotedCols := make([]string, len(idx.Columns))
	for i, c := range idx.Columns {
		quotedCols[i] = quoteIdentifier(c)
	}
	columns := strings.Join(quotedCols, ", ")
	return fmt.Sprintf("-- op: add_index risk:low\nCREATE %sINDEX %s ON %s (%s);",
		unique, quoteQualifiedIdentifier(op.Schema, idx.Name), quoteQualifiedIdentifier(op.Schema, idx.Table), columns)
}
```

- [ ] **步骤 4：运行测试验证通过**

运行：`go test ./internal/diff ./internal/render -v`
预期：PASS

- [ ] **步骤 5：Commit**

```bash
git add internal/diff/diff_tables.go internal/render/render.go internal/render/render_test.go
git commit -m "fix: make map iteration deterministic and quote index columns"
```

### 任务 2：性能优化：ComputeDiff 和 normalize 避免重复扫描与内存分配（Issue 2, 3, 4）

**文件：**
- 修改：`internal/app/diff_service.go:91-131`
- 修改：`internal/normalize/normalize.go:30-65`

- [ ] **步骤 1：编写失败的测试**

没有新的行为变化，此任务只涉及优化，无需修改已有用例逻辑。我们通过运行 benchmark 验证：
```bash
go test -bench=. ./internal/app ./internal/normalize
```

- [ ] **步骤 2：编写最少实现代码**

修改 `internal/app/diff_service.go` 的 `ComputeDiff`：
```go
	// Report destructive changes (Single pass logic)
	var destructiveOps []diff.Operation
	for _, op := range operations {
		if op.IsDestructive() {
			destructiveOps = append(destructiveOps, op)
		}
	}
	
	if len(destructiveOps) > 0 {
		warnings = append(warnings, fmt.Sprintf("%d destructive operation(s) detected!", len(destructiveOps)))
		for _, op := range destructiveOps {
			warnings = append(warnings, fmt.Sprintf("  - %s: %s (destructive)", op.Kind(), op.ObjectKey()))
		}
		if !cfg.UnsafeDrop {
			warnings = append(warnings, "Use --unsafe-drop to include destructive DROP operations in output")
		}
	}

	// Plan execution
	planner := plan.NewPlanner(cfg.UnsafeDrop)
	stages := planner.Plan(operations)

	// Build execution list with deterministic stage order and topo sorting
	stageOrder := []plan.Stage{
		plan.StagePreDeploy,
		plan.StageDeploy,
		plan.StagePostDeploy,
	}

	allOps := make([]diff.Operation, 0, len(operations))
	for _, stage := range stageOrder {
		stageOps := stages[stage]
		if len(stageOps) == 0 {
			continue
		}
		sortedStageOps, err := plan.TopoSort(stageOps)
		if err != nil {
			return nil, nil, fmt.Errorf("failed to topologically sort %s operations: %w", stage, err)
		}
		allOps = append(allOps, sortedStageOps...)
	}
```

修改 `internal/normalize/normalize.go` 中的函数：
```go
func canonicalizeTableInPlace(table *model.Table) {
	newColumnByName := make(map[string]*model.Column, len(table.Columns))
	for _, col := range table.Columns {
		canonicalizeColumnInPlace(col)
		newColumnByName[col.Name] = col
	}
	table.ColumnByName = newColumnByName
}

func canonicalizeColumnInPlace(col *model.Column) {
	col.Name = normalizeIdentifier(col.Name)
	col.DataType = normalizeDataType(col.DataType)
	if col.DefaultExpr != nil {
		*col.DefaultExpr = normalizeDefaultExpr(*col.DefaultExpr)
	}
}
```

- [ ] **步骤 3：运行测试验证通过**

运行：`go test ./internal/app ./internal/normalize -v`
预期：PASS

- [ ] **步骤 4：Commit**

```bash
git add internal/app/diff_service.go internal/normalize/normalize.go
git commit -m "perf: preallocate slices/maps and reduce heap allocation in normalize"
```

### 任务 3：提升代码质量与报错上下文可观测性（Issue 5, 6, 9）

**文件：**
- 修改：`internal/parser/parser.go:68-71` 和 `305-307`
- 修改：`internal/introspect/enums.go:53-55`

- [ ] **步骤 1：编写最少实现代码**

修改 `internal/parser/parser.go`：
```go
func (p *Parser) ParseSQL(sql string) (*model.Schema, error) {
    // ...
	if len(p.errors) > 0 {
		return p.schema, fmt.Errorf("parsing completed with %d errors, first: %w", len(p.errors), p.errors[0])
	}
	return p.schema, nil
}

func (p *Parser) Errors() []error {
	out := make([]error, len(p.errors))
	copy(out, p.errors)
	return out
}
```

修改 `internal/introspect/enums.go`：
```go
	if err := rows.Err(); err != nil {
		return fmt.Errorf("iterate enum rows: %w", err)
	}
	return nil
```

- [ ] **步骤 2：运行测试验证通过**

运行：`go test ./internal/parser ./internal/introspect -v`
预期：PASS

- [ ] **步骤 3：Commit**

```bash
git add internal/parser/parser.go internal/introspect/enums.go
git commit -m "refactor: improve error wrapping and prevent parser error slice mutation"
```

### 任务 4：统一接口语义与 Differ 并发安全标记（Issue 7, 10）

**文件：**
- 修改：`internal/app/diff_service.go:28`
- 修改：`internal/diff/differ.go:10-30`
- 修改：`internal/plan/plan.go:24-30`

- [ ] **步骤 1：重命名服务接口并提供无状态 Diff API**

修改 `internal/app/diff_service.go`：
```go
type DiffService interface {
	Run(ctx context.Context, cfg Config) (output string, warnings []string, err error)
}
// 并更新使用它的地方，如 newService 返回 DiffService
```

修改 `internal/diff/differ.go`：
```go
type DiffEngine interface {
	Diff(source, target *model.Schema) ([]Operation, []string)
}

// Ensure Differ satisfies DiffEngine
var _ DiffEngine = (*Differ)(nil)

// Differ is not safe for concurrent use.
type Differ struct { }

func NewDiffer() *Differ { return &Differ{} }

func (d *Differ) Diff(source, target *model.Schema) ([]Operation, []string) {
	ctx := newDiffContext(source, target)
	ctx.diffTables(source, target)
	ctx.diffEnums(source, target)
	return ctx.operations, ctx.warnings
}
```
*(注意：需要清理 `Warnings()` 方法，并同步修改 `diffContext` 的返回逻辑，这可能需要在 `diff_service.go:87` 中同步调用 `operations, warnings := differ.Diff(source, target)`。)*

修改 `internal/plan/plan.go`：
```go
type PlanEngine interface {
	Plan(ops []diff.Operation) map[Stage][]diff.Operation
}
var _ PlanEngine = (*Planner)(nil)
```

- [ ] **步骤 2：运行测试并修复任何由于重构带来的编译错误**

运行：`go build ./... && go test ./...`
预期：可能在 `internal/app/diff_service.go` 中有错误（调用 `differ.Warnings()`），需要修复为接收双返回值。

- [ ] **步骤 3：Commit**

```bash
git add internal/app/diff_service.go internal/diff/differ.go internal/plan/plan.go
git commit -m "refactor: rename Engine interfaces and make Differ stateless API"
```
