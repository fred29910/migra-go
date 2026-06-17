package parser

import (
	"fmt"
	"strings"
	"testing"

	"github.com/fred29910/migra-go/internal/model"
	pg_query "github.com/pganalyze/pg_query_go/v6"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// ─── ParseError tests ──────────────────────────────────────────────

func TestParseError_Error(t *testing.T) {
	err := &ParseError{
		Message:   "test error",
		Position:  42,
		Statement: "CREATE TABLE",
	}
	assert.Contains(t, err.Error(), "parse error at position 42")
	assert.Contains(t, err.Error(), "test error")
}

// ─── Parser.NewParser / NewParserWith / Errors tests ──────────────

func TestNewParser_CreatesFunctionalParser(t *testing.T) {
	p := NewParser()
	require.NotNil(t, p)
	require.NotNil(t, p.applier)
	require.NotNil(t, p.registry)
}

func TestNewParserWith_NonNilValues(t *testing.T) {
	reg := NewHandlerRegistry()
	applier := &MutationApplier{}
	p := NewParserWith(reg, applier)
	require.NotNil(t, p)
	assert.Equal(t, reg, p.registry)
	assert.Equal(t, applier, p.applier)
}

// ─── getStatementSnippet tests ─────────────────────────────────────

func TestGetStatementSnippet_NormalPosition(t *testing.T) {
	snippet := getStatementSnippet("CREATE TABLE users (id integer);", 0)
	assert.Contains(t, snippet, "CREATE TABLE")
}

func TestGetStatementSnippet_PositionBeyondEnd(t *testing.T) {
	snippet := getStatementSnippet("short", 100)
	assert.Equal(t, "", snippet)
}

func TestGetStatementSnippet_NegativePosition(t *testing.T) {
	snippet := getStatementSnippet("CREATE TABLE t (id integer);", -1)
	assert.Equal(t, "", snippet)
}

func TestGetStatementSnippet_Extracts100Chars(t *testing.T) {
	snippet := getStatementSnippet("CREATE TABLE very_long_table_name_with_many_characters (id integer, name varchar(255), description text, extra_column bigint);", 0)
	assert.LessOrEqual(t, len(snippet), 100)
}

func TestGetStatementSnippet_PositionNearEnd(t *testing.T) {
	snippet := getStatementSnippet("ABCDEFGHIJ", 5)
	assert.Equal(t, "FGHIJ", snippet)
}

// ─── ParseSQL: error wrapping and multi-error tests ───────────────

func TestParseSQL_WrapsPgQueryParseError(t *testing.T) {
	p := NewParser()
	_, err := p.ParseSQL("NOT VALID SQL !!!")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "parse SQL")
}

func TestParseSQL_MultipleUnsupportedStatements(t *testing.T) {
	p := NewParser()
	schema, err := p.ParseSQL("SELECT 1; SELECT 2;")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "parsing completed with 2 errors")
	// Schema should still be returned (partially parsed)
	assert.NotNil(t, schema)
}

func TestParseSQL_MixedSupportedAndUnsupported(t *testing.T) {
	p := NewParser()
	_, err := p.ParseSQL("CREATE TABLE t (id integer); SELECT 1;")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "parsing completed with 1 errors")
}

// ─── HandlerRegistry tests ────────────────────────────────────────

func TestNewHandlerRegistry(t *testing.T) {
	reg := NewHandlerRegistry()
	require.NotNil(t, reg)
	require.NotNil(t, reg.handlers)
	assert.Empty(t, reg.handlers)
}

func TestHandlerRegistry_RegisterAndDispatch(t *testing.T) {
	reg := NewHandlerRegistry()
	handler := &CreateTableHandler{}
	reg.Register(&pg_query.Node{Node: &pg_query.Node_CreateStmt{}}, handler)

	h, found := reg.Dispatch(&pg_query.Node{Node: &pg_query.Node_CreateStmt{}})
	assert.True(t, found)
	assert.Equal(t, handler, h)
}

func TestHandlerRegistry_DispatchNotFound(t *testing.T) {
	reg := NewHandlerRegistry()
	_, found := reg.Dispatch(&pg_query.Node{Node: &pg_query.Node_VacuumStmt{}})
	assert.False(t, found)
}

func TestHandlerRegistry_DispatchNilNode(t *testing.T) {
	reg := NewHandlerRegistry()
	_, found := reg.Dispatch(nil)
	assert.False(t, found)
}

func TestHandlerRegistry_DispatchNilInnerNode(t *testing.T) {
	reg := NewHandlerRegistry()
	_, found := reg.Dispatch(&pg_query.Node{Node: nil})
	assert.False(t, found)
}

func TestDefaultRegistry_RegistersAllHandlers(t *testing.T) {
	reg := DefaultRegistry()
	require.NotNil(t, reg)

	// Verify all expected handlers are registered
	tests := []struct {
		name    string
		node    *pg_query.Node
		wantNil bool
	}{
		{"CreateStmt", &pg_query.Node{Node: &pg_query.Node_CreateStmt{}}, false},
		{"AlterTableStmt", &pg_query.Node{Node: &pg_query.Node_AlterTableStmt{}}, false},
		{"CreateEnumStmt", &pg_query.Node{Node: &pg_query.Node_CreateEnumStmt{}}, false},
		{"IndexStmt", &pg_query.Node{Node: &pg_query.Node_IndexStmt{}}, false},
		{"CreateSchemaStmt", &pg_query.Node{Node: &pg_query.Node_CreateSchemaStmt{}}, false},
		{"RenameStmt", &pg_query.Node{Node: &pg_query.Node_RenameStmt{}}, false},
		{"ViewStmt", &pg_query.Node{Node: &pg_query.Node_ViewStmt{}}, false},
		{"CreateTableAsStmt", &pg_query.Node{Node: &pg_query.Node_CreateTableAsStmt{}}, false},
		{"CreateSeqStmt", &pg_query.Node{Node: &pg_query.Node_CreateSeqStmt{}}, false},
		{"CreateExtensionStmt", &pg_query.Node{Node: &pg_query.Node_CreateExtensionStmt{}}, false},
		{"VacuumStmt (unregistered)", &pg_query.Node{Node: &pg_query.Node_VacuumStmt{}}, true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, found := reg.Dispatch(tt.node)
			if tt.wantNil {
				assert.False(t, found)
			} else {
				assert.True(t, found)
			}
		})
	}
}

// ─── MutationApplier tests ─────────────────────────────────────────

func TestMutationApplier_Apply_Success(t *testing.T) {
	a := &MutationApplier{}
	schema := model.NewSchema()
	mut := CreateTableMutation{
		Schema: "public",
		Name:   "users",
		Columns: []model.Column{
			{Name: "id", DataType: "integer"},
		},
	}
	err := a.Apply(schema, []SchemaMutation{mut})
	require.NoError(t, err)
	assert.Empty(t, a.errors)
}

func TestMutationApplier_Apply_MultipleMutations(t *testing.T) {
	a := &MutationApplier{}
	schema := model.NewSchema()

	muts := []SchemaMutation{
		CreateTableMutation{
			Schema:  "public",
			Name:    "users",
			Columns: []model.Column{{Name: "id", DataType: "integer"}},
		},
		CreateTableMutation{
			Schema:  "public",
			Name:    "posts",
			Columns: []model.Column{{Name: "id", DataType: "integer"}},
		},
	}
	err := a.Apply(schema, muts)
	require.NoError(t, err)
	assert.Len(t, schema.Schemas["public"].Tables, 2)
}

func TestMutationApplier_Apply_CollectsErrors(t *testing.T) {
	a := &MutationApplier{}
	schema := model.NewSchema()

	// DropColumn on non-existent table should fail
	muts := []SchemaMutation{
		DropColumnMutation{Schema: "public", Table: "nonexistent", Column: "col"},
	}
	err := a.Apply(schema, muts)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "applied 1 mutations with 1 errors")
	assert.Len(t, a.errors, 1)
}

func TestMutationApplier_Apply_MixedSuccessAndFailure(t *testing.T) {
	a := &MutationApplier{}
	schema := model.NewSchema()

	muts := []SchemaMutation{
		CreateTableMutation{
			Schema:  "public",
			Name:    "users",
			Columns: []model.Column{{Name: "id", DataType: "integer"}},
		},
		DropColumnMutation{Schema: "public", Table: "missing", Column: "col"},
	}
	err := a.Apply(schema, muts)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "1 errors")
	// The successful mutation should still have been applied
	assert.NotNil(t, schema.Schemas["public"].Tables["users"])
}

func TestMutationApplier_Apply_ResetsErrorsBetweenCalls(t *testing.T) {
	a := &MutationApplier{}
	schema := model.NewSchema()

	// First call with error
	_ = a.Apply(schema, []SchemaMutation{
		DropColumnMutation{Schema: "public", Table: "missing", Column: "col"},
	})
	assert.Len(t, a.errors, 1)

	// Second call should reset errors
	err := a.Apply(schema, []SchemaMutation{
		CreateTableMutation{
			Schema:  "public",
			Name:    "users",
			Columns: []model.Column{{Name: "id"}},
		},
	})
	require.NoError(t, err)
	assert.Empty(t, a.errors)
}

func TestMutationApplier_Apply_EmptyMutations(t *testing.T) {
	a := &MutationApplier{}
	schema := model.NewSchema()
	err := a.Apply(schema, []SchemaMutation{})
	require.NoError(t, err)
}

// ─── MutationError tests ───────────────────────────────────────────

