package diff

import (
	"testing"

	"github.com/fred29910/migra-go/internal/model"
)

func TestRenameColumnOp_Metadata(t *testing.T) {
	op := NewRenameColumnOp("public", "users", "old_name", "new_name")
	if op.Kind() != KindRenameColumn {
		t.Fatalf("expected KindRenameColumn, got %s", op.Kind())
	}
	if op.IsDestructive() {
		t.Fatal("RenameColumnOp should not be destructive")
	}
	expectedKey := model.ObjectKey{Schema: "public", Kind: model.KindColumn, Name: "users.old_name"}
	if op.ObjectKey() != expectedKey {
		t.Fatalf("expected object key %v, got %v", expectedKey, op.ObjectKey())
	}
}

func TestRenameColumnOp_DependsOn(t *testing.T) {
	op := NewRenameColumnOp("public", "users", "old_name", "new_name")
	deps := op.DependsOn()
	if len(deps) != 1 {
		t.Fatalf("expected 1 dependency, got %d", len(deps))
	}
	expectedDep := model.ObjectKey{Schema: "public", Kind: model.KindTable, Name: "users"}
	if deps[0] != expectedDep {
		t.Fatalf("expected dependency %v, got %v", expectedDep, deps[0])
	}
}

func TestRenameColumnOp_Fields(t *testing.T) {
	op := NewRenameColumnOp("public", "users", "old_name", "new_name")
	if op.Schema != "public" || op.Table != "users" || op.OldName != "old_name" || op.NewName != "new_name" {
		t.Fatalf("unexpected fields: %+v", op)
	}
}
