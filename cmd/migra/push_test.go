package main

import (
	"testing"
	"time"

	"github.com/spf13/cobra"
	"github.com/stretchr/testify/assert"
)

func TestIsDestructiveOperation(t *testing.T) {
	tests := []struct {
		name          string
		isDestructive bool
		unsafeDrop    bool
		want          bool
	}{
		{
			name:          "destructive without unsafe drop",
			isDestructive: true,
			unsafeDrop:    false,
			want:          true,
		},
		{
			name:          "destructive with unsafe drop",
			isDestructive: true,
			unsafeDrop:    true,
			want:          false,
		},
		{
			name:          "non-destructive without unsafe drop",
			isDestructive: false,
			unsafeDrop:    false,
			want:          false,
		},
		{
			name:          "non-destructive with unsafe drop",
			isDestructive: false,
			unsafeDrop:    true,
			want:          false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := isDestructiveOperation(tt.isDestructive, tt.unsafeDrop)
			assert.Equal(t, tt.want, got)
		})
	}
}

func TestIsNonTransactionalSQL(t *testing.T) {
	tests := []struct {
		name string
		sql  string
		want bool
	}{
		{
			name: "create index concurrently",
			sql:  "CREATE INDEX CONCURRENTLY idx ON t(c)",
			want: true,
		},
		{
			name: "drop index concurrently",
			sql:  "DROP INDEX CONCURRENTLY idx",
			want: true,
		},
		{
			name: "regular create index",
			sql:  "CREATE INDEX idx ON t(c)",
			want: false,
		},
		{
			name: "regular alter table",
			sql:  "ALTER TABLE t ADD COLUMN c int",
			want: false,
		},
		{
			name: "reindex concurrently",
			sql:  "REINDEX INDEX CONCURRENTLY idx",
			want: true,
		},
		{
			name: "case insensitive",
			sql:  "create index concurrently idx on t(c)",
			want: true,
		},
		{
			name: "comment before does not match",
			sql:  "-- This will CREATE INDEX CONCURRENTLY later\nALTER TABLE t ADD COLUMN c int",
			want: false,
		},
		{
			name: "in-line comment does not match",
			sql:  "ALTER TABLE t ADD COLUMN c int; -- runs CREATE INDEX CONCURRENTLY",
			want: false,
		},
		{
			name: "no false positive on similar words",
			sql:  "CREATE INDEX idx ON t (concurrently_col)",
			want: false,
		},
		{
			name: "empty string",
			sql:  "",
			want: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := isNonTransactionalSQL(tt.sql)
			assert.Equal(t, tt.want, got)
		})
	}
}

func TestStripSQLComments(t *testing.T) {
	tests := []struct {
		name string
		sql  string
		want string
	}{
		{
			name: "no comments",
			sql:  "CREATE INDEX CONCURRENTLY idx ON t(c);",
			want: "CREATE INDEX CONCURRENTLY idx ON t(c);",
		},
		{
			name: "inline comment removed",
			sql:  "ALTER TABLE t ADD COLUMN c int; -- some comment",
			want: "ALTER TABLE t ADD COLUMN c int; ",
		},
		{
			name: "full line comment removed",
			sql:  "-- comment\nSELECT 1",
			want: "\nSELECT 1",
		},
		{
			name: "multiple comments",
			sql:  "CREATE INDEX CONCURRENTLY idx ON t(c); -- first\nALTER TABLE t DROP COLUMN c; -- second",
			want: "CREATE INDEX CONCURRENTLY idx ON t(c); \nALTER TABLE t DROP COLUMN c; ",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := stripSQLComments(tt.sql)
			assert.Equal(t, tt.want, got)
		})
	}
}

func TestParsePushConfig(t *testing.T) {
	tests := []struct {
		name    string
		args    []string
		wantErr bool
	}{
		{
			name:    "valid two args",
			args:    []string{"file.sql", "postgres://localhost/db"},
			wantErr: false,
		},
		{
			name:    "valid two args pg prefix",
			args:    []string{"file.sql", "pg://localhost/db"},
			wantErr: false,
		},
		{
			name:    "valid two args postgresql prefix",
			args:    []string{"file.sql", "postgresql://localhost/db"},
			wantErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cmd := &cobra.Command{Use: "push", Args: cobra.ExactArgs(2)}
			cmd.Flags().StringSlice("schema", []string{"public"}, "")
			cmd.Flags().Bool("unsafe-drop", false, "")
			cmd.Flags().Bool("dry-run", true, "")
			cmd.Flags().Bool("execute", false, "")
			cmd.Flags().Bool("no-verify", false, "")
			cmd.Flags().Duration("timeout", 30*time.Second, "")

			_, err := parsePushConfig(cmd, tt.args)
			if tt.wantErr {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
			}
		})
	}
}
