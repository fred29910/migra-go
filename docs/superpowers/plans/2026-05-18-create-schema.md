# CREATE SCHEMA / DROP SCHEMA Diff + Render 实现计划

> **面向 AI 代理的工作者：** 必需子技能：使用 superpowers:subagent-driven-development（推荐）或 superpowers:executing-plans 逐任务实现此计划。步骤使用复选框（`- [ ]`）语法来跟踪进度。

**目标：** 在 migra-go 的 diff 引擎中生成 `CreateSchemaOp`（目标有而源没有的命名空间），在渲染器中输出 `CREATE SCHEMA IF NOT EXISTS` SQL；添加 `DropSchemaOp` 类型和 `DROP SCHEMA IF EXISTS` 渲染供将来使用（当前 diff 引擎仍只给警告）。

**架构：**
1. `operation.go` 新增两个 Kind 常量（`KindCreateSchema` / `KindDropSchema`）和两个 Operation 结构体（`CreateSchemaOp` / `DropSchemaOp`），遵循已有模式（嵌入 `baseOperation`，构造器函数，`IsDestructive` 方法）。
2. `differ.go` 的 `diffSchemas()` 在检测到目标 namespace 在源中不存在时，emit `CreateSchemaOp` 而不是只 `warnf`。源有目标没有的 namespace 继续保持 `warnf`（DROP 支持将来再做），但 `DropSchemaOp` 类型已准备好。
3. `render.go` 的 `Render()` switch 添加两个新 case：调用 `renderCreateSchema`（输出 `CREATE SCHEMA IF NOT EXISTS "name";`）和 `renderDropSchema`（输出 `DROP SCHEMA IF EXISTS "name";`）。

**技术栈：** Go 1.26, testify (测试)

---

## 文件结构

| 文件 | 角色 | 变更类型 |
|------|------|----------|
| `internal/diff/operation.go` | 新增 `KindCreateSchema` / `KindDropSchema` 常量；新增 `CreateSchemaOp` / `DropSchemaOp` 结构体 + 构造器 | 修改 |
| `internal/diff/operation_test.go` | 新增测试：验证新 operations 满足 Operation 接口，验证 IsDestructive 语义 | 修改 |
| `internal/diff/differ.go` | `diffSchemas()` 中 emit `CreateSchemaOp` 而非仅 warn；`diffNamespace()` 前无需单独 emit CREATE TABLE（namespace 存在后 Table diff 自动工作） | 修改 |
| `internal/diff/differ_test.go` | 新增 `TestDiffer_CreateSchemaOp`：验证 target 有 public + auth，source 只有 public 时，diff 输出 `CreateSchemaOp` | 修改 |
| `internal/render/render.go` | `Render()` switch 添加 `CreateSchemaOp` / `DropSchemaOp`；新增 `renderCreateSchema` / `renderDropSchema` 方法 | 修改 |
| `internal/render/render_test.go` | 新增 `TestRenderCreateSchema` / `TestRenderDropSchema`；`TestRenderNewOperations` 添加两条新 case | 修改 |

---

### 任务 1：新增 Operation 类型（KindCreateSchema / KindDropSchema + 结构体）

**文件：**
- 修改：`internal/diff/operation.go:8-25`（新增常量）
- 修改：`internal/diff/operation.go`（在文件末尾新增结构体）
- 测试：`internal/diff/operation_test.go`

- [ ] **步骤 1.1：在 operation.go 的 const 块中添加两个新 Kind**

在 `KindDropDefault` 之后添加：

```go
	KindCreateSchema Kind = "create_schema"
	KindDropSchema   Kind = "drop_schema"
```

使用 ast_grep_replace：

```
pattern: '	KindDropDefault Kind = "drop_default"'
rewrite: '	KindDropDefault Kind = "drop_default"
	KindCreateSchema Kind = "create_schema"
	KindDropSchema   Kind = "drop_schema"'
lang: go
paths: ["internal/diff/operation.go"]
```

- [ ] **步骤 1.2：在 operation.go 末尾添加 CreateSchemaOp 和 DropSchemaOp 结构体**

