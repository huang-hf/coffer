package cli

import (
	"fmt"
	"io"
	"os"
	"os/signal"
	"strconv"
	"strings"

	"github.com/huang-hf/coffer/internal/config"
	"github.com/huang-hf/coffer/internal/dbproxy"
	"github.com/huang-hf/coffer/internal/dbproxy/pg"
	"github.com/huang-hf/coffer/internal/secret"
)

func runDB(args []string, stdout io.Writer, stderr io.Writer, opts *Options) int {
	if isHelp(args) {
		printDBUsage(stdout)
		return 0
	}
	if len(args) == 0 {
		printDBUsage(stderr)
		return 1
	}

	switch args[0] {
	case "add":
		return runDBAdd(args[1:], stdout, stderr, opts)
	case "list":
		return runDBList(args[1:], stdout, stderr, opts)
	case "remove":
		return runDBRemove(args[1:], stdout, stderr, opts)
	case "proxy":
		return runDBProxy(args[1:], stdout, stderr, opts)
	default:
		fmt.Fprintf(stderr, "Unknown db command: %s\n", args[0])
		return 1
	}
}

func printDBUsage(w io.Writer) {
	fmt.Fprintln(w, "Usage: coffer db <add|list|remove|proxy> [options]")
	fmt.Fprintln(w)
	fmt.Fprintln(w, "Commands:")
	fmt.Fprintln(w, "  add <name>     Add a PostgreSQL connection")
	fmt.Fprintln(w, "  list           List database connections")
	fmt.Fprintln(w, "  remove <name>  Remove a database connection")
	fmt.Fprintln(w, "  proxy <name>   Start a local database proxy")
}

func runDBAdd(args []string, stdout io.Writer, stderr io.Writer, opts *Options) int {
	if isHelp(args) {
		fmt.Fprintln(stdout, "Usage: coffer db add <name> --host <host> --port <port> --user <user> --database <db> [--type postgres]")
		return 0
	}
	var name, host, user, database, dbType string
	var port int

	flags := parseDBAddFlags(args, &name, &host, &user, &database, &dbType, &port)
	remaining := flags

	if name == "" && len(remaining) > 0 {
		name = remaining[0]
		remaining = remaining[1:]
	}

	if name == "" {
		fmt.Fprintln(stderr, "Usage: coffer db add <name> --host <host> --port <port> --user <user> --database <db> [--type postgres]")
		return 1
	}

	if host == "" {
		fmt.Fprintln(stderr, "Error: --host is required")
		return 1
	}
	if port == 0 {
		port = 5432
	}
	if user == "" {
		fmt.Fprintln(stderr, "Error: --user is required")
		return 1
	}
	if database == "" {
		fmt.Fprintln(stderr, "Error: --database is required")
		return 1
	}
	if dbType == "" {
		dbType = config.DBTypePostgres
	}

	fmt.Fprintf(stdout, "Enter password for %s@%s/%s: ", user, host, database)
	password, err := readPassword(stderr)
	if err != nil {
		fmt.Fprintf(stderr, "Error reading password: %v\n", err)
		return 1
	}
	if password == "" {
		fmt.Fprintln(stderr, "Error: password cannot be empty")
		return 1
	}

	dbCfg := &config.DatabaseConfig{
		Type:     dbType,
		Host:     host,
		Port:     port,
		User:     user,
		Database: database,
	}

	cfgPath := configPath(opts)
	if err := dbproxy.AddConfig(cfgPath, name, dbCfg, password); err != nil {
		fmt.Fprintf(stderr, "Error: %v\n", err)
		return 1
	}

	scope := "local"
	if opts.Global {
		scope = "global"
	}
	fmt.Fprintf(stdout, "Database '%s' added to %s config\n", name, scope)
	return 0
}

func parseDBAddFlags(args []string, name, host, user, database, dbType *string, port *int) []string {
	var remaining []string
	for i := 0; i < len(args); i++ {
		switch {
		case strings.HasPrefix(args[i], "--host="):
			*host = strings.TrimPrefix(args[i], "--host=")
		case args[i] == "--host" && i+1 < len(args):
			i++
			*host = args[i]
		case strings.HasPrefix(args[i], "--port="):
			*port, _ = strconv.Atoi(strings.TrimPrefix(args[i], "--port="))
		case args[i] == "--port" && i+1 < len(args):
			i++
			*port, _ = strconv.Atoi(args[i])
		case strings.HasPrefix(args[i], "--user="):
			*user = strings.TrimPrefix(args[i], "--user=")
		case args[i] == "--user" && i+1 < len(args):
			i++
			*user = args[i]
		case strings.HasPrefix(args[i], "--database="):
			*database = strings.TrimPrefix(args[i], "--database=")
		case args[i] == "--database" && i+1 < len(args):
			i++
			*database = args[i]
		case strings.HasPrefix(args[i], "--type="):
			*dbType = strings.TrimPrefix(args[i], "--type=")
		case args[i] == "--type" && i+1 < len(args):
			i++
			*dbType = args[i]
		default:
			remaining = append(remaining, args[i])
		}
	}
	return remaining
}

