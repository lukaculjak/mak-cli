package validation

import (
	"fmt"
	"os"
)

func writeFile(path, content string) error {
	if _, err := os.Stat(path); err == nil {
		return fmt.Errorf("file already exists: %s (remove it first if you want to regenerate)", path)
	}
	return os.WriteFile(path, []byte(content), 0o644)
}
