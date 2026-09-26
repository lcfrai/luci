// Package hubtest is the integration harness: a scratch database per test,
// migrated, and dropped when the test ends. Only _test.go files built with
// -tags integration import it.
package hubtest

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"net/url"
	"os"
	"testing"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/kpmg-au-emu/panacea-app/control-plane/internal/hub/migrate"
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
func New(t *testing.T) *DB {
	t.Helper()
	admin := os.Getenv(EnvAdminURL)
	if admin == "" {
		t.Skipf("hubtest: %s is unset; set it to an administrator's URL on a stack initialised with migrate/roles_local.sql", EnvAdminURL)
	}
	base, err := url.Parse(admin)
	if err != nil || (base.Scheme != "postgres" && base.Scheme != "postgresql") || base.User == nil {
		t.Fatalf("hubtest: %s must be a postgres:// URL naming its user (got a value that is not)", EnvAdminURL)
	}
	var b [4]byte
	if _, err := rand.Read(b[:]); err != nil {
		t.Fatalf("hubtest: random name: %v", err)
	}
	d := &DB{Name: "hub_test_" + hex.EncodeToString(b[:])}
	d.AdminURL = withDB(base, nil, d.Name)
	d.OwnerURL = withDB(base, url.User("hub_owner"), d.Name)
	d.AppURL = withDB(base, url.User("hub_app"), d.Name)

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	conn, err := pgx.Connect(ctx, admin)
	if err != nil {
		t.Fatalf("hubtest: connect as the administrator: %v", err)
	}
	defer conn.Close(context.Background())
	// The name is generated above, [a-z0-9_] only, so quoting it is enough.
	ident := pgx.Identifier{d.Name}.Sanitize()
	if _, err := conn.Exec(ctx, "CREATE DATABASE "+ident); err != nil {
		t.Fatalf("hubtest: create database %s: %v", d.Name, err)
	}
	t.Cleanup(func() { drop(t, admin, ident) })
	if _, err := conn.Exec(ctx, "GRANT CREATE ON DATABASE "+ident+" TO hub_owner"); err != nil {
		t.Fatalf("hubtest: grant CREATE on %s to hub_owner (was migrate/roles_local.sql run?): %v", d.Name, err)
	}
	return d
}

// Migrated is New followed by migrate.Apply of the embedded migrations
// (migrate.Embedded()), as hub_owner.
func Migrated(t *testing.T) *DB {
	t.Helper()
	d := New(t)
	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()
	conn, err := pgx.Connect(ctx, d.OwnerURL)
	if err != nil {
		t.Fatalf("hubtest: connect as hub_owner to migrate: %v", err)
	}
	defer conn.Close(context.Background())
	if _, err := migrate.Apply(ctx, conn, migrate.Embedded(), nil); err != nil {
		t.Fatalf("hubtest: migrate %s: %v", d.Name, err)
	}
	return d
}

// drop removes the scratch database, forcing out any connection a test left.
func drop(t *testing.T, admin, ident string) {
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	conn, err := pgx.Connect(ctx, admin)
	if err != nil {
		t.Errorf("hubtest: connect to drop %s: %v", ident, err)
		return
	}
	defer conn.Close(context.Background())
	if _, err := conn.Exec(ctx, "DROP DATABASE IF EXISTS "+ident+" WITH (FORCE)"); err != nil {
		t.Errorf("hubtest: drop %s: %v", ident, err)
	}
}

// withDB is base pointed at database name, as user (base's own user when nil).
// A role other than the administrator gets no password: the local stack
// trusts 127.0.0.1 (roles_local.sql).
func withDB(base *url.URL, user *url.Userinfo, name string) string {
	u := *base
	if user != nil {
		u.User = user
	}
	u.Path = "/" + name
	u.RawPath = ""
	return u.String()
}

// AppPool opens a pool as hub_app with search_path=hub, closed at cleanup.
func (d *DB) AppPool(t *testing.T) *pgxpool.Pool {
	t.Helper()
	cfg, err := pgxpool.ParseConfig(d.AppURL)
	if err != nil {
		t.Fatalf("hubtest: parse the hub_app URL: %v", err)
	}
	cfg.ConnConfig.RuntimeParams["search_path"] = "hub"
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	pool, err := pgxpool.NewWithConfig(ctx, cfg)
	if err != nil {
		t.Fatalf("hubtest: open a pool as hub_app: %v", err)
	}
	t.Cleanup(pool.Close)
	if err := pool.Ping(ctx); err != nil {
		t.Fatalf("hubtest: reach %s as hub_app: %v", d.Name, err)
	}
	return pool
}

// OwnerConn opens one connection as hub_owner, for tests that must tamper.
func (d *DB) OwnerConn(t *testing.T) *pgx.Conn {
	t.Helper()
	return connect(t, d.OwnerURL, "hub_owner")
}

// connect opens one connection, closed at cleanup.
func connect(t *testing.T, u, who string) *pgx.Conn {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	conn, err := pgx.Connect(ctx, u)
	if err != nil {
		t.Fatalf("hubtest: connect as %s: %v", who, err)
	}
	t.Cleanup(func() { conn.Close(context.Background()) })
	return conn
}
