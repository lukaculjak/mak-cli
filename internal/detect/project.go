package detect

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
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

	nuxtMarkers := []string{"nuxt.config.ts", "nuxt.config.js", "nuxt.config.mjs"}
	for _, f := range nuxtMarkers {
		if fileExists(filepath.Join(dir, f)) {
			major, err := nuxtMajorVersion(dir)
			if err != nil {
				return "", err
			}
			if major != 4 {
				return "", fmt.Errorf("Nuxt %d project detected; validation generation requires Nuxt 4", major)
			}
			return Nuxt4, nil
		}
	}

	return "", fmt.Errorf("not a Quasar or Nuxt project, run mak from the project root")
}

var versionNumber = regexp.MustCompile(`\d+`)

func nuxtMajorVersion(dir string) (int, error) {
	data, err := os.ReadFile(filepath.Join(dir, "package.json"))
	if err != nil {
		return 0, fmt.Errorf("reading package.json to determine Nuxt version: %w", err)
	}

	var pkg struct {
		Dependencies    map[string]string `json:"dependencies"`
		DevDependencies map[string]string `json:"devDependencies"`
	}
	if err := json.Unmarshal(data, &pkg); err != nil {
		return 0, fmt.Errorf("reading Nuxt version from package.json: %w", err)
	}
	version := pkg.Dependencies["nuxt"]
	if version == "" {
		version = pkg.DevDependencies["nuxt"]
	}
	majorText := versionNumber.FindString(version)
	if majorText == "" {
		return 0, fmt.Errorf("could not determine Nuxt version from package.json")
	}
	major, err := strconv.Atoi(majorText)
	if err != nil {
		return 0, fmt.Errorf("reading Nuxt version from package.json: %w", err)
	}
	return major, nil
}

func fileExists(path string) bool {
	_, err := os.Stat(path)
	return err == nil
}
