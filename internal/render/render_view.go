package render

import (
	"fmt"

	"github.com/fred29910/migra-go/internal/diff"
	"github.com/fred29910/migra-go/internal/util"
)

func renderCreateView(r *Renderer, op *diff.CreateViewOp) string {
	return fmt.Sprintf("-- op: create_view risk:low\nCREATE VIEW %s AS %s;",
		util.QuoteQualifiedIdentifier(op.Schema, op.View.Name), op.View.Definition)
}

func renderReplaceView(r *Renderer, op *diff.ReplaceViewOp) string {
	return fmt.Sprintf("-- op: replace_view risk:medium\nCREATE OR REPLACE VIEW %s AS %s;",
		util.QuoteQualifiedIdentifier(op.Schema, op.View.Name), op.View.Definition)
}

func renderDropView(r *Renderer, op *diff.DropViewOp) string {
	return fmt.Sprintf("-- op: drop_view risk:high\nDROP VIEW %s%s;",
		ifExistsPrefix(r.useIfExists), util.QuoteQualifiedIdentifier(op.Schema, op.Name))
}

func renderCreateMaterializedView(r *Renderer, op *diff.CreateMaterializedViewOp) string {
	return fmt.Sprintf("-- op: create_materialized_view risk:low\nCREATE MATERIALIZED VIEW %s AS %s;",
		util.QuoteQualifiedIdentifier(op.Schema, op.MaterializedView.Name), op.MaterializedView.Definition)
}

func renderDropMaterializedView(r *Renderer, op *diff.DropMaterializedViewOp) string {
	return fmt.Sprintf("-- op: drop_materialized_view risk:high\nDROP MATERIALIZED VIEW %s%s;",
		ifExistsPrefix(r.useIfExists), util.QuoteQualifiedIdentifier(op.Schema, op.Name))
}
