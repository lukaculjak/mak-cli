package productivity

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"runtime"
	"slices"
	"syscall"
	"time"

	"golang.org/x/sys/unix"
)

const iCloudSetup = "Enable iCloud Drive in System Settings > Apple Account > iCloud > Drive > Sync this Mac"

func iCloudDir() (string, error) {
	if runtime.GOOS != "darwin" {
		return "", fmt.Errorf("Neorg iCloud sync currently supports macOS only")
	}
	home, err := os.UserHomeDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(home, "Library", "Mobile Documents", "com~apple~CloudDocs"), nil
}

func availableCloud(root string) error {
	st, err := os.Stat(root)
	if os.IsNotExist(err) {
		return fmt.Errorf("iCloud Drive is not available locally. %s", iCloudSetup)
	}
	if err != nil {
		return fmt.Errorf("accessing iCloud Drive: %w", err)
	}
	if !st.IsDir() {
		return fmt.Errorf("iCloud Drive path is not a directory: %s", root)
	}
	if err := unix.Access(root, unix.W_OK|unix.R_OK|unix.X_OK); err != nil {
		return fmt.Errorf("iCloud Drive is not readable and writable: %w", err)
	}
	return nil
}

type transfer struct {
	source   fileVersion
	target   string
	previous *fileVersion
	history  string
}

func planMerge(ctx context.Context, local, cloud map[string]fileVersion, localRoot, cloudRoot, kind string, backup bool) ([]transfer, error) {
	names := map[string]bool{}
	for name := range local {
		names[name] = true
	}
	for name := range cloud {
		names[name] = true
	}
	ordered := make([]string, 0, len(names))
	for name := range names {
		ordered = append(ordered, name)
	}
	slices.Sort(ordered)
	var plan []transfer
	for _, name := range ordered {
		l, hasLocal := local[name]
		c, hasCloud := cloud[name]
		var t transfer
		switch {
		case hasLocal && hasCloud:
			if l.hash == c.hash {
				continue
			}
			if backup && c.modified.After(l.modified) {
				continue
			}
			if l.modified.After(c.modified) || backup {
				t = transfer{source: l, target: c.path, previous: &c, history: filepath.Join("icloud", kind, name)}
			} else {
				t = transfer{source: c, target: l.path, previous: &l, history: filepath.Join("local", kind, name)}
			}
		case hasLocal:
			t = transfer{source: l, target: filepath.Join(cloudRoot, name)}
		case !backup:
			t = transfer{source: c, target: filepath.Join(localRoot, name)}
		default:
			continue
		}
		if err := checkDestination(ctx, t.target, t.previous); err != nil {
			return nil, err
		}
		plan = append(plan, t)
	}
	return plan, nil
}

func Sync(ctx context.Context, out io.Writer, backup bool) error {
	cloud, err := iCloudDir()
	if err != nil {
		return err
	}
	return syncNotes(ctx, out, backup, cloud, time.Now())
}

func syncNotes(ctx context.Context, out io.Writer, backup bool, cloud string, now time.Time) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	if err := availableCloud(cloud); err != nil {
		return err
	}
	notes, err := notesDir()
	if err != nil {
		return err
	}
	state, err := stateDir()
	if err != nil {
		return err
	}
	config, err := configDir()
	if err != nil {
		return err
	}
	plugin := filepath.Join(config, "nvim", "lua", "plugins", "neorg.lua")
	roots := []*string{&cloud, &notes, &state, &plugin}
	for _, path := range roots {
		resolved, err := canonicalPath(*path)
		if err != nil {
			return err
		}
		*path = resolved
	}
	mak := filepath.Join(cloud, ".MAK")
	if overlaps(notes, mak) || overlaps(state, mak) || overlaps(notes, state) || overlaps(plugin, mak) || overlaps(plugin, notes) || overlaps(plugin, state) {
		return fmt.Errorf("notes, Neorg config, iCloud .MAK and mak state must have separate paths")
	}
	// Never fabricate the iCloud root when the account is signed out. Reject a
	// redirected .MAK so all cloud copies stay in the actual iCloud Drive folder.
	if st, err := os.Lstat(mak); err == nil && !st.IsDir() {
		return fmt.Errorf("iCloud .MAK must be a directory, not a file or symlink")
	} else if err != nil && !os.IsNotExist(err) {
		return err
	} else if err == nil {
		if err := checkDownloaded(ctx, mak); err != nil {
			return err
		}
	}
	if err := os.MkdirAll(state, 0700); err != nil {
		return err
	}
	lock, err := os.OpenFile(filepath.Join(state, "sync.lock"), os.O_CREATE|os.O_RDWR, 0600)
	if err != nil {
		return err
	}
	defer lock.Close()
	if err := syscall.Flock(int(lock.Fd()), syscall.LOCK_EX|syscall.LOCK_NB); err != nil {
		return fmt.Errorf("another mak sync/backup is running: %w", err)
	}
	defer syscall.Flock(int(lock.Fd()), syscall.LOCK_UN)
	if st, err := os.Stat(notes); backup && (err != nil || !st.IsDir()) {
		return fmt.Errorf("no local notes directory at %s; run mak setup neorg first", notes)
	}
	cloudNotes := filepath.Join(mak, "neorg")
	cloudConfig := filepath.Join(mak, "neorg-config")
	fmt.Fprintln(out, "Checking notes and downloading any iCloud-only files…")
	localFiles, err := scanTree(ctx, notes)
	if err != nil {
		return err
	}
	cloudFiles, err := scanTree(ctx, cloudNotes)
	if err != nil {
		return err
	}
	localConfig := map[string]fileVersion{}
	if v, err := readVersion(ctx, plugin); err == nil {
		localConfig["neorg.lua"] = v
	} else if !os.IsNotExist(err) {
		return err
	}
	remoteConfig, err := scanTree(ctx, cloudConfig)
	if err != nil {
		return err
	}
	// The config folder has one managed file; other files are left untouched.
	for name := range remoteConfig {
		if name != "neorg.lua" {
			delete(remoteConfig, name)
		}
	}

	plan, err := planMerge(ctx, localFiles, cloudFiles, notes, cloudNotes, "notes", backup)
	if err != nil {
		return err
	}
	configPlan, err := planMerge(ctx, localConfig, remoteConfig, filepath.Dir(plugin), cloudConfig, "config", backup)
	if err != nil {
		return err
	}
	plan = append(plan, configPlan...)
	for _, dir := range []string{notes, cloudNotes, cloudConfig} {
		if err := checkParents(filepath.Join(dir, "placeholder")); err != nil {
			return err
		}
		if err := os.MkdirAll(dir, 0700); err != nil {
			return err
		}
	}
	stamp := now.UTC().Format("20060102T150405.000000000Z")
	if backup {
		snapshot, err := createSnapshot(ctx, mak, stamp, localFiles, localConfig, now)
		if err != nil {
			return err
		}
		fmt.Fprintf(out, "Local backup snapshot created in iCloud Drive: %s\n", snapshot)
	}
	for _, t := range plan {
		if t.previous != nil {
			for _, root := range []string{state, mak} {
				target := filepath.Join(root, "history", stamp, t.history)
				if err := copyVersion(ctx, *t.previous, target, nil); err != nil {
					return fmt.Errorf("preserving previous version: %w", err)
				}
			}
		}
		if err := copyVersion(ctx, t.source, t.target, t.previous); err != nil {
			return fmt.Errorf("Neorg transfer stopped; prior versions are retained in history: %w", err)
		}
	}
	if err := ctx.Err(); err != nil {
		return err
	}
	record, err := json.MarshalIndent(struct {
		Time      time.Time `json:"completed_locally_at"`
		Backup    bool      `json:"backup_only"`
		Directory string    `json:"icloud_directory"`
		Uploaded  bool      `json:"cloud_upload_confirmed"`
	}{now.UTC(), backup, mak, false}, "", "  ")
	if err != nil {
		return err
	}
	if err := os.WriteFile(filepath.Join(state, "last-transfer.json"), append(record, '\n'), 0600); err != nil {
		return err
	}
	fmt.Fprintf(out, "Neorg local transfer complete: %s\nmacOS handles iCloud uploads and downloads; cloud upload completion is not verified.\n", mak)
	return nil
}

