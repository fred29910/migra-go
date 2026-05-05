# PSV1 P0/P1 缺陷修复 实现计划

> **面向 AI 代理的工作者：** 必需子技能：使用 superpowers:subagent-driven-development（推荐）或 superpowers:executing-plans 逐任务实现此计划。步骤使用复选框（`- [ ]`）语法来跟踪进度。

**目标：** 完成 `docs/reviews/psv1-2026-05-05.md` 中 P0/P1 问题修复，消除输出不确定性、修复 URL 识别 bug、补齐告警可见性、补齐约束 diff 最小闭环，并优化 DAG 热路径。

**架构：** 以“小步快跑 + 每任务可独立回滚”的方式推进。优先修复功能正确性（P0），随后修复健壮性/可观测性（P1），并通过定向单测与全量回归锁定行为。

**技术栈：** Go 1.24、Cobra、Viper、pgx、`go test`

---

## 范围拆分结论

本评审同时覆盖 CLI、diff、plan、render、架构演进。为降低一次改动跨度，本计划仅覆盖 P0/P1：P1、P4、P5、P7、P8、P9、P14、P18。P2/P3 另立计划。

## 文件结构

**创建：**
- `cmd/migra/diff_test.go`：CLI helper 行为测试（URL 识别、SQL 文件解析 warning）。
- `internal/diff/differ_test.go`：diff 顺序确定性和 warning 行为测试。

**修改：**
- `cmd/migra/diff.go`：修复 `isPostgresURL`、修复 parse error shadowing、输出 diff warning。
- `cmd/migra/main.go`：移除 `init()` 中 panic，新增 `setupFlags() error`。
- `internal/diff/differ.go`：`diffTypes` 排序、warning 收集接口。
- `internal/diff/diff_columns.go`：DEFAULT 变更写入 warning。
- `internal/diff/diff_tables.go`：列删除/namespace 删除写入 warning；增加约束 diff 调用。
- `internal/diff/operation.go`：新增 `AddConstraintOp`/`DropConstraintOp`。
- `internal/render/render.go`：新增约束操作渲染。
- `internal/plan/dag.go`：依赖去重集合 + head 游标队列。
- `internal/plan/dag_test.go`：DAG 去重与顺序测试。
- `CHANGELOG.md`：记录修复项。
- `docs/reviews/psv1-2026-05-05.md`：回填修复状态。

**测试：**
- `go test ./cmd/migra -v`
- `go test ./internal/diff -v`
- `go test ./internal/plan -v`
- `go test ./...`

### 任务 1：修复 URL 识别与 SQL 解析错误可见性（P5 + P9）

**文件：**
- 修改：`cmd/migra/diff.go`
- 测试：`cmd/migra/diff_test.go`

- [ ] **步骤 1：编写失败测试（`pg://` 短串必须识别为 DB URL）**

```go
func TestIsPostgresURL_AcceptsPgScheme(t *testing.T) {
	cases := []string{"pg://db", "pg://a", "postgres://localhost/db"}
	for _, c := range cases {
		if !isPostgresURL(c) {
			t.Fatalf("expected postgres url: %s", c)
		}
	}
}
```

- [ ] **步骤 2：编写失败测试（non-strict 下 parse warning 必须可见）**

```go
func TestLoadFromSQLFile_NonStrictReturnsSchemaAndLogsWarnings(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "bad.sql")
	sql := "CREATE TABLE users (id integer); SELECT * FROM users;"
	if err := os.WriteFile(path, []byte(sql), 0o644); err != nil {
		t.Fatal(err)
	}

	oldStderr := os.Stderr
	r, w, _ := os.Pipe()
	os.Stderr = w

	schema, err := loadFromSQLFile(path, false)

	_ = w.Close()
	os.Stderr = oldStderr
	out, _ := io.ReadAll(r)

	if err != nil {
		t.Fatalf("expected non-strict mode to continue, got: %v", err)
	}
	if schema == nil {
		t.Fatal("expected non-nil schema")
	}
	if !strings.Contains(string(out), "Warning") {
		t.Fatalf("expected warning output, got: %s", string(out))
	}
}
```

- [ ] **步骤 3：实现最小修复代码**

```go
func isPostgresURL(s string) bool {
	return strings.HasPrefix(s, "postgres://") || strings.HasPrefix(s, "pg://")
}

schema, parseErr := p.ParseSQL(string(data))
if parseErr != nil && strict {
	return nil, fmt.Errorf("failed to parse SQL file %s: %w", path, parseErr)
}
for _, warnErr := range p.Errors() {
	fmt.Fprintf(os.Stderr, "Warning [%s]: %v\n", path, warnErr)
}
```

