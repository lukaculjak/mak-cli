package devenv

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"strings"

	"github.com/lukaculjak/mak-cli/internal/ui"
	"golang.org/x/sys/unix"
)

// The journal is durable before packages, directory moves, or shell changes.
// Commit is written after verification, before changing the final ownership record.
type setupJournal struct {
	Record installationRecord  `json:"rollback"`
	Commit *installationRecord `json:"commit,omitempty"`
}

func acquireSetupLock(path string) (*os.File, error) {
	fd, err := unix.Open(path, unix.O_CREAT|unix.O_RDWR|unix.O_NOFOLLOW|unix.O_CLOEXEC, 0600)
	if err != nil {
		return nil, fmt.Errorf("cannot open setup lock %s (legacy lock directories require inspection before removal): %w", path, err)
	}
	f := os.NewFile(uintptr(fd), path)
	if st, err := f.Stat(); err != nil || !st.Mode().IsRegular() {
		f.Close()
		return nil, fmt.Errorf("setup lock %s must be a regular file", path)
	}
	if err := unix.Flock(fd, unix.LOCK_EX|unix.LOCK_NB); err != nil {
		f.Close()
		return nil, fmt.Errorf("cannot acquire setup lock %s (another setup or child process may be running): %w", path, err)
	}
	return f, nil
}

// Probe an existing lock without creating or modifying any files.
func setupLocked(path string) (bool, error) {
	st, err := os.Lstat(path)
	if os.IsNotExist(err) {
		return false, nil
	}
	if err != nil {
		return false, err
	}
	if st.IsDir() {
		return true, nil
	} // Legacy lock: never automatically remove it.
	if !st.Mode().IsRegular() {
		return false, fmt.Errorf("setup lock is not a regular file")
	}
	fd, err := unix.Open(path, unix.O_RDONLY|unix.O_NONBLOCK|unix.O_NOFOLLOW|unix.O_CLOEXEC, 0)
	if err != nil {
		return false, err
	}
	f := os.NewFile(uintptr(fd), path)
	defer f.Close()
	err = unix.Flock(int(f.Fd()), unix.LOCK_EX|unix.LOCK_NB)
	if errors.Is(err, unix.EWOULDBLOCK) {
		return true, nil
	}
	return false, err
}

func syncDirectory(path string) error {
	f, err := os.Open(path)
	if err != nil {
		return err
	}
	defer f.Close()
	return f.Sync()
}

// Sync the parent entries for newly created journal directories as well as the
// journal itself, so a fresh state directory survives an interrupted first setup.
func mkdirAllSynced(path string, mode os.FileMode) error {
	var created []string
	for parent := path; ; parent = filepath.Dir(parent) {
		if _, err := os.Stat(parent); err == nil {
			break
		} else if !os.IsNotExist(err) {
			return err
		}
		created = append(created, parent)
	}
	if err := os.MkdirAll(path, mode); err != nil {
		return err
	}
	for n := len(created) - 1; n >= 0; n-- {
		if err := syncDirectory(filepath.Dir(created[n])); err != nil {
			return err
		}
	}
	return nil
}

func renameDirectory(from, to string) error {
	if err := os.Rename(from, to); err != nil {
		return err
	}
	return syncDirectory(filepath.Dir(from))
}

func (i *installer) journalPath() string {
	return filepath.Join(envValue(i.env, "XDG_STATE_HOME"), "mak/dev-setup-journal.json")
}

func (i *installer) saveSetupJournal() error {
	path := i.journalPath()
	if err := mkdirAllSynced(filepath.Dir(path), 0700); err != nil {
		return err
	}
	b, err := json.MarshalIndent(i.journal, "", "  ")
	if err != nil {
		return err
	}
	return atomicWrite(path, append(b, '\n'), 0600)
}

