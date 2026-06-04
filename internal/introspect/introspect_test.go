package introspect

import (
	"context"
	"fmt"
	"strings"
	"testing"

	"github.com/fred29910/migra-go/internal/model"
	"github.com/jackc/pgx/v5"
	"github.com/pashagolub/pgxmock/v4"
)

type mockQuerier struct {
	conn pgxmock.PgxConnIface
}

func (m *mockQuerier) Query(ctx context.Context, sql string, args ...any) (pgx.Rows, error) {
	return m.conn.Query(ctx, sql, args...)
}

func newMock(t *testing.T) (*mockQuerier, pgxmock.PgxConnIface) {
	t.Helper()
	mock, err := pgxmock.NewConn()
	if err != nil {
		t.Fatalf("pgxmock.NewConn: %v", err)
	}
	t.Cleanup(func() { _ = mock.Close(context.Background()) })
	return &mockQuerier{conn: mock}, mock
}

func emptyTableRows() *pgxmock.Rows {
	return pgxmock.NewRows([]string{
		"table_name", "column_name", "data_type", "character_maximum_length",
		"is_nullable", "column_default", "ordinal_position",
		"is_identity", "identity_generation", "collation_name",
	})
}

func emptyConstraintRows() *pgxmock.Rows {
	return pgxmock.NewRows([]string{"conname", "contype", "table_name", "column_names", "definition"})
}

func emptyFKRows() *pgxmock.Rows {
	return pgxmock.NewRows([]string{
		"conname", "table_name", "column_names", "definition",
		"ref_table", "ref_schema", "ref_column_names",
		"confupdtype", "confdeltype",
	})
}

func emptyIndexRows() *pgxmock.Rows {
	return pgxmock.NewRows([]string{
		"index_name", "table_name", "column_names", "element_defs",
		"is_unique", "method", "definition", "predicate",
	})
}

func emptyEnumRows() *pgxmock.Rows {
	return pgxmock.NewRows([]string{"typname", "enumlabel", "enumsortorder"})
}

func emptyViewRows() *pgxmock.Rows {
	return pgxmock.NewRows([]string{"viewname", "definition", "materialized"})
}

func emptySequenceRows() *pgxmock.Rows {
	return pgxmock.NewRows([]string{
		"sequencename", "data_type", "start_value", "min_value",
		"max_value", "increment_by", "cycle", "cache_size",
	})
}

func emptyExtensionRows() *pgxmock.Rows {
	return pgxmock.NewRows([]string{"extname", "extversion"})
}

func expectAllEmptyQueries(mock pgxmock.PgxConnIface) {
	mock.ExpectQuery("SELECT").WithArgs(pgxmock.AnyArg()).WillReturnRows(emptyTableRows())
	mock.ExpectQuery("SELECT").WithArgs(pgxmock.AnyArg()).WillReturnRows(emptyConstraintRows())
	mock.ExpectQuery("SELECT").WithArgs(pgxmock.AnyArg()).WillReturnRows(emptyFKRows())
	mock.ExpectQuery("SELECT").WithArgs(pgxmock.AnyArg()).WillReturnRows(emptyIndexRows())
	mock.ExpectQuery("SELECT").WithArgs(pgxmock.AnyArg()).WillReturnRows(emptyEnumRows())
	mock.ExpectQuery("SELECT").WithArgs(pgxmock.AnyArg()).WillReturnRows(emptyViewRows())
	mock.ExpectQuery("SELECT").WithArgs(pgxmock.AnyArg()).WillReturnRows(emptySequenceRows())
	mock.ExpectQuery("SELECT").WithArgs(pgxmock.AnyArg()).WillReturnRows(emptyExtensionRows())
}

func TestPgConstraintAction(t *testing.T) {
	tests := []struct {
		code     string
		expected string
	}{
		{"a", "NO ACTION"},
		{"r", "RESTRICT"},
		{"c", "CASCADE"},
		{"n", "SET NULL"},
		{"d", "SET DEFAULT"},
		{"x", ""},
		{"", ""},
		{"A", ""},
		{"cascade", ""},
	}

	for _, tt := range tests {
		t.Run(fmt.Sprintf("code_%s", tt.code), func(t *testing.T) {
			got := pgConstraintAction(tt.code)
			if got != tt.expected {
				t.Errorf("pgConstraintAction(%q) = %q, want %q", tt.code, got, tt.expected)
			}
		})
	}
}

