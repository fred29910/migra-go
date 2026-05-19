package parser

import (
	"testing"

	"github.com/fred29910/migra-go/internal/model"
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

func TestCreateTableHandler_WithCollation(t *testing.T) {
	node := mustParseFirstStmt(t, `CREATE TABLE users (name text COLLATE "en_US.UTF-8")`)
	h := &CreateTableHandler{}

	mutations, err := h.Handle(node)
	require.NoError(t, err)
	require.Len(t, mutations, 1)

	mut, ok := mutations[0].(CreateTableMutation)
	require.True(t, ok)
	require.Len(t, mut.Columns, 1)
	assert.Equal(t, "name", mut.Columns[0].Name)
	assert.Equal(t, "text", mut.Columns[0].DataType)
	assert.Equal(t, "en_US.UTF-8", mut.Columns[0].Collation)
}

func TestCreateTableHandler_WithCollationAndNotNull(t *testing.T) {
	node := mustParseFirstStmt(t, `CREATE TABLE t (s text COLLATE "de_DE" NOT NULL)`)
	h := &CreateTableHandler{}

	mutations, err := h.Handle(node)
	require.NoError(t, err)
	require.Len(t, mutations, 1)

	mut, ok := mutations[0].(CreateTableMutation)
	require.True(t, ok)
	require.Len(t, mut.Columns, 1)
	assert.Equal(t, "s", mut.Columns[0].Name)
	assert.Equal(t, "de_DE", mut.Columns[0].Collation)
	assert.False(t, mut.Columns[0].IsNullable)
}

func TestCreateTableHandler_WithoutCollation(t *testing.T) {
	node := mustParseFirstStmt(t, `CREATE TABLE users (name text)`)
	h := &CreateTableHandler{}

	mutations, err := h.Handle(node)
	require.NoError(t, err)
	require.Len(t, mutations, 1)

	mut, ok := mutations[0].(CreateTableMutation)
	require.True(t, ok)
	require.Len(t, mut.Columns, 1)
	assert.Equal(t, "name", mut.Columns[0].Name)
	assert.Equal(t, "", mut.Columns[0].Collation, "expected empty collation for default")
}

func TestAlterTableHandler_AddColumnWithCollation(t *testing.T) {
	node := mustParseFirstStmt(t, `ALTER TABLE users ADD COLUMN full_name text COLLATE "en_US.UTF-8"`)
	h := &AlterTableHandler{}

	mutations, err := h.Handle(node)
	require.NoError(t, err)
	require.Len(t, mutations, 1)

	mut, ok := mutations[0].(AddColumnMutation)
	require.True(t, ok)
	assert.Equal(t, "full_name", mut.Column.Name)
	assert.Equal(t, "en_US.UTF-8", mut.Column.Collation)
}

func TestAlterTableHandler_AddColumnWithoutCollation(t *testing.T) {
	node := mustParseFirstStmt(t, `ALTER TABLE users ADD COLUMN bio text`)
	h := &AlterTableHandler{}

	mutations, err := h.Handle(node)
	require.NoError(t, err)
	require.Len(t, mutations, 1)

	mut, ok := mutations[0].(AddColumnMutation)
	require.True(t, ok)
	assert.Equal(t, "", mut.Column.Collation, "expected empty collation for default")
}

func TestCreateTableHandler_ForeignKeyCascade(t *testing.T) {
	sql := `CREATE TABLE orders (
        id integer PRIMARY KEY,
        user_id integer REFERENCES users(id) ON DELETE CASCADE ON UPDATE SET NULL
    )`
	node := mustParseFirstStmt(t, sql)
	handler := &CreateTableHandler{}
	mutations, err := handler.Handle(node)
	require.NoError(t, err)
	require.Len(t, mutations, 1)
	ctMut := mutations[0].(CreateTableMutation)
	require.Len(t, ctMut.Constraints, 2)

	var fkConstraint *model.Constraint
	for _, c := range ctMut.Constraints {
		if c.Type == "foreign_key" {
			fkConstraint = &c
			break
		}
	}
	require.NotNil(t, fkConstraint)
	assert.Equal(t, "CASCADE", fkConstraint.OnDelete)
	assert.Equal(t, "SET NULL", fkConstraint.OnUpdate)
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
