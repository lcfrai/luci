### Task 06: the migrate runner, `roles.sql`, the integration harness — and `pgx` pinned

**Lane A. Depends on: nothing. About 3 hours. Contract task H0.**

Migrations are plain SQL files embedded in the binary, applied in order under an advisory lock,
each recorded with the SHA-256 of its text so an edited, already-applied migration is refused
(ADR-H1 §5.6; contract §10.1). This task is the runner, the two role scripts, and the harness every
later integration test uses. It adds the module's one new direct requirement, `pgx/v5`, **pinned at
v5.7.5** (see the global constraint on the `go` directive).

The runner reads migrations from an `fs.FS` it is handed, rather than embedding them itself, so it
can be built and tested **before** task 07a writes `0001_init.sql`: `//go:embed sql/*.sql` with no
matching file is a compile error.

**Files:**
- `control-plane/internal/hub/migrate/migrate.go` + `migrate_test.go` (unit) + `migrate_integration_test.go` (new)
- `control-plane/internal/hub/migrate/roles.sql` — per environment, deployed (new)
- `control-plane/internal/hub/migrate/roles_local.sql` — the compose stack's variant (new)
- `control-plane/internal/hub/hubtest/hubtest.go` — the integration harness (new)
- `control-plane/go.mod`, `control-plane/go.sum` — `github.com/jackc/pgx/v5 v5.7.5` direct; its three
  `jackc` requirements indirect; `golang.org/x/sys`, `x/term`, `x/text` **bumped** and `x/crypto`,
  `x/sync` **added**, all indirect (Step 1); the `go` line to `go 1.23.0` (modify ⚑)
- `go.work` — its `go` line to `go 1.23.0`, no `use` or `toolchain` change; `go.work.sum` if `go get`
  writes it (modify ⚑, plan re-review b03 N2)

#### Contract — complete

```go
// control-plane/internal/hub/migrate/migrate.go

// Package migrate applies the Hub's embedded SQL migrations (contract §10.1).
// `hub migrate` is its only production caller; `hub serve` calls Current and
// never Apply.
package migrate

import (
	"context"
	"errors"
	"io/fs"
	"log/slog"

	"github.com/jackc/pgx/v5"
)

// Migration is one file, sql/NNNN_<name>.sql.
type Migration struct {
	Version int    // NNNN, from 1, contiguous
	Name    string // <name>: [a-z0-9_]+
	SQL     string
	SHA256  [32]byte // of the file's bytes, exactly as embedded
}

var (
	ErrEdited   = errors.New("migrate: an applied migration's text has changed; write a new migration instead")
	ErrBadName  = errors.New("migrate: a migration file must be named NNNN_<name>.sql, numbered from 0001 without gaps")
	ErrAhead    = errors.New("migrate: the database records a version this build does not have")
)

// Load reads every *.sql file at the root of fsys, validates the names, and
// returns them in version order.
func Load(fsys fs.FS) ([]Migration, error)

// LockName is the advisory lock Apply holds: pg_advisory_lock(hashtext(LockName)).
// The janitor's lock (task 24) is a different name, so a migration never waits
// on a sweep.
const LockName = "panacea-app-hub-migrate"

// Apply runs on conn — opened by the caller with PANACEA_APP_HUB_MIGRATE_DATABASE_URL —
// in this order: SET ROLE hub_owner; SELECT pg_advisory_lock(hashtext(LockName));
// CREATE SCHEMA IF NOT EXISTS hub AUTHORIZATION hub_owner; CREATE TABLE IF NOT
// EXISTS hub.schema_migrations (contract §10.1's DDL, verbatim); then, for each
// migration in ms not yet recorded, ONE transaction: the file's SQL, then
// INSERT INTO hub.schema_migrations (version, name, sha256). A recorded
// version whose sha256 differs is ErrEdited before anything is applied; a
// recorded version above the highest in ms is ErrAhead. The lock is released
// on every path. It returns the versions it applied, in order.
func Apply(ctx context.Context, conn *pgx.Conn, ms []Migration, log *slog.Logger) ([]int, error)

// Querier is the one method Current needs; *pgx.Conn and *pgxpool.Pool both have it.
type Querier interface {
	QueryRow(ctx context.Context, sql string, args ...any) pgx.Row
}

// Current returns max(version) in hub.schema_migrations, or 0 when the table
// does not exist yet. `hub serve`'s /readyz compares it with the build's.
func Current(ctx context.Context, q Querier) (int, error)
```

