package devenv

import (
	"bufio"
	"context"
	"encoding/json"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
)

func TestSetupJournalRecoversEachFilesystemBoundary(t *testing.T) {
	for _, phase := range []string{"planned", "empty-stage", "owned-stage", "backup-moved", "one-directory", "all-directories", "shell-planned", "shell-written", "partial-delete", "restored-before-checkpoint", "package-only"} {
		t.Run(phase, func(t *testing.T) {
			i, f := testInstaller(t)
			paths := seedEnvironment(t, i)
			profile := filepath.Join(i.home, ".zprofile")
			if err := os.WriteFile(profile, []byte("export KEEP=1\n"), 0600); err != nil {
				t.Fatal(err)
			}
			unlock, err := i.prepare(context.Background())
			if err != nil {
				t.Fatal(err)
			}
			i.beforeFormulae = stringSet(mapKeys(f.formulae))
			i.beforeCasks = stringSet(mapKeys(f.casks))
			if err := i.beginSetupJournal(phase != "package-only"); err != nil {
				t.Fatal(err)
			}
			f.formulae["new-package"] = true
			f.casks["new-app"] = true
			if phase == "empty-stage" || phase == "owned-stage" || phase == "backup-moved" {
				p := i.journal.Record.Paths[0]
				stage := discardPath(p, i.journal.Record.ID)
				if err := os.Mkdir(stage, 0700); err != nil {
					t.Fatal(err)
				}
				if phase != "empty-stage" {
					if err := atomicWrite(filepath.Join(stage, ownershipFile), []byte(i.journal.Record.ID), 0600); err != nil {
						t.Fatal(err)
					}
				}
				if phase == "backup-moved" {
					if err := renameDirectory(p.Path, p.Backup); err != nil {
						t.Fatal(err)
					}
				}
			}
			if phase == "one-directory" || phase == "all-directories" || phase == "shell-planned" || phase == "shell-written" || phase == "partial-delete" || phase == "restored-before-checkpoint" {
				count := 1
				if phase == "all-directories" || strings.HasPrefix(phase, "shell") {
					count = 4
				}
				for n := 0; n < count; n++ {
					if err := i.replace(&i.paths[n]); err != nil {
						t.Fatal(err)
					}
				}
			}
			if phase == "shell-written" {
				if err := i.configureShell("/fake/brew"); err != nil {
					t.Fatal(err)
				}
			}
			if phase == "shell-planned" {
				i.journal.Record.Shell = []shellChange{{Path: profile, Block: "\n# Homebrew coding tools (mak setup dev)\neval \"$('/fake/brew/bin/brew' shellenv)\"\n"}}
				if err := i.saveSetupJournal(); err != nil {
					t.Fatal(err)
				}
			}
			if phase == "partial-delete" || phase == "restored-before-checkpoint" {
				p := &i.journal.Record.Paths[0]
				p.Restoring = true
				if err := i.saveSetupJournal(); err != nil {
					t.Fatal(err)
				}
				stage := discardPath(*p, i.journal.Record.ID)
				if err := renameDirectory(p.Path, stage); err != nil {
					t.Fatal(err)
				}
				if err := os.Remove(filepath.Join(stage, ownershipFile)); err != nil {
					t.Fatal(err)
				}
				if phase == "partial-delete" {
					if err := os.WriteFile(filepath.Join(stage, "leftover"), []byte("managed"), 0600); err != nil {
						t.Fatal(err)
					}
				} else {
					if err := os.RemoveAll(stage); err != nil {
						t.Fatal(err)
					}
					if err := renameDirectory(p.Backup, p.Path); err != nil {
						t.Fatal(err)
					}
				}
			}
			unlock() // Simulate a process disappearing without running rollback.
			next := nextInvocation(i, f)
			release, err := next.prepare(context.Background())
			if err != nil {
				t.Fatal(err)
			}
			release()
			assertOriginals(t, paths)
			b, err := os.ReadFile(profile)
			if err != nil || string(b) != "export KEEP=1\n" {
				t.Fatalf("profile changed: %v %s", err, b)
			}
			if !reflect.DeepEqual(f.formulae, map[string]bool{"git": true, "shared-dependency": true}) || !reflect.DeepEqual(f.casks, map[string]bool{"existing-app": true}) {
				t.Fatal("preexisting packages were not preserved")
			}
			if _, err := os.Stat(next.journalPath()); !os.IsNotExist(err) {
				t.Fatal("recovery journal remains")
			}
		})
	}
}

