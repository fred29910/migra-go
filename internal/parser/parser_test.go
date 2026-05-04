package parser

import (
	"testing"
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
func TestParseCreateIndex(t *testing.T) {
	p := NewParser()
	sql := `
		CREATE TABLE users (id integer, name varchar(50));
		CREATE INDEX idx_users_name ON users (name);
		CREATE UNIQUE INDEX idx_users_id ON users (id);
	`

	schema, err := p.ParseSQL(sql)
	if err != nil {
		t.Fatalf("ParseSQL failed: %v", err)
	}

	ns := schema.Schemas["public"]
	table, exists := ns.Tables["users"]
	if !exists {
		t.Fatal("expected 'users' table")
	}

	// Check indexes
	if len(table.Indexes) != 2 {
		t.Errorf("expected 2 indexes, got %d", len(table.Indexes))
	}

	// Check index properties
	for name, idx := range table.Indexes {
		if name == "idx_users_name" {
			if idx.Unique {
				t.Error("idx_users_name should not be unique")
			}
			if len(idx.Columns) != 1 || idx.Columns[0] != "name" {
				t.Errorf("idx_users_name should index 'name' column")
			}
		}
		if name == "idx_users_id" {
			if !idx.Unique {
				t.Error("idx_users_id should be unique")
			}
		}
	}
}

// TestParseCreateEnumType tests parsing CREATE TYPE ... AS ENUM statements
func TestParseCreateEnumType(t *testing.T) {
	p := NewParser()
	sql := `
		CREATE TYPE user_role AS ENUM ('admin', 'user', 'guest');
	`

	schema, err := p.ParseSQL(sql)
	if err != nil {
		t.Fatalf("ParseSQL failed: %v", err)
	}

	ns, exists := schema.Schemas["public"]
	if !exists {
		t.Fatal("expected 'public' schema")
	}

	// Check enum type
	enumType, exists := ns.Types["user_role"]
	if !exists {
		t.Fatal("expected 'user_role' enum type")
	}

	// Check labels
	expectedLabels := []string{"admin", "user", "guest"}
	if len(enumType.Labels) != len(expectedLabels) {
		t.Errorf("expected %d labels, got %d", len(expectedLabels), len(enumType.Labels))
	}

	for i, label := range enumType.Labels {
		if label != expectedLabels[i] {
			t.Errorf("label %d: expected '%s', got '%s'", i, expectedLabels[i], label)
		}
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
		CREATE INDEX idx_posts_title ON posts (title);
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

// TestIntegrationParserToDiff tests parser -> diff pipeline
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

	// Note: Diff integration would need import of diff package
	// For now, just verify schemas are different
	if len(sourceSchema.Schemas) == 0 || len(targetSchema.Schemas) == 0 {
		t.Fatal("expected schemas")
	}

	t.Log("Parser -> Diff pipeline test completed (placeholder)")
}
