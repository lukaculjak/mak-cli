package setup

import (
	"fmt"

	"github.com/lukaculjak/mak-cli/internal/detect"
	"github.com/lukaculjak/mak-cli/internal/generator/validation"
	"github.com/spf13/cobra"
)

func newValidationCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "validation",
		Short: "Generate useForm and useValidationRules composables",
		Long: `Detects whether the current directory is a Quasar or Nuxt 4 project
and generates the useForm + useValidationRules composables in the right location.`,
		RunE: func(cmd *cobra.Command, args []string) error {
			pt, err := detect.Detect(".")
			if err != nil {
				return err
			}

			fmt.Printf("Detected project: %s\n\n", pt)

			gen, err := validation.NewGenerator(pt)
			if err != nil {
				return err
			}

			return gen.Generate(".")
		},
	}
}
