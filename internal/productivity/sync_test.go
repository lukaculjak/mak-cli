package productivity

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"io"
	"os"
	"path/filepath"
	"strings"
	"syscall"
	"testing"
	"time"
)

func cloudFixture(t *testing.T) string {
	t.Helper()
	cloud := filepath.Join(t.TempDir(), "iCloud Drive")
	if err := os.Mkdir(cloud, 0700); err != nil {
		t.Fatal(err)
	}
	return cloud
}

func TestICloudMergeRestoreHistoryAndBackupSnapshot(t *testing.T) {
	home := isolatedHome(t)
	cloud := cloudFixture(t)
	local := filepath.Join(home, "neorg")
	mak := filepath.Join(cloud, ".MAK")
	remote := filepath.Join(mak, "neorg")
	config, _ := configDir()
	plugin := filepath.Join(config, "nvim", "lua", "plugins", "neorg.lua")
	when := time.Date(2026, 10, 23, 9, 0, 0, 0, time.UTC)
	put(t, filepath.Join(cloud, "unrelated.txt"), "untouched", when)
	put(t, filepath.Join(mak, "existing.txt"), "keep existing .MAK", when)
	put(t, filepath.Join(local, "local.norg"), "local", when)
	put(t, filepath.Join(remote, "remote.norg"), "remote", when)
	put(t, filepath.Join(local, "local-newer.norg"), "local wins", when.Add(time.Hour))
	put(t, filepath.Join(remote, "local-newer.norg"), "remote old", when)
	put(t, filepath.Join(local, "remote-newer.norg"), "local old", when)
	put(t, filepath.Join(remote, "remote-newer.norg"), "remote wins", when.Add(time.Hour))
	put(t, filepath.Join(local, "equal-time.norg"), "aaaa", when)
	put(t, filepath.Join(remote, "equal-time.norg"), "bbbb", when)
	put(t, filepath.Join(local, ".hidden", "attachment.bin"), "\x00\x01extra data", when)
	put(t, plugin, "old config", when)
	put(t, filepath.Join(config, "mak", "rclone.conf"), "private legacy credentials", when)
	put(t, filepath.Join(mak, "neorg-config", "neorg.lua"), "new config", when.Add(time.Hour))
	var out bytes.Buffer
	if err := syncNotes(context.Background(), &out, false, cloud, when); err != nil {
		t.Fatal(err)
	}
	for _, dir := range []string{local, remote} {
		for name, content := range map[string]string{"local.norg": "local", "remote.norg": "remote", "local-newer.norg": "local wins", "remote-newer.norg": "remote wins", "equal-time.norg": "bbbb", ".hidden/attachment.bin": "\x00\x01extra data"} {
			assertFile(t, filepath.Join(dir, name), content)
		}
	}
	assertFile(t, plugin, "new config")
	stamp := when.Format("20060102T150405.000000000Z")
	for _, root := range []string{mak, filepath.Join(home, ".state", "mak", "neorg")} {
		history := filepath.Join(root, "history", stamp)
		assertFile(t, filepath.Join(history, "icloud", "notes", "local-newer.norg"), "remote old")
		assertFile(t, filepath.Join(history, "local", "notes", "remote-newer.norg"), "local old")
		assertFile(t, filepath.Join(history, "local", "notes", "equal-time.norg"), "aaaa")
		assertFile(t, filepath.Join(history, "local", "config", "neorg.lua"), "old config")
	}
	assertFile(t, filepath.Join(cloud, "unrelated.txt"), "untouched")
	assertFile(t, filepath.Join(mak, "existing.txt"), "keep existing .MAK")
	if !strings.Contains(out.String(), "cloud upload completion is not verified") {
		t.Fatal("transfer claimed a confirmed upload:", out.String())
	}
	// Backup saves a complete local snapshot without pulling cloud-only notes or
	// overwriting a newer cloud file. Even a stale local version has a snapshot.
	put(t, filepath.Join(remote, "cloud-only.norg"), "cloud only", when)
	put(t, filepath.Join(local, "remote-newer.norg"), "stale local", when)
	put(t, filepath.Join(local, "new-local.norg"), "new local", when.Add(2*time.Hour))
	if err := syncNotes(context.Background(), io.Discard, true, cloud, when.Add(2*time.Hour)); err != nil {
		t.Fatal(err)
	}
	assertFile(t, filepath.Join(remote, "remote-newer.norg"), "remote wins")
	assertFile(t, filepath.Join(local, "remote-newer.norg"), "stale local")
	assertFile(t, filepath.Join(remote, "new-local.norg"), "new local")
	if _, err := os.Stat(filepath.Join(local, "cloud-only.norg")); !os.IsNotExist(err) {
		t.Fatal("backup pulled a file")
	}
	snapshot := filepath.Join(mak, "backups", when.Add(2*time.Hour).Format("20060102T150405.000000000Z"))
	assertFile(t, filepath.Join(snapshot, "neorg", "remote-newer.norg"), "stale local")
	assertFile(t, filepath.Join(snapshot, "neorg-config", "neorg.lua"), "new config")
	if _, err := os.Stat(filepath.Join(snapshot, "manifest.json")); err != nil {
		t.Fatal("snapshot did not finish:", err)
	}
	if _, err := os.Stat(filepath.Join(snapshot, "neorg", "cloud-only.norg")); !os.IsNotExist(err) {
		t.Fatal("snapshot included a cloud-only file")
	}
	// A new Mac needs no cloud credentials: its empty local tree restores from .MAK.
	newHome := isolatedHome(t)
	if err := syncNotes(context.Background(), io.Discard, false, cloud, when.Add(3*time.Hour)); err != nil {
		t.Fatal(err)
	}
	assertFile(t, filepath.Join(newHome, "neorg", "cloud-only.norg"), "cloud only")
	assertFile(t, filepath.Join(newHome, ".config", "nvim", "lua", "plugins", "neorg.lua"), "new config")
	// Deletions are retained on the other side and restored by the next sync.
	if err := os.Remove(filepath.Join(newHome, "neorg", "local.norg")); err != nil {
		t.Fatal(err)
	}
	if err := syncNotes(context.Background(), io.Discard, false, cloud, when.Add(4*time.Hour)); err != nil {
		t.Fatal(err)
	}
	assertFile(t, filepath.Join(newHome, "neorg", "local.norg"), "local")
	if _, err := os.Stat(filepath.Join(mak, "neorg-config", "rclone.conf")); !os.IsNotExist(err) {
		t.Fatal("credentials were copied")
	}
	state, _ := stateDir()
	record, err := os.ReadFile(filepath.Join(state, "last-transfer.json"))
	if err != nil {
		t.Fatal(err)
	}
	var decoded map[string]any
	if err := json.Unmarshal(record, &decoded); err != nil {
		t.Fatal(err)
	}
	canonicalMak, err := canonicalPath(mak)
	if err != nil {
		t.Fatal(err)
	}
	if decoded["cloud_upload_confirmed"] != false || decoded["icloud_directory"] != canonicalMak || decoded["completed_locally_at"] == nil {
		t.Fatalf("misleading transfer metadata: %s", record)
	}
}

