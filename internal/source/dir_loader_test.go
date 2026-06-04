package source

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestDirectoryLoader_Match(t *testing.T) {
	loader := &DirectoryLoader{}

	// 创建临时目录
	dir := t.TempDir()

	// 创建临时文件
	file := filepath.Join(dir, "test.sql")
	if err := os.WriteFile(file, []byte("SELECT 1"), 0644); err != nil {
		t.Fatal(err)
	}

	cases := []struct {
		name   string
		source string
		want   bool
	}{
		{"directory", dir, true},
		{"sql file", file, false},
		{"postgres url", "postgres://localhost/db", false},
		{"file prefix", "file:///tmp/test.sql", false},
		{"file:// directory", "file://" + dir, true},
		{"non-existent", "/non/existent/path", false},
	}

	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			if got := loader.Match(c.source); got != c.want {
				t.Errorf("Match(%q) = %v, want %v", c.source, got, c.want)
			}
		})
	}
}

func TestDirectoryLoader_Load_MultiFile(t *testing.T) {
	dir := t.TempDir()

	sql1 := `CREATE TABLE users (
		id SERIAL PRIMARY KEY,
		username VARCHAR(50) NOT NULL
	);`
	sql2 := `CREATE TABLE posts (
		id SERIAL PRIMARY KEY,
		user_id INTEGER NOT NULL,
		title VARCHAR(200) NOT NULL
	);`

	if err := os.WriteFile(filepath.Join(dir, "01_users.sql"), []byte(sql1), 0644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "02_posts.sql"), []byte(sql2), 0644); err != nil {
		t.Fatal(err)
	}

	loader := &DirectoryLoader{}
	schema, errs, err := loader.Load(context.TODO(), dir, LoadOptions{})
	if err != nil {
		t.Fatalf("Load failed: %v", err)
	}
	if len(errs) > 0 {
		t.Fatalf("unexpected errors: %v", errs)
	}
	if schema == nil {
		t.Fatal("expected non-nil schema")
	}
	ns := schema.Schemas["public"]
	if ns == nil {
		t.Fatal("expected public namespace")
	}
	if _, ok := ns.Tables["users"]; !ok {
		t.Error("expected users table")
	}
	if _, ok := ns.Tables["posts"]; !ok {
		t.Error("expected posts table")
	}
}

func TestDirectoryLoader_Load_EmptyDir(t *testing.T) {
	dir := t.TempDir()

	loader := &DirectoryLoader{}
	schema, errs, err := loader.Load(context.TODO(), dir, LoadOptions{})
	if err != nil {
		t.Fatalf("Load failed: %v", err)
	}
	if schema == nil {
		t.Fatal("expected non-nil schema")
	}
	if len(schema.Schemas) != 0 {
		t.Errorf("expected empty schema, got %d namespaces", len(schema.Schemas))
	}
	if len(errs) == 0 {
		t.Error("expected warning for empty directory, got none")
	}
	if !strings.Contains(errs[0].Error(), "no .sql files found") {
		t.Errorf("expected 'no .sql files found' warning, got: %v", errs[0])
	}
}

func TestDirectoryLoader_Load_NestedDir(t *testing.T) {
	dir := t.TempDir()
	subDir := filepath.Join(dir, "tables")
	if err := os.MkdirAll(subDir, 0755); err != nil {
		t.Fatal(err)
	}

	sql1 := `CREATE TABLE users (id SERIAL PRIMARY KEY);`
	sql2 := `CREATE TABLE posts (id SERIAL PRIMARY KEY);`

	if err := os.WriteFile(filepath.Join(dir, "01_users.sql"), []byte(sql1), 0644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(subDir, "02_posts.sql"), []byte(sql2), 0644); err != nil {
		t.Fatal(err)
	}

	loader := &DirectoryLoader{}
	schema, _, err := loader.Load(context.TODO(), dir, LoadOptions{})
	if err != nil {
		t.Fatalf("Load failed: %v", err)
	}
	ns := schema.Schemas["public"]
	if ns == nil {
		t.Fatal("expected public namespace")
	}
	if _, ok := ns.Tables["users"]; !ok {
		t.Error("expected users table")
	}
	if _, ok := ns.Tables["posts"]; !ok {
		t.Error("expected posts table from nested dir")
	}
}

func TestDirectoryLoader_Load_DuplicateTable(t *testing.T) {
	dir := t.TempDir()

	sql1 := `CREATE TABLE users (id SERIAL PRIMARY KEY);`
	sql2 := `CREATE TABLE users (id SERIAL PRIMARY KEY, name VARCHAR(50));`

	if err := os.WriteFile(filepath.Join(dir, "01_users.sql"), []byte(sql1), 0644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "02_users.sql"), []byte(sql2), 0644); err != nil {
		t.Fatal(err)
	}

	loader := &DirectoryLoader{}
	_, _, err := loader.Load(context.TODO(), dir, LoadOptions{Strict: true})
	if err == nil {
		t.Fatal("expected error for duplicate table, got nil")
	}
	// With merged parsing, duplicate table is caught by CreateTableMutation.Apply
	if !strings.Contains(err.Error(), "already exists") {
		t.Errorf("expected 'already exists' in error, got: %v", err)
	}
}

