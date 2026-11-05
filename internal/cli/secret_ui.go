package cli

import (
	"context"
	"fmt"
	"io"
	"os"
	"os/exec"
	"os/signal"
	"runtime"
	"sort"
	"strings"
	"time"

	"github.com/huang-hf/coffer/internal/config"
	"github.com/huang-hf/coffer/internal/secret"
	"github.com/huang-hf/coffer/internal/secretui"
)

type secretAddArgs struct {
	Names   []string // deduplicated, order preserved
	UI      bool
	Descs   map[string]string
	Timeout time.Duration
}

const secretAddUsage = "Usage: coffer secret add <name> [--ns=<namespace>] [--global]\n" +
	"       coffer secret add <name>... --ui [--desc NAME=text]... [--timeout 5m] [--ns=<namespace>] [--global]"

func parseSecretAddArgs(args []string) (secretAddArgs, error) {
	a := secretAddArgs{Descs: map[string]string{}, Timeout: 5 * time.Minute}
	var descs []string
	timeoutSet := false
	seen := map[string]bool{}

	for i := 0; i < len(args); i++ {
		arg := args[i]
		switch {
		case arg == "--ui":
			a.UI = true
		case strings.HasPrefix(arg, "--desc="):
			descs = append(descs, strings.TrimPrefix(arg, "--desc="))
		case arg == "--desc":
			if i+1 >= len(args) {
				return a, fmt.Errorf("--desc requires NAME=text")
			}
			i++
			descs = append(descs, args[i])
		case strings.HasPrefix(arg, "--timeout=") || arg == "--timeout":
			var v string
			if arg == "--timeout" {
				if i+1 >= len(args) {
					return a, fmt.Errorf("--timeout requires a duration")
				}
				i++
				v = args[i]
			} else {
				v = strings.TrimPrefix(arg, "--timeout=")
			}
			d, err := time.ParseDuration(v)
			if err != nil {
				return a, fmt.Errorf("invalid --timeout '%s': %v", v, err)
			}
			if d <= 0 {
				return a, fmt.Errorf("--timeout must be positive")
			}
			a.Timeout = d
			timeoutSet = true
		case strings.HasPrefix(arg, "-") && arg != "-":
			return a, fmt.Errorf("unknown option '%s'", arg)
		default:
			if !isValidSecretName(arg) {
				return a, fmt.Errorf("invalid secret name '%s'\nSecret names must contain only letters, numbers, hyphens, and underscores", arg)
			}
			if !seen[arg] {
				seen[arg] = true
				a.Names = append(a.Names, arg)
			}
		}
	}

	if len(a.Names) == 0 {
		return a, fmt.Errorf("missing secret name")
	}
	if !a.UI {
		if len(a.Names) > 1 {
			return a, fmt.Errorf("multiple names require --ui")
		}
		if len(descs) > 0 {
			return a, fmt.Errorf("--desc requires --ui")
		}
		if timeoutSet {
			return a, fmt.Errorf("--timeout requires --ui")
		}
	}
	for _, d := range descs {
		idx := strings.Index(d, "=")
		if idx <= 0 {
			return a, fmt.Errorf("invalid --desc '%s': expected NAME=text", d)
		}
		name, text := d[:idx], d[idx+1:]
		if !seen[name] {
			return a, fmt.Errorf("--desc refers to '%s' which is not among the names", name)
		}
		a.Descs[name] = text
	}
	return a, nil
}

// buildEntries merges config and store names into sorted UI entries.
func buildEntries(cfgNames, storeNames []string, modTime func(name string) (time.Time, bool)) []secretui.Entry {
	inStore := map[string]bool{}
	all := map[string]bool{}
	for _, n := range storeNames {
		inStore[n] = true
		all[n] = true
	}
	for _, n := range cfgNames {
		all[n] = true
	}
	names := make([]string, 0, len(all))
	for n := range all {
		names = append(names, n)
	}
	sort.Strings(names)

	entries := make([]secretui.Entry, 0, len(names))
	for _, n := range names {
		e := secretui.Entry{Name: n, Status: secretui.EntryMissing}
		if inStore[n] {
			e.Status = secretui.EntrySet
			if modTime != nil {
				if t, ok := modTime(n); ok {
					t := t
					e.UpdatedAt = &t
				}
			}
		}
		entries = append(entries, e)
	}
	return entries
}

