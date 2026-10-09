package devenv

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"time"

	"github.com/lukaculjak/mak-cli/internal/ui"
)

var ErrUnhealthy = errors.New("coding environment checks failed")

type diagnostic struct {
	Status string `json:"status"`
	Label  string `json:"label"`
	Detail string `json:"detail"`
	Repair string `json:"repair"`
}

type healthReport struct {
	out              io.Writer
	errors, warnings int
}

func (r *healthReport) add(status, label, detail, repair string) {
	message := label
	if detail != "" {
		message += ": " + detail
	}
	switch status {
	case "ok":
		ui.Success(r.out, "%s", message)
	case "warning":
		r.warnings++
		ui.Warning(r.out, "%s", message)
	default:
		r.errors++
		ui.Error(r.out, "%s", message)
	}
	if repair != "" {
		fmt.Fprintf(r.out, "  Suggested repair: %s\n", repair)
	}
}

// Doctor diagnoses the coding environment without installing, updating or repairing it.
func Doctor(ctx context.Context, out io.Writer) error {
	if runtime.GOOS != "darwin" {
		return fmt.Errorf("mak's bundled coding environment currently supports macOS only")
	}
	home, err := os.UserHomeDir()
	if err != nil {
		return err
	}
	ctx, cancel := context.WithTimeout(ctx, 5*time.Minute)
	defer cancel()
	i := &installer{home: home, env: os.Environ(), out: out, run: runner(strings.NewReader(""), io.Discard, nil)}
	return i.doctor(ctx)
}

func executable(path, name string) string {
	for _, dir := range filepath.SplitList(path) {
		candidate := filepath.Join(dir, name)
		if st, err := os.Stat(candidate); err == nil && !st.IsDir() && st.Mode().Perm()&0o111 != 0 {
			return candidate
		}
	}
	return ""
}

