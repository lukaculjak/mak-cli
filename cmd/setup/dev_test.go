package setup

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestDevConfirmationDeclineHasNoSetupEffects(t *testing.T) {
	for _, args := range [][]string{{}, {"-i", "nvim"}, {"--install", "neovim"}, {"--install", "ghostty"}} {
		t.Run(strings.Join(args, " "), func(t *testing.T) {
			home := t.TempDir()
			t.Setenv("HOME", home)
			for _, key := range []string{"XDG_CONFIG_HOME", "XDG_DATA_HOME", "XDG_STATE_HOME", "XDG_CACHE_HOME"} {
				t.Setenv(key, filepath.Join(home, key))
			}
			cmd := newDevCmd()
			var out bytes.Buffer
			cmd.SetOut(&out)
			cmd.SetErr(&out)
			cmd.SetIn(strings.NewReader("no\n"))
			cmd.SetArgs(args)
			if err := cmd.Execute(); err != nil {
				t.Fatal(err)
			}
			for _, text := range []string{"--list", "[Y/n]", "Installation cancelled. Nothing was changed."} {
				if !strings.Contains(out.String(), text) {
					t.Fatalf("missing %q: %s", text, out.String())
				}
			}
			entries, err := os.ReadDir(home)
			if err != nil || len(entries) != 0 {
				t.Fatalf("declining setup changed home: %v %v", entries, err)
			}
		})
	}
}

func TestDevRejectsConflictingActionsAndUnknownPackages(t *testing.T) {
	for _, args := range [][]string{
		{"-l", "-i", "node"}, {"--uninstall", "-r", "nvim"}, {"-i", "node", "-r", "node"},
		{"-i", "unknown"}, {"-r", "unknown"}, {"--install="}, {"--remove="},
	} {
		t.Run(strings.Join(args, " "), func(t *testing.T) {
			cmd := newDevCmd()
			var out bytes.Buffer
			cmd.SetOut(&out)
			cmd.SetErr(&out)
			cmd.SetIn(strings.NewReader(""))
			cmd.SetArgs(args)
			if err := cmd.Execute(); err == nil {
				t.Fatal("invalid action accepted")
			}
			if strings.Contains(out.String(), "[Y/n]") {
				t.Fatal("invalid action prompted for installation")
			}
		})
	}
}
