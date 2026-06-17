# migra-go 项目 Code Review 报告

> 评审日期：2025-06-17  
> 评审范围：CLI 交互设计、功能逻辑、Go 代码质量、错误处理与日志、依赖与性能  
> 验证环境：Go 1.26.2 linux/amd64

## 总体评价

`migra-go` 是一个功能完整、架构清晰的 PostgreSQL Schema 差异比较工具。项目采用 Cobra + Viper 构建 CLI，基于 `pg_query_go` 解析 DDL，使用 `pgx` 连接数据库，整体代码组织符合 `cmd/` / `internal/` 规范，核心逻辑（34 种 Diff 操作、DAG 拓扑排序、三阶段执行计划）设计合理且测试覆盖充分。

主要问题集中在 **CLI 交互规范**、**错误输出通道**、**配置绑定完整性** 和 **输入数据的可变性** 四个方面。下面按维度展开。

---

## 验证结果

| 检查项 | 结果 | 说明 |
|---|---|---|
| `go build ./cmd/migra` | ✅ 通过 | 可正常构建 |
| `go vet ./cmd/... ./internal/...` | ✅ 通过 | 无静态检查问题 |
| `golangci-lint run ./...` | ✅ 通过 | 0 issues |
| `go test ./... -short` | ✅ 通过 | 853 个测试用例通过 |
| `gofmt -l .` | ⚠️ 4 个文件未格式化 | 详见下文 |

---

## 1. CLI 规范与交互设计

### 1.1 命令与参数结构

**优点**
- `diff` 支持 0/1/2 参数模式，与配置文件配合灵活（`cmd/migra/diff_runner.go:22-39`）。
- `push` 的事务保护、逐条确认、信号处理（`internal/app/push/push_service.go:80-94`）设计良好。

**问题**

**[建议修改]** `-v` 与 `-V` 的语义与业界惯例相反。

- 当前：`--verbose` 简写为 `-V`，`--version` 简写为 `-v`（`cmd/migra/main.go:25-26`）。
- 惯例：`git`、`curl`、`cargo`、`kubectl` 等工具普遍使用 `-v` / `--verbose` 表示 verbose，`-V` / `--version` 表示版本。
- 影响：muscle memory 强的用户会频繁误触，且与 Cobra 的 `InitDefaultVersionFlag()` 行为冲突（当 `-v` 已被占用时，Cobra 会静默不为 `--version` 分配短标记）。

**重构建议**

```go
// 重构前
rootCmd.PersistentFlags().BoolP("verbose", "V", false, "verbose output")
rootCmd.PersistentFlags().BoolP("version", "v", false, "print version and exit")

// 重构后
rootCmd.PersistentFlags().BoolP("verbose", "v", false, "verbose output")
rootCmd.PersistentFlags().BoolP("version", "V", false, "print version and exit")
```

---

**[建议修改]** `--version` 以 flag 形式实现，而非子命令。

- 当前通过 `PersistentPreRun` + `os.Exit(0)` 处理（`cmd/migra/main.go:28-37`）。
- 问题：无法参与 Cobra 的测试流程（`version_test.go` 已不得不 `exec` 二进制来测试），且 `migra diff --version` 会触发版本输出而非报错。
- 建议：提供 `migra version` 子命令，同时保留 `--version` 作为兼容别名。

---

**[建议修改]** `push` 固定要求 2 个参数，与 `diff` 不对称。

- `push` 使用 `cobra.ExactArgs(2)`（`cmd/migra/push.go:28`），但 `diff` 已支持从配置读取 `database.source` / `database.target`。
- 建议：让 `push` 也支持 0 参数模式，从配置读取源和目标。

---

### 1.2 Flag 设计与配置绑定

**[必须修复]** `--timeout` 未绑定到 Viper，导致配置文件/环境变量不生效。

- `diff.go:50` 和 `push.go:40` 定义了 `--timeout`，但都没有 `viper.BindPFlag`。
- 文档 `docs/configuration.md:60` 已明确说明此问题，说明团队已知但尚未修复。
- 影响：`MIGRA_DIFF_TIMEOUT` 和 `diff.timeout` 配置项被静默忽略。

**重构建议**

```go
// diff.go / push.go init()
_ = viper.BindPFlag("diff.timeout", diffCmd.Flags().Lookup("timeout"))
```

```go
// diff_runner.go parseDiffConfig()
timeout, err := cmd.Flags().GetDuration("timeout")
if err != nil {
    return app.DiffConfig{}, fmt.Errorf("failed to get timeout flag: %w", err)
}
// 如需支持配置 fallback：
if timeout == defaultDiffTimeout {
    if cfgTimeout := viper.GetDuration("diff.timeout"); cfgTimeout > 0 {
        timeout = cfgTimeout
    }
}
```

