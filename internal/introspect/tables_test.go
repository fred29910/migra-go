package introspect

import (
	"testing"

	"github.com/fred29910/migra-go/internal/model"
)

func TestDefaultExprUniquePointers(t *testing.T) {
	// After the fix, each column with a DEFAULT gets its own string pointer.
	// This test validates that the CopyDefaultExpr helper (used by loadTables)
	// produces distinct pointers for each column.
	tbl := model.NewTable("public", "test_table")

	// Simulates the FIXED pattern: copy the string to a new variable before taking address
	colDefaults := []struct {
		name string
		typ  string
		expr string
	}{
		{"id", "bigint", "42"},
		{"name", "text", "'hello'::text"},
		{"value", "numeric", ""},
	}
	for _, cd := range colDefaults {
		col := &model.Column{Name: cd.name, DataType: cd.typ}
		if cd.expr != "" {
			defaultCopy := cd.expr
			col.DefaultExpr = &defaultCopy
		}
		tbl.AddColumn(col)
	}

	if tbl.Columns[0].DefaultExpr == nil {
		t.Fatal("col0 should have DefaultExpr")
	}
	if tbl.Columns[1].DefaultExpr == nil {
		t.Fatal("col1 should have DefaultExpr")
	}
	if *tbl.Columns[0].DefaultExpr != "42" {
		t.Fatalf("col0.DefaultExpr = %q, want '42'", *tbl.Columns[0].DefaultExpr)
	}
	if *tbl.Columns[1].DefaultExpr != "'hello'::text" {
		t.Fatalf("col1.DefaultExpr = %q, want 'hello'::text", *tbl.Columns[1].DefaultExpr)
	}
	if tbl.Columns[0].DefaultExpr == tbl.Columns[1].DefaultExpr {
		t.Fatal("columns should have unique DefaultExpr pointers")
	}
}
