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

func packageForTest(t *testing.T, name string) packageSpec {
	t.Helper()
	p, err := lookupPackage(name)
	if err != nil {
		t.Fatal(err)
	}
	return p
}

func TestIndividualTreeSitterCLIInstallPreservesLibrary(t *testing.T) {
	i, f := testInstaller(t)
	f.formulae["tree-sitter"] = true
	if err := i.installPackage(context.Background(), packageForTest(t, "tree-sitter-cli")); err != nil {
		t.Fatal(err)
	}
	if !f.formulae["tree-sitter-cli"] {
		t.Fatal("CLI was not installed")
	}
	if err := nextInvocation(i, f).removePackage(context.Background(), packageForTest(t, "tree-sitter-cli")); err != nil {
		t.Fatal(err)
	}
	if f.formulae["tree-sitter-cli"] || !f.formulae["tree-sitter"] {
		t.Fatalf("CLI removal affected the library: %v", f.formulae)
	}
}

func TestListPackagesShowsInstalledAndOwnershipWithoutWriting(t *testing.T) {
	i, f := testInstaller(t)
	var out bytes.Buffer
	i.out = &out
	f.formulae["python@3.14"] = true
	f.aliases = map[string]string{"python": "python@3.14"}
	if err := i.listPackages(context.Background()); err != nil {
		t.Fatal(err)
	}
	for _, line := range []string{"[x] git", "[x] python", "[ ] neovim", "[ ] node", "(python@3.14)", "not tracked by mak"} {
		if !strings.Contains(out.String(), line) {
			t.Fatalf("missing %s in %s", line, out.String())
		}
	}
	entries, _ := os.ReadDir(i.home)
	if len(entries) != 0 || strings.Contains(out.String(), "\x1b") {
		t.Fatalf("list wrote files or emitted ANSI in a pipe: %v %q", entries, out.String())
	}
	if err := nextInvocation(i, f).installPackage(context.Background(), packageForTest(t, "node")); err != nil {
		t.Fatal(err)
	}
	j := nextInvocation(i, f)
	out.Reset()
	j.out = &out
	f.calls = nil
	if err := j.listPackages(context.Background()); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(out.String(), "[x] node") {
		t.Fatal("installed Node missing from list:", out.String())
	}
	for _, line := range strings.Split(out.String(), "\n") {
		if strings.HasPrefix(line, "[x] node") && !strings.Contains(line, "tracked by mak") {
			t.Fatal("missing mak ownership:", line)
		}
	}
	for _, call := range f.calls {
		if !strings.HasPrefix(call, "brew list ") && !strings.HasPrefix(call, "brew info ") {
			t.Fatal("list made a non-query call:", call)
		}
	}
}

func TestIndividualInstallAndRemovalKeepsOtherSoftwareAndDependencies(t *testing.T) {
	i, f := testInstaller(t)
	paths := seedEnvironment(t, i)
	if err := i.installPackage(context.Background(), packageForTest(t, "node")); err != nil {
		t.Fatal(err)
	}
	assertOriginals(t, paths)
	r := mustRecord(t, i)
	if len(r.Paths) != 0 || !slices.Contains(r.Formulae, "node") || !slices.Contains(r.Formulae, "new-transitive-dependency") || slices.Contains(r.Formulae, "git") {
		t.Fatalf("wrong standalone record: %+v", r)
	}
	if f.formulae["go"] || f.casks[font] {
		t.Fatal("individual install installed the whole environment")
	}
	if err := nextInvocation(i, f).removePackage(context.Background(), packageForTest(t, "node")); err != nil {
		t.Fatal(err)
	}
	assertOriginals(t, paths)
	if f.formulae["node"] || !f.formulae["new-transitive-dependency"] || !f.formulae["git"] {
		t.Fatalf("single removal affected other software: %v", f.formulae)
	}
	if err := nextInvocation(i, f).uninstall(context.Background()); err != nil {
		t.Fatal(err)
	}
	assertOriginals(t, paths)
	if !reflect.DeepEqual(f.formulae, map[string]bool{"git": true, "shared-dependency": true}) {
		t.Fatalf("remaining dependency not cleaned up: %v", f.formulae)
	}
}

func TestIndividualInstallDoesNotClaimPreexistingPackages(t *testing.T) {
	i, f := testInstaller(t)
	f.aliases = map[string]string{"python": "python@3.14"}
	f.formulae["python@3.14"] = true
	if err := i.installPackage(context.Background(), packageForTest(t, "python")); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(i.recordPath()); !os.IsNotExist(err) {
		t.Fatalf("preexisting package got a record: %v", err)
	}
	if err := nextInvocation(i, f).removePackage(context.Background(), packageForTest(t, "python")); err == nil {
		t.Fatal("allowed removal of preexisting package")
	}
	if !f.formulae["python@3.14"] {
		t.Fatal("preexisting package removed")
	}
}

func TestIndividualInstallFailureRollsBackNewPackages(t *testing.T) {
	i, f := testInstaller(t)
	paths := seedEnvironment(t, i)
	f.fail = "install --formula"
	if err := i.installPackage(context.Background(), packageForTest(t, "node")); err == nil {
		t.Fatal("expected failed installation")
	}
	assertOriginals(t, paths)
	if !reflect.DeepEqual(f.formulae, map[string]bool{"git": true, "shared-dependency": true}) {
		t.Fatalf("new packages not rolled back: %v", f.formulae)
	}
}

