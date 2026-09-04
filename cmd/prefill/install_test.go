package prefill

import (
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"testing"
)

func TestValidExtensionID(t *testing.T) {
	if !validExtensionID("abcdefghijklmnopabcdefghijklmnop") {
		t.Fatal("valid Chrome extension ID was rejected")
	}
	for _, id := range []string{"short", "abcdefghijklmnopabcdefghijklmnoq", "ABCDEFGHIJKLMNOPABCDEFGHIJKLMNOP"} {
		if validExtensionID(id) {
			t.Fatalf("invalid Chrome extension ID %q was accepted", id)
		}
	}
}

func TestWriteExtensionFiles(t *testing.T) {
	dir := t.TempDir()
	if err := writeExtensionFiles(dir); err != nil {
		t.Fatal(err)
	}

	manifestData, err := os.ReadFile(filepath.Join(dir, "manifest.json"))
	if err != nil {
		t.Fatal(err)
	}
	var manifest map[string]any
	if err := json.Unmarshal(manifestData, &manifest); err != nil {
		t.Fatalf("generated manifest is invalid JSON: %v", err)
	}

	node, err := exec.LookPath("node")
	if err != nil {
		t.Skip("node is not installed; skipping generated JavaScript syntax check")
	}
	for _, name := range []string{"background.js", "popup.js"} {
		if output, err := exec.Command(node, "--check", filepath.Join(dir, name)).CombinedOutput(); err != nil {
			t.Fatalf("generated %s has invalid JavaScript: %v\n%s", name, err, output)
		}
	}
}
