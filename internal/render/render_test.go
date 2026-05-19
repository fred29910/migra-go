package render

import (
	"strings"
	"testing"

	"github.com/fred29910/migra-go/internal/diff"
	"github.com/fred29910/migra-go/internal/model"
)

func TestRenderer_RenderAll_IdempotentAcrossCalls(t *testing.T) {
	r := NewRenderer()
	ops := []diff.Operation{diff.NewDropIndexOp("public", "idx_a")}
	got1 := r.RenderAll(ops)
	got2 := r.RenderAll(ops)
	if got1 != got2 {
		t.Fatalf("expected stable output, got1=%q got2=%q", got1, got2)
	}
}

func TestRenderOutput_SupportsSQLAndJSON(t *testing.T) {
	ops := []diff.Operation{diff.NewDropIndexOp("public", "idx_a")}
	r := NewRenderer()
	sql, err := r.RenderOutput(ops, "sql")
	if err != nil || !strings.Contains(sql, "DROP INDEX") {
		t.Fatalf("unexpected sql render: %v %q", err, sql)
	}
	js, err := r.RenderOutput(ops, "json")
	if err != nil {
		t.Fatalf("unexpected json render error: %v", err)
	}
	for _, want := range []string{`"kind"`, `"object_key"`, `"sql"`} {
		if !strings.Contains(js, want) {
			t.Fatalf("json output missing field %q, got:\n%s", want, js)
		}
	}
	if !strings.Contains(js, "public.idx_a") {
		t.Fatalf("json output missing object_key value 'public.idx_a', got:\n%s", js)
	}
}

func TestRenderAddTable_WithDefaultsAndConstraints(t *testing.T) {
	defaultExpr := "now()"
	table := model.NewTable("public", "comments")
	table.AddColumn(&model.Column{Name: "id", DataType: "serial", IsNullable: false})
	table.AddColumn(&model.Column{Name: "post_id", DataType: "integer", IsNullable: false})
	table.AddColumn(&model.Column{Name: "created_at", DataType: "timestamp", IsNullable: true, DefaultExpr: &defaultExpr})
	table.PrimaryKey = &model.PrimaryKey{Name: "comments_pkey", Columns: []string{"id"}}
	table.Constraints["comments_post_id_fkey"] = &model.Constraint{
		Name:       "comments_post_id_fkey",
		Type:       "foreign_key",
		Table:      "comments",
		Columns:    []string{"post_id"},
		RefSchema:  "public",
		RefTable:   "posts",
		RefColumns: []string{"id"},
	}

	sql := NewRenderer().Render(diff.NewAddTableOp("public", "comments", table))
	for _, want := range []string{
		`"created_at" timestamp DEFAULT now()`,
		`CONSTRAINT "comments_pkey" PRIMARY KEY ("id")`,
		`CONSTRAINT "comments_post_id_fkey" FOREIGN KEY ("post_id") REFERENCES "public"."posts" ("id")`,
	} {
		if !strings.Contains(sql, want) {
			t.Errorf("expected SQL to contain %q, got:\n%s", want, sql)
		}
	}
}

func TestRenderNewOperations(t *testing.T) {
	r := NewRenderer()
	cases := map[string]diff.Operation{
		`ALTER TYPE "public"."user_role" ADD VALUE 'guest';`:                        diff.NewAddEnumLabelOp("public", "user_role", "guest"),
		`ALTER TABLE "public"."users" ALTER COLUMN "created_at" SET DEFAULT now();`: diff.NewSetDefaultOp("public", "users", "created_at", "now()"),
		`ALTER TABLE "public"."users" ALTER COLUMN "created_at" DROP DEFAULT;`:      diff.NewDropDefaultOp("public", "users", "created_at"),
		`ALTER TABLE "public"."users" DROP COLUMN IF EXISTS "old_col";`:             diff.NewDropColumnOp("public", "users", "old_col"),
		`CREATE SCHEMA IF NOT EXISTS "auth";`:                                       diff.NewCreateSchemaOp("auth"),
		`DROP SCHEMA IF EXISTS "old_schema";`:                                       diff.NewDropSchemaOp("old_schema"),
	}
	for want, op := range cases {
		if got := r.Render(op); !strings.Contains(got, want) {
			t.Errorf("expected %q in %q", want, got)
		}
	}
}

