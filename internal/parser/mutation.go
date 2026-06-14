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
	MutKindCreateTable     MutationKind = "create_table"
	MutKindAddColumn       MutationKind = "add_column"
	MutKindCreateEnumType  MutationKind = "create_enum_type"
	MutKindCreateIndex     MutationKind = "create_index"
	MutKindDropColumn      MutationKind = "drop_column"
	MutKindAlterColumnType MutationKind = "alter_column_type"
	MutKindSetNotNull      MutationKind = "set_not_null"
	MutKindDropNotNull     MutationKind = "drop_not_null"
	MutKindSetDefault      MutationKind = "set_default"
	MutKindDropDefault     MutationKind = "drop_default"
	MutKindCreateSchema    MutationKind = "create_schema"
	MutKindRenameColumn    MutationKind = "rename_column"
	MutKindAddConstraint   MutationKind = "add_constraint"
	MutKindDropConstraint  MutationKind = "drop_constraint"
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

// DropColumnMutation describes dropping a column from a table.
type DropColumnMutation struct {
	Schema string
	Table  string
	Column string
}

func (m DropColumnMutation) Kind() MutationKind { return MutKindDropColumn }
func (m DropColumnMutation) Target() model.ObjectKey {
	return model.NewObjectKey(m.Schema, m.Table+"."+m.Column, model.KindColumn)
}
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

func (m AlterColumnTypeMutation) Kind() MutationKind { return MutKindAlterColumnType }
func (m AlterColumnTypeMutation) Target() model.ObjectKey {
	return model.NewObjectKey(m.Schema, m.Table+"."+m.Column, model.KindColumn)
}
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

func (m SetNotNullMutation) Kind() MutationKind { return MutKindSetNotNull }
func (m SetNotNullMutation) Target() model.ObjectKey {
	return model.NewObjectKey(m.Schema, m.Table+"."+m.Column, model.KindColumn)
}
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

func (m DropNotNullMutation) Kind() MutationKind { return MutKindDropNotNull }
func (m DropNotNullMutation) Target() model.ObjectKey {
	return model.NewObjectKey(m.Schema, m.Table+"."+m.Column, model.KindColumn)
}
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

func (m SetDefaultMutation) Kind() MutationKind { return MutKindSetDefault }
func (m SetDefaultMutation) Target() model.ObjectKey {
	return model.NewObjectKey(m.Schema, m.Table+"."+m.Column, model.KindColumn)
}
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

func (m DropDefaultMutation) Kind() MutationKind { return MutKindDropDefault }
func (m DropDefaultMutation) Target() model.ObjectKey {
	return model.NewObjectKey(m.Schema, m.Table+"."+m.Column, model.KindColumn)
}
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

func (m AddConstraintMutation) Kind() MutationKind { return MutKindAddConstraint }
func (m AddConstraintMutation) Target() model.ObjectKey {
	return model.NewObjectKey(m.Schema, m.Table+"."+m.Constraint.Name, model.KindConstraint)
}
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

func (m DropConstraintMutation) Kind() MutationKind { return MutKindDropConstraint }
func (m DropConstraintMutation) Target() model.ObjectKey {
	return model.NewObjectKey(m.Schema, m.Table+"."+m.Name, model.KindConstraint)
}
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
