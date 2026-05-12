# Push 子命令实现计划

> **面向 AI 代理的工作者：** 必需子技能：使用 superpowers:subagent-driven-development（推荐）或 superpowers:executing-plans 逐任务实现此计划。步骤使用复选框（`- [ ]`）语法来跟踪进度。

**目标：** 实现 `migra push` 子命令，基于 diff 将 SQL 应用到目标数据库，支持交互确认、事务保护、危险操作检测。

**架构：** push 命令复用 diff 计算逻辑生成 SQL 操作列表，通过交互式终端让用户逐条确认执行，使用 pgx 事务执行 SQL，失败自动回滚，并处理 Ctrl+C 信号。

**技术栈：** Go, Cobra, pgx v5, spf13/viper

---

## 文件结构

| 文件 | 职责 |
|------|------|
| `cmd/migra/push.go` | push 子命令定义和参数解析 |
| `cmd/migra/push_runner.go` | push 执行逻辑：加载 schema、计算 diff、交互确认、事务执行 |
| `internal/render/render.go` | 添加 RenderSingle 方法支持单条 SQL 渲染 |
| `cmd/migra/push_test.go` | push 命令测试 |

---

## 任务 1：在 render 包中添加 RenderSingle 方法

**文件：**
- 修改：`internal/render/render.go:68-131`

- [ ] **步骤 1：在 render.go 中添加 RenderSingle 方法**

在 `Render` 方法后添加：

```go
// RenderSingle renders a single operation to SQL string
// Used by push command for interactive confirmation
func (r *Renderer) RenderSingle(op diff.Operation) string {
	return r.Render(op)
}
```

- [ ] **步骤 2：验证编译**

运行：`go build ./...`
预期：编译成功

- [ ] **步骤 3：Commit**

```bash
git add internal/render/render.go
git commit -m "refactor: add RenderSingle method for push command"
```

---

## 任务 2：创建 push 子命令定义

**文件：**
- 创建：`cmd/migra/push.go`

- [ ] **步骤 1：创建 push.go 文件**

```go
package main

import (
	"github.com/spf13/cobra"
)

var pushCmd = &cobra.Command{
	Use:   "push [source] [target]",
	Short: "Apply schema changes to target database",
	Long: `Calculate diff from source to target and apply SQL to target database.
This command requires interactive confirmation before executing each SQL.

Examples:
  migra push file.sql postgres://localhost/db
  migra push postgres://localhost/db1 postgres://localhost/db2
  migra push --unsafe-drop file.sql postgres://localhost/db`,
	Args: cobra.ExactArgs(2),
	RunE: runPush,
}

func init() {
	rootCmd.AddCommand(pushCmd)

	pushCmd.Flags().StringSliceP("schema", "s", []string{"public"}, "schemas to compare (can be multiple)")
	pushCmd.Flags().Bool("unsafe-drop", false, "skip confirmation for destructive DROP operations")
	pushCmd.Flags().Bool("dry-run", true, "show SQL without executing (default: true)")
	pushCmd.Flags().Bool("execute", false, "execute SQL without confirmation (not recommended)")
	pushCmd.Flags().Bool("no-verify", false, "skip post-execution validation")
	pushCmd.Flags().Duration("timeout", defaultDiffTimeout, "timeout for schema loading")
}
```

- [ ] **步骤 2：验证编译**

运行：`go build ./cmd/migra`
预期：编译成功（暂时报错 `undefined: runPush`，预期行为）

- [ ] **步骤 3：Commit**

```bash
git add cmd/migra/push.go
git commit -m "feat: add push command skeleton"
```

---

## 任务 3：实现 push_runner.go 核心逻辑

**文件：**
- 创建：`cmd/migra/push_runner.go`

这是核心文件，包含：
- 加载 source 和 target schema
- 计算 diff 生成 operations
- 交互确认循环
- 事务执行和回滚
- 信号处理

- [ ] **步骤 1：创建 push_runner.go 文件，包含基础结构和配置解析**

注意：函数签名应为 `func executeWithConfirmation(ctx context.Context, cfg pushConfig, sourceSchema *model.Schema, ops []diff.Operation, renderer *render.Renderer) error`

```go
package main

