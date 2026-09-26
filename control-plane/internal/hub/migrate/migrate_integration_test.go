//go:build integration

package migrate_test

import (
	"bytes"
	"context"
	"crypto/sha256"
	"errors"
	"sync"
	"testing"
	"testing/fstest"
	"time"

	"github.com/jackc/pgx/v5"

	"github.com/kpmg-au-emu/panacea-app/control-plane/internal/hub/hubtest"
	"github.com/kpmg-au-emu/panacea-app/control-plane/internal/hub/migrate"
)

func ctxT(t *testing.T) context.Context {
	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	t.Cleanup(cancel)
	return ctx
}

// load is migrate.Load of files, name to text; a load error fails the test.
func load(t *testing.T, files map[string]string) []migrate.Migration {
	t.Helper()
	fsys := fstest.MapFS{}
	for n, s := range files {
		fsys[n] = &fstest.MapFile{Data: []byte(s)}
	}
	ms, err := migrate.Load(fsys)
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	return ms
}

func adminConn(t *testing.T, d *hubtest.DB) *pgx.Conn {
	t.Helper()
	conn, err := pgx.Connect(ctxT(t), d.AdminURL)
	if err != nil {
		t.Fatalf("connect as the administrator: %v", err)
	}
	t.Cleanup(func() { conn.Close(context.Background()) })
	return conn
}

type row struct {
	version int
	name    string
	sha     []byte
	at      time.Time
}

// recorded is every row of hub.schema_migrations, in version order.
func recorded(t *testing.T, conn *pgx.Conn) []row {
	t.Helper()
	rows, err := conn.Query(ctxT(t), "SELECT version, name, sha256, applied_at FROM hub.schema_migrations ORDER BY version")
	if err != nil {
		t.Fatalf("read hub.schema_migrations: %v", err)
	}
	var out []row
	for rows.Next() {
		var r row
		if err := rows.Scan(&r.version, &r.name, &r.sha, &r.at); err != nil {
			t.Fatalf("scan: %v", err)
		}
		out = append(out, r)
	}
	if err := rows.Err(); err != nil {
		t.Fatalf("rows: %v", err)
	}
	return out
}

func tableExists(t *testing.T, conn *pgx.Conn, name string) bool {
	t.Helper()
	var ok bool
	if err := conn.QueryRow(ctxT(t), "SELECT to_regclass($1) IS NOT NULL", name).Scan(&ok); err != nil {
		t.Fatalf("to_regclass(%s): %v", name, err)
	}
	return ok
}

func sameInts(a, b []int) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}

var two = map[string]string{
	"0001_a.sql": "CREATE TABLE hub.a (x int);\nINSERT INTO hub.a VALUES (1);\n",
	"0002_b.sql": "CREATE TABLE hub.b (y int);\n",
}

func TestApplyToAnEmptyDatabase(t *testing.T) {
	d := hubtest.New(t)
	conn := d.OwnerConn(t)
	ms := load(t, two)
	// A slice that is not versions 1..n is refused before the database is touched.
	if got, err := migrate.Apply(ctxT(t), conn, ms[1:], nil); !errors.Is(err, migrate.ErrBadName) || len(got) != 0 {
		t.Fatalf("Apply([2]) = %v, %v; want nothing applied, ErrBadName", got, err)
	}
	if tableExists(t, conn, "hub.schema_migrations") || tableExists(t, conn, "hub.b") {
		t.Fatalf("Apply of a gapped slice touched the database")
	}
	got, err := migrate.Apply(ctxT(t), conn, ms, nil)
	if err != nil || !sameInts(got, []int{1, 2}) {
		t.Fatalf("Apply = %v, %v; want [1 2], nil", got, err)
	}
	// The lock is free after a successful Apply, with its connection still open.
	var free bool
	if err := d.OwnerConn(t).QueryRow(ctxT(t), "SELECT pg_try_advisory_lock(hashtext($1))", migrate.LockName).Scan(&free); err != nil || !free {
		t.Fatalf("pg_try_advisory_lock after Apply = %v, %v; the lock was kept", free, err)
	}
	rs := recorded(t, conn)
	if len(rs) != 2 {
		t.Fatalf("recorded %d rows, want 2", len(rs))
	}
	for i, r := range rs {
		want := sha256.Sum256([]byte(two[[]string{"0001_a.sql", "0002_b.sql"}[i]]))
		if r.version != i+1 || r.name != ms[i].Name || !bytes.Equal(r.sha, want[:]) || r.at.IsZero() {
			t.Fatalf("row %d = %+v; want version %d, name %s, the file's sha256", i, r, i+1, ms[i].Name)
		}
	}
	if !tableExists(t, conn, "hub.a") || !tableExists(t, conn, "hub.b") {
		t.Fatalf("a migration was recorded but its table is missing")
	}
	var n int
	if err := conn.QueryRow(ctxT(t), "SELECT count(*) FROM hub.a").Scan(&n); err != nil || n != 1 {
		t.Fatalf("hub.a holds %d rows (%v); want 1: every statement of the file ran", n, err)
	}
	if v, err := migrate.Current(ctxT(t), conn); err != nil || v != 2 {
		t.Fatalf("Current = %d, %v; want 2", v, err)
	}
}