func TestRenderAddConstraint_UsesStructuredConstraint(t *testing.T) {
	r := NewRenderer()
	op := diff.NewAddConstraintOp("public", "comments", &model.Constraint{
		Name:       "comments_post_id_fkey",
		Type:       "foreign_key",
		Table:      "comments",
		Columns:    []string{"post_id"},
		RefSchema:  "public",
		RefTable:   "posts",
		RefColumns: []string{"id"},
	})

	got := r.Render(op)
	want := `ALTER TABLE "public"."comments" ADD CONSTRAINT "comments_post_id_fkey" FOREIGN KEY ("post_id") REFERENCES "public"."posts" ("id");`
	if !strings.Contains(got, want) {
		t.Fatalf("expected %q in %q", want, got)
	}
	if strings.Contains(got, `ADD CONSTRAINT "comments_post_id_fkey" ;`) {
		t.Fatalf("structured constraint rendered as empty definition: %q", got)
	}
}

func TestRenderAddTable_WithCollation(t *testing.T) {
	table := model.NewTable("public", "users")
	table.AddColumn(&model.Column{Name: "id", DataType: "integer", IsNullable: false})
	table.AddColumn(&model.Column{Name: "name", DataType: "text", IsNullable: true, Collation: "en_US.UTF-8"})
	table.AddColumn(&model.Column{Name: "label", DataType: "varchar(50)", IsNullable: false, Collation: "de_DE"})
	table.PrimaryKey = &model.PrimaryKey{Name: "users_pkey", Columns: []string{"id"}}

	sql := NewRenderer().Render(diff.NewAddTableOp("public", "users", table))
	for _, want := range []string{
		`"name" text COLLATE "en_US.UTF-8"`,
		`"label" varchar(50) COLLATE "de_DE" NOT NULL`,
		`"id" integer NOT NULL`,
	} {
		if !strings.Contains(sql, want) {
			t.Errorf("expected SQL to contain %q, got:\n%s", want, sql)
		}
	}
}

func TestRenderAddColumn_WithCollation(t *testing.T) {
	col := &model.Column{Name: "full_name", DataType: "text", IsNullable: true, Collation: "en_US.UTF-8"}
	op := diff.NewAddColumnOp("public", "users", col)

	sql := NewRenderer().Render(op)
	want := `ALTER TABLE "public"."users" ADD COLUMN "full_name" text COLLATE "en_US.UTF-8";`
	if !strings.Contains(sql, want) {
		t.Errorf("expected SQL to contain %q, got:\n%s", want, sql)
	}

	// Also test without collation (no regression)
	col2 := &model.Column{Name: "age", DataType: "integer", IsNullable: false}
	op2 := diff.NewAddColumnOp("public", "users", col2)
	sql2 := NewRenderer().Render(op2)
	if strings.Contains(sql2, "COLLATE") {
		t.Errorf("expected no COLLATE for default collation, got:\n%s", sql2)
	}
}

func TestRenderAlterColumnCollation(t *testing.T) {
	r := NewRenderer()

	t.Run("add collation", func(t *testing.T) {
		op := diff.NewAlterColumnCollationOp("public", "users", "name", "text", "", "en_US.UTF-8")
		sql := r.Render(op)
		want := `ALTER TABLE "public"."users" ALTER COLUMN "name" SET DATA TYPE text COLLATE "en_US.UTF-8";`
		if !strings.Contains(sql, want) {
			t.Errorf("expected SQL to contain %q, got:\n%s", want, sql)
		}
	})

	t.Run("remove collation", func(t *testing.T) {
		op := diff.NewAlterColumnCollationOp("public", "users", "name", "text", "en_US.UTF-8", "")
		sql := r.Render(op)
		if strings.Contains(sql, "COLLATE") {
			t.Errorf("expected no COLLATE when removing collation, got:\n%s", sql)
		}
		if !strings.Contains(sql, "SET DATA TYPE text") {
			t.Errorf("expected SET DATA TYPE, got:\n%s", sql)
		}
	})
}

