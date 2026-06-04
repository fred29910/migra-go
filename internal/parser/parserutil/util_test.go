package parserutil

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	pg_query "github.com/pganalyze/pg_query_go/v6"
)

func mustParseFirstStmt(t *testing.T, sql string) *pg_query.Node {
	t.Helper()
	tree, err := pg_query.Parse(sql)
	require.NoError(t, err)
	require.NotEmpty(t, tree.Stmts, "expected at least one statement")
	return tree.Stmts[0].Stmt
}

func getRangeVar(t *testing.T, sql string) *pg_query.RangeVar {
	t.Helper()
	node := mustParseFirstStmt(t, sql)
	createStmt := node.GetCreateStmt()
	require.NotNil(t, createStmt, "expected CreateStmt")
	return createStmt.Relation
}

func getColumnDef(t *testing.T, sql string) *pg_query.ColumnDef {
	t.Helper()
	node := mustParseFirstStmt(t, sql)
	createStmt := node.GetCreateStmt()
	require.NotNil(t, createStmt, "expected CreateStmt")
	require.NotEmpty(t, createStmt.TableElts, "expected at least one column")
	return createStmt.TableElts[0].GetColumnDef()
}

func TestMapTypeName(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected string
	}{
		{"int4 to integer", "int4", "integer"},
		{"int8 to bigint", "int8", "bigint"},
		{"float4 to real", "float4", "real"},
		{"float8 to double precision", "float8", "double precision"},
		{"serial to integer", "serial", "integer"},
		{"bigserial to bigint", "bigserial", "bigint"},
		{"smallserial to smallint", "smallserial", "smallint"},
		{"unknown type passes through", "varchar", "varchar"},
		{"empty string passes through", "", ""},
		{"text passes through", "text", "text"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := MapTypeName(tt.input)
			assert.Equal(t, tt.expected, got)
		})
	}
}

func TestExtractCollation(t *testing.T) {
	t.Run("with collation", func(t *testing.T) {
		colDef := getColumnDef(t, `CREATE TABLE t (name text COLLATE "en_US.UTF-8")`)
		got := ExtractCollation(colDef)
		assert.Equal(t, "en_US.UTF-8", got)
	})

	t.Run("without collation", func(t *testing.T) {
		colDef := getColumnDef(t, `CREATE TABLE t (name text)`)
		got := ExtractCollation(colDef)
		assert.Equal(t, "", got)
	})

	t.Run("nil CollClause", func(t *testing.T) {
		colDef := &pg_query.ColumnDef{}
		got := ExtractCollation(colDef)
		assert.Equal(t, "", got)
	})
}

func TestParseRelation(t *testing.T) {
	t.Run("with schema", func(t *testing.T) {
		rv := getRangeVar(t, `CREATE TABLE myschema.users (id integer)`)
		table, schema := ParseRelation(rv)
		assert.Equal(t, "users", table)
		assert.Equal(t, "myschema", schema)
	})

	t.Run("without schema defaults to public", func(t *testing.T) {
		rv := getRangeVar(t, `CREATE TABLE users (id integer)`)
		table, schema := ParseRelation(rv)
		assert.Equal(t, "users", table)
		assert.Equal(t, "public", schema)
	})

	t.Run("nil relation returns empty table and public schema", func(t *testing.T) {
		table, schema := ParseRelation(nil)
		assert.Equal(t, "", table)
		assert.Equal(t, "public", schema)
	})
}

