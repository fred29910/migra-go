package app

import (
	"context"
	"fmt"
	"time"

	"github.com/fred29910/migra-go/internal/diff"
	"github.com/fred29910/migra-go/internal/errors"
	"github.com/fred29910/migra-go/internal/model"
)

// Config holds configuration for a diff operation.
type Config struct {
	Source     string
	Target     string
	Schemas    []string
	Format     string
	OutputFile string
	UnsafeDrop bool
	Strict     bool
	Timeout    time.Duration
	Execute    bool // push: skip interactive confirmation (--execute)
	NoVerify   bool // push: skip post-execution validation (--no-verify)
}

// DiffService defines the interface for running schema diff operations.
type DiffService interface {
	Run(ctx context.Context, cfg Config) (output string, warnings []string, err error)
}

// RunnerDeps holds the dependencies for the diff service.
type RunnerDeps struct {
	LoadSchema func(ctx context.Context, source string, schemas []string, strict bool) (*model.Schema, error)
	Compute    func(source, target *model.Schema, cfg Config) ([]diff.Operation, []string, error)
	Render     func(ops []diff.Operation, format string) (string, error)
}

type diffService struct {
	deps RunnerDeps
}

// NewDiffService creates a new DiffService with the given dependencies.
func NewDiffService(deps RunnerDeps) DiffService {
	return &diffService{deps: deps}
}

func (s *diffService) Run(parent context.Context, cfg Config) (string, []string, error) {
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
