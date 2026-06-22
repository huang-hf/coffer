package config

import (
	"fmt"
	"os"
	"path/filepath"

	"gopkg.in/yaml.v3"
)

const (
	EnvNamespace     = "COFFER_NS"
	EnvConfig        = "COFFER_CONFIG"
	globalConfigDir  = ".config/coffer"
	globalConfigFile = "config.yaml"

	// DBAuthSecretNS is the keychain namespace used for database passwords.
	DBAuthSecretNS = "_db"
)

// DBSecretName returns the keychain account name for a database capability's password.
func DBSecretName(dbName string) string {
	return "db:" + dbName
}

func (d *DatabaseConfig) Validate() error {
	switch {
	case d.Type == "":
		return fmt.Errorf("database type is required")
	case d.Type != DBTypePostgres:
		return fmt.Errorf("unsupported database type %q (only %q is supported)", d.Type, DBTypePostgres)
	case d.Host == "":
		return fmt.Errorf("database host is required")
	case d.Port <= 0 || d.Port > 65535:
		return fmt.Errorf("database port must be between 1 and 65535")
	case d.User == "":
		return fmt.Errorf("database user is required")
	case d.Database == "":
		return fmt.Errorf("database name is required")
	default:
		return nil
	}
}

type Config struct {
	DefaultNS  string                       `yaml:"default_ns"`
	Inject     string                       `yaml:"inject"`
	Config     string                       `yaml:"config"`
	Secrets    map[string]string            `yaml:"secrets"`
	Namespaces map[string]*NamespaceConfig  `yaml:"namespaces,omitempty"`
	Databases  map[string]*DatabaseConfig   `yaml:"databases,omitempty"`
}

type NamespaceConfig struct {
	Secrets map[string]string `yaml:"secrets"`
}

type DatabaseConfig struct {
	Type     string `yaml:"type"`
	Host     string `yaml:"host"`
	Port     int    `yaml:"port"`
	User     string `yaml:"user"`
	Database string `yaml:"database"`
}

const (
	DBTypePostgres = "postgres"
)

func Load(path string) (*Config, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}

	cfg := &Config{}
	if err := yaml.Unmarshal(data, cfg); err != nil {
		return nil, err
	}

	if cfg.Secrets == nil {
		cfg.Secrets = make(map[string]string)
	}

	if cfg.Namespaces == nil {
		cfg.Namespaces = make(map[string]*NamespaceConfig)
	}

	if cfg.Databases == nil {
		cfg.Databases = make(map[string]*DatabaseConfig)
	}

	return cfg, nil
}

func Save(cfg *Config, path string) error {
	data, err := yaml.Marshal(cfg)
	if err != nil {
		return err
	}

	return os.WriteFile(path, data, 0644)
}

// GlobalConfigPath returns path to global config (~/.config/coffer/config.yaml)
func GlobalConfigPath() string {
	home, err := os.UserHomeDir()
	if err != nil {
		return ""
	}
	return filepath.Join(home, globalConfigDir, globalConfigFile)
}

// LoadChain loads local config and falls back to global for secrets.
// If both exist: global secrets as base, local overrides/adds.
// Settings (default_ns, inject, config) come from local only.
func LoadChain(localPath string) (*Config, error) {
	// Try local first
	localCfg, localErr := Load(localPath)
	if localErr == nil {
		// Local exists — merge global secrets underneath
		globalCfg, gErr := Load(GlobalConfigPath())
		if gErr == nil {
			merged := &Config{
				DefaultNS:  localCfg.DefaultNS,
				Inject:     localCfg.Inject,
				Config:     localCfg.Config,
				Secrets:    make(map[string]string),
				Namespaces: make(map[string]*NamespaceConfig),
			}
			merged.Merge(globalCfg) // global secrets as base
			merged.Merge(localCfg)  // local overrides/adds
			return merged, nil
		}
		// No global config — just return local
		return localCfg, nil
	}

	// No local — try global
	globalCfg, gErr := Load(GlobalConfigPath())
	if gErr == nil {
		return globalCfg, nil
	}

	// Neither exists — return the original error
	return nil, localErr
}

// Merge merges another config's secrets, namespaces, and databases into this one.
// other's values override/add to this config's values.
func (c *Config) Merge(other *Config) {
	for k, v := range other.Secrets {
		c.Secrets[k] = v
	}
	for ns, nsConfig := range other.Namespaces {
		if _, ok := c.Namespaces[ns]; !ok {
			c.Namespaces[ns] = &NamespaceConfig{
				Secrets: make(map[string]string),
			}
		}
		for k, v := range nsConfig.Secrets {
			c.Namespaces[ns].Secrets[k] = v
		}
	}
	for name, db := range other.Databases {
		c.Databases[name] = db
	}
}

func (c *Config) ResolveNamespace(cliNS string) string {
	if cliNS != "" {
		return cliNS
	}

	if envNS := os.Getenv(EnvNamespace); envNS != "" {
		return envNS
	}

	return c.DefaultNS
}

func (c *Config) GetSecretsForNamespace(ns string) map[string]string {
	if ns == "" || ns == "default" {
		return c.Secrets
	}

	if nsConfig, ok := c.Namespaces[ns]; ok {
		return nsConfig.Secrets
	}

	return make(map[string]string)
}

func (c *Config) SetSecretForNamespace(ns, name, value string) {
	if ns == "" || ns == "default" {
		c.Secrets[name] = value
		return
	}

	if c.Namespaces[ns] == nil {
		c.Namespaces[ns] = &NamespaceConfig{
			Secrets: make(map[string]string),
		}
	}

	c.Namespaces[ns].Secrets[name] = value
}

func (c *Config) DeleteSecretForNamespace(ns, name string) bool {
	if ns == "" || ns == "default" {
		if _, ok := c.Secrets[name]; ok {
			delete(c.Secrets, name)
			return true
		}
		return false
	}

	if nsConfig, ok := c.Namespaces[ns]; ok {
		if _, ok := nsConfig.Secrets[name]; ok {
			delete(nsConfig.Secrets, name)
			return true
		}
	}

	return false
}

func (c *Config) ListSecretsForNamespace(ns string) []string {
	var secrets []string

	if ns == "" || ns == "default" {
		for name := range c.Secrets {
			secrets = append(secrets, name)
		}
		return secrets
	}

	if nsConfig, ok := c.Namespaces[ns]; ok {
		for name := range nsConfig.Secrets {
			secrets = append(secrets, name)
		}
	}

	return secrets
}
