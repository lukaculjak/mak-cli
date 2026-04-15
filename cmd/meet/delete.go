package meet

import (
	"bufio"
	"fmt"
	"os"

	"github.com/lukaculjak/mak-cli/internal/meetings"
	"github.com/spf13/cobra"
)

func newDeleteCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "delete <alias>",
		Short: "Delete a meeting",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			list, err := meetings.Load()
			if err != nil {
				return err
			}

			if len(list) == 0 {
				fmt.Println("Your meetings list is empty.")
				return nil
			}

			idx, m := meetings.FindByAlias(list, args[0])
			if m == nil {
				fmt.Printf("No meeting found with alias %q.\n", args[0])
				return nil
			}

			r := bufio.NewReader(os.Stdin)
			confirmed, err := promptConfirm(r, fmt.Sprintf("Are you sure you want to delete %q?", m.Alias))
			if err != nil {
				return err
			}
			if !confirmed {
				fmt.Println("Cancelled.")
				return nil
			}

			alias := m.Alias
			list = append(list[:idx], list[idx+1:]...)
			if err := meetings.Save(list); err != nil {
				return err
			}

			fmt.Printf("%q removed from your meetings.\n", alias)

			if err := meetings.RemoveCronJob(alias); err != nil {
				fmt.Printf("Warning: could not remove cron job: %v\n", err)
			} else {
				fmt.Println("Cron job removed.")
			}

			return nil
		},
	}
}
