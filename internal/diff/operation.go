package diff

import "github.com/migra-go/migra-go/internal/model"

// Kind represents the type of diff operation
type Kind string

const (
	KindAddTable      Kind = "add_table"
	KindDropTable     Kind = "drop_table"
	KindAddColumn     Kind = "add_column"
	KindDropColumn    Kind = "drop_column"
	KindAlterColumnType Kind = "alter_column_type"
	KindSetNotNull    Kind = "set_not_null"
	KindDropNotNull   Kind = "drop_not_null"
	KindAddIndex      Kind = "add_index"
	KindDropIndex     Kind = "drop_index"
	KindAddConstraint Kind = "add_constraint"
	KindDropConstraint Kind = "drop_constraint"
	KindAddEnumType    Kind = "add_enum_type"
	KindDropEnumType   Kind = "drop_enum_type"
)

// Operation is the interface for all diff operations
type Operation interface {
	Kind() Kind
	ObjectKey() model.ObjectKey
	IsDestructive() bool
}

// baseOperation provides common fields for operations
type baseOperation struct {
	kind      Kind
	objectKey model.ObjectKey
}

func (op *baseOperation) Kind() Kind {
	return op.kind
}

func (op *baseOperation) ObjectKey() model.ObjectKey {
	return op.objectKey
}

// AddTableOp represents adding a new table
type AddTableOp struct {
	baseOperation
	Table *model.Table
}

func NewAddTableOp(schema, name string, table *model.Table) *AddTableOp {
	return &AddTableOp{
		baseOperation: baseOperation{
			kind:      KindAddTable,
			objectKey: model.NewObjectKey(schema, name, model.KindTable),
		},
		Table: table,
	}
}

func (op *AddTableOp) IsDestructive() bool {
	return false
}

// DropTableOp represents dropping a table
type DropTableOp struct {
	baseOperation
	Schema string
	Name   string
}

func NewDropTableOp(schema, name string) *DropTableOp {
	return &DropTableOp{
		baseOperation: baseOperation{
			kind:      KindDropTable,
			objectKey: model.NewObjectKey(schema, name, model.KindTable),
		},
		Schema: schema,
		Name:   name,
	}
}

func (op *DropTableOp) IsDestructive() bool {
	return true
}

// AddColumnOp represents adding a new column
type AddColumnOp struct {
	baseOperation
	Schema string
	Table  string
	Column *model.Column
}

func NewAddColumnOp(schema, table string, col *model.Column) *AddColumnOp {
	return &AddColumnOp{
		baseOperation: baseOperation{
			kind:      KindAddColumn,
			objectKey: model.NewObjectKey(schema, table+"."+col.Name, model.KindColumn),
		},
		Schema: schema,
		Table:  table,
		Column: col,
	}
}

func (op *AddColumnOp) IsDestructive() bool {
	return false
}

// AlterColumnTypeOp represents changing a column's data type
type AlterColumnTypeOp struct {
	baseOperation
	Schema   string
	Table    string
	Column   string
	FromType string
	ToType   string
}

func NewAlterColumnTypeOp(schema, table, column, fromType, toType string) *AlterColumnTypeOp {
	return &AlterColumnTypeOp{
		baseOperation: baseOperation{
			kind:      KindAlterColumnType,
			objectKey: model.NewObjectKey(schema, table+"."+column, model.KindColumn),
		},
		Schema:   schema,
		Table:    table,
		Column:   column,
		FromType: fromType,
		ToType:   toType,
	}
}

func (op *AlterColumnTypeOp) IsDestructive() bool {
	return true // Type changes can be destructive
}

// SetNotNullOp represents setting a column to NOT NULL
type SetNotNullOp struct {
	baseOperation
	Schema string
	Table  string
	Column string
}

func NewSetNotNullOp(schema, table, column string) *SetNotNullOp {
	return &SetNotNullOp{
		baseOperation: baseOperation{
			kind:      KindSetNotNull,
			objectKey: model.NewObjectKey(schema, table+"."+column, model.KindColumn),
		},
		Schema: schema,
		Table:  table,
		Column: column,
	}
}

func (op *SetNotNullOp) IsDestructive() bool {
	return false
}

// DropNotNullOp represents dropping NOT NULL constraint
type DropNotNullOp struct {
	baseOperation
	Schema string
	Table  string
	Column string
}

func NewDropNotNullOp(schema, table, column string) *DropNotNullOp {
	return &DropNotNullOp{
		baseOperation: baseOperation{
			kind:      KindDropNotNull,
			objectKey: model.NewObjectKey(schema, table+"."+column, model.KindColumn),
		},
		Schema: schema,
		Table:  table,
		Column: column,
	}
}

func (op *DropNotNullOp) IsDestructive() bool {
	return false
}

// CreateIndexOp represents creating a new index
type CreateIndexOp struct {
	baseOperation
	Index *model.Index
}

func NewCreateIndexOp(index *model.Index) *CreateIndexOp {
	return &CreateIndexOp{
		baseOperation: baseOperation{
			kind:      KindAddIndex,
			objectKey: model.NewObjectKey("", index.Name, model.KindIndex),
		},
		Index: index,
	}
}

func (op *CreateIndexOp) IsDestructive() bool {
	return false
}

// DropIndexOp represents dropping an index
type DropIndexOp struct {
	baseOperation
	Schema string
	Name   string
}

func NewDropIndexOp(schema, name string) *DropIndexOp {
	return &DropIndexOp{
		baseOperation: baseOperation{
			kind:      KindDropIndex,
			objectKey: model.NewObjectKey(schema, name, model.KindIndex),
		},
		Schema: schema,
		Name:   name,
	}
}

func (op *DropIndexOp) IsDestructive() bool {
	return false
}

// AddEnumTypeOp represents adding a new enum type
type AddEnumTypeOp struct {
	baseOperation
	Schema string
	Type   *model.EnumType
}

func NewAddEnumTypeOp(schema string, enumType *model.EnumType) *AddEnumTypeOp {
	return &AddEnumTypeOp{
		baseOperation: baseOperation{
			kind:      KindAddEnumType,
			objectKey: model.NewObjectKey(schema, enumType.Name, model.KindType),
		},
		Schema: schema,
		Type:   enumType,
	}
}

func (op *AddEnumTypeOp) IsDestructive() bool {
	return false
}

// DropEnumTypeOp represents dropping an enum type
type DropEnumTypeOp struct {
	baseOperation
	Schema string
	Name   string
}

func NewDropEnumTypeOp(schema, name string) *DropEnumTypeOp {
	return &DropEnumTypeOp{
		baseOperation: baseOperation{
			kind:      KindDropEnumType,
			objectKey: model.NewObjectKey(schema, name, model.KindType),
		},
		Schema: schema,
		Name:   name,
	}
}

func (op *DropEnumTypeOp) IsDestructive() bool {
	return true
}
