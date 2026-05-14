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
	_, _, err := loader.Load(context.TODO(), dir, LoadOptions{})
	if err == nil {
		t.Fatal("expected error for duplicate table, got nil")
	}
	if !strings.Contains(err.Error(), "duplicate table") {
		t.Errorf("expected 'duplicate table' in error, got: %v", err)
	}
}

func TestDirectoryLoader_Load_ParseError(t *testing.T) {
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
	_, _, err := loader.Load(context.TODO(), dir, LoadOptions{})
	if err == nil {
		t.Fatal("expected error for parse failure, got nil")
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
