package prefill

import (
	"bufio"
	"fmt"
	"os"
	"strings"

	"github.com/lukaculjak/mak-cli/internal/prefills"
	"golang.org/x/term"
)

func promptString(r *bufio.Reader, label, current string) (string, error) {
	for {
		if current != "" {
			fmt.Printf("%s [%s]: ", label, current)
		} else {
			fmt.Printf("%s: ", label)
		}
		input, err := r.ReadString('\n')
		if err != nil {
			return "", err
		}
		input = strings.TrimSpace(input)
		if input == "" && current != "" {
			return current, nil
		}
		if input != "" {
			return input, nil
		}
		fmt.Println("  This field cannot be empty.")
	}
}

func promptPassword(label, current string) (string, error) {
	for {
		if current != "" {
			fmt.Printf("%s [leave blank to keep current]: ", label)
		} else {
			fmt.Printf("%s: ", label)
		}
		pw, err := term.ReadPassword(int(os.Stdin.Fd()))
		fmt.Println()
		if err != nil {
			return "", err
		}
		s := strings.TrimSpace(string(pw))
		if s == "" && current != "" {
			return current, nil
		}
		if s != "" {
			return s, nil
		}
		fmt.Println("  Password cannot be empty.")
	}
}

func promptConfirm(r *bufio.Reader, label string) (bool, error) {
	for {
		fmt.Printf("%s (y/n): ", label)
		input, err := r.ReadString('\n')
		if err != nil {
			return false, err
		}
		switch strings.ToLower(strings.TrimSpace(input)) {
		case "y", "yes":
			return true, nil
		case "n", "no":
			return false, nil
		default:
			fmt.Println("  Please enter y or n.")
		}
	}
}

// collectProjectDetails interactively collects or edits a project and its domains.
// Pass existing to pre-fill values for edit mode.
func collectProjectDetails(r *bufio.Reader, existing *prefills.Project, nameConflicts func(string) bool) (*prefills.Project, error) {
	var defaultName string
	var defaultDomains []prefills.Domain

	if existing != nil {
		defaultName = existing.Name
		defaultDomains = existing.Domains
	}

	for {
		fmt.Println()

		// Project name
		name, err := promptString(r, "Project name", defaultName)
		if err != nil {
			return nil, err
		}
		if existing == nil && nameConflicts(name) {
			fmt.Printf("  A project named %q already exists.\n", name)
			defaultName = name
			continue
		}

		// Domains
		domains, err := collectDomains(r, defaultDomains)
		if err != nil {
			return nil, err
		}

		p := &prefills.Project{Name: name, Domains: domains}

		// Summary
		fmt.Println()
		fmt.Println("Does this look okay to you?")
		fmt.Println(p.Summary())

		ok, err := promptConfirm(r, "Confirm")
		if err != nil {
			return nil, err
		}
		if ok {
			return p, nil
		}

		defaultName = name
		defaultDomains = domains
	}
}

func collectDomains(r *bufio.Reader, existing []prefills.Domain) ([]prefills.Domain, error) {
	domains := make([]prefills.Domain, len(existing))
	copy(domains, existing)

	if len(domains) > 0 {
		fmt.Printf("\nCurrent domains (%d):\n", len(domains))
		for i, d := range domains {
			fmt.Printf("  %d. [%s] %s\n", i+1, d.Label, d.URL)
		}
		fmt.Println()
	}

	for {
		d, err := collectSingleDomain(r)
		if err != nil {
			return nil, err
		}
		domains = append(domains, *d)

		more, err := promptConfirm(r, "Add another domain to this project?")
		if err != nil {
			return nil, err
		}
		if !more {
			break
		}
	}
	return domains, nil
}

func collectSingleDomain(r *bufio.Reader) (*prefills.Domain, error) {
	fmt.Println()
	label, err := promptString(r, "Domain label (e.g. Staging, Production, Localhost)", "")
	if err != nil {
		return nil, err
	}
	url, err := promptString(r, "URL", "")
	if err != nil {
		return nil, err
	}
	email, err := promptString(r, "Email / username", "")
	if err != nil {
		return nil, err
	}
	password, err := promptPassword("Password", "")
	if err != nil {
		return nil, err
	}
	return &prefills.Domain{Label: label, URL: url, Email: email, Password: password}, nil
}

func editSingleDomain(r *bufio.Reader, d prefills.Domain) (*prefills.Domain, error) {
	fmt.Println()
	label, err := promptString(r, "Domain label", d.Label)
	if err != nil {
		return nil, err
	}
	url, err := promptString(r, "URL", d.URL)
	if err != nil {
		return nil, err
	}
	email, err := promptString(r, "Email / username", d.Email)
	if err != nil {
		return nil, err
	}
	password, err := promptPassword("Password", d.Password)
	if err != nil {
		return nil, err
	}
	return &prefills.Domain{Label: label, URL: url, Email: email, Password: password}, nil
}
