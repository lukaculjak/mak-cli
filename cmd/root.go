package cmd

import (
	"fmt"
	"os"
	"strings"

	"github.com/lukaculjak/mak/cmd/setup"
	"github.com/lukaculjak/mak/internal/updater"
	"github.com/spf13/cobra"
)

// Version is set at build time by GoReleaser via ldflags.
// Falls back to "dev" for local builds.
var Version = "dev"

var rootCmd = &cobra.Command{
	Use:   "mak",
	Short: "MagicAtworK CLI tool",
	Long: `mak is a personal CLI tool for scaffolding frontend validation,
setting up dev environments, automating meetings while speeding up the development process.`,
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
	rootCmd.AddCommand(newUpdateCmd())
	rootCmd.AddCommand(newUninstallCmd())
}
