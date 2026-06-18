package inject

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

type FileInjector struct {
	tempDir string
}

func NewFileInjector() *FileInjector {
	return &FileInjector{}
}

func (f *FileInjector) Inject(secrets map[string]string, command string, args []string) error {
	tempDir, err := os.MkdirTemp("", "coffer-*")
	if err != nil {
		return fmt.Errorf("failed to create temp directory: %w", err)
	}
	f.tempDir = tempDir

	for name, value := range secrets {
		filePath := filepath.Join(tempDir, name)
		if err := os.WriteFile(filePath, []byte(value), 0600); err != nil {
			f.Cleanup(secrets)
			return fmt.Errorf("failed to write secret file: %w", err)
		}
	}

	return nil
}

func (f *FileInjector) Cleanup(secrets map[string]string) {
	if f.tempDir != "" {
		os.RemoveAll(f.tempDir)
		f.tempDir = ""
	}
}

func (f *FileInjector) InjectIntoTemplate(template string, secrets map[string]string) string {
	for name, value := range secrets {
		placeholder := "{{coffer:" + name + "}}"
		template = strings.ReplaceAll(template, placeholder, value)
	}
	return template
}

func (f *FileInjector) GetSecretPath(name string) string {
	return filepath.Join(f.tempDir, name)
}
