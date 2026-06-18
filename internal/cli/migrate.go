package cli

import (
	"bufio"
	"fmt"
	"io"
	"os"
	"regexp"
	"strings"

	"coffer/internal/config"
	"coffer/internal/secret"
)

var sensitivePatterns = []*regexp.Regexp{
	regexp.MustCompile(`(?i)PASSWORD`),
	regexp.MustCompile(`(?i)SECRET`),
	regexp.MustCompile(`(?i)KEY`),
	regexp.MustCompile(`(?i)TOKEN`),
	regexp.MustCompile(`(?i)CREDENTIAL`),
	regexp.MustCompile(`(?i)AUTH`),
	regexp.MustCompile(`(?i)PRIVATE`),
	regexp.MustCompile(`(?i)^AWS_`),
	regexp.MustCompile(`(?i)^GCP_`),
	regexp.MustCompile(`(?i)^AZURE_`),
}

var nonSensitivePatterns = []*regexp.Regexp{
	regexp.MustCompile(`(?i)HOST`),
	regexp.MustCompile(`(?i)PORT`),
	regexp.MustCompile(`(?i)URL`),
	regexp.MustCompile(`(?i)NAME`),
	regexp.MustCompile(`(?i)TIMEOUT`),
	regexp.MustCompile(`(?i)RETRY`),
	regexp.MustCompile(`(?i)DEBUG`),
	regexp.MustCompile(`(?i)MODE`),
	regexp.MustCompile(`(?i)LOG`),
}

type migrateOptions struct {
	envFile     string
	template    string
	namespace   string
	dryRun      bool
	force       bool
}

type envEntry struct {
	key       string
	value     string
	isComment bool
 isEmpty   bool
	original  string
}

