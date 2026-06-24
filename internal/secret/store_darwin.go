//go:build darwin

package secret

import (
	"fmt"
	"os"
	"path/filepath"
)

// Store is the interface for secret storage backends.
type Store interface {
	Set(namespace, name string, value []byte) error
	Get(namespace, name string) ([]byte, error)
	Delete(namespace, name string) error
	List(namespace string) ([]string, error)
}

// NewStore creates a secret store for macOS.
//
// By default it uses the file-based store (no keychain dialog).
// To use the system Keychain instead, set COFFER_USE_KEYCHAIN=true.
func NewStore() (Store, error) {
	if os.Getenv("COFFER_USE_KEYCHAIN") == "true" {
		return NewKeychainStore("coffer"), nil
	}
	return newFileStore()
}

func getStoreDir() (string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return "", fmt.Errorf("getting home directory: %w", err)
	}

	dir := filepath.Join(home, ".coffer")
	if err := os.MkdirAll(dir, 0700); err != nil {
		return "", fmt.Errorf("creating coffer directory: %w", err)
	}

	return dir, nil
}
