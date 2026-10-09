package cmd

import (
	"bytes"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/spf13/cobra"
)

func completionCommand(out *bytes.Buffer) *cobra.Command {
	root := &cobra.Command{Use: "mak"}
	root.AddCommand(&cobra.Command{Use: "setup"})
	root.SetOut(out)
	root.SetErr(out)
	configureCompletion(root)
	return root
}

func TestZshCompletionGenerationDoesNotEditProfiles(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	t.Setenv("ZDOTDIR", "")
	if err := os.Unsetenv("ZDOTDIR"); err != nil {
		t.Fatal(err)
	}
	for _, args := range [][]string{{"completion", "zsh"}, {"completion", "zsh", "--no-descriptions"}} {
		var out bytes.Buffer
		cmd := completionCommand(&out)
		cmd.SetArgs(args)
		if err := cmd.Execute(); err != nil {
			t.Fatal(err)
		}
		if !strings.HasPrefix(out.String(), "#compdef mak\n") || strings.Contains(out.String(), "Script printed only") {
			t.Fatalf("invalid generated script: %s", &out)
		}
		if zsh, err := exec.LookPath("zsh"); err == nil {
			check := exec.Command(zsh, "-n")
			check.Stdin = &out
			if result, err := check.CombinedOutput(); err != nil {
				t.Fatalf("invalid zsh syntax: %v: %s", err, result)
			}
		}
	}
	entries, err := os.ReadDir(home)
	if err != nil || len(entries) != 0 {
		t.Fatalf("generation changed home: %v %v", entries, err)
	}
}

func TestZshCompletionInstallPreservesProfileAndLoadsInNewShell(t *testing.T) {
	for _, useZdotdir := range []bool{false, true} {
		t.Run(map[bool]string{false: "HOME", true: "ZDOTDIR"}[useZdotdir], func(t *testing.T) {
			home := t.TempDir()
			t.Setenv("HOME", home)
			t.Setenv("ZDOTDIR", "")
			if err := os.Unsetenv("ZDOTDIR"); err != nil {
				t.Fatal(err)
			}
			dir := home
			if useZdotdir {
				dir = filepath.Join(home, "shell's $(printf unexpected) settings")
				t.Setenv("ZDOTDIR", dir)
			}
			if err := os.MkdirAll(dir, 0700); err != nil {
				t.Fatal(err)
			}
			profile := filepath.Join(dir, ".zshrc")
			original := "export MAK_EXISTING_SETTING=preserved" // No trailing newline.
			if err := os.WriteFile(profile, []byte(original), 0600); err != nil {
				t.Fatal(err)
			}
			for range 2 {
				var out bytes.Buffer
				cmd := completionCommand(&out)
				cmd.SetArgs([]string{"completion", "zsh", "--install"})
				if err := cmd.Execute(); err != nil {
					t.Fatal(err)
				}
			}
			contents, err := os.ReadFile(profile)
			if err != nil || string(contents) != original+zshCompletionBlock {
				t.Fatalf("profile was replaced or duplicated: %q %v", contents, err)
			}
			info, err := os.Stat(profile)
			if err != nil || info.Mode().Perm() != 0600 {
				t.Fatalf("profile permissions changed: %v %v", info, err)
			}
			zsh, err := exec.LookPath("zsh")
			if err != nil {
				t.Skip("zsh unavailable for shell integration check")
			}
			var script bytes.Buffer
			cmd := completionCommand(&script)
			cmd.SetArgs([]string{"completion", "zsh"})
			if err := cmd.Execute(); err != nil {
				t.Fatal(err)
			}
			bin := filepath.Join(home, "bin")
			if err := os.Mkdir(bin, 0700); err != nil {
				t.Fatal(err)
			}
			// A local stub supplies the real generated script without a build or network.
			stub := "#!/bin/sh\ncat <<'MAK_COMPLETION_EOF'\n" + script.String() + "MAK_COMPLETION_EOF\n"
			if err := os.WriteFile(filepath.Join(bin, "mak"), []byte(stub), 0700); err != nil {
				t.Fatal(err)
			}
			check := exec.Command(zsh, "-i", "-c", `[[ "$MAK_EXISTING_SETTING" == preserved && "${_comps[mak]}" == _mak ]] && (( $+functions[_mak] ))`)
			check.Env = append(os.Environ(), "PATH="+bin+":/usr/bin:/bin")
			if result, err := check.CombinedOutput(); err != nil {
				t.Fatalf("new shell did not register mak completion: %v: %s", err, result)
			}
		})
	}
}
