//go:build darwin

package secret

import (
	"fmt"
	"os"
	"path/filepath"
)

type Store interface {
	Set(namespace, name string, value []byte) error
	Get(namespace, name string) ([]byte, error)
	Delete(namespace, name string) error
	List(namespace string) ([]string, error)
}

func NewStore() (Store, error) {
	return NewKeychainStore("coffer"), nil
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
