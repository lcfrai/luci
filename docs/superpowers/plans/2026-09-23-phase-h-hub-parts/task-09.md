### Task 09: the local stack — Postgres in Docker, the Hub and `devissuer` on the host (correction A2); the import boundary test

**Lane B. Depends on: 07b, 08 (lane A) and 12. About 2 hours. Contract task H0.**

`PROGRAM.md`'s local-first rule: the whole stack runs on a laptop before any Azure spend. The
contract's §12.1 put all four services in `compose.yaml`; correction **A2** explains why that fails
here (no Go build inside Docker behind the proxy; `entra` refuses a non-loopback `http` authority;
the token issuer must be the same string for the browser, the control plane and the Hub). So:

- `compose.yaml` runs **Postgres only**, on `127.0.0.1:5433`, with the roles created at init.
- `devstack.sh` builds `hub` and `devissuer` **on the host**, migrates, starts both as background
  processes (PIDs in files, stopped by PID — never by name, and never with `go run`, whose child
  outlives its parent), waits for readiness, and prints the control plane's four variables.

Every port and variable of contract §12.1 is unchanged: `5433`, `8740`, `8750`, and the four
control-plane variables. This task also writes **the import boundary test** (ADR-H1 §4.1, contract
§2, correction A9), now that every binary it constrains exists.

**Files:**
- `control-plane/cmd/hub/compose.yaml` (new)
- `control-plane/cmd/hub/devstack.sh` (new, executable)
- `control-plane/cmd/hub/README.md` (new)
- `.gitignore` — add `control-plane/cmd/hub/.out/` (modify)
- `control-plane/internal/hub/boundary_test.go` (new)

#### Contract — complete

```yaml
# control-plane/cmd/hub/compose.yaml
# The Hub's local database, and nothing else (plan correction A2): devstack.sh
# runs devissuer and the Hub on the host, so the token issuer is the same
# http://127.0.0.1:8740 for the browser, the control plane and the Hub.
# Trust auth is safe only because the port is published on 127.0.0.1.
name: panacea-app-hub
services:
  db:
    image: postgres:16-alpine
    ports:
      - "127.0.0.1:5433:5432"
    environment:
      POSTGRES_HOST_AUTH_METHOD: trust
    volumes:
      - ../../internal/hub/migrate/roles_local.sql:/docker-entrypoint-initdb.d/01-roles.sql:ro
      - hubdb:/var/lib/postgresql/data
    healthcheck:
      test: ["CMD-SHELL", "pg_isready -U postgres -d hub"]
      interval: 2s
      timeout: 2s
      retries: 30
volumes:
  hubdb: {}
```

```bash
#!/usr/bin/env bash
# control-plane/cmd/hub/devstack.sh — the Hub's local stack, one command each way.
#
#   devstack.sh up       db up (and healthy) → build hub, devissuer → migrate → start
#                        devissuer → wait → start hub → wait for /readyz → print `env`
#   devstack.sh down     stop hub and devissuer BY PID, then stop db (data kept)
#   devstack.sh status   each process: running (pid) | stopped; db: healthy | …
#   devstack.sh env      the control plane's variables, as `export NAME=value` lines
#   devstack.sh reset    down, then `docker compose down -v` (the database is deleted)
#
# Binaries, logs and PID files live in control-plane/cmd/hub/.out/ (gitignored).
# Every value it needs comes from `devissuer env`, which reads dev-tenant.json:
# no jq, no second copy of an id.
set -euo pipefail
HERE="$(cd "$(dirname "$0")" && pwd)"
MODULE="$(cd "$HERE/../.." && pwd)"          # control-plane/
OUT="$HERE/.out"
EXE=""; case "$(uname -s)" in MINGW*|MSYS*|CYGWIN*) EXE=".exe" ;; esac
COMPOSE=(docker compose -f "$HERE/compose.yaml")
# … functions: build, start_bg NAME CMD…, wait_http URL SECONDS, stop_pid NAME, and the
# five verbs above, dispatched on "$1". Each wait fails with the last 20 log lines after
# 30 s rather than hanging. `up` is idempotent: a running process is left running.
```

The script's body is described, not verbatim; its **interface** above is the contract. The
environment it gives each process:

| Process | Variables |
|---|---|
| `hub migrate` | `PANACEA_APP_HUB_MIGRATE_DATABASE_URL=postgres://hub_owner@127.0.0.1:5433/hub?sslmode=disable` |
| `devissuer` | `PANACEA_APP_DEVISSUER_ADDR=127.0.0.1:8740`, `PANACEA_APP_DEVISSUER_CONFIG=$MODULE/cmd/devissuer/dev-tenant.json` |
| `hub serve` | `PANACEA_APP_HUB_ENV=local`, `PANACEA_APP_HUB_DEV_ISSUER=true`, `PANACEA_APP_HUB_ENTRA_AUTHORITY_HOST=http://127.0.0.1:8740`, `PANACEA_APP_HUB_DATABASE_URL=postgres://hub_app@127.0.0.1:5433/hub?sslmode=disable`, and `PANACEA_APP_HUB_TENANTS`, `_AUDIENCE`, `_ALLOWED_CLIENTS` from `devissuer env` |

`devstack.sh env` prints exactly (values from `dev-tenant.json`, task 11):

```text
export PANACEA_APP_HUB_URL=http://127.0.0.1:8750
export PANACEA_APP_ENTRA_AUTHORITY=http://127.0.0.1:8740/11111111-1111-4111-8111-111111111111
export PANACEA_APP_ENTRA_CLIENT_ID=22222222-2222-4222-8222-222222222222
export PANACEA_APP_HUB_SCOPE=api://33333333-3333-4333-8333-333333333333/Hub.Access
```

