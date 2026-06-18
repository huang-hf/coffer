package inject

import (
	"os"
	"path/filepath"
	"testing"
)

func TestEnvInjector_Inject(t *testing.T) {
	injector := NewEnvInjector()
	secrets := map[string]string{
		"DB_PWD":   "test-password",
		"API_KEY":  "test-api-key",
	}

	if err := injector.Inject(secrets, "echo", nil); err != nil {
		t.Fatalf("Inject() error = %v", err)
	}

	for name, expected := range secrets {
		if value := os.Getenv(name); value != expected {
			t.Errorf("Inject() env %s = %v, want %v", name, value, expected)
		}
	}

	injector.Cleanup(secrets)

	for name := range secrets {
		if value := os.Getenv(name); value != "" {
			t.Errorf("Cleanup() env %s = %v, want empty", name, value)
		}
	}
}

func TestEnvInjector_InjectIntoTemplate(t *testing.T) {
	injector := NewEnvInjector()
	secrets := map[string]string{
		"DB_PWD":  "test-password",
		"API_KEY": "test-api-key",
	}

	template := "database={{coffer:DB_PWD}}&key={{coffer:API_KEY}}"
	result := injector.InjectIntoTemplate(template, secrets)

	expected := "database=test-password&key=test-api-key"
	if result != expected {
		t.Errorf("InjectIntoTemplate() = %v, want %v", result, expected)
	}
}

func TestFileInjector_Inject(t *testing.T) {
	injector := NewFileInjector()
	secrets := map[string]string{
		"DB_PWD":  "test-password",
		"API_KEY": "test-api-key",
	}

	if err := injector.Inject(secrets, "echo", nil); err != nil {
		t.Fatalf("Inject() error = %v", err)
	}

	for name, expected := range secrets {
		path := injector.GetSecretPath(name)
		data, err := os.ReadFile(path)
		if err != nil {
			t.Errorf("Inject() failed to read secret file %s: %v", path, err)
			continue
		}

		if string(data) != expected {
			t.Errorf("Inject() secret %s = %v, want %v", name, string(data), expected)
		}
	}

	injector.Cleanup(secrets)

	for name := range secrets {
		path := injector.GetSecretPath(name)
		if _, err := os.Stat(path); !os.IsNotExist(err) {
			t.Errorf("Cleanup() secret file %s still exists", path)
		}
	}
}

func TestFileInjector_InjectIntoTemplate(t *testing.T) {
	injector := NewFileInjector()
	secrets := map[string]string{
		"DB_PWD":  "test-password",
		"API_KEY": "test-api-key",
	}

	if err := injector.Inject(secrets, "echo", nil); err != nil {
		t.Fatalf("Inject() error = %v", err)
	}
	defer injector.Cleanup(secrets)

	template := "database={{coffer:DB_PWD}}&key={{coffer:API_KEY}}"
	result := injector.InjectIntoTemplate(template, secrets)

	expected := "database=test-password&key=test-api-key"
	if result != expected {
		t.Errorf("InjectIntoTemplate() = %v, want %v", result, expected)
	}
}

func TestNewInjector(t *testing.T) {
	tests := []struct {
		name     string
		mode     string
		expected string
	}{
		{
			name:     "env mode",
			mode:     "env",
			expected: "*inject.EnvInjector",
		},
		{
			name:     "file mode",
			mode:     "file",
			expected: "*inject.FileInjector",
		},
		{
			name:     "default mode",
			mode:     "",
			expected: "*inject.EnvInjector",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			injector := NewInjector(tt.mode)
			if injector == nil {
				t.Fatalf("NewInjector() returned nil")
			}

			actualType := "*inject.EnvInjector"
			switch injector.(type) {
			case *FileInjector:
				actualType = "*inject.FileInjector"
			}

			if actualType != tt.expected {
				t.Errorf("NewInjector() type = %v, want %v", actualType, tt.expected)
			}
		})
	}
}

func TestFileInjector_TempDir(t *testing.T) {
	injector := NewFileInjector()
	secrets := map[string]string{
		"TEST_SECRET": "test-value",
	}

	if err := injector.Inject(secrets, "echo", nil); err != nil {
		t.Fatalf("Inject() error = %v", err)
	}
	defer injector.Cleanup(secrets)

	tempDir := injector.GetSecretPath("TEST_SECRET")
	if tempDir == "" {
		t.Error("GetSecretPath() returned empty string")
	}

	dir := filepath.Dir(tempDir)
	if _, err := os.Stat(dir); os.IsNotExist(err) {
		t.Errorf("Temp directory %s does not exist", dir)
	}
}
