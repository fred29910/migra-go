package plan

import (
	"testing"

	"github.com/fred29910/migra-go/internal/diff"
)

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
