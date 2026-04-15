package meet

import (
	"bufio"
	"fmt"
	"os"
	"strings"

	"github.com/lukaculjak/mak-cli/internal/meetings"
	"github.com/spf13/cobra"
)

func newEditCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "edit <alias>",
		Short: "Edit an existing meeting",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			list, err := meetings.Load()
			if err != nil {
				return err
			}

			idx, existing := meetings.FindByAlias(list, args[0])
			if existing == nil {
				fmt.Printf("No meeting found with alias %q.\n", args[0])
				if len(list) == 0 {
					fmt.Println("Your meetings list is empty.")
				}
				return nil
			}

			r := bufio.NewReader(os.Stdin)
			originalAlias := existing.Alias

			aliasConflicts := func(alias string) bool {
				if strings.EqualFold(alias, originalAlias) {
					return false
				}
				_, m := meetings.FindByAlias(list, alias)
				return m != nil
			}

			updated, err := collectMeetingDetails(r, existing, aliasConflicts)
			if err != nil {
				return err
			}

			list[idx] = *updated
			if err := meetings.Save(list); err != nil {
				return err
			}

			fmt.Printf("\n%q updated.\n", updated.Alias)

			if err := meetings.SyncCronJob(originalAlias, *updated); err != nil {
				fmt.Printf("Warning: could not update cron job: %v\n", err)
			} else {
				fmt.Println("Cron job updated.")
			}

			return nil
		},
	}
}
