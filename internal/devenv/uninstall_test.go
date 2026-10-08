package devenv

import (
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
)

func nextInvocation(i *installer, f *fakeSystem) *installer {
	return &installer{home: i.home, env: i.env, brew: i.brew, run: f.run, out: io.Discard}
}

func assertOriginals(t *testing.T, paths []string) {
	t.Helper()
	for n, p := range paths {
		b, err := os.ReadFile(filepath.Join(p, "original"))
		if err != nil || string(b) != fmt.Sprint(n) {
			t.Fatalf("original %s: %q %v", p, b, err)
		}
	}
}

func TestUninstallRestoresFilesAndPreservesOtherPackagesAndProfileEdits(t *testing.T) {
	i, f := testInstaller(t)
	paths := seedEnvironment(t, i)
	profile := filepath.Join(i.home, ".zprofile")
	before := "export EXISTING=value\n"
	if err := os.WriteFile(profile, []byte(before), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := i.setup(context.Background()); err != nil {
		t.Fatal(err)
	}
	st, err := os.Stat(i.recordPath())
	if err != nil || st.Mode().Perm() != 0o600 {
		t.Fatalf("record permissions: %v %v", st, err)
	}
	f.formulae["installed-later"] = true
	f.casks["another-app"] = true
	b, _ := os.ReadFile(profile)
	if err := os.WriteFile(profile, append(b, []byte("# later edit\n")...), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := nextInvocation(i, f).uninstall(context.Background()); err != nil {
		t.Fatal(err)
	}
	assertOriginals(t, paths)
	if !reflect.DeepEqual(f.formulae, map[string]bool{"git": true, "shared-dependency": true, "installed-later": true}) || !reflect.DeepEqual(f.casks, map[string]bool{"existing-app": true, "another-app": true}) {
		t.Fatalf("wrong remaining packages: %v %v", f.formulae, f.casks)
	}
	b, _ = os.ReadFile(profile)
	if string(b) != before+"# later edit\n" {
		t.Fatalf("profile edits lost: %s", b)
	}
	st, _ = os.Stat(profile)
	if st.Mode().Perm() != 0o600 {
		t.Fatal("profile permissions changed")
	}
	if _, err := os.Stat(i.recordPath()); !os.IsNotExist(err) {
		t.Fatalf("record not removed: %v", err)
	}
	for _, call := range f.calls {
		if strings.HasPrefix(call, "brew uninstall") && strings.Contains(call, "--ignore-dependencies") {
			t.Fatalf("uninstall ignored dependencies: %s", call)
		}
	}
}

func TestFreshUninstallRemovesDirectoriesAndOnlyItsShellBlock(t *testing.T) {
	for _, preexistingShellenv := range []bool{false, true} {
		t.Run(fmt.Sprint(preexistingShellenv), func(t *testing.T) {
			i, f := testInstaller(t)
			f.casks[font] = true
			profile := filepath.Join(i.home, ".zprofile")
			original := "eval \"$('/fake/brew/bin/brew' shellenv)\"\n"
			if preexistingShellenv {
				if err := os.WriteFile(profile, []byte(original), 0o600); err != nil {
					t.Fatal(err)
				}
			}
			if err := i.setup(context.Background()); err != nil {
				t.Fatal(err)
			}
			if err := nextInvocation(i, f).uninstall(context.Background()); err != nil {
				t.Fatal(err)
			}
			for _, p := range i.paths {
				if _, err := os.Lstat(p.path); !os.IsNotExist(err) {
					t.Fatalf("directory remains: %s %v", p.path, err)
				}
			}
			b, err := os.ReadFile(profile)
			if preexistingShellenv {
				if err != nil || string(b) != original {
					t.Fatalf("preexisting shellenv changed: %s %v", b, err)
				}
			} else if !os.IsNotExist(err) {
				t.Fatalf("new shell profile remains: %v", err)
			}
			if !f.casks[font] {
				t.Fatal("preexisting font removed")
			}
		})
	}
}

func TestRepeatedSetupRestoresTheOriginalBaseline(t *testing.T) {
	i, f := testInstaller(t)
	paths := seedEnvironment(t, i)
	if err := i.setup(context.Background()); err != nil {
		t.Fatal(err)
	}
	run := f.run
	j := nextInvocation(i, f)
	j.run = func(ctx context.Context, env []string, program string, args ...string) (string, error) {
		result, err := run(ctx, env, program, args...)
		if args[0] == "install" && args[1] == "--formula" {
			f.formulae["new-package-on-rerun"] = true
		}
		return result, err
	}
	if err := j.setup(context.Background()); err != nil {
		t.Fatal(err)
	}
	// Failure of a later setup must retain the successful installation record.
	before, _ := os.ReadFile(i.recordPath())
	f.fail = "nvim --headless"
	if err := nextInvocation(i, f).setup(context.Background()); err == nil {
		t.Fatal("expected failed rerun")
	}
	after, _ := os.ReadFile(i.recordPath())
	if string(before) != string(after) {
		t.Fatal("failed rerun replaced the recovery record")
	}
	f.fail = ""
	if err := nextInvocation(i, f).uninstall(context.Background()); err != nil {
		t.Fatal(err)
	}
	assertOriginals(t, paths)
	if f.formulae["new-package-on-rerun"] || f.formulae["neovim"] {
		t.Fatal("packages from repeated setup remain")
	}
	// Intermediate backups can contain local edits and are retained.
	if _, err := os.Stat(filepath.Join(j.paths[0].backup, "init.lua")); err != nil {
		t.Fatal("intermediate backup removed:", err)
	}
}

func TestUninstallKeepsPackagesNowRequiredByOtherTools(t *testing.T) {
	i, f := testInstaller(t)
	if err := i.setup(context.Background()); err != nil {
		t.Fatal(err)
	}
	f.formulae["user-tool"] = true
	f.casks["user-app"] = true
	f.dependents = map[string]string{"node": "user-tool", "new-transitive-dependency": "node\nuser-tool", "ruby": "user-app"}
	if err := nextInvocation(i, f).uninstall(context.Background()); err != nil {
		t.Fatal(err)
	}
	for _, name := range []string{"node", "ruby", "new-transitive-dependency", "user-tool"} {
		if !f.formulae[name] {
			t.Fatalf("shared package removed: %s", name)
		}
	}
	if f.formulae["neovim"] || !f.casks["user-app"] {
		t.Fatal("wrong package removal")
	}
}

func TestUninstallFailureCanRetryWithoutDeletingRestoredFiles(t *testing.T) {
	i, f := testInstaller(t)
	paths := seedEnvironment(t, i)
	if err := i.setup(context.Background()); err != nil {
		t.Fatal(err)
	}
	f.fail = "uninstall --formula"
	if err := nextInvocation(i, f).uninstall(context.Background()); err == nil {
		t.Fatal("expected package removal failure")
	}
	assertOriginals(t, paths)
	if err := os.WriteFile(filepath.Join(paths[0], "later-edit"), []byte("keep"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := nextInvocation(i, f).setup(context.Background()); err == nil || !strings.Contains(err.Error(), "previous uninstall") {
		t.Fatalf("setup overwrote partially uninstalled environment: %v", err)
	}
	f.fail = ""
	if err := nextInvocation(i, f).uninstall(context.Background()); err != nil {
		t.Fatal(err)
	}
	assertOriginals(t, paths)
	if _, err := os.Stat(filepath.Join(paths[0], "later-edit")); err != nil {
		t.Fatal("retry deleted restored files:", err)
	}
}

func TestUninstallRefusesUntrackedOrChangedFiles(t *testing.T) {
	for _, change := range []string{"legacy", "missing-record", "corrupt-record", "missing-backup", "missing-marker", "replacement-symlink", "changed-xdg", "invalid-backup", "invalid-path", "dependency-query"} {
		t.Run(change, func(t *testing.T) {
			i, f := testInstaller(t)
			paths := seedEnvironment(t, i)
			if change != "legacy" {
				if err := i.setup(context.Background()); err != nil {
					t.Fatal(err)
				}
			}
			j := nextInvocation(i, f)
			switch change {
			case "missing-record":
				os.Remove(i.recordPath())
			case "corrupt-record":
				os.WriteFile(i.recordPath(), []byte("{}"), 0o600)
			case "missing-backup":
				os.RemoveAll(i.paths[3].backup)
			case "missing-marker":
				os.Remove(filepath.Join(paths[3], ownershipFile))
			case "replacement-symlink":
				os.RemoveAll(paths[3])
				os.Symlink(paths[0], paths[3])
			case "changed-xdg":
				j.env = setEnv(j.env, "XDG_DATA_HOME", filepath.Join(i.home, "other-data"))
			case "invalid-backup", "invalid-path":
				r, _ := i.readRecord()
				if change == "invalid-path" {
					r.Paths[3].Path = i.home
				} else {
					r.Paths[3].Backup = filepath.Join(i.home, "nvim.mak-backup-wrong-parent")
				}
				i.saveRecord(r)
			case "dependency-query":
				f.fail = "uses --installed"
			}
			f.calls = nil
			if err := j.uninstall(context.Background()); err == nil {
				t.Fatal("expected refusal")
			}
			for _, call := range f.calls {
				if strings.Contains(call, "uninstall") || strings.Contains(call, "install ") {
					t.Fatalf("packages modified despite refusal: %s", call)
				}
			}
			if change == "legacy" {
				assertOriginals(t, paths)
			} else if _, err := os.Stat(filepath.Join(paths[0], "init.lua")); err != nil {
				t.Fatal("refusal modified Neovim files:", err)
			}
		})
	}
}

func TestUninstallRestoresOriginalSymlink(t *testing.T) {
	i, f := testInstaller(t)
	source := filepath.Join(i.home, "dotfiles")
	os.MkdirAll(source, 0o755)
	os.WriteFile(filepath.Join(source, "init.lua"), []byte("original"), 0o600)
	os.MkdirAll(filepath.Join(i.home, ".config"), 0o755)
	config := filepath.Join(i.home, ".config/nvim")
	os.Symlink(source, config)
	if err := i.setup(context.Background()); err != nil {
		t.Fatal(err)
	}
	if err := nextInvocation(i, f).uninstall(context.Background()); err != nil {
		t.Fatal(err)
	}
	target, err := os.Readlink(config)
	if err != nil || target != source {
		t.Fatalf("symlink not restored: %s %v", target, err)
	}
	b, _ := os.ReadFile(filepath.Join(source, "init.lua"))
	if string(b) != "original" {
		t.Fatal("dotfiles target changed")
	}
}

func TestResumeAfterRestoreBeforeCheckpoint(t *testing.T) {
	i, f := testInstaller(t)
	paths := seedEnvironment(t, i)
	if err := i.setup(context.Background()); err != nil {
		t.Fatal(err)
	}
	r, _ := i.readRecord()
	r.Uninstalling = true
	r.Paths[0].Restoring = true
	if err := i.saveRecord(r); err != nil {
		t.Fatal(err)
	}
	if err := os.RemoveAll(paths[0]); err != nil {
		t.Fatal(err)
	}
	if err := os.Rename(r.Paths[0].Backup, paths[0]); err != nil {
		t.Fatal(err)
	}
	if err := nextInvocation(i, f).uninstall(context.Background()); err != nil {
		t.Fatal(err)
	}
	assertOriginals(t, paths)
}

func TestResumeAfterPartiallyDeletingStagedEnvironment(t *testing.T) {
	i, f := testInstaller(t)
	paths := seedEnvironment(t, i)
	if err := i.setup(context.Background()); err != nil {
		t.Fatal(err)
	}
	r := mustRecord(t, i)
	r.Uninstalling = true
	r.Paths[0].Restoring = true
	if err := i.saveRecord(r); err != nil {
		t.Fatal(err)
	}
	discard := discardPath(r.Paths[0], r.ID)
	if err := os.Rename(paths[0], discard); err != nil {
		t.Fatal(err)
	}
	if err := os.Remove(filepath.Join(discard, ownershipFile)); err != nil {
		t.Fatal(err)
	}
	if err := nextInvocation(i, f).uninstall(context.Background()); err != nil {
		t.Fatal(err)
	}
	assertOriginals(t, paths)
	if _, err := os.Lstat(discard); !os.IsNotExist(err) {
		t.Fatalf("staged files remain: %v", err)
	}
}

func TestRecordWriteFailureRetainsVerifiedCommitForRecovery(t *testing.T) {
	i, f := testInstaller(t)
	seedEnvironment(t, i)
	profile := filepath.Join(i.home, ".zprofile")
	original := "export KEEP=1\n"
	if err := os.WriteFile(profile, []byte(original), 0o600); err != nil {
		t.Fatal(err)
	}
	run := f.run
	i.run = func(ctx context.Context, env []string, program string, args ...string) (string, error) {
		result, err := run(ctx, env, program, args...)
		if filepath.Base(program) == "nvim" && envValue(env, "MAK_NVIM_PHASE") == "verify" {
			// Obstruct the final record without obstructing its recovery journal.
			if err := os.Mkdir(filepath.Join(envValue(env, "XDG_STATE_HOME"), "mak/dev-environment.json"), 0o700); err != nil {
				return "", err
			}
		}
		return result, err
	}
	if err := i.setup(context.Background()); err == nil || !strings.Contains(err.Error(), "saving installation record") {
		t.Fatalf("expected record failure: %v", err)
	}
	j, err := i.readSetupJournal()
	if err != nil || j.Commit == nil {
		t.Fatalf("verified commit was not retained: %v", err)
	}
	if err := os.Remove(i.recordPath()); err != nil {
		t.Fatal(err)
	}
	next := nextInvocation(i, f)
	unlock, err := next.prepare(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	unlock()
	if _, err := next.readRecord(); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(filepath.Join(next.paths[0].path, "init.lua")); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(next.journalPath()); !os.IsNotExist(err) {
		t.Fatal("commit journal was not cleared")
	}
	b, _ := os.ReadFile(profile)
	if !strings.Contains(string(b), original) || !strings.Contains(string(b), "Homebrew coding tools") {
		t.Fatal("verified shell changes were lost")
	}
	if !f.formulae["neovim"] {
		t.Fatal("verified packages were removed")
	}
}

func TestUninstallCancellationBeforeRemovalLeavesEnvironmentIntact(t *testing.T) {
	i, f := testInstaller(t)
	if err := i.setup(context.Background()); err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if err := nextInvocation(i, f).uninstall(ctx); !errors.Is(err, context.Canceled) {
		t.Fatalf("expected cancellation: %v", err)
	}
	for _, p := range i.paths {
		if !ownedDirectory(p.path, mustRecord(t, i).ID) {
			t.Fatalf("cancelled uninstall removed %s", p.path)
		}
	}
}

func mustRecord(t *testing.T, i *installer) *installationRecord {
	t.Helper()
	r, err := i.readRecord()
	if err != nil {
		t.Fatal(err)
	}
	return r
}

func TestUninstallWithCustomXDGAndSymlinkedShellProfile(t *testing.T) {
	i, f := testInstaller(t)
	root := t.TempDir()
	i.env = append(i.env, "XDG_CONFIG_HOME="+filepath.Join(root, "config"), "XDG_DATA_HOME="+filepath.Join(root, "data"), "XDG_STATE_HOME="+filepath.Join(root, "state"), "XDG_CACHE_HOME="+filepath.Join(root, "cache"), "ZDOTDIR="+i.home)
	profileTarget := filepath.Join(i.home, "profile-dotfile")
	if err := os.WriteFile(profileTarget, []byte("# dotfile\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	profile := filepath.Join(i.home, ".zprofile")
	if err := os.Symlink(profileTarget, profile); err != nil {
		t.Fatal(err)
	}
	if err := i.setup(context.Background()); err != nil {
		t.Fatal(err)
	}
	resolvedRoot, err := filepath.EvalSymlinks(root)
	if err != nil {
		t.Fatal(err)
	}
	if i.recordPath() != filepath.Join(resolvedRoot, "state/mak/dev-environment.json") {
		t.Fatalf("wrong record location: %s", i.recordPath())
	}
	if err := nextInvocation(i, f).uninstall(context.Background()); err != nil {
		t.Fatal(err)
	}
	target, err := os.Readlink(profile)
	if err != nil || target != profileTarget {
		t.Fatalf("shell symlink changed: %s %v", target, err)
	}
	b, _ := os.ReadFile(profileTarget)
	if string(b) != "# dotfile\n" {
		t.Fatalf("shell dotfile changed: %s", b)
	}
	for _, p := range i.paths {
		if _, err := os.Lstat(p.path); !os.IsNotExist(err) {
			t.Fatalf("custom XDG Neovim files remain: %s %v", p.path, err)
		}
	}
}
