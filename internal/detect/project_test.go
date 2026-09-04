package detect

import (
	"os"
	"path/filepath"
	"testing"
)

func TestDetectNuxt4(t *testing.T) {
	dir := t.TempDir()
	writeTestFile(t, filepath.Join(dir, "nuxt.config.ts"), "export default defineNuxtConfig({})")
	writeTestFile(t, filepath.Join(dir, "package.json"), `{"devDependencies":{"nuxt":"^4.1.0"}}`)

	got, err := Detect(dir)
	if err != nil {
		t.Fatal(err)
	}
	if got != Nuxt4 {
		t.Fatalf("Detect() = %q, want %q", got, Nuxt4)
	}
}

func TestDetectRejectsNuxt3(t *testing.T) {
	dir := t.TempDir()
	writeTestFile(t, filepath.Join(dir, "nuxt.config.ts"), "export default defineNuxtConfig({})")
	writeTestFile(t, filepath.Join(dir, "package.json"), `{"dependencies":{"nuxt":"~3.20.0"}}`)

	if _, err := Detect(dir); err == nil {
		t.Fatal("Detect() accepted a Nuxt 3 project as Nuxt 4")
	}
}

func TestDetectQuasar(t *testing.T) {
	dir := t.TempDir()
	writeTestFile(t, filepath.Join(dir, "quasar.config.ts"), "export default {}")

	got, err := Detect(dir)
	if err != nil {
		t.Fatal(err)
	}
	if got != Quasar {
		t.Fatalf("Detect() = %q, want %q", got, Quasar)
	}
}

func writeTestFile(t *testing.T, path, content string) {
	t.Helper()
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
}
