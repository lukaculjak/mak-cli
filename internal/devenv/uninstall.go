package devenv

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"regexp"
	"runtime"
	"slices"
	"strings"
	"time"

	"github.com/lukaculjak/mak-cli/internal/ui"
)

const ownershipFile = ".mak-install-id"

type installationRecord struct {
	Version      int               `json:"version"`
	ID           string            `json:"id"`
	Brew         string            `json:"brew"`
	Paths        []installedPath   `json:"paths"`
	Formulae     []string          `json:"formulae"`
	Casks        []string          `json:"casks"`
	Packages     map[string]string `json:"packages,omitempty"`
	Shell        []shellChange     `json:"shell,omitempty"`
	Uninstalling bool              `json:"uninstalling,omitempty"`
	Removing     string            `json:"removing,omitempty"`
}

type installedPath struct {
	Path      string `json:"path"`
	Backup    string `json:"backup,omitempty"`
	Restoring bool   `json:"restoring,omitempty"`
	Done      bool   `json:"done,omitempty"`
}

type shellChange struct {
	Path    string `json:"path"`
	Block   string `json:"block"`
	Created bool   `json:"created,omitempty"`
}

func (i *installer) recordPath() string {
	// Keep the recovery record outside both Neovim and mak uninstall --purge.
	return filepath.Join(envValue(i.env, "XDG_STATE_HOME"), "mak", "dev-environment.json")
}

func (i *installer) readRecord() (*installationRecord, error) {
	path := i.recordPath()
	st, err := os.Lstat(path)
	if err != nil {
		return nil, err
	}
	if !st.Mode().IsRegular() {
		return nil, fmt.Errorf("installation record %s must be a regular file", path)
	}
	b, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	var r installationRecord
	if err := json.Unmarshal(b, &r); err != nil {
		return nil, fmt.Errorf("invalid installation record %s: %w", path, err)
	}
	if err := validateRecord(&r); err != nil {
		return nil, fmt.Errorf("invalid installation record %s: %w", path, err)
	}
	return &r, nil
}

func validateRecord(r *installationRecord) error {
	id, err := hex.DecodeString(r.ID)
	if r.Version != 1 || err != nil || len(id) != 16 || !filepath.IsAbs(r.Brew) || (len(r.Paths) != 0 && len(r.Paths) != 4) {
		return fmt.Errorf("invalid or unsupported installation record; files left untouched")
	}
	if r.Removing != "" {
		if _, err := PackageName(r.Removing); err != nil {
			return fmt.Errorf("invalid package removal in installation record: %w", err)
		}
	}
	packageName := regexp.MustCompile(`^[a-zA-Z0-9][a-zA-Z0-9@+._/\-]*$`)
	for _, name := range append(slices.Clone(r.Formulae), r.Casks...) {
		if !packageName.MatchString(name) {
			return fmt.Errorf("invalid package in installation record: %q", name)
		}
	}
	for alias, name := range r.Packages {
		canonical, err := PackageName(alias)
		if err != nil || canonical != alias || (!slices.Contains(r.Formulae, name) && !slices.Contains(r.Casks, name)) {
			return fmt.Errorf("invalid package mapping in installation record")
		}
	}
	for _, s := range r.Shell {
		if !filepath.IsAbs(s.Path) || !strings.HasPrefix(s.Block, "\n# Homebrew coding tools (mak setup dev)\neval ") || !strings.HasSuffix(s.Block, " shellenv)\"\n") {
			return fmt.Errorf("invalid shell change in installation record")
		}
	}
	return nil
}

func (i *installer) saveRecord(r *installationRecord) error {
	b, err := json.MarshalIndent(r, "", "  ")
	if err != nil {
		return err
	}
	path := i.recordPath()
	if err := mkdirAllSynced(filepath.Dir(path), 0o700); err != nil {
		return err
	}
	return atomicWrite(path, append(b, '\n'), 0o600)
}

