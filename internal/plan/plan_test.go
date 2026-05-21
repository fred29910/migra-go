package plan

import (
	"testing"

	"github.com/fred29910/migra-go/internal/diff"
	"github.com/fred29910/migra-go/internal/model"
)

func TestPlanner_SchemaStages(t *testing.T) {
	planner := NewPlanner(true)
	stages := planner.Plan([]diff.Operation{
		diff.NewCreateSchemaOp("auth"),
		diff.NewDropSchemaOp("old_schema"),
	})

	if len(stages[StagePreDeploy]) != 1 || stages[StagePreDeploy][0].Kind() != diff.KindCreateSchema {
		t.Fatalf("expected create_schema in pre-deploy, got %#v", stages[StagePreDeploy])
	}
	if len(stages[StagePostDeploy]) != 1 || stages[StagePostDeploy][0].Kind() != diff.KindDropSchema {
		t.Fatalf("expected drop_schema in post-deploy, got %#v", stages[StagePostDeploy])
	}
}

func TestPlanner_NewOperationStages(t *testing.T) {
	ops := []diff.Operation{
		diff.NewAddEnumLabelOp("public", "user_role", "guest"),
		diff.NewSetDefaultOp("public", "users", "created_at", "now()"),
		diff.NewDropDefaultOp("public", "users", "created_at"),
		diff.NewDropColumnOp("public", "users", "old_col"),
	}

	safeStages := NewPlanner(false).Plan(ops)
	if len(safeStages[StageDeploy]) != 3 {
		t.Fatalf("expected three deploy ops, got %#v", safeStages[StageDeploy])
	}
	if len(safeStages[StagePostDeploy]) != 0 {
		t.Fatalf("drop column must be skipped without unsafe-drop")
	}

	unsafeStages := NewPlanner(true).Plan(ops)
	if len(unsafeStages[StagePostDeploy]) != 1 {
		t.Fatalf("expected drop column in post-deploy with unsafe-drop")
	}
}

func TestPlanner_P3ObjectStages(t *testing.T) {
	stages := NewPlanner(true).Plan([]diff.Operation{
		diff.NewCreateViewOp("public", &model.View{Name: "v"}),
		diff.NewDropViewOp("public", "old_v"),
		diff.NewCreateSequenceOp("public", &model.Sequence{Name: "s"}),
		diff.NewDropExtensionOp("public", "old_ext"),
	})
	if len(stages[StagePreDeploy]) != 2 {
		t.Fatalf("expected 2 pre-deploy ops, got %d: %#v", len(stages[StagePreDeploy]), stages[StagePreDeploy])
	}
	if len(stages[StagePostDeploy]) != 2 {
		t.Fatalf("expected 2 post-deploy ops, got %d: %#v", len(stages[StagePostDeploy]), stages[StagePostDeploy])
	}
}