func TestFirstBackupCreatesMAKAndRepeatedBackupsKeepSnapshots(t *testing.T) {
	home := isolatedHome(t)
	cloud := cloudFixture(t)
	when := time.Now().UTC()
	put(t, filepath.Join(home, "neorg", "note.norg"), "first", when)
	for day := 0; day < 2; day++ {
		if err := syncNotes(context.Background(), io.Discard, true, cloud, when.AddDate(0, 0, day)); err != nil {
			t.Fatal(err)
		}
		snapshot := filepath.Join(cloud, ".MAK", "backups", when.AddDate(0, 0, day).Format("20060102T150405.000000000Z"))
		assertFile(t, filepath.Join(snapshot, "neorg", "note.norg"), "first")
	}
	assertFile(t, filepath.Join(cloud, ".MAK", "neorg", "note.norg"), "first")
}

func TestUnavailableICloudDoesNotCreateFakeCloudRoot(t *testing.T) {
	home := isolatedHome(t)
	cloud := filepath.Join(home, "Library", "Mobile Documents", "com~apple~CloudDocs")
	err := syncNotes(context.Background(), io.Discard, false, cloud, time.Now())
	if err == nil || !strings.Contains(err.Error(), "System Settings") {
		t.Fatal(err)
	}
	if _, err := os.Stat(filepath.Join(home, "Library")); !os.IsNotExist(err) {
		t.Fatal("fabricated an iCloud root")
	}
	if _, err := os.Stat(filepath.Join(home, "neorg")); !os.IsNotExist(err) {
		t.Fatal("changed notes when iCloud was unavailable")
	}
}

