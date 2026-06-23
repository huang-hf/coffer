package inject

import (
	"os"

	"github.com/huang-hf/coffer/internal/config"
	"github.com/huang-hf/coffer/internal/secret"
)

type Injector interface {
	Inject(secrets map[string]string, command string, args []string) error
	Cleanup(secrets map[string]string)
	InjectIntoTemplate(template string, secrets map[string]string) string
}

func NewInjector(mode string) Injector {
	switch mode {
	case "file":
		return NewFileInjector()
	default:
		return NewEnvInjector()
	}
}

func RenderConfigFile(cfg *config.Config, store secret.Store, ns string) (string, error) {
	data, err := os.ReadFile(cfg.Config)
	if err != nil {
		return "", err
	}

	template := string(data)
	secrets := cfg.GetSecretsForNamespace(ns)
	resolvedSecrets := make(map[string]string)

	for name := range secrets {
		value, err := store.Get(ns, name)
		if err != nil {
			return "", err
		}
		resolvedSecrets[name] = string(value)
	}

	injector := NewInjector(cfg.Inject)
	result := injector.InjectIntoTemplate(template, resolvedSecrets)

	tempFile, err := os.CreateTemp("", "coffer-config-*")
	if err != nil {
		return "", err
	}

	if _, err := tempFile.WriteString(result); err != nil {
		tempFile.Close()
		os.Remove(tempFile.Name())
		return "", err
	}

	if err := tempFile.Close(); err != nil {
		os.Remove(tempFile.Name())
		return "", err
	}

	return tempFile.Name(), nil
}
