// Package migrate applies the Hub's embedded SQL migrations (contract §10.1).
// `hub migrate` is its only production caller; `hub serve` calls Current and
// never Apply.
package migrate

import (
	"context"
	"crypto/sha256"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"log/slog"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"time"

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
	ErrEdited  = errors.New("migrate: an applied migration's text has changed; write a new migration instead")
	ErrBadName = errors.New("migrate: a migration file must be named NNNN_<name>.sql, numbered from 0001 without gaps")
	ErrAhead   = errors.New("migrate: the database records a version this build does not have")
)

// fileName is NNNN_<name>.sql. Only a root-level name ending in ".sql" is a
// candidate at all; a candidate that does not match is refused, not skipped.
var fileName = regexp.MustCompile(`^([0-9]{4})_([a-z0-9_]+)\.sql$`)

// Load reads every *.sql file at the root of fsys, validates the names, and
// returns them in version order.
func Load(fsys fs.FS) ([]Migration, error) {
	entries, err := fs.ReadDir(fsys, ".")
	if err != nil {
		return nil, fmt.Errorf("migrate: read migrations: %w", err)
	}
	var ms []Migration
	for _, e := range entries {
		if !e.Type().IsRegular() || !strings.HasSuffix(e.Name(), ".sql") {
			continue
		}
		sub := fileName.FindStringSubmatch(e.Name())
		if sub == nil {
			return nil, fmt.Errorf("%w: %q", ErrBadName, e.Name())
		}
		v, _ := strconv.Atoi(sub[1]) // four ASCII digits: cannot fail
		data, err := fs.ReadFile(fsys, e.Name())
		if err != nil {
			return nil, fmt.Errorf("migrate: read %s: %w", e.Name(), err)
		}
		ms = append(ms, Migration{Version: v, Name: sub[2], SQL: string(data), SHA256: sha256.Sum256(data)})
	}
	sort.Slice(ms, func(i, j int) bool { return ms[i].Version < ms[j].Version })
	if err := contiguous(ms); err != nil {
		return nil, err
	}
	return ms, nil
}

// contiguous is ErrBadName unless ms holds versions 1..len(ms), in order, once each.
func contiguous(ms []Migration) error {
	for i, m := range ms {
		if m.Version != i+1 {
			return fmt.Errorf("%w: version %04d (%s) where %04d was expected", ErrBadName, m.Version, m.Name, i+1)
		}
	}
	return nil
}

// LockName is the advisory lock Apply holds: pg_advisory_lock(hashtext(LockName)).
// The janitor's lock (task 24) is a different name, so a migration never waits
// on a sweep.
const LockName = "panacea-app-hub-migrate"

// ddl is contract §10.1's, verbatim.
const ddl = `CREATE SCHEMA IF NOT EXISTS hub AUTHORIZATION hub_owner;
CREATE TABLE IF NOT EXISTS hub.schema_migrations (
  version    integer     PRIMARY KEY,
  name       text        NOT NULL,
  sha256     bytea       NOT NULL,
  applied_at timestamptz NOT NULL DEFAULT now()
);`

// Apply runs on conn — opened by the caller with PANACEA_APP_HUB_MIGRATE_DATABASE_URL —
// in this order: SET ROLE hub_owner; SELECT pg_advisory_lock(hashtext(LockName));
// CREATE SCHEMA IF NOT EXISTS hub AUTHORIZATION hub_owner; CREATE TABLE IF NOT
// EXISTS hub.schema_migrations (contract §10.1's DDL, verbatim); then, for each
// migration in ms not yet recorded, ONE transaction: the file's SQL, then
// INSERT INTO hub.schema_migrations (version, name, sha256). A recorded
// version whose sha256 differs is ErrEdited before anything is applied; a
// recorded version above the highest in ms is ErrAhead. The lock is released
// on every path. It returns the versions it applied, in order.
//
// A recorded version that ms does not hold at all (above its highest, or below
// 1) is ErrAhead: this build cannot vouch for it. A recorded version whose name
// differs from ms's is ErrEdited too: the record must name what ran. ms must be
// what Load returns, versions 1..n; anything else is ErrBadName before the
// database is touched. Once SET ROLE has succeeded, Apply ends with RESET ROLE
// on every path, so conn goes back to its session's own role.
func Apply(ctx context.Context, conn *pgx.Conn, ms []Migration, log *slog.Logger) (applied []int, err error) {
	if log == nil {
		log = slog.New(slog.NewTextHandler(io.Discard, nil))
	}
	if err := contiguous(ms); err != nil {
		return nil, err
	}
	if _, err := conn.Exec(ctx, "SET ROLE hub_owner"); err != nil {
		return nil, fmt.Errorf("migrate: set role hub_owner: %w", err)
	}
	// Deferred first, so it runs last: after the unlock.
	defer onFreshContext(&err, conn, "reset the role", "RESET ROLE")
	if _, err := conn.Exec(ctx, "SELECT pg_advisory_lock(hashtext($1))", LockName); err != nil {
		return nil, fmt.Errorf("migrate: take the advisory lock: %w", err)
	}
	defer onFreshContext(&err, conn, "release the advisory lock", "SELECT pg_advisory_unlock(hashtext($1))", LockName)
	if _, err := conn.Exec(ctx, ddl); err != nil {
		return nil, fmt.Errorf("migrate: create hub.schema_migrations: %w", err)
	}
	recorded, err := readRecorded(ctx, conn)
	if err != nil {
		return nil, err
	}
	vs := make([]int, 0, len(recorded))
	for v := range recorded {
		vs = append(vs, v)
	}
	sort.Ints(vs) // the same database, the same error, every run
	for _, v := range vs {
		if v < 1 || v > len(ms) {
			return nil, fmt.Errorf("%w: version %d is recorded; this build's highest is %d", ErrAhead, v, len(ms))
		}
		m, r := ms[v-1], recorded[v]
		if r.sum != m.SHA256 {
			return nil, fmt.Errorf("%w: version %04d (%s)", ErrEdited, m.Version, m.Name)
		}
		if r.name != m.Name {
			return nil, fmt.Errorf("%w: version %04d is recorded as %q; this build names it %q", ErrEdited, m.Version, r.name, m.Name)
		}
	}
	applied = []int{}
	for _, m := range ms {
		if _, done := recorded[m.Version]; done {
			continue
		}
		if err := applyOne(ctx, conn, m); err != nil {
			return applied, err
		}
		log.Info("migrate: applied", "version", m.Version, "name", m.Name)
		applied = append(applied, m.Version)
	}
	return applied, nil
}

