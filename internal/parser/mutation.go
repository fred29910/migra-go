package parser

import (
	"fmt"

	"github.com/fred29910/migra-go/internal/model"
)

// resolveColumn resolves a column by schema name, table name, and column name.
// It returns the namespace, table, and column, or an error if any step fails.
func resolveColumn(schema *model.Schema, schemaName, table, column string) (*model.Namespace, *model.Table, *model.Column, error) {
	ns := schema.GetNamespace(schemaName)
	if ns == nil {
		return nil, nil, nil, fmt.Errorf("schema %s not found", schemaName)
	}
	tbl, exists := ns.Tables[table]
	if !exists {
		return nil, nil, nil, fmt.Errorf("table %s.%s not found", schemaName, table)
	}
	col := tbl.ColumnByName[column]
	if col == nil {
		return nil, nil, nil, fmt.Errorf("column %s.%s.%s not found", schemaName, table, column)
	}
	return ns, tbl, col, nil
}

// MutationKind identifies the type of schema mutation.
type MutationKind string

const (
	// MutKindCreateTable represents creating a new table.
	MutKindCreateTable MutationKind = "create_table"
	// MutKindAddColumn represents adding a column to a table.
	MutKindAddColumn MutationKind = "add_column"
	// MutKindCreateEnumType represents creating an enum type.
	MutKindCreateEnumType MutationKind = "create_enum_type"
	// MutKindCreateIndex represents creating an index.
	MutKindCreateIndex MutationKind = "create_index"
	// MutKindDropColumn represents dropping a column from a table.
	MutKindDropColumn MutationKind = "drop_column"
	// MutKindAlterColumnType represents changing a column's data type.
	MutKindAlterColumnType MutationKind = "alter_column_type"
	// MutKindSetNotNull represents setting a column to NOT NULL.
	MutKindSetNotNull MutationKind = "set_not_null"
	// MutKindDropNotNull represents dropping NOT NULL from a column.
	MutKindDropNotNull MutationKind = "drop_not_null"
	// MutKindSetDefault represents setting a default expression on a column.
	MutKindSetDefault MutationKind = "set_default"
	// MutKindDropDefault represents dropping a default expression from a column.
	MutKindDropDefault MutationKind = "drop_default"
	// MutKindCreateSchema represents creating a schema.
	MutKindCreateSchema MutationKind = "create_schema"
	// MutKindRenameColumn represents renaming a column.
	MutKindRenameColumn MutationKind = "rename_column"
	// MutKindAddConstraint represents adding a constraint to a table.
	MutKindAddConstraint MutationKind = "add_constraint"
	// MutKindDropConstraint represents dropping a constraint from a table.
	MutKindDropConstraint MutationKind = "drop_constraint"
)

// SchemaMutation is a self-describing and self-applying schema change.
// Implementations represent DDL operations that can modify a model.Schema.
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

// Kind returns the mutation kind.
func (m CreateTableMutation) Kind() MutationKind { return MutKindCreateTable }
// Target returns the ObjectKey of the table to be created.
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

// Apply creates or merges the table in the given schema.
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

// Kind returns the mutation kind.
func (m AddColumnMutation) Kind() MutationKind { return MutKindAddColumn }
// Target returns the ObjectKey of the column to be added.
func (m AddColumnMutation) Target() model.ObjectKey {
	return model.NewObjectKey(m.Schema, m.Table+"."+m.Column.Name, model.KindColumn)
}
// Apply adds the column to the target table, creating a placeholder table if needed.
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

// Kind returns the mutation kind.
func (m CreateEnumTypeMutation) Kind() MutationKind { return MutKindCreateEnumType }
// Target returns the ObjectKey of the enum type to be created.
func (m CreateEnumTypeMutation) Target() model.ObjectKey {
	return model.NewObjectKey(m.Schema, m.Name, model.KindType)
}
// Apply creates the enum type in the given schema.
func (m CreateEnumTypeMutation) Apply(schema *model.Schema) error {
	ns := schema.GetOrCreateNamespace(m.Schema)
	if _, exists := ns.Types[m.Name]; exists {
		return fmt.Errorf("type %s.%s already exists", m.Schema, m.Name)
	}
	ns.Types[m.Name] = &model.EnumType{Name: m.Name, Labels: m.Labels}
	return nil
}

// DropColumnMutation describes dropping a column from a table.
type DropColumnMutation struct {
	Schema string
	Table  string
	Column string
}

// Kind returns the mutation kind.
func (m DropColumnMutation) Kind() MutationKind { return MutKindDropColumn }
// Target returns the ObjectKey of the column to be dropped.
func (m DropColumnMutation) Target() model.ObjectKey {
	return model.NewObjectKey(m.Schema, m.Table+"."+m.Column, model.KindColumn)
}
// Apply removes the column from the target table.
func (m DropColumnMutation) Apply(schema *model.Schema) error {
	_, _, _, err := resolveColumn(schema, m.Schema, m.Table, m.Column)
	if err != nil {
		return err
	}
	schema.GetNamespace(m.Schema).Tables[m.Table].RemoveColumn(m.Column)
	return nil
}

// AlterColumnTypeMutation describes changing a column's data type.
type AlterColumnTypeMutation struct {
	Schema    string
	Table     string
	Column    string
	FromType  string
	ToType    string
	UsingExpr string
}

