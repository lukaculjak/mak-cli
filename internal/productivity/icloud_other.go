//go:build !darwin

package productivity

import "context"

// The public sync command is macOS-only; filesystem tests also run on Linux.
func checkDownloaded(context.Context, string) error { return nil }
