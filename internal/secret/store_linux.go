//go:build linux

package secret

import (
	"fmt"
	"os"
	"os/exec"
	"strings"
)

type LinuxSecretStore struct {
	serviceName string
}

func NewLinuxSecretStore(serviceName string) *LinuxSecretStore {
	return &LinuxSecretStore{
		serviceName: serviceName,
	}
}

func (l *LinuxSecretStore) Set(namespace, name string, value []byte) error {
	fullName := l.serviceName + "." + namespace + "." + name

	cmd := exec.Command("secret-tool", "store",
		"--label", fullName,
		"service", l.serviceName,
		"name", fullName)

	cmd.Stdin = strings.NewReader(string(value))

	output, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("failed to store secret: %w, output: %s", err, string(output))
	}

	return nil
}

func (l *LinuxSecretStore) Get(namespace, name string) ([]byte, error) {
	fullName := l.serviceName + "." + namespace + "." + name

	cmd := exec.Command("secret-tool", "lookup",
		"service", l.serviceName,
		"name", fullName)

	output, err := cmd.CombinedOutput()
	if err != nil {
		return nil, fmt.Errorf("failed to get secret: %w, output: %s", err, string(output))
	}

	return []byte(strings.TrimSpace(string(output))), nil
}

func (l *LinuxSecretStore) Delete(namespace, name string) error {
	fullName := l.serviceName + "." + namespace + "." + name

	cmd := exec.Command("secret-tool", "clear",
		"service", l.serviceName,
		"name", fullName)

	output, err := cmd.CombinedOutput()
	if err != nil {
		if strings.Contains(string(output), "not found") {
			return nil
		}
		return fmt.Errorf("failed to delete secret: %w, output: %s", err, string(output))
	}

	return nil
}

func NewStore() (Store, error) {
	if os.Getenv("COFFER_USE_SECRET_TOOL") == "true" {
		return NewLinuxSecretStore("coffer"), nil
	}
	return newFileStore()
}

func (l *LinuxSecretStore) List(namespace string) ([]string, error) {
	cmd := exec.Command("secret-tool", "search",
		"service", l.serviceName)

	output, err := cmd.CombinedOutput()
	if err != nil {
		return nil, fmt.Errorf("failed to list secrets: %w, output: %s", err, string(output))
	}

	var secrets []string
	lines := strings.Split(string(output), "\n")

	prefix := l.serviceName + "." + namespace + "."
	for _, line := range lines {
		if strings.Contains(line, prefix) {
			if idx := strings.Index(line, prefix); idx != -1 {
				secretName := line[idx+len(prefix):]
				if secretName != "" {
					secrets = append(secrets, secretName)
				}
			}
		}
	}

	return secrets, nil
}