- [ ] **步骤 4：运行定向测试验证通过**

运行：`go test ./cmd/migra -run 'TestIsPostgresURL_AcceptsPgScheme|TestLoadFromSQLFile_NonStrictReturnsSchemaAndLogsWarnings' -v`
预期：PASS。

- [ ] **步骤 5：Commit**

```bash
git add cmd/migra/diff.go cmd/migra/diff_test.go
git commit -m "fix(cli): accept pg:// urls and expose parser warnings"
```

### 任务 2：修复 `diffTypes` 输出顺序不确定（P4）

**文件：**
- 修改：`internal/diff/differ.go`
- 测试：`internal/diff/differ_test.go`

- [ ] **步骤 1：编写失败测试（同输入 50 次输出一致）**

```go
func TestDiffTypes_OrderIsDeterministic(t *testing.T) {
	source := model.NewSchema()
	target := model.NewSchema()
	targetNs := target.GetOrCreateNamespace("public")
	targetNs.Types["z_role"] = &model.EnumType{Name: "z_role", Labels: []string{"a"}}
	targetNs.Types["a_role"] = &model.EnumType{Name: "a_role", Labels: []string{"a"}}
	targetNs.Types["m_role"] = &model.EnumType{Name: "m_role", Labels: []string{"a"}}

	var baseline string
	for i := 0; i < 50; i++ {
		d := NewDiffer()
		ops := d.Diff(source, target)
		sig := make([]string, 0, len(ops))
		for _, op := range ops {
			sig = append(sig, string(op.Kind())+":"+op.ObjectKey().Name)
		}
		joined := strings.Join(sig, ",")
		if i == 0 {
			baseline = joined
			continue
		}
		if joined != baseline {
			t.Fatalf("non-deterministic order: got=%s baseline=%s", joined, baseline)
		}
	}
}
```

- [ ] **步骤 2：运行测试确认当前不稳定**

运行：`go test ./internal/diff -run TestDiffTypes_OrderIsDeterministic -count=50`
预期：修复前出现失败或顺序抖动。

- [ ] **步骤 3：实现排序修复**

```go
targetTypeNames := make([]string, 0, len(target.Types))
for name := range target.Types {
	targetTypeNames = append(targetTypeNames, name)
}
sort.Strings(targetTypeNames)
for _, name := range targetTypeNames {
	enumType := target.Types[name]
	if _, exists := source.Types[name]; !exists {
		d.addOp(NewAddEnumTypeOp(target.Name, enumType))
	}
}

sourceTypeNames := make([]string, 0, len(source.Types))
for name := range source.Types {
	sourceTypeNames = append(sourceTypeNames, name)
}
sort.Strings(sourceTypeNames)
for _, name := range sourceTypeNames {
	if _, exists := target.Types[name]; !exists {
		d.addOp(NewDropEnumTypeOp(source.Name, name))
	}
}
```

- [ ] **步骤 4：复跑测试**

运行：`go test ./internal/diff -run TestDiffTypes_OrderIsDeterministic -count=50`
预期：PASS。

- [ ] **步骤 5：Commit**

```bash
git add internal/diff/differ.go internal/diff/differ_test.go
git commit -m "fix(diff): make enum diff deterministic"
```

### 任务 3：把静默丢弃改为显式 warning（P8）

**文件：**
- 修改：`internal/diff/differ.go`
- 修改：`internal/diff/diff_tables.go`
- 修改：`internal/diff/diff_columns.go`
- 修改：`cmd/migra/diff.go`
- 测试：`internal/diff/differ_test.go`

- [ ] **步骤 1：编写失败测试（DEFAULT 变更 warning）**

```go
func TestDiffer_WarnsOnDefaultExprChange(t *testing.T) {
	src := model.NewSchema()
	tgt := model.NewSchema()
	srcNs := src.GetOrCreateNamespace("public")
	tgtNs := tgt.GetOrCreateNamespace("public")
	srcTable := model.NewTable("public", "users")
	tgtTable := model.NewTable("public", "users")
	a := "'a'"
	b := "'b'"
	srcTable.AddColumn(&model.Column{Name: "name", DataType: "text", DefaultExpr: &a, IsNullable: true})
	tgtTable.AddColumn(&model.Column{Name: "name", DataType: "text", DefaultExpr: &b, IsNullable: true})
	srcNs.Tables["users"] = srcTable
	tgtNs.Tables["users"] = tgtTable

	d := NewDiffer()
	_ = d.Diff(src, tgt)
	if !strings.Contains(strings.Join(d.Warnings(), "\n"), "default change is not implemented") {
		t.Fatalf("expected default warning, got: %v", d.Warnings())
	}
}
```

