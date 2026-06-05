package render

import (
	"fmt"

	"github.com/fred29910/migra-go/internal/diff"
	"github.com/fred29910/migra-go/internal/util"
)

func renderCreateSchema(r *Renderer, op *diff.CreateSchemaOp) string {
	return fmt.Sprintf("-- op: create_schema risk:low\nCREATE SCHEMA IF NOT EXISTS %s;",
		util.QuoteIdentifier(op.Schema))
}

func renderDropSchema(r *Renderer, op *diff.DropSchemaOp) string {
	return fmt.Sprintf("-- op: drop_schema risk:high\nDROP SCHEMA IF EXISTS %s;",
		util.QuoteIdentifier(op.Schema))
}
