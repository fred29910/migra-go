package plan

import (
	"context"
	"strings"
	"testing"

	"github.com/fred29910/migra-go/internal/diff"
	"github.com/fred29910/migra-go/internal/model"
)

func TestTopoSortKeepsMultipleOpsForSameColumn(t *testing.T) {
	ops := []diff.Operation{
		diff.NewSetNotNullOp("public", "users", "name"),
		diff.NewAlterColumnTypeOp("public", "users", "name", "varchar", "text"),
	}

	sorted, err := TopoSort(context.Background(), ops)
	if err != nil {
		t.Fatalf("TopoSort failed: %v", err)
	}

	if len(sorted) != 2 {
		t.Fatalf("expected 2 operations, got %d", len(sorted))
	}

	kinds := map[diff.Kind]int{}
	for _, op := range sorted {
		kinds[op.Kind()]++
	}
	if kinds[diff.KindSetNotNull] != 1 {
		t.Fatalf("expected 1 set_not_null op, got %d", kinds[diff.KindSetNotNull])
	}
	if kinds[diff.KindAlterColumnType] != 1 {
		t.Fatalf("expected 1 alter_column_type op, got %d", kinds[diff.KindAlterColumnType])
	}
}

func TestTopoSortCreateIndexDependsOnAddTable(t *testing.T) {
	idx := &model.Index{Name: "idx_users_name", Table: "users", Columns: []string{"name"}, Method: "btree"}
	ops := []diff.Operation{
		diff.NewCreateIndexOp("public", idx),
		diff.NewAddTableOp("public", "users", &model.Table{Schema: "public", Name: "users"}),
	}

	sorted, err := TopoSort(context.Background(), ops)
	if err != nil {
		t.Fatalf("TopoSort failed: %v", err)
	}
	if len(sorted) != 2 {
		t.Fatalf("expected 2 operations, got %d", len(sorted))
	}
	if sorted[0].Kind() != diff.KindAddTable {
		t.Fatalf("expected first op to be add_table, got %s", sorted[0].Kind())
	}
	if sorted[1].Kind() != diff.KindAddIndex {
		t.Fatalf("expected second op to be add_index, got %s", sorted[1].Kind())
	}
}

func TestDAGAddDependency_DeduplicatesEdges(t *testing.T) {
	dag := NewDAG()
	a := dag.AddNode(diff.NewAddTableOp("public", "a", model.NewTable("public", "a")))
	b := dag.AddNode(diff.NewAddTableOp("public", "b", model.NewTable("public", "b")))
	dag.AddDependency(a, b)
	dag.AddDependency(a, b)
	if len(a.Dependencies) != 1 {
		t.Fatalf("expected 1 dependency, got %d", len(a.Dependencies))
	}
	if len(b.Dependents) != 1 {
		t.Fatalf("expected 1 dependent, got %d", len(b.Dependents))
	}
}

func TestDAGGetExecutionOrder_StableForLinearChain(t *testing.T) {
	dag := NewDAG()
	c := dag.AddNode(diff.NewAddTableOp("public", "c", model.NewTable("public", "c")))
	b := dag.AddNode(diff.NewAddTableOp("public", "b", model.NewTable("public", "b")))
	a := dag.AddNode(diff.NewAddTableOp("public", "a", model.NewTable("public", "a")))
	dag.AddDependency(a, b)
	dag.AddDependency(b, c)

	ops, err := dag.GetExecutionOrder(context.Background())
	if err != nil {
		t.Fatalf("GetExecutionOrder failed: %v", err)
	}
	got := []string{ops[0].ObjectKey().Name, ops[1].ObjectKey().Name, ops[2].ObjectKey().Name}
	want := []string{"c", "b", "a"}
	if strings.Join(got, ",") != strings.Join(want, ",") {
		t.Fatalf("unexpected order: got=%v want=%v", got, want)
	}
}

func TestTopoSortDropConstraintBeforeAddConstraint(t *testing.T) {
	// Simulate the scenario: changeNames generates DROP + ADD for the same constraint.
	// ADD CONSTRAINT should depend on DROP CONSTRAINT, so DROP runs first.
	ops := []diff.Operation{
		diff.NewDropConstraintOp("public", "posts", "posts_pkey"),
		diff.NewAddConstraintOp("public", "posts", &model.Constraint{
			Name:    "posts_pkey",
			Type:    "primary_key",
			Columns: []string{"id"},
		}),
	}

	sorted, err := TopoSort(context.Background(), ops)
	if err != nil {
		t.Fatalf("TopoSort failed: %v", err)
	}
	if len(sorted) != 2 {
		t.Fatalf("expected 2 operations, got %d", len(sorted))
	}
	// DROP must come before ADD
	if sorted[0].Kind() != diff.KindDropConstraint {
		t.Fatalf("expected first op to be drop_constraint, got %s", sorted[0].Kind())
	}
	if sorted[1].Kind() != diff.KindAddConstraint {
		t.Fatalf("expected second op to be add_constraint, got %s", sorted[1].Kind())
	}
}
