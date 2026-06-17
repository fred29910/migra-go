package model

import "sort"

// Schema represents the complete database schema with namespaces
type Schema struct {
	Schemas map[string]*Namespace
}

// Namespace represents a schema namespace (e.g., public, app)
type Namespace struct {
	Name       string
	Tables     map[string]*Table
	Types      map[string]*EnumType
	Views      map[string]*View
	Sequences  map[string]*Sequence
	Extensions map[string]*Extension
}

// NewNamespace creates a new Namespace with initialized object maps
func NewNamespace(name string) *Namespace {
	return &Namespace{
		Name:       name,
		Tables:     make(map[string]*Table),
		Types:      make(map[string]*EnumType),
		Views:      make(map[string]*View),
		Sequences:  make(map[string]*Sequence),
		Extensions: make(map[string]*Extension),
	}
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
	ns := NewNamespace(name)
	s.Schemas[name] = ns
	return ns
}

// GetNamespace gets a namespace by name, returns nil if not found
func (s *Schema) GetNamespace(name string) *Namespace {
	return s.Schemas[name]
}

// --- Schema convenience methods ---

// GetTable is a shortcut to get a table from a specific namespace.
// Returns nil if the namespace or table does not exist.
func (s *Schema) GetTable(namespace, tableName string) *Table {
	ns := s.GetNamespace(namespace)
	if ns == nil {
		return nil
	}
	return ns.GetTable(tableName)
}

// HasNamespace returns true if a namespace with the given name exists.
func (s *Schema) HasNamespace(name string) bool {
	_, exists := s.Schemas[name]
	return exists
}

// NamespaceNames returns all namespace names in sorted order.
func (s *Schema) NamespaceNames() []string {
	return sortedKeys(s.Schemas)
}

// --- Namespace accessor methods ---

// GetTable returns the table by name, or nil if not found.
func (ns *Namespace) GetTable(name string) *Table {
	return ns.Tables[name]
}

// HasTable returns true if a table with the given name exists.
func (ns *Namespace) HasTable(name string) bool {
	_, exists := ns.Tables[name]
	return exists
}

// TableNames returns all table names in sorted order.
func (ns *Namespace) TableNames() []string {
	return sortedKeys(ns.Tables)
}

// GetType returns the enum type by name, or nil.
func (ns *Namespace) GetType(name string) *EnumType {
	return ns.Types[name]
}

// HasType returns true if an enum type with the given name exists.
func (ns *Namespace) HasType(name string) bool {
	_, exists := ns.Types[name]
	return exists
}

// TypeNames returns all enum type names in sorted order.
func (ns *Namespace) TypeNames() []string {
	return sortedKeys(ns.Types)
}

// GetView returns the view by name, or nil.
func (ns *Namespace) GetView(name string) *View {
	return ns.Views[name]
}

// HasView returns true if a view with the given name exists.
func (ns *Namespace) HasView(name string) bool {
	_, exists := ns.Views[name]
	return exists
}

// ViewNames returns all view names in sorted order.
func (ns *Namespace) ViewNames() []string {
	return sortedKeys(ns.Views)
}

// GetSequence returns the sequence by name, or nil.
func (ns *Namespace) GetSequence(name string) *Sequence {
	return ns.Sequences[name]
}

// HasSequence returns true if a sequence with the given name exists.
func (ns *Namespace) HasSequence(name string) bool {
	_, exists := ns.Sequences[name]
	return exists
}

// SequenceNames returns all sequence names in sorted order.
func (ns *Namespace) SequenceNames() []string {
	return sortedKeys(ns.Sequences)
}

// GetExtension returns the extension by name, or nil.
func (ns *Namespace) GetExtension(name string) *Extension {
	return ns.Extensions[name]
}

// HasExtension returns true if an extension with the given name exists.
func (ns *Namespace) HasExtension(name string) bool {
	_, exists := ns.Extensions[name]
	return exists
}

// ExtensionNames returns all extension names in sorted order.
func (ns *Namespace) ExtensionNames() []string {
	return sortedKeys(ns.Extensions)
}

// sortedKeys is a package-private generic helper for deterministic map iteration.
func sortedKeys[V any](m map[string]V) []string {
	if len(m) == 0 {
		return nil
	}
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	return keys
}
