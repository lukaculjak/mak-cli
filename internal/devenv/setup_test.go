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

type fakeSystem struct {
	formulae, casks map[string]bool
	fail            string
	calls           []string
	dependents      map[string]string
}

func (f *fakeSystem) run(ctx context.Context, env []string, program string, args ...string) (string, error) {
	if err := ctx.Err(); err != nil {
		return "", err
	}
	call := filepath.Base(program) + " " + strings.Join(args, " ")
	f.calls = append(f.calls, call)
	if args[0] == "list" {
		packages := f.formulae
		if args[1] == "--cask" {
			packages = f.casks
		}
		return strings.Join(mapKeys(packages), "\n"), nil
	}
	if args[0] == "--prefix" {
		return "/fake/brew\n", nil
	}
	if args[0] == "uses" {
		if f.fail != "" && strings.Contains(call, f.fail) {
			return "", errors.New("simulated failure: " + call)
		}
		return f.dependents[args[len(args)-1]], nil
	}
	if args[0] == "install" {
		packages := f.formulae
		if args[1] == "--cask" {
			packages = f.casks
		}
		for _, p := range args[2:] {
			packages[p] = true
		}
		if args[1] == "--formula" {
			f.formulae["new-transitive-dependency"] = true
		}
	}
	if args[0] == "uninstall" {
		if envValue(env, "HOMEBREW_NO_AUTOREMOVE") != "1" {
			return "", errors.New("uninstall must disable Homebrew's global autoremove")
		}
		if strings.HasPrefix(f.fail, "uninstall") && strings.Contains(call, f.fail) {
			return "", errors.New("simulated failure: " + call)
		}
		packages := f.formulae
		if args[1] == "--cask" {
			packages = f.casks
		}
		for _, p := range args[2:] {
			delete(packages, p)
		}
		return "", nil
	}
	if f.fail != "" && strings.Contains(call, f.fail) {
		return "", errors.New("simulated failure: " + call)
	}
	if filepath.Base(program) == "nvim" {
		marker := filepath.Join(envValue(env, "MAK_NVIM_CONFIG"), ".mak-"+envValue(env, "MAK_NVIM_PHASE")+"-ok")
		return "", os.WriteFile(marker, []byte("ok"), 0o644)
	}
	return "", nil
}

func mapKeys(m map[string]bool) []string {
	keys := make([]string, 0, len(m))
	for name := range m {
		keys = append(keys, name)
	}
	return keys
}

func testInstaller(t *testing.T) (*installer, *fakeSystem) {
	t.Helper()
	f := &fakeSystem{formulae: map[string]bool{"git": true, "shared-dependency": true}, casks: map[string]bool{"existing-app": true}}
	i := &installer{home: t.TempDir(), env: []string{"SHELL=/bin/zsh", "PATH=/usr/bin:/bin"}, brew: "/fake/brew/bin/brew", run: f.run, out: io.Discard}
	return i, f
}

func seedEnvironment(t *testing.T, i *installer) []string {
	t.Helper()
	paths := []string{filepath.Join(i.home, ".config/nvim"), filepath.Join(i.home, ".local/share/nvim"), filepath.Join(i.home, ".local/state/nvim"), filepath.Join(i.home, ".cache/nvim")}
	for n, p := range paths {
		if err := os.MkdirAll(p, 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(p, "original"), []byte(fmt.Sprint(n)), 0o600); err != nil {
			t.Fatal(err)
		}
	}
	return paths
}

func TestSetupRetainsBackupsAndEmbedsConfig(t *testing.T) {
	i, _ := testInstaller(t)
	paths := seedEnvironment(t, i)
	if err := i.setup(context.Background()); err != nil {
		t.Fatal(err)
	}
	for n, p := range i.paths {
		b, err := os.ReadFile(filepath.Join(p.backup, "original"))
		if err != nil || string(b) != fmt.Sprint(n) {
			t.Fatalf("backup %s: %q %v", p.backup, b, err)
		}
		if _, err := os.Stat(filepath.Join(paths[n], "original")); !os.IsNotExist(err) {
			t.Fatalf("original still active: %v", err)
		}
	}
	for _, file := range []string{"init.lua", "lazy-lock.json", "lazyvim.json", "lua/plugins/theme.lua", "lua/plugins/css.lua", "lua/plugins/go.lua", "lua/plugins/ruby.lua", "mak-brew-prefix"} {
		if _, err := os.Stat(filepath.Join(paths[0], file)); err != nil {
			t.Fatal(err)
		}
	}
	profile, err := os.ReadFile(filepath.Join(i.home, ".zprofile"))
	if err != nil || !strings.Contains(string(profile), "shellenv") {
		t.Fatalf("profile: %q %v", profile, err)
	}
}

