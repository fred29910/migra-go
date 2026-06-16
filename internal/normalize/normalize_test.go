package normalize

import (
	"testing"

	"github.com/fred29910/migra-go/internal/model"
	"github.com/fred29910/migra-go/internal/util"
	"github.com/stretchr/testify/assert"
)

// TestCanonicalizeIndex_DBIntrospectColumnsPopulatedAsElements tests that when an index
// comes from DB introspect (only Columns populated, Elements empty), canonicalization
// fills in Elements so that sameIndexContent comparison works correctly and push is idempotent.
func TestCanonicalizeIndex_DBIntrospectColumnsPopulatedAsElements(t *testing.T) {
	s := model.NewSchema()
	ns := s.GetOrCreateNamespace("public")
	tbl := model.NewTable("public", "posts")
	ns.Tables["posts"] = tbl

	// Simulate DB introspect: Columns set, Elements empty (the bug scenario)
	idx := &model.Index{
		Name:    "idx_posts_user_id",
		Table:   "posts",
		Columns: []string{"user_id"},
		Unique:  false,
		Method:  "btree",
		// Elements intentionally empty - as returned by DB introspect
	}
	tbl.Indexes["idx_posts_user_id"] = idx

	if err := CanonicalizeSchema(s); err != nil {
		t.Fatal(err)
	}

	got := s.Schemas["public"].Tables["posts"].Indexes["idx_posts_user_id"]
	if len(got.Elements) != 1 {
		t.Fatalf("expected 1 element after canonicalization, got %d", len(got.Elements))
	}
	if got.Elements[0].Name != "user_id" {
		t.Errorf("expected element Name 'user_id', got %q", got.Elements[0].Name)
	}
	if got.Elements[0].Ordering != "default" {
		t.Errorf("expected element Ordering 'default', got %q", got.Elements[0].Ordering)
	}
	if got.Elements[0].NullsOrdering != "default" {
		t.Errorf("expected element NullsOrdering 'default', got %q", got.Elements[0].NullsOrdering)
	}
}

// TestCanonicalizeIndex_SQLFileParserElementsNotAffected tests that indexes from the SQL
// file parser (Elements already set) are not modified by canonicalization.
func TestCanonicalizeIndex_SQLFileParserElementsNotAffected(t *testing.T) {
	s := model.NewSchema()
	ns := s.GetOrCreateNamespace("public")
	tbl := model.NewTable("public", "users")
	ns.Tables["users"] = tbl

	// Simulate SQL file parser: Elements set, Columns empty
	idx := &model.Index{
		Name:  "idx_users_username",
		Table: "users",
		Elements: []model.IndexElem{
			{Name: "username", Ordering: "default", NullsOrdering: "default"},
		},
		Unique: true,
		Method: "btree",
	}
	tbl.Indexes["idx_users_username"] = idx

	if err := CanonicalizeSchema(s); err != nil {
		t.Fatal(err)
	}

	got := s.Schemas["public"].Tables["users"].Indexes["idx_users_username"]
	if len(got.Elements) != 1 {
		t.Fatalf("expected 1 element (unchanged), got %d", len(got.Elements))
	}
	if got.Elements[0].Name != "username" {
		t.Errorf("expected element Name 'username', got %q", got.Elements[0].Name)
	}
}

// TestNormalizeDataType_VarcharWithLength tests that 'character varying(N)' is normalized to 'varchar(N)'.
// This is critical for idempotency: PostgreSQL information_schema returns 'character varying' as data_type
// but the actual type with length comes from character_maximum_length. After introspect builds
// 'character varying(50)', normalize must produce 'varchar(50)' to match the SQL file parser output.
func TestNormalizeDataType_CharacterWithLength(t *testing.T) {
	tests := []struct {
		input string
		want  string
	}{
		{"character(10)", "char(10)"},
		{"CHARACTER(10)", "char(10)"},
		{"char(10)", "char(10)"},
		{"character", "char"},
		{"CHARACTER", "char"},
	}
	for _, tt := range tests {
		got := util.NormalizeDataType(tt.input)
		if got != tt.want {
			t.Errorf("util.NormalizeDataType(%q) = %q, want %q", tt.input, got, tt.want)
		}
	}
}

