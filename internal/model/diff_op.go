package model

// OperationKind represents the type of diff operation
type OperationKind string

const (
	AddTable      OperationKind = "add_table"
	DropTable     OperationKind = "drop_table"
	AddColumn     OperationKind = "add_column"
	AlterColumn   OperationKind = "alter_column"
	DropColumn    OperationKind = "drop_column"
	AddIndex      OperationKind = "add_index"
	DropIndex     OperationKind = "drop_index"
	AddConstraint OperationKind = "add_constraint"
	DropConstraint OperationKind = "drop_constraint"
	AlterType     OperationKind = "alter_type"
)

// DiffOp represents a single differential operation between two schemas
type DiffOp struct {
	Kind    OperationKind
	Obj     string
	Details map[string]string
}

// NewDiffOp creates a new DiffOp
func NewDiffOp(kind OperationKind, obj string) *DiffOp {
	return &DiffOp{
		Kind:    kind,
		Obj:     obj,
		Details: make(map[string]string),
	}
}
