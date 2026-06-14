# migra-go 代码质量评审报告

> 评审日期：2026-06-14
> 分支：develop
> 评审范围：全量代码（79 源文件 + 47 测试文件，~17,700 行）

---

## 📊 项目概览

| 指标 | 数值 |
|------|------|
| 源文件 (非测试) | 79 个 |
| 测试文件 | 47 个 |
| 总代码行数 | ~17,700 行 |
| Go 版本 | 1.26.2 |
| 测试通过 | ✅ 全部通过 |
| go vet | ✅ 无警告 |
| 整体覆盖率 | ~85%+ |

### 各包覆盖率

| 包 | 覆盖率 |
|----|--------|
| model | 100.0% |
| version | 100.0% |
| render | 98.5% |
| parser | 93.9% |
| plan | 92.1% |
| diff | 89.3% |
| introspect | 89.0% |
| parserutil | 87.4% |
| normalize | 83.3% |
| indexdef | 81.6% |
| source | 77.9% |
| app | 73.1% |

---

## ✅ 优点

### 1. 架构设计清晰

- 分层合理：`parser` → `model` → `diff` → `render`，职责分明
- `Mutation` 模式设计优雅：每种 DDL 变更都是自描述、自应用的 `SchemaMutation`，新增 DDL 类型只需添加 Handler + Mutation + 注册
- `HandlerRegistry` 实现了解耦的路由机制
- `source.Loader` 接口 + `Registry` 模式支持多数据源（DB / SQL 文件 / 目录）

### 2. 测试覆盖出色

- 47 个测试文件，覆盖率高
- 测试风格统一，使用 table-driven tests
- 有 golden test、benchmark test、integration test

### 3. 规范化（Normalize）做得好

- `normalize` 包处理了大小写、引号、类型转换、默认值表达式等差异
- 有效减少了 false diff

### 4. 错误处理有层次

- 定义了 `ParseError` 带位置信息
- 使用 `errors.Is` 兼容的 sentinel errors
- `Mutation.Apply` 返回具体错误而非 panic

---

## ⚠️ 问题与优化方向

### P0：`alter_table_handler.go` 直接写 `os.Stderr`（7 处）

**文件**：`internal/parser/alter_table_handler.go`

**问题**：直接写 stderr，无法测试、无法重定向、在生产环境中可能污染输出。

```go
// 当前：直接写 stderr，无法测试、无法重定向
fmt.Fprintf(os.Stderr, "warning: DROP COLUMN missing column name\n")
```

**影响**：可测试性、可维护性

**建议**：通过回调函数或 `io.Writer` 注入，将警告收集到统一通道中，由调用方决定如何处理。

---

### P0：`parser.go` 中裸 `recover()` 过于宽泛

**文件**：`internal/parser/parser.go:96-104`

**问题**：捕获了所有 panic（包括真正的编程错误如 nil pointer、index out of range），会掩盖 bug。

```go
defer func() {
    if r := recover(); r != nil {
        err = &ParseError{Message: fmt.Sprintf("recovered from panic: %v", r), Position: -1}
    }
}()
```

**影响**：正确性

**建议**：在 recover 后至少打印 stack trace 用于调试，或只捕获预期的 panic。

---

### P1：`mutation.go` 中大量重复的 Apply 错误检查模式

**文件**：`internal/parser/mutation.go`

**问题**：`SetNotNullMutation.Apply`、`DropNotNullMutation.Apply`、`SetDefaultMutation.Apply`、`DropDefaultMutation.Apply` 等几乎完全相同的代码结构：

```go
func (m XxxMutation) Apply(schema *model.Schema) error {
    ns := schema.GetNamespace(m.Schema)
    if ns == nil { return fmt.Errorf("schema %s not found", m.Schema) }
    table, exists := ns.Tables[m.Table]
    if !exists { return fmt.Errorf("table %s.%s not found", m.Schema, m.Table) }
    col := table.ColumnByName[m.Column]
    if col == nil { return fmt.Errorf("column %s.%s.%s not found", ...) }
    // 实际逻辑只有 1-2 行
}
```

**影响**：可维护性

**建议**：提取通用的 `resolveColumn(schema, schemaName, table, column string) (*model.Namespace, *model.Table, *model.Column, error)` 辅助函数。

