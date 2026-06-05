package render

import (
	"fmt"
	"strings"

	"github.com/fred29910/migra-go/internal/diff"
	"github.com/fred29910/migra-go/internal/util"
)

func renderAddEnumType(r *Renderer, op *diff.AddEnumTypeOp) string {
	labels := make([]string, len(op.Type.Labels))
	for i, label := range op.Type.Labels {
		labels[i] = quoteString(label)
	}
	sql := fmt.Sprintf("CREATE TYPE %s AS ENUM (%s)",
		util.QuoteQualifiedIdentifier(op.Schema, op.Type.Name), strings.Join(labels, ", "))
	return fmt.Sprintf("-- op: add_enum_type risk:low\n%s;", sql)
}

func renderDropEnumType(r *Renderer, op *diff.DropEnumTypeOp) string {
	return fmt.Sprintf("-- op: drop_enum_type risk:high\nDROP TYPE %s%s;",
		ifExistsPrefix(r.useIfExists), util.QuoteQualifiedIdentifier(op.Schema, op.Name))
}

func renderAddEnumLabel(r *Renderer, op *diff.AddEnumLabelOp) string {
	return fmt.Sprintf("-- op: add_enum_label risk:low\nALTER TYPE %s ADD VALUE %s;",
		util.QuoteQualifiedIdentifier(op.Schema, op.Type),
		quoteString(op.Label),
	)
}