func TestSetupRollbackAtEachExternalStep(t *testing.T) {
	for _, failure := range []string{"install --formula", "install --cask", "git clone", "checkout --detach", "nvim --headless"} {
		t.Run(failure, func(t *testing.T) {
			i, f := testInstaller(t)
			paths := seedEnvironment(t, i)
			f.fail = failure
			err := i.setup(context.Background())
			if err == nil || !strings.Contains(err.Error(), "simulated failure") {
				t.Fatalf("unexpected error: %v", err)
			}
			for n, p := range paths {
				b, err := os.ReadFile(filepath.Join(p, "original"))
				if err != nil || string(b) != fmt.Sprint(n) {
					t.Fatalf("original %s: %q %v", p, b, err)
				}
				entries, _ := os.ReadDir(p)
				if len(entries) != 1 {
					t.Fatalf("partial environment left in %s: %v", p, entries)
				}
			}
			if !reflect.DeepEqual(f.formulae, map[string]bool{"git": true, "shared-dependency": true}) || !reflect.DeepEqual(f.casks, map[string]bool{"existing-app": true}) {
				t.Fatalf("packages were not restored: %v %v", f.formulae, f.casks)
			}
			if _, err := os.Stat(filepath.Join(i.home, ".config/mak/dev-setup.lock")); !os.IsNotExist(err) {
				t.Fatalf("setup lock left behind: %v", err)
			}
		})
	}
}

func TestFreshSetupFailureLeavesNoNeovimFiles(t *testing.T) {
	i, f := testInstaller(t)
	f.fail = "nvim --headless"
	if err := i.setup(context.Background()); err == nil {
		t.Fatal("expected failure")
	}
	for _, p := range i.paths {
		if _, err := os.Stat(p.path); !os.IsNotExist(err) {
			t.Fatalf("partial setup remains: %s: %v", p.path, err)
		}
	}
}