func atomicWrite(path string, contents []byte, mode os.FileMode) error {
	f, err := os.CreateTemp(filepath.Dir(path), ".mak-write-*")
	if err != nil {
		return err
	}
	defer os.Remove(f.Name())
	defer f.Close()
	if err := f.Chmod(mode); err != nil {
		return err
	}
	if _, err := f.Write(contents); err != nil {
		return err
	}
	if err := f.Sync(); err != nil {
		return err
	}
	if err := f.Close(); err != nil {
		return err
	}
	if err := os.Rename(f.Name(), path); err != nil {
		return err
	}
	return syncDirectory(filepath.Dir(path))
}

func (i *installer) recordInstallation(ctx context.Context, r *installationRecord) error {
	if r == nil {
		var err error
		r, err = i.newRecord()
		if err != nil {
			return err
		}
	}
	if len(r.Paths) == 0 {
		for _, p := range i.paths {
			r.Paths = append(r.Paths, installedPath{Path: p.path, Backup: p.backup})
		}
	}
	if i.journal != nil {
		return i.recordPackages(ctx, r)
	}
	for _, p := range i.paths {
		if err := os.WriteFile(filepath.Join(p.path, ownershipFile), []byte(r.ID), 0o600); err != nil {
			return err
		}
	}
	return i.recordPackages(ctx, r)
}

func (i *installer) newRecord() (*installationRecord, error) {
	id := make([]byte, 16)
	if _, err := rand.Read(id); err != nil {
		return nil, err
	}
	return &installationRecord{Version: 1, ID: hex.EncodeToString(id), Brew: i.brew}, nil
}

func (i *installer) recordPackages(ctx context.Context, r *installationRecord) error {
	for _, packages := range []struct {
		kind   string
		before map[string]bool
		owned  *[]string
	}{
		{"--formula", i.beforeFormulae, &r.Formulae}, {"--cask", i.beforeCasks, &r.Casks},
	} {
		current, err := i.inventory(ctx, packages.kind)
		if err != nil {
			return err
		}
		for name := range current {
			if !packages.before[name] && !slices.Contains(*packages.owned, name) {
				*packages.owned = append(*packages.owned, name)
			}
		}
		slices.Sort(*packages.owned)
	}
	names, err := i.catalogNames(ctx)
	if err != nil {
		return err
	}
	if r.Packages == nil {
		r.Packages = map[string]string{}
	}
	for alias, name := range names {
		if slices.Contains(r.Formulae, name) || slices.Contains(r.Casks, name) {
			r.Packages[alias] = name
		}
	}
	if i.shell != nil {
		r.Shell = append(r.Shell, *i.shell)
	}
	return i.commitInstallationRecord(r)
}

func ownedDirectory(path, id string) bool {
	st, err := os.Lstat(path)
	if err != nil || !st.IsDir() {
		return false
	}
	b, err := os.ReadFile(filepath.Join(path, ownershipFile))
	return err == nil && string(b) == id
}

// Validate every resource before changing any of them. A missing marker means
// the user replaced the directory, so it is no longer ours to delete.
func (i *installer) checkPaths(r *installationRecord) error {
	for n := range r.Paths {
		p := &r.Paths[n]
		if p.Path != i.paths[n].path {
			return fmt.Errorf("XDG directories differ from the installation record; use the same XDG settings as setup")
		}
		if p.Backup != "" && (filepath.Dir(p.Backup) != filepath.Dir(p.Path) || !strings.HasPrefix(filepath.Base(p.Backup), "nvim.mak-backup-")) {
			return fmt.Errorf("invalid backup path %s; files left untouched", p.Backup)
		}
		if p.Done {
			continue
		}
		if _, err := os.Lstat(discardPath(*p, r.ID)); err == nil && !p.Restoring {
			return fmt.Errorf("uninstall staging path already exists for %s; files left untouched", p.Path)
		} else if err != nil && !os.IsNotExist(err) {
			return err
		}
		if p.Backup != "" {
			if _, err := os.Lstat(p.Backup); err != nil {
				// Rename succeeded but the final checkpoint may have failed.
				if os.IsNotExist(err) && p.Restoring && !ownedDirectory(p.Path, r.ID) {
					if _, err := os.Lstat(p.Path); err == nil {
						p.Done = true
						continue
					}
				}
				return fmt.Errorf("backup %s is unavailable; files left untouched: %w", p.Backup, err)
			}
		}
		if _, err := os.Lstat(p.Path); os.IsNotExist(err) {
			continue
		} else if err != nil {
			return err
		}
		if !ownedDirectory(p.Path, r.ID) {
			return fmt.Errorf("%s no longer has mak's ownership marker; files left untouched", p.Path)
		}
	}
	return nil
}

