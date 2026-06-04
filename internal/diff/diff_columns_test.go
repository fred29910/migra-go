package diff

import (
	"testing"

	"github.com/fred29910/migra-go/internal/model"
)

func TestDiffColumn_DataTypeChange(t *testing.T) {
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
	alterOp := ctx.ops[0].(*AlterColumnTypeOp)
	if alterOp.FromType != "integer" || alterOp.ToType != "bigint" {
		t.Errorf("expected integer -> bigint, got %s -> %s", alterOp.FromType, alterOp.ToType)
	}
}

func TestDiffColumn_NullableToNotNull(t *testing.T) {
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

func TestDiffColumn_NotNullToNullable(t *testing.T) {
	ctx := &diffContext{ops: make([]Operation, 0), warnings: make([]string, 0)}
	src := &model.Column{Name: "email", DataType: "text", IsNullable: false}
	tgt := &model.Column{Name: "email", DataType: "text", IsNullable: true}

	ctx.diffColumn("public", "users", src, tgt)

	if len(ctx.ops) != 1 {
		t.Fatalf("expected 1 op, got %d", len(ctx.ops))
	}
	if ctx.ops[0].Kind() != KindDropNotNull {
		t.Fatalf("expected KindDropNotNull, got %v", ctx.ops[0].Kind())
	}
}

func TestDiffColumn_SetDefault(t *testing.T) {
	ctx := &diffContext{ops: make([]Operation, 0), warnings: make([]string, 0)}
	defaultExpr := "now()"
	src := &model.Column{Name: "created_at", DataType: "timestamp", IsNullable: true, DefaultExpr: nil}
	tgt := &model.Column{Name: "created_at", DataType: "timestamp", IsNullable: true, DefaultExpr: &defaultExpr}

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

func TestDiffColumn_CollationChange(t *testing.T) {
	ctx := &diffContext{ops: make([]Operation, 0), warnings: make([]string, 0)}
	src := &model.Column{Name: "name", DataType: "text", IsNullable: true, Collation: ""}
	tgt := &model.Column{Name: "name", DataType: "text", IsNullable: true, Collation: "en_US.UTF-8"}

	ctx.diffColumn("public", "users", src, tgt)

	if len(ctx.ops) != 1 {
		t.Fatalf("expected 1 op, got %d", len(ctx.ops))
	}
	if ctx.ops[0].Kind() != KindAlterColumnCollation {
		t.Fatalf("expected KindAlterColumnCollation, got %v", ctx.ops[0].Kind())
	}
	collOp := ctx.ops[0].(*AlterColumnCollationOp)
	if collOp.FromCollation != "" || collOp.ToCollation != "en_US.UTF-8" {
		t.Errorf("expected '' -> 'en_US.UTF-8', got '%s' -> '%s'", collOp.FromCollation, collOp.ToCollation)
	}
}

func TestDiffColumn_AddIdentity(t *testing.T) {
	ctx := &diffContext{ops: make([]Operation, 0), warnings: make([]string, 0)}
	src := &model.Column{Name: "id", DataType: "integer", IsNullable: false, IsIdentity: false}
	tgt := &model.Column{Name: "id", DataType: "integer", IsNullable: false, IsIdentity: true, IdentityKind: "ALWAYS"}

	ctx.diffColumn("public", "users", src, tgt)

	if len(ctx.ops) != 1 {
		t.Fatalf("expected 1 op, got %d", len(ctx.ops))
	}
	if ctx.ops[0].Kind() != KindAddIdentity {
		t.Fatalf("expected KindAddIdentity, got %v", ctx.ops[0].Kind())
	}
	idOp := ctx.ops[0].(*AddIdentityOp)
	if idOp.IdentityKind != "ALWAYS" {
		t.Errorf("expected IdentityKind 'ALWAYS', got '%s'", idOp.IdentityKind)
	}
}

func TestDiffColumn_SetIdentity(t *testing.T) {
	ctx := &diffContext{ops: make([]Operation, 0), warnings: make([]string, 0)}
	src := &model.Column{Name: "id", DataType: "integer", IsNullable: false, IsIdentity: true, IdentityKind: "ALWAYS"}
	tgt := &model.Column{Name: "id", DataType: "integer", IsNullable: false, IsIdentity: true, IdentityKind: "BY DEFAULT"}

	ctx.diffColumn("public", "users", src, tgt)

	if len(ctx.ops) != 1 {
		t.Fatalf("expected 1 op, got %d", len(ctx.ops))
	}
	if ctx.ops[0].Kind() != KindSetIdentity {
		t.Fatalf("expected KindSetIdentity, got %v", ctx.ops[0].Kind())
	}
	idOp := ctx.ops[0].(*SetIdentityOp)
	if idOp.IdentityKind != "BY DEFAULT" {
		t.Errorf("expected IdentityKind 'BY DEFAULT', got '%s'", idOp.IdentityKind)
	}
}

func TestDiffColumn_DropIdentity(t *testing.T) {
	ctx := &diffContext{ops: make([]Operation, 0), warnings: make([]string, 0)}
	src := &model.Column{Name: "id", DataType: "integer", IsNullable: false, IsIdentity: true, IdentityKind: "ALWAYS"}
	tgt := &model.Column{Name: "id", DataType: "integer", IsNullable: false, IsIdentity: false}

	ctx.diffColumn("public", "users", src, tgt)

	if len(ctx.ops) != 1 {
		t.Fatalf("expected 1 op, got %d", len(ctx.ops))
	}
	if ctx.ops[0].Kind() != KindDropIdentity {
		t.Fatalf("expected KindDropIdentity, got %v", ctx.ops[0].Kind())
	}
}

func TestDiffColumn_NoChange(t *testing.T) {
	ctx := &diffContext{ops: make([]Operation, 0), warnings: make([]string, 0)}
	defaultExpr := "now()"
	src := &model.Column{Name: "id", DataType: "integer", IsNullable: false, DefaultExpr: &defaultExpr, Collation: "en_US.UTF-8", IsIdentity: true, IdentityKind: "ALWAYS"}
	tgt := &model.Column{Name: "id", DataType: "integer", IsNullable: false, DefaultExpr: &defaultExpr, Collation: "en_US.UTF-8", IsIdentity: true, IdentityKind: "ALWAYS"}

	ctx.diffColumn("public", "users", src, tgt)

	if len(ctx.ops) != 0 {
		t.Fatalf("expected 0 ops for identical columns, got %d", len(ctx.ops))
	}
}

func TestDiffColumn_MultipleChanges(t *testing.T) {
	ctx := &diffContext{ops: make([]Operation, 0), warnings: make([]string, 0)}
	src := &model.Column{Name: "age", DataType: "integer", IsNullable: true, DefaultExpr: nil, Collation: ""}
	tgt := &model.Column{Name: "age", DataType: "bigint", IsNullable: false, DefaultExpr: nil, Collation: "en_US.UTF-8"}

	ctx.diffColumn("public", "users", src, tgt)

	kinds := map[Kind]bool{}
	for _, op := range ctx.ops {
		kinds[op.Kind()] = true
	}

	expected := []Kind{KindAlterColumnType, KindSetNotNull, KindAlterColumnCollation}
	for _, k := range expected {
		if !kinds[k] {
			t.Errorf("expected %v in ops, not found", k)
		}
	}
}

func TestSameDefault_BothNil(t *testing.T) {
	if !sameDefault(nil, nil) {
		t.Error("expected sameDefault(nil, nil) to be true")
	}
}

func TestSameDefault_OneNil(t *testing.T) {
	s := "now()"
	if sameDefault(nil, &s) {
		t.Error("expected sameDefault(nil, 'now()') to be false")
	}
	if sameDefault(&s, nil) {
		t.Error("expected sameDefault('now()', nil) to be false")
	}
}

func TestSameDefault_SameValue(t *testing.T) {
	a := "now()"
	b := "now()"
	if !sameDefault(&a, &b) {
		t.Error("expected sameDefault('now()', 'now()') to be true")
	}
}

func TestSameDefault_DifferentValue(t *testing.T) {
	a := "now()"
	b := "'2024-01-01'"
	if sameDefault(&a, &b) {
		t.Error("expected sameDefault('now()', ''2024-01-01'') to be false")
	}
}

func TestDiffColumn_EmptyDataTypeFallsBackToSource(t *testing.T) {
	ctx := &diffContext{ops: make([]Operation, 0), warnings: make([]string, 0)}
	src := &model.Column{Name: "name", DataType: "text", IsNullable: true, Collation: "C"}
	tgt := &model.Column{Name: "name", DataType: "", IsNullable: true, Collation: "en_US.UTF-8"}

	ctx.diffColumn("public", "users", src, tgt)

	if len(ctx.ops) != 1 {
		t.Fatalf("expected 1 op, got %d", len(ctx.ops))
	}
	collOp := ctx.ops[0].(*AlterColumnCollationOp)
	if collOp.DataType != "text" {
		t.Errorf("expected DataType to fall back to source 'text', got '%s'", collOp.DataType)
	}
}
