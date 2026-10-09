package devenv

import (
	"bytes"
	"context"
	"embed"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"sync"
	"syscall"
	"time"

	"github.com/lukaculjak/mak-cli/internal/ui"
)

//go:embed assets
var assets embed.FS

var formulae = []string{"neovim", "git", "node", "go", "python", "ruby", "elixir", "ghc", "cabal-install", "haskell-language-server", "ripgrep", "fd", "fzf", "lazygit", "tree-sitter", "tree-sitter-cli", "unzip"}

const font = "font-jetbrains-mono-nerd-font"

type commandRunner func(context.Context, []string, string, ...string) (string, error)

type installer struct {
	home                        string
	env                         []string
	run                         commandRunner
	out                         io.Writer
	brew                        string
	beforeFormulae, beforeCasks map[string]bool
	paths                       []replacement
	shell                       *shellChange
	journal                     *setupJournal
	lockFile                    *os.File
	recoveredSetup              bool
}

type replacement struct {
	path, backup string
}

// Setup installs the bundled environment, retaining existing Neovim files as backups.
func Setup(ctx context.Context, in io.Reader, out io.Writer) error {
	if runtime.GOOS != "darwin" {
		return fmt.Errorf("mak setup dev currently supports macOS only")
	}
	if os.Geteuid() == 0 {
		return fmt.Errorf("run mak setup dev as your normal user, without sudo")
	}
	home, err := os.UserHomeDir()
	if err != nil {
		return err
	}
	i := &installer{home: home, env: os.Environ(), out: out}
	i.run = runner(in, out, func() *os.File { return i.lockFile })
	return i.setup(ctx)
}

func runner(in io.Reader, out io.Writer, lock func() *os.File) commandRunner {
	output := &lockedWriter{writer: out}
	return func(ctx context.Context, env []string, name string, args ...string) (string, error) {
		cmd := exec.CommandContext(ctx, name, args...)
		cmd.Env, cmd.Stdin = env, in
		// Keep the setup lock alive if mak is killed while a child still works.
		if lock != nil && lock() != nil {
			cmd.ExtraFiles = []*os.File{lock()}
		}
		// Homebrew's installer must share the foreground group to read terminal
		// input and sudo passwords. Other commands can be cancelled with children.
		if filepath.Base(name) != "bash" {
			cmd.SysProcAttr = &syscall.SysProcAttr{Setpgid: true}
			cmd.Cancel = func() error { return syscall.Kill(-cmd.Process.Pid, syscall.SIGKILL) }
		}
		cmd.WaitDelay = 5 * time.Second
		var stdout, stderr bytes.Buffer
		// Keep machine-readable stdout separate from Homebrew warnings on stderr.
		cmd.Stdout, cmd.Stderr = io.MultiWriter(output, &stdout), io.MultiWriter(output, &stderr)
		if filepath.Base(name) == "pgrep" || filepath.Base(name) == "xcrun" || (len(args) > 0 && (args[0] == "list" || args[0] == "info" || args[0] == "uses" || args[0] == "--prefix")) {
			cmd.Stdout, cmd.Stderr = &stdout, &stderr
		}
		err := cmd.Run()
		if err != nil {
			diagnostic := stdout.String() + stderr.String()
			if len(diagnostic) > 8192 {
				diagnostic = diagnostic[len(diagnostic)-8192:]
			}
			return stdout.String(), fmt.Errorf("%s: %w\n%s", filepath.Base(name), err, diagnostic)
		}
		return stdout.String(), nil
	}
}

type lockedWriter struct {
	mu     sync.Mutex
	writer io.Writer
}

func (w *lockedWriter) Write(p []byte) (int, error) {
	w.mu.Lock()
	defer w.mu.Unlock()
	return w.writer.Write(p)
}

func envValue(env []string, key string) string {
	for n := len(env) - 1; n >= 0; n-- {
		if v, ok := strings.CutPrefix(env[n], key+"="); ok {
			return v
		}
	}
	return ""
}

func setEnv(env []string, key, value string) []string {
	result := make([]string, 0, len(env)+1)
	for _, entry := range env {
		if !strings.HasPrefix(entry, key+"=") {
			result = append(result, entry)
		}
	}
	return append(result, key+"="+value)
}

