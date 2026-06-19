package diff

import (
	"testing"

	"github.com/fred29910/migra-go/internal/model"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestOperationInterfaceHasDependsOn(t *testing.T) {
	// 验证 Operation 接口包含 DependsOn 方法
	// 由于 Go 是结构类型，我们检查具体实现
	ops := []Operation{
		NewAddTableOp("public", "users", &model.Table{}),
		NewDropTableOp("public", "users"),
		NewAddColumnOp("public", "users", &model.Column{Name: "email", DataType: "varchar"}),
		NewAlterColumnTypeOp("public", "users", "email", "varchar(50)", "varchar(100)"),
		NewSetNotNullOp("public", "users", "email"),
		NewDropNotNullOp("public", "users", "email"),
		NewCreateIndexOp("public", &model.Index{Name: "idx_users_email", Table: "users"}),
		NewDropIndexOp("public", "idx_users_email"),
		NewAddEnumTypeOp("public", &model.EnumType{Name: "status"}),
		NewDropEnumTypeOp("public", "status"),
		NewAddConstraintOp("public", "users", &model.Constraint{Name: "chk_email"}),
		NewDropConstraintOp("public", "users", "chk_email"),
		NewAddEnumLabelOp("public", "user_role", "guest"),
		NewSetDefaultOp("public", "users", "email", "now()"),
		NewDropDefaultOp("public", "users", "email"),
		NewDropColumnOp("public", "users", "old_col"),
		NewCreateSchemaOp("auth"),
		NewDropSchemaOp("old_schema"),
		NewSetIdentityOp("public", "users", "id", "ALWAYS"),
		NewDropIdentityOp("public", "users", "id"),
		NewRenameColumnOp("public", "users", "old_name", "new_name"),
	}
	for _, op := range ops {
		deps := op.DependsOn() // 编译时检查此方法存在
		switch op.(type) {
		case *AddConstraintOp:
			// AddConstraint depends on: (1) its table, (2) the constraint itself (to ensure DROP runs before ADD)
			if len(deps) != 2 {
				t.Errorf("%T.DependsOn() should return 2 dependencies, got %d", op, len(deps))
				continue
			}
			expectedTableDep := model.ObjectKey{Schema: "public", Kind: model.KindTable, Name: "users"}
			if deps[0] != expectedTableDep {
				t.Errorf("%T.DependsOn()[0] = %v, want %v", op, deps[0], expectedTableDep)
			}
		case *AddColumnOp, *AlterColumnTypeOp, *SetNotNullOp, *DropNotNullOp, *CreateIndexOp, *SetDefaultOp, *DropDefaultOp, *DropColumnOp, *SetIdentityOp, *DropIdentityOp, *RenameColumnOp:
			// These operations should have exactly one dependency: their table
			if len(deps) != 1 {
				t.Errorf("%T.DependsOn() should return 1 dependency, got %d", op, len(deps))
				continue
			}
			expectedDep := model.ObjectKey{Schema: "public", Kind: model.KindTable, Name: "users"}
			if deps[0] != expectedDep {
				t.Errorf("%T.DependsOn()[0] = %v, want %v", op, deps[0], expectedDep)
			}
		case *AddEnumLabelOp:
			if len(deps) != 1 {
				t.Errorf("%T.DependsOn() should return 1 dependency, got %d", op, len(deps))
				continue
			}
			expectedDep := model.ObjectKey{Schema: "public", Kind: model.KindType, Name: "user_role"}
			if deps[0] != expectedDep {
				t.Errorf("%T.DependsOn()[0] = %v, want %v", op, deps[0], expectedDep)
			}
		case *CreateSchemaOp:
			if deps != nil {
				t.Errorf("%T.DependsOn() should return nil, got %v", op, deps)
			}
			if op.Kind() != KindCreateSchema {
				t.Errorf("%T.Kind() = %v, want %v", op, op.Kind(), KindCreateSchema)
			}
		case *DropSchemaOp:
			if deps != nil {
				t.Errorf("%T.DependsOn() should return nil, got %v", op, deps)
			}
			if op.Kind() != KindDropSchema {
				t.Errorf("%T.Kind() = %v, want %v", op, op.Kind(), KindDropSchema)
			}
		default:
			// These operations should have no dependencies
			if deps != nil {
				t.Errorf("%T.DependsOn() should return nil, got %v", op, deps)
			}
		}
	}
}

func TestNewOperationsMetadata(t *testing.T) {
	enumOp := NewAddEnumLabelOp("public", "user_role", "guest")
	if enumOp.Kind() != KindAddEnumLabel || enumOp.IsDestructive() {
		t.Fatalf("unexpected enum label op metadata")
	}
	if enumOp.ObjectKey() == enumOp.DependsOn()[0] {
		t.Fatalf("enum label object key should be distinct from enum type dependency")
	}
	if len(enumOp.DependsOn()) != 1 || enumOp.DependsOn()[0].Kind != model.KindType {
		t.Fatalf("expected enum type dependency, got %#v", enumOp.DependsOn())
	}

	dropCol := NewDropColumnOp("public", "users", "old_col")
	if !dropCol.IsDestructive() {
		t.Fatal("drop column must be destructive")
	}

	setDefault := NewSetDefaultOp("public", "users", "created_at", "now()")
	if setDefault.Kind() != KindSetDefault || setDefault.IsDestructive() {
		t.Fatalf("unexpected set default op metadata")
	}
	if len(setDefault.DependsOn()) != 1 || setDefault.DependsOn()[0].Kind != model.KindTable {
		t.Fatalf("expected set default table dependency, got %#v", setDefault.DependsOn())
	}

	dropDefault := NewDropDefaultOp("public", "users", "created_at")
	if dropDefault.Kind() != KindDropDefault || dropDefault.IsDestructive() {
		t.Fatalf("unexpected drop default op metadata")
	}
	if len(dropDefault.DependsOn()) != 1 || dropDefault.DependsOn()[0].Kind != model.KindTable {
		t.Fatalf("expected drop default table dependency, got %#v", dropDefault.DependsOn())
	}
}

func TestSetIdentityOp(t *testing.T) {
	op := NewSetIdentityOp("public", "users", "id", "ALWAYS")
	assert.Equal(t, KindSetIdentity, op.Kind())
	assert.Equal(t, "public", op.Schema)
	assert.Equal(t, "users", op.Table)
	assert.Equal(t, "id", op.Column)
	assert.Equal(t, "ALWAYS", op.IdentityKind)
	assert.False(t, op.IsDestructive())
	deps := op.DependsOn()
	require.Len(t, deps, 1)
	assert.Equal(t, "public", deps[0].Schema)
	assert.Equal(t, "users", deps[0].Name)
}

func TestDropIdentityOp(t *testing.T) {
	op := NewDropIdentityOp("public", "users", "id")
	assert.Equal(t, KindDropIdentity, op.Kind())
	assert.Equal(t, "public", op.Schema)
	assert.Equal(t, "users", op.Table)
	assert.Equal(t, "id", op.Column)
	assert.True(t, op.IsDestructive())
}
