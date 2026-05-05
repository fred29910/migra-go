package diff

import (
	"strings"
	"testing"

	"github.com/fred29910/migra-go/internal/model"
)

func TestDiffTypes_OrderIsDeterministic(t *testing.T) {
	source := model.NewSchema()
	target := model.NewSchema()
	targetNs := target.GetOrCreateNamespace("public")
	targetNs.Types["z_role"] = &model.EnumType{Name: "z_role", Labels: []string{"a"}}
	targetNs.Types["a_role"] = &model.EnumType{Name: "a_role", Labels: []string{"a"}}
	targetNs.Types["m_role"] = &model.EnumType{Name: "m_role", Labels: []string{"a"}}

	var baseline string
	for i := 0; i < 50; i++ {
		d := NewDiffer()
		ops := d.Diff(source, target)
		sig := make([]string, 0, len(ops))
		for _, op := range ops {
			sig = append(sig, string(op.Kind())+":"+op.ObjectKey().Name)
		}
		joined := strings.Join(sig, ",")
		if i == 0 {
			baseline = joined
			continue
		}
		if joined != baseline {
			t.Fatalf("non-deterministic order: got=%s baseline=%s", joined, baseline)
		}
	}
}

func TestDiffer_WarnsOnDefaultExprChange(t *testing.T) {
	src := model.NewSchema()
	tgt := model.NewSchema()
	srcNs := src.GetOrCreateNamespace("public")
	tgtNs := tgt.GetOrCreateNamespace("public")
	srcTable := model.NewTable("public", "users")
	tgtTable := model.NewTable("public", "users")
	a := "'a'"
	b := "'b'"
	srcTable.AddColumn(&model.Column{Name: "name", DataType: "text", DefaultExpr: &a, IsNullable: true})
	tgtTable.AddColumn(&model.Column{Name: "name", DataType: "text", DefaultExpr: &b, IsNullable: true})
	srcNs.Tables["users"] = srcTable
	tgtNs.Tables["users"] = tgtTable

	d := NewDiffer()
	_ = d.Diff(src, tgt)
	if !strings.Contains(strings.Join(d.Warnings(), "\n"), "default change is not implemented") {
		t.Fatalf("expected default warning, got: %v", d.Warnings())
	}
}

func TestDiffer_WarnsOnUnsupportedDrops(t *testing.T) {
	src := model.NewSchema()
	tgt := model.NewSchema()
	
	// Setup for namespace drop
	src.GetOrCreateNamespace("legacy")
	
	// Setup for column drop in a shared namespace
	srcNs := src.GetOrCreateNamespace("public")
	tgtNs := tgt.GetOrCreateNamespace("public")
	
	srcTbl := model.NewTable("public", "users")
	srcTbl.AddColumn(&model.Column{Name: "obsolete_col", DataType: "integer", IsNullable: true})
	srcNs.Tables["users"] = srcTbl
	
	tgtTbl := model.NewTable("public", "users")
	tgtNs.Tables["users"] = tgtTbl

	d := NewDiffer()
	_ = d.Diff(src, tgt)
	warnings := strings.Join(d.Warnings(), "\n")
	if !strings.Contains(warnings, "namespace drop is not implemented") {
		t.Fatalf("expected namespace warning, got: %s", warnings)
	}
	if !strings.Contains(warnings, "column drop is not implemented") {
		t.Fatalf("expected column warning, got: %s", warnings)
	}
}
