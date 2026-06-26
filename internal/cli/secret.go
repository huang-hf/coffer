package cli

import (
	"bufio"
	"fmt"
	"io"
	"os"
	"sort"
	"strings"

	"golang.org/x/term"

	"github.com/huang-hf/coffer/internal/config"
	"github.com/huang-hf/coffer/internal/secret"
)

func runSecret(args []string, stdout io.Writer, stderr io.Writer, opts *Options) int {
	if isHelp(args) {
		printSecretUsage(stdout)
		return 0
	}
	if len(args) == 0 {
		printSecretUsage(stderr)
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

func printSecretUsage(w io.Writer) {
	fmt.Fprintln(w, "Usage: coffer secret <add|update|list|delete|get> [name]")
	fmt.Fprintln(w)
	fmt.Fprintln(w, "Commands:")
	fmt.Fprintln(w, "  add <name>       Add a secret")
	fmt.Fprintln(w, "  update <name>    Update a secret")
	fmt.Fprintln(w, "  list             List configured secrets")
	fmt.Fprintln(w, "  delete <name>    Delete a secret")
	fmt.Fprintln(w, "  get <name>       Print a secret value")
	fmt.Fprintln(w)
	fmt.Fprintln(w, "Options:")
	fmt.Fprintln(w, "  --ns=<namespace> Specify namespace")
	fmt.Fprintln(w, "  --global         Use global config")
	fmt.Fprintln(w, "  --json           JSON output where supported")
}

// configPath returns the config file path based on global flag
func configPath(opts *Options) string {
	if opts.Global {
		return config.GlobalConfigPath()
	}
	return ".coffer"
}

func runSecretAdd(args []string, stdout io.Writer, stderr io.Writer, opts *Options) int {
	if isHelp(args) {
		fmt.Fprintln(stdout, "Usage: coffer secret add <name> [--ns=<namespace>] [--global]")
		return 0
	}
	if len(args) != 1 {
		fmt.Fprintln(stderr, "Usage: coffer secret add <name> [--ns=<namespace>] [--global]")
		return 1
	}

	name := args[0]
	if !isValidSecretName(name) {
		fmt.Fprintf(stderr, "Error: invalid secret name '%s'\n", name)
		fmt.Fprintln(stderr, "Secret names must contain only letters, numbers, hyphens, and underscores")
		return 1
	}

	cfgPath := configPath(opts)

	cfg, err := config.Load(cfgPath)
	if err != nil {
		if opts.Global {
			fmt.Fprintln(stderr, "Error: global config not found. Run 'coffer init --global' first")
		} else {
			fmt.Fprintln(stderr, "Error: not initialized. Run 'coffer init' first or use --global")
		}
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
	if err := config.Save(cfg, cfgPath); err != nil {
		fmt.Fprintf(stderr, "Warning: secret saved to keychain but failed to update config: %v\n", err)
	}

	scope := "local"
	if opts.Global {
		scope = "global"
	}
	fmt.Fprintf(stdout, "✓ Secret '%s' saved to %s namespace '%s'\n", name, scope, ns)
	return 0
}

func runSecretUpdate(args []string, stdout io.Writer, stderr io.Writer, opts *Options) int {
	if isHelp(args) {
		fmt.Fprintln(stdout, "Usage: coffer secret update <name> [--ns=<namespace>] [--global]")
		return 0
	}
	if len(args) != 1 {
		fmt.Fprintln(stderr, "Usage: coffer secret update <name> [--ns=<namespace>] [--global]")
		return 1
	}

	name := args[0]
	if !isValidSecretName(name) {
		fmt.Fprintf(stderr, "Error: invalid secret name '%s'\n", name)
		fmt.Fprintln(stderr, "Secret names must contain only letters, numbers, hyphens, and underscores")
		return 1
	}

	cfgPath := configPath(opts)

	cfg, err := config.Load(cfgPath)
	if err != nil {
		if opts.Global {
			fmt.Fprintln(stderr, "Error: global config not found. Run 'coffer init --global' first")
		} else {
			fmt.Fprintln(stderr, "Error: not initialized. Run 'coffer init' first or use --global")
		}
		return 1
	}

	ns := cfg.ResolveNamespace(opts.NS)

	// Check secret exists in config (avoid keychain SIGKILL on macOS)
	nsSecrets := cfg.GetSecretsForNamespace(ns)
	if _, found := nsSecrets[name]; !found {
		fmt.Fprintf(stderr, "Error: secret '%s' not found in namespace '%s'\n", name, ns)
		fmt.Fprintln(stderr, "Use 'coffer secret add' to create a new secret")
		return 1
	}

	store, err := secret.NewStore()
	if err != nil {
		fmt.Fprintf(stderr, "Error creating secret store: %v\n", err)
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

	scope := "local"
	if opts.Global {
		scope = "global"
	}
	fmt.Fprintf(stdout, "✓ Secret '%s' updated in %s namespace '%s'\n", name, scope, ns)
	return 0
}

func runSecretList(args []string, stdout io.Writer, stderr io.Writer, opts *Options) int {
	if isHelp(args) {
		fmt.Fprintln(stdout, "Usage: coffer secret list [--ns=<namespace>] [--global]")
		return 0
	}
	if len(args) > 0 {
		fmt.Fprintln(stderr, "Usage: coffer secret list [--ns=<namespace>] [--global]")
		return 1
	}

	var cfg *config.Config
	var err error
	var ns string

	if opts.Global {
		cfg, err = config.Load(config.GlobalConfigPath())
		if err != nil {
			fmt.Fprintln(stderr, "Error: global config not found. Run 'coffer init --global' first")
			return 1
		}
		ns = cfg.ResolveNamespace(opts.NS)
	} else {
		cfg, err = config.LoadChain(".coffer")
		if err != nil {
			fmt.Fprintln(stderr, "Error: not initialized. Run 'coffer init' first or use --global")
			return 1
		}
		// For listing, ResolveNamespace works on any config
		ns = cfg.ResolveNamespace(opts.NS)
	}

	secrets := cfg.ListSecretsForNamespace(ns)
	sort.Strings(secrets)

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

	scope := "merged"
	if opts.Global {
		scope = "global"
	}
	fmt.Fprintf(stdout, "Secrets in %s namespace '%s':\n", scope, ns)
	for _, name := range secrets {
		fmt.Fprintf(stdout, "  - %s\n", name)
	}

	return 0
}

func runSecretDelete(args []string, stdout io.Writer, stderr io.Writer, opts *Options) int {
	if isHelp(args) {
		fmt.Fprintln(stdout, "Usage: coffer secret delete <name> [--ns=<namespace>] [--global]")
		return 0
	}
	if len(args) != 1 {
		fmt.Fprintln(stderr, "Usage: coffer secret delete <name> [--ns=<namespace>] [--global]")
		return 1
	}

	name := args[0]

	cfgPath := configPath(opts)

	cfg, err := config.Load(cfgPath)
	if err != nil {
		if opts.Global {
			fmt.Fprintln(stderr, "Error: global config not found. Run 'coffer init --global' first")
		} else {
			fmt.Fprintln(stderr, "Error: not initialized. Run 'coffer init' first or use --global")
		}
		return 1
	}

	ns := cfg.ResolveNamespace(opts.NS)

	// Check if secret exists in config
	secretExistsInConfig := false
	for s := range cfg.GetSecretsForNamespace(ns) {
		if s == name {
			secretExistsInConfig = true
			break
		}
	}
	if !secretExistsInConfig {
		fmt.Fprintf(stderr, "Error: secret '%s' not found in namespace '%s' config\n", name, ns)
		return 1
	}

	fmt.Fprintf(stdout, "Remove secret '%s' from namespace '%s'? (y/N): ", name, ns)
	reader := bufio.NewReader(os.Stdin)
	confirm, _ := reader.ReadString('\n')
	confirm = strings.TrimSpace(confirm)
	if confirm != "y" && confirm != "Y" {
		fmt.Fprintln(stdout, "Cancelled")
		return 0
	}

	cfg.DeleteSecretForNamespace(ns, name)
	if err := config.Save(cfg, cfgPath); err != nil {
		fmt.Fprintf(stderr, "Error updating config: %v\n", err)
		return 1
	}

	// Try to delete from store, but don't fail if already gone
	store, err := secret.NewStore()
	if err == nil {
		if err := store.Delete(ns, name); err != nil {
			fmt.Fprintf(stderr, "⚠  Secret not found in store (already deleted or never added)\n")
		}
	}

	scope := "local"
	if opts.Global {
		scope = "global"
	}
	fmt.Fprintf(stdout, "✓ Secret '%s' removed from %s namespace '%s'\n", name, scope, ns)
	return 0
}

func runSecretGet(args []string, stdout io.Writer, stderr io.Writer, opts *Options) int {
	if isHelp(args) {
		fmt.Fprintln(stdout, "Usage: coffer secret get <name> [--ns=<namespace>] [--global]")
		return 0
	}
	if len(args) != 1 {
		fmt.Fprintln(stderr, "Usage: coffer secret get <name> [--ns=<namespace>] [--global]")
		return 1
	}

	if opts.JSON {
		fmt.Fprintln(stderr, "Error: secret get is not allowed in JSON mode (agent mode)")
		fmt.Fprintln(stderr, "This command requires human interaction for security")
		return 1
	}

	name := args[0]

	cfgPath := configPath(opts)

	cfg, err := config.Load(cfgPath)
	if err != nil {
		if opts.Global {
			fmt.Fprintln(stderr, "Error: global config not found. Run 'coffer init --global' first")
		} else {
			fmt.Fprintln(stderr, "Error: not initialized. Run 'coffer init' first or use --global")
		}
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
		fmt.Fprintln(stderr)
		return string(value), nil
	}

	// Not a terminal — read from stdin line
	reader := bufio.NewReader(os.Stdin)
	value, err := reader.ReadString('\n')
	if err != nil && !(err == io.EOF && value != "") {
		return "", err
	}
	return strings.TrimRight(value, "\r\n"), nil
}

func isValidSecretName(name string) bool {
	if len(name) == 0 {
		return false
	}
	for _, c := range name {
		if !((c >= 'a' && c <= 'z') || (c >= 'A' && c <= 'Z') ||
			(c >= '0' && c <= '9') || c == '-' || c == '_') {
			return false
		}
	}
	return true
}