func TestParseTypeName(t *testing.T) {
	t.Run("simple type", func(t *testing.T) {
		colDef := getColumnDef(t, `CREATE TABLE t (id integer)`)
		got := ParseTypeName(colDef.TypeName)
		assert.Equal(t, "integer", got)
	})

	t.Run("type with typmod", func(t *testing.T) {
		colDef := getColumnDef(t, `CREATE TABLE t (name varchar(100))`)
		got := ParseTypeName(colDef.TypeName)
		assert.Equal(t, "varchar(100)", got)
	})

	t.Run("mapped type int4", func(t *testing.T) {
		colDef := getColumnDef(t, `CREATE TABLE t (id int4)`)
		got := ParseTypeName(colDef.TypeName)
		assert.Equal(t, "integer", got)
	})

	t.Run("mapped type int8", func(t *testing.T) {
		colDef := getColumnDef(t, `CREATE TABLE t (id int8)`)
		got := ParseTypeName(colDef.TypeName)
		assert.Equal(t, "bigint", got)
	})

	t.Run("text type", func(t *testing.T) {
		colDef := getColumnDef(t, `CREATE TABLE t (bio text)`)
		got := ParseTypeName(colDef.TypeName)
		assert.Equal(t, "text", got)
	})

	t.Run("boolean type", func(t *testing.T) {
		colDef := getColumnDef(t, `CREATE TABLE t (active boolean)`)
		got := ParseTypeName(colDef.TypeName)
		assert.Equal(t, "bool", got)
	})

	t.Run("numeric with precision and scale", func(t *testing.T) {
		colDef := getColumnDef(t, `CREATE TABLE t (price numeric(10,2))`)
		got := ParseTypeName(colDef.TypeName)
		assert.Equal(t, "numeric(10,2)", got)
	})
}

func TestParseConstraintColumns(t *testing.T) {
	t.Run("single column", func(t *testing.T) {
		node := mustParseFirstStmt(t, "CREATE TABLE t (CONSTRAINT pk PRIMARY KEY (id))")
		createStmt := node.GetCreateStmt()
		require.NotNil(t, createStmt)

		var pkNode *pg_query.Node
		for _, elt := range createStmt.TableElts {
			if c := elt.GetConstraint(); c != nil && c.Contype == pg_query.ConstrType_CONSTR_PRIMARY {
				pkNode = elt
				break
			}
		}
		require.NotNil(t, pkNode, "expected primary key constraint")
		constraint := pkNode.GetConstraint()
		cols := ParseConstraintColumns(constraint.Keys)
		assert.Equal(t, []string{"id"}, cols)
	})

	t.Run("multi-column primary key", func(t *testing.T) {
		node := mustParseFirstStmt(t, "CREATE TABLE t (a integer, b integer, PRIMARY KEY (a, b))")
		createStmt := node.GetCreateStmt()
		require.NotNil(t, createStmt)

		var pkNode *pg_query.Node
		for _, elt := range createStmt.TableElts {
			if c := elt.GetConstraint(); c != nil && c.Contype == pg_query.ConstrType_CONSTR_PRIMARY {
				pkNode = elt
				break
			}
		}
		require.NotNil(t, pkNode, "expected primary key constraint")
		constraint := pkNode.GetConstraint()
		cols := ParseConstraintColumns(constraint.Keys)
		assert.Equal(t, []string{"a", "b"}, cols)
	})

	t.Run("empty keys", func(t *testing.T) {
		cols := ParseConstraintColumns(nil)
		assert.Empty(t, cols)
	})
}

