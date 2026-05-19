package diff

import "github.com/fred29910/migra-go/internal/model"

// Kind represents the type of diff operation
type Kind string

const (
	KindAddTable        Kind = "add_table"
	KindDropTable       Kind = "drop_table"
	KindAddColumn       Kind = "add_column"
	KindDropColumn      Kind = "drop_column"
	KindAlterColumnType Kind = "alter_column_type"
	KindSetNotNull      Kind = "set_not_null"
	KindDropNotNull     Kind = "drop_not_null"
	KindAddIndex        Kind = "add_index"
	KindDropIndex       Kind = "drop_index"
	KindAddConstraint   Kind = "add_constraint"
	KindDropConstraint  Kind = "drop_constraint"
	KindAddEnumType     Kind = "add_enum_type"
	KindDropEnumType    Kind = "drop_enum_type"
	KindAddEnumLabel    Kind = "add_enum_label"
	KindSetDefault           Kind = "set_default"
	KindDropDefault          Kind = "drop_default"
	KindAlterColumnCollation Kind = "alter_column_collation"
)

// Operation is the interface for all diff operations
type Operation interface {
	Kind() Kind
	ObjectKey() model.ObjectKey
	DependsOn() []model.ObjectKey
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

func (op *baseOperation) DependsOn() []model.ObjectKey {
	return nil
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

func (op *AddTableOp) DependsOn() []model.ObjectKey {
	var deps []model.ObjectKey
	for _, constraint := range op.Table.Constraints {
		if constraint.Type == "foreign_key" && constraint.RefTable != "" {
			deps = append(deps, model.NewObjectKey(constraint.RefSchema, constraint.RefTable, model.KindTable))
		}
	}
	return deps
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

func (op *AddColumnOp) DependsOn() []model.ObjectKey {
	return []model.ObjectKey{
		model.NewObjectKey(op.Schema, op.Table, model.KindTable),
	}
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
	return true
}

func (op *AlterColumnTypeOp) DependsOn() []model.ObjectKey {
	return []model.ObjectKey{
		model.NewObjectKey(op.Schema, op.Table, model.KindTable),
	}
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

func (op *SetNotNullOp) DependsOn() []model.ObjectKey {
	return []model.ObjectKey{
		model.NewObjectKey(op.Schema, op.Table, model.KindTable),
	}
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

func (op *DropNotNullOp) DependsOn() []model.ObjectKey {
	return []model.ObjectKey{
		model.NewObjectKey(op.Schema, op.Table, model.KindTable),
	}
}

// CreateIndexOp represents creating a new index
type CreateIndexOp struct {
	baseOperation
	Schema string
	Index  *model.Index
}

func NewCreateIndexOp(schema string, index *model.Index) *CreateIndexOp {
	return &CreateIndexOp{
		baseOperation: baseOperation{
			kind:      KindAddIndex,
			objectKey: model.NewObjectKey(schema, index.Name, model.KindIndex),
		},
		Schema: schema,
		Index:  index,
	}
}

func (op *CreateIndexOp) IsDestructive() bool {
	return false
}

func (op *CreateIndexOp) DependsOn() []model.ObjectKey {
	return []model.ObjectKey{
		model.NewObjectKey(op.Schema, op.Index.Table, model.KindTable),
	}
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

// DropColumnOp represents dropping a column
type DropColumnOp struct {
	baseOperation
	Schema string
	Table  string
	Column string
}

func NewDropColumnOp(schema, table, column string) *DropColumnOp {
	return &DropColumnOp{
		baseOperation: baseOperation{
			kind:      KindDropColumn,
			objectKey: model.NewObjectKey(schema, table+"."+column, model.KindColumn),
		},
		Schema: schema,
		Table:  table,
		Column: column,
	}
}

func (op *DropColumnOp) IsDestructive() bool {
	return true
}

func (op *DropColumnOp) DependsOn() []model.ObjectKey {
	return []model.ObjectKey{
		model.NewObjectKey(op.Schema, op.Table, model.KindTable),
	}
}

// AddEnumLabelOp represents adding a label to an enum type
type AddEnumLabelOp struct {
	baseOperation
	Schema string
	Type   string
	Label  string
}

func NewAddEnumLabelOp(schema, typeName, label string) *AddEnumLabelOp {
	return &AddEnumLabelOp{
		baseOperation: baseOperation{
			kind:      KindAddEnumLabel,
			objectKey: model.NewObjectKey(schema, typeName+"."+label, model.KindType),
		},
		Schema: schema,
		Type:   typeName,
		Label:  label,
	}
}

func (op *AddEnumLabelOp) IsDestructive() bool {
	return false
}

func (op *AddEnumLabelOp) DependsOn() []model.ObjectKey {
	return []model.ObjectKey{
		model.NewObjectKey(op.Schema, op.Type, model.KindType),
	}
}

// SetDefaultOp represents setting a default expression on a column
type SetDefaultOp struct {
	baseOperation
	Schema      string
	Table       string
	Column      string
	DefaultExpr string
}

func NewSetDefaultOp(schema, table, column, defaultExpr string) *SetDefaultOp {
	return &SetDefaultOp{
		baseOperation: baseOperation{
			kind:      KindSetDefault,
			objectKey: model.NewObjectKey(schema, table+"."+column, model.KindColumn),
		},
		Schema:      schema,
		Table:       table,
		Column:      column,
		DefaultExpr: defaultExpr,
	}
}

func (op *SetDefaultOp) IsDestructive() bool {
	return false
}

func (op *SetDefaultOp) DependsOn() []model.ObjectKey {
	return []model.ObjectKey{
		model.NewObjectKey(op.Schema, op.Table, model.KindTable),
	}
}

// DropDefaultOp represents dropping a default expression from a column
type DropDefaultOp struct {
	baseOperation
	Schema string
	Table  string
	Column string
}

func NewDropDefaultOp(schema, table, column string) *DropDefaultOp {
	return &DropDefaultOp{
		baseOperation: baseOperation{
			kind:      KindDropDefault,
			objectKey: model.NewObjectKey(schema, table+"."+column, model.KindColumn),
		},
		Schema: schema,
		Table:  table,
		Column: column,
	}
}

func (op *DropDefaultOp) IsDestructive() bool {
	return false
}

func (op *DropDefaultOp) DependsOn() []model.ObjectKey {
	return []model.ObjectKey{
		model.NewObjectKey(op.Schema, op.Table, model.KindTable),
	}
}

type AddConstraintOp struct {
	baseOperation
	Schema     string
	Table      string
	Constraint *model.Constraint
}

func NewAddConstraintOp(schema, table string, c *model.Constraint) *AddConstraintOp {
	return &AddConstraintOp{
		baseOperation: baseOperation{kind: KindAddConstraint, objectKey: model.NewObjectKey(schema, table+"."+c.Name, model.KindConstraint)},
		Schema:        schema,
		Table:         table,
		Constraint:    c,
	}
}

func (op *AddConstraintOp) IsDestructive() bool {
	return false
}

func (op *AddConstraintOp) DependsOn() []model.ObjectKey {
	return []model.ObjectKey{
		model.NewObjectKey(op.Schema, op.Table, model.KindTable),
		// Ensure ADD CONSTRAINT runs after DROP CONSTRAINT for the same constraint
		model.NewObjectKey(op.Schema, op.Table+"."+op.Constraint.Name, model.KindConstraint),
	}
}

type DropConstraintOp struct {
	baseOperation
	Schema string
	Table  string
	Name   string
}

func NewDropConstraintOp(schema, table, name string) *DropConstraintOp {
	return &DropConstraintOp{
		baseOperation: baseOperation{kind: KindDropConstraint, objectKey: model.NewObjectKey(schema, table+"."+name, model.KindConstraint)},
		Schema:        schema,
		Table:         table,
		Name:          name,
	}
}

func (op *DropConstraintOp) IsDestructive() bool {
	return true
}
