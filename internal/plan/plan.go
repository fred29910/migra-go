package plan

import (
	"github.com/fred29910/migra-go/internal/diff"
	"github.com/fred29910/migra-go/internal/model"
)

// Stage represents an execution stage
type Stage string

const (
	StagePreDeploy  Stage = "pre-deploy"  // Create objects
	StageDeploy     Stage = "deploy"      // Alter objects
	StagePostDeploy Stage = "post-deploy" // Drop objects (dangerous)
)

// PlannedOp wraps an operation with its stage
type PlannedOp struct {
	Op    diff.Operation
	Stage Stage
}

// PlanEngine defines the interface for execution plan creation.
type PlanEngine interface {
	Plan(ops []diff.Operation) map[Stage][]diff.Operation
}

// Compile-time check: Planner must satisfy PlanEngine.
var _ PlanEngine = (*Planner)(nil)

// Planner creates execution plans from diff operations
type Planner struct {
	unsafeDrops bool
}

// NewPlanner creates a new Planner
func NewPlanner(unsafeDrops bool) *Planner {
	return &Planner{
		unsafeDrops: unsafeDrops,
	}
}

// Plan creates an execution plan from operations
// Returns operations grouped by stage
func (p *Planner) Plan(ops []diff.Operation) map[Stage][]diff.Operation {
	stages := make(map[Stage][]diff.Operation)
	stages[StagePreDeploy] = make([]diff.Operation, 0)
	stages[StageDeploy] = make([]diff.Operation, 0)
	stages[StagePostDeploy] = make([]diff.Operation, 0)

	for _, op := range ops {
		stage := p.assignStage(op)
		if stage != "" {
			stages[stage] = append(stages[stage], op)
		}
	}

	return stages
}

// assignStage assigns an operation to a stage
func (p *Planner) assignStage(op diff.Operation) Stage {
	kind := op.Kind()

	switch kind {
	// Pre-deploy: create new objects
	case diff.KindAddTable, diff.KindAddColumn, diff.KindAddIndex, diff.KindAddConstraint, diff.KindAddEnumType:
		return StagePreDeploy

	// Deploy: alter existing objects
	case diff.KindAlterColumnType, diff.KindSetNotNull, diff.KindDropNotNull, diff.KindAddEnumLabel, diff.KindSetDefault, diff.KindDropDefault:
		return StageDeploy

	// Post-deploy: drop objects (dangerous)
	case diff.KindDropTable, diff.KindDropColumn, diff.KindDropIndex, diff.KindDropConstraint, diff.KindDropEnumType:
		if p.unsafeDrops {
			return StagePostDeploy
		}
		// Skip dangerous operations if unsafeDrops is false
		return ""
	}

	return StageDeploy
}

// TopoSort performs topological sort on operations using DAG
func TopoSort(ops []diff.Operation) ([]diff.Operation, error) {
	dag := BuildDAG(ops)
	return dag.GetExecutionOrder()
}

// GetObjectKey extracts object key from operation for dependency tracking
func GetObjectKey(op diff.Operation) model.ObjectKey {
	return op.ObjectKey()
}