func TestRenderAddTable_WithCollationDefaultNotNull(t *testing.T) {
	defaultExpr := "current_timestamp"
	table := model.NewTable("public", "logs")
	table.AddColumn(&model.Column{
		Name:        "ts",
		DataType:    "timestamptz",
		IsNullable:  false,
		DefaultExpr: &defaultExpr,
	})
	table.AddColumn(&model.Column{
		Name:       "message",
		DataType:   "text",
		IsNullable: false,
		Collation:  "en_US.UTF-8",
	})

	sql := NewRenderer().Render(diff.NewAddTableOp("public", "logs", table))
	for _, want := range []string{
		`"message" text COLLATE "en_US.UTF-8" NOT NULL`,
		`"ts" timestamptz NOT NULL DEFAULT current_timestamp`,
	} {
		if !strings.Contains(sql, want) {
			t.Errorf("expected SQL to contain %q, got:\n%s", want, sql)
		}
	}
}

func TestRenderForeignKeyCascade(t *testing.T) {
	t.Run("CASCADE ON DELETE", func(t *testing.T) {
		c := &model.Constraint{
			Name:       "fk_user_id",
			Type:       "foreign_key",
			Columns:    []string{"user_id"},
			RefSchema:  "public",
			RefTable:   "users",
			RefColumns: []string{"id"},
			OnDelete:   "CASCADE",
		}
		sql := renderConstraintDefinition(c)
		if !strings.Contains(sql, `ON DELETE CASCADE`) {
			t.Fatalf("expected ON DELETE CASCADE in %q", sql)
		}
	})

	t.Run("SET NULL ON UPDATE", func(t *testing.T) {
		c := &model.Constraint{
			Name:       "fk_user_id",
			Type:       "foreign_key",
			Columns:    []string{"user_id"},
			RefSchema:  "public",
			RefTable:   "users",
			RefColumns: []string{"id"},
			OnUpdate:   "SET NULL",
		}
		sql := renderConstraintDefinition(c)
		if !strings.Contains(sql, `ON UPDATE SET NULL`) {
			t.Fatalf("expected ON UPDATE SET NULL in %q", sql)
		}
	})

	t.Run("Both CASCADE and SET NULL", func(t *testing.T) {
		c := &model.Constraint{
			Name:       "fk_user_id",
			Type:       "foreign_key",
			Columns:    []string{"user_id"},
			RefSchema:  "public",
			RefTable:   "users",
			RefColumns: []string{"id"},
			OnDelete:   "CASCADE",
			OnUpdate:   "SET NULL",
		}
		sql := renderConstraintDefinition(c)
		if !strings.Contains(sql, `ON DELETE CASCADE`) {
			t.Fatalf("expected ON DELETE CASCADE in %q", sql)
		}
		if !strings.Contains(sql, `ON UPDATE SET NULL`) {
			t.Fatalf("expected ON UPDATE SET NULL in %q", sql)
		}
	})
}

func TestRenderCreateIndex_Quoted(t *testing.T) {
	r := NewRenderer()
	op := &diff.CreateIndexOp{
		Schema: "public",
		Index: &model.Index{
			Name:    "idx_user",
			Table:   "users",
			Columns: []string{"User Name", "id"},
		},
	}
	sql := r.renderCreateIndex(op)
	expected := "-- op: add_index risk:low\nCREATE INDEX \"idx_user\" ON \"public\".\"users\" (\"User Name\", \"id\");"
	if sql != expected {
		t.Errorf("got %q, want %q", sql, expected)
	}
}

func TestRenderAddTableWithIdentity(t *testing.T) {
	table := model.NewTable("public", "users")
	table.AddColumn(&model.Column{
		Name:         "id",
		DataType:     "integer",
		IsNullable:   false,
		IsIdentity:   true,
		IdentityKind: "ALWAYS",
	})
	table.PrimaryKey = &model.PrimaryKey{Name: "users_pkey", Columns: []string{"id"}}

	sql := NewRenderer().Render(diff.NewAddTableOp("public", "users", table))
	want := `"id" integer GENERATED ALWAYS AS IDENTITY NOT NULL`
	if !strings.Contains(sql, want) {
		t.Errorf("expected SQL to contain %q, got:\n%s", want, sql)
	}
}

