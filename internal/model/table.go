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
	Name         string
	Table        string
	Columns      []string    // deprecated: use Elements
	Elements     []IndexElem // new field replacing Columns
	Unique       bool
	Method       string // btree, hash, gin, etc.
	Primary      bool   // is primary key index
	IsConstraint bool   // is it for a pkey/unique constraint
	WhereClause  string // partial index predicate (WHERE clause)
	Definition   string // pg_get_indexdef output for advanced index comparison
	Predicate    string // partial index predicate (WHERE clause), empty = no predicate
	Concurrent   bool   // concurrent index build
	IfNotExists  bool   // IF NOT EXISTS
}

// Constraint represents a table constraint (check, unique, foreign key, etc.)
type Constraint struct {
	Name       string
	Type       string
	Definition string
	Table      string
	Columns    []string
	RefSchema  string
	RefTable   string
	RefColumns []string
	Expression string
	OnDelete   string
	OnUpdate   string
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

// RemoveColumn removes a column by name from the table
func (t *Table) RemoveColumn(name string) {
	delete(t.ColumnByName, name)
	for i, col := range t.Columns {
		if col.Name == name {
			t.Columns = append(t.Columns[:i], t.Columns[i+1:]...)
			return
		}
	}
}
