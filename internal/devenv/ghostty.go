package devenv

import (
	"fmt"
	"os"
	"path/filepath"
)

func (i *installer) writeGhosttyConfig() error {
	config, err := assets.ReadFile("assets/ghostty/config")
	if err != nil {
		return err
	}
	// Ghostty loads the macOS file after XDG. Keep one copy of repeatable
	// settings such as font-family and keybind, without earlier overrides.
	for n, contents := range [][]byte{[]byte("# Luka's configuration is in ~/Library/Application Support/com.mitchellh.ghostty/config\n"), config} {
		if err := os.WriteFile(filepath.Join(i.paths[4+n].path, "config"), contents, 0o644); err != nil {
			return fmt.Errorf("writing Ghostty configuration: %w", err)
		}
	}
	return nil
}

func (i *installer) pathPackage(path string) string {
	for n, p := range i.paths {
		if p.path == path {
			if n < 4 {
				return "neovim"
			}
			return "ghostty"
		}
	}
	return ""
}

func (i *installer) validateManagedPaths(paths []installedPath) error {
	seen := map[string]bool{}
	counts := map[string]int{}
	for _, p := range paths {
		pkg := i.pathPackage(p.Path)
		if pkg == "" || seen[p.Path] {
			return fmt.Errorf("configuration directories differ from the installation record; use the same HOME and XDG settings as setup")
		}
		seen[p.Path] = true
		counts[pkg]++
	}
	if (counts["neovim"] != 0 && counts["neovim"] != 4) || (counts["ghostty"] != 0 && counts["ghostty"] != 2) {
		return fmt.Errorf("incomplete managed configuration paths")
	}
	return nil
}
