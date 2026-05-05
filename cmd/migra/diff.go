package main

import (
	"context"
	"fmt"
	"os"
	"strings"

	"github.com/fred29910/migra-go/internal/app"
	"github.com/fred29910/migra-go/internal/introspect"
	"github.com/fred29910/migra-go/internal/model"
	"github.com/fred29910/migra-go/internal/parser"
	"github.com/spf13/cobra"
)

// diffCmd represents the diff command
var diffCmd = &cobra.Command{
	Use:   "diff [source] [target]",
	Short: "Compare two schema sources and output the differences",
	Long: `Compare two schema sources (SQL files or PostgreSQL connections) and output the SQL needed to migrate from source to target.

Examples:
  migra diff file.sql postgres://localhost/db
  migra diff postgres://localhost/db1 postgres://localhost/db2
  migra diff file_a.sql file_b.sql`,
	Args: cobra.ExactArgs(2),
	RunE: runDiff,
}

func init() {
	rootCmd.AddCommand(diffCmd)

	// Diff-specific flags
	diffCmd.Flags().StringSliceP("schema", "s", []string{"public"}, "schemas to compare (can be multiple)")
	diffCmd.Flags().StringP("format", "f", "sql", "output format: sql or json")
	diffCmd.Flags().Bool("unsafe-drop", false, "allow destructive drop operations")
	diffCmd.Flags().Bool("strict", false, "fail on unsupported statements")
	diffCmd.Flags().StringP("output", "o", "", "output file (default: stdout)")
	diffCmd.Flags().Duration("timeout", defaultDiffTimeout, "timeout for schema loading (e.g. 30s, 2m)")
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

// loadSchemaWithContext loads a schema from either a SQL file or PostgreSQL connection
func loadSchemaWithContext(ctx context.Context, source string, schemas []string, strict bool) (*model.Schema, error) {
	if isPostgresURL(source) {
		return loadFromDB(ctx, source, schemas)
	}
	if isSQLFile(source) {
		return loadFromSQLFile(source, strict)
	}
	return nil, fmt.Errorf("unsupported source: %s (must be .sql file or postgres:// URL)", source)
}

// loadFromDB loads schema from a PostgreSQL database
func loadFromDB(ctx context.Context, connStr string, schemas []string) (*model.Schema, error) {
	opt := introspect.LoadOptions{
		Schemas: schemas,
	}
	schema, err := introspect.LoadFromDB(ctx, connStr, opt)
	if err != nil {
		return nil, fmt.Errorf("failed to load schema from database: %w", err)
	}
	return schema, nil
}

// loadFromSQLFile loads schema from a SQL file
func loadFromSQLFile(path string, strict bool) (*model.Schema, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("failed to read file %s: %w", path, err)
	}

	p := parser.NewParser()
	schema, parseErr := p.ParseSQL(string(data))
	if parseErr != nil && strict {
		return nil, fmt.Errorf("failed to parse SQL file %s: %w", path, parseErr)
	}

	// Log errors if any
	for _, warnErr := range p.Errors() {
		fmt.Fprintf(os.Stderr, "Warning [%s]: %v\n", path, warnErr)
	}

	return schema, nil
}

// isPostgresURL checks if the string is a PostgreSQL connection URL
func isPostgresURL(s string) bool {
	return strings.HasPrefix(s, "postgres://") || strings.HasPrefix(s, "pg://")
}

// isSQLFile checks if the string is a SQL file
func isSQLFile(s string) bool {
	return strings.HasSuffix(strings.ToLower(s), ".sql")
}
