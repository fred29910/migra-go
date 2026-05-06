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
