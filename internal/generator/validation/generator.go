package validation

import (
	"fmt"

	"github.com/lukaculjak/mak/internal/detect"
)

type Generator interface {
	Generate(dir string) error
}

func NewGenerator(pt detect.ProjectType) (Generator, error) {
	switch pt {
	case detect.Quasar:
		return &quasarGenerator{}, nil
	case detect.Nuxt4:
		return &nuxt4Generator{}, nil
	default:
		return nil, fmt.Errorf("no validation generator for project type %q", pt)
	}
}