func TestSyncLockCancellationAndHistoryFailure(t *testing.T) {
	home := isolatedHome(t)
	cloud := cloudFixture(t)
	state, _ := stateDir()
	if err := os.MkdirAll(state, 0700); err != nil {
		t.Fatal(err)
	}
	lock, err := os.OpenFile(filepath.Join(state, "sync.lock"), os.O_CREATE|os.O_RDWR, 0600)
	if err != nil {
		t.Fatal(err)
	}
	defer lock.Close()
	if err := syscall.Flock(int(lock.Fd()), syscall.LOCK_EX|syscall.LOCK_NB); err != nil {
		t.Fatal(err)
	}
	if err := syncNotes(context.Background(), io.Discard, false, cloud, time.Now()); err == nil || !strings.Contains(err.Error(), "another mak") {
		t.Fatal(err)
	}
	if err := syscall.Flock(int(lock.Fd()), syscall.LOCK_UN); err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if err := syncNotes(ctx, io.Discard, false, cloud, time.Now()); !errors.Is(err, context.Canceled) {
		t.Fatal(err)
	}
	when := time.Now().Add(-time.Hour)
	note := filepath.Join(home, "neorg", "note.norg")
	put(t, note, "local old", when)
	put(t, filepath.Join(cloud, ".MAK", "neorg", "note.norg"), "cloud new", when.Add(time.Hour))
	put(t, filepath.Join(cloud, ".MAK", "history"), "not a directory", when)
	if err := syncNotes(context.Background(), io.Discard, false, cloud, time.Now()); err == nil {
		t.Fatal("history failure ignored")
	}
	assertFile(t, note, "local old")
	if _, err := os.Stat(filepath.Join(state, "last-transfer.json")); !os.IsNotExist(err) {
		t.Fatal("failed transfer saved a success record")
	}
}

func TestSyncPreflightsPathConflictsAndPlaceholders(t *testing.T) {
	for _, problem := range []string{"placeholder", "config placeholder", "symlink", "file-directory collision", "MAK symlink", "overlapping notes", "overlapping state"} {
		t.Run(problem, func(t *testing.T) {
			home := isolatedHome(t)
			cloud := cloudFixture(t)
			when := time.Now()
			notes := filepath.Join(home, "neorg")
			put(t, filepath.Join(notes, "a.norg"), "local old", when)
			put(t, filepath.Join(cloud, ".MAK", "neorg", "a.norg"), "cloud new", when.Add(time.Hour))
			switch problem {
			case "placeholder":
				put(t, filepath.Join(cloud, ".MAK", "neorg", ".missing.norg.icloud"), "", when)
			case "config placeholder":
				put(t, filepath.Join(cloud, ".MAK", "neorg-config", ".neorg.lua.icloud"), "", when)
			case "symlink":
				if err := os.Symlink(filepath.Join(home, "outside"), filepath.Join(notes, "link")); err != nil {
					t.Fatal(err)
				}
			case "file-directory collision":
				put(t, filepath.Join(notes, "collision"), "file", when)
				put(t, filepath.Join(cloud, ".MAK", "neorg", "collision", "nested.norg"), "nested", when)
			case "MAK symlink":
				if err := os.RemoveAll(filepath.Join(cloud, ".MAK")); err != nil {
					t.Fatal(err)
				}
				if err := os.Symlink(t.TempDir(), filepath.Join(cloud, ".MAK")); err != nil {
					t.Fatal(err)
				}
			case "overlapping notes":
				if err := os.RemoveAll(notes); err != nil {
					t.Fatal(err)
				}
				if err := os.Symlink(filepath.Join(cloud, ".MAK", "neorg"), notes); err != nil {
					t.Fatal(err)
				}
			case "overlapping state":
				t.Setenv("XDG_STATE_HOME", notes)
			}
			if err := syncNotes(context.Background(), io.Discard, false, cloud, when); err == nil {
				t.Fatal("unsafe sync accepted")
			}
			if problem != "overlapping notes" {
				assertFile(t, filepath.Join(notes, "a.norg"), "local old")
			}
		})
	}
}

