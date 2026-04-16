package prefill

import (
	"fmt"

	"github.com/lukaculjak/mak-cli/internal/prefills"
	"github.com/spf13/cobra"
)

func newListCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "list",
		Short: "List all projects and their domains",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			if !prefills.HasStore() {
				fmt.Println("No prefills configured yet. Run `mak prefill add` to get started.")
				return nil
			}

			_, projects, err := verifyAndLoad()
			if err != nil {
				return err
			}

			if len(projects) == 0 {
				fmt.Println("No projects found. Run `mak prefill add` to add one.")
				return nil
			}

			fmt.Println()
			for _, p := range projects {
				fmt.Println(p.Summary())
			}
			return nil
		},
	}
}
