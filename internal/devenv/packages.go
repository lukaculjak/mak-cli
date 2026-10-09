package devenv

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"runtime"
	"slices"
	"strings"
	"time"

	"github.com/lukaculjak/mak-cli/internal/ui"
)

type packageSpec struct {
	name, description string
	cask              bool
}

func packageCatalog() []packageSpec {
	descriptions := map[string]string{
		"neovim": "Neovim + full bundled LazyVim environment (alias: nvim)",
		"git":    "Git version control", "node": "Node.js and npm",
		"go": "Go toolchain", "python": "Python runtime", "ruby": "Ruby runtime",
		"elixir": "Elixir and its Erlang dependency", "ghc": "Glasgow Haskell Compiler",
		"cabal-install": "Haskell build and package tools", "haskell-language-server": "Haskell language server",
		"ripgrep": "Fast text search", "fd": "File search", "fzf": "Fuzzy finder",
		"lazygit": "Terminal Git interface", "tree-sitter": "Tree-sitter parsing library",
		"tree-sitter-cli": "Tree-sitter parser generator (provides tree-sitter)", "unzip": "Archive extraction",
	}
	var result []packageSpec
	for _, name := range formulae {
		result = append(result, packageSpec{name: name, description: descriptions[name]})
	}
	return append(result,
		packageSpec{name: font, description: "JetBrains Mono Nerd Font (alias: font)", cask: true},
		packageSpec{name: "ghostty", description: "Ghostty terminal + Luka's bundled configuration", cask: true},
		packageSpec{name: ghosttyFont, description: "Meslo LG Nerd Font (Ghostty fallback)", cask: true},
	)
}

// PackageName validates a catalog entry and resolves its supported aliases.
func PackageName(name string) (string, error) {
	p, err := lookupPackage(name)
	return p.name, err
}

func lookupPackage(name string) (packageSpec, error) {
	switch name {
	case "nvim":
		name = "neovim"
	case "font":
		name = font
	}
	for _, p := range packageCatalog() {
		if p.name == name {
			return p, nil
		}
	}
	return packageSpec{}, fmt.Errorf("unknown coding package %q; run mak setup dev --list for available names", name)
}

// Homebrew aliases such as python resolve to versioned formula names. Use
// Homebrew's metadata rather than guessing a version from installed directories.
func (i *installer) catalogNames(ctx context.Context) (map[string]string, error) {
	b, err := i.run(ctx, i.env, i.brew, append([]string{"info", "--json=v2", "--formula"}, formulae...)...)
	if err != nil {
		return nil, err
	}
	var info struct {
		Formulae []struct {
			Name     string   `json:"name"`
			Aliases  []string `json:"aliases"`
			OldNames []string `json:"oldnames"`
		} `json:"formulae"`
	}
	if err := json.Unmarshal([]byte(b), &info); err != nil {
		return nil, fmt.Errorf("reading Homebrew package names: %w", err)
	}
	names := map[string]string{}
	for _, name := range casks {
		names[name] = name
	}
	for _, name := range formulae {
		for _, f := range info.Formulae {
			if f.Name == name || slices.Contains(f.Aliases, name) || slices.Contains(f.OldNames, name) {
				names[name] = f.Name
				break
			}
		}
		if names[name] == "" {
			return nil, fmt.Errorf("Homebrew did not resolve %s", name)
		}
	}
	return names, nil
}

func packageInstaller(in io.Reader, out io.Writer) (*installer, error) {
	if runtime.GOOS != "darwin" {
		return nil, fmt.Errorf("mak setup dev currently supports macOS only")
	}
	if os.Geteuid() == 0 {
		return nil, fmt.Errorf("run mak setup dev as your normal user, without sudo")
	}
	home, err := os.UserHomeDir()
	if err != nil {
		return nil, err
	}
	i := &installer{home: home, env: os.Environ(), out: out}
	i.run = runner(in, out, func() *os.File { return i.lockFile })
	return i, nil
}

// ListPackages checks Homebrew without installing anything or acquiring a write lock.
func ListPackages(ctx context.Context, in io.Reader, out io.Writer) error {
	i, err := packageInstaller(in, out)
	if err != nil {
		return err
	}
	return i.listPackages(ctx)
}

