package cmd

import (
	"fmt"
	"os"
	"strings"

	"github.com/lukaculjak/mak-cli/cmd/meet"
	"github.com/lukaculjak/mak-cli/cmd/setup"
	"github.com/lukaculjak/mak-cli/internal/updater"
	"github.com/spf13/cobra"
)

var Version = "dev"

var rootCmd = &cobra.Command{
	Use:   "mak",
	Short: "MagicAtworK CLI tool",
	Long: `mak is a personal CLI tool designed for scaffolding projects, blocks of code and automating various tasks in the development workflow.`,
	PersistentPostRunE: func(cmd *cobra.Command, args []string) error {
		if !strings.Contains(cmd.CommandPath(), "update") && !strings.Contains(cmd.CommandPath(), "upgrade") {
			updater.CheckAndNotify(Version)
		}
		return nil
	},
}

func Execute() {
	if err := rootCmd.Execute(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

func init() {
	rootCmd.Version = Version
	rootCmd.AddCommand(setup.NewSetupCmd())
	rootCmd.AddCommand(meet.NewMeetCmd())
	rootCmd.AddCommand(newUpdateCmd())
	rootCmd.AddCommand(newUninstallCmd())
}