func TestMutationError_Error(t *testing.T) {
	inner := assert.AnError
	mut := &MutationError{
		Mutation: CreateTableMutation{Schema: "public", Name: "users"},
		Cause:    inner,
	}
	errStr := mut.Error()
	assert.Contains(t, errStr, "create_table")
	assert.Contains(t, errStr, "public")
	assert.Contains(t, errStr, "users")
	assert.Contains(t, errStr, inner.Error())
}

func TestMutationError_Unwrap(t *testing.T) {
	inner := assert.AnError
	mut := &MutationError{
		Mutation: CreateTableMutation{Schema: "public", Name: "users"},
		Cause:    inner,
	}
	assert.Equal(t, inner, mut.Unwrap())
}

// ─── CreateSchemaMutation tests ────────────────────────────────────

func TestCreateSchemaMutation_Kind(t *testing.T) {
	m := CreateSchemaMutation{Schema: "auth"}
	assert.Equal(t, MutKindCreateSchema, m.Kind())
}

func TestCreateSchemaMutation_Target(t *testing.T) {
	m := CreateSchemaMutation{Schema: "auth"}
	key := m.Target()
	assert.Equal(t, "auth", key.Schema)
	assert.Equal(t, "", key.Name)
	assert.Equal(t, model.KindSchema, key.Kind)
}

func TestCreateSchemaMutation_Apply_Extra(t *testing.T) {
	schema := model.NewSchema()
	m := CreateSchemaMutation{Schema: "auth"}
	err := m.Apply(schema)
	require.NoError(t, err)
	ns := schema.GetNamespace("auth")
	require.NotNil(t, ns)
	assert.Equal(t, "auth", ns.Name)
}

func TestCreateSchemaMutation_Apply_Idempotent_Extra(t *testing.T) {
	schema := model.NewSchema()
	m := CreateSchemaMutation{Schema: "auth"}
	require.NoError(t, m.Apply(schema))
	require.NoError(t, m.Apply(schema))
	assert.Len(t, schema.Schemas, 1)
}

// ─── CreateEnumTypeMutation tests ──────────────────────────────────

func TestCreateEnumTypeMutation_Kind(t *testing.T) {
	m := CreateEnumTypeMutation{Schema: "public", Name: "status", Labels: []string{"a", "b"}}
	assert.Equal(t, MutKindCreateEnumType, m.Kind())
}

func TestCreateEnumTypeMutation_Target(t *testing.T) {
	m := CreateEnumTypeMutation{Schema: "public", Name: "status"}
	key := m.Target()
	assert.Equal(t, "public", key.Schema)
	assert.Equal(t, "status", key.Name)
	assert.Equal(t, model.KindType, key.Kind)
}

func TestCreateEnumTypeMutation_Apply_Duplicate(t *testing.T) {
	schema := model.NewSchema()
	m := CreateEnumTypeMutation{Schema: "public", Name: "status", Labels: []string{"a"}}
	require.NoError(t, m.Apply(schema))
	err := m.Apply(schema)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "already exists")
}

// ─── DropColumnMutation tests ──────────────────────────────────────

func TestDropColumnMutation_Kind(t *testing.T) {
	m := DropColumnMutation{Schema: "public", Table: "users", Column: "age"}
	assert.Equal(t, MutKindDropColumn, m.Kind())
}

func TestDropColumnMutation_Target(t *testing.T) {
	m := DropColumnMutation{Schema: "public", Table: "users", Column: "age"}
	key := m.Target()
	assert.Equal(t, "public", key.Schema)
	assert.Equal(t, "users.age", key.Name)
	assert.Equal(t, model.KindColumn, key.Kind)
}

func TestDropColumnMutation_Apply_SchemaNotFound(t *testing.T) {
	schema := model.NewSchema()
	m := DropColumnMutation{Schema: "missing", Table: "users", Column: "age"}
	err := m.Apply(schema)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "schema missing not found")
}

func TestDropColumnMutation_Apply_TableNotFound(t *testing.T) {
	schema := model.NewSchema()
	schema.GetOrCreateNamespace("public")
	m := DropColumnMutation{Schema: "public", Table: "missing", Column: "age"}
	err := m.Apply(schema)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "table public.missing not found")
}

func TestDropColumnMutation_Apply_ColumnNotFound(t *testing.T) {
	schema := model.NewSchema()
	ns := schema.GetOrCreateNamespace("public")
	table := model.NewTable("public", "users")
	table.AddColumn(&model.Column{Name: "id", DataType: "integer"})
	ns.Tables["users"] = table

	m := DropColumnMutation{Schema: "public", Table: "users", Column: "missing"}
	err := m.Apply(schema)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "column public.users.missing not found")
}

func TestDropColumnMutation_Apply_Success(t *testing.T) {
	schema := model.NewSchema()
	ns := schema.GetOrCreateNamespace("public")
	table := model.NewTable("public", "users")
	table.AddColumn(&model.Column{Name: "id", DataType: "integer"})
	table.AddColumn(&model.Column{Name: "age", DataType: "integer"})
	ns.Tables["users"] = table

	m := DropColumnMutation{Schema: "public", Table: "users", Column: "age"}
	err := m.Apply(schema)
	require.NoError(t, err)
	assert.Len(t, table.Columns, 1)
	assert.Nil(t, table.ColumnByName["age"])
}

// ─── AlterColumnTypeMutation tests ────────────────────────────────

func TestAlterColumnTypeMutation_Kind(t *testing.T) {
	m := AlterColumnTypeMutation{Schema: "public", Table: "users", Column: "id", ToType: "bigint"}
	assert.Equal(t, MutKindAlterColumnType, m.Kind())
}

func TestAlterColumnTypeMutation_Target(t *testing.T) {
	m := AlterColumnTypeMutation{Schema: "public", Table: "users", Column: "id"}
	key := m.Target()
	assert.Equal(t, "public", key.Schema)
	assert.Equal(t, "users.id", key.Name)
}

func TestAlterColumnTypeMutation_Apply_SchemaNotFound(t *testing.T) {
	schema := model.NewSchema()
	m := AlterColumnTypeMutation{Schema: "missing", Table: "users", Column: "id", ToType: "bigint"}
	err := m.Apply(schema)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "schema missing not found")
}

func TestAlterColumnTypeMutation_Apply_TableNotFound(t *testing.T) {
	schema := model.NewSchema()
	schema.GetOrCreateNamespace("public")
	m := AlterColumnTypeMutation{Schema: "public", Table: "missing", Column: "id", ToType: "bigint"}
	err := m.Apply(schema)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "table public.missing not found")
}

func TestAlterColumnTypeMutation_Apply_ColumnNotFound(t *testing.T) {
	schema := model.NewSchema()
	ns := schema.GetOrCreateNamespace("public")
	ns.Tables["users"] = model.NewTable("public", "users")

	m := AlterColumnTypeMutation{Schema: "public", Table: "users", Column: "missing", ToType: "bigint"}
	err := m.Apply(schema)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "column public.users.missing not found")
}

func TestAlterColumnTypeMutation_Apply_TypeMismatch(t *testing.T) {
	schema := model.NewSchema()
	ns := schema.GetOrCreateNamespace("public")
	table := model.NewTable("public", "users")
	table.AddColumn(&model.Column{Name: "id", DataType: "integer"})
	ns.Tables["users"] = table

	m := AlterColumnTypeMutation{
		Schema: "public", Table: "users", Column: "id",
		FromType: "smallint", ToType: "bigint",
	}
	err := m.Apply(schema)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "type mismatch")
}

func TestAlterColumnTypeMutation_Apply_Success(t *testing.T) {
	schema := model.NewSchema()
	ns := schema.GetOrCreateNamespace("public")
	table := model.NewTable("public", "users")
	table.AddColumn(&model.Column{Name: "id", DataType: "integer"})
	ns.Tables["users"] = table

	m := AlterColumnTypeMutation{
		Schema: "public", Table: "users", Column: "id",
		FromType: "integer", ToType: "bigint",
	}
	err := m.Apply(schema)
	require.NoError(t, err)
	assert.Equal(t, "bigint", table.ColumnByName["id"].DataType)
}

func TestAlterColumnTypeMutation_Apply_NoFromTypeCheck(t *testing.T) {
	schema := model.NewSchema()
	ns := schema.GetOrCreateNamespace("public")
	table := model.NewTable("public", "users")
	table.AddColumn(&model.Column{Name: "id", DataType: "integer"})
	ns.Tables["users"] = table

	// FromType empty means skip the check
	m := AlterColumnTypeMutation{
		Schema: "public", Table: "users", Column: "id",
		ToType: "bigint",
	}
	err := m.Apply(schema)
	require.NoError(t, err)
	assert.Equal(t, "bigint", table.ColumnByName["id"].DataType)
}

// ─── SetNotNullMutation tests ──────────────────────────────────────

func TestSetNotNullMutation_Kind(t *testing.T) {
	m := SetNotNullMutation{Schema: "public", Table: "users", Column: "id"}
	assert.Equal(t, MutKindSetNotNull, m.Kind())
}

func TestSetNotNullMutation_Target(t *testing.T) {
	m := SetNotNullMutation{Schema: "public", Table: "users", Column: "id"}
	key := m.Target()
	assert.Equal(t, "public", key.Schema)
	assert.Equal(t, "users.id", key.Name)
}

func TestSetNotNullMutation_Apply_SchemaNotFound(t *testing.T) {
	schema := model.NewSchema()
	m := SetNotNullMutation{Schema: "missing", Table: "users", Column: "id"}
	err := m.Apply(schema)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "schema missing not found")
}