func (i *installer) resolvePaths() error {
	i.paths = nil
	defaults := []struct{ key, path string }{
		{"XDG_CONFIG_HOME", ".config"}, {"XDG_DATA_HOME", ".local/share"},
		{"XDG_STATE_HOME", ".local/state"}, {"XDG_CACHE_HOME", ".cache"},
	}
	for _, d := range defaults {
		base := envValue(i.env, d.key)
		if base == "" {
			base = filepath.Join(i.home, d.path)
		}
		if !filepath.IsAbs(base) {
			return fmt.Errorf("%s must be an absolute path", d.key)
		}
		base, err := resolveDirectory(filepath.Clean(base))
		if err != nil {
			return fmt.Errorf("resolving %s: %w", d.key, err)
		}
		i.env = setEnv(i.env, d.key, base)
		i.paths = append(i.paths, replacement{path: filepath.Join(base, "nvim")})
	}
	for n, p := range i.paths {
		for _, q := range i.paths[:n] {
			if p.path == q.path || strings.HasPrefix(p.path, q.path+string(os.PathSeparator)) || strings.HasPrefix(q.path, p.path+string(os.PathSeparator)) {
				return fmt.Errorf("Neovim XDG directories must be separate: %s and %s", p.path, q.path)
			}
		}
	}
	return nil
}

func (i *installer) prepare(ctx context.Context) (func(), error) {
	i.recoveredSetup = false
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	if err := i.resolvePaths(); err != nil {
		return nil, err
	}
	lock := filepath.Join(envValue(i.env, "XDG_CONFIG_HOME"), "mak", "dev-setup.lock")
	if err := os.MkdirAll(filepath.Dir(lock), 0o755); err != nil {
		return nil, err
	}
	f, err := acquireSetupLock(lock)
	if err != nil {
		return nil, err
	}
	i.lockFile = f
	unlock := func() { f.Close(); i.lockFile = nil }
	if err := i.recoverPendingSetup(ctx); err != nil {
		unlock()
		return nil, err
	}
	return unlock, nil
}

func (i *installer) checkNeovim(ctx context.Context) error {
	if app := envValue(i.env, "NVIM_APPNAME"); app != "" && app != "nvim" {
		return fmt.Errorf("unset NVIM_APPNAME to manage the default nvim environment")
	}
	if processes, checkErr := i.run(ctx, i.env, "/usr/bin/pgrep", "-u", fmt.Sprint(os.Getuid()), "-x", "nvim"); checkErr == nil && strings.TrimSpace(processes) != "" {
		return fmt.Errorf("close your running Neovim sessions before running mak setup dev")
	}
	return ctx.Err()
}

