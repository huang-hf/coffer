package config

import (
	"os"
	"path/filepath"
	"strings"
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

func TestMerge(t *testing.T) {
	base := &Config{
		DefaultNS: "production",
		Inject:    "env",
		Secrets: map[string]string{
			"db-pwd":  "{{coffer:db-pwd}}",
			"api-key": "{{coffer:api-key}}",
		},
		Namespaces: map[string]*NamespaceConfig{
			"staging": {
				Secrets: map[string]string{
					"db-pwd": "{{coffer:db-pwd}}",
				},
			},
		},
	}
	base.Secrets = make(map[string]string) // re-init after struct literal
	base.Namespaces = make(map[string]*NamespaceConfig)
	base.Secrets["db-pwd"] = "{{coffer:db-pwd}}"
	base.Secrets["api-key"] = "{{coffer:api-key}}"
	base.Namespaces["staging"] = &NamespaceConfig{Secrets: map[string]string{"db-pwd": "{{coffer:db-pwd}}"}}

	override := &Config{
		Secrets: map[string]string{
			"api-key":   "{{coffer:api-key-v2}}", // override
			"new-secret": "{{coffer:new-secret}}", // add
		},
		Namespaces: map[string]*NamespaceConfig{
			"staging": {
				Secrets: map[string]string{
					"api-key": "{{coffer:api-key-staging}}",
				},
			},
			"production": {
				Secrets: map[string]string{
					"db-pwd": "{{coffer:db-pwd-prod}}",
				},
			},
		},
	}

	base.Merge(override)

	// Check merged top-level secrets
	if base.Secrets["db-pwd"] != "{{coffer:db-pwd}}" {
		t.Errorf("db-pwd should stay, got %v", base.Secrets["db-pwd"])
	}
	if base.Secrets["api-key"] != "{{coffer:api-key-v2}}" {
		t.Errorf("api-key should be overridden, got %v", base.Secrets["api-key"])
	}
	if base.Secrets["new-secret"] != "{{coffer:new-secret}}" {
		t.Errorf("new-secret should be added, got %v", base.Secrets["new-secret"])
	}

	// Check merged namespace secrets
	if base.Namespaces["staging"].Secrets["api-key"] != "{{coffer:api-key-staging}}" {
		t.Errorf("staging/api-key should be overridden, got %v", base.Namespaces["staging"].Secrets["api-key"])
	}
	if base.Namespaces["staging"].Secrets["db-pwd"] != "{{coffer:db-pwd}}" {
		t.Errorf("staging/db-pwd should stay from base, got %v", base.Namespaces["staging"].Secrets["db-pwd"])
	}
	if base.Namespaces["production"].Secrets["db-pwd"] != "{{coffer:db-pwd-prod}}" {
		t.Errorf("production/db-pwd should be added from override, got %v", base.Namespaces["production"].Secrets["db-pwd"])
	}
}

func TestLoadChain_LocalOnly(t *testing.T) {
	dir := t.TempDir()
	configPath := filepath.Join(dir, ".coffer")
	t.Setenv("HOME", dir)

	content := `
default_ns: staging
inject: file
secrets:
  db-pwd: "{{coffer:db-pwd}}"
`
	if err := os.WriteFile(configPath, []byte(content), 0644); err != nil {
		t.Fatalf("Failed to write local config: %v", err)
	}

	// No global config at all — should work with just local
	cfg, err := LoadChain(configPath)
	if err != nil {
		t.Fatalf("LoadChain() error = %v", err)
	}

	if cfg.DefaultNS != "staging" {
		t.Errorf("DefaultNS = %v, want staging", cfg.DefaultNS)
	}
	if cfg.Inject != "file" {
		t.Errorf("Inject = %v, want file", cfg.Inject)
	}
	if len(cfg.Secrets) != 1 {
		t.Errorf("Secrets length = %v, want 1", len(cfg.Secrets))
	}
}

func TestGlobalConfigPath(t *testing.T) {
	path := GlobalConfigPath()
	if path == "" {
		t.Fatal("GlobalConfigPath() returned empty")
	}
	if !strings.HasSuffix(path, ".config/coffer/config.yaml") {
		t.Errorf("GlobalConfigPath() = %v, should end with .config/coffer/config.yaml", path)
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
