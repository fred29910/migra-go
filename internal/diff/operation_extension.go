package diff

import (
	"fmt"
	"strings"

	"github.com/fred29910/migra-go/internal/model"
	"github.com/fred29910/migra-go/internal/util"
)

// quoteString quotes a string value for SQL.
func quoteString(s string) string {
	return `'` + strings.ReplaceAll(s, `'`, `''`) + `'`
}

// CreateExtensionOp represents an operation to create a PostgreSQL extension.
type CreateExtensionOp struct {
	baseOperation
	Schema    string
	Extension *model.Extension
}

// NewCreateExtensionOp creates a new CreateExtensionOp.
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

// IsDestructive returns false; creating an extension is not destructive.
func (op *CreateExtensionOp) IsDestructive() bool { return false }

// RenderString renders the SQL statement for creating an extension.
func (op *CreateExtensionOp) RenderString(ctx RenderContext) string {
	sql := "CREATE EXTENSION IF NOT EXISTS " + util.QuoteIdentifier(op.Extension.Name)
	if op.Extension.Version != "" {
		sql += " WITH VERSION " + quoteString(op.Extension.Version)
	}
	return "-- op: create_extension risk:low\n" + sql + ";"
}

// DropExtensionOp represents an operation to drop a PostgreSQL extension.
type DropExtensionOp struct {
	baseOperation
	Schema string
	Name   string
}

// NewDropExtensionOp creates a new DropExtensionOp.
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

// IsDestructive returns true; dropping an extension is destructive.
func (op *DropExtensionOp) IsDestructive() bool { return true }

// RenderString renders the SQL statement for dropping an extension.
func (op *DropExtensionOp) RenderString(ctx RenderContext) string {
	return fmt.Sprintf("-- op: drop_extension risk:high\nDROP EXTENSION %s%s;",
		ifExistsPrefix(ctx.UseIfExists()), util.QuoteIdentifier(op.Name))
}

// AlterExtensionUpdateOp represents an operation to update a PostgreSQL extension version.
type AlterExtensionUpdateOp struct {
	baseOperation
	Schema    string
	Extension *model.Extension
}

// NewAlterExtensionUpdateOp creates a new AlterExtensionUpdateOp.
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

// IsDestructive returns false; updating an extension is not destructive.
func (op *AlterExtensionUpdateOp) IsDestructive() bool { return false }

// RenderString renders the SQL statement for updating an extension.
func (op *AlterExtensionUpdateOp) RenderString(ctx RenderContext) string {
	sql := fmt.Sprintf("ALTER EXTENSION %s UPDATE TO %s",
		util.QuoteIdentifier(op.Extension.Name), quoteString(op.Extension.Version))
	return "-- op: alter_extension_update risk:low\n" + sql + ";"
}
