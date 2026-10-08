package devenv

import (
	"bytes"
	"context"
	"errors"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
)

func doctorFixture(t *testing.T) (*installer, *bytes.Buffer, *bool) {
	t.Helper()
	i, f := testInstaller(t)
	seedEnvironment(t, i)
	if err := i.setup(context.Background()); err != nil {
		t.Fatal(err)
	}
	bin := filepath.Join(i.home, "bin")
	if err := os.MkdirAll(bin, 0700); err != nil {
		t.Fatal(err)
	}
	for _, name := range []string{"nvim", "git", "node", "npm", "go", "python3", "ruby", "elixir", "ghc", "cabal", "haskell-language-server-wrapper", "rg", "fd", "fzf", "lazygit", "tree-sitter", "unzip", "brew"} {
		if err := os.WriteFile(filepath.Join(bin, name), []byte("placeholder"), 0700); err != nil {
			t.Fatal(err)
		}
	}
	i.env = setEnv(i.env, "PATH", bin)
	for _, name := range []string{"lazy.nvim", "LazyVim"} {
		if err := os.MkdirAll(filepath.Join(i.paths[1].path, "lazy", name, "lua"), 0700); err != nil {
			t.Fatal(err)
		}
	}
	var out bytes.Buffer
	i.out = &out
	started := false
	i.run = func(ctx context.Context, env []string, program string, args ...string) (string, error) {
		if err := ctx.Err(); err != nil {
			return "", err
		}
		switch filepath.Base(program) {
		case "xcrun":
			return "/compiler", nil
		case "brew":
			if !reflect.DeepEqual(args, []string{"list", "--formula", "-1"}) && !reflect.DeepEqual(args, []string{"list", "--cask", "-1"}) {
				t.Fatalf("mutating/unexpected Homebrew call: %v", args)
			}
			if envValue(env, "HOMEBREW_NO_AUTO_UPDATE") != "1" {
				t.Fatal("Homebrew updates not disabled")
			}
			return f.run(ctx, env, program, args...)
		case "sandbox-exec":
			started = true
			for _, key := range []string{"HOME", "XDG_CONFIG_HOME", "XDG_STATE_HOME", "XDG_CACHE_HOME", "MIX_HOME", "MIX_INSTALL_DIR"} {
				if strings.HasPrefix(envValue(env, key), i.home+string(os.PathSeparator)) {
					t.Fatalf("diagnostics reused original %s", key)
				}
			}
			if envValue(env, "XDG_DATA_HOME") != envValue(i.env, "XDG_DATA_HOME") {
				t.Fatal("did not check installed data")
			}
			policy, err := os.ReadFile(args[1])
			if err != nil || !strings.Contains(string(policy), "(deny file-write*)") || !strings.Contains(string(policy), "(deny network*)") {
				t.Fatal("missing read-only sandbox")
			}
			return "", os.WriteFile(envValue(env, "MAK_DOCTOR_RESULT"), []byte(`{"complete":true,"checks":[{"status":"ok","label":"Language server: gopls","detail":"Initialized"}]}`), 0600)
		default:
			t.Fatalf("unexpected command: %s %v", program, args)
		}
		return "", nil
	}
	return i, &out, &started
}

func TestDoctorReadOnlyAndDiagnostics(t *testing.T) {
	i, out, started := doctorFixture(t)
	before := snapshotFiles(t, i.home)
	if err := i.doctor(context.Background()); err != nil {
		t.Fatal(err, out.String())
	}
	if !reflect.DeepEqual(before, snapshotFiles(t, i.home)) {
		t.Fatal("doctor changed installation, backups or shell configuration")
	}
	if _, err := os.Stat("/usr/bin/sandbox-exec"); err == nil && !*started {
		t.Fatal("runtime not checked")
	}
	for _, text := range []string{"PATH: go", "Recovery files", "Bundled Neovim files"} {
		if !strings.Contains(out.String(), text) {
			t.Fatalf("missing %q: %s", text, out.String())
		}
	}
}

func TestDoctorMissingToolsAndRecoveryFiles(t *testing.T) {
	i, out, _ := doctorFixture(t)
	if err := os.Remove(filepath.Join(i.home, "bin/node")); err != nil {
		t.Fatal(err)
	}
	record := mustRecord(t, i)
	if err := os.RemoveAll(record.Paths[0].Backup); err != nil {
		t.Fatal(err)
	}
	if err := i.doctor(context.Background()); !errors.Is(err, ErrUnhealthy) {
		t.Fatalf("error=%v", err)
	}
	for _, text := range []string{"PATH: node: Executable is unavailable", "--install node", "Recovery files", "Suggested repair:"} {
		if !strings.Contains(out.String(), text) {
			t.Fatalf("missing %q: %s", text, out.String())
		}
	}
}

