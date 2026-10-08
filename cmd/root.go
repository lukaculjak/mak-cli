package cmd

import (
	"errors"
	"fmt"
	"os"
	"strings"

	"github.com/lukaculjak/mak-cli/cmd/meet"
	"github.com/lukaculjak/mak-cli/cmd/prefill"
	"github.com/lukaculjak/mak-cli/cmd/setup"
	"github.com/lukaculjak/mak-cli/internal/devenv"
	"github.com/lukaculjak/mak-cli/internal/ui"
	"github.com/lukaculjak/mak-cli/internal/updater"
	"github.com/spf13/cobra"
)

var Version = "dev"

const banner = `▄▄ ▄  ▄▄  ▄ ▄     ▄▄▄ ▄   ▄▄▄
█ █ █ █▄█ █▄▀ ▄▄▄ █   █    █
█   █ █ █ █ █     ▀▄▄ █▄▄ ▄█▄`

var rootCmd = &cobra.Command{
	Use:           "mak",
	SilenceErrors: true,
	SilenceUsage:  true,
	Short:         "MagicAtworK CLI tool",
	Long: banner + `

mak is a personal CLI tool designed for scaffolding projects, blocks of code and automating various tasks in the development workflow.`,
	PersistentPreRun: func(cmd *cobra.Command, args []string) {
		if humanOutput(cmd) {
			ui.Begin(cmd.OutOrStdout(), cmd.CommandPath())
		}
	},
	PersistentPostRunE: func(cmd *cobra.Command, args []string) error {
		if !humanOutput(cmd) {
			return nil
		}
		if !strings.Contains(cmd.CommandPath(), "update") && !strings.Contains(cmd.CommandPath(), "upgrade") {
			updater.CheckAndNotify(Version)
		}
		ui.End(cmd.OutOrStdout())
		return nil
	},
}

func Execute() {
	if prefill.IsNativeHostInvocation(os.Args[1:]) {
		if err := prefill.RunNativeHost(os.Stdin, os.Stdout); err != nil {
			fmt.Fprintln(os.Stderr, err)
			os.Exit(1)
		}
		return
	}

	if command, err := rootCmd.ExecuteC(); err != nil {
		if !errors.Is(err, errDoctorReported) {
			ui.Error(os.Stderr, "%v", err)
		}
		if command != nil && (command == rootCmd || !command.SilenceUsage) {
			fmt.Fprintf(os.Stderr, "Run '%s --help' for usage.\n", command.CommandPath())
		}
		os.Exit(1)
	}
}

func init() {
	configureHelp(rootCmd)
	rootCmd.Version = Version
	rootCmd.AddCommand(setup.NewSetupCmd())
	rootCmd.AddCommand(meet.NewMeetCmd())
	rootCmd.AddCommand(prefill.NewPrefillCmd())
	rootCmd.AddCommand(newUpdateCmd())
	rootCmd.AddCommand(newUninstallCmd())
	rootCmd.AddCommand(newDoctorCmd(devenv.Doctor))
	rootCmd.AddCommand(newShellenvCmd())
}