func (i *installer) readSetupJournal() (*setupJournal, error) {
	path := i.journalPath()
	st, err := os.Lstat(path)
	if err != nil {
		return nil, err
	}
	if !st.Mode().IsRegular() {
		return nil, fmt.Errorf("setup journal must be a regular file")
	}
	b, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	var j setupJournal
	if err := json.Unmarshal(b, &j); err != nil {
		return nil, err
	}
	if err := validateRecord(&j.Record); err != nil {
		return nil, err
	}
	if j.Record.Uninstalling || j.Record.Removing != "" {
		return nil, fmt.Errorf("invalid setup rollback state")
	}
	for n, p := range j.Record.Paths {
		if p.Path != i.paths[n].path || (p.Backup != "" && p.Backup != p.Path+".mak-backup-"+j.Record.ID) {
			return nil, fmt.Errorf("setup journal paths differ; use the original XDG settings")
		}
	}
	if j.Commit != nil {
		if err := validateRecord(j.Commit); err != nil {
			return nil, err
		}
		if j.Commit.Brew != j.Record.Brew || j.Commit.Removing != "" || j.Commit.Uninstalling {
			return nil, fmt.Errorf("invalid setup commit")
		}
		if len(j.Record.Paths) != 0 && len(j.Commit.Paths) != 4 {
			return nil, fmt.Errorf("committed Neovim paths are missing")
		}
		for n, p := range j.Commit.Paths {
			if p.Path != i.paths[n].path || (p.Backup != "" && (filepath.Dir(p.Backup) != filepath.Dir(p.Path) || !strings.HasPrefix(filepath.Base(p.Backup), "nvim.mak-backup-"))) {
				return nil, fmt.Errorf("invalid committed setup paths")
			}
		}
	}
	return &j, nil
}

func (i *installer) beginSetupJournal(neovim bool) error {
	r, err := i.newRecord()
	if err != nil {
		return err
	}
	for name := range i.beforeFormulae {
		r.Formulae = append(r.Formulae, name)
	}
	for name := range i.beforeCasks {
		r.Casks = append(r.Casks, name)
	}
	slices.Sort(r.Formulae)
	slices.Sort(r.Casks)
	if neovim {
		for _, p := range i.paths {
			item := installedPath{Path: p.path}
			if _, err := os.Lstat(p.path); err == nil {
				item.Backup = p.path + ".mak-backup-" + r.ID
			} else if !os.IsNotExist(err) {
				return err
			}
			for _, reserved := range []string{item.Backup, discardPath(item, r.ID)} {
				if reserved == "" {
					continue
				}
				if _, err := os.Lstat(reserved); !os.IsNotExist(err) {
					return fmt.Errorf("reserved recovery path %s already exists or is unreadable", reserved)
				}
			}
			r.Paths = append(r.Paths, item)
		}
	}
	i.journal = &setupJournal{Record: *r}
	if err := i.saveSetupJournal(); err != nil {
		i.journal = nil
		return err
	}
	return nil
}

func (i *installer) replaceJournaled(p *replacement) error {
	var item *installedPath
	for n := range i.journal.Record.Paths {
		if i.journal.Record.Paths[n].Path == p.path {
			item = &i.journal.Record.Paths[n]
			break
		}
	}
	if item == nil {
		return fmt.Errorf("directory is absent from the setup journal")
	}
	_, err := os.Lstat(p.path)
	if (err == nil) != (item.Backup != "") || (err != nil && !os.IsNotExist(err)) {
		return fmt.Errorf("%s changed after setup planning; files left untouched", p.path)
	}
	if err := mkdirAllSynced(filepath.Dir(p.path), 0755); err != nil {
		return err
	}
	stage := discardPath(*item, i.journal.Record.ID)
	if err := os.Mkdir(stage, 0700); err != nil {
		return err
	}
	if err := atomicWrite(filepath.Join(stage, ownershipFile), []byte(i.journal.Record.ID), 0600); err != nil {
		return err
	}
	if item.Backup != "" {
		if err := renameDirectory(p.path, item.Backup); err != nil {
			return err
		}
		ui.Warning(i.out, "Replacing %s; backup: %s", p.path, item.Backup)
	}
	if err := renameDirectory(stage, p.path); err != nil {
		return err
	}
	p.backup = item.Backup
	return nil
}

func (i *installer) recoverPendingSetup(ctx context.Context) error {
	j, err := i.readSetupJournal()
	if os.IsNotExist(err) {
		return nil
	}
	if err != nil {
		return fmt.Errorf("cannot read setup recovery journal %s; files left untouched: %w", i.journalPath(), err)
	}
	i.journal = j
	if err := i.checkNeovim(ctx); err != nil {
		return err
	}
	ui.Warning(i.out, "Recovering an interrupted coding-environment installation...")
	if err := i.recoverSetupJournal(ctx); err != nil {
		return fmt.Errorf("setup recovery needs attention: %w; journal retained at %s", err, i.journalPath())
	}
	ui.Success(i.out, "Interrupted installation recovered.")
	i.recoveredSetup = true
	return nil
}

