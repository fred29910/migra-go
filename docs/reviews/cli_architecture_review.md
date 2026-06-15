# migra-go 架构与代码深度评审报告

**评审维度**：CLI 规范与交互设计、功能与逻辑实现、Go 代码质量与目录结构、错误处理与日志记录、依赖与性能优化。
**目标**：指导后续的架构优化、重构与 Bug 修复。

---

## 1. CLI 规范与交互设计 (CLI Conventions & UX)

**✅ 亮点**：选用了业界标准的 `Cobra` + `Viper`，全局配置、环境变量、Flag 的层级设计得很完善。针对 `diff` 命令的“0/1/2个参数自动降级解析”机制对用户非常便利。

**⚠️ 待优化：修复 Cobra 与 Viper 的生命周期竞态条件**
在目前的实现中，直接在 `init()` 函数中读取 Viper 的值作为 Flag 的默认值，会导致配置文件中的默认值失效（因为 `init()` 执行时配置往往还未被 `initConfig()` 加载）。

**🛠 重构对比：**
```go
// ❌ 存在竞态条件的代码 (当前模式)
func init() {
    // 此时 Viper 还未读取 ~/.migra.yaml，拿到的是空值
    diffCmd.Flags().StringSliceP("schema", "s", viper.GetStringSlice("diff.schemas"), "Schemas to compare")
}

// ✅ 正确的结合方式 (延后绑定)
func init() {
    diffCmd.Flags().StringSliceP("schema", "s", []string{"public"}, "Schemas to compare")
    // 将 Flag 与 Viper 绑定，在运行时 Viper 会自动合并 Flag、Env 和 Config
    viper.BindPFlag("diff.schemas", diffCmd.Flags().Lookup("schema"))
}

// 在 RunE 中获取最终值：
// schemas := viper.GetStringSlice("diff.schemas")
```

---

## 2. 功能与逻辑实现 (Logic Implementation)

**✅ 亮点**：`push` 命令实现了交互式确认、事务包装、Ctrl+C (SIGINT) 安全回滚，以及非事务性 DDL 的检测，这对于数据库迁移工具来说是极其严谨且必须的。

**⚠️ 待优化：缓解 `cmd/` 目录过重（Fat Cmd Smell）**
`cmd/migra/push_runner.go`（超过 400 行）承载了极重的核心业务逻辑（事务管理、信号处理、交互式 I/O）。这违反了关注点分离，也让 CLI 难以被其他工具作为库调用。

**🛠 重构方向：**
将 `push_runner.go` 的核心逻辑下沉到 `internal/app/` 或专门的 `internal/push/` 包中。
```go
// ❌ 当前：cmd/migra/push_runner.go 直接处理事务和中断
func executeWithConfirmation(conn *pgx.Conn, plan []Operation) error {
    tx, _ := conn.Begin(ctx)
    // 监听 os.Interrupt...
}

// ✅ 推荐：cmd/ 仅负责参数解析和依赖注入
// cmd/migra/push.go
func runPush(cmd *cobra.Command, args []string) error {
    // 1. 解析参数
    // 2. 构造依赖
    svc := app.NewPushService(db, os.Stdout, os.Stdin)
    // 3. 执行业务
    return svc.ExecutePlan(cmd.Context(), plan, app.PushOptions{DryRun: false})
}
```

---

## 3. Go 代码质量与目录结构 (Go Quality & Project Layout)

**✅ 亮点**：项目结构清晰（`internal/app, model, source, parser, diff, plan, render`），高度遵循 SOLID 原则。策略模式（Loader Registry）和模板方法（baseOperation）用得恰到好处。

**⚠️ 待优化：消除包级别的全局可变状态（Global Mutable State）**
在 `cmd/migra/` 下存在包级别的变量（如 `sourceRegistry`, `connectToDB`）。虽然在测试中通过替换这些函数实现了 Mock，但这并非 Go 的最佳实践（并发测试时不安全）。

**🛠 重构对比：**
引入**命令结构体（Command Struct）**模式，将依赖作为实例属性注入：

```go
// ❌ 当前：包级别变量
var connectToDB = defaultConnectToDB

// ✅ 推荐：依赖注入模式
type AppCLI struct {
    DBConnector   func(ctx context.Context, url string) (dbConnector, error)
    SourceLoader  source.Registry
    Out           io.Writer
}

func NewRootCmd(cli *AppCLI) *cobra.Command {
    cmd := &cobra.Command{Use: "migra"}
    cmd.AddCommand(newDiffCmd(cli))
    // ...
    return cmd
}
```

---

## 4. 错误处理与日志记录 (Error Handling & Logging)

**✅ 亮点**：定义了结构化的 `errors.Error{Code, Message, Cause}`，并在 Parser 层面提供了包含代码位置的错误诊断。

**⚠️ 待优化：修复错误包裹（Error Wrapping）导致的上下文丢失**
如 `diff_service.go` 中，使用 `fmt.Errorf("context: %w", errors.ErrLoadFailed)` 会遮蔽底层真实错误（如网络超时）。只有 `errors.Is` 能匹配，但原始的 cause 被丢弃了。

**🛠 重构对比：**
结合 Go 1.20+ 的 `errors.Join` 或多重 `%w` 进行重构：

```go
// ❌ 错误做法：丢失了真实的底层错误(err)
if err != nil {
    return fmt.Errorf("failed to load source: %w", errors.ErrLoadFailed) 
}

// ✅ 推荐做法 1：使用 Go 1.20 多重 wrapping
if err != nil {
    return fmt.Errorf("failed to load source %s: %w, %w", target, errors.ErrLoadFailed, err)
}

// ✅ 推荐做法 2：完善自定义 Error 结构体，支持无损 Wrap
func (e *Error) Wrap(err error) *Error {
    e.Cause = err // 确保 Error 实现了 Unwrap() error { return e.Cause }
    return e
}
```
**日志建议**：逐步引入 `log/slog`（Go 1.21+ 内置），常规模式下仅输出人类可读信息，在开启 `-V` 时通过 `slog` 输出带有请求耗时、AST 解析耗时等 Debug 级别的结构化日志。

---

## 5. 依赖与性能优化 (Dependencies & Performance)

**✅ 亮点**：依赖极为克制。`pgx/v5` 保证了高性能，`pg_query_go` 保证了 100% 的语法兼容性。DAG（Kahn 算法）依赖排序设计极佳。

**⚠️ 评估与前瞻：**
1. **内存消耗**：`pg_query_go` 会将 SQL 解析为庞大的 JSON/Protobuf 格式的 AST 树。面对极大规模的 `pg_dump` 产物时可能会产生内存峰值。未来可针对超大 SQL 文件考虑流式读取或正则预处理拆分后再交由 AST 解析。
2. **DAG 排序性能**：对于数据库 schema 而言（通常表数量 < 5000），目前的 $O(V+E)$ 的 Kahn 算法性能损耗微乎其微，当前的抽象和实现非常合理，无需过度优化。

---

## 总结行动项 (Action Items)

1. [ ] **Refactor**: 修复 `cmd/migra` 中的 Viper Flag 绑定逻辑，解决配置读取竞态问题。
2. [ ] **Refactor**: 重构 `cmd/migra/push_runner.go`，将核心业务逻辑和事务控制抽离至 `internal/app` 或 `internal/push`。
3. [ ] **Refactor**: 引入命令结构体（Command Struct）重构 `cmd/migra`，消除全局变量，改用显式依赖注入。
4. [ ] **Fix**: 全局排查 `fmt.Errorf` 的 `%w` 使用情况，避免丢失底层错误原因，保障错误追踪的完整性。