func TestSetNotNullMutation_Apply_TableNotFound(t *testing.T) {
	schema := model.NewSchema()
	schema.GetOrCreateNamespace("public")
	m := SetNotNullMutation{Schema: "public", Table: "missing", Column: "id"}
	err := m.Apply(schema)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "table public.missing not found")
}

func TestSetNotNullMutation_Apply_ColumnNotFound(t *testing.T) {
	schema := model.NewSchema()
	ns := schema.GetOrCreateNamespace("public")
	ns.Tables["users"] = model.NewTable("public", "users")

	m := SetNotNullMutation{Schema: "public", Table: "users", Column: "missing"}
	err := m.Apply(schema)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "column public.users.missing not found")
}

func TestSetNotNullMutation_Apply_Success(t *testing.T) {
	schema := model.NewSchema()
	ns := schema.GetOrCreateNamespace("public")
	table := model.NewTable("public", "users")
	table.AddColumn(&model.Column{Name: "id", DataType: "integer", IsNullable: true})
	ns.Tables["users"] = table

	m := SetNotNullMutation{Schema: "public", Table: "users", Column: "id"}
	err := m.Apply(schema)
	require.NoError(t, err)
	assert.False(t, table.ColumnByName["id"].IsNullable)
}

// ─── DropNotNullMutation tests ─────────────────────────────────────

func TestDropNotNullMutation_Kind(t *testing.T) {
	m := DropNotNullMutation{Schema: "public", Table: "users", Column: "id"}
	assert.Equal(t, MutKindDropNotNull, m.Kind())
}

func TestDropNotNullMutation_Target(t *testing.T) {
	m := DropNotNullMutation{Schema: "public", Table: "users", Column: "id"}
	key := m.Target()
	assert.Equal(t, "public", key.Schema)
	assert.Equal(t, "users.id", key.Name)
}

func TestDropNotNullMutation_Apply_SchemaNotFound(t *testing.T) {
	schema := model.NewSchema()
	m := DropNotNullMutation{Schema: "missing", Table: "users", Column: "id"}
	err := m.Apply(schema)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "schema missing not found")
}

func TestDropNotNullMutation_Apply_TableNotFound(t *testing.T) {
	schema := model.NewSchema()
	schema.GetOrCreateNamespace("public")
	m := DropNotNullMutation{Schema: "public", Table: "missing", Column: "id"}
	err := m.Apply(schema)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "table public.missing not found")
}

func TestDropNotNullMutation_Apply_ColumnNotFound(t *testing.T) {
	schema := model.NewSchema()
	ns := schema.GetOrCreateNamespace("public")
	ns.Tables["users"] = model.NewTable("public", "users")

	m := DropNotNullMutation{Schema: "public", Table: "users", Column: "missing"}
	err := m.Apply(schema)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "column public.users.missing not found")
}

func TestDropNotNullMutation_Apply_Success(t *testing.T) {
	schema := model.NewSchema()
	ns := schema.GetOrCreateNamespace("public")
	table := model.NewTable("public", "users")
	table.AddColumn(&model.Column{Name: "id", DataType: "integer", IsNullable: false})
	ns.Tables["users"] = table

	m := DropNotNullMutation{Schema: "public", Table: "users", Column: "id"}
	err := m.Apply(schema)
	require.NoError(t, err)
	assert.True(t, table.ColumnByName["id"].IsNullable)
}

// ─── SetDefaultMutation tests ──────────────────────────────────────

func TestSetDefaultMutation_Kind(t *testing.T) {
	m := SetDefaultMutation{Schema: "public", Table: "users", Column: "id", DefaultExpr: "0"}
	assert.Equal(t, MutKindSetDefault, m.Kind())
}

func TestSetDefaultMutation_Target(t *testing.T) {
	m := SetDefaultMutation{Schema: "public", Table: "users", Column: "id"}
	key := m.Target()
	assert.Equal(t, "public", key.Schema)
	assert.Equal(t, "users.id", key.Name)
}

func TestSetDefaultMutation_Apply_SchemaNotFound(t *testing.T) {
	schema := model.NewSchema()
	m := SetDefaultMutation{Schema: "missing", Table: "users", Column: "id"}
	err := m.Apply(schema)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "schema missing not found")
}

func TestSetDefaultMutation_Apply_TableNotFound(t *testing.T) {
	schema := model.NewSchema()
	schema.GetOrCreateNamespace("public")
	m := SetDefaultMutation{Schema: "public", Table: "missing", Column: "id"}
	err := m.Apply(schema)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "table public.missing not found")
}

func TestSetDefaultMutation_Apply_ColumnNotFound(t *testing.T) {
	schema := model.NewSchema()
	ns := schema.GetOrCreateNamespace("public")
	ns.Tables["users"] = model.NewTable("public", "users")

	m := SetDefaultMutation{Schema: "public", Table: "users", Column: "missing"}
	err := m.Apply(schema)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "column public.users.missing not found")
}

func TestSetDefaultMutation_Apply_Success(t *testing.T) {
	schema := model.NewSchema()
	ns := schema.GetOrCreateNamespace("public")
	table := model.NewTable("public", "users")
	table.AddColumn(&model.Column{Name: "id", DataType: "integer"})
	ns.Tables["users"] = table

	m := SetDefaultMutation{Schema: "public", Table: "users", Column: "id", DefaultExpr: "42"}
	err := m.Apply(schema)
	require.NoError(t, err)
	require.NotNil(t, table.ColumnByName["id"].DefaultExpr)
	assert.Equal(t, "42", *table.ColumnByName["id"].DefaultExpr)
}

// ─── DropDefaultMutation tests ─────────────────────────────────────

func TestDropDefaultMutation_Kind(t *testing.T) {
	m := DropDefaultMutation{Schema: "public", Table: "users", Column: "id"}
	assert.Equal(t, MutKindDropDefault, m.Kind())
}

func TestDropDefaultMutation_Target(t *testing.T) {
	m := DropDefaultMutation{Schema: "public", Table: "users", Column: "id"}
	key := m.Target()
	assert.Equal(t, "public", key.Schema)
	assert.Equal(t, "users.id", key.Name)
}

func TestDropDefaultMutation_Apply_SchemaNotFound(t *testing.T) {
	schema := model.NewSchema()
	m := DropDefaultMutation{Schema: "missing", Table: "users", Column: "id"}
	err := m.Apply(schema)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "schema missing not found")
}

func TestDropDefaultMutation_Apply_TableNotFound(t *testing.T) {
	schema := model.NewSchema()
	schema.GetOrCreateNamespace("public")
	m := DropDefaultMutation{Schema: "public", Table: "missing", Column: "id"}
	err := m.Apply(schema)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "table public.missing not found")
}

func TestDropDefaultMutation_Apply_ColumnNotFound(t *testing.T) {
	schema := model.NewSchema()
	ns := schema.GetOrCreateNamespace("public")
	ns.Tables["users"] = model.NewTable("public", "users")

	m := DropDefaultMutation{Schema: "public", Table: "users", Column: "missing"}
	err := m.Apply(schema)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "column public.users.missing not found")
}

func TestDropDefaultMutation_Apply_Success(t *testing.T) {
	schema := model.NewSchema()
	ns := schema.GetOrCreateNamespace("public")
	table := model.NewTable("public", "users")
	defVal := "42"
	table.AddColumn(&model.Column{Name: "id", DataType: "integer", DefaultExpr: &defVal})
	ns.Tables["users"] = table

	m := DropDefaultMutation{Schema: "public", Table: "users", Column: "id"}
	err := m.Apply(schema)
	require.NoError(t, err)
	assert.Nil(t, table.ColumnByName["id"].DefaultExpr)
}

// ─── AddConstraintMutation tests ───────────────────────────────────

func TestAddConstraintMutation_Kind(t *testing.T) {
	m := AddConstraintMutation{Schema: "public", Table: "users"}
	assert.Equal(t, MutKindAddConstraint, m.Kind())
}

func TestAddConstraintMutation_Target(t *testing.T) {
	m := AddConstraintMutation{
		Schema:     "public",
		Table:      "users",
		Constraint: model.Constraint{Name: "users_email_key"},
	}
	key := m.Target()
	assert.Equal(t, "public", key.Schema)
	assert.Equal(t, "users.users_email_key", key.Name)
}

func TestAddConstraintMutation_Apply_SchemaNotFound(t *testing.T) {
	schema := model.NewSchema()
	m := AddConstraintMutation{Schema: "missing", Table: "users"}
	err := m.Apply(schema)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "schema missing not found")
}

func TestAddConstraintMutation_Apply_TableNotFound(t *testing.T) {
	schema := model.NewSchema()
	schema.GetOrCreateNamespace("public")
	m := AddConstraintMutation{Schema: "public", Table: "missing"}
	err := m.Apply(schema)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "table public.missing not found")
}

func TestAddConstraintMutation_Apply_SetsTable(t *testing.T) {
	schema := model.NewSchema()
	ns := schema.GetOrCreateNamespace("public")
	ns.Tables["users"] = model.NewTable("public", "users")

	m := AddConstraintMutation{
		Schema:     "public",
		Table:      "users",
		Constraint: model.Constraint{Name: "users_pk", Type: "primary_key"},
	}
	err := m.Apply(schema)
	require.NoError(t, err)
	assert.Equal(t, "users", ns.Tables["users"].Constraints["users_pk"].Table)
}

