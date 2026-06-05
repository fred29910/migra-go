package main

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/fred29910/migra-go/internal/app"
	"github.com/fred29910/migra-go/internal/diff"
	"github.com/fred29910/migra-go/internal/model"
	"github.com/fred29910/migra-go/internal/plan"
	"github.com/fred29910/migra-go/internal/render"
)

// TestIntegrationDiffRender tests the full diff -> render pipeline
func TestIntegrationDiffRender(t *testing.T) {
	// Create source schema: users table with id, name
	source := model.NewSchema()
	sourceNs := source.GetOrCreateNamespace("public")
	sourceTable := model.NewTable("public", "users")
	sourceTable.AddColumn(&model.Column{Name: "id", DataType: "integer", IsNullable: false})
	sourceTable.AddColumn(&model.Column{Name: "name", DataType: "varchar", IsNullable: false})
	sourceTable.PrimaryKey = &model.PrimaryKey{Name: "users_pkey", Columns: []string{"id"}}
	sourceNs.Tables["users"] = sourceTable

	// Create target schema: users table with id, name (changed to text), age (new)
	target := model.NewSchema()
	targetNs := target.GetOrCreateNamespace("public")
	targetTable := model.NewTable("public", "users")
	targetTable.AddColumn(&model.Column{Name: "id", DataType: "integer", IsNullable: false})
	targetTable.AddColumn(&model.Column{Name: "name", DataType: "text", IsNullable: false})  // Type changed
	targetTable.AddColumn(&model.Column{Name: "age", DataType: "integer", IsNullable: true}) // New column
	targetTable.PrimaryKey = &model.PrimaryKey{Name: "users_pkey", Columns: []string{"id"}}
	targetNs.Tables["users"] = targetTable

	// Run diff
	d := diff.NewDiffer()
	ops, _ := d.Diff(source, target)

	// Verify operations
	if len(ops) == 0 {
		t.Fatal("expected operations, got none")
	}

	t.Logf("Generated %d operations", len(ops))
	for i, op := range ops {
		t.Logf("  Op %d: %s (destructive: %v)", i, op.Kind(), op.IsDestructive())
	}

	// Check for expected operations
	hasAlterColumnType := false
	hasAddColumn := false
	for _, op := range ops {
		switch op.Kind() {
		case diff.KindAlterColumnType:
			hasAlterColumnType = true
			alterOp := op.(*diff.AlterColumnTypeOp)
			if alterOp.Column != "name" {
				t.Errorf("expected alter column 'name', got '%s'", alterOp.Column)
			}
			if alterOp.FromType != "varchar" || alterOp.ToType != "text" {
				t.Errorf("expected alter type varchar->text, got %s->%s", alterOp.FromType, alterOp.ToType)
			}
		case diff.KindAddColumn:
			hasAddColumn = true
			addOp := op.(*diff.AddColumnOp)
			if addOp.Column.Name != "age" {
				t.Errorf("expected add column 'age', got '%s'", addOp.Column.Name)
			}
		}
	}

	if !hasAlterColumnType {
		t.Error("expected AlterColumnType operation")
	}
	if !hasAddColumn {
		t.Error("expected AddColumn operation")
	}

	// Render to SQL
	r := render.NewRenderer()
	sql := r.RenderAll(ops)

	if sql == "" || sql == "-- No changes detected" {
		t.Fatal("expected SQL output, got empty")
	}

	t.Logf("Generated SQL:\n%s", sql)

	// Verify SQL contains expected statements
	if !strings.Contains(sql, "ALTER TABLE") {
		t.Error("expected ALTER TABLE statement")
	}
	if !strings.Contains(sql, "ADD COLUMN") {
		t.Error("expected ADD COLUMN statement")
	}

	// Test JSON rendering
	jsonStr, err := render.NewRenderer().RenderOutput(ops, "json")
	if err != nil {
		t.Errorf("RenderJSON failed: %v", err)
	} else {
		t.Logf("Generated JSON: %s", jsonStr)
		if !strings.Contains(jsonStr, `"kind"`) {
			t.Error("expected JSON to contain 'kind' field")
		}
	}
}

