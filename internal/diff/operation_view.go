package diff

import (
	"fmt"

	"github.com/fred29910/migra-go/internal/model"
	"github.com/fred29910/migra-go/internal/util"
)

// CreateViewOp represents the operation of creating a view.
type CreateViewOp struct {
	baseOperation
	Schema string
	View   *model.View
}

// NewCreateViewOp creates a new CreateViewOp for the given schema and view.
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

// IsDestructive returns whether the operation is destructive.
func (op *CreateViewOp) IsDestructive() bool { return false }

// RenderString renders the operation as a SQL string.
func (op *CreateViewOp) RenderString(ctx RenderContext) string {
	return fmt.Sprintf("-- op: create_view risk:low\nCREATE VIEW %s AS %s;",
		util.QuoteQualifiedIdentifier(op.Schema, op.View.Name), op.View.Definition)
}

// DropViewOp represents the operation of dropping a view.
type DropViewOp struct {
	baseOperation
	Schema     string
	Name       string
	IsRecreate bool
}

// NewDropViewOp creates a new DropViewOp for the given schema and view name.
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

// IsDestructive returns whether the operation is destructive.
func (op *DropViewOp) IsDestructive() bool { return !op.IsRecreate }

// RenderString renders the operation as a SQL string.
func (op *DropViewOp) RenderString(ctx RenderContext) string {
	return fmt.Sprintf("-- op: drop_view risk:high\nDROP VIEW %s%s;",
		ifExistsPrefix(ctx.UseIfExists()), util.QuoteQualifiedIdentifier(op.Schema, op.Name))
}

// ReplaceViewOp represents the operation of replacing (CREATE OR REPLACE) a view.
type ReplaceViewOp struct {
	baseOperation
	Schema string
	View   *model.View
}

// NewReplaceViewOp creates a new ReplaceViewOp for the given schema and view.
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

// IsDestructive returns whether the operation is destructive.
func (op *ReplaceViewOp) IsDestructive() bool { return false }

// RenderString renders the operation as a SQL string.
func (op *ReplaceViewOp) RenderString(ctx RenderContext) string {
	return fmt.Sprintf("-- op: replace_view risk:medium\nCREATE OR REPLACE VIEW %s AS %s;",
		util.QuoteQualifiedIdentifier(op.Schema, op.View.Name), op.View.Definition)
}

// CreateMaterializedViewOp represents the operation of creating a materialized view.
type CreateMaterializedViewOp struct {
	baseOperation
	Schema           string
	MaterializedView *model.View
}

// NewCreateMaterializedViewOp creates a new CreateMaterializedViewOp for the given schema and view.
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

// IsDestructive returns whether the operation is destructive.
func (op *CreateMaterializedViewOp) IsDestructive() bool { return false }

// RenderString renders the operation as a SQL string.
func (op *CreateMaterializedViewOp) RenderString(ctx RenderContext) string {
	return fmt.Sprintf("-- op: create_materialized_view risk:low\nCREATE MATERIALIZED VIEW %s AS %s;",
		util.QuoteQualifiedIdentifier(op.Schema, op.MaterializedView.Name), op.MaterializedView.Definition)
}

// DropMaterializedViewOp represents the operation of dropping a materialized view.
type DropMaterializedViewOp struct {
	baseOperation
	Schema string
	Name   string
}

// NewDropMaterializedViewOp creates a new DropMaterializedViewOp for the given schema and view name.
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

// IsDestructive returns whether the operation is destructive.
func (op *DropMaterializedViewOp) IsDestructive() bool { return true }

// RenderString renders the operation as a SQL string.
func (op *DropMaterializedViewOp) RenderString(ctx RenderContext) string {
	return fmt.Sprintf("-- op: drop_materialized_view risk:high\nDROP MATERIALIZED VIEW %s%s;",
		ifExistsPrefix(ctx.UseIfExists()), util.QuoteQualifiedIdentifier(op.Schema, op.Name))
}