func TestDoctorEditsAndSetupLockWarnWithoutStartingNeovim(t *testing.T) {
	i, out, started := doctorFixture(t)
	if err := os.WriteFile(filepath.Join(i.paths[0].path, "init.lua"), []byte("-- local edits"), 0600); err != nil {
		t.Fatal(err)
	}
	lock := filepath.Join(envValue(i.env, "XDG_CONFIG_HOME"), "mak/dev-setup.lock")
	f, err := acquireSetupLock(lock)
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()
	if err := i.doctor(context.Background()); err != nil {
		t.Fatal(err, out.String())
	}
	if *started || !strings.Contains(out.String(), "Local edits") || !strings.Contains(out.String(), "runtime checks skipped") {
		t.Fatal(out.String())
	}
}

func TestDoctorFailedOrIncompleteRuntime(t *testing.T) {
	if _, err := os.Stat("/usr/bin/sandbox-exec"); err != nil {
		t.Skip("macOS runtime checks")
	}
	for _, result := range []string{`{"complete":false,"checks":[{"status":"ok","label":"Startup"}]}`, `{"complete":true,"checks":[{"status":"error","label":"Language server: gopls","detail":"Timed out","repair":"mak setup dev"}]}`} {
		t.Run(result, func(t *testing.T) {
			i, out, _ := doctorFixture(t)
			run := i.run
			i.run = func(ctx context.Context, env []string, program string, args ...string) (string, error) {
				if filepath.Base(program) == "sandbox-exec" {
					return "", os.WriteFile(envValue(env, "MAK_DOCTOR_RESULT"), []byte(result), 0600)
				}
				return run(ctx, env, program, args...)
			}
			if err := i.doctor(context.Background()); !errors.Is(err, ErrUnhealthy) {
				t.Fatalf("error=%v: %s", err, out.String())
			}
		})
	}
}

func TestMixCacheCopySelectsOnlyElixirLS(t *testing.T) {
	i, _ := testInstaller(t)
	cache := filepath.Join(i.home, "cache")
	i.env = setEnv(i.env, "MIX_INSTALL_DIR", cache)
	for _, rel := range []string{"version/ls/deps/elixir_ls/.git", "version/other/deps/other"} {
		if err := os.MkdirAll(filepath.Join(cache, rel), 0700); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(cache, rel, "example"), []byte("cached"), 0700); err != nil {
			t.Fatal(err)
		}
	}
	before := snapshotFiles(t, i.home)
	tmp := t.TempDir()
	if err := i.copyMixCaches(tmp); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(filepath.Join(tmp, "mix-installs/version/ls/deps/elixir_ls/.git/example")); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(filepath.Join(tmp, "mix-installs/version/other")); !os.IsNotExist(err) {
		t.Fatal("copied unrelated project")
	}
	if !reflect.DeepEqual(before, snapshotFiles(t, i.home)) {
		t.Fatal("modified original cache")
	}
}

func TestDoctorInvalidRecordIsPreserved(t *testing.T) {
	i, out, _ := doctorFixture(t)
	if err := os.WriteFile(i.recordPath(), []byte("invalid record"), 0600); err != nil {
		t.Fatal(err)
	}
	before := snapshotFiles(t, i.home)
	if err := i.doctor(context.Background()); !errors.Is(err, ErrUnhealthy) {
		t.Fatalf("error=%v: %s", err, out.String())
	}
	if !strings.Contains(out.String(), "Recover the original record") || !reflect.DeepEqual(before, snapshotFiles(t, i.home)) {
		t.Fatal("doctor did not preserve an invalid record")
	}
}

func TestMixXDGCacheSnapshot(t *testing.T) {
	i, _ := testInstaller(t)
	if err := i.resolvePaths(); err != nil {
		t.Fatal(err)
	}
	i.env = setEnv(i.env, "MIX_XDG", "1")
	mixHome := filepath.Join(envValue(i.env, "XDG_DATA_HOME"), "mix")
	if err := os.MkdirAll(mixHome, 0700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(mixHome, "archive"), []byte("archive"), 0600); err != nil {
		t.Fatal(err)
	}
	tmp := t.TempDir()
	if err := i.copyMixCaches(tmp); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(filepath.Join(tmp, "mix-home/archive")); err != nil {
		t.Fatal(err)
	}
}

func snapshotFiles(t *testing.T, root string) map[string]string {
	t.Helper()
	files := map[string]string{}
	err := filepath.WalkDir(root, func(path string, d os.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if !d.IsDir() {
			b, err := os.ReadFile(path)
			if err != nil {
				return err
			}
			files[path] = string(b)
		}
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	return files
}
