# Version Info Design

## 概述

在 `migra` CLI 工具中添加版本信息支持。通过 Makefile 在构建时自动从 git tag 获取版本号，并将版本号、构建时间、git commit SHA、Go 版本等信息注入到二进制包中。用户可通过 `migra --version` 或 `migra -v` 查看详细信息。

## 目标

- 创建独立的 `internal/version` 包管理版本信息
- 在 `make build` 时通过 `-ldflags` 自动注入版本信息
- 将 verbose 短标志从 `-v` 改为 `-V`（大写），释放 `-v` 给 `--version` 使用
- 通过 `migra --version` / `migra -v` 全局 flag 输出版本信息
- 保持与现有 cobra + viper 架构一致

## 架构

### 整体流程

```
git tag v0.2.0
       │
       ▼
make build
       │
       ├── git describe --tags --always --dirty → "v0.2.0"
       ├── date -u +%Y-%m-%dT%H:%M:%SZ         → "2026-05-14T09:38:00Z"
       ├── git rev-parse --short HEAD           → "a1b2c3d"
       └── go version | awk '{print $3}'        → "go1.26.2"
       │
       ▼
go build -ldflags="-X 'internal/version.Version=v0.2.0'
                    -X 'internal/version.BuildTime=2026-05-14T09:38:00Z'
                    -X 'internal/version.GitCommit=a1b2c3d'
                    -X 'internal/version.GoVersion=go1.26.2'"
       │
       ▼
./migra --version
│
└──▶ Version:    v0.2.0
     Built:      2026-05-14T09:38:00Z
     Git Commit: a1b2c3d
     Go Version: go1.26.2
```

### 组件关系

```
┌─────────────────────────────────────────────────────────┐
│                      Makefile                           │
│  通过 shell 命令获取版本信息，通过 -ldflags 注入          │
└──────────────┬──────────────────────────────────────────┘
               │ -ldflags "-X"
               ▼
┌──────────────────────────────────────────────────────────┐
│            internal/version/version.go                   │
│                                                          │
│  var Version   = "dev"     // 可被 ldflags 覆盖          │
│  var BuildTime = "unknown"                               │
│  var GitCommit = "unknown"                               │
│  var GoVersion = "unknown"                               │
│                                                          │
│  func Info() string   → 多行格式化输出                    │
│  func Short() string  → 仅版本号                         │
└──────────────┬───────────────────────────────────────────┘
               │ 调用
               ▼
┌──────────────────────────────────────────────────────────┐
│            cmd/migra/main.go (修改)                      │
│                                                          │
│  rootCmd.PersistentFlags().BoolP("verbose", "V", ...)   │
│  rootCmd.PersistentFlags().BoolP("version", "v", ...)   │
│                                                          │
│  rootCmd.PersistentPreRun:                               │
│    if version flag → print version.Info(), os.Exit(0)   │
│    bind flags to viper                                   │
└──────────────────────────────────────────────────────────┘
```

## 组件

### 组件 1：`internal/version/version.go`（新文件）

版本信息包，定义可被 ldflags 覆盖的包级变量和格式化函数。

> **注意：** 包级变量 `Version`、`BuildTime`、`GitCommit`、`GoVersion` 在构建时通过 `-ldflags "-X"` 注入。当直接使用 `go build`（不通过 Makefile）时，变量取默认值 `"dev"` / `"unknown"`。

**变量：**

| 变量 | 类型 | 默认值 | ldflags key | 说明 |
|------|------|--------|-------------|------|
| `Version` | `string` | `"dev"` | `internal/version.Version` | 版本号，来自 git describe |
| `BuildTime` | `string` | `"unknown"` | `internal/version.BuildTime` | UTC 构建时间 |
| `GitCommit` | `string` | `"unknown"` | `internal/version.GitCommit` | 短 commit SHA |
| `GoVersion` | `string` | `"unknown"` | `internal/version.GoVersion` | Go 编译器版本 |

**函数：**

- `func Info() string` — 返回多行格式化版本信息
- `func Short() string` — 返回纯版本号，适用于 CI/CD 中提取版本号做 docker tag 等场景

**输出格式示例：**

```
Version:    v0.2.0
Built:      2026-05-14T09:38:00Z
Git Commit: a1b2c3d
Go Version: go1.26.2
```

### 组件 2：`cmd/migra/main.go`（修改）

**变更 1：verbose 短标志从 `-v` 改为 `-V`**

```go
// 之前
rootCmd.PersistentFlags().BoolP("verbose", "v", false, "verbose output")
// 之后
rootCmd.PersistentFlags().BoolP("verbose", "V", false, "verbose output")
```

**变更 2：新增 `--version` / `-v` 全局 flag**

```go
rootCmd.PersistentFlags().BoolP("version", "v", false, "print version and exit")
```

**变更 3：在 `init()` 中通过函数组合方式设置 `PersistentPreRun`**

使用函数组合而非直接赋值，确保未来扩展 `PersistentPreRun` 时不会覆盖现有逻辑：

```go
existingPreRun := rootCmd.PersistentPreRun
rootCmd.PersistentPreRun = func(cmd *cobra.Command, args []string) {
    if existingPreRun != nil {
        existingPreRun(cmd, args)
    }
    if v, _ := cmd.Flags().GetBool("version"); v {
        fmt.Println(version.Info())
        os.Exit(0)
    }
}
```

**关键点：**
- 使用 `cmd.Flags().GetBool("version")` 而非 `viper.GetBool("version")`，避免对 `BindPFlag` 时序的依赖
- `version` flag 不需要从配置文件读取，因此不需要经过 viper
- 函数组合模式确保未来子命令添加自己的 `PersistentPreRun` 时，`--version` 检查仍然有效

