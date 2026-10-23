package productivity

import (
	"context"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
)

// Homebrew paths are explicit because cron does not load the login shell's PATH.
func tool(name string) (string, error) {
	if path, err := exec.LookPath(name); err == nil {
		return path, nil
	}
	for _, prefix := range []string{"/opt/homebrew/bin", "/usr/local/bin"} {
		path := filepath.Join(prefix, name)
		if st, err := os.Stat(path); err == nil && st.Mode().IsRegular() && st.Mode().Perm()&0111 != 0 {
			return path, nil
		}
	}
	return "", fmt.Errorf("%s is unavailable; run mak setup neorg or mak setup pomodoro", name)
}

func run(ctx context.Context, in io.Reader, out io.Writer, name string, args ...string) error {
	path, err := tool(name)
	if err != nil {
		return err
	}
	cmd := exec.CommandContext(ctx, path, args...)
	cmd.Stdin, cmd.Stdout, cmd.Stderr = in, out, out
	return cmd.Run()
}

func notesDir() (string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(home, "neorg"), nil
}

func configDir() (string, error) {
	root := os.Getenv("XDG_CONFIG_HOME")
	if root == "" {
		home, err := os.UserHomeDir()
		if err != nil {
			return "", err
		}
		root = filepath.Join(home, ".config")
	}
	if !filepath.IsAbs(root) {
		return "", fmt.Errorf("XDG_CONFIG_HOME must be absolute")
	}
	return root, nil
}

func stateDir() (string, error) {
	root := os.Getenv("XDG_STATE_HOME")
	if root == "" {
		home, err := os.UserHomeDir()
		if err != nil {
			return "", err
		}
		root = filepath.Join(home, ".local", "state")
	}
	if !filepath.IsAbs(root) {
		return "", fmt.Errorf("XDG_STATE_HOME must be absolute")
	}
	return filepath.Join(root, "mak", "neorg"), nil
}

func Notify(ctx context.Context, title, message string, out io.Writer) error {
	args := []string{"-title", title, "-message", message, "-sound", "Crystal"}
	if home, err := os.UserHomeDir(); err == nil {
		icon := filepath.Join(home, "Pictures", "tomato.png")
		if st, err := os.Stat(icon); err == nil && st.Mode().IsRegular() {
			args = append(args, "-appIcon", icon)
		}
	}
	return run(ctx, nil, out, "terminal-notifier", args...)
}

func Timer(ctx context.Context, in io.Reader, out io.Writer, rest bool) error {
	// Fail before waiting if notifications cannot run.
	if _, err := tool("terminal-notifier"); err != nil {
		return err
	}
	duration, title := "50m", "Work Timer is up! Take a Break 😊"
	if rest {
		duration, title = "10m", "Break is over! Get back to work 😬"
	}
	if err := run(ctx, in, out, "timer", duration); err != nil {
		return err
	}
	return Notify(ctx, title, "Pomodoro", out)
}