func (i *installer) listPackages(ctx context.Context) error {
	if err := i.resolvePaths(); err != nil {
		return err
	}
	r, err := i.readRecord()
	if err != nil && !os.IsNotExist(err) {
		return err
	}
	installedFormulae, installedCasks := map[string]bool{}, map[string]bool{}
	names := map[string]string{}
	if i.findBrew() {
		i.env = setEnv(i.env, "HOMEBREW_NO_AUTO_UPDATE", "1")
		if installedFormulae, err = i.inventory(ctx, "--formula"); err != nil {
			return err
		}
		if installedCasks, err = i.inventory(ctx, "--cask"); err != nil {
			return err
		}
		if names, err = i.catalogNames(ctx); err != nil {
			return err
		}
	} else {
		fmt.Fprintln(i.out, "Homebrew is not installed; none of these Homebrew packages are installed.")
	}
	ui.Heading(i.out, "Available coding software (Homebrew status)")
	fmt.Fprintln(i.out, "[x] installed  [ ] not installed; ownership is shown separately.")
	for _, p := range packageCatalog() {
		name := names[p.name]
		if r != nil && r.Brew == i.brew && r.Packages[p.name] != "" {
			name = r.Packages[p.name]
		}
		installed := installedFormulae[name]
		owned := r != nil && r.Brew == i.brew && slices.Contains(r.Formulae, name)
		if p.cask {
			installed = installedCasks[name]
			owned = r != nil && r.Brew == i.brew && slices.Contains(r.Casks, name)
		}
		status := ""
		if name != "" && name != p.name {
			status = " (" + name + ")"
		}
		if owned {
			status += " (tracked by mak)"
		} else if installed {
			status += " (not tracked by mak)"
		}
		ui.Check(i.out, installed, "%-34s %s%s", p.name, p.description, status)
	}
	fmt.Fprintln(i.out, "\nNeovim's checkbox shows the application, not whether the full LazyVim setup is complete.")
	fmt.Fprintln(i.out, "--install neovim (or nvim) installs the full environment. Ghostty includes its configuration and Meslo font; other names install only that package and its dependencies.")
	fmt.Fprintln(i.out, "Homebrew and Apple developer tools are shared prerequisites. Software installed outside Homebrew is not detected.")
	return nil
}

// InstallPackage installs a catalog package; Neovim selects the complete setup.
func InstallPackage(ctx context.Context, in io.Reader, out io.Writer, name string) error {
	p, err := lookupPackage(name)
	if err != nil {
		return err
	}
	if p.name == "neovim" {
		return Setup(ctx, in, out)
	}
	i, err := packageInstaller(in, out)
	if err != nil {
		return err
	}
	return i.installPackage(ctx, p)
}

