---
name: coffer
description: Secure secret management for AI agents. Use when working with credentials, API keys, database passwords, AWS/ECR auth, template injection, or commands that need secrets without exposing plaintext.
triggers:
  - "coffer"
  - "secret management"
  - "inject secrets"
  - "secure credentials"
  - "AWS credentials"
  - "ECR login"
  - "database proxy"
  - "PostgreSQL proxy"
---

# Coffer - Secure Secret Management for Agents

Coffer stores secrets in `~/.coffer/` as owner-only-readable files (0600). Secrets are encrypted at rest with age (X25519) automatically when the age key exists. System keychain backends (macOS Keychain, Linux secret-tool, Windows Credential Manager) are available as opt-in.

Secrets are injected into commands via environment variables, file templates, or local database proxies. No plaintext secrets in project files, `.env`, or git history.

## Agent Rules

1. Prefer `coffer check --json` or `coffer secret list` to inspect readiness.
2. Do not use `coffer secret get` unless the user explicitly asks to reveal a value.
3. When a secret is missing, ask the user to run `coffer secret add <name>` rather than asking them to paste the value into chat.
4. Prefer `coffer run` for commands that need credentials.
5. Prefer `coffer inject` for rendering config templates.
6. Use `--global --ns=<namespace>` for shared machine/user secrets and environment-specific namespaces.
7. Environment injection preserves the exact secret name. Name secrets exactly as the target tool expects, such as `AWS_ACCESS_KEY_ID`, `AWS_SECRET_ACCESS_KEY`, or `db_password`.
8. Secrets are always stored in `~/.coffer/` (the store directory), regardless of `--global` flag.

## Architecture

| Component | Path | Description |
|-----------|------|-------------|
| Secret store | `~/.coffer/` | Age-encrypted `.json.age` files (or plain `.json` fallback) |
| Age encryption key | `~/.coffer/key` | Auto-generated on `coffer init` |
| Global config | `~/.config/coffer/config.yaml` | Created by `coffer init --global` |
| Local config | `./.coffer` | Created by `coffer init` |

Age encryption is transparent: if `~/.coffer/key` exists, `secret add` writes encrypted files and `secret get/list/run` decrypts on read. Plain `.json` secrets from before the encryption migration are still readable via fallback.

## Config Levels

Two levels are supported, merged at runtime (global base + local overrides):

| Level | Path | Initialize |
| --- | --- | --- |
| Global | `~/.config/coffer/config.yaml` | `coffer init --global` |
| Local | `./.coffer` | `coffer init` |

Use `--global` to operate on the global config only. Without it, local `.coffer` takes priority.

Namespace priority: CLI `--ns` > `COFFER_NS` environment variable > config `default_ns`.

## Core Commands

### Initialize

```bash
coffer init            # local config (.coffer) + age key (~/.coffer/key)
coffer init --global   # global config (~/.config/coffer/config.yaml) + age key
```

### Manage Secrets

```bash
coffer secret add <name> [--ns=<namespace>] [--global]
coffer secret update <name> [--ns=<namespace>] [--global]
coffer secret list [--ns=<namespace>] [--global]
coffer secret delete <name> [--ns=<namespace>] [--global]
coffer secret get <name> [--ns=<namespace>] [--global]
```

Secrets are always stored in `~/.coffer/` even when `--global` is not set — the flag only determines which config file to use.

`secret add` and `secret update` prompt interactively. Non-interactive stdin is supported:

```bash
printf '%s' "$value" | coffer secret add NAME --global --ns=dev
```

### Check Readiness

```bash
coffer check [--ns=<namespace>] [--global] [--json]
```

Use JSON mode for agents:

```bash
coffer check --global --ns=inference-dev --json
```

### Run Commands with Secrets

```bash
coffer run [--global] [--ns=<namespace>] [--inject=env|file] <command> [args...]
```

Env mode is default and preserves exact secret names:

```bash
coffer run --global --ns=inference-dev python main.py
```

If the secret is named `db_password`, the child process receives `db_password`. If the secret is named `AWS_ACCESS_KEY_ID`, the child process receives `AWS_ACCESS_KEY_ID`.

### Inject Templates

Render `{{coffer:name}}` placeholders from stored secrets:

```bash
coffer inject -i config.tmpl -o config.yaml [--global] [--ns=<namespace>]
echo 'token={{coffer:API_TOKEN}}' | coffer inject --global --ns=dev
```

