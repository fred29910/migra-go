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

// Service defines the interface for the diff service
type Service interface {
	Run(ctx context.Context, cfg Config) (output string, warnings []string, err error)
}

// RunnerDeps holds injectable dependencies for DiffService
type RunnerDeps struct {
	LoadSchema func(ctx context.Context, source string, schemas []string, strict bool) (*model.Schema, error)
	Compute    func(source, target *model.Schema, cfg Config) ([]diff.Operation, []string, error)
	Render     func(ops []diff.Operation, format string) (string, error)
}

// DiffService orchestrates the diff pipeline
type DiffService struct {
	deps RunnerDeps
}

// NewDiffService creates a new diff service
func NewDiffService(deps RunnerDeps) *DiffService {
	return &DiffService{deps: deps}
}

// Run executes the diff pipeline
func (s *DiffService) Run(parent context.Context, cfg Config) (string, []string, error) {
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

// ComputeDiff runs the normalize -> diff -> plan pipeline and returns operations and warnings
func ComputeDiff(source, target *model.Schema, cfg Config) ([]diff.Operation, []string, error) {
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
		if !cfg.UnsafeDrop {
			warnings = append(warnings, "Use --unsafe-drop to include destructive DROP operations in output")
		}
	}

	// Plan execution
	planner := plan.NewPlanner(cfg.UnsafeDrop)
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

// RenderOutput renders operations to the specified format
func RenderOutput(ops []diff.Operation, format string) (string, error) {
	renderer := render.NewRenderer()
	return renderer.RenderOutput(ops, format)
}