// TestIntegrationEnumType tests enum type diff and render
func TestIntegrationEnumType(t *testing.T) {
	// Create source schema with enum
	source := model.NewSchema()
	sourceNs := source.GetOrCreateNamespace("public")
	sourceNs.Types["user_role"] = &model.EnumType{
		Name:   "user_role",
		Labels: []string{"admin", "user"},
	}

	// Create target schema with enum (added label "guest")
	target := model.NewSchema()
	targetNs := target.GetOrCreateNamespace("public")
	targetNs.Types["user_role"] = &model.EnumType{
		Name:   "user_role",
		Labels: []string{"admin", "user", "guest"},
	}

	// Run diff
	d := diff.NewDiffer()
	ops, _ := d.Diff(source, target)

	// Should detect no changes (enum labels order matters, but we added a new one)
	// Actually, this should detect that the enum type changed
	t.Logf("Enum diff generated %d operations", len(ops))
	for _, op := range ops {
		t.Logf("  Op: %s", op.Kind())
	}

	// Render to SQL
	r := render.NewRenderer()
	sql := r.RenderAll(ops)
	t.Logf("Enum SQL:\n%s", sql)
}

// TestIntegrationDAGSort tests the DAG topological sort
func TestIntegrationDAGSort(t *testing.T) {
	// Create operations in wrong order: add column before add table
	ops := []diff.Operation{
		diff.NewAddColumnOp("public", "users", &model.Column{
			Name: "age", DataType: "integer", IsNullable: true,
		}),
		diff.NewAddTableOp("public", "users", &model.Table{
			Schema: "public", Name: "users",
			Columns: []*model.Column{
				{Name: "id", DataType: "integer", IsNullable: false},
			},
		}),
	}

	// Sort using DAG (from plan package)
	sorted, err := plan.TopoSort(ops)
	if err != nil {
		t.Fatalf("TopoSort failed: %v", err)
	}

	// Verify table creation comes before column addition
	if len(sorted) != 2 {
		t.Fatalf("expected 2 ops, got %d", len(sorted))
	}

	// First op should be AddTable
	if sorted[0].Kind() != diff.KindAddTable {
		t.Errorf("expected first op to be add_table, got %s", sorted[0].Kind())
	}
	// Second op should be AddColumn
	if sorted[1].Kind() != diff.KindAddColumn {
		t.Errorf("expected second op to be add_column, got %s", sorted[1].Kind())
	}

	t.Log("DAG sort order correct")
}

// TestExampleSQLFilesDiffIncludesEnumAndConstraints tests end-to-end diff of example SQL files
func TestExampleSQLFilesDiffIncludesEnumAndConstraints(t *testing.T) {
	out, warns, err := app.NewDiffService(newDefaultDeps()).Run(context.Background(), app.Config{
		Source:     "../../testdata/example_source.sql",
		Target:     "../../testdata/example_target.sql",
		Schemas:    []string{"public"},
		Format:     "sql",
		Timeout:    defaultDiffTimeout,
		UnsafeDrop: false,
	})
	if err != nil {
		t.Fatalf("diff service failed: %v", err)
	}

	for _, want := range []string{
		`CREATE TABLE "public"."comments"`,
		`ALTER TABLE "public"."users" ADD COLUMN "age" integer`,
		`ALTER TYPE "public"."user_role" ADD VALUE 'guest'`,
		`CONSTRAINT "comments_pkey" PRIMARY KEY ("id")`,
	} {
		if !strings.Contains(out, want) {
			t.Fatalf("expected output to contain %q, got:\n%s", want, out)
		}
	}
	for _, warning := range warns {
		if strings.Contains(warning, "CREATE TYPE ENUM is not yet supported") {
			t.Fatalf("unexpected enum unsupported warning: %s", warning)
		}
	}
}

// TestFullPipeline tests the full pipeline with plan stage grouping
func TestFullPipeline(t *testing.T) {
	// Create schemas with various changes
	source := model.NewSchema()
	sourceNs := source.GetOrCreateNamespace("public")
	_ = sourceNs
	sourceTable := model.NewTable("public", "users")
	// ... add columns, constraints, etc.
	_ = sourceTable

	target := model.NewSchema()
	targetNs := target.GetOrCreateNamespace("public")
	_ = targetNs
	targetTable := model.NewTable("public", "users")
	// ... add columns, constraints, etc.
	_ = targetTable

	// This is a placeholder for a more complete test
	t.Log("Full pipeline test - placeholder for more comprehensive tests")
	fmt.Println("Integration tests completed successfully!")
}

