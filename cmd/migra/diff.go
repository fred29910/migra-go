package main

import (
	"context"
	"fmt"
	"os"

	"github.com/fred29910/migra-go/internal/app"
	"github.com/fred29910/migra-go/internal/model"
	"github.com/fred29910/migra-go/internal/source"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
)

// sourceRegistry is the global registry of schema loaders
var sourceRegistry = func() *source.Registry {
	reg := source.NewRegistry()
	reg.Register(&source.DBLoader{})
	reg.Register(&source.SQLFileLoader{})
	return reg
}()

// diffCmd represents the diff command
var diffCmd = &cobra.Command{
	Use:   "diff [source] [target]",
	Short: "Compare two schema sources and output the differences",
	Long: `Compare two schema sources (SQL files or PostgreSQL connections) and output the SQL needed to migrate from source to target.

Examples:
  migra diff file.sql postgres://localhost/db
  migra diff postgres://localhost/db1 postgres://localhost/db2
  migra diff file_a.sql file_b.sql
  migra diff file.sql  # target from config database.url
  migra diff          # both from config database.source and database.target`,
	Args: cobra.RangeArgs(0, 2),
	RunE: runDiff,
}

func init() {
	rootCmd.AddCommand(diffCmd)

	// Diff-specific flags（默认值从 viper 读取）
	diffCmd.Flags().StringSliceP("schema", "s", viper.GetStringSlice("diff.schemas"), "schemas to compare (can be multiple)")
	diffCmd.Flags().StringP("format", "f", "sql", "output format: sql or json")
	diffCmd.Flags().Bool("unsafe-drop", viper.GetBool("diff.unsafe_drop"), "allow destructive drop operations")
	diffCmd.Flags().Bool("strict", viper.GetBool("diff.strict"), "fail on unsupported statements")
	diffCmd.Flags().StringP("output", "o", viper.GetString("output.file"), "output file (default: stdout)")
	diffCmd.Flags().Duration("timeout", defaultDiffTimeout, "timeout for schema loading (e.g. 30s, 2m)")

	// 绑定到 viper
	_ = viper.BindPFlag("diff.schemas", diffCmd.Flags().Lookup("schema"))
	_ = viper.BindPFlag("diff.format", diffCmd.Flags().Lookup("format"))
	_ = viper.BindPFlag("diff.unsafe_drop", diffCmd.Flags().Lookup("unsafe-drop"))
	_ = viper.BindPFlag("diff.strict", diffCmd.Flags().Lookup("strict"))
	_ = viper.BindPFlag("output.file", diffCmd.Flags().Lookup("output"))
}

func runDiff(cmd *cobra.Command, args []string) error {
	cfg, err := parseDiffConfig(cmd, args)
	if err != nil {
		return err
	}
	out, warns, err := app.NewDiffService(newDefaultDeps()).Run(cmd.Context(), cfg)
	if err != nil {
		return err
	}
	for _, w := range warns {
		fmt.Fprintf(os.Stderr, "Warning: %s\n", w)
	}
	return writeOutput(out, cfg.OutputFile)
}

// loadSchemaWithContext loads a schema using the source registry
func loadSchemaWithContext(ctx context.Context, sourceStr string, schemas []string, strict bool) (*model.Schema, error) {
	opts := source.LoadOptions{
		Schemas: schemas,
		Strict:  strict,
	}
	schema, errs, err := sourceRegistry.Load(ctx, sourceStr, opts)
	if err != nil {
		return nil, err
	}
	// Log parsing errors if any
	for _, e := range errs {
		fmt.Fprintf(os.Stderr, "Warning [%s]: %v\n", sourceStr, e)
	}
	return schema, nil
}
