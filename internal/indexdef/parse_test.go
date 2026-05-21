package indexdef

import (
	"testing"

	"github.com/fred29910/migra-go/internal/model"
)

func TestParseElement_AdvancedColumn(t *testing.T) {
	got, err := ParseElement(`email COLLATE "C" text_pattern_ops DESC NULLS LAST`)
	if err != nil {
		t.Fatalf("ParseElement failed: %v", err)
	}
	want := model.IndexElem{
		Name:          "email",
		Collation:     "C",
		Opclass:       "text_pattern_ops",
		Ordering:      "DESC",
		NullsOrdering: "LAST",
	}
	if got != want {
		t.Fatalf("got %#v, want %#v", got, want)
	}
}

func TestParseElement_Expression(t *testing.T) {
	got, err := ParseElement(`(lower(email))`)
	if err != nil {
		t.Fatalf("ParseElement failed: %v", err)
	}
	if got.Expr != "lower(email)" {
		t.Fatalf("expected expression lower(email), got %#v", got)
	}
}
