# P2 阶段：可扩展性提升实现计划

> 分支：`optimize/interface-and-pipeline`
> 工作目录：`/opt/codes/workspace/migra-go`
> 前置条件：P0 + P1 阶段已完成，785 tests passing

---

## 任务 P2-1：Model 深拷贝 Clone 方法

**文件：**
- 新增：`internal/model/clone.go`
- 无其他文件修改（方法定义在类型所在文件或统一文件均可）

### 步骤 1：定义 Clone 方法清单

为每个类型添加 `Clone()` 方法，纯值类型（View, Sequence, Extension, IndexElem）使用简单结构体拷贝，复杂类型逐字段深拷贝。

```go
// Schema.Clone() *Schema
// Namespace.Clone() *Namespace  
// Table.Clone() *Table
// Column.Clone() *Column
// PrimaryKey.Clone() *PrimaryKey
// Index.Clone() *Index
// Constraint.Clone() *Constraint
// EnumType.Clone() *EnumType
// View.Clone() *View
// Sequence.Clone() *Sequence
// Extension.Clone() *Extension
// IndexElem.Clone() IndexElem // 值类型接收者，返回值
```

### 步骤 2：实现顺序（从叶子到根）

**叶子类型（无引用）：** View, Sequence, Extension, IndexElem, PrimaryKey, EnumType
→ 纯值结构体，`*clone = *orig` 或类 JSON 语义拷贝

**中级类型（引用叶子）：**
- `Column.Clone()`：克隆 `*string DefaultExpr`
- `Constraint.Clone()`：复制 `[]string Columns` + `[]string RefColumns`
- `Index.Clone()`：复制 `[]string Columns` + `[]IndexElem Elements`

**高级类型（引用中级 + 叶子）：**
- `Table.Clone()`：
  1. 克隆所有 `*Column` → 新 `[]*Column`
  2. 重建 `ColumnByName` map（指向克隆的 Column）
  3. 重建 `ColumnIndex` map（name → position in new slice）
  4. 克隆 `*PrimaryKey`
  5. 克隆 `map[string]*Constraint`
  6. 克隆 `map[string]*Index`

**根类型：**
- `Namespace.Clone()`：克隆 5 个 map（Tables, Types, Views, Sequences, Extensions）
- `Schema.Clone()`：克隆 `map[string]*Namespace`

### 步骤 3：测试

- `go test ./internal/model/... -count=1`
- 核心验证：克隆后修改原始对象，克隆对象不受影响
- 应覆盖：
  - 空字段
  - nil 指针字段
  - 空 map/slice
  - 嵌套深度

### 验证

```bash
go build ./...
go test ./internal/model/... -count=1
go test ./... -count=1
```

---

## 任务 P2-2：Loader 优先级排序

**文件：**
- 修改：`internal/source/loader.go`（Loader 接口 + 优先级常量）
- 修改：`internal/source/registry.go`（排序逻辑）
- 修改：`internal/source/db_loader.go`（Priority 方法）
- 修改：`internal/source/dir_loader.go`（Priority 方法 + 移除防御性 DB 检查）
- 修改：`internal/source/sql_file_loader.go`（Priority 方法）
- 修改：`internal/source/registry_test.go`（优先级排序测试）

### 步骤 1：扩展 Loader 接口

```go
// internal/source/loader.go

// LoaderPriority defines the priority levels for loader ordering.
// Higher priority loaders are matched first.
type LoaderPriority int

const (
    LoaderPriorityLowest  LoaderPriority = 10
    LoaderPriorityDefault LoaderPriority = 50
    LoaderPriorityHighest LoaderPriority = 100
)

type Loader interface {
    Match(source string) bool
    Load(ctx context.Context, source string, opt LoadOptions) (*model.Schema, []error, error)
    Priority() LoaderPriority
}
```

### 步骤 2：为每个 Loader 实现 Priority()

```go
func (l *DBLoader) Priority() LoaderPriority       { return LoaderPriorityHighest } // 100
func (l *DirectoryLoader) Priority() LoaderPriority { return LoaderPriorityDefault } // 50
func (l *SQLFileLoader) Priority() LoaderPriority   { return LoaderPriorityLowest }  // 10
```

### 步骤 3：Registry 注册后排序

```go
// internal/source/registry.go

func (r *Registry) Register(loader Loader) {
    r.loaders = append(r.loaders, loader)
    sort.SliceStable(r.loaders, func(i, j int) bool {
        return r.loaders[i].Priority() > r.loaders[j].Priority() // descending
    })
}
```

添加 `sort` 到 import 中。注意排序在 `Register` 内部自动完成，调用方无需手动排序。

### 步骤 4：移除 DirectoryLoader 的防御性检查

`dir_loader.go` 中现有的 DB URL 前缀检查（line ~26）可以移除，因为优先级排序保证了 `DBLoader` 总是在 `DirectoryLoader` 之前匹配。