func (i *installer) recoverSetupJournal(ctx context.Context) error {
	j := i.journal
	if j.Commit != nil {
		return i.finishSetupCommit()
	}
	r := &j.Record
	// Validate every resource before deleting anything. An absent backup with an
	// existing original means its planned rename never happened.
	var unusedStages []string
	for n := range r.Paths {
		p := &r.Paths[n]
		if p.Done || p.Restoring {
			continue
		}
		_, liveErr := os.Lstat(p.Path)
		if liveErr != nil && !os.IsNotExist(liveErr) {
			return liveErr
		}
		backupExists := false
		if p.Backup != "" {
			_, err := os.Lstat(p.Backup)
			if err != nil && !os.IsNotExist(err) {
				return err
			}
			backupExists = err == nil
		}
		unchanged := (p.Backup != "" && !backupExists && liveErr == nil && !ownedDirectory(p.Path, r.ID)) || (p.Backup == "" && os.IsNotExist(liveErr))
		stage := discardPath(*p, r.ID)
		if st, err := os.Lstat(stage); err == nil {
			if !st.IsDir() {
				return fmt.Errorf("unexpected staging path %s; files left untouched", stage)
			}
			entries, err := os.ReadDir(stage)
			if err != nil {
				return err
			}
			if !ownedDirectory(stage, r.ID) && len(entries) != 0 {
				return fmt.Errorf("staging ownership missing at %s; files left untouched", stage)
			}
			if unchanged {
				unusedStages = append(unusedStages, stage)
			} else if os.IsNotExist(liveErr) && backupExists && ownedDirectory(stage, r.ID) {
				p.Restoring = true
			} else {
				return fmt.Errorf("unexpected staging directory %s; files left untouched", stage)
			}
		} else if !os.IsNotExist(err) {
			return err
		}
		if unchanged {
			p.Done = true
		}
	}
	if err := i.checkPaths(r); err != nil {
		return err
	}
	for _, stage := range unusedStages {
		if err := os.RemoveAll(stage); err != nil {
			return err
		}
	}
	if err := i.saveSetupJournal(); err != nil {
		return err
	}
	checkpoint := func(*installationRecord) error { return i.saveSetupJournal() }
	if err := i.restorePaths(ctx, r, checkpoint); err != nil {
		return err
	}
	for _, change := range r.Shell {
		if err := removeShellChange(change); err != nil {
			return err
		}
	}
	i.brew = r.Brew
	i.beforeFormulae = stringSet(r.Formulae)
	i.beforeCasks = stringSet(r.Casks)
	if err := i.rollbackPackages(ctx); err != nil {
		return err
	}
	return i.finishSetupJournal()
}

func stringSet(names []string) map[string]bool {
	result := map[string]bool{}
	for _, name := range names {
		result[name] = true
	}
	return result
}

func (i *installer) commitInstallationRecord(r *installationRecord) error {
	if i.journal == nil {
		return i.saveRecord(r)
	}
	i.journal.Commit = r
	if err := i.saveSetupJournal(); err != nil {
		i.journal.Commit = nil
		return err
	}
	return i.finishSetupCommit()
}

func (i *installer) finishSetupCommit() error {
	r := i.journal.Commit
	if len(i.journal.Record.Paths) != 0 {
		for _, p := range i.journal.Record.Paths {
			if !ownedDirectory(p.Path, i.journal.Record.ID) && !ownedDirectory(p.Path, r.ID) {
				return fmt.Errorf("%s changed before committing setup; files left untouched", p.Path)
			}
		}
		for _, p := range i.journal.Record.Paths {
			if err := atomicWrite(filepath.Join(p.Path, ownershipFile), []byte(r.ID), 0600); err != nil {
				return err
			}
		}
		if err := i.checkPaths(r); err != nil {
			return err
		}
	}
	if err := i.saveRecord(r); err != nil {
		return err
	}
	return i.finishSetupJournal()
}

func (i *installer) finishSetupJournal() error {
	if err := os.Remove(i.journalPath()); err != nil {
		return err
	}
	if err := syncDirectory(filepath.Dir(i.journalPath())); err != nil {
		return err
	}
	i.journal = nil
	i.beforeFormulae = nil
	i.beforeCasks = nil
	i.shell = nil
	return nil
}
