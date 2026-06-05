package diff

import "github.com/fred29910/migra-go/internal/model"

// AddTableOp represents adding a new table
type AddTableOp struct {
	baseOperation
	Table *model.Table
}

// NewAddTableOp creates a new AddTableOp
func NewAddTableOp(schema, name string, table *model.Table) *AddTableOp {
	return &AddTableOp{
		baseOperation: baseOperation{
			kind:      KindAddTable,
			objectKey: model.NewObjectKey(schema, name, model.KindTable),
		},
		Table: table,
	}
}

// IsDestructive returns false for add table operations
func (op *AddTableOp) IsDestructive() bool {
	return false
}

// DependsOn returns dependencies for add table operations (foreign key references)
func (op *AddTableOp) DependsOn() []model.ObjectKey {
	var deps []model.ObjectKey
	for _, constraint := range op.Table.Constraints {
		if constraint.Type == "foreign_key" && constraint.RefTable != "" {
			deps = append(deps, model.NewObjectKey(constraint.RefSchema, constraint.RefTable, model.KindTable))
		}
	}
	return deps
}

// DropTableOp represents dropping a table
type DropTableOp struct {
	baseOperation
	Schema string
	Name   string
}

// NewDropTableOp creates a new DropTableOp
func NewDropTableOp(schema, name string) *DropTableOp {
	return &DropTableOp{
		baseOperation: baseOperation{
			kind:      KindDropTable,
			objectKey: model.NewObjectKey(schema, name, model.KindTable),
		},
		Schema: schema,
		Name:   name,
	}
}

// IsDestructive returns true for drop table operations
func (op *DropTableOp) IsDestructive() bool {
	return true
}
