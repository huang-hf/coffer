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

## Config Levels

Two levels, merged at runtime:

| Level | Path | Command |
|-------|------|---------|
| **Global** | `~/.config/coffer/config.yaml` | `coffer init --global` |
| **Local** | `./.coffer` | `coffer init` |

**Resolution**: local settings (`default_ns`, `inject`, `config`) take full control. Secrets: global as base, local overrides/adds.

## Commands

### Initialize
```bash
coffer init              # local .coffer
coffer init --global     # global ~/.config/coffer/config.yaml
```

### Manage Secrets
```bash
coffer secret add <name> [--ns=<namespace>] [--global]
coffer secret update <name> [--ns=<namespace>] [--global]
coffer secret list [--ns=<namespace>] [--global]
coffer secret delete <name> [--ns=<namespace>] [--global]
```

### Run
```bash
coffer run <command> [args...]           # env mode (default)
coffer run --inject=file <command>       # file mode
coffer run --ns=staging python app.py   # namespace
```

### Check
```bash
coffer check [--ns=<namespace>] [--json]
```

## Namespace System

Isolate secrets by environment:

Priority: CLI `--ns` > env `COFFER_NS` > .coffer `default_ns`

## Agent Usage

```bash
# Check status (merged: local + global)
coffer check --json
# {"ready": true, "ns": "default", "secrets": [...]}

# Run with secrets
coffer run python app.py
```

## Platform Support

- **macOS**: Keychain via `security` command
- **Linux**: GNOME Keyring via `secret-tool`
- **Windows**: Credential Manager via `cmdkey`
