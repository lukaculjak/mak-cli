package prefill

import (
	"bufio"
	"fmt"
	"os"

	"github.com/lukaculjak/mak-cli/internal/prefills"
	"github.com/spf13/cobra"
)

func newAddCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "add",
		Short: "Add a new project with login credentials",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			var (
				pw       string
				projects []prefills.Project
				err      error
			)

			if !prefills.HasStore() {
				pw, err = setupMasterPassword()
				if err != nil {
					return err
				}
				projects = []prefills.Project{}
			} else {
				pw, projects, err = verifyAndLoad()
				if err != nil {
					return err
				}
			}

			r := bufio.NewReader(os.Stdin)

			nameConflicts := func(name string) bool {
				_, p := prefills.FindProject(projects, name)
				return p != nil
			}

			p, err := collectProjectDetails(r, nil, nameConflicts)
			if err != nil {
				return err
			}

			projects = append(projects, *p)
			if err := prefills.Save(pw, projects); err != nil {
				return err
			}

			fmt.Printf("\n%q saved.\n", p.Name)
			return nil
		},
	}
}
