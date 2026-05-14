# migra-go 优化实现计划

> **For Claude:** REQUIRED SUB-SKILL: Use superpowers:executing-plans to implement this plan task-by-task.

**Goal:** 根据技术评审报告，完成剩余 5 个问题的优化：错误上下文改进（问题 5、6）、接口命名规范（问题 7）、架构解耦（问题 11、12）

**Architecture:** 分三个阶段实施：1) 错误包装改进（parser/introspect），2) 接口重命名重构，3) 架构解耦（source loader + operation 依赖声明）。每个阶段独立可验证。

**Tech Stack:** Go 1.21+, standard library, `errors` package, interface design

---

### Task 1: 改进解析错误上下文（问题 5）

**Files:**
- Modify: `internal/parser/parser.go:69-71`
- Test: `internal/parser/parser_test.go`

**Step 1: 编写失败测试 - 验证错误包含首个错误详情**

```go
func TestParseSQLReturnsFirstErrorDetail(t *testing.T) {
    p := NewParser()
    // 使用能触发 visitNode 错误的 SQL，而非 pg_query 语法错误
    _, err := p.ParseSQL("SELECT * FROM users;")
    if err == nil {
        t.Fatal("expected error")
    }
    // 错误应包含 "parsing completed with" 和首个错误详情
    errMsg := err.Error()
    if !strings.Contains(errMsg, "parsing completed with") {
        t.Errorf("expected error to contain 'parsing completed with', got: %s", errMsg)
    }
    // 验证错误包装了首个错误（可通过 Unwrap 提取）
    unwrapped := errors.Unwrap(err)
    if unwrapped == nil {
        t.Errorf("expected error to wrap the first error via Unwrap")
    }
}
```

**Note:** `errors.As(err, &firstErr)` 其中 `firstErr` 类型为 `error` 是无效用法（不能使用 `*error` 作为第二个参数）。使用 `errors.Unwrap` 验证错误包装。

**Step 3: 修改 ParseSQL 返回首个错误详情**

修改 `internal/parser/parser.go:69-71`:

```go
// 原代码:
// return p.schema, fmt.Errorf("parsing completed with %d errors", len(p.errors))

// 新代码:
if len(p.errors) > 0 {
    first := p.errors[0]
    return p.schema, fmt.Errorf("parsing completed with %d errors, first: %w", len(p.errors), first)
}
return p.schema, nil
```

**Step 4: 运行测试验证通过**

Run: `go test ./internal/parser/ -run TestParseSQLReturnsFirstErrorDetail -v`
Expected: PASS

**Step 5: 运行全量测试确保无回归**

Run: `go test ./...`
Expected: All PASS

**Step 6: Commit**

```bash
git add internal/parser/parser.go internal/parser/parser_test.go
git commit -m "fix: 改进解析错误上下文，包装首个错误详情（问题5）"
```

---

### Task 2: 改进枚举加载错误上下文（问题 6）

**Files:**
- Modify: `internal/introspect/enums.go:53-55`
- Test: `internal/introspect/enums_test.go` (如不存在则创建)

**Step 1: 编写失败测试 - 验证错误包含场景上下文**

```go
func TestLoadEnumTypesWrapsRowsError(t *testing.T) {
    // 使用 mock 或错误注入验证 rows.Err() 被包装
    // 由于 introspect 依赖数据库，这里测试错误格式
    // 实际验证可通过检查错误信息是否包含上下文
    t.Skip("integration test requires database, verifying code directly")
}
```

**Step 2: 修改 loadEnumTypes 包装 rows.Err()**

修改 `internal/introspect/enums.go:53-55`:

```go
// 原代码:
// if err := rows.Err(); err != nil {
//     return err
// }

// 新代码:
if err := rows.Err(); err != nil {
    return fmt.Errorf("iterate enum rows: %w", err)
}
```

检查同目录其他文件（如 `tables.go`, `columns.go`）是否也需要类似改进，如有则一并修改。

**Step 3: 运行测试验证**

Run: `go test ./internal/introspect/... -v`
Expected: PASS (现有测试通过，新代码无语法错误)

**Step 4: Commit**

