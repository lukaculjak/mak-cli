package cmd

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/lukaculjak/mak-cli/internal/ui"
	"github.com/spf13/cobra"
	"golang.org/x/term"
)

const zshCompletionBlock = `
# mak shell completion
if command -v mak >/dev/null 2>&1; then
  if (( ! $+functions[compdef] )); then
    autoload -Uz compinit
    compinit
  fi
  source <(mak completion zsh)
fi
`

func configureCompletion(root *cobra.Command) {
	root.InitDefaultCompletionCmd()
	zsh, _, _ := root.Find([]string{"completion", "zsh"})
	var install bool
	zsh.Flags().BoolVar(&install, "install", false, "Enable mak completion in your .zshrc")
	zsh.Long = `Generate the zsh completion script. Printing it does not enable completion.

To enable completion for every new terminal:
  mak completion zsh --install

To enable it in the current session:
  autoload -Uz compinit; compinit
  source <(mak completion zsh)

The installer appends a block to ${ZDOTDIR:-$HOME}/.zshrc, preserving your settings.
It does not require Homebrew or sudo. Open a new terminal afterward.`
	zsh.RunE = func(cmd *cobra.Command, args []string) error {
		if install {
			return installZshCompletion(cmd)
		}
		noDescriptions, _ := cmd.Flags().GetBool("no-descriptions")
		var err error
		if noDescriptions {
			err = cmd.Root().GenZshCompletionNoDesc(cmd.OutOrStdout())
		} else {
			err = cmd.Root().GenZshCompletion(cmd.OutOrStdout())
		}
		if err == nil {
			if file, ok := cmd.OutOrStdout().(interface{ Fd() uintptr }); ok && term.IsTerminal(int(file.Fd())) {
				ui.Heading(cmd.ErrOrStderr(), "\nScript printed only. Enable completion with: mak completion zsh --install\nThen open a new terminal, or run: autoload -Uz compinit; compinit; source <(mak completion zsh)")
			}
		}
		return err
	}
}

func installZshCompletion(cmd *cobra.Command) error {
	dir := os.Getenv("ZDOTDIR")
	if dir == "" {
		var err error
		dir, err = os.UserHomeDir()
		if err != nil {
			return err
		}
	}
	if !filepath.IsAbs(dir) {
		return fmt.Errorf("ZDOTDIR must be an absolute path")
	}
	profile := filepath.Join(dir, ".zshrc")
	contents, err := os.ReadFile(profile)
	if err != nil && !os.IsNotExist(err) {
		return err
	}
	if !strings.Contains(string(contents), zshCompletionBlock) {
		if err := os.MkdirAll(dir, 0o755); err != nil {
			return err
		}
		file, err := os.OpenFile(profile, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0o644)
		if err != nil {
			return err
		}
		_, writeErr := file.WriteString(zshCompletionBlock)
		closeErr := file.Close()
		if writeErr != nil {
			return writeErr
		}
		if closeErr != nil {
			return closeErr
		}
	}
	fmt.Fprintf(cmd.OutOrStdout(), "mak completion enabled in %s. Open a new terminal, or run:\n  autoload -Uz compinit; compinit; source <(mak completion zsh)\n", profile)
	return nil
}