func runDBList(args []string, stdout io.Writer, stderr io.Writer, opts *Options) int {
	if isHelp(args) {
		fmt.Fprintln(stdout, "Usage: coffer db list [--global]")
		return 0
	}
	if len(args) > 0 {
		fmt.Fprintln(stderr, "Usage: coffer db list [--global]")
		return 1
	}

	cfg, err := loadDBConfig(opts)
	if err != nil {
		fmt.Fprintf(stderr, "Error: %v\n", err)
		return 1
	}

	configs := dbproxy.ListConfigs(cfg)
	if len(configs) == 0 {
		fmt.Fprintln(stdout, "No databases configured")
		return 0
	}

	scope := "merged"
	if opts.Global {
		scope = "global"
	}
	fmt.Fprintf(stdout, "Databases (%s):\n", scope)
	for _, db := range configs {
		fmt.Fprintf(stdout, "  %s (%s %s@%s:%d/%s)\n",
			db.Name, db.Type, db.User, db.Host, db.Port, db.Database)
	}
	return 0
}

func runDBRemove(args []string, stdout io.Writer, stderr io.Writer, opts *Options) int {
	if isHelp(args) {
		fmt.Fprintln(stdout, "Usage: coffer db remove <name> [--global]")
		return 0
	}
	if len(args) != 1 {
		fmt.Fprintln(stderr, "Usage: coffer db remove <name> [--global]")
		return 1
	}

	name := args[0]
	cfgPath := configPath(opts)
	if err := dbproxy.RemoveConfig(cfgPath, name); err != nil {
		fmt.Fprintf(stderr, "Error: %v\n", err)
		return 1
	}

	scope := "local"
	if opts.Global {
		scope = "global"
	}
	fmt.Fprintf(stdout, "Database '%s' removed from %s config\n", name, scope)
	return 0
}

func runDBProxy(args []string, stdout io.Writer, stderr io.Writer, opts *Options) int {
	if isHelp(args) {
		fmt.Fprintln(stdout, "Usage: coffer db proxy <name> [--global]")
		return 0
	}
	if len(args) != 1 {
		fmt.Fprintln(stderr, "Usage: coffer db proxy <name> [--global]")
		return 1
	}
	name := args[0]

	cfg, err := loadDBConfig(opts)
	if err != nil {
		fmt.Fprintf(stderr, "Error: %v\n", err)
		return 1
	}

	dbCfg, exists := cfg.Databases[name]
	if !exists {
		fmt.Fprintf(stderr, "Error: database %q not found\n", name)
		return 1
	}

	store, err := secret.NewStore()
	if err != nil {
		fmt.Fprintf(stderr, "Error creating secret store: %v\n", err)
		return 1
	}

	secretName := config.DBSecretName(name)
	passwordBytes, err := store.Get(config.DBAuthSecretNS, secretName)
	if err != nil {
		fmt.Fprintf(stderr, "Error: password for database %q not found in keychain\n", name)
		fmt.Fprintln(stderr, "Use 'coffer db add' to set up the database connection")
		return 1
	}

	proxy := pg.NewProxy(pg.DBConfig{
		Host:     dbCfg.Host,
		Port:     dbCfg.Port,
		User:     dbCfg.User,
		Database: dbCfg.Database,
		Password: string(passwordBytes),
	})

	listenPort, err := proxy.Start()
	if err != nil {
		fmt.Fprintf(stderr, "Error starting proxy: %v\n", err)
		return 1
	}

	fmt.Fprintf(stdout, "Proxy started: psql -h 127.0.0.1 -p %d -U %s -d %s\n",
		listenPort, dbCfg.User, dbCfg.Database)
	fmt.Fprintf(stdout, "Proxying to: %s:%d\n", dbCfg.Host, dbCfg.Port)
	fmt.Fprintln(stdout, "Press Ctrl+C to stop")

	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, os.Interrupt)
	<-sigCh

	fmt.Fprintln(stdout, "\nShutting down...")
	proxy.Close()
	return 0
}

func loadDBConfig(opts *Options) (*config.Config, error) {
	if opts.Global {
		return config.Load(config.GlobalConfigPath())
	}
	return config.LoadChain(".coffer")
}