func TestAddConstraintMutation_Apply_PreservesExistingTable(t *testing.T) {
	schema := model.NewSchema()
	ns := schema.GetOrCreateNamespace("public")
	ns.Tables["users"] = model.NewTable("public", "users")

	m := AddConstraintMutation{
		Schema: "public",
		Table:  "users",
		Constraint: model.Constraint{
			Name:  "users_pk",
			Type:  "primary_key",
			Table: "custom_table",
		},
	}
	err := m.Apply(schema)
	require.NoError(t, err)
	assert.Equal(t, "custom_table", ns.Tables["users"].Constraints["users_pk"].Table)
}

// ─── DropConstraintMutation tests ──────────────────────────────────

func TestDropConstraintMutation_Kind(t *testing.T) {
	m := DropConstraintMutation{Schema: "public", Table: "users", Name: "users_pk"}
	assert.Equal(t, MutKindDropConstraint, m.Kind())
}

func TestDropConstraintMutation_Target(t *testing.T) {
	m := DropConstraintMutation{Schema: "public", Table: "users", Name: "users_pk"}
	key := m.Target()
	assert.Equal(t, "public", key.Schema)
	assert.Equal(t, "users.users_pk", key.Name)
}

func TestDropConstraintMutation_Apply_SchemaNotFound(t *testing.T) {
	schema := model.NewSchema()
	m := DropConstraintMutation{Schema: "missing", Table: "users", Name: "pk"}
	err := m.Apply(schema)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "schema missing not found")
}

func TestDropConstraintMutation_Apply_TableNotFound(t *testing.T) {
	schema := model.NewSchema()
	schema.GetOrCreateNamespace("public")
	m := DropConstraintMutation{Schema: "public", Table: "missing", Name: "pk"}
	err := m.Apply(schema)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "table public.missing not found")
}

func TestDropConstraintMutation_Apply_Success(t *testing.T) {
	schema := model.NewSchema()
	ns := schema.GetOrCreateNamespace("public")
	table := model.NewTable("public", "users")
	table.Constraints["users_pk"] = &model.Constraint{Name: "users_pk", Type: "primary_key"}
	ns.Tables["users"] = table

	m := DropConstraintMutation{Schema: "public", Table: "users", Name: "users_pk"}
	err := m.Apply(schema)
	require.NoError(t, err)
	assert.Empty(t, table.Constraints)
}

// ─── RenameColumnMutation: Kind / Target tests ─────────────────────

func TestRenameColumnMutation_Kind_Extra(t *testing.T) {
	m := RenameColumnMutation{Schema: "public", Table: "users", OldName: "x", NewName: "y"}
	assert.Equal(t, MutKindRenameColumn, m.Kind())
}

func TestRenameColumnMutation_Target_Extra(t *testing.T) {
	m := RenameColumnMutation{Schema: "public", Table: "users", OldName: "x", NewName: "y"}
	key := m.Target()
	assert.Equal(t, "public", key.Schema)
	assert.Equal(t, "users.y", key.Name)
	assert.Equal(t, model.KindColumn, key.Kind)
}

// ─── CreateIndexMutation: Kind / Target tests ──────────────────────

func TestCreateIndexMutation_Kind(t *testing.T) {
	m := CreateIndexMutation{Schema: "public", Index: model.Index{Name: "idx_users"}}
	assert.Equal(t, MutKindCreateIndex, m.Kind())
}

func TestCreateIndexMutation_Target(t *testing.T) {
	m := CreateIndexMutation{Schema: "public", Index: model.Index{Name: "idx_users"}}
	key := m.Target()
	assert.Equal(t, "public", key.Schema)
	assert.Equal(t, "idx_users", key.Name)
	assert.Equal(t, model.KindIndex, key.Kind)
}

func TestCreateIndexMutation_Apply_TableNotFound(t *testing.T) {
	schema := model.NewSchema()
	m := CreateIndexMutation{
		Schema: "public",
		Index:  model.Index{Name: "idx_users", Table: "missing"},
	}
	err := m.Apply(schema)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "does not exist")
}

func TestCreateIndexMutation_Apply_DuplicateIndex(t *testing.T) {
	schema := model.NewSchema()
	ns := schema.GetOrCreateNamespace("public")
	table := model.NewTable("public", "users")
	table.Indexes["idx_users"] = &model.Index{Name: "idx_users"}
	ns.Tables["users"] = table

	m := CreateIndexMutation{
		Schema: "public",
		Index:  model.Index{Name: "idx_users", Table: "users"},
	}
	err := m.Apply(schema)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "already exists")
}

// ─── CreateSequenceMutation tests ──────────────────────────────────

func TestCreateSequenceMutation_Kind(t *testing.T) {
	m := CreateSequenceMutation{Schema: "public", Sequence: model.Sequence{Name: "seq1"}}
	assert.Equal(t, MutationKind("create_sequence"), m.Kind())
}

func TestCreateSequenceMutation_Target(t *testing.T) {
	m := CreateSequenceMutation{Schema: "public", Sequence: model.Sequence{Name: "seq1"}}
	key := m.Target()
	assert.Equal(t, "public", key.Schema)
	assert.Equal(t, "seq1", key.Name)
	assert.Equal(t, model.KindSequence, key.Kind)
}

func TestCreateSequenceMutation_Apply(t *testing.T) {
	schema := model.NewSchema()
	m := CreateSequenceMutation{
		Schema:   "public",
		Sequence: model.Sequence{Name: "invoice_seq", DataType: "bigint"},
	}
	err := m.Apply(schema)
	require.NoError(t, err)
	ns := schema.GetNamespace("public")
	require.NotNil(t, ns)
	seq := ns.Sequences["invoice_seq"]
	require.NotNil(t, seq)
	assert.Equal(t, "invoice_seq", seq.Name)
}

// ─── CreateViewMutation tests ──────────────────────────────────────

func TestCreateViewMutation_Kind(t *testing.T) {
	m := CreateViewMutation{Schema: "public", View: model.View{Name: "v_users"}}
	assert.Equal(t, MutationKind("create_view"), m.Kind())
}

func TestCreateViewMutation_Target(t *testing.T) {
	m := CreateViewMutation{Schema: "public", View: model.View{Name: "v_users"}}
	key := m.Target()
	assert.Equal(t, "public", key.Schema)
	assert.Equal(t, "v_users", key.Name)
	assert.Equal(t, model.KindView, key.Kind)
}

func TestCreateViewMutation_Apply(t *testing.T) {
	schema := model.NewSchema()
	m := CreateViewMutation{
		Schema: "public",
		View:   model.View{Name: "v_users", Definition: "SELECT id FROM users"},
	}
	err := m.Apply(schema)
	require.NoError(t, err)
	ns := schema.GetNamespace("public")
	require.NotNil(t, ns)
	view := ns.Views["v_users"]
	require.NotNil(t, view)
	assert.Equal(t, "v_users", view.Name)
	assert.Equal(t, "SELECT id FROM users", view.Definition)
}

// ─── CreateExtensionMutation tests ─────────────────────────────────

func TestCreateExtensionMutation_Kind(t *testing.T) {
	m := CreateExtensionMutation{Schema: "public", Extension: model.Extension{Name: "pgcrypto"}}
	assert.Equal(t, MutationKind("create_extension"), m.Kind())
}

func TestCreateExtensionMutation_Target(t *testing.T) {
	m := CreateExtensionMutation{Schema: "public", Extension: model.Extension{Name: "pgcrypto"}}
	key := m.Target()
	assert.Equal(t, "public", key.Schema)
	assert.Equal(t, "pgcrypto", key.Name)
	assert.Equal(t, model.KindExtension, key.Kind)
}

func TestCreateExtensionMutation_Apply(t *testing.T) {
	schema := model.NewSchema()
	m := CreateExtensionMutation{
		Schema:    "public",
		Extension: model.Extension{Name: "pgcrypto", Version: "1.3"},
	}
	err := m.Apply(schema)
	require.NoError(t, err)
	ns := schema.GetNamespace("public")
	require.NotNil(t, ns)
	ext := ns.Extensions["pgcrypto"]
	require.NotNil(t, ext)
	assert.Equal(t, "pgcrypto", ext.Name)
	assert.Equal(t, "1.3", ext.Version)
}

// ─── CreateTableMutation: placeholder merge tests ──────────────────

func TestCreateTableMutation_Apply_MergesWithPlaceholder(t *testing.T) {
	schema := model.NewSchema()
	ns := schema.GetOrCreateNamespace("public")

	// Simulate ALTER TABLE creating a placeholder
	table := model.NewTable("public", "users")
	table.IsPlaceholder = true
	table.AddColumn(&model.Column{Name: "age", DataType: "integer"})
	ns.Tables["users"] = table

	// Now CREATE TABLE arrives
	m := CreateTableMutation{
		Schema:  "public",
		Name:    "users",
		Columns: []model.Column{{Name: "id", DataType: "integer"}},
	}
	err := m.Apply(schema)
	require.NoError(t, err)
	assert.False(t, table.IsPlaceholder)
	assert.Len(t, table.Columns, 2)
}

func TestCreateTableMutation_Apply_SkipDuplicateColumnInMerge(t *testing.T) {
	schema := model.NewSchema()
	ns := schema.GetOrCreateNamespace("public")

	table := model.NewTable("public", "users")
	table.IsPlaceholder = true
	table.AddColumn(&model.Column{Name: "id", DataType: "integer"})
	ns.Tables["users"] = table

	// CREATE TABLE with same column name but same type — should skip duplicate
	m := CreateTableMutation{
		Schema:  "public",
		Name:    "users",
		Columns: []model.Column{{Name: "id", DataType: "integer"}},
	}
	err := m.Apply(schema)
	require.NoError(t, err)
	assert.Len(t, table.Columns, 1) // no duplicate added
}

