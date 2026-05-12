package main

import (
	"testing"
	"time"

	"github.com/spf13/cobra"
	"github.com/stretchr/testify/assert"
)

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
			name: "create index using concurrently",
			sql:  "CREATE INDEX USING CONCURRENTLY",
			want: true,
		},
		{
			name: "case insensitive",
			sql:  "create index concurrently idx on t(c)",
			want: true,
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
