package parser

import (
	"testing"

	"github.com/fred29910/migra-go/internal/model"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestRenameColumnMutation_Kind(t *testing.T) {
	mut := RenameColumnMutation{Schema: "public", Table: "users", OldName: "username", NewName: "login_name"}
	assert.Equal(t, MutKindRenameColumn, mut.Kind())
}

func TestRenameColumnMutation_Target(t *testing.T) {
	mut := RenameColumnMutation{Schema: "public", Table: "users", OldName: "username", NewName: "login_name"}
	key := mut.Target()
	assert.Equal(t, "public", key.Schema)
	assert.Equal(t, "users.login_name", key.Name)
	assert.Equal(t, model.KindColumn, key.Kind)
}

func TestRenameColumnMutation_Apply_RenamesColumn(t *testing.T) {
	schema := model.NewSchema()
	ns := schema.GetOrCreateNamespace("public")
	table := model.NewTable("public", "users")
	table.AddColumn(&model.Column{Name: "username", DataType: "text", IsNullable: true})
	table.AddColumn(&model.Column{Name: "email", DataType: "text"})
	ns.Tables["users"] = table

	mut := RenameColumnMutation{
		Schema:  "public",
		Table:   "users",
		OldName: "username",
		NewName: "login_name",
	}
	err := mut.Apply(schema)
	require.NoError(t, err)

	col := table.ColumnByName["login_name"]
	require.NotNil(t, col, "new column name should exist in ColumnByName")
	assert.Equal(t, "login_name", col.Name)
	assert.Nil(t, table.ColumnByName["username"], "old column name should be removed from ColumnByName")
	assert.Len(t, table.Columns, 2)
	assert.Equal(t, "login_name", table.Columns[0].Name)
	assert.Equal(t, "email", table.Columns[1].Name)
	assert.Equal(t, "text", col.DataType)
	assert.True(t, col.IsNullable)
}

func TestRenameColumnMutation_Apply_TableNotFound(t *testing.T) {
	// Missing table creates a placeholder (no error).
	schema := model.NewSchema()
	schema.GetOrCreateNamespace("public")
	mut := RenameColumnMutation{Schema: "public", Table: "nonexistent", OldName: "x", NewName: "y"}
	err := mut.Apply(schema)
	require.NoError(t, err)
	table := schema.GetNamespace("public").Tables["nonexistent"]
	require.NotNil(t, table)
	assert.True(t, table.IsPlaceholder)
}

func TestRenameColumnMutation_Apply_ColumnNotFound(t *testing.T) {
	schema := model.NewSchema()
	ns := schema.GetOrCreateNamespace("public")
	ns.Tables["users"] = model.NewTable("public", "users")
	mut := RenameColumnMutation{Schema: "public", Table: "users", OldName: "nope", NewName: "y"}
	err := mut.Apply(schema)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "not found")
}

func TestRenameColumnMutation_Apply_NameCollision(t *testing.T) {
	schema := model.NewSchema()
	ns := schema.GetOrCreateNamespace("public")
	table := model.NewTable("public", "users")
	table.AddColumn(&model.Column{Name: "username", DataType: "text"})
	table.AddColumn(&model.Column{Name: "email", DataType: "text"})
	ns.Tables["users"] = table

	mut := RenameColumnMutation{Schema: "public", Table: "users", OldName: "username", NewName: "email"}
	err := mut.Apply(schema)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "already exists")
}

func TestRenameColumnMutation_Apply_NilNamespace(t *testing.T) {
	// GetOrCreateNamespace auto-creates the schema and missing table
	// creates a placeholder — no error expected.
	schema := model.NewSchema()
	mut := RenameColumnMutation{Schema: "public", Table: "users", OldName: "x", NewName: "y"}
	err := mut.Apply(schema)
	require.NoError(t, err)
	ns := schema.GetNamespace("public")
	require.NotNil(t, ns)
	table := ns.Tables["users"]
	require.NotNil(t, table)
	assert.True(t, table.IsPlaceholder)
}