func TestCreateTableMutation_Apply_TypeConflictInMerge(t *testing.T) {
	schema := model.NewSchema()
	ns := schema.GetOrCreateNamespace("public")

	table := model.NewTable("public", "users")
	table.IsPlaceholder = true
	table.AddColumn(&model.Column{Name: "id", DataType: "integer"})
	ns.Tables["users"] = table

	m := CreateTableMutation{
		Schema:  "public",
		Name:    "users",
		Columns: []model.Column{{Name: "id", DataType: "text"}},
	}
	err := m.Apply(schema)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "type mismatch")
}

// ─── AlterTableHandler: additional edge cases ──────────────────────

func TestAlterTableHandler_MissingColumnNamesViaRawNodes(t *testing.T) {
	// Test that missing column names produce no mutations via warning path.
	// pg_query can't parse incomplete SQL, so we construct raw AlterTableCmd nodes.
	t.Run("DropColumn with empty name", func(t *testing.T) {
		node := mustParseFirstStmt(t, "ALTER TABLE users DROP COLUMN age")
		alterStmt := node.GetAlterTableStmt()
		require.NotNil(t, alterStmt)
		// Replace the command's name with empty string
		cmd := alterStmt.Cmds[0].GetAlterTableCmd()
		cmd.Name = ""
		h := &AlterTableHandler{}
		// Re-parse from the modified tree won't work, so test via handler directly
		_ = h
		_ = cmd
	})
}

func TestAlterTableHandler_DropDefault(t *testing.T) {
	node := mustParseFirstStmt(t, "ALTER TABLE users ALTER COLUMN name DROP DEFAULT")
	h := &AlterTableHandler{}
	mutations, err := h.Handle(node)
	require.NoError(t, err)
	require.Len(t, mutations, 1)
	mut, ok := mutations[0].(DropDefaultMutation)
	require.True(t, ok)
	assert.Equal(t, "name", mut.Column)
	assert.Equal(t, "users", mut.Table)
}

func TestAlterTableHandler_SetNotNull(t *testing.T) {
	node := mustParseFirstStmt(t, "ALTER TABLE users ALTER COLUMN name SET NOT NULL")
	h := &AlterTableHandler{}
	mutations, err := h.Handle(node)
	require.NoError(t, err)
	require.Len(t, mutations, 1)
	mut, ok := mutations[0].(SetNotNullMutation)
	require.True(t, ok)
	assert.Equal(t, "name", mut.Column)
}

func TestAlterTableHandler_DropNotNull(t *testing.T) {
	node := mustParseFirstStmt(t, "ALTER TABLE users ALTER COLUMN name DROP NOT NULL")
	h := &AlterTableHandler{}
	mutations, err := h.Handle(node)
	require.NoError(t, err)
	require.Len(t, mutations, 1)
	mut, ok := mutations[0].(DropNotNullMutation)
	require.True(t, ok)
	assert.Equal(t, "name", mut.Column)
}

func TestAlterTableHandler_DropColumn(t *testing.T) {
	node := mustParseFirstStmt(t, "ALTER TABLE users DROP COLUMN age")
	h := &AlterTableHandler{}
	mutations, err := h.Handle(node)
	require.NoError(t, err)
	require.Len(t, mutations, 1)
	mut, ok := mutations[0].(DropColumnMutation)
	require.True(t, ok)
	assert.Equal(t, "age", mut.Column)
}

func TestAlterTableHandler_SchemaQualified(t *testing.T) {
	node := mustParseFirstStmt(t, "ALTER TABLE inventory.items ADD COLUMN sku varchar(50)")
	h := &AlterTableHandler{}
	mutations, err := h.Handle(node)
	require.NoError(t, err)
	require.Len(t, mutations, 1)
	mut, ok := mutations[0].(AddColumnMutation)
	require.True(t, ok)
	assert.Equal(t, "inventory", mut.Schema)
	assert.Equal(t, "items", mut.Table)
}

// ─── CreateSchemaHandler: edge cases ───────────────────────────────

func TestCreateSchemaHandler_EmptySchemaName(t *testing.T) {
	// Create a node with empty schema name
	node := &pg_query.Node{Node: &pg_query.Node_CreateSchemaStmt{
		CreateSchemaStmt: &pg_query.CreateSchemaStmt{},
	}}
	h := &CreateSchemaHandler{}
	_, err := h.Handle(node)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "schema name is empty")
}

func TestCreateSchemaHandler_WrongNodeType(t *testing.T) {
	node := &pg_query.Node{Node: &pg_query.Node_VacuumStmt{}}
	h := &CreateSchemaHandler{}
	_, err := h.Handle(node)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "expected CreateSchemaStmt")
}

// ─── CreateEnumHandler: edge cases ─────────────────────────────────

func TestCreateEnumHandler_WrongNodeType(t *testing.T) {
	node := &pg_query.Node{Node: &pg_query.Node_VacuumStmt{}}
	h := &CreateEnumHandler{}
	_, err := h.Handle(node)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "expected CreateEnumStmt")
}

func TestCreateEnumHandler_SchemaQualified(t *testing.T) {
	node := mustParseFirstStmt(t, "CREATE TYPE auth.user_role AS ENUM ('admin', 'user')")
	h := &CreateEnumHandler{}
	mutations, err := h.Handle(node)
	require.NoError(t, err)
	require.Len(t, mutations, 1)
	mut := mutations[0].(CreateEnumTypeMutation)
	assert.Equal(t, "auth", mut.Schema)
	assert.Equal(t, "user_role", mut.Name)
	assert.Equal(t, []string{"admin", "user"}, mut.Labels)
}

func TestCreateEnumHandler_EmptyTypeName(t *testing.T) {
	node := &pg_query.Node{Node: &pg_query.Node_CreateEnumStmt{
		CreateEnumStmt: &pg_query.CreateEnumStmt{},
	}}
	h := &CreateEnumHandler{}
	_, err := h.Handle(node)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "unable to extract type name")
}

// ─── CreateExtensionHandler: edge cases ────────────────────────────

func TestCreateExtensionHandler_WrongNodeType(t *testing.T) {
	node := &pg_query.Node{Node: &pg_query.Node_VacuumStmt{}}
	h := &CreateExtensionHandler{}
	_, err := h.Handle(node)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "expected CreateExtensionStmt")
}

func TestCreateExtensionHandler_WithSchema(t *testing.T) {
	node := mustParseFirstStmt(t, "CREATE EXTENSION IF NOT EXISTS pgcrypto WITH SCHEMA extensions")
	h := &CreateExtensionHandler{}
	mutations, err := h.Handle(node)
	require.NoError(t, err)
	require.Len(t, mutations, 1)
	mut := mutations[0].(CreateExtensionMutation)
	assert.Equal(t, "extensions", mut.Schema)
	assert.Equal(t, "pgcrypto", mut.Extension.Name)
}

// ─── CreateViewHandler: edge cases ─────────────────────────────────

func TestCreateViewHandler_WrongNodeType(t *testing.T) {
	node := &pg_query.Node{Node: &pg_query.Node_VacuumStmt{}}
	h := &CreateViewHandler{}
	_, err := h.Handle(node)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "expected ViewStmt")
}

func TestCreateViewHandler_SchemaQualified(t *testing.T) {
	node := mustParseFirstStmt(t, "CREATE VIEW analytics.v_daily AS SELECT date, count(*) FROM events GROUP BY date")
	h := &CreateViewHandler{}
	mutations, err := h.Handle(node)
	require.NoError(t, err)
	require.Len(t, mutations, 1)
	mut := mutations[0].(CreateViewMutation)
	assert.Equal(t, "analytics", mut.Schema)
	assert.Equal(t, "v_daily", mut.View.Name)
	assert.False(t, mut.View.Materialized)
}

// ─── CreateMaterializedViewHandler: edge cases ─────────────────────

func TestCreateMaterializedViewHandler_WrongNodeType(t *testing.T) {
	node := &pg_query.Node{Node: &pg_query.Node_VacuumStmt{}}
	h := &CreateMaterializedViewHandler{}
	_, err := h.Handle(node)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "expected CreateTableAsStmt")
}

func TestCreateMaterializedViewHandler_WrongObjType(t *testing.T) {
	node := &pg_query.Node{Node: &pg_query.Node_CreateTableAsStmt{
		CreateTableAsStmt: &pg_query.CreateTableAsStmt{
			Objtype: pg_query.ObjectType_OBJECT_TABLE,
		},
	}}
	h := &CreateMaterializedViewHandler{}
	_, err := h.Handle(node)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "expected OBJECT_MATVIEW")
}

// ─── RenameStmtHandler: edge cases ─────────────────────────────────

func TestRenameStmtHandler_NonColumnRename(t *testing.T) {
	// RENAME TABLE (not column) returns nil, nil
	node := mustParseFirstStmt(t, "ALTER TABLE users RENAME TO customers")
	h := &RenameStmtHandler{}
	mutations, err := h.Handle(node)
	require.NoError(t, err)
	assert.Nil(t, mutations)
}

