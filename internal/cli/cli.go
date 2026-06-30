package cli

import (
	"encoding/json"
	"fmt"
	"io"
	"strings"
)

type Options struct {
	NS     string
	JSON   bool
	Inject string
	Config string
	Global bool
}

// Run is the main entry point for the CLI
func Run(args []string, stdout io.Writer, stderr io.Writer) int {
	opts, err := parseGlobalFlags(&args)
	if err != nil {
		fmt.Fprintf(stderr, "Error: %v\n", err)
		return 1
	}

	if len(args) == 0 {
		printUsage(stderr)
		return 1
	}

	switch args[0] {
	case "init":
		return runInit(args[1:], stdout, stderr, opts)
	case "secret":
		return runSecret(args[1:], stdout, stderr, opts)
	case "db":
		return runDB(args[1:], stdout, stderr, opts)
	case "run":
		return runRun(args[1:], stdout, stderr, opts)
	case "check":
		return runCheck(args[1:], stdout, stderr, opts)
	case "inject":
		return runInject(args[1:], stdout, stderr, opts)
	case "status":
		return runStatus(args[1:], stdout, stderr, opts)
	case "migrate":
		return runMigrate(args[1:], stdout, stderr, opts)
	case "install-claude-code":
		return runInstallSkill("claude-code", stdout, stderr)
	case "install-codex":
		return runInstallSkill("codex", stdout, stderr)
	case "--help", "-h":
		printUsage(stdout)
		return 0
	case "--version", "-v":
		fmt.Fprintln(stdout, "coffer v0.1.0")
		return 0
	default:
		fmt.Fprintf(stderr, "Unknown command: %s\n\n", args[0])
		printUsage(stderr)
		return 1
	}
}

func parseGlobalFlags(args *[]string) (*Options, error) {
	opts := &Options{
		NS:     "default",
		Inject: "env",
	}

	var remaining []string
	for i := 0; i < len(*args); i++ {
		arg := (*args)[i]

		// -- terminates option parsing; everything after is positional.
		// Preserve -- itself so subcommands like kubectl exec pod -- cmd work.
		if arg == "--" {
			remaining = append(remaining, arg)
			remaining = append(remaining, (*args)[i+1:]...)
			break
		}

		switch {
		case strings.HasPrefix(arg, "--ns=") || strings.HasPrefix(arg, "-n="):
			opts.NS = strings.TrimPrefix(strings.TrimPrefix(arg, "--ns="), "-n=")
		case (arg == "--ns" || arg == "-n") && i+1 < len(*args):
			i++
			opts.NS = (*args)[i]
		case arg == "--json":
			opts.JSON = true
		case strings.HasPrefix(arg, "--inject="):
			opts.Inject = strings.TrimPrefix(arg, "--inject=")
		case arg == "--inject" && i+1 < len(*args):
			i++
			opts.Inject = (*args)[i]
		case strings.HasPrefix(arg, "--config="):
			opts.Config = strings.TrimPrefix(arg, "--config=")
		case arg == "--config" && i+1 < len(*args):
			i++
			opts.Config = (*args)[i]
		case arg == "--global" || arg == "-g":
			opts.Global = true
		default:
			remaining = append(remaining, arg)
		}
	}

	*args = remaining
	return opts, nil
}

func isHelp(args []string) bool {
	return len(args) > 0 && (args[0] == "--help" || args[0] == "-h")
}

func printUsage(w io.Writer) {
	fmt.Fprintln(w, "Usage: coffer <command> [options]")
	fmt.Fprintln(w)
	fmt.Fprintln(w, "Commands:")
	fmt.Fprintln(w, "  init                Initialize project")
	fmt.Fprintln(w, "  secret add <name>   Add a secret")
	fmt.Fprintln(w, "  secret update <name> Update a secret")
	fmt.Fprintln(w, "  secret list         List secrets")
	fmt.Fprintln(w, "  secret delete <name> Delete a secret")
	fmt.Fprintln(w, "  secret get <name>   Get secret value (interactive only)")
	fmt.Fprintln(w, "  db add <name>       Add a database connection")
	fmt.Fprintln(w, "  db list             List database connections")
	fmt.Fprintln(w, "  db remove <name>    Remove a database connection")
	fmt.Fprintln(w, "  db proxy <name>     Start database proxy")
	fmt.Fprintln(w, "  inject              Inject secrets into template")
	fmt.Fprintln(w, "  run <command>       Run command with secrets injected")
	fmt.Fprintln(w, "  check               Check if secrets are ready")
	fmt.Fprintln(w, "  status              Show current status")
	fmt.Fprintln(w, "  migrate <env-file>  Migrate .env file to coffer")
	fmt.Fprintln(w, "  install-claude-code Install/update Claude Code skill")
	fmt.Fprintln(w, "  install-codex       Install/update Codex skill")
	fmt.Fprintln(w)
	fmt.Fprintln(w, "Global Options:")
	fmt.Fprintln(w, "  -n, --ns=<namespace> Specify namespace (default: 'default')")
	fmt.Fprintln(w, "  -g, --global         Use global config instead of local .coffer")
	fmt.Fprintln(w, "  --json               Output JSON (for agent)")
	fmt.Fprintln(w, "  --inject=<mode>      Injection mode: env or file (default: env)")
	fmt.Fprintln(w, "  --config=<path>      Config file path")
	fmt.Fprintln(w, "  -h, --help           Show this help")
	fmt.Fprintln(w, "  -v, --version        Show version")
}

func writeJSON(w io.Writer, data interface{}) error {
	encoder := json.NewEncoder(w)
	encoder.SetIndent("", "  ")
	return encoder.Encode(data)
}

func printJSON(w io.Writer, data interface{}) {
	writeJSON(w, data)
}

type ErrorResponse struct {
	Error   string `json:"error"`
	Message string `json:"message,omitempty"`
	Fix     string `json:"fix,omitempty"`
}

func writeError(w io.Writer, err *ErrorResponse, jsonMode bool) int {
	if jsonMode {
		writeJSON(w, err)
	} else {
		fmt.Fprintf(w, "Error: %s\n", err.Error)
		if err.Message != "" {
			fmt.Fprintf(w, "Message: %s\n", err.Message)
		}
		if err.Fix != "" {
			fmt.Fprintf(w, "Fix: %s\n", err.Fix)
		}
	}
	return 1
}
