package app

import (
	"fmt"

	"github.com/fred29910/migra-go/internal/diff"
	"github.com/fred29910/migra-go/internal/model"
	"github.com/fred29910/migra-go/internal/normalize"
	"github.com/fred29910/migra-go/internal/plan"
	"github.com/fred29910/migra-go/internal/render"
)

func FilterNamespaces(source, target *model.Schema, schemas []string) []string {
	if len(schemas) == 0 {
		return nil
	}

	schemaSet := make(map[string]bool, len(schemas))
	for _, s := range schemas {
		schemaSet[s] = true
	}

	filterSchema := func(s *model.Schema) {
		for name := range s.Schemas {
			if !schemaSet[name] {
				delete(s.Schemas, name)
			}
		}
	}

	filterSchema(source)
	filterSchema(target)

	if len(source.Schemas) == 0 && len(target.Schemas) == 0 {
		return []string{"no schemas matched the filter — check your --schema flag(s)"}
	}
	return nil
}

func NormalizeSchemas(source, target *model.Schema) error {
	if err := normalize.CanonicalizeSchema(source); err != nil {
		return fmt.Errorf("failed to normalize source schema: %w", err)
	}
	if err := normalize.CanonicalizeSchema(target); err != nil {
		return fmt.Errorf("failed to normalize target schema: %w", err)
	}
	return nil
}

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

func ComputeDiff(source, target *model.Schema, cfg Config) ([]diff.Operation, []string, error) {
	if err := NormalizeSchemas(source, target); err != nil {
		return nil, nil, err
	}

	filterWarnings := FilterNamespaces(source, target, cfg.Schemas)

	differ := diff.NewDiffer()
	operations, warnings := differ.Diff(source, target)
	warnings = append(warnings, filterWarnings...)

	filteredOps, filterWarnings2 := FilterDestructiveOps(operations, cfg.UnsafeDrop)
	warnings = append(warnings, filterWarnings2...)

	sortedOps, err := BuildExecutionPlan(filteredOps, cfg.UnsafeDrop)
	if err != nil {
		return nil, nil, err
	}

	return sortedOps, warnings, nil
}

func RenderOutput(ops []diff.Operation, format string) (string, error) {
	renderer := render.NewRenderer()
	return renderer.RenderOutput(ops, format)
}
