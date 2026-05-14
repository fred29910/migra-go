# Version Info 实现计划

> **面向 AI 代理的工作者：** 必需子技能：使用 superpowers:subagent-driven-development（推荐）或 superpowers:executing-plans 逐任务实现此计划。步骤使用复选框（`- [ ]`）语法来跟踪进度。

**目标：** 在 migra CLI 工具中添加版本信息支持，通过 Makefile ldflags 注入版本号/构建时间/git commit/Go 版本，用户可通过 `migra --version` / `migra -v` 查看。

**架构：** 创建独立的 `internal/version` 包管理版本变量，修改 `cmd/migra/main.go` 添加 `--version` 全局 flag 和 `PersistentPreRun` 处理，修改 `Makefile` 在构建时通过 `-ldflags` 注入版本信息。

**技术栈：** Go, cobra, viper, Makefile

---

## 文件结构

| 文件 | 操作 | 职责 |
|------|------|------|
| `internal/version/version.go` | 创建 | 定义版本变量（Version/BuildTime/GitCommit/GoVersion）和格式化函数（Info/Short） |
| `internal/version/version_test.go` | 创建 | 验证 Info() 和 Short() 的输出格式 |
| `cmd/migra/main.go` | 修改 | 添加 `--version`/`-v` flag，PersistentPreRun 处理，verbose 短标志改为 `-V` |
| `cmd/migra/version_test.go` | 创建 | 验证 version flag 注册和 CLI 集成测试（os/exec 子进程） |
| `Makefile` | 修改 | build 目标添加版本信息获取和 ldflags 注入 |

---

### 任务 1：创建 `internal/version/version.go`

**文件：**
- 创建：`internal/version/version.go`

- [ ] **步骤 1：编写版本包代码**

```go
// Package version holds build-time injected version information.
//
// Variables Version, BuildTime, GitCommit, and GoVersion are set
// at build time via -ldflags "-X". When built directly with
// 'go build' (without Makefile), they default to "dev"/"unknown".
package version

import "fmt"

var (
	// Version is the semantic version or git describe output.
	Version = "dev"
	// BuildTime is the UTC build timestamp in RFC3339 format.
	BuildTime = "unknown"
	// GitCommit is the short SHA of the git commit.
	GitCommit = "unknown"
	// GoVersion is the Go compiler version used to build.
	GoVersion = "unknown"
)

// Info returns a multi-line formatted version string.
func Info() string {
	return fmt.Sprintf(
		"Version:    %s\nBuilt:      %s\nGit Commit: %s\nGo Version: %s",
		Version, BuildTime, GitCommit, GoVersion,
	)
}

// Short returns the version string only.
func Short() string {
	return Version
}
```

- [ ] **步骤 2：Commit**

```bash
git add internal/version/version.go
git commit -m "feat(version): add version info package"
```

---

### 任务 2：创建 `internal/version/version_test.go`

**文件：**
- 创建：`internal/version/version_test.go`

- [ ] **步骤 1：编写测试代码**

```go
package version

import (
	"strings"
	"testing"
)

func TestInfo_ContainsAllFields(t *testing.T) {
	// Set known values for testing
	Version = "v0.2.0"
	BuildTime = "2026-05-14T10:00:00Z"
	GitCommit = "abc1234"
	GoVersion = "go1.26.2"
	defer func() {
		Version = "dev"
		BuildTime = "unknown"
		GitCommit = "unknown"
		GoVersion = "unknown"
	}()

	got := Info()
	for _, field := range []string{"Version:", "Built:", "Git Commit:", "Go Version:"} {
		if !strings.Contains(got, field) {
			t.Errorf("Info() missing field %q, got:\n%s", field, got)
		}
	}
	if !strings.Contains(got, "v0.2.0") {
		t.Errorf("Info() should contain version, got:\n%s", got)
	}
}

func TestShort_ReturnsVersion(t *testing.T) {
	Version = "v0.2.0"
	defer func() { Version = "dev" }()

	got := Short()
	if got != "v0.2.0" {
		t.Errorf("Short() = %q, want %q", got, "v0.2.0")
	}
}

func TestDefaults(t *testing.T) {
	// After resetting to defaults
	Version = "dev"
	BuildTime = "unknown"
	GitCommit = "unknown"
	GoVersion = "unknown"

	got := Info()
	if !strings.Contains(got, "dev") {
		t.Errorf("Info() with defaults should contain 'dev', got:\n%s", got)
	}
	if Short() != "dev" {
		t.Errorf("Short() with defaults = %q, want %q", Short(), "dev")
	}
}
```

- [ ] **步骤 2：运行测试验证通过**

```bash
go test ./internal/version/ -v
```

预期输出：

```
=== RUN   TestInfo_ContainsAllFields
--- PASS: TestInfo_ContainsAllFields (0.00s)
=== RUN   TestShort_ReturnsVersion
--- PASS: TestShort_ReturnsVersion (0.00s)
=== RUN   TestDefaults
--- PASS: TestDefaults (0.00s)
PASS
ok      github.com/fred29910/migra-go/internal/version
```

- [ ] **步骤 3：Commit**