func storageNote(kind secret.BackendKind) string {
	switch kind {
	case secret.BackendFileEncrypted:
		return "🔒 只在本机传输，加密写入 ~/.coffer，agent 看不到内容"
	case secret.BackendFilePlain:
		return "🔒 只在本机传输，以仅本人可读的文件写入 ~/.coffer（未加密），agent 看不到内容"
	case secret.BackendOSKeychain:
		return "🔒 只在本机传输，写入 macOS 钥匙串，agent 看不到内容"
	default:
		return "🔒 只在本机传输，写入系统凭据存储，agent 看不到内容"
	}
}

func openBrowserFunc() func(string) error {
	if runtime.GOOS == "darwin" {
		return func(url string) error { return exec.Command("open", url).Run() }
	}
	return nil
}

func runSecretAddUI(a secretAddArgs, stdout, stderr io.Writer, opts *Options) int {
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
	storeNames, err := store.List(ns)
	if err != nil {
		fmt.Fprintf(stderr, "Warning: could not list stored secrets: %v\n", err)
		storeNames = nil
	}

	scope := "local"
	if opts.Global {
		scope = "global"
	}

	required := make([]secretui.RequiredSecret, 0, len(a.Names))
	for _, n := range a.Names {
		required = append(required, secretui.RequiredSecret{Name: n, Description: a.Descs[n]})
	}

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt)
	defer stop()

	out, err := secretui.Run(ctx, secretui.Request{
		Namespace:   ns,
		Scope:       scope,
		StorageNote: storageNote(secret.Backend(store)),
		Required:    required,
		Existing: buildEntries(cfg.ListSecretsForNamespace(ns), storeNames, func(name string) (time.Time, bool) {
			return secret.ModTime(store, ns, name)
		}),
		Save: func(name string, value []byte) (string, error) {
			return saveSecretValue(store, cfg, cfgPath, ns, name, value)
		},
		Timeout:     a.Timeout,
		OpenBrowser: openBrowserFunc(),
		OnReady: func(url string) {
			fmt.Fprintln(stderr, "Opening browser to set secrets. If it does not open, visit:")
			fmt.Fprintf(stderr, "  %s\n", url)
			fmt.Fprintf(stderr, "Waiting up to %s...\n", a.Timeout)
		},
		Log: stderr,
	})
	if err != nil {
		fmt.Fprintf(stderr, "Error: %v\n", err)
		return 1
	}

	if opts.JSON {
		results := out.Results
		if results == nil {
			results = []secretui.Result{}
		}
		printJSON(stdout, map[string]interface{}{
			"status":    string(out.Status),
			"namespace": ns,
			"scope":     scope,
			"results":   results,
		})
	} else {
		for _, r := range out.Results {
			if r.Saved {
				fmt.Fprintf(stdout, "✓ Secret '%s' saved to %s namespace '%s'\n", r.Name, scope, ns)
			} else {
				fmt.Fprintf(stdout, "✗ Secret '%s' failed: %s\n", r.Name, r.Error)
			}
			if r.Warning != "" {
				fmt.Fprintf(stderr, "Warning: %s: %s\n", r.Name, r.Warning)
			}
		}
	}

	switch out.Status {
	case secretui.StatusSaved:
		return 0
	case secretui.StatusCancelled:
		fmt.Fprintln(stderr, "Error: cancelled by user")
	case secretui.StatusTimeout:
		fmt.Fprintf(stderr, "Error: timed out after %s\n", a.Timeout)
	}
	return 1
}
