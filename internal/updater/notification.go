package updater

import (
	"encoding/json"
	"os"
	"path/filepath"
	"time"

	"golang.org/x/mod/semver"
)

type updateCheck struct {
	CheckedAt time.Time `json:"checked_at"`
	Latest    string    `json:"latest,omitempty"`
}

// Cache failed checks too: an offline connection should not delay every command.
func cachedLatestVersion(path string, now time.Time, fetch func() (string, error)) string {
	var cached updateCheck
	if b, err := os.ReadFile(path); err == nil {
		_ = json.Unmarshal(b, &cached)
	}
	if !semver.IsValid("v" + cached.Latest) {
		cached.Latest = ""
	}
	if age := now.Sub(cached.CheckedAt); age >= 0 && age < 24*time.Hour {
		return cached.Latest
	}
	cached.CheckedAt = now
	if latest, err := fetch(); err == nil {
		cached.Latest = latest
	}
	// Notification caching is best-effort and must not fail the user's command.
	if os.MkdirAll(filepath.Dir(path), 0700) == nil {
		if f, err := os.CreateTemp(filepath.Dir(path), ".update-check-*"); err == nil {
			defer os.Remove(f.Name())
			b, _ := json.Marshal(cached)
			_, writeErr := f.Write(b)
			closeErr := f.Close()
			if writeErr == nil && closeErr == nil {
				_ = os.Rename(f.Name(), path)
			}
		}
	}
	return cached.Latest
}
