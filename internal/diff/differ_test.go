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
