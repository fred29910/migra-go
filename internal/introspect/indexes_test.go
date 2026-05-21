package introspect

import "testing"

func TestParseIndexElementDefinitions(t *testing.T) {
	got, err := parseIndexElementDefinitions([]string{
		`email COLLATE "C" text_pattern_ops DESC NULLS LAST`,
		`(lower(name))`,
	})
	if err != nil {
		t.Fatalf("parseIndexElementDefinitions failed: %v", err)
	}
	if len(got) != 2 {
		t.Fatalf("expected 2 elements, got %d", len(got))
	}
	if got[0].Name != "email" || got[0].Collation != "C" || got[0].Opclass != "text_pattern_ops" || got[0].Ordering != "DESC" || got[0].NullsOrdering != "LAST" {
		t.Fatalf("unexpected first element: %#v", got[0])
	}
	if got[1].Expr != "lower(name)" {
		t.Fatalf("unexpected expression element: %#v", got[1])
	}
}
