package meet

import (
	"fmt"
	"os/exec"
	"runtime"

	"github.com/lukaculjak/mak-cli/internal/meetings"
	"github.com/spf13/cobra"
)

func newOpenCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "open <alias>",
		Short: "Open a meeting link in your browser",
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

			_, m := meetings.FindByAlias(list, args[0])
			if m == nil {
				fmt.Printf("No meeting found with alias %q.\n", args[0])
				return nil
			}

			fmt.Printf("Opening %q...\n", m.Alias)
			return openURL(m.Link)
		},
	}
}

func openURL(url string) error {
	var cmd *exec.Cmd
	switch runtime.GOOS {
	case "darwin":
		cmd = exec.Command("open", url)
	case "windows":
		cmd = exec.Command("cmd", "/c", "start", url)
	default:
		cmd = exec.Command("xdg-open", url)
	}
	return cmd.Start()
}
