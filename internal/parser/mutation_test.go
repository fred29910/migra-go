package parser

import (
	"testing"

	"github.com/fred29910/migra-go/internal/model"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestCreateTableMutation_Apply(t *testing.T) {
	schema := model.NewSchema()
	mut := CreateTableMutation{
		Schema: "public",
		Name:   "users",
		Columns: []model.Column{
			{Name: "id", DataType: "integer", IsNullable: false},
			{Name: "name", DataType: "varchar(50)", IsNullable: true},
		},
	}

	err := mut.Apply(schema)
	require.NoError(t, err)

	ns := schema.GetNamespace("public")
	require.NotNil(t, ns)
	table := ns.Tables["users"]
	require.NotNil(t, table)
	assert.Equal(t, "users", table.Name)
	assert.Len(t, table.Columns, 2)
	assert.Equal(t, "id", table.Columns[0].Name)
}

func TestCreateTableMutation_Apply_Duplicate(t *testing.T) {
	schema := model.NewSchema()
	mut1 := CreateTableMutation{Schema: "public", Name: "users", Columns: []model.Column{{Name: "id"}}}
	mut2 := CreateTableMutation{Schema: "public", Name: "users", Columns: []model.Column{{Name: "id"}}}

	require.NoError(t, mut1.Apply(schema))
	err := mut2.Apply(schema)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "already exists")
}

func TestAddColumnMutation_Apply_ToExistingTable(t *testing.T) {
	schema := model.NewSchema()
	mutTable := CreateTableMutation{Schema: "public", Name: "users", Columns: []model.Column{{Name: "id"}}}
	require.NoError(t, mutTable.Apply(schema))

	mutCol := AddColumnMutation{Schema: "public", Table: "users", Column: model.Column{Name: "age", DataType: "integer"}}
	err := mutCol.Apply(schema)
	require.NoError(t, err)

	ns := schema.GetNamespace("public")
	require.Len(t, ns.Tables["users"].Columns, 2)
	assert.Equal(t, "age", ns.Tables["users"].Columns[1].Name)
}

func TestAddColumnMutation_Apply_CreatesPlaceholderTable(t *testing.T) {
	schema := model.NewSchema()
	mutCol := AddColumnMutation{Schema: "public", Table: "users", Column: model.Column{Name: "age", DataType: "integer"}}
	err := mutCol.Apply(schema)
	require.NoError(t, err)

	ns := schema.GetNamespace("public")
	require.NotNil(t, ns.Tables["users"])
	assert.Len(t, ns.Tables["users"].Columns, 1)
}
