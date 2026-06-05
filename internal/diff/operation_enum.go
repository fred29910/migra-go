package diff

import "github.com/fred29910/migra-go/internal/model"

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