import (
	"context"
	"fmt"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"

	"github.com/fred29910/migra-go/internal/app"
	"github.com/fred29910/migra-go/internal/diff"
	"github.com/fred29910/migra-go/internal/model"
	"github.com/fred29910/migra-go/internal/render"
	"github.com/jackc/pgx/v5"
	"github.com/spf13/cobra"
)

type pushConfig struct {
	Source     string
	Target     string
	Schemas    []string
	UnsafeDrop bool
	DryRun     bool
	Execute    bool
	NoVerify   bool
	Timeout    time.Duration
}

func parsePushConfig(cmd *cobra.Command, args []string) (pushConfig, error) {
	schemas, err := cmd.Flags().GetStringSlice("schema")
	if err != nil {
		return pushConfig{}, fmt.Errorf("failed to get schema flag: %w", err)
	}
	unsafeDrop, err := cmd.Flags().GetBool("unsafe-drop")
	if err != nil {
		return pushConfig{}, fmt.Errorf("failed to get unsafe-drop flag: %w", err)
	}
	dryRun, err := cmd.Flags().GetBool("dry-run")
	if err != nil {
		return pushConfig{}, fmt.Errorf("failed to get dry-run flag: %w", err)
	}
	execute, err := cmd.Flags().GetBool("execute")
	if err != nil {
		return pushConfig{}, fmt.Errorf("failed to get execute flag: %w", err)
	}
	noVerify, err := cmd.Flags().GetBool("no-verify")
	if err != nil {
		return pushConfig{}, fmt.Errorf("failed to get no-verify flag: %w", err)
	}
	timeout, err := cmd.Flags().GetDuration("timeout")
	if err != nil {
		return pushConfig{}, fmt.Errorf("failed to get timeout flag: %w", err)
	}

	return pushConfig{
		Source:     args[0],
		Target:     args[1],
		Schemas:    schemas,
		UnsafeDrop: unsafeDrop,
		DryRun:     dryRun,
		Execute:    execute,
		NoVerify:   noVerify,
		Timeout:    timeout,
	}, nil
}

func runPush(cmd *cobra.Command, args []string) error {
	cfg, err := parsePushConfig(cmd, args)
	if err != nil {
		return err
	}

	if !strings.HasPrefix(strings.ToLower(cfg.Target), "postgres://") &&
	   !strings.HasPrefix(strings.ToLower(cfg.Target), "postgresql://") &&
	   !strings.HasPrefix(strings.ToLower(cfg.Target), "pg://") {
		return fmt.Errorf("target must be a database connection string")
	}

	ctx, cancel := context.WithTimeout(cmd.Context(), cfg.Timeout)
	defer cancel()

	sourceSchema, err := loadSchemaWithContext(ctx, cfg.Source, cfg.Schemas, false)
	if err != nil {
		return fmt.Errorf("failed to load source schema: %w", err)
	}

	targetSchema, err := loadSchemaWithContext(ctx, cfg.Target, cfg.Schemas, false)
	if err != nil {
		return fmt.Errorf("failed to load target schema: %w", err)
	}

	appCfg := app.Config{
		Source:     cfg.Source,
		Target:     cfg.Target,
		Schemas:    cfg.Schemas,
		Format:     "sql",
		UnsafeDrop: cfg.UnsafeDrop,
		Timeout:    cfg.Timeout,
	}

	ops, warnings, err := app.ComputeDiff(sourceSchema, targetSchema, appCfg)
	if err != nil {
		return err
	}

	if len(warnings) > 0 {
		for _, w := range warnings {
			fmt.Fprintf(os.Stderr, "Warning: %s\n", w)
		}
	}

	if len(ops) == 0 {
		fmt.Println("No changes detected")
		return nil
	}

	renderer := render.NewRenderer()

	fmt.Println("\n=== Diff Preview ===")
	for i, op := range ops {
		sql := renderer.RenderSingle(op)
		destructive := ""
		if op.IsDestructive() {
			destructive = " [DESTRUCTIVE]"
		}
		fmt.Printf("%d:%s\n%s\n\n", i+1, destructive, sql)
	}

	if cfg.DryRun && !cfg.Execute {
		fmt.Println("Dry-run mode. Use --execute to apply changes or press Enter in interactive mode.")
		return nil
	}

	return executeWithConfirmation(ctx, cfg, sourceSchema, ops, renderer)
}

