# SafeHouse CLI Skeleton Design

Date: 2026-06-12
Status: Approved for implementation planning

## Context

SafeHouse is a local capability broker for AI agents. Its long-term purpose is to let agents use infrastructure without receiving raw credentials. The v0.1 product scope includes Database Capability and AWS Capability, but the first implementation slice will build only the project skeleton and local broker shape.

This first slice intentionally avoids real database and AWS integrations. It establishes the command layout, daemon lifecycle, local state handling, authentication boundary, capability registry, and tests needed before adding concrete capabilities.

## Goals

- Create a new standalone Go project at `safehouse`.
- Build one binary named `safehouse`.
- Support a manually started local daemon with `safehouse daemon`.
- Provide CLI commands that call the daemon over localhost.
- Register `db` and `aws` capabilities as stubs.
- Avoid storing real credentials in the skeleton.
- Keep the design simple enough for fast MVP iteration.

## Non-Goals

- No real MySQL, PostgreSQL, or database proxying.
- No real AWS SSO, STS, CLI credential process, or SDK integration.
- No background auto-start service.
- No external plugin process model.
- No project directory scanning or secret scanning.
- No filesystem sandboxing or agent sandboxing.

## Architecture

The first version uses a single Go binary. The binary contains the CLI, daemon, store, and capability registry in one process/package tree.

Recommended project layout:

```text
safehouse/
  cmd/
    safehouse/
      main.go
  internal/
    capability/
    cli/
    daemon/
    store/
    version/
  docs/
    superpowers/
      specs/
        2026-06-12-safehouse-cli-skeleton-design.md
```

Package responsibilities:

- `cmd/safehouse`: binary entry point.
- `internal/cli`: command parsing and user-facing command behavior.
- `internal/daemon`: localhost HTTP server and API handlers.
- `internal/store`: local SafeHouse directory, config, state, token, and permissions.
- `internal/capability`: in-process registry for known capabilities.
- `internal/version`: version constants used by CLI and daemon responses.

The single-binary approach is chosen for speed and clarity. It avoids a premature plugin abstraction while still keeping package boundaries clear enough to evolve later.

## CLI Commands

Initial command surface:

```bash
safehouse daemon
safehouse status
safehouse db list
safehouse aws status
```

Command behavior:

- `safehouse daemon` starts the localhost daemon and blocks in the foreground.
- `safehouse status` reads the daemon token and calls `GET /v1/status`.
- `safehouse db list` calls the DB capability endpoint and shows stub state.
- `safehouse aws status` calls the AWS capability endpoint and shows stub state.

The CLI should print concise, stable messages. It must not print daemon tokens or future credential material.

## Local State

SafeHouse stores local state under the user's home directory:

```text
~/.safehouse/
  config.json
  state.json
  daemon.token
  safehouse.log
```

Initial requirements:

- Directory permission: `0700`.
- Sensitive files permission: `0600`.
- The skeleton must not store real DB or AWS credentials.
- `daemon.token` is generated once and reused by daemon and CLI commands.
- If local state cannot be created or read, commands fail with a clear path and permission-oriented error.

The exact JSON schema can stay minimal in the skeleton. It only needs enough structure to support versioned future growth.

## Daemon

The daemon listens only on localhost:

```text
127.0.0.1:4317
```

Initial daemon behavior:

- Start via `safehouse daemon`.
- Read or create `~/.safehouse/daemon.token`.
- Serve HTTP API routes under `/v1`.
- Require `Authorization: Bearer <token>` for non-health endpoints.
- Return JSON responses.
- Fail clearly if the configured port is unavailable.

## Daemon API

Initial routes:

```text
GET /v1/health
GET /v1/status
GET /v1/capabilities
GET /v1/capabilities/db
GET /v1/capabilities/aws
```

`GET /v1/health`:

- No authentication required.
- Returns a minimal health response for local checks.

`GET /v1/status`:

- Authentication required.
- Returns daemon version, uptime or start time, listen address, and registered capability names.

`GET /v1/capabilities`:

- Authentication required.
- Returns all registered capabilities and their stub status.

`GET /v1/capabilities/db`:

- Authentication required.
- Returns DB capability stub state:

```json
{
  "name": "db",
  "configured": false,
  "available": false,
  "reason": "not implemented in skeleton"
}
```

`GET /v1/capabilities/aws`:

- Authentication required.
- Returns AWS capability stub state:

```json
{
  "name": "aws",
  "configured": false,
  "available": false,
  "reason": "not implemented in skeleton"
}
```

## Error Handling

Required user-facing cases:

- Daemon not running: tell the user to start it with `safehouse daemon`.
- Token missing or invalid: return an unauthorized error without printing token content.
- Port already in use: fail daemon startup and identify the listen address.
- State directory creation failure: print the affected path and permission-oriented guidance.
- Non-2xx API response: print a stable summary without leaking internal details.

Implementation should preserve internal errors for tests and debugging while keeping CLI output clean.

## Testing

Required tests:

- `internal/store`: directory creation, permission mode, token generation, token reuse.
- `internal/daemon`: `/v1/health`, authenticated `/v1/status`, unauthorized request handling.
- `internal/capability`: `db` and `aws` stubs are registered and queryable.
- CLI layer: command parsing and daemon-not-running message.

No real DB or AWS integration tests belong in this slice.

## Acceptance Criteria

The implementation is acceptable when these commands work as expected:

```bash
go test ./...
go run ./cmd/safehouse daemon
go run ./cmd/safehouse status
go run ./cmd/safehouse db list
go run ./cmd/safehouse aws status
```

The daemon must bind only to `127.0.0.1`, use bearer-token authentication for protected routes, and avoid storing or printing real credentials.

## Future Extension Points

The single-binary design should not block future extraction of:

- Real DB proxy capability.
- Real AWS SSO/STS broker capability.
- Auto-start daemon support.
- More capability modules such as OpenAI, Anthropic, GitHub, Kubernetes, Docker, and SSH.

Those features remain outside this skeleton and should each get a separate spec before implementation.