func TestDirectoryLoader_Load_CrossFileIndex(t *testing.T) {
	dir := t.TempDir()

	// Reproduce the bug scenario: CREATE TABLE in one file, CREATE INDEX
	// referencing that table in another. With merged parsing, the index
	// creation sees the table and succeeds.
	usersSQL := `CREATE TABLE users (
		id SERIAL PRIMARY KEY,
		username VARCHAR(50) NOT NULL
	);`
	postsSQL := `CREATE TABLE posts (
		id SERIAL PRIMARY KEY,
		user_id INTEGER NOT NULL,
		title VARCHAR(200) NOT NULL
	);`
	indexesSQL := `CREATE INDEX idx_posts_user_id ON posts(user_id);
CREATE UNIQUE INDEX idx_users_username ON users(username);`

	if err := os.WriteFile(filepath.Join(dir, "01_users.sql"), []byte(usersSQL), 0644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "02_posts.sql"), []byte(postsSQL), 0644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "03_indexes.sql"), []byte(indexesSQL), 0644); err != nil {
		t.Fatal(err)
	}

	loader := &DirectoryLoader{}
	schema, errs, err := loader.Load(context.TODO(), dir, LoadOptions{Strict: true})
	if err != nil {
		t.Fatalf("Load failed (cross-file dependencies not resolved): %v", err)
	}
	if len(errs) > 0 {
		t.Fatalf("unexpected errors: %v", errs)
	}
	if schema == nil {
		t.Fatal("expected non-nil schema")
	}

	ns := schema.Schemas["public"]
	if ns == nil {
		t.Fatal("expected public namespace")
	}

	if _, ok := ns.Tables["users"]; !ok {
		t.Error("expected users table")
	}
	if _, ok := ns.Tables["posts"]; !ok {
		t.Error("expected posts table")
	}

	usersTable := ns.Tables["users"]
	if _, ok := usersTable.Indexes["idx_users_username"]; !ok {
		t.Error("expected idx_users_username index on users table")
	}

	postsTable := ns.Tables["posts"]
	if _, ok := postsTable.Indexes["idx_posts_user_id"]; !ok {
		t.Error("expected idx_posts_user_id index on posts table")
	}
}

func TestDirectoryLoader_Load_ParseError_NonStrict(t *testing.T) {
	dir := t.TempDir()

	// Use a statement that pg_query can parse but that produces a
	// statement-level error (unsupported type), so ParseSQL returns
	// a non-nil schema with partial results + error.
	sql1 := `CREATE TABLE users (id SERIAL PRIMARY KEY);`
	sql2 := `CREATE TABLE users (id SERIAL PRIMARY KEY);` // duplicate table causes statement-level error

	if err := os.WriteFile(filepath.Join(dir, "01_good.sql"), []byte(sql1), 0644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "02_dup.sql"), []byte(sql2), 0644); err != nil {
		t.Fatal(err)
	}

	loader := &DirectoryLoader{}
	// Non-strict mode: parse errors should not be fatal, partial results returned
	schema, errs, err := loader.Load(context.TODO(), dir, LoadOptions{Strict: false})
	if err != nil {
		t.Fatalf("non-strict mode should not return fatal error, got: %v", err)
	}
	if schema == nil {
		t.Fatal("expected non-nil schema in non-strict mode")
	}
	if len(errs) == 0 {
		t.Error("expected parse errors in errs, got none")
	}
}

func TestDirectoryLoader_Load_ParseError_Strict(t *testing.T) {
	dir := t.TempDir()

	sql1 := `CREATE TABLE users (id SERIAL PRIMARY KEY);`
	sql2 := `THIS IS NOT VALID SQL;`

	if err := os.WriteFile(filepath.Join(dir, "01_good.sql"), []byte(sql1), 0644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "02_bad.sql"), []byte(sql2), 0644); err != nil {
		t.Fatal(err)
	}

	loader := &DirectoryLoader{}
	// Strict mode: parse errors should be fatal
	_, _, err := loader.Load(context.TODO(), dir, LoadOptions{Strict: true})
	if err == nil {
		t.Fatal("expected error for parse failure in strict mode, got nil")
	}
}

func TestDirectoryLoader_Load_SkipNonSQL(t *testing.T) {
	dir := t.TempDir()

	sql := `CREATE TABLE users (id SERIAL PRIMARY KEY);`
	readme := "This is not SQL"

	if err := os.WriteFile(filepath.Join(dir, "schema.sql"), []byte(sql), 0644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "readme.txt"), []byte(readme), 0644); err != nil {
		t.Fatal(err)
	}

	loader := &DirectoryLoader{}
	schema, _, err := loader.Load(context.TODO(), dir, LoadOptions{})
	if err != nil {
		t.Fatalf("Load failed: %v", err)
	}
	ns := schema.Schemas["public"]
	if ns == nil {
		t.Fatal("expected public namespace")
	}
	if _, ok := ns.Tables["users"]; !ok {
		t.Error("expected users table")
	}
}

func TestDirectoryLoader_Load_SkipHidden(t *testing.T) {
	dir := t.TempDir()

	sql := `CREATE TABLE users (id SERIAL PRIMARY KEY);`
	hidden := `CREATE TABLE secret (id SERIAL PRIMARY KEY);`

	if err := os.WriteFile(filepath.Join(dir, "schema.sql"), []byte(sql), 0644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, ".hidden.sql"), []byte(hidden), 0644); err != nil {
		t.Fatal(err)
	}

	loader := &DirectoryLoader{}
	schema, _, err := loader.Load(context.TODO(), dir, LoadOptions{})
	if err != nil {
		t.Fatalf("Load failed: %v", err)
	}
	ns := schema.Schemas["public"]
	if ns == nil {
		t.Fatal("expected public namespace")
	}
	if _, ok := ns.Tables["users"]; !ok {
		t.Error("expected users table")
	}
	if _, ok := ns.Tables["secret"]; ok {
		t.Error("hidden file should be skipped")
	}
}
