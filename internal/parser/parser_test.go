package parser

import (
	"errors"
	"strings"
	"testing"

	"github.com/fred29910/migra-go/internal/diff"
	"github.com/fred29910/migra-go/internal/render"
	pg_nodes "github.com/lfittl/pg_query_go/nodes"
)

// TestParseCreateTable tests parsing CREATE TABLE statements
func TestParseCreateTable(t *testing.T) {
	p := NewParser()
	sql := `
		CREATE TABLE users (
			id integer NOT NULL,
			username varchar(50) NOT NULL,
			email varchar(100) DEFAULT 'unknown',
			created_at timestamp
		);
	`

	schema, err := p.ParseSQL(sql)
	if err != nil {
		t.Fatalf("ParseSQL failed: %v", err)
	}

	// Check schema
	if len(schema.Schemas) == 0 {
		t.Fatal("expected schemas, got none")
	}

	ns, exists := schema.Schemas["public"]
	if !exists {
		t.Fatal("expected 'public' schema")
	}

	// Check table
	table, exists := ns.Tables["users"]
	if !exists {
		t.Fatal("expected 'users' table")
	}

	// Check columns
	if len(table.Columns) != 4 {
		t.Errorf("expected 4 columns, got %d", len(table.Columns))
	}

	// Check column names and types
	expectedCols := map[string]string{
		"id":         "integer",
		"username":   "varchar(50)",
		"email":      "varchar(100)",
		"created_at": "timestamp",
	}

	for _, col := range table.Columns {
		expectedType, ok := expectedCols[col.Name]
		if !ok {
			t.Errorf("unexpected column '%s'", col.Name)
			continue
		}
		if col.DataType != expectedType {
			t.Errorf("column '%s': expected type '%s', got '%s'", col.Name, expectedType, col.DataType)
		}
	}

	// Check NOT NULL constraints
	if table.Columns[0].Name == "id" && table.Columns[0].IsNullable {
		t.Error("column 'id' should be NOT NULL")
	}
}

// TestParseCreateIndex tests parsing CREATE INDEX statements
// MVP 范围不包含 CREATE INDEX，此测试跳过
func TestParseCreateIndex(t *testing.T) {
	t.Skip("CREATE INDEX not in MVP scope (only CREATE TABLE and ALTER TABLE ADD COLUMN are supported)")
}

func TestParseCreateEnumType(t *testing.T) {
	p := NewParser()
	schema, err := p.ParseSQL(`CREATE TYPE user_role AS ENUM ('admin', 'user');`)
	if err != nil {
		t.Fatalf("ParseSQL failed: %v", err)
	}
	enumType := schema.Schemas["public"].Types["user_role"]
	if enumType == nil {
		t.Fatal("expected enum type")
	}
	if strings.Join(enumType.Labels, ",") != "admin,user" {
		t.Fatalf("unexpected enum labels: %#v", enumType.Labels)
	}
}

func TestParseCreateTablePrimaryKeyAndForeignKey(t *testing.T) {
	p := NewParser()
	schema, err := p.ParseSQL(`
		CREATE TABLE posts (id integer PRIMARY KEY);
		CREATE TABLE comments (
			id integer PRIMARY KEY,
			post_id integer NOT NULL,
			FOREIGN KEY (post_id) REFERENCES posts(id)
		);
	`)
	if err != nil {
		t.Fatalf("ParseSQL failed: %v", err)
	}
	comments := schema.Schemas["public"].Tables["comments"]
	if comments.PrimaryKey == nil || strings.Join(comments.PrimaryKey.Columns, ",") != "id" {
		t.Fatalf("expected comments primary key, got %#v", comments.PrimaryKey)
	}
	if comments.ColumnByName["id"].IsNullable {
		t.Fatal("column-level primary key should make id NOT NULL")
	}
	if comments.Constraints["comments_pkey"] == nil {
		t.Fatal("expected primary key to also be stored as a table constraint")
	}
	fk := comments.Constraints["comments_post_id_fkey"]
	if fk == nil {
		t.Fatal("expected generated foreign key constraint")
	}
	if fk.Type != "foreign_key" || fk.RefSchema != "public" || fk.RefTable != "posts" || strings.Join(fk.RefColumns, ",") != "id" {
		t.Fatalf("unexpected foreign key: %#v", fk)
	}
}

