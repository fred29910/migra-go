# 最近三个 Commit 代码评审（按优先级）

评审范围：`2afd97a`、`c46cd3e`、`940a6a3`

## 1. [必须修复] DAG 节点按 ObjectKey 去重导致操作丢失

- 位置：`internal/plan/dag.go`
- 问题：当前 `AddNode` 以 `ObjectKey` 去重；同一列上的多个操作（如 `alter type` + `set not null`）会共享同一个 key，后加入的操作会被吞掉。
- 风险：生成迁移 SQL 不完整，可能出现“看起来成功、实际少执行”的隐蔽故障。
- 建议：DAG 节点应以“操作实例”为粒度建模，`ObjectKey` 仅用于依赖索引，不应用于节点去重。

## 2. [必须修复] Parser 未处理 scanner.Err()，可能静默截断 SQL

- 位置：`internal/parser/parser.go`
- 问题：`bufio.Scanner` 循环后未检查 `scanner.Err()`。
- 风险：遇到超长 token 或扫描异常时，解析结果不完整且无明确报错。
- 建议：补 `scanner.Err()` 检查并记录错误；同时提高 scanner buffer 上限。

## 3. [必须修复] Parser 跨多次调用存在状态残留

- 位置：`internal/parser/parser.go`
- 问题：`schema` 和 `errors` 存在于 `Parser` 实例中，`ParseSQL` 开头未重置。
- 风险：复用同一 parser 时，旧错误和旧 schema 混入新结果。
- 建议：`ParseSQL` 开始时重置 `schema/errors`，确保每次解析相互独立。

## 4. [必须修复] README 命令示例与真实 CLI 不一致

- 位置：`README.md`、`cmd/schemadiff/diff.go`
- 问题：README 使用 `schemadiff file_a.sql file_b.sql`，而程序实际子命令是 `schemadiff diff ...`。
- 风险：用户按文档执行失败，影响可用性和信任度。
- 建议：README 示例统一改为 `schemadiff diff ...`。

## 5. [建议修改] 多行注释移除正则未跨行

- 位置：`internal/parser/parser.go`
- 问题：`/\*.*?\*/` 中 `.` 默认不匹配换行，真实多行注释可能残留。
- 建议：使用 `(?s)/\*.*?\*/` 或状态机处理。

## 6. [问题] TopoSort 已实现但主流程未接入

- 位置：`internal/plan/plan.go`、`cmd/schemadiff/diff.go`
- 现状：`TopoSort` 存在，但执行主路径仍按 map 遍历聚合，顺序不稳定。
- 建议：按 stage 固定顺序 + stage 内 `TopoSort`，保证迁移顺序可预测。

---

总体评价：三个 commit 在功能覆盖上推进很快，尤其 parser 测试和 golden 文件补全是明显加分项；但上面 4 个必须修复项涉及结果正确性与用户可用性，建议优先落地后再继续扩展能力。