```bash
git add internal/version/version_test.go
git commit -m "test(version): add unit tests for version package"
```

---

### 任务 3：修改 `cmd/migra/main.go`

**文件：**
- 修改：`cmd/migra/main.go`

- [ ] **步骤 1：修改 import 块，添加 version 包**

将 import 块从：

```go
import (
	"fmt"
	"os"

	"github.com/spf13/cobra"
	"github.com/spf13/viper"
)
```

改为：

```go
import (
	"fmt"
	"os"

	"github.com/fred29910/migra-go/internal/version"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
)
```

- [ ] **步骤 2：修改 init() — verbose 短标志改为 `-V`，添加 `--version` flag**

将 `init()` 从：

```go
func init() {
	cobra.OnInitialize(initConfig)

	// Global flags
	rootCmd.PersistentFlags().StringP("config", "c", "", "config file (default is $HOME/.migra.yaml)")
	rootCmd.PersistentFlags().BoolP("verbose", "v", false, "verbose output")
}
```

改为：

```go
func init() {
	cobra.OnInitialize(initConfig)

	// Global flags
	rootCmd.PersistentFlags().StringP("config", "c", "", "config file (default is $HOME/.migra.yaml)")
	rootCmd.PersistentFlags().BoolP("verbose", "V", false, "verbose output")
	rootCmd.PersistentFlags().BoolP("version", "v", false, "print version and exit")

	// Handle --version in PersistentPreRun using function composition
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
}
```

- [ ] **步骤 3：修改 `setupFlags()` — 添加 version flag binding**

将 `setupFlags()` 从：

```go
func setupFlags(cmd *cobra.Command) error {
	if err := viper.BindPFlag("config", cmd.PersistentFlags().Lookup("config")); err != nil {
		return fmt.Errorf("failed to bind config flag: %w", err)
	}
	if err := viper.BindPFlag("verbose", cmd.PersistentFlags().Lookup("verbose")); err != nil {
		return fmt.Errorf("failed to bind verbose flag: %w", err)
	}
	return nil
}
```

改为：

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

- [ ] **步骤 4：运行现有测试验证无回归**

```bash
go test ./cmd/migra/ -v -run TestSetupFlags_BindsViperKeys
```

预期输出：

```
=== RUN   TestSetupFlags_BindsViperKeys
--- PASS: TestSetupFlags_BindsViperKeys (0.00s)
PASS
```

- [ ] **步骤 5：Commit**

```bash
git add cmd/migra/main.go
git commit -m "feat(cli): add --version/-v global flag, change verbose short flag to -V"
```

---

### 任务 4：创建 `cmd/migra/version_test.go`

**文件：**
- 创建：`cmd/migra/version_test.go`

- [ ] **步骤 1：编写测试代码**

```go
package main

import (
	"os"
	"os/exec"
	"strings"
	"testing"
)

func TestVersionFlag_Registered(t *testing.T) {
	// Verify --version is registered as a PersistentFlag
	f := rootCmd.PersistentFlags().Lookup("version")
	if f == nil {
		t.Fatal("--version flag not registered on rootCmd")
	}
	if f.Shorthand != "v" {
		t.Errorf("--version shorthand = %q, want %q", f.Shorthand, "v")
	}
}

func TestVerboseFlag_ShorthandChangedToUppercase(t *testing.T) {
	f := rootCmd.PersistentFlags().Lookup("verbose")
	if f == nil {
		t.Fatal("--verbose flag not registered on rootCmd")
	}
	if f.Shorthand != "V" {
		t.Errorf("--verbose shorthand = %q, want %q", f.Shorthand, "V")
	}
}

func TestVersionCLI_Output(t *testing.T) {
	// Build the binary for integration testing
	// (os.Exit in PersistentPreRun requires subprocess testing)
	cmd := exec.Command("go", "build", "--trimpath", "-ldflags=-s -w",
		"-o", "/tmp/migra-test", "./cmd/migra")
	cmd.Dir = "../../" // adjust if running from cmd/migra/
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("failed to build migra: %v\n%s", err, out)
	}
	defer os.Remove("/tmp/migra-test")

	// Test --version
	cmd = exec.Command("/tmp/migra-test", "--version")
	out, err = cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("--version exited with error: %v\n%s", err, out)
	}
	output := string(out)
	for _, field := range []string{"Version:", "Built:", "Git Commit:", "Go Version:"} {
		if !strings.Contains(output, field) {
			t.Errorf("--version output missing %q, got:\n%s", field, output)
		}
	}

	// Test -v (short flag)
	cmd = exec.Command("/tmp/migra-test", "-v")
	out, err = cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("-v exited with error: %v\n%s", err, out)
	}
	outputShort := string(out)
	if !strings.Contains(outputShort, "Version:") {
		t.Errorf("-v output should contain 'Version:', got:\n%s", outputShort)
	}
}
```

- [ ] **步骤 2：运行测试验证通过**

```bash
go test ./cmd/migra/ -v -run "TestVersionFlag_Registered|TestVerboseFlag_ShorthandChangedToUppercase"
```

预期输出：

