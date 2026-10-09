package setup

import (
	"fmt"
	"os"
	"os/signal"
	"syscall"

	"github.com/lukaculjak/mak-cli/internal/devenv"
	"github.com/lukaculjak/mak-cli/internal/ui"
	"github.com/spf13/cobra"
)

func newDevCmd() *cobra.Command {
	var uninstall, list, yes bool
	var install, remove string
	cmd := &cobra.Command{
		Use:     "dev",
		Aliases: []string{"codeenv", "codingenv"},
		Short:   "Install Luka's Neovim and Ghostty coding environment (macOS)",
		Long: `Install Homebrew prerequisites, the bundled LazyVim environment and Ghostty settings.
The installation confirmation defaults to yes when you press Enter; --yes skips it.
Use --list (-l) to preview software and Homebrew installation status.
Use --install (-i) PACKAGE or --remove (-r) PACKAGE to manage one tracked package.
Installing neovim (alias: nvim) runs the full LazyVim environment setup.
Installing ghostty includes Luka's configuration and Meslo Nerd Font.
Existing Neovim configuration, data, state and cache are backed up before replacement.
Existing Ghostty configuration directories are also backed up before replacement.
Failed setup restores those backups and removes newly installed Homebrew packages.
Close Neovim before running. Homebrew may request your macOS administrator password.
Use --uninstall to restore original Neovim and Ghostty files and remove tracked packages.
Homebrew, Apple developer tools, and packages needed by other tools are retained.`,
		Args:         cobra.NoArgs,
		SilenceUsage: true,
		RunE: func(cmd *cobra.Command, args []string) error {
			ctx, stop := signal.NotifyContext(cmd.Context(), os.Interrupt, syscall.SIGTERM)
			defer stop()
			if list {
				return devenv.ListPackages(ctx, cmd.InOrStdin(), cmd.OutOrStdout())
			}
			if cmd.Flags().Changed("install") {
				name, err := devenv.PackageName(install)
				if err != nil {
					return err
				}
				if name != "neovim" && name != "ghostty" {
					return devenv.InstallPackage(ctx, cmd.InOrStdin(), cmd.OutOrStdout(), name)
				}
			}
			if cmd.Flags().Changed("remove") {
				return devenv.RemovePackage(ctx, cmd.InOrStdin(), cmd.OutOrStdout(), remove)
			}
			if uninstall {
				return devenv.Uninstall(ctx, cmd.InOrStdin(), cmd.OutOrStdout())
			}
			if !yes {
				ui.Heading(cmd.OutOrStdout(), "Luka's coding environment")
				if install == "ghostty" {
					fmt.Fprintln(cmd.OutOrStdout(), "Install Ghostty, Meslo Nerd Font and Luka's configuration. Existing Ghostty files will be backed up and replaced.")
				} else {
					fmt.Fprintln(cmd.OutOrStdout(), "Install coding tools, LazyVim and Ghostty. Existing Neovim and Ghostty files will be backed up and replaced.")
				}
				fmt.Fprintln(cmd.OutOrStdout(), "Preview available software and installation status: mak setup dev --list (or -l).")
				confirmed, err := ui.Confirm(ctx, cmd.InOrStdin(), cmd.OutOrStdout(), "Install the development environment?")
				if err != nil {
					return err
				}
				if !confirmed {
					fmt.Fprintln(cmd.OutOrStdout(), "Installation cancelled. Nothing was changed.")
					return nil
				}
			}
			if install == "ghostty" {
				return devenv.InstallPackage(ctx, cmd.InOrStdin(), cmd.OutOrStdout(), install)
			}
			return devenv.Setup(ctx, cmd.InOrStdin(), cmd.OutOrStdout())
		},
	}
	cmd.Flags().BoolVar(&uninstall, "uninstall", false, "remove the tracked coding environment and restore previous Neovim and Ghostty files")
	cmd.Flags().BoolVarP(&list, "list", "l", false, "list available packages, their Homebrew status and mak ownership")
	cmd.Flags().StringVarP(&install, "install", "i", "", "install a package; neovim/nvim installs the full LazyVim environment")
	cmd.Flags().StringVarP(&remove, "remove", "r", "", "remove one package tracked by mak (e.g. --remove neovim)")
	cmd.Flags().BoolVarP(&yes, "yes", "y", false, "confirm the full environment or Ghostty installation without a prompt")
	cmd.MarkFlagsMutuallyExclusive("list", "install", "remove", "uninstall")
	return cmd
}
