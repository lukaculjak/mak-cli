package detect

import (
	"fmt"
	"os"
	"path/filepath"
)

type ProjectType string

const (
	Quasar ProjectType = "quasar"
	Nuxt4  ProjectType = "nuxt4"
)

func Detect(dir string) (ProjectType, error) {
	quasarMarkers := []string{"quasar.config.js", "quasar.config.ts", "quasar.conf.js"}
	for _, f := range quasarMarkers {
		if fileExists(filepath.Join(dir, f)) {
			return Quasar, nil
		}
	}

	nuxtMarkers := []string{"nuxt.config.ts", "nuxt.config.js"}
	for _, f := range nuxtMarkers {
		if fileExists(filepath.Join(dir, f)) {
			return Nuxt4, nil
		}
	}

	return "", fmt.Errorf("not a Quasar or Nuxt project, run mak from the project root")
}

func fileExists(path string) bool {
	_, err := os.Stat(path)
	return err == nil
}
