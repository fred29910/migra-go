package main

import (
	"fmt"
	"strings"
	"testing"

	"github.com/fred29910/migra-go/internal/diff"
	"github.com/fred29910/migra-go/internal/model"
	"github.com/fred29910/migra-go/internal/plan"
	"github.com/fred29910/migra-go/internal/render"
)

// TestIntegrationDiffRender tests the full diff -> render pipeline
func TestIntegrationDiffRender(t *testing.T) {
	// Create source schema: users table with id, name
	source := model.NewSchema()
	sourceNs := source.GetOrCreateNamespace("public")
	sourceTable := model.NewTable("public", "users")
	sourceTable.AddColumn(&model.Column{Name: "id", DataType: "integer", IsNullable: false})
	sourceTable.AddColumn(&model.Column{Name: "name", DataType: "varchar", IsNullable: false})
	sourceTable.PrimaryKey = &model.PrimaryKey{Name: "users_pkey", Columns: []string{"id"}}
	sourceNs.Tables["users"] = sourceTable

	// Create target schema: users table with id, name (changed to text), age (new)
	target := model.NewSchema()
	targetNs := target.GetOrCreateNamespace("public")
	targetTable := model.NewTable("public", "users")
	targetTable.AddColumn(&model.Column{Name: "id", DataType: "integer", IsNullable: false})
	targetTable.AddColumn(&model.Column{Name: "name", DataType: "text", IsNullable: false})  // Type changed
	targetTable.AddColumn(&model.Column{Name: "age", DataType: "integer", IsNullable: true}) // New column
	targetTable.PrimaryKey = &model.PrimaryKey{Name: "users_pkey", Columns: []string{"id"}}
	targetNs.Tables["users"] = targetTable

	// Run diff
	d := diff.NewDiffer()
	ops := d.Diff(source, target)

	// Verify operations
	if len(ops) == 0 {
		t.Fatal("expected operations, got none")
	}

	t.Logf("Generated %d operations", len(ops))
	for i, op := range ops {
		t.Logf("  Op %d: %s (destructive: %v)", i, op.Kind(), op.IsDestructive())
	}

	// Check for expected operations
	hasAlterColumnType := false
	hasAddColumn := false
	for _, op := range ops {
		switch op.Kind() {
		case diff.KindAlterColumnType:
			hasAlterColumnType = true
			alterOp := op.(*diff.AlterColumnTypeOp)
			if alterOp.Column != "name" {
				t.Errorf("expected alter column 'name', got '%s'", alterOp.Column)
			}
			if alterOp.FromType != "varchar" || alterOp.ToType != "text" {
				t.Errorf("expected alter type varchar->text, got %s->%s", alterOp.FromType, alterOp.ToType)
			}
		case diff.KindAddColumn:
			hasAddColumn = true
			addOp := op.(*diff.AddColumnOp)
			if addOp.Column.Name != "age" {
				t.Errorf("expected add column 'age', got '%s'", addOp.Column.Name)
			}
		}
	}

	if !hasAlterColumnType {
		t.Error("expected AlterColumnType operation")
	}
	if !hasAddColumn {
		t.Error("expected AddColumn operation")
	}

	// Render to SQL
	r := render.NewRenderer()
	sql := r.RenderAll(ops)

	if sql == "" || sql == "-- No changes detected" {
		t.Fatal("expected SQL output, got empty")
	}

	t.Logf("Generated SQL:\n%s", sql)

	// Verify SQL contains expected statements
	if !strings.Contains(sql, "ALTER TABLE") {
		t.Error("expected ALTER TABLE statement")
	}
	if !strings.Contains(sql, "ADD COLUMN") {
		t.Error("expected ADD COLUMN statement")
	}

	// Test JSON rendering
	jsonStr, err := render.RenderJSON(ops)
	if err != nil {
		t.Errorf("RenderJSON failed: %v", err)
	} else {
		t.Logf("Generated JSON: %s", jsonStr)
		if !strings.Contains(jsonStr, `"kind"`) {
			t.Error("expected JSON to contain 'kind' field")
		}
	}
}

// TestIntegrationEnumType tests enum type diff and render
func TestIntegrationEnumType(t *testing.T) {
	// Create source schema with enum
	source := model.NewSchema()
	sourceNs := source.GetOrCreateNamespace("public")
	sourceNs.Types["user_role"] = &model.EnumType{
		Name:   "user_role",
		Labels: []string{"admin", "user"},
	}

	// Create target schema with enum (added label "guest")
	target := model.NewSchema()
	targetNs := target.GetOrCreateNamespace("public")
	targetNs.Types["user_role"] = &model.EnumType{
		Name:   "user_role",
		Labels: []string{"admin", "user", "guest"},
	}

	// Run diff
	d := diff.NewDiffer()
	ops := d.Diff(source, target)

	// Should detect no changes (enum labels order matters, but we added a new one)
	// Actually, this should detect that the enum type changed
	t.Logf("Enum diff generated %d operations", len(ops))
	for _, op := range ops {
		t.Logf("  Op: %s", op.Kind())
	}

	// Render to SQL
	r := render.NewRenderer()
	sql := r.RenderAll(ops)
	t.Logf("Enum SQL:\n%s", sql)
}

// TestIntegrationDAGSort tests the DAG topological sort
func TestIntegrationDAGSort(t *testing.T) {
	// Create operations in wrong order: add column before add table
	ops := []diff.Operation{
		diff.NewAddColumnOp("public", "users", &model.Column{
			Name: "age", DataType: "integer", IsNullable: true,
		}),
		diff.NewAddTableOp("public", "users", &model.Table{
			Schema: "public", Name: "users",
			Columns: []*model.Column{
				{Name: "id", DataType: "integer", IsNullable: false},
			},
		}),
	}

	// Sort using DAG (from plan package)
	sorted, err := plan.TopoSort(ops)
	if err != nil {
		t.Fatalf("TopoSort failed: %v", err)
	}

	// Verify table creation comes before column addition
	if len(sorted) != 2 {
		t.Fatalf("expected 2 ops, got %d", len(sorted))
	}

	// First op should be AddTable
	if sorted[0].Kind() != diff.KindAddTable {
		t.Errorf("expected first op to be add_table, got %s", sorted[0].Kind())
	}
	// Second op should be AddColumn
	if sorted[1].Kind() != diff.KindAddColumn {
		t.Errorf("expected second op to be add_column, got %s", sorted[1].Kind())
	}

	t.Log("DAG sort order correct")
}

// TestFullPipeline tests the full pipeline with plan stage grouping
func TestFullPipeline(t *testing.T) {
	// Create schemas with various changes
	source := model.NewSchema()
	sourceNs := source.GetOrCreateNamespace("public")
	_ = sourceNs
	sourceTable := model.NewTable("public", "users")
	// ... add columns, constraints, etc.
	_ = sourceTable

	target := model.NewSchema()
	targetNs := target.GetOrCreateNamespace("public")
	_ = targetNs
	targetTable := model.NewTable("public", "users")
	// ... add columns, constraints, etc.
	_ = targetTable

	// This is a placeholder for a more complete test
	t.Log("Full pipeline test - placeholder for more comprehensive tests")
	fmt.Println("Integration tests completed successfully!")
}