func TestParseIndexElementDefinitions_SimpleColumn(t *testing.T) {
	got, err := parseIndexElementDefinitions([]string{"id"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(got) != 1 || got[0].Name != "id" {
		t.Errorf("expected Name='id', got %v", got)
	}
}

func TestParseIndexElementDefinitions_Expression(t *testing.T) {
	got, err := parseIndexElementDefinitions([]string{"(lower(name))"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(got) != 1 || got[0].Expr != "lower(name)" {
		t.Errorf("expected Expr='lower(name)', got %v", got)
	}
}

func TestParseIndexElementDefinitions_Empty(t *testing.T) {
	got, err := parseIndexElementDefinitions([]string{})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(got) != 0 {
		t.Errorf("expected 0 elements, got %d", len(got))
	}
}

func TestLoadTables_EmptySchema(t *testing.T) {
	q, mock := newMock(t)
	mock.ExpectQuery("SELECT").WithArgs(pgxmock.AnyArg()).WillReturnRows(emptyTableRows())

	ns := model.NewNamespace("public")
	if err := loadTables(context.Background(), q, "public", ns); err != nil {
		t.Fatalf("loadTables: %v", err)
	}
	if len(ns.Tables) != 0 {
		t.Errorf("expected 0 tables, got %d", len(ns.Tables))
	}
}

func TestLoadTables_SingleTable(t *testing.T) {
	q, mock := newMock(t)

	rows := emptyTableRows().
		AddRow("users", "id", "bigint", nil, "NO", "nextval('users_id_seq'::regclass)", 1, "NO", nil, "").
		AddRow("users", "name", "varchar", int64(255), "YES", nil, 2, "NO", nil, "").
		AddRow("users", "email", "text", nil, "NO", nil, 3, "NO", nil, "en_US")
	mock.ExpectQuery("SELECT").WithArgs(pgxmock.AnyArg()).WillReturnRows(rows)

	ns := model.NewNamespace("public")
	if err := loadTables(context.Background(), q, "public", ns); err != nil {
		t.Fatalf("loadTables: %v", err)
	}

	table, ok := ns.Tables["users"]
	if !ok {
		t.Fatal("expected 'users' table")
	}
	if len(table.Columns) != 3 {
		t.Fatalf("expected 3 columns, got %d", len(table.Columns))
	}

	idCol := table.Columns[0]
	if idCol.Name != "id" || idCol.DataType != "bigint" || idCol.IsNullable {
		t.Errorf("id column: name=%q, type=%q, nullable=%v", idCol.Name, idCol.DataType, idCol.IsNullable)
	}
	if idCol.DefaultExpr == nil || *idCol.DefaultExpr != "nextval('users_id_seq'::regclass)" {
		t.Errorf("id column default: %v", idCol.DefaultExpr)
	}

	nameCol := table.Columns[1]
	if nameCol.DataType != "varchar(255)" {
		t.Errorf("name column type: got %q, want 'varchar(255)'", nameCol.DataType)
	}
	if !nameCol.IsNullable {
		t.Error("name column should be nullable")
	}

	emailCol := table.Columns[2]
	if emailCol.Collation != "en_US" {
		t.Errorf("email column collation: got %q, want 'en_US'", emailCol.Collation)
	}
}

func TestLoadTables_IdentityColumn(t *testing.T) {
	q, mock := newMock(t)

	rows := emptyTableRows().
		AddRow("items", "id", "integer", nil, "NO", nil, 1, "YES", "ALWAYS", "")
	mock.ExpectQuery("SELECT").WithArgs(pgxmock.AnyArg()).WillReturnRows(rows)

	ns := model.NewNamespace("public")
	if err := loadTables(context.Background(), q, "public", ns); err != nil {
		t.Fatalf("loadTables: %v", err)
	}

	col := ns.Tables["items"].Columns[0]
	if !col.IsIdentity {
		t.Error("expected IsIdentity=true")
	}
	if col.IdentityKind != "ALWAYS" {
		t.Errorf("expected IdentityKind='ALWAYS', got %q", col.IdentityKind)
	}
}

func TestLoadTables_QueryError(t *testing.T) {
	q, mock := newMock(t)
	mock.ExpectQuery("SELECT").WithArgs(pgxmock.AnyArg()).WillReturnError(fmt.Errorf("connection lost"))

	ns := model.NewNamespace("public")
	err := loadTables(context.Background(), q, "public", ns)
	if err == nil {
		t.Fatal("expected error from loadTables")
	}
	if !strings.Contains(err.Error(), "query tables") {
		t.Errorf("unexpected error message: %v", err)
	}
}

func TestLoadTables_ScanError(t *testing.T) {
	q, mock := newMock(t)
	rows := pgxmock.NewRows([]string{"table_name", "column_name"}).AddRow("users", "id")
	mock.ExpectQuery("SELECT").WithArgs(pgxmock.AnyArg()).WillReturnRows(rows)

	ns := model.NewNamespace("public")
	err := loadTables(context.Background(), q, "public", ns)
	if err == nil {
		t.Fatal("expected scan error")
	}
	if !strings.Contains(err.Error(), "scan table row") {
		t.Errorf("unexpected error message: %v", err)
	}
}

func TestLoadConstraints_EmptySchema(t *testing.T) {
	q, mock := newMock(t)
	mock.ExpectQuery("SELECT").WithArgs(pgxmock.AnyArg()).WillReturnRows(emptyConstraintRows())

	ns := model.NewNamespace("public")
	if err := loadConstraints(context.Background(), q, "public", ns); err != nil {
		t.Fatalf("loadConstraints: %v", err)
	}
}

func TestLoadConstraints_PrimaryKey(t *testing.T) {
	q, mock := newMock(t)

	ns := model.NewNamespace("public")
	ns.Tables["users"] = model.NewTable("public", "users")

	rows := emptyConstraintRows().
		AddRow("users_pkey", "p", "users", []string{"id"}, "PRIMARY KEY (id)")
	mock.ExpectQuery("SELECT").WithArgs(pgxmock.AnyArg()).WillReturnRows(rows)

	if err := loadConstraints(context.Background(), q, "public", ns); err != nil {
		t.Fatalf("loadConstraints: %v", err)
	}

	con, ok := ns.Tables["users"].Constraints["users_pkey"]
	if !ok {
		t.Fatal("expected constraint 'users_pkey'")
	}
	if con.Type != "primary_key" {
		t.Errorf("expected type 'primary_key', got %q", con.Type)
	}
	if ns.Tables["users"].PrimaryKey == nil {
		t.Fatal("expected PrimaryKey to be set")
	}
	if ns.Tables["users"].PrimaryKey.Columns[0] != "id" {
		t.Errorf("expected PrimaryKey.Columns=['id'], got %v", ns.Tables["users"].PrimaryKey.Columns)
	}
}

func TestLoadConstraints_Unique(t *testing.T) {
	q, mock := newMock(t)

	ns := model.NewNamespace("public")
	ns.Tables["users"] = model.NewTable("public", "users")

	rows := emptyConstraintRows().
		AddRow("uq_email", "u", "users", []string{"email"}, "UNIQUE (email)")
	mock.ExpectQuery("SELECT").WithArgs(pgxmock.AnyArg()).WillReturnRows(rows)

	if err := loadConstraints(context.Background(), q, "public", ns); err != nil {
		t.Fatalf("loadConstraints: %v", err)
	}

	con := ns.Tables["users"].Constraints["uq_email"]
	if con.Type != "unique" {
		t.Errorf("expected type 'unique', got %q", con.Type)
	}
}

func TestLoadConstraints_Check(t *testing.T) {
	q, mock := newMock(t)

	ns := model.NewNamespace("public")
	ns.Tables["users"] = model.NewTable("public", "users")

	rows := emptyConstraintRows().
		AddRow("chk_age", "c", "users", []string{"age"}, "CHECK (age >= 0)")
	mock.ExpectQuery("SELECT").WithArgs(pgxmock.AnyArg()).WillReturnRows(rows)

	if err := loadConstraints(context.Background(), q, "public", ns); err != nil {
		t.Fatalf("loadConstraints: %v", err)
	}

	con := ns.Tables["users"].Constraints["chk_age"]
	if con.Type != "check" {
		t.Errorf("expected type 'check', got %q", con.Type)
	}
}

func TestLoadConstraints_SkipsUnknownTable(t *testing.T) {
	q, mock := newMock(t)

	ns := model.NewNamespace("public")
	rows := emptyConstraintRows().
		AddRow("con1", "p", "unknown", []string{"id"}, "PRIMARY KEY (id)")
	mock.ExpectQuery("SELECT").WithArgs(pgxmock.AnyArg()).WillReturnRows(rows)

	if err := loadConstraints(context.Background(), q, "public", ns); err != nil {
		t.Fatalf("loadConstraints: %v", err)
	}
}

func TestLoadConstraints_QueryError(t *testing.T) {
	q, mock := newMock(t)
	mock.ExpectQuery("SELECT").WithArgs(pgxmock.AnyArg()).WillReturnError(fmt.Errorf("db error"))

	ns := model.NewNamespace("public")
	err := loadConstraints(context.Background(), q, "public", ns)
	if err == nil {
		t.Fatal("expected error")
	}
	if !strings.Contains(err.Error(), "query constraints") {
		t.Errorf("unexpected error: %v", err)
	}
}

func TestLoadForeignKeys_Empty(t *testing.T) {
	q, mock := newMock(t)
	mock.ExpectQuery("SELECT").WithArgs(pgxmock.AnyArg()).WillReturnRows(emptyFKRows())

	ns := model.NewNamespace("public")
	if err := loadForeignKeys(context.Background(), q, "public", ns); err != nil {
		t.Fatalf("loadForeignKeys: %v", err)
	}
}

func TestLoadForeignKeys_WithAction(t *testing.T) {
	q, mock := newMock(t)

	ns := model.NewNamespace("public")
	ns.Tables["orders"] = model.NewTable("public", "orders")

	rows := emptyFKRows().
		AddRow("fk_user", "orders", []string{"user_id"},
			"FOREIGN KEY (user_id) REFERENCES users(id)",
			"users", "public", []string{"id"}, "a", "c")
	mock.ExpectQuery("SELECT").WithArgs(pgxmock.AnyArg()).WillReturnRows(rows)

	if err := loadForeignKeys(context.Background(), q, "public", ns); err != nil {
		t.Fatalf("loadForeignKeys: %v", err)
	}

	con := ns.Tables["orders"].Constraints["fk_user"]
	if con.Type != "foreign_key" {
		t.Errorf("expected type 'foreign_key', got %q", con.Type)
	}
	if con.RefTable != "users" {
		t.Errorf("expected RefTable='users', got %q", con.RefTable)
	}
	if con.RefSchema != "public" {
		t.Errorf("expected RefSchema='public', got %q", con.RefSchema)
	}
	if con.OnUpdate != "NO ACTION" {
		t.Errorf("expected OnUpdate='NO ACTION', got %q", con.OnUpdate)
	}
	if con.OnDelete != "CASCADE" {
		t.Errorf("expected OnDelete='CASCADE', got %q", con.OnDelete)
	}
}

func TestLoadForeignKeys_SkipsUnknownTable(t *testing.T) {
	q, mock := newMock(t)

	ns := model.NewNamespace("public")
	rows := emptyFKRows().
		AddRow("fk1", "missing", []string{"col"}, "FK", "other", "public", []string{"id"}, "a", "a")
	mock.ExpectQuery("SELECT").WithArgs(pgxmock.AnyArg()).WillReturnRows(rows)

	if err := loadForeignKeys(context.Background(), q, "public", ns); err != nil {
		t.Fatalf("loadForeignKeys: %v", err)
	}
}

func TestLoadForeignKeys_QueryError(t *testing.T) {
	q, mock := newMock(t)
	mock.ExpectQuery("SELECT").WithArgs(pgxmock.AnyArg()).WillReturnError(fmt.Errorf("connection refused"))

	ns := model.NewNamespace("public")
	err := loadForeignKeys(context.Background(), q, "public", ns)
	if err == nil {
		t.Fatal("expected error")
	}
	if !strings.Contains(err.Error(), "query foreign keys") {
		t.Errorf("unexpected error: %v", err)
	}
}

func TestLoadEnumTypes_Empty(t *testing.T) {
	q, mock := newMock(t)
	mock.ExpectQuery("SELECT").WithArgs(pgxmock.AnyArg()).WillReturnRows(emptyEnumRows())

	ns := model.NewNamespace("public")
	if err := loadEnumTypes(context.Background(), q, "public", ns); err != nil {
		t.Fatalf("loadEnumTypes: %v", err)
	}
	if len(ns.Types) != 0 {
		t.Errorf("expected 0 types, got %d", len(ns.Types))
	}
}

func TestLoadEnumTypes_SingleEnum(t *testing.T) {
	q, mock := newMock(t)

	rows := emptyEnumRows().
		AddRow("status", "active", float32(1)).
		AddRow("status", "inactive", float32(2)).
		AddRow("status", "deleted", float32(3))
	mock.ExpectQuery("SELECT").WithArgs(pgxmock.AnyArg()).WillReturnRows(rows)

	ns := model.NewNamespace("public")
	if err := loadEnumTypes(context.Background(), q, "public", ns); err != nil {
		t.Fatalf("loadEnumTypes: %v", err)
	}

	enumType, ok := ns.Types["status"]
	if !ok {
		t.Fatal("expected enum type 'status'")
	}
	if len(enumType.Labels) != 3 {
		t.Fatalf("expected 3 labels, got %d", len(enumType.Labels))
	}
	if enumType.Labels[0] != "active" || enumType.Labels[1] != "inactive" || enumType.Labels[2] != "deleted" {
		t.Errorf("unexpected labels: %v", enumType.Labels)
	}
}

func TestLoadEnumTypes_MultipleEnums(t *testing.T) {
	q, mock := newMock(t)

	rows := emptyEnumRows().
		AddRow("color", "red", float32(1)).
		AddRow("color", "blue", float32(2)).
		AddRow("size", "small", float32(1)).
		AddRow("size", "large", float32(2))
	mock.ExpectQuery("SELECT").WithArgs(pgxmock.AnyArg()).WillReturnRows(rows)

	ns := model.NewNamespace("public")
	if err := loadEnumTypes(context.Background(), q, "public", ns); err != nil {
		t.Fatalf("loadEnumTypes: %v", err)
	}
	if len(ns.Types) != 2 {
		t.Errorf("expected 2 types, got %d", len(ns.Types))
	}
}

func TestLoadEnumTypes_QueryError(t *testing.T) {
	q, mock := newMock(t)
	mock.ExpectQuery("SELECT").WithArgs(pgxmock.AnyArg()).WillReturnError(fmt.Errorf("timeout"))

	ns := model.NewNamespace("public")
	err := loadEnumTypes(context.Background(), q, "public", ns)
	if err == nil {
		t.Fatal("expected error")
	}
	if !strings.Contains(err.Error(), "query enum types") {
		t.Errorf("unexpected error: %v", err)
	}
}

func TestLoadIndexes_Empty(t *testing.T) {
	q, mock := newMock(t)
	mock.ExpectQuery("SELECT").WithArgs(pgxmock.AnyArg()).WillReturnRows(emptyIndexRows())

	ns := model.NewNamespace("public")
	if err := loadIndexes(context.Background(), q, "public", ns); err != nil {
		t.Fatalf("loadIndexes: %v", err)
	}
}

func TestLoadIndexes_WithIndex(t *testing.T) {
	q, mock := newMock(t)

	ns := model.NewNamespace("public")
	ns.Tables["users"] = model.NewTable("public", "users")

	rows := emptyIndexRows().
		AddRow("idx_users_email", "users", []string{"email"}, []string{"email"},
			true, "btree", "CREATE UNIQUE INDEX idx_users_email ON users USING btree (email)", "")
	mock.ExpectQuery("SELECT").WithArgs(pgxmock.AnyArg()).WillReturnRows(rows)

	if err := loadIndexes(context.Background(), q, "public", ns); err != nil {
		t.Fatalf("loadIndexes: %v", err)
	}

	idx, ok := ns.Tables["users"].Indexes["idx_users_email"]
	if !ok {
		t.Fatal("expected index 'idx_users_email'")
	}
	if !idx.Unique {
		t.Error("expected Unique=true")
	}
	if idx.Method != "btree" {
		t.Errorf("expected Method='btree', got %q", idx.Method)
	}
}

func TestLoadIndexes_SkipsUnknownTable(t *testing.T) {
	q, mock := newMock(t)

	ns := model.NewNamespace("public")
	rows := emptyIndexRows().
		AddRow("idx1", "nonexistent", []string{"col"}, []string{"col"},
			false, "btree", "CREATE INDEX idx1 ON nonexistent (col)", "")
	mock.ExpectQuery("SELECT").WithArgs(pgxmock.AnyArg()).WillReturnRows(rows)

	if err := loadIndexes(context.Background(), q, "public", ns); err != nil {
		t.Fatalf("loadIndexes: %v", err)
	}
}

func TestLoadIndexes_QueryError(t *testing.T) {
	q, mock := newMock(t)
	mock.ExpectQuery("SELECT").WithArgs(pgxmock.AnyArg()).WillReturnError(fmt.Errorf("db down"))

	ns := model.NewNamespace("public")
	err := loadIndexes(context.Background(), q, "public", ns)
	if err == nil {
		t.Fatal("expected error")
	}
	if !strings.Contains(err.Error(), "query indexes") {
		t.Errorf("unexpected error: %v", err)
	}
}

func TestLoadViews_Empty(t *testing.T) {
	q, mock := newMock(t)
	mock.ExpectQuery("SELECT").WithArgs(pgxmock.AnyArg()).WillReturnRows(emptyViewRows())

	ns := model.NewNamespace("public")
	if err := loadViews(context.Background(), q, "public", ns); err != nil {
		t.Fatalf("loadViews: %v", err)
	}
	if len(ns.Views) != 0 {
		t.Errorf("expected 0 views, got %d", len(ns.Views))
	}
}

func TestLoadViews_WithView(t *testing.T) {
	q, mock := newMock(t)

	rows := emptyViewRows().
		AddRow("active_users", "SELECT id, name FROM users WHERE active = true;", false)
	mock.ExpectQuery("SELECT").WithArgs(pgxmock.AnyArg()).WillReturnRows(rows)

	ns := model.NewNamespace("public")
	if err := loadViews(context.Background(), q, "public", ns); err != nil {
		t.Fatalf("loadViews: %v", err)
	}

	view, ok := ns.Views["active_users"]
	if !ok {
		t.Fatal("expected view 'active_users'")
	}
	if view.Materialized {
		t.Error("expected Materialized=false")
	}
	if view.Definition != "SELECT id, name FROM users WHERE active = true" {
		t.Errorf("unexpected definition: %q", view.Definition)
	}
}

func TestLoadViews_MaterializedView(t *testing.T) {
	q, mock := newMock(t)

	rows := emptyViewRows().
		AddRow("mv_stats", "SELECT count(*) FROM orders;", true)
	mock.ExpectQuery("SELECT").WithArgs(pgxmock.AnyArg()).WillReturnRows(rows)

	ns := model.NewNamespace("public")
	if err := loadViews(context.Background(), q, "public", ns); err != nil {
		t.Fatalf("loadViews: %v", err)
	}

	view := ns.Views["mv_stats"]
	if !view.Materialized {
		t.Error("expected Materialized=true")
	}
}

func TestLoadViews_QueryError(t *testing.T) {
	q, mock := newMock(t)
	mock.ExpectQuery("SELECT").WithArgs(pgxmock.AnyArg()).WillReturnError(fmt.Errorf("permission denied"))

	ns := model.NewNamespace("public")
	err := loadViews(context.Background(), q, "public", ns)
	if err == nil {
		t.Fatal("expected error")
	}
	if !strings.Contains(err.Error(), "query views") {
		t.Errorf("unexpected error: %v", err)
	}
}

func TestLoadSequences_Empty(t *testing.T) {
	q, mock := newMock(t)
	mock.ExpectQuery("SELECT").WithArgs(pgxmock.AnyArg()).WillReturnRows(emptySequenceRows())

	ns := model.NewNamespace("public")
	if err := loadSequences(context.Background(), q, "public", ns); err != nil {
		t.Fatalf("loadSequences: %v", err)
	}
	if len(ns.Sequences) != 0 {
		t.Errorf("expected 0 sequences, got %d", len(ns.Sequences))
	}
}

func TestLoadSequences_WithSequence(t *testing.T) {
	q, mock := newMock(t)

	rows := emptySequenceRows().
		AddRow("users_id_seq", "bigint", int64(1), int64(1), int64(9223372036854775807), int64(1), false, int64(1))
	mock.ExpectQuery("SELECT").WithArgs(pgxmock.AnyArg()).WillReturnRows(rows)

	ns := model.NewNamespace("public")
	if err := loadSequences(context.Background(), q, "public", ns); err != nil {
		t.Fatalf("loadSequences: %v", err)
	}

	seq, ok := ns.Sequences["users_id_seq"]
	if !ok {
		t.Fatal("expected sequence 'users_id_seq'")
	}
	if seq.DataType != "bigint" {
		t.Errorf("expected DataType='bigint', got %q", seq.DataType)
	}
	if seq.IncrementBy != 1 {
		t.Errorf("expected IncrementBy=1, got %d", seq.IncrementBy)
	}
	if seq.Cycle {
		t.Error("expected Cycle=false")
	}
}

func TestLoadSequences_QueryError(t *testing.T) {
	q, mock := newMock(t)
	mock.ExpectQuery("SELECT").WithArgs(pgxmock.AnyArg()).WillReturnError(fmt.Errorf("timeout"))

	ns := model.NewNamespace("public")
	err := loadSequences(context.Background(), q, "public", ns)
	if err == nil {
		t.Fatal("expected error")
	}
	if !strings.Contains(err.Error(), "query sequences") {
		t.Errorf("unexpected error: %v", err)
	}
}

func TestLoadExtensions_Empty(t *testing.T) {
	q, mock := newMock(t)
	mock.ExpectQuery("SELECT").WithArgs(pgxmock.AnyArg()).WillReturnRows(emptyExtensionRows())

	ns := model.NewNamespace("public")
	if err := loadExtensions(context.Background(), q, "public", ns); err != nil {
		t.Fatalf("loadExtensions: %v", err)
	}
	if len(ns.Extensions) != 0 {
		t.Errorf("expected 0 extensions, got %d", len(ns.Extensions))
	}
}

func TestLoadExtensions_WithExtension(t *testing.T) {
	q, mock := newMock(t)

	rows := emptyExtensionRows().
		AddRow("uuid-ossp", "1.1").
		AddRow("pgcrypto", "1.3")
	mock.ExpectQuery("SELECT").WithArgs(pgxmock.AnyArg()).WillReturnRows(rows)

	ns := model.NewNamespace("public")
	if err := loadExtensions(context.Background(), q, "public", ns); err != nil {
		t.Fatalf("loadExtensions: %v", err)
	}

	if len(ns.Extensions) != 2 {
		t.Fatalf("expected 2 extensions, got %d", len(ns.Extensions))
	}

	ext1, ok := ns.Extensions["uuid-ossp"]
	if !ok {
		t.Fatal("expected extension 'uuid-ossp'")
	}
	if ext1.Version != "1.1" {
		t.Errorf("expected Version='1.1', got %q", ext1.Version)
	}

	ext2, ok := ns.Extensions["pgcrypto"]
	if !ok {
		t.Fatal("expected extension 'pgcrypto'")
	}
	if ext2.Version != "1.3" {
		t.Errorf("expected Version='1.3', got %q", ext2.Version)
	}
}

func TestLoadExtensions_QueryError(t *testing.T) {
	q, mock := newMock(t)
	mock.ExpectQuery("SELECT").WithArgs(pgxmock.AnyArg()).WillReturnError(fmt.Errorf("access denied"))

	ns := model.NewNamespace("public")
	err := loadExtensions(context.Background(), q, "public", ns)
	if err == nil {
		t.Fatal("expected error")
	}
	if !strings.Contains(err.Error(), "query extensions") {
		t.Errorf("unexpected error: %v", err)
	}
}

func TestLoadFromQuerier_DefaultSchemas(t *testing.T) {
	q, mock := newMock(t)
	expectAllEmptyQueries(mock)

	schema, err := loadFromQuerier(context.Background(), q, LoadOptions{})
	if err != nil {
		t.Fatalf("loadFromQuerier: %v", err)
	}
	if _, ok := schema.Schemas["public"]; !ok {
		t.Error("expected 'public' schema to be loaded by default")
	}
}

func TestLoadFromQuerier_MultipleSchemas(t *testing.T) {
	q, mock := newMock(t)
	for range []string{"public", "auth"} {
		expectAllEmptyQueries(mock)
	}

	schema, err := loadFromQuerier(context.Background(), q, LoadOptions{
		Schemas: []string{"public", "auth"},
	})
	if err != nil {
		t.Fatalf("loadFromQuerier: %v", err)
	}
	if len(schema.Schemas) != 2 {
		t.Errorf("expected 2 schemas, got %d", len(schema.Schemas))
	}
	if _, ok := schema.Schemas["public"]; !ok {
		t.Error("expected 'public' schema")
	}
	if _, ok := schema.Schemas["auth"]; !ok {
		t.Error("expected 'auth' schema")
	}
}

func TestLoadFromQuerier_ErrorInLoadTables(t *testing.T) {
	q, mock := newMock(t)
	mock.ExpectQuery("SELECT").WithArgs(pgxmock.AnyArg()).WillReturnError(fmt.Errorf("tables query failed"))

	_, err := loadFromQuerier(context.Background(), q, LoadOptions{})
	if err == nil {
		t.Fatal("expected error when loadTables fails")
	}
	if !strings.Contains(err.Error(), "failed to load tables") {
		t.Errorf("unexpected error: %v", err)
	}
}

func TestLoadFromQuerier_ErrorInLoadConstraints(t *testing.T) {
	q, mock := newMock(t)
	mock.ExpectQuery("SELECT").WithArgs(pgxmock.AnyArg()).WillReturnRows(emptyTableRows())
	mock.ExpectQuery("SELECT").WithArgs(pgxmock.AnyArg()).WillReturnError(fmt.Errorf("constraints query failed"))

	_, err := loadFromQuerier(context.Background(), q, LoadOptions{})
	if err == nil {
		t.Fatal("expected error when loadConstraints fails")
	}
	if !strings.Contains(err.Error(), "failed to load constraints") {
		t.Errorf("unexpected error: %v", err)
	}
}

func TestLoadFromQuerier_ErrorInLoadForeignKeys(t *testing.T) {
	q, mock := newMock(t)
	mock.ExpectQuery("SELECT").WithArgs(pgxmock.AnyArg()).WillReturnRows(emptyTableRows())
	mock.ExpectQuery("SELECT").WithArgs(pgxmock.AnyArg()).WillReturnRows(emptyConstraintRows())
	mock.ExpectQuery("SELECT").WithArgs(pgxmock.AnyArg()).WillReturnError(fmt.Errorf("fk query failed"))

	_, err := loadFromQuerier(context.Background(), q, LoadOptions{})
	if err == nil {
		t.Fatal("expected error when loadForeignKeys fails")
	}
	if !strings.Contains(err.Error(), "failed to load foreign keys") {
		t.Errorf("unexpected error: %v", err)
	}
}

func TestLoadFromQuerier_ErrorInLoadIndexes(t *testing.T) {
	q, mock := newMock(t)
	mock.ExpectQuery("SELECT").WithArgs(pgxmock.AnyArg()).WillReturnRows(emptyTableRows())
	mock.ExpectQuery("SELECT").WithArgs(pgxmock.AnyArg()).WillReturnRows(emptyConstraintRows())
	mock.ExpectQuery("SELECT").WithArgs(pgxmock.AnyArg()).WillReturnRows(emptyFKRows())
	mock.ExpectQuery("SELECT").WithArgs(pgxmock.AnyArg()).WillReturnError(fmt.Errorf("index query failed"))

	_, err := loadFromQuerier(context.Background(), q, LoadOptions{})
	if err == nil {
		t.Fatal("expected error when loadIndexes fails")
	}
	if !strings.Contains(err.Error(), "failed to load indexes") {
		t.Errorf("unexpected error: %v", err)
	}
}

func TestLoadFromQuerier_ErrorInLoadEnumTypes(t *testing.T) {
	q, mock := newMock(t)
	mock.ExpectQuery("SELECT").WithArgs(pgxmock.AnyArg()).WillReturnRows(emptyTableRows())
	mock.ExpectQuery("SELECT").WithArgs(pgxmock.AnyArg()).WillReturnRows(emptyConstraintRows())
	mock.ExpectQuery("SELECT").WithArgs(pgxmock.AnyArg()).WillReturnRows(emptyFKRows())
	mock.ExpectQuery("SELECT").WithArgs(pgxmock.AnyArg()).WillReturnRows(emptyIndexRows())
	mock.ExpectQuery("SELECT").WithArgs(pgxmock.AnyArg()).WillReturnError(fmt.Errorf("enum query failed"))

	_, err := loadFromQuerier(context.Background(), q, LoadOptions{})
	if err == nil {
		t.Fatal("expected error when loadEnumTypes fails")
	}
	if !strings.Contains(err.Error(), "failed to load enum types") {
		t.Errorf("unexpected error: %v", err)
	}
}

func TestLoadFromQuerier_ErrorInLoadViews(t *testing.T) {
	q, mock := newMock(t)
	mock.ExpectQuery("SELECT").WithArgs(pgxmock.AnyArg()).WillReturnRows(emptyTableRows())
	mock.ExpectQuery("SELECT").WithArgs(pgxmock.AnyArg()).WillReturnRows(emptyConstraintRows())
	mock.ExpectQuery("SELECT").WithArgs(pgxmock.AnyArg()).WillReturnRows(emptyFKRows())
	mock.ExpectQuery("SELECT").WithArgs(pgxmock.AnyArg()).WillReturnRows(emptyIndexRows())
	mock.ExpectQuery("SELECT").WithArgs(pgxmock.AnyArg()).WillReturnRows(emptyEnumRows())
	mock.ExpectQuery("SELECT").WithArgs(pgxmock.AnyArg()).WillReturnError(fmt.Errorf("views query failed"))

	_, err := loadFromQuerier(context.Background(), q, LoadOptions{})
	if err == nil {
		t.Fatal("expected error when loadViews fails")
	}
	if !strings.Contains(err.Error(), "failed to load views") {
		t.Errorf("unexpected error: %v", err)
	}
}

func TestLoadFromQuerier_ErrorInLoadSequences(t *testing.T) {
	q, mock := newMock(t)
	mock.ExpectQuery("SELECT").WithArgs(pgxmock.AnyArg()).WillReturnRows(emptyTableRows())
	mock.ExpectQuery("SELECT").WithArgs(pgxmock.AnyArg()).WillReturnRows(emptyConstraintRows())
	mock.ExpectQuery("SELECT").WithArgs(pgxmock.AnyArg()).WillReturnRows(emptyFKRows())
	mock.ExpectQuery("SELECT").WithArgs(pgxmock.AnyArg()).WillReturnRows(emptyIndexRows())
	mock.ExpectQuery("SELECT").WithArgs(pgxmock.AnyArg()).WillReturnRows(emptyEnumRows())
	mock.ExpectQuery("SELECT").WithArgs(pgxmock.AnyArg()).WillReturnRows(emptyViewRows())
	mock.ExpectQuery("SELECT").WithArgs(pgxmock.AnyArg()).WillReturnError(fmt.Errorf("sequences query failed"))

	_, err := loadFromQuerier(context.Background(), q, LoadOptions{})
	if err == nil {
		t.Fatal("expected error when loadSequences fails")
	}
	if !strings.Contains(err.Error(), "failed to load sequences") {
		t.Errorf("unexpected error: %v", err)
	}
}

func TestLoadFromQuerier_ErrorInLoadExtensions(t *testing.T) {
	q, mock := newMock(t)
	mock.ExpectQuery("SELECT").WithArgs(pgxmock.AnyArg()).WillReturnRows(emptyTableRows())
	mock.ExpectQuery("SELECT").WithArgs(pgxmock.AnyArg()).WillReturnRows(emptyConstraintRows())
	mock.ExpectQuery("SELECT").WithArgs(pgxmock.AnyArg()).WillReturnRows(emptyFKRows())
	mock.ExpectQuery("SELECT").WithArgs(pgxmock.AnyArg()).WillReturnRows(emptyIndexRows())
	mock.ExpectQuery("SELECT").WithArgs(pgxmock.AnyArg()).WillReturnRows(emptyEnumRows())
	mock.ExpectQuery("SELECT").WithArgs(pgxmock.AnyArg()).WillReturnRows(emptyViewRows())
	mock.ExpectQuery("SELECT").WithArgs(pgxmock.AnyArg()).WillReturnRows(emptySequenceRows())
	mock.ExpectQuery("SELECT").WithArgs(pgxmock.AnyArg()).WillReturnError(fmt.Errorf("extensions query failed"))

	_, err := loadFromQuerier(context.Background(), q, LoadOptions{})
	if err == nil {
		t.Fatal("expected error when loadExtensions fails")
	}
	if !strings.Contains(err.Error(), "failed to load extensions") {
		t.Errorf("unexpected error: %v", err)
	}
}

func TestLoadFromQuerier_FullPipeline(t *testing.T) {
	q, mock := newMock(t)

	mock.ExpectQuery("SELECT").WithArgs(pgxmock.AnyArg()).WillReturnRows(emptyTableRows().
			AddRow("posts", "id", "bigint", nil, "NO", "nextval('posts_id_seq'::regclass)", 1, "NO", nil, "").
			AddRow("posts", "title", "text", nil, "NO", nil, 2, "NO", nil, ""))
	mock.ExpectQuery("SELECT").WithArgs(pgxmock.AnyArg()).WillReturnRows(emptyConstraintRows().
			AddRow("posts_pkey", "p", "posts", []string{"id"}, "PRIMARY KEY (id)"))
	mock.ExpectQuery("SELECT").WithArgs(pgxmock.AnyArg()).WillReturnRows(emptyFKRows())
	mock.ExpectQuery("SELECT").WithArgs(pgxmock.AnyArg()).WillReturnRows(emptyIndexRows().
			AddRow("idx_posts_title", "posts", []string{"title"}, []string{"title"},
				false, "btree", "CREATE INDEX idx_posts_title ON posts USING btree (title)", ""))
	mock.ExpectQuery("SELECT").WithArgs(pgxmock.AnyArg()).WillReturnRows(emptyEnumRows().
			AddRow("post_status", "draft", float32(1)).
			AddRow("post_status", "published", float32(2)))
	mock.ExpectQuery("SELECT").WithArgs(pgxmock.AnyArg()).WillReturnRows(emptyViewRows().
			AddRow("published_posts", "SELECT * FROM posts WHERE status = 'published';", false))
	mock.ExpectQuery("SELECT").WithArgs(pgxmock.AnyArg()).WillReturnRows(emptySequenceRows().
			AddRow("posts_id_seq", "bigint", int64(1), int64(1), int64(9223372036854775807), int64(1), false, int64(1)))
	mock.ExpectQuery("SELECT").WithArgs(pgxmock.AnyArg()).WillReturnRows(emptyExtensionRows())

	schema, err := loadFromQuerier(context.Background(), q, LoadOptions{Schemas: []string{"public"}})
	if err != nil {
		t.Fatalf("loadFromQuerier: %v", err)
	}

	ns := schema.Schemas["public"]
	if ns == nil {
		t.Fatal("expected public namespace")
	}

	table, ok := ns.Tables["posts"]
	if !ok {
		t.Fatal("expected 'posts' table")
	}
	if len(table.Columns) != 2 {
		t.Errorf("expected 2 columns, got %d", len(table.Columns))
	}
	if table.PrimaryKey == nil || table.PrimaryKey.Columns[0] != "id" {
		t.Error("expected primary key on 'id'")
	}

	idx, ok := table.Indexes["idx_posts_title"]
	if !ok {
		t.Fatal("expected index 'idx_posts_title'")
	}
	if idx.Method != "btree" {
		t.Errorf("expected btree method, got %q", idx.Method)
	}

	enum, ok := ns.Types["post_status"]
	if !ok {
		t.Fatal("expected enum type 'post_status'")
	}
	if len(enum.Labels) != 2 || enum.Labels[0] != "draft" || enum.Labels[1] != "published" {
		t.Errorf("unexpected labels: %v", enum.Labels)
	}

	view, ok := ns.Views["published_posts"]
	if !ok {
		t.Fatal("expected view 'published_posts'")
	}
	if view.Definition != "SELECT * FROM posts WHERE status = 'published'" {
		t.Errorf("unexpected view definition: %q", view.Definition)
	}

	seq, ok := ns.Sequences["posts_id_seq"]
	if !ok {
		t.Fatal("expected sequence 'posts_id_seq'")
	}
	if seq.DataType != "bigint" {
		t.Errorf("expected bigint sequence, got %q", seq.DataType)
	}
}
