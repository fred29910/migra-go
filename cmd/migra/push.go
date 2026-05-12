package main

import (
	"github.com/spf13/cobra"
)

var pushCmd = &cobra.Command{
	Use:   "push [source] [target]",
	Short: "Apply schema changes to target database",
	Long: `Calculate diff from source to target and apply SQL to target database.
This command requires interactive confirmation before executing each SQL.

Examples:
  migra push file.sql postgres://localhost/db
  migra push postgres://localhost/db1 postgres://localhost/db2
  migra push --unsafe-drop file.sql postgres://localhost/db`,
	Args: cobra.ExactArgs(2),
	RunE: runPush,
}

func init() {
	rootCmd.AddCommand(pushCmd)

	pushCmd.Flags().StringSliceP("schema", "s", []string{"public"}, "schemas to compare (can be multiple)")
	pushCmd.Flags().Bool("unsafe-drop", false, "skip confirmation for destructive DROP operations")
	pushCmd.Flags().Bool("dry-run", true, "show SQL without executing (default: true)")
	pushCmd.Flags().Bool("execute", false, "execute SQL without confirmation (not recommended)")
	pushCmd.Flags().Bool("no-verify", false, "skip post-execution validation")
	pushCmd.Flags().Duration("timeout", defaultDiffTimeout, "timeout for schema loading")
}
