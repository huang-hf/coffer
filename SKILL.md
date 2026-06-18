---
name: coffer
description: Secure secret management tool for AI agents. Inject secrets into commands via environment variables or file templates.
triggers:
  - "coffer"
  - "secret management"
  - "inject secrets"
  - "secure credentials"
---

# Coffer - Secure Secret Management

A CLI tool for managing and injecting secrets into commands, designed for AI agent workflows.

## Core Concepts

- **Namespaces**: Isolate secrets by environment (staging, production)
- **Injection Modes**: `env` (environment variables) or `file` (temporary files)
- **System Keychain**: Secrets stored in OS-native secure storage
- **Caller Detection**: Environment variable + process tree verification

## Commands

### Initialize Project
```bash
coffer init
```
Creates `.coffer` configuration file.

### Manage Secrets
```bash
# Add secret (interactive prompt)
coffer secret add <name> [--ns=<namespace>]

# List secrets
coffer secret list [--ns=<namespace>]

# Delete secret
coffer secret delete <name> [--ns=<namespace>]

# Check status
coffer check [--ns=<namespace>] [--json]
```

### Run Commands with Secrets
```bash
# Environment variable injection (default)
coffer run <command> [args...]

# File injection mode
coffer run --inject=file <command> [args...]

# Specific namespace
coffer run --ns=staging python app.py
```

## Configuration File (`.coffer`)

```yaml
default_ns: production
inject: env
secrets:
  db-pwd: "{{coffer:db-pwd}}"
  api-key: "{{coffer:api-key}}"
namespaces:
  staging:
    secrets:
      db-pwd: "{{coffer:db-pwd}}"
```

## Environment Variables

When running `coffer run`:
- `COFFER_NS`: Current namespace
- `COFFER_CALLER`: Set to "1" for caller detection
- Secret values: Uppercased, underscores for hyphens (e.g., `DB_PWD`)

## JSON Output (for Agents)

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

## Security Notes

- `secret get` is blocked in JSON mode (agent mode)
- Secrets stored in OS Keychain, not filesystem
- Caller verification via environment + process tree

## Platform Support

- **macOS**: Keychain via `security` command
- **Linux**: GNOME Keyring via `secret-tool`
- **Windows**: Credential Manager via `cmdkey`
- **Fallback**: File-based storage
