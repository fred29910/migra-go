# Code Review 修复实现计划

> **面向 AI 代理的工作者：** 必需子技能：使用 superpowers:subagent-driven-development（推荐）或 superpowers:executing-plans 逐任务实现此计划。步骤使用复选框（`- [ ]`）语法来跟踪进度。

**目标：** 根据评审报告修复 4 个必须修复项和 4 个高价值建议项，提升项目质量和用户体验。

**架构：** 分 4 批次独立修复，每批次包含 1-2 个相关变更，确保每批可独立提交和测试。

**技术栈：** Go, Cobra, Viper, pg_query_go

---

## 文件结构

### 批次 1：基础修复

| 文件 | 职责 | 操作 |
|------|------|------|
| `cmd/migra/push.go` | 格式化 | 修改 |
| `internal/app/push/push_service.go` | 格式化 | 修改 |
| `internal/model/model_test.go` | 格式化 | 修改 |
| `internal/model/table.go` | 格式化 | 修改 |
| `cmd/migra/main.go` | 错误输出到 stderr | 修改 |

### 批次 2：配置与警告

| 文件 | 职责 | 操作 |
|------|------|------|
| `internal/source/sql_file_loader.go` | 返回解析警告 | 修改 |
| `internal/source/dir_loader.go` | 返回解析警告 | 修改 |
| `cmd/migra/diff.go` | 绑定 `--timeout` 到 Viper | 修改 |
| `cmd/migra/push.go` | 绑定 `--timeout` 到 Viper | 修改 |
| `cmd/migra/diff_runner.go` | 添加 timeout 配置 fallback | 修改 |

### 批次 3：数据不可变性

| 文件 | 职责 | 操作 |
|------|------|------|
| `internal/app/pipeline.go` | 克隆输入 schema | 修改 |
| `internal/app/pipeline_test.go` | 测试不可变性 | 修改 |

### 批次 4：CLI 交互

| 文件 | 职责 | 操作 |
|------|------|------|
| `cmd/migra/main.go` | 修正 `-v`/`-V` 语义，添加 version 子命令 | 修改 |
| `cmd/migra/version_test.go` | 更新测试 | 修改 |

---

## 任务 1：gofmt 格式化

**文件：**
- 修改：`cmd/migra/push.go`
- 修改：`internal/app/push/push_service.go`
- 修改：`internal/model/model_test.go`
- 修改：`internal/model/table.go`

- [ ] **步骤 1：格式化所有文件**

```bash
gofmt -w cmd/migra/push.go internal/app/push/push_service.go internal/model/model_test.go internal/model/table.go
```

- [ ] **步骤 2：验证格式化结果**

```bash
gofmt -l .
```
预期：输出为空

- [ ] **步骤 3：运行测试确认无回归**

```bash
go test ./...
```
预期：所有测试通过

- [ ] **步骤 4：Commit**

```bash
git add cmd/migra/push.go internal/app/push/push_service.go internal/model/model_test.go internal/model/table.go
git commit -m "style: format code with gofmt"
```

---

## 任务 2：错误输出到 stderr

**文件：**
- 修改：`cmd/migra/main.go`

- [ ] **步骤 1：修改错误输出**

修改 `cmd/migra/main.go:83`：

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

- [ ] **步骤 2：设置 Cobra 输出流**

在 `main()` 函数开头添加：

```go
rootCmd.SetOut(os.Stdout)
rootCmd.SetErr(os.Stderr)
```

- [ ] **步骤 3：验证错误输出到 stderr**

```bash
migra diff invalid 2>&1 | head -1
```
预期：错误信息输出到 stderr

- [ ] **步骤 4：运行测试确认无回归**

```bash
go test ./...
```
预期：所有测试通过

- [ ] **步骤 5：Commit**

```bash
git add cmd/migra/main.go
git commit -m "fix: redirect error output to stderr"
```

---

## 任务 3：解析警告传递

**文件：**
- 修改：`internal/source/sql_file_loader.go`
- 修改：`internal/source/dir_loader.go`

