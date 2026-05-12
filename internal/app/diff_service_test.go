package app

import (
	"context"
	"strings"
	"testing"
	"time"

	"github.com/fred29910/migra-go/internal/diff"
	"github.com/fred29910/migra-go/internal/model"
	"github.com/stretchr/testify/require"
)

// fakeDeps is a test double for runnerDeps with call tracking
type fakeDeps struct {
	computeCalled int
	deps          RunnerDeps
}

func newFakeDeps() *fakeDeps {
	fd := &fakeDeps{}
	fd.deps = RunnerDeps{
		LoadSchema: func(ctx context.Context, source string, schemas []string, strict bool) (*model.Schema, error) {
			return model.NewSchema(), nil
		},
		Compute: func(source, target *model.Schema, cfg Config) ([]diff.Operation, []string, error) {
			fd.computeCalled++
			return []diff.Operation{}, []string{}, nil
		},
		Render: func(ops []diff.Operation, format string) (string, error) {
			return "-- No changes detected", nil
		},
	}
	return fd
}

func TestDiffService_Run(t *testing.T) {
	fd := newFakeDeps()
	svc := NewDiffService(fd.deps)
	cfg := Config{
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

	result, err := BuildExecutionPlan(ops, true)
	require.NoError(t, err)
	require.Len(t, result, 2)

	require.Equal(t, diff.KindAddTable, result[0].Kind())
}
