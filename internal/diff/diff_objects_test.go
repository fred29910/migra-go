package diff

import (
	"context"
	"testing"

	"github.com/fred29910/migra-go/internal/model"
)

func TestDiffMaterializedView_AddDropReplaceChangeType(t *testing.T) {
	t.Run("add materialized view", func(t *testing.T) {
		source := model.NewSchema()
		target := model.NewSchema()
		tgtNS := target.GetOrCreateNamespace("public")
		tgtNS.Views["mv_summary"] = &model.View{Name: "mv_summary", Definition: "SELECT count(*) FROM users", Materialized: true}

ops, _, err := NewDiffer().Diff(context.Background(), source, target)
		if err != nil {
			t.Fatalf("Diff: %v", err)
		}
		// Filter to only materialized view ops (there may also be a CreateSchemaOp)
		var mvOps []Operation
		for _, op := range ops {
			if op.Kind() == KindCreateMaterializedView {
				mvOps = append(mvOps, op)
			}
		}
		if len(mvOps) != 1 {
			t.Fatalf("expected 1 CreateMaterializedView op, got %d from all ops: %#v", len(mvOps), ops)
		}
	})

	t.Run("drop materialized view", func(t *testing.T) {
		source := model.NewSchema()
		srcNS := source.GetOrCreateNamespace("public")
		srcNS.Views["mv_old"] = &model.View{Name: "mv_old", Definition: "SELECT 1", Materialized: true}
		target := model.NewSchema()
		target.GetOrCreateNamespace("public") // ensure target has the namespace

ops, _, err := NewDiffer().Diff(context.Background(), source, target)
		if err != nil {
			t.Fatalf("Diff: %v", err)
		}
		var mvOps []Operation
		for _, op := range ops {
			if op.Kind() == KindDropMaterializedView {
				mvOps = append(mvOps, op)
			}
		}
		if len(mvOps) != 1 {
			t.Fatalf("expected 1 DropMaterializedView op, got %d from all ops: %#v", len(mvOps), ops)
		}
	})

	t.Run("regular view and materialized view are independent", func(t *testing.T) {
		source := model.NewSchema()
		srcNS := source.GetOrCreateNamespace("public")
		srcNS.Views["v_users"] = &model.View{Name: "v_users", Definition: "SELECT id FROM users", Materialized: false}
		srcNS.Views["mv_users"] = &model.View{Name: "mv_users", Definition: "SELECT id FROM users", Materialized: true}

		target := model.NewSchema()
		tgtNS := target.GetOrCreateNamespace("public")
		tgtNS.Views["v_users"] = &model.View{Name: "v_users", Definition: "SELECT id FROM users", Materialized: false}
		tgtNS.Views["mv_users"] = &model.View{Name: "mv_users", Definition: "SELECT id FROM users", Materialized: true}

ops, _, err := NewDiffer().Diff(context.Background(), source, target)
		if err != nil {
			t.Fatalf("Diff: %v", err)
		}
		if len(ops) != 0 {
			t.Fatalf("expected 0 ops (identical), got %d: %#v", len(ops), ops)
		}
	})

	t.Run("view type change regular->materialized", func(t *testing.T) {
		source := model.NewSchema()
		srcNS := source.GetOrCreateNamespace("public")
		srcNS.Views["myview"] = &model.View{Name: "myview", Definition: "SELECT 1", Materialized: false}

		target := model.NewSchema()
		tgtNS := target.GetOrCreateNamespace("public")
		tgtNS.Views["myview"] = &model.View{Name: "myview", Definition: "SELECT 1", Materialized: true}

ops, _, err := NewDiffer().Diff(context.Background(), source, target)
		if err != nil {
			t.Fatalf("Diff: %v", err)
		}
		// Should drop old view + create new materialized view
		if len(ops) != 2 {
			t.Fatalf("expected 2 ops, got %d: %#v", len(ops), ops)
		}
		kinds := map[Kind]bool{}
		for _, op := range ops {
			kinds[op.Kind()] = true
		}
		if !kinds[KindDropView] || !kinds[KindCreateMaterializedView] {
			t.Fatalf("expected DropView+CreateMaterializedView, got %v", kinds)
		}
	})
}

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

ops, _, err := NewDiffer().Diff(context.Background(), source, target)
	if err != nil {
		t.Fatalf("Diff: %v", err)
	}
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
