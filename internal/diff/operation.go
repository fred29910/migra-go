package diff

import "github.com/fred29910/migra-go/internal/model"

type Kind string

const (
	KindAddTable             Kind = "add_table"
	KindDropTable            Kind = "drop_table"
	KindAddColumn            Kind = "add_column"
	KindDropColumn           Kind = "drop_column"
	KindAlterColumnType      Kind = "alter_column_type"
	KindSetNotNull           Kind = "set_not_null"
	KindDropNotNull          Kind = "drop_not_null"
	KindAddIndex             Kind = "add_index"
	KindDropIndex            Kind = "drop_index"
	KindAddConstraint        Kind = "add_constraint"
	KindDropConstraint       Kind = "drop_constraint"
	KindAddEnumType          Kind = "add_enum_type"
	KindDropEnumType         Kind = "drop_enum_type"
	KindAddEnumLabel         Kind = "add_enum_label"
	KindSetDefault           Kind = "set_default"
	KindDropDefault          Kind = "drop_default"
	KindAlterColumnCollation Kind = "alter_column_collation"
	KindCreateSchema         Kind = "create_schema"
	KindDropSchema           Kind = "drop_schema"
	KindSetIdentity          Kind = "set_identity"
	KindDropIdentity         Kind = "drop_identity"
	KindAddIdentity          Kind = "add_identity"
	KindRenameColumn         Kind = "rename_column"

	KindCreateView             Kind = "create_view"
	KindDropView               Kind = "drop_view"
	KindReplaceView            Kind = "replace_view"
	KindCreateMaterializedView Kind = "create_materialized_view"
	KindDropMaterializedView   Kind = "drop_materialized_view"
	KindCreateSequence         Kind = "create_sequence"
	KindDropSequence           Kind = "drop_sequence"
	KindAlterSequence          Kind = "alter_sequence"
	KindCreateExtension        Kind = "create_extension"
	KindDropExtension          Kind = "drop_extension"
	KindAlterExtensionUpdate   Kind = "alter_extension_update"
)

type Operation interface {
	Kind() Kind
	ObjectKey() model.ObjectKey
	DependsOn() []model.ObjectKey
	IsDestructive() bool
}

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
