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
