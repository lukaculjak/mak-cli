package cmd

import (
	"fmt"
	"os"

	"github.com/lukaculjak/mak/cmd/setup"
	"github.com/spf13/cobra"
)

var rootCmd = &cobra.Command{
	Use:   "mak",
	Short: "MagicAtworK CLI — personal productivity tool",
	Long: `mak is a personal CLI tool for scaffolding frontend validation,
setting up dev environments, automating meetings while speeding up the development process.`,
}

func Execute() {
	if err := rootCmd.Execute(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

func init() {
	rootCmd.AddCommand(setup.NewSetupCmd())
}
