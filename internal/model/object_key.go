package model

// ObjectKind represents the type of database object
type ObjectKind string

const (
	// KindTable represents a table object.
	KindTable ObjectKind = "table"
	// KindColumn represents a column object.
	KindColumn ObjectKind = "column"
	// KindIndex represents an index object.
	KindIndex ObjectKind = "index"
	// KindConstraint represents a constraint object.
	KindConstraint ObjectKind = "constraint"
	// KindType represents a user-defined type object.
	KindType ObjectKind = "type"
	// KindView represents a view object.
	KindView ObjectKind = "view"
	// KindFunction represents a function object.
	KindFunction ObjectKind = "function"
	// KindSchema represents a schema object.
	KindSchema ObjectKind = "schema"
	// KindSequence represents a sequence object.
	KindSequence ObjectKind = "sequence"
	// KindExtension represents an extension object.
	KindExtension ObjectKind = "extension"
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
