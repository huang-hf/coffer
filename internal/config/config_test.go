package config

import (
	"os"
	"path/filepath"
	"testing"
)

func TestLoad(t *testing.T) {
	tempDir := t.TempDir()
	configPath := filepath.Join(tempDir, ".coffer")

	content := `
default_ns: production
inject: env
config: config.yaml
secrets:
  db-pwd: "{{coffer:db-pwd}}"
  api-key: "{{coffer:api-key}}"
namespaces:
  staging:
    secrets:
      db-pwd: "{{coffer:db-pwd}}"
`
	if err := os.WriteFile(configPath, []byte(content), 0644); err != nil {
		t.Fatalf("Failed to write config file: %v", err)
	}

	cfg, err := Load(configPath)
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}

	if cfg.DefaultNS != "production" {
		t.Errorf("DefaultNS = %v, want %v", cfg.DefaultNS, "production")
	}

	if cfg.Inject != "env" {
		t.Errorf("Inject = %v, want %v", cfg.Inject, "env")
	}

	if cfg.Config != "config.yaml" {
		t.Errorf("Config = %v, want %v", cfg.Config, "config.yaml")
	}

	if len(cfg.Secrets) != 2 {
		t.Errorf("Secrets length = %v, want %v", len(cfg.Secrets), 2)
	}

	if len(cfg.Namespaces) != 1 {
		t.Errorf("Namespaces length = %v, want %v", len(cfg.Namespaces), 1)
	}
}

func TestSave(t *testing.T) {
	tempDir := t.TempDir()
	configPath := filepath.Join(tempDir, ".coffer")

	cfg := &Config{
		DefaultNS: "production",
		Inject:    "env",
		Config:    "config.yaml",
		Secrets: map[string]string{
			"db-pwd": "{{coffer:db-pwd}}",
		},
		Namespaces: map[string]*NamespaceConfig{
			"staging": {
				Secrets: map[string]string{
					"db-pwd": "{{coffer:db-pwd}}",
				},
			},
		},
	}

	if err := Save(cfg, configPath); err != nil {
		t.Fatalf("Save() error = %v", err)
	}

	loadedCfg, err := Load(configPath)
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}

	if loadedCfg.DefaultNS != cfg.DefaultNS {
		t.Errorf("DefaultNS = %v, want %v", loadedCfg.DefaultNS, cfg.DefaultNS)
	}
}

func TestResolveNamespace(t *testing.T) {
	cfg := &Config{
		DefaultNS: "production",
	}

	tests := []struct {
		name     string
		cliNS    string
		envNS    string
		expected string
	}{
		{
			name:     "CLI namespace takes priority",
			cliNS:    "staging",
			envNS:    "development",
			expected: "staging",
		},
		{
			name:     "Environment variable used when no CLI",
			cliNS:    "",
			envNS:    "development",
			expected: "development",
		},
		{
			name:     "Default namespace used when no CLI or env",
			cliNS:    "",
			envNS:    "",
			expected: "production",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if tt.envNS != "" {
				os.Setenv(EnvNamespace, tt.envNS)
				defer os.Unsetenv(EnvNamespace)
			}

			result := cfg.ResolveNamespace(tt.cliNS)
			if result != tt.expected {
				t.Errorf("ResolveNamespace() = %v, want %v", result, tt.expected)
			}
		})
	}
}

func TestGetSecretsForNamespace(t *testing.T) {
	cfg := &Config{
		Secrets: map[string]string{
			"db-pwd": "{{coffer:db-pwd}}",
		},
		Namespaces: map[string]*NamespaceConfig{
			"staging": {
				Secrets: map[string]string{
					"api-key": "{{coffer:api-key}}",
				},
			},
		},
	}

	tests := []struct {
		name     string
		ns       string
		expected int
	}{
		{
			name:     "Default namespace",
			ns:       "",
			expected: 1,
		},
		{
			name:     "Staging namespace",
			ns:       "staging",
			expected: 1,
		},
		{
			name:     "Non-existent namespace",
			ns:       "production",
			expected: 0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			secrets := cfg.GetSecretsForNamespace(tt.ns)
			if len(secrets) != tt.expected {
				t.Errorf("GetSecretsForNamespace() length = %v, want %v", len(secrets), tt.expected)
			}
		})
	}
}

func TestSetSecretForNamespace(t *testing.T) {
	cfg := &Config{
		Secrets: make(map[string]string),
		Namespaces: make(map[string]*NamespaceConfig),
	}

	cfg.SetSecretForNamespace("", "db-pwd", "{{coffer:db-pwd}}")
	if _, ok := cfg.Secrets["db-pwd"]; !ok {
		t.Error("SetSecretForNamespace() did not set secret in default namespace")
	}

	cfg.SetSecretForNamespace("staging", "api-key", "{{coffer:api-key}}")
	if _, ok := cfg.Namespaces["staging"].Secrets["api-key"]; !ok {
		t.Error("SetSecretForNamespace() did not set secret in staging namespace")
	}
}

func TestDeleteSecretForNamespace(t *testing.T) {
	cfg := &Config{
		Secrets: map[string]string{
			"db-pwd": "{{coffer:db-pwd}}",
		},
		Namespaces: map[string]*NamespaceConfig{
			"staging": {
				Secrets: map[string]string{
					"api-key": "{{coffer:api-key}}",
				},
			},
		},
	}

	if !cfg.DeleteSecretForNamespace("", "db-pwd") {
		t.Error("DeleteSecretForNamespace() returned false for existing secret")
	}

	if _, ok := cfg.Secrets["db-pwd"]; ok {
		t.Error("DeleteSecretForNamespace() did not delete secret from default namespace")
	}

	if !cfg.DeleteSecretForNamespace("staging", "api-key") {
		t.Error("DeleteSecretForNamespace() returned false for existing secret in staging")
	}

	if cfg.DeleteSecretForNamespace("staging", "nonexistent") {
		t.Error("DeleteSecretForNamespace() returned true for non-existent secret")
	}
}

func TestListSecretsForNamespace(t *testing.T) {
	cfg := &Config{
		Secrets: map[string]string{
			"db-pwd":   "{{coffer:db-pwd}}",
			"api-key":  "{{coffer:api-key}}",
		},
		Namespaces: map[string]*NamespaceConfig{
			"staging": {
				Secrets: map[string]string{
					"staging-key": "{{coffer:staging-key}}",
				},
			},
		},
	}

	defaultSecrets := cfg.ListSecretsForNamespace("")
	if len(defaultSecrets) != 2 {
		t.Errorf("ListSecretsForNamespace() default length = %v, want %v", len(defaultSecrets), 2)
	}

	stagingSecrets := cfg.ListSecretsForNamespace("staging")
	if len(stagingSecrets) != 1 {
		t.Errorf("ListSecretsForNamespace() staging length = %v, want %v", len(stagingSecrets), 1)
	}

	prodSecrets := cfg.ListSecretsForNamespace("production")
	if len(prodSecrets) != 0 {
		t.Errorf("ListSecretsForNamespace() production length = %v, want %v", len(prodSecrets), 0)
	}
}