---

### P1：`render.go` 中 `Render` 方法的 type switch 有 28 个 case

**文件**：`internal/render/render.go`

**问题**：巨型 switch 语句，每次新增 Operation 类型都需要修改此处。

```go
func (r *Renderer) Render(op diff.Operation) string {
    switch v := op.(type) {
    case *diff.AddTableOp:    return renderAddTable(r, v)
    case *diff.DropTableOp:   return renderDropTable(r, v)
    // ... 26 more cases
    }
}
```

**影响**：可扩展性

**建议**：让 `Operation` 接口增加一个 `Render(r Renderer) string` 方法，每个 Operation 自己实现渲染逻辑。

---

### P2：`diff_tables.go` 中列重命名检测是 O(n²)

**文件**：`internal/diff/diff_tables.go`

**问题**：大表场景下性能不佳，且可能产生误匹配。

```go
for _, srcName := range sourceOnlyNames {
    for _, tgtName := range targetOnlyNames {
        if isColumnRenameCandidate(srcCol, tgtCol) { ... }
    }
}
```

**影响**：性能

**建议**：使用 map 按类型分组后匹配，降到 O(n)。

---

### P2：`model/table.go` 中 `RemoveColumn` 是 O(n)

**文件**：`internal/model/table.go`

**问题**：线性扫描删除列。

```go
func (t *Table) RemoveColumn(name string) {
    delete(t.ColumnByName, name)
    for i, col := range t.Columns {  // O(n) scan
        if col.Name == name {
            t.Columns = append(t.Columns[:i], t.Columns[i+1:]...)
            return
        }
    }
}
```

**影响**：性能

**建议**：如果频繁操作，考虑用标记删除 + 延迟压缩。

---

### P3：`Index.Columns` 字段标记为 deprecated 但未清理

**文件**：`internal/model/table.go`

**问题**：`Columns` 字段标记为 deprecated，但仍在代码中使用。

```go
type Index struct {
    Columns  []string    // deprecated: use Elements
    Elements []IndexElem // new field replacing Columns
}
```

**影响**：技术债

**建议**：制定清理计划，在下一个 major 版本中移除 `Columns`。

---

### P3：`errors/errors.go` 缺乏上下文

**文件**：`internal/errors/errors.go`

**问题**：使用 `errors.New` 创建的错误无法携带上下文信息。

```go
var ErrNotFound = errors.New("resource not found")
```

**影响**：可观测性

**建议**：考虑使用自定义 error 类型，支持 wrapping 和 context。

---

## 📋 优化优先级汇总

| 优先级 | 问题 | 影响 |
|--------|------|------|
| **P0** | `os.Stderr` 硬编码警告输出 | 可测试性、可维护性 |
| **P0** | 裸 `recover()` 掩盖 bug | 正确性 |
| **P1** | Mutation.Apply 重复代码 | 可维护性 |
| **P1** | Render 巨型 switch | 可扩展性 |
| **P2** | 列重命名 O(n²) | 性能 |
| **P2** | RemoveColumn O(n) | 性能 |
| **P3** | Index.Columns deprecated 清理 | 技术债 |
| **P3** | errors 缺乏上下文 | 可观测性 |

---

## 🏗️ 架构改进建议

1. **引入结构化日志**：用 `slog` 替代 `fmt.Fprintf(os.Stderr)`，统一警告和错误输出
2. **Operation 自渲染**：将渲染逻辑从 `Renderer` 的巨型 switch 分散到各个 `Operation` 类型
3. **泛型辅助函数**：Go 1.26 支持泛型，可以用泛型简化一些重复的集合操作
4. **增加 fuzz testing**：对 parser 和 normalize 做 fuzz 测试，提高鲁棒性
5. **考虑 `context` 传递**：部分函数链（如 `Mutation.Apply`）缺少 context 传递，不利于未来添加超时/取消支持

---

## 总结

这是一个**架构设计优秀、测试覆盖充分**的项目。核心数据流（SQL → Parse → Model → Diff → Render → SQL）清晰合理，Mutation 模式是亮点。主要改进点集中在：消除重复代码、替换硬编码的 stderr 输出、以及将巨型 switch 重构为多态分发。整体代码质量在同类开源项目中属于**中上水平**。