func TestSetupJournalRejectsUnexpectedFiles(t *testing.T) {
	for _, change := range []string{"missing-marker", "missing-backup", "changed-xdg", "corrupt-journal"} {
		t.Run(change, func(t *testing.T) {
			i, f := testInstaller(t)
			seedEnvironment(t, i)
			unlock, err := i.prepare(context.Background())
			if err != nil {
				t.Fatal(err)
			}
			i.beforeFormulae = stringSet(mapKeys(f.formulae))
			i.beforeCasks = stringSet(mapKeys(f.casks))
			if err := i.beginSetupJournal(true); err != nil {
				t.Fatal(err)
			}
			if err := i.replace(&i.paths[0]); err != nil {
				t.Fatal(err)
			}
			switch change {
			case "missing-marker":
				if err := os.Remove(filepath.Join(i.paths[0].path, ownershipFile)); err != nil {
					t.Fatal(err)
				}
			case "missing-backup":
				if err := os.RemoveAll(i.paths[0].backup); err != nil {
					t.Fatal(err)
				}
			case "changed-xdg":
				i.env = setEnv(i.env, "XDG_CONFIG_HOME", filepath.Join(i.home, "other-config"))
				// A rejected invocation can create its coordination file; precreate
				// it so the assertion measures changes to user/recovery data.
				otherLock := filepath.Join(i.home, "other-config/mak/dev-setup.lock")
				if err := os.MkdirAll(filepath.Dir(otherLock), 0700); err != nil {
					t.Fatal(err)
				}
				f, err := acquireSetupLock(otherLock)
				if err != nil {
					t.Fatal(err)
				}
				f.Close()
			case "corrupt-journal":
				if err := os.WriteFile(i.journalPath(), []byte("invalid"), 0600); err != nil {
					t.Fatal(err)
				}
			}
			unlock()
			before := snapshotFiles(t, i.home)
			if release, err := nextInvocation(i, f).prepare(context.Background()); err == nil {
				release()
				t.Fatal("unsafe recovery was accepted")
			}
			if !reflect.DeepEqual(before, snapshotFiles(t, i.home)) {
				t.Fatal("unsafe recovery changed files")
			}
		})
	}
}

