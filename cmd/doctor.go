package cmd

import (
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"os/signal"
	"syscall"

	"github.com/lukaculjak/mak-cli/internal/devenv"
	"github.com/lukaculjak/mak-cli/internal/ui"
	"github.com/spf13/cobra"
)

var errDoctorReported = errors.New("doctor already reported its findings")

func newDoctorCmd(check func(context.Context, io.Writer) error) *cobra.Command {
	return &cobra.Command{
		Use: "doctor", Aliases: []string{"healthcheck"},
		Short: "Check the coding environment without changing it",
		Long: `Check coding tools on PATH, the bundled Neovim configuration, locked plugins,
syntax parsers, language-server startup, installation tracking and backups.
Neovim checks use temporary files with downloads and installation writes blocked.
Print suggested repairs and the current mak version. No repairs run automatically.
Exit with status 1 when checks fail; warnings alone do not fail the command.`,
		Args: cobra.NoArgs, SilenceUsage: true,
		// Keep the version last; doctor does not run the update notifier or Done footer.
		PersistentPostRunE: func(*cobra.Command, []string) error { return nil },
		RunE: func(cmd *cobra.Command, args []string) error {
			ctx, stop := signal.NotifyContext(cmd.Context(), os.Interrupt, syscall.SIGTERM)
			defer stop()
			err := check(ctx, cmd.OutOrStdout())
			if err != nil && !errors.Is(err, devenv.ErrUnhealthy) {
				ui.Error(cmd.OutOrStdout(), "%v", err)
			}
			fmt.Fprintf(cmd.OutOrStdout(), "\nmak version: %s\n", Version)
			if err != nil {
				return errors.Join(errDoctorReported, err)
			}
			return nil
		},
	}
}
