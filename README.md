# Coffer

Secure secret management for AI agents and development workflows.

Coffer stores secrets in your OS native credential store (macOS Keychain, Linux GNOME Keyring, Windows Credential Manager) and injects them into commands via environment variables, file templates, or local database proxies. Secrets never live in plain-text files.

---

## Installation

Coffer is a Go project — installation means building the binary and placing it on your target machine.

### Prerequisites

| Platform | Dependency | Notes |
|----------|-----------|-------|
| **macOS** | `security` | Built-in, nothing to install |
| **Linux** | `secret-tool` (`libsecret`) + running keyring daemon | See below |
| **Windows** | `cmdkey` | Built-in |

#### Linux Dependencies

```bash
# Debian / Ubuntu
sudo apt-get update && sudo apt-get install -y libsecret-tools gnome-keyring

# RHEL / CentOS / Fedora
sudo yum install -y libsecret-tools gnome-keyring
```

Headless Linux servers need a D-Bus session and a keyring daemon for `secret-tool`:

```bash
# Start a D-Bus session bus
export $(dbus-launch)

# Unlock gnome-keyring (set any password, it won't be used again)
echo -n 'any-password' | gnome-keyring-daemon --unlock --daemonize --components=secrets

# Verify secret-tool works
echo 'test' | secret-tool store --label=test service coffer name coffer.test.test
secret-tool lookup service coffer name coffer.test.test
secret-tool clear service coffer name coffer.test.test
```

> Add `dbus-launch` and `gnome-keyring-daemon` to your `~/.bashrc` or `~/.zshrc` so they start automatically on login.

### Option A: Cross-compile from your dev machine and deploy (recommended)

Build a Linux binary on your macOS machine and copy it to the server:

```bash
# 1. Clone and cross-compile
git clone <repo-url>
cd coffer

GOOS=linux GOARCH=amd64 go build -o coffer-linux ./cmd/coffer

# 2. Copy to server
scp coffer-linux user@your-server:/tmp/coffer

# 3. Install on server
ssh user@your-server
sudo mv /tmp/coffer /usr/local/bin/coffer
sudo chmod +x /usr/local/bin/coffer

# 4. Verify
coffer --version
```

> For ARM servers (AWS Graviton, Raspberry Pi), use `GOARCH=arm64` instead of `GOARCH=amd64`.

### Option B: Build directly on the server

If Go ≥ 1.25 is already installed on the server:

```bash
git clone <repo-url>
cd coffer
go build -o coffer ./cmd/coffer
sudo mv coffer /usr/local/bin/
```

### Option C: `go install`

```bash
# Requires Go ≥ 1.25
go install example.com/coffer/cmd/coffer@latest
```

`coffer` will be placed in `$GOPATH/bin`. Make sure that directory is in your `PATH`.

### Verify Installation

```bash
coffer --version
coffer --help
```

---

## Quick Start

```bash
# 1. Initialize (global config recommended for servers)
coffer init --global

# 2. Add secrets
coffer secret add db-password --global --ns=production
printf '%s' 'my-secret-value' | coffer secret add API_KEY --global --ns=dev

# 3. Check status
coffer check --global --ns=production --json

# 4. Run a command with secrets injected
coffer run --global --ns=production python app.py
```

Secret names are used exactly as provided — coffer does not uppercase or transform them:

```bash
coffer secret add AWS_ACCESS_KEY_ID --global --ns=aws
coffer secret add AWS_SECRET_ACCESS_KEY --global --ns=aws
```

---

## Tutorial

This section walks through a complete scenario: building coffer, installing it, and using it with a Python application.

### Build and Install

```bash
# Clone and build
git clone <repo-url>
cd coffer
go build -o coffer ./cmd/coffer

# Install to user directory
mkdir -p ~/bin
cp coffer ~/bin/
chmod +x ~/bin/coffer
echo 'export PATH="$HOME/bin:$PATH"' >> ~/.zshrc
source ~/.zshrc

# Verify
coffer --version
```

### Project Setup

```bash
cd /path/to/your/project
coffer init
```

### Add Secrets

```bash
coffer secret add db-password
# Enter: mysecretpassword

coffer secret add api-key
# Enter: myapikey123
```

### List Secrets

```bash
coffer secret list
```

### Run a Command

```bash
coffer run python app.py
```

### Complete Python Example

Suppose you have `app.py`:

```python
import os

db_password = os.environ.get('DB_PASSWORD')
api_key = os.environ.get('API_KEY')

print(f"Database password: {db_password}")
print(f"API key: {api_key}")
```

Run it with secrets injected:

```bash
coffer run python app.py
```

Output:
```
Database password: mysecretpassword
API key: myapikey123
```

### Using Namespaces

Namespaces isolate secrets per environment:

```bash
# Dev environment
coffer secret add db-password --ns=development
# Enter: devpassword

# Production environment
coffer secret add db-password --ns=production
# Enter: prodpassword

coffer run --ns=development python app.py
# Output: devpassword

coffer run --ns=production python app.py
# Output: prodpassword
```

Priority: `--ns` flag > `COFFER_NS` env var > config `default_ns`.

---

## Agent Usage

Coffer is designed for AI agent workflows. Agents use `coffer check --json` to inspect readiness and `coffer run` to inject secrets without exposing them.

### Core Concepts