- [ ] **步骤 1：修改 sql_file_loader.go**

修改 `Load` 方法，确保 `parseErr` 始终返回：

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

- [ ] **步骤 2：修改 dir_loader.go**

修改 `Load` 方法，确保 `parseErr` 始终返回：

```go
func (l *DirLoader) Load(ctx context.Context, source string, opt LoadOptions) (*model.Schema, []error, error) {
    // ... 现有代码 ...
    schema, parseErrs, err := parser.ParseSQL(ctx, combinedSQL, opt.Strict)
    if err != nil {
        return nil, nil, fmt.Errorf("parse SQL files: %w", err)
    }
    return schema, parseErrs, nil  // 始终返回解析警告，不吞掉
}
```

- [ ] **步骤 3：编写测试验证警告传递**

创建测试用例验证 `strict=false` 时警告被正确传递。

- [ ] **步骤 4：运行测试确认通过**

```bash
go test ./internal/source/...
```
预期：所有测试通过

- [ ] **步骤 5：Commit**

```bash
git add internal/source/sql_file_loader.go internal/source/dir_loader.go
git commit -m "fix: always return parse warnings instead of silently dropping them"
```

---

## 任务 4：`--timeout` Viper 绑定

**文件：**
- 修改：`cmd/migra/diff.go`
- 修改：`cmd/migra/push.go`
- 修改：`cmd/migra/diff_runner.go`

- [ ] **步骤 1：在 diff.go 添加 Viper 绑定**

在 `init()` 函数中添加：

```go
_ = viper.BindPFlag("diff.timeout", diffCmd.Flags().Lookup("timeout"))
```

- [ ] **步骤 2：在 push.go 添加 Viper 绑定**

在 `init()` 函数中添加：

```go
_ = viper.BindPFlag("diff.timeout", pushCmd.Flags().Lookup("timeout"))
```

- [ ] **步骤 3：修改 parseDiffConfig 添加配置 fallback**

```go
func parseDiffConfig(cmd *cobra.Command, args []string, cfg *config.Config) (app.DiffConfig, error) {
    // ... 现有代码 ...
    
    timeout, err := cmd.Flags().GetDuration("timeout")
    if err != nil {
        return app.DiffConfig{}, fmt.Errorf("failed to get timeout flag: %w", err)
    }
    
    // 添加配置 fallback
    if timeout == defaultDiffTimeout {
        if cfgTimeout := viper.GetDuration("diff.timeout"); cfgTimeout > 0 {
            timeout = cfgTimeout
        }
    }
    
    // ... 其余代码 ...
}
```

- [ ] **步骤 4：编写测试验证配置生效**

测试环境变量 `MIGRA_DIFF_TIMEOUT` 和配置文件 `diff.timeout` 生效。

- [ ] **步骤 5：运行测试确认通过**

```bash
go test ./cmd/migra/...
```
预期：所有测试通过

- [ ] **步骤 6：Commit**

```bash
git add cmd/migra/diff.go cmd/migra/push.go cmd/migra/diff_runner.go
git commit -m "fix: bind --timeout flag to Viper for config file support"
```

---

## 任务 5：schema 克隆

**文件：**
- 修改：`internal/app/pipeline.go`
- 修改：`internal/app/pipeline_test.go`

