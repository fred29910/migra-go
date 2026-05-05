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
)

const defaultDiffTimeout = 30 * time.Second

// parseDiffConfig parses the diff command flags into a app.Config struct
func parseDiffConfig(cmd *cobra.Command, args []string) (app.Config, error) {
	if len(args) != 2 {
		return app.Config{}, fmt.Errorf("expected 2 arguments (source and target), got %d", len(args))
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
		Source:     args[0],
		Target:     args[1],
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
