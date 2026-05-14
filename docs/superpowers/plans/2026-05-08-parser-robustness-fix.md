# 解析器健壮性修复实现计划

> **面向 AI 代理的工作者：** 必需子技能：使用 superpowers:subagent-driven-development（推荐）或 superpowers:executing-plans 逐任务实现此计划。步骤使用复选框（`- [ ]`）语法来跟踪进度。

**目标：** 修复解析器在处理无序 DDL 时的逻辑冲突，并增强运行期的稳定性（防御 nil 和类型断言 panic）。

**架构：** 在 `model.Table` 中引入 `IsPlaceholder` 标记，由 `CreateTableMutation` 执行合并与一致性校验；在解析器层级增加依赖注入防护和全局 panic 恢复机制。

**技术栈：** Go, pg_query_go

---

### 任务 1：更新模型层，引入占位标记

**文件：**
- 修改：`internal/model/table.go`

- [ ] **步骤 1：在 Table 结构体中添加 IsPlaceholder 字段**
    在 `Table` 结构体中添加 `IsPlaceholder bool `json:"-"``。

- [ ] **步骤 2：更新 NewTable 构造函数**
    确保 `IsPlaceholder` 默认为 `false`。

- [ ] **步骤 3：编写测试验证 JSON 序列化忽略该字段**
    在 `internal/model/schema_test.go` 或新开测试中验证。

- [ ] **步骤 4：Commit**
    `git add internal/model/table.go && git commit -m "model: add IsPlaceholder field to Table"`

---

### 任务 2：重写 Mutation 应用逻辑（核心合并逻辑）

**文件：**
- 修改：`internal/parser/mutation.go`
- 测试：`internal/parser/mutation_test.go`

- [ ] **步骤 1：编写失败的测试（ALTER 先于 CREATE）**
    编写一个测试用例，先应用 `AddColumnMutation` 再应用 `CreateTableMutation`，期望成功且列定义一致。

- [ ] **步骤 2：修改 AddColumnMutation.Apply**
    如果表不存在，创建并设为占位表。

- [ ] **步骤 3：修改 CreateTableMutation.Apply**
    实现合并逻辑：检查 `IsPlaceholder`，执行一致性校验，合并列定义，并重置标记。

- [ ] **步骤 4：运行测试验证通过**
    `go test ./internal/parser/mutation_test.go -v`

- [ ] **步骤 5：编写列冲突测试**
    验证 `ALTER` 和 `CREATE` 中同名列类型不一致时报错。

- [ ] **步骤 6：Commit**
    `git add internal/parser/mutation.go internal/parser/mutation_test.go && git commit -m "parser: implement placeholder merging and consistency check in mutations"`

---

### 任务 3：解析器初始化防护

**文件：**
- 修改：`internal/parser/parser.go`

- [ ] **步骤 1：修改 NewParserWith**
    增加对 `registry` 和 `applier` 的 nil 检查及默认值回退。

- [ ] **步骤 2：在 ParseSQL 入口增加兜底检查**
    确保哪怕通过字面量创建的 `Parser` 在调用时也能补全依赖。

- [ ] **步骤 3：编写测试验证 nil 依赖下的安全性**
    在 `internal/parser/parser_test.go` 中验证。

- [ ] **步骤 4：Commit**
    `git add internal/parser/parser.go internal/parser/parser_test.go && git commit -m "parser: add nil dependency protection and lazy initialization"`

---

### 任务 4：Handler 类型安全改造

**文件：**
- 修改：`internal/parser/create_table_handler.go`
- 修改：`internal/parser/alter_table_handler.go`
- 修改：`internal/parser/enum_handler.go`
- 修改：`internal/parser/index_handler.go`

- [ ] **步骤 1：修改 CreateTableHandler**
    使用 `ok := node.(pg_nodes.CreateStmt)` 防护。

- [ ] **步骤 2：修改 AlterTableHandler**
    使用 `ok := node.(pg_nodes.AlterTableStmt)` 防护。

- [ ] **步骤 3：修改其他 Handler (Enum, Index)**
    同步进行类型安全改造。

- [ ] **步骤 4：编写测试验证类型错误时返回 error 而非 panic**
    在 `internal/parser/handler_test.go` 中验证。

- [ ] **步骤 5：Commit**
    `git commit -a -m "parser: ensure type safety in all handlers to prevent panics"`

---

### 任务 5：Panic 恢复机制

**文件：**
- 修改：`internal/parser/parser.go`

- [ ] **步骤 1：在 visitNode 中添加 recover 逻辑**
    捕获可能出现的运行时 panic，并将其记录到 `p.errors` 中。

- [ ] **步骤 2：编写测试验证 Recover 效果**
    注入一个故意 panic 的 Handler，验证 `ParseSQL` 返回错误而非进程退出。

- [ ] **步骤 3：运行所有测试**
    `make test` 或 `go test ./...`

- [ ] **步骤 4：Commit**
    `git add internal/parser/parser.go && git commit -m "parser: add recover mechanism to catch unexpected panics"`
