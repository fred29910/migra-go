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
