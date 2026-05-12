package parser

import (
	"fmt"

	"github.com/fred29910/migra-go/internal/model"
)

// MutationKind identifies the type of schema mutation.
type MutationKind string

const (
	MutKindCreateTable    MutationKind = "create_table"
	MutKindAddColumn      MutationKind = "add_column"
	MutKindCreateEnumType MutationKind = "create_enum_type"
	MutKindCreateIndex    MutationKind = "create_index"
)

// SchemaMutation is a self-describing and self-applying schema change.
type SchemaMutation interface {
	Kind() MutationKind
	Target() model.ObjectKey
	Apply(schema *model.Schema) error
}

// CreateTableMutation describes creating a new table.
type CreateTableMutation struct {
	Schema      string
	Name        string
	Columns     []model.Column
	PrimaryKey  *model.PrimaryKey
	Constraints []model.Constraint
}

func (m CreateTableMutation) Kind() MutationKind { return MutKindCreateTable }
func (m CreateTableMutation) Target() model.ObjectKey {
	return model.NewObjectKey(m.Schema, m.Name, model.KindTable)
}
func applyTableMetadata(table *model.Table, primaryKey *model.PrimaryKey, constraints []model.Constraint) {
	if primaryKey != nil {
		table.PrimaryKey = primaryKey
	}
	for i := range constraints {
		c := constraints[i]
		if c.Table == "" {
			c.Table = table.Name
		}
		table.Constraints[c.Name] = &c
	}
}

func (m CreateTableMutation) Apply(schema *model.Schema) error {
	ns := schema.GetOrCreateNamespace(m.Schema)
	table, exists := ns.Tables[m.Name]
	if exists {
		if !table.IsPlaceholder {
			return fmt.Errorf("table %s.%s already exists", m.Schema, m.Name)
		}
		// Merge with placeholder table
		for _, col := range m.Columns {
			existingCol := table.GetColumn(col.Name)
			if existingCol != nil {
				// Consistency check: type must match if column was already added by ALTER
				if existingCol.DataType != col.DataType {
					return fmt.Errorf("column %s.%s type mismatch: exists as %s, trying to create as %s",
						m.Name, col.Name, existingCol.DataType, col.DataType)
				}
				// If matches, we skip adding it to avoid duplicates
				continue
			}
			// Append new column from CREATE TABLE
			table.AddColumn(&col)
		}
		table.IsPlaceholder = false
		applyTableMetadata(table, m.PrimaryKey, m.Constraints)
		return nil
	}

	table = model.NewTable(m.Schema, m.Name)
	for i := range m.Columns {
		table.AddColumn(&m.Columns[i])
	}
	ns.Tables[m.Name] = table
	applyTableMetadata(table, m.PrimaryKey, m.Constraints)
	return nil
}

// AddColumnMutation describes adding a column to an existing table.
type AddColumnMutation struct {
	Schema string
	Table  string
	Column model.Column
}

func (m AddColumnMutation) Kind() MutationKind { return MutKindAddColumn }
func (m AddColumnMutation) Target() model.ObjectKey {
	return model.NewObjectKey(m.Schema, m.Table+"."+m.Column.Name, model.KindColumn)
}
func (m AddColumnMutation) Apply(schema *model.Schema) error {
	ns := schema.GetOrCreateNamespace(m.Schema)
	table, exists := ns.Tables[m.Table]
	if !exists {
		// ALTER TABLE may appear before CREATE TABLE in SQL;
		// create an empty placeholder table.
		table = model.NewTable(m.Schema, m.Table)
		table.IsPlaceholder = true
		ns.Tables[m.Table] = table
	}
	table.AddColumn(&m.Column)
	return nil
}

// CreateEnumTypeMutation describes creating an enum type.
type CreateEnumTypeMutation struct {
	Schema string
	Name   string
	Labels []string
}

func (m CreateEnumTypeMutation) Kind() MutationKind { return MutKindCreateEnumType }
func (m CreateEnumTypeMutation) Target() model.ObjectKey {
	return model.NewObjectKey(m.Schema, m.Name, model.KindType)
}
func (m CreateEnumTypeMutation) Apply(schema *model.Schema) error {
	ns := schema.GetOrCreateNamespace(m.Schema)
	if _, exists := ns.Types[m.Name]; exists {
		return fmt.Errorf("type %s.%s already exists", m.Schema, m.Name)
	}
	ns.Types[m.Name] = &model.EnumType{Name: m.Name, Labels: m.Labels}
	return nil
}
