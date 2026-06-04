package diff

import (
	"testing"

	"github.com/fred29910/migra-go/internal/model"
)

func TestDiffColumn_SetDefault(t *testing.T) {
	ctx := &diffContext{ops: make([]Operation, 0), warnings: make([]string, 0)}
	src := &model.Column{Name: "created_at", DataType: "timestamp", IsNullable: true}
	tgt := &model.Column{Name: "created_at", DataType: "timestamp", IsNullable: true, DefaultExpr: strPtr("now()")}

	ctx.diffColumn("public", "users", src, tgt)

	if len(ctx.ops) != 1 {
		t.Fatalf("expected 1 op, got %d", len(ctx.ops))
	}
	if ctx.ops[0].Kind() != KindSetDefault {
		t.Fatalf("expected KindSetDefault, got %v", ctx.ops[0].Kind())
	}
	setOp := ctx.ops[0].(*SetDefaultOp)
	if setOp.DefaultExpr != "now()" {
		t.Errorf("expected default 'now()', got '%s'", setOp.DefaultExpr)
	}
}

func TestDiffColumn_DropDefault(t *testing.T) {
	ctx := &diffContext{ops: make([]Operation, 0), warnings: make([]string, 0)}
	defaultExpr := "now()"
	src := &model.Column{Name: "created_at", DataType: "timestamp", IsNullable: true, DefaultExpr: &defaultExpr}
	tgt := &model.Column{Name: "created_at", DataType: "timestamp", IsNullable: true, DefaultExpr: nil}

	ctx.diffColumn("public", "users", src, tgt)

	if len(ctx.ops) != 1 {
		t.Fatalf("expected 1 op, got %d", len(ctx.ops))
	}
	if ctx.ops[0].Kind() != KindDropDefault {
		t.Fatalf("expected KindDropDefault, got %v", ctx.ops[0].Kind())
	}
}

func TestDiffColumn_NoChange(t *testing.T) {
	ctx := &diffContext{ops: make([]Operation, 0), warnings: make([]string, 0)}
	src := &model.Column{Name: "id", DataType: "integer", IsNullable: false}
	tgt := &model.Column{Name: "id", DataType: "integer", IsNullable: false}

	ctx.diffColumn("public", "users", src, tgt)

	if len(ctx.ops) != 0 {
		t.Fatalf("expected 0 ops, got %d", len(ctx.ops))
	}
}

func TestDiffColumn_TypeChange(t *testing.T) {
	ctx := &diffContext{ops: make([]Operation, 0), warnings: make([]string, 0)}
	src := &model.Column{Name: "age", DataType: "integer", IsNullable: true}
	tgt := &model.Column{Name: "age", DataType: "bigint", IsNullable: true}

	ctx.diffColumn("public", "users", src, tgt)

	if len(ctx.ops) != 1 {
		t.Fatalf("expected 1 op, got %d", len(ctx.ops))
	}
	if ctx.ops[0].Kind() != KindAlterColumnType {
		t.Fatalf("expected KindAlterColumnType, got %v", ctx.ops[0].Kind())
	}
}

func TestDiffColumn_NullableChange(t *testing.T) {
	ctx := &diffContext{ops: make([]Operation, 0), warnings: make([]string, 0)}
	src := &model.Column{Name: "email", DataType: "text", IsNullable: true}
	tgt := &model.Column{Name: "email", DataType: "text", IsNullable: false}

	ctx.diffColumn("public", "users", src, tgt)

	if len(ctx.ops) != 1 {
		t.Fatalf("expected 1 op, got %d", len(ctx.ops))
	}
	if ctx.ops[0].Kind() != KindSetNotNull {
		t.Fatalf("expected KindSetNotNull, got %v", ctx.ops[0].Kind())
	}
}

func TestDiffColumn_CollationChange(t *testing.T) {
	ctx := &diffContext{ops: make([]Operation, 0), warnings: make([]string, 0)}
	src := &model.Column{Name: "name", DataType: "text", IsNullable: true, Collation: "en_US"}
	tgt := &model.Column{Name: "name", DataType: "text", IsNullable: true, Collation: "fr_FR"}

	ctx.diffColumn("public", "users", src, tgt)

	if len(ctx.ops) != 1 {
		t.Fatalf("expected 1 op, got %d", len(ctx.ops))
	}
	if ctx.ops[0].Kind() != KindAlterColumnCollation {
		t.Fatalf("expected KindAlterColumnCollation, got %v", ctx.ops[0].Kind())
	}
}

func strPtr(s string) *string {
	return &s
}
