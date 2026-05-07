package parser

import (
	"fmt"

	"github.com/fred29910/migra-go/internal/model"
)

// MutationKind identifies the type of schema mutation.
type MutationKind string

const (
	MutKindCreateTable    MutationKind = "create_table"
	MutKindAddColumn      MutationKind = "add_column"
	MutKindCreateEnumType MutationKind = "create_enum_type"
	MutKindCreateIndex    MutationKind = "create_index"
)

// SchemaMutation is a self-describing and self-applying schema change.
type SchemaMutation interface {
	Kind() MutationKind
	Target() model.ObjectKey
	Apply(schema *model.Schema) error
}

// CreateTableMutation describes creating a new table.
type CreateTableMutation struct {
	Schema  string
	Name    string
	Columns []model.Column
}

func (m CreateTableMutation) Kind() MutationKind { return MutKindCreateTable }
func (m CreateTableMutation) Target() model.ObjectKey {
	return model.NewObjectKey(m.Schema, m.Name, model.KindTable)
}
func (m CreateTableMutation) Apply(schema *model.Schema) error {
	return fmt.Errorf("CreateTableMutation.Apply not implemented yet")
}

// AddColumnMutation describes adding a column to an existing table.
type AddColumnMutation struct {
	Schema string
	Table  string
	Column model.Column
}

func (m AddColumnMutation) Kind() MutationKind { return MutKindAddColumn }
func (m AddColumnMutation) Target() model.ObjectKey {
	return model.NewObjectKey(m.Schema, m.Table+"."+m.Column.Name, model.KindColumn)
}
func (m AddColumnMutation) Apply(schema *model.Schema) error {
	return fmt.Errorf("AddColumnMutation.Apply not implemented yet")
}

// CreateEnumTypeMutation describes creating an enum type.
type CreateEnumTypeMutation struct {
	Schema string
	Name   string
	Labels []string
}

func (m CreateEnumTypeMutation) Kind() MutationKind { return MutKindCreateEnumType }
func (m CreateEnumTypeMutation) Target() model.ObjectKey {
	return model.NewObjectKey(m.Schema, m.Name, model.KindType)
}
func (m CreateEnumTypeMutation) Apply(schema *model.Schema) error {
	return fmt.Errorf("CreateEnumTypeMutation.Apply not implemented yet")
}
