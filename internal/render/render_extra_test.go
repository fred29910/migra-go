package render

import (
	"strings"
	"testing"

	"github.com/fred29910/migra-go/internal/diff"
	"github.com/fred29910/migra-go/internal/model"
)

// Tests for uncovered functions: renderSetNotNull, renderDropNotNull,
// renderDropConstraint, renderAddEnumType, renderDropEnumType,
// renderDropSequence, renderDropTable, renderReplaceView

func TestRenderSetNotNull(t *testing.T) {
	r := NewRenderer()
	op := diff.NewSetNotNullOp("public", "users", "email")
	sql := r.Render(op)

	want := `ALTER TABLE "public"."users" ALTER COLUMN "email" SET NOT NULL;`
	if !strings.Contains(sql, want) {
		t.Errorf("expected SQL to contain %q, got:\n%s", want, sql)
	}
	if !strings.Contains(sql, "set_not_null") {
		t.Errorf("expected operation tag set_not_null, got:\n%s", sql)
	}
	if !strings.Contains(sql, "risk:low") {
		t.Errorf("expected risk:low, got:\n%s", sql)
	}
}

func TestRenderDropNotNull(t *testing.T) {
	r := NewRenderer()
	op := diff.NewDropNotNullOp("public", "users", "email")
	sql := r.Render(op)

	want := `ALTER TABLE "public"."users" ALTER COLUMN "email" DROP NOT NULL;`
	if !strings.Contains(sql, want) {
		t.Errorf("expected SQL to contain %q, got:\n%s", want, sql)
	}
	if !strings.Contains(sql, "drop_not_null") {
		t.Errorf("expected operation tag drop_not_null, got:\n%s", sql)
	}
	if !strings.Contains(sql, "risk:low") {
		t.Errorf("expected risk:low, got:\n%s", sql)
	}
}

func TestRenderDropConstraint(t *testing.T) {
	r := NewRenderer()

	t.Run("with IF EXISTS", func(t *testing.T) {
		op := diff.NewDropConstraintOp("public", "users", "users_email_key")
		sql := r.Render(op)

		want := `ALTER TABLE "public"."users" DROP CONSTRAINT IF EXISTS "users_email_key";`
		if !strings.Contains(sql, want) {
			t.Errorf("expected SQL to contain %q, got:\n%s", want, sql)
		}
		if !strings.Contains(sql, "drop_constraint") {
			t.Errorf("expected operation tag drop_constraint, got:\n%s", sql)
		}
	})

	t.Run("without IF EXISTS", func(t *testing.T) {
		r2 := &Renderer{useIfExists: false}
		op := diff.NewDropConstraintOp("public", "users", "users_email_key")
		sql := r2.Render(op)

		want := `ALTER TABLE "public"."users" DROP CONSTRAINT "users_email_key";`
		if !strings.Contains(sql, want) {
			t.Errorf("expected SQL to contain %q, got:\n%s", want, sql)
		}
		if strings.Contains(sql, "IF EXISTS") {
			t.Errorf("expected no IF EXISTS when useIfExists=false, got:\n%s", sql)
		}
	})
}

func TestRenderAddEnumType(t *testing.T) {
	r := NewRenderer()
	op := diff.NewAddEnumTypeOp("public", &model.EnumType{
		Name:   "user_role",
		Labels: []string{"admin", "user", "guest"},
	})
	sql := r.Render(op)

	want := `CREATE TYPE "public"."user_role" AS ENUM ('admin', 'user', 'guest');`
	if !strings.Contains(sql, want) {
		t.Errorf("expected SQL to contain %q, got:\n%s", want, sql)
	}
	if !strings.Contains(sql, "add_enum_type") {
		t.Errorf("expected operation tag add_enum_type, got:\n%s", sql)
	}
	if !strings.Contains(sql, "risk:low") {
		t.Errorf("expected risk:low, got:\n%s", sql)
	}
}

func TestRenderAddEnumType_SingleLabel(t *testing.T) {
	r := NewRenderer()
	op := diff.NewAddEnumTypeOp("public", &model.EnumType{
		Name:   "status",
		Labels: []string{"active"},
	})
	sql := r.Render(op)

	want := `CREATE TYPE "public"."status" AS ENUM ('active');`
	if !strings.Contains(sql, want) {
		t.Errorf("expected SQL to contain %q, got:\n%s", want, sql)
	}
}

