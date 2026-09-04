package updater

import (
	"archive/tar"
	"bufio"
	"compress/gzip"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"time"

	"golang.org/x/mod/semver"
)

const apiURL = "https://api.github.com/repos/lukaculjak/mak-cli/releases/latest"

const releaseURL = "https://github.com/lukaculjak/mak-cli/releases/download"

var downloadClient = &http.Client{Timeout: 30 * time.Second}
var metadataClient = &http.Client{Timeout: 3 * time.Second}

// LatestVersion fetches the latest release tag from GitHub and returns the
// version string without the "v" prefix (e.g. "0.1.0").
func LatestVersion() (string, error) {
	resp, err := metadataClient.Get(apiURL)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("GitHub returned status %d", resp.StatusCode)
	}

	var result struct {
		TagName string `json:"tag_name"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return "", err
	}
	version := strings.TrimPrefix(result.TagName, "v")
	if !semver.IsValid("v" + version) {
		return "", fmt.Errorf("GitHub returned invalid release version %q", result.TagName)
	}
	return version, nil
}

func versionIsNewer(latest, current string) bool {
	return semver.Compare("v"+latest, "v"+strings.TrimPrefix(current, "v")) > 0
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
	if versionIsNewer(latest, currentVersion) {
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

	if currentVersion != "dev" && !versionIsNewer(latest, currentVersion) {
		fmt.Println("mak is already up to date!")
		return nil
	}

	fmt.Printf("Updating mak v%s → v%s...\n", currentVersion, latest)

	tarName := fmt.Sprintf("mak_%s_%s.tar.gz", runtime.GOOS, runtime.GOARCH)
	baseURL := fmt.Sprintf("%s/v%s", releaseURL, latest)
	url := fmt.Sprintf("%s/%s", baseURL, tarName)

	// download tarball to a temp file
	tmp, err := os.CreateTemp("", "mak-update-*.tar.gz")
	if err != nil {
		return fmt.Errorf("creating temp file: %w", err)
	}
	defer os.Remove(tmp.Name())

	resp, err := downloadClient.Get(url)
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
	if err := tmp.Close(); err != nil {
		return fmt.Errorf("closing download: %w", err)
	}

	expected, err := fetchChecksum(fmt.Sprintf("%s/checksums.txt", baseURL), tarName)
	if err != nil {
		return fmt.Errorf("verifying update: %w", err)
	}
	actual, err := fileChecksum(tmp.Name())
	if err != nil {
		return fmt.Errorf("verifying update: %w", err)
	}
	if actual != expected {
		return fmt.Errorf("verifying update: checksum mismatch for %s", tarName)
	}

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

	if err := replaceBinary(newBin, execPath); err != nil {
		return err
	}

	fmt.Printf("mak updated to v%s\n", latest)
	return nil
}

func fetchChecksum(url, filename string) (string, error) {
	resp, err := downloadClient.Get(url)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("unexpected status %d downloading checksums", resp.StatusCode)
	}
	return parseChecksum(resp.Body, filename)
}

func parseChecksum(r io.Reader, filename string) (string, error) {
	scanner := bufio.NewScanner(r)
	for scanner.Scan() {
		fields := strings.Fields(scanner.Text())
		if len(fields) == 2 && strings.TrimPrefix(fields[1], "*") == filename {
			if len(fields[0]) != sha256.Size*2 {
				return "", fmt.Errorf("invalid checksum for %s", filename)
			}
			if _, err := hex.DecodeString(fields[0]); err != nil {
				return "", fmt.Errorf("invalid checksum for %s", filename)
			}
			return strings.ToLower(fields[0]), nil
		}
	}
	if err := scanner.Err(); err != nil {
		return "", err
	}
	return "", fmt.Errorf("checksum for %s not found", filename)
}

func fileChecksum(path string) (string, error) {
	f, err := os.Open(path)
	if err != nil {
		return "", err
	}
	defer f.Close()

	hash := sha256.New()
	if _, err := io.Copy(hash, f); err != nil {
		return "", err
	}
	return fmt.Sprintf("%x", hash.Sum(nil)), nil
}

// replaceBinary atomically replaces the executable at execPath with the
// contents of newBin: it stages a copy in the same directory, marks it
// executable, then renames it over execPath (rename is atomic on Unix,
// so execPath never holds a half-written binary).
func replaceBinary(newBin, execPath string) error {
	staged, err := os.CreateTemp(filepath.Dir(execPath), "mak-new-*")
	if err != nil {
		return fmt.Errorf("staging new binary (try with sudo?): %w", err)
	}
	if err := staged.Close(); err != nil {
		return fmt.Errorf("staging new binary: %w", err)
	}
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
	if err := dst.Close(); err != nil {
		return fmt.Errorf("closing staged binary: %w", err)
	}

	if err := os.Chmod(staged.Name(), 0o755); err != nil {
		return fmt.Errorf("making new binary executable: %w", err)
	}

	if err := os.Rename(staged.Name(), execPath); err != nil {
		return fmt.Errorf("replacing binary (try running with sudo): %w", err)
	}
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
			if err := out.Close(); err != nil {
				os.Remove(out.Name())
				return "", err
			}
			return out.Name(), nil
		}
	}
	return "", fmt.Errorf("mak binary not found in archive")
}