```bash
git add internal/introspect/enums.go
git commit -m "fix: 为枚举加载错误添加场景上下文包装（问题6）"
```

---

### Task 3: 重命名 internal/app 的 Service 接口（问题 7 - 第一部分）

**Files:**
- Modify: `internal/app/diff_service.go:27` (Service → DiffService)
- Modify: 所有引用 Service 的地方（实现类、调用方、测试）

**Step 1: 使用工具或手动重命名 Service → DiffService**

在 `internal/app/diff_service.go` 中:

```go
// 原代码:
// type Service interface { ... }

// 新代码:
type DiffService interface {
    ComputeDiff(ctx context.Context, source, target *model.Schema, opts ComputeDiffOptions) (*DiffResult, error)
    ValidateSchema(ctx context.Context, schema *model.Schema) error
}
```

**Step 2: 更新所有引用点**

搜索并替换整个项目中 `app.Service` 的使用:

```bash
# 查找引用
grep -r "app\.Service\|var.*Service\|.*Service)" internal/ --include="*.go"
```

更新:
- `internal/app/diff_service.go` 中的实现类（如 `diffService` 结构体可能不需要改名，但接口实现声明需要更新）
- 测试文件中的 mock 或引用

**Step 3: 编译验证**

Run: `go build ./...`
Expected: 编译成功

**Step 4: 运行测试验证**

Run: `go test ./internal/app/... -v`
Expected: PASS

**Step 5: Commit**

```bash
git add internal/app/diff_service.go internal/app/diff_service_test.go
git commit -m "refactor: 重命名 app.Service 为 app.DiffService（问题7-1）"
```

---

### Task 4: 重命名 internal/diff 的 Engine 接口（问题 7 - 第二部分）

**Files:**
- Modify: `internal/diff/differ.go:9` (Engine → DiffEngine)
- Modify: 所有引用 Engine 的地方

**Step 1: 重命名 Engine → DiffEngine**

在 `internal/diff/differ.go` 中:

```go
// 原代码:
// type Engine interface { ... }

// 新代码:
type DiffEngine interface {
    Diff(source, target *model.Schema) ([]Operation, []string)
}
```

更新编译时检查:
```go
// 原代码:
// var _ Engine = (*Differ)(nil)

// 新代码:
var _ DiffEngine = (*Differ)(nil)
```

**Step 2: 更新所有引用点**

搜索并替换:
```bash
grep -r "diff\.Engine\|var.*Engine" internal/ --include="*.go"
```

更新:
- `internal/diff/differ_test.go`
- `internal/app/diff_service.go` 中可能引用了 `diff.Engine`

**Step 3: 编译验证**

Run: `go build ./...`
Expected: 编译成功

**Step 4: 运行测试验证**

Run: `go test ./internal/diff/... ./internal/app/... -v`
Expected: PASS

**Step 5: Commit**

```bash
git add internal/diff/differ.go internal/diff/differ_test.go internal/app/diff_service.go
git commit -m "refactor: 重命名 diff.Engine 为 diff.DiffEngine（问题7-2）"
```

---

### Task 5: 重命名 internal/plan 的 Engine 接口（问题 7 - 第三部分）

**Files:**
- Modify: `internal/plan/plan.go:23` (Engine → PlanEngine)
- Modify: 所有引用 Engine 的地方

**Step 1: 重命名 Engine → PlanEngine**

在 `internal/plan/plan.go` 中:

```go
// 原代码:
// type Engine interface { ... }

// 新代码:
type PlanEngine interface {
    GeneratePlan(ctx context.Context, schema *model.Schema, ops []diff.Operation) (*model.Schema, []string, error)
}
```

**Step 2: 更新所有引用点**

搜索并替换:
```bash
grep -r "plan\.Engine\|var.*Engine" internal/ --include="*.go"
```

更新:
- `internal/plan/plan_test.go`
- 其他可能引用 `plan.Engine` 的地方

**Step 3: 编译验证**

Run: `go build ./...`
Expected: 编译成功

**Step 4: 运行测试验证**

Run: `go test ./internal/plan/... -v`
Expected: PASS

**Step 5: Commit**

