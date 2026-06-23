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

Coffer stores secrets in the OS credential store and injects them into commands, templates, or local database proxies. It is designed for agent workflows where raw credentials should not be printed, written into project files, or exposed unnecessarily.

## Agent Rules

1. Prefer `coffer check --json` or `coffer secret list` to inspect readiness.
2. Do not use `coffer secret get` unless the user explicitly asks to reveal a value.
3. When a secret is missing, ask the user to run `coffer secret add <name>` rather than asking them to paste the value into chat.
4. Prefer `coffer run` for commands that need credentials.
5. Prefer `coffer inject` for rendering config templates.
6. Use `--global --ns=<namespace>` for shared machine/user secrets and environment-specific namespaces.
7. Environment injection preserves the exact secret name. Name secrets exactly as the target tool expects, such as `AWS_ACCESS_KEY_ID`, `AWS_SECRET_ACCESS_KEY`, or `db_password`.

## Config Levels

Two levels are supported:

| Level | Path | Initialize |
| --- | --- | --- |
| Global | `~/.config/coffer/config.yaml` | `coffer init --global` |
| Local | `./.coffer` | `coffer init` |

Default behavior merges global and local config: global secrets/databases are the base, local entries override or add to them. Use `--global` to operate on the global config only.

Namespace priority: CLI `--ns` > `COFFER_NS` environment variable > config `default_ns`.

## Core Commands

### Initialize

```bash
coffer init
coffer init --global
```

### Manage Secrets

```bash
coffer secret add <name> [--ns=<namespace>] [--global]
coffer secret update <name> [--ns=<namespace>] [--global]
coffer secret list [--ns=<namespace>] [--global]
coffer secret delete <name> [--ns=<namespace>] [--global]
coffer secret get <name> [--ns=<namespace>] [--global]
```

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

Render `{{coffer:name}}` placeholders from keychain values:

```bash
coffer inject -i config.tmpl -o config.yaml [--global] [--ns=<namespace>]
echo 'token={{coffer:API_TOKEN}}' | coffer inject --global --ns=dev
```

Use this when a tool needs a config file rather than environment variables.

### Migrate `.env`

```bash
coffer migrate <env-file> [--global] [--ns=<namespace>] [--inject=env|file] [--template=<path>] [--dry-run] [--force]
```

Migrate sensitive keys from `.env` into the keychain and generate a template with `{{coffer:name}}` placeholders.

### PostgreSQL Database Proxy

Register a PostgreSQL connection:

```bash
coffer db add my-pg --host db.example.com --port 5432 --user admin --database app [--global]
```

Start a local proxy:

```bash
coffer db proxy my-pg [--global]
```

The proxy listens on `127.0.0.1:<port>`, authenticates to the real PostgreSQL server using the keychain password, then relays traffic. Client tools connect to the local proxy without needing the database password.

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

Use only when a tool requires `~/.aws/credentials`:

```ini
[default]
aws_access_key_id = {{coffer:AWS_ACCESS_KEY_ID}}
aws_secret_access_key = {{coffer:AWS_SECRET_ACCESS_KEY}}
```

```bash
coffer inject -i credentials.tmpl -o ~/.aws/credentials --global --ns=aws-dev
```

Remove generated credential files when no longer needed.

## Installing This Skill

```bash
coffer install-claude-code   # for Claude Code
coffer install-codex         # for Codex
```

These commands are embedded in the coffer binary. Running them again overwrites the skill with the latest version.

## Platform Support

- macOS: Keychain via `security`
- Linux: GNOME Keyring via `secret-tool`
- Windows: Credential Manager via `cmdkey`
