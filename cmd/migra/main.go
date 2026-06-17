package main

import (
	"fmt"
	"os"

	"github.com/fred29910/migra-go/internal/version"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
)

var rootCmd = &cobra.Command{
	Use:   "migra",
	Short: "PostgreSQL schema migration tool",
	Long:  `A modern Go implementation of schema diff and migration tool for PostgreSQL databases`,
	Run: func(cmd *cobra.Command, args []string) {
		_ = cmd.Help()
	},
}

func init() {
	cobra.OnInitialize(initConfig)

	rootCmd.PersistentFlags().StringP("config", "c", "", "config file (default is $HOME/.migra.yaml)")
	rootCmd.PersistentFlags().BoolP("verbose", "V", false, "verbose output")
	rootCmd.PersistentFlags().BoolP("version", "v", false, "print version and exit")

	existingPreRun := rootCmd.PersistentPreRun
	rootCmd.PersistentPreRun = func(cmd *cobra.Command, args []string) {
		if existingPreRun != nil {
			existingPreRun(cmd, args)
		}
		if v, _ := cmd.Flags().GetBool("version"); v {
			fmt.Println(version.Info())
			os.Exit(0)
		}
	}
}

func setupFlags(cmd *cobra.Command) error {
	if err := viper.BindPFlag("config", cmd.PersistentFlags().Lookup("config")); err != nil {
		return fmt.Errorf("failed to bind config flag: %w", err)
	}
	if err := viper.BindPFlag("verbose", cmd.PersistentFlags().Lookup("verbose")); err != nil {
		return fmt.Errorf("failed to bind verbose flag: %w", err)
	}
	if err := viper.BindPFlag("version", cmd.PersistentFlags().Lookup("version")); err != nil {
		return fmt.Errorf("failed to bind version flag: %w", err)
	}
	return nil
}

func initConfig() {
	viper.SetEnvPrefix("MIGRA")
	viper.AutomaticEnv()

	if cfgFile := viper.GetString("config"); cfgFile != "" {
		viper.SetConfigFile(cfgFile)
	} else {
		home, err := os.UserHomeDir()
		if err == nil {
			viper.AddConfigPath(home)
			viper.SetConfigName(".migra")
		}

		viper.AddConfigPath(".")
		viper.SetConfigName("migra")
	}

	if err := viper.ReadInConfig(); err == nil {
		if viper.GetBool("verbose") {
			fmt.Println("Using config file:", viper.ConfigFileUsed())
		}
	}
}

func main() {
	rootCmd.SetOut(os.Stdout)
	rootCmd.SetErr(os.Stderr)

	if err := setupFlags(rootCmd); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	if err := rootCmd.Execute(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}
