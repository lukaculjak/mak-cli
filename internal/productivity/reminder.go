package productivity

import (
	"context"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"
)

// HasJournalContent ignores Neorg's generated metadata and empty headings.
func HasJournalContent(content string) bool {
	metadata := false
	for _, line := range strings.Split(content, "\n") {
		line = strings.TrimSpace(line)
		if line == "@document.meta" {
			metadata = true
			continue
		}
		if metadata {
			if line == "@end" {
				metadata = false
			}
			continue
		}
		if line != "" && strings.Trim(line, "* ") != "" {
			return true
		}
	}
	return false
}

func Remind(ctx context.Context, out io.Writer, now time.Time) error {
	if now.Weekday() == time.Saturday || now.Weekday() == time.Sunday {
		return nil
	}
	root, err := notesDir()
	if err != nil {
		return err
	}
	for _, date := range []string{now.Format("2006/01/02"), now.Format("2006-01-02")} {
		path := filepath.Join(root, "journal", date+".norg")
		b, err := os.ReadFile(path)
		if os.IsNotExist(err) {
			continue
		}
		if err != nil {
			return err
		}
		if HasJournalContent(string(b)) {
			return Notify(ctx, "Neorg journal", "You have notes for today. Check your Neorg todo list.", out)
		}
	}
	return nil
}

const reminderStart = "# mak:neorg:reminder"
const reminderEnd = "# mak:neorg:reminder:end"

func cronContent(previous, executable string, remove bool) (string, error) {
	// Cron treats % specially even inside shell quotes; escape it below.
	if strings.ContainsAny(executable, "\r\n\x00") || !filepath.IsAbs(executable) {
		return "", fmt.Errorf("invalid mak executable path")
	}
	var lines []string
	skipping := false
	for _, line := range strings.Split(previous, "\n") {
		if line == reminderStart {
			if skipping {
				return "", fmt.Errorf("malformed Neorg cron block")
			}
			skipping = true
			continue
		}
		if line == reminderEnd {
			if !skipping {
				return "", fmt.Errorf("malformed Neorg cron block")
			}
			skipping = false
			continue
		}
		if !skipping {
			lines = append(lines, line)
		}
	}
	if skipping {
		return "", fmt.Errorf("unterminated Neorg cron block; repair crontab -e before retrying")
	}
	content := strings.TrimRight(strings.Join(lines, "\n"), "\n")
	if content != "" {
		content += "\n"
	}
	if !remove {
		quoted := "'" + strings.ReplaceAll(executable, "'", "'\"'\"'") + "'"
		quoted = strings.ReplaceAll(quoted, "%", "\\%")
		content += reminderStart + "\n17 9 * * 1-5 " + quoted + " neorg remind\n" + reminderEnd + "\n"
	}
	return content, nil
}

func ConfigureReminder(ctx context.Context, out io.Writer, remove bool) error {
	// Schedule the binary actually running this setup, rather than an older mak
	// that may also be present on PATH.
	binary, err := os.Executable()
	if err != nil {
		return err
	}
	binary, err = filepath.Abs(binary)
	if err != nil {
		return err
	}
	if !remove && strings.Contains(binary, "/go-build") {
		return fmt.Errorf("install the mak binary on PATH before configuring its reminder")
	}
	cmd := exec.CommandContext(ctx, "/usr/bin/crontab", "-l")
	b, err := cmd.Output()
	if err != nil {
		// Exit 1 can also be a permission error. Ignore only the explicit empty case.
		ee, ok := err.(*exec.ExitError)
		if !ok || ee.ExitCode() != 1 || !strings.Contains(strings.ToLower(string(ee.Stderr)), "no crontab") {
			return fmt.Errorf("reading crontab: %w", err)
		}
	}
	next, err := cronContent(string(b), binary, remove)
	if err != nil {
		return err
	}
	if next == string(b) {
		return nil
	}
	cmd = exec.CommandContext(ctx, "/usr/bin/crontab", "-")
	cmd.Stdin, cmd.Stdout, cmd.Stderr = strings.NewReader(next), out, out
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("updating Neorg reminder: %w", err)
	}
	if remove {
		fmt.Fprintln(out, "Neorg reminder removed.")
	} else {
		fmt.Fprintln(out, "Neorg reminder scheduled Monday–Friday at 09:17 in this machine's local timezone.")
	}
	return nil
}
