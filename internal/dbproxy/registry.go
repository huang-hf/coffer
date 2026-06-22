package dbproxy

import (
	"fmt"
	"regexp"

	"coffer/internal/config"
	"coffer/internal/secret"
)

var safeNamePattern = regexp.MustCompile(`^[A-Za-z0-9_-]+$`)

type PublicDBConfig struct {
	Name     string `json:"name"`
	Type     string `json:"type"`
	Host     string `json:"host"`
	Port     int    `json:"port"`
	Database string `json:"database"`
	User     string `json:"user"`
	NS       string `json:"ns"`
}

func AddConfig(cfgPath string, name string, dbCfg *config.DatabaseConfig, password string) error {
	if err := dbCfg.Validate(); err != nil {
		return err
	}
	if !safeNamePattern.MatchString(name) {
		return fmt.Errorf("invalid database name %q", name)
	}

	cfg, err := config.Load(cfgPath)
	if err != nil {
		return fmt.Errorf("loading config: %w", err)
	}
	if _, exists := cfg.Databases[name]; exists {
		return fmt.Errorf("database %q already exists", name)
	}

	store, err := secret.NewStore()
	if err != nil {
		return fmt.Errorf("creating secret store: %w", err)
	}
	secretName := config.DBSecretName(name)
	if err := store.Set(config.DBAuthSecretNS, secretName, []byte(password)); err != nil {
		return fmt.Errorf("saving password to keychain: %w", err)
	}

	cfg.Databases[name] = dbCfg
	if err := config.Save(cfg, cfgPath); err != nil {
		_ = store.Delete(config.DBAuthSecretNS, secretName)
		return fmt.Errorf("saving config: %w", err)
	}
	return nil
}

func RemoveConfig(cfgPath string, name string) error {
	cfg, err := config.Load(cfgPath)
	if err != nil {
		return fmt.Errorf("loading config: %w", err)
	}
	if _, exists := cfg.Databases[name]; !exists {
		return fmt.Errorf("database %q not found", name)
	}

	store, err := secret.NewStore()
	if err != nil {
		return fmt.Errorf("creating secret store: %w", err)
	}
	secretName := config.DBSecretName(name)
	if err := store.Delete(config.DBAuthSecretNS, secretName); err != nil {
		return err
	}

	delete(cfg.Databases, name)
	if err := config.Save(cfg, cfgPath); err != nil {
		return fmt.Errorf("saving config: %w", err)
	}
	return nil
}

func ListConfigs(cfg *config.Config) []PublicDBConfig {
	var list []PublicDBConfig
	for name, db := range cfg.Databases {
		list = append(list, PublicDBConfig{
			Name:     name,
			Type:     db.Type,
			Host:     db.Host,
			Port:     db.Port,
			Database: db.Database,
			User:     db.User,
			NS:       config.DBAuthSecretNS,
		})
	}
	if list == nil {
		return []PublicDBConfig{}
	}
	return list
}

func SecretName(name string) string {
	return config.DBSecretName(name)
}