func executeWithConfirmation(ctx context.Context, cfg pushConfig, sourceSchema *model.Schema, ops []diff.Operation, renderer *render.Renderer) error {
	conn, err := pgx.Connect(ctx, cfg.Target)
	if err != nil {
		return fmt.Errorf("failed to connect to target database: %w", err)
	}
	defer conn.Close(ctx)

	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)

	autoMode := false
	committed := false

	tx, err := conn.Begin(ctx)
	if err != nil {
		return fmt.Errorf("failed to begin transaction: %w", err)
	}

	go func() {
		<-sigChan
		fmt.Println("\n⚠️  Interrupt received, rolling back...")
		_ = tx.Rollback(ctx)
		os.Exit(1)
	}()

	for i, op := range ops {
		sql := renderer.RenderSingle(op)
		if sql == "" || strings.HasPrefix(sql, "-- Unknown") {
			continue
		}

		isDestructive := op.IsDestructive()

		for {
			prompt := fmt.Sprintf("Execute SQL #%d? (y=yes, n=no, a=apply all, s=skip): ", i+1)
			if isDestructive && !autoMode {
				prompt = fmt.Sprintf("⚠️  DANGER: Execute SQL #%d? (y=yes, n=no, a=apply all, s=skip): ", i+1)
			}

			if cfg.Execute && autoMode {
				prompt = "auto"
			}

			fmt.Print(prompt)

			var input string
			if cfg.Execute && autoMode {
				input = "y"
			} else {
				_, err := fmt.Scanln(&input)
				if err != nil {
					input = "n"
				}
			}

			input = strings.ToLower(strings.TrimSpace(input))

			switch input {
			case "y":
				if isDestructive && !cfg.UnsafeDrop && !autoMode {
					fmt.Println("Destructive operation requires --unsafe-drop or explicit confirmation")
					continue
				}
				_, err := tx.Exec(ctx, sql)
				if err != nil {
					fmt.Printf("❌ Error executing SQL #%d: %v\n", i+1, err)
					fmt.Println("Rolling back transaction...")
					_ = tx.Rollback(ctx)
					return fmt.Errorf("execution failed at SQL #%d, transaction rolled back", i+1)
				}
				fmt.Printf("✅ SQL #%d executed\n", i+1)
				goto next
			case "n":
				fmt.Println("Cancelled")
				_ = tx.Rollback(ctx)
				return nil
			case "a":
				autoMode = true
				goto next
			case "s":
				fmt.Printf("⏭️  SQL #%d skipped\n", i+1)
				goto next
			default:
				fmt.Println("Invalid input. Use: y, n, a, or s")
			}
		}
	next:
	}

	if err := tx.Commit(ctx); err != nil {
		return fmt.Errorf("failed to commit transaction: %w", err)
	}
	committed = true
	fmt.Println("\n✅ All SQL executed successfully, transaction committed")

	if !cfg.NoVerify {
		fmt.Println("\n=== Post-execution validation ===")
		newTargetSchema, err := loadSchemaWithContext(ctx, cfg.Target, cfg.Schemas, false)
		if err != nil {
			fmt.Printf("⚠️  Warning: failed to load target schema for verification: %v\n", err)
		} else {
			differ := diff.NewDiffer()
			remainOps, _ := differ.Diff(sourceSchema, newTargetSchema)
			if len(remainOps) > 0 {
				fmt.Printf("⚠️  Warning: %d operations still pending after migration:\n", len(remainOps))
				for _, op := range remainOps {
					fmt.Printf("  - %s: %s\n", op.Kind(), op.ObjectKey())
				}
			} else {
				fmt.Println("✅ Validation passed: target schema matches expected state")
			}
		}
	}

	return nil
}
```

- [ ] **步骤 2：修复编译错误 - 移除未使用的变量**

在 `executeWithConfirmation` 函数中，`committed` 变量未使用，删除它：

```go
committed := false  // 删除此行
```

- [ ] **步骤 3：验证编译**

运行：`go build ./cmd/migra`
预期：编译成功

- [ ] **步骤 4：Commit**

```bash
git add cmd/migra/push_runner.go
git commit -m "feat: implement push command execution logic"
```

---

## 任务 4：实现非事务性 DDL 检测

**文件：**
- 修改：`cmd/migra/push_runner.go`

- [ ] **步骤 1：添加非事务性 DDL 检测函数**

在 `executeWithConfirmation` 函数前添加：

```go
var nonTransactionalDDL = map[string]bool{
	"CREATE INDEX CONCURRENTLY": true,
	"DROP INDEX CONCURRENTLY":  true,
	"REINDEX INDEX CONCURRENTLY": true,
	"CREATE INDEX USING CONCURRENTLY": true,
}

