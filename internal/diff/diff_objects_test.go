package diff

import (
	"testing"

	"github.com/fred29910/migra-go/internal/model"
)

func TestDiffP3Objects_AddChangeDrop(t *testing.T) {
	source := model.NewSchema()
	srcNS := source.GetOrCreateNamespace("public")
	srcNS.Views["old_view"] = &model.View{Name: "old_view", Definition: "SELECT 1"}
	srcNS.Sequences["invoice_id_seq"] = &model.Sequence{Name: "invoice_id_seq", DataType: "bigint", StartValue: 1}
	srcNS.Extensions["pgcrypto"] = &model.Extension{Name: "pgcrypto", Version: "1.2"}

	target := model.NewSchema()
	tgtNS := target.GetOrCreateNamespace("public")
	tgtNS.Views["active_users"] = &model.View{Name: "active_users", Definition: "SELECT id FROM users"}
	tgtNS.Sequences["invoice_id_seq"] = &model.Sequence{Name: "invoice_id_seq", DataType: "bigint", StartValue: 100}
	tgtNS.Extensions["pgcrypto"] = &model.Extension{Name: "pgcrypto", Version: "1.3"}

	ops, _ := NewDiffer().Diff(source, target)
	kinds := map[Kind]bool{}
	for _, op := range ops {
		kinds[op.Kind()] = true
	}
	for _, want := range []Kind{KindCreateView, KindDropView, KindAlterSequence, KindAlterExtensionUpdate} {
		if !kinds[want] {
			t.Fatalf("expected %s in ops %#v", want, ops)
		}
	}
}