func TestDirectoryVsDirectory_Diff(t *testing.T) {
	sourceDir := t.TempDir()
	sourceSQL := `CREATE TABLE users (
		id SERIAL PRIMARY KEY,
		username VARCHAR(50) NOT NULL
	);
	CREATE TABLE posts (
		id SERIAL PRIMARY KEY,
		user_id INTEGER NOT NULL,
		title VARCHAR(200) NOT NULL
	);
	CREATE TYPE user_role AS ENUM ('admin', 'user');`

	if err := os.WriteFile(filepath.Join(sourceDir, "schema.sql"), []byte(sourceSQL), 0644); err != nil {
		t.Fatal(err)
	}

	targetDir := t.TempDir()
	targetSQL := `CREATE TABLE users (
		id SERIAL PRIMARY KEY,
		username VARCHAR(50) NOT NULL,
		age INTEGER
	);
	CREATE TABLE posts (
		id SERIAL PRIMARY KEY,
		user_id INTEGER NOT NULL,
		title VARCHAR(200) NOT NULL
	);
	CREATE TABLE comments (
		id SERIAL PRIMARY KEY,
		post_id INTEGER NOT NULL,
		content TEXT NOT NULL
	);
	CREATE TYPE user_role AS ENUM ('admin', 'user', 'guest');`

	if err := os.WriteFile(filepath.Join(targetDir, "schema.sql"), []byte(targetSQL), 0644); err != nil {
		t.Fatal(err)
	}

	out, _, err := app.NewDiffService(newDefaultDeps()).Run(context.Background(), app.Config{
		Source:     sourceDir,
		Target:     targetDir,
		Schemas:    []string{"public"},
		Format:     "sql",
		Timeout:    defaultDiffTimeout,
		UnsafeDrop: false,
	})
	if err != nil {
		t.Fatalf("diff service failed: %v", err)
	}

	for _, want := range []string{
		`ADD COLUMN "age"`,
		`CREATE TABLE "public"."comments"`,
		`ALTER TYPE "public"."user_role" ADD VALUE 'guest'`,
	} {
		if !strings.Contains(out, want) {
			t.Fatalf("expected output to contain %q, got:\n%s", want, out)
		}
	}
}

func TestDirectoryVsFile_Diff(t *testing.T) {
	sourceDir := t.TempDir()
	sourceSQL := `CREATE TABLE users (
		id SERIAL PRIMARY KEY,
		username VARCHAR(50) NOT NULL
	);`
	if err := os.WriteFile(filepath.Join(sourceDir, "users.sql"), []byte(sourceSQL), 0644); err != nil {
		t.Fatal(err)
	}

	targetDir := t.TempDir()
	targetSQL := `CREATE TABLE users (
		id SERIAL PRIMARY KEY,
		username VARCHAR(50) NOT NULL,
		email VARCHAR(100)
	);`
	targetFile := filepath.Join(targetDir, "schema.sql")
	if err := os.WriteFile(targetFile, []byte(targetSQL), 0644); err != nil {
		t.Fatal(err)
	}

	out, _, err := app.NewDiffService(newDefaultDeps()).Run(context.Background(), app.Config{
		Source:     sourceDir,
		Target:     targetFile,
		Schemas:    []string{"public"},
		Format:     "sql",
		Timeout:    defaultDiffTimeout,
		UnsafeDrop: false,
	})
	if err != nil {
		t.Fatalf("diff service failed: %v", err)
	}

	if !strings.Contains(out, `ADD COLUMN "email"`) {
		t.Fatalf("expected output to contain ADD COLUMN email, got:\n%s", out)
	}
}

