package diff

import (
	"fmt"

	"github.com/fred29910/migra-go/internal/model"
	"github.com/fred29910/migra-go/internal/util"
)

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

func (op *CreateSchemaOp) RenderString(ctx RenderContext) string {
	return fmt.Sprintf("-- op: create_schema risk:low\nCREATE SCHEMA IF NOT EXISTS %s;",
		util.QuoteIdentifier(op.Schema))
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

func (op *DropSchemaOp) RenderString(ctx RenderContext) string {
	return fmt.Sprintf("-- op: drop_schema risk:high\nDROP SCHEMA IF EXISTS %s;",
		util.QuoteIdentifier(op.Schema))
}