func (i *installer) setup(ctx context.Context) (err error) {
	unlock, err := i.prepare(ctx)
	if err != nil {
		return err
	}
	defer unlock()
	if err := i.checkNeovim(ctx); err != nil {
		return err
	}
	record, err := i.readRecord()
	if err != nil && !os.IsNotExist(err) {
		return err
	}
	if record != nil {
		if record.Removing != "" {
			return fmt.Errorf("finish the previous removal with mak setup dev --remove %s before installing again", record.Removing)
		}
		if record.Uninstalling {
			return fmt.Errorf("finish the previous uninstall with mak setup dev --uninstall before installing again")
		}
		if err = i.checkPaths(record); err != nil {
			return err
		}
	}
	ctx, cancel := context.WithTimeout(ctx, 60*time.Minute)
	defer cancel()
	defer func() {
		if err == nil {
			return
		}
		committing := i.journal != nil && i.journal.Commit != nil
		if committing {
			ui.Warning(i.out, "Setup verified; completing its installation record. Files and recovery journal are retained until this succeeds.")
		} else {
			ui.Warning(i.out, "Setup failed. Removing the incomplete environment and restoring backups...")
		}
		// Cancellation must not prevent recovery.
		cleanupCtx, cleanupCancel := context.WithTimeout(context.Background(), 10*time.Minute)
		defer cleanupCancel()
		if cleanupErr := i.rollback(cleanupCtx); cleanupErr != nil {
			err = errors.Join(err, fmt.Errorf("recovery needs attention: %w; journal retained at %s", cleanupErr, i.journalPath()))
		} else if committing {
			err = fmt.Errorf("%w; verified environment and installation record recovered. Run mak doctor", err)
		} else {
			err = fmt.Errorf("%w; previous Neovim environment restored. Fix the reported problem and retry mak setup dev", err)
		}
	}()

	ui.Step(i.out, "Installing Luka's coding environment.")
	ui.Warning(i.out, "Existing Neovim files will be replaced with backups retained.")
	fmt.Fprintln(i.out, "Homebrew and Apple developer tools are shared prerequisites and remain after a failure.")
	if err = i.ensureBrew(ctx); err != nil {
		return fmt.Errorf("installing Homebrew: %w", err)
	}
	if record != nil && record.Brew != i.brew {
		return fmt.Errorf("Homebrew location changed; restore %s and uninstall the previous environment first", record.Brew)
	}
	if err = i.ensureCompiler(ctx); err != nil {
		return fmt.Errorf("installing Apple developer tools: %w", err)
	}
	i.env = setEnv(i.env, "HOMEBREW_NO_AUTO_UPDATE", "1")
	i.env = setEnv(i.env, "HOMEBREW_NO_INSTALL_UPGRADE", "1")
	i.env = setEnv(i.env, "HOMEBREW_NO_INSTALL_CLEANUP", "1")
	if i.beforeFormulae, err = i.inventory(ctx, "--formula"); err != nil {
		return err
	}
	if i.beforeCasks, err = i.inventory(ctx, "--cask"); err != nil {
		return err
	}
	if err = i.beginSetupJournal(true); err != nil {
		return err
	}
	ui.Step(i.out, "Installing Neovim, language runtimes, search tools and a Nerd Font...")
	if _, err = i.run(ctx, i.env, i.brew, append([]string{"install", "--formula"}, formulae...)...); err != nil {
		return fmt.Errorf("installing coding tools: %w", err)
	}
	if _, err = i.run(ctx, i.env, i.brew, "install", "--cask", font); err != nil {
		return fmt.Errorf("installing Nerd Font: %w", err)
	}
	prefix, err := i.run(ctx, i.env, i.brew, "--prefix")
	if err != nil {
		return err
	}
	prefix = strings.TrimSpace(prefix)
	if !filepath.IsAbs(prefix) || strings.ContainsAny(prefix, "\r\n") {
		return fmt.Errorf("invalid Homebrew prefix %q", prefix)
	}
	i.env = setEnv(i.env, "PATH", strings.Join([]string{filepath.Join(prefix, "opt/ruby/bin"), filepath.Join(prefix, "opt/python/libexec/bin"), filepath.Join(prefix, "bin"), filepath.Join(prefix, "sbin"), envValue(i.env, "PATH")}, string(os.PathListSeparator)))
	// Homebrew's tree-sitter formula supplies only the library. Require the CLI
	// before LazyVim can attempt competing Mason installs during plugin startup.
	ui.Step(i.out, "Checking Tree-sitter CLI...")
	if _, err = i.run(ctx, i.env, filepath.Join(prefix, "bin/tree-sitter"), "--version"); err != nil {
		return fmt.Errorf("Tree-sitter CLI could not run; install it with mak setup dev --install tree-sitter-cli and retry: %w", err)
	}
	for n := range i.paths {
		if err = i.replace(&i.paths[n]); err != nil {
			return fmt.Errorf("backing up Neovim: %w", err)
		}
	}
	config, data := i.paths[0].path, i.paths[1].path
	if err = writeConfig(config); err != nil {
		return fmt.Errorf("writing bundled configuration: %w", err)
	}
	if err = os.WriteFile(filepath.Join(config, "mak-brew-prefix"), []byte(prefix+"\n"), 0o644); err != nil {
		return err
	}
	var lockfile map[string]struct {
		Commit string `json:"commit"`
	}
	b, _ := assets.ReadFile("assets/nvim/lazy-lock.json")
	if err = json.Unmarshal(b, &lockfile); err != nil {
		return err
	}
	for _, plugin := range []struct{ name, repo string }{
		{"lazy.nvim", "https://github.com/folke/lazy.nvim.git"},
		{"LazyVim", "https://github.com/LazyVim/LazyVim.git"},
	} {
		target := filepath.Join(data, "lazy", plugin.name)
		if err = os.MkdirAll(filepath.Dir(target), 0o755); err != nil {
			return err
		}
		ui.Step(i.out, "Bootstrapping %s at %s...", plugin.name, lockfile[plugin.name].Commit)
		git := filepath.Join(prefix, "bin/git")
		if _, err = i.run(ctx, i.env, git, "clone", "--filter=blob:none", "--no-checkout", plugin.repo, target); err != nil {
			return err
		}
		if _, err = i.run(ctx, i.env, git, "-C", target, "checkout", "--detach", lockfile[plugin.name].Commit); err != nil {
			return err
		}
	}
	script, err := os.CreateTemp("", "mak-nvim-*.lua")
	if err != nil {
		return err
	}
	defer os.Remove(script.Name())
	scriptBytes, _ := assets.ReadFile("assets/setup.lua")
	if _, err = script.Write(scriptBytes); err != nil {
		script.Close()
		return err
	}
	if err = script.Close(); err != nil {
		return err
	}
	// Run from the configuration directory so project plugins cannot influence setup.
	i.env = setEnv(i.env, "MAK_NVIM_CONFIG", config)
	for _, phase := range []string{"plugins", "tools", "verify"} {
		ui.Step(i.out, "Neovim setup: %s...", phase)
		i.env = setEnv(i.env, "MAK_NVIM_PHASE", phase)
		marker := filepath.Join(config, ".mak-"+phase+"-ok")
		if _, err = i.run(ctx, i.env, filepath.Join(prefix, "bin/nvim"), "--headless", "-u", "NONE", "-l", script.Name()); err != nil {
			return fmt.Errorf("Neovim %s phase: %w", phase, err)
		}
		if _, err = os.Stat(marker); err != nil {
			return fmt.Errorf("Neovim %s phase did not finish successfully", phase)
		}
		if err = os.Remove(marker); err != nil {
			return err
		}
	}
	if err = i.configureShell(prefix); err != nil {
		return fmt.Errorf("configuring shell PATH: %w", err)
	}
	if err = i.recordInstallation(ctx, record); err != nil {
		return fmt.Errorf("saving installation record: %w", err)
	}
	ui.Success(i.out, "Coding environment ready at %s.", config)
	fmt.Fprintln(i.out, "Open a new terminal, or refresh this zsh/bash session with: eval \"$(mak shellenv)\"")
	fmt.Fprintln(i.out, "Then run nvim.")
	fmt.Fprintln(i.out, "Select JetBrainsMono Nerd Font in your terminal's font settings.")
	for _, p := range i.paths {
		if p.backup != "" {
			fmt.Fprintf(i.out, "Previous files saved at %s\n", p.backup)
		}
	}
	return nil
}