func TestInterruptedRepeatSetupRestoresLastWorkingEnvironment(t *testing.T) {
	i, f := testInstaller(t)
	seedEnvironment(t, i)
	if err := i.setup(context.Background()); err != nil {
		t.Fatal(err)
	}
	recordBefore, err := os.ReadFile(i.recordPath())
	if err != nil {
		t.Fatal(err)
	}
	config := filepath.Join(i.paths[0].path, "init.lua")
	if err := os.WriteFile(config, []byte("-- local edits"), 0600); err != nil {
		t.Fatal(err)
	}
	j := nextInvocation(i, f)
	unlock, err := j.prepare(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	j.beforeFormulae = stringSet(mapKeys(f.formulae))
	j.beforeCasks = stringSet(mapKeys(f.casks))
	if err := j.beginSetupJournal(true); err != nil {
		t.Fatal(err)
	}
	if err := j.replace(&j.paths[0]); err != nil {
		t.Fatal(err)
	}
	unlock()
	next := nextInvocation(j, f)
	release, err := next.prepare(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	release()
	b, err := os.ReadFile(config)
	if err != nil || string(b) != "-- local edits" {
		t.Fatal("working environment was not restored")
	}
	after, err := os.ReadFile(next.recordPath())
	if err != nil || string(after) != string(recordBefore) {
		t.Fatal("previous installation record changed")
	}
	if err := next.uninstall(context.Background()); err != nil {
		t.Fatal(err)
	}
}

func TestChildKeepsSetupLockAfterParentClosesIt(t *testing.T) {
	path := filepath.Join(t.TempDir(), "lock")
	f, err := acquireSetupLock(path)
	if err != nil {
		t.Fatal(err)
	}
	cmd := exec.Command("/bin/sh", "-c", "printf 'ready\\n'; read ignored")
	cmd.ExtraFiles = []*os.File{f}
	input, err := cmd.StdinPipe()
	if err != nil {
		t.Fatal(err)
	}
	output, err := cmd.StdoutPipe()
	if err != nil {
		t.Fatal(err)
	}
	if err := cmd.Start(); err != nil {
		t.Fatal(err)
	}
	defer func() { input.Close(); cmd.Process.Kill(); cmd.Wait() }()
	if _, err := bufio.NewReader(output).ReadString('\n'); err != nil {
		t.Fatal(err)
	}
	f.Close()
	if locked, err := setupLocked(path); err != nil || !locked {
		t.Fatalf("child lost lock: %v", err)
	}
	input.Close()
	_ = cmd.Wait()
	if locked, err := setupLocked(path); err != nil || locked {
		t.Fatalf("lock wasn't released after child exit: %v", err)
	}
}

type crashInventory struct{ Formulae, Casks map[string]bool }

func TestSetupCrashHelper(t *testing.T) {
	home := os.Getenv("MAK_CRASH_TEST_HOME")
	if home == "" {
		return
	}
	var state crashInventory
	b, err := os.ReadFile(filepath.Join(home, "inventory.json"))
	if err != nil {
		os.Exit(90)
	}
	if json.Unmarshal(b, &state) != nil {
		os.Exit(91)
	}
	f := &fakeSystem{formulae: state.Formulae, casks: state.Casks}
	i := &installer{home: home, env: []string{"SHELL=/bin/zsh", "PATH=/usr/bin:/bin"}, brew: "/fake/brew/bin/brew", out: io.Discard}
	i.run = func(ctx context.Context, env []string, program string, args ...string) (string, error) {
		result, err := f.run(ctx, env, program, args...)
		if args[0] == "install" {
			b, _ := json.Marshal(crashInventory{f.formulae, f.casks})
			if os.WriteFile(filepath.Join(home, "inventory.json"), b, 0600) != nil {
				os.Exit(92)
			}
			os.Exit(73) // Bypass defers exactly as an uncatchable termination would.
		}
		return result, err
	}
	_ = i.setup(context.Background())
	os.Exit(93)
}

func TestNewProcessRecoversUncatchableSetupExit(t *testing.T) {
	i, f := testInstaller(t)
	paths := seedEnvironment(t, i)
	b, _ := json.Marshal(crashInventory{f.formulae, f.casks})
	inventory := filepath.Join(i.home, "inventory.json")
	if err := os.WriteFile(inventory, b, 0600); err != nil {
		t.Fatal(err)
	}
	cmd := exec.Command(os.Args[0], "-test.run=^TestSetupCrashHelper$")
	cmd.Env = append(os.Environ(), "MAK_CRASH_TEST_HOME="+i.home)
	if err := cmd.Run(); err == nil {
		t.Fatal("helper did not exit unexpectedly")
	}
	var state crashInventory
	b, err := os.ReadFile(inventory)
	if err != nil {
		t.Fatal(err)
	}
	if err := json.Unmarshal(b, &state); err != nil {
		t.Fatal(err)
	}
	f.formulae, f.casks = state.Formulae, state.Casks
	if !f.formulae["neovim"] {
		t.Fatal("helper did not install packages before exiting")
	}
	release, err := i.prepare(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	release()
	assertOriginals(t, paths)
	if f.formulae["neovim"] || !f.formulae["git"] {
		t.Fatal("recovered package ownership incorrectly")
	}
}

func TestRemovalAfterInterruptedFirstSetupCompletesRecovery(t *testing.T) {
	for _, single := range []bool{false, true} {
		t.Run(map[bool]string{false: "uninstall", true: "remove"}[single], func(t *testing.T) {
			i, f := testInstaller(t)
			paths := seedEnvironment(t, i)
			unlock, err := i.prepare(context.Background())
			if err != nil {
				t.Fatal(err)
			}
			i.beforeFormulae = stringSet(mapKeys(f.formulae))
			i.beforeCasks = stringSet(mapKeys(f.casks))
			if err := i.beginSetupJournal(true); err != nil {
				t.Fatal(err)
			}
			f.formulae["node"] = true
			if err := i.replace(&i.paths[0]); err != nil {
				t.Fatal(err)
			}
			unlock()
			next := nextInvocation(i, f)
			if single {
				p, _ := lookupPackage("node")
				err = next.removePackage(context.Background(), p)
			} else {
				err = next.uninstall(context.Background())
			}
			if err != nil {
				t.Fatal(err)
			}
			assertOriginals(t, paths)
			if f.formulae["node"] {
				t.Fatal("interrupted packages remain")
			}
		})
	}
}