func (i *installer) doctor(ctx context.Context) error {
	r := &healthReport{out: i.out}
	ui.Heading(i.out, "Coding environment health")
	fmt.Fprintln(i.out, "Read-only diagnostics. No software or configuration will be changed.")
	if err := i.resolvePaths(); err != nil {
		r.add("error", "XDG directories", err.Error(), "Correct the XDG directory settings and rerun mak doctor")
		return ErrUnhealthy
	}
	path := envValue(i.env, "PATH")
	if app := envValue(i.env, "NVIM_APPNAME"); app != "" && app != "nvim" {
		r.add("warning", "NVIM_APPNAME", "Checking the default nvim environment; your shell selects "+app, "unset NVIM_APPNAME; mak doctor")
	}
	for _, tool := range []struct{ name, pkg string }{
		{"nvim", "neovim"}, {"git", "git"}, {"node", "node"}, {"npm", "node"}, {"go", "go"},
		{"python3", "python"}, {"ruby", "ruby"}, {"elixir", "elixir"}, {"ghc", "ghc"},
		{"cabal", "cabal-install"}, {"haskell-language-server-wrapper", "haskell-language-server"},
		{"rg", "ripgrep"}, {"fd", "fd"}, {"fzf", "fzf"}, {"lazygit", "lazygit"}, {"tree-sitter", "tree-sitter-cli"}, {"unzip", "unzip"},
	} {
		if err := ctx.Err(); err != nil {
			return err
		}
		if found := executable(path, tool.name); found != "" {
			r.add("ok", "PATH: "+tool.name, found, "")
		} else {
			r.add("error", "PATH: "+tool.name, "Executable is unavailable in this shell", "Open a new terminal; if still missing, run mak setup dev --install "+tool.pkg)
		}
	}
	if _, err := i.run(ctx, i.env, "/usr/bin/xcrun", "--find", "clang"); err != nil {
		r.add("error", "Apple developer tools", err.Error(), "xcode-select --install (complete Apple's installation dialog)")
	} else {
		r.add("ok", "Apple developer tools", "C compiler is available", "")
	}
	var installedFormulae, installedCasks map[string]bool
	if i.findBrew() {
		status, repair := "ok", ""
		if executable(path, "brew") == "" {
			status, repair = "warning", "Open a new terminal or source your login-shell profile"
		}
		r.add(status, "Homebrew", i.brew, repair)
		i.env = setEnv(i.env, "HOMEBREW_NO_AUTO_UPDATE", "1")
		i.env = setEnv(i.env, "HOMEBREW_NO_ANALYTICS", "1")
		var err error
		installedFormulae, err = i.inventory(ctx, "--formula")
		if err == nil {
			installedCasks, err = i.inventory(ctx, "--cask")
		}
		if err != nil {
			r.add("error", "Homebrew inventory", err.Error(), "brew doctor")
		} else if installedCasks[font] {
			r.add("ok", "Nerd Font", "JetBrains Mono Nerd Font is installed; terminal font selection is manual", "")
		} else {
			r.add("warning", "Nerd Font", "JetBrains Mono Nerd Font is not installed via Homebrew", "mak setup dev --install font")
		}
	} else {
		r.add("error", "Homebrew", "Not found", "mak setup dev")
	}

	ui.Heading(i.out, "Installation tracking and configuration")
	record, err := i.readRecord()
	switch {
	case os.IsNotExist(err):
		r.add("warning", "Installation tracking", "No recovery record; preexisting or older installations are not automatically removable", "Use mak setup dev for a tracked setup (backs up and replaces the current environment)")
	case err != nil:
		r.add("error", "Installation tracking", err.Error(), "Recover the original record at "+i.recordPath()+" from your backup; do not delete Neovim backups")
	default:
		r.add("ok", "Installation tracking", i.recordPath(), "")
		if record.Uninstalling || record.Removing != "" {
			repair := "mak setup dev --uninstall"
			if record.Removing != "" {
				repair = "mak setup dev --remove " + record.Removing
			}
			r.add("warning", "Pending removal", "The last removal has not completed", repair)
		}
		if record.Brew != i.brew {
			r.add("error", "Homebrew tracking", "Recorded Homebrew location differs: "+record.Brew, "Restore the recorded Homebrew location before changing tracked packages")
		}
		for _, packages := range []struct {
			owned   []string
			current map[string]bool
		}{{record.Formulae, installedFormulae}, {record.Casks, installedCasks}} {
			if packages.current == nil {
				continue
			}
			for _, name := range packages.owned {
				if !packages.current[name] {
					r.add("warning", "Tracked package: "+name, "No longer installed", "mak setup dev --list; reinstall the corresponding listed package if needed")
				}
			}
		}
		if err := i.checkPaths(record); err != nil {
			r.add("error", "Recovery files", err.Error(), "Restore the missing backup/ownership files, or use the original XDG settings; inspect the installation record before changing files")
		} else if len(record.Paths) != 0 {
			r.add("ok", "Recovery files", "Managed directories and original backups are intact", "")
		}
	}
	configOK, changed, err := bundledConfigStatus(i.paths[0].path)
	if err != nil {
		return err
	}
	if !configOK {
		r.add("error", "Bundled Neovim files", "Required configuration files are missing or unreadable at "+i.paths[0].path, "mak setup dev (backs up and replaces the current environment)")
	} else if len(changed) != 0 {
		r.add("warning", "Bundled Neovim files", "Local edits in "+strings.Join(changed, ", "), "To restore bundled defaults: mak setup dev (backs up local edits)")
	} else {
		r.add("ok", "Bundled Neovim files", "Match this mak version", "")
	}
	lock := filepath.Join(envValue(i.env, "XDG_CONFIG_HOME"), "mak", "dev-setup.lock")
	pending := false
	if _, err := i.readSetupJournal(); err == nil {
		pending = true
		r.add("warning", "Interrupted setup", "Recovery journal is present; runtime checks skipped", "Close Neovim and rerun your setup/install/remove command to recover automatically")
	} else if !os.IsNotExist(err) {
		pending = true
		r.add("error", "Setup recovery journal", err.Error(), "Inspect "+i.journalPath()+" and use the original XDG settings; preserve backups")
	}
	locked, lockErr := setupLocked(lock)
	if lockErr == nil && locked {
		r.add("warning", "Setup lock", "A setup/removal may be running, or a legacy lock directory remains; runtime checks skipped", "Wait for active processes to finish. For legacy lock directories, inspect backups before removing the stale directory")
	} else if lockErr != nil {
		r.add("error", "Setup lock", lockErr.Error(), "Correct access to "+lock)
	} else if configOK && !pending {
		i.doctorNeovim(ctx, r)
	}
	if err := ctx.Err(); err != nil {
		return err
	}
	fmt.Fprintln(i.out)
	if r.errors != 0 {
		ui.Error(i.out, "Doctor found %d error(s) and %d warning(s).", r.errors, r.warnings)
		return ErrUnhealthy
	}
	if r.warnings != 0 {
		ui.Warning(i.out, "Checks completed with %d warning(s).", r.warnings)
	} else {
		ui.Success(i.out, "Coding environment checks passed.")
	}
	return nil
}

