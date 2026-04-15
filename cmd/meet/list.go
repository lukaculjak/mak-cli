package meet

import (
	"fmt"

	"github.com/lukaculjak/mak-cli/internal/meetings"
	"github.com/spf13/cobra"
)

func newListCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "list [alias]",
		Short: "List all meetings, or show details of a single meeting",
		Args:  cobra.MaximumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			list, err := meetings.Load()
			if err != nil {
				return err
			}

			if len(args) == 1 {
				return showMeeting(list, args[0])
			}
			return listMeetings(list)
		},
	}
}

func listMeetings(list []meetings.Meeting) error {
	if len(list) == 0 {
		fmt.Println("No meetings yet. Use `mak meet add` to create one.")
		return nil
	}
	fmt.Println("All scheduled meetings:")
	for i, m := range list {
		fmt.Printf("  %d  %-24s %s\n", i+1, m.Alias, m.FormatSchedule())
	}
	return nil
}

func showMeeting(list []meetings.Meeting, alias string) error {
	_, m := meetings.FindByAlias(list, alias)
	if m == nil {
		fmt.Printf("No meeting found with alias %q.\n", alias)
		return nil
	}
	fmt.Printf("Meeting: %s\n", m.Alias)
	fmt.Printf("Link:    %s\n", m.Link)
	fmt.Printf("%s\n", m.FormatScheduleDetailed())
	return nil
}
