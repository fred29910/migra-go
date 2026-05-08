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

func TestMutationOrder_AlterBeforeCreate(t *testing.T) {
	schema := model.NewSchema()
	
	// 1. ALTER TABLE users ADD COLUMN age integer (happens first)
	alterMut := AddColumnMutation{
		Schema: "public",
		Table:  "users",
		Column: model.Column{Name: "age", DataType: "integer"},
	}
	require.NoError(t, alterMut.Apply(schema))

	// 2. CREATE TABLE users (id integer)
	createMut := CreateTableMutation{
		Schema: "public",
		Name:   "users",
		Columns: []model.Column{
			{Name: "id", DataType: "integer"},
		},
	}
	
	// This currently fails in current implementation because of "already exists"
	err := createMut.Apply(schema)
	require.NoError(t, err, "CREATE TABLE should merge with placeholder table created by ALTER TABLE")

	table := schema.GetNamespace("public").Tables["users"]
	assert.False(t, table.IsPlaceholder, "Table should no longer be a placeholder")
	assert.Len(t, table.Columns, 2)
	assert.Equal(t, "age", table.Columns[0].Name, "Existing column from ALTER should be first")
	assert.Equal(t, "id", table.Columns[1].Name, "New column from CREATE should be appended")
}

func TestMutationOrder_TypeConflict(t *testing.T) {
	schema := model.NewSchema()
	
	// 1. ALTER TABLE users ADD COLUMN age integer
	alterMut := AddColumnMutation{
		Schema: "public",
		Table:  "users",
		Column: model.Column{Name: "age", DataType: "integer"},
	}
	require.NoError(t, alterMut.Apply(schema))

	// 2. CREATE TABLE users (age text) -> Conflict!
	createMut := CreateTableMutation{
		Schema: "public",
		Name:   "users",
		Columns: []model.Column{
			{Name: "age", DataType: "text"},
		},
	}
	
	err := createMut.Apply(schema)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "type mismatch")
}