func TestRenderAddEnumType_WithSpecialChars(t *testing.T) {
	r := NewRenderer()
	op := diff.NewAddEnumTypeOp("public", &model.EnumType{
		Name:   "mood",
		Labels: []string{"it's good", "it's bad"},
	})
	sql := r.Render(op)

	// Single quotes should be escaped
	want := `CREATE TYPE "public"."mood" AS ENUM ('it''s good', 'it''s bad');`
	if !strings.Contains(sql, want) {
		t.Errorf("expected SQL to contain %q, got:\n%s", want, sql)
	}
}

func TestRenderDropEnumType(t *testing.T) {
	r := NewRenderer()

	t.Run("with IF EXISTS", func(t *testing.T) {
		op := diff.NewDropEnumTypeOp("public", "user_role")
		sql := r.Render(op)

		want := `DROP TYPE IF EXISTS "public"."user_role";`
		if !strings.Contains(sql, want) {
			t.Errorf("expected SQL to contain %q, got:\n%s", want, sql)
		}
		if !strings.Contains(sql, "drop_enum_type") {
			t.Errorf("expected operation tag drop_enum_type, got:\n%s", sql)
		}
		if !strings.Contains(sql, "risk:high") {
			t.Errorf("expected risk:high, got:\n%s", sql)
		}
	})

	t.Run("without IF EXISTS", func(t *testing.T) {
		r2 := &Renderer{useIfExists: false}
		op := diff.NewDropEnumTypeOp("public", "user_role")
		sql := r2.Render(op)

		want := `DROP TYPE "public"."user_role";`
		if !strings.Contains(sql, want) {
			t.Errorf("expected SQL to contain %q, got:\n%s", want, sql)
		}
		if strings.Contains(sql, "IF EXISTS") {
			t.Errorf("expected no IF EXISTS when useIfExists=false, got:\n%s", sql)
		}
	})
}

func TestRenderDropSequence(t *testing.T) {
	r := NewRenderer()

	t.Run("with IF EXISTS", func(t *testing.T) {
		op := diff.NewDropSequenceOp("public", "invoice_seq")
		sql := r.Render(op)

		want := `DROP SEQUENCE IF EXISTS "public"."invoice_seq";`
		if !strings.Contains(sql, want) {
			t.Errorf("expected SQL to contain %q, got:\n%s", want, sql)
		}
		if !strings.Contains(sql, "drop_sequence") {
			t.Errorf("expected operation tag drop_sequence, got:\n%s", sql)
		}
		if !strings.Contains(sql, "risk:high") {
			t.Errorf("expected risk:high, got:\n%s", sql)
		}
	})

	t.Run("without IF EXISTS", func(t *testing.T) {
		r2 := &Renderer{useIfExists: false}
		op := diff.NewDropSequenceOp("public", "invoice_seq")
		sql := r2.Render(op)

		want := `DROP SEQUENCE "public"."invoice_seq";`
		if !strings.Contains(sql, want) {
			t.Errorf("expected SQL to contain %q, got:\n%s", want, sql)
		}
		if strings.Contains(sql, "IF EXISTS") {
			t.Errorf("expected no IF EXISTS when useIfExists=false, got:\n%s", sql)
		}
	})
}

func TestRenderDropTable(t *testing.T) {
	r := NewRenderer()

	t.Run("with IF EXISTS", func(t *testing.T) {
		op := diff.NewDropTableOp("public", "old_table")
		sql := r.Render(op)

		want := `DROP TABLE IF EXISTS "public"."old_table";`
		if !strings.Contains(sql, want) {
			t.Errorf("expected SQL to contain %q, got:\n%s", want, sql)
		}
		if !strings.Contains(sql, "drop_table") {
			t.Errorf("expected operation tag drop_table, got:\n%s", sql)
		}
		if !strings.Contains(sql, "risk:high") {
			t.Errorf("expected risk:high, got:\n%s", sql)
		}
	})

	t.Run("without IF EXISTS", func(t *testing.T) {
		r2 := &Renderer{useIfExists: false}
		op := diff.NewDropTableOp("public", "old_table")
		sql := r2.Render(op)

		want := `DROP TABLE "public"."old_table";`
		if !strings.Contains(sql, want) {
			t.Errorf("expected SQL to contain %q, got:\n%s", want, sql)
		}
		if strings.Contains(sql, "IF EXISTS") {
			t.Errorf("expected no IF EXISTS when useIfExists=false, got:\n%s", sql)
		}
	})
}

