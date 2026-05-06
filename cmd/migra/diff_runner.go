package main

import (
	"context"
	"fmt"
	"os"
	"time"

	"github.com/fred29910/migra-go/internal/app"
	"github.com/fred29910/migra-go/internal/diff"
	"github.com/fred29910/migra-go/internal/model"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
)

const defaultDiffTimeout = 30 * time.Second

// parseDiffConfig parses the diff command flags into a app.Config struct
func parseDiffConfig(cmd *cobra.Command, args []string) (app.Config, error) {
	var source, target string

	switch len(args) {
	case 2:
		source, target = args[0], args[1]
	case 1:
		source = args[0]
		target = viper.GetString("database.url")
		if target == "" {
			return app.Config{}, fmt.Errorf("target not specified: provide 2 arguments, or set database.url in config file")
		}
	case 0:
		source = viper.GetString("database.source")
		target = viper.GetString("database.target")
		if source == "" || target == "" {
			return app.Config{}, fmt.Errorf("source and target not specified: provide arguments, or set database.source and database.target in config file")
		}
	default:
		return app.Config{}, fmt.Errorf("expected 0, 1 or 2 arguments, got %d", len(args))
	}

	schemas, err := cmd.Flags().GetStringSlice("schema")
	if err != nil {
		return app.Config{}, fmt.Errorf("failed to get schema flag: %w", err)
	}
	format, err := cmd.Flags().GetString("format")
	if err != nil {
		return app.Config{}, fmt.Errorf("failed to get format flag: %w", err)
	}
	unsafeDrop, err := cmd.Flags().GetBool("unsafe-drop")
	if err != nil {
		return app.Config{}, fmt.Errorf("failed to get unsafe-drop flag: %w", err)
	}
	strict, err := cmd.Flags().GetBool("strict")
	if err != nil {
		return app.Config{}, fmt.Errorf("failed to get strict flag: %w", err)
	}
	outputFile, err := cmd.Flags().GetString("output")
	if err != nil {
		return app.Config{}, fmt.Errorf("failed to get output flag: %w", err)
	}
	timeout, err := cmd.Flags().GetDuration("timeout")
	if err != nil {
		return app.Config{}, fmt.Errorf("failed to get timeout flag: %w", err)
	}

	return app.Config{
		Source:     source,
		Target:     target,
		Schemas:    schemas,
		Format:     format,
		OutputFile: outputFile,
		UnsafeDrop: unsafeDrop,
		Strict:     strict,
		Timeout:    timeout,
	}, nil
}

// newDefaultDeps creates the default production dependencies
func newDefaultDeps() app.RunnerDeps {
	return app.RunnerDeps{
		LoadSchema: func(ctx context.Context, source string, schemas []string, strict bool) (*model.Schema, error) {
			return loadSchemaWithContext(ctx, source, schemas, strict)
		},
		Compute: func(source, target *model.Schema, cfg app.Config) ([]diff.Operation, []string, error) {
			return app.ComputeDiff(source, target, cfg)
		},
		Render: func(ops []diff.Operation, format string) (string, error) {
			return app.RenderOutput(ops, format)
		},
	}
}

// writeOutput writes output text to file or stdout
func writeOutput(output string, outputFile string) error {
	if outputFile != "" {
		return os.WriteFile(outputFile, []byte(output), 0644)
	}
	fmt.Println(output)
	return nil
}
