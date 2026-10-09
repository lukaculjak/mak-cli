package devenv

import (
	"bytes"
	"context"
	"os"
	"path/filepath"
	"reflect"
	"slices"
	"strings"
	"testing"
)

func seedGhostty(t *testing.T, i *installer) []string {
	t.Helper()
	if err := i.resolvePaths(); err != nil {
		t.Fatal(err)
	}
	var paths []string
	for _, p := range i.paths[4:] {
		if err := os.MkdirAll(p.path, 0o755); err != nil {
			t.Fatal(err)
		}
		// Modern filenames must not survive and override the bundled config.
		if err := os.WriteFile(filepath.Join(p.path, "config.ghostty"), []byte("theme = original\n"), 0o600); err != nil {
			t.Fatal(err)
		}
		paths = append(paths, p.path)
	}
	return paths
}

func assertGhosttyOriginals(t *testing.T, paths []string) {
	t.Helper()
	for _, path := range paths {
		contents, err := os.ReadFile(filepath.Join(path, "config.ghostty"))
		if err != nil || string(contents) != "theme = original\n" {
			t.Fatalf("original Ghostty settings at %s: %q %v", path, contents, err)
		}
		entries, err := os.ReadDir(path)
		if err != nil || len(entries) != 1 {
			t.Fatalf("managed files remain: %s: %v %v", path, entries, err)
		}
	}
}

func TestGhosttyStandaloneSetupRerunAndRemoval(t *testing.T) {
	i, f := testInstaller(t)
	nvimPaths := seedEnvironment(t, i)
	paths := seedGhostty(t, i)
	for range 2 {
		if err := nextInvocation(i, f).installPackage(context.Background(), packageForTest(t, "ghostty")); err != nil {
			t.Fatal(err)
		}
	}
	r := mustRecord(t, i)
	if len(r.Paths) != 2 || !f.casks["ghostty"] || !f.casks[ghosttyFont] || f.casks[font] || f.formulae["neovim"] {
		t.Fatalf("wrong standalone install: %+v %v", r, f.casks)
	}
	config, _ := os.ReadFile(filepath.Join(paths[1], "config"))
	bundled, _ := assets.ReadFile("assets/ghostty/config")
	if !bytes.Equal(config, bundled) || !strings.Contains(string(config), "theme = niji") {
		t.Fatal("bundled settings missing")
	}
	for _, path := range paths {
		if _, err := os.Stat(filepath.Join(path, "config.ghostty")); !os.IsNotExist(err) {
			t.Fatal("old configuration can override bundled settings")
		}
	}
	assertOriginals(t, nvimPaths)
	if err := nextInvocation(i, f).removePackage(context.Background(), packageForTest(t, "ghostty")); err != nil {
		t.Fatal(err)
	}
	assertGhosttyOriginals(t, paths)
	assertOriginals(t, nvimPaths)
	if f.casks["ghostty"] || !f.casks[ghosttyFont] {
		t.Fatal("Ghostty removal must retain its font dependency")
	}
	if err := nextInvocation(i, f).uninstall(context.Background()); err != nil {
		t.Fatal(err)
	}
	if f.casks[ghosttyFont] {
		t.Fatal("font was not removed by full uninstall")
	}
}

func TestGhosttyFullSetupRollbackAndPreexistingApplication(t *testing.T) {
	for _, failure := range []string{"", "nvim --headless"} {
		t.Run(failure, func(t *testing.T) {
			i, f := testInstaller(t)
			paths := seedGhostty(t, i)
			f.casks["ghostty"] = true
			f.casks[ghosttyFont] = true
			before := slices.Clone(mapKeys(f.casks))
			f.fail = failure
			err := i.setup(context.Background())
			if (err != nil) != (failure != "") {
				t.Fatal(err)
			}
			if failure == "" {
				r := mustRecord(t, i)
				if r.Packages["ghostty"] != "" || r.Packages[ghosttyFont] != "" {
					t.Fatal("preexisting application or font claimed by mak")
				}
				if err := nextInvocation(i, f).uninstall(context.Background()); err != nil {
					t.Fatal(err)
				}
			}
			assertGhosttyOriginals(t, paths)
			if !reflect.DeepEqual(f.casks, stringSet(before)) {
				t.Fatalf("preexisting casks changed: %v", f.casks)
			}
		})
	}
}

func TestGhosttyRemovalKeepsNeovimAndUninstallRestoresBoth(t *testing.T) {
	i, f := testInstaller(t)
	nvimPaths := seedEnvironment(t, i)
	paths := seedGhostty(t, i)
	if err := i.setup(context.Background()); err != nil {
		t.Fatal(err)
	}
	if err := nextInvocation(i, f).removePackage(context.Background(), packageForTest(t, "ghostty")); err != nil {
		t.Fatal(err)
	}
	assertGhosttyOriginals(t, paths)
	if _, err := os.Stat(filepath.Join(nvimPaths[0], "init.lua")); err != nil {
		t.Fatal("Ghostty removal affected Neovim:", err)
	}
	if err := nextInvocation(i, f).uninstall(context.Background()); err != nil {
		t.Fatal(err)
	}
	assertOriginals(t, nvimPaths)
	assertGhosttyOriginals(t, paths)
}

func TestInterruptedGhosttyInstallRestoresBothConfigLocations(t *testing.T) {
	i, f := testInstaller(t)
	paths := seedGhostty(t, i)
	i.beforeFormulae, i.beforeCasks = stringSet(mapKeys(f.formulae)), stringSet(mapKeys(f.casks))
	if err := i.beginSetupJournalPaths(i.paths[4:]); err != nil {
		t.Fatal(err)
	}
	for n := 4; n < 6; n++ {
		if err := i.replace(&i.paths[n]); err != nil {
			t.Fatal(err)
		}
	}
	f.casks["ghostty"] = true
	f.casks[ghosttyFont] = true
	if err := nextInvocation(i, f).uninstall(context.Background()); err != nil {
		t.Fatal(err)
	}
	assertGhosttyOriginals(t, paths)
	if f.casks["ghostty"] || f.casks[ghosttyFont] {
		t.Fatal("interrupted install did not roll back casks")
	}
}

func TestAddingGhosttyToPreviousNeovimRecordRetainsOriginalBackups(t *testing.T) {
	i, f := testInstaller(t)
	nvimPaths := seedEnvironment(t, i)
	paths := seedGhostty(t, i)
	i.beforeFormulae, i.beforeCasks = stringSet(mapKeys(f.formulae)), stringSet(mapKeys(f.casks))
	if err := i.beginSetupJournalPaths(i.paths[:4]); err != nil {
		t.Fatal(err)
	}
	for n := 0; n < 4; n++ {
		if err := i.replace(&i.paths[n]); err != nil {
			t.Fatal(err)
		}
	}
	if err := i.recordInstallation(context.Background(), nil); err != nil {
		t.Fatal(err)
	}
	old := mustRecord(t, i)
	if err := nextInvocation(i, f).setup(context.Background()); err != nil {
		t.Fatal(err)
	}
	updated := mustRecord(t, i)
	if len(updated.Paths) != 6 || !reflect.DeepEqual(old.Paths, updated.Paths[:4]) {
		t.Fatal("previous Neovim backups changed")
	}
	if err := nextInvocation(i, f).uninstall(context.Background()); err != nil {
		t.Fatal(err)
	}
	assertOriginals(t, nvimPaths)
	assertGhosttyOriginals(t, paths)
}
