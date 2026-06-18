package secret

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
)

type fileStore struct {
	dir string
}

type fileData struct {
	Value string `json:"value"`
}

func newFileStore() (*fileStore, error) {
	dir, err := getStoreDir()
	if err != nil {
		return nil, err
	}

	return &fileStore{dir: dir}, nil
}

func (s *fileStore) getFilePath(namespace, name string) string {
	return filepath.Join(s.dir, namespace, name+".json")
}

func (s *fileStore) Set(namespace, name string, value []byte) error {
	dir := filepath.Join(s.dir, namespace)
	if err := os.MkdirAll(dir, 0700); err != nil {
		return fmt.Errorf("creating namespace directory: %w", err)
	}

	data := &fileData{
		Value: string(value),
	}

	jsonData, err := json.Marshal(data)
	if err != nil {
		return fmt.Errorf("marshaling secret: %w", err)
	}

	path := s.getFilePath(namespace, name)
	if err := os.WriteFile(path, jsonData, 0600); err != nil {
		return fmt.Errorf("writing secret: %w", err)
	}

	return nil
}

func (s *fileStore) Get(namespace, name string) ([]byte, error) {
	path := s.getFilePath(namespace, name)
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, fmt.Errorf("secret not found")
		}
		return nil, fmt.Errorf("reading secret: %w", err)
	}

	var fileData fileData
	if err := json.Unmarshal(data, &fileData); err != nil {
		return nil, fmt.Errorf("unmarshaling secret: %w", err)
	}

	return []byte(fileData.Value), nil
}

func (s *fileStore) Delete(namespace, name string) error {
	path := s.getFilePath(namespace, name)
	if err := os.Remove(path); err != nil {
		if os.IsNotExist(err) {
			return fmt.Errorf("secret not found")
		}
		return fmt.Errorf("deleting secret: %w", err)
	}

	return nil
}

func (s *fileStore) List(namespace string) ([]string, error) {
	dir := filepath.Join(s.dir, namespace)
	if _, err := os.Stat(dir); os.IsNotExist(err) {
		return nil, nil
	}

	entries, err := os.ReadDir(dir)
	if err != nil {
		return nil, fmt.Errorf("reading namespace directory: %w", err)
	}

	var names []string
	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}
		name := entry.Name()
		if filepath.Ext(name) == ".json" {
			names = append(names, name[:len(name)-5])
		}
	}

	sort.Strings(names)
	return names, nil
}