func TestRenderAddTableWithIdentityByDefault(t *testing.T) {
	table := model.NewTable("public", "users")
	table.AddColumn(&model.Column{
		Name:         "id",
		DataType:     "integer",
		IsNullable:   false,
		IsIdentity:   true,
		IdentityKind: "BY DEFAULT",
	})
	table.PrimaryKey = &model.PrimaryKey{Name: "users_pkey", Columns: []string{"id"}}

	sql := NewRenderer().Render(diff.NewAddTableOp("public", "users", table))
	want := `"id" integer GENERATED BY DEFAULT AS IDENTITY NOT NULL`
	if !strings.Contains(sql, want) {
		t.Errorf("expected SQL to contain %q, got:\n%s", want, sql)
	}
}

func TestRenderAlterTableAddIdentityColumn(t *testing.T) {
	col := &model.Column{
		Name:         "id",
		DataType:     "integer",
		IsNullable:   false,
		IsIdentity:   true,
		IdentityKind: "ALWAYS",
	}
	op := diff.NewAddColumnOp("public", "users", col)

	sql := NewRenderer().Render(op)
	want := `"id" integer GENERATED ALWAYS AS IDENTITY NOT NULL`
	if !strings.Contains(sql, want) {
		t.Errorf("expected SQL to contain %q, got:\n%s", want, sql)
	}
}

func TestRenderAddTableWithoutIdentity(t *testing.T) {
	table := model.NewTable("public", "users")
	table.AddColumn(&model.Column{
		Name:       "id",
		DataType:   "integer",
		IsNullable: false,
	})
	table.PrimaryKey = &model.PrimaryKey{Name: "users_pkey", Columns: []string{"id"}}

	sql := NewRenderer().Render(diff.NewAddTableOp("public", "users", table))
	if strings.Contains(sql, "IDENTITY") {
		t.Errorf("expected no IDENTITY in output, got:\n%s", sql)
	}
}

func TestRenderCreateSchema(t *testing.T) {
	r := NewRenderer()

	t.Run("CreateSchemaOp", func(t *testing.T) {
		op := diff.NewCreateSchemaOp("auth")
		sql := r.Render(op)
		want := "-- op: create_schema risk:low\nCREATE SCHEMA IF NOT EXISTS \"auth\";"
		if sql != want {
			t.Errorf("got %q, want %q", sql, want)
		}
	})

	t.Run("DropSchemaOp", func(t *testing.T) {
		op := diff.NewDropSchemaOp("old_schema")
		sql := r.Render(op)
		want := "-- op: drop_schema risk:high\nDROP SCHEMA IF EXISTS \"old_schema\";"
		if sql != want {
			t.Errorf("got %q, want %q", sql, want)
		}
	})
}

func TestRenderSetIdentityAlways(t *testing.T) {
	op := diff.NewSetIdentityOp("public", "users", "id", "ALWAYS")
	sql := NewRenderer().Render(op)
	want := `-- op: set_identity risk:low` + "\n" + `ALTER TABLE "public"."users" ALTER COLUMN "id" SET GENERATED ALWAYS;`
	if !strings.Contains(sql, want) {
		t.Errorf("expected SQL to contain %q, got:\n%s", want, sql)
	}
}

func TestRenderSetIdentityByDefault(t *testing.T) {
	op := diff.NewSetIdentityOp("public", "users", "id", "BY DEFAULT")
	sql := NewRenderer().Render(op)
	want := `-- op: set_identity risk:low` + "\n" + `ALTER TABLE "public"."users" ALTER COLUMN "id" SET GENERATED BY DEFAULT;`
	if !strings.Contains(sql, want) {
		t.Errorf("expected SQL to contain %q, got:\n%s", want, sql)
	}
}

func TestRenderDropIdentity(t *testing.T) {
	op := diff.NewDropIdentityOp("public", "users", "id")
	sql := NewRenderer().Render(op)
	want := `-- op: drop_identity risk:medium` + "\n" + `ALTER TABLE "public"."users" ALTER COLUMN "id" DROP IDENTITY;`
	if !strings.Contains(sql, want) {
		t.Errorf("expected SQL to contain %q, got:\n%s", want, sql)
	}
}
