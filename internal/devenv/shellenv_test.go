package devenv

import (
	"bytes"
	"context"
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
)

func shellenvFixture(t *testing.T) (*installer, string) {
	t.Helper()
	prefix := filepath.Join(t.TempDir(), "brew's $(printf unexpected) tools")
	for _, rel := range []string{"opt/ruby/bin", "opt/python/libexec/bin", "bin", "sbin"} {
		if err := os.MkdirAll(filepath.Join(prefix, rel), 0700); err != nil {
			t.Fatal(err)
		}
	}
	i := &installer{brew: filepath.Join(prefix, "bin/brew"), env: []string{"SHELL=/bin/zsh", "PATH=/usr/bin:/bin"}}
	i.run = func(ctx context.Context, env []string, program string, args ...string) (string, error) {
		if envValue(env, "HOMEBREW_NO_AUTO_UPDATE") != "1" || envValue(env, "HOMEBREW_NO_ANALYTICS") != "1" {
			t.Fatal("Homebrew updates/analytics not disabled")
		}
		if err := ctx.Err(); err != nil {
			return "", err
		}
		switch {
		case reflect.DeepEqual(args, []string{"--prefix"}):
			return prefix + "\n", nil
		case reflect.DeepEqual(args, []string{"shellenv", "bash"}):
			return "export HOMEBREW_PREFIX='test-prefix';\n", nil
		default:
			t.Fatalf("unexpected command: %s %v", program, args)
			return "", nil
		}
	}
	return i, prefix
}

func TestShellenvWorksInCurrentShellWithoutPathDuplicates(t *testing.T) {
	for _, shell := range []string{"/bin/bash", "/bin/zsh"} {
		t.Run(shell, func(t *testing.T) {
			if _, err := os.Stat(shell); err != nil {
				t.Skip("shell unavailable")
			}
			i, prefix := shellenvFixture(t)
			i.env = setEnv(i.env, "SHELL", shell)
			oldBin := filepath.Join(t.TempDir(), "old-bin")
			if err := os.MkdirAll(oldBin, 0700); err != nil {
				t.Fatal(err)
			}
			for dir, result := range map[string]string{oldBin: "old", filepath.Join(prefix, "bin"): "new"} {
				if err := os.WriteFile(filepath.Join(dir, "mak-shellenv-test-tool"), []byte("#!/bin/sh\nprintf '"+result+"\\n'\n"), 0700); err != nil {
					t.Fatal(err)
				}
			}
			originalPath := "/usr/bin:/bin:" + oldBin
			i.env = setEnv(i.env, "PATH", originalPath)
			var first, second bytes.Buffer
			if err := i.shellenv(context.Background(), &first); err != nil {
				t.Fatal(err)
			}
			cmd := exec.Command(shell, "-c", "mak-shellenv-test-tool\neval \"$MAK_TEST_SCRIPT\"\nmak-shellenv-test-tool\nprintf '%s\\n%s\\n' \"$PATH\" \"$HOMEBREW_PREFIX\"")
			cmd.Env = append(os.Environ(), "PATH="+originalPath, "MAK_TEST_SCRIPT="+first.String())
			out, err := cmd.CombinedOutput()
			if err != nil {
				t.Fatalf("%v: %s", err, out)
			}
			lines := strings.Split(strings.TrimSpace(string(out)), "\n")
			if len(lines) != 4 || lines[0] != "old" || lines[1] != "new" || lines[3] != "test-prefix" || !strings.HasSuffix(lines[2], ":"+originalPath) {
				t.Fatalf("failed to refresh current shell: %s", out)
			}
			i.env = setEnv(i.env, "PATH", lines[2])
			if err := i.shellenv(context.Background(), &second); err != nil {
				t.Fatal(err)
			}
			if first.String() != second.String() {
				t.Fatalf("repeated invocation changed PATH:\n%s\n%s", first.String(), second.String())
			}
		})
	}
}

func TestShellenvFailuresProduceNoScript(t *testing.T) {
	for _, failure := range []string{"prefix command", "invalid prefix", "shellenv command", "unsupported shell", "cancelled"} {
		t.Run(failure, func(t *testing.T) {
			i, _ := shellenvFixture(t)
			run := i.run
			i.run = func(ctx context.Context, env []string, program string, args ...string) (string, error) {
				if (failure == "prefix command" && args[0] == "--prefix") || (failure == "shellenv command" && args[0] == "shellenv") {
					return "", errors.New("simulated failure")
				}
				if failure == "invalid prefix" && args[0] == "--prefix" {
					return "/brew\ninvalid", nil
				}
				return run(ctx, env, program, args...)
			}
			if failure == "unsupported shell" {
				i.env = setEnv(i.env, "SHELL", "/bin/fish")
			}
			ctx, cancel := context.WithCancel(context.Background())
			defer cancel()
			if failure == "cancelled" {
				cancel()
			}
			var out bytes.Buffer
			if err := i.shellenv(ctx, &out); err == nil || out.Len() != 0 {
				t.Fatalf("error=%v output=%s", err, out.String())
			}
		})
	}
}

func TestShellenvPreservesProjectRuntimePrecedence(t *testing.T) {
	for _, shell := range []string{"/bin/bash", "/bin/zsh"} {
		t.Run(shell, func(t *testing.T) {
			if _, err := os.Stat(shell); err != nil {
				t.Skip("shell unavailable")
			}
			i, prefix := shellenvFixture(t)
			project := filepath.Join(t.TempDir(), "project tools")
			if err := os.MkdirAll(project, 0700); err != nil {
				t.Fatal(err)
			}
			for _, tool := range []string{"python", "python3", "node", "ruby"} {
				for dir, label := range map[string]string{project: "project", filepath.Join(prefix, "bin"): "brew", filepath.Join(prefix, "opt/ruby/bin"): "brew", filepath.Join(prefix, "opt/python/libexec/bin"): "brew"} {
					if err := os.WriteFile(filepath.Join(dir, tool), []byte("#!/bin/sh\nprintf '"+label+"\\n'\n"), 0700); err != nil {
						t.Fatal(err)
					}
				}
			}
			i.env = setEnv(i.env, "PATH", filepath.Join(prefix, "bin")+":"+project+":/usr/bin:/bin")
			var script bytes.Buffer
			if err := i.shellenv(context.Background(), &script); err != nil {
				t.Fatal(err)
			}
			cmd := exec.Command(shell, "-c", "eval \"$MAK_TEST_SCRIPT\"; python; python3; node; ruby")
			cmd.Env = append(os.Environ(), "MAK_TEST_SCRIPT="+script.String())
			out, err := cmd.CombinedOutput()
			if err != nil || string(out) != "project\nproject\nproject\nproject\n" {
				t.Fatalf("project runtime overridden: %v %s", err, out)
			}
		})
	}
}
