//go:build integration

package hubtest_test

import (
	"context"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/jackc/pgx/v5"

	"github.com/kpmg-au-emu/panacea-app/control-plane/internal/hub/hubtest"
)

func ctxT(t *testing.T) context.Context {
	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	t.Cleanup(cancel)
	return ctx
}

// TestEachURLIsItsRole: every later RLS and grant test means something only if
// AppURL really is hub_app and OwnerURL really is hub_owner, neither a
// superuser, each on the scratch database; an AppPool that silently connected
// as the administrator would pass them all.
func TestEachURLIsItsRole(t *testing.T) {
	d := hubtest.New(t)
	if !strings.HasPrefix(d.Name, "hub_test_") || len(d.Name) != len("hub_test_")+8 {
		t.Fatalf("Name = %q; want hub_test_<8 hex>", d.Name)
	}
	var user, db, path string
	var super bool
	pool := d.AppPool(t)
	if err := pool.QueryRow(ctxT(t), "SELECT current_user, current_database(), current_setting('search_path'), (SELECT rolsuper FROM pg_roles WHERE rolname = current_user)").Scan(&user, &db, &path, &super); err != nil {
		t.Fatalf("as AppPool: %v", err)
	}
	if user != "hub_app" || db != d.Name || path != "hub" || super {
		t.Fatalf("AppPool is %s on %s, search_path %q, superuser %v; want hub_app on %s, hub, false", user, db, path, super, d.Name)
	}
	owner := d.OwnerConn(t)
	if err := owner.QueryRow(ctxT(t), "SELECT current_user, current_database(), (SELECT rolsuper FROM pg_roles WHERE rolname = current_user)").Scan(&user, &db, &super); err != nil {
		t.Fatalf("as OwnerConn: %v", err)
	}
	if user != "hub_owner" || db != d.Name || super {
		t.Fatalf("OwnerConn is %s on %s, superuser %v; want hub_owner on %s, false", user, db, super, d.Name)
	}
	// New granted CREATE on the scratch database to hub_owner, and only there.
	var can bool
	if err := owner.QueryRow(ctxT(t), "SELECT has_database_privilege('hub_owner', $1, 'CREATE')", d.Name).Scan(&can); err != nil || !can {
		t.Fatalf("hub_owner CREATE on %s = %v, %v; want true", d.Name, can, err)
	}
	if err := owner.QueryRow(ctxT(t), "SELECT has_database_privilege('hub_app', $1, 'CREATE')", d.Name).Scan(&can); err != nil || can {
		t.Fatalf("hub_app CREATE on %s = %v, %v; want false", d.Name, can, err)
	}
}

// TestTheScratchDatabaseIsDroppedAfterTheTest: even with a connection left open.
func TestTheScratchDatabaseIsDroppedAfterTheTest(t *testing.T) {
	var name string
	t.Run("uses one", func(t *testing.T) {
		d := hubtest.New(t)
		name = d.Name
		conn, err := pgx.Connect(ctxT(t), d.OwnerURL)
		if err != nil {
			t.Fatalf("connect: %v", err)
		}
		_ = conn // left open on purpose: WITH (FORCE) must end it
	})
	if name == "" {
		t.Skip("the subtest skipped")
	}
	admin, err := pgx.Connect(ctxT(t), os.Getenv(hubtest.EnvAdminURL))
	if err != nil {
		t.Fatalf("connect as the administrator: %v", err)
	}
	defer admin.Close(context.Background())
	var n int
	if err := admin.QueryRow(ctxT(t), "SELECT count(*) FROM pg_database WHERE datname = $1", name).Scan(&n); err != nil || n != 0 {
		t.Fatalf("%s still exists (%d, %v) after its test ended", name, n, err)
	}
}

// TestNewSkipsWithoutTheVariable: no database, no failure, a skip naming it.
func TestNewSkipsWithoutTheVariable(t *testing.T) {
	t.Setenv(hubtest.EnvAdminURL, "")
	var reached bool
	ok := t.Run("inner", func(t *testing.T) {
		hubtest.New(t)
		reached = true
	})
	if !ok || reached {
		t.Fatalf("inner ok=%v reached=%v; want a skip before New returns", ok, reached)
	}
}
