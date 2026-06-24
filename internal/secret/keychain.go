//go:build darwin

package secret

import (
	"fmt"
	"os/exec"
	"regexp"
	"sort"
	"strings"
)

// KeychainStore implements Store using macOS Keychain
type KeychainStore struct {
	serviceName string
}

// NewKeychainStore creates a new KeychainStore with the given service name
func NewKeychainStore(serviceName string) *KeychainStore {
	return &KeychainStore{
		serviceName: serviceName,
	}
}

// Set stores a secret in macOS Keychain
func (k *KeychainStore) Set(namespace, name string, value []byte) error {
	// No Delete() call needed — the -U flag below handles both create and update.
	cmd := exec.Command("security", "add-generic-password",
		"-a", name,
		"-s", k.serviceName+"."+namespace,
		"-w", string(value),
		"-U")

	output, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("failed to store secret in Keychain: %w, output: %s", err, string(output))
	}

	return nil
}

// Get retrieves a secret from macOS Keychain
func (k *KeychainStore) Get(namespace, name string) ([]byte, error) {
	cmd := exec.Command("security", "find-generic-password",
		"-a", name,
		"-s", k.serviceName+"."+namespace,
		"-w")

	output, err := cmd.CombinedOutput()
	if err != nil {
		return nil, fmt.Errorf("failed to get secret from Keychain: %w, output: %s", err, string(output))
	}

	return []byte(strings.TrimSpace(string(output))), nil
}

func (k *KeychainStore) Delete(namespace, name string) error {
	cmd := exec.Command("security", "delete-generic-password",
		"-a", name,
		"-s", k.serviceName+"."+namespace)

	output, err := cmd.CombinedOutput()
	if err != nil {
		if strings.Contains(string(output), "could not be found") {
			return nil
		}
		return fmt.Errorf("failed to delete secret from Keychain: %w, output: %s", err, string(output))
	}

	return nil
}

// List returns all secret names from macOS Keychain for this service
func (k *KeychainStore) List(namespace string) ([]string, error) {
	cmd := exec.Command("security", "dump-keychain",
		"-a")

	output, err := cmd.CombinedOutput()
	if err != nil {
		return nil, fmt.Errorf("failed to list secrets from Keychain: %w, output: %s", err, string(output))
	}

	acctPattern := regexp.MustCompile(`"acct"<[^>]*>="([^"]*)"`)
	svcePattern := regexp.MustCompile(`"svce"<[^>]*>="([^"]*)"`)
	items := make(map[string]struct{})
	lines := strings.Split(string(output), "\n")
	servicePattern := k.serviceName + "." + namespace

	var acct string
	var svce string
	flush := func() {
		if svce == servicePattern && acct != "" && acct != "<NULL>" {
			items[acct] = struct{}{}
		}
		acct = ""
		svce = ""
	}

	for _, line := range lines {
		if strings.HasPrefix(line, "keychain:") && (acct != "" || svce != "") {
			flush()
		}
		if match := acctPattern.FindStringSubmatch(line); len(match) == 2 {
			acct = match[1]
		}
		if match := svcePattern.FindStringSubmatch(line); len(match) == 2 {
			svce = match[1]
		}
	}
	flush()

	secrets := make([]string, 0, len(items))
	for name := range items {
		secrets = append(secrets, name)
	}
	sort.Strings(secrets)

	return secrets, nil
}