func TestNormalizeDataType_VarcharWithLength(t *testing.T) {
	tests := []struct {
		input string
		want  string
	}{
		{"character varying(50)", "varchar(50)"},
		{"character varying(100)", "varchar(100)"},
		{"character varying(200)", "varchar(200)"},
		{"character varying", "varchar"},
		{"CHARACTER VARYING(50)", "varchar(50)"},
		{"varchar(50)", "varchar(50)"},
		{"integer", "integer"},
		{"timestamp without time zone", "timestamp"},
	}
	for _, tt := range tests {
		got := util.NormalizeDataType(tt.input)
		if got != tt.want {
			t.Errorf("util.NormalizeDataType(%q) = %q, want %q", tt.input, got, tt.want)
		}
	}
}

// TestNormalizeDefaultExpr_TypeCast tests that type cast suffixes like ::character varying are stripped.
// PostgreSQL stores defaults with type casts (e.g. 'unknown'::character varying) but SQL files
// use plain literals (e.g. 'unknown'). Both should normalize to the same value for idempotency.
func TestNormalizeDefaultExpr_TypeCast(t *testing.T) {
	tests := []struct {
		input string
		want  string
	}{
		{"'unknown'::character varying", "'unknown'"},
		{"'admin'::text", "'admin'"},
		{"now()::timestamp", "now()"},
		{"'unknown'", "'unknown'"},
		{"now()", "now()"},
		{"42", "42"},
		{"nextval('seq'::regclass)", "nextval('seq')"},
	}
	for _, tt := range tests {
		got := normalizeDefaultExpr(tt.input)
		if got != tt.want {
			t.Errorf("normalizeDefaultExpr(%q) = %q, want %q", tt.input, got, tt.want)
		}
	}
}

func TestCanonicalizeIndex_PreservesDefinitionAndPredicate(t *testing.T) {
	s := model.NewSchema()
	ns := s.GetOrCreateNamespace("public")
	tbl := model.NewTable("public", "users")
	ns.Tables["users"] = tbl
	tbl.Indexes["idx_users_email"] = &model.Index{
		Name:        "idx_users_email",
		Table:       "users",
		Columns:     []string{"email"},
		Method:      "btree",
		Definition:  "CREATE INDEX idx_users_email ON public.users USING btree (email)",
		WhereClause: "email IS NOT NULL",
	}

	if err := CanonicalizeSchema(s); err != nil {
		t.Fatal(err)
	}
	idx := tbl.Indexes["idx_users_email"]
	if idx.WhereClause != "email IS NOT NULL" {
		t.Fatalf("unexpected predicate: %q", idx.WhereClause)
	}
	if idx.Definition == "" {
		t.Fatal("expected definition to be preserved")
	}
}

func TestCanonicalizeSchema_InPlace(t *testing.T) {
	s := model.NewSchema()
	ns := s.GetOrCreateNamespace("Public")
	tbl := model.NewTable("Public", "Users")
	d := "( now() )"
	tbl.AddColumn(&model.Column{Name: "Name", DataType: "INT4", DefaultExpr: &d, IsNullable: true})
	ns.Tables["Users"] = tbl

	beforePtr := tbl.Columns[0]
	if err := CanonicalizeSchema(s); err != nil {
		t.Fatal(err)
	}
	afterPtr := s.Schemas["Public"].Tables["Users"].Columns[0]
	if beforePtr != afterPtr {
		t.Fatal("expected in-place mutation without replacing column pointer")
	}
	if afterPtr.DataType != "integer" {
		t.Fatalf("expected normalized type integer, got %s", afterPtr.DataType)
	}
}

func TestNormalizeIdentifier_UnquotedLowersCase(t *testing.T) {
	assert.Equal(t, "users", normalizeIdentifier("USERS"))
	assert.Equal(t, "users", normalizeIdentifier("Users"))
	assert.Equal(t, "users", normalizeIdentifier("users"))
}