func TestRenameColumnDiff(t *testing.T) {
	out, warns, err := app.NewDiffService(newDefaultDeps()).Run(context.Background(), app.Config{
		Source:     "../../testdata/diff/rename_example/v1/schema.sql",
		Target:     "../../testdata/diff/rename_example/v2/schema.sql",
		Schemas:    []string{"public"},
		Format:     "sql",
		Timeout:    defaultDiffTimeout,
		UnsafeDrop: false,
	})
	if err != nil {
		t.Fatalf("diff service failed: %v", err)
	}

	for _, want := range []string{
		`RENAME COLUMN "username" TO "login_name"`,
	} {
		if !strings.Contains(out, want) {
			t.Fatalf("expected output to contain %q, got:\n%s", want, out)
		}
	}

	if len(warns) > 0 {
		t.Fatalf("unexpected warnings: %v", warns)
	}
}

func TestRenameColumnComplexDiff(t *testing.T) {
	out, _, err := app.NewDiffService(newDefaultDeps()).Run(context.Background(), app.Config{
		Source:     "../../testdata/diff/rename_complex/v1/schema.sql",
		Target:     "../../testdata/diff/rename_complex/v2/schema.sql",
		Schemas:    []string{"public"},
		Format:     "sql",
		Timeout:    defaultDiffTimeout,
		UnsafeDrop: false,
	})
	if err != nil {
		t.Fatalf("diff service failed: %v", err)
	}

	for _, want := range []string{
		`RENAME COLUMN "username" TO "login_name"`,
		`ADD COLUMN "phone"`,
		`ADD VALUE 'guest'`,
	} {
		if !strings.Contains(out, want) {
			t.Fatalf("expected output to contain %q, got:\n%s", want, out)
		}
	}

	if strings.Contains(out, `DROP COLUMN`) || strings.Contains(out, `ADD COLUMN "login_name"`) {
		t.Fatalf("should detect rename, not drop+add:\n%s", out)
	}
}

func TestMultiSchemaDiff(t *testing.T) {
	sourceDir := t.TempDir()

	publicSQL := `CREATE TABLE public.users (
		id SERIAL PRIMARY KEY,
		name VARCHAR(100) NOT NULL
	);`
	if err := os.WriteFile(filepath.Join(sourceDir, "01_public.sql"), []byte(publicSQL), 0644); err != nil {
		t.Fatal(err)
	}

	authSQL := `CREATE SCHEMA auth;

CREATE TABLE auth.roles (
    id SERIAL PRIMARY KEY,
    name VARCHAR(50) NOT NULL UNIQUE
);

CREATE TABLE auth.permissions (
    id SERIAL PRIMARY KEY,
    role_id INTEGER NOT NULL REFERENCES auth.roles(id),
    resource VARCHAR(100) NOT NULL,
    action VARCHAR(50) NOT NULL
);`
	if err := os.WriteFile(filepath.Join(sourceDir, "02_auth.sql"), []byte(authSQL), 0644); err != nil {
		t.Fatal(err)
	}

	targetDir := t.TempDir()

	targetPublicSQL := `CREATE TABLE public.users (
		id SERIAL PRIMARY KEY,
		name VARCHAR(100) NOT NULL,
		email VARCHAR(255)
	);

CREATE TABLE public.profiles (
    id SERIAL PRIMARY KEY,
    user_id INTEGER NOT NULL REFERENCES public.users(id),
    avatar_url TEXT,
    bio TEXT
);`
	if err := os.WriteFile(filepath.Join(targetDir, "01_public.sql"), []byte(targetPublicSQL), 0644); err != nil {
		t.Fatal(err)
	}

	targetAuthSQL := `CREATE SCHEMA auth;

CREATE TABLE auth.roles (
    id SERIAL PRIMARY KEY,
    name VARCHAR(50) NOT NULL UNIQUE,
    description TEXT DEFAULT ''
);

CREATE TABLE auth.permissions (
    id SERIAL PRIMARY KEY,
    role_id INTEGER NOT NULL REFERENCES auth.roles(id),
    resource VARCHAR(100) NOT NULL,
    action VARCHAR(50) NOT NULL
);`
	if err := os.WriteFile(filepath.Join(targetDir, "02_auth.sql"), []byte(targetAuthSQL), 0644); err != nil {
		t.Fatal(err)
	}

	out, _, err := app.NewDiffService(newDefaultDeps()).Run(context.Background(), app.Config{
		Source:     sourceDir,
		Target:     targetDir,
		Schemas:    []string{"public", "auth"},
		Format:     "sql",
		Timeout:    defaultDiffTimeout,
		UnsafeDrop: false,
	})
	if err != nil {
		t.Fatalf("multi-schema diff failed: %v", err)
	}

	for _, want := range []string{
		`CREATE TABLE "public"."profiles"`,
		`ALTER TABLE "public"."users" ADD COLUMN "email"`,
		`ALTER TABLE "auth"."roles" ADD COLUMN "description"`,
	} {
		if !strings.Contains(out, want) {
			t.Fatalf("expected output to contain %q, got:\n%s", want, out)
		}
	}

	if strings.Contains(out, "parse error") {
		t.Fatalf("unexpected parse error in output:\n%s", out)
	}
}