func bundledConfigStatus(config string) (bool, []string, error) {
	complete := true
	var changed []string
	err := fs.WalkDir(assets, "assets/nvim", func(path string, entry fs.DirEntry, err error) error {
		if err != nil || entry.IsDir() {
			return err
		}
		rel, _ := filepath.Rel("assets/nvim", path)
		actual, err := os.ReadFile(filepath.Join(config, rel))
		if err != nil {
			complete = false
			return nil
		}
		expected, _ := assets.ReadFile(path)
		if !bytes.Equal(actual, expected) {
			changed = append(changed, rel)
		}
		return nil
	})
	return complete, changed, err
}

func (i *installer) doctorNeovim(ctx context.Context, r *healthReport) {
	nvim := executable(envValue(i.env, "PATH"), "nvim")
	if nvim == "" {
		return
	}
	for _, name := range []string{"lazy.nvim", "LazyVim"} {
		if _, err := os.Stat(filepath.Join(i.paths[1].path, "lazy", name, "lua")); err != nil {
			r.add("error", "Plugin: "+name, "Missing or unreadable", "mak setup dev")
			return
		}
	}
	if _, err := os.Stat("/usr/bin/sandbox-exec"); err != nil {
		r.add("warning", "Neovim runtime checks", "macOS sandbox-exec is unavailable; startup cannot be checked read-only", "Run mak doctor on a macOS installation with sandbox-exec available")
		return
	}
	tmp, err := os.MkdirTemp("", "mak-doctor-*")
	if err != nil {
		r.add("error", "Temporary diagnostics", err.Error(), "Check TMPDIR permissions and available disk space")
		return
	}
	defer os.RemoveAll(tmp)
	tmp, err = filepath.EvalSymlinks(tmp)
	if err != nil {
		r.add("error", "Temporary diagnostics", err.Error(), "Check TMPDIR and rerun mak doctor")
		return
	}
	if err := copyDirectory(i.paths[0].path, filepath.Join(tmp, "config/nvim"), true); err != nil {
		r.add("error", "Neovim configuration snapshot", err.Error(), "Correct access to the Neovim configuration and rerun mak doctor")
		return
	}
	if err := os.MkdirAll(filepath.Join(tmp, "home"), 0o700); err != nil {
		r.add("error", "Temporary diagnostics", err.Error(), "Check TMPDIR permissions and available disk space")
		return
	}
	if err := i.copyMixCaches(tmp); err != nil {
		r.add("error", "Elixir diagnostics cache", err.Error(), "Correct access to the existing Mix cache and rerun mak doctor")
		return
	}
	script, _ := assets.ReadFile("assets/doctor.lua")
	scriptPath, profile := filepath.Join(tmp, "doctor.lua"), filepath.Join(tmp, "sandbox.sb")
	policy := "(version 1)\n(allow default)\n(deny file-write*)\n(allow file-write* (subpath " + strconv.Quote(tmp) + ") (literal \"/dev/null\"))\n(deny network*)\n"
	if err := errors.Join(os.WriteFile(scriptPath, script, 0o600), os.WriteFile(profile, []byte(policy), 0o600)); err != nil {
		r.add("error", "Temporary diagnostics", err.Error(), "Check TMPDIR permissions and available disk space")
		return
	}
	env := i.env
	for key, value := range map[string]string{
		"NVIM_APPNAME": "nvim",
		"HOME":         filepath.Join(tmp, "home"), "XDG_CONFIG_HOME": filepath.Join(tmp, "config"), "XDG_STATE_HOME": filepath.Join(tmp, "state"), "XDG_CACHE_HOME": filepath.Join(tmp, "cache"),
		"TMPDIR": tmp, "GOCACHE": filepath.Join(tmp, "go-cache"), "GOMODCACHE": filepath.Join(tmp, "go-mod-cache"), "GOTELEMETRY": "off",
		"MIX_HOME": filepath.Join(tmp, "mix-home"), "MIX_INSTALL_DIR": filepath.Join(tmp, "mix-installs"),
		// The private cache is used by one process; Mix's TCP lock is unnecessary
		// and cannot operate with networking blocked in the diagnostic sandbox.
		"MIX_OS_CONCURRENCY_LOCK": "0",
		"MAK_DOCTOR_RESULT":       filepath.Join(tmp, "result.json"), "MAK_DOCTOR_WORKSPACE": filepath.Join(tmp, "workspace"),
	} {
		env = setEnv(env, key, value)
	}
	ui.Step(i.out, "Checking Neovim startup, locked plugins, parsers and language servers (downloads and installation writes blocked)...")
	_, runErr := i.run(ctx, env, "/usr/bin/sandbox-exec", "-f", profile, nvim, "--headless", "-u", "NONE", "-i", "NONE", "-S", scriptPath)
	b, readErr := os.ReadFile(filepath.Join(tmp, "result.json"))
	var result struct {
		Checks   []diagnostic `json:"checks"`
		Complete bool         `json:"complete"`
	}
	if readErr == nil {
		readErr = json.Unmarshal(b, &result)
	}
	for _, check := range result.Checks {
		r.add(check.Status, check.Label, check.Detail, check.Repair)
	}
	if runErr != nil || readErr != nil || !result.Complete || len(result.Checks) == 0 {
		detail := "Neovim did not complete diagnostics"
		if runErr != nil {
			detail = runErr.Error()
		} else if readErr != nil {
			detail = readErr.Error()
		}
		r.add("error", "Neovim runtime checks", detail, "Run mak doctor from a normal terminal; inspect the reported startup/sandbox error")
	}
}

