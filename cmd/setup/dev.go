package setup

import (
	"os"
	"os/signal"
	"syscall"

	"github.com/lukaculjak/mak-cli/internal/devenv"
	"github.com/spf13/cobra"
)

func newDevCmd() *cobra.Command {
	return &cobra.Command{
		Use:     "dev",
		Aliases: []string{"codeenv", "codingenv"},
		Short:   "Install Luka's Neovim coding environment (macOS)",
		Long: `Install Homebrew prerequisites and the bundled, locked LazyVim configuration.
Existing Neovim configuration, data, state and cache are backed up before replacement.
Failed setup restores those backups and removes newly installed Homebrew packages.
Close Neovim before running. Homebrew may request your macOS administrator password.`,
		Args:         cobra.NoArgs,
		SilenceUsage: true,
		RunE: func(cmd *cobra.Command, args []string) error {
			ctx, stop := signal.NotifyContext(cmd.Context(), os.Interrupt, syscall.SIGTERM)
			defer stop()
			return devenv.Setup(ctx, cmd.InOrStdin(), cmd.OutOrStdout())
		},
	}
}
