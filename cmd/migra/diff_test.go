package main

import (
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/spf13/cobra"
)

func TestIsPostgresURL_AcceptsPgScheme(t *testing.T) {
	cases := []string{"pg://db", "pg://a", "postgres://localhost/db"}
	for _, c := range cases {
		if !isPostgresURL(c) {
			t.Fatalf("expected postgres url: %s", c)
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

	schema, err := loadFromSQLFile(path, false)

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
	if err := setupFlags(cmd); err != nil {
		t.Fatalf("setupFlags failed: %v", err)
	}
}
