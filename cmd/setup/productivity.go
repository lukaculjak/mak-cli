package setup

import (
	"context"
	"fmt"
	"io"
	"os"
	"os/signal"
	"syscall"

	"github.com/lukaculjak/mak-cli/internal/devenv"
	"github.com/lukaculjak/mak-cli/internal/productivity"
	"github.com/spf13/cobra"
)

func setupPomodoro(ctx context.Context, in io.Reader, out io.Writer) error {
	for _, name := range []string{"timer", "terminal-notifier"} {
		if err := devenv.InstallPackage(ctx, in, out, name); err != nil {
			return err
		}
	}
	fmt.Fprintln(out, "Pomodoro ready: mak work (50 minutes), mak rest (10 minutes).")
	return nil
}

func setupNeorg(ctx context.Context, in io.Reader, out io.Writer) error {
	if err := devenv.SetupNeorg(ctx, in, out); err != nil {
		return err
	}
	return productivity.ConfigureReminder(ctx, out, false)
}

func newProductivitySetupCmd(neorg bool) *cobra.Command {
	name, short := "pomodoro", "Install terminal Pomodoro timers and notifications (macOS)"
	if neorg {
		name, short = "neorg", "Install Neorg for an existing LazyVim setup and schedule its journal reminder"
	}
	return &cobra.Command{Use: name, Short: short, Args: cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			ctx, stop := signal.NotifyContext(cmd.Context(), os.Interrupt, syscall.SIGTERM)
			defer stop()
			if neorg {
				return setupNeorg(ctx, cmd.InOrStdin(), cmd.OutOrStdout())
			}
			return setupPomodoro(ctx, cmd.InOrStdin(), cmd.OutOrStdout())
		},
	}
}
