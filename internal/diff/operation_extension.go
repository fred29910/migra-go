package diff

import (
	"fmt"
	"strings"

	"github.com/fred29910/migra-go/internal/model"
	"github.com/fred29910/migra-go/internal/util"
)

func quoteString(s string) string {
	return `'` + strings.ReplaceAll(s, `'`, `''`) + `'`
}

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

func (op *CreateExtensionOp) RenderString(ctx RenderContext) string {
	sql := "CREATE EXTENSION IF NOT EXISTS " + util.QuoteIdentifier(op.Extension.Name)
	if op.Extension.Version != "" {
		sql += " WITH VERSION " + quoteString(op.Extension.Version)
	}
	return "-- op: create_extension risk:low\n" + sql + ";"
}

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

func (op *DropExtensionOp) RenderString(ctx RenderContext) string {
	return fmt.Sprintf("-- op: drop_extension risk:high\nDROP EXTENSION %s%s;",
		ifExistsPrefix(ctx.UseIfExists()), util.QuoteIdentifier(op.Name))
}

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

func (op *AlterExtensionUpdateOp) RenderString(ctx RenderContext) string {
	sql := fmt.Sprintf("ALTER EXTENSION %s UPDATE TO %s",
		util.QuoteIdentifier(op.Extension.Name), quoteString(op.Extension.Version))
	return "-- op: alter_extension_update risk:low\n" + sql + ";"
}
