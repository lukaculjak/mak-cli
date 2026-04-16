package prefills

import (
	"encoding/base64"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
)

// ErrWrongPassword is returned when decryption fails due to a bad master password.
var ErrWrongPassword = errors.New("wrong master password")

type encryptedStore struct {
	Salt   string `json:"salt"`   // base64-encoded random salt
	Verify string `json:"verify"` // base64-encoded encrypted verifyTag
	Data   string `json:"data"`   // base64-encoded encrypted JSON of []Project
}

func storePath() (string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(home, ".config", "mak", "prefills.enc"), nil
}

// HasStore reports whether the encrypted store file exists.
func HasStore() bool {
	path, err := storePath()
	if err != nil {
		return false
	}
	_, err = os.Stat(path)
	return err == nil
}

// InitStore creates a new store protected by masterPassword with an empty project list.
func InitStore(masterPassword string) error {
	salt, err := newSalt()
	if err != nil {
		return err
	}
	key, err := deriveKey(masterPassword, salt)
	if err != nil {
		return err
	}

	verifyEnc, err := encrypt(key, []byte(verifyTag))
	if err != nil {
		return err
	}

	dataEnc, err := encrypt(key, []byte("[]"))
	if err != nil {
		return err
	}

	return writeStore(encryptedStore{
		Salt:   base64.StdEncoding.EncodeToString(salt),
		Verify: base64.StdEncoding.EncodeToString(verifyEnc),
		Data:   base64.StdEncoding.EncodeToString(dataEnc),
	})
}

// VerifyPassword returns true if masterPassword can decrypt the store.
func VerifyPassword(masterPassword string) (bool, error) {
	st, err := readStore()
	if err != nil {
		return false, err
	}
	salt, err := base64.StdEncoding.DecodeString(st.Salt)
	if err != nil {
		return false, err
	}
	key, err := deriveKey(masterPassword, salt)
	if err != nil {
		return false, err
	}
	verifyEnc, err := base64.StdEncoding.DecodeString(st.Verify)
	if err != nil {
		return false, err
	}
	plain, err := decrypt(key, verifyEnc)
	if err != nil {
		return false, nil // decryption failure = wrong password
	}
	return string(plain) == verifyTag, nil
}

// Load decrypts and returns all projects. Returns ErrWrongPassword on bad password.
func Load(masterPassword string) ([]Project, error) {
	st, err := readStore()
	if err != nil {
		return nil, err
	}
	salt, err := base64.StdEncoding.DecodeString(st.Salt)
	if err != nil {
		return nil, err
	}
	key, err := deriveKey(masterPassword, salt)
	if err != nil {
		return nil, err
	}
	dataEnc, err := base64.StdEncoding.DecodeString(st.Data)
	if err != nil {
		return nil, err
	}
	plain, err := decrypt(key, dataEnc)
	if err != nil {
		return nil, ErrWrongPassword
	}
	var projects []Project
	if err := json.Unmarshal(plain, &projects); err != nil {
		return nil, err
	}
	return projects, nil
}

// Save encrypts and persists projects using masterPassword.
func Save(masterPassword string, projects []Project) error {
	st, err := readStore()
	if err != nil {
		return err
	}
	salt, err := base64.StdEncoding.DecodeString(st.Salt)
	if err != nil {
		return err
	}
	key, err := deriveKey(masterPassword, salt)
	if err != nil {
		return err
	}
	raw, err := json.Marshal(projects)
	if err != nil {
		return err
	}
	dataEnc, err := encrypt(key, raw)
	if err != nil {
		return err
	}
	st.Data = base64.StdEncoding.EncodeToString(dataEnc)
	return writeStore(*st)
}

func readStore() (*encryptedStore, error) {
	path, err := storePath()
	if err != nil {
		return nil, err
	}
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	var st encryptedStore
	if err := json.Unmarshal(data, &st); err != nil {
		return nil, err
	}
	return &st, nil
}

func writeStore(st encryptedStore) error {
	path, err := storePath()
	if err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
		return err
	}
	data, err := json.MarshalIndent(st, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(path, data, 0600)
}
