package app

import (
	"context"
	"fmt"
	"time"

	"github.com/fred29910/migra-go/internal/diff"
	"github.com/fred29910/migra-go/internal/errors"
	"github.com/fred29910/migra-go/internal/model"
)

// DiffConfig holds configuration for a diff operation.
type DiffConfig struct {
	Source     string
	Target     string
	Schemas    []string
	Format     string
	OutputFile string
	UnsafeDrop bool
	Strict     bool
	Timeout    time.Duration
	NoRename   bool
}

// PushConfig holds configuration for a push operation.
type PushConfig struct {
	DiffConfig
	Execute  bool // skip interactive confirmation (--execute)
	NoVerify bool // skip post-execution validation (--no-verify)
}

// Deprecated: Use DiffConfig instead.
type Config = DiffConfig

// HookStage identifies which pipeline stage a hook attaches to.
type HookStage string

const (
	HookAfterLoad    HookStage = "after_load"
	HookAfterCompute HookStage = "after_compute"
	HookAfterRender  HookStage = "after_render"
)

// HookContext carries stage-specific data to hook functions.
type HookContext struct {
	Stage        HookStage
	LoadSource   string
	LoadTarget   string
	SourceSchema *model.Schema
	TargetSchema *model.Schema
	Operations   []diff.Operation
	Warnings     []string
	Output       string
	LoadErr      error
	ComputeErr   error
	RenderErr    error
}

// HookFunc is a callback invoked at pipeline stage boundaries.
// Return error to abort the pipeline.
type HookFunc func(ctx context.Context, hctx HookContext) error

// DiffService defines the interface for running schema diff operations.
type DiffService interface {
	Run(ctx context.Context, cfg DiffConfig) (output string, warnings []string, err error)
}

// RunnerDeps holds the dependencies for the diff service.
type RunnerDeps struct {
	LoadSchema func(ctx context.Context, source string, schemas []string, strict bool) (*model.Schema, error)
	Compute    func(ctx context.Context, source, target *model.Schema, cfg DiffConfig) ([]diff.Operation, []string, error)
	Render     func(ctx context.Context, ops []diff.Operation, format string) (string, error)

	// Hooks are optional callbacks invoked at pipeline stage boundaries.
	// Return error from a hook to abort the pipeline.
	// Production default: nil (no hooks).
	Hooks []HookFunc
}

type diffService struct {
	deps RunnerDeps
}

// NewDiffService creates a new DiffService with the given dependencies.
func NewDiffService(deps RunnerDeps) DiffService {
	return &diffService{deps: deps}
}

func (s *diffService) runHooks(ctx context.Context, hctx HookContext) error {
	for _, hook := range s.deps.Hooks {
		if err := hook(ctx, hctx); err != nil {
			return err
		}
	}
	return nil
}

func (s *diffService) Run(parent context.Context, cfg DiffConfig) (string, []string, error) {
	ctx, cancel := context.WithTimeout(parent, cfg.Timeout)
	defer cancel()

	sourceSchema, err := s.deps.LoadSchema(ctx, cfg.Source, cfg.Schemas, cfg.Strict)
	if err != nil {
		return "", nil, fmt.Errorf("load source: %w, %w", errors.ErrLoadFailed, err)
	}
	targetSchema, err := s.deps.LoadSchema(ctx, cfg.Target, cfg.Schemas, cfg.Strict)
	if err != nil {
		return "", nil, fmt.Errorf("load target: %w, %w", errors.ErrLoadFailed, err)
	}

	// Hook: after_load — both schemas loaded
	if err := s.runHooks(ctx, HookContext{
		Stage:        HookAfterLoad,
		SourceSchema: sourceSchema,
		TargetSchema: targetSchema,
		LoadSource:   cfg.Source,
		LoadTarget:   cfg.Target,
	}); err != nil {
		return "", nil, err
	}

	ops, warnings, err := s.deps.Compute(ctx, sourceSchema, targetSchema, cfg)
	if err != nil {
		return "", nil, err
	}

	// Hook: after_compute — diff operations computed
	if err := s.runHooks(ctx, HookContext{
		Stage:      HookAfterCompute,
		Operations: ops,
		Warnings:   warnings,
	}); err != nil {
		return "", warnings, err
	}

	output, err := s.deps.Render(ctx, ops, cfg.Format)
	if err != nil {
		return "", nil, err
	}

	// Hook: after_render — output rendered
	if err := s.runHooks(ctx, HookContext{
		Stage:  HookAfterRender,
		Output: output,
	}); err != nil {
		return "", warnings, err
	}

	return output, warnings, nil
}