// onFreshContext runs sql on conn for a deferred clean-up. ctx may be what
// ended the run, so it gets a context of its own; its error becomes *err only
// when nothing failed before it.
func onFreshContext(err *error, conn *pgx.Conn, what, sql string, args ...any) {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	if _, cerr := conn.Exec(ctx, sql, args...); cerr != nil && *err == nil {
		*err = fmt.Errorf("migrate: %s: %w", what, cerr)
	}
}

// record is one row of hub.schema_migrations.
type record struct {
	name string
	sum  [32]byte
}

// readRecorded is every row of hub.schema_migrations, by version.
func readRecorded(ctx context.Context, conn *pgx.Conn) (map[int]record, error) {
	rows, err := conn.Query(ctx, "SELECT version, name, sha256 FROM hub.schema_migrations")
	if err != nil {
		return nil, fmt.Errorf("migrate: read hub.schema_migrations: %w", err)
	}
	defer rows.Close()
	out := map[int]record{}
	for rows.Next() {
		var v int
		var r record
		var b []byte
		if err := rows.Scan(&v, &r.name, &b); err != nil {
			return nil, fmt.Errorf("migrate: read hub.schema_migrations: %w", err)
		}
		if len(b) != len(r.sum) {
			// A digest that is not 32 bytes matches nothing this build holds.
			return nil, fmt.Errorf("%w: version %d records a %d-byte sha256", ErrEdited, v, len(b))
		}
		copy(r.sum[:], b)
		out[v] = r
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("migrate: read hub.schema_migrations: %w", err)
	}
	return out, nil
}

// applyOne is one migration and its record, in one transaction: either both
// land or neither does.
func applyOne(ctx context.Context, conn *pgx.Conn, m Migration) error {
	tx, err := conn.Begin(ctx)
	if err != nil {
		return fmt.Errorf("migrate: begin %04d (%s): %w", m.Version, m.Name, err)
	}
	defer tx.Rollback(context.Background()) // a no-op once committed
	// No arguments: pgx sends it over the simple protocol, so a file may hold
	// many statements.
	if _, err := tx.Exec(ctx, m.SQL); err != nil {
		return fmt.Errorf("migrate: apply %04d (%s): %w", m.Version, m.Name, err)
	}
	if _, err := tx.Exec(ctx,
		"INSERT INTO hub.schema_migrations (version, name, sha256) VALUES ($1, $2, $3)",
		m.Version, m.Name, m.SHA256[:]); err != nil {
		return fmt.Errorf("migrate: record %04d (%s): %w", m.Version, m.Name, err)
	}
	if err := tx.Commit(ctx); err != nil {
		return fmt.Errorf("migrate: commit %04d (%s): %w", m.Version, m.Name, err)
	}
	return nil
}

// Querier is the one method Current needs; *pgx.Conn and *pgxpool.Pool both have it.
type Querier interface {
	QueryRow(ctx context.Context, sql string, args ...any) pgx.Row
}

// Current returns max(version) in hub.schema_migrations, or 0 when the table
// does not exist yet. `hub serve`'s /readyz compares it with the build's.
//
// Whether the table exists is read from the catalog, so a database no
// migration has touched (no schema hub at all) is 0, not an error.
func Current(ctx context.Context, q Querier) (int, error) {
	var exists bool
	if err := q.QueryRow(ctx, `SELECT EXISTS (
		SELECT 1 FROM pg_catalog.pg_class c
		JOIN pg_catalog.pg_namespace n ON n.oid = c.relnamespace
		WHERE n.nspname = 'hub' AND c.relname = 'schema_migrations')`).Scan(&exists); err != nil {
		return 0, fmt.Errorf("migrate: look for hub.schema_migrations: %w", err)
	}
	if !exists {
		return 0, nil
	}
	var v int
	if err := q.QueryRow(ctx, "SELECT coalesce(max(version), 0) FROM hub.schema_migrations").Scan(&v); err != nil {
		return 0, fmt.Errorf("migrate: read the schema version: %w", err)
	}
	return v, nil
}