---

**[建议修改]** `push` 的 `--dry-run`、`--execute`、`--no-verify` 也未绑定到 Viper。

- 这三个 flag 只通过 CLI 设置（`push.go:37-39`），不能通过配置或环境变量控制。
- 对于安全关键参数，不绑定到配置是合理设计，但应在文档中显式说明，避免用户误以为环境变量生效。

---

**[建议修改]** `--unsafe-drop` 的 Viper 键在 `diff` 和 `push` 之间共享。

- `diff` 将其绑定到 `diff.unsafe_drop`（`diff.go:54`）。
- `push` 也绑定到 `diff.unsafe_drop`（`push.go:43`）。
- 语义差异：`diff` 表示“允许输出 DROP 语句”；`push` 表示“跳过 DROP 确认”。虽然当前都是布尔值，但未来可能分化，建议拆分为 `diff.unsafe_drop` 和 `push.unsafe_drop`。

---

**[建议修改]** 缺少 `--quiet` / `-q` 和 shell completion 子命令。

- 工具会在 stdout 输出大量进度信息（`push` 的 diff preview、执行反馈等），脚本化使用时没有安静模式。
- 建议：增加 `--quiet` 控制非错误输出，并注册 `migra completion`。

---

## 2. 功能与逻辑实现

### 2.1 核心逻辑优点

- **DAG 拓扑排序**：Kahn 算法实现正确，支持环检测（`internal/plan/dag.go:104-147`）。
- **重命名列检测**：使用启发式签名匹配，时间复杂度 O(n)，并明确标注 false positive 风险（`internal/diff/diff_tables.go:172-222`）。
- **HandlerRegistry 扩展性**：reflect.Type 路由 + OCP 设计，新增 DDL 只需实现 Handler + Mutation（`internal/parser/registry.go`）。
- **panic 防护**：`parser.go:108-118` 对 handler panic 进行 recover，转换为 ParseError。

### 2.2 关键问题

**[必须修复]** 非 strict 模式下解析错误被静默丢弃。

- `sql_file_loader.go:39-43`：当 `opt.Strict` 为 false 时，`parseErr` 被完全忽略，且返回的 `[]error` 警告切片为 `nil`。
- `dir_loader.go:88-93` 同样丢弃 `parseErr`。
- 影响：用户可能得到一个“部分解析”的 schema，却不知道有语句被跳过。

**重构建议**

```go
func (l *SQLFileLoader) Load(ctx context.Context, source string, opt LoadOptions) (*model.Schema, []error, error) {
    sql, err := os.ReadFile(source)
    if err != nil {
        return nil, nil, fmt.Errorf("read SQL file: %w", err)
    }
    schema, parseErrs, err := parser.ParseSQL(ctx, string(sql), opt.Strict)
    if err != nil {
        return nil, nil, fmt.Errorf("parse SQL file: %w", err)
    }
    return schema, parseErrs, nil  // 始终返回解析警告，不吞掉
}
```

对于 strict=false 的情况，调用方（如 `cmd/migra/diff.go:85-87`）已在输出 warning，这里只需要把警告传出来即可。

---

**[必须修复]** `ComputeDiff` 会原地修改输入的 schema。

- `NormalizeSchemas`（`internal/app/pipeline.go:44-52`）和 `FilterNamespaces`（`internal/app/pipeline.go:16-41`）直接修改 `source` / `target` 的 `Schemas` map。
- 影响：同一组 schema 不能安全地多次调用 `ComputeDiff`；`push` 虽然通过重新加载 schema 规避，但属于架构脆弱点。

**重构建议**

```go
func ComputeDiff(ctx context.Context, source, target *model.Schema, cfg DiffConfig) ([]diff.Operation, []string, error) {
    sourceClone := source.Clone()
    targetClone := target.Clone()

    if err := NormalizeSchemas(sourceClone, targetClone); err != nil {
        return nil, nil, err
    }
    filterWarnings := FilterNamespaces(sourceClone, targetClone, cfg.Schemas)

    differ := diff.NewDiffer()
    operations, warnings := differ.Diff(ctx, sourceClone, targetClone)
    // ...
}
```

需要确保 `internal/model/clone.go` 已覆盖所有字段（目前看起来是完整的）。

---

**[建议修改]** 枚举类型非追加变更不产生任何操作。

