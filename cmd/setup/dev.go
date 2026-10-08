package setup

import (
	"os"
	"os/signal"
	"syscall"

	"github.com/lukaculjak/mak-cli/internal/devenv"
	"github.com/spf13/cobra"
)

func newDevCmd() *cobra.Command {
	var uninstall bool
	cmd := &cobra.Command{
		Use:     "dev",
		Aliases: []string{"codeenv", "codingenv"},
		Short:   "Install Luka's Neovim coding environment (macOS)",
		Long: `Install Homebrew prerequisites and the bundled, locked LazyVim configuration.
Existing Neovim configuration, data, state and cache are backed up before replacement.
Failed setup restores those backups and removes newly installed Homebrew packages.
Close Neovim before running. Homebrew may request your macOS administrator password.
Use --uninstall to restore the original Neovim files and remove packages tracked by mak.
Homebrew, Apple developer tools, and packages needed by other tools are retained.`,
		Args:         cobra.NoArgs,
		SilenceUsage: true,
		RunE: func(cmd *cobra.Command, args []string) error {
			ctx, stop := signal.NotifyContext(cmd.Context(), os.Interrupt, syscall.SIGTERM)
			defer stop()
			if uninstall {
				return devenv.Uninstall(ctx, cmd.InOrStdin(), cmd.OutOrStdout())
			}
			return devenv.Setup(ctx, cmd.InOrStdin(), cmd.OutOrStdout())
		},
	}
	cmd.Flags().BoolVar(&uninstall, "uninstall", false, "remove the tracked coding environment and restore previous Neovim files")
	return cmd
}