```
=== RUN   TestVersionFlag_Registered
--- PASS: TestVersionFlag_Registered (0.00s)
=== RUN   TestVerboseFlag_ShorthandChangedToUppercase
--- PASS: TestVerboseFlag_ShorthandChangedToUppercase (0.00s)
PASS
```

- [ ] **步骤 3：运行 CLI 集成测试（需要构建）**

```bash
go test ./cmd/migra/ -v -run TestVersionCLI_Output
```

预期输出：

```
=== RUN   TestVersionCLI_Output
--- PASS: TestVersionCLI_Output (1.234s)
PASS
```

- [ ] **步骤 4：Commit**

```bash
git add cmd/migra/version_test.go
git commit -m "test(cli): add version flag registration and CLI integration tests"
```

---

### 任务 5：修改 `Makefile`

**文件：**
- 修改：`Makefile`

- [ ] **步骤 1：修改 build 目标**

将 `build` 目标从：

```makefile
build:
	go build --trimpath -ldflags="-s -w" -o migra ./cmd/migra
```

改为：

```makefile
build:
	VERSION=$$(git describe --tags --always --dirty 2>/dev/null || echo "dev"); \
	BUILD_TIME=$$(date -u +%Y-%m-%dT%H:%M:%SZ); \
	COMMIT=$$(git rev-parse --short HEAD 2>/dev/null || echo "unknown"); \
	GO_VERSION=$$(go version | awk '{print $$3}'); \
	@go build --trimpath -ldflags="-s -w \
		-X 'internal/version.Version=$$VERSION' \
		-X 'internal/version.BuildTime=$$BUILD_TIME' \
		-X 'internal/version.GitCommit=$$COMMIT' \
		-X 'internal/version.GoVersion=$$GO_VERSION'" \
		-o migra ./cmd/migra
```

- [ ] **步骤 2：运行 make build 验证**

```bash
make build
```

预期输出（变量定义行可见，go build 行被隐藏）：

```
VERSION=dev; \
BUILD_TIME=2026-05-14T10:00:00Z; \
COMMIT=abc1234; \
GO_VERSION=go1.26.2;
```

- [ ] **步骤 3：验证二进制版本信息**

```bash
./migra --version
```

预期输出：

```
Version:    dev
Built:      2026-05-14T10:00:00Z
Git Commit: abc1234
Go Version: go1.26.2
```

- [ ] **步骤 4：运行全部测试验证无回归**

```bash
go test ./... -v
```

- [ ] **步骤 5：Commit**

```bash
git add Makefile
git commit -m "build(makefile): inject version info via ldflags at build time"
```

---

### 任务 6：最终验证

- [ ] **步骤 1：完整构建并验证**

```bash
make clean && make build && ./migra --version
```

- [ ] **步骤 2：验证 -v 短标志**

```bash
./migra -v
```

- [ ] **步骤 3：验证 -V 是 verbose（不输出版本信息）**

```bash
./migra -V --version  # 应该输出版本信息（--version 优先触发退出）
```

- [ ] **步骤 4：运行全部测试**

```bash
go test ./... -count=1
```

- [ ] **步骤 5：最终 Commit（如果有遗漏）**

```bash
git add -A
git commit -m "feat(version): complete version info injection implementation"
```

---

## 自检

### 1. 规格覆盖度

| 规格需求 | 对应任务 |
|----------|----------|
| 创建 `internal/version` 包 | 任务 1 |
| 版本变量可被 ldflags 覆盖 | 任务 1（默认值）+ 任务 5（ldflags 注入） |
| `Info()` 和 `Short()` 函数 | 任务 1 |
| 单元测试 | 任务 2 |
| verbose 短标志改为 `-V` | 任务 3 步骤 2 |
| `--version`/`-v` 全局 flag | 任务 3 步骤 2 |
| PersistentPreRun 函数组合 | 任务 3 步骤 2 |
| BindPFlag 在 setupFlags() 中 | 任务 3 步骤 3 |
| flag 注册测试 | 任务 4 |
| CLI 集成测试（os/exec） | 任务 4 |
| Makefile ldflags 注入 | 任务 5 |
| release.sh 无需修改 | 已说明（任务 5 不涉及） |

### 2. 占位符扫描

- 无 "待定"、"TODO"、"后续实现"
- 每个代码步骤都有实际代码块
- 无重复任务描述
- 无模糊引用

### 3. 类型一致性

- `version.Info()` 和 `version.Short()` 在任务 1 定义，任务 3 和任务 4 使用方式一致
- `rootCmd.PersistentFlags()` 在任务 3 中注册，任务 4 中查询验证
- Makefile 变量名 `VERSION`/`BUILD_TIME`/`COMMIT`/`GO_VERSION` 与 ldflags `-X` 路径一致

计划已完成并保存到 `docs/superpowers/plans/2026-05-14-version-info.md`。两种执行方式：

**1. 子代理驱动（推荐）** - 每个任务调度一个新的子代理，任务间进行审查，快速迭代

**2. 内联执行** - 在当前会话中使用 executing-plans 执行任务，批量执行并设有检查点

**选哪种方式？**
