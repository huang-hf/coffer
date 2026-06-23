package cli

import (
	"fmt"
	"io"
	"os"
	"os/exec"
	"strings"

	"coffer/internal/config"
	"coffer/internal/inject"
	"coffer/internal/secret"
)

func runRun(args []string, stdout io.Writer, stderr io.Writer, opts *Options) int {
	if isHelp(args) {
		printRunUsage(stdout)
		return 0
	}
	if len(args) == 0 {
		printRunUsage(stderr)
		return 1
	}

	var cfg *config.Config
	var err error
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

	env := os.Environ()
	env = append(env, "COFFER_NS="+ns)
	env = append(env, "COFFER_CALLER=coffer")

	secrets := cfg.GetSecretsForNamespace(ns)
	for secretName := range secrets {
		value, err := store.Get(ns, secretName)
		if err != nil {
			fmt.Fprintf(stderr, "Error getting secret '%s': %v\n", secretName, err)
			return 1
		}

		env = append(env, secretName+"="+string(value))
	}

	if opts.Inject == "file" && cfg.Config != "" {
		configPath, err := inject.RenderConfigFile(cfg, store, ns)
		if err != nil {
			fmt.Fprintf(stderr, "Error rendering config: %v\n", err)
			return 1
		}
		defer os.RemoveAll(strings.TrimSuffix(configPath, "/"+cfg.Config))

		env = append(env, "CONFIG_PATH="+configPath)
	}

	cmd := exec.Command(args[0], args[1:]...)
	cmd.Env = env
	cmd.Stdout = stdout
	cmd.Stderr = stderr
	cmd.Stdin = os.Stdin

	if err := cmd.Run(); err != nil {
		if exitErr, ok := err.(*exec.ExitError); ok {
			return exitErr.ExitCode()
		}
		fmt.Fprintf(stderr, "Error running command: %v\n", err)
		return 1
	}

	return 0
}

func printRunUsage(w io.Writer) {
	fmt.Fprintln(w, "Usage: coffer run <command> [args...]")
	fmt.Fprintln(w)
	fmt.Fprintln(w, "Options:")
	fmt.Fprintln(w, "  --ns=<namespace> Specify namespace")
	fmt.Fprintln(w, "  --global         Use global config")
	fmt.Fprintln(w, "  --inject=<mode>  Injection mode: env or file")
}
