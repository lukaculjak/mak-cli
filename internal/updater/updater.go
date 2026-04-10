package updater

import (
	"archive/tar"
	"compress/gzip"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"time"
)

const apiURL = "https://api.github.com/repos/lukaculjak/mak-cli/releases/latest"

// LatestVersion fetches the latest release tag from GitHub and returns the
// version string without the "v" prefix (e.g. "0.1.0").
func LatestVersion() (string, error) {
	client := &http.Client{Timeout: 3 * time.Second}
	resp, err := client.Get(apiURL)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	var result struct {
		TagName string `json:"tag_name"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return "", err
	}
	return strings.TrimPrefix(result.TagName, "v"), nil
}

// CheckAndNotify compares currentVersion against the latest GitHub release.
// If a newer version exists, it prints a notice. Silently no-ops on any error
// so it never interrupts a running command.
func CheckAndNotify(currentVersion string) {
	if currentVersion == "dev" {
		return
	}
	latest, err := LatestVersion()
	if err != nil {
		return
	}
	if latest != currentVersion {
		fmt.Printf("\nA new version of mak is available (v%s)! Run `mak update` to upgrade.\n", latest)
	}
}

// SelfUpdate downloads the latest release binary and replaces the running
// executable. Returns nil if already on the latest version.
func SelfUpdate(currentVersion string) error {
	latest, err := LatestVersion()
	if err != nil {
		return fmt.Errorf("could not fetch latest version: %w", err)
	}

	if latest == currentVersion {
		fmt.Println("mak is already up to date!")
		return nil
	}

	fmt.Printf("Updating mak v%s → v%s...\n", currentVersion, latest)

	tarName := fmt.Sprintf("mak_%s_%s.tar.gz", runtime.GOOS, runtime.GOARCH)
	url := fmt.Sprintf("https://github.com/lukaculjak/mak-cli/releases/download/v%s/%s", latest, tarName)

	// download tarball to a temp file
	tmp, err := os.CreateTemp("", "mak-update-*.tar.gz")
	if err != nil {
		return fmt.Errorf("creating temp file: %w", err)
	}
	defer os.Remove(tmp.Name())

	resp, err := http.Get(url)
	if err != nil {
		return fmt.Errorf("downloading update: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("unexpected status %d downloading %s", resp.StatusCode, url)
	}

	if _, err := io.Copy(tmp, resp.Body); err != nil {
		return fmt.Errorf("writing download: %w", err)
	}
	tmp.Close()

	// extract the mak binary from the tarball
	newBin, err := extractBinary(tmp.Name())
	if err != nil {
		return fmt.Errorf("extracting binary: %w", err)
	}
	defer os.Remove(newBin)

	// find the path of the currently running executable
	execPath, err := os.Executable()
	if err != nil {
		return fmt.Errorf("finding current executable: %w", err)
	}

	// write the new binary to a temp file in the same directory, then rename
	// (rename is atomic on Unix, avoids a half-written binary)
	staged, err := os.CreateTemp(filepath.Dir(execPath), "mak-new-*")
	if err != nil {
		return fmt.Errorf("staging new binary (try with sudo?): %w", err)
	}
	staged.Close()
	defer os.Remove(staged.Name())

	src, err := os.Open(newBin)
	if err != nil {
		return err
	}
	defer src.Close()

	dst, err := os.OpenFile(staged.Name(), os.O_WRONLY|os.O_TRUNC, 0o755)
	if err != nil {
		return err
	}
	if _, err := io.Copy(dst, src); err != nil {
		dst.Close()
		return fmt.Errorf("writing new binary: %w", err)
	}
	dst.Close()

	if err := os.Rename(staged.Name(), execPath); err != nil {
		return fmt.Errorf("replacing binary (try running with sudo): %w", err)
	}

	fmt.Printf("mak updated to v%s\n", latest)
	return nil
}

func extractBinary(tarPath string) (string, error) {
	f, err := os.Open(tarPath)
	if err != nil {
		return "", err
	}
	defer f.Close()

	gz, err := gzip.NewReader(f)
	if err != nil {
		return "", err
	}
	defer gz.Close()

	tr := tar.NewReader(gz)
	for {
		hdr, err := tr.Next()
		if err == io.EOF {
			break
		}
		if err != nil {
			return "", err
		}
		if filepath.Base(hdr.Name) == "mak" {
			out, err := os.CreateTemp("", "mak-binary-*")
			if err != nil {
				return "", err
			}
			if _, err := io.Copy(out, tr); err != nil {
				out.Close()
				os.Remove(out.Name())
				return "", err
			}
			out.Close()
			return out.Name(), nil
		}
	}
	return "", fmt.Errorf("mak binary not found in archive")
}
