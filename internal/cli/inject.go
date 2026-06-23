package cli

import (
	"fmt"
	"io"
	"os"
	"strings"

	"coffer/internal/config"
	"coffer/internal/secret"
)

func runInject(args []string, stdout io.Writer, stderr io.Writer, opts *Options) int {
	if isHelp(args) {
		printInjectUsage(stdout)
		return 0
	}

	var inputFile, outputFile string

	for i := 0; i < len(args); i++ {
		switch {
		case strings.HasPrefix(args[i], "--input="):
			inputFile = strings.TrimPrefix(args[i], "--input=")
		case args[i] == "--input" && i+1 < len(args):
			i++
			inputFile = args[i]
		case strings.HasPrefix(args[i], "-i="):
			inputFile = strings.TrimPrefix(args[i], "-i=")
		case args[i] == "-i" && i+1 < len(args):
			i++
			inputFile = args[i]
		case strings.HasPrefix(args[i], "--output="):
			outputFile = strings.TrimPrefix(args[i], "--output=")
		case args[i] == "--output" && i+1 < len(args):
			i++
			outputFile = args[i]
		case strings.HasPrefix(args[i], "-o="):
			outputFile = strings.TrimPrefix(args[i], "-o=")
		case args[i] == "-o" && i+1 < len(args):
			i++
			outputFile = args[i]
		default:
			printInjectUsage(stderr)
			return 1
		}
	}

	template, err := readInput(inputFile)
	if err != nil {
		fmt.Fprintf(stderr, "Error reading input: %v\n", err)
		return 1
	}

	var cfg *config.Config
	if opts.Global {
		cfg, err = config.Load(config.GlobalConfigPath())
	} else {
		cfg, err = config.LoadChain(".coffer")
	}
	if err != nil {
		if opts.Global {
			fmt.Fprintf(stderr, "Error: global config not found. Run 'coffer init --global' first\n")
		} else {
			fmt.Fprintf(stderr, "Error: not initialized. Run 'coffer init' first\n")
		}
		return 1
	}

	ns := cfg.ResolveNamespace(opts.NS)

	store, err := secret.NewStore()
	if err != nil {
		fmt.Fprintf(stderr, "Error creating secret store: %v\n", err)
		return 1
	}

	secrets := cfg.GetSecretsForNamespace(ns)
	resolved := make(map[string]string)
	for name := range secrets {
		value, err := store.Get(ns, name)
		if err != nil {
			fmt.Fprintf(stderr, "Warning: secret '%s' not found in keychain, skipping\n", name)
			continue
		}
		resolved[name] = string(value)
	}

	result := template
	for name, value := range resolved {
		placeholder := "{{coffer:" + name + "}}"
		result = strings.ReplaceAll(result, placeholder, value)
	}

	if err := writeOutput(outputFile, result); err != nil {
		fmt.Fprintf(stderr, "Error writing output: %v\n", err)
		return 1
	}

	return 0
}

func printInjectUsage(w io.Writer) {
	fmt.Fprintln(w, "Usage: coffer inject [-i <input>] [-o <output>] [--ns=<namespace>] [--global]")
	fmt.Fprintln(w)
	fmt.Fprintln(w, "Options:")
	fmt.Fprintln(w, "  -i, --input <path>   Template input file (default: stdin)")
	fmt.Fprintln(w, "  -o, --output <path>  Rendered output file (default: stdout)")
	fmt.Fprintln(w, "  --ns=<namespace>     Specify namespace")
	fmt.Fprintln(w, "  --global             Use global config")
}

func readInput(path string) (string, error) {
	if path == "" {
		data, err := io.ReadAll(os.Stdin)
		if err != nil {
			return "", fmt.Errorf("reading stdin: %w", err)
		}
		return string(data), nil
	}
	data, err := os.ReadFile(path)
	if err != nil {
		return "", fmt.Errorf("reading %s: %w", path, err)
	}
	return string(data), nil
}

func writeOutput(path, content string) error {
	if path == "" {
		fmt.Print(content)
		if !strings.HasSuffix(content, "\n") {
			fmt.Println()
		}
		return nil
	}
	return os.WriteFile(path, []byte(content), 0644)
}
