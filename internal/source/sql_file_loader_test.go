package source

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestSQLFileLoader_Load_Success(t *testing.T) {
	dir := t.TempDir()
	file := filepath.Join(dir, "schema.sql")
	sql := `CREATE TABLE users (
		id SERIAL PRIMARY KEY,
		username VARCHAR(50) NOT NULL
	);`

	require.NoError(t, os.WriteFile(file, []byte(sql), 0644))

	loader := &SQLFileLoader{}
	schema, errs, err := loader.Load(context.TODO(), file, LoadOptions{})
	require.NoError(t, err)
	assert.Empty(t, errs)
	require.NotNil(t, schema)

	ns := schema.Schemas["public"]
	require.NotNil(t, ns)
	_, ok := ns.Tables["users"]
	assert.True(t, ok, "expected users table")
}

func TestSQLFileLoader_Load_FileURL(t *testing.T) {
	dir := t.TempDir()
	file := filepath.Join(dir, "schema.sql")
	sql := `CREATE TABLE items (id SERIAL PRIMARY KEY);`

	require.NoError(t, os.WriteFile(file, []byte(sql), 0644))

	loader := &SQLFileLoader{}
	schema, errs, err := loader.Load(context.TODO(), "file://"+file, LoadOptions{})
	require.NoError(t, err)
	assert.Empty(t, errs)
	require.NotNil(t, schema)

	ns := schema.Schemas["public"]
	require.NotNil(t, ns)
	_, ok := ns.Tables["items"]
	assert.True(t, ok, "expected items table")
}

func TestSQLFileLoader_Load_FileNotFound(t *testing.T) {
	loader := &SQLFileLoader{}
	schema, errs, err := loader.Load(context.TODO(), "/nonexistent/path/schema.sql", LoadOptions{})
	require.Error(t, err)
	assert.Nil(t, schema)
	assert.Nil(t, errs)
	assert.Contains(t, err.Error(), "failed to read file")
}

func TestSQLFileLoader_Load_InvalidSQL_NonStrict(t *testing.T) {
	dir := t.TempDir()
	file := filepath.Join(dir, "bad.sql")
	// pg_query.Parse fails entirely on invalid SQL (returns nil tree + error),
	// so ParseSQL returns nil schema. In non-strict mode, Load returns that
	// nil schema without a fatal error.
	sql := "THIS IS NOT VALID SQL;"

	require.NoError(t, os.WriteFile(file, []byte(sql), 0644))

	loader := &SQLFileLoader{}
	schema, errs, err := loader.Load(context.TODO(), file, LoadOptions{Strict: false})
	require.NoError(t, err, "non-strict mode should not return fatal error for parse failures")
	// When pg_query.Parse fails at the top level, schema is nil
	assert.Nil(t, schema)
	_ = errs
}

func TestSQLFileLoader_Load_InvalidSQL_Strict(t *testing.T) {
	dir := t.TempDir()
	file := filepath.Join(dir, "bad.sql")
	sql := `THIS IS NOT VALID SQL;`

	require.NoError(t, os.WriteFile(file, []byte(sql), 0644))

	loader := &SQLFileLoader{}
	schema, errs, err := loader.Load(context.TODO(), file, LoadOptions{Strict: true})
	require.Error(t, err)
	assert.Nil(t, schema)
	assert.Contains(t, err.Error(), "parse SQL")
	_ = errs
}

func TestSQLFileLoader_Load_EmptyFile(t *testing.T) {
	dir := t.TempDir()
	file := filepath.Join(dir, "empty.sql")

	require.NoError(t, os.WriteFile(file, []byte(""), 0644))

	loader := &SQLFileLoader{}
	schema, errs, err := loader.Load(context.TODO(), file, LoadOptions{})
	require.NoError(t, err)
	assert.Empty(t, errs)
	require.NotNil(t, schema)
	assert.Empty(t, schema.Schemas)
}

func TestSQLFileLoader_Load_MultipleStatements(t *testing.T) {
	dir := t.TempDir()
	file := filepath.Join(dir, "multi.sql")
	sql := `CREATE TABLE users (id SERIAL PRIMARY KEY);
CREATE TABLE posts (id SERIAL PRIMARY KEY, user_id INTEGER NOT NULL);
CREATE INDEX idx_posts_user_id ON posts(user_id);`

	require.NoError(t, os.WriteFile(file, []byte(sql), 0644))

	loader := &SQLFileLoader{}
	schema, errs, err := loader.Load(context.TODO(), file, LoadOptions{})
	require.NoError(t, err)
	assert.Empty(t, errs)
	require.NotNil(t, schema)

	ns := schema.Schemas["public"]
	require.NotNil(t, ns)

	_, ok := ns.Tables["users"]
	assert.True(t, ok, "expected users table")

	postsTable, ok := ns.Tables["posts"]
	assert.True(t, ok, "expected posts table")

	_, ok = postsTable.Indexes["idx_posts_user_id"]
	assert.True(t, ok, "expected idx_posts_user_id index on posts table")
}

func TestSQLFileLoader_Load_PartialSuccess_NonStrict(t *testing.T) {
	dir := t.TempDir()
	file := filepath.Join(dir, "partial.sql")
	// First statement is valid, second causes a duplicate table error
	sql := `CREATE TABLE users (id SERIAL PRIMARY KEY);
CREATE TABLE users (id SERIAL PRIMARY KEY, name VARCHAR(50));`

	require.NoError(t, os.WriteFile(file, []byte(sql), 0644))

	loader := &SQLFileLoader{}
	schema, _, err := loader.Load(context.TODO(), file, LoadOptions{Strict: false})
	require.NoError(t, err, "non-strict should not return fatal error")
	require.NotNil(t, schema)

	// The first CREATE TABLE should have succeeded
	ns := schema.Schemas["public"]
	require.NotNil(t, ns)
	_, ok := ns.Tables["users"]
	assert.True(t, ok, "expected users table from first statement")
}

func TestSQLFileLoader_Load_DuplicateTable_Strict(t *testing.T) {
	dir := t.TempDir()
	file := filepath.Join(dir, "dup.sql")
	sql := `CREATE TABLE users (id SERIAL PRIMARY KEY);
CREATE TABLE users (id SERIAL PRIMARY KEY);`

	require.NoError(t, os.WriteFile(file, []byte(sql), 0644))

	loader := &SQLFileLoader{}
	_, _, err := loader.Load(context.TODO(), file, LoadOptions{Strict: true})
	require.Error(t, err)
	assert.True(t, strings.Contains(err.Error(), "already exists") || strings.Contains(err.Error(), "duplicate"),
		"expected duplicate/already exists error, got: %v", err)
}

func TestSQLFileLoader_CancelledContext(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "s.sql")
	if err := os.WriteFile(path, []byte("CREATE TABLE t (id int);"), 0644); err != nil {
		t.Fatal(err)
	}
	l := &SQLFileLoader{}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	_, _, err := l.Load(ctx, path, LoadOptions{})
	if err == nil {
		t.Fatal("expected error on cancelled context, got nil")
	}
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("expected context.Canceled, got %v", err)
	}
}
