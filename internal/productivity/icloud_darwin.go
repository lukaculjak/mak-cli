//go:build darwin

package productivity

import (
	"context"
	"fmt"
	"os/exec"
	"strings"

	"golang.org/x/sys/unix"
)

func checkDownloaded(ctx context.Context, path string) error {
	return downloadAndWait(ctx, path, func() (bool, error) {
		var st unix.Stat_t
		if err := unix.Lstat(path, &st); err != nil {
			return false, err
		}
		return st.Flags&unix.SF_DATALESS != 0, nil
	}, func(ctx context.Context) error {
		output, err := exec.CommandContext(ctx, "/usr/bin/brctl", "download", path).CombinedOutput()
		if err != nil {
			return fmt.Errorf("requesting iCloud download: %w (%s)", err, strings.TrimSpace(string(output)))
		}
		return nil
	})
}
