//go:build windows

package secret

import (
	"fmt"
	"os"
	"os/exec"
	"strings"
)

func NewStore() (Store, error) {
	if os.Getenv("COFFER_USE_CMDKEY") == "true" {
		return NewWindowsSecretStore("coffer"), nil
	}
	return newFileStore()
}

type WindowsSecretStore struct {
	serviceName string
}

func NewWindowsSecretStore(serviceName string) *WindowsSecretStore {
	return &WindowsSecretStore{
		serviceName: serviceName,
	}
}

func (w *WindowsSecretStore) Set(namespace, name string, value []byte) error {
	fullName := w.serviceName + "." + namespace + "." + name

	cmd := exec.Command("cmdkey", "/generic:"+fullName, "/user:coffer", "/pass:"+string(value))

	output, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("failed to store secret: %w, output: %s", err, string(output))
	}

	return nil
}

func (w *WindowsSecretStore) Get(namespace, name string) ([]byte, error) {
	fullName := w.serviceName + "." + namespace + "." + name

	cmd := exec.Command("cmdkey", "/list:"+fullName)

	output, err := cmd.CombinedOutput()
	if err != nil {
		return nil, fmt.Errorf("failed to get secret: %w, output: %s", err, string(output))
	}

	lines := strings.Split(string(output), "\n")
	for _, line := range lines {
		if strings.Contains(line, "Password:") {
			parts := strings.SplitN(line, ":", 2)
			if len(parts) == 2 {
				return []byte(strings.TrimSpace(parts[1])), nil
			}
		}
	}

	return nil, fmt.Errorf("secret not found")
}

func (w *WindowsSecretStore) Delete(namespace, name string) error {
	fullName := w.serviceName + "." + namespace + "." + name

	cmd := exec.Command("cmdkey", "/delete:"+fullName)

	output, err := cmd.CombinedOutput()
	if err != nil {
		if strings.Contains(string(output), "not found") {
			return nil
		}
		return fmt.Errorf("failed to delete secret: %w, output: %s", err, string(output))
	}

	return nil
}

func (w *WindowsSecretStore) List(namespace string) ([]string, error) {
	cmd := exec.Command("cmdkey", "/list")

	output, err := cmd.CombinedOutput()
	if err != nil {
		return nil, fmt.Errorf("failed to list secrets: %w, output: %s", err, string(output))
	}

	var secrets []string
	lines := strings.Split(string(output), "\n")

	prefix := w.serviceName + "." + namespace + "."
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