- **Secret storage**: OS native credential store (macOS Keychain, Linux GNOME Keyring, Windows Credential Manager). Never in the file system.
- **Namespaces**: Isolate secrets per environment. Fully isolated from each other.
- **Injection modes**: `env` (environment variables, default) or `file` (temporary files).

### Agent Workflow

1. **Check status** — `coffer check --json` returns whether all secrets are configured
2. **Prompt the user** — if a secret is missing, ask them to run `coffer secret add <name>` on their terminal
3. **Verify** — re-run `coffer check --json` to confirm readiness
4. **Run** — `coffer run <command>` injects secrets without exposing them

JSON response example:

```json
{
  "ready": false,
  "ns": "production",
  "secrets": [
    {"name": "db-pwd", "configured": false, "fix": "coffer secret add db-pwd --ns=production"}
  ]
}
```

### Example Interaction

```
Agent: Let me check secret status first.
$ coffer check --json
{
  "ready": false,
  "ns": "default",
  "secrets": [
    {"name": "db-pwd", "configured": false, "fix": "coffer secret add db-pwd"}
  ]
}

Agent: The project is missing the db-pwd secret. Please run:
coffer secret add db-pwd

User: [runs the command and enters the value]

Agent: Secret is configured. Running the app.
$ coffer run python app.py
```

### Safety Notes

- `coffer secret get` is blocked in `--json` mode to prevent agents from reading plaintext secrets
- Environment marker `COFFER_CALLER=1` is set for called processes
- Different namespaces are fully isolated from each other
- Secrets never reside in the file system

### Troubleshooting

| Error | Fix |
|-------|-----|
| `not initialized` | Run `coffer init` |
| `secret not found` | Run `coffer secret list` to check, then `coffer secret add <name>` |
| `secret get is not allowed in JSON mode` | Expected behavior — agents cannot retrieve plaintext secrets |
| `invalid secret name` | Use only letters, numbers, hyphens, and underscores |

### Best Practices

- Always use `coffer check --json` (not plain `coffer check`) for agent parsing
- Always check readiness before running commands
- Use namespaces to separate environments
- Never use `coffer secret get` in agent workflows — always use `coffer run` for injection
- When a secret is missing, prompt the user to run `coffer secret add <name>` locally — never ask them to paste the value into chat

---

## Command Reference

| Command | Description |
|---------|-------------|
| `coffer init [--global]` | Initialize config (local or global) |
| `coffer secret add <name>` | Add a secret (interactive or stdin) |
| `coffer secret update <name>` | Update an existing secret |
| `coffer secret list` | List secrets in the current namespace |
| `coffer secret get <name>` | Display a secret value (terminal only) |
| `coffer secret delete <name>` | Delete a secret |
| `coffer check [--json]` | Check if all secrets are ready |
| `coffer run <command>` | Inject secrets and run a command |
| `coffer inject -i <tmpl> -o <out>` | Render `{{coffer:name}}` templates |
| `coffer db add <name>` | Register a PostgreSQL connection |
| `coffer db proxy <name>` | Start a local database proxy |
| `coffer migrate <env-file>` | Migrate `.env` secrets to keychain |
| `coffer status` | Show current configuration status |

### Global Options

```
--ns=<namespace>    Namespace (priority: CLI > COFFER_NS > config default_ns)
--global            Operate on global config (~/.config/coffer/)
--json              JSON output (for agent parsing)
--inject=env|file   Injection mode (env=environment vars, file=temp files)
--config=<path>     Custom config file path
```

### Template Injection (`coffer inject`)

For tools that need config files instead of env vars:

```bash
# Template config.tmpl:
# ---
# database:
#   password: "{{coffer:DB_PASSWORD}}"
# api_key: "{{coffer:API_KEY}}"
# ---

coffer inject -i config.tmpl -o config.yaml --global --ns=prod
```

### PostgreSQL Database Proxy

```bash
# Register connection (password stays in keychain)
coffer db add my-pg \
  --host db.example.com --port 5432 \
  --user admin --database app --global

# Start proxy — listens on 127.0.0.1:<port>, authenticates using keychain
coffer db proxy my-pg --global
```

Clients connect to `127.0.0.1:<port>` without needing the database password.

### Migrate .env Files

```bash
coffer migrate .env --global --ns=prod --dry-run   # Preview
coffer migrate .env --global --ns=prod              # Execute
```

- Sensitive values → keychain
- `.env` → template with `{{coffer:name}}` placeholders, plaintext removed

---

## Configuration Reference

### Global Config

`~/.config/coffer/config.yaml`:

```yaml
default_ns: default
inject: env
secrets:
  DB_PASSWORD: "{{coffer:DB_PASSWORD}}"
namespaces:
  production:
    secrets:
      DB_PASSWORD: "{{coffer:DB_PASSWORD}}"
      API_KEY: "{{coffer:API_KEY}}"
  staging:
    secrets:
      DB_PASSWORD: "{{coffer:DB_PASSWORD}}"
```

### Local Config

`.coffer` uses the same format.

### Merge Rules

- Global config is the base
- Local config overrides/appends to global
- `--global` flag operates on global config only

## Platform Support

| Platform | Backend | Dependencies |
|----------|---------|-------------|
| macOS | Keychain (`security`) | Built-in |
| Linux | GNOME Keyring (`secret-tool`) | `libsecret-tools gnome-keyring` |
| Windows | Credential Manager (`cmdkey`) | Built-in |
