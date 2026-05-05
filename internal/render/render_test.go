package render

import (
	"strings"
	"testing"

	"github.com/fred29910/migra-go/internal/diff"
)

func TestRenderer_RenderAll_IdempotentAcrossCalls(t *testing.T) {
	r := NewRenderer()
	ops := []diff.Operation{diff.NewDropIndexOp("public", "idx_a")}
	got1 := r.RenderAll(ops)
	got2 := r.RenderAll(ops)
	if got1 != got2 {
		t.Fatalf("expected stable output, got1=%q got2=%q", got1, got2)
	}
}

func TestRenderOutput_SupportsSQLAndJSON(t *testing.T) {
	ops := []diff.Operation{diff.NewDropIndexOp("public", "idx_a")}
	r := NewRenderer()
	sql, err := r.RenderOutput(ops, "sql")
	if err != nil || !strings.Contains(sql, "DROP INDEX") {
		t.Fatalf("unexpected sql render: %v %q", err, sql)
	}
	js, err := r.RenderOutput(ops, "json")
	if err != nil || !strings.Contains(js, `"kind"`) {
		t.Fatalf("unexpected json render: %v %q", err, js)
	}
}
