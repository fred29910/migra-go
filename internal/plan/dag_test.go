package plan

import (
	"testing"

	"github.com/fred29910/migra-go/internal/diff"
	"github.com/fred29910/migra-go/internal/model"
)

func TestTopoSortKeepsMultipleOpsForSameColumn(t *testing.T) {
	ops := []diff.Operation{
		diff.NewSetNotNullOp("public", "users", "name"),
		diff.NewAlterColumnTypeOp("public", "users", "name", "varchar", "text"),
	}

	sorted, err := TopoSort(ops)
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

	sorted, err := TopoSort(ops)
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
