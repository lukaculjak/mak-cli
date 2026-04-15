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
	return os.WriteFile(path, data, 0644)
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
