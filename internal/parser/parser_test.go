package parser

import (
	"strings"
	"testing"

	"github.com/fred29910/migra-go/internal/diff"
	"github.com/fred29910/migra-go/internal/render"
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

// TestParseCreateEnumType tests parsing CREATE TYPE ... AS ENUM statements
// MVP 范围不包含 CREATE TYPE，此测试跳过
func TestParseCreateEnumType(t *testing.T) {
	t.Skip("CREATE TYPE not in MVP scope (only CREATE TABLE and ALTER TABLE ADD COLUMN are supported)")
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
	ops := d.Diff(sourceSchema, targetSchema)

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
