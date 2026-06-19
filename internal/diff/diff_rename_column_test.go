package diff

import (
	"context"
	"testing"

	"github.com/fred29910/migra-go/internal/model"
)

func TestDiffer_DetectsColumnRename(t *testing.T) {
	source := model.NewSchema()
	sourceNs := source.GetOrCreateNamespace("public")
	sourceTable := model.NewTable("public", "users")
	sourceTable.AddColumn(&model.Column{Name: "id", DataType: "integer", IsNullable: false})
	sourceTable.AddColumn(&model.Column{Name: "username", DataType: "text", IsNullable: true})
	sourceTable.AddColumn(&model.Column{Name: "email", DataType: "text", IsNullable: false})
	sourceNs.Tables["users"] = sourceTable

	target := model.NewSchema()
	targetNs := target.GetOrCreateNamespace("public")
	targetTable := model.NewTable("public", "users")
	targetTable.AddColumn(&model.Column{Name: "id", DataType: "integer", IsNullable: false})
	targetTable.AddColumn(&model.Column{Name: "login_name", DataType: "text", IsNullable: true})
	targetTable.AddColumn(&model.Column{Name: "email", DataType: "text", IsNullable: false})
	targetNs.Tables["users"] = targetTable

ops, warnings, err := NewDiffer().Diff(context.Background(), source, target)
	if err != nil {
		t.Fatalf("Diff: %v", err)
	}
	if len(warnings) != 0 {
		t.Fatalf("unexpected warnings: %v", warnings)
	}

	var hasRename, hasDrop, hasAdd bool
	for _, op := range ops {
		switch op.Kind() {
		case KindRenameColumn:
			hasRename = true
			rop := op.(*RenameColumnOp)
			if rop.OldName != "username" || rop.NewName != "login_name" {
				t.Fatalf("expected rename username -> login_name, got %s -> %s",
					rop.OldName, rop.NewName)
			}
		case KindDropColumn:
			hasDrop = true
		case KindAddColumn:
			hasAdd = true
		}
	}
	if !hasRename {
		t.Fatal("expected RenameColumnOp, got none")
	}
	if hasDrop || hasAdd {
		t.Fatal("expected no DropColumn/AddColumn when rename is detected")
	}
}

func TestDiffer_NoFalsePositiveOnTypeMismatch(t *testing.T) {
	source := model.NewSchema()
	sourceNs := source.GetOrCreateNamespace("public")
	st := model.NewTable("public", "users")
	st.AddColumn(&model.Column{Name: "username", DataType: "text", IsNullable: true})
	sourceNs.Tables["users"] = st

	target := model.NewSchema()
	targetNs := target.GetOrCreateNamespace("public")
	tt := model.NewTable("public", "users")
	tt.AddColumn(&model.Column{Name: "login_name", DataType: "integer", IsNullable: true})
	targetNs.Tables["users"] = tt

ops, _, err := NewDiffer().Diff(context.Background(), source, target)
	if err != nil {
		t.Fatalf("Diff: %v", err)
	}
	for _, op := range ops {
		if op.Kind() == KindRenameColumn {
			t.Fatal("should not detect rename when types differ")
		}
	}
}

func TestDiffer_RenameWithAddColumn(t *testing.T) {
	source := model.NewSchema()
	sourceNs := source.GetOrCreateNamespace("public")
	st := model.NewTable("public", "users")
	st.AddColumn(&model.Column{Name: "username", DataType: "text", IsNullable: true})
	sourceNs.Tables["users"] = st

	target := model.NewSchema()
	targetNs := target.GetOrCreateNamespace("public")
	tt := model.NewTable("public", "users")
	tt.AddColumn(&model.Column{Name: "login_name", DataType: "text", IsNullable: true})
	tt.AddColumn(&model.Column{Name: "age", DataType: "integer", IsNullable: true})
	targetNs.Tables["users"] = tt

ops, _, err := NewDiffer().Diff(context.Background(), source, target)
	if err != nil {
		t.Fatalf("Diff: %v", err)
	}
	var hasRename, hasAdd bool
	for _, op := range ops {
		switch op.Kind() {
		case KindRenameColumn:
			hasRename = true
		case KindAddColumn:
			hasAdd = true
		}
	}
	if !hasRename {
		t.Fatal("expected rename op")
	}
	if !hasAdd {
		t.Fatal("expected add op for age")
	}
}

