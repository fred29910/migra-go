package diff

import "github.com/fred29910/migra-go/internal/model"

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
