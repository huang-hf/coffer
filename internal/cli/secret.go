package cli

import (
	"bufio"
	"fmt"
	"io"
	"os"
	"strings"

	"golang.org/x/term"

	"coffer/internal/config"
	"coffer/internal/secret"
)

func runSecret(args []string, stdout io.Writer, stderr io.Writer, opts *Options) int {
	if len(args) == 0 {
		fmt.Fprintln(stderr, "Usage: coffer secret <add|update|list|delete|get> [name]")
		return 1
	}

	switch args[0] {
	case "add":
		return runSecretAdd(args[1:], stdout, stderr, opts)
	case "update":
		return runSecretUpdate(args[1:], stdout, stderr, opts)
	case "list":
		return runSecretList(args[1:], stdout, stderr, opts)
	case "delete":
		return runSecretDelete(args[1:], stdout, stderr, opts)
	case "get":
		return runSecretGet(args[1:], stdout, stderr, opts)
	default:
		fmt.Fprintf(stderr, "Unknown secret command: %s\n", args[0])
		return 1
	}
}

func runSecretAdd(args []string, stdout io.Writer, stderr io.Writer, opts *Options) int {
	if len(args) != 1 {
		fmt.Fprintln(stderr, "Usage: coffer secret add <name> --ns=<namespace>")
		return 1
	}

	name := args[0]
	if !isValidSecretName(name) {
		fmt.Fprintf(stderr, "Error: invalid secret name '%s'\n", name)
		fmt.Fprintln(stderr, "Secret names must contain only letters, numbers, hyphens, and underscores")
		return 1
	}

	cfg, err := config.Load(".coffer")
	if err != nil {
		fmt.Fprintf(stderr, "Error: not initialized. Run 'coffer init' first\n")
		return 1
	}

	ns := cfg.ResolveNamespace(opts.NS)

	fmt.Fprintf(stdout, "Enter value for %s: ", name)
	value, err := readPassword(stderr)
	if err != nil {
		fmt.Fprintf(stderr, "Error reading password: %v\n", err)
		return 1
	}

	if value == "" {
		fmt.Fprintln(stderr, "Error: value cannot be empty")
		return 1
	}

	store, err := secret.NewStore()
	if err != nil {
		fmt.Fprintf(stderr, "Error creating secret store: %v\n", err)
		return 1
	}

	if err := store.Set(ns, name, []byte(value)); err != nil {
		fmt.Fprintf(stderr, "Error saving secret: %v\n", err)
		return 1
	}

	cfg.SetSecretForNamespace(ns, name, "")
	if err := config.Save(cfg, ".coffer"); err != nil {
		fmt.Fprintf(stderr, "Warning: secret saved to keychain but failed to update config: %v\n", err)
	}

	fmt.Fprintf(stdout, "✓ Secret '%s' saved to namespace '%s'\n", name, ns)
	return 0
}

func runSecretUpdate(args []string, stdout io.Writer, stderr io.Writer, opts *Options) int {
	if len(args) != 1 {
		fmt.Fprintln(stderr, "Usage: coffer secret update <name> --ns=<namespace>")
		return 1
	}

	name := args[0]
	if !isValidSecretName(name) {
		fmt.Fprintf(stderr, "Error: invalid secret name '%s'\n", name)
		fmt.Fprintln(stderr, "Secret names must contain only letters, numbers, hyphens, and underscores")
		return 1
	}

	cfg, err := config.Load(".coffer")
	if err != nil {
		fmt.Fprintf(stderr, "Error: not initialized. Run 'coffer init' first\n")
		return 1
	}

	ns := cfg.ResolveNamespace(opts.NS)

	store, err := secret.NewStore()
	if err != nil {
		fmt.Fprintf(stderr, "Error creating secret store: %v\n", err)
		return 1
	}

	nsSecrets := cfg.GetSecretsForNamespace(ns)
	if _, found := nsSecrets[name]; !found {
		fmt.Fprintf(stderr, "Error: secret '%s' not found in namespace '%s'\n", name, ns)
		fmt.Fprintln(stderr, "Use 'coffer secret add' to create a new secret")
		return 1
	}

	fmt.Fprintf(stdout, "Enter new value for %s: ", name)
	value, err := readPassword(stderr)
	if err != nil {
		fmt.Fprintf(stderr, "Error reading password: %v\n", err)
		return 1
	}

	if value == "" {
		fmt.Fprintln(stderr, "Error: value cannot be empty")
		return 1
	}

	if err := store.Set(ns, name, []byte(value)); err != nil {
		fmt.Fprintf(stderr, "Error updating secret: %v\n", err)
		return 1
	}

	fmt.Fprintf(stdout, "✓ Secret '%s' updated in namespace '%s'\n", name, ns)
	return 0
}

