# Code Review 修复设计文档

> 日期：2025-06-17  
> 基于：docs/reviews/comprehensive-code-review.md  
> 范围：必须修复 + 高价值建议（方案 A：保守渐进）

## 背景

`migra-go` 项目经过全面代码审查，识别出 12 项问题。本文档设计分 4 批次修复，每批次独立可验证。

## 修复清单

### 批次 1：基础修复（极低风险）

#### 1.1 gofmt 格式化

**问题**：4 个文件未通过 `gofmt` 检查。

**文件**：
- `cmd/migra/push.go`
- `internal/app/push/push_service.go`
- `internal/model/model_test.go`
- `internal/model/table.go`

**修复**：运行 `gofmt -w` 格式化。

**验证**：`gofmt -l .` 输出为空。

---

#### 1.2 错误输出到 stderr

**问题**：`cmd/migra/main.go:83` 将错误输出到 stdout，破坏 Unix 管道。

**当前代码**：
```go
if err := rootCmd.Execute(); err != nil {
    fmt.Println(err)
    os.Exit(1)
}
```

**修复后**：
```go
if err := rootCmd.Execute(); err != nil {
    fmt.Fprintln(os.Stderr, err)
    os.Exit(1)
}
```

**额外修复**：设置 Cobra 输出流：
```go
rootCmd.SetOut(os.Stdout)
rootCmd.SetErr(os.Stderr)
```

**验证**：`migra diff invalid 2>&1 | head -1` 输出到 stderr。

---

### 批次 2：配置与警告（低风险）

#### 2.1 解析警告传递

**问题**：`internal/source/sql_file_loader.go:39-43` 和 `dir_loader.go:88-93` 在 `strict=false` 时静默丢弃解析警告。

**修复**：确保 `parseErr` 始终返回给调用方，不再静默丢弃。

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

**验证**：编写测试验证 `strict=false` 时警告被正确传递。

---

#### 2.2 `--timeout` Viper 绑定

**问题**：`diff.go:50` 和 `push.go:40` 定义了 `--timeout`，但都没有 `viper.BindPFlag`。

**修复**：在 `cmd/migra/diff.go` 和 `push.go` 的 `init()` 中添加：
```go
_ = viper.BindPFlag("diff.timeout", diffCmd.Flags().Lookup("timeout"))
```

修改 `parseDiffConfig()` 添加配置 fallback：
```go
if timeout == defaultDiffTimeout {
    if cfgTimeout := viper.GetDuration("diff.timeout"); cfgTimeout > 0 {
        timeout = cfgTimeout
    }
}
```

**验证**：编写测试验证环境变量 `MIGRA_DIFF_TIMEOUT` 和配置文件 `diff.timeout` 生效。

---

### 批次 3：数据不可变性（中风险）

#### 3.1 schema 克隆

**问题**：`ComputeDiff` 会原地修改输入的 schema，导致同一组 schema 不能安全地多次调用。

**修复**：修改 `internal/app/pipeline.go` 中的 `ComputeDiff`，在开始时克隆输入 schema：
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

**验证**：添加单元测试验证 `ComputeDiff` 不修改原始输入：
```go
func TestComputeDiff_Immutability(t *testing.T) {
    source := &model.Schema{...}
    target := &model.Schema{...}
    sourceCopy := source.Clone()
    targetCopy := target.Clone()

    _, _, err := ComputeDiff(context.Background(), source, target, DiffConfig{})
    require.NoError(t, err)

    assert.Equal(t, sourceCopy, source)
    assert.Equal(t, targetCopy, target)
}
```

---

### 批次 4：CLI 交互（中风险）

#### 4.1 `-v`/`-V` 语义修正

**问题**：当前 `-v` 是 `--version`，`-V` 是 `--verbose`，与业界惯例相反。

**修复**：修改 `cmd/migra/main.go:25-26`：
```go
// 重构前
rootCmd.PersistentFlags().BoolP("verbose", "V", false, "verbose output")
rootCmd.PersistentFlags().BoolP("version", "v", false, "print version and exit")

// 重构后
rootCmd.PersistentFlags().BoolP("verbose", "v", false, "verbose output")
rootCmd.PersistentFlags().BoolP("version", "V", false, "print version and exit")
```

**验证**：`migra -v` 输出 verbose 日志，`migra -V` 输出版本信息。

---

#### 4.2 `--version` 子命令化

**问题**：`--version` 通过 `PersistentPreRun` + `os.Exit(0)` 处理，无法参与 Cobra 测试流程。

**修复**：
1. 将 `--version` 从 `PersistentPreRun` 中的 flag 检测改为独立子命令
2. 保留 `--version` 作为兼容别名

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

**验证**：`migra version` 和 `migra --version` 都能正常工作。

---

## 执行顺序

```
批次 1 → 批次 2 → 批次 3 → 批次 4
```

每个批次可以独立提交和测试，无交叉依赖。按顺序执行是为了保持 git 历史清晰，便于回滚。

## 验证策略

每批次完成后运行：
```bash
make ci
```

确保所有 853 个测试用例通过。

## 风险评估

| 批次 | 风险 | 回滚难度 |
|------|------|----------|
| 1 | 极低 | 简单 |
| 2 | 低 | 简单 |
| 3 | 中 | 中等 |
| 4 | 中 | 中等 |

## 成功标准

1. 所有 `gofmt` 检查通过
2. 错误输出到 stderr 而非 stdout
3. 解析警告在 `strict=false` 时正确传递
4. `--timeout` 支持配置文件和环境变量
5. `ComputeDiff` 不修改输入 schema
6. `-v`/`-V` 语义符合业界惯例
7. `migra version` 子命令正常工作
8. 所有测试通过（853 个）