```go
// CreateSchemaOp represents creating a new schema (namespace)
type CreateSchemaOp struct {
	baseOperation
	Schema string
}

func NewCreateSchemaOp(schema string) *CreateSchemaOp {
	return &CreateSchemaOp{
		baseOperation: baseOperation{
			kind:      KindCreateSchema,
			objectKey: model.NewObjectKey(schema, "", model.KindSchema),
		},
		Schema: schema,
	}
}

func (op *CreateSchemaOp) IsDestructive() bool {
	return false
}

// DropSchemaOp represents dropping a schema (namespace)
type DropSchemaOp struct {
	baseOperation
	Schema string
}

func NewDropSchemaOp(schema string) *DropSchemaOp {
	return &DropSchemaOp{
		baseOperation: baseOperation{
			kind:      KindDropSchema,
			objectKey: model.NewObjectKey(schema, "", model.KindSchema),
		},
		Schema: schema,
	}
}

func (op *DropSchemaOp) IsDestructive() bool {
	return true
}
```

- [ ] **步骤 1.3：运行编译检查**

运行：`go build ./...`
预期：编译成功，无错误

- [ ] **步骤 1.4：更新 operation_test.go 添加新 operations 的接口测试**

在 `TestOperationInterfaceHasDependsOn` 的 `ops` 切片末尾追加两个元素：

```go
		NewCreateSchemaOp("auth"),
		NewDropSchemaOp("auth"),
```

同时在该测试的 `default` case（无依赖分支）中，确认 len(deps) == 0 或 nil。新 operations 应无依赖（`baseOperation.DependsOn` 返回 nil）。

编辑文件：

```go
// 查找 ops := []Operation{ 行
// 在最后一个元素 NewDropColumnOp("public", "users", "old_col") 后追加：
		NewCreateSchemaOp("auth"),
		NewDropSchemaOp("auth"),
```

- [ ] **步骤 1.5：运行测试验证通过**

运行：`go test ./internal/diff/ -run TestOperationInterfaceHasDependsOn -v`
预期：PASS，新 operations 通过所有断言（DependsOn 返回 nil，无额外断言分支报错）

- [ ] **步骤 1.6：Commit**

```bash
git add internal/diff/operation.go internal/diff/operation_test.go
git commit -m "feat(diff): add CreateSchemaOp and DropSchemaOp operation types"
```

---

### 任务 2：Diff 引擎 emit CreateSchemaOp

**文件：**
- 修改：`internal/diff/differ.go:59-63`
- 测试：`internal/diff/differ_test.go`

- [ ] **步骤 2.1：为 CreateSchemaOp 添加 addOp 辅助方法（或直接使用 addOp）**

`diffContext.addOp()` 已存在且接受 `Operation` 接口。无需新方法。我们直接在 `diffSchemas` 中调用 `c.addOp(NewCreateSchemaOp(name))`。

- [ ] **步骤 2.2：编写失败的测试**

在 `differ_test.go` 末尾添加：

```go
func TestDiffer_CreateSchemaOp(t *testing.T) {
	source := model.NewSchema()
	source.GetOrCreateNamespace("public")
	// public has a table
	sourceNs := source.GetOrCreateNamespace("public")
	sourceNs.Tables["users"] = model.NewTable("public", "users")

	target := model.NewSchema()
	target.GetOrCreateNamespace("public")
	targetNs := target.GetOrCreateNamespace("public")
	targetNs.Tables["users"] = model.NewTable("public", "users")
	// target also has "auth" schema (doesn't exist in source)
	target.GetOrCreateNamespace("auth")
	authNs := target.GetOrCreateNamespace("auth")
	authNs.Tables["roles"] = model.NewTable("auth", "roles")

	ops, warnings := NewDiffer().Diff(source, target)

	// Verify warnings: first is the DROP schema warning for source-only schemas
	// (source has no schemas that target doesn't, so there should be no drop warnings)
	for _, w := range warnings {
		t.Logf("warning: %s", w)
	}

	var foundCreateSchema bool
	for _, op := range ops {
		if op.Kind() == KindCreateSchema {
			foundCreateSchema = true
			createOp, ok := op.(*CreateSchemaOp)
			if !ok {
				t.Fatal("expected *CreateSchemaOp type")
			}
			if createOp.Schema != "auth" {
				t.Fatalf("expected schema 'auth', got %q", createOp.Schema)
			}
			if createOp.IsDestructive() {
				t.Fatal("CreateSchemaOp should not be destructive")
			}
			if createOp.ObjectKey().Kind != model.KindSchema {
				t.Fatalf("expected ObjectKey KindSchema, got %v", createOp.ObjectKey().Kind)
			}
		}
	}
	if !foundCreateSchema {
		t.Fatal("expected CreateSchemaOp in diff output, but none found")
	}

	// Verify that tables inside the new schema are also emitted
	var foundAddAuthRoles bool
	for _, op := range ops {
		if op.Kind() == KindAddTable {
			addTable, ok := op.(*AddTableOp)
			if ok && addTable.Schema == "auth" && addTable.Table.Name == "roles" {
				foundAddAuthRoles = true
			}
		}
	}
	if !foundAddAuthRoles {
		t.Fatal("expected AddTableOp for auth.roles")
	}
}
```

