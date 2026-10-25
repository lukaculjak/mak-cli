package devenv

import (
	"os"
	"path/filepath"
	"strings"
)

// Commands must all be available before their Homebrew package can be skipped.
// Libraries and fonts have no executable; their Homebrew inventory still applies.
var packageCommands = map[string][]string{
	"neovim": {"nvim"}, "git": {"git"}, "node": {"node", "npm"},
	"go": {"go"}, "python": {"python3"}, "ruby": {"ruby", "gem"},
	"elixir": {"elixir", "mix"}, "ghc": {"ghc"}, "cabal-install": {"cabal"},
	"haskell-language-server": {"haskell-language-server-wrapper"},
	"ripgrep":                 {"rg"}, "fd": {"fd"}, "fzf": {"fzf"}, "lazygit": {"lazygit"},
	"tree-sitter-cli": {"tree-sitter"}, "unzip": {"unzip"}, "luarocks": {"luarocks"},
	"terminal-notifier": {"terminal-notifier"}, timerFormula: {"timer"},
	"ghostty": {"ghostty"}, "rclone": {"rclone"},
}

func (i *installer) availablePackage(name string) string {
	commands := packageCommands[name]
	if len(commands) == 0 {
		return ""
	}
	var found []string
	for _, command := range commands {
		path := executable(envValue(i.env, "PATH"), command)
		if path == "" && name == "ghostty" {
			applications := i.applications
			if applications == "" {
				applications = "/Applications"
			}
			for _, dir := range []string{filepath.Join(i.home, "Applications"), applications} {
				candidate := filepath.Join(dir, "Ghostty.app", "Contents", "MacOS", "ghostty")
				if st, err := os.Stat(candidate); err == nil && st.Mode().IsRegular() && st.Mode().Perm()&0111 != 0 {
					path = candidate
					break
				}
			}
		}
		if path == "" {
			return ""
		}
		found = append(found, path)
	}
	return strings.Join(found, ", ")
}

// Resolve against the setup environment, including existing version managers.
func (i *installer) toolPath(prefix, name string) string {
	if path := executable(envValue(i.env, "PATH"), name); path != "" {
		return path
	}
	return filepath.Join(prefix, "bin", name)
}