**变更 4：在 `setupFlags()` 中绑定 version flag 到 viper**

与现有的 `config` 和 `verbose` binding 保持一致，统一放在 `setupFlags()` 中：

```go
func setupFlags(cmd *cobra.Command) error {
    if err := viper.BindPFlag("config", cmd.PersistentFlags().Lookup("config")); err != nil {
        return fmt.Errorf("failed to bind config flag: %w", err)
    }
    if err := viper.BindPFlag("verbose", cmd.PersistentFlags().Lookup("verbose")); err != nil {
        return fmt.Errorf("failed to bind verbose flag: %w", err)
    }
    if err := viper.BindPFlag("version", cmd.PersistentFlags().Lookup("version")); err != nil {
        return fmt.Errorf("failed to bind version flag: %w", err)
    }
    return nil
}
```

### 组件 3：`Makefile`（修改）

在 `build` 目标中添加版本信息获取和 ldflags 注入。变量定义行不加 `@` 以便构建失败时可调试，仅 `go build` 行加 `@` 减少输出噪音：

```makefile
build:
	VERSION=$$(git describe --tags --always --dirty 2>/dev/null || echo "dev"); \
	BUILD_TIME=$$(date -u +%Y-%m-%dT%H:%M:%SZ); \
	COMMIT=$$(git rev-parse --short HEAD 2>/dev/null || echo "unknown"); \
	GO_VERSION=$$(go version | awk '{print $3}'); \
	@go build --trimpath -ldflags="-s -w \
		-X 'internal/version.Version=$$VERSION' \
		-X 'internal/version.BuildTime=$$BUILD_TIME' \
		-X 'internal/version.GitCommit=$$COMMIT' \
		-X 'internal/version.GoVersion=$$GO_VERSION'" \
		-o migra ./cmd/migra
```

> **说明：** `scripts/release.sh` 无需修改。发布流程的构建由 CI（`.github/workflows/release.yml`）完成，`make build` 仅用于本地开发。

## 使用方式

| 命令 | 效果 |
|------|------|
| `migra --version` | 打印详细版本信息并退出 |
| `migra -v` | 同上（短标志） |
| `migra -V` | 开启 verbose 模式 |
| `migra diff ... --version` | 在任何子命令中都能用（PersistentFlag） |

## 破坏性变更

### verbose 短标志从 `-v` 改为 `-V`

| 影响范围 | 变更前 | 变更后 | 说明 |
|----------|--------|--------|------|
| CLI 参数 | `-v` (verbose) | `-V` (verbose) | 短标志变更 |
| CLI 参数 | N/A | `-v` (version) | 新增短标志 |
| 环境变量 | `MIGRA_VERBOSE` | `MIGRA_VERBOSE` | 不受影响（绑定的是长标志 `--verbose`） |
| README | `-v, --verbose` | `-V, --verbose` | 需同步更新 CLI 参数表 |
| 现有测试 | `TestSetupFlags_BindsViperKeys` | 不变 | 测试使用 `Bool()` 而非 `BoolP()`，不依赖短标志名 |

**影响评估：** 当前 verbose 仅在 `initConfig()` 中用于条件打印配置路径（`main.go:54`），`diff` 和 `push` 子命令中均未使用。变更影响范围小。

## 错误处理

- **无 git tag**：`git describe --tags --always --dirty` 返回 short commit hash（如 `a1b2c3d`）
- **不在 git 仓库中**：`git describe` 失败，fallback 到 `"dev"`；`git rev-parse` 失败，fallback 到 `"unknown"`
- **ldflags 未注入**（如直接 `go build`）：使用默认值 `"dev"` / `"unknown"`
- **`-v` 冲突**：通过将 verbose 改为 `-V` 彻底解决

## 测试策略

1. **`internal/version` 包单元测试**：验证 `Info()` 和 `Short()` 的输出格式
2. **全局 flag 注册测试**：验证 `-v`/`--version` 被正确注册为 `PersistentFlag`
3. **CLI 集成测试**（`os/exec` 子进程）：`./migra --version` 输出包含 Version/Built/Git Commit/Go Version 四行
   - 注意：`PersistentPreRun` 中调用 `os.Exit(0)`，必须用 `exec.Command` 启动子进程测试，不能直接在 `go test` 中调用
4. **Makefile 构建验证**：执行 `make build` 后运行 `./migra --version` 检查版本信息
5. **`-v` 短标志回归测试**：验证 `-v` 触发 version 输出而非 verbose 模式

## 设计说明

- **`--version` 对子命令 Runner 零侵入**：版本检查在 `PersistentPreRun` 中通过 `os.Exit(0)` 提前退出，不会到达任何子命令的 `RunE`（`diff_runner.go`、`push_runner.go` 均不受影响）
- **`version` flag 不经过 viper 读取**：`cmd.Flags().GetBool("version")` 直接读取 cobra flag，避免与 viper binding 的时序耦合。`BindPFlag` 仍然注册以保持一致性和未来扩展性

## 成功标准

- `make build` 后 `./migra --version` 输出包含 Version、Built、Git Commit、Go Version 四行
- `./migra -v` 输出与 `--version` 相同
- `./migra -V` 开启 verbose 模式
- 无 git tag 时版本号显示为 commit hash 或 `"dev"`
- 所有现有测试通过

## 未来扩展

- 可在 `internal/version` 包中添加 `JSON()` 函数，支持机器可读输出
- 可在 CI/CD 中通过 `git tag` 自动设置语义化版本号
- 可将 `version.Info()` 集成到日志输出中