```sql
-- control-plane/internal/hub/migrate/roles.sql — contract §10.1, verbatim.
-- Run ONCE per environment by a database administrator, in database `hub`,
-- before the first `hub migrate`. Phase 8's deploy pipeline runs it (phase 8 task 29/30).
CREATE ROLE hub_owner NOLOGIN;   -- owns every object; used only by `hub migrate`
CREATE ROLE hub_app   NOLOGIN;   -- the running Hub; DML only, and not all of it
-- Deployed (phase 8, P8-D10, Entra-only auth): grant the roles to the workload identities.
--   GRANT hub_owner TO "id-hub-migrate";
--   GRANT hub_app   TO "id-hub";
-- Local (compose, trust auth on 127.0.0.1): the roles log in directly.
--   ALTER ROLE hub_owner LOGIN;  ALTER ROLE hub_app LOGIN;
GRANT CREATE ON DATABASE hub TO hub_owner;
```

```sql
-- control-plane/internal/hub/migrate/roles_local.sql — the compose stack's init script
-- (task 09 mounts it into /docker-entrypoint-initdb.d/). Local only: trust auth on
-- 127.0.0.1, so the roles log in directly and hold no password.
CREATE ROLE hub_owner LOGIN;
CREATE ROLE hub_app   LOGIN;
CREATE DATABASE hub;
GRANT CREATE ON DATABASE hub TO hub_owner;
-- The integration harness creates scratch databases as the admin and grants
-- CREATE on each to hub_owner itself (hubtest.New).
```

```go
// control-plane/internal/hub/hubtest/hubtest.go

// Package hubtest is the integration harness: a scratch database per test,
// migrated, and dropped when the test ends. Only _test.go files built with
// -tags integration import it.
package hubtest

import (
	"testing"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

// EnvAdminURL names the variable holding an administrator's URL for the local
// stack, e.g. postgres://postgres@127.0.0.1:5433/postgres?sslmode=disable.
const EnvAdminURL = "PANACEA_APP_HUB_TEST_DATABASE_URL"

// DB is one scratch database.
type DB struct {
	Name     string // hub_test_<8 hex>
	AdminURL string // …/<Name> as the administrator
	OwnerURL string // …/<Name> as hub_owner
	AppURL   string // …/<Name> as hub_app
}

// New skips the test (t.Skip, naming EnvAdminURL) when the variable is unset.
// Otherwise it creates the database, grants CREATE on it to hub_owner, and
// registers a t.Cleanup that runs DROP DATABASE … WITH (FORCE). It does NOT
// migrate: Migrated does, so task 06's own tests can start from empty.
func New(t *testing.T) *DB

// Migrated is New followed by migrate.Apply of the embedded migrations
// (task 07a's migrate.Embedded()), as hub_owner.
func Migrated(t *testing.T) *DB

// AppPool opens a pool as hub_app with search_path=hub, closed at cleanup.
func (d *DB) AppPool(t *testing.T) *pgxpool.Pool

// OwnerConn opens one connection as hub_owner, for tests that must tamper.
func (d *DB) OwnerConn(t *testing.T) *pgx.Conn
```

`Migrated` references `migrate.Embedded()`, which task 07a creates. Until then `hubtest.go` has
no `Migrated`; task 07a adds it in the same commit as `Embedded`.

#### Tests to write first

Unit tests (`migrate_test.go`, no database) use `testing/fstest.MapFS`.

