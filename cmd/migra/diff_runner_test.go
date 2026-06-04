package main

import (
	"context"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/fred29910/migra-go/internal/app"
	"github.com/fred29910/migra-go/internal/diff"
	"github.com/fred29910/migra-go/internal/model"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
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

// TestParseDiffConfig_FormatFallsBackToViperWhenFlagEmpty tests that when the --format
// flag was registered with an empty default (due to cobra init/viper timing), parseDiffConfig
// falls back to viper's "diff.format" value rather than returning "".
func TestParseDiffConfig_FormatFallsBackToViperWhenFlagEmpty(t *testing.T) {
	// Simulate the scenario: flag was registered with "" default (cobra init timing bug)
	// but viper has "diff.format" = "sql" from config file
	viper.Reset()
	viper.Set("diff.format", "sql")
	defer viper.Reset()

	cmd := &cobra.Command{}
	cmd.Flags().StringSlice("schema", []string{}, "")
	cmd.Flags().String("format", "", "") // empty default = the bug scenario
	cmd.Flags().Bool("unsafe-drop", false, "")
	cmd.Flags().Bool("strict", false, "")
	cmd.Flags().String("output", "", "")
	cmd.Flags().Duration("timeout", defaultDiffTimeout, "")

	cfg, err := parseDiffConfig(cmd, []string{"a.sql", "b.sql"})
	if err != nil {
		t.Fatalf("parseDiffConfig failed: %v", err)
	}
	if cfg.Format != "sql" {
		t.Errorf("expected format 'sql' (from viper fallback), got %q", cfg.Format)
	}
}

// TestParseDiffConfig_FormatDefaultsToSQLWhenNeitherFlagNorViperSet tests the final
// hardcoded fallback: if neither flag nor viper has a format, default to "sql".
func TestParseDiffConfig_FormatDefaultsToSQLWhenNeitherFlagNorViperSet(t *testing.T) {
	viper.Reset()
	defer viper.Reset()

	cmd := &cobra.Command{}
	cmd.Flags().StringSlice("schema", []string{}, "")
	cmd.Flags().String("format", "", "")
	cmd.Flags().Bool("unsafe-drop", false, "")
	cmd.Flags().Bool("strict", false, "")
	cmd.Flags().String("output", "", "")
	cmd.Flags().Duration("timeout", defaultDiffTimeout, "")

	cfg, err := parseDiffConfig(cmd, []string{"a.sql", "b.sql"})
	if err != nil {
		t.Fatalf("parseDiffConfig failed: %v", err)
	}
	if cfg.Format != "sql" {
		t.Errorf("expected default format 'sql', got %q", cfg.Format)
	}
}

func TestParseDiffConfig_OneArgWithViperConfig(t *testing.T) {
	viper.Reset()
	defer viper.Reset()
	viper.Set("database.url", "postgres://localhost/configdb")

	cmd := newDiffTestCommand()
	cfg, err := parseDiffConfig(cmd, []string{"a.sql"})
	if err != nil {
		t.Fatalf("parseDiffConfig failed: %v", err)
	}
	if cfg.Source != "a.sql" {
		t.Errorf("expected source 'a.sql', got %q", cfg.Source)
	}
	if cfg.Target != "postgres://localhost/configdb" {
		t.Errorf("expected target from viper config, got %q", cfg.Target)
	}
}

func TestParseDiffConfig_OneArgWithoutViperConfig(t *testing.T) {
	viper.Reset()
	defer viper.Reset()

	cmd := newDiffTestCommand()
	_, err := parseDiffConfig(cmd, []string{"a.sql"})
	if err == nil {
		t.Fatal("expected error when 1 arg and no database.url config")
	}
}

func TestParseDiffConfig_ZeroArgsWithViperConfig(t *testing.T) {
	viper.Reset()
	defer viper.Reset()
	viper.Set("database.source", "postgres://localhost/src")
	viper.Set("database.target", "postgres://localhost/tgt")

	cmd := newDiffTestCommand()
	cfg, err := parseDiffConfig(cmd, nil)
	if err != nil {
		t.Fatalf("parseDiffConfig failed: %v", err)
	}
	if cfg.Source != "postgres://localhost/src" {
		t.Errorf("expected source from viper, got %q", cfg.Source)
	}
	if cfg.Target != "postgres://localhost/tgt" {
		t.Errorf("expected target from viper, got %q", cfg.Target)
	}
}

func TestParseDiffConfig_ZeroArgsWithoutViperConfig(t *testing.T) {
	viper.Reset()
	defer viper.Reset()

	cmd := newDiffTestCommand()
	_, err := parseDiffConfig(cmd, nil)
	if err == nil {
		t.Fatal("expected error when 0 args and no database config")
	}
}

func TestParseDiffConfig_AllFlags(t *testing.T) {
	cmd := newDiffTestCommand()
	_ = cmd.Flags().Set("schema", "public,auth")
	_ = cmd.Flags().Set("format", "json")
	_ = cmd.Flags().Set("unsafe-drop", "true")
	_ = cmd.Flags().Set("strict", "true")
	_ = cmd.Flags().Set("output", "/tmp/out.sql")
	_ = cmd.Flags().Set("timeout", "60s")

	cfg, err := parseDiffConfig(cmd, []string{"a.sql", "b.sql"})
	if err != nil {
		t.Fatal(err)
	}
	if cfg.Source != "a.sql" || cfg.Target != "b.sql" {
		t.Errorf("unexpected source/target: %s / %s", cfg.Source, cfg.Target)
	}
	if cfg.Format != "json" {
		t.Errorf("expected format 'json', got %q", cfg.Format)
	}
	if !cfg.UnsafeDrop {
		t.Error("expected unsafe-drop=true")
	}
	if !cfg.Strict {
		t.Error("expected strict=true")
	}
	if cfg.OutputFile != "/tmp/out.sql" {
		t.Errorf("expected output '/tmp/out.sql', got %q", cfg.OutputFile)
	}
}

func TestWriteOutputToFile(t *testing.T) {
	tmpFile := filepath.Join(t.TempDir(), "output.sql")
	err := writeOutput("CREATE TABLE t(id int);", tmpFile)
	if err != nil {
		t.Fatalf("writeOutput failed: %v", err)
	}
	data, err := os.ReadFile(tmpFile)
	if err != nil {
		t.Fatal(err)
	}
	if string(data) != "CREATE TABLE t(id int);" {
		t.Errorf("unexpected file content: %s", string(data))
	}
}

func TestWriteOutputToStdout(t *testing.T) {
	err := writeOutput("some sql", "")
	if err != nil {
		t.Fatalf("writeOutput to stdout failed: %v", err)
	}
}

func TestInitConfigWithExplicitFile(t *testing.T) {
	tmpDir := t.TempDir()
	cfgFile := filepath.Join(tmpDir, "migra.yaml")
	if err := os.WriteFile(cfgFile, []byte("database:\n  url: postgres://test/configdb\n"), 0644); err != nil {
		t.Fatal(err)
	}

	viper.Reset()
	defer viper.Reset()
	viper.Set("config", cfgFile)

	initConfig()

	if got := viper.GetString("database.url"); got != "postgres://test/configdb" {
		t.Errorf("expected database.url from config file, got %q", got)
	}
}

func TestInitConfigWithoutExplicitFile(t *testing.T) {
	viper.Reset()
	defer viper.Reset()

	initConfig()
}

func TestRunDiff_SQLFiles(t *testing.T) {
	srcDir := t.TempDir()
	tgtDir := t.TempDir()

	srcSQL := "CREATE TABLE users (id SERIAL PRIMARY KEY, name VARCHAR(100) NOT NULL);"
	tgtSQL := "CREATE TABLE users (id SERIAL PRIMARY KEY, name VARCHAR(100) NOT NULL, email VARCHAR(255));"

	if err := os.WriteFile(filepath.Join(srcDir, "schema.sql"), []byte(srcSQL), 0644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(tgtDir, "schema.sql"), []byte(tgtSQL), 0644); err != nil {
		t.Fatal(err)
	}

	cmd := newDiffTestCommand()
	cmd.SetContext(context.Background())

	err := runDiff(cmd, []string{srcDir, tgtDir})
	if err != nil {
		t.Fatalf("runDiff failed: %v", err)
	}
}