func (i *installer) installPackage(ctx context.Context, p packageSpec) (err error) {
	if p.name == "neovim" {
		return i.setup(ctx)
	}
	unlock, err := i.prepare(ctx)
	if err != nil {
		return err
	}
	defer unlock()
	r, err := i.readRecord()
	if err != nil && !os.IsNotExist(err) {
		return err
	}
	if err := checkPendingRemoval(r, ""); err != nil {
		return err
	}
	if p.name == "ghostty" && r != nil {
		if err := i.checkPaths(r); err != nil {
			return err
		}
	}
	ctx, cancel := context.WithTimeout(ctx, 60*time.Minute)
	defer cancel()
	defer func() {
		if err != nil {
			cleanupCtx, stop := context.WithTimeout(context.Background(), 10*time.Minute)
			defer stop()
			if cleanupErr := i.rollback(cleanupCtx); cleanupErr != nil {
				err = errors.Join(err, fmt.Errorf("package rollback needs attention: %w", cleanupErr))
			}
		}
	}()
	if err := i.ensureBrew(ctx); err != nil {
		return err
	}
	if r != nil && r.Brew != i.brew {
		return fmt.Errorf("Homebrew location changed; restore %s before installing more packages", r.Brew)
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
	names, err := i.catalogNames(ctx)
	if err != nil {
		return err
	}
	name := names[p.name]
	before, kind := i.beforeFormulae, "--formula"
	if p.cask {
		before, kind = i.beforeCasks, "--cask"
	}
	if before[name] && p.name != "ghostty" {
		ui.Success(i.out, "%s is already installed; leaving it as is.", p.name)
		return nil
	}
	if err := i.ensureCompiler(ctx); err != nil {
		return err
	}
	var paths []replacement
	if p.name == "ghostty" {
		paths = i.paths[4:]
	}
	if err := i.beginSetupJournalPaths(paths); err != nil {
		return err
	}
	ui.Step(i.out, "Installing %s and its dependencies...", p.name)
	args := []string{"install", kind, name}
	if p.name == "ghostty" {
		args = append(args, ghosttyFont)
	}
	if _, err := i.run(ctx, i.env, i.brew, args...); err != nil {
		return fmt.Errorf("installing %s: %w", p.name, err)
	}
	installed, err := i.inventory(ctx, kind)
	if err != nil {
		return err
	}
	if !installed[name] {
		return fmt.Errorf("Homebrew did not install %s", name)
	}
	if p.name == "ghostty" {
		for n := 4; n < len(i.paths); n++ {
			if err := i.replace(&i.paths[n]); err != nil {
				return err
			}
		}
		if err := i.writeGhosttyConfig(); err != nil {
			return err
		}
	}
	prefix, err := i.run(ctx, i.env, i.brew, "--prefix")
	if err != nil {
		return err
	}
	prefix = strings.TrimSpace(prefix)
	if !filepath.IsAbs(prefix) || strings.ContainsAny(prefix, "\r\n") {
		return fmt.Errorf("invalid Homebrew prefix %q", prefix)
	}
	if err := i.configureShell(prefix); err != nil {
		return err
	}
	if r == nil {
		r, err = i.newRecord()
		if err != nil {
			return err
		}
	}
	if err := i.recordInstallation(ctx, r); err != nil {
		return fmt.Errorf("saving package ownership: %w", err)
	}
	ui.Success(i.out, "%s installed. Open a new terminal to refresh your PATH.", p.name)
	return nil
}

func checkPendingRemoval(r *installationRecord, name string) error {
	if r == nil {
		return nil
	}
	if r.Uninstalling {
		return fmt.Errorf("finish the previous uninstall with mak setup dev --uninstall first")
	}
	if r.Removing != "" && r.Removing != name {
		return fmt.Errorf("finish the previous removal with mak setup dev --remove %s first", r.Removing)
	}
	return nil
}

// RemovePackage removes only the selected package owned by mak, keeping its dependencies.
func RemovePackage(ctx context.Context, in io.Reader, out io.Writer, name string) error {
	p, err := lookupPackage(name)
	if err != nil {
		return err
	}
	i, err := packageInstaller(in, out)
	if err != nil {
		return err
	}
	return i.removePackage(ctx, p)
}

func (i *installer) removePackage(ctx context.Context, p packageSpec) error {
	unlock, err := i.prepare(ctx)
	if err != nil {
		return err
	}
	defer unlock()
	r, err := i.readRecord()
	if os.IsNotExist(err) {
		if i.recoveredSetup {
			ui.Success(i.out, "Interrupted setup recovered; no tracked packages remain to remove. Preexisting software is preserved.")
			return nil
		}
		return fmt.Errorf("%s is not tracked by mak; no packages or Neovim files were changed", p.name)
	}
	if err != nil {
		return err
	}
	if err := checkPendingRemoval(r, p.name); err != nil {
		return err
	}
	owned, kind := &r.Formulae, "--formula"
	if p.cask {
		owned, kind = &r.Casks, "--cask"
	}
	i.brew = r.Brew
	i.env = setEnv(i.env, "HOMEBREW_NO_AUTO_UPDATE", "1")
	name := r.Packages[p.name]
	if name == "" {
		names, err := i.catalogNames(ctx)
		if err != nil {
			return err
		}
		name = names[p.name]
	}
	hasPaths := slices.ContainsFunc(r.Paths, func(path installedPath) bool { return i.pathPackage(path.Path) == p.name })
	if !slices.Contains(*owned, name) {
		if hasPaths {
			return fmt.Errorf("%s was already installed before mak; use mak setup dev --uninstall to restore its files while preserving the application", p.name)
		}
		return fmt.Errorf("%s is not tracked by mak; existing software is preserved", p.name)
	}
	ctx, cancel := context.WithTimeout(ctx, 30*time.Minute)
	defer cancel()
	current, err := i.inventory(ctx, kind)
	if err != nil {
		return err
	}
	if current[name] && !p.cask {
		dependents, err := i.run(ctx, i.env, i.brew, "uses", "--installed", "--recursive", name)
		if err != nil {
			return err
		}
		if names := strings.Fields(dependents); len(names) != 0 {
			return fmt.Errorf("keeping %s: required by %s; remove those packages first", p.name, strings.Join(names, ", "))
		}
	}
	if hasPaths {
		if p.name == "neovim" {
			if err := i.checkNeovim(ctx); err != nil {
				return err
			}
		}
		if err := i.checkPaths(r); err != nil {
			return err
		}
	}
	r.Removing = p.name
	if err := i.saveRecord(r); err != nil {
		return err
	}
	if hasPaths {
		ui.Warning(i.out, "Removing the managed %s files, including local edits; restoring original backups.", p.name)
		if err := i.restorePackagePaths(ctx, r, i.saveRecord, p.name); err != nil {
			return fmt.Errorf("%w; retry mak setup dev --remove %s", err, p.name)
		}
		r.Paths = slices.DeleteFunc(r.Paths, func(path installedPath) bool { return i.pathPackage(path.Path) == p.name })
		if err := i.saveRecord(r); err != nil {
			return err
		}
	}
	if current[name] {
		ui.Step(i.out, "Removing %s...", p.name)
		if err := i.removePackages(ctx, kind, []string{name}); err != nil {
			return fmt.Errorf("%w; record retained, retry mak setup dev --remove %s", err, p.name)
		}
	}
	*owned = slices.DeleteFunc(*owned, func(candidate string) bool { return candidate == name })
	delete(r.Packages, p.name)
	r.Removing = ""
	remaining := len(r.Formulae) != 0 || len(r.Casks) != 0 || len(r.Paths) != 0
	if !remaining {
		for _, s := range r.Shell {
			if err := removeShellChange(s); err != nil {
				return err
			}
		}
		if err := os.Remove(i.recordPath()); err != nil {
			return err
		}
	} else if err := i.saveRecord(r); err != nil {
		return err
	}
	ui.Success(i.out, "%s removed; other packages and dependencies retained.", p.name)
	if remaining {
		fmt.Fprintln(i.out, "Use mak setup dev --uninstall to remove the remaining tracked environment.")
	}
	return nil
}
