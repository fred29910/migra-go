package model

// Schema represents the complete database schema with namespaces
type Schema struct {
	Schemas map[string]*Namespace
}

// Namespace represents a schema namespace (e.g., public, app)
type Namespace struct {
	Name   string
	Tables map[string]*Table
	Types  map[string]*EnumType
}

// EnumType represents a PostgreSQL enum type
type EnumType struct {
	Name   string
	Labels []string
}

// NewSchema creates a new empty schema
func NewSchema() *Schema {
	return &Schema{
		Schemas: make(map[string]*Namespace),
	}
}

// GetOrCreateNamespace gets or creates a namespace by name
func (s *Schema) GetOrCreateNamespace(name string) *Namespace {
	if ns, ok := s.Schemas[name]; ok {
		return ns
	}
	ns := &Namespace{
		Name:   name,
		Tables: make(map[string]*Table),
		Types:  make(map[string]*EnumType),
	}
	s.Schemas[name] = ns
	return ns
}

// GetNamespace gets a namespace by name, returns nil if not found
func (s *Schema) GetNamespace(name string) *Namespace {
	return s.Schemas[name]
}
