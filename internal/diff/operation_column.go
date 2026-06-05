package diff

import "github.com/fred29910/migra-go/internal/model"

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

type AlterColumnTypeOp struct {
	baseOperation
	Schema    string
	Table     string
	Column    string
	FromType  string
	ToType    string
	UsingExpr string
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

type SetIdentityOp struct {
	baseOperation
	Schema       string
	Table        string
	Column       string
	IdentityKind string
}

func NewSetIdentityOp(schema, table, column, identityKind string) *SetIdentityOp {
	return &SetIdentityOp{
		baseOperation: baseOperation{
			kind:      KindSetIdentity,
			objectKey: model.NewObjectKey(schema, table+"."+column, model.KindColumn),
		},
		Schema:       schema,
		Table:        table,
		Column:       column,
		IdentityKind: identityKind,
	}
}

func (op *SetIdentityOp) IsDestructive() bool {
	return false
}

func (op *SetIdentityOp) DependsOn() []model.ObjectKey {
	return []model.ObjectKey{
		model.NewObjectKey(op.Schema, op.Table, model.KindTable),
	}
}

type DropIdentityOp struct {
	baseOperation
	Schema string
	Table  string
	Column string
}

func NewDropIdentityOp(schema, table, column string) *DropIdentityOp {
	return &DropIdentityOp{
		baseOperation: baseOperation{
			kind:      KindDropIdentity,
			objectKey: model.NewObjectKey(schema, table+"."+column, model.KindColumn),
		},
		Schema: schema,
		Table:  table,
		Column: column,
	}
}

func (op *DropIdentityOp) IsDestructive() bool {
	return true
}

func (op *DropIdentityOp) DependsOn() []model.ObjectKey {
	return []model.ObjectKey{
		model.NewObjectKey(op.Schema, op.Table, model.KindTable),
	}
}

type AddIdentityOp struct {
	baseOperation
	Schema       string
	Table        string
	Column       string
	IdentityKind string
}

func NewAddIdentityOp(schema, table, column, identityKind string) *AddIdentityOp {
	return &AddIdentityOp{
		baseOperation: baseOperation{
			kind:      KindAddIdentity,
			objectKey: model.NewObjectKey(schema, table+"."+column, model.KindColumn),
		},
		Schema:       schema,
		Table:        table,
		Column:       column,
		IdentityKind: identityKind,
	}
}

func (op *AddIdentityOp) IsDestructive() bool { return false }

func (op *AddIdentityOp) DependsOn() []model.ObjectKey {
	return []model.ObjectKey{model.NewObjectKey(op.Schema, op.Table, model.KindTable)}
}

type AlterColumnCollationOp struct {
	baseOperation
	Schema        string
	Table         string
	Column        string
	DataType      string
	FromCollation string
	ToCollation   string
}

func NewAlterColumnCollationOp(schema, table, column, dataType, fromCollation, toCollation string) *AlterColumnCollationOp {
	return &AlterColumnCollationOp{
		baseOperation: baseOperation{
			kind:      KindAlterColumnCollation,
			objectKey: model.NewObjectKey(schema, table+"."+column, model.KindColumn),
		},
		Schema:        schema,
		Table:         table,
		Column:        column,
		DataType:      dataType,
		FromCollation: fromCollation,
		ToCollation:   toCollation,
	}
}

func (op *AlterColumnCollationOp) IsDestructive() bool {
	return false
}

func (op *AlterColumnCollationOp) DependsOn() []model.ObjectKey {
	return []model.ObjectKey{
		model.NewObjectKey(op.Schema, op.Table, model.KindTable),
	}
}
