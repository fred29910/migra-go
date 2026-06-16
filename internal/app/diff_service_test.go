package app

import (
	"context"
	"fmt"
	"strings"
	"testing"
	"time"

	"github.com/fred29910/migra-go/internal/diff"
	"github.com/fred29910/migra-go/internal/model"
	"github.com/stretchr/testify/require"
)

// fakeDeps is a test double for runnerDeps with call tracking
type fakeDeps struct {
	deps          RunnerDeps
	computeCalled int
}

func newFakeDeps() *fakeDeps {
	fd := &fakeDeps{}
	fd.deps = RunnerDeps{
		LoadSchema: func(ctx context.Context, source string, schemas []string, strict bool) (*model.Schema, error) {
			return model.NewSchema(), nil
		},
		Compute: func(ctx context.Context, source, target *model.Schema, cfg DiffConfig) ([]diff.Operation, []string, error) {
			fd.computeCalled++
			return []diff.Operation{}, []string{}, nil
		},
		Render: func(ctx context.Context, ops []diff.Operation, format string) (string, error) {
			return "-- No changes detected", nil
		},
	}
	return fd
}

func TestDiffService_Run(t *testing.T) {
	fd := newFakeDeps()
	svc := NewDiffService(fd.deps)
	cfg := DiffConfig{
		Source:  "testdata/example_source.sql",
		Target:  "testdata/example_target.sql",
		Format:  "sql",
		Schemas: []string{"public"},
		Timeout: 5 * time.Second,
	}
	out, warns, err := svc.Run(context.Background(), cfg)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(out, "-- No changes detected") {
		t.Fatalf("expected empty output, got %q", out)
	}
	if fd.computeCalled == 0 {
		t.Fatal("expected compute to be called")
	}
	_ = warns
}

func TestDiffService_Hooks(t *testing.T) {
	called := make([]HookStage, 0)
	fd := newFakeDeps()
	fd.deps.Hooks = []HookFunc{
		func(ctx context.Context, hctx HookContext) error {
			called = append(called, hctx.Stage)
			return nil
		},
	}
	svc := NewDiffService(fd.deps)
	_, _, err := svc.Run(context.Background(), DiffConfig{Timeout: 5 * time.Second})
	require.NoError(t, err)
	require.Equal(t, []HookStage{HookAfterLoad, HookAfterCompute, HookAfterRender}, called)
}

func TestDiffService_HookErrorAborts(t *testing.T) {
	fd := newFakeDeps()
	fd.deps.Hooks = []HookFunc{
		func(ctx context.Context, hctx HookContext) error {
			if hctx.Stage == HookAfterCompute {
				return fmt.Errorf("hook error")
			}
			return nil
		},
	}
	svc := NewDiffService(fd.deps)
	_, _, err := svc.Run(context.Background(), DiffConfig{Timeout: 5 * time.Second})
	require.Error(t, err)
	require.Contains(t, err.Error(), "hook error")
}

func TestNormalizeSchemas(t *testing.T) {
	source := model.NewSchema()
	source.GetOrCreateNamespace("public").Tables["users"] = &model.Table{
		Name: "users",
	}

	target := model.NewSchema()
	target.GetOrCreateNamespace("public")

	err := NormalizeSchemas(source, target)
	require.NoError(t, err)
}

func TestFilterDestructiveOps_DropSchema(t *testing.T) {
	ops := []diff.Operation{diff.NewDropSchemaOp("old_schema")}
	filtered, warnings := FilterDestructiveOps(ops, false)
	require.Empty(t, filtered)
	require.NotEmpty(t, warnings)

	filtered, warnings = FilterDestructiveOps(ops, true)
	require.Len(t, filtered, 1)
	require.Empty(t, warnings)
}

func TestFilterDestructiveOps(t *testing.T) {
	ops := []diff.Operation{
		diff.NewAddTableOp("public", "new_table", &model.Table{Name: "new_table"}),
		diff.NewDropTableOp("public", "old_table"),
		diff.NewAddColumnOp("public", "users", &model.Column{Name: "id", DataType: "integer"}),
		diff.NewDropColumnOp("public", "users", "old_column"),
	}

	filtered, warnings := FilterDestructiveOps(ops, false)
	require.Len(t, filtered, 2)
	require.Len(t, warnings, 4)

	filteredAll, warningsAll := FilterDestructiveOps(ops, true)
	require.Len(t, filteredAll, 4)
	require.Len(t, warningsAll, 0)
}

