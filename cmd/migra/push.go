package main

import (
	"context"
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/fred29910/migra-go/internal/app"
	"github.com/fred29910/migra-go/internal/diff"
	"github.com/fred29910/migra-go/internal/render"
	"github.com/fred29910/migra-go/internal/app/push"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
)

var pushCmd = &cobra.Command{
	Use:   "push [source] [target]",
	Short: "Apply schema changes to target database",
	Long: `Compare source and target schemas, then apply SQL to make the target database match the source schema.
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
	pushCmd.Flags().Bool("dry-run", false, "show SQL without executing (default: false)")
	pushCmd.Flags().Bool("execute", false, "execute SQL without confirmation (not recommended)")
	pushCmd.Flags().Bool("no-verify", false, "skip post-execution validation")
	pushCmd.Flags().Duration("timeout", defaultDiffTimeout, "timeout for schema loading")

	_ = viper.BindPFlag("diff.schemas", pushCmd.Flags().Lookup("schema"))
	_ = viper.BindPFlag("diff.unsafe_drop", pushCmd.Flags().Lookup("unsafe-drop"))
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
		UnsafeDrop: cfg.UnsafeDrop,
		Timeout:    cfg.Timeout,
		Execute:    cfg.Execute,
		NoVerify:   cfg.NoVerify,
	}
	ops, _, err := app.ComputeDiff(targetSchema, sourceSchema, appCfg)
	if err != nil {
		return err
	}

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
		sql := renderer.Render(op)
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

	pushService := push.NewPushService()
	if err := pushService.ExecutePlan(cmd.Context(), appCfg, sourceSchema, ops); err != nil {
		return err
	}

	// Post-execution validation (unless disabled via --no-verify).
	if !cfg.NoVerify {
		fmt.Println("\n=== Post-execution validation ===")
		newTargetSchema, err := loadSchemaWithContext(loadCtx, cfg.Target, cfg.Schemas, false)
		if err != nil {
			return fmt.Errorf("post-execution validation: failed to reload target schema: %w", err)
		}
		remainingOps, _, err := app.ComputeDiff(sourceSchema, newTargetSchema, appCfg)
		if err != nil {
			return fmt.Errorf("post-execution validation: diff failed: %w", err)
		}
		if len(remainingOps) == 0 {
			fmt.Println("Validation passed: target schema matches expected state")
		} else {
			fmt.Printf("Warning: %d difference(s) remain after execution:\n", len(remainingOps))
			for _, op := range remainingOps {
				fmt.Printf("  - %s: %s\n", op.Kind(), op.ObjectKey())
			}
		}
	}

	return nil
}
