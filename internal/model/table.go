package model

// Table represents a database table
type Table struct {
	Name        string
	Columns     map[string]*Column
	PrimaryKey  *PrimaryKey
	Indexes     map[string]*Index
	Constraints map[string]*Constraint
}

// Column represents a table column
type Column struct {
	Name     string
	DataType string
	Nullable bool
	Default  *string
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

// Constraint represents a table constraint (foreign key, check, unique, etc.)
type Constraint struct {
	Name       string
	Type       string
	Definition string
	Table      string
}

// NewTable creates a new table with initialized maps
func NewTable(name string) *Table {
	return &Table{
		Name:        name,
		Columns:     make(map[string]*Column),
		Indexes:     make(map[string]*Index),
		Constraints: make(map[string]*Constraint),
	}
}
