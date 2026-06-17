package model

// Table represents a database table
type Table struct {
	Schema        string
	Name          string
	Columns       []*Column          // Preserve order for column ordering strategies
	ColumnByName  map[string]*Column `json:"-"` // Index for quick lookup by name, excluded from JSON
	ColumnIndex   map[string]int     `json:"-"` // Index of each column in Columns slice, for O(1) removal
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
		ColumnIndex:   make(map[string]int),
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
	t.ColumnIndex[col.Name] = len(t.Columns)
	t.Columns = append(t.Columns, col)
	t.ColumnByName[col.Name] = col
}

// RemoveColumn removes a column by name from the table.
// Uses ColumnIndex for O(1) lookup of the column's position in the slice.
func (t *Table) RemoveColumn(name string) {
	idx, ok := t.ColumnIndex[name]
	if !ok {
		return
	}
	delete(t.ColumnByName, name)
	delete(t.ColumnIndex, name)
	// Remove from slice by swapping with last element and truncating
	last := len(t.Columns) - 1
	if idx != last {
		t.Columns[idx] = t.Columns[last]
		t.ColumnIndex[t.Columns[idx].Name] = idx
	}
	t.Columns = t.Columns[:last]
}

// --- Table accessor methods ---

// HasColumn returns true if a column with the given name exists.
func (t *Table) HasColumn(name string) bool {
	_, exists := t.ColumnByName[name]
	return exists
}

// ColumnNames returns all column names in order (matching Columns slice order).
func (t *Table) ColumnNames() []string {
	names := make([]string, len(t.Columns))
	for i, col := range t.Columns {
		names[i] = col.Name
	}
	return names
}

// ColumnCount returns the number of columns.
func (t *Table) ColumnCount() int {
	return len(t.Columns)
}

// GetIndex returns the index by name, or nil if not found.
func (t *Table) GetIndex(name string) *Index {
	return t.Indexes[name]
}

// HasIndex returns true if an index with the given name exists.
func (t *Table) HasIndex(name string) bool {
	_, exists := t.Indexes[name]
	return exists
}

// IndexNames returns all index names in sorted order.
func (t *Table) IndexNames() []string {
	return sortedKeys(t.Indexes)
}

// GetConstraint returns the constraint by name, or nil if not found.
func (t *Table) GetConstraint(name string) *Constraint {
	return t.Constraints[name]
}

// HasConstraint returns true if a constraint with the given name exists.
func (t *Table) HasConstraint(name string) bool {
	_, exists := t.Constraints[name]
	return exists
}

// ConstraintNames returns all constraint names in sorted order.
func (t *Table) ConstraintNames() []string {
	return sortedKeys(t.Constraints)
}