// TestParseAlterTableAddColumn tests parsing ALTER TABLE ADD COLUMN
func TestParseAlterTableAddColumn(t *testing.T) {
	p := NewParser()
	sql := `
		CREATE TABLE users (id integer);
		ALTER TABLE users ADD COLUMN name varchar(50);
	`

	schema, err := p.ParseSQL(sql)
	if err != nil {
		t.Fatalf("ParseSQL failed: %v", err)
	}

	ns := schema.Schemas["public"]
	table := ns.Tables["users"]

	// Check that table has 2 columns
	if len(table.Columns) != 2 {
		t.Errorf("expected 2 columns, got %d", len(table.Columns))
	}

	// Check new column
	if table.Columns[1].Name != "name" {
		t.Errorf("expected column 'name', got '%s'", table.Columns[1].Name)
	}
	if table.Columns[1].DataType != "varchar(50)" {
		t.Errorf("expected type 'varchar(50)', got '%s'", table.Columns[1].DataType)
	}
}

// TestParseMultipleStatements tests parsing multiple SQL statements
func TestParseMultipleStatements(t *testing.T) {
	p := NewParser()
	sql := `
		CREATE TABLE users (id integer);
		CREATE TABLE posts (id integer, title varchar(200));
		ALTER TABLE users ADD COLUMN name varchar(50);
	`

	schema, err := p.ParseSQL(sql)
	if err != nil {
		t.Fatalf("ParseSQL failed: %v", err)
	}

	ns := schema.Schemas["public"]

	// Check tables
	if len(ns.Tables) != 2 {
		t.Errorf("expected 2 tables, got %d", len(ns.Tables))
	}

	if _, exists := ns.Tables["users"]; !exists {
		t.Error("expected 'users' table")
	}
	if _, exists := ns.Tables["posts"]; !exists {
		t.Error("expected 'posts' table")
	}

	// Check ALTER TABLE worked
	usersTable := ns.Tables["users"]
	if len(usersTable.Columns) != 2 {
		t.Errorf("expected 2 columns in users table, got %d", len(usersTable.Columns))
	}
}

// TestParseErrors tests error handling
func TestParseErrors(t *testing.T) {
	p := NewParser()

	// Test unsupported statement
	sql := `SELECT * FROM users;`
	_, err := p.ParseSQL(sql)
	if err == nil {
		t.Error("expected error for unsupported statement")
	}

	// Check errors
	if len(p.Errors()) == 0 {
		t.Error("expected parsing errors")
	}
}

func TestParseSQLReturnsFirstErrorDetail(t *testing.T) {
	p := NewParser()
	// 使用能触发 visitNode 错误的 SQL，而非 pg_query 语法错误
	_, err := p.ParseSQL("SELECT * FROM users;")
	if err == nil {
		t.Fatal("expected error")
	}
	// 错误应包含 "parsing completed with" 和首个错误详情
	errMsg := err.Error()
	if !strings.Contains(errMsg, "parsing completed with") {
		t.Errorf("expected error to contain 'parsing completed with', got: %s", errMsg)
	}
	// 验证错误包装了首个错误（可通过 Unwrap 提取）
	unwrapped := errors.Unwrap(err)
	if unwrapped == nil {
		t.Errorf("expected error to wrap the first error via Unwrap")
	}
}

func TestParseSQLResetsStateBetweenCalls(t *testing.T) {
	p := NewParser()

	// First call produces an error.
	_, err := p.ParseSQL("SELECT * FROM users;")
	if err == nil {
		t.Fatal("expected first ParseSQL call to fail")
	}

	// Second call should not be affected by previous errors.
	schema, err := p.ParseSQL("CREATE TABLE users (id integer);")
	if err != nil {
		t.Fatalf("expected second ParseSQL call to succeed, got error: %v", err)
	}
	if len(p.Errors()) != 0 {
		t.Fatalf("expected parser error list to be reset, got %d", len(p.Errors()))
	}

	ns, exists := schema.Schemas["public"]
	if !exists {
		t.Fatal("expected 'public' schema")
	}
	if _, exists := ns.Tables["users"]; !exists {
		t.Fatal("expected 'users' table")
	}
}