- [ ] **步骤 2：编写失败测试（namespace/column 删除 warning）**

```go
func TestDiffer_WarnsOnUnsupportedDrops(t *testing.T) {
	src := model.NewSchema()
	tgt := model.NewSchema()
	srcNs := src.GetOrCreateNamespace("legacy")
	tbl := model.NewTable("legacy", "users")
	tbl.AddColumn(&model.Column{Name: "obsolete_col", DataType: "integer", IsNullable: true})
	srcNs.Tables["users"] = tbl
	tgt.GetOrCreateNamespace("public")

	d := NewDiffer()
	_ = d.Diff(src, tgt)
	warnings := strings.Join(d.Warnings(), "\n")
	if !strings.Contains(warnings, "namespace drop is not implemented") {
		t.Fatalf("expected namespace warning, got: %s", warnings)
	}
	if !strings.Contains(warnings, "column drop is not implemented") {
		t.Fatalf("expected column warning, got: %s", warnings)
	}
}
```

- [ ] **步骤 3：实现 warning 收集与 CLI 输出**

```go
type Differ struct {
	ops      []Operation
	warnings []string
}

func (d *Differ) warnf(format string, args ...any) {
	d.warnings = append(d.warnings, fmt.Sprintf(format, args...))
}

func (d *Differ) Warnings() []string {
	out := make([]string, len(d.warnings))
	copy(out, d.warnings)
	return out
}
```

```go
if !sameDefault(source.DefaultExpr, target.DefaultExpr) {
	d.warnf("column %s.%s.%s default change is not implemented yet (ignored)", schema, table, source.Name)
}
```

```go
for _, w := range differ.Warnings() {
	fmt.Fprintf(os.Stderr, "Warning: %s\n", w)
}
```

- [ ] **步骤 4：运行定向测试**

运行：`go test ./internal/diff -run 'TestDiffer_WarnsOnDefaultExprChange|TestDiffer_WarnsOnUnsupportedDrops' -v`
预期：PASS。

- [ ] **步骤 5：Commit**

```bash
git add internal/diff/differ.go internal/diff/diff_tables.go internal/diff/diff_columns.go cmd/migra/diff.go internal/diff/differ_test.go
git commit -m "feat(diff): report warnings for unsupported changes"
```

### 任务 4：优化 DAG 热路径（P1 + P18）

**文件：**
- 修改：`internal/plan/dag.go`
- 修改：`internal/plan/dag_test.go`

- [ ] **步骤 1：编写失败测试（重复依赖去重）**

```go
func TestDAGAddDependency_DeduplicatesEdges(t *testing.T) {
	dag := NewDAG()
	a := dag.AddNode(diff.NewAddTableOp("public", "a", model.NewTable("public", "a")))
	b := dag.AddNode(diff.NewAddTableOp("public", "b", model.NewTable("public", "b")))
	dag.AddDependency(a, b)
	dag.AddDependency(a, b)
	if len(a.Dependencies) != 1 {
		t.Fatalf("expected 1 dependency, got %d", len(a.Dependencies))
	}
	if len(b.Dependents) != 1 {
		t.Fatalf("expected 1 dependent, got %d", len(b.Dependents))
	}
}
```

- [ ] **步骤 2：编写失败测试（线性链顺序稳定）**

```go
func TestDAGGetExecutionOrder_StableForLinearChain(t *testing.T) {
	dag := NewDAG()
	c := dag.AddNode(diff.NewAddTableOp("public", "c", model.NewTable("public", "c")))
	b := dag.AddNode(diff.NewAddTableOp("public", "b", model.NewTable("public", "b")))
	a := dag.AddNode(diff.NewAddTableOp("public", "a", model.NewTable("public", "a")))
	dag.AddDependency(a, b)
	dag.AddDependency(b, c)

	ops, err := dag.GetExecutionOrder()
	if err != nil {
		t.Fatalf("GetExecutionOrder failed: %v", err)
	}
	got := []string{ops[0].ObjectKey().Name, ops[1].ObjectKey().Name, ops[2].ObjectKey().Name}
	want := []string{"c", "b", "a"}
	if strings.Join(got, ",") != strings.Join(want, ",") {
		t.Fatalf("unexpected order: got=%v want=%v", got, want)
	}
}
```

- [ ] **步骤 3：实现 `DependencySet` + head 游标**

```go
type Node struct {
	Op            diff.Operation
	Dependencies  []*Node
	Dependents    []*Node
	DependencySet map[*Node]struct{}
}

if _, exists := from.DependencySet[to]; exists {
	return
}
from.DependencySet[to] = struct{}{}
from.Dependencies = append(from.Dependencies, to)
```