```bash
git add internal/plan/plan.go internal/plan/plan_test.go
git commit -m "refactor: 重命名 plan.Engine 为 plan.PlanEngine（问题7-3）"
```

---

### Task 6: 创建 internal/source 包和 Loader 接口（问题 11 - 第一部分）

**Files:**
- Create: `internal/source/loader.go`
- Create: `internal/source/registry.go`
- Create: `internal/source/db_loader.go`
- Create: `internal/source/sql_file_loader.go`
- Create: `internal/source/loader_test.go`

**Step 1: 编写测试 - DBLoader 匹配和加载**

```go
// internal/source/loader_test.go
package source

import (
    "context"
    "testing"
    "github.com/fred29910/migra-go/internal/model"
)

func TestDBLoader_Match(t *testing.T) {
    loader := &DBLoader{}
    tests := []struct {
        source  string
        expected bool
    }{
        {"postgres://user:pass@localhost/db", true},
        {"mysql://user:pass@localhost/db", true},
        {"file:///path/to/file.sql", false},
        {"/path/to/file.sql", false},
    }
    for _, tt := range tests {
        got := loader.Match(tt.source)
        if got != tt.expected {
            t.Errorf("DBLoader.Match(%q) = %v, want %v", tt.source, got, tt.expected)
        }
    }
}

func TestSQLFileLoader_Match(t *testing.T) {
    loader := &SQLFileLoader{}
    tests := []struct {
        source  string
        expected bool
    }{
        {"file:///path/to/file.sql", true},
        {"/path/to/file.sql", true},
        {"postgres://user:pass@localhost/db", false},
    }
    for _, tt := range tests {
        got := loader.Match(tt.source)
        if got != tt.expected {
            t.Errorf("SQLFileLoader.Match(%q) = %v, want %v", tt.source, got, tt.expected)
        }
    }
}
```

**Step 2: 运行测试验证失败**

Run: `go test ./internal/source/ -run TestDBLoader_Match -v`
Expected: FAIL (文件不存在)

**Step 3: 创建 Loader 接口和 Registry**

创建 `internal/source/loader.go`:

```go
package source

import (
    "context"
    "github.com/fred29910/migra-go/internal/model"
)

type LoadOptions struct {
    // 可扩展的加载选项
}

type Loader interface {
    Match(source string) bool
    Load(ctx context.Context, source string, opt LoadOptions) (*model.Schema, []error, error)
}
```

创建 `internal/source/registry.go`:

```go
package source

import "context"

type Registry struct {
    loaders []Loader
}

func NewRegistry() *Registry {
    return &Registry{
        loaders: make([]Loader, 0),
    }
}

func (r *Registry) Register(loader Loader) {
    r.loaders = append(r.loaders, loader)
}

func (r *Registry) Load(ctx context.Context, source string, opt LoadOptions) (*model.Schema, []error, error) {
    for _, loader := range r.loaders {
        if loader.Match(source) {
            return loader.Load(ctx, source, opt)
        }
    }
    return nil, nil, fmt.Errorf("no loader found for source: %s", source)
}
```

**Step 4: 创建 DBLoader 和 SQLFileLoader 骨架**

创建 `internal/source/db_loader.go`:

```go
package source

import (
    "context"
    "strings"
)

type DBLoader struct{}

func (l *DBLoader) Match(source string) bool {
    return strings.HasPrefix(source, "postgres://") || strings.HasPrefix(source, "mysql://")
}

func (l *DBLoader) Load(ctx context.Context, source string, opt LoadOptions) (*model.Schema, []error, error) {
    // TODO: 从 cmd/migra/diff.go 的 loadFromDB 迁移逻辑
    return nil, nil, nil
}
```

创建 `internal/source/sql_file_loader.go`:

```go
package source

import (
    "context"
    "strings"
)

type SQLFileLoader struct{}

func (l *SQLFileLoader) Match(source string) bool {
    return strings.HasPrefix(source, "file://") || strings.HasSuffix(source, ".sql")
}

func (l *SQLFileLoader) Load(ctx context.Context, source string, opt LoadOptions) (*model.Schema, []error, error) {
    // TODO: 从 cmd/migra/diff.go 的 loadFromSQLFile 迁移逻辑
    return nil, nil, nil
}
```

