package validation

import (
	"fmt"
	"os"
	"path/filepath"
)

type nuxt4Generator struct{}

// Nuxt 4 uses the app/ directory by default.
func (g *nuxt4Generator) Generate(dir string) error {
	composablesDir := filepath.Join(dir, "app", "composables")

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
		fmt.Printf("  created  app/composables/%s\n", name)
	}

	fmt.Println("\nNuxt 4 validation setup complete.")
	fmt.Println("Composables are auto-imported — use useForm() and useValidationRules directly.")
	return nil
}
