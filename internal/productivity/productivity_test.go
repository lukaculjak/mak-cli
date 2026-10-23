package productivity

import (
	"bytes"
	"context"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func isolatedHome(t *testing.T) string {
	t.Helper()
	home := t.TempDir()
	t.Setenv("HOME", home)
	t.Setenv("XDG_CONFIG_HOME", filepath.Join(home, ".config"))
	t.Setenv("XDG_STATE_HOME", filepath.Join(home, ".state"))
	return home
}

func put(t *testing.T, path, content string, when time.Time) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(content), 0600); err != nil {
		t.Fatal(err)
	}
	if err := os.Chtimes(path, when, when); err != nil {
		t.Fatal(err)
	}
}

func assertFile(t *testing.T, path, want string) {
	t.Helper()
	b, err := os.ReadFile(path)
	if err != nil || string(b) != want {
		t.Fatalf("%s = %q, %v; want %q", path, b, err, want)
	}
}

func TestJournalContent(t *testing.T) {
	for _, text := range []string{"", " \n", "@document.meta\ntitle: Journal\ncreated: today\n@end\n\n* "} {
		if HasJournalContent(text) {
			t.Fatalf("empty journal considered content: %q", text)
		}
	}
	for _, text := range []string{"* Today", "- ( ) Call someone", "Remember this", "@document.meta\ntitle: Journal\n@end\nA note"} {
		if !HasJournalContent(text) {
			t.Fatalf("content ignored: %q", text)
		}
	}
}

func TestCronIdempotencyPreservesOtherJobsAndQuotesPath(t *testing.T) {
	original := "MAILTO=user@example.com\n0 8 * * * existing-command\n# mak:meet:v1:meeting\n0 9 * * * mak meet open meeting\n# mak:meet:v1:meeting:end\n"
	next, err := cronContent(original, "/Applications/Mak's 100%/mak", false)
	if err != nil {
		t.Fatal(err)
	}
	again, err := cronContent(next, "/Applications/Mak's 100%/mak", false)
	if err != nil || again != next || !strings.HasPrefix(next, original) || !strings.Contains(next, "17 9 * * 1-5 '/Applications/Mak'\"'\"'s 100\\%/mak' neorg remind") {
		t.Fatalf("%s: %v", next, err)
	}
	removed, err := cronContent(next, "/bin/mak", true)
	if err != nil || removed != original {
		t.Fatalf("remove changed other jobs: %q %v", removed, err)
	}
	if _, err := cronContent(original+reminderStart+"\n", "/bin/mak", false); err == nil {
		t.Fatal("malformed cron block accepted")
	}
}

func TestTimersAndJournalNotifications(t *testing.T) {
	home := isolatedHome(t)
	bin := filepath.Join(home, "bin")
	if err := os.MkdirAll(bin, 0700); err != nil {
		t.Fatal(err)
	}
	t.Setenv("PATH", bin)
	log := filepath.Join(home, "calls")
	t.Setenv("MAK_TEST_LOG", log)
	for _, name := range []string{"timer", "terminal-notifier"} {
		if err := os.WriteFile(filepath.Join(bin, name), []byte("#!/bin/sh\nprintf '%s\\n' '"+name+"' \"$@\" >> \"$MAK_TEST_LOG\"\n"), 0700); err != nil {
			t.Fatal(err)
		}
	}
	for _, rest := range []bool{false, true} {
		if err := Timer(context.Background(), nil, io.Discard, rest); err != nil {
			t.Fatal(err)
		}
	}
	b, _ := os.ReadFile(log)
	if !strings.Contains(string(b), "timer\n50m\nterminal-notifier") || !strings.Contains(string(b), "timer\n10m\nterminal-notifier") || !strings.Contains(string(b), "Crystal") {
		t.Fatalf("wrong timer/notification: %s", b)
	}
	if err := os.Remove(log); err != nil {
		t.Fatal(err)
	}
	monday := time.Date(2026, 10, 26, 9, 17, 0, 0, time.Local)
	notes, _ := notesDir()
	put(t, filepath.Join(notes, "journal", monday.Format("2006/01/02")+".norg"), "@document.meta\ntitle: Empty\n@end", monday)
	if err := Remind(context.Background(), io.Discard, monday); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(log); !os.IsNotExist(err) {
		t.Fatal("empty journal notified")
	}
	put(t, filepath.Join(notes, "journal", monday.Format("2006-01-02")+".norg"), "- ( ) Review notes", monday)
	if err := Remind(context.Background(), io.Discard, monday); err != nil {
		t.Fatal(err)
	}
	b, _ = os.ReadFile(log)
	if strings.Count(string(b), "terminal-notifier") != 1 {
		t.Fatal(string(b))
	}
	weekend := monday.AddDate(0, 0, -1)
	put(t, filepath.Join(notes, "journal", weekend.Format("2006/01/02")+".norg"), "Weekend note", weekend)
	if err := Remind(context.Background(), io.Discard, weekend); err != nil {
		t.Fatal(err)
	}
	after, _ := os.ReadFile(log)
	if !bytes.Equal(b, after) {
		t.Fatal("weekend notified")
	}
	// A failed/cancelled timer must not notify.
	if err := os.WriteFile(filepath.Join(bin, "timer"), []byte("#!/bin/sh\nexit 1\n"), 0700); err != nil {
		t.Fatal(err)
	}
	if err := Timer(context.Background(), nil, io.Discard, false); err == nil {
		t.Fatal("timer error ignored")
	}
	after, _ = os.ReadFile(log)
	if !bytes.Equal(b, after) {
		t.Fatal("failed timer notified")
	}
}
