package cli

import (
	"fmt"
	"io"

	"github.com/huang-hf/coffer/internal/config"
	"github.com/huang-hf/coffer/internal/secret"
)

func runCheck(args []string, stdout io.Writer, stderr io.Writer, opts *Options) int {
	if len(args) > 0 {
		fmt.Fprintln(stderr, "Usage: coffer check [--ns=<namespace>] [--json]")
		return 1
	}

	cfg, err := config.LoadChain(".coffer")
	if err != nil {
		if opts.JSON {
			writeJSON(stdout, &ErrorResponse{
				Error: "not_initialized",
				Fix:   "coffer init",
			})
		} else {
			fmt.Fprintln(stderr, "Error: not initialized. Run 'coffer init' first")
		}
		return 1
	}

	ns := opts.NS
	if ns == "default" {
		ns = cfg.DefaultNS
	}

	store, err := secret.NewStore()
	if err != nil {
		fmt.Fprintf(stderr, "Error creating secret store: %v\n", err)
		return 1
	}

	type SecretStatus struct {
		Name       string `json:"name"`
		Configured bool   `json:"configured"`
		Fix        string `json:"fix,omitempty"`
	}

	var secrets []SecretStatus
	allReady := true

	for secretName := range cfg.Secrets {
		_, err := store.Get(ns, secretName)
		configured := err == nil

		status := SecretStatus{
			Name:       secretName,
			Configured: configured,
		}

		if !configured {
			allReady = false
			status.Fix = fmt.Sprintf("coffer secret add %s --ns=%s", secretName, ns)
		}

		secrets = append(secrets, status)
	}

	if opts.JSON {
		writeJSON(stdout, &struct {
			Ready   bool           `json:"ready"`
			NS      string         `json:"ns"`
			Secrets []SecretStatus `json:"secrets"`
		}{
			Ready:   allReady,
			NS:      ns,
			Secrets: secrets,
		})
		return 0
	}

	if allReady {
		fmt.Fprintf(stdout, "✓ All secrets ready in namespace '%s'\n", ns)
	} else {
		fmt.Fprintf(stdout, "✗ Some secrets missing in namespace '%s'\n", ns)
		fmt.Fprintln(stdout)
		for _, s := range secrets {
			if s.Configured {
				fmt.Fprintf(stdout, "  ✓ %s\n", s.Name)
			} else {
				fmt.Fprintf(stdout, "  ✗ %s\n", s.Name)
				fmt.Fprintf(stdout, "    Fix: %s\n", s.Fix)
			}
		}
	}

	return 0
}

func runStatus(args []string, stdout io.Writer, stderr io.Writer, opts *Options) int {
	if len(args) > 0 {
		fmt.Fprintln(stderr, "Usage: coffer status")
		return 1
	}

	cfg, err := config.LoadChain(".coffer")
	if err != nil {
		fmt.Fprintln(stderr, "Error: not initialized. Run 'coffer init' first")
		return 1
	}

	store, err := secret.NewStore()
	if err != nil {
		fmt.Fprintf(stderr, "Error creating secret store: %v\n", err)
		return 1
	}

	fmt.Fprintln(stdout, "Coffer Status")
	fmt.Fprintln(stdout, "───────────────")
	fmt.Fprintf(stdout, "Default namespace: %s\n", cfg.DefaultNS)
	fmt.Fprintf(stdout, "Inject mode: %s\n", cfg.Inject)
	if cfg.Config != "" {
		fmt.Fprintf(stdout, "Config file: %s\n", cfg.Config)
	}
	fmt.Fprintln(stdout)

	defaultSecrets, err := store.List(cfg.DefaultNS)
	if err != nil {
		fmt.Fprintf(stderr, "Error listing secrets for default namespace: %v\n", err)
	} else {
		fmt.Fprintf(stdout, "Namespace '%s': %d secrets\n", cfg.DefaultNS, len(defaultSecrets))
	}

	for ns := range cfg.Namespaces {
		secrets, err := store.List(ns)
		if err != nil {
			fmt.Fprintf(stderr, "Error listing secrets for namespace '%s': %v\n", ns, err)
			continue
		}
		fmt.Fprintf(stdout, "Namespace '%s': %d secrets\n", ns, len(secrets))
	}

	return 0
}