func TestDiffer_RenameWithDropColumn(t *testing.T) {
	source := model.NewSchema()
	sourceNs := source.GetOrCreateNamespace("public")
	st := model.NewTable("public", "users")
	st.AddColumn(&model.Column{Name: "username", DataType: "text", IsNullable: true})
	st.AddColumn(&model.Column{Name: "age", DataType: "integer", IsNullable: true})
	sourceNs.Tables["users"] = st

	target := model.NewSchema()
	targetNs := target.GetOrCreateNamespace("public")
	tt := model.NewTable("public", "users")
	tt.AddColumn(&model.Column{Name: "login_name", DataType: "text", IsNullable: true})
	targetNs.Tables["users"] = tt

ops, _, err := NewDiffer().Diff(context.Background(), source, target)
	if err != nil {
		t.Fatalf("Diff: %v", err)
	}
	var hasRename, hasDrop bool
	for _, op := range ops {
		switch op.Kind() {
		case KindRenameColumn:
			hasRename = true
		case KindDropColumn:
			hasDrop = true
		}
	}
	if !hasRename {
		t.Fatal("expected rename op")
	}
	if !hasDrop {
		t.Fatal("expected drop op for age")
	}
}

func TestDiffer_RenameNullableChangeDoesNotMatch(t *testing.T) {
	source := model.NewSchema()
	sourceNs := source.GetOrCreateNamespace("public")
	st := model.NewTable("public", "users")
	st.AddColumn(&model.Column{Name: "username", DataType: "text", IsNullable: true})
	sourceNs.Tables["users"] = st

	target := model.NewSchema()
	targetNs := target.GetOrCreateNamespace("public")
	tt := model.NewTable("public", "users")
	tt.AddColumn(&model.Column{Name: "login_name", DataType: "text", IsNullable: false})
	targetNs.Tables["users"] = tt

ops, _, err := NewDiffer().Diff(context.Background(), source, target)
	if err != nil {
		t.Fatalf("Diff: %v", err)
	}
	for _, op := range ops {
		if op.Kind() == KindRenameColumn {
			t.Fatal("should not detect rename when nullable differs")
		}
	}
}

func TestDiffer_RenameWithColumnReorder(t *testing.T) {
	source := model.NewSchema()
	sourceNs := source.GetOrCreateNamespace("public")
	st := model.NewTable("public", "users")
	st.AddColumn(&model.Column{Name: "id", DataType: "integer", IsNullable: false})
	st.AddColumn(&model.Column{Name: "username", DataType: "text", IsNullable: true})
	st.AddColumn(&model.Column{Name: "email", DataType: "text", IsNullable: false})
	sourceNs.Tables["users"] = st

	target := model.NewSchema()
	targetNs := target.GetOrCreateNamespace("public")
	tt := model.NewTable("public", "users")
	tt.AddColumn(&model.Column{Name: "login_name", DataType: "text", IsNullable: true})
	tt.AddColumn(&model.Column{Name: "id", DataType: "integer", IsNullable: false})
	tt.AddColumn(&model.Column{Name: "email", DataType: "text", IsNullable: false})
	targetNs.Tables["users"] = tt

ops, warnings, err := NewDiffer().Diff(context.Background(), source, target)
	if err != nil {
		t.Fatalf("Diff: %v", err)
	}
	if len(warnings) != 0 {
		t.Fatalf("unexpected warnings: %v", warnings)
	}
	var hasRename bool
	for _, op := range ops {
		if op.Kind() == KindRenameColumn {
			hasRename = true
			rop := op.(*RenameColumnOp)
			if rop.OldName != "username" || rop.NewName != "login_name" {
				t.Fatalf("expected username -> login_name, got %s -> %s", rop.OldName, rop.NewName)
			}
		}
	}
	if !hasRename {
		t.Fatal("expected rename even with column reorder")
	}
}