// ElixirLS uses Mix.install outside Mason. Diagnose a copy of its compiled cache,
// so starting the server neither recompiles into the real cache nor downloads it.
func (i *installer) copyMixCaches(tmp string) error {
	mixHome := envValue(i.env, "MIX_HOME")
	if mixHome == "" {
		mixHome = filepath.Join(i.home, ".mix")
		if envValue(i.env, "MIX_XDG") != "" {
			mixHome = filepath.Join(envValue(i.env, "XDG_DATA_HOME"), "mix")
		}
	}
	if _, err := os.Stat(mixHome); err == nil {
		if err := copyDirectory(mixHome, filepath.Join(tmp, "mix-home"), false); err != nil {
			return err
		}
	} else if !os.IsNotExist(err) {
		return err
	}
	cache := envValue(i.env, "MIX_INSTALL_DIR")
	if cache == "" {
		cache = filepath.Join(i.home, "Library/Caches/mix/installs")
		if envValue(i.env, "MIX_XDG") != "" {
			cache = filepath.Join(envValue(i.env, "XDG_CACHE_HOME"), "mix/installs")
		}
	}
	versions, err := os.ReadDir(cache)
	if os.IsNotExist(err) {
		return nil
	}
	if err != nil {
		return err
	}
	for _, version := range versions {
		if !version.IsDir() {
			continue
		}
		projects, err := os.ReadDir(filepath.Join(cache, version.Name()))
		if err != nil {
			return err
		}
		for _, project := range projects {
			source := filepath.Join(cache, version.Name(), project.Name())
			if _, err := os.Stat(filepath.Join(source, "deps/elixir_ls")); err == nil {
				if err := copyDirectory(source, filepath.Join(tmp, "mix-installs", version.Name(), project.Name()), false); err != nil {
					return err
				}
			}
		}
	}
	return nil
}

func copyDirectory(source, target string, skipGit bool) error {
	source, err := filepath.EvalSymlinks(source)
	if err != nil {
		return err
	}
	return filepath.WalkDir(source, func(path string, entry fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		rel, _ := filepath.Rel(source, path)
		dest := filepath.Join(target, rel)
		if entry.IsDir() {
			if skipGit && entry.Name() == ".git" {
				return filepath.SkipDir
			}
			return os.MkdirAll(dest, 0o700)
		}
		if entry.Type()&os.ModeSymlink != 0 {
			resolved, err := filepath.EvalSymlinks(path)
			if err != nil {
				return err
			}
			return os.Symlink(resolved, dest)
		}
		info, err := entry.Info()
		if err != nil {
			return err
		}
		if !info.Mode().IsRegular() {
			return fmt.Errorf("cannot snapshot non-regular file %s", path)
		}
		b, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		return os.WriteFile(dest, b, 0o600|info.Mode().Perm()&0o111)
	})
}
