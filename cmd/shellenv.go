package cmd

import (
	"github.com/lukaculjak/mak-cli/internal/devenv"
	"github.com/lukaculjak/mak-cli/internal/ui"
	"github.com/spf13/cobra"
	"golang.org/x/term"
)

func newShellenvCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "shellenv",
		Short: "Print coding-environment settings for the current shell",
		Long: `Print shell code that refreshes Homebrew and coding-tool PATH settings.
Apply it in your current zsh or bash session:

  eval "$(mak shellenv)"

Running mak shellenv by itself only prints the settings. A child process cannot
change its parent shell. No software is installed and no profiles are changed.`,
		Args: cobra.NoArgs, SilenceUsage: true,
		RunE: func(cmd *cobra.Command, args []string) error {
			if err := devenv.Shellenv(cmd.Context(), cmd.OutOrStdout()); err != nil {
				return err
			}
			if file, ok := cmd.OutOrStdout().(interface{ Fd() uintptr }); ok && term.IsTerminal(int(file.Fd())) {
				ui.Heading(cmd.ErrOrStderr(), "\nSettings printed only; your current shell has not been refreshed.\nApply them with: eval \"$(mak shellenv)\"\nFrom source:    eval \"$(go run . shellenv)\"")
			}
			return nil
		},
	}
}
