package parser

import (
	"fmt"

	"github.com/fred29910/migra-go/internal/model"
)

// RenameColumnMutation describes renaming a column in a table.
type RenameColumnMutation struct {
	Schema  string
	Table   string
	OldName string
	NewName string
}

func (m RenameColumnMutation) Kind() MutationKind { return MutKindRenameColumn }
func (m RenameColumnMutation) Target() model.ObjectKey {
	return model.NewObjectKey(m.Schema, m.Table+"."+m.NewName, model.KindColumn)
}
func (m RenameColumnMutation) Apply(schema *model.Schema) error {
	ns := schema.GetNamespace(m.Schema)
	if ns == nil {
		return fmt.Errorf("schema %s not found", m.Schema)
	}
	table, exists := ns.Tables[m.Table]
	if !exists {
		return fmt.Errorf("table %s.%s not found", m.Schema, m.Table)
	}
	col := table.ColumnByName[m.OldName]
	if col == nil {
		return fmt.Errorf("column %s.%s.%s not found", m.Schema, m.Table, m.OldName)
	}
	if _, exists := table.ColumnByName[m.NewName]; exists {
		return fmt.Errorf("column %s.%s.%s already exists", m.Schema, m.Table, m.NewName)
	}
	col.Name = m.NewName
	delete(table.ColumnByName, m.OldName)
	table.ColumnByName[m.NewName] = col
	return nil
}