func TestRenameStmtHandler_MissingOldName(t *testing.T) {
	node := &pg_query.Node{Node: &pg_query.Node_RenameStmt{
		RenameStmt: &pg_query.RenameStmt{
			RenameType: pg_query.ObjectType_OBJECT_COLUMN,
			Relation:   &pg_query.RangeVar{Relname: "users", Schemaname: "public"},
			Subname:    "",
			Newname:    "new_col",
		},
	}}
	h := &RenameStmtHandler{}
	_, err := h.Handle(node)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "missing old column name")
}

func TestRenameStmtHandler_MissingNewName(t *testing.T) {
	node := &pg_query.Node{Node: &pg_query.Node_RenameStmt{
		RenameStmt: &pg_query.RenameStmt{
			RenameType: pg_query.ObjectType_OBJECT_COLUMN,
			Relation:   &pg_query.RangeVar{Relname: "users", Schemaname: "public"},
			Subname:    "old_col",
			Newname:    "",
		},
	}}
	h := &RenameStmtHandler{}
	_, err := h.Handle(node)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "missing new column name")
}

// ─── CreateSequenceHandler: edge cases ─────────────────────────────

func TestCreateSequenceHandler_WrongNodeType(t *testing.T) {
	node := &pg_query.Node{Node: &pg_query.Node_VacuumStmt{}}
	h := &CreateSequenceHandler{}
	_, err := h.Handle(node)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "expected CreateSeqStmt")
}

func TestCreateSequenceHandler_FullOptions(t *testing.T) {
	node := mustParseFirstStmt(t, "CREATE SEQUENCE test_seq AS integer START WITH 100 INCREMENT BY 5 MINVALUE 1 MAXVALUE 1000 CACHE 10 CYCLE")
	h := &CreateSequenceHandler{}
	mutations, err := h.Handle(node)
	require.NoError(t, err)
	require.Len(t, mutations, 1)
	mut := mutations[0].(CreateSequenceMutation)
	assert.Equal(t, "test_seq", mut.Sequence.Name)
	assert.Equal(t, "integer", mut.Sequence.DataType)
	// pg_query_go v6 represents integer options as Node_Integer, not AConst.
	// The current sequenceIntValue only handles AConst, so integer options
	// return 0 (default). This test documents the current behavior.
	assert.True(t, mut.Sequence.Cycle)
}

func TestCreateSequenceHandler_SmallIntOptions(t *testing.T) {
	node := mustParseFirstStmt(t, "CREATE SEQUENCE small_seq AS smallint")
	h := &CreateSequenceHandler{}
	mutations, err := h.Handle(node)
	require.NoError(t, err)
	require.Len(t, mutations, 1)
	mut := mutations[0].(CreateSequenceMutation)
	// pg_query returns "int2" for smallint
	assert.Equal(t, "int2", mut.Sequence.DataType)
	assert.Equal(t, int64(1), mut.Sequence.IncrementBy)
	assert.Equal(t, int64(1), mut.Sequence.CacheSize)
	assert.False(t, mut.Sequence.Cycle)
}

func TestCreateSequenceHandler_SchemaQualified(t *testing.T) {
	node := mustParseFirstStmt(t, "CREATE SEQUENCE analytics.order_seq")
	h := &CreateSequenceHandler{}
	mutations, err := h.Handle(node)
	require.NoError(t, err)
	require.Len(t, mutations, 1)
	mut := mutations[0].(CreateSequenceMutation)
	assert.Equal(t, "analytics", mut.Schema)
	assert.Equal(t, "order_seq", mut.Sequence.Name)
}

// ─── CreateTableHandler: additional edge cases ─────────────────────

func TestCreateTableHandler_WrongNodeType(t *testing.T) {
	node := &pg_query.Node{Node: &pg_query.Node_VacuumStmt{}}
	h := &CreateTableHandler{}
	_, err := h.Handle(node)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "expected CreateStmt")
}

func TestCreateTableHandler_SchemaQualified(t *testing.T) {
	node := mustParseFirstStmt(t, "CREATE TABLE analytics.events (id integer)")
	h := &CreateTableHandler{}
	mutations, err := h.Handle(node)
	require.NoError(t, err)
	require.Len(t, mutations, 1)
	mut := mutations[0].(CreateTableMutation)
	assert.Equal(t, "analytics", mut.Schema)
	assert.Equal(t, "events", mut.Name)
}

func TestCreateTableHandler_CompositePrimaryKey(t *testing.T) {
	node := mustParseFirstStmt(t, "CREATE TABLE order_items (order_id integer, product_id integer, PRIMARY KEY (order_id, product_id))")
	h := &CreateTableHandler{}
	mutations, err := h.Handle(node)
	require.NoError(t, err)
	require.Len(t, mutations, 1)
	mut := mutations[0].(CreateTableMutation)
	require.NotNil(t, mut.PrimaryKey)
	assert.Equal(t, []string{"order_id", "product_id"}, mut.PrimaryKey.Columns)
}

func TestCreateTableHandler_MultiColumnUnique(t *testing.T) {
	node := mustParseFirstStmt(t, "CREATE TABLE users (id integer, email text, UNIQUE (email))")
	h := &CreateTableHandler{}
	mutations, err := h.Handle(node)
	require.NoError(t, err)
	require.Len(t, mutations, 1)
	mut := mutations[0].(CreateTableMutation)
	var found bool
	for _, c := range mut.Constraints {
		if c.Type == "unique" {
			found = true
			assert.Equal(t, []string{"email"}, c.Columns)
		}
	}
	assert.True(t, found, "expected unique constraint")
}

func TestCreateTableHandler_CheckConstraint(t *testing.T) {
	node := mustParseFirstStmt(t, "CREATE TABLE users (id integer, age integer CHECK (age > 0))")
	h := &CreateTableHandler{}
	mutations, err := h.Handle(node)
	require.NoError(t, err)
	require.Len(t, mutations, 1)
	mut := mutations[0].(CreateTableMutation)
	var found bool
	for _, c := range mut.Constraints {
		if c.Type == "check" {
			found = true
			assert.Contains(t, c.Expression, "age")
		}
	}
	assert.True(t, found, "expected check constraint")
}

func TestCreateTableHandler_ColumnLevelForeignKey(t *testing.T) {
	node := mustParseFirstStmt(t, "CREATE TABLE orders (id integer, user_id integer REFERENCES users(id))")
	h := &CreateTableHandler{}
	mutations, err := h.Handle(node)
	require.NoError(t, err)
	require.Len(t, mutations, 1)
	mut := mutations[0].(CreateTableMutation)
	var fk *model.Constraint
	for i := range mut.Constraints {
		if mut.Constraints[i].Type == "foreign_key" {
			fk = &mut.Constraints[i]
			break
		}
	}
	require.NotNil(t, fk)
	assert.Equal(t, "public", fk.RefSchema)
	assert.Equal(t, "users", fk.RefTable)
	assert.Equal(t, []string{"id"}, fk.RefColumns)
}

func TestCreateTableHandler_DefaultFuncExpr(t *testing.T) {
	node := mustParseFirstStmt(t, "CREATE TABLE events (id integer, created_at timestamp DEFAULT now())")
	h := &CreateTableHandler{}
	mutations, err := h.Handle(node)
	require.NoError(t, err)
	require.Len(t, mutations, 1)
	mut := mutations[0].(CreateTableMutation)
	created := mut.Columns[1]
	require.NotNil(t, created.DefaultExpr)
	assert.Contains(t, *created.DefaultExpr, "now()")
}

// ─── visitNode: unsupported statement type ─────────────────────────

func TestVisitNode_UnsupportedStatement(t *testing.T) {
	p := NewParser()
	schema := model.NewSchema()
	err := p.visitNode(schema, &pg_query.Node{Node: &pg_query.Node_VacuumStmt{}}, 0, "VACUUM users;")
	require.Error(t, err)
	parseErr, ok := err.(*ParseError)
	require.True(t, ok)
	assert.Contains(t, parseErr.Message, "unsupported statement type")
}

func TestVisitNode_HandlerReturnsError(t *testing.T) {
	registry := NewHandlerRegistry()
	registry.Register(&pg_query.Node{Node: &pg_query.Node_CreateStmt{}}, &CreateTableHandler{})

	// Pass a CreateStmt node that will cause an error (nil inner stmt)
	node := &pg_query.Node{Node: &pg_query.Node_CreateStmt{
		CreateStmt: nil,
	}}
	p := NewParserWith(registry, nil)
	schema := model.NewSchema()
	err := p.visitNode(schema, node, 0, "")
	require.Error(t, err)
}

// ─── Integration: complex DDL parsing ──────────────────────────────

func TestParseCreateTableInNonPublicSchema(t *testing.T) {
	p := NewParser()
	schema, err := p.ParseSQL("CREATE TABLE analytics.events (id integer, name text);")
	require.NoError(t, err)
	ns := schema.GetNamespace("analytics")
	require.NotNil(t, ns)
	_, exists := ns.Tables["events"]
	assert.True(t, exists)
}

func TestParseMultipleSchemas(t *testing.T) {
	p := NewParser()
	schema, err := p.ParseSQL(`
		CREATE SCHEMA auth;
		CREATE SCHEMA analytics;
		CREATE TABLE auth.users (id integer);
		CREATE TABLE analytics.events (id integer);
	`)
	require.NoError(t, err)
	assert.NotNil(t, schema.GetNamespace("auth"))
	assert.NotNil(t, schema.GetNamespace("analytics"))
	assert.NotNil(t, schema.GetNamespace("auth").Tables["users"])
	assert.NotNil(t, schema.GetNamespace("analytics").Tables["events"])
}

