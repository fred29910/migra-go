package main

import (
	"context"
	"testing"
	"time"

	"github.com/fred29910/migra-go/internal/diff"
	"github.com/fred29910/migra-go/internal/model"
	"github.com/spf13/cobra"
)

// newDiffTestCommand creates a cobra command with all diff flags for testing
func newDiffTestCommand() *cobra.Command {
	cmd := &cobra.Command{Use: "diff"}
	cmd.Flags().StringSlice("schema", []string{"public"}, "")
	cmd.Flags().String("format", "sql", "")
	cmd.Flags().Bool("unsafe-drop", false, "")
	cmd.Flags().Bool("strict", false, "")
	cmd.Flags().String("output", "", "")
	cmd.Flags().Duration("timeout", defaultDiffTimeout, "")
	return cmd
}

func TestParseDiffConfig(t *testing.T) {
	cmd := &cobra.Command{Use: "diff"}
	cmd.Flags().StringSlice("schema", []string{"public"}, "")
	cmd.Flags().String("format", "sql", "")
	cmd.Flags().Bool("unsafe-drop", false, "")
	cmd.Flags().Bool("strict", false, "")
	cmd.Flags().String("output", "", "")
	cmd.Flags().Duration("timeout", defaultDiffTimeout, "")
	_ = cmd.Flags().Set("schema", "public,app")
	_ = cmd.Flags().Set("format", "json")

	cfg, err := parseDiffConfig(cmd, []string{"a.sql", "b.sql"})
	if err != nil {
		t.Fatal(err)
	}
	if cfg.source != "a.sql" || cfg.target != "b.sql" || cfg.format != "json" {
		t.Fatalf("unexpected cfg: %+v", cfg)
	}
}

// fakeDeps is a test double for runnerDeps with call tracking
type fakeDeps struct {
	computeCalled int
	deps          runnerDeps
}

func newFakeDeps() *fakeDeps {
	fd := &fakeDeps{}
	fd.deps = runnerDeps{
		loadSchema: func(ctx context.Context, source string, schemas []string, strict bool) (*model.Schema, error) {
			return model.NewSchema(), nil
		},
		compute: func(source, target *model.Schema, cfg diffConfig) ([]diff.Operation, []string, error) {
			fd.computeCalled++
			return []diff.Operation{}, []string{}, nil
		},
		reportWarnings: func(warnings []string) {},
		render: func(ops []diff.Operation, format string) (string, error) {
			return "-- No changes detected", nil
		},
		writeOutput: func(output string, outputFile string) error {
			return nil
		},
	}
	return fd
}

func TestRunDiffWithDeps_UsesInjectedEngines(t *testing.T) {
	fd := newFakeDeps()
	cfg := diffConfig{source: "a.sql", target: "b.sql", format: "sql", timeout: time.Second}
	if err := runDiffWithDeps(context.Background(), cfg, fd.deps); err != nil {
		t.Fatal(err)
	}
	if fd.computeCalled == 0 {
		t.Fatal("expected injected compute to be called")
	}
}
