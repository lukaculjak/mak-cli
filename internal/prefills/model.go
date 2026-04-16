package prefills

import (
	"fmt"
	"strings"
)

type Domain struct {
	Label    string `json:"label"`
	URL      string `json:"url"`
	Email    string `json:"email"`
	Password string `json:"password"`
}

type Project struct {
	Name    string   `json:"name"`
	Domains []Domain `json:"domains"`
}

func FindProject(list []Project, name string) (int, *Project) {
	lower := strings.ToLower(name)
	for i := range list {
		if strings.ToLower(list[i].Name) == lower {
			return i, &list[i]
		}
	}
	return -1, nil
}

func (p *Project) Summary() string {
	var sb strings.Builder
	sb.WriteString(fmt.Sprintf("  Project: %s\n", p.Name))
	for _, d := range p.Domains {
		sb.WriteString(fmt.Sprintf("  [%s]\n", d.Label))
		sb.WriteString(fmt.Sprintf("    URL:      %s\n", d.URL))
		sb.WriteString(fmt.Sprintf("    Email:    %s\n", d.Email))
		sb.WriteString(fmt.Sprintf("    Password: %s  (only shown locally)\n", d.Password))
	}
	return sb.String()
}