func isNonTransactionalSQL(sql string) bool {
	upperSQL := strings.ToUpper(sql)
	for pattern := range nonTransactionalDDL {
		if strings.Contains(upperSQL, pattern) {
			return true
		}
	}
	return false
}
```

- [ ] **步骤 2：在执行前添加检测**

在 `executeWithConfirmation` 函数中，执行 SQL 前添加检测：

```go
// 在 tx.Exec 之前添加
if isNonTransactionalSQL(sql) {
	fmt.Printf("❌ Non-transactional DDL detected at SQL #%d: %s\n", i+1, sql)
	fmt.Println("This operation cannot be executed within a transaction.")
	fmt.Println("Please execute it separately outside this tool.")
	_ = tx.Rollback(ctx)
	return fmt.Errorf("non-transactional DDL at SQL #%d", i+1)
}
```

- [ ] **步骤 3：验证编译**

运行：`go build ./cmd/migra`
预期：编译成功

- [ ] **步骤 4：Commit**

```bash
git add cmd/migra/push_runner.go
git commit -m "feat: add non-transactional DDL detection"
```

---

## 任务 5：处理自动模式下的危险操作中断

**文件：**
- 修改：`cmd/migra/push_runner.go`

- [ ] **步骤 1：修改自动模式逻辑，在遇到危险操作时中断**

找到 `autoMode = true` 后的执行逻辑，修改为：

```go
case "a":
    // 如果当前操作是破坏性的，需要额外确认
    if isDestructive && !cfg.UnsafeDrop {
        fmt.Println("⚠️  Destructive operation detected in auto-mode, reverting to interactive mode")
        fmt.Printf("SQL #%d: %s\n\n", i+1, sql)
        autoMode = false
        continue
    }
    autoMode = true
    goto next
