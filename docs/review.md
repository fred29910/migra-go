# migra-go 代码质量评审报告

> 评审日期：2026-06-15
> 分支：main (v0.3.1)
> 评审范围：全量代码（118 Go 文件，含 47 个测试文件，~18,000 行）

---

## 📊 项目概览

| 指标 | 数值 |
|------|------|
| Go 文件总数 | 118 个 |
| 测试文件 | 47 个 |
| 总代码行数 | ~18,000 行 |
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
- 渲染器采用多态分发（`op.RenderString`），消除巨型 switch

### 2. 测试覆盖出色

- 47 个测试文件，覆盖率高
- 测试风格统一，使用 table-driven tests
- 有 golden test、benchmark test、integration test

### 3. 规范化（Normalize）做得好

- `normalize` 包处理了大小写、引号、类型转换、默认值表达式等差异
- 有效减少了 false diff

### 4. 错误处理有层次

- 定义了结构化 `Error` 类型（Code/Message/Cause），支持 `Unwrap`
- 保留哨兵错误（`errors.New`）向后兼容
- `Mutation.Apply` 返回具体错误而非 panic
- Parser 通过 `WarningEmitter` 回调发射警告，不再硬编码 `os.Stderr`

### 5. 渲染器已重构

- 从巨型 switch（28+ case）重构为多态分发模式
- 每个 Operation 自己实现 `RenderString` 方法
- 渲染逻辑拆分为 `render.go`（73 行）+ `render_helpers.go`（131 行）+ `render_json.go`（36 行）

### 6. Push 中断处理改进

- 使用 `context.WithCancel` + signal handling（SIGINT/SIGTERM）
- 不再使用 `os.Exit(1)`，改为优雅回滚

---

## ⚠️ 问题与优化方向

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

### P3：`parser.go` 中裸 `recover()` 仍存在（已改进）

**文件**：`internal/parser/parser.go:118-127`

**当前状态**：已改进——在 recover 时记录完整 stack trace，便于调试。

```go
defer func() {
    if r := recover(); r != nil {
        buf := make([]byte, 4096)
        n := runtime.Stack(buf, false)
        err = &ParseError{
            Message: fmt.Sprintf("recovered from panic: %v\nstack trace:\n%s", r, buf[:n]),
            Position: -1,
        }
    }
}()
```

**影响**：低（已改进）

**建议**：未来可考虑只捕获预期的 panic，或添加 panic 来源标记。

---

## 📋 优化优先级汇总

| 优先级 | 问题 | 影响 |
|--------|------|------|
| **P1** | Mutation.Apply 重复代码 | 可维护性 |
| **P2** | 列重命名 O(n²) | 性能 |
| **P2** | RemoveColumn O(n) | 性能 |
| **P3** | Index.Columns deprecated 清理 | 技术债 |
| **P3** | parser recover 来源标记 | 可调试性 |

---

## 🏗️ 架构改进建议

1. **引入结构化日志**：用 `slog` 替代 `fmt.Fprintf(os.Stderr)`，统一警告和错误输出
2. **泛型辅助函数**：Go 1.26 支持泛型，可以用泛型简化一些重复的集合操作
3. **增加 fuzz testing**：对 parser 和 normalize 做 fuzz 测试，提高鲁棒性
4. **考虑 `context` 传递**：部分函数链（如 `Mutation.Apply`）缺少 context 传递，不利于未来添加超时/取消支持
5. **提取 resolveColumn 辅助函数**：消除 mutation.go 中的重复代码

---

## 已修复的历史问题

| 问题 | 修复版本 | 说明 |
|------|---------|------|
| `os.Stderr` 硬编码警告输出 | v0.3.0 | 改为 WarningEmitter 回调 |
| Render 巨型 switch（28+ case） | v0.3.0 | 重构为多态分发 |
| push 中断使用 `os.Exit(1)` | v0.2.1 | 改为 context.WithCancel + signal handling |
| errors 缺乏上下文 | v0.3.0 | 新增结构化 Error 类型 |
| recover() 无 stack trace | v0.3.0 | 添加 runtime.Stack 记录 |

---

## 总结

这是一个**架构设计优秀、测试覆盖充分**的项目。核心数据流（SQL → Parse → Model → Diff → Render → SQL）清晰合理，Mutation 模式和渲染多态分发是亮点。v0.3.0 以来的重构解决了大部分历史遗留问题（os.Stderr 硬编码、巨型 switch、错误处理）。主要改进点集中在：消除重复代码（Mutation.Apply）、优化算法复杂度（列重命名 O(n²)）、以及清理技术债（Index.Columns deprecated）。整体代码质量在同类开源项目中属于**中上水平**。
