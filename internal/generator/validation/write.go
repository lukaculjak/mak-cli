package validation

import (
	"fmt"
	"os"
	"path/filepath"
)

type generatedFile struct {
	name    string
	content string
}

func writeFiles(dir string, files []generatedFile) error {
	for _, file := range files {
		path := filepath.Join(dir, file.name)
		if _, err := os.Stat(path); err == nil {
			return fmt.Errorf("file already exists: %s (remove it first if you want to regenerate)", path)
		} else if !os.IsNotExist(err) {
			return fmt.Errorf("checking %s: %w", path, err)
		}
	}

	for _, file := range files {
		path := filepath.Join(dir, file.name)
		if err := os.WriteFile(path, []byte(file.content), 0o644); err != nil {
			return fmt.Errorf("writing %s: %w", path, err)
		}
	}
	return nil
}
