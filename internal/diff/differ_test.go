package diff

import (
	"testing"

	"github.com/fred29910/migra-go/internal/model"
)

func TestDiffer_DiffNoSharedState(t *testing.T) {
	d := NewDiffer()
	src := model.NewSchema()
	tgt := model.NewSchema()
	tgt.GetOrCreateNamespace("public")
	for i := 0; i < 20; i++ {
		ops, _ := d.Diff(src, tgt)
		if len(ops) != 0 {
			t.Fatalf("expected 0 ops, got %d", len(ops))
		}
	}
}

func TestDiffer_EnumLabelAppendDefaultChangeAndDropColumn(t *testing.T) {
	source := model.NewSchema()
	sourceNs := source.GetOrCreateNamespace("public")
	sourceNs.Types["user_role"] = &model.EnumType{Name: "user_role", Labels: []string{"admin", "user"}}
	sourceTable := model.NewTable("public", "users")
	sourceTable.AddColumn(&model.Column{Name: "id", DataType: "integer", IsNullable: false})
	sourceTable.AddColumn(&model.Column{Name: "old_col", DataType: "text", IsNullable: true})
	sourceNs.Tables["users"] = sourceTable

	target := model.NewSchema()
	targetNs := target.GetOrCreateNamespace("public")
	targetNs.Types["user_role"] = &model.EnumType{Name: "user_role", Labels: []string{"admin", "user", "guest"}}
	targetTable := model.NewTable("public", "users")
	defaultExpr := "now()"
	targetTable.AddColumn(&model.Column{Name: "id", DataType: "integer", IsNullable: false, DefaultExpr: &defaultExpr})
	targetNs.Tables["users"] = targetTable

	ops, warnings := NewDiffer().Diff(source, target)
	if len(warnings) != 0 {
		t.Fatalf("unexpected warnings: %#v", warnings)
	}
	kinds := map[Kind]bool{}
	for _, op := range ops {
		kinds[op.Kind()] = true
	}
	if !kinds[KindAddEnumLabel] || !kinds[KindSetDefault] || !kinds[KindDropColumn] {
		t.Fatalf("missing expected operations, got %#v", kinds)
	}
}

func TestDiffer_DetectsSameNameConstraintContentChange(t *testing.T) {
	source := model.NewSchema()
	sourceNs := source.GetOrCreateNamespace("public")
	sourceTable := model.NewTable("public", "comments")
	sourceTable.Constraints["comments_post_id_fkey"] = &model.Constraint{
		Name:       "comments_post_id_fkey",
		Type:       "foreign_key",
		Table:      "comments",
		Columns:    []string{"post_id"},
		RefSchema:  "public",
		RefTable:   "posts",
		RefColumns: []string{"id"},
	}
	sourceNs.Tables["comments"] = sourceTable

	target := model.NewSchema()
	targetNs := target.GetOrCreateNamespace("public")
	targetTable := model.NewTable("public", "comments")
	targetTable.Constraints["comments_post_id_fkey"] = &model.Constraint{
		Name:       "comments_post_id_fkey",
		Type:       "foreign_key",
		Table:      "comments",
		Columns:    []string{"post_id"},
		RefSchema:  "public",
		RefTable:   "articles",
		RefColumns: []string{"id"},
	}
	targetNs.Tables["comments"] = targetTable

	ops, _ := NewDiffer().Diff(source, target)
	var hasDrop, hasAdd bool
	for _, op := range ops {
		switch op.Kind() {
		case KindDropConstraint:
			hasDrop = true
		case KindAddConstraint:
			hasAdd = true
		}
	}
	if !hasDrop || !hasAdd {
		t.Fatalf("expected drop+add for same-name constraint change, got %#v", ops)
	}
}