func runSecretList(args []string, stdout io.Writer, stderr io.Writer, opts *Options) int {
	if len(args) > 0 {
		fmt.Fprintln(stderr, "Usage: coffer secret list [--ns=<namespace>]")
		return 1
	}

	cfg, err := config.Load(".coffer")
	if err != nil {
		fmt.Fprintf(stderr, "Error: not initialized. Run 'coffer init' first\n")
		return 1
	}

	ns := cfg.ResolveNamespace(opts.NS)

	store, err := secret.NewStore()
	if err != nil {
		fmt.Fprintf(stderr, "Error creating secret store: %v\n", err)
		return 1
	}

	secrets, err := store.List(ns)
	if err != nil {
		fmt.Fprintf(stderr, "Error listing secrets: %v\n", err)
		return 1
	}

	if opts.JSON {
		output := map[string]interface{}{
			"namespace": ns,
			"secrets":   secrets,
		}
		printJSON(stdout, output)
		return 0
	}

	if len(secrets) == 0 {
		fmt.Fprintf(stdout, "No secrets found in namespace '%s'\n", ns)
		return 0
	}

	fmt.Fprintf(stdout, "Secrets in namespace '%s':\n", ns)
	for _, name := range secrets {
		fmt.Fprintf(stdout, "  - %s\n", name)
	}

	return 0
}

func runSecretDelete(args []string, stdout io.Writer, stderr io.Writer, opts *Options) int {
	if len(args) != 1 {
		fmt.Fprintln(stderr, "Usage: coffer secret delete <name> --ns=<namespace>")
		return 1
	}

	name := args[0]

	cfg, err := config.Load(".coffer")
	if err != nil {
		fmt.Fprintf(stderr, "Error: not initialized. Run 'coffer init' first\n")
		return 1
	}

	ns := cfg.ResolveNamespace(opts.NS)

	store, err := secret.NewStore()
	if err != nil {
		fmt.Fprintf(stderr, "Error creating secret store: %v\n", err)
		return 1
	}

	fmt.Fprintf(stdout, "Delete secret '%s' from namespace '%s'? (y/N): ", name, ns)
	reader := bufio.NewReader(os.Stdin)
	confirm, _ := reader.ReadString('\n')
	confirm = strings.TrimSpace(confirm)
	if confirm != "y" && confirm != "Y" {
		fmt.Fprintln(stdout, "Cancelled")
		return 0
	}

	if err := store.Delete(ns, name); err != nil {
		fmt.Fprintf(stderr, "Error deleting secret: %v\n", err)
		return 1
	}

	cfg.DeleteSecretForNamespace(ns, name)
	if err := config.Save(cfg, ".coffer"); err != nil {
		fmt.Fprintf(stderr, "Warning: secret deleted from keychain but failed to update config: %v\n", err)
	}

	fmt.Fprintf(stdout, "✓ Secret '%s' deleted from namespace '%s'\n", name, ns)
	return 0
}

func runSecretGet(args []string, stdout io.Writer, stderr io.Writer, opts *Options) int {
	if len(args) != 1 {
		fmt.Fprintln(stderr, "Usage: coffer secret get <name> --ns=<namespace>")
		return 1
	}

	if opts.JSON {
		fmt.Fprintln(stderr, "Error: secret get is not allowed in JSON mode (agent mode)")
		fmt.Fprintln(stderr, "This command requires human interaction for security")
		return 1
	}

	name := args[0]

	cfg, err := config.Load(".coffer")
	if err != nil {
		fmt.Fprintf(stderr, "Error: not initialized. Run 'coffer init' first\n")
		return 1
	}

	ns := cfg.ResolveNamespace(opts.NS)

	store, err := secret.NewStore()
	if err != nil {
		fmt.Fprintf(stderr, "Error creating secret store: %v\n", err)
		return 1
	}

	value, err := store.Get(ns, name)
	if err != nil {
		fmt.Fprintf(stderr, "Error getting secret: %v\n", err)
		return 1
	}

	fmt.Fprintf(stdout, "%s\n", string(value))
	return 0
}

func readPassword(stderr io.Writer) (string, error) {
	if term.IsTerminal(int(os.Stdin.Fd())) {
		value, err := term.ReadPassword(int(os.Stdin.Fd()))
		if err != nil {
			return "", err
		}
		fmt.Fprintln(stderr, "") // newline after hidden input
		return strings.TrimSpace(string(value)), nil
	}
	// fallback for non-terminal (piped input)
	var value string
	_, err := fmt.Scanln(&value)
	if err != nil {
		return "", err
	}
	return strings.TrimSpace(value), nil
}

func isValidSecretName(name string) bool {
	if name == "" {
		return false
	}
	for _, c := range name {
		if !((c >= 'a' && c <= 'z') || (c >= 'A' && c <= 'Z') || (c >= '0' && c <= '9') || c == '-' || c == '_') {
			return false
		}
	}
	return true
}
