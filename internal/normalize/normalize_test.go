package normalize

import (
	"testing"

	"github.com/fred29910/migra-go/internal/model"
)

func TestCanonicalizeSchema_InPlace(t *testing.T) {
	s := model.NewSchema()
	ns := s.GetOrCreateNamespace("Public")
	tbl := model.NewTable("Public", "Users")
	d := "( now() )"
	tbl.AddColumn(&model.Column{Name: "Name", DataType: "INT4", DefaultExpr: &d, IsNullable: true})
	ns.Tables["Users"] = tbl

	beforePtr := tbl.Columns[0]
	if err := CanonicalizeSchema(s); err != nil {
		t.Fatal(err)
	}
	afterPtr := s.Schemas["Public"].Tables["Users"].Columns[0]
	if beforePtr != afterPtr {
		t.Fatal("expected in-place mutation without replacing column pointer")
	}
	if afterPtr.DataType != "integer" {
		t.Fatalf("expected normalized type integer, got %s", afterPtr.DataType)
	}
}