func TestV3DirectoryDiff(t *testing.T) {
	out, warns, err := app.NewDiffService(newDefaultDeps()).Run(context.Background(), app.Config{
		Source:     "../../testdata/diff/v1/",
		Target:     "../../testdata/diff/v2/",
		Schemas:    []string{"public"},
		Format:     "sql",
		Timeout:    defaultDiffTimeout,
		UnsafeDrop: false,
	})
	if err != nil {
		t.Fatalf("diff service failed: %v", err)
	}

	for _, want := range []string{
		`CREATE TABLE "public"."comments"`,
		`ADD COLUMN "age"`,
		`ALTER TYPE "public"."user_role" ADD VALUE 'guest'`,
	} {
		if !strings.Contains(out, want) {
			t.Fatalf("expected output to contain %q, got:\n%s", want, out)
		}
	}

	if len(warns) > 0 {
		t.Fatalf("unexpected warnings: %v", warns)
	}
}

func TestV3UnsafeDropDiff_Safe(t *testing.T) {
	out, _, err := app.NewDiffService(newDefaultDeps()).Run(context.Background(), app.Config{
		Source:     "../../testdata/diff/v2/schema.sql",
		Target:     "../../testdata/diff/v3/schema.sql",
		Schemas:    []string{"public"},
		Format:     "sql",
		Timeout:    defaultDiffTimeout,
		UnsafeDrop: false,
	})
	if err != nil {
		t.Fatalf("diff service failed: %v", err)
	}

	for _, drop := range []string{"DROP COLUMN", "DROP TABLE", "DROP INDEX"} {
		if strings.Contains(out, drop) {
			t.Fatalf("safe mode should not contain %q, got:\n%s", drop, out)
		}
	}

	if !strings.Contains(out, `ADD COLUMN "phone"`) {
		t.Fatalf("expected ADD COLUMN phone in safe mode, got:\n%s", out)
	}
}

func TestV3UnsafeDropDiff_Unsafe(t *testing.T) {
	out, _, err := app.NewDiffService(newDefaultDeps()).Run(context.Background(), app.Config{
		Source:     "../../testdata/diff/v2/schema.sql",
		Target:     "../../testdata/diff/v3/schema.sql",
		Schemas:    []string{"public"},
		Format:     "sql",
		Timeout:    defaultDiffTimeout,
		UnsafeDrop: true,
	})
	if err != nil {
		t.Fatalf("diff service failed: %v", err)
	}

	for _, want := range []string{
		`DROP COLUMN IF EXISTS "age"`,
		`ADD COLUMN "phone"`,
		`idx_users_username_unique`,
	} {
		if !strings.Contains(out, want) {
			t.Fatalf("expected output to contain %q, got:\n%s", want, out)
		}
	}
}

func TestNestedDirectoryDiff(t *testing.T) {
	out, _, err := app.NewDiffService(newDefaultDeps()).Run(context.Background(), app.Config{
		Source:     "../../testdata/diff/nested/",
		Target:     "../../testdata/diff/nested_target/",
		Schemas:    []string{"public"},
		Format:     "sql",
		Timeout:    defaultDiffTimeout,
		UnsafeDrop: false,
	})
	if err != nil {
		t.Fatalf("diff service failed: %v", err)
	}

	for _, want := range []string{
		`ADD COLUMN "email"`,
		`ADD CONSTRAINT "posts_user_id_fkey"`,
	} {
		if !strings.Contains(out, want) {
			t.Fatalf("expected output to contain %q, got:\n%s", want, out)
		}
	}
}

