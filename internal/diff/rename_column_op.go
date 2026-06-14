package diff

import "github.com/fred29910/migra-go/internal/model"

// RenameColumnOp represents renaming a column.
type RenameColumnOp struct {
	baseOperation
	Schema  string
	Table   string
	OldName string
	NewName string
}

// NewRenameColumnOp creates a new RenameColumnOp that renames a column from oldName to newName.
func NewRenameColumnOp(schema, table, oldName, newName string) *RenameColumnOp {
	return &RenameColumnOp{
		baseOperation: baseOperation{
			kind:      KindRenameColumn,
			objectKey: model.NewObjectKey(schema, table+"."+oldName, model.KindColumn),
		},
		Schema:  schema,
		Table:   table,
		OldName: oldName,
		NewName: newName,
	}
}

// IsDestructive returns whether the operation is destructive.
func (op *RenameColumnOp) IsDestructive() bool { return false }

// DependsOn returns the object keys this operation depends on.
func (op *RenameColumnOp) DependsOn() []model.ObjectKey {
	return []model.ObjectKey{
		model.NewObjectKey(op.Schema, op.Table, model.KindTable),
	}
}
