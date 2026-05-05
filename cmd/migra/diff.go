package main

import (
	"context"
	"fmt"
	"os"
	"strings"

	"github.com/fred29910/migra-go/internal/diff"
	"github.com/fred29910/migra-go/internal/introspect"
	"github.com/fred29910/migra-go/internal/model"
	"github.com/fred29910/migra-go/internal/parser"
	"github.com/fred29910/migra-go/internal/plan"
	"github.com/fred29910/migra-go/internal/render"
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
}

func runDiff(cmd *cobra.Command, args []string) error {
	source := args[0]
	target := args[1]

	// Get flags
	schemas, _ := cmd.Flags().GetStringSlice("schema")
	format, _ := cmd.Flags().GetString("format")
	unsafeDrop, _ := cmd.Flags().GetBool("unsafe-drop")
	strict, _ := cmd.Flags().GetBool("strict")
	output, _ := cmd.Flags().GetString("output")

	// Load source schema
	sourceSchema, err := loadSchema(source, schemas, strict)
	if err != nil {
		return fmt.Errorf("failed to load source: %w", err)
	}

	// Load target schema
	targetSchema, err := loadSchema(target, schemas, strict)
	if err != nil {
		return fmt.Errorf("failed to load target: %w", err)
	}

	// Normalize schemas
	// TODO: implement normalize.CanonicalizeSchema
	_ = sourceSchema
	_ = targetSchema

	// Diff schemas
	differ := diff.NewDiffer()
	operations := differ.Diff(sourceSchema, targetSchema)

	// Analyze destructive changes
	destructiveCount := 0
	for _, op := range operations {
		if op.IsDestructive() {
			destructiveCount++
		}
	}

	// Report destructive changes summary
	if destructiveCount > 0 {
		fmt.Fprintf(os.Stderr, "Warning: %d destructive operation(s) detected!\n", destructiveCount)
		for _, op := range operations {
			if op.IsDestructive() {
				fmt.Fprintf(os.Stderr, "  - %s: %s (destructive)\n", op.Kind(), op.ObjectKey())
			}
		}
		if !unsafeDrop {
			fmt.Fprintln(os.Stderr, "Use --unsafe-drop to include destructive DROP operations in output")
		}
	}

	// Plan execution
	planner := plan.NewPlanner(unsafeDrop)
	stages := planner.Plan(operations)

	// Build execution list with deterministic stage order and topo sorting.
	stageOrder := []plan.Stage{
		plan.StagePreDeploy,
		plan.StageDeploy,
		plan.StagePostDeploy,
	}
	var allOps []diff.Operation
	for _, stage := range stageOrder {
		stageOps := stages[stage]
		if len(stageOps) == 0 {
			continue
		}
		sortedStageOps, err := plan.TopoSort(stageOps)
		if err != nil {
			return fmt.Errorf("failed to topologically sort %s operations: %w", stage, err)
		}
		allOps = append(allOps, sortedStageOps...)
	}

	var outputText string
	switch format {
	case "sql":
		renderer := render.NewRenderer()
		outputText = renderer.RenderAll(allOps)
	case "json":
		jsonStr, err := render.RenderJSON(allOps)
		if err != nil {
			return fmt.Errorf("failed to render json: %w", err)
		}
		outputText = jsonStr
	default:
		return fmt.Errorf("unsupported format: %s (allowed: sql, json)", format)
	}

	// Output
	if output != "" {
		return os.WriteFile(output, []byte(outputText), 0644)
	}

	fmt.Println(outputText)
	return nil
}

// loadSchema loads a schema from either a SQL file or PostgreSQL connection
func loadSchema(source string, schemas []string, strict bool) (*model.Schema, error) {
	if isPostgresURL(source) {
		return loadFromDB(source, schemas)
	}
	if isSQLFile(source) {
		return loadFromSQLFile(source, strict)
	}
	return nil, fmt.Errorf("unsupported source: %s (must be .sql file or postgres:// URL)", source)
}

// loadFromDB loads schema from a PostgreSQL database
func loadFromDB(connStr string, schemas []string) (*model.Schema, error) {
	ctx := context.Background()
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
	schema, err := p.ParseSQL(string(data))
	if err != nil && strict {
		return nil, fmt.Errorf("failed to parse SQL file %s: %w", path, err)
	}

	// Log errors if any
	if err := p.Errors(); len(err) > 0 {
		for _, e := range err {
			fmt.Fprintf(os.Stderr, "Warning: %v\n", e)
		}
	}

	return schema, nil
}

// isPostgresURL checks if the string is a PostgreSQL connection URL
func isPostgresURL(s string) bool {
	return len(s) > 8 && (strings.HasPrefix(s, "postgres://") || strings.HasPrefix(s, "pg://"))
}

// isSQLFile checks if the string is a SQL file
func isSQLFile(s string) bool {
	return len(s) >= 4 && s[len(s)-4:] == ".sql"
}
