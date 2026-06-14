package diff

import (
	"fmt"

	"github.com/fred29910/migra-go/internal/model"
	"github.com/fred29910/migra-go/internal/util"
)

type AddConstraintOp struct {
	baseOperation
	Schema     string
	Table      string
	Constraint *model.Constraint
}

func NewAddConstraintOp(schema, table string, c *model.Constraint) *AddConstraintOp {
	return &AddConstraintOp{
		baseOperation: baseOperation{kind: KindAddConstraint, objectKey: model.NewObjectKey(schema, table+"."+c.Name, model.KindConstraint)},
		Schema:        schema,
		Table:         table,
		Constraint:    c,
	}
}

func (op *AddConstraintOp) IsDestructive() bool {
	return false
}

func (op *AddConstraintOp) RenderString(ctx RenderContext) string {
	definition := renderConstraintDefinition(op.Constraint)
	if definition == "" {
		return ""
	}
	return fmt.Sprintf("-- op: add_constraint risk:medium\nALTER TABLE %s ADD CONSTRAINT %s %s;",
		util.QuoteQualifiedIdentifier(op.Schema, op.Table),
		util.QuoteIdentifier(op.Constraint.Name),
		definition,
	)
}

func (op *AddConstraintOp) DependsOn() []model.ObjectKey {
	return []model.ObjectKey{
		model.NewObjectKey(op.Schema, op.Table, model.KindTable),
		model.NewObjectKey(op.Schema, op.Table+"."+op.Constraint.Name, model.KindConstraint),
	}
}

type DropConstraintOp struct {
	baseOperation
	Schema string
	Table  string
	Name   string
}

func NewDropConstraintOp(schema, table, name string) *DropConstraintOp {
	return &DropConstraintOp{
		baseOperation: baseOperation{kind: KindDropConstraint, objectKey: model.NewObjectKey(schema, table+"."+name, model.KindConstraint)},
		Schema:        schema,
		Table:         table,
		Name:          name,
	}
}

func (op *DropConstraintOp) IsDestructive() bool {
	return true
}

func (op *DropConstraintOp) RenderString(ctx RenderContext) string {
	return fmt.Sprintf("-- op: drop_constraint risk:medium\nALTER TABLE %s DROP CONSTRAINT %s%s;",
		util.QuoteQualifiedIdentifier(op.Schema, op.Table),
		ifExistsPrefix(ctx.UseIfExists()),
		util.QuoteIdentifier(op.Name),
	)
}

func ifExistsPrefix(use bool) string {
	if use {
		return "IF EXISTS "
	}
	return ""
}
