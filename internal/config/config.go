package config

import (
	"os"

	"gopkg.in/yaml.v3"
)

const (
	EnvNamespace = "COFFER_NS"
)

type Config struct {
	DefaultNS string            `yaml:"default_ns"`
	Inject    string            `yaml:"inject"`
	Config    string            `yaml:"config"`
	Secrets   map[string]string `yaml:"secrets"`
	Namespaces map[string]*NamespaceConfig `yaml:"namespaces,omitempty"`
}

type NamespaceConfig struct {
	Secrets map[string]string `yaml:"secrets"`
}

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

	return cfg, nil
}

func Save(cfg *Config, path string) error {
	data, err := yaml.Marshal(cfg)
	if err != nil {
		return err
	}

	return os.WriteFile(path, data, 0644)
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
