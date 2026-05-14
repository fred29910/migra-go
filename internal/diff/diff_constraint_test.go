package diff

import (
	"testing"

	"github.com/fred29910/migra-go/internal/model"
	"github.com/stretchr/testify/assert"
)

// TestSameConstraintContent_DBIntrospectColumns verifies that constraints loaded
// from DB introspect (which now set Columns) match SQL-parsed constraints.
func TestSameConstraintContent_DBIntrospectColumns(t *testing.T) {
	// Simulate SQL-parsed constraint
	sqlConstraint := &model.Constraint{
		Name:    "posts_pkey",
		Type:    "primary_key",
		Columns: []string{"id"},
	}

	// Simulate DB-introspected constraint (now with Columns set)
	dbConstraint := &model.Constraint{
		Name:       "posts_pkey",
		Type:       "primary_key",
		Columns:    []string{"id"},
		Definition: "PRIMARY KEY (id)",
	}

	assert.True(t, sameConstraintContent(sqlConstraint, dbConstraint),
		"sameConstraintContent should return true when only Definition differs")
}

// TestDiffTableConstraints_NoFalseChangeForPK verifies that diffTableConstraints
// does not generate DROP+ADD for a primary key that exists in both schemas.
func TestDiffTableConstraints_NoFalseChangeForPK(t *testing.T) {
	differ := NewDiffer()

	// Source: DB schema with constraints (Columns populated)
	sourceSchema := model.NewSchema()
	sourceNs := sourceSchema.GetOrCreateNamespace("public")
	sourceTable := model.NewTable("public", "posts")
	sourceTable.Constraints["posts_pkey"] = &model.Constraint{
		Name:    "posts_pkey",
		Type:    "primary_key",
		Columns: []string{"id"},
		Table:   "posts",
	}
	sourceNs.Tables["posts"] = sourceTable

	// Target: SQL file schema with constraints
	targetSchema := model.NewSchema()
	targetNs := targetSchema.GetOrCreateNamespace("public")
	targetTable := model.NewTable("public", "posts")
	targetTable.Constraints["posts_pkey"] = &model.Constraint{
		Name:    "posts_pkey",
		Type:    "primary_key",
		Columns: []string{"id"},
		Table:   "posts",
	}
	targetNs.Tables["posts"] = targetTable

	ops, _ := differ.Diff(sourceSchema, targetSchema)

	// Should have no operations since constraints are identical
	for _, op := range ops {
		if op.Kind() == KindDropConstraint || op.Kind() == KindAddConstraint {
			t.Fatalf("unexpected constraint operation: %s %s", op.Kind(), op.ObjectKey())
		}
	}
}

// TestDiffTableConstraints_NilColumnsMismatch verifies the old bug:
// when DB introspect didn't set Columns (nil), sameConstraintContent
// would return false even for identical constraints.
func TestDiffTableConstraints_NilColumnsMismatch(t *testing.T) {
	// SQL-parsed constraint has Columns
	sqlConstraint := &model.Constraint{
		Name:    "posts_pkey",
		Type:    "primary_key",
		Columns: []string{"id"},
	}

	// Old DB-introspected constraint without Columns (nil) — the bug
	dbConstraintOld := &model.Constraint{
		Name:       "posts_pkey",
		Type:       "primary_key",
		Columns:    nil, // This was the bug
		Definition: "PRIMARY KEY (id)",
	}

	// This should be false (nil != ["id"])
	assert.False(t, sameConstraintContent(sqlConstraint, dbConstraintOld),
		"sameConstraintContent should return false when Columns is nil vs non-nil")

	// Fixed DB-introspected constraint with Columns
	dbConstraintFixed := &model.Constraint{
		Name:       "posts_pkey",
		Type:       "primary_key",
		Columns:    []string{"id"},
		Definition: "PRIMARY KEY (id)",
	}

	// This should be true
	assert.True(t, sameConstraintContent(sqlConstraint, dbConstraintFixed),
		"sameConstraintContent should return true when Columns match")
}
