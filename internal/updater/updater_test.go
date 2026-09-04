package updater

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestVersionIsNewer(t *testing.T) {
	tests := []struct {
		latest  string
		current string
		want    bool
	}{
		{latest: "1.2.0", current: "1.1.9", want: true},
		{latest: "1.2.0", current: "1.2.0", want: false},
		{latest: "1.2.0", current: "1.3.0", want: false},
		{latest: "1.2.0", current: "v1.1.0", want: true},
		{latest: "1.2.0", current: "1.2.0-beta.1", want: true},
	}

	for _, tt := range tests {
		if got := versionIsNewer(tt.latest, tt.current); got != tt.want {
			t.Errorf("versionIsNewer(%q, %q) = %v, want %v", tt.latest, tt.current, got, tt.want)
		}
	}
}

func TestParseChecksum(t *testing.T) {
	checksum := strings.Repeat("a", 64)
	input := strings.NewReader(checksum + "  mak_darwin_arm64.tar.gz\n")
	got, err := parseChecksum(input, "mak_darwin_arm64.tar.gz")
	if err != nil {
		t.Fatal(err)
	}
	if got != checksum {
		t.Fatalf("fetchChecksum() = %q, want %q", got, checksum)
	}
}

func TestReplaceBinaryKeepsExecutableBit(t *testing.T) {
	dir := t.TempDir()

	newBin := filepath.Join(dir, "downloaded-mak")
	if err := os.WriteFile(newBin, []byte("new binary contents"), 0o600); err != nil {
		t.Fatal(err)
	}

	execPath := filepath.Join(dir, "mak")
	if err := os.WriteFile(execPath, []byte("old binary contents"), 0o755); err != nil {
		t.Fatal(err)
	}

	if err := replaceBinary(newBin, execPath); err != nil {
		t.Fatalf("replaceBinary: %v", err)
	}

	got, err := os.ReadFile(execPath)
	if err != nil {
		t.Fatal(err)
	}
	if string(got) != "new binary contents" {
		t.Fatalf("binary not replaced: got %q", got)
	}

	info, err := os.Stat(execPath)
	if err != nil {
		t.Fatal(err)
	}
	if info.Mode().Perm()&0o111 == 0 {
		t.Errorf("replaced binary is not executable: mode %v — next `mak` invocation would fail with permission denied", info.Mode().Perm())
	}
}