func TestRenderReplaceView(t *testing.T) {
	r := NewRenderer()
	op := diff.NewReplaceViewOp("public", &model.View{
		Name:       "active_users",
		Definition: "SELECT id FROM users WHERE active = true",
	})
	sql := r.Render(op)

	want := `CREATE OR REPLACE VIEW "public"."active_users" AS SELECT id FROM users WHERE active = true;`
	if !strings.Contains(sql, want) {
		t.Errorf("expected SQL to contain %q, got:\n%s", want, sql)
	}
	if !strings.Contains(sql, "replace_view") {
		t.Errorf("expected operation tag replace_view, got:\n%s", sql)
	}
	if !strings.Contains(sql, "risk:medium") {
		t.Errorf("expected risk:medium, got:\n%s", sql)
	}
}

// Additional edge case tests for better coverage

func TestRenderOutput_UnsupportedFormat(t *testing.T) {
	r := NewRenderer()
	ops := []diff.Operation{diff.NewDropIndexOp("public", "idx_a")}

	_, err := r.RenderOutput(ops, "yaml")
	if err == nil {
		t.Fatal("expected error for unsupported format")
	}
	if !strings.Contains(err.Error(), "unsupported format") {
		t.Errorf("expected 'unsupported format' error, got: %v", err)
	}
}

func TestRenderAll_EmptyOps(t *testing.T) {
	r := NewRenderer()
	got := r.RenderAll(nil)
	want := "-- No changes detected"
	if got != want {
		t.Errorf("expected %q, got %q", want, got)
	}

	got = r.RenderAll([]diff.Operation{})
	if got != want {
		t.Errorf("expected %q for empty slice, got %q", want, got)
	}
}

func TestRenderAll_MultipleOps(t *testing.T) {
	r := NewRenderer()
	ops := []diff.Operation{
		diff.NewDropIndexOp("public", "idx_a"),
		diff.NewDropIndexOp("public", "idx_b"),
	}
	got := r.RenderAll(ops)

	if !strings.Contains(got, "-- Begin Diff") {
		t.Errorf("expected '-- Begin Diff' in output, got:\n%s", got)
	}
	if !strings.Contains(got, "-- End Diff") {
		t.Errorf("expected '-- End Diff' in output, got:\n%s", got)
	}
	if !strings.Contains(got, "idx_a") {
		t.Errorf("expected 'idx_a' in output, got:\n%s", got)
	}
	if !strings.Contains(got, "idx_b") {
		t.Errorf("expected 'idx_b' in output, got:\n%s", got)
	}
}

func TestRenderAll_WithEmptyRenderResult(t *testing.T) {
	r := NewRenderer()
	// Create an operation that renders to empty string (constraint with no definition)
	op := diff.NewAddConstraintOp("public", "users", &model.Constraint{
		Name: "empty_def",
		Type: "unknown_type",
	})
	// This should be skipped in RenderAll
	ops := []diff.Operation{op}
	got := r.RenderAll(ops)

	// Should return "No changes detected" since the constraint renders to empty
	want := "-- No changes detected"
	if got != want {
		t.Errorf("expected %q, got %q", want, got)
	}
}

func TestRender_UnknownOperation(t *testing.T) {
	r := NewRenderer()
	// mockOperation.RenderString returns empty, so RenderAll would skip it
	op := &mockOperation{}
	got := r.Render(op)

	// mockOperation.RenderString returns empty string
	if got != "" {
		t.Errorf("expected empty string for mock operation, got %q", got)
	}
}

// mockOperation is a test helper that implements diff.Operation
type mockOperation struct{}

func (m *mockOperation) Kind() diff.Kind                            { return "mock" }
func (m *mockOperation) ObjectKey() model.ObjectKey                 { return model.ObjectKey{} }
func (m *mockOperation) DependsOn() []model.ObjectKey               { return nil }
func (m *mockOperation) IsDestructive() bool                        { return false }
func (m *mockOperation) RenderString(ctx diff.RenderContext) string { return "" }

func TestRenderConstraintDefinition_NilConstraint(t *testing.T) {
	got := renderConstraintDefinition(nil)
	if got != "" {
		t.Errorf("expected empty string for nil constraint, got: %q", got)
	}
}

