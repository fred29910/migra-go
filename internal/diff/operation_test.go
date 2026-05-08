package diff

import (
	"testing"

	"github.com/fred29910/migra-go/internal/model"
)

func TestOperationInterfaceHasDependsOn(t *testing.T) {
	// 验证 Operation 接口包含 DependsOn 方法
	// 由于 Go 是结构类型，我们检查具体实现
	ops := []Operation{
		NewAddTableOp("public", "users", &model.Table{}),
		NewDropTableOp("public", "users"),
		NewAddColumnOp("public", "users", &model.Column{Name: "email"}),
		NewAlterColumnTypeOp("public", "users", "email", "varchar(50)", "varchar(100)"),
		NewSetNotNullOp("public", "users", "email"),
		NewDropNotNullOp("public", "users", "email"),
		NewCreateIndexOp("public", &model.Index{Name: "idx_users_email", Table: "users"}),
		NewDropIndexOp("public", "idx_users_email"),
		NewAddEnumTypeOp("public", &model.EnumType{Name: "status"}),
		NewDropEnumTypeOp("public", "status"),
		NewAddConstraintOp("public", "users", &model.Constraint{Name: "chk_email"}),
		NewDropConstraintOp("public", "users", "chk_email"),
	}
	for _, op := range ops {
		deps := op.DependsOn() // 编译时检查此方法存在
		switch op.(type) {
		case *AddColumnOp, *AlterColumnTypeOp, *SetNotNullOp, *DropNotNullOp, *AddConstraintOp, *CreateIndexOp:
			// These operations should have exactly one dependency: their table
			if len(deps) != 1 {
				t.Errorf("%T.DependsOn() should return 1 dependency, got %d", op, len(deps))
				continue
			}
			expectedDep := model.ObjectKey{Schema: "public", Kind: model.KindTable, Name: "users"}
			if deps[0] != expectedDep {
				t.Errorf("%T.DependsOn()[0] = %v, want %v", op, deps[0], expectedDep)
			}
		default:
			// These operations should have no dependencies
			if deps != nil {
				t.Errorf("%T.DependsOn() should return nil, got %v", op, deps)
			}
		}
	}
}