- [ ] **步骤 2.3：运行测试验证失败**

运行：`go test ./internal/diff/ -run TestDiffer_CreateSchemaOp -v`
预期：FAIL — diffSchemas() 未 emit CreateSchemaOp

- [ ] **步骤 2.4：修改 diffSchemas() emit CreateSchemaOp**

将 `differ.go:59-63` 中：

```go
	for _, name := range sourceNames {
		if _, exists := target.Schemas[name]; !exists {
			c.warnf("namespace drop is not implemented yet (ignored): %s", name)
		}
	}
```

改为：在 `else` 分支（target namespace 不存在于 source 时）emit `CreateSchemaOp`：

```go
		} else {
			// Entire namespace needs to be created
			c.addOp(NewCreateSchemaOp(name))
			c.diffNamespace(nil, targetNs)
		}
```

修改后 `diffSchemas` 的完整 `for _, name := range targetNames` 循环变为：

```go
	for _, name := range targetNames {
		targetNs := target.Schemas[name]
		if sourceNs, exists := source.Schemas[name]; exists {
			c.diffNamespace(sourceNs, targetNs)
		} else {
			// Entire namespace needs to be created
			c.addOp(NewCreateSchemaOp(name))
			c.diffNamespace(nil, targetNs)
		}
	}
```

注意：`diffNamespace(nil, targetNs)` 会触发 source==nil 分支，emit 所有表/类型 — 这正是我们想要的。

- [ ] **步骤 2.5：运行测试验证通过**

运行：`go test ./internal/diff/ -run TestDiffer_CreateSchemaOp -v`
预期：PASS，输出包含 `foundCreateSchema = true`

- [ ] **步骤 2.6：运行全部 diff 测试确保不退化**

运行：`go test ./internal/diff/ -v`
预期：ALL PASS（注意：其他测试可能在 Diff 中创建新的 schema，但 `TestDiffer_*` 共用同一个 `public`，所以不应受影响。`TestDiffer_DiffNoSharedState` 使用空 source 和仅 public target，现在会 emit 一个 `CreateSchemaOp` 了 — 这个测试的断言是 `len(ops) != 0` 所以不会失败）

- [ ] **步骤 2.7：Commit**

```bash
git add internal/diff/differ.go internal/diff/differ_test.go
git commit -m "feat(diff): emit CreateSchemaOp for new schemas in target"
```

---

### 任务 3：渲染器支持 CREATE SCHEMA / DROP SCHEMA

**文件：**
- 修改：`internal/render/render.go:69-131`（Render switch + 新方法）
- 测试：`internal/render/render_test.go`

- [ ] **步骤 3.1：编写测试**

在 `render_test.go` 末尾添加：

