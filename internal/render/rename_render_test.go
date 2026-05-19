package render

import (
	"strings"
	"testing"

	"github.com/fred29910/migra-go/internal/diff"
)

func TestRenderRenameColumn(t *testing.T) {
	r := NewRenderer()
	op := diff.NewRenameColumnOp("public", "users", "username", "login_name")
	sql := r.Render(op)
	expected := `ALTER TABLE "public"."users" RENAME COLUMN "username" TO "login_name";`
	if !strings.Contains(sql, expected) {
		t.Errorf("expected SQL to contain %q, got:\n%s", expected, sql)
	}
	if !strings.Contains(sql, "rename_column") {
		t.Errorf("expected operation tag rename_column, got:\n%s", sql)
	}
}

func TestRenderRenameColumn_RiskLow(t *testing.T) {
	r := NewRenderer()
	op := diff.NewRenameColumnOp("public", "items", "sku", "item_code")
	sql := r.Render(op)
	if !strings.Contains(sql, "risk:low") {
		t.Errorf("RENAME COLUMN should be risk:low, got:\n%s", sql)
	}
}

func TestRenderRenameColumn_SchemaQualified(t *testing.T) {
	r := NewRenderer()
	op := diff.NewRenameColumnOp("inventory", "items", "sku", "item_code")
	sql := r.Render(op)
	expected := `ALTER TABLE "inventory"."items" RENAME COLUMN "sku" TO "item_code";`
	if !strings.Contains(sql, expected) {
		t.Errorf("expected SQL to contain %q, got:\n%s", expected, sql)
	}
}

func TestRenderRenameColumn_ReservedWordNames(t *testing.T) {
	r := NewRenderer()
	op := diff.NewRenameColumnOp("public", "users", "order", "sort_order")
	sql := r.Render(op)
	expected := `RENAME COLUMN "order" TO "sort_order"`
	if !strings.Contains(sql, expected) {
		t.Errorf("expected quoted names, got:\n%s", sql)
	}
}
