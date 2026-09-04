package updater

import (
	"os"
	"path/filepath"
	"testing"
)

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
