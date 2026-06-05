package diff

import "github.com/fred29910/migra-go/internal/model"

type CreateExtensionOp struct {
	baseOperation
	Schema    string
	Extension *model.Extension
}

func NewCreateExtensionOp(schema string, ext *model.Extension) *CreateExtensionOp {
	return &CreateExtensionOp{
		baseOperation: baseOperation{
			kind:      KindCreateExtension,
			objectKey: model.NewObjectKey(schema, ext.Name, model.KindExtension),
		},
		Schema:    schema,
		Extension: ext,
	}
}

func (op *CreateExtensionOp) IsDestructive() bool { return false }

type DropExtensionOp struct {
	baseOperation
	Schema string
	Name   string
}

func NewDropExtensionOp(schema, name string) *DropExtensionOp {
	return &DropExtensionOp{
		baseOperation: baseOperation{
			kind:      KindDropExtension,
			objectKey: model.NewObjectKey(schema, name, model.KindExtension),
		},
		Schema: schema,
		Name:   name,
	}
}

func (op *DropExtensionOp) IsDestructive() bool { return true }

type AlterExtensionUpdateOp struct {
	baseOperation
	Schema    string
	Extension *model.Extension
}

func NewAlterExtensionUpdateOp(schema string, ext *model.Extension) *AlterExtensionUpdateOp {
	return &AlterExtensionUpdateOp{
		baseOperation: baseOperation{
			kind:      KindAlterExtensionUpdate,
			objectKey: model.NewObjectKey(schema, ext.Name, model.KindExtension),
		},
		Schema:    schema,
		Extension: ext,
	}
}

func (op *AlterExtensionUpdateOp) IsDestructive() bool { return false }
