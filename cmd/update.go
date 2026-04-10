package cmd

import (
	"github.com/lukaculjak/mak/internal/updater"
	"github.com/spf13/cobra"
)

func newUpdateCmd() *cobra.Command {
	return &cobra.Command{
		Use:     "update",
		Aliases: []string{"upgrade"},
		Short:   "Update mak to the latest version",
		RunE: func(cmd *cobra.Command, args []string) error {
			return updater.SelfUpdate(Version)
		},
	}
}