Use this when a tool needs a config file rather than environment variables.

### Migrate `.env`

Quickly import an existing `.env` file into coffer. Sensitive keys (matching `PASSWORD`, `SECRET`, `KEY`, `TOKEN`, `AWS_*`, etc.) are stored in coffer; the rest stay in the template.

```bash
# Preview only
coffer migrate .env --global --ns=prod --dry-run

# Expected output:
# 🔐 Sensitive keys (will be migrated):
#   DATABASE_PASSWORD
#   API_SECRET_KEY
#   AWS_ACCESS_KEY_ID
#
# ⚠️  Found 3 sensitive key(s) to migrate.
# 📝 Template will be written to: .env.template
# 🔧 Dry-run mode: no secrets were stored.

# Execute migration
coffer migrate .env --global --ns=prod

# After migration, .env.template will look like:
#   DATABASE_PASSWORD={{coffer:DATABASE_PASSWORD}}
#   API_SECRET_KEY={{coffer:API_SECRET_KEY}}
#   AWS_ACCESS_KEY_ID={{coffer:AWS_ACCESS_KEY_ID}}
#   HOST=localhost            ← non-sensitive, kept as-is
#   PORT=5432                 ← non-sensitive, kept as-is

# Run with migrated secrets
coffer run --global --ns=prod python app.py
```

### PostgreSQL Database Proxy

Register a PostgreSQL connection:

```bash
coffer db add my-pg --host db.example.com --port 5432 --user admin --database app [--global]
```

Start a local proxy:

```bash
coffer db proxy my-pg [--global]
```

The proxy listens on `127.0.0.1:<port>`, authenticates to the real PostgreSQL server using the stored password, then relays traffic. Client tools connect to the local proxy without needing the database password.

## Common Agent Workflows

### Missing Secret

```bash
coffer check --global --ns=dev --json
```

If missing, tell the user:

```bash
coffer secret add SECRET_NAME --global --ns=dev
```

Do not ask the user to paste the value into chat.

### AWS / ECR Without `~/.aws/credentials`

Store AWS credentials with exact AWS environment variable names:

```bash
coffer secret add AWS_ACCESS_KEY_ID --global --ns=aws-dev
coffer secret add AWS_SECRET_ACCESS_KEY --global --ns=aws-dev
```

Run AWS CLI with injected env vars:

```bash
coffer run --global --ns=aws-dev aws sts get-caller-identity
```

For ECR login on a remote server, keep AWS credentials local and pipe only the temporary ECR token over SSH:

```bash
coffer run --global --ns=aws-dev aws ecr get-login-password --region ap-northeast-1 | \
  ssh user@server "docker login --username AWS --password-stdin <account>.dkr.ecr.ap-northeast-1.amazonaws.com"
```

### Render Temporary AWS Credentials File

For tools that insist on reading credentials from a file:

```bash
coffer run --global --ns=aws-dev --inject=file -- aws sts get-caller-identity
```

### Database Proxy Workflow

```bash
# Register
coffer db add my-pg --host db.example.com --port 5432 --user admin --database app --global

# Proxy (background)
coffer db proxy my-pg --global &

# Connect
psql -h 127.0.0.1 -p 5432 -U admin -d app
```

## System Keychain Opt-in

By default, coffer uses `~/.coffer/` file store. To use system keychain instead, set:

| Backend | Env var | Platform |
|---------|---------|----------|
| macOS Keychain | `COFFER_USE_KEYCHAIN=true` | macOS |
| Linux secret-tool | `COFFER_USE_SECRET_TOOL=true` | Linux |
| Windows Credential Manager | `COFFER_USE_CMDKEY=true` | Windows |

No additional dependencies required for the default file store on any platform.

## Upgrading

### Upgrade the coffer binary

```bash
# Option A: go install (requires Go)
go install github.com/huang-hf/coffer/cmd/coffer@latest

# Option B: build from source
git pull
go build -o ~/bin/coffer ./cmd/coffer
```

If you have an old coffer in `~/bin/coffer` but `go install` puts it in `~/go/bin/coffer`, make sure `PATH` picks up the right one.

### Update this skill

After upgrading the binary, update the installed skill:

```bash
coffer install-claude-code   # for Claude Code
coffer install-codex         # for Codex
```

Restart your agent session to pick up the updated skill.