// Resolve existing ancestors so XDG aliases cannot hide overlapping directories.
// Do not resolve the final nvim entry: a dotfiles symlink must itself be backed up.
func resolveDirectory(path string) (string, error) {
	ancestor := path
	var suffix []string
	for {
		if _, err := os.Lstat(ancestor); err == nil {
			break
		} else if !os.IsNotExist(err) {
			return "", err
		}
		suffix = append(suffix, filepath.Base(ancestor))
		ancestor = filepath.Dir(ancestor)
	}
	resolved, err := filepath.EvalSymlinks(ancestor)
	if err != nil {
		return "", err
	}
	for n := len(suffix) - 1; n >= 0; n-- {
		resolved = filepath.Join(resolved, suffix[n])
	}
	return resolved, nil
}

func (i *installer) ensureCompiler(ctx context.Context) error {
	if _, err := i.run(ctx, i.env, "/usr/bin/xcrun", "--find", "clang"); err == nil {
		return nil
	}
	ui.Step(i.out, "Apple developer tools are required. Complete Apple's installation dialog; mak will wait for it to finish...")
	if _, err := i.run(ctx, i.env, "/usr/bin/xcode-select", "--install"); err != nil {
		return fmt.Errorf("complete xcode-select --install and retry: %w", err)
	}
	waitCtx, cancel := context.WithTimeout(ctx, 20*time.Minute)
	defer cancel()
	ticker := time.NewTicker(5 * time.Second)
	defer ticker.Stop()
	for {
		select {
		case <-waitCtx.Done():
			return fmt.Errorf("Apple developer tools were not ready; finish their installation and retry: %w", waitCtx.Err())
		case <-ticker.C:
			if _, err := i.run(waitCtx, i.env, "/usr/bin/xcrun", "--find", "clang"); err == nil {
				return nil
			}
		}
	}
}

func (i *installer) ensureBrew(ctx context.Context) error {
	if i.findBrew() {
		return nil
	}
	candidates := []string{filepath.Join("/opt/homebrew", "bin/brew"), "/usr/local/bin/brew"}
	ui.Step(i.out, "Installing Homebrew using its official installer (may request an administrator password)...")
	tmp, err := os.CreateTemp("", "mak-homebrew-*.sh")
	if err != nil {
		return err
	}
	tmp.Close()
	defer os.Remove(tmp.Name())
	if _, err = i.run(ctx, i.env, "/usr/bin/curl", "--fail", "--show-error", "--location", "--max-time", "120", "https://raw.githubusercontent.com/Homebrew/install/HEAD/install.sh", "--output", tmp.Name()); err != nil {
		return err
	}
	if _, err = i.run(ctx, i.env, "/bin/bash", tmp.Name()); err != nil {
		return err
	}
	for _, p := range candidates {
		if _, err = os.Stat(p); err == nil {
			i.brew = p
			return nil
		}
	}
	return fmt.Errorf("Homebrew installer finished but brew was not found in its standard macOS locations")
}