func TestParseColumnDef(t *testing.T) {
	t.Run("nullable column with type", func(t *testing.T) {
		colDef := getColumnDef(t, `CREATE TABLE t (name text)`)
		col := ParseColumnDef(colDef)
		assert.Equal(t, "name", col.Name)
		assert.Equal(t, "text", col.DataType)
		assert.True(t, col.IsNullable)
		assert.Nil(t, col.DefaultExpr)
	})

	t.Run("not null column", func(t *testing.T) {
		colDef := getColumnDef(t, `CREATE TABLE t (id integer NOT NULL)`)
		col := ParseColumnDef(colDef)
		assert.Equal(t, "id", col.Name)
		assert.False(t, col.IsNullable)
	})

	t.Run("column with default value", func(t *testing.T) {
		colDef := getColumnDef(t, `CREATE TABLE t (status text DEFAULT 'active')`)
		col := ParseColumnDef(colDef)
		assert.Equal(t, "status", col.Name)
		assert.NotNil(t, col.DefaultExpr)
		assert.Equal(t, "'active'", *col.DefaultExpr)
	})

	t.Run("column with default integer", func(t *testing.T) {
		colDef := getColumnDef(t, `CREATE TABLE t (count integer DEFAULT 0)`)
		col := ParseColumnDef(colDef)
		assert.NotNil(t, col.DefaultExpr)
		assert.Equal(t, "0", *col.DefaultExpr)
	})

	t.Run("column with collation", func(t *testing.T) {
		colDef := getColumnDef(t, `CREATE TABLE t (name text COLLATE "de_DE")`)
		col := ParseColumnDef(colDef)
		assert.Equal(t, "de_DE", col.Collation)
	})

	t.Run("identity column ALWAYS", func(t *testing.T) {
		colDef := getColumnDef(t, `CREATE TABLE t (id integer GENERATED ALWAYS AS IDENTITY)`)
		col := ParseColumnDef(colDef)
		assert.True(t, col.IsIdentity)
		assert.Equal(t, "ALWAYS", col.IdentityKind)
	})

	t.Run("identity column BY DEFAULT", func(t *testing.T) {
		colDef := getColumnDef(t, `CREATE TABLE t (id integer GENERATED BY DEFAULT AS IDENTITY)`)
		col := ParseColumnDef(colDef)
		assert.True(t, col.IsIdentity)
		assert.Equal(t, "BY DEFAULT", col.IdentityKind)
	})

	t.Run("non-identity column", func(t *testing.T) {
		colDef := getColumnDef(t, `CREATE TABLE t (id serial)`)
		col := ParseColumnDef(colDef)
		assert.False(t, col.IsIdentity)
		assert.Empty(t, col.IdentityKind)
	})

	t.Run("column with varchar type", func(t *testing.T) {
		colDef := getColumnDef(t, `CREATE TABLE t (email varchar(255))`)
		col := ParseColumnDef(colDef)
		assert.Equal(t, "email", col.Name)
		assert.Equal(t, "varchar(255)", col.DataType)
	})

	t.Run("column with NOT NULL constraint sets IsNullable false", func(t *testing.T) {
		colDef := getColumnDef(t, `CREATE TABLE t (id integer CONSTRAINT not_null_id NOT NULL)`)
		col := ParseColumnDef(colDef)
		assert.False(t, col.IsNullable)
	})
}

func TestFormatExpression(t *testing.T) {
	t.Run("nil node", func(t *testing.T) {
		got := FormatExpression(nil)
		assert.Equal(t, "", got)
	})

	t.Run("string constant", func(t *testing.T) {
		node := mustParseFirstStmt(t, "SELECT * FROM t WHERE x = 'hello'")
		selectStmt := node.GetSelectStmt()
		require.NotNil(t, selectStmt)
		where := selectStmt.WhereClause
		require.NotNil(t, where)
		got := FormatExpression(where)
		assert.Contains(t, got, "x")
		assert.Contains(t, got, "hello")
	})

	t.Run("integer constant", func(t *testing.T) {
		node := mustParseFirstStmt(t, "SELECT * FROM t WHERE x = 42")
		selectStmt := node.GetSelectStmt()
		require.NotNil(t, selectStmt)
		where := selectStmt.WhereClause
		require.NotNil(t, where)
		got := FormatExpression(where)
		assert.Contains(t, got, "x = 42")
	})

	t.Run("column ref", func(t *testing.T) {
		node := mustParseFirstStmt(t, "SELECT id FROM t")
		selectStmt := node.GetSelectStmt()
		require.NotNil(t, selectStmt)
		require.NotEmpty(t, selectStmt.TargetList)
		got := FormatExpression(selectStmt.TargetList[0].GetResTarget().Val)
		assert.NotEmpty(t, got)
	})

	t.Run("NULL IS NULL", func(t *testing.T) {
		node := mustParseFirstStmt(t, "SELECT * FROM t WHERE x IS NULL")
		selectStmt := node.GetSelectStmt()
		require.NotNil(t, selectStmt)
		where := selectStmt.WhereClause
		require.NotNil(t, where)
		got := FormatExpression(where)
		assert.Equal(t, "x IS NULL", got)
	})

	t.Run("NOT expression", func(t *testing.T) {
		node := mustParseFirstStmt(t, "SELECT * FROM t WHERE NOT active")
		selectStmt := node.GetSelectStmt()
		require.NotNil(t, selectStmt)
		where := selectStmt.WhereClause
		require.NotNil(t, where)
		got := FormatExpression(where)
		assert.Contains(t, got, "NOT")
	})

	t.Run("AND expression", func(t *testing.T) {
		node := mustParseFirstStmt(t, "SELECT * FROM t WHERE a = 1 AND b = 2")
		selectStmt := node.GetSelectStmt()
		require.NotNil(t, selectStmt)
		where := selectStmt.WhereClause
		require.NotNil(t, where)
		got := FormatExpression(where)
		assert.Contains(t, got, "AND")
		assert.Contains(t, got, "a = 1")
		assert.Contains(t, got, "b = 2")
	})

	t.Run("OR expression", func(t *testing.T) {
		node := mustParseFirstStmt(t, "SELECT * FROM t WHERE a = 1 OR b = 2")
		selectStmt := node.GetSelectStmt()
		require.NotNil(t, selectStmt)
		where := selectStmt.WhereClause
		require.NotNil(t, where)
		got := FormatExpression(where)
		assert.Contains(t, got, "OR")
	})

	t.Run("IS NOT NULL", func(t *testing.T) {
		node := mustParseFirstStmt(t, "SELECT * FROM t WHERE x IS NOT NULL")
		selectStmt := node.GetSelectStmt()
		require.NotNil(t, selectStmt)
		where := selectStmt.WhereClause
		require.NotNil(t, where)
		got := FormatExpression(where)
		assert.Equal(t, "x IS NOT NULL", got)
	})

	t.Run("func call", func(t *testing.T) {
		node := mustParseFirstStmt(t, "SELECT lower(name) FROM t")
		selectStmt := node.GetSelectStmt()
		require.NotNil(t, selectStmt)
		require.NotEmpty(t, selectStmt.TargetList)
		got := FormatExpression(selectStmt.TargetList[0].GetResTarget().Val)
		assert.Contains(t, got, "lower")
	})

	t.Run("unsupported node returns empty", func(t *testing.T) {
		node := &pg_query.Node{Node: &pg_query.Node_VacuumStmt{}}
		got := FormatExpression(node)
		assert.Equal(t, "", got)
	})
}

