package diff

import "github.com/fred29910/migra-go/internal/model"

type CreateSchemaOp struct {
	baseOperation
	Schema string
}

func NewCreateSchemaOp(schema string) *CreateSchemaOp {
	return &CreateSchemaOp{
		baseOperation: baseOperation{
			kind:      KindCreateSchema,
			objectKey: model.NewObjectKey(schema, "", model.KindSchema),
		},
		Schema: schema,
	}
}

func (op *CreateSchemaOp) IsDestructive() bool {
	return false
}

type DropSchemaOp struct {
	baseOperation
	Schema string
}

func NewDropSchemaOp(schema string) *DropSchemaOp {
	return &DropSchemaOp{
		baseOperation: baseOperation{
			kind:      KindDropSchema,
			objectKey: model.NewObjectKey(schema, "", model.KindSchema),
		},
		Schema: schema,
	}
}

func (op *DropSchemaOp) IsDestructive() bool {
	return true
}
