package devenv

import (
	"bytes"
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"slices"
	"strings"
	"testing"
	"time"
)

func externalTool(t *testing.T, i *installer, name string) string {
	t.Helper()
	path := filepath.Join(i.home, "bin", name)
	if err := os.MkdirAll(filepath.Dir(path), 0700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte("#!/bin/sh\nexit 0\n"), 0700); err != nil {
		t.Fatal(err)
	}
	return path
}

func TestInstallReusesExternalCommandsWithoutClaimingThem(t *testing.T) {
	i, f := testInstaller(t)
	var out bytes.Buffer
	i.out = &out
	for _, commands := range packageCommands {
		for _, command := range commands {
			externalTool(t, i, command)
		}
	}
	i.run = func(ctx context.Context, env []string, program string, args ...string) (string, error) {
		name := filepath.Base(program)
		if slices.Contains([]string{"git", "nvim", "tree-sitter"}, name) && program != filepath.Join(i.home, "bin", name) {
			t.Fatalf("setup ignored the external executable: %s", program)
		}
		return f.run(ctx, env, program, args...)
	}
	if err := i.setup(context.Background()); err != nil {
		t.Fatal(err)
	}
	r := mustRecord(t, i)
	for name := range packageCommands {
		if name == "git" { // This fixture already has Homebrew Git.
			continue
		}
		if slices.Contains(r.Formulae, name) || slices.Contains(r.Casks, name) || r.Packages[name] != "" {
			t.Fatalf("claimed external %s: %+v", name, r)
		}
		if f.formulae[name] || f.casks[name] {
			t.Fatalf("installed duplicate %s", name)
		}
	}
	if !f.formulae["tree-sitter"] || !f.casks[font] {
		t.Fatal("binary detection incorrectly skipped a library or font")
	}
	if !strings.Contains(out.String(), "Keeping existing node:") {
		t.Fatal("skipped executable was not reported:", out.String())
	}
	for _, name := range []string{"nvim", "git", "tree-sitter"} {
		if !slices.ContainsFunc(f.calls, func(call string) bool { return strings.HasPrefix(call, name+" ") }) {
			t.Fatalf("setup did not execute existing %s", name)
		}
	}
	if err := nextInvocation(i, f).uninstall(context.Background()); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(filepath.Join(i.home, "bin", "node")); err != nil {
		t.Fatal("uninstall removed external Node:", err)
	}
}

func TestBundledOptionsKeepVersionManagerPrecedence(t *testing.T) {
	nvim, err := exec.LookPath("nvim")
	if err != nil {
		t.Skip("Neovim unavailable")
	}
	root := t.TempDir()
	config := filepath.Join(root, "config", "nvim")
	if err := os.MkdirAll(config, 0700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(config, "mak-brew-prefix"), []byte("/test-brew\n"), 0600); err != nil {
		t.Fatal(err)
	}
	options, err := assets.ReadFile("assets/nvim/lua/config/options.lua")
	if err != nil {
		t.Fatal(err)
	}
	script := `vim.env.PATH = "/test-brew/bin:/project/nvm/bin:/project/rbenv/shims:/usr/bin:/bin"` + "\n" + string(options) + `
assert(vim.env.PATH == "/project/nvm/bin:/project/rbenv/shims:/test-brew/opt/ruby/bin:/test-brew/opt/python/libexec/bin:/test-brew/bin:/test-brew/sbin:/usr/bin:/bin", vim.env.PATH)
vim.cmd("qa!")
`
	path := filepath.Join(root, "options-test.lua")
	if err := os.WriteFile(path, []byte(script), 0600); err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	cmd := exec.CommandContext(ctx, nvim, "--headless", "-u", "NONE", "-i", "NONE", "-S", path)
	cmd.Env = os.Environ()
	for key, value := range map[string]string{"HOME": root, "XDG_CONFIG_HOME": filepath.Join(root, "config"), "XDG_DATA_HOME": filepath.Join(root, "data"), "XDG_STATE_HOME": filepath.Join(root, "state"), "XDG_CACHE_HOME": filepath.Join(root, "cache"), "NVIM_APPNAME": "nvim", "NVIM_LOG_FILE": filepath.Join(root, "nvim.log")} {
		cmd.Env = setEnv(cmd.Env, key, value)
	}
	if output, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("%v: %s", err, output)
	}
}

func TestIndividualInstallRequiresAllPackageCommands(t *testing.T) {
	for _, npm := range []bool{false, true} {
		t.Run(map[bool]string{false: "node without npm", true: "node and npm"}[npm], func(t *testing.T) {
			i, f := testInstaller(t)
			externalTool(t, i, "node")
			if npm {
				externalTool(t, i, "npm")
			}
			if err := i.installPackage(context.Background(), packageForTest(t, "node")); err != nil {
				t.Fatal(err)
			}
			if npm {
				if len(f.calls) != 0 {
					t.Fatalf("existing commands triggered Homebrew: %v", f.calls)
				}
				if _, err := os.Stat(i.recordPath()); !os.IsNotExist(err) {
					t.Fatal("external installation was recorded as owned")
				}
				var out bytes.Buffer
				i.out = &out
				if err := i.listPackages(context.Background()); err != nil {
					t.Fatal(err)
				}
				if !strings.Contains(out.String(), "[x] node") || !strings.Contains(out.String(), "existing:") {
					t.Fatal("external Node not listed:", out.String())
				}
			} else if !f.formulae["node"] {
				t.Fatal("skipped Node package despite missing npm")
			}
		})
	}
}

func TestAvailabilityRejectsNonExecutablesAndFindsGhosttyApp(t *testing.T) {
	i, f := testInstaller(t)
	path := externalTool(t, i, "go")
	if err := os.Chmod(path, 0600); err != nil {
		t.Fatal(err)
	}
	if i.availablePackage("go") != "" {
		t.Fatal("non-executable file treated as installed")
	}
	if err := os.Remove(path); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(filepath.Join(i.home, "missing"), path); err != nil {
		t.Fatal(err)
	}
	if i.availablePackage("go") != "" {
		t.Fatal("broken symlink treated as installed")
	}
	app := filepath.Join(i.applications, "Ghostty.app", "Contents", "MacOS", "ghostty")
	if err := os.MkdirAll(filepath.Dir(app), 0700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(app, []byte("#!/bin/sh\n"), 0700); err != nil {
		t.Fatal(err)
	}
	if got := i.availablePackage("ghostty"); got != app {
		t.Fatalf("Ghostty app not detected: %q", got)
	}
	if err := i.installPackage(context.Background(), packageForTest(t, "ghostty")); err != nil {
		t.Fatal(err)
	}
	r := mustRecord(t, i)
	if f.casks["ghostty"] || slices.Contains(r.Casks, "ghostty") {
		t.Fatal("installed or claimed the external Ghostty app")
	}
	if err := nextInvocation(i, f).uninstall(context.Background()); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(app); err != nil {
		t.Fatal("removed the external Ghostty app:", err)
	}
}

func TestInstallResolvesVersionedFormulaAfterCheckingExternalBinary(t *testing.T) {
	i, f := testInstaller(t)
	f.aliases = map[string]string{"python": "python@3.14"}
	externalTool(t, i, "python3")
	if err := i.installMissing(context.Background(), "--formula", []string{"python", "tree-sitter"}, map[string]string{"python": "python@3.14", "tree-sitter": "tree-sitter"}, f.formulae); err != nil {
		t.Fatal(err)
	}
	if f.formulae["python@3.14"] || !f.formulae["tree-sitter"] {
		t.Fatalf("incorrect installs: %v", f.formulae)
	}
}
