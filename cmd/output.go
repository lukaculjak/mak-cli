package cmd

import (
	"bytes"

	"github.com/lukaculjak/mak-cli/internal/ui"
	"github.com/spf13/cobra"
)

func configureHelp(root *cobra.Command) {
	defaultHelp := root.HelpFunc()
	root.SetHelpFunc(func(cmd *cobra.Command, args []string) {
		out := cmd.OutOrStdout()
		var text bytes.Buffer
		cmd.SetOut(&text)
		defer cmd.SetOut(out)
		defaultHelp(cmd, args)
		ui.Help(out, cmd.CommandPath(), text.String())
	})
}

func humanOutput(cmd *cobra.Command) bool {
	for current := cmd; current != nil; current = current.Parent() {
		if current.Hidden || current.Name() == "completion" || current.Name() == "shellenv" {
			return false
		}
	}
	return true
}
