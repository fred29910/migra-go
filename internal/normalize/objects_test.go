package normalize

import (
	"testing"

	"github.com/fred29910/migra-go/internal/model"
)

func TestCanonicalizeSchema_P3Objects(t *testing.T) {
	s := model.NewSchema()
	ns := s.GetOrCreateNamespace("public")
	ns.Views["v"] = &model.View{Name: "v", Definition: "SELECT  id  FROM users;"}
	ns.Extensions["pgcrypto"] = &model.Extension{Name: `"PgCrypto"`, Version: "1.3"}
	ns.Sequences["seq"] = &model.Sequence{Name: `"Seq"`, DataType: "INT8"}

	if err := CanonicalizeSchema(s); err != nil {
		t.Fatal(err)
	}
	if ns.Views["v"].Definition != "SELECT  id  FROM users" {
		t.Fatalf("unexpected view definition: %q", ns.Views["v"].Definition)
	}
	if ns.Extensions["pgcrypto"].Name != "PgCrypto" {
		t.Fatalf("quoted extension name should preserve case, got %q", ns.Extensions["pgcrypto"].Name)
	}
	if ns.Sequences["seq"].DataType != "bigint" {
		t.Fatalf("expected bigint, got %q", ns.Sequences["seq"].DataType)
	}
}
