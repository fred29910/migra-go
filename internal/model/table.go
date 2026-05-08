package model

// Table represents a database table
type Table struct {
	Schema        string
	Name          string
	Columns       []*Column          // Preserve order for column ordering strategies
	ColumnByName  map[string]*Column `json:"-"` // Index for quick lookup by name, excluded from JSON
	PrimaryKey    *PrimaryKey
	Constraints   map[string]*Constraint
	Indexes       map[string]*Index
	IsPlaceholder bool `json:"-"` // Internal state for parser-time placeholder tables
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
		Schema:        schema,
		Name:          name,
		Columns:       make([]*Column, 0),
		ColumnByName:  make(map[string]*Column),
		Constraints:   make(map[string]*Constraint),
		Indexes:       make(map[string]*Index),
		IsPlaceholder: false,
	}
}

// GetColumn finds a column by name
func (t *Table) GetColumn(name string) *Column {
	return t.ColumnByName[name]
}

// AddColumn adds a column to the table and maintains the ColumnByName index
func (t *Table) AddColumn(col *Column) {
	t.Columns = append(t.Columns, col)
	t.ColumnByName[col.Name] = col
}
