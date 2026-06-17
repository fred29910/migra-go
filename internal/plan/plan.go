package plan

import (
	"context"

	"github.com/fred29910/migra-go/internal/diff"
	"github.com/fred29910/migra-go/internal/model"
)

// Stage represents an execution stage
type Stage string

const (
	// StagePreDeploy is the stage for creating new objects.
	StagePreDeploy Stage = "pre-deploy"
	// StageDeploy is the stage for altering existing objects.
	StageDeploy Stage = "deploy"
	// StagePostDeploy is the stage for dropping objects (dangerous).
	StagePostDeploy Stage = "post-deploy"
)

// HasStage is an optional interface that operations can implement
// to declare their execution stage directly.
type HasStage interface {
	Stage() Stage
}

// PlannedOp wraps an operation with its stage
type PlannedOp struct {
	Op    diff.Operation
	Stage Stage
}

// Engine defines the interface for execution plan creation.
// Implementations group diff operations into stages for ordered execution.
type Engine interface {
	Plan(ctx context.Context, ops []diff.Operation) (map[Stage][]diff.Operation, error)
}

// Compile-time check: Planner must satisfy Engine.
var _ Engine = (*Planner)(nil)

// Planner creates execution plans from diff operations
type Planner struct {
	unsafeDrops bool
}

// NewPlanner creates a new Planner with the given unsafe drops setting.
func NewPlanner(unsafeDrops bool) *Planner {
	return &Planner{
		unsafeDrops: unsafeDrops,
	}
}

// Plan creates an execution plan from operations.
// It returns operations grouped by stage (pre-deploy, deploy, post-deploy).
func (p *Planner) Plan(ctx context.Context, ops []diff.Operation) (map[Stage][]diff.Operation, error) {
	stages := make(map[Stage][]diff.Operation)
	stages[StagePreDeploy] = make([]diff.Operation, 0)
	stages[StageDeploy] = make([]diff.Operation, 0)
	stages[StagePostDeploy] = make([]diff.Operation, 0)

	for _, op := range ops {
		if err := ctx.Err(); err != nil {
			return nil, err
		}
		stage := p.assignStage(op)
		if stage != "" {
			stages[stage] = append(stages[stage], op)
		}
	}

	return stages, nil
}

// assignStage assigns an operation to the appropriate execution stage.
func (p *Planner) assignStage(op diff.Operation) Stage {
	// 如果 Operation 实现了 HasStage，优先使用
	if stager, ok := op.(HasStage); ok {
		return stager.Stage()
	}

	kind := op.Kind()
	switch kind {
	case diff.KindAddTable, diff.KindAddColumn, diff.KindSetDefault,
		diff.KindAddEnumType, diff.KindAddEnumLabel, diff.KindAddIndex,
		diff.KindAddConstraint, diff.KindCreateView, diff.KindCreateMaterializedView,
		diff.KindCreateSchema, diff.KindCreateSequence, diff.KindCreateExtension,
		diff.KindAlterExtensionUpdate:
		return StagePreDeploy
	case diff.KindAlterColumnType, diff.KindSetNotNull, diff.KindDropNotNull,
		diff.KindDropDefault, diff.KindRenameColumn, diff.KindAddIdentity,
		diff.KindSetIdentity, diff.KindDropIdentity, diff.KindAlterColumnCollation,
		diff.KindAlterSequence:
		return StageDeploy
	case diff.KindDropTable, diff.KindDropColumn, diff.KindDropIndex,
		diff.KindDropConstraint, diff.KindDropEnumType, diff.KindDropView,
		diff.KindDropMaterializedView, diff.KindDropSchema, diff.KindDropSequence,
		diff.KindDropExtension, diff.KindReplaceView:
		return StagePostDeploy
	default:
		return StageDeploy
	}
}

// TopoSort performs topological sort on operations using a DAG.
// It returns operations in dependency-resolved execution order.
func TopoSort(ctx context.Context, ops []diff.Operation) ([]diff.Operation, error) {
	dag := BuildDAG(ops)
	return dag.GetExecutionOrder(ctx)
}

// GetObjectKey extracts the object key from an operation for dependency tracking.
func GetObjectKey(op diff.Operation) model.ObjectKey {
	return op.ObjectKey()
}
