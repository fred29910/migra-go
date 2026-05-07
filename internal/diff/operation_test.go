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
		NewCreateIndexOp("public", &model.Index{Name: "idx_users_email"}),
		NewDropIndexOp("public", "idx_users_email"),
		NewAddEnumTypeOp("public", &model.EnumType{Name: "status"}),
		NewDropEnumTypeOp("public", "status"),
		NewAddConstraintOp("public", "users", &model.Constraint{Name: "chk_email"}),
		NewDropConstraintOp("public", "users", "chk_email"),
	}
	for _, op := range ops {
		deps := op.DependsOn() // 编译时检查此方法存在
		if deps == nil {
			// nil is valid for operations with no dependencies
			continue
		}
		// Verify returned dependencies are valid ObjectKeys
		for _, dep := range deps {
			if dep.Schema == "" && dep.Name == "" {
				t.Errorf("DependsOn returned empty ObjectKey")
			}
		}
	}
}
