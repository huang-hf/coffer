package secret

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"filippo.io/age"
)

const (
	extPlaintext = ".json"
	extEncrypted = ".json.age"
)

type fileStore struct {
	dir      string
	ageKey   *age.X25519Identity
	ageKeyID string // cache of identity.Recipient().String() for display
}

type fileData struct {
	Value string `json:"value"`
}

func newFileStore() (*fileStore, error) {
	dir, err := StoreDir()
	if err != nil {
		return nil, err
	}

	fs := &fileStore{dir: dir}

	// Try to load age key — if not found, operate in plain mode
	keyPath := AgeKeyPath(dir)
	key, err := LoadAgeKey(keyPath)
	if err != nil {
		return nil, err
	}
	if key != nil {
		fs.ageKey = key
		fs.ageKeyID = key.Recipient().String()
	}

	return fs, nil
}

func (s *fileStore) isEncrypted() bool {
	return s.ageKey != nil
}

func (s *fileStore) getPaths(namespace, name string) (plain, encrypted string) {
	base := filepath.Join(s.dir, namespace, name)
	return base + extPlaintext, base + extEncrypted
}

func (s *fileStore) Set(namespace, name string, value []byte) error {
	dir := filepath.Join(s.dir, namespace)
	if err := os.MkdirAll(dir, 0700); err != nil {
		return fmt.Errorf("creating namespace directory: %w", err)
	}

	var jsonData []byte
	if s.isEncrypted() {
		// Marshal to JSON first, then encrypt
		var err error
		jsonData, err = json.Marshal(&fileData{Value: string(value)})
		if err != nil {
			return fmt.Errorf("marshaling secret: %w", err)
		}
	} else {
		data := &fileData{Value: string(value)}
		var err error
		jsonData, err = json.Marshal(data)
		if err != nil {
			return fmt.Errorf("marshaling secret: %w", err)
		}
	}

	if s.isEncrypted() {
		encrypted, err := ageEncrypt(jsonData, s.ageKey)
		if err != nil {
			return fmt.Errorf("encrypting secret: %w", err)
		}

		_, path := s.getPaths(namespace, name)
		if err := os.WriteFile(path, encrypted, 0600); err != nil {
			return fmt.Errorf("writing encrypted secret: %w", err)
		}
	} else {
		path, _ := s.getPaths(namespace, name)
		if err := os.WriteFile(path, jsonData, 0600); err != nil {
			return fmt.Errorf("writing secret: %w", err)
		}
	}

	return nil
}

// Get reads a secret. If the age key is loaded, it decrypts the .json.age file.
// Falls back to plain .json for backward compatibility.
func (s *fileStore) Get(namespace, name string) ([]byte, error) {
	plainPath, encPath := s.getPaths(namespace, name)

	if s.isEncrypted() {
		// Primary: read encrypted file
		data, err := os.ReadFile(encPath)
		if err != nil {
			if !os.IsNotExist(err) {
				return nil, fmt.Errorf("reading secret: %w", err)
			}
			// Fallback: try plaintext .json (legacy)
			return s.getPlaintext(plainPath)
		}

		decrypted, err := ageDecrypt(data, s.ageKey)
		if err != nil {
			return nil, fmt.Errorf("decrypting secret: %w", err)
		}

		var fd fileData
		if err := json.Unmarshal(decrypted, &fd); err != nil {
			return nil, fmt.Errorf("parsing decrypted secret: %w", err)
		}
		return []byte(fd.Value), nil
	}

	return s.getPlaintext(plainPath)
}

func (s *fileStore) getPlaintext(path string) ([]byte, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, fmt.Errorf("secret not found")
		}
		return nil, fmt.Errorf("reading secret: %w", err)
	}

	var fd fileData
	if err := json.Unmarshal(data, &fd); err != nil {
		return nil, fmt.Errorf("parsing secret: %w", err)
	}
	return []byte(fd.Value), nil
}

func (s *fileStore) Delete(namespace, name string) error {
	plainPath, encPath := s.getPaths(namespace, name)

	removed := false
	for _, p := range []string{plainPath, encPath} {
		if err := os.Remove(p); err != nil {
			if os.IsNotExist(err) {
				continue
			}
			return fmt.Errorf("deleting secret: %w", err)
		}
		removed = true
	}

	if !removed {
		return fmt.Errorf("secret not found")
	}

	return nil
}

// List returns all secret names in a namespace, from both .json and .json.age files.
func (s *fileStore) List(namespace string) ([]string, error) {
	dir := filepath.Join(s.dir, namespace)
	entries, err := os.ReadDir(dir)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, fmt.Errorf("reading namespace directory: %w", err)
	}

	seen := make(map[string]struct{})
	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}
		name := entry.Name()
		if strings.HasSuffix(name, extEncrypted) {
			seen[strings.TrimSuffix(name, extEncrypted)] = struct{}{}
		} else if strings.HasSuffix(name, extPlaintext) {
			seen[strings.TrimSuffix(name, extPlaintext)] = struct{}{}
		}
	}

	names := make([]string, 0, len(seen))
	for n := range seen {
		names = append(names, n)
	}
	sort.Strings(names)
	return names, nil
}