func TestBuildExecutionPlan(t *testing.T) {
	ops := []diff.Operation{
		diff.NewAddTableOp("public", "users", &model.Table{Name: "users"}),
		diff.NewAddColumnOp("public", "users", &model.Column{Name: "id", DataType: "integer"}),
	}

	result, err := BuildExecutionPlan(context.Background(), ops, true)
	require.NoError(t, err)
	require.Len(t, result, 2)

	require.Equal(t, diff.KindAddTable, result[0].Kind())
}

func TestFilterNamespaces_EmptySchemas(t *testing.T) {
	source := model.NewSchema()
	source.GetOrCreateNamespace("public").Tables["t1"] = &model.Table{Name: "t1"}
	source.GetOrCreateNamespace("auth").Tables["t2"] = &model.Table{Name: "t2"}

	target := model.NewSchema()
	target.GetOrCreateNamespace("public").Tables["t1"] = &model.Table{Name: "t1"}
	target.GetOrCreateNamespace("auth").Tables["t2"] = &model.Table{Name: "t2"}
	target.GetOrCreateNamespace("extra").Tables["t3"] = &model.Table{Name: "t3"}

	// When schemas is nil/empty, all namespaces should be preserved
	FilterNamespaces(source, target, nil)
	require.Len(t, source.Schemas, 2)
	require.Len(t, target.Schemas, 3)

	FilterNamespaces(source, target, []string{})
	require.Len(t, source.Schemas, 2)
	require.Len(t, target.Schemas, 3)
}

func TestFilterNamespaces_FilterToSpecified(t *testing.T) {
	source := model.NewSchema()
	source.GetOrCreateNamespace("public").Tables["t1"] = &model.Table{Name: "t1"}
	source.GetOrCreateNamespace("auth").Tables["t2"] = &model.Table{Name: "t2"}
	source.GetOrCreateNamespace("internal").Tables["t3"] = &model.Table{Name: "t3"}

	target := model.NewSchema()
	target.GetOrCreateNamespace("public").Tables["t1"] = &model.Table{Name: "t1"}
	target.GetOrCreateNamespace("auth").Tables["t2"] = &model.Table{Name: "t2"}
	target.GetOrCreateNamespace("extra").Tables["t4"] = &model.Table{Name: "t4"}

	FilterNamespaces(source, target, []string{"public", "auth"})

	require.Len(t, source.Schemas, 2)
	require.NotNil(t, source.GetNamespace("public"))
	require.NotNil(t, source.GetNamespace("auth"))
	require.Nil(t, source.GetNamespace("internal"))

	require.Len(t, target.Schemas, 2)
	require.NotNil(t, target.GetNamespace("public"))
	require.NotNil(t, target.GetNamespace("auth"))
	require.Nil(t, target.GetNamespace("extra"))
}

func TestFilterNamespaces_NonExistentSchema(t *testing.T) {
	source := model.NewSchema()
	source.GetOrCreateNamespace("public").Tables["t1"] = &model.Table{Name: "t1"}

	target := model.NewSchema()
	target.GetOrCreateNamespace("public").Tables["t1"] = &model.Table{Name: "t1"}

	FilterNamespaces(source, target, []string{"public", "nonexistent"})
	require.Len(t, source.Schemas, 1)
	require.Len(t, target.Schemas, 1)
}

func TestFilterNamespaces_EmptyResultWarning(t *testing.T) {
	source := model.NewSchema()
	source.GetOrCreateNamespace("public").Tables["t1"] = &model.Table{Name: "t1"}

	target := model.NewSchema()
	target.GetOrCreateNamespace("public").Tables["t1"] = &model.Table{Name: "t1"}

	warnings := FilterNamespaces(source, target, []string{"nonexistent"})
	require.Len(t, source.Schemas, 0)
	require.Len(t, target.Schemas, 0)
	require.Len(t, warnings, 1)
	require.Contains(t, warnings[0], "no schemas matched")
}
