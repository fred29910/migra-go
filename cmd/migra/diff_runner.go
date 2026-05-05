package main

import (
	"context"
	"fmt"
	"os"
	"time"

	"github.com/fred29910/migra-go/internal/diff"
	"github.com/fred29910/migra-go/internal/model"
	"github.com/fred29910/migra-go/internal/normalize"
	"github.com/fred29910/migra-go/internal/plan"
	"github.com/fred29910/migra-go/internal/render"
	"github.com/spf13/cobra"
)

const defaultDiffTimeout = 30 * time.Second

// diffConfig holds all parsed configuration for the diff command
type diffConfig struct {
	source     string
	target     string
	schemas    []string
	format     string
	outputFile string
	unsafeDrop bool
	strict     bool
	timeout    time.Duration
}

// runnerDeps holds injectable dependencies for runDiffWithDeps
type runnerDeps struct {
	loadSchema     func(ctx context.Context, source string, schemas []string, strict bool) (*model.Schema, error)
	compute        func(source, target *model.Schema, cfg diffConfig) ([]diff.Operation, []string, error)
	reportWarnings func(warnings []string)
	render         func(ops []diff.Operation, format string) (string, error)
	writeOutput    func(output string, outputFile string) error
}

// newDefaultDeps creates the default production dependencies
func newDefaultDeps() runnerDeps {
	return runnerDeps{
		loadSchema: func(ctx context.Context, source string, schemas []string, strict bool) (*model.Schema, error) {
			return loadSchemaWithContext(ctx, source, schemas, strict)
		},
		compute: func(source, target *model.Schema, cfg diffConfig) ([]diff.Operation, []string, error) {
			return computeDiff(source, target, cfg)
		},
		reportWarnings: func(warnings []string) {
			for _, w := range warnings {
				fmt.Fprintf(os.Stderr, "Warning: %s\n", w)
			}
		},
		render: func(ops []diff.Operation, format string) (string, error) {
			renderer := render.NewRenderer()
			return renderer.RenderOutput(ops, format)
		},
		writeOutput: writeOutput,
	}
}

// parseDiffConfig parses the diff command flags into a diffConfig struct
func parseDiffConfig(cmd *cobra.Command, args []string) (diffConfig, error) {
	if len(args) != 2 {
		return diffConfig{}, fmt.Errorf("expected 2 arguments (source and target), got %d", len(args))
	}

	schemas, err := cmd.Flags().GetStringSlice("schema")
	if err != nil {
		return diffConfig{}, fmt.Errorf("failed to get schema flag: %w", err)
	}
	format, err := cmd.Flags().GetString("format")
	if err != nil {
		return diffConfig{}, fmt.Errorf("failed to get format flag: %w", err)
	}
	unsafeDrop, err := cmd.Flags().GetBool("unsafe-drop")
	if err != nil {
		return diffConfig{}, fmt.Errorf("failed to get unsafe-drop flag: %w", err)
	}
	strict, err := cmd.Flags().GetBool("strict")
	if err != nil {
		return diffConfig{}, fmt.Errorf("failed to get strict flag: %w", err)
	}
	outputFile, err := cmd.Flags().GetString("output")
	if err != nil {
		return diffConfig{}, fmt.Errorf("failed to get output flag: %w", err)
	}
	timeout, err := cmd.Flags().GetDuration("timeout")
	if err != nil {
		return diffConfig{}, fmt.Errorf("failed to get timeout flag: %w", err)
	}

	return diffConfig{
		source:     args[0],
		target:     args[1],
		schemas:    schemas,
		format:     format,
		outputFile: outputFile,
		unsafeDrop: unsafeDrop,
		strict:     strict,
		timeout:    timeout,
	}, nil
}

// runDiffWithDeps runs the diff pipeline with injectable dependencies
func runDiffWithDeps(parent context.Context, cfg diffConfig, deps runnerDeps) error {
	ctx, cancel := context.WithTimeout(parent, cfg.timeout)
	defer cancel()

	sourceSchema, err := deps.loadSchema(ctx, cfg.source, cfg.schemas, cfg.strict)
	if err != nil {
		return fmt.Errorf("failed to load source: %w", err)
	}
	targetSchema, err := deps.loadSchema(ctx, cfg.target, cfg.schemas, cfg.strict)
	if err != nil {
		return fmt.Errorf("failed to load target: %w", err)
	}

	ops, warnings, err := deps.compute(sourceSchema, targetSchema, cfg)
	if err != nil {
		return err
	}
	deps.reportWarnings(warnings)

	output, err := deps.render(ops, cfg.format)
	if err != nil {
		return err
	}
	return deps.writeOutput(output, cfg.outputFile)
}

// computeDiff runs the normalize → diff → plan pipeline and returns operations and warnings
func computeDiff(source, target *model.Schema, cfg diffConfig) ([]diff.Operation, []string, error) {
	// Normalize schemas
	if err := normalize.CanonicalizeSchema(source); err != nil {
		return nil, nil, fmt.Errorf("failed to normalize source schema: %w", err)
	}
	if err := normalize.CanonicalizeSchema(target); err != nil {
		return nil, nil, fmt.Errorf("failed to normalize target schema: %w", err)
	}

	// Diff schemas
	differ := diff.NewDiffer()
	operations := differ.Diff(source, target)
	warnings := differ.Warnings()

	// Report destructive changes
	destructiveCount := 0
	for _, op := range operations {
		if op.IsDestructive() {
			destructiveCount++
		}
	}
	if destructiveCount > 0 {
		warnings = append(warnings, fmt.Sprintf("%d destructive operation(s) detected!", destructiveCount))
		for _, op := range operations {
			if op.IsDestructive() {
				warnings = append(warnings, fmt.Sprintf("  - %s: %s (destructive)", op.Kind(), op.ObjectKey()))
			}
		}
		if !cfg.unsafeDrop {
			warnings = append(warnings, "Use --unsafe-drop to include destructive DROP operations in output")
		}
	}

	// Plan execution
	planner := plan.NewPlanner(cfg.unsafeDrop)
	stages := planner.Plan(operations)

	// Build execution list with deterministic stage order and topo sorting
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
			return nil, nil, fmt.Errorf("failed to topologically sort %s operations: %w", stage, err)
		}
		allOps = append(allOps, sortedStageOps...)
	}

	return allOps, warnings, nil
}

// writeOutput writes output text to file or stdout
func writeOutput(output string, outputFile string) error {
	if outputFile != "" {
		return os.WriteFile(outputFile, []byte(output), 0644)
	}
	fmt.Println(output)
	return nil
}