```go
func TestRenderCreateSchema(t *testing.T) {
	r := NewRenderer()

	t.Run("CreateSchemaOp", func(t *testing.T) {
		op := diff.NewCreateSchemaOp("auth")
		sql := r.Render(op)
		want := "-- op: create_schema risk:low\nCREATE SCHEMA IF NOT EXISTS \"auth\";"
		if sql != want {
			t.Errorf("got %q, want %q", sql, want)
		}
	})

	t.Run("DropSchemaOp", func(t *testing.T) {
		op := diff.NewDropSchemaOp("old_schema")
		sql := r.Render(op)
		want := "-- op: drop_schema risk:high\nDROP SCHEMA IF EXISTS \"old_schema\";"
		if sql != want {
			t.Errorf("got %q, want %q", sql, want)
		}
	})
}
```

同时将 `CreateSchemaOp` 和 `DropSchemaOp` 加入到 `TestRenderNewOperations` 的 `cases` map：

在 `TestRenderNewOperations` 的 cases map 末尾添加：

```go
		`CREATE SCHEMA IF NOT EXISTS "auth";`:		diff.NewCreateSchemaOp("auth"),
		`DROP SCHEMA IF EXISTS "old_schema";`:		diff.NewDropSchemaOp("old_schema"),
```

- [ ] **步骤 3.2：运行测试验证失败**

运行：`go test ./internal/render/ -run TestRenderCreateSchema -v`
预期：FAIL — Render switch 不认识 CreateSchemaOp/DropSchemaOp，走 default 分支输出 `"-- Unknown operation: *diff.CreateSchemaOp"`

- [ ] **步骤 3.3：在 Render switch 中添加 case**

在 `render.go` 的 `Render()` 函数（`case *diff.DropColumnOp:` 之后、`default:` 之前）添加：

```go
	case *diff.CreateSchemaOp:
		return r.renderCreateSchema(v)
	case *diff.DropSchemaOp:
		return r.renderDropSchema(v)
```

- [ ] **步骤 3.4：添加 renderCreateSchema 和 renderDropSchema 方法**

在 `renderDropEnumType` 方法之后（或 `renderDropColumn` 之前）添加：

```go
func (r *Renderer) renderCreateSchema(op *diff.CreateSchemaOp) string {
	return fmt.Sprintf("-- op: create_schema risk:low\nCREATE SCHEMA IF NOT EXISTS %s;",
		quoteIdentifier(op.Schema))
}

func (r *Renderer) renderDropSchema(op *diff.DropSchemaOp) string {
	return fmt.Sprintf("-- op: drop_schema risk:high\nDROP SCHEMA IF EXISTS %s;",
		quoteIdentifier(op.Schema))
}
```

- [ ] **步骤 3.5：运行测试验证通过**

运行：`go test ./internal/render/ -run "TestRenderCreateSchema|TestRenderNewOperations" -v`
预期：PASS，所有 case 输出正确的 SQL

- [ ] **步骤 3.6：运行全部渲染测试确保不退化**

运行：`go test ./internal/render/ -v`
预期：ALL PASS

- [ ] **步骤 3.7：全量编译检查**

运行：`go build ./...`
预期：编译成功

- [ ] **步骤 3.8：全量测试**

运行：`go test ./...`
预期：ALL PASS

- [ ] **步骤 3.9：Commit**

```bash
git add internal/render/render.go internal/render/render_test.go
git commit -m "feat(render): add CREATE SCHEMA and DROP SCHEMA SQL rendering"
```

---

## 完成后的集成行为

执行计划后的完整场景：

1. **源**仅有 `public` schema，**目标**有 `public` + `auth` schemas，且 `auth` 下有表：
   - diff 输出：`CreateSchemaOp("auth")`、`AddTableOp("auth", "roles")`、列 ops、索引 ops 等
   - render 输出：`CREATE SCHEMA IF NOT EXISTS "auth";` + `CREATE TABLE "auth"."roles" (...);` + ...
2. **DROP SCHEMA**：当前 diff 引擎仍只给 warning，不 emit `DropSchemaOp`。`DropSchemaOp` 类型和渲染端已就绪，将来在 differ 中启用即可。
3. CreateSchemaOp 为非破坏性（CREATE SCHEMA IF NOT EXISTS 是幂等的）。
4. DropSchemaOp 为破坏性（`IsDestructive() = true`），遵循 DROP 语义的已有模式（如 `DropTableOp`、`DropEnumTypeOp`）。