func TestParseMultilineComments(t *testing.T) {
	p := NewParser()
	sql := `
		/* this is
		   a multiline
		   comment */
		CREATE TABLE users (id integer);
	`

	schema, err := p.ParseSQL(sql)
	if err != nil {
		t.Fatalf("ParseSQL failed: %v", err)
	}

	ns, exists := schema.Schemas["public"]
	if !exists {
		t.Fatal("expected 'public' schema")
	}
	if _, exists := ns.Tables["users"]; !exists {
		t.Fatal("expected 'users' table")
	}
}

// TestIntegrationParserToDiff tests parser -> diff -> render pipeline
func TestIntegrationParserToDiff(t *testing.T) {
	// Source: users table with id, name
	sourceSQL := `
		CREATE TABLE users (
			id integer NOT NULL,
			name varchar(50) NOT NULL
		);
	`

	// Target: users table with id, name (changed to text), age (new)
	targetSQL := `
		CREATE TABLE users (
			id integer NOT NULL,
			name text NOT NULL,
			age integer
		);
	`

	// Parse source
	p1 := NewParser()
	sourceSchema, err := p1.ParseSQL(sourceSQL)
	if err != nil {
		t.Fatalf("Parse source failed: %v", err)
	}

	// Parse target
	p2 := NewParser()
	targetSchema, err := p2.ParseSQL(targetSQL)
	if err != nil {
		t.Fatalf("Parse target failed: %v", err)
	}

	// Run diff
	d := diff.NewDiffer()
	ops, _ := d.Diff(sourceSchema, targetSchema)

	if len(ops) == 0 {
		t.Fatal("expected diff operations, got none")
	}

	// Render SQL
	r := render.NewRenderer()
	sql := r.RenderAll(ops)

	t.Logf("Generated SQL:\n%s", sql)

	// Verify SQL contains expected content
	if !strings.Contains(sql, "ALTER TABLE") {
		t.Error("expected ALTER TABLE statement in rendered SQL")
	}
}

func TestParserInitializationRobustness(t *testing.T) {
	t.Run("NewParserWith nil dependencies", func(t *testing.T) {
		p := NewParserWith(nil, nil)
		_, err := p.ParseSQL("CREATE TABLE t1 (id int);")
		if err != nil {
			t.Fatalf("expected NewParserWith(nil, nil) to be functional, got err: %v", err)
		}
	})

	t.Run("Parser struct literal lazy initialization", func(t *testing.T) {
		p := &Parser{} // Naked literal
		_, err := p.ParseSQL("CREATE TABLE t1 (id int);")
		if err != nil {
			t.Fatalf("expected naked Parser literal to be functional via ParseSQL lazy init, got err: %v", err)
		}
	})
}

func TestParseDefaultFunctionExpressionConsistentForCreateAndAlter(t *testing.T) {
	p := NewParser()
	schema, err := p.ParseSQL(`
		CREATE TABLE events (
			id integer,
			created_at timestamp DEFAULT now()
		);
		ALTER TABLE events ADD COLUMN updated_at timestamp DEFAULT now();
	`)
	if err != nil {
		t.Fatalf("ParseSQL failed: %v", err)
	}

	table := schema.Schemas["public"].Tables["events"]
	created := table.ColumnByName["created_at"]
	updated := table.ColumnByName["updated_at"]
	if created.DefaultExpr == nil || *created.DefaultExpr != "now()" {
		t.Fatalf("expected created_at default now(), got %#v", created.DefaultExpr)
	}
	if updated.DefaultExpr == nil || *updated.DefaultExpr != "now()" {
		t.Fatalf("expected updated_at default now(), got %#v", updated.DefaultExpr)
	}
}

type panickingHandler struct{}

func (h *panickingHandler) Handle(node pg_nodes.Node) ([]SchemaMutation, error) {
	panic("intentional panic for testing recover")
}

func TestParser_RecoverFromPanic(t *testing.T) {
	registry := NewHandlerRegistry()
	// Map CreateStmt to our panicking handler
	registry.Register(pg_nodes.CreateStmt{}, &panickingHandler{})

	p := NewParserWith(registry, nil)
	_, err := p.ParseSQL("CREATE TABLE t1 (id int);")

	if err == nil {
		t.Fatal("expected error from recovered panic, got nil")
	}
	if !strings.Contains(err.Error(), "recovered from panic") {
		t.Errorf("expected error to mention panic recovery, got: %s", err.Error())
	}
	if !strings.Contains(err.Error(), "intentional panic for testing recover") {
		t.Errorf("expected error to contain panic message, got: %s", err.Error())
	}
}
