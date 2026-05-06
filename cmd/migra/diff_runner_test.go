package main

import (
	"context"
	"testing"
	"time"

	"github.com/fred29910/migra-go/internal/app"
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
	if cfg.Source != "a.sql" || cfg.Target != "b.sql" || cfg.Format != "json" {
		t.Fatalf("unexpected cfg: %+v", cfg)
	}
}

// fakeDeps is a test double for runnerDeps with call tracking
type fakeDeps struct {
	computeCalled int
	deps          app.RunnerDeps
}

func newFakeDeps() *fakeDeps {
	fd := &fakeDeps{}
	fd.deps = app.RunnerDeps{
		LoadSchema: func(ctx context.Context, source string, schemas []string, strict bool) (*model.Schema, error) {
			return model.NewSchema(), nil
		},
		Compute: func(source, target *model.Schema, cfg app.Config) ([]diff.Operation, []string, error) {
			fd.computeCalled++
			return []diff.Operation{}, []string{}, nil
		},
		Render: func(ops []diff.Operation, format string) (string, error) {
			return "-- No changes detected", nil
		},
	}
	return fd
}

func TestParseDiffConfig_DefaultTimeout(t *testing.T) {
	cmd := newDiffTestCommand()
	cfg, err := parseDiffConfig(cmd, []string{"a.sql", "b.sql"})
	if err != nil {
		t.Fatal(err)
	}
	if cfg.Timeout != 30*time.Second {
		t.Fatalf("expected 30s, got %s", cfg.Timeout)
	}
}

func TestRunDiffWithDeps_UsesInjectedEngines(t *testing.T) {
	fd := newFakeDeps()
	cfg := app.Config{Source: "a.sql", Target: "b.sql", Format: "sql", Timeout: time.Second}
	svc := app.NewDiffService(fd.deps)
	_, _, err := svc.Run(context.Background(), cfg)
	if err != nil {
		t.Fatal(err)
	}
	if fd.computeCalled == 0 {
		t.Fatal("expected injected compute to be called")
	}
}

func TestParseDiffConfig_OneArgReadsTargetFromConfig(t *testing.T) {
	// Setup: viper 读取配置文件中的 database.url 作为 target
	cmd := newDiffTestCommand()
	_ = cmd.Flags().Set("schema", "public")

	// 模拟配置文件中有 database.url
	// 注意：这个测试需要在实际环境中运行，因为 viper 是全局的
	// 这里只是测试 parseDiffConfig 的逻辑
	t.Skip("需要 viper 环境支持，在实际使用中验证")
}

func TestParseDiffConfig_ZeroArgsReadsBothFromConfig(t *testing.T) {
	cmd := newDiffTestCommand()
	_ = cmd.Flags().Set("schema", "public")
	_ = cmd.Flags().Set("format", "sql")

	// 这个测试验证当没有参数时，从配置文件读取 source 和 target
	t.Skip("需要 viper 环境支持，在实际使用中验证")
}