// Uninstall reverses a successful, tracked setup without removing preexisting packages.
func Uninstall(ctx context.Context, in io.Reader, out io.Writer) error {
	if runtime.GOOS != "darwin" || os.Geteuid() == 0 {
		return fmt.Errorf("run mak setup dev --uninstall as your normal macOS user, without sudo")
	}
	home, err := os.UserHomeDir()
	if err != nil {
		return err
	}
	i := &installer{home: home, env: os.Environ(), out: out}
	i.run = runner(in, out, func() *os.File { return i.lockFile })
	return i.uninstall(ctx)
}

func (i *installer) uninstall(ctx context.Context) error {
	unlock, err := i.prepare(ctx)
	if err != nil {
		return err
	}
	defer unlock()
	r, err := i.readRecord()
	if os.IsNotExist(err) {
		if i.recoveredSetup {
			ui.Success(i.out, "Interrupted setup recovered; no tracked environment remains to uninstall. Preexisting software is preserved.")
			return nil
		}
		return fmt.Errorf("no tracked coding environment at %s; older setups cannot be safely uninstalled automatically. No Neovim files or packages were changed", i.recordPath())
	}
	if err != nil {
		return err
	}
	if len(r.Paths) != 0 {
		if err := i.checkNeovim(ctx); err != nil {
			return err
		}
	}
	if err := i.checkPaths(r); err != nil {
		return err
	}
	ctx, cancel := context.WithTimeout(ctx, 30*time.Minute)
	defer cancel()
	i.brew = r.Brew
	i.env = setEnv(i.env, "HOMEBREW_NO_AUTO_UPDATE", "1")
	i.env = setEnv(i.env, "HOMEBREW_NO_AUTOREMOVE", "1")
	removeFormulae, removeCasks, err := i.removablePackages(ctx, r)
	if err != nil {
		return err
	}
	r.Uninstalling = true
	r.Removing = ""
	if err := i.saveRecord(r); err != nil {
		return err
	}
	if len(r.Paths) != 0 {
		ui.Warning(i.out, "Removing mak's Neovim environment, including local edits, plugins and language servers; restoring original backups.")
	} else {
		ui.Step(i.out, "Removing the remaining tracked coding packages.")
	}
	if err := i.restorePaths(ctx, r, i.saveRecord); err != nil {
		return err
	}
	for _, s := range r.Shell {
		if err := removeShellChange(s); err != nil {
			return fmt.Errorf("restoring shell profile: %w; retry mak setup dev --uninstall", err)
		}
	}
	for _, packages := range []struct {
		kind  string
		names []string
	}{
		{"--cask", removeCasks}, {"--formula", removeFormulae},
	} {
		if len(packages.names) == 0 {
			continue
		}
		ui.Step(i.out, "Removing Homebrew packages: %s", strings.Join(packages.names, ", "))
		if err := i.removePackages(ctx, packages.kind, packages.names); err != nil {
			return fmt.Errorf("%w; installation record retained, retry mak setup dev --uninstall", err)
		}
	}
	if err := os.Remove(i.recordPath()); err != nil {
		return err
	}
	ui.Success(i.out, "Coding environment removed; previous Neovim files restored where backups existed.")
	fmt.Fprintln(i.out, "Homebrew, Apple developer tools, download caches and shared packages remain. Open a new terminal to refresh your PATH.")
	return nil
}