```go
queue := make([]*Node, 0, len(d.nodes))
head := 0
for head < len(queue) {
	node := queue[head]
	head++
	result = append(result, node.Op)
	for _, dep := range node.Dependents {
		inDegree[dep]--
		if inDegree[dep] == 0 {
			queue = append(queue, dep)
		}
	}
}
```

- [ ] **步骤 4：运行 plan 包测试**

运行：`go test ./internal/plan -v`
预期：PASS。

- [ ] **步骤 5：Commit**

```bash
git add internal/plan/dag.go internal/plan/dag_test.go
git commit -m "perf(plan): optimize dag dedupe and queue traversal"
```

### 任务 5：移除 `init()` panic（P7）

**文件：**
- 修改：`cmd/migra/main.go`
- 测试：`cmd/migra/diff_test.go`

- [ ] **步骤 1：编写失败测试（`setupFlags` 可成功绑定）**

```go
func TestSetupFlags_BindsViperKeys(t *testing.T) {
	cmd := &cobra.Command{Use: "test"}
	cmd.PersistentFlags().String("config", "", "")
	cmd.PersistentFlags().Bool("verbose", false, "")
	if err := setupFlags(cmd); err != nil {
		t.Fatalf("setupFlags failed: %v", err)
	}
}
```

- [ ] **步骤 2：实现 `setupFlags` 并在 `main` 中处理错误**

```go
func setupFlags(cmd *cobra.Command) error {
	if err := viper.BindPFlag("config", cmd.PersistentFlags().Lookup("config")); err != nil {
		return fmt.Errorf("failed to bind config flag: %w", err)
	}
	if err := viper.BindPFlag("verbose", cmd.PersistentFlags().Lookup("verbose")); err != nil {
		return fmt.Errorf("failed to bind verbose flag: %w", err)
	}
	return nil
}

func main() {
	if err := setupFlags(rootCmd); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	if err := rootCmd.Execute(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}
```

- [ ] **步骤 3：运行 cmd 测试**

运行：`go test ./cmd/migra -v`
预期：PASS。

- [ ] **步骤 4：手工 smoke test**

运行：`go run ./cmd/migra --help`
预期：退出码 0。

- [ ] **步骤 5：Commit**

```bash
git add cmd/migra/main.go cmd/migra/diff_test.go
git commit -m "refactor(cli): replace init panic with setup error handling"
```

### 任务 6：补齐约束 diff 最小闭环（P14）

**文件：**
- 修改：`internal/diff/operation.go`
- 修改：`internal/diff/diff_tables.go`
- 修改：`internal/render/render.go`
- 测试：`internal/diff/differ_test.go`
- 测试：`cmd/migra/integration_test.go`

- [ ] **步骤 1：编写失败测试（约束增删应产出操作）**

```go
func TestDiffTableConstraints_AddAndDrop(t *testing.T) {
	source := model.NewSchema()
	target := model.NewSchema()
	srcNs := source.GetOrCreateNamespace("public")
	tgtNs := target.GetOrCreateNamespace("public")
	srcTable := model.NewTable("public", "users")
	tgtTable := model.NewTable("public", "users")
	srcTable.Constraints["uq_users_email"] = &model.Constraint{Name: "uq_users_email", Type: "unique", Definition: "UNIQUE (email)", Table: "users"}
	tgtTable.Constraints["ck_users_age"] = &model.Constraint{Name: "ck_users_age", Type: "check", Definition: "CHECK (age >= 0)", Table: "users"}
	srcNs.Tables["users"] = srcTable
	tgtNs.Tables["users"] = tgtTable

	d := NewDiffer()
	ops := d.Diff(source, target)
	var hasAdd, hasDrop bool
	for _, op := range ops {
		if op.Kind() == KindAddConstraint {
			hasAdd = true
		}
		if op.Kind() == KindDropConstraint {
			hasDrop = true
		}
	}
	if !hasAdd || !hasDrop {
		t.Fatalf("expected add/drop constraint ops, got add=%v drop=%v", hasAdd, hasDrop)
	}
}
```

- [ ] **步骤 2：在 `operation.go` 新增约束操作类型**

```go
type AddConstraintOp struct {
	baseOperation
	Schema     string
	Table      string
	Constraint *model.Constraint
}

func NewAddConstraintOp(schema, table string, c *model.Constraint) *AddConstraintOp {
	return &AddConstraintOp{
		baseOperation: baseOperation{kind: KindAddConstraint, objectKey: model.NewObjectKey(schema, table+"."+c.Name, model.KindConstraint)},
		Schema: schema,
		Table:  table,
		Constraint: c,
	}
}

type DropConstraintOp struct {
	baseOperation
	Schema string
	Table  string
	Name   string
}
```

