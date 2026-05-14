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
	if err != nil || !strings.Contains(js, `"kind"`) {
		t.Fatalf("unexpected json render: %v %q", err, js)
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
