package model

// Schema represents a database schema with all its objects
type Schema struct {
	Tables map[string]*Table
	Types  map[string]*EnumType
	Views  map[string]*View
	Funcs  map[string]*Function
}

// EnumType represents a PostgreSQL enum type
type EnumType struct {
	Name   string
	Labels []string
}

// View represents a database view
type View struct {
	Name string
	Definition string
}

// Function represents a database function
type Function struct {
	Name       string
	Definition string
}

// NewSchema creates a new empty schema
func NewSchema() *Schema {
	return &Schema{
		Tables: make(map[string]*Table),
		Types:  make(map[string]*EnumType),
		Views:  make(map[string]*View),
		Funcs:  make(map[string]*Function),
	}
}