func TestParseExpression(t *testing.T) {
	t.Run("string constant", func(t *testing.T) {
		node := mustParseFirstStmt(t, "SELECT * FROM t WHERE x = 'hello'")
		selectStmt := node.GetSelectStmt()
		require.NotNil(t, selectStmt)
		aExpr := selectStmt.WhereClause.GetAExpr()
		require.NotNil(t, aExpr)
		right := aExpr.Rexpr
		got, ok := ParseExpression(right)
		assert.True(t, ok)
		assert.Equal(t, "'hello'", got)
	})

	t.Run("integer constant", func(t *testing.T) {
		node := mustParseFirstStmt(t, "SELECT * FROM t WHERE x = 42")
		selectStmt := node.GetSelectStmt()
		require.NotNil(t, selectStmt)
		aExpr := selectStmt.WhereClause.GetAExpr()
		require.NotNil(t, aExpr)
		right := aExpr.Rexpr
		got, ok := ParseExpression(right)
		assert.True(t, ok)
		assert.Equal(t, "42", got)
	})

	t.Run("unsupported node returns false", func(t *testing.T) {
		node := &pg_query.Node{Node: &pg_query.Node_VacuumStmt{}}
		_, ok := ParseExpression(node)
		assert.False(t, ok)
	})
}

func TestDeparseNode(t *testing.T) {
	t.Run("deparse CREATE TABLE", func(t *testing.T) {
		node := mustParseFirstStmt(t, "CREATE TABLE users (id integer NOT NULL)")
		got := DeparseNode(node)
		assert.Contains(t, got, "CREATE TABLE users")
		assert.Contains(t, got, "id")
		assert.Contains(t, got, "NOT NULL")
	})

	t.Run("expression falls back to FormatExpression", func(t *testing.T) {
		node := mustParseFirstStmt(t, "SELECT * FROM t WHERE x = 1")
		selectStmt := node.GetSelectStmt()
		require.NotNil(t, selectStmt)
		where := selectStmt.WhereClause
		require.NotNil(t, where)
		got := DeparseNode(where)
		assert.Contains(t, got, "x = 1")
	})
}

func TestFormatExpression_TypeCast(t *testing.T) {
	node := mustParseFirstStmt(t, "SELECT x::text FROM t")
	selectStmt := node.GetSelectStmt()
	require.NotNil(t, selectStmt)
	require.NotEmpty(t, selectStmt.TargetList)
	got := FormatExpression(selectStmt.TargetList[0].GetResTarget().Val)
	assert.Contains(t, got, "::text")
}