### 步骤 5：更新测试

- `registry_test.go`：增加优先级排序测试，验证高优先级 Loader 在低优先级之前匹配
- 确保现有 first-match 测试仍然有效（优先级顺序应该与旧注册顺序一致）

### 验证

```bash
go build ./...
go test ./internal/source/... -count=1
```

---

## 任务 P2-3：Pipeline Hook 机制

**文件：**
- 修改：`internal/app/diff_service.go`（扩展 RunnerDeps + 调用 hooks）
- 修改：`internal/app/pipeline.go`（不直接改，但 hooks 的调用点在此修改 diff_service.go）
- 修改：`cmd/migra/diff_runner.go`（newDefaultDeps 提供默认 no-op hooks）
- 修改：`internal/app/diff_service_test.go`（测试 hook 调用）

### 步骤 1：定义 Hook 类型

```go
// internal/app/diff_service.go

// HookStage identifies which pipeline stage a hook attaches to.
type HookStage string

const (
    HookBeforeLoad   HookStage = "before_load"
    HookAfterLoad    HookStage = "after_load"
    HookBeforeDiff   HookStage = "before_diff"
    HookAfterDiff    HookStage = "after_diff"
    HookBeforePlan   HookStage = "before_plan"
    HookAfterPlan    HookStage = "after_plan"
    HookBeforeRender HookStage = "before_render"
    HookAfterRender  HookStage = "after_render"
)

// HookContext carries stage-specific data to hook functions.
type HookContext struct {
    Stage        HookStage
    SourceSchema *model.Schema
    TargetSchema *model.Schema
    Operations   []diff.Operation
    Output       string
    Error        error
}

// HookFunc is a callback invoked at pipeline stages.
// Return error to abort the pipeline.
type HookFunc func(ctx context.Context, hctx HookContext) error
```

### 步骤 2：扩展 RunnerDeps

```go
type RunnerDeps struct {
    LoadSchema func(ctx context.Context, source string, schemas []string, strict bool) (*model.Schema, error)
    Compute    func(ctx context.Context, source, target *model.Schema, cfg DiffConfig) ([]diff.Operation, []string, error)
    Render     func(ctx context.Context, ops []diff.Operation, format string) (string, error)

    // Hooks — optional callbacks at pipeline stage boundaries.
    // Default no-op if nil.
    Hooks []HookFunc
}
```

### 步骤 3：在 DiffService.Run 中注入 hook 调用

```go
func (s *diffService) Run(parent context.Context, cfg DiffConfig) (string, []string, error) {
    ctx, cancel := context.WithTimeout(parent, cfg.Timeout)
    defer cancel()

    // Hook: before_load
    s.runHooks(ctx, HookBeforeLoad, HookContext{Stage: HookBeforeLoad})

    sourceSchema, err := s.deps.LoadSchema(ctx, cfg.Source, cfg.Schemas, cfg.Strict)
    if err != nil {
        return "", nil, fmt.Errorf("load source: %w, %w", errors.ErrLoadFailed, err)
    }

    // ...
}
```

`runHooks` 辅助方法：
```go
func (s *diffService) runHooks(ctx context.Context, stage HookStage, hctx HookContext) error {
    hctx.Stage = stage
    for _, hook := range s.deps.Hooks {
        if err := hook(ctx, hctx); err != nil {
            return err
        }
    }
    return nil
}
```

### 步骤 4：为每个关键阶段插入 hook 调用

在 `diffService.Run()` 中：
- `HookBeforeLoad` → 在第一个 `LoadSchema` 之前
- `HookAfterLoad` → 在目标 `LoadSchema` 之后（两个 schema 均已加载）
- `HookBeforeDiff` → 在 `Compute` 调用之前
- `HookAfterDiff` → 在 `Compute` 返回之后（含 ops + warnings）
- `HookBeforeRender` → 在 `Render` 调用之前
- `HookAfterRender` → 在 `Render` 返回之后

### 步骤 5：默认 no-op hooks

```go
// cmd/migra/diff_runner.go
func newDefaultDeps() app.RunnerDeps {
    return app.RunnerDeps{
        // ... existing LoadSchema, Compute, Render ...
        Hooks: nil, // no hooks in production
    }
}
```

### 步骤 6：更新测试

- `diff_service_test.go`：添加 `TestDiffService_Hooks`，验证 hook 在正确的阶段被调用，hook error 能终止管道
- `fakeDeps` 可保留 `Hooks: nil`

### 验证

```bash
go build ./...
go test ./internal/app/... -count=1
go test ./cmd/migra/... -count=1
```

---

## 验证清单

每个任务完成后：

- [ ] `go build ./...` 编译通过
- [ ] 相关包测试全部通过
- [ ] `go test ./... -count=1` 全部通过
- [ ] LSP diagnostics 无错误
