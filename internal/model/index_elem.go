package model

// IndexElem represents an element in an index definition
type IndexElem struct {
	Name          string // column name (for simple column index)
	Expr          string // expression (for expression index, e.g., "(col1 + col2)")
	IndexColName  string // name for index column, empty = default
	Collation     string // collation name, empty = default
	Opclass       string // operator class, empty = default
	Ordering      string // ASC/DESC/default
	NullsOrdering string // FIRST/LAST/default
}