func TestReapplyIsANoOp(t *testing.T) {
	d := hubtest.New(t)
	conn := d.OwnerConn(t)
	ms := load(t, two)
	if _, err := migrate.Apply(ctxT(t), conn, ms, nil); err != nil {
		t.Fatalf("first Apply: %v", err)
	}
	before := recorded(t, conn)
	got, err := migrate.Apply(ctxT(t), conn, ms, nil)
	if err != nil || got == nil || len(got) != 0 {
		t.Fatalf("second Apply = %#v, %v; want []int{}, nil", got, err)
	}
	after := recorded(t, conn)
	if len(after) != len(before) {
		t.Fatalf("rows %d -> %d", len(before), len(after))
	}
	for i := range before {
		if before[i].version != after[i].version || !before[i].at.Equal(after[i].at) || !bytes.Equal(before[i].sha, after[i].sha) {
			t.Fatalf("row %d changed: %+v -> %+v", i, before[i], after[i])
		}
	}
	var n int
	if err := conn.QueryRow(ctxT(t), "SELECT count(*) FROM hub.a").Scan(&n); err != nil || n != 1 {
		t.Fatalf("hub.a holds %d rows (%v); want 1: 0001 ran again", n, err)
	}
}

func TestAnEditedMigrationIsRefused(t *testing.T) {
	d := hubtest.New(t)
	conn := d.OwnerConn(t)
	if _, err := migrate.Apply(ctxT(t), conn, load(t, two), nil); err != nil {
		t.Fatalf("first Apply: %v", err)
	}
	edited := map[string]string{
		"0001_a.sql": "CREATE TABLE hub.a (x int);\nINSERT INTO hub.a VALUES (2);\n", // one byte
		"0002_b.sql": two["0002_b.sql"],
		"0003_c.sql": "CREATE TABLE hub.c (z int);\n",
	}
	got, err := migrate.Apply(ctxT(t), conn, load(t, edited), nil)
	if !errors.Is(err, migrate.ErrEdited) || len(got) != 0 {
		t.Fatalf("Apply = %v, %v; want nothing applied, ErrEdited", got, err)
	}
	if tableExists(t, conn, "hub.c") || len(recorded(t, conn)) != 2 {
		t.Fatalf("0003 was applied past the refusal")
	}
	if v, err := migrate.Current(ctxT(t), conn); err != nil || v != 2 {
		t.Fatalf("Current = %d, %v; want 2", v, err)
	}
}

// TestEachMigrationIsItsOwnTransaction: 0002's second statement fails at run
// time (not at parse time), after its first has run; 0001 stays, 0002 leaves
// nothing, and 0003 is never reached.
func TestEachMigrationIsItsOwnTransaction(t *testing.T) {
	d := hubtest.New(t)
	conn := d.OwnerConn(t)
	ms := load(t, map[string]string{
		"0001_a.sql": "CREATE TABLE hub.a (x int);\n",
		"0002_b.sql": "CREATE TABLE hub.b (y int);\nINSERT INTO hub.no_such_table VALUES (1);\n",
		"0003_c.sql": "CREATE TABLE hub.c (z int);\n",
	})
	got, err := migrate.Apply(ctxT(t), conn, ms, nil)
	if err == nil {
		t.Fatalf("Apply = %v, nil; want 0002's error", got)
	}
	if !sameInts(got, []int{1}) {
		t.Fatalf("Apply applied %v; want [1]", got)
	}
	rs := recorded(t, conn)
	if len(rs) != 1 || rs[0].version != 1 {
		t.Fatalf("recorded %+v; want 0001 alone", rs)
	}
	if !tableExists(t, conn, "hub.a") {
		t.Fatalf("0001's table is gone: one transaction for all")
	}
	if tableExists(t, conn, "hub.b") {
		t.Fatalf("0002's first statement left hub.b behind: a half-applied migration")
	}
	if tableExists(t, conn, "hub.c") {
		t.Fatalf("0003 ran after 0002 failed")
	}
	// The lock was released on the failing path: a second connection takes it.
	other := d.OwnerConn(t)
	var ok bool
	if err := other.QueryRow(ctxT(t), "SELECT pg_try_advisory_lock(hashtext($1))", migrate.LockName).Scan(&ok); err != nil || !ok {
		t.Fatalf("pg_try_advisory_lock = %v, %v; the failed Apply still holds the lock", ok, err)
	}
}