func TestNeovimSelectionInstallsFullEnvironmentAndRemovalRestoresOnlyNeovim(t *testing.T) {
	i, f := testInstaller(t)
	paths := seedEnvironment(t, i)
	if err := i.installPackage(context.Background(), packageForTest(t, "nvim")); err != nil {
		t.Fatal(err)
	}
	if !f.formulae["go"] || !f.formulae["node"] || !f.casks[font] {
		t.Fatal("Neovim selection did not install full environment")
	}
	if _, err := os.Stat(filepath.Join(paths[0], "lua/plugins/go.lua")); err != nil {
		t.Fatal("LazyVim configuration missing:", err)
	}
	if err := nextInvocation(i, f).removePackage(context.Background(), packageForTest(t, "nvim")); err != nil {
		t.Fatal(err)
	}
	assertOriginals(t, paths)
	if f.formulae["neovim"] || !f.formulae["node"] || !f.casks[font] {
		t.Fatal("Neovim removal affected other tools")
	}
	r := mustRecord(t, i)
	if len(r.Paths) != 0 || r.Removing != "" || slices.Contains(r.Formulae, "neovim") {
		t.Fatalf("record not updated: %+v", r)
	}
	if err := nextInvocation(i, f).uninstall(context.Background()); err != nil {
		t.Fatal(err)
	}
	assertOriginals(t, paths)
}

func TestFullSetupAfterStandaloneInstallKeepsOriginalOwnership(t *testing.T) {
	i, f := testInstaller(t)
	paths := seedEnvironment(t, i)
	if err := i.installPackage(context.Background(), packageForTest(t, "node")); err != nil {
		t.Fatal(err)
	}
	if err := nextInvocation(i, f).setup(context.Background()); err != nil {
		t.Fatal(err)
	}
	if err := nextInvocation(i, f).uninstall(context.Background()); err != nil {
		t.Fatal(err)
	}
	assertOriginals(t, paths)
	if f.formulae["node"] {
		t.Fatal("full setup forgot individually installed package ownership")
	}
}

func TestSingleRemovalProtectsDependentsIncludingOtherMakPackages(t *testing.T) {
	i, f := testInstaller(t)
	if err := i.installPackage(context.Background(), packageForTest(t, "node")); err != nil {
		t.Fatal(err)
	}
	f.dependents = map[string]string{"node": "user-tool"}
	f.formulae["user-tool"] = true
	if err := nextInvocation(i, f).removePackage(context.Background(), packageForTest(t, "node")); err == nil || !strings.Contains(err.Error(), "user-tool") {
		t.Fatalf("dependent not protected: %v", err)
	}
	if !f.formulae["node"] {
		t.Fatal("required package removed")
	}
}

func TestFailedNeovimRemovalCanResumeWithoutDeletingRestoredFiles(t *testing.T) {
	i, f := testInstaller(t)
	paths := seedEnvironment(t, i)
	if err := i.setup(context.Background()); err != nil {
		t.Fatal(err)
	}
	f.fail = "uninstall --formula"
	if err := nextInvocation(i, f).removePackage(context.Background(), packageForTest(t, "nvim")); err == nil {
		t.Fatal("expected removal failure")
	}
	assertOriginals(t, paths)
	if err := nextInvocation(i, f).installPackage(context.Background(), packageForTest(t, "node")); err == nil {
		t.Fatal("install allowed during pending removal")
	}
	if err := os.WriteFile(filepath.Join(paths[0], "later-edit"), []byte("keep"), 0o600); err != nil {
		t.Fatal(err)
	}
	f.fail = ""
	if err := nextInvocation(i, f).removePackage(context.Background(), packageForTest(t, "nvim")); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(filepath.Join(paths[0], "later-edit")); err != nil {
		t.Fatal("retry deleted restored configuration:", err)
	}
}

func TestVersionedAliasRemovesTheTrackedVersionAfterHomebrewChangesAlias(t *testing.T) {
	i, f := testInstaller(t)
	f.aliases = map[string]string{"python": "python@3.14"}
	if err := i.installPackage(context.Background(), packageForTest(t, "python")); err != nil {
		t.Fatal(err)
	}
	if mustRecord(t, i).Packages["python"] != "python@3.14" {
		t.Fatal("canonical name not recorded")
	}
	f.aliases["python"] = "python@3.15"
	f.formulae["python@3.15"] = true
	if err := nextInvocation(i, f).removePackage(context.Background(), packageForTest(t, "python")); err != nil {
		t.Fatal(err)
	}
	if f.formulae["python@3.14"] || !f.formulae["python@3.15"] {
		t.Fatalf("removed the wrong version: %v", f.formulae)
	}
}

func TestIndividualFontInstallAndRemoval(t *testing.T) {
	i, f := testInstaller(t)
	if err := i.installPackage(context.Background(), packageForTest(t, "font")); err != nil {
		t.Fatal(err)
	}
	if err := nextInvocation(i, f).removePackage(context.Background(), packageForTest(t, "font")); err != nil {
		t.Fatal(err)
	}
	if f.casks[font] || !f.casks["existing-app"] {
		t.Fatalf("wrong cask removal: %v", f.casks)
	}
	if _, err := os.Stat(i.recordPath()); !os.IsNotExist(err) {
		t.Fatalf("record remains after last package removal: %v", err)
	}
	if _, err := os.Stat(filepath.Join(i.home, ".zprofile")); !os.IsNotExist(err) {
		t.Fatalf("mak's shell block remains: %v", err)
	}
}