func TestRenderConstraintDefinition_UnknownType(t *testing.T) {
	c := &model.Constraint{
		Name: "test",
		Type: "unknown_type",
	}
	got := renderConstraintDefinition(c)
	if got != "" {
		t.Errorf("expected empty string for unknown type, got: %q", got)
	}
}

func TestRenderConstraintDefinition_WithDefinition(t *testing.T) {
	c := &model.Constraint{
		Name:       "test",
		Type:       "custom",
		Definition: "CUSTOM CONSTRAINT DEFINITION",
	}
	got := renderConstraintDefinition(c)
	if got != "CUSTOM CONSTRAINT DEFINITION" {
		t.Errorf("expected custom definition, got: %q", got)
	}
}

func TestRenderConstraintDefinition_CheckNoExpression(t *testing.T) {
	c := &model.Constraint{
		Name: "test_check",
		Type: "check",
		// Expression is empty
	}
	got := renderConstraintDefinition(c)
	if got != "" {
		t.Errorf("expected empty string for check without expression, got: %q", got)
	}
}

func TestRenderConstraint_NilConstraint(t *testing.T) {
	got := renderConstraint(nil)
	if got != "" {
		t.Errorf("expected empty string for nil constraint, got: %q", got)
	}
}

func TestIfExistsPrefix(t *testing.T) {
	tests := []struct {
		name        string
		useIfExists bool
		want        string
	}{
		{"with IF EXISTS", true, "IF EXISTS "},
		{"without IF EXISTS", false, ""},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := ifExistsPrefix(tt.useIfExists)
			if got != tt.want {
				t.Errorf("ifExistsPrefix(%v) = %q, want %q", tt.useIfExists, got, tt.want)
			}
		})
	}
}

func TestQuoteString(t *testing.T) {
	tests := []struct {
		input string
		want  string
	}{
		{"hello", "'hello'"},
		{"it's", "'it''s'"},
		{"", "''"},
		{"a'b'c", "'a''b''c'"},
	}
	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			got := quoteString(tt.input)
			if got != tt.want {
				t.Errorf("quoteString(%q) = %q, want %q", tt.input, got, tt.want)
			}
		})
	}
}

func TestRenderIndexElem_ExprOnly(t *testing.T) {
	elem := model.IndexElem{
		Expr: "lower(name)",
	}
	got := renderIndexElem(elem)
	want := "(lower(name))"
	if got != want {
		t.Errorf("expected %q, got %q", want, got)
	}
}

func TestRenderIndexElem_Empty(t *testing.T) {
	elem := model.IndexElem{}
	got := renderIndexElem(elem)
	if got != "" {
		t.Errorf("expected empty string, got: %q", got)
	}
}

func TestRenderIndexElem_AllOptions(t *testing.T) {
	elem := model.IndexElem{
		Name:          "email",
		Collation:     "C",
		Opclass:       "text_pattern_ops",
		Ordering:      "DESC",
		NullsOrdering: "FIRST",
	}
	got := renderIndexElem(elem)
	want := `"email" COLLATE "C" text_pattern_ops DESC NULLS FIRST`
	if got != want {
		t.Errorf("expected %q, got %q", want, got)
	}
}

func TestRenderIndexElem_ExprWithOptions(t *testing.T) {
	elem := model.IndexElem{
		Expr:          "lower(name)",
		Collation:     "C",
		Ordering:      "ASC",
		NullsOrdering: "LAST",
	}
	got := renderIndexElem(elem)
	want := "(lower(name)) COLLATE \"C\" ASC NULLS LAST"
	if got != want {
		t.Errorf("expected %q, got %q", want, got)
	}
}

func TestRenderCreateSequence_AllOptions(t *testing.T) {
	r := NewRenderer()
	op := diff.NewCreateSequenceOp("public", &model.Sequence{
		Name:        "order_seq",
		DataType:    "bigint",
		StartValue:  1000,
		IncrementBy: 5,
		MinValue:    1,
		MaxValue:    999999,
		CacheSize:   10,
		Cycle:       true,
	})
	sql := r.Render(op)

	for _, want := range []string{
		`CREATE SEQUENCE "public"."order_seq"`,
		`AS bigint`,
		`START WITH 1000`,
		`INCREMENT BY 5`,
		`MINVALUE 1`,
		`MAXVALUE 999999`,
		`CACHE 10`,
		`CYCLE`,
	} {
		if !strings.Contains(sql, want) {
			t.Errorf("expected SQL to contain %q, got:\n%s", want, sql)
		}
	}
}