func (i *installer) restorePaths(ctx context.Context, r *installationRecord, checkpoint func(*installationRecord) error) error {
	for n := len(r.Paths) - 1; n >= 0; n-- {
		p := &r.Paths[n]
		if p.Done {
			continue
		}
		if err := ctx.Err(); err != nil {
			return err
		}
		p.Restoring = true
		if err := checkpoint(r); err != nil {
			return err
		}
		// Move the managed directory before deleting it. If deletion fails
		// halfway through (including removal of its ownership marker), retry
		// can continue on the recorded staging path without touching originals.
		discard := discardPath(*p, r.ID)
		if _, err := os.Lstat(p.Path); err == nil {
			if !ownedDirectory(p.Path, r.ID) {
				return fmt.Errorf("%s changed during uninstall; files left untouched", p.Path)
			}
			if _, err := os.Lstat(discard); err == nil {
				return fmt.Errorf("both %s and %s exist; inspect them before retrying", p.Path, discard)
			} else if !os.IsNotExist(err) {
				return err
			}
			if err := renameDirectory(p.Path, discard); err != nil {
				return fmt.Errorf("staging %s for removal: %w", p.Path, err)
			}
		} else if !os.IsNotExist(err) {
			return err
		}
		if err := os.RemoveAll(discard); err != nil {
			return fmt.Errorf("removing %s: %w; retry mak setup dev --uninstall", discard, err)
		}
		if p.Backup != "" {
			if err := renameDirectory(p.Backup, p.Path); err != nil {
				return fmt.Errorf("restoring %s from %s: %w; retry mak setup dev --uninstall", p.Path, p.Backup, err)
			}
			ui.Step(i.out, "Restored %s.", p.Path)
		}
		p.Done = true
		if err := checkpoint(r); err != nil {
			return err
		}
	}
	return nil
}

func (i *installer) removePackages(ctx context.Context, kind string, names []string) error {
	// Never force removal or ignore dependencies; also disable global autoremove.
	env := setEnv(i.env, "HOMEBREW_NO_AUTOREMOVE", "1")
	if _, err := i.run(ctx, env, i.brew, append([]string{"uninstall", kind}, names...)...); err != nil {
		return fmt.Errorf("removing coding tools: %w", err)
	}
	remaining, err := i.inventory(ctx, kind)
	if err != nil {
		return err
	}
	for _, name := range names {
		if remaining[name] {
			return fmt.Errorf("Homebrew still lists %s", name)
		}
	}
	return nil
}

func discardPath(p installedPath, id string) string {
	return p.Path + ".mak-remove-" + id
}

func (i *installer) removablePackages(ctx context.Context, r *installationRecord) ([]string, []string, error) {
	currentFormulae, err := i.inventory(ctx, "--formula")
	if err != nil {
		return nil, nil, err
	}
	currentCasks, err := i.inventory(ctx, "--cask")
	if err != nil {
		return nil, nil, err
	}
	owned := map[string]bool{}
	for _, name := range append(slices.Clone(r.Formulae), r.Casks...) {
		owned[filepath.Base(name)] = true
	}
	var formulae, casks []string
	for _, name := range r.Casks {
		if currentCasks[name] {
			casks = append(casks, name)
		}
	}
	for _, name := range r.Formulae {
		if !currentFormulae[name] {
			continue
		}
		dependents, err := i.run(ctx, i.env, i.brew, "uses", "--installed", "--recursive", name)
		if err != nil {
			return nil, nil, fmt.Errorf("checking dependents of %s: %w; no files changed", name, err)
		}
		shared := false
		for _, dependent := range strings.Fields(dependents) {
			if !owned[filepath.Base(dependent)] {
				ui.Warning(i.out, "Keeping %s: also needed by %s.", name, dependent)
				shared = true
				break
			}
		}
		if !shared {
			formulae = append(formulae, name)
		}
	}
	return formulae, casks, nil
}

func removeShellChange(s shellChange) error {
	b, err := os.ReadFile(s.Path)
	if os.IsNotExist(err) {
		return nil
	}
	if err != nil {
		return err
	}
	pos := strings.Index(string(b), s.Block)
	if pos < 0 {
		return nil
	}
	contents := append(slices.Clone(b[:pos]), b[pos+len(s.Block):]...)
	if s.Created && len(contents) == 0 {
		return os.Remove(s.Path)
	}
	st, err := os.Stat(s.Path)
	if err != nil {
		return err
	}
	return atomicWrite(s.Path, contents, st.Mode().Perm())
}
