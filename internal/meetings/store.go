package meetings

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
)

func storePath() (string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(home, ".config", "mak", "meetings.json"), nil
}

func Load() ([]Meeting, error) {
	path, err := storePath()
	if err != nil {
		return nil, err
	}
	data, err := os.ReadFile(path)
	if os.IsNotExist(err) {
		return []Meeting{}, nil
	}
	if err != nil {
		return nil, err
	}
	var list []Meeting
	if err := json.Unmarshal(data, &list); err != nil {
		return nil, err
	}
	return list, nil
}

func Save(list []Meeting) error {
	path, err := storePath()
	if err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
		return err
	}
	data, err := json.MarshalIndent(list, "", "  ")
	if err != nil {
		return err
	}
	return atomicWriteFile(path, data, 0o644)
}

func atomicWriteFile(path string, data []byte, mode os.FileMode) error {
	tmp, err := os.CreateTemp(filepath.Dir(path), ".mak-meetings-*")
	if err != nil {
		return err
	}
	tmpPath := tmp.Name()
	defer os.Remove(tmpPath)

	if err := tmp.Chmod(mode); err != nil {
		tmp.Close()
		return err
	}
	if _, err := tmp.Write(data); err != nil {
		tmp.Close()
		return err
	}
	if err := tmp.Sync(); err != nil {
		tmp.Close()
		return err
	}
	if err := tmp.Close(); err != nil {
		return err
	}
	return os.Rename(tmpPath, path)
}

// FindByAlias returns the index and pointer to the meeting with the given alias (case-insensitive).
// Returns -1, nil if not found.
func FindByAlias(list []Meeting, alias string) (int, *Meeting) {
	lower := strings.ToLower(alias)
	for i := range list {
		if strings.ToLower(list[i].Alias) == lower {
			return i, &list[i]
		}
	}
	return -1, nil
}
