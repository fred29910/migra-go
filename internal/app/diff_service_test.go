package app

import (
	"context"
	"strings"
	"testing"
	"time"

	"github.com/fred29910/migra-go/internal/diff"
	"github.com/fred29910/migra-go/internal/model"
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
