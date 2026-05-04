package model

// Column represents a table column
type Column struct {
	Name         string
	DataType     string
	IsNullable   bool
	DefaultExpr  *string
	IsIdentity   bool
	IdentityKind string // "ALWAYS" or "BY DEFAULT"
}