| Test | Asserts | Mutant it kills |
|---|---|---|
| `TestLoadOrdersAndHashes` | `0002_b.sql`, `0001_a.sql` → versions `[1 2]`; `SHA256` of each equals `sha256.Sum256` of its bytes | lexical order by file name only; a hash of a trimmed or normalised text |
| `TestLoadRejectsBadNames` | `1_a.sql`, `0001-a.sql`, `0001_A.sql`, two `0001_*`, `0001` then `0003` (a gap), a `0000_x.sql` → `ErrBadName`; a non-`.sql` file is ignored | a gap that makes `Latest` skip a migration; two files racing for one version |

Integration tests (`migrate_integration_test.go`, `//go:build integration`), each on
`hubtest.New(t)`, each with a two- or three-file `MapFS`:

| Test | Asserts | Mutant it kills |
|---|---|---|
| `TestApplyToAnEmptyDatabase` | applies `[1 2]`; `hub.schema_migrations` holds both rows with the right `sha256`; `Current` = 2 | a runner that records before it applies |
| `TestReapplyIsANoOp` | a second `Apply` returns `[]` and changes no row | re-running DDL on every start |
| `TestAnEditedMigrationIsRefused` | change one byte of applied `0001`; `Apply` → `ErrEdited` and **`0003`, new, is not applied** | an edit silently accepted; a partial run past the refusal |
| `TestEachMigrationIsItsOwnTransaction` | `0002` has a syntax error; `Apply` fails; `0001` is recorded, `0002`'s first statement left nothing behind | one transaction for all (a failed 0003 undoing 0001 and 0002) or none (a half-applied 0002) |
| `TestConcurrentMigratorsSerialise` | two goroutines `Apply` on two connections; both return nil; each version is recorded once; one of them applied `[]` | two pods of a Job both applying 0001 |
| `TestDatabaseAheadIsRefused` | record version 9 by hand; `Apply` of `[1 2]` → `ErrAhead` | an old image migrating a newer schema backwards |
| `TestCurrentIsZeroBeforeAnyMigration` | on an empty database, `Current` = 0 and no error | `/readyz` 500-ing instead of reporting `schema` |
| `TestObjectsAreOwnedByHubOwner` | a table created by a migration has owner `hub_owner` (`pg_tables.tableowner`) | migrations run as the connecting admin, so `hub_app`'s grants and RLS stop meaning anything |

#### Steps

- [ ] **Step 1:** `cd control-plane && go get github.com/jackc/pgx/v5@v5.7.5`. pgx itself is in the
  download cache, but **five `golang.org/x` modules come with it — three bumped, two added** — because pgx v5.7.5 requires
  x/crypto v0.37.0, x/text v0.24.0 and x/sync v0.13.0, and x/crypto v0.37.0 requires x/sys v0.32.0
  and x/term v0.31.0 (both `.mod` files, read from the cache on 2026-09-24). Today
  `control-plane/go.mod:54-56` holds x/sys v0.24.0, x/term v0.21.0 and x/text v0.16.0, and has no
  x/crypto or x/sync line, so after this step the graph selects **x/sys v0.32.0, x/term v0.31.0 and
  x/text v0.24.0 (bumped), and x/crypto v0.37.0 and x/sync v0.13.0 (new indirect lines)** (or higher,
  if another requirement already asks for more). x/crypto and x/sync are BSD-3-Clause, like every
  `golang.org/x` module, so OD-11 holds. x/crypto's `x/net v0.21.0` is below go.mod:52's v0.26.0 and
  moves nothing (every `.mod` named here read from the cache on 2026-09-24). The cache holds
  neither `x/sys@v0.32.0` nor `x/term@v0.31.0`, so this `go get` **may need the network**, through the
  corporate proxy. If it cannot reach it, stop and record the blocker: do not pin an older x/sys or
  x/term to dodge the download, because that lowers x/crypto and with it pgx. Then `git diff go.mod`:
  the three bumped lines and the two added lines above are expected, and nothing else under
  `golang.org/x`. The `go` line becomes `go 1.23.0`, because pgx and x/crypto both declare it —
  accepted, it is the same floor spelled with a patch; any other change to it, or a `toolchain` line,
  is not: remove the `toolchain` line if one appears and re-run `go mod tidy`.
  **`go.work` moves too** (plan re-review b03 N2). Since Go 1.21 a workspace whose `go` line is lower
  than a member's refuses to load (*"module … listed in go.work file requires go >= 1.23.0, but
  go.work lists go 1.23"*), and `go 1.23` sorts below `1.23.0`; `go get` rewrites `go.work`'s line to
  `go 1.23.0` itself (seen on a scratch workspace with `go get go@1.23.0`, the same bump; the pgx `go
  get` was not run). If it did not, run `go work edit -go=1.23.0` from the root. Then `git diff go.work`: only
  `-go 1.23` / `+go 1.23.0`, no `use` line and no `toolchain` line. `sandboxd/go.mod` stays `go 1.23`
  (a member may sit below the workspace).
