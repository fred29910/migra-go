package parser

import (
	"testing"

	pg_query "github.com/pganalyze/pg_query_go/v6"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func mustParseFirstStmt(t *testing.T, sql string) *pg_query.Node {
	t.Helper()
	tree, err := pg_query.Parse(sql)
	require.NoError(t, err)
	require.Len(t, tree.Stmts, 1)
	return tree.Stmts[0].Stmt
}

func TestCreateTableHandler(t *testing.T) {
	node := mustParseFirstStmt(t, "CREATE TABLE users (id integer NOT NULL, name varchar(50))")
	h := &CreateTableHandler{}

	mutations, err := h.Handle(node)
	require.NoError(t, err)
	require.Len(t, mutations, 1)

	mut, ok := mutations[0].(CreateTableMutation)
	require.True(t, ok)
	assert.Equal(t, "users", mut.Name)
	assert.Equal(t, "public", mut.Schema)
	assert.Len(t, mut.Columns, 2)
	assert.Equal(t, "id", mut.Columns[0].Name)
	assert.Equal(t, "integer", mut.Columns[0].DataType)
	assert.False(t, mut.Columns[0].IsNullable)
	assert.Equal(t, "name", mut.Columns[1].Name)
}

func TestAlterTableHandler_AddColumn(t *testing.T) {
	node := mustParseFirstStmt(t, "ALTER TABLE users ADD COLUMN age integer")
	h := &AlterTableHandler{}

	mutations, err := h.Handle(node)
	require.NoError(t, err)
	require.Len(t, mutations, 1)

	mut, ok := mutations[0].(AddColumnMutation)
	require.True(t, ok)
	assert.Equal(t, "users", mut.Table)
	assert.Equal(t, "public", mut.Schema)
	assert.Equal(t, "age", mut.Column.Name)
	assert.Equal(t, "integer", mut.Column.DataType)
}

func TestAlterTableHandler_MultipleAddColumns(t *testing.T) {
	node := mustParseFirstStmt(t, "ALTER TABLE users ADD COLUMN age integer, ADD COLUMN email varchar(100)")
	h := &AlterTableHandler{}

	mutations, err := h.Handle(node)
	require.NoError(t, err)
	require.Len(t, mutations, 2)

	mut0, ok := mutations[0].(AddColumnMutation)
	require.True(t, ok)
	assert.Equal(t, "age", mut0.Column.Name)

	mut1, ok := mutations[1].(AddColumnMutation)
	require.True(t, ok)
	assert.Equal(t, "email", mut1.Column.Name)
}

func TestHandlers_TypeSafety(t *testing.T) {
	node := &pg_query.Node{Node: &pg_query.Node_VacuumStmt{}}

	t.Run("CreateTableHandler", func(t *testing.T) {
		h := &CreateTableHandler{}
		_, err := h.Handle(node)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "expected")
	})

	t.Run("AlterTableHandler", func(t *testing.T) {
		h := &AlterTableHandler{}
		_, err := h.Handle(node)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "expected")
	})

	t.Run("CreateSchemaHandler", func(t *testing.T) {
		h := &CreateSchemaHandler{}
		_, err := h.Handle(node)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "expected")
	})
}

func TestCreateSchemaHandler(t *testing.T) {
	node := mustParseFirstStmt(t, "CREATE SCHEMA auth")
	h := &CreateSchemaHandler{}

	mutations, err := h.Handle(node)
	require.NoError(t, err)
	require.Len(t, mutations, 1)

	mut, ok := mutations[0].(CreateSchemaMutation)
	require.True(t, ok)
	assert.Equal(t, "auth", mut.Schema)
}

func TestCreateSchemaHandler_IfNotExists(t *testing.T) {
	node := mustParseFirstStmt(t, "CREATE SCHEMA IF NOT EXISTS auth")
	h := &CreateSchemaHandler{}

	mutations, err := h.Handle(node)
	require.NoError(t, err)
	require.Len(t, mutations, 1)

	mut, ok := mutations[0].(CreateSchemaMutation)
	require.True(t, ok)
	assert.Equal(t, "auth", mut.Schema)
}
