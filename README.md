# Coffer

Secure secret management tool for AI agents. Inject secrets into commands via environment variables or file templates.

## Quick Start

```bash
# 1. Build and install
go build -o coffer ./cmd/coffer
sudo mv coffer /usr/local/bin/

# 2. Initialize project
cd /path/to/your/project
coffer init

# 3. Add secrets
coffer secret add db-pwd
coffer secret add api-key

# 4. Run commands with secrets
coffer run python app.py
```

## Installation

### From Source
```bash
git clone <repo-url>
cd coffer
go build -o coffer ./cmd/coffer
sudo mv coffer /usr/local/bin/
```

### Using Make
```bash
make build
make install
```

## Usage

### Initialize Project
```bash
coffer init
```
Creates `.coffer` configuration file in current directory.

### Manage Secrets

```bash
# Add secret (interactive prompt)
coffer secret add <name> [--ns=<namespace>]

# List all secrets
coffer secret list [--ns=<namespace>]

# Delete secret
coffer secret delete <name> [--ns=<namespace>]

# Check if all secrets are configured
coffer check [--ns=<namespace>]
```

### Run Commands

```bash
# Inject secrets as environment variables
coffer run python app.py

# Inject secrets as files
coffer run --inject=file python app.py

# Use specific namespace
coffer run --ns=staging python app.py
```

## Configuration

### `.coffer` File Format

```yaml
default_ns: production
inject: env
config: config.yaml
secrets:
  db-pwd: "{{coffer:db-pwd}}"
  api-key: "{{coffer:api-key}}"
namespaces:
  staging:
    secrets:
      db-pwd: "{{coffer:db-pwd}}"
  production:
    secrets:
      db-pwd: "{{coffer:db-pwd}}"
```

### Configuration Options

- `default_ns`: Default namespace (default: "default")
- `inject`: Injection mode - "env" or "file" (default: "env")
- `config`: Path to config file template (optional)
- `secrets`: Map of secret names to placeholders
- `namespaces`: Per-namespace secret configurations

## Namespaces

Namespaces isolate secrets by environment (staging, production, etc.).

**Priority**:
1. CLI argument: `--ns=staging`
2. Environment variable: `COFFER_NS=staging`
3. Default from `.coffer` file

```bash
# Add secret to specific namespace
coffer secret add db-pwd --ns=staging

# List secrets in namespace
coffer secret list --ns=production

# Run with namespace
coffer run --ns=staging python app.py
```

## Injection Modes

### Environment Variable Mode (Default)

Secrets are injected as environment variables:

```bash
coffer run python app.py
```

Secret `db-pwd` becomes `DB_PWD` environment variable.

### File Mode

Secrets are written to temporary files:

```bash
coffer run --inject=file python app.py
```

Secrets are written to `/tmp/coffer-<random>/db-pwd` with permissions `0600`.

## Config File Rendering

If you have a config file template:

```yaml
# config.yaml
database:
  password: {{coffer:db-pwd}}
api:
  key: {{coffer:api-key}}
```

Run with:
```bash
coffer run --config=config.yaml python app.py
```

The template is rendered to a temporary file with secrets replaced.

## JSON Output (for AI Agents)

```bash
coffer check --json
```

Response:
```json
{
  "ready": true,
  "ns": "production",
  "secrets": [
    {"name": "db-pwd", "configured": true},
    {"name": "api-key", "configured": true}
  ]
}
```

## Security Features

### Secret Storage
- **macOS**: System Keychain
- **Linux**: GNOME Keyring
- **Windows**: Credential Manager
- **Fallback**: File-based storage

### Caller Detection
- Environment variable marker: `COFFER_CALLER=1`
- Process tree verification against authorized prefixes

### Agent Mode Restrictions
- `secret get` is blocked in JSON mode
- Prevents agents from accessing plaintext secrets

## Platform Support

| Platform | Storage Backend | Command |
|----------|----------------|---------|
| macOS | Keychain | `security` |
| Linux | GNOME Keyring | `secret-tool` |
| Windows | Credential Manager | `cmdkey` |
| Fallback | File storage | N/A |

## Environment Variables

When running `coffer run`:

| Variable | Description |
|----------|-------------|
| `COFFER_NS` | Current namespace |
| `COFFER_CALLER` | Set to "1" for caller detection |
| `<SECRET_NAME>` | Secret values (uppercased, hyphens → underscores) |

## Examples

### Python Application

```bash
# Initialize
coffer init

# Add secrets
coffer secret add db-password
coffer secret add api-key

# Run application
coffer run python app.py
```

### Docker Development

```bash
# Add secrets
coffer secret add db-pwd
coffer secret add redis-password

# Run docker with secrets
coffer run docker-compose up
```

### CI/CD Pipeline

```bash
# Check secrets before deployment
coffer check --json

# Run deployment with secrets
coffer run --ns=production ./deploy.sh
```

## Troubleshooting

### "not initialized" Error
Run `coffer init` in your project directory.

### "secret not found" Error
Check if secret exists: `coffer secret list`

### Permission Denied
Ensure secrets are added: `coffer secret add <name>`

### Wrong Namespace
Check current namespace: `coffer status`

## License

MIT
