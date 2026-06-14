package diff

import (
	"fmt"

	"github.com/fred29910/migra-go/internal/model"
	"github.com/fred29910/migra-go/internal/util"
)

// CreateSchemaOp represents the operation of creating a schema.
type CreateSchemaOp struct {
	baseOperation
	Schema string
}

// NewCreateSchemaOp creates a new CreateSchemaOp for the given schema name.
func NewCreateSchemaOp(schema string) *CreateSchemaOp {
	return &CreateSchemaOp{
		baseOperation: baseOperation{
			kind:      KindCreateSchema,
			objectKey: model.NewObjectKey(schema, "", model.KindSchema),
		},
		Schema: schema,
	}
}

// IsDestructive returns whether the operation is destructive.
func (op *CreateSchemaOp) IsDestructive() bool {
	return false
}

// RenderString renders the operation as a SQL string.
func (op *CreateSchemaOp) RenderString(ctx RenderContext) string {
	return fmt.Sprintf("-- op: create_schema risk:low\nCREATE SCHEMA IF NOT EXISTS %s;",
		util.QuoteIdentifier(op.Schema))
}

// DropSchemaOp represents the operation of dropping a schema.
type DropSchemaOp struct {
	baseOperation
	Schema string
}

// NewDropSchemaOp creates a new DropSchemaOp for the given schema name.
func NewDropSchemaOp(schema string) *DropSchemaOp {
	return &DropSchemaOp{
		baseOperation: baseOperation{
			kind:      KindDropSchema,
			objectKey: model.NewObjectKey(schema, "", model.KindSchema),
		},
		Schema: schema,
	}
}

// IsDestructive returns whether the operation is destructive.
func (op *DropSchemaOp) IsDestructive() bool {
	return true
}

// RenderString renders the operation as a SQL string.
func (op *DropSchemaOp) RenderString(ctx RenderContext) string {
	return fmt.Sprintf("-- op: drop_schema risk:high\nDROP SCHEMA IF EXISTS %s;",
		util.QuoteIdentifier(op.Schema))
}
