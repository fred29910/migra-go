package diff

import (
	"fmt"

	"github.com/fred29910/migra-go/internal/model"
	"github.com/fred29910/migra-go/internal/util"
)

// AddConstraintOp represents an operation to add a constraint to an existing table.
type AddConstraintOp struct {
	baseOperation
	Schema     string
	Table      string
	Constraint *model.Constraint
}

// NewAddConstraintOp creates a new AddConstraintOp.
func NewAddConstraintOp(schema, table string, c *model.Constraint) *AddConstraintOp {
	return &AddConstraintOp{
		baseOperation: baseOperation{kind: KindAddConstraint, objectKey: model.NewObjectKey(schema, table+"."+c.Name, model.KindConstraint)},
		Schema:        schema,
		Table:         table,
		Constraint:    c,
	}
}

// IsDestructive returns false; adding a constraint is not destructive.
func (op *AddConstraintOp) IsDestructive() bool {
	return false
}

// RenderString renders the SQL statement for adding a constraint.
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

// DependsOn returns the object keys this operation depends on.
func (op *AddConstraintOp) DependsOn() []model.ObjectKey {
	return []model.ObjectKey{
		model.NewObjectKey(op.Schema, op.Table, model.KindTable),
		model.NewObjectKey(op.Schema, op.Table+"."+op.Constraint.Name, model.KindConstraint),
	}
}

// DropConstraintOp represents an operation to drop a constraint from a table.
type DropConstraintOp struct {
	baseOperation
	Schema string
	Table  string
	Name   string
}

// NewDropConstraintOp creates a new DropConstraintOp.
func NewDropConstraintOp(schema, table, name string) *DropConstraintOp {
	return &DropConstraintOp{
		baseOperation: baseOperation{kind: KindDropConstraint, objectKey: model.NewObjectKey(schema, table+"."+name, model.KindConstraint)},
		Schema:        schema,
		Table:         table,
		Name:          name,
	}
}

// IsDestructive returns true; dropping a constraint is considered destructive.
func (op *DropConstraintOp) IsDestructive() bool {
	return true
}

// RenderString renders the SQL statement for dropping a constraint.
func (op *DropConstraintOp) RenderString(ctx RenderContext) string {
	return fmt.Sprintf("-- op: drop_constraint risk:medium\nALTER TABLE %s DROP CONSTRAINT %s%s;",
		util.QuoteQualifiedIdentifier(op.Schema, op.Table),
		ifExistsPrefix(ctx.UseIfExists()),
		util.QuoteIdentifier(op.Name),
	)
}

// ifExistsPrefix returns "IF EXISTS " if use is true, otherwise an empty string.
func ifExistsPrefix(use bool) string {
	if use {
		return "IF EXISTS "
	}
	return ""
}
