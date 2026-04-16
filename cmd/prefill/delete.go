package prefill

import (
	"bufio"
	"fmt"
	"os"

	"github.com/lukaculjak/mak-cli/internal/prefills"
	"github.com/spf13/cobra"
)

func newDeleteCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "delete <project-name>",
		Short: "Delete a project and all its credentials",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			if !prefills.HasStore() {
				fmt.Println("No prefills configured yet.")
				return nil
			}

			pw, projects, err := verifyAndLoad()
			if err != nil {
				return err
			}

			idx, existing := prefills.FindProject(projects, args[0])
			if existing == nil {
				return fmt.Errorf("project %q not found", args[0])
			}

			r := bufio.NewReader(os.Stdin)

			fmt.Printf("\nYou are about to delete %q and all its domains:\n", existing.Name)
			for _, d := range existing.Domains {
				fmt.Printf("  [%s] %s\n", d.Label, d.URL)
			}
			fmt.Println()

			ok, err := promptConfirm(r, "Are you sure?")
			if err != nil {
				return err
			}
			if !ok {
				fmt.Println("Cancelled.")
				return nil
			}

			projects = append(projects[:idx], projects[idx+1:]...)
			if err := prefills.Save(pw, projects); err != nil {
				return err
			}

			fmt.Printf("%q deleted.\n", args[0])
			return nil
		},
	}
}