func TestNormalizeIdentifier_QuotedPreservesCase(t *testing.T) {
	assert.Equal(t, "Users", normalizeIdentifier(`"Users"`))
	assert.Equal(t, "USERS", normalizeIdentifier(`"USERS"`))
	assert.Equal(t, "users", normalizeIdentifier(`"users"`))
}

func TestNormalizeIdentifier_EmptyString(t *testing.T) {
	assert.Equal(t, "", normalizeIdentifier(""))
}

func TestNormalizeIdentifier_SpacesTrimmed(t *testing.T) {
	assert.Equal(t, "users", normalizeIdentifier("  users  "))
}

func TestNormalizeIdentifier_UnmatchedQuote(t *testing.T) {
	// A single unmatched quote should NOT strip
	result := normalizeIdentifier(`"Users`)
	assert.Contains(t, result, `"`)
}

func TestNormalizeConstraintDef_NormalizesWhitespace(t *testing.T) {
	assert.Equal(t, "unique (a, b)", normalizeConstraintDef("  UNIQUE   (a,  b)  "))
	assert.Equal(t, "check (x > 0)", normalizeConstraintDef("CHECK (x > 0)"))
}

func TestNormalizeConstraintDef_Lowercases(t *testing.T) {
	assert.Equal(t, "foreign key (a) references b (c)", normalizeConstraintDef("FOREIGN KEY (a) REFERENCES b (c)"))
	assert.Equal(t, "unique (a, b)", normalizeConstraintDef("UNIQUE (a, b)"))
}

func TestNormalizeConstraintDef_EmptyString(t *testing.T) {
	assert.Equal(t, "", normalizeConstraintDef(""))
}

func TestIsBalancedParens_BasicCases(t *testing.T) {
	assert.True(t, isBalancedParens("(a)"))
	assert.True(t, isBalancedParens("((a))"))
	assert.True(t, isBalancedParens(""))
	assert.True(t, isBalancedParens("(a + (b * c))"))
}

func TestIsBalancedParens_Unbalanced(t *testing.T) {
	assert.False(t, isBalancedParens("(a"))
	assert.False(t, isBalancedParens("a)"))
	assert.False(t, isBalancedParens(")("))
	assert.False(t, isBalancedParens("((a)"))
}

func TestNormalizeDefaultExpr_DoubleTypeCast(t *testing.T) {
	assert.Equal(t, "'val'", normalizeDefaultExpr("'val'::varchar::text"))
}

func TestNormalizeDefaultExpr_WhitespaceInCast(t *testing.T) {
	assert.Equal(t, "'val'", normalizeDefaultExpr("'val':: character varying"))
}

func TestNormalizeDefaultExpr_EmptyString(t *testing.T) {
	assert.Equal(t, "", normalizeDefaultExpr(""))
}

func TestNormalizeDefaultExpr_Literals(t *testing.T) {
	assert.Equal(t, "true", normalizeDefaultExpr("true"))
	assert.Equal(t, "false", normalizeDefaultExpr("false"))
	assert.Equal(t, "-1", normalizeDefaultExpr("-1"))
	assert.Equal(t, "gen_random_uuid()", normalizeDefaultExpr("gen_random_uuid()"))
}

func TestNormalizeDataType_EdgeCases(t *testing.T) {
	assert.Equal(t, "integer", util.NormalizeDataType("  integer  "))
	assert.Equal(t, "integer", util.NormalizeDataType(" int4 "))
	assert.Equal(t, "", util.NormalizeDataType(""))
	assert.Equal(t, "numeric(10,2)", util.NormalizeDataType("numeric(10,2)"))
	assert.Equal(t, "text[]", util.NormalizeDataType("text[]"))
	assert.Equal(t, "integer[]", util.NormalizeDataType("int4[]"))
	assert.Equal(t, "pg_catalog.int4", util.NormalizeDataType("pg_catalog.int4"))
	assert.Equal(t, "custom_type", util.NormalizeDataType("custom_type"))
}
