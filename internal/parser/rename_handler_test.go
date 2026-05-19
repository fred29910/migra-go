package parser

import (
	"testing"

	pg_query "github.com/pganalyze/pg_query_go/v6"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestRenameStmtHandler_Handle_RenameColumn(t *testing.T) {
	node := mustParseFirstStmt(t, "ALTER TABLE users RENAME COLUMN sku TO item_code")
	h := &RenameStmtHandler{}
	mutations, err := h.Handle(node)
	require.NoError(t, err)
	require.Len(t, mutations, 1)

	mut, ok := mutations[0].(RenameColumnMutation)
	require.True(t, ok)
	assert.Equal(t, "users", mut.Table)
	assert.Equal(t, "public", mut.Schema)
	assert.Equal(t, "sku", mut.OldName)
	assert.Equal(t, "item_code", mut.NewName)
}

func TestRenameStmtHandler_Handle_RenameColumn_SchemaQualified(t *testing.T) {
	node := mustParseFirstStmt(t, "ALTER TABLE inventory.items RENAME COLUMN sku TO item_code")
	h := &RenameStmtHandler{}
	mutations, err := h.Handle(node)
	require.NoError(t, err)
	require.Len(t, mutations, 1)

	mut, ok := mutations[0].(RenameColumnMutation)
	require.True(t, ok)
	assert.Equal(t, "items", mut.Table)
	assert.Equal(t, "inventory", mut.Schema)
	assert.Equal(t, "sku", mut.OldName)
	assert.Equal(t, "item_code", mut.NewName)
}

func TestRenameStmtHandler_Handle_WrongNodeType(t *testing.T) {
	node := mustParseFirstStmt(t, "CREATE TABLE users (id integer)")
	h := &RenameStmtHandler{}
	_, err := h.Handle(node)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "expected RenameStmt")
}

func TestRenameStmtHandler_RenameTypeCheck(t *testing.T) {
	node := mustParseFirstStmt(t, "ALTER TABLE users RENAME COLUMN sku TO item_code")
	stmt := node.GetRenameStmt()
	require.NotNil(t, stmt)
	assert.Equal(t, pg_query.ObjectType_OBJECT_COLUMN, stmt.RenameType)
	assert.Equal(t, "sku", stmt.Subname)
	assert.Equal(t, "item_code", stmt.Newname)
}

func TestRenameStmtHandler_Integration(t *testing.T) {
	parser := NewParser()
	schema, err := parser.ParseSQL(`
		CREATE TABLE users (id integer, username text);
		ALTER TABLE users RENAME COLUMN username TO login_name;
	`)
	require.NoError(t, err)

	ns := schema.GetNamespace("public")
	require.NotNil(t, ns)
	table := ns.Tables["users"]
	require.NotNil(t, table)
	require.False(t, table.IsPlaceholder)

	assert.Nil(t, table.ColumnByName["username"])
	col := table.ColumnByName["login_name"]
	require.NotNil(t, col)
	assert.Equal(t, "login_name", col.Name)
	assert.Equal(t, "text", col.DataType)
	assert.True(t, col.IsNullable)
	assert.Len(t, table.Columns, 2)
	assert.Equal(t, "id", table.Columns[0].Name)
	assert.Equal(t, "login_name", table.Columns[1].Name)
}

func TestRenameStmtHandler_Integration_Placeholder(t *testing.T) {
	parser := NewParser()
	schema, err := parser.ParseSQL("ALTER TABLE users RENAME COLUMN old_name TO new_name;")
	require.NoError(t, err)
	ns := schema.GetNamespace("public")
	require.NotNil(t, ns)
	table := ns.Tables["users"]
	require.NotNil(t, table)
	assert.True(t, table.IsPlaceholder)
	assert.Nil(t, table.ColumnByName["old_name"])
	assert.Nil(t, table.ColumnByName["new_name"])
}
