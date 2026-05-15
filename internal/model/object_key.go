package model

// ObjectKind represents the type of database object
type ObjectKind string

const (
	KindTable      ObjectKind = "table"
	KindColumn     ObjectKind = "column"
	KindIndex      ObjectKind = "index"
	KindConstraint ObjectKind = "constraint"
	KindType       ObjectKind = "type"
	KindView       ObjectKind = "view"
	KindFunction   ObjectKind = "function"
	KindSchema     ObjectKind = "schema"
)

// ObjectKey uniquely identifies a database object
type ObjectKey struct {
	Schema    string
	Name      string
	Kind      ObjectKind
	Signature string // function parameter signature; empty for other objects
}

// NewObjectKey creates a new ObjectKey
func NewObjectKey(schema, name string, kind ObjectKind) ObjectKey {
	return ObjectKey{
		Schema: schema,
		Name:   name,
		Kind:   kind,
	}
}

// NewFunctionKey creates a new ObjectKey for a function with signature
func NewFunctionKey(schema, name, signature string) ObjectKey {
	return ObjectKey{
		Schema:    schema,
		Name:      name,
		Kind:      KindFunction,
		Signature: signature,
	}
}