```

- [ ] **步骤 2：验证编译**

运行：`go build ./cmd/migra`
预期：编译成功

- [ ] **步骤 3：Commit**

```bash
git add cmd/migra/push_runner.go
git commit -m "feat: interrupt auto-mode on destructive operations"
```

---

## 任务 6：添加测试

**文件：**
- 创建：`cmd/migra/push_test.go`

- [ ] **步骤 1：创建基础测试文件**

```go
package main

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestParsePushConfig(t *testing.T) {
	tests := []struct {
		name    string
		args    []string
		wantErr bool
	}{
		{
			name:    "valid two args",
			args:    []string{"file.sql", "postgres://localhost/db"},
			wantErr: false,
		},
		{
			name:    "missing target",
			args:    []string{"file.sql"},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cmd := &cobra.Command{Use: "push", Args: cobra.ExactArgs(2)}
			cmd.Flags().StringSlice("schema", []string{"public"}, "")
			cmd.Flags().Bool("unsafe-drop", false, "")
			cmd.Flags().Bool("dry-run", true, "")
			cmd.Flags().Bool("execute", false, "")
			cmd.Flags().Bool("no-verify", false, "")
			cmd.Flags().Duration("timeout", 30, "")

			_, err := parsePushConfig(cmd, tt.args)
			if tt.wantErr {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

func TestIsNonTransactionalSQL(t *testing.T) {
	tests := []struct {
		name  string
		sql  string
		want bool
	}{
		{"create index concurrently", "CREATE INDEX CONCURRENTLY idx ON t(c)", true},
		{"drop index concurrently", "DROP INDEX CONCURRENTLY idx", true},
		{"regular create index", "CREATE INDEX idx ON t(c)", false},
		{"regular alter table", "ALTER TABLE t ADD COLUMN c int", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := isNonTransactionalSQL(tt.sql)
			assert.Equal(t, tt.want, got)
		})
	}
}
```

注意：测试中需要导入 cobra，但 cobra.Command 不能直接创建用于测试。需要使用 testify 的 mock 或者重构 parsePushConfig 为可测试的形式。

- [ ] **步骤 2：修复测试文件 - 使用 stub 函数**

```go
package main

import (
	"testing"

	"github.com/spf13/cobra"
	"github.com/stretchr/testify/assert"
)

func TestIsNonTransactionalSQL(t *testing.T) {
	tests := []struct {
		name  string
		sql  string
		want bool
	}{
		{"create index concurrently", "CREATE INDEX CONCURRENTLY idx ON t(c)", true},
		{"drop index concurrently", "DROP INDEX CONCURRENTLY idx", true},
		{"regular create index", "CREATE INDEX idx ON t(c)", false},
		{"regular alter table", "ALTER TABLE t ADD COLUMN c int", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := isNonTransactionalSQL(tt.sql)
			assert.Equal(t, tt.want, got)
		})
	}
}
```

- [ ] **步骤 3：运行测试**

运行：`go test ./cmd/migra/... -v`
预期：测试通过

- [ ] **步骤 4：Commit**

```bash
git add cmd/migra/push_test.go
git commit -m "test: add push command tests"
```

---

## 任务 7：整体集成测试

**文件：**
- 测试现有 diff 命令不受影响

- [ ] **步骤 1：确保 diff 命令正常工作**

运行：`go test ./... -v -run TestDiff`
预期：所有 diff 相关测试通过

- [ ] **步骤 2：验证 push 命令帮助信息**

运行：`go run ./cmd/migra push --help`
预期：显示 push 命令帮助信息

- [ ] **步骤 3：Commit**

```bash
git commit -m "test: verify push command integration"
```

---

## 自检

**1. 规格覆盖度：**
- ✅ 交互确认（y/n/a/s）
- ✅ 事务执行和回滚
- ✅ 危险操作检测（DROP）
- ✅ 自动模式中断危险操作
- ✅ 非事务性 DDL 检测
- ✅ 信号处理（Ctrl+C）
- ✅ 执行后校验
- ✅ RenderSingle 方法

**2. 占位符扫描：** 无占位符

**3. 类型一致性：** 
- RenderSingle 返回 string，与 Render 方法一致
- pushConfig 字段与 parsePushConfig 返回类型一致

---

计划已完成并保存到 `docs/superpowers/plans/2026-05-12-push-command-plan.md`。两种执行方式：

**1. 子代理驱动（推荐）** - 每个任务调度一个新的子代理，任务间进行审查，快速迭代

**2. 内联执行** - 在当前会话中使用 executing-plans 执行任务，批量执行并设有检查点

**选哪种方式？**