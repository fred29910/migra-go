package diff

import (
	"context"
	"testing"
)

func TestDiffContext_AddOp(t *testing.T) {
	ctx := newDiffContext(context.Background(), nil)

	if len(ctx.ops) != 0 {
		t.Fatalf("expected 0 ops initially, got %d", len(ctx.ops))
	}

	op := NewCreateSchemaOp("test_schema")
	ctx.addOp(op)

	if len(ctx.ops) != 1 {
		t.Fatalf("expected 1 op after add, got %d", len(ctx.ops))
	}

	if ctx.ops[0].Kind() != KindCreateSchema {
		t.Errorf("expected KindCreateSchema, got %v", ctx.ops[0].Kind())
	}
}

func TestDiffContext_AddOp_Multiple(t *testing.T) {
	ctx := newDiffContext(context.Background(), nil)

	ctx.addOp(NewCreateSchemaOp("s1"))
	ctx.addOp(NewDropSchemaOp("s2"))
	ctx.addOp(NewAddTableOp("public", "users", nil))

	if len(ctx.ops) != 3 {
		t.Fatalf("expected 3 ops, got %d", len(ctx.ops))
	}

	expectedKinds := []Kind{KindCreateSchema, KindDropSchema, KindAddTable}
	for i, expected := range expectedKinds {
		if ctx.ops[i].Kind() != expected {
			t.Errorf("op[%d]: expected %v, got %v", i, expected, ctx.ops[i].Kind())
		}
	}
}

func TestDiffContext_Warnf(t *testing.T) {
	ctx := newDiffContext(context.Background(), nil)

	if len(ctx.warnings) != 0 {
		t.Fatalf("expected 0 warnings initially, got %d", len(ctx.warnings))
	}

	ctx.warnf("test warning: %s", "hello")

	if len(ctx.warnings) != 1 {
		t.Fatalf("expected 1 warning, got %d", len(ctx.warnings))
	}

	if ctx.warnings[0] != "test warning: hello" {
		t.Errorf("expected 'test warning: hello', got '%s'", ctx.warnings[0])
	}
}

func TestDiffContext_Warnf_Multiple(t *testing.T) {
	ctx := newDiffContext(context.Background(), nil)

	ctx.warnf("warning 1")
	ctx.warnf("warning 2: %d", 42)
	ctx.warnf("warning 3: %s", "test")

	if len(ctx.warnings) != 3 {
		t.Fatalf("expected 3 warnings, got %d", len(ctx.warnings))
	}

	expected := []string{"warning 1", "warning 2: 42", "warning 3: test"}
	for i, exp := range expected {
		if ctx.warnings[i] != exp {
			t.Errorf("warning[%d]: expected '%s', got '%s'", i, exp, ctx.warnings[i])
		}
	}
}

func TestDiffContext_OpsAndWarningsIndependent(t *testing.T) {
	ctx := newDiffContext(context.Background(), nil)

	ctx.addOp(NewCreateSchemaOp("s1"))
	ctx.warnf("warn 1")
	ctx.addOp(NewDropSchemaOp("s2"))
	ctx.warnf("warn 2")

	if len(ctx.ops) != 2 {
		t.Fatalf("expected 2 ops, got %d", len(ctx.ops))
	}

	if len(ctx.warnings) != 2 {
		t.Fatalf("expected 2 warnings, got %d", len(ctx.warnings))
	}
}
