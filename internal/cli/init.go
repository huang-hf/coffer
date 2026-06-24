package cli

import (
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"

	"github.com/huang-hf/coffer/internal/config"
	"github.com/huang-hf/coffer/internal/secret"
)

func runInit(args []string, stdout io.Writer, stderr io.Writer, opts *Options) int {
	if len(args) > 0 && args[0] != "--global" {
		fmt.Fprintln(stderr, "Usage: coffer init [--global]")
		return 1
	}

	if opts.Global {
		return runInitGlobal(stdout, stderr)
	}

	projectDir, err := os.Getwd()
	if err != nil {
		fmt.Fprintf(stderr, "Error getting current directory: %v\n", err)
		return 1
	}

	configPath := filepath.Join(projectDir, ".coffer")
	if _, err := os.Stat(configPath); err == nil {
		fmt.Fprintln(stderr, "Error: .coffer already exists")
		return 1
	}

	cfg := &config.Config{
		DefaultNS: "default",
		Inject:    "env",
		Secrets:   make(map[string]string),
	}

	if err := config.Save(cfg, configPath); err != nil {
		fmt.Fprintf(stderr, "Error creating .coffer: %v\n", err)
		return 1
	}

	if code := initAgeKey(stdout, stderr); code != 0 {
		return code
	}

	fmt.Fprintln(stdout, "✓ Created .coffer")
	fmt.Fprintln(stdout)
	fmt.Fprintln(stdout, "Next steps:")
	fmt.Fprintln(stdout, "  1. Add secrets: coffer secret add <name> --ns=<namespace>")
	fmt.Fprintln(stdout, "  2. Run your app: coffer run <command>")
	return 0
}

func runInitGlobal(stdout io.Writer, stderr io.Writer) int {
	globalPath := config.GlobalConfigPath()
	if globalPath == "" {
		fmt.Fprintln(stderr, "Error: cannot determine home directory")
		return 1
	}

	dir := filepath.Dir(globalPath)
	if err := os.MkdirAll(dir, 0700); err != nil {
		fmt.Fprintf(stderr, "Error creating config directory: %v\n", err)
		return 1
	}

	if _, err := os.Stat(globalPath); err == nil {
		fmt.Fprintln(stderr, "Error: global config already exists at", globalPath)
		return 1
	}

	cfg := &config.Config{
		DefaultNS: "default",
		Inject:    "env",
		Secrets:   make(map[string]string),
	}

	if err := config.Save(cfg, globalPath); err != nil {
		fmt.Fprintf(stderr, "Error creating global config: %v\n", err)
		return 1
	}

	if code := initAgeKey(stdout, stderr); code != 0 {
		return code
	}

	fmt.Fprintf(stdout, "✓ Created global config at %s\n", globalPath)
	fmt.Fprintln(stdout)
	fmt.Fprintln(stdout, "Next steps:")
	fmt.Fprintln(stdout, "  1. Add global secrets: coffer secret add --global <name>")
	fmt.Fprintln(stdout, "  2. Use in any project: coffer run <command>")
	return 0
}

func initAgeKey(stdout io.Writer, stderr io.Writer) int {
	storeDir, err := secret.StoreDir()
	if err != nil {
		fmt.Fprintf(stderr, "Error determining store directory: %v\n", err)
		return 1
	}

	if err := secret.EnsureAgeKey(storeDir); err != nil {
		if errors.Is(err, secret.ErrAgeKeyAlreadyExists) {
			// age key already exists — not an error on re-init
			return 0
		}
		fmt.Fprintf(stderr, "Error generating age key: %v\n", err)
		return 1
	}

	fmt.Fprintf(stdout, "✓ Created age encryption key at %s\n", secret.AgeKeyPath(storeDir))
	return 0
}
