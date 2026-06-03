package main

import (
	"bufio"
	"context"
	"fmt"
	"os"
	"os/signal"
	"regexp"
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

var stdinReader = bufio.NewReader(os.Stdin)

func readUserInput() string {
	input, err := stdinReader.ReadString('\n')
	if err != nil {
		return "n"
	}
	return strings.ToLower(strings.TrimSpace(input))
}

func execSQL(tx pgx.Tx, ctx context.Context, sql string, idx int) error {
	_, err := tx.Exec(ctx, sql)
	if err != nil {
		return fmt.Errorf("error executing SQL #%d: %w", idx, err)
	}
	fmt.Printf("SQL #%d executed\n", idx)
	return nil
}

func isDestructiveOperation(isDestructive, unsafeDrop bool) bool {
	return isDestructive && !unsafeDrop
}

var nonTransactionalPatterns = []*regexp.Regexp{
	regexp.MustCompile(`(?i)\bCREATE\s+INDEX\s+CONCURRENTLY\b`),
	regexp.MustCompile(`(?i)\bDROP\s+INDEX\s+CONCURRENTLY\b`),
	regexp.MustCompile(`(?i)\bREINDEX\s+INDEX\s+CONCURRENTLY\b`),
}

func stripSQLComments(sql string) string {
	lines := strings.Split(sql, "\n")
	for i, line := range lines {
		if idx := strings.Index(line, "--"); idx >= 0 {
			lines[i] = line[:idx]
		}
	}
	return strings.Join(lines, "\n")
}

func isNonTransactionalSQL(sql string) bool {
	cleaned := stripSQLComments(sql)
	for _, pattern := range nonTransactionalPatterns {
		if pattern.MatchString(cleaned) {
			return true
		}
	}
	return false
}

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

	lower := strings.ToLower(cfg.Target)
	if !strings.HasPrefix(lower, "postgres://") &&
		!strings.HasPrefix(lower, "postgresql://") &&
		!strings.HasPrefix(lower, "pg://") {
		return fmt.Errorf("target must be a database connection string (postgres://...)")
	}

	// Schema loading phase: uses the configured timeout to avoid hanging on slow connections.
	loadCtx, loadCancel := context.WithTimeout(cmd.Context(), cfg.Timeout)
	defer loadCancel()

	sourceSchema, err := loadSchemaWithContext(loadCtx, cfg.Source, cfg.Schemas, false)
	if err != nil {
		return fmt.Errorf("failed to load source schema: %w", err)
	}

	targetSchema, err := loadSchemaWithContext(loadCtx, cfg.Target, cfg.Schemas, false)
	if err != nil {
		return fmt.Errorf("failed to load target schema: %w", err)
	}

	appCfg := app.Config{
		Source:     cfg.Source,
		Target:     cfg.Target,
		Schemas:    cfg.Schemas,
		Format:     "sql",
		UnsafeDrop: true, // Show all ops (including destructive) in preview
		Timeout:    cfg.Timeout,
	}
	// ComputeDiff(source, target): returns ops needed to apply to target to match source
	ops, _, err := app.ComputeDiff(targetSchema, sourceSchema, appCfg)
	if err != nil {
		return err
	}

	// Warn about destructive ops when user hasn't opted in
	if !cfg.UnsafeDrop {
		var destructiveOps []diff.Operation
		for _, op := range ops {
			if op.IsDestructive() {
				destructiveOps = append(destructiveOps, op)
			}
		}
		if len(destructiveOps) > 0 {
			fmt.Fprintf(os.Stderr, "Warning: %d destructive operation(s) detected!\n", len(destructiveOps))
			for _, op := range destructiveOps {
				fmt.Fprintf(os.Stderr, "  - %s: %s (destructive)\n", op.Kind(), op.ObjectKey())
			}
			fmt.Fprintf(os.Stderr, "Use --unsafe-drop to include destructive DROP operations\n\n")
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
		fmt.Println("Dry-run mode. Use --execute to apply changes.")
		return nil
	}

	// Execution phase: uses cmd.Context() directly (no timeout) so interactive
	// confirmation is not interrupted by the schema-loading timeout.
	return executeWithConfirmation(cmd.Context(), cfg, sourceSchema, ops, renderer)
}

func executeWithConfirmation(ctx context.Context, cfg pushConfig, sourceSchema *model.Schema, ops []diff.Operation, renderer *render.Renderer) error {
	conn, err := pgx.Connect(ctx, cfg.Target)
	if err != nil {
		return fmt.Errorf("failed to connect to target database: %w", err)
	}
	defer func() { _ = conn.Close(ctx) }()

	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)

	autoMode := cfg.Execute

	tx, err := conn.Begin(ctx)
	if err != nil {
		return fmt.Errorf("failed to begin transaction: %w", err)
	}
	txActive := true
	defer func() {
		if txActive {
			_ = tx.Rollback(ctx)
		}
	}()

	go func() {
		<-sigChan
		fmt.Println("\nInterrupt received, rolling back...")
		_ = tx.Rollback(ctx)
		os.Exit(1)
	}()

next:
	for i, op := range ops {
		sql := renderer.RenderSingle(op)
		if sql == "" || strings.HasPrefix(sql, "-- Unknown") {
			continue
		}

		isDestructive := op.IsDestructive()

		if isNonTransactionalSQL(sql) {
			fmt.Printf("\nNon-transactional DDL detected at SQL #%d:\n  %s\nThis operation cannot be executed within a transaction.\nPlease execute it separately outside this tool.\n", i+1, sql)
			_ = tx.Rollback(ctx)
			txActive = false
			return fmt.Errorf("non-transactional DDL at SQL #%d", i+1)
		}

		if autoMode {
			if isDestructive && !cfg.UnsafeDrop {
				fmt.Printf("SQL #%d skipped (destructive) \u2014 use --unsafe-drop to execute\n", i+1)
				continue next
			}
			if err := execSQL(tx, ctx, sql, i+1); err != nil {
				fmt.Println("Rolling back transaction...")
				_ = tx.Rollback(ctx)
				txActive = false
				return fmt.Errorf("execution failed at SQL #%d, transaction rolled back: %w", i+1, err)
			}
			continue next
		}

		prompt := fmt.Sprintf("Execute SQL #%d? (y=yes, n=no, a=apply all, s=skip): ", i+1)
		if isDestructive && !autoMode {
			prompt = fmt.Sprintf("DANGER: Execute SQL #%d? (y=yes, n=no, a=apply all, s=skip): ", i+1)
		}

		fmt.Print(prompt)

		input := readUserInput()

		switch input {
		case "y":
			if isDestructiveOperation(isDestructive, cfg.UnsafeDrop) && !autoMode {
				fmt.Println("Destructive operation requires --unsafe-drop or explicit confirmation")
				continue
			}
			if err := execSQL(tx, ctx, sql, i+1); err != nil {
				fmt.Println("Rolling back transaction...")
				_ = tx.Rollback(ctx)
				txActive = false
				return fmt.Errorf("execution failed at SQL #%d, transaction rolled back: %w", i+1, err)
			}
			continue next
		case "n":
			fmt.Println("Cancelled")
			_ = tx.Rollback(ctx)
			txActive = false
			return nil
		case "a":
			if isDestructiveOperation(isDestructive, cfg.UnsafeDrop) {
				fmt.Printf("Destructive operation detected in auto-mode, reverting to interactive mode.\nSQL #%d: %s\n\n", i+1, sql)
				autoMode = false
				continue
			}
			autoMode = true
			continue next
		case "s":
			fmt.Printf("SQL #%d skipped\n", i+1)
			continue next
		default:
			fmt.Println("Invalid input. Use: y, n, a, or s")
		}
	}

	if err := tx.Commit(ctx); err != nil {
		return fmt.Errorf("failed to commit transaction: %w", err)
	}
	txActive = false
	fmt.Println("\nAll SQL executed successfully, transaction committed")

	if !cfg.NoVerify {
		fmt.Println("\n=== Post-execution validation ===")
		newTargetSchema, err := loadSchemaWithContext(ctx, cfg.Target, cfg.Schemas, false)
		if err != nil {
			fmt.Printf("Warning: failed to load target schema for verification: %v\n", err)
		} else {
			// ComputeDiff(source, target): returns remaining ops if target doesn't yet match source
			remainOps, _, err := app.ComputeDiff(newTargetSchema, sourceSchema, app.Config{
				UnsafeDrop: true, // Check all ops including destructive for honest validation
			})
			if err != nil {
				fmt.Printf("Warning: failed to compute validation diff: %v\n", err)
			} else if len(remainOps) > 0 {
				fmt.Printf("Warning: %d operations still pending after migration:\n", len(remainOps))
				for _, op := range remainOps {
					fmt.Printf("  - %s: %s\n", op.Kind(), op.ObjectKey())
				}
			} else {
				fmt.Println("Validation passed: target schema matches expected state")
			}
		}
	}

	return nil
}
