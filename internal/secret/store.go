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

// StoreDir returns the path to the coffer store directory (~/.coffer/).
func StoreDir() (string, error) {
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