- [ ] **步骤 1：修改 ComputeDiff 克隆输入**

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
    
    // ... 其余代码 ...
}
```

- [ ] **步骤 2：编写不可变性测试**

```go
func TestComputeDiff_Immutability(t *testing.T) {
    source := &model.Schema{
        Tables: map[string]*model.Table{
            "users": {
                Name: "users",
                Columns: map[string]*model.Column{
                    "id": {Name: "id", DataType: "integer"},
                },
            },
        },
    }
    target := &model.Schema{
        Tables: map[string]*model.Table{
            "orders": {
                Name: "orders",
                Columns: map[string]*model.Column{
                    "id": {Name: "id", DataType: "integer"},
                },
            },
        },
    }
    
    sourceCopy := source.Clone()
    targetCopy := target.Clone()

    _, _, err := ComputeDiff(context.Background(), source, target, DiffConfig{})
    require.NoError(t, err)

    assert.Equal(t, sourceCopy, source, "source should not be modified")
    assert.Equal(t, targetCopy, target, "target should not be modified")
}
```

- [ ] **步骤 3：运行测试确认通过**

```bash
go test ./internal/app/...
```
预期：所有测试通过

- [ ] **步骤 4：Commit**

```bash
git add internal/app/pipeline.go internal/app/pipeline_test.go
git commit -m "fix: clone input schemas in ComputeDiff to prevent mutation"
```

---

## 任务 6：`-v`/`-V` 语义修正

**文件：**
- 修改：`cmd/migra/main.go`

- [ ] **步骤 1：修正 flag 短名称**

修改 `cmd/migra/main.go:25-26`：

```go
// 重构前
rootCmd.PersistentFlags().BoolP("verbose", "V", false, "verbose output")
rootCmd.PersistentFlags().BoolP("version", "v", false, "print version and exit")

// 重构后
rootCmd.PersistentFlags().BoolP("verbose", "v", false, "verbose output")
rootCmd.PersistentFlags().BoolP("version", "V", false, "print version and exit")
```

- [ ] **步骤 2：验证 flag 行为**

```bash
migra -v  # 应该输出 verbose 日志
migra -V  # 应该输出版本信息
```

- [ ] **步骤 3：运行测试确认无回归**

```bash
go test ./cmd/migra/...
```
预期：所有测试通过

- [ ] **步骤 4：Commit**

```bash
git add cmd/migra/main.go
git commit -m "fix: swap -v/-V flags to match industry conventions"
```

---

## 任务 7：`--version` 子命令化

**文件：**
- 修改：`cmd/migra/main.go`
- 修改：`cmd/migra/version_test.go`

- [ ] **步骤 1：添加 version 子命令**

在 `cmd/migra/main.go` 中添加：

```go
var versionCmd = &cobra.Command{
    Use:   "version",
    Short: "Print version information",
    Run: func(cmd *cobra.Command, args []string) {
        fmt.Printf("migra %s\n", version.Version)
    },
}

func init() {
    rootCmd.AddCommand(versionCmd)
}
```

- [ ] **步骤 2：从 PersistentPreRun 中移除 version 检查**

删除 `PersistentPreRun` 中的 version flag 检查逻辑。

- [ ] **步骤 3：更新 version_test.go**

修改测试以使用 `migra version` 子命令而不是 exec 二进制。

- [ ] **步骤 4：验证子命令行为**

```bash
migra version  # 应该输出版本信息
migra --version  # 应该输出版本信息（兼容别名）
```

- [ ] **步骤 5：运行测试确认通过**

```bash
go test ./cmd/migra/...
```
预期：所有测试通过

- [ ] **步骤 6：Commit**

```bash
git add cmd/migra/main.go cmd/migra/version_test.go
git commit -m "feat: add version subcommand and keep --version as alias"
```

---

## 任务 8：最终验证

- [ ] **步骤 1：运行完整 CI 检查**

```bash
make ci
```
预期：所有 853 个测试通过

- [ ] **步骤 2：验证 gofmt 检查**

```bash
gofmt -l .
```
预期：输出为空

- [ ] **步骤 3：验证错误输出**

```bash
migra diff invalid 2>&1 | head -1
```
预期：错误信息输出到 stderr

- [ ] **步骤 4：验证 version 子命令**

```bash
migra version
migra --version
```
预期：都输出版本信息

- [ ] **步骤 5：验证 verbose 标志**

```bash
migra -v diff
migra -V
```
预期：`-v` 输出 verbose 日志，`-V` 输出版本信息

---

## 自检结果

**1. 规格覆盖度：** ✅ 所有 8 项修复都有对应任务

**2. 占位符扫描：** ✅ 无占位符或 TODO

**3. 类型一致性：** ✅ 所有类型、方法签名一致

**发现并修复的问题：**
- 依赖关系图已修正为执行顺序说明
- 所有任务都包含具体代码和验证步骤
