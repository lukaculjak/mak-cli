package productivity

import (
	"context"
	"fmt"
	"time"
)

// Request hydration once, then wait until macOS has made the contents available.
// Never read a dataless file or assume that requesting a download completed it.
func downloadAndWait(ctx context.Context, path string, dataless func() (bool, error), request func(context.Context) error) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	pending, err := dataless()
	if err != nil || !pending {
		return err
	}
	ctx, cancel := context.WithTimeout(ctx, 2*time.Minute)
	defer cancel()
	guidance := "check your internet connection, or choose Keep Downloaded on .MAK in Finder, wait for downloads, then retry"
	if err := request(ctx); err != nil {
		if ctx.Err() != nil {
			err = ctx.Err()
		}
		return fmt.Errorf("downloading %s: %w; %s", path, err, guidance)
	}
	ticker := time.NewTicker(250 * time.Millisecond)
	defer ticker.Stop()
	for {
		if err := ctx.Err(); err != nil {
			return fmt.Errorf("waiting for iCloud download of %s: %w; %s", path, err, guidance)
		}
		pending, err := dataless()
		if err != nil {
			return err
		}
		if !pending {
			return nil
		}
		select {
		case <-ctx.Done():
		case <-ticker.C:
		}
	}
}