// Kind returns the mutation kind.
func (m AlterColumnTypeMutation) Kind() MutationKind { return MutKindAlterColumnType }
// Target returns the ObjectKey of the column whose type is being altered.
func (m AlterColumnTypeMutation) Target() model.ObjectKey {
	return model.NewObjectKey(m.Schema, m.Table+"."+m.Column, model.KindColumn)
}
// Apply changes the data type of the target column.
func (m AlterColumnTypeMutation) Apply(schema *model.Schema) error {
	_, _, col, err := resolveColumn(schema, m.Schema, m.Table, m.Column)
	if err != nil {
		return err
	}
	if m.FromType != "" && col.DataType != m.FromType {
		return fmt.Errorf("column %s.%s.%s type mismatch: current %s, expected %s",
			m.Schema, m.Table, m.Column, col.DataType, m.FromType)
	}
	col.DataType = m.ToType
	return nil
}

// SetNotNullMutation describes setting a column to NOT NULL.
type SetNotNullMutation struct {
	Schema string
	Table  string
	Column string
}

// Kind returns the mutation kind.
func (m SetNotNullMutation) Kind() MutationKind { return MutKindSetNotNull }
// Target returns the ObjectKey of the column to set NOT NULL.
func (m SetNotNullMutation) Target() model.ObjectKey {
	return model.NewObjectKey(m.Schema, m.Table+"."+m.Column, model.KindColumn)
}
// Apply sets the target column to NOT NULL.
func (m SetNotNullMutation) Apply(schema *model.Schema) error {
	_, _, col, err := resolveColumn(schema, m.Schema, m.Table, m.Column)
	if err != nil {
		return err
	}
	col.IsNullable = false
	return nil
}

// DropNotNullMutation describes dropping NOT NULL from a column.
type DropNotNullMutation struct {
	Schema string
	Table  string
	Column string
}

// Kind returns the mutation kind.
func (m DropNotNullMutation) Kind() MutationKind { return MutKindDropNotNull }
// Target returns the ObjectKey of the column to drop NOT NULL.
func (m DropNotNullMutation) Target() model.ObjectKey {
	return model.NewObjectKey(m.Schema, m.Table+"."+m.Column, model.KindColumn)
}
// Apply drops the NOT NULL constraint from the target column.
func (m DropNotNullMutation) Apply(schema *model.Schema) error {
	_, _, col, err := resolveColumn(schema, m.Schema, m.Table, m.Column)
	if err != nil {
		return err
	}
	col.IsNullable = true
	return nil
}

// SetDefaultMutation describes setting a default expression on a column.
type SetDefaultMutation struct {
	Schema      string
	Table       string
	Column      string
	DefaultExpr string
}

// Kind returns the mutation kind.
func (m SetDefaultMutation) Kind() MutationKind { return MutKindSetDefault }
// Target returns the ObjectKey of the column to set default.
func (m SetDefaultMutation) Target() model.ObjectKey {
	return model.NewObjectKey(m.Schema, m.Table+"."+m.Column, model.KindColumn)
}
// Apply sets the default expression on the target column.
func (m SetDefaultMutation) Apply(schema *model.Schema) error {
	_, _, col, err := resolveColumn(schema, m.Schema, m.Table, m.Column)
	if err != nil {
		return err
	}
	col.DefaultExpr = &m.DefaultExpr
	return nil
}

// DropDefaultMutation describes dropping a default expression from a column.
type DropDefaultMutation struct {
	Schema string
	Table  string
	Column string
}

// Kind returns the mutation kind.
func (m DropDefaultMutation) Kind() MutationKind { return MutKindDropDefault }
// Target returns the ObjectKey of the column to drop default.
func (m DropDefaultMutation) Target() model.ObjectKey {
	return model.NewObjectKey(m.Schema, m.Table+"."+m.Column, model.KindColumn)
}
// Apply removes the default expression from the target column.
func (m DropDefaultMutation) Apply(schema *model.Schema) error {
	_, _, col, err := resolveColumn(schema, m.Schema, m.Table, m.Column)
	if err != nil {
		return err
	}
	col.DefaultExpr = nil
	return nil
}

// AddConstraintMutation describes adding a constraint to an existing table.
type AddConstraintMutation struct {
	Schema     string
	Table      string
	Constraint model.Constraint
}

// Kind returns the mutation kind.
func (m AddConstraintMutation) Kind() MutationKind { return MutKindAddConstraint }
// Target returns the ObjectKey of the constraint to be added.
func (m AddConstraintMutation) Target() model.ObjectKey {
	return model.NewObjectKey(m.Schema, m.Table+"."+m.Constraint.Name, model.KindConstraint)
}
// Apply adds the constraint to the target table.
func (m AddConstraintMutation) Apply(schema *model.Schema) error {
	ns := schema.GetNamespace(m.Schema)
	if ns == nil {
		return fmt.Errorf("schema %s not found", m.Schema)
	}
	table := ns.Tables[m.Table]
	if table == nil {
		return fmt.Errorf("table %s.%s not found", m.Schema, m.Table)
	}
	c := m.Constraint
	if c.Table == "" {
		c.Table = m.Table
	}
	table.Constraints[c.Name] = &c
	return nil
}

// DropConstraintMutation describes dropping a constraint from an existing table.
type DropConstraintMutation struct {
	Schema string
	Table  string
	Name   string
}

// Kind returns the mutation kind.
func (m DropConstraintMutation) Kind() MutationKind { return MutKindDropConstraint }
// Target returns the ObjectKey of the constraint to be dropped.
func (m DropConstraintMutation) Target() model.ObjectKey {
	return model.NewObjectKey(m.Schema, m.Table+"."+m.Name, model.KindConstraint)
}
// Apply removes the constraint from the target table.
func (m DropConstraintMutation) Apply(schema *model.Schema) error {
	ns := schema.GetNamespace(m.Schema)
	if ns == nil {
		return fmt.Errorf("schema %s not found", m.Schema)
	}
	table := ns.Tables[m.Table]
	if table == nil {
		return fmt.Errorf("table %s.%s not found", m.Schema, m.Table)
	}
	delete(table.Constraints, m.Name)
	return nil
}
