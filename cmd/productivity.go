package cmd

import (
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/lukaculjak/mak-cli/internal/productivity"
	"github.com/spf13/cobra"
)

func newSyncCmd(backup bool) *cobra.Command {
	name, short := "sync", "Merge newer Neorg notes with iCloud Drive (macOS)"
	if backup {
		name, short = "backup", "Back up local Neorg notes to iCloud Drive (macOS)"
	}
	return &cobra.Command{Use: name, Short: short, Args: cobra.NoArgs,
		Long: short + `. Notes live in ~/neorg and iCloud Drive's .MAK/neorg folder.
Backup also creates a dated snapshot under .MAK/backups.
Replaced files are saved in history. Deleted files are retained on the other side.
Sync before editing on another machine; sync/backup again after saving your notes.
Neorg Lua configuration uses the same newer-file merge and version history.
Enable iCloud Drive in macOS settings. mak does not request cloud credentials.
Cloud-only files are downloaded automatically, with a two-minute timeout per file.
Keep Downloaded on .MAK in Finder prevents macOS from evicting local copies.
macOS manages cloud uploads; mak verifies only local file transfers.`,
		RunE: func(cmd *cobra.Command, _ []string) error {
			ctx, stop := signal.NotifyContext(cmd.Context(), os.Interrupt, syscall.SIGTERM)
			defer stop()
			return productivity.Sync(ctx, cmd.OutOrStdout(), backup)
		},
	}
}

func newTimerCmd(rest bool) *cobra.Command {
	name, short := "work", "Start a 50-minute work timer, then send a desktop notification"
	if rest {
		name, short = "rest", "Start a 10-minute break timer, then send a desktop notification"
	}
	return &cobra.Command{Use: name, Short: short, Args: cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			ctx, stop := signal.NotifyContext(cmd.Context(), os.Interrupt, syscall.SIGTERM)
			defer stop()
			return productivity.Timer(ctx, cmd.InOrStdin(), cmd.OutOrStdout(), rest)
		},
	}
}

func newNeorgCmd() *cobra.Command {
	cmd := &cobra.Command{Use: "neorg", Short: "Neorg journal reminders"}
	cmd.AddCommand(&cobra.Command{Use: "remind", Short: "Notify if today's journal contains notes (weekdays only)", Args: cobra.NoArgs,
		PersistentPreRun:   func(*cobra.Command, []string) {},
		PersistentPostRunE: func(*cobra.Command, []string) error { return nil },
		RunE: func(cmd *cobra.Command, _ []string) error {
			return productivity.Remind(cmd.Context(), cmd.OutOrStdout(), time.Now())
		},
	})
	var remove bool
	reminder := &cobra.Command{Use: "reminder", Short: "Install the weekday 09:17 journal cron job", Args: cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			return productivity.ConfigureReminder(cmd.Context(), cmd.OutOrStdout(), remove)
		},
	}
	reminder.Flags().BoolVar(&remove, "remove", false, "remove mak's journal reminder, preserving other cron jobs")
	cmd.AddCommand(reminder)
	return cmd
}
