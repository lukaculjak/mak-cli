package meet

import "github.com/spf13/cobra"

func NewMeetCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "meet",
		Short: "Manage your recurring meetings",
	}

	cmd.AddCommand(newListCmd())
	cmd.AddCommand(newAddCmd())
	cmd.AddCommand(newEditCmd())
	cmd.AddCommand(newDeleteCmd())
	cmd.AddCommand(newOpenCmd())

	return cmd
}
