package model

// Table represents a database table
type Table struct {
	Schema      string
	Name        string
	Columns     []*Column         // Preserve order for column ordering strategies
	PrimaryKey  *PrimaryKey
	Constraints map[string]*Constraint
	Indexes     map[string]*Index
}

// PrimaryKey represents a primary key constraint
type PrimaryKey struct {
	Name    string
	Columns []string
}

// Index represents a database index
type Index struct {
	Name    string
	Table   string
	Columns []string
	Unique  bool
	Method  string
}

// Constraint represents a table constraint (check, unique, etc.)
type Constraint struct {
	Name       string
	Type       string
	Definition string
	Table      string
}

// NewTable creates a new table with initialized maps
func NewTable(schema, name string) *Table {
	return &Table{
		Schema:      schema,
		Name:        name,
		Columns:     make([]*Column, 0),
		Constraints: make(map[string]*Constraint),
		Indexes:     make(map[string]*Index),
	}
}

// GetColumn finds a column by name
func (t *Table) GetColumn(name string) *Column {
	for _, col := range t.Columns {
		if col.Name == name {
			return col
		}
	}
	return nil
}

// AddColumn adds a column to the table
func (t *Table) AddColumn(col *Column) {
	t.Columns = append(t.Columns, col)
}
