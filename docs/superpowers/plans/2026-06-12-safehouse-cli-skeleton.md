# SafeHouse CLI Skeleton Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Build the first Go skeleton for the `safehouse` CLI, including a manual localhost daemon, authenticated status API, local token store, and DB/AWS capability stubs.

**Architecture:** One Go binary owns the CLI, daemon, store, and capability registry. The daemon listens on `127.0.0.1:4317`, exposes JSON endpoints under `/v1`, and protects non-health routes with a locally stored bearer token.

**Tech Stack:** Go standard library, `net/http`, `encoding/json`, `flag`-style command parsing, Go tests.

---

## File Structure

- Create `go.mod`: module definition.
- Create `cmd/safehouse/main.go`: binary entry point that delegates to `internal/cli`.
- Create `internal/version/version.go`: central version constant.
- Create `internal/capability/capability.go`: capability type, registry, default DB/AWS stubs.
- Create `internal/capability/capability_test.go`: registry tests.
- Create `internal/store/store.go`: SafeHouse home path, directory creation, token read/create.
- Create `internal/store/store_test.go`: store permission and token tests.
- Create `internal/daemon/server.go`: HTTP server, routing, auth middleware, JSON responses.
- Create `internal/daemon/server_test.go`: health/status/auth tests.
- Create `internal/cli/cli.go`: command parsing, daemon start, API client commands, user-facing errors.
- Create `internal/cli/cli_test.go`: CLI parsing and daemon-not-running tests.
- Create `.gitignore`: build artifacts and local temp files.

## Task 1: Initialize Go Project

**Files:**
- Create: `go.mod`
- Create: `.gitignore`
- Create: `cmd/safehouse/main.go`
- Create: `internal/version/version.go`

- [ ] **Step 1: Write the minimal entrypoint and version package**

Create `go.mod`:

```go
module safehouse

go 1.22
```

Create `.gitignore`:

```gitignore
/safehouse
/bin/
*.test
```

Create `internal/version/version.go`:

```go
package version

const Version = "0.1.0-dev"
```

Create `cmd/safehouse/main.go`:

```go
package main

import (
	"os"

	"safehouse/internal/cli"
)

func main() {
	os.Exit(cli.Run(os.Args[1:], os.Stdout, os.Stderr))
}
```

- [ ] **Step 2: Run compile check**

Run: `go test ./...`

Expected: fails until `internal/cli` exists in the next task.

- [ ] **Step 3: Commit**

Run:

```bash
git add go.mod .gitignore cmd/safehouse/main.go internal/version/version.go
git commit -m "chore: initialize Go project"
```

## Task 2: Capability Registry

**Files:**
- Create: `internal/capability/capability.go`
- Create: `internal/capability/capability_test.go`

- [ ] **Step 1: Write failing capability tests**

Test that default capabilities include `db` and `aws`, that each stub is unavailable and unconfigured, and that unknown names are not found.

- [ ] **Step 2: Run failing tests**

Run: `go test ./internal/capability`

Expected: FAIL because package does not exist or exported functions are missing.

- [ ] **Step 3: Implement registry**

Implement:

- `type Capability`
- `type Registry`
- `func NewDefaultRegistry() *Registry`
- `func (r *Registry) List() []Capability`
- `func (r *Registry) Get(name string) (Capability, bool)`

Default `db` and `aws` capabilities return:

```json
{
  "configured": false,
  "available": false,
  "reason": "not implemented in skeleton"
}
```

- [ ] **Step 4: Run tests**

Run: `go test ./internal/capability`

Expected: PASS.

- [ ] **Step 5: Commit**

Run:

```bash
git add internal/capability
git commit -m "feat: add capability registry stubs"
```

## Task 3: Local Store and Token

**Files:**
- Create: `internal/store/store.go`
- Create: `internal/store/store_test.go`

- [ ] **Step 1: Write failing store tests**

Cover:

- SafeHouse home directory is created with `0700`.
- `daemon.token` is created with `0600`.
- A generated token is reused on subsequent calls.
- Empty or whitespace token files are treated as invalid and regenerated or rejected consistently.

- [ ] **Step 2: Run failing tests**

Run: `go test ./internal/store`

Expected: FAIL because package does not exist or functions are missing.

- [ ] **Step 3: Implement store**

Implement:

