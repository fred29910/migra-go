package render

import (
	"fmt"

	"github.com/fred29910/migra-go/internal/diff"
	"github.com/fred29910/migra-go/internal/util"
)

func renderCreateExtension(r *Renderer, op *diff.CreateExtensionOp) string {
	sql := "CREATE EXTENSION IF NOT EXISTS " + util.QuoteIdentifier(op.Extension.Name)
	if op.Extension.Version != "" {
		sql += " WITH VERSION " + quoteString(op.Extension.Version)
	}
	return "-- op: create_extension risk:low\n" + sql + ";"
}

func renderDropExtension(r *Renderer, op *diff.DropExtensionOp) string {
	return fmt.Sprintf("-- op: drop_extension risk:high\nDROP EXTENSION %s%s;",
		ifExistsPrefix(r.useIfExists), util.QuoteIdentifier(op.Name))
}

func renderAlterExtensionUpdate(r *Renderer, op *diff.AlterExtensionUpdateOp) string {
	sql := fmt.Sprintf("ALTER EXTENSION %s UPDATE TO %s",
		util.QuoteIdentifier(op.Extension.Name), quoteString(op.Extension.Version))
	return "-- op: alter_extension_update risk:low\n" + sql + ";"
}
