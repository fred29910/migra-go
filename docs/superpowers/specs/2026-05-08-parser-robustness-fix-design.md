# 设计文档：解析器健壮性与逻辑自洽性修复

## 1. 背景与目标
在 `migra-go` 的 DDL 解析过程中，当前实现存在以下三个关键问题：
- **逻辑冲突**：`ALTER TABLE` 在 `CREATE TABLE` 之前出现时，会创建占位表导致后续 `CREATE` 失败。
- **运行期 Panic (1)**：`NewParserWith` 允许注入 nil 依赖。
- **运行期 Panic (2)**：Handlers 使用强制类型断言，类型不匹配时直接崩溃。

本方案旨在通过引入模型占位状态、增强依赖注入防护和类型安全检查，提升解析器的稳定性和准确性。

## 2. 设计方案

### 2.1 占位表合并逻辑 (Placeholder Merging)
为了正式支持 `ALTER` 先于 `CREATE` 的场景，我们将通过“占位状态”来协调两者的行为。

#### 模型变更 (`internal/model/table.go`)
- `Table` 结构体新增字段 `IsPlaceholder bool`。
- **JSON 行为**：使用 `json:"-"` 标签，明确该字段为解析期内部状态，不序列化到最终 Schema。
- 默认为 `false`。

#### 列合并冲突策略
在 `CreateTableMutation.Apply` 执行合并时：
- **一致性检查**：若 `CREATE TABLE` 中定义的列在占位表中已存在（由先前的 `ALTER` 创建）：
    - 检查列定义（类型、可空性等）是否一致。
    - **不一致**：返回合并冲突错误（Merge Conflict Error）。
    - **一致**：跳过该列（避免重复添加）。
- **列顺序策略**：
    - `ALTER` 添加的列保持其在占位表中的顺序。
    - `CREATE` 中新定义的列按顺序追加到现有列之后。
    - *理由*：由于 DDL 顺序本身已乱序，逻辑上应以“发现顺序”为准，后续通过正式 `CREATE` 补充。

#### Mutation 行为调整 (`internal/parser/mutation.go`)
- **`AddColumnMutation.Apply`**:
    - 若表不存在：创建新表并设置 `IsPlaceholder = true`。
    - 若表已存在：正常添加列。
- **`CreateTableMutation.Apply`**:
    - 若表不存在：创建新表，`IsPlaceholder = false`。
    - 若表已存在且 `IsPlaceholder == true`:
        - 按照“列合并冲突策略”执行合并。
        - 将 `IsPlaceholder` 设为 `false`。
    - 若表已存在且 `IsPlaceholder == false`:
        - 返回错误 `table %s.%s already exists`。

### 2.2 依赖注入与初始化防护
修改 `NewParserWith` 函数及 `ParseSQL` 入口。

#### `NewParserWith`
对注入的 `registry` 和 `applier` 进行回退处理。

#### `ParseSQL` 入口兜底
为了防止用户跳过构造函数直接使用结构体字面量（如 `p := &Parser{}`），在 `ParseSQL` 开头进行 nil 检查：
```go
func (p *Parser) ParseSQL(sql string) (*model.Schema, error) {
    if p.registry == nil { p.registry = DefaultRegistry() }
    if p.applier == nil { p.applier = &MutationApplier{} }
    // ...
}
```

### 2.3 Handler 类型安全与 Panic 恢复

#### Handler 安全断言
所有实现 `Handler` 接口的结构体必须使用类型防护断言，避免强转触发 panic。

#### Parser 层 Recover 兜底
在 `visitNode` 或 `ParseSQL` 的主循环中增加 `recover` 机制，将未捕获的运行时 panic 转化为结构化的 `ParseError`。
- **目标**：即使某些 Handler 或 util 发生意外溢出/nil 引用，也能返回友好的错误信息而非崩溃。

## 3. 测试策略
- **冲突测试**：验证 `ALTER` 添加 `INT` 列后，`CREATE` 定义同名 `TEXT` 列会触发错误。
- **Panic 恢复测试**：在 Mock Handler 中人为触发 `panic`，验证 Parser 能捕获并返回 `error`。
- **初始化测试**：验证字面量初始化的 `Parser` 仍能正常运行。
- **JSON 测试**：验证导出的 JSON 中不包含 `IsPlaceholder`。

## 4. 规格自检
- [x] 占位符扫描：无。
- [x] 内部一致性：架构与功能描述匹配。
- [x] 范围检查：聚焦于健壮性修复，不涉及无关重构。
- [x] 模糊性检查：明确了合并语义。