- `differ.go:180-187` 仅对追加标签生成 `add_enum_label`，对删除/重排序等发出 warning 但不生成 operation。
- 建议：在文档中明确标注此限制，或考虑生成 `ALTER TYPE ... DROP VALUE` / 重建类型等操作。

---

**[建议修改]** `renderIndexElem` 重复定义。

- `internal/render/render_helpers.go:111-131` 与 `internal/diff/operation_index.go:116-136` 内容几乎一致。
- 建议：提取到 `internal/render/` 或 `internal/util/`，避免双向维护。

---

## 3. Go 代码质量与目录结构

### 3.1 结构优点

- 遵循 `cmd/migra/` + `internal/` 标准布局，与 [go.dev/doc/modules/layout](https://go.dev/doc/modules/layout) 一致。
- 100 余个 `internal/` 源文件按 `app` / `model` / `source` / `parser` / `diff` / `plan` / `render` / `introspect` 分层清晰，职责单一。
- 依赖注入通过 `RunnerDeps` 实现（`cmd/migra/diff_runner.go:86-99`），测试友好。

### 3.2 问题

**[必须修复]** 4 个文件未通过 `gofmt`。

```bash
$ gofmt -l .
cmd/migra/push.go
internal/app/push/push_service.go
internal/model/model_test.go
internal/model/table.go
```

- 在 CI 的 `lint.yml` 中已启用 `gofmt` 检查，但本地/提交前显然未严格执行。
- 建议：立即运行 `make fmt` 或 `gofmt -w .`，并在 CI 中保留该检查。

---

**[建议修改]** Go 版本声明为 `1.26.2`。

- `go.mod:3` 写 `go 1.26.2`，但目前公开 Go 版本最新稳定版为 1.24.x（截至 2025 年中）。
- 当前环境已安装 Go 1.26.2，因此本地构建和 CI 使用 `go-version: '1.26'` 可通过。
- 风险：外部贡献者或标准 CI 环境可能没有 1.26 工具链，会导致 `go mod` 和构建失败。建议确认是否确实需要 1.26 特性，否则降至公开稳定版。

---

**[仅供参考]** 缺少 `.golangci.yml` 配置文件。

- 虽然 `golangci-lint run ./...` 当前返回 0 issues，但使用默认配置。
- 建议：显式配置 `.golangci.yml`，启用 `gofmt`、`govet`、`errcheck`、`ineffassign`、`revive` 等规则，确保不同环境下 linter 行为一致。

---

## 4. 错误处理与日志记录

### 4.1 优点

- 自定义 `internal/errors.Error` 结构（`internal/errors/errors.go:9-41`），支持 `Code` / `Message` / `Cause` 和 `Unwrap()`。
- `push` 服务正确使用 context 取消 + deferred rollback 处理中断，避免 `os.Exit`（`internal/app/push/push_service.go:80-94`）。
- 解析错误包含 SQL 位置信息（`internal/parser/parser.go:20-28`）。

### 4.2 关键问题

**[必须修复]** 错误输出到 stdout，而非 stderr。

- `cmd/migra/main.go:83`：
  ```go
  if err := rootCmd.Execute(); err != nil {
      fmt.Println(err)
      os.Exit(1)
  }
  ```
- 影响：破坏 Unix 管道。例如 `migra diff file.sql pg://... | grep CREATE` 会把错误文本也 grep 进去；`migra diff ... | jq .` 会解析错误文本为 JSON 失败。
- 仅 `setupFlags` 的错误正确输出到 stderr（`main.go:79`）。

**重构建议**

```go
// 重构前
if err := rootCmd.Execute(); err != nil {
    fmt.Println(err)
    os.Exit(1)
}

// 重构后
if err := rootCmd.Execute(); err != nil {
    fmt.Fprintln(os.Stderr, err)
    os.Exit(1)
}
```

更彻底的做法是让 Cobra 使用 `cmd.ErrOrStderr()`：

```go
rootCmd.SetOut(os.Stdout)
rootCmd.SetErr(os.Stderr)
```

---

**[建议修改]** 没有结构化日志，verbose 模式形同虚设。

- `--verbose` 只控制“是否打印配置文件路径”（`cmd/migra/main.go:71-73`）。
- 核心流程（schema 加载、diff、plan、render）没有任何 debug 日志。
- 建议：引入 `log/slog`（Go 1.21+），在 verbose 模式下输出关键步骤。

**示例**

```go
import "log/slog"

func (s *diffService) Run(parent context.Context, cfg DiffConfig) (string, []string, error) {
    if cfg.Verbose {
        slog.Info("loading source schema", "source", cfg.Source)
    }
    sourceSchema, err := s.deps.LoadSchema(ctx, cfg.Source, cfg.Schemas, cfg.Strict)
    // ...
}
```

---

**[建议修改]** 错误链使用 `%w, %w` 双包装。

- `internal/app/diff_service.go:104,108`：
  ```go
  return "", nil, fmt.Errorf("load source: %w, %w", errors.ErrLoadFailed, err)
  ```
- Go 1.20+ 支持多 `%w`，但会让错误文本变成 `load source: load failed: ...`，且 `errors.As` 解析时可能优先匹配 sentinel 而不是底层错误。
- 建议：优先使用 `errors.ErrCodeLoadFailed.WithCause(err)`，或保持单一 `%w`：
  ```go
  return "", nil, fmt.Errorf("load source: %w", errors.ErrCodeLoadFailed.WithCause(err))
  ```

---

## 5. 依赖与性能优化

### 5.1 依赖评估

| 依赖 | 评估 |
|---|---|
| `spf13/cobra` | 标准选择，使用得当。 |
| `spf13/viper` | 合理，但部分 flag 未绑定。 |
| `jackc/pgx/v5` | PostgreSQL 驱动的 Go 生态标准。 |
| `pganalyze/pg_query_go/v6` | 正确选择，DDL 解析与 PG 官方解析器一致。 |
| `stretchr/testify` | 测试标准库。 |
| `pashagolub/pgxmock/v4` | 用于 mock 数据库测试。 |

整体依赖精简、成熟，无过度依赖。

### 5.2 性能问题

**[建议修改]** 没有连接池，`push` 会创建多次独立连接。

- `push` 流程：加载源 schema → 加载目标 schema → 执行 SQL → 验证时重新加载目标 schema。
- 每次数据库加载都新建 `pgx.Connect`（`internal/source/db_loader.go:35`）。
- 建议：在执行阶段使用 `pgxpool` 或复用单个连接；加载阶段可在 push 命令内复用连接。

---

**[建议修改]** 目录加载一次性拼接所有 SQL。

- `internal/source/dir_loader.go:72-82` 用 `strings.Builder` 把所有 `.sql` 文件拼成一个大字符串。
- 对于大型 schema 目录，内存占用较高。建议：流式读取或按文件分批解析。

---

**[建议修改]** 内省查询无批处理。

- `internal/introspect/tables.go:29` 等通过 `rows.Next()` 一次性加载所有元数据。
- 对于超大数据库，可考虑分页或流式处理，但当前场景下优先级不高。

---

## 优先级汇总

| 优先级 | 问题 | 位置 |
|---|---|---|
| [必须修复] | 错误输出到 stdout | `cmd/migra/main.go:83` |
| [必须修复] | 非 strict 模式丢弃解析警告 | `internal/source/sql_file_loader.go:39-43`, `dir_loader.go:88-93` |
| [必须修复] | `ComputeDiff` 原地修改输入 schema | `internal/app/pipeline.go:16-52` |
| [必须修复] | 4 个文件未 `gofmt` | `cmd/migra/push.go`, `internal/app/push/push_service.go`, `internal/model/model_test.go`, `internal/model/table.go` |
| [建议修改] | `-v`/`-V` 语义与惯例相反 | `cmd/migra/main.go:25-26` |
| [建议修改] | `--timeout` 未绑定 Viper | `cmd/migra/diff.go:50`, `push.go:40` |
| [建议修改] | `--version` 建议改为子命令 | `cmd/migra/main.go:28-37` |
| [建议修改] | 缺少结构化日志 | 全局 |
| [建议修改] | 无连接池，多次建连 | `internal/source/db_loader.go` |
| [建议修改] | `renderIndexElem` 重复定义 | `render_helpers.go` / `operation_index.go` |
| [建议修改] | `go.mod` 使用未公开 Go 版本 | `go.mod:3` |
| [仅供参考] | 缺少 `.golangci.yml` | 根目录 |
| [仅供参考] | 缺少 `completion` / `--quiet` | CLI |

---

## 建议的修复顺序

1. **立即修复**：`gofmt` 所有文件 + `main.go` 错误输出到 stderr。
2. **本轮迭代**：修复解析警告传递、`--timeout` Viper 绑定、schema 克隆。
3. **下轮迭代**：调整 `-v`/`-V` 语义、引入 `log/slog`、抽离 `renderIndexElem`、评估连接池。
4. **长期优化**：补充 `.golangci.yml`、增加 `--quiet` 和 completion 子命令。

整体而言，项目基础扎实，主要问题都是可快速修复的交互与配置细节，不影响核心架构的正确性。
