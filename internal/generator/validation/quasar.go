package validation

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/lukaculjak/mak-cli/internal/ui"
)

type quasarGenerator struct{}

func (g *quasarGenerator) Generate(dir string) error {
	composablesDir := filepath.Join(dir, "src", "composables")

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
		ui.Success(os.Stdout, "Created src/composables/%s", file.name)
	}

	fmt.Println()
	ui.Success(os.Stdout, "Quasar validation setup complete.")
	fmt.Println("Import with: import { useForm } from 'src/composables/useForm'")
	return nil
}
