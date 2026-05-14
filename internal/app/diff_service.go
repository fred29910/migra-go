package app

import (
	"context"
	"fmt"
	"time"

	"github.com/fred29910/migra-go/internal/diff"
	"github.com/fred29910/migra-go/internal/model"
	"github.com/fred29910/migra-go/internal/normalize"
	"github.com/fred29910/migra-go/internal/plan"
	"github.com/fred29910/migra-go/internal/render"
)

// Config holds all configuration for the diff service
type Config struct {
	Source     string
	Target     string
	Schemas    []string
	Format     string
	OutputFile string
	UnsafeDrop bool
	Strict     bool
	Timeout    time.Duration
}

// DiffService defines the interface for the diff service
type DiffService interface {
	Run(ctx context.Context, cfg Config) (output string, warnings []string, err error)
}

// RunnerDeps holds injectable dependencies for diffService
type RunnerDeps struct {
	LoadSchema func(ctx context.Context, source string, schemas []string, strict bool) (*model.Schema, error)
	Compute    func(source, target *model.Schema, cfg Config) ([]diff.Operation, []string, error)
	Render     func(ops []diff.Operation, format string) (string, error)
}

// diffService orchestrates the diff pipeline
type diffService struct {
	deps RunnerDeps
}

// NewDiffService creates a new diff service
func NewDiffService(deps RunnerDeps) DiffService {
	return &diffService{deps: deps}
}

// Run executes the diff pipeline
func (s *diffService) Run(parent context.Context, cfg Config) (string, []string, error) {
	ctx, cancel := context.WithTimeout(parent, cfg.Timeout)
	defer cancel()

	sourceSchema, err := s.deps.LoadSchema(ctx, cfg.Source, cfg.Schemas, cfg.Strict)
	if err != nil {
		return "", nil, fmt.Errorf("failed to load source: %w", err)
	}
	targetSchema, err := s.deps.LoadSchema(ctx, cfg.Target, cfg.Schemas, cfg.Strict)
	if err != nil {
		return "", nil, fmt.Errorf("failed to load target: %w", err)
	}

	ops, warnings, err := s.deps.Compute(sourceSchema, targetSchema, cfg)
	if err != nil {
		return "", nil, err
	}

	output, err := s.deps.Render(ops, cfg.Format)
	if err != nil {
		return "", nil, err
	}

	return output, warnings, nil
}

// NormalizeSchemas normalizes source and target schemas in place
func NormalizeSchemas(source, target *model.Schema) error {
	if err := normalize.CanonicalizeSchema(source); err != nil {
		return fmt.Errorf("failed to normalize source schema: %w", err)
	}
	if err := normalize.CanonicalizeSchema(target); err != nil {
		return fmt.Errorf("failed to normalize target schema: %w", err)
	}
	return nil
}

// FilterDestructiveOps filters operations based on destructive flag
func FilterDestructiveOps(ops []diff.Operation, unsafeDrop bool) ([]diff.Operation, []string) {
	if unsafeDrop {
		return ops, nil
	}

	filtered := make([]diff.Operation, 0, len(ops))
	warnings := make([]string, 0, 4)

	destructiveCount := 0
	for _, op := range ops {
		if !op.IsDestructive() {
			filtered = append(filtered, op)
		} else {
			destructiveCount++
		}
	}

	if destructiveCount > 0 {
		warnings = append(warnings, fmt.Sprintf("%d destructive operation(s) detected!", destructiveCount))
		for _, op := range ops {
			if op.IsDestructive() {
				warnings = append(warnings, fmt.Sprintf("  - %s: %s (destructive)", op.Kind(), op.ObjectKey()))
			}
		}
		warnings = append(warnings, "Use --unsafe-drop to include destructive DROP operations in output")
	}

	return filtered, warnings
}

// BuildExecutionPlan creates an ordered execution plan from operations
func BuildExecutionPlan(ops []diff.Operation, unsafeDrop bool) ([]diff.Operation, error) {
	planner := plan.NewPlanner(unsafeDrop)
	stages := planner.Plan(ops)

	stageOrder := []plan.Stage{
		plan.StagePreDeploy,
		plan.StageDeploy,
		plan.StagePostDeploy,
	}

	allOps := make([]diff.Operation, 0, len(ops))
	for _, stage := range stageOrder {
		stageOps := stages[stage]
		if len(stageOps) == 0 {
			continue
		}
		sortedStageOps, err := plan.TopoSort(stageOps)
		if err != nil {
			return nil, fmt.Errorf("failed to topologically sort %s operations: %w", stage, err)
		}
		allOps = append(allOps, sortedStageOps...)
	}

	return allOps, nil
}

// ComputeDiff runs the normalize -> diff -> plan pipeline and returns operations and warnings
func ComputeDiff(source, target *model.Schema, cfg Config) ([]diff.Operation, []string, error) {
	if err := NormalizeSchemas(source, target); err != nil {
		return nil, nil, err
	}

	differ := diff.NewDiffer()
	operations, warnings := differ.Diff(source, target)

	filteredOps, filterWarnings := FilterDestructiveOps(operations, cfg.UnsafeDrop)
	warnings = append(warnings, filterWarnings...)

	sortedOps, err := BuildExecutionPlan(filteredOps, cfg.UnsafeDrop)
	if err != nil {
		return nil, nil, err
	}

	return sortedOps, warnings, nil
}

// RenderOutput renders operations to the specified format
func RenderOutput(ops []diff.Operation, format string) (string, error) {
	renderer := render.NewRenderer()
	return renderer.RenderOutput(ops, format)
}