**Step 5: 运行测试验证通过**

Run: `go test ./internal/source/ -v`
Expected: PASS

**Step 6: Commit**

```bash
git add internal/source/
git commit -m "feat: 创建 source 包和 Loader 接口（问题11-1）"
```

---

### Task 7: 迁移 DB 和 SQL File 加载逻辑到 source 包（问题 11 - 第二部分）

**Files:**
- Modify: `internal/source/db_loader.go` (迁移 loadFromDB 逻辑)
- Modify: `internal/source/sql_file_loader.go` (迁移 loadFromSQLFile 逻辑)
- Modify: `cmd/migra/diff.go` (使用 source.Registry)
- Delete: 可选删除 cmd 中的 loadFromDB、loadFromSQLFile（或标记为 deprecated）

**Step 1: 从 cmd/migra/diff.go 迁移 loadFromDB 到 internal/source/db_loader.go**

阅读 `cmd/migra/diff.go:57-99` 的 `loadFromDB` 函数，将其核心逻辑迁移到 `DBLoader.Load` 方法。

**Step 2: 从 cmd/migra/diff.go 迁移 loadFromSQLFile 到 internal/source/sql_file_loader.go**

阅读 `cmd/migra/diff.go` 的 `loadFromSQLFile` 函数，将其核心逻辑迁移到 `SQLFileLoader.Load` 方法。

**Step 3: 修改 cmd/migra/diff.go 使用 source.Registry**

```go
// 在 cmd/migra/diff.go 的合适位置（如 init 或 main）:
func setupSourceRegistry() *source.Registry {
    reg := source.NewRegistry()
    reg.Register(&source.DBLoader{})
    reg.Register(&source.SQLFileLoader{})
    return reg
}

// 修改或替换 loadSchemaWithContext 函数:
func loadSchemaWithContext(ctx context.Context, sourceStr string, opts LoadOptions, reg *source.Registry) (*model.Schema, error) {
    schema, errs, err := reg.Load(ctx, sourceStr, opts)
    if err != nil {
        return nil, err
    }
    if len(errs) > 0 {
        // 处理解析错误
    }
    return schema, nil
}
```

**Step 4: 运行测试验证**

Run: `go build ./... && go test ./...`
Expected: 编译成功，测试通过

**Step 5: Commit**

```bash
git add internal/source/db_loader.go internal/source/sql_file_loader.go cmd/migra/diff.go
git commit -m "refactor: 迁移加载逻辑到 source 包，解耦 cmd 层（问题11-2）"
```

---

### Task 8: 为 Operation 添加 DependsOn 方法（问题 12 - 第一部分）

**Files:**
- Modify: `internal/diff/operations.go` 或新建 `internal/diff/operation_interface.go`
- Modify: 所有 Operation 实现（如 AddColumnOp、CreateTableOp 等）

**Step 1: 编写测试 - 验证 Operation 接口包含 DependsOn**

```go
// internal/diff/operation_test.go
package diff

import (
    "testing"
    "github.com/fred29910/migra-go/internal/model"
)

func TestOperationInterfaceHasDependsOn(t *testing.T) {
    // 验证 Operation 接口（或新接口）包含 DependsOn 方法
    // 由于 Go 是结构类型，我们检查具体实现
    ops := []Operation{
        &AddColumnOp{},
        &CreateTableOp{},
        // 添加其他 Operation 类型
    }
    for _, op := range ops {
        deps := op.DependsOn() // 编译时检查此方法存在
        _ = deps
    }
}
```

**Step 2: 定义包含 DependsOn 的接口**

在 `internal/diff/operations.go` 或新文件中:

```go
// Operation 定义变更操作的基础接口
type Operation interface {
    Kind() Kind
    ObjectKey() model.ObjectKey
    DependsOn() []model.ObjectKey
    IsDestructive() bool
}
```

**Step 3: 为所有 Operation 实现 DependsOn 方法**

为每个 Operation 添加 `DependsOn()` 方法，返回其依赖的对象键列表。例如:

