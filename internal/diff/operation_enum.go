package diff

import (
	"fmt"
	"strings"

	"github.com/fred29910/migra-go/internal/model"
	"github.com/fred29910/migra-go/internal/util"
)

// AddEnumTypeOp represents an operation to create a new enum type.
type AddEnumTypeOp struct {
	baseOperation
	Schema string
	Type   *model.EnumType
}

// NewAddEnumTypeOp creates a new AddEnumTypeOp.
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

// IsDestructive returns false; creating an enum type is not destructive.
func (op *AddEnumTypeOp) IsDestructive() bool {
	return false
}

// RenderString renders the SQL statement for creating an enum type.
func (op *AddEnumTypeOp) RenderString(ctx RenderContext) string {
	labels := make([]string, len(op.Type.Labels))
	for i, label := range op.Type.Labels {
		labels[i] = "'" + strings.ReplaceAll(label, "'", "''") + "'"
	}
	sql := fmt.Sprintf("CREATE TYPE %s AS ENUM (%s)",
		util.QuoteQualifiedIdentifier(op.Schema, op.Type.Name), strings.Join(labels, ", "))
	return fmt.Sprintf("-- op: add_enum_type risk:low\n%s;", sql)
}

// DropEnumTypeOp represents an operation to drop an enum type.
type DropEnumTypeOp struct {
	baseOperation
	Schema string
	Name   string
}

// NewDropEnumTypeOp creates a new DropEnumTypeOp.
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

// IsDestructive returns true; dropping an enum type is destructive.
func (op *DropEnumTypeOp) IsDestructive() bool {
	return true
}

// RenderString renders the SQL statement for dropping an enum type.
func (op *DropEnumTypeOp) RenderString(ctx RenderContext) string {
	ifExists := ""
	if ctx.UseIfExists() {
		ifExists = "IF EXISTS "
	}
	return fmt.Sprintf("-- op: drop_enum_type risk:high\nDROP TYPE %s%s;",
		ifExists, util.QuoteQualifiedIdentifier(op.Schema, op.Name))
}

// AddEnumLabelOp represents an operation to add a label to an existing enum type.
type AddEnumLabelOp struct {
	baseOperation
	Schema string
	Type   string
	Label  string
}

// NewAddEnumLabelOp creates a new AddEnumLabelOp.
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

// IsDestructive returns false; adding an enum label is not destructive.
func (op *AddEnumLabelOp) IsDestructive() bool {
	return false
}

// RenderString renders the SQL statement for adding an enum label.
func (op *AddEnumLabelOp) RenderString(ctx RenderContext) string {
	return fmt.Sprintf("-- op: add_enum_label risk:low\nALTER TYPE %s ADD VALUE %s;",
		util.QuoteQualifiedIdentifier(op.Schema, op.Type),
		"'"+strings.ReplaceAll(op.Label, "'", "''")+"'",
	)
}

// DependsOn returns the object keys this operation depends on.
func (op *AddEnumLabelOp) DependsOn() []model.ObjectKey {
	return []model.ObjectKey{
		model.NewObjectKey(op.Schema, op.Type, model.KindType),
	}
}
