# 代码评审报告

**评审时间**：2026-06-03T02:09:04Z  
**评审分支**：develop  
**评审范围**：cmd/migra 与 internal/* 主链路（源码加载、解析、差异比较、执行计划、渲染、push 交互）

## 评审范围与方法
- 评审对象：当前分支 `develop` 的完整仓库代码（重点覆盖 `cmd/migra` 与 `internal/*` 主链路）。
- 评审方式：静态代码审查 + 本地质量信号验证（测试/静态检查/覆盖率）。
- 备注：工作区存在未提交改动：`.gitignore`（`git status` 显示）。

## 质量信号（实测）
- `go test -short`：通过（259 tests）
- `go vet`：通过（无问题）
- `golangci-lint`：通过（0 issues）
- 覆盖率（关键观察）：
  - `internal/introspect`：3.2%
  - `internal/parser/parserutil`：0.0%
  - `cmd/migra`：30.9%

## 总体评价
整体架构是清晰的（`source -> parser -> normalize -> diff -> plan -> render -> push` 分层明确，DAG + 三阶段执行计划设计也合理），但在 **SQL 生成正确性** 和 **push 执行可靠性** 上存在几处高风险问题。  
尤其是 `push` 的超时上下文设计、默认值解析，以及 P3 对象（view/extension/sequence）渲染细节，建议优先修复后再用于高风险环境。

## 主要问题清单（按优先级）

### [必须修复] 1) `push` 交互流程复用 30s 超时上下文，容易在人工确认过程中超时失败
- 证据：
  - `cmd/migra/push.go:30`：`--timeout` 默认 `defaultDiffTimeout`（30s）
  - `cmd/migra/push_runner.go:133`：`context.WithTimeout(cmd.Context(), cfg.Timeout)` 包裹整个 push 流程（包含交互确认/执行/校验）
- 风险：
  - 用户在交互确认时稍慢，就可能触发 context cancel，导致事务执行或 commit 失败，行为不稳定。
- 建议：
  - 将“schema 加载超时”和“执行阶段上下文”拆分：
    - `loadCtx`：保留 timeout
    - `execCtx`：可不设超时或单独配置较长执行超时

### [必须修复] 2) `ALTER TABLE ... SET DEFAULT` 默认值提取逻辑错误
- 证据：
  - `internal/parser/alter_table_handler.go:110` 调用 `extractDefaultExpr`
  - `internal/parser/alter_table_handler.go:165-167`：`return fmt.Sprintf("%v", node)`
- 风险：
  - 默认表达式被序列化成 Go 对象字符串，而不是 SQL 表达式，可能导致 diff 误报、生成错误 SQL。
- 建议：
  - 改为使用 `parserutil.DeparseNode` / `FormatExpression`。
  - 增加 `ALTER COLUMN SET/DROP DEFAULT` 的专门测试用例。

### [必须修复] 3) View 与 Materialized View 渲染模型不一致
- 证据：
  - `internal/introspect/views.go:19,32` 已区分 `materialized`
  - `internal/render/render.go:415,420,424` 始终输出 `CREATE/REPLACE/DROP VIEW`
- 风险：
  - 对 materialized view 生成的 SQL 语义错误或不可执行（如 `CREATE OR REPLACE VIEW` 不适用于 materialized view）。
- 建议：
  - 为 materialized view 引入独立操作或在操作结构中携带 `Materialized` 信息；
  - 渲染时输出 `CREATE MATERIALIZED VIEW` / `DROP MATERIALIZED VIEW`，并避免错误的 replace 语义。

### [必须修复] 4) `DROP EXTENSION` 生成了 schema-qualified 名称，SQL 可能无效
- 证据：
  - `internal/render/render.go:497`：`DROP EXTENSION ... quoteQualifiedIdentifier(op.Schema, op.Name)`
- 风险：
  - PostgreSQL 的 `DROP EXTENSION` 使用扩展名，不应使用 `"schema"."extension"` 形式。
- 建议：
  - 改为仅使用 `quoteIdentifier(op.Name)`。
  - 补充 `drop_extension` 渲染测试（当前测试集中未覆盖该场景）。

## 建议修改（次要但值得改进）

### [建议修改] 5) `ALTER SEQUENCE` 对 `CYCLE` 仅能“打开”，不能正确“关闭”
- 证据：
  - `internal/render/render.go:483`：`seq.Cycle != op.From.Cycle` 时统一追加 `CYCLE`
- 风险：
  - 当目标是 `NO CYCLE` 时，仍可能生成 `CYCLE`，无法收敛到目标状态。
- 建议：
  - 根据目标值分别渲染 `CYCLE` / `NO CYCLE`，并补测试。

### [建议修改] 6) `strict` 行为在不同 source loader 上不一致
- 证据：
  - `internal/source/sql_file_loader.go:37`：仅 strict 模式下 parseErr 失败
  - `internal/source/dir_loader.go:97`：无论 strict 与否，parseErr 都直接失败
- 风险：
  - 同一个 CLI 选项在不同输入源语义不一致，增加用户理解成本。
- 建议：
  - 明确统一策略（推荐统一为：strict 控制是否因解析错误中断）。

### [建议修改] 7) `push` 中断处理使用 goroutine + `os.Exit(1)`，可维护性与可测性较差
- 证据：
  - `cmd/migra/push_runner.go:210`（`signal.Notify`）
  - `cmd/migra/push_runner.go:229`（goroutine 内 `os.Exit(1)`）
- 风险：
  - 直接退出进程会绕过正常返回路径，不利于测试和上层统一处理。
- 建议：
  - 用 context cancel + 返回错误替代 `os.Exit`，并 `signal.Stop(sigChan)` 做资源释放。

## 亮点（做得好的地方）
- 分层架构清晰，模块边界较好（`source/parser/diff/plan/render`）。
- diff 输出做了排序与 DAG 依赖排序，结果稳定性较好。
- 测试总量可观，`plan`、`diff`、`parser` 主流程覆盖不错，lint/vet/test 基线健康。

## 建议修复顺序（落地优先级）
1. 先修：问题 1、2、3、4（直接影响正确性/可执行性）。
2. 再修：问题 5、6、7（一致性、可维护性、可靠性）。
3. 最后补测试：优先补 `introspect`、`parserutil`、`push` 交互中断/超时路径。

---
*报告由 Oz 代理基于 brainstorming 和 chinese-code-review 技能生成。*