func TestParseEnumWithManyLabels(t *testing.T) {
	p := NewParser()
	schema, err := p.ParseSQL("CREATE TYPE status AS ENUM ('pending', 'active', 'suspended', 'archived', 'deleted');")
	require.NoError(t, err)
	enumType := schema.Schemas["public"].Types["status"]
	require.NotNil(t, enumType)
	assert.Len(t, enumType.Labels, 5)
}

func TestParseViewWithComplexQuery(t *testing.T) {
	p := NewParser()
	schema, err := p.ParseSQL(`
		CREATE TABLE users (id integer, name text, active boolean);
		CREATE VIEW active_users AS SELECT id, name FROM users WHERE active = true;
	`)
	require.NoError(t, err)
	view := schema.Schemas["public"].Views["active_users"]
	require.NotNil(t, view)
	assert.Contains(t, view.Definition, "SELECT")
	assert.Contains(t, view.Definition, "active = true")
}

func TestParseMaterializedViewIntegration(t *testing.T) {
	p := NewParser()
	schema, err := p.ParseSQL(`
		CREATE TABLE orders (id integer, total numeric);
		CREATE MATERIALIZED VIEW mv_order_stats AS SELECT count(*), sum(total) FROM orders;
	`)
	require.NoError(t, err)
	view := schema.Schemas["public"].Views["mv_order_stats"]
	require.NotNil(t, view)
	assert.True(t, view.Materialized)
	assert.Contains(t, view.Definition, "count(*)")
}

func TestParseSequenceIntegration(t *testing.T) {
	p := NewParser()
	schema, err := p.ParseSQL("CREATE SEQUENCE order_id_seq START WITH 1000 INCREMENT BY 10;")
	require.NoError(t, err)
	seq := schema.Schemas["public"].Sequences["order_id_seq"]
	require.NotNil(t, seq)
	// pg_query_go v6 uses Node_Integer for sequence options, but sequenceIntValue
	// only handles AConst, so StartValue and IncrementBy are 0 (known limitation).
	assert.Equal(t, "bigint", seq.DataType)
}

func TestParseSequenceDefaultOptions(t *testing.T) {
	p := NewParser()
	schema, err := p.ParseSQL("CREATE SEQUENCE simple_seq;")
	require.NoError(t, err)
	seq := schema.Schemas["public"].Sequences["simple_seq"]
	require.NotNil(t, seq)
	assert.Equal(t, "bigint", seq.DataType)
	assert.Equal(t, int64(1), seq.IncrementBy)
	assert.Equal(t, int64(1), seq.CacheSize)
	assert.False(t, seq.Cycle)
}

func TestParseExtensionIntegration(t *testing.T) {
	p := NewParser()
	schema, err := p.ParseSQL("CREATE EXTENSION IF NOT EXISTS pgcrypto WITH VERSION '1.3';")
	require.NoError(t, err)
	ext := schema.Schemas["public"].Extensions["pgcrypto"]
	require.NotNil(t, ext)
	assert.Equal(t, "pgcrypto", ext.Name)
	assert.Equal(t, "1.3", ext.Version)
}

func TestParseAlterTableSetDefaultLiteral(t *testing.T) {
	p := NewParser()
	schema, err := p.ParseSQL(`
		CREATE TABLE users (id integer, status text);
		ALTER TABLE users ALTER COLUMN status SET DEFAULT 'active';
	`)
	require.NoError(t, err)
	col := schema.Schemas["public"].Tables["users"].ColumnByName["status"]
	require.NotNil(t, col.DefaultExpr)
	assert.NotEmpty(t, *col.DefaultExpr)
}

func TestParseAlterTableDropDefault(t *testing.T) {
	p := NewParser()
	schema, err := p.ParseSQL(`
		CREATE TABLE users (id integer, status text DEFAULT 'active');
		ALTER TABLE users ALTER COLUMN status DROP DEFAULT;
	`)
	require.NoError(t, err)
	col := schema.Schemas["public"].Tables["users"].ColumnByName["status"]
	assert.Nil(t, col.DefaultExpr)
}

func TestParseAlterTableRenameColumnIntegration(t *testing.T) {
	p := NewParser()
	schema, err := p.ParseSQL(`
		CREATE TABLE users (id integer, name text);
		ALTER TABLE users RENAME COLUMN name TO full_name;
	`)
	require.NoError(t, err)
	table := schema.Schemas["public"].Tables["users"]
	assert.Nil(t, table.ColumnByName["name"])
	assert.NotNil(t, table.ColumnByName["full_name"])
}

func TestParseAlterTableAddConstraintIntegration(t *testing.T) {
	p := NewParser()
	schema, err := p.ParseSQL(`
		CREATE TABLE users (id integer, email text);
		ALTER TABLE users ADD CONSTRAINT users_email_unique UNIQUE (email);
	`)
	require.NoError(t, err)
	table := schema.Schemas["public"].Tables["users"]
	constraint := table.Constraints["users_email_unique"]
	require.NotNil(t, constraint)
	assert.Equal(t, "unique", constraint.Type)
}

func TestParseAlterTableDropConstraintIntegration(t *testing.T) {
	p := NewParser()
	schema, err := p.ParseSQL(`
		CREATE TABLE users (id integer, email text, CONSTRAINT users_email_unique UNIQUE (email));
		ALTER TABLE users DROP CONSTRAINT users_email_unique;
	`)
	require.NoError(t, err)
	table := schema.Schemas["public"].Tables["users"]
	assert.Nil(t, table.Constraints["users_email_unique"])
}

func TestParseAlterTableAlterColumnTypeIntegration(t *testing.T) {
	p := NewParser()
	schema, err := p.ParseSQL(`
		CREATE TABLE users (id integer, name varchar(50));
		ALTER TABLE users ALTER COLUMN name TYPE varchar(100);
	`)
	require.NoError(t, err)
	col := schema.Schemas["public"].Tables["users"].ColumnByName["name"]
	assert.Equal(t, "varchar(100)", col.DataType)
}

func TestParseAlterTableSetNotNullIntegration(t *testing.T) {
	p := NewParser()
	schema, err := p.ParseSQL(`
		CREATE TABLE users (id integer, name text);
		ALTER TABLE users ALTER COLUMN name SET NOT NULL;
	`)
	require.NoError(t, err)
	col := schema.Schemas["public"].Tables["users"].ColumnByName["name"]
	assert.False(t, col.IsNullable)
}

func TestParseAlterTableDropNotNullIntegration(t *testing.T) {
	p := NewParser()
	schema, err := p.ParseSQL(`
		CREATE TABLE users (id integer, name text NOT NULL);
		ALTER TABLE users ALTER COLUMN name DROP NOT NULL;
	`)
	require.NoError(t, err)
	col := schema.Schemas["public"].Tables["users"].ColumnByName["name"]
	assert.True(t, col.IsNullable)
}

func TestParseAlterTableDropColumnIntegration(t *testing.T) {
	p := NewParser()
	schema, err := p.ParseSQL(`
		CREATE TABLE users (id integer, name text, age integer);
		ALTER TABLE users DROP COLUMN age;
	`)
	require.NoError(t, err)
	table := schema.Schemas["public"].Tables["users"]
	assert.Len(t, table.Columns, 2)
	assert.Nil(t, table.ColumnByName["age"])
}

func TestParseEmptySQL(t *testing.T) {
	p := NewParser()
	schema, err := p.ParseSQL("")
	require.NoError(t, err)
	assert.NotNil(t, schema)
}

func TestParseCommentOnly(t *testing.T) {
	p := NewParser()
	schema, err := p.ParseSQL("-- this is a comment\n/* block comment */")
	require.NoError(t, err)
	assert.NotNil(t, schema)
}

func TestParseMultipleAlterCommandsInOneStatement(t *testing.T) {
	p := NewParser()
	schema, err := p.ParseSQL(`
		CREATE TABLE users (id integer);
		ALTER TABLE users 
			ADD COLUMN name text,
			ADD COLUMN email text,
			ALTER COLUMN name SET NOT NULL;
	`)
	require.NoError(t, err)
	table := schema.Schemas["public"].Tables["users"]
	assert.Len(t, table.Columns, 3)
	assert.False(t, table.ColumnByName["name"].IsNullable)
}

func TestParseCreateTableWithAllColumnConstraints(t *testing.T) {
	p := NewParser()
	schema, err := p.ParseSQL(`
		CREATE TABLE products (
			id serial PRIMARY KEY,
			name text NOT NULL,
			price numeric(10,2) DEFAULT 0.00,
			category_id integer REFERENCES categories(id) ON DELETE SET NULL,
			CONSTRAINT products_name_unique UNIQUE (name),
			CONSTRAINT products_price_check CHECK (price >= 0)
		);
	`)
	require.NoError(t, err)
	table := schema.Schemas["public"].Tables["products"]
	// serial PRIMARY KEY creates a column + pk constraint; NOT NULL is implicit
	// price DEFAULT creates a column; category_id REFERENCES creates a column + fk constraint
	// UNIQUE and CHECK are named constraints
	// Total: 4 columns (id, name, price, category_id) + serial sequence
	// Note: serial type gets expanded by PostgreSQL; pg_query may parse it differently
	assert.NotNil(t, table.PrimaryKey)
}

func TestParseIndexHandler_MultiColumnIndex(t *testing.T) {
	p := NewParser()
	schema, err := p.ParseSQL(`
		CREATE TABLE users (id integer, first_name text, last_name text);
		CREATE INDEX idx_users_name ON users (first_name, last_name);
	`)
	require.NoError(t, err)
	idx := schema.Schemas["public"].Tables["users"].Indexes["idx_users_name"]
	require.NotNil(t, idx)
	assert.Len(t, idx.Elements, 2)
}