- [ ] **步骤 3：在 `diff_tables.go` 接入 `diffTableConstraints`**

```go
for _, name := range bothNames {
	targetTable := target.Tables[name]
	sourceTable := source.Tables[name]
	d.diffTableColumns(source.Name, sourceTable, targetTable)
	d.diffTableIndexes(source.Name, sourceTable, targetTable)
	d.diffTableConstraints(source.Name, sourceTable, targetTable)
}

func (d *Differ) diffTableConstraints(schema string, source, target *model.Table) {
	for name, c := range target.Constraints {
		if _, exists := source.Constraints[name]; !exists {
			d.addOp(NewAddConstraintOp(schema, target.Name, c))
		}
	}
	for name := range source.Constraints {
		if _, exists := target.Constraints[name]; !exists {
			d.addOp(NewDropConstraintOp(schema, source.Name, name))
		}
	}
}
```

- [ ] **步骤 4：在 `render.go` 增加约束渲染**

```go
case *diff.AddConstraintOp:
	return fmt.Sprintf("-- op: add_constraint risk:medium\nALTER TABLE %s ADD CONSTRAINT %s %s;",
		quoteQualifiedIdentifier(v.Schema, v.Table),
		quoteIdentifier(v.Constraint.Name),
		v.Constraint.Definition,
	)
case *diff.DropConstraintOp:
	return fmt.Sprintf("-- op: drop_constraint risk:medium\nALTER TABLE %s DROP CONSTRAINT %s%s;",
		quoteQualifiedIdentifier(v.Schema, v.Table),
		ifExistsPrefix(r.useIfExists),
		quoteIdentifier(v.Name),
	)
```

- [ ] **步骤 5：运行测试并提交**

运行：`go test ./internal/diff ./cmd/migra -run 'TestDiffTableConstraints_AddAndDrop|TestIntegrationDiffRender' -v`
预期：PASS。

```bash
git add internal/diff/operation.go internal/diff/diff_tables.go internal/render/render.go internal/diff/differ_test.go cmd/migra/integration_test.go
git commit -m "feat(diff): add minimal constraint add/drop pipeline"
```

### 任务 7：全量回归与文档回填

**文件：**
- 修改：`CHANGELOG.md`
- 修改：`docs/reviews/psv1-2026-05-05.md`

- [ ] **步骤 1：执行全量测试**

运行：`go test ./...`
预期：PASS。

- [ ] **步骤 2：更新 CHANGELOG**

```markdown
## Unreleased
- fix(cli): accept `pg://` short postgres URLs
- fix(diff): deterministic enum diff ordering
- feat(diff): report warnings for unsupported changes
- perf(plan): optimize DAG dedupe and queue traversal
- feat(diff): add minimal add/drop constraint pipeline
- refactor(cli): replace init panic with setup error handling
```

- [ ] **步骤 3：在评审文档标记修复状态**

```markdown
- [x] P4 已修复（commit: `$(git rev-parse --short HEAD)`）
- [x] P5 已修复（commit: `$(git rev-parse --short HEAD)`）
- [x] P7 已修复（commit: `$(git rev-parse --short HEAD)`）
- [x] P8 已修复（commit: `$(git rev-parse --short HEAD)`）
- [x] P9 已修复（commit: `$(git rev-parse --short HEAD)`）
- [x] P14 已修复（commit: `$(git rev-parse --short HEAD)`）
- [x] P1/P18 已修复（commit: `$(git rev-parse --short HEAD)`）
```

- [ ] **步骤 4：文档提交**

```bash
git add CHANGELOG.md docs/reviews/psv1-2026-05-05.md
git commit -m "docs: record psv1 p0/p1 remediation"
```

- [ ] **步骤 5：验证提交序列**

运行：`git log --oneline -n 7`
预期：包含任务 1-7 的独立提交。

## 自检

1. 规格覆盖度：P0/P1 全部映射到任务 1-7。
2. 占位符扫描：无 “TODO/待定/后续实现” 类占位步骤。
3. 类型一致性：`Differ.Warnings()`、`warnf(...)`、`setupFlags(...)`、`DependencySet` 在文档内保持一致。

## 后续独立计划（不在本文件执行范围）

- `docs/plans/2026-05-05-psv1-p2-runtime-refactor-plan.md`：P6/P10/P12
- `docs/plans/2026-05-05-psv1-p3-architecture-evolution-plan.md`：P3/P11/P15/P16/P17