- `type Store struct`
- `func New(baseDir string) *Store`
- `func DefaultDir() (string, error)`
- `func (s *Store) Ensure() error`
- `func (s *Store) TokenPath() string`
- `func (s *Store) ReadOrCreateToken() (string, error)`

Use `crypto/rand` and base64 URL encoding for token generation.

- [ ] **Step 4: Run tests**

Run: `go test ./internal/store`

Expected: PASS.

- [ ] **Step 5: Commit**

Run:

```bash
git add internal/store
git commit -m "feat: add local store token handling"
```

## Task 4: Daemon HTTP API

**Files:**
- Create: `internal/daemon/server.go`
- Create: `internal/daemon/server_test.go`
- Modify: `internal/capability/capability.go`
- Use: `internal/version/version.go`

- [ ] **Step 1: Write failing daemon tests**

Cover:

- `GET /v1/health` returns 200 without auth.
- `GET /v1/status` returns 401 without auth.
- `GET /v1/status` returns version, listen address, and capability names with valid auth.
- `GET /v1/capabilities/db` returns the DB stub.
- Unknown capability route returns 404.

- [ ] **Step 2: Run failing tests**

Run: `go test ./internal/daemon`

Expected: FAIL because package does not exist or handlers are missing.

- [ ] **Step 3: Implement daemon server**

Implement:

- `type Server`
- `type Config`
- `func New(Config) *Server`
- `func (s *Server) Handler() http.Handler`
- `func (s *Server) ListenAndServe(ctx context.Context) error`

Use only `127.0.0.1:4317` as the default listen address. All protected routes require `Authorization: Bearer <token>`.

- [ ] **Step 4: Run daemon tests**

Run: `go test ./internal/daemon`

Expected: PASS.

- [ ] **Step 5: Commit**

Run:

```bash
git add internal/daemon internal/capability internal/version
git commit -m "feat: add authenticated daemon API"
```

## Task 5: CLI Commands and Daemon Client

**Files:**
- Create: `internal/cli/cli.go`
- Create: `internal/cli/cli_test.go`
- Modify: `cmd/safehouse/main.go`

- [ ] **Step 1: Write failing CLI tests**

Cover:

- No args prints usage and returns non-zero.
- Unknown command prints usage and returns non-zero.
- `status` reports daemon-not-running when it cannot connect.
- `db list` maps to `/v1/capabilities/db`.
- `aws status` maps to `/v1/capabilities/aws`.

- [ ] **Step 2: Run failing tests**

Run: `go test ./internal/cli`

Expected: FAIL because package does not exist or commands are missing.

- [ ] **Step 3: Implement CLI**

Implement:

- `func Run(args []string, stdout io.Writer, stderr io.Writer) int`
- `daemon` command starts the daemon in the foreground.
- `status`, `db list`, and `aws status` call the daemon over HTTP.
- CLI reads the token from `~/.safehouse/daemon.token`.
- Connection refusal prints `SafeHouse daemon is not running. Start it with: safehouse daemon`.

- [ ] **Step 4: Run CLI tests**

Run: `go test ./internal/cli`

Expected: PASS.

- [ ] **Step 5: Run all tests**

Run: `go test ./...`

Expected: PASS.

- [ ] **Step 6: Commit**

Run:

```bash
git add cmd/safehouse/main.go internal/cli
git commit -m "feat: add SafeHouse CLI commands"
```

## Task 6: Manual Acceptance Verification

**Files:**
- Modify only if verification exposes defects.

- [ ] **Step 1: Run full test suite**

Run: `go test ./...`

Expected: PASS.

- [ ] **Step 2: Start daemon**

Run: `go run ./cmd/safehouse daemon`

Expected: daemon listens on `127.0.0.1:4317` and stays in the foreground.

- [ ] **Step 3: Query status in another shell**

Run: `go run ./cmd/safehouse status`

Expected: prints version, listen address, and `db, aws`.

- [ ] **Step 4: Query DB capability**

Run: `go run ./cmd/safehouse db list`

Expected: prints DB stub with `configured: false` and `available: false`.

- [ ] **Step 5: Query AWS capability**

Run: `go run ./cmd/safehouse aws status`

Expected: prints AWS stub with `configured: false` and `available: false`.

- [ ] **Step 6: Commit verification fixes if needed**

Run only if changes were made:

```bash
git add .
git commit -m "fix: address skeleton verification issues"
```