func TestConcurrentMigratorsSerialise(t *testing.T) {
	d := hubtest.New(t)
	ms := load(t, map[string]string{
		// The sleep holds 0001 open, so without the lock both runners are in it.
		"0001_a.sql": "SELECT pg_sleep(0.5);\nCREATE TABLE hub.a (x int);\n",
		"0002_b.sql": "CREATE TABLE hub.b (y int);\n",
	})
	conns := []*pgx.Conn{d.OwnerConn(t), d.OwnerConn(t)}
	got := make([][]int, 2)
	errs := make([]error, 2)
	// Bounded: a runner that kept the lock makes the other fail here, not hang.
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	var start, wg sync.WaitGroup
	start.Add(1)
	for i := range conns {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			start.Wait()
			got[i], errs[i] = migrate.Apply(ctx, conns[i], ms, nil)
		}(i)
	}
	start.Done()
	wg.Wait()
	for i := range errs {
		if errs[i] != nil {
			t.Fatalf("runner %d: %v", i, errs[i])
		}
	}
	one, other := got[0], got[1]
	if len(one) != 0 {
		one, other = other, one
	}
	if len(one) != 0 || !sameInts(other, []int{1, 2}) {
		t.Fatalf("applied %v and %v; want [] and [1 2]", got[0], got[1])
	}
	rs := recorded(t, conns[0])
	if len(rs) != 2 || rs[0].version != 1 || rs[1].version != 2 {
		t.Fatalf("recorded %+v; want each version once", rs)
	}
}

func TestDatabaseAheadIsRefused(t *testing.T) {
	d := hubtest.New(t)
	conn := d.OwnerConn(t)
	// Apply of nothing creates the schema and the table, and applies nothing.
	if got, err := migrate.Apply(ctxT(t), conn, nil, nil); err != nil || len(got) != 0 {
		t.Fatalf("Apply(nil) = %v, %v; want [], nil", got, err)
	}
	sum := sha256.Sum256([]byte("from a newer build"))
	if _, err := conn.Exec(ctxT(t), "INSERT INTO hub.schema_migrations (version, name, sha256) VALUES (9, 'newer', $1)", sum[:]); err != nil {
		t.Fatalf("record version 9 by hand: %v", err)
	}
	got, err := migrate.Apply(ctxT(t), conn, load(t, two), nil)
	if !errors.Is(err, migrate.ErrAhead) || len(got) != 0 {
		t.Fatalf("Apply = %v, %v; want nothing applied, ErrAhead", got, err)
	}
	if tableExists(t, conn, "hub.a") || len(recorded(t, conn)) != 1 {
		t.Fatalf("an old build migrated a newer schema")
	}
}

func TestCurrentIsZeroBeforeAnyMigration(t *testing.T) {
	d := hubtest.New(t)
	conn := d.OwnerConn(t)
	if v, err := migrate.Current(ctxT(t), conn); err != nil || v != 0 {
		t.Fatalf("Current on an empty database = %d, %v; want 0, nil", v, err)
	}
	// The table exists but holds no row: still 0.
	if _, err := migrate.Apply(ctxT(t), conn, nil, nil); err != nil {
		t.Fatalf("Apply(nil): %v", err)
	}
	if v, err := migrate.Current(ctxT(t), conn); err != nil || v != 0 {
		t.Fatalf("Current on an empty table = %d, %v; want 0, nil", v, err)
	}
}

// TestObjectsAreOwnedByHubOwner: run as the administrator (a superuser), Apply
// still creates everything as hub_owner.
func TestObjectsAreOwnedByHubOwner(t *testing.T) {
	d := hubtest.New(t)
	conn := adminConn(t, d)
	if _, err := migrate.Apply(ctxT(t), conn, load(t, two), nil); err != nil {
		t.Fatalf("Apply as the administrator: %v", err)
	}
	check := d.OwnerConn(t)
	for _, tbl := range []string{"a", "b", "schema_migrations"} {
		var owner string
		if err := check.QueryRow(ctxT(t), "SELECT tableowner FROM pg_tables WHERE schemaname = 'hub' AND tablename = $1", tbl).Scan(&owner); err != nil {
			t.Fatalf("pg_tables hub.%s: %v", tbl, err)
		}
		if owner != "hub_owner" {
			t.Fatalf("hub.%s is owned by %q; want hub_owner", tbl, owner)
		}
	}
	var schemaOwner string
	if err := check.QueryRow(ctxT(t), "SELECT pg_get_userbyid(nspowner) FROM pg_namespace WHERE nspname = 'hub'").Scan(&schemaOwner); err != nil || schemaOwner != "hub_owner" {
		t.Fatalf("schema hub is owned by %q (%v); want hub_owner", schemaOwner, err)
	}
}
