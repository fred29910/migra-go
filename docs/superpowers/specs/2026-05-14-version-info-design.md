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

**变量：**

| 变量 | 类型 | 默认值 | ldflags key | 说明 |
|------|------|--------|-------------|------|
| `Version` | `string` | `"dev"` | `internal/version.Version` | 版本号，来自 git describe |
| `BuildTime` | `string` | `"unknown"` | `internal/version.BuildTime` | UTC 构建时间 |
| `GitCommit` | `string` | `"unknown"` | `internal/version.GitCommit` | 短 commit SHA |
| `GoVersion` | `string` | `"unknown"` | `internal/version.GoVersion` | Go 编译器版本 |

**函数：**

- `func Info() string` — 返回多行格式化版本信息
- `func Short() string` — 返回纯版本号

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

**变更 3：在 `PersistentPreRun` 中处理 `--version`**

```go
rootCmd.PersistentPreRun = func(cmd *cobra.Command, args []string) {
    if viper.GetBool("version") {
        fmt.Println(version.Info())
        os.Exit(0)
    }
}
```

**变更 4：绑定 version flag 到 viper**

```go
viper.BindPFlag("version", cmd.PersistentFlags().Lookup("version"))
```

### 组件 3：`Makefile`（修改）

在 `build` 目标中添加版本信息获取和 ldflags 注入：

```makefile
build:
	@VERSION=$$(git describe --tags --always --dirty 2>/dev/null || echo "dev"); \
	BUILD_TIME=$$(date -u +%Y-%m-%dT%H:%M:%SZ); \
	COMMIT=$$(git rev-parse --short HEAD 2>/dev/null || echo "unknown"); \
	GO_VERSION=$$(go version | awk '{print $3}'); \
	go build --trimpath -ldflags="-s -w \
		-X 'internal/version.Version=$$VERSION' \
		-X 'internal/version.BuildTime=$$BUILD_TIME' \
		-X 'internal/version.GitCommit=$$COMMIT' \
		-X 'internal/version.GoVersion=$$GO_VERSION'" \
		-o migra ./cmd/migra
```

## 使用方式

| 命令 | 效果 |
|------|------|
| `migra --version` | 打印详细版本信息并退出 |
| `migra -v` | 同上（短标志） |
| `migra -V` | 开启 verbose 模式 |
| `migra diff ... --version` | 在任何子命令中都能用（PersistentFlag） |

## 错误处理

- **无 git tag**：`git describe --tags --always --dirty` 返回 short commit hash（如 `a1b2c3d`）
- **不在 git 仓库中**：`git describe` 失败，fallback 到 `"dev"`；`git rev-parse` 失败，fallback 到 `"unknown"`
- **ldflags 未注入**（如直接 `go build`）：使用默认值 `"dev"` / `"unknown"`
- **`-v` 冲突**：通过将 verbose 改为 `-V` 彻底解决

## 测试策略

1. **`internal/version` 包测试**：验证 `Info()` 和 `Short()` 的输出格式
2. **集成测试**：构建后运行 `./migra --version` 验证输出包含所有字段
3. **Makefile 测试**：执行 `make build` 后检查二进制包中的版本信息

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