- [ ] **Step 2:** Start a Postgres for the integration tests. Task 09's stack is not built yet, so
  run the same image by hand (PowerShell):
  `docker run -d --name hub-pg-dev -p 127.0.0.1:5433:5432 -e POSTGRES_HOST_AUTH_METHOD=trust -v ${PWD}/control-plane/internal/hub/migrate/roles_local.sql:/docker-entrypoint-initdb.d/01-roles.sql:ro postgres:16-alpine`,
  then set `PANACEA_APP_HUB_TEST_DATABASE_URL=postgres://postgres@127.0.0.1:5433/postgres?sslmode=disable`.
- [ ] **Step 3:** Write the tests; they fail.
- [ ] **Step 4:** `migrate.go`, `hubtest.go`, the two SQL files. All pass, unit and integration.
- [ ] **Step 5:** Commit by pathspec: the seven files, `control-plane/go.mod`, `control-plane/go.sum`
  and `go.work` (and `go.work.sum` if Step 1 changed it). Without `go.work`, every root `go` command
  fails on every other checkout and in CI, while this tree works (plan re-review b03 N2). Leave the container
  for task 07a, or remove it: `docker rm -f hub-pg-dev`.

#### Acceptance

```bash
go test ./control-plane/internal/hub/migrate/ -race -count=1 -v 2>&1 | grep -c "^--- PASS"                      # 2  (unit only)
go test -tags integration ./control-plane/internal/hub/migrate/ -race -count=1 -v 2>&1 | grep -c "^--- PASS"     # 10
PANACEA_APP_HUB_TEST_DATABASE_URL= go test -tags integration ./control-plane/internal/hub/migrate/ -v -count=1 2>&1 | grep -c "^--- SKIP"   # 8
git diff control-plane/go.mod | grep "^[-+]go "        # at most: -go 1.23 / +go 1.23.0
grep -n "^toolchain" control-plane/go.mod              # no output
git diff HEAD~1 -- go.work | grep "^[-+][^-+]"         # at most: -go 1.23 / +go 1.23.0 (no use, no toolchain)
git show HEAD --name-only --format= | grep -cx "go.work"   # 1: go.work is in the commit
go list -m all > "$TEMP/golist.txt" 2>&1; echo $?     # 0, from the root after the commit: the committed go.work loads
go test ./sandboxd/... ./control-plane/... -race -count=1                                                     # ok
```

#### What this task does not claim

- That `roles.sql` runs unchanged on Azure Postgres Flexible Server with Entra-only auth; phase 8
  task 29's principals template runs after it and owns that.
- Any Hub table: task 07a; the guards and owner functions: task 07b.
- That `go get` succeeds behind the corporate proxy on a machine without the module cached; the
  owner's laptop has it cached (read 2026-09-23).