func TestSymlinkConfigurationRestoredWithoutTouchingTarget(t *testing.T) {
	i, f := testInstaller(t)
	source := filepath.Join(i.home, "dotfiles")
	if err := os.MkdirAll(source, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(source, "init.lua"), []byte("original"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(filepath.Join(i.home, ".config"), 0o755); err != nil {
		t.Fatal(err)
	}
	config := filepath.Join(i.home, ".config/nvim")
	if err := os.Symlink(source, config); err != nil {
		t.Fatal(err)
	}
	f.fail = "nvim --headless"
	if err := i.setup(context.Background()); err == nil {
		t.Fatal("expected failure")
	}
	target, err := os.Readlink(config)
	if err != nil || target != source {
		t.Fatalf("symlink changed: %s %v", target, err)
	}
	contents, _ := os.ReadFile(filepath.Join(source, "init.lua"))
	if string(contents) != "original" {
		t.Fatal("dotfiles target modified")
	}
}

func TestXDGDirectoriesAndSetupLock(t *testing.T) {
	i, _ := testInstaller(t)
	root := t.TempDir()
	i.env = append(i.env, "XDG_CONFIG_HOME="+filepath.Join(root, "config"), "XDG_DATA_HOME="+filepath.Join(root, "data"), "XDG_STATE_HOME="+filepath.Join(root, "state"), "XDG_CACHE_HOME="+filepath.Join(root, "cache"))
	lock := filepath.Join(root, "config/mak/dev-setup.lock")
	if err := os.MkdirAll(lock, 0o700); err != nil {
		t.Fatal(err)
	}
	if err := i.setup(context.Background()); err == nil || !strings.Contains(err.Error(), "lock") {
		t.Fatalf("expected lock error: %v", err)
	}
	if err := os.Remove(lock); err != nil {
		t.Fatal(err)
	}
	// A second invocation uses a fresh installer, like a new command invocation.
	i.paths = nil
	if err := i.setup(context.Background()); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(filepath.Join(root, "config/nvim/init.lua")); err != nil {
		t.Fatal(err)
	}
}

func TestRejectOverlappingXDGDirectoriesBeforeInstalling(t *testing.T) {
	i, f := testInstaller(t)
	i.env = append(i.env, "XDG_DATA_HOME="+filepath.Join(i.home, ".config"))
	if err := i.setup(context.Background()); err == nil {
		t.Fatal("expected overlapping directory error")
	}
	if len(f.calls) != 0 {
		t.Fatal("started installation before validation")
	}
}

func TestRejectSymlinkAliasesForXDGDirectories(t *testing.T) {
	i, f := testInstaller(t)
	config := filepath.Join(i.home, ".config")
	if err := os.Mkdir(config, 0o755); err != nil {
		t.Fatal(err)
	}
	alias := filepath.Join(i.home, "alias")
	if err := os.Symlink(config, alias); err != nil {
		t.Fatal(err)
	}
	i.env = append(i.env, "XDG_DATA_HOME="+alias)
	if err := i.setup(context.Background()); err == nil {
		t.Fatal("accepted aliased XDG directories")
	}
	if len(f.calls) != 0 {
		t.Fatal("installed before validating paths")
	}
}

func TestCompilerInstallationCancellation(t *testing.T) {
	i, f := testInstaller(t)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	run := f.run
	i.run = func(ctx context.Context, env []string, program string, args ...string) (string, error) {
		switch filepath.Base(program) {
		case "xcrun":
			return "", errors.New("compiler missing")
		case "xcode-select":
			cancel()
			return "", nil
		default:
			return run(ctx, env, program, args...)
		}
	}
	if err := i.ensureCompiler(ctx); !errors.Is(err, context.Canceled) {
		t.Fatalf("expected cancelled compiler installation: %v", err)
	}
}

func TestMissingNeovimSuccessMarkerRollsBack(t *testing.T) {
	i, f := testInstaller(t)
	run := f.run
	i.run = func(ctx context.Context, env []string, program string, args ...string) (string, error) {
		if filepath.Base(program) == "nvim" {
			return "", nil
		}
		return run(ctx, env, program, args...)
	}
	if err := i.setup(context.Background()); err == nil || !strings.Contains(err.Error(), "did not finish") {
		t.Fatalf("expected marker error: %v", err)
	}
	for _, p := range i.paths {
		if _, err := os.Stat(p.path); !os.IsNotExist(err) {
			t.Fatalf("not cleaned: %s", p.path)
		}
	}
}

func TestShellProfilePreservedAndIdempotent(t *testing.T) {
	i, _ := testInstaller(t)
	profile := filepath.Join(i.home, ".zprofile")
	if err := os.WriteFile(profile, []byte("export EXISTING=value\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	for range 2 {
		if err := i.configureShell("/fake/brew"); err != nil {
			t.Fatal(err)
		}
	}
	text, _ := os.ReadFile(profile)
	if !strings.HasPrefix(string(text), "export EXISTING=value\n") || strings.Count(string(text), "shellenv") != 1 {
		t.Fatalf("profile corrupted: %s", text)
	}
}

func TestCancellationStillRestoresFilesAndPackages(t *testing.T) {
	i, f := testInstaller(t)
	paths := seedEnvironment(t, i)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	run := f.run
	i.run = func(ctx context.Context, env []string, program string, args ...string) (string, error) {
		if filepath.Base(program) == "nvim" {
			cancel()
			return "", ctx.Err()
		}
		return run(ctx, env, program, args...)
	}
	if err := i.setup(ctx); !errors.Is(err, context.Canceled) {
		t.Fatalf("expected cancellation: %v", err)
	}
	for n, p := range paths {
		contents, err := os.ReadFile(filepath.Join(p, "original"))
		if err != nil || string(contents) != fmt.Sprint(n) {
			t.Fatalf("file not restored: %s %v", p, err)
		}
	}
	if !reflect.DeepEqual(f.formulae, map[string]bool{"git": true, "shared-dependency": true}) {
		t.Fatalf("cancellation prevented package cleanup: %v", f.formulae)
	}
}
