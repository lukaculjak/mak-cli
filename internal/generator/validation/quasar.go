package validation

import (
	"fmt"
	"os"
	"path/filepath"
)

type quasarGenerator struct{}

func (g *quasarGenerator) Generate(dir string) error {
	composablesDir := filepath.Join(dir, "src", "composables")

	if err := os.MkdirAll(composablesDir, 0o755); err != nil {
		return fmt.Errorf("creating composables dir: %w", err)
	}

	files := map[string]string{
		"useValidationRules.ts": useValidationRules,
		"useForm.ts":            useForm,
	}

	for name, content := range files {
		path := filepath.Join(composablesDir, name)
		if err := writeFile(path, content); err != nil {
			return err
		}
		fmt.Printf("  created  src/composables/%s\n", name)
	}

	fmt.Println("\nQuasar validation setup complete.")
	fmt.Println("Import with: import { useForm } from 'src/composables/useForm'")
	return nil
}