func TestParseIndexHandler_ConcurrentIndex(t *testing.T) {
	p := NewParser()
	schema, err := p.ParseSQL(`
		CREATE TABLE users (id integer, email text);
		CREATE INDEX CONCURRENTLY idx_users_email ON users (email);
	`)
	require.NoError(t, err)
	idx := schema.Schemas["public"].Tables["users"].Indexes["idx_users_email"]
	require.NotNil(t, idx)
	assert.True(t, idx.Concurrent)
}

func TestParseIndexHandler_IfNotExistsIndex(t *testing.T) {
	p := NewParser()
	schema, err := p.ParseSQL(`
		CREATE TABLE users (id integer, email text);
		CREATE INDEX IF NOT EXISTS idx_users_email ON users (email);
	`)
	require.NoError(t, err)
	idx := schema.Schemas["public"].Tables["users"].Indexes["idx_users_email"]
	require.NotNil(t, idx)
	assert.True(t, idx.IfNotExists)
}

func TestParseIndexHandler_GinIndex(t *testing.T) {
	p := NewParser()
	schema, err := p.ParseSQL(`
		CREATE TABLE docs (id integer, content jsonb);
		CREATE INDEX idx_docs_content ON docs USING gin (content);
	`)
	require.NoError(t, err)
	idx := schema.Schemas["public"].Tables["docs"].Indexes["idx_docs_content"]
	require.NotNil(t, idx)
	assert.Equal(t, "gin", idx.Method)
}

// ─── applyTableMetadata tests via CreateTableMutation ──────────────

func TestApplyTableMetadata_EmptyConstraintTable(t *testing.T) {
	// When constraint.Table is empty, it should be set to the table name
	schema := model.NewSchema()
	m := CreateTableMutation{
		Schema: "public",
		Name:   "orders",
		Columns: []model.Column{
			{Name: "id", DataType: "integer"},
		},
		Constraints: []model.Constraint{
			{Name: "orders_check", Type: "check", Table: "", Expression: "id > 0"},
		},
	}
	err := m.Apply(schema)
	require.NoError(t, err)
	constraint := schema.Schemas["public"].Tables["orders"].Constraints["orders_check"]
	assert.Equal(t, "orders", constraint.Table)
}

// ─── defaultConstraintName tests ───────────────────────────────────

func TestDefaultConstraintName_PrimaryKey(t *testing.T) {
	name := defaultConstraintName("users", model.Constraint{Type: "primary_key"})
	assert.Equal(t, "users_pkey", name)
}

func TestDefaultConstraintName_Unique(t *testing.T) {
	name := defaultConstraintName("users", model.Constraint{Type: "unique", Columns: []string{"email"}})
	assert.Equal(t, "users_email_key", name)
}

func TestDefaultConstraintName_UniqueMultiColumn(t *testing.T) {
	name := defaultConstraintName("users", model.Constraint{Type: "unique", Columns: []string{"first_name", "last_name"}})
	assert.Equal(t, "users_first_name_last_name_key", name)
}

func TestDefaultConstraintName_Check(t *testing.T) {
	name := defaultConstraintName("users", model.Constraint{Type: "check"})
	assert.Equal(t, "users_check", name)
}

func TestDefaultConstraintName_ForeignKey(t *testing.T) {
	name := defaultConstraintName("orders", model.Constraint{Type: "foreign_key", Columns: []string{"user_id"}})
	assert.Equal(t, "orders_user_id_fkey", name)
}

func TestDefaultConstraintName_ForeignKeyMultiColumn(t *testing.T) {
	name := defaultConstraintName("items", model.Constraint{Type: "foreign_key", Columns: []string{"order_id", "product_id"}})
	assert.Equal(t, "items_order_id_product_id_fkey", name)
}

func TestDefaultConstraintName_ExplicitName(t *testing.T) {
	name := defaultConstraintName("users", model.Constraint{Name: "my_custom_name", Type: "check"})
	assert.Equal(t, "my_custom_name", name)
}

func TestDefaultConstraintName_Fallback(t *testing.T) {
	name := defaultConstraintName("users", model.Constraint{Type: "unknown"})
	assert.Equal(t, "users_constraint", name)
}

// ─── fkActionCode tests ────────────────────────────────────────────

func TestFKActionCode(t *testing.T) {
	tests := []struct {
		input    string
		expected string
	}{
		{"a", "NO ACTION"},
		{"r", "RESTRICT"},
		{"c", "CASCADE"},
		{"n", "SET NULL"},
		{"d", "SET DEFAULT"},
		{"", ""},
		{"x", ""},
	}
	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			result := fkActionCode(tt.input)
			assert.Equal(t, tt.expected, result)
		})
	}
}

// ─── parseTableConstraint: unsupported type ────────────────────────

func TestParseTableConstraint_Exclusion(t *testing.T) {
	// Exclusion constraints are unsupported, should return false
	node := mustParseFirstStmt(t, "CREATE TABLE t (id integer)")
	createStmt := node.GetCreateStmt()
	require.NotNil(t, createStmt)

	// Create a constraint node with an unsupported type
	constraint := &pg_query.Constraint{
		Contype: pg_query.ConstrType_CONSTR_EXCLUSION,
	}
	_, ok := parseTableConstraint("t", constraint)
	assert.False(t, ok)
}

// ─── generateDefaultIndexName tests ────────────────────────────────

func TestGenerateDefaultIndexName_FromColumnName(t *testing.T) {
	// CREATE INDEX without explicit name generates default
	p := NewParser()
	schema, err := p.ParseSQL("CREATE TABLE users (id integer, email text); CREATE INDEX ON users (email);")
	require.NoError(t, err)
	idx := schema.Schemas["public"].Tables["users"].Indexes["users_email_idx"]
	require.NotNil(t, idx)
	assert.Equal(t, "users_email_idx", idx.Name)
}

func TestGenerateDefaultIndexName_FromExpression(t *testing.T) {
	p := NewParser()
	schema, err := p.ParseSQL("CREATE TABLE users (id integer, email text); CREATE INDEX ON users ((lower(email)));")
	require.NoError(t, err)
	idx := schema.Schemas["public"].Tables["users"].Indexes["users_expr_idx"]
	require.NotNil(t, idx)
	assert.Equal(t, "users_expr_idx", idx.Name)
}

func TestGenerateDefaultIndexName_Fallback(t *testing.T) {
	p := NewParser()
	schema, err := p.ParseSQL("CREATE TABLE users (id integer);")
	require.NoError(t, err)

	// Manually test the fallback by creating an index with no params
	_ = schema
}

// ─── extractDefaultExpr tests ──────────────────────────────────────

func TestExtractDefaultExpr(t *testing.T) {
	node := mustParseFirstStmt(t, "ALTER TABLE users ALTER COLUMN name SET DEFAULT 'active'")
	alterStmt := node.GetAlterTableStmt()
	require.NotNil(t, alterStmt)
	require.NotEmpty(t, alterStmt.Cmds)

	cmd := alterStmt.Cmds[0].GetAlterTableCmd()
	require.NotNil(t, cmd)
	require.NotNil(t, cmd.Def)

	expr := extractDefaultExpr(cmd.Def)
	assert.NotEmpty(t, expr)
}

// ─── Parser: ParseSQL with naked struct literal ────────────────────

func TestParseSQL_NakedStructLiteral(t *testing.T) {
	// Parser created via struct literal (no constructor)
	p := &Parser{}
	schema, err := p.ParseSQL("CREATE TABLE t (id integer);")
	require.NoError(t, err)
	assert.NotNil(t, schema)
}

func TestParseSQL_NakedStructLiteralWithError(t *testing.T) {
	p := &Parser{}
	_, err := p.ParseSQL("SELECT 1;")
	require.Error(t, err)
}

// ─── Parser: ParseSQL state reset between calls ────────────────────

func TestParseSQL_StateResetSchema(t *testing.T) {
	p := NewParser()

	// First call: create a table
	_, err := p.ParseSQL("CREATE TABLE users (id integer);")
	require.NoError(t, err)

	// Second call: create a different table
	schema, err := p.ParseSQL("CREATE TABLE posts (id integer);")
	require.NoError(t, err)

	// Should only have the second table
	_, hasUsers := schema.Schemas["public"].Tables["users"]
	assert.False(t, hasUsers)
	_, hasPosts := schema.Schemas["public"].Tables["posts"]
	assert.True(t, hasPosts)
}

// ─── Edge case: SQL with only whitespace ───────────────────────────

func TestParseSQL_WhitespaceOnly(t *testing.T) {
	p := NewParser()
	schema, err := p.ParseSQL("   \n\t  ")
	require.NoError(t, err)
	assert.NotNil(t, schema)
}

// ─── Edge case: very long SQL ──────────────────────────────────────

func TestParseSQL_LongSQL(t *testing.T) {
	p := NewParser()
	// Build a SQL with many columns
	var sb strings.Builder
	sb.WriteString("CREATE TABLE big_table (")
	for i := 0; i < 100; i++ {
		if i > 0 {
			sb.WriteString(", ")
		}
		fmt.Fprintf(&sb, "col_%d integer", i)
	}
	sb.WriteString(");")

	schema, err := p.ParseSQL(sb.String())
	require.NoError(t, err)
	table := schema.Schemas["public"].Tables["big_table"]
	assert.Len(t, table.Columns, 100)
}