```go
// control-plane/internal/hub/boundary_test.go
package hub_test

// The rules, as data. A module-local package outside a binary's allowlist is a
// failure naming the importer; so is a third-party package outside the named
// closures. "pgx closure" is `go list -deps github.com/jackc/pgx/v5/pgxpool`,
// computed by the test, so pgx's own dependencies (x/crypto/pbkdf2, x/text,
// x/sync/semaphore, puddle) are allowed without being listed by hand.
var rules = []struct {
	pkg        string   // the main package or library under test
	local      []string // allowed module-local import paths (prefix match on …/control-plane/)
	thirdParty []string // allowed closures: "pgx", "x/time/rate", or none
	forbidden  []string // module-local prefixes that must NOT appear
}{
	{"./cmd/hub",
		[]string{"cmd/hub", "internal/hub/", "internal/httpx", "internal/ids", "internal/entra"},
		[]string{"pgx", "x/time/rate"},
		[]string{"internal/hub/hubtest", "internal/devissuer", "internal/api", "internal/backend",
			"internal/registry", "internal/credstore", "internal/entraauth"}},
	{"./cmd/devissuer", []string{"cmd/devissuer", "internal/devissuer"}, nil, nil},
	{"./cmd/api", nil, nil, // cmd/api keeps its own set; only the Hub side is constrained here
		[]string{"internal/hub/server", "internal/hub/store", "internal/hub/migrate",
			"internal/hub/hubtest", "internal/devissuer"}},
	{"./internal/hub/hubapi", []string{"internal/hub/hubapi", "internal/httpx"}, nil, nil},
	{"./internal/hub/hubclient", []string{"internal/hub/hubclient", "internal/hub/hubapi",
		"internal/httpx", "internal/ids"}, nil, nil},
}

// checkDeps is the pure half, so the test can prove it catches a violation.
func checkDeps(rule int, deps []string, pgxClosure map[string]bool) []string
```

Plus one rule over the whole module: **no package outside `internal/hub/...` imports
`internal/hub/server` or `internal/hub/store`** — from
`go list -f '{{.ImportPath}}{{range .Imports}} {{.}}{{end}}' ./...`.

#### Tests to write first

| Test | Asserts | Mutant it kills |
|---|---|---|
| `TestHubBinaryImportBoundary` | `go list -deps ./cmd/hub` passes rule 0; `./cmd/api` does not link `pgx` nor any forbidden Hub package | the Hub dragging in `client-go`, or the control plane linking the Hub's store (ADR-H1 R6) |
| `TestDevissuerIsStandardLibraryOnly` | `./cmd/devissuer`'s non-stdlib deps are exactly its own two packages | a JOSE library creeping into the issuer |
| `TestHubapiAndHubclientImports` | rules 3 and 4 (correction A9) | `hubclient` importing `entraauth` for a sentinel |
| `TestNothingOutsideTheHubImportsItsServer` | the module-wide rule | a control-plane handler calling `store` directly |
| `TestTheBoundaryCatchesAViolation` | `checkDeps(0, […, "…/control-plane/internal/api"], …)` returns one failure naming it; a stray `github.com/docker/docker/client` → one failure | a boundary test that always passes |

Manual checks for the script (Windows, Git Bash, Docker Desktop running), recorded in the commit
message with what was seen:

1. `control-plane/cmd/hub/devstack.sh up` ends by printing the four `export` lines; `curl -s
   http://127.0.0.1:8750/readyz` → `{"status":"ready"}`.
2. `devstack.sh status` → both running with PIDs; db healthy.
3. `devstack.sh down` → both stopped; `status` agrees; `docker ps` shows no `panacea-app-hub-db`
   running. **No `hub` or `devissuer` process is left** (`tasklist | grep -i -E "hub|devissuer"`
   empty).
4. `devstack.sh up` twice in a row leaves one of each process.

#### Steps

- [ ] **Step 1:** Stop the task-06 container if it is still running (`docker rm -f hub-pg-dev`):
  it holds port 5433.
- [ ] **Step 2:** The boundary tests; they pass or fail on the tree as it is — fix the tree, never
  the rule, and record any rule change in the commit.
- [ ] **Step 3:** `compose.yaml`, `devstack.sh`, `README.md`, the `.gitignore` line.
- [ ] **Step 4:** The four manual checks above.
- [ ] **Step 5:** Commit by pathspec. `git update-index --chmod=+x control-plane/cmd/hub/devstack.sh`
  after the `git add`, so the mode survives a Windows checkout.

#### Acceptance

```bash
go test ./control-plane/internal/hub/ -run 'Boundary|Imports|Standard|Outside|Catches' -count=1 -v 2>&1 | grep -c "^--- PASS"   # 5
control-plane/cmd/hub/devstack.sh up && curl -s http://127.0.0.1:8750/readyz                                                   # …{"status":"ready"}
control-plane/cmd/hub/devstack.sh down && control-plane/cmd/hub/devstack.sh status                                             # hub: stopped  devissuer: stopped  db: stopped
git check-ignore control-plane/cmd/hub/.out/hub.exe                                                                             # prints the path
```

#### What this task does not claim

- That Docker Desktop's `--wait` flag exists on the owner's version; `up` falls back to polling the
  health status if `docker compose up --wait` is refused, and says so.
- That the stack runs on macOS or Linux; it is written for them and checked only on Windows.
- A real Entra tenant: `devissuer` stands in (task 43's checklist covers the real one).
