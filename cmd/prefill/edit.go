package prefill

import (
	"bufio"
	"fmt"
	"os"
	"strconv"
	"strings"

	"github.com/lukaculjak/mak-cli/internal/prefills"
	"github.com/spf13/cobra"
)

func newEditCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "edit <project-name>",
		Short: "Edit an existing project",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			if !prefills.HasStore() {
				fmt.Println("No prefills configured yet. Run `mak prefill add` to get started.")
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

			fmt.Println()
			fmt.Printf("Editing project: %s\n", existing.Name)
			fmt.Println()
			fmt.Println("What would you like to edit?")
			fmt.Println("  1) Project name")
			fmt.Println("  2) Edit a domain")
			fmt.Println("  3) Add a domain")
			fmt.Println("  4) Remove a domain")
			fmt.Println("  5) Replace all domains")

			updated := *existing

			for {
				fmt.Print("\nChoice (1-5): ")
				line, err := r.ReadString('\n')
				if err != nil {
					return err
				}
				choice := strings.TrimSpace(line)

				switch choice {
				case "1":
					name, err := promptString(r, "Project name", existing.Name)
					if err != nil {
						return err
					}
					updated.Name = name

				case "2":
					if len(updated.Domains) == 0 {
						fmt.Println("  No domains to edit.")
						continue
					}
					for i, d := range updated.Domains {
						fmt.Printf("  %d. [%s] %s\n", i+1, d.Label, d.URL)
					}
					n, err := pickDomainIndex(r, len(updated.Domains))
					if err != nil {
						return err
					}
					edited, err := editSingleDomain(r, updated.Domains[n])
					if err != nil {
						return err
					}
					updated.Domains[n] = *edited

				case "3":
					d, err := collectSingleDomain(r)
					if err != nil {
						return err
					}
					updated.Domains = append(updated.Domains, *d)

				case "4":
					if len(updated.Domains) == 0 {
						fmt.Println("  No domains to remove.")
						continue
					}
					for i, d := range updated.Domains {
						fmt.Printf("  %d. [%s] %s\n", i+1, d.Label, d.URL)
					}
					n, err := pickDomainIndex(r, len(updated.Domains))
					if err != nil {
						return err
					}
					updated.Domains = append(updated.Domains[:n], updated.Domains[n+1:]...)
					fmt.Println("  Domain removed.")

				case "5":
					updated.Domains = nil
					domains, err := collectDomains(r, nil)
					if err != nil {
						return err
					}
					updated.Domains = domains

				default:
					fmt.Println("  Please enter a number between 1 and 5.")
					continue
				}

				// Show summary and confirm
				fmt.Println()
				fmt.Println("Updated project:")
				fmt.Println(updated.Summary())

				ok, err := promptConfirm(r, "Save changes?")
				if err != nil {
					return err
				}
				if ok {
					break
				}
			}

			projects[idx] = updated
			if err := prefills.Save(pw, projects); err != nil {
				return err
			}

			fmt.Printf("\n%q updated.\n", updated.Name)
			return nil
		},
	}
}

func pickDomainIndex(r *bufio.Reader, count int) (int, error) {
	for {
		fmt.Printf("Domain number (1-%d): ", count)
		line, err := r.ReadString('\n')
		if err != nil {
			return 0, err
		}
		n, err := strconv.Atoi(strings.TrimSpace(line))
		if err != nil || n < 1 || n > count {
			fmt.Printf("  Please enter a number between 1 and %d.\n", count)
			continue
		}
		return n - 1, nil
	}
}