func TestRenderCreateSequence_DefaultCache(t *testing.T) {
	r := NewRenderer()
	op := diff.NewCreateSequenceOp("public", &model.Sequence{
		Name:      "simple_seq",
		CacheSize: 1, // Default cache size should not be rendered
	})
	sql := r.Render(op)

	if strings.Contains(sql, "CACHE") {
		t.Errorf("expected no CACHE for default cache size 1, got:\n%s", sql)
	}
}

func TestRenderAlterSequence_AllChanged(t *testing.T) {
	r := NewRenderer()
	from := &model.Sequence{
		Name:        "order_seq",
		DataType:    "integer",
		StartValue:  1,
		IncrementBy: 1,
		MinValue:    1,
		MaxValue:    1000,
		CacheSize:   1,
		Cycle:       false,
	}
	to := &model.Sequence{
		Name:        "order_seq",
		DataType:    "bigint",
		StartValue:  100,
		IncrementBy: 5,
		MinValue:    10,
		MaxValue:    999999,
		CacheSize:   20,
		Cycle:       true,
	}
	op := diff.NewAlterSequenceOp("public", from, to)
	sql := r.Render(op)

	for _, want := range []string{
		`ALTER SEQUENCE "public"."order_seq"`,
		`AS bigint`,
		`START WITH 100`,
		`INCREMENT BY 5`,
		`MINVALUE 10`,
		`MAXVALUE 999999`,
		`CACHE 20`,
		`CYCLE`,
	} {
		if !strings.Contains(sql, want) {
			t.Errorf("expected SQL to contain %q, got:\n%s", want, sql)
		}
	}
}

func TestRenderAlterSequence_NoChanges(t *testing.T) {
	r := NewRenderer()
	seq := &model.Sequence{
		Name:        "order_seq",
		DataType:    "bigint",
		StartValue:  100,
		IncrementBy: 5,
		MinValue:    10,
		MaxValue:    999999,
		CacheSize:   20,
		Cycle:       true,
	}
	op := diff.NewAlterSequenceOp("public", seq, seq)
	sql := r.Render(op)

	// Should only have the ALTER SEQUENCE prefix with no changes
	want := `ALTER SEQUENCE "public"."order_seq";`
	if !strings.Contains(sql, want) {
		t.Errorf("expected SQL to contain %q, got:\n%s", want, sql)
	}
}

func TestRenderCreateExtension_WithVersion(t *testing.T) {
	r := NewRenderer()
	op := diff.NewCreateExtensionOp("public", &model.Extension{
		Name:    "pgcrypto",
		Version: "1.3",
	})
	sql := r.Render(op)

	want := `CREATE EXTENSION IF NOT EXISTS "pgcrypto" WITH VERSION '1.3';`
	if !strings.Contains(sql, want) {
		t.Errorf("expected SQL to contain %q, got:\n%s", want, sql)
	}
}

func TestRenderCreateExtension_WithoutVersion(t *testing.T) {
	r := NewRenderer()
	op := diff.NewCreateExtensionOp("public", &model.Extension{
		Name: "uuid-ossp",
	})
	sql := r.Render(op)

	want := `CREATE EXTENSION IF NOT EXISTS "uuid-ossp";`
	if !strings.Contains(sql, want) {
		t.Errorf("expected SQL to contain %q, got:\n%s", want, sql)
	}
	if strings.Contains(sql, "VERSION") {
		t.Errorf("expected no VERSION when not specified, got:\n%s", sql)
	}
}

func TestRenderAddColumn_AllOptions(t *testing.T) {
	r := NewRenderer()
	defaultExpr := "gen_random_uuid()"
	col := &model.Column{
		Name:         "id",
		DataType:     "uuid",
		IsNullable:   false,
		DefaultExpr:  &defaultExpr,
		IsIdentity:   true,
		IdentityKind: "BY DEFAULT",
		Collation:    "C",
	}
	op := diff.NewAddColumnOp("public", "users", col)
	sql := r.Render(op)

	for _, want := range []string{
		`ALTER TABLE "public"."users" ADD COLUMN "id" uuid`,
		`GENERATED BY DEFAULT AS IDENTITY`,
		`COLLATE "C"`,
		`NOT NULL`,
		`DEFAULT gen_random_uuid()`,
	} {
		if !strings.Contains(sql, want) {
			t.Errorf("expected SQL to contain %q, got:\n%s", want, sql)
		}
	}
}

