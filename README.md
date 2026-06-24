# Coffer

Secure secret management for AI agents and development workflows.

Coffer stores secrets in `~/.coffer/` as owner-only-readable JSON files (0600) and injects them into commands via environment variables, file templates, or local database proxies. Secrets never live in project files, `.env`, or source control.

System keyring backends (macOS Keychain, Linux GNOME Keyring, Windows Credential Manager) are available as opt-in via environment variables.

---

## Quick Start

```bash
# 1. Install
go install github.com/huang-hf/coffer/cmd/coffer@latest

# 2. Initialize global config
coffer init --global

# 3. Add a secret
coffer secret add DB_PASSWORD --global --ns=prod

# 4. Run a command with secrets injected
coffer run --global --ns=prod python app.py
```

Secret names are used exactly as provided — coffer does not uppercase or transform them:

```bash
coffer secret add AWS_ACCESS_KEY_ID --global --ns=aws
coffer secret add AWS_SECRET_ACCESS_KEY --global --ns=aws
```

---

## Installation

### Prerequisites

None. Coffer uses a file-based store by default (`~/.coffer/`, 0600 permissions). System keyring backends are opt-in only.

| Opt-in backend | Env var | Platform |
|---------------|---------|----------|
| macOS Keychain | `COFFER_USE_KEYCHAIN=true` | macOS |
| Linux GNOME Keyring | `COFFER_USE_SECRET_TOOL=true` | Linux |
| Windows Credential Manager | `COFFER_USE_CMDKEY=true` | Windows |

### Option A: Cross-compile and deploy (recommended for servers)

Build a Linux binary on your development machine and copy it to the server:

```bash
git clone <repo-url>
cd coffer

GOOS=linux GOARCH=amd64 go build -o coffer-linux ./cmd/coffer

scp coffer-linux user@your-server:/tmp/coffer

ssh user@your-server
sudo mv /tmp/coffer /usr/local/bin/coffer
sudo chmod +x /usr/local/bin/coffer
```

> For ARM servers (AWS Graviton, Raspberry Pi), use `GOARCH=arm64`.

### Option B: Build directly on the server

If Go ≥ 1.25 is already installed:

```bash
git clone <repo-url>
cd coffer
go build -o coffer ./cmd/coffer
sudo mv coffer /usr/local/bin/
```

### Option C: `go install`

```bash
go install github.com/huang-hf/coffer/cmd/coffer@latest
```

`coffer` will be placed in `$GOPATH/bin`.

### Verify

```bash
coffer --version
coffer --help
```

---

## Tutorial

### Build and Install

```bash
git clone <repo-url>
cd coffer
go build -o coffer ./cmd/coffer

mkdir -p ~/bin
cp coffer ~/bin/
chmod +x ~/bin/coffer
echo 'export PATH="$HOME/bin:$PATH"' >> ~/.zshrc
source ~/.zshrc

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

Run it:

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
coffer secret add db-password --ns=development
# Enter: devpassword

coffer secret add db-password --ns=production
# Enter: prodpassword

coffer run --ns=development python app.py
# Output: devpassword

coffer run --ns=production python app.py
# Output: prodpassword
```

Priority: `--ns` flag > `COFFER_NS` env var > config `default_ns`.

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
| `coffer db list` | List registered database connections |
| `coffer db remove <name>` | Remove a database connection |
| `coffer db proxy <name>` | Start a local database proxy |
| `coffer migrate <env-file>` | Migrate `.env` secrets into coffer |
| `coffer status` | Show current configuration status |
| `coffer install-claude-code` | Install/update skill for Claude Code |
| `coffer install-codex` | Install/update skill for Codex |

### Global Options

```
--ns=<namespace>    Namespace (priority: CLI > COFFER_NS > config default_ns)
--global            Operate on global config (~/.config/coffer/)
--json              JSON output
--inject=env|file   Injection mode (env=environment vars, file=temp files)
--config=<path>     Custom config file path
```

### Template Injection

For tools that need config files instead of env vars, use `{{coffer:name}}` placeholders:

Template file `config.tmpl`:
```yaml
database:
  password: "{{coffer:DB_PASSWORD}}"
api_key: "{{coffer:API_KEY}}"
```

Render it:
```bash
coffer inject -i config.tmpl -o config.yaml --global --ns=prod
```

### PostgreSQL Database Proxy

Register a database connection (password stays in coffer store):
```bash
coffer db add my-pg \
  --host db.example.com --port 5432 \
  --user admin --database app --global
```

Start a local proxy:
```bash
coffer db proxy my-pg --global
```

Clients connect to `127.0.0.1:<port>` without needing the database password.

### Migrate .env Files

```bash
coffer migrate .env --global --ns=prod --dry-run   # Preview
coffer migrate .env --global --ns=prod              # Execute
```

- Sensitive values → coffer store (`~/.coffer/`)
- `.env` → template with `{{coffer:name}}` placeholders

---

## Skill Installation

Coffer includes a built-in SKILL.md for AI agents. Install it with:

```bash
coffer install-claude-code   # for Claude Code
coffer install-codex         # for Codex
```

The commands are idempotent — run them again to update the installed skill to the latest version bundled with your coffer binary.

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

---

## Platform Support

| Platform | Default backend | Opt-in system keyring |
|----------|----------------|----------------------|
| macOS | File store (`~/.coffer/`) | `COFFER_USE_KEYCHAIN=true` → Keychain |
| Linux | File store (`~/.coffer/`) | `COFFER_USE_SECRET_TOOL=true` → GNOME Keyring |
| Windows | File store (`~/.coffer/`) | `COFFER_USE_CMDKEY=true` → Credential Manager |
