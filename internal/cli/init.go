package cli

import (
	"fmt"
	"io"
	"os"
	"path/filepath"

	"coffer/internal/config"
)

func runInit(args []string, stdout io.Writer, stderr io.Writer, opts *Options) int {
	if len(args) > 0 {
		fmt.Fprintln(stderr, "Usage: coffer init")
		return 1
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

	fmt.Fprintln(stdout, "✓ Created .coffer")
	fmt.Fprintln(stdout)
	fmt.Fprintln(stdout, "Next steps:")
	fmt.Fprintln(stdout, "  1. Add secrets: coffer secret add <name> --ns=<namespace>")
	fmt.Fprintln(stdout, "  2. Run your app: coffer run <command>")
	return 0
}