func TestRenderAddColumn_NullableNoDefault(t *testing.T) {
	r := NewRenderer()
	col := &model.Column{
		Name:       "description",
		DataType:   "text",
		IsNullable: true,
	}
	op := diff.NewAddColumnOp("public", "users", col)
	sql := r.Render(op)

	want := `ALTER TABLE "public"."users" ADD COLUMN "description" text;`
	if !strings.Contains(sql, want) {
		t.Errorf("expected SQL to contain %q, got:\n%s", want, sql)
	}
	if strings.Contains(sql, "NOT NULL") {
		t.Errorf("expected no NOT NULL for nullable column, got:\n%s", sql)
	}
	if strings.Contains(sql, "DEFAULT") {
		t.Errorf("expected no DEFAULT for column without default, got:\n%s", sql)
	}
}

func TestRenderAddTable_EmptyColumns(t *testing.T) {
	r := NewRenderer()
	table := model.NewTable("public", "empty_table")
	// No columns added
	sql := r.Render(diff.NewAddTableOp("public", "empty_table", table))

	// Verify the table name is present
	if !strings.Contains(sql, `"public"."empty_table"`) {
		t.Errorf("expected table name in SQL, got:\n%s", sql)
	}
	// Verify the comment prefix is present
	if !strings.Contains(sql, "-- op: add_table risk:low") {
		t.Errorf("expected comment prefix, got:\n%s", sql)
	}
	// Verify it's a CREATE TABLE statement
	if !strings.Contains(sql, "CREATE TABLE") {
		t.Errorf("expected CREATE TABLE in SQL, got:\n%s", sql)
	}
}

func TestRenderAddTable_MultipleConstraints(t *testing.T) {
	r := NewRenderer()
	table := model.NewTable("public", "orders")
	table.AddColumn(&model.Column{Name: "id", DataType: "integer", IsNullable: false})
	table.AddColumn(&model.Column{Name: "user_id", DataType: "integer", IsNullable: false})
	table.AddColumn(&model.Column{Name: "email", DataType: "text", IsNullable: false})
	table.PrimaryKey = &model.PrimaryKey{Name: "orders_pkey", Columns: []string{"id"}}
	table.Constraints = map[string]*model.Constraint{
		"orders_user_id_fkey": {
			Name:       "orders_user_id_fkey",
			Type:       "foreign_key",
			Columns:    []string{"user_id"},
			RefSchema:  "public",
			RefTable:   "users",
			RefColumns: []string{"id"},
			OnDelete:   "CASCADE",
		},
		"orders_email_key": {
			Name:    "orders_email_key",
			Type:    "unique",
			Columns: []string{"email"},
		},
	}

	sql := r.Render(diff.NewAddTableOp("public", "orders", table))

	for _, want := range []string{
		`CONSTRAINT "orders_pkey" PRIMARY KEY ("id")`,
		`CONSTRAINT "orders_user_id_fkey" FOREIGN KEY ("user_id") REFERENCES "public"."users" ("id") ON DELETE CASCADE`,
		`CONSTRAINT "orders_email_key" UNIQUE ("email")`,
	} {
		if !strings.Contains(sql, want) {
			t.Errorf("expected SQL to contain %q, got:\n%s", want, sql)
		}
	}
}

func TestRenderJSON_MultipleOps(t *testing.T) {
	r := NewRenderer()
	ops := []diff.Operation{
		diff.NewDropIndexOp("public", "idx_a"),
		diff.NewDropTableOp("public", "old_table"),
	}

	json, err := r.RenderOutput(ops, "json")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	for _, want := range []string{
		`"kind"`,
		`"object_key"`,
		`"destructive"`,
		`"sql"`,
		`public.idx_a`,
		`public.old_table`,
	} {
		if !strings.Contains(json, want) {
			t.Errorf("expected JSON to contain %q, got:\n%s", want, json)
		}
	}
}

func TestRender_DropIndex(t *testing.T) {
	r := NewRenderer()
	op := diff.NewDropIndexOp("public", "idx_test")

	got := r.Render(op)
	if got == "" {
		t.Error("expected non-empty render result for DropIndexOp")
	}
}

func TestNewRenderer_DefaultUseIfExists(t *testing.T) {
	r := NewRenderer()
	if !r.useIfExists {
		t.Error("expected useIfExists to be true by default")
	}
}
