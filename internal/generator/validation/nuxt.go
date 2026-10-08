package validation

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/lukaculjak/mak-cli/internal/ui"
)

type nuxt4Generator struct{}

func (g *nuxt4Generator) Generate(dir string) error {
	composablesDir := filepath.Join(dir, "app", "composables")

	if err := os.MkdirAll(composablesDir, 0o755); err != nil {
		return fmt.Errorf("creating composables dir: %w", err)
	}

	files := []generatedFile{
		{name: "useForm.ts", content: useForm},
		{name: "useValidationRules.ts", content: useValidationRules},
	}

	if err := writeFiles(composablesDir, files); err != nil {
		return err
	}
	for _, file := range files {
		ui.Success(os.Stdout, "Created app/composables/%s", file.name)
	}

	fmt.Println()
	ui.Success(os.Stdout, "Nuxt 4 validation setup complete.")
	fmt.Println("Composables are auto-imported, use useForm() and useValidationRules directly.")
	return nil
}