func TestCopyDetectsEditsAndKeepsTheDestination(t *testing.T) {
	home := isolatedHome(t)
	when := time.Now()
	source, target := filepath.Join(home, "source"), filepath.Join(home, "target")
	put(t, source, "aaaa", when)
	put(t, target, "bbbb", when)
	v, err := readVersion(context.Background(), source)
	if err != nil {
		t.Fatal(err)
	}
	previous, err := readVersion(context.Background(), target)
	if err != nil {
		t.Fatal(err)
	}
	put(t, source, "cccc", when)
	if err := copyVersion(context.Background(), v, target, &previous); err == nil {
		t.Fatal("source edit went undetected")
	}
	assertFile(t, target, "bbbb")
	v, err = readVersion(context.Background(), source)
	if err != nil {
		t.Fatal(err)
	}
	put(t, target, "dddd", when)
	if err := copyVersion(context.Background(), v, target, &previous); err == nil {
		t.Fatal("destination edit went undetected")
	}
	assertFile(t, target, "dddd")
}

func TestConfigSymlinkIsRetained(t *testing.T) {
	home := isolatedHome(t)
	cloud := cloudFixture(t)
	when := time.Now()
	actual := filepath.Join(home, "dotfiles", "neorg-custom.lua")
	plugin := filepath.Join(home, ".config", "nvim", "lua", "plugins", "neorg.lua")
	put(t, actual, "old", when)
	if err := os.MkdirAll(filepath.Dir(plugin), 0700); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(actual, plugin); err != nil {
		t.Fatal(err)
	}
	put(t, filepath.Join(cloud, ".MAK", "neorg-config", "neorg.lua"), "new", when.Add(time.Hour))
	if err := syncNotes(context.Background(), io.Discard, false, cloud, when); err != nil {
		t.Fatal(err)
	}
	st, err := os.Lstat(plugin)
	if err != nil || st.Mode()&os.ModeSymlink == 0 {
		t.Fatal("config symlink replaced:", err)
	}
	assertFile(t, actual, "new")
}

func TestICloudDoctorReportsLocalAvailabilityWithoutWriting(t *testing.T) {
	isolatedHome(t)
	for _, state := range []string{"missing", "available", "MAK exists", "MAK file"} {
		t.Run(state, func(t *testing.T) {
			root := filepath.Join(t.TempDir(), "cloud")
			want := "unavailable"
			if state != "missing" {
				if err := os.Mkdir(root, 0700); err != nil {
					t.Fatal(err)
				}
				want = "available locally"
			}
			if state == "MAK exists" {
				if err := os.Mkdir(filepath.Join(root, ".MAK"), 0700); err != nil {
					t.Fatal(err)
				}
				want = "readable and writable"
			}
			if state == "MAK file" {
				put(t, filepath.Join(root, ".MAK"), "keep", time.Now())
				want = "must be a directory"
			}
			var out bytes.Buffer
			iCloudStatus(&out, root)
			if !strings.Contains(out.String(), want) {
				t.Fatal(out.String())
			}
			if state == "available" {
				entries, err := os.ReadDir(root)
				if err != nil || len(entries) != 0 {
					t.Fatal("doctor wrote to iCloud")
				}
			}
			if state == "MAK file" {
				assertFile(t, filepath.Join(root, ".MAK"), "keep")
			}
		})
	}
}
