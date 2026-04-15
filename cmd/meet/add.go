package meet

import (
	"bufio"
	"fmt"
	"os"

	"github.com/lukaculjak/mak-cli/internal/meetings"
	"github.com/spf13/cobra"
)

func newAddCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "add",
		Short: "Add a new meeting",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			list, err := meetings.Load()
			if err != nil {
				return err
			}

			r := bufio.NewReader(os.Stdin)

			aliasConflicts := func(alias string) bool {
				_, m := meetings.FindByAlias(list, alias)
				return m != nil
			}

			m, err := collectMeetingDetails(r, nil, aliasConflicts)
			if err != nil {
				return err
			}

			list = append(list, *m)
			if err := meetings.Save(list); err != nil {
				return err
			}

			fmt.Printf("\n%q added to your meetings.\n", m.Alias)

			if err := meetings.SyncCronJob(m.Alias, *m); err != nil {
				fmt.Printf("Warning: could not create cron job: %v\n", err)
			} else {
				fmt.Println("Cron job created, mak will open it automatically at the scheduled time.")
			}

			return nil
		},
	}
}
