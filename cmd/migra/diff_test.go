package main

import (
	"context"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/fred29910/migra-go/internal/source"
	"github.com/spf13/cobra"
)

func TestDBLoader_Match(t *testing.T) {
	loader := &source.DBLoader{}
	cases := []struct {
		source string
		want   bool
	}{
		{"postgres://localhost/db", true},
		{"postgresql://localhost/db", true},
		{"pg://db", true},
		{"mysql://localhost/db", false},
		{"file.sql", false},
	}
	for _, c := range cases {
		if got := loader.Match(c.source); got != c.want {
			t.Fatalf("DBLoader.Match(%s) = %v, want %v", c.source, got, c.want)
		}
	}
}

func TestLoadFromSQLFile_NonStrictReturnsSchemaAndLogsWarnings(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "bad.sql")
	sql := "CREATE TABLE users (id integer); SELECT * FROM users;"
	if err := os.WriteFile(path, []byte(sql), 0o644); err != nil {
		t.Fatal(err)
	}

	oldStderr := os.Stderr
	r, w, _ := os.Pipe()
	os.Stderr = w

	schema, err := loadSchemaWithContext(context.Background(), path, nil, false)

	_ = w.Close()
	os.Stderr = oldStderr
	out, _ := io.ReadAll(r)

	if err != nil {
		t.Fatalf("expected non-strict mode to continue, got: %v", err)
	}
	if schema == nil {
		t.Fatal("expected non-nil schema")
	}
	if !strings.Contains(string(out), "Warning") {
		t.Fatalf("expected warning output, got: %s", string(out))
	}
}

func TestSetupFlags_BindsViperKeys(t *testing.T) {
	cmd := &cobra.Command{Use: "test"}
	cmd.PersistentFlags().String("config", "", "")
	cmd.PersistentFlags().Bool("verbose", false, "")
	cmd.PersistentFlags().Bool("version", false, "")
	if err := setupFlags(cmd); err != nil {
		t.Fatalf("setupFlags failed: %v", err)
	}
}
