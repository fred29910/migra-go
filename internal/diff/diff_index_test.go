package diff

import (
	"testing"

	"github.com/fred29910/migra-go/internal/model"
)

func TestDiffIndex_ContentChange(t *testing.T) {
	// Source: regular index
	sourceSchema := model.NewSchema()
	ns := sourceSchema.GetOrCreateNamespace("public")
	sourceTable := model.NewTable("public", "users")
	sourceTable.Indexes["idx_users_email"] = &model.Index{
		Name:     "idx_users_email",
		Table:    "users",
		Elements: []model.IndexElem{{Name: "email"}},
		Unique:   false,
		Method:   "btree",
	}
	ns.Tables["users"] = sourceTable

	// Target: unique index (content changed)
	targetSchema := model.NewSchema()
	ns2 := targetSchema.GetOrCreateNamespace("public")
	targetTable := model.NewTable("public", "users")
	targetTable.Indexes["idx_users_email"] = &model.Index{
		Name:     "idx_users_email",
		Table:    "users",
		Elements: []model.IndexElem{{Name: "email"}},
		Unique:   true, // changed
		Method:   "btree",
	}
	ns2.Tables["users"] = targetTable

	d := NewDiffer()
	ops, _ := d.Diff(sourceSchema, targetSchema)

	// Should detect change and generate DROP + CREATE
	foundDrop := false
	foundCreate := false
	for _, op := range ops {
		if _, ok := op.(*DropIndexOp); ok {
			foundDrop = true
		}
		if createOp, ok := op.(*CreateIndexOp); ok {
			if createOp.Index.Unique {
				foundCreate = true
			}
		}
	}

	if !foundDrop {
		t.Error("expected DROP INDEX operation")
	}
	if !foundCreate {
		t.Error("expected CREATE INDEX operation with Unique=true")
	}
}

func TestDiffIndex_SameContent(t *testing.T) {
	// Source and Target have same index content
	sourceSchema := model.NewSchema()
	ns := sourceSchema.GetOrCreateNamespace("public")
	sourceTable := model.NewTable("public", "users")
	sourceTable.Indexes["idx_users_email"] = &model.Index{
		Name:     "idx_users_email",
		Table:    "users",
		Elements: []model.IndexElem{{Name: "email"}},
		Unique:   false,
		Method:   "btree",
	}
	ns.Tables["users"] = sourceTable

	targetSchema := model.NewSchema()
	ns2 := targetSchema.GetOrCreateNamespace("public")
	targetTable := model.NewTable("public", "users")
	targetTable.Indexes["idx_users_email"] = &model.Index{
		Name:     "idx_users_email",
		Table:    "users",
		Elements: []model.IndexElem{{Name: "email"}},
		Unique:   false,
		Method:   "btree",
	}
	ns2.Tables["users"] = targetTable

	d := NewDiffer()
	ops, _ := d.Diff(sourceSchema, targetSchema)

	// Should NOT generate any index operations
	for _, op := range ops {
		if _, ok := op.(*DropIndexOp); ok {
			t.Error("should not generate DROP INDEX for same content")
		}
		if _, ok := op.(*CreateIndexOp); ok {
			t.Error("should not generate CREATE INDEX for same content")
		}
	}
}

func TestDiffIndex_ExpressionChange(t *testing.T) {
	// Source: column index
	sourceSchema := model.NewSchema()
	ns := sourceSchema.GetOrCreateNamespace("public")
	sourceTable := model.NewTable("public", "users")
	sourceTable.Indexes["idx_users_lower"] = &model.Index{
		Name:     "idx_users_lower",
		Table:    "users",
		Elements: []model.IndexElem{{Name: "email"}},
	}
	ns.Tables["users"] = sourceTable

	// Target: expression index
	targetSchema := model.NewSchema()
	ns2 := targetSchema.GetOrCreateNamespace("public")
	targetTable := model.NewTable("public", "users")
	targetTable.Indexes["idx_users_lower"] = &model.Index{
		Name:     "idx_users_lower",
		Table:    "users",
		Elements: []model.IndexElem{{Expr: "lower(email)"}},
	}
	ns2.Tables["users"] = targetTable

	d := NewDiffer()
	ops, _ := d.Diff(sourceSchema, targetSchema)

	// Should detect change
	foundDrop := false
	foundCreate := false
	for _, op := range ops {
		if _, ok := op.(*DropIndexOp); ok {
			foundDrop = true
		}
		if _, ok := op.(*CreateIndexOp); ok {
			foundCreate = true
		}
	}

	if !foundDrop {
		t.Error("expected DROP INDEX operation for expression change")
	}
	if !foundCreate {
		t.Error("expected CREATE INDEX operation for expression change")
	}
}
