package devenv

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"unicode"

	"github.com/lukaculjak/mak-cli/internal/ui"
)

func (i *installer) ensureBrewPermissions(ctx context.Context) error {
	permissionErr := i.checkBrewPermissions(ctx)
	if permissionErr == nil || i.in == nil || ctx.Err() != nil {
		return permissionErr
	}
	prefix, err := i.run(ctx, i.env, i.brew, "--prefix")
	if err != nil {
		return permissionErr
	}
	prefix = strings.TrimSpace(prefix)
	// Automatically repair only standard macOS installations or a private
	// installation under this user's home; never take ownership of /usr/local.
	if prefix != "/usr/local" && prefix != "/opt/homebrew" && !strings.HasPrefix(prefix, i.home+"/") {
		return permissionErr
	}
	dirs := brewPermissionDirectories(permissionErr.Error(), prefix)
	if len(dirs) == 0 {
		return permissionErr
	}
	resolvedPrefix, err := filepath.EvalSymlinks(prefix)
	if err != nil {
		return permissionErr
	}
	if prefix == "/usr/local" || prefix == "/opt/homebrew" {
		if resolvedPrefix != prefix {
			return permissionErr
		}
	} else {
		resolvedHome, err := filepath.EvalSymlinks(i.home)
		if err != nil || !strings.HasPrefix(resolvedPrefix, resolvedHome+"/") {
			return permissionErr
		}
	}
	for _, dir := range dirs {
		resolved, err := filepath.EvalSymlinks(dir)
		if err != nil || (resolved != resolvedPrefix && !strings.HasPrefix(resolved, resolvedPrefix+"/")) || (resolved == resolvedPrefix && dir != prefix) {
			return permissionErr
		}
		if st, err := os.Stat(dir); err != nil || !st.IsDir() {
			return permissionErr
		}
	}
	if i.progress != nil {
		i.progress.Pause()
		defer i.progress.Resume()
	}
	ui.Warning(i.out, "Homebrew directories are not writable by this account.")
	fmt.Fprintln(i.out, "Repair will recursively change ownership to your account and grant it write access to:")
	for _, dir := range dirs {
		fmt.Fprintf(i.out, "  %s\n", dir)
	}
	fmt.Fprintln(i.out, "This may affect other accounts using Homebrew. Permission repairs remain after a setup failure or uninstall. An administrator password may be required.")
	confirmed, err := ui.ConfirmOptIn(ctx, i.in, i.out, "Repair these Homebrew permissions and continue?")
	if err != nil {
		return err
	}
	if !confirmed {
		return permissionErr
	}
	// Pass arguments directly. Never execute the suggested shell commands from
	// diagnostic output, and don't follow symlinks during recursive ownership repair.
	args := append([]string{"/usr/sbin/chown", "-R", "-P", strconv.Itoa(os.Getuid())}, dirs...)
	if _, err := i.run(ctx, i.env, "/usr/bin/sudo", args...); err != nil {
		return fmt.Errorf("repairing Homebrew ownership: %w", err)
	}
	if _, err := i.run(ctx, i.env, "/bin/chmod", append([]string{"u+w"}, dirs...)...); err != nil {
		return fmt.Errorf("repairing Homebrew write access: %w", err)
	}
	if err := i.checkBrewPermissions(ctx); err != nil {
		return err
	}
	ui.Success(i.out, "Homebrew permissions repaired; continuing installation.")
	return nil
}

// Read only the directory list from the known access warning. Unknown output,
// missing directories, truncated lists, and paths outside the prefix need manual repair.
func brewPermissionDirectories(report, prefix string) []string {
	if !filepath.IsAbs(prefix) || filepath.Clean(prefix) != prefix || prefix == "/" || strings.Contains(report, "The following directories do not exist:") {
		return nil
	}
	const heading = "The following directories are not writable by your user:"
	_, list, found := strings.Cut(report, heading+"\n")
	if !found {
		return nil
	}
	var dirs []string
	for _, line := range strings.Split(list, "\n") {
		dir := strings.TrimSpace(line)
		if dir == "" {
			return dirs
		}
		if filepath.Clean(dir) != dir || strings.IndexFunc(dir, unicode.IsControl) != -1 || (!strings.HasPrefix(dir, prefix+"/") && !(dir == prefix && prefix == "/opt/homebrew")) {
			return nil
		}
		dirs = append(dirs, dir)
	}
	return nil
}
