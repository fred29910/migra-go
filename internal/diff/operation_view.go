package diff

import (
	"fmt"

	"github.com/fred29910/migra-go/internal/model"
	"github.com/fred29910/migra-go/internal/util"
)

type CreateViewOp struct {
	baseOperation
	Schema string
	View   *model.View
}

func NewCreateViewOp(schema string, view *model.View) *CreateViewOp {
	return &CreateViewOp{
		baseOperation: baseOperation{
			kind:      KindCreateView,
			objectKey: model.NewObjectKey(schema, view.Name, model.KindView),
		},
		Schema: schema,
		View:   view,
	}
}

func (op *CreateViewOp) IsDestructive() bool { return false }

func (op *CreateViewOp) RenderString(ctx RenderContext) string {
	return fmt.Sprintf("-- op: create_view risk:low\nCREATE VIEW %s AS %s;",
		util.QuoteQualifiedIdentifier(op.Schema, op.View.Name), op.View.Definition)
}

type DropViewOp struct {
	baseOperation
	Schema     string
	Name       string
	IsRecreate bool
}

func NewDropViewOp(schema, name string) *DropViewOp {
	return &DropViewOp{
		baseOperation: baseOperation{
			kind:      KindDropView,
			objectKey: model.NewObjectKey(schema, name, model.KindView),
		},
		Schema: schema,
		Name:   name,
	}
}

func (op *DropViewOp) IsDestructive() bool { return !op.IsRecreate }

func (op *DropViewOp) RenderString(ctx RenderContext) string {
	return fmt.Sprintf("-- op: drop_view risk:high\nDROP VIEW %s%s;",
		ifExistsPrefix(ctx.UseIfExists()), util.QuoteQualifiedIdentifier(op.Schema, op.Name))
}

type ReplaceViewOp struct {
	baseOperation
	Schema string
	View   *model.View
}

func NewReplaceViewOp(schema string, view *model.View) *ReplaceViewOp {
	return &ReplaceViewOp{
		baseOperation: baseOperation{
			kind:      KindReplaceView,
			objectKey: model.NewObjectKey(schema, view.Name, model.KindView),
		},
		Schema: schema,
		View:   view,
	}
}

func (op *ReplaceViewOp) IsDestructive() bool { return false }

func (op *ReplaceViewOp) RenderString(ctx RenderContext) string {
	return fmt.Sprintf("-- op: replace_view risk:medium\nCREATE OR REPLACE VIEW %s AS %s;",
		util.QuoteQualifiedIdentifier(op.Schema, op.View.Name), op.View.Definition)
}

type CreateMaterializedViewOp struct {
	baseOperation
	Schema           string
	MaterializedView *model.View
}

func NewCreateMaterializedViewOp(schema string, view *model.View) *CreateMaterializedViewOp {
	return &CreateMaterializedViewOp{
		baseOperation: baseOperation{
			kind:      KindCreateMaterializedView,
			objectKey: model.NewObjectKey(schema, view.Name, model.KindView),
		},
		Schema:           schema,
		MaterializedView: view,
	}
}

func (op *CreateMaterializedViewOp) IsDestructive() bool { return false }

func (op *CreateMaterializedViewOp) RenderString(ctx RenderContext) string {
	return fmt.Sprintf("-- op: create_materialized_view risk:low\nCREATE MATERIALIZED VIEW %s AS %s;",
		util.QuoteQualifiedIdentifier(op.Schema, op.MaterializedView.Name), op.MaterializedView.Definition)
}

type DropMaterializedViewOp struct {
	baseOperation
	Schema string
	Name   string
}

func NewDropMaterializedViewOp(schema, name string) *DropMaterializedViewOp {
	return &DropMaterializedViewOp{
		baseOperation: baseOperation{
			kind:      KindDropMaterializedView,
			objectKey: model.NewObjectKey(schema, name, model.KindView),
		},
		Schema: schema,
		Name:   name,
	}
}

func (op *DropMaterializedViewOp) IsDestructive() bool { return true }

func (op *DropMaterializedViewOp) RenderString(ctx RenderContext) string {
	return fmt.Sprintf("-- op: drop_materialized_view risk:high\nDROP MATERIALIZED VIEW %s%s;",
		ifExistsPrefix(ctx.UseIfExists()), util.QuoteQualifiedIdentifier(op.Schema, op.Name))
}