func runMigrate(args []string, stdout io.Writer, stderr io.Writer, opts *Options) int {
	if len(args) == 0 {
		fmt.Fprintln(stderr, "Usage: coffer migrate <env-file> [options]")
		fmt.Fprintln(stderr, "")
		fmt.Fprintln(stderr, "Options:")
		fmt.Fprintln(stderr, "  --template=<path>    Target template file (default: .env.template)")
		fmt.Fprintln(stderr, "  --namespace=<name>   Target namespace (default: 'default')")
		fmt.Fprintln(stderr, "  --dry-run            Only generate template, don't store secrets")
		fmt.Fprintln(stderr, "  --force              Skip confirmation")
		return 1
	}

	mOpts := &migrateOptions{
		envFile:   args[0],
		template:  ".env.template",
		namespace: "default",
	}

	// Parse flags
	for _, arg := range args[1:] {
		switch {
		case strings.HasPrefix(arg, "--template="):
			mOpts.template = strings.TrimPrefix(arg, "--template=")
		case strings.HasPrefix(arg, "--namespace="):
			mOpts.namespace = strings.TrimPrefix(arg, "--namespace=")
		case arg == "--dry-run":
			mOpts.dryRun = true
		case arg == "--force":
			mOpts.force = true
		}
	}

	// Check if .env file exists
	if _, err := os.Stat(mOpts.envFile); os.IsNotExist(err) {
		fmt.Fprintf(stderr, "Error: file not found: %s\n", mOpts.envFile)
		return 1
	}

	// Read .env file
	entries, err := parseEnvFile(mOpts.envFile)
	if err != nil {
		fmt.Fprintf(stderr, "Error reading .env file: %v\n", err)
		return 1
	}

	// Analyze entries
	var sensitive []envEntry
	var nonSensitive []envEntry

	for _, entry := range entries {
		if entry.isComment || entry.isEmpty {
			nonSensitive = append(nonSensitive, entry)
			continue
		}

		if isSensitiveKey(entry.key) {
			sensitive = append(sensitive, entry)
		} else {
			nonSensitive = append(nonSensitive, entry)
		}
	}

	// Display analysis
	fmt.Fprintf(stdout, "\n📋 Analysis of %s:\n", mOpts.envFile)
	fmt.Fprintf(stdout, "━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━\n")

	if len(sensitive) > 0 {
		fmt.Fprintf(stdout, "\n🔐 Sensitive keys (will be migrated):\n")
		for _, entry := range sensitive {
			fmt.Fprintf(stdout, "   • %s\n", entry.key)
		}
	}

	if len(nonSensitive) > 0 {
		fmt.Fprintf(stdout, "\n📄 Non-sensitive (will keep as-is):\n")
		for _, entry := range nonSensitive {
			if !entry.isComment && !entry.isEmpty {
				fmt.Fprintf(stdout, "   • %s\n", entry.key)
			}
		}
	}

	fmt.Fprintf(stdout, "\n━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━\n")

	if len(sensitive) == 0 {
		fmt.Fprintln(stdout, "\n✅ No sensitive keys found. Nothing to migrate.")
		return 0
	}

	// Confirm with user
	if !mOpts.force && !mOpts.dryRun {
		fmt.Fprintf(stdout, "\n⚠️  Found %d sensitive key(s) to migrate.\n", len(sensitive))
		fmt.Fprintf(stdout, "Do you want to proceed? [y/N]: ")

		reader := bufio.NewReader(os.Stdin)
		response, _ := reader.ReadString('\n')
		response = strings.TrimSpace(strings.ToLower(response))

		if response != "y" && response != "yes" {
			fmt.Fprintln(stdout, "❌ Migration cancelled.")
			return 0
		}
	}

	// Load config
	cfg, err := config.Load(".coffer")
	if err != nil {
		fmt.Fprintf(stderr, "Error: not initialized. Run 'coffer init' first\n")
		return 1
	}

	ns := cfg.ResolveNamespace(mOpts.namespace)

	if mOpts.dryRun {
		fmt.Fprintf(stdout, "\n🔍 Dry run mode - generating template only\n")
	} else {
		store, err := secret.NewStore()
		if err != nil {
			fmt.Fprintf(stderr, "Error creating secret store: %v\n", err)
			return 1
		}

		fmt.Fprintf(stdout, "\n💾 Storing secrets in OS keychain...\n")
		for _, entry := range sensitive {
			if err := store.Set(ns, entry.key, []byte(entry.value)); err != nil {
				fmt.Fprintf(stderr, "Error storing %s: %v\n", entry.key, err)
				return 1
			}
			fmt.Fprintf(stdout, "   ✓ %s\n", entry.key)
		}

		if cfg.Secrets == nil {
			cfg.Secrets = make(map[string]string)
		}
		if cfg.Namespaces == nil {
			cfg.Namespaces = make(map[string]*config.NamespaceConfig)
		}

		if _, ok := cfg.Namespaces[ns]; !ok {
			cfg.Namespaces[ns] = &config.NamespaceConfig{
				Secrets: make(map[string]string),
			}
		}

		for _, entry := range sensitive {
			placeholder := fmt.Sprintf("{{coffer:%s}}", entry.key)
			cfg.Secrets[entry.key] = placeholder
			cfg.Namespaces[ns].Secrets[entry.key] = placeholder
		}

		cfg.Inject = "file"
		cfg.Config = mOpts.template

		if err := config.Save(cfg, ".coffer"); err != nil {
			fmt.Fprintf(stderr, "Error saving config: %v\n", err)
			return 1
		}
		fmt.Fprintf(stdout, "   ✓ Config updated\n")
	}

	// Generate template
	fmt.Fprintf(stdout, "\n📝 Generating template: %s\n", mOpts.template)

	templateContent := generateTemplate(entries, sensitive)

	if err := os.WriteFile(mOpts.template, []byte(templateContent), 0644); err != nil {
		fmt.Fprintf(stderr, "Error writing template: %v\n", err)
		return 1
	}
	fmt.Fprintf(stdout, "   ✓ Template created\n")

	// Summary
	fmt.Fprintf(stdout, "\n━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━\n")
	fmt.Fprintf(stdout, "✅ Migration complete!\n\n")
	fmt.Fprintf(stdout, "Next steps:\n")
	fmt.Fprintf(stdout, "   1. Test: coffer run --inject=file cat %s\n", mOpts.template)
	fmt.Fprintf(stdout, "   2. Run:  coffer run --inject=file <your-command>\n")

	return 0
}

func parseEnvFile(path string) ([]envEntry, error) {
	file, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer file.Close()

	var entries []envEntry
	scanner := bufio.NewScanner(file)

	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		entry := envEntry{original: line}

		if strings.HasPrefix(line, "#") {
			entry.isComment = true
		} else if line == "" {
			entry.isEmpty = true
		} else {
			parts := strings.SplitN(line, "=", 2)
			if len(parts) == 2 {
				entry.key = strings.TrimSpace(parts[0])
				entry.value = strings.TrimSpace(parts[1])
				entry.value = strings.Trim(entry.value, `"'`)
			}
		}

		entries = append(entries, entry)
	}

	return entries, scanner.Err()
}

func isSensitiveKey(key string) bool {
	for _, pattern := range nonSensitivePatterns {
		if pattern.MatchString(key) {
			return false
		}
	}

	for _, pattern := range sensitivePatterns {
		if pattern.MatchString(key) {
			return true
		}
	}

	return false
}

func generateTemplate(entries []envEntry, sensitive []envEntry) string {
	sensitiveMap := make(map[string]bool)
	for _, entry := range sensitive {
		sensitiveMap[entry.key] = true
	}

	var lines []string
	for _, entry := range entries {
		if entry.isComment {
			lines = append(lines, entry.original)
		} else if entry.isEmpty {
			lines = append(lines, "")
		} else if sensitiveMap[entry.key] {
			lines = append(lines, fmt.Sprintf("%s={{coffer:%s}}", entry.key, entry.key))
		} else {
			lines = append(lines, entry.original)
		}
	}

	return strings.Join(lines, "\n") + "\n"
}