func (i *installer) findBrew() bool {
	if i.brew != "" {
		return true
	}
	candidates := []string{filepath.Join("/opt/homebrew", "bin/brew"), "/usr/local/bin/brew"}
	if p, err := exec.LookPath("brew"); err == nil {
		candidates = append([]string{p}, candidates...)
	}
	for _, p := range candidates {
		if st, err := os.Stat(p); err == nil && !st.IsDir() {
			i.brew = p
			return true
		}
	}
	return false
}

func (i *installer) inventory(ctx context.Context, kind string) (map[string]bool, error) {
	text, err := i.run(ctx, i.env, i.brew, "list", kind, "-1")
	if err != nil {
		return nil, fmt.Errorf("listing Homebrew packages: %w", err)
	}
	result := map[string]bool{}
	for _, name := range strings.Fields(text) {
		result[name] = true
	}
	return result, nil
}

func (i *installer) replace(p *replacement) error {
	if i.journal == nil {
		return fmt.Errorf("setup journal is required before replacing files")
	}
	return i.replaceJournaled(p)
}

func writeConfig(target string) error {
	return fs.WalkDir(assets, "assets/nvim", func(path string, entry fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		rel, err := filepath.Rel("assets/nvim", path)
		if err != nil {
			return err
		}
		dest := filepath.Join(target, rel)
		if entry.IsDir() {
			return os.MkdirAll(dest, 0o755)
		}
		contents, err := assets.ReadFile(path)
		if err != nil {
			return err
		}
		return os.WriteFile(dest, contents, 0o644)
	})
}

func (i *installer) rollback(ctx context.Context) error {
	if i.journal != nil {
		return i.recoverSetupJournal(ctx)
	}
	return i.rollbackPackages(ctx)
}

func (i *installer) rollbackPackages(ctx context.Context) error {
	var errs []error
	for _, inventory := range []struct {
		kind   string
		before map[string]bool
	}{
		{"--cask", i.beforeCasks}, {"--formula", i.beforeFormulae},
	} {
		if inventory.before == nil {
			continue
		}
		current, err := i.inventory(ctx, inventory.kind)
		if err != nil {
			errs = append(errs, err)
			continue
		}
		args := []string{"uninstall", inventory.kind}
		// All new formulae, including dependencies, are removed together.
		if inventory.kind == "--formula" {
			args = append(args, "--ignore-dependencies")
		}
		for name := range current {
			if !inventory.before[name] {
				args = append(args, name)
			}
		}
		if len(args) > 2 && !(inventory.kind == "--formula" && len(args) == 3) {
			if _, err = i.run(ctx, setEnv(i.env, "HOMEBREW_NO_AUTOREMOVE", "1"), i.brew, args...); err != nil {
				errs = append(errs, err)
			}
		}
	}
	return errors.Join(errs...)
}

func (i *installer) configureShell(prefix string) error {
	name := envValue(i.env, "SHELL")
	if name != "" {
		name = filepath.Base(name)
	}
	profile := ".zprofile"
	switch name {
	case "", "zsh":
	case "bash":
		profile = ".bash_profile"
	default:
		return fmt.Errorf("unsupported login shell %q; use zsh or bash", name)
	}
	if name == "zsh" {
		if dir := envValue(i.env, "ZDOTDIR"); dir != "" {
			if !filepath.IsAbs(dir) {
				return fmt.Errorf("ZDOTDIR must be absolute")
			}
			profile = filepath.Join(dir, profile)
		}
	}
	path := profile
	if !filepath.IsAbs(path) {
		path = filepath.Join(i.home, path)
	}
	previous, err := os.ReadFile(path)
	if err != nil && !os.IsNotExist(err) {
		return err
	}
	// Shellenv is safe to repeat and does not replace the user's shell settings.
	quoted := "'" + strings.ReplaceAll(filepath.Join(prefix, "bin/brew"), "'", "'\\''") + "'"
	line := "eval \"$(" + quoted + " shellenv)\""
	if strings.Contains(string(previous), line) {
		return nil
	}
	resolved, err := resolveDirectory(path)
	if err != nil {
		return err
	}
	block := "\n# Homebrew coding tools (mak setup dev)\n" + line + "\n"
	i.shell = &shellChange{Path: resolved, Block: block, Created: previous == nil}
	if i.journal != nil {
		i.journal.Record.Shell = []shellChange{*i.shell}
		if err := i.saveSetupJournal(); err != nil {
			return err
		}
	}
	mode := os.FileMode(0o644)
	if info, err := os.Stat(resolved); err == nil {
		mode = info.Mode().Perm()
	}
	return atomicWrite(resolved, append(previous, block...), mode)
}