```go
// AddColumnOp 依赖对应的表已存在
func (op *AddColumnOp) DependsOn() []model.ObjectKey {
    return []model.ObjectKey{
        {Kind: KindAddTable, Name: op.Table}, // 假设表需要先创建
    }
}

// CreateTableOp 通常无依赖
func (op *CreateTableOp) DependsOn() []model.ObjectKey {
    return nil
}

// CreateIndexOp 依赖表和列
func (op *CreateIndexOp) DependsOn() []model.ObjectKey {
    return []model.ObjectKey{
        {Kind: KindAddTable, Name: op.Table},
    }
}
```

具体依赖关系需要根据每个 Operation 的实际逻辑确定。

**Step 4: 运行测试验证**

Run: `go test ./internal/diff/... -v`
Expected: PASS

**Step 5: Commit**

```bash
git add internal/diff/operations.go internal/diff/*_op.go
git commit -m "feat: 为 Operation 添加 DependsOn 方法（问题12-1）"
```

---

### Task 9: 修改 plan.addDependencies 使用 DependsOn（问题 12 - 第二部分）

**Files:**
- Modify: `internal/plan/dag.go:77-119` (addDependencies 函数)

**Step 1: 编写测试 - 验证 addDependencies 不再使用 switch**

```go
// internal/plan/dag_test.go
func TestAddDependenciesUsesDependsOn(t *testing.T) {
    // 创建测试 Operation，验证 plan 使用 DependsOn 而非 switch
    // 这个测试主要是确保现有测试通过，证明重构未破坏功能
}
```

**Step 2: 修改 addDependencies 函数**

修改 `internal/plan/dag.go` 的 `addDependencies`:

```go
// 原代码: switch op := node.Op.(type) { case *AddColumnOp: ... }

// 新代码: 使用 op.DependsOn()
func (d *DAG) addDependencies(node *Node) {
    for _, depKey := range node.Op.DependsOn() {
        // 根据 depKey 查找依赖节点
        if depNode := d.findNodeByObjectKey(depKey); depNode != nil {
            d.AddDependency(node, depNode)
        }
    }
}

// 需要添加辅助方法 findNodeByObjectKey
func (d *DAG) findNodeByObjectKey(key model.ObjectKey) *Node {
    for _, node := range d.nodes {
        if node.Op.ObjectKey() == key {
            return node
        }
    }
    return nil
}
```

**Step 3: 删除或简化原有 switch 逻辑**

确认所有 Operation 都已实现 `DependsOn()` 后，删除 `addDependencies` 中的 switch 分支。

**Step 4: 运行测试验证**

Run: `go test ./internal/plan/... -v`
Expected: PASS (所有现有测试通过，证明重构正确)

**Step 5: 检查圈复杂度**

Run: `gocyclo -over 8 internal/plan/dag.go`
Expected: `addDependencies` 复杂度从 13 降低到合理范围（< 10）

**Step 6: Commit**

```bash
git add internal/plan/dag.go internal/plan/dag_test.go
git commit -m "refactor: 使用 Operation.DependsOn 替代 switch 分支（问题12-2）"
```

---

### Task 10: 最终验证和清理

**Step 1: 全量编译检查**

Run: `go build ./...`
Expected: 编译成功

**Step 2: 全量测试**

Run: `go test ./...`
Expected: All PASS

**Step 3: 代码质量检查**

Run: `go vet ./...`
Expected: No issues

**Step 4: 圈复杂度检查**

Run: `gocyclo -over 8 internal/...`
Expected: 无超过 8 的函数（或至少 `addDependencies` 已降低）

**Step 5: 确认所有问题已解决**

对照设计文档验证:
- ✅ 问题 5: 解析错误包含首个错误详情
- ✅ 问题 6: 枚举加载错误包含场景上下文
- ✅ 问题 7: 接口已按职责重命名
- ✅ 问题 11: cmd 层不再包含 source-adapter 逻辑
- ✅ 问题 12: plan.addDependencies 无 switch 分支

**Step 6: Final Commit (如有遗漏修复)**

```bash
git add -A
git commit -m "chore: 优化计划最终验证和清理"
```