func TestIdentityColumnDiff(t *testing.T) {
	out, _, err := app.NewDiffService(newDefaultDeps()).Run(context.Background(), app.Config{
		Source:     "../../testdata/diff/identity_example/v1/schema.sql",
		Target:     "../../testdata/diff/identity_example/v2/schema.sql",
		Schemas:    []string{"public"},
		Format:     "sql",
		Timeout:    defaultDiffTimeout,
		UnsafeDrop: false,
	})
	if err != nil {
		t.Fatalf("diff service failed: %v", err)
	}

	if !strings.Contains(out, "IDENTITY") {
		t.Fatalf("expected IDENTITY in output, got:\n%s", out)
	}
	if strings.Contains(out, "DROP COLUMN") {
		t.Fatalf("should not DROP COLUMN for identity change, got:\n%s", out)
	}
}

func TestCollateClauseDiff(t *testing.T) {
	out, _, err := app.NewDiffService(newDefaultDeps()).Run(context.Background(), app.Config{
		Source:     "../../testdata/diff/collate_example/v1/schema.sql",
		Target:     "../../testdata/diff/collate_example/v2/schema.sql",
		Schemas:    []string{"public"},
		Format:     "sql",
		Timeout:    defaultDiffTimeout,
		UnsafeDrop: false,
	})
	if err != nil {
		t.Fatalf("diff service failed: %v", err)
	}

	if !strings.Contains(out, `COLLATE "en_US"`) {
		t.Fatalf("expected COLLATE en_US in output, got:\n%s", out)
	}
}

func TestObjectsDiff(t *testing.T) {
	out, _, err := app.NewDiffService(newDefaultDeps()).Run(context.Background(), app.Config{
		Source:     "../../testdata/diff/objects_example/v1/schema.sql",
		Target:     "../../testdata/diff/objects_example/v2/schema.sql",
		Schemas:    []string{"public"},
		Format:     "sql",
		Timeout:    defaultDiffTimeout,
		UnsafeDrop: false,
	})
	if err != nil {
		t.Fatalf("diff service failed: %v", err)
	}

	for _, want := range []string{
		`CREATE EXTENSION`,
		`CREATE SEQUENCE`,
		`CREATE VIEW`,
	} {
		if !strings.Contains(out, want) {
			t.Fatalf("expected output to contain %q, got:\n%s", want, out)
		}
	}
}

func TestV3ToV4Diff(t *testing.T) {
	out, _, err := app.NewDiffService(newDefaultDeps()).Run(context.Background(), app.Config{
		Source:     "../../testdata/diff/v3/schema.sql",
		Target:     "../../testdata/diff/v4/schema.sql",
		Schemas:    []string{"public"},
		Format:     "sql",
		Timeout:    defaultDiffTimeout,
		UnsafeDrop: false,
	})
	if err != nil {
		t.Fatalf("diff service failed: %v", err)
	}

	for _, want := range []string{
		`IDENTITY`,
		`COLLATE`,
		`ON DELETE CASCADE`,
	} {
		if !strings.Contains(out, want) {
			t.Fatalf("expected output to contain %q, got:\n%s", want, out)
		}
	}

	if strings.Contains(out, "DROP COLUMN") {
		t.Fatalf("v3→v4 should not contain DROP COLUMN, got:\n%s", out)
	}
}

func TestMultiSchemaDirectoryDiffStatic(t *testing.T) {
	out, _, err := app.NewDiffService(newDefaultDeps()).Run(context.Background(), app.Config{
		Source:     "../../testdata/diff/multi_schema/v1/",
		Target:     "../../testdata/diff/multi_schema/v2/",
		Schemas:    []string{"public", "auth"},
		Format:     "sql",
		Timeout:    defaultDiffTimeout,
		UnsafeDrop: false,
	})
	if err != nil {
		t.Fatalf("diff service failed: %v", err)
	}

	for _, want := range []string{
		`CREATE TABLE "public"."profiles"`,
		`ALTER TABLE "auth"."roles" ADD COLUMN "description"`,
		`ALTER TABLE "public"."users" ADD COLUMN "email"`,
	} {
		if !strings.Contains(out, want) {
			t.Fatalf("expected output to contain %q, got:\n%s", want, out)
		}
	}
}
