package inject

import (
	"os"
	"strings"
)

type EnvInjector struct{}

func NewEnvInjector() *EnvInjector {
	return &EnvInjector{}
}

func (e *EnvInjector) Inject(secrets map[string]string, command string, args []string) error {
	for name, value := range secrets {
		os.Setenv(name, value)
	}

	return nil
}

func (e *EnvInjector) Cleanup(secrets map[string]string) {
	for name := range secrets {
		os.Unsetenv(name)
	}
}

func (e *EnvInjector) InjectIntoTemplate(template string, secrets map[string]string) string {
	for name, value := range secrets {
		placeholder := "{{coffer:" + name + "}}"
		template = strings.ReplaceAll(template, placeholder, value)
	}
	return template
}