func createSnapshot(ctx context.Context, mak, stamp string, notes, config map[string]fileVersion, now time.Time) (string, error) {
	root := filepath.Join(mak, "backups")
	if err := checkParents(filepath.Join(root, "placeholder")); err != nil {
		return "", err
	}
	if err := os.MkdirAll(root, 0700); err != nil {
		return "", err
	}
	target := filepath.Join(root, stamp)
	if _, err := os.Lstat(target); err == nil {
		return "", fmt.Errorf("backup snapshot already exists at %s; retry the command", target)
	} else if !os.IsNotExist(err) {
		return "", err
	}
	stage, err := os.MkdirTemp(root, ".mak-snapshot-")
	if err != nil {
		return "", err
	}
	defer os.RemoveAll(stage)
	for _, group := range []struct {
		dir   string
		files map[string]fileVersion
	}{{"neorg", notes}, {"neorg-config", config}} {
		if err := os.MkdirAll(filepath.Join(stage, group.dir), 0700); err != nil {
			return "", err
		}
		for name, v := range group.files {
			if err := copyVersion(ctx, v, filepath.Join(stage, group.dir, name), nil); err != nil {
				return "", err
			}
		}
	}
	manifest, err := json.Marshal(struct {
		Created time.Time `json:"created_at"`
	}{now.UTC()})
	if err != nil {
		return "", err
	}
	if err := os.WriteFile(filepath.Join(stage, "manifest.json"), append(manifest, '\n'), 0600); err != nil {
		return "", err
	}
	if err := ctx.Err(); err != nil {
		return "", err
	}
	if err := os.Rename(stage, target); err != nil {
		return "", err
	}
	return target, nil
}

func ICloudStatus(ctx context.Context, out io.Writer) {
	cloud, err := iCloudDir()
	if err == nil {
		err = ctx.Err()
	}
	if err != nil {
		fmt.Fprintf(out, "iCloud Drive: unavailable (%v).\n", err)
		return
	}
	iCloudStatus(out, cloud)
}

func iCloudStatus(out io.Writer, cloud string) {
	if err := availableCloud(cloud); err != nil {
		fmt.Fprintf(out, "iCloud Drive: unavailable (%v).\n", err)
		return
	}
	mak := filepath.Join(cloud, ".MAK")
	status := "available locally; .MAK will be created on first sync/backup"
	if st, err := os.Lstat(mak); err == nil {
		if !st.IsDir() {
			fmt.Fprintln(out, "iCloud Drive: .MAK must be a directory, not a file or symlink.")
			return
		}
		if err := unix.Access(mak, unix.W_OK|unix.R_OK|unix.X_OK); err != nil {
			fmt.Fprintf(out, "iCloud Drive: .MAK is not readable and writable (%v).\n", err)
			return
		}
		status = "available locally; .MAK is readable and writable"
	} else if !os.IsNotExist(err) {
		fmt.Fprintf(out, "iCloud Drive: cannot inspect .MAK (%v).\n", err)
		return
	}
	fmt.Fprintf(out, "iCloud Drive: %s (%s). Cloud upload completion is managed by macOS and is not verified.\n", status, mak)
}
