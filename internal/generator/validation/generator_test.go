package validation

import (
	"os"
	"path/filepath"
	"testing"
)

func TestQuasarGeneratorCreatesExpectedFiles(t *testing.T) {
	dir := t.TempDir()
	if err := (&quasarGenerator{}).Generate(dir); err != nil {
		t.Fatal(err)
	}

	for _, name := range []string{"useForm.ts", "useValidationRules.ts"} {
		if _, err := os.Stat(filepath.Join(dir, "src", "composables", name)); err != nil {
			t.Fatalf("expected %s to be generated: %v", name, err)
		}
	}
}

func TestGeneratorPreflightsAllDestinations(t *testing.T) {
	dir := t.TempDir()
	composables := filepath.Join(dir, "app", "composables")
	if err := os.MkdirAll(composables, 0o755); err != nil {
		t.Fatal(err)
	}
	existing := filepath.Join(composables, "useValidationRules.ts")
	if err := os.WriteFile(existing, []byte("keep me"), 0o644); err != nil {
		t.Fatal(err)
	}

	if err := (&nuxt4Generator{}).Generate(dir); err == nil {
		t.Fatal("Generate() succeeded despite an existing destination")
	}
	if _, err := os.Stat(filepath.Join(composables, "useForm.ts")); !os.IsNotExist(err) {
		t.Fatalf("generator wrote a partial result; stat error = %v", err)
	}
	data, err := os.ReadFile(existing)
	if err != nil {
		t.Fatal(err)
	}
	if string(data) != "keep me" {
		t.Fatalf("existing file was modified: %q", data)
	}
}
