package setup

import "github.com/spf13/cobra"

func NewSetupCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:     "setup",
		Aliases: []string{"s"},
		Short:   "Scaffold and configure things for your project or machine",
	}

	cmd.AddCommand(newValidationCmd())

	return cmd
}
