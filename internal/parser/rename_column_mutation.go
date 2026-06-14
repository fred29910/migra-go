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

// Kind returns the mutation kind.
func (m RenameColumnMutation) Kind() MutationKind { return MutKindRenameColumn }
// Target returns the ObjectKey of the renamed column.
func (m RenameColumnMutation) Target() model.ObjectKey {
	return model.NewObjectKey(m.Schema, m.Table+"."+m.NewName, model.KindColumn)
}
// Apply renames the column in the target table.
func (m RenameColumnMutation) Apply(schema *model.Schema) error {
	ns := schema.GetOrCreateNamespace(m.Schema)
	table, exists := ns.Tables[m.Table]
	if !exists {
		// ALTER TABLE RENAME COLUMN may appear before CREATE TABLE;
		// create an empty placeholder table.
		table = model.NewTable(m.Schema, m.Table)
		table.IsPlaceholder = true
		ns.Tables[m.Table] = table
		return nil
	}
	if table.IsPlaceholder {
		// Nothing meaningful to rename on a placeholder.
		return nil
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
