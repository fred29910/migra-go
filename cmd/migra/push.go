package main

import (
	"github.com/spf13/cobra"
	"github.com/spf13/viper"

	"github.com/fred29910/migra-go/internal/app"
	"github.com/fred29910/migra-go/internal/diff"
	"github.com/fred29910/migra-go/internal/model"
	"github.com/fred29910/migra-go/internal/render"
	"github.com/fred29910/migra-go/internal/app/push"
)

var pushCmd = &cobra.Command{
	Use:   "push [source] [target]",
	Short: "Apply schema changes to target database",
	Long: `Compare source and target schemas, then apply SQL to make the target database match the source schema.
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
	pushCmd.Flags().Bool("dry-run", false, "show SQL without executing (default: false)")
	pushCmd.Flags().Bool("execute", false, "execute SQL without confirmation (not recommended)")
	pushCmd.Flags().Bool("no-verify", false, "skip post-execution validation")
	pushCmd.Flags().Duration("timeout", defaultDiffTimeout, "timeout for schema loading")

	// 绑定到 viper
	_ = viper.BindPFlag("diff.schemas", pushCmd.Flags().Lookup("schema"))
	_ = viper.BindPFlag("diff.unsafe_drop", pushCmd.Flags().Lookup("unsafe-drop"))
}
