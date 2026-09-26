//go:build integration

package migrate_test

// Task 07a's database tests: the tables, row-level security and the grants of
// sql/0001_init.sql, with no Go handler in the way. The seed helper here is
// task 07b's too (guards_integration_test.go).

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"sort"
	"strings"
	"testing"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"

	"github.com/kpmg-au-emu/panacea-app/control-plane/internal/hub/hubtest"
	"github.com/kpmg-au-emu/panacea-app/control-plane/internal/hub/migrate"
)

// hx is n random bytes as hex.
func hx(t *testing.T, n int) string {
	t.Helper()
	b := make([]byte, n)
	if _, err := rand.Read(b); err != nil {
		t.Fatalf("rand: %v", err)
	}
	return hex.EncodeToString(b)
}

// newID is prefix-<16 hex>, the shape every Hub id has.
func newID(t *testing.T, prefix string) string { return prefix + "-" + hx(t, 8) }

func sum(b []byte) []byte { s := sha256.Sum256(b); return s[:] }

// pgErr is err's *pgconn.PgError, or nil.
func pgErr(err error) *pgconn.PgError {
	var pe *pgconn.PgError
	if errors.As(err, &pe) {
		return pe
	}
	return nil
}

// wantCode fails the test unless err is a *pgconn.PgError with code.
func wantCode(t *testing.T, err error, code, what string) *pgconn.PgError {
	t.Helper()
	pe := pgErr(err)
	if pe == nil || pe.Code != code {
		t.Errorf("%s: want SQLSTATE %s, got %v", what, code, err)
		return nil
	}
	return pe
}

// appConn is one connection as hub_app, search_path hub, closed at cleanup.
func appConn(t *testing.T, d *hubtest.DB) *pgx.Conn {
	t.Helper()
	cfg, err := pgx.ParseConfig(d.AppURL)
	if err != nil {
		t.Fatalf("parse the hub_app URL: %v", err)
	}
	cfg.RuntimeParams["search_path"] = "hub"
	conn, err := pgx.ConnectConfig(ctxT(t), cfg)
	if err != nil {
		t.Fatalf("connect as hub_app: %v", err)
	}
	t.Cleanup(func() { conn.Close(context.Background()) })
	return conn
}

// inOrg runs fn in one transaction on conn with hub.org_id = org (transaction
// local, as the Hub sets it), and rolls it back unless commit.
func inOrg(t *testing.T, conn *pgx.Conn, org string, commit bool, fn func(tx pgx.Tx) error) error {
	t.Helper()
	ctx := ctxT(t)
	tx, err := conn.Begin(ctx)
	if err != nil {
		t.Fatalf("begin: %v", err)
	}
	defer tx.Rollback(context.Background())
	if org != "" {
		if _, err := tx.Exec(ctx, "SELECT set_config('hub.org_id', $1, true)", org); err != nil {
			t.Fatalf("set hub.org_id: %v", err)
		}
	}
	if err := fn(tx); err != nil {
		return err
	}
	if commit {
		return tx.Commit(ctx)
	}
	return nil
}

// db is what both a pgx.Tx and a *pgx.Conn offer.
type db interface {
	Exec(ctx context.Context, sql string, args ...any) (pgconn.CommandTag, error)
	QueryRow(ctx context.Context, sql string, args ...any) pgx.Row
}

// trow is one row to insert, column to value.
type trow map[string]any

// with is r with kv (column, value, column, value, …) set; r is not changed.
func (r trow) with(kv ...any) trow {
	out := trow{}
	for k, v := range r {
		out[k] = v
	}
	for i := 0; i+1 < len(kv); i += 2 {
		out[kv[i].(string)] = kv[i+1]
	}
	return out
}

// ins inserts r into hub.<table>, columns in name order, and returns r.
func ins(ctx context.Context, q db, table string, r trow) (trow, error) {
	cols := make([]string, 0, len(r))
	for c := range r {
		cols = append(cols, c)
	}
	sort.Strings(cols)
	ph := make([]string, len(cols))
	args := make([]any, len(cols))
	for i, c := range cols {
		ph[i] = fmt.Sprintf("$%d", i+1)
		args[i] = r[c]
	}
	_, err := q.Exec(ctx, fmt.Sprintf("INSERT INTO hub.%s (%s) VALUES (%s)",
		table, strings.Join(cols, ", "), strings.Join(ph, ", ")), args...)
	return r, err
}

// mustIns is ins that fails the test on an error.
func mustIns(t *testing.T, q db, table string, r trow) trow {
	t.Helper()
	if _, err := ins(ctxT(t), q, table, r); err != nil {
		t.Fatalf("insert into %s: %v", table, err)
	}
	return r
}

// orgSeed is one org's seeded parents: one of every kind a link can name.
type orgSeed struct {
	Label, Org, Tenant   string
	User, User2          string // User is the team's admin; User2 is in no team
	Team, Project, Brief string
	Digest               []byte // revision 1's digest
	Blob                 []byte // in project_blobs and the approval's manifest and blobs
	Blob2                []byte // in project_blobs only
	Blob3                []byte // in blobs only
	Approval, DesignKey  string
	Release              string // a confirmed run with a pin
	RepoKey              string
}

// uuidOf is 16 random bytes in uuid text form.
func uuidOf(t *testing.T) string {
	h := hx(t, 16)
	return h[0:8] + "-" + h[8:12] + "-" + h[12:16] + "-" + h[16:20] + "-" + h[20:32]
}

var (
	zeroHash = make([]byte, 32)
	tNow     = time.Date(2026, 9, 27, 0, 0, 0, 0, time.UTC)
	tLater   = time.Date(2099, 1, 1, 0, 0, 0, 0, time.UTC)
)

// auditTip is the org's last event (seq, hash), or (0, zeros): the head, since
// task 07b's audit_append keeps them equal.
func auditTip(ctx context.Context, q db, org string) (int64, []byte, error) {
	var seq int64
	var h []byte
	err := q.QueryRow(ctx, "SELECT seq, hash FROM hub.audit_events WHERE org_id = $1 ORDER BY seq DESC LIMIT 1", org).Scan(&seq, &h)
	if errors.Is(err, pgx.ErrNoRows) {
		return 0, zeroHash, nil
	}
	return seq, h, err
}

// base is a valid new row of table in s's org: every link names one of s's
// parents, and every unique column is fresh, so it can be inserted beside the
// seed. audit_events is chained on the org's tip as q sees it.
func base(t *testing.T, q db, table string, s *orgSeed) trow {
	t.Helper()
	fresh := []byte("fresh " + hx(t, 8))
	o, u := s.Org, s.User
	switch table {
	case "teams":
		return trow{"id": newID(t, "tm"), "org_id": o, "name": "team " + hx(t, 4), "created_by": u}
	case "team_members":
		return trow{"org_id": o, "team_id": s.Team, "user_id": s.User2, "role": "member", "added_by": u}
	case "team_invites":
		return trow{"id": newID(t, "iv"), "org_id": o, "team_id": s.Team, "email": hx(t, 4) + "@x.test",
			"role": "member", "state": "bound", "created_by": u, "bound_user_id": s.User2, "bound_at": tNow}
	case "projects":
		return trow{"id": newID(t, "pj"), "org_id": o, "team_id": s.Team, "name": "project " + hx(t, 4), "created_by": u}
	case "project_remotes":
		return trow{"org_id": o, "project_id": s.Project, "repo_key": "github.com/o/" + hx(t, 4), "host": "github", "added_by": u}
	case "briefs":
		return trow{"id": newID(t, "bf"), "org_id": o, "project_id": s.Project, "scope": "branch",
			"branch_ref": "refs/heads/b-" + hx(t, 4), "created_by": u}
	case "brief_revisions":
		return trow{"org_id": o, "brief_id": s.Brief, "revision": 2, "project_id": s.Project, "scope": "project",
			"branch_ref": "", "status": "open", "canonical": fresh, "digest": sum(fresh), "saved_by": u}
	case "blobs":
		return trow{"org_id": o, "digest": sum(fresh), "size": len(fresh), "bytes": fresh}
	case "project_blobs":
		return trow{"org_id": o, "project_id": s.Project, "digest": s.Blob3, "content_type": "text/html", "uploaded_by": u}
	case "design_approvals":
		cv := "cv-" + hx(t, 4)
		return trow{"id": newID(t, "ap"), "org_id": o, "project_id": s.Project, "design_key": "design:" + cv + "@1",
			"canvas_id": cv, "design_revision": 1, "review_digest": sum(fresh), "brief_id": s.Brief,
			"brief_revision": 1, "brief_digest": s.Digest, "branch_ref": "refs/heads/main",
			"manifest_digest": s.Blob, "approved_by": u}
	case "design_approval_blobs":
		return trow{"org_id": o, "approval_id": s.Approval, "project_id": s.Project, "digest": s.Blob2}
	case "session_links":
		return trow{"id": newID(t, "ln"), "org_id": o, "project_id": s.Project, "design_key": s.DesignKey,
			"brief_id": s.Brief, "brief_revision": 1, "session_id": "s-" + hx(t, 4),
			"branch_ref": "refs/heads/main", "created_by": u}
	case "releases":
		return trow{"id": newID(t, "rl"), "org_id": o, "project_id": s.Project, "kind": "run",
			"panacea_project": "pp-" + s.Label, "environment": "prod", "branch_ref": "refs/heads/main",
			"commit_sha": hx(t, 20), "source_release_id": s.Release, "pin_reason": "ship_head",
			"pin_brief_id": s.Brief, "pin_brief_revision": 1, "pin_brief_digest": s.Digest,
			"pin_head_revision_at_ship": 1, "design_key": s.DesignKey, "reserved_by": u,
			"expires_at": tLater}
	case "release_deployments":
		return trow{"org_id": o, "project_id": s.Project, "release_id": s.Release, "panacea_project": "pp-" + s.Label,
			"deployment_id": "dp-" + hx(t, 4), "joined_by": u}
	case "evidence":
		return trow{"id": newID(t, "ev"), "org_id": o, "project_id": s.Project, "repo_key": s.RepoKey,
			"brief_id": s.Brief, "brief_revision": 1, "criterion_id": "c1", "criterion_digest": sum(fresh),
			"commit_sha": hx(t, 20), "verdict": "settled", "because": "it holds", "recorded_by": u}
	case "audit_events":
		seq, prev, err := auditTip(ctxT(t), q, o)
		if err != nil {
			t.Fatalf("read %s's audit tip: %v", o, err)
		}
		return trow{"org_id": o, "seq": seq + 1, "at": tNow, "kind": "brief.saved", "feed": true,
			"team_id": s.Team, "project_id": s.Project, "subject": "brief:" + s.Brief, "actor_user_id": u,
			"canonical": fresh, "prev_hash": prev, "hash": sum(append(append([]byte{}, prev...), fresh...))}
	case "idempotency_keys":
		return trow{"org_id": o, "user_id": u, "key": hx(t, 8), "method": "POST", "path": "/v1/x",
			"request_sha256": sum(fresh), "status": 0}
	}
	t.Fatalf("base: no row for table %s", table)
	return nil
}

// seedOrg writes one org's rows. orgs, users and audit_heads go in as the owner
// (hub_app may not insert them); every other row goes in as hub_app with
// hub.org_id set, in the order the Hub writes: a brief's head moves only after
// its revision exists, and the audit event is chained on the org's head.
func seedOrg(t *testing.T, owner, app *pgx.Conn, label string) *orgSeed {
	t.Helper()
	ctx := ctxT(t)
	s := &orgSeed{Label: label, Org: newID(t, "org"), Tenant: uuidOf(t), User: newID(t, "u"), User2: newID(t, "u")}
	if _, err := owner.Exec(ctx, "INSERT INTO hub.orgs (id, entra_tenant_id, display_name) VALUES ($1, $2::text::uuid, $3)",
		s.Org, s.Tenant, "Org "+label); err != nil {
		t.Fatalf("seed org %s: %v", label, err)
	}
	for _, u := range []string{s.User, s.User2} {
		if _, err := owner.Exec(ctx, "INSERT INTO hub.users (id, org_id, entra_oid, display_name, email) VALUES ($1, $2, $3::text::uuid, $4, $5)",
			u, s.Org, uuidOf(t), "User "+u, u+"@x.test"); err != nil {
			t.Fatalf("seed user %s: %v", u, err)
		}
	}
	if _, err := owner.Exec(ctx, "INSERT INTO hub.audit_heads (org_id) VALUES ($1)", s.Org); err != nil {
		t.Fatalf("seed audit head %s: %v", label, err)
	}
	err := inOrg(t, app, s.Org, true, func(tx pgx.Tx) error {
		s.Team = mustIns(t, tx, "teams", base(t, tx, "teams", s))["id"].(string)
		mustIns(t, tx, "team_members", base(t, tx, "team_members", s).with("user_id", s.User, "role", "admin", "added_by", nil))
		mustIns(t, tx, "team_invites", base(t, tx, "team_invites", s))
		s.Project = mustIns(t, tx, "projects", base(t, tx, "projects", s))["id"].(string)
		s.RepoKey = mustIns(t, tx, "project_remotes", base(t, tx, "project_remotes", s))["repo_key"].(string)
		s.Brief = mustIns(t, tx, "briefs", base(t, tx, "briefs", s).with("scope", "project", "branch_ref", ""))["id"].(string)
		s.Digest = mustIns(t, tx, "brief_revisions", base(t, tx, "brief_revisions", s).with("revision", 1, "status", "complete"))["digest"].([]byte)
		if _, err := tx.Exec(ctx, "UPDATE hub.briefs SET head_revision = 1, latest_complete_revision = 1, updated_at = now() WHERE id = $1", s.Brief); err != nil {
			t.Fatalf("seed: move %s's head: %v", s.Brief, err)
		}
		s.Blob = mustIns(t, tx, "blobs", base(t, tx, "blobs", s))["digest"].([]byte)
		s.Blob2 = mustIns(t, tx, "blobs", base(t, tx, "blobs", s))["digest"].([]byte)
		s.Blob3 = mustIns(t, tx, "blobs", base(t, tx, "blobs", s))["digest"].([]byte)
		mustIns(t, tx, "project_blobs", base(t, tx, "project_blobs", s).with("digest", s.Blob))
		mustIns(t, tx, "project_blobs", base(t, tx, "project_blobs", s).with("digest", s.Blob2))
		ap := mustIns(t, tx, "design_approvals", base(t, tx, "design_approvals", s))
		s.Approval, s.DesignKey = ap["id"].(string), ap["design_key"].(string)
		mustIns(t, tx, "design_approval_blobs", base(t, tx, "design_approval_blobs", s).with("digest", s.Blob))
		mustIns(t, tx, "session_links", base(t, tx, "session_links", s))
		s.Release = mustIns(t, tx, "releases", base(t, tx, "releases", s).with("source_release_id", nil))["id"].(string)
		if _, err := tx.Exec(ctx, "UPDATE hub.releases SET state = 'confirmed', pipeline_run = $2, confirmed_at = now() WHERE id = $1",
			s.Release, "run-"+hx(t, 4)); err != nil {
			t.Fatalf("seed: confirm %s: %v", s.Release, err)
		}
		mustIns(t, tx, "release_deployments", base(t, tx, "release_deployments", s))
		mustIns(t, tx, "evidence", base(t, tx, "evidence", s))
		mustIns(t, tx, "audit_events", base(t, tx, "audit_events", s))
		mustIns(t, tx, "idempotency_keys", base(t, tx, "idempotency_keys", s))
		return nil
	})
	if err != nil {
		t.Fatalf("seed %s: %v", label, err)
	}
	return s
}

// seeded is a migrated database with orgs A and B seeded, and a connection as
// the owner and one as hub_app.
func seeded(t *testing.T) (d *hubtest.DB, owner, app *pgx.Conn, a, b *orgSeed) {
	t.Helper()
	d = hubtest.Migrated(t)
	owner, app = d.OwnerConn(t), appConn(t, d)
	return d, owner, app, seedOrg(t, owner, app, "a"), seedOrg(t, owner, app, "b")
}

// appTables is contract §10.3's loop: every table with an org_id column.
var appTables = []string{"users", "teams", "team_members", "team_invites", "projects",
	"project_remotes", "briefs", "brief_revisions", "blobs", "project_blobs", "design_approvals",
	"design_approval_blobs", "session_links", "releases", "release_deployments", "evidence",
	"audit_heads", "audit_events", "idempotency_keys"}

func TestMigrationAppliesAndReappliesClean(t *testing.T) {
	d := hubtest.New(t)
	ctx, conn := ctxT(t), d.OwnerConn(t)
	got, err := migrate.Apply(ctx, conn, migrate.Embedded(), nil)
	if err != nil || !sameInts(got, []int{1}) {
		t.Fatalf("first Apply = %v, %v; want [1], nil", got, err)
	}
	got, err = migrate.Apply(ctx, conn, migrate.Embedded(), nil)
	if err != nil || len(got) != 0 {
		t.Fatalf("second Apply = %v, %v; want [], nil", got, err)
	}
	cur, err := migrate.Current(ctx, conn)
	if err != nil || cur != 1 || migrate.Latest() != 1 {
		t.Fatalf("Current = %d, %v; Latest = %d; want 1, nil, 1", cur, err, migrate.Latest())
	}
}

func TestNamedConstraintsExist(t *testing.T) {
	d := hubtest.Migrated(t)
	conn := d.OwnerConn(t)
	for _, name := range []string{"users_org_id_entra_oid_key", "brief_revisions_canonical_hashes",
		"brief_revisions_erased_iff_no_content", "audit_events_hash_chains"} {
		var n int
		if err := conn.QueryRow(ctxT(t), `SELECT count(*) FROM pg_constraint
			WHERE connamespace = 'hub'::regnamespace AND conname = $1`, name).Scan(&n); err != nil || n != 1 {
			t.Errorf("constraint %s: %d found, %v; want 1", name, n, err)
		}
	}
}

func TestHubAppCannotUpdateOrDeleteImmutableTables(t *testing.T) {
	_, _, app, a, _ := seeded(t)
	for _, tb := range []string{"brief_revisions", "design_approvals", "session_links", "release_deployments",
		"evidence", "audit_events", "orgs", "users", "audit_heads"} {
		set := "org_id = org_id"
		if tb == "orgs" {
			set = "display_name = display_name"
		}
		for _, stmt := range []string{"UPDATE hub." + tb + " SET " + set, "DELETE FROM hub." + tb} {
			err := inOrg(t, app, a.Org, false, func(tx pgx.Tx) error {
				_, err := tx.Exec(ctxT(t), stmt)
				return err
			})
			wantCode(t, err, "42501", "hub_app: "+stmt)
		}
	}
}

func TestARevisionMustHashToItsDigest(t *testing.T) {
	_, _, app, a, _ := seeded(t)
	err := inOrg(t, app, a.Org, false, func(tx pgx.Tx) error {
		_, err := ins(ctxT(t), tx, "brief_revisions", base(t, tx, "brief_revisions", a).with("digest", sum([]byte("other"))))
		return err
	})
	if pe := wantCode(t, err, "23514", "a revision whose digest is not sha256(canonical)"); pe != nil &&
		pe.ConstraintName != "brief_revisions_canonical_hashes" {
		t.Errorf("refused by %s, want brief_revisions_canonical_hashes", pe.ConstraintName)
	}
	err = inOrg(t, app, a.Org, false, func(tx pgx.Tx) error {
		_, err := ins(ctxT(t), tx, "brief_revisions", base(t, tx, "brief_revisions", a))
		return err
	})
	if err != nil {
		t.Errorf("a revision that hashes to its digest: %v", err)
	}
}

func TestANullEnvironmentIsARunsOnly(t *testing.T) {
	_, _, app, a, _ := seeded(t)
	try := func(r trow) error {
		return inOrg(t, app, a.Org, false, func(tx pgx.Tx) error {
			_, err := ins(ctxT(t), tx, "releases", r)
			return err
		})
	}
	run := base(t, app, "releases", a).with("environment", nil, "source_release_id", nil)
	if err := try(run); err != nil {
		t.Errorf("a run with no environment and no source: %v", err)
	}
	for what, r := range map[string]trow{
		"a rollback with no environment":    run.with("id", newID(t, "rl"), "kind", "rollback", "source_deployment_id", "dp-1"),
		"a sourced run with no environment": run.with("id", newID(t, "rl"), "source_deployment_id", "dp-1"),
	} {
		wantCode(t, try(r), "23514", what)
	}
}

// insertable is every table hub_app may INSERT into (contract §10.5).
var insertable = []string{"teams", "team_members", "team_invites", "projects", "project_remotes", "briefs",
	"brief_revisions", "blobs", "project_blobs", "design_approvals", "design_approval_blobs",
	"session_links", "releases", "release_deployments", "evidence", "audit_events", "idempotency_keys"}

func TestRowLevelSecuritySeparatesOrgs(t *testing.T) {
	_, owner, app, a, b := seeded(t)
	ctx := ctxT(t)
	for _, tb := range appTables { // the seed is not vacuous: both orgs hold rows in every table
		var na, nb int
		if err := owner.QueryRow(ctx, fmt.Sprintf(`SELECT count(*) FILTER (WHERE org_id = $1),
			count(*) FILTER (WHERE org_id = $2) FROM hub.%s`, tb), a.Org, b.Org).Scan(&na, &nb); err != nil || na == 0 || nb == 0 {
			t.Fatalf("owner: %s holds %d of A's rows and %d of B's (%v); the seed must write both", tb, na, nb, err)
		}
	}
	err := inOrg(t, app, a.Org, false, func(tx pgx.Tx) error {
		for _, tb := range appTables {
			var all, other int
			if err := tx.QueryRow(ctx, fmt.Sprintf(`SELECT count(*), count(*) FILTER (WHERE org_id <> $1) FROM hub.%s`, tb),
				a.Org).Scan(&all, &other); err != nil {
				return fmt.Errorf("select %s: %w", tb, err)
			}
			if all == 0 || other != 0 {
				t.Errorf("hub_app in A: %s shows %d rows, %d not A's; want A's only", tb, all, other)
			}
		}
		rows, err := tx.Query(ctx, "SELECT id, display_name FROM hub.orgs")
		if err != nil {
			return err
		}
		type org struct{ id, name string }
		got, err := pgx.CollectRows(rows, func(r pgx.CollectableRow) (org, error) {
			var o org
			return o, r.Scan(&o.id, &o.name)
		})
		if err != nil {
			return err
		}
		if len(got) != 1 || got[0].id != a.Org || got[0].name != "Org a" {
			t.Errorf("hub_app in A: orgs shows %v; want only A (%s, Org a)", got, a.Org)
		}
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	for _, tb := range insertable { // WITH CHECK: an org-B row, written in org A's session
		err := inOrg(t, app, a.Org, false, func(tx pgx.Tx) error {
			_, err := ins(ctx, tx, tb, base(t, tx, tb, b))
			return err
		})
		wantCode(t, err, "42501", "hub_app in A inserts B's row into "+tb)
	}
}

func TestAnUnsetOrgSeesNothingEvenOnAReusedConnection(t *testing.T) {
	_, _, app, a, _ := seeded(t)
	ctx := ctxT(t)
	count := func(tx pgx.Tx, tb string) int {
		var n int
		if err := tx.QueryRow(ctx, "SELECT count(*) FROM hub."+tb).Scan(&n); err != nil {
			t.Fatalf("count %s: %v", tb, err)
		}
		return n
	}
	// One connection: a transaction sets org A and commits…
	if err := inOrg(t, app, a.Org, true, func(tx pgx.Tx) error {
		if n := count(tx, "users"); n == 0 {
			t.Errorf("in A: users shows 0 rows; the seed wrote 2")
		}
		return nil
	}); err != nil {
		t.Fatal(err)
	}
	// …and the next, which sets nothing, sees no row of it.
	if err := inOrg(t, app, "", false, func(tx pgx.Tx) error {
		var setting *string
		if err := tx.QueryRow(ctx, "SELECT current_setting('hub.org_id', true)").Scan(&setting); err != nil {
			return err
		}
		if setting == nil {
			t.Log("hub.org_id after a committed transaction-local set_config: NULL")
		} else {
			t.Logf("hub.org_id after a committed transaction-local set_config: %q", *setting)
		}
		for _, tb := range []string{"users", "orgs"} {
			if n := count(tx, tb); n != 0 {
				t.Errorf("unset org on a reused connection: %s shows %d rows; want 0", tb, n)
			}
		}
		return nil
	}); err != nil {
		t.Fatal(err)
	}
}

// fkey is one foreign key of schema hub, from the catalog.
type fkey struct {
	name, table, ref string
	cols, refCols    []string // in key order
	targetCols       []string // the unique index the key points at
	targetUnique     bool
}

// fkeys is every foreign key in schema hub, by name.
func fkeys(t *testing.T, q *pgx.Conn) []fkey {
	t.Helper()
	rows, err := q.Query(ctxT(t), `
SELECT c.conname, cl.relname, rf.relname,
  ARRAY(SELECT a.attname::text FROM unnest(c.conkey) WITH ORDINALITY k(n, i)
        JOIN pg_attribute a ON a.attrelid = c.conrelid AND a.attnum = k.n ORDER BY k.i),
  ARRAY(SELECT a.attname::text FROM unnest(c.confkey) WITH ORDINALITY k(n, i)
        JOIN pg_attribute a ON a.attrelid = c.confrelid AND a.attnum = k.n ORDER BY k.i),
  ARRAY(SELECT a.attname::text FROM pg_index ix CROSS JOIN LATERAL unnest(ix.indkey::int2[]) k(n)
        JOIN pg_attribute a ON a.attrelid = ix.indrelid AND a.attnum = k.n WHERE ix.indexrelid = c.conindid),
  coalesce((SELECT ix.indisunique FROM pg_index ix WHERE ix.indexrelid = c.conindid), false)
FROM pg_constraint c
JOIN pg_class cl ON cl.oid = c.conrelid
JOIN pg_class rf ON rf.oid = c.confrelid
WHERE c.contype = 'f' AND c.connamespace = 'hub'::regnamespace
ORDER BY c.conname`)
	if err != nil {
		t.Fatalf("read the foreign keys: %v", err)
	}
	out, err := pgx.CollectRows(rows, func(r pgx.CollectableRow) (fkey, error) {
		var k fkey
		return k, r.Scan(&k.name, &k.table, &k.ref, &k.cols, &k.refCols, &k.targetCols, &k.targetUnique)
	})
	if err != nil {
		t.Fatalf("read the foreign keys: %v", err)
	}
	return out
}

func indexOf(xs []string, x string) int {
	for i, v := range xs {
		if v == x {
			return i
		}
	}
	return -1
}

// linkCols is k's columns other than org_id.
func (k fkey) linkCols() []string {
	var out []string
	for _, c := range k.cols {
		if c != "org_id" {
			out = append(out, c)
		}
	}
	return out
}

// wantForeignKeys is how many foreign keys, not to orgs, 0001_init.sql holds: a
// key dropped by a later edit is red here, as a key added without org_id is red
// in the checks below.
const wantForeignKeys = 41

func TestEveryForeignKeyCarriesTheOrg(t *testing.T) {
	d := hubtest.Migrated(t)
	n := 0
	for _, k := range fkeys(t, d.OwnerConn(t)) {
		if k.ref == "orgs" {
			continue
		}
		n++
		i, j := indexOf(k.cols, "org_id"), indexOf(k.refCols, "org_id")
		if i < 0 || j < 0 || i != j {
			t.Errorf("%s: %s(%v) -> %s(%v) does not pair org_id with org_id at one position (I5)",
				k.name, k.table, k.cols, k.ref, k.refCols)
		}
		if !k.targetUnique || indexOf(k.targetCols, "org_id") < 0 {
			t.Errorf("%s: its target index on %s (%v, unique %v) is not a unique key holding org_id",
				k.name, k.ref, k.targetCols, k.targetUnique)
		}
	}
	if n != wantForeignKeys {
		t.Errorf("%d foreign keys not to orgs; want %d", n, wantForeignKeys)
	}
}

// parentIn is the value of link column col that names s's parent. A column no
// case knows fails the test: a new foreign key gets a case, or goes red.
func parentIn(t *testing.T, s *orgSeed, table, col string) any {
	t.Helper()
	switch col {
	case "created_by", "added_by", "uploaded_by", "approved_by", "saved_by", "reserved_by",
		"joined_by", "recorded_by", "user_id", "bound_user_id", "actor_user_id":
		return s.User
	case "team_id":
		return s.Team
	case "project_id":
		return s.Project
	case "brief_id", "pin_brief_id":
		return s.Brief
	case "brief_revision", "pin_brief_revision":
		return 1
	case "brief_digest", "pin_brief_digest":
		return s.Digest
	case "digest", "manifest_digest":
		return s.Blob
	case "approval_id":
		return s.Approval
	case "design_key":
		return s.DesignKey
	case "release_id", "source_release_id":
		return s.Release
	}
	t.Fatalf("%s.%s: no case names another org's parent for this link column", table, col)
	return nil
}

func TestACrossOrgReferenceIsRefused(t *testing.T) {
	_, owner, app, a, b := seeded(t)
	ctx := ctxT(t)
	keys := fkeys(t, owner)
	exercised := 0
	for _, k := range keys {
		if k.ref == "orgs" {
			continue
		}
		if indexOf(insertable, k.table) < 0 {
			t.Errorf("%s: %s is not insertable by hub_app, so no case can exercise it", k.name, k.table)
			continue
		}
		// A key that shares a column with this one may refuse the row first
		// (projects, for a design key that carries project_id): allowed, and
		// TestEveryForeignKeyCarriesTheOrg holds each key's shape on its own.
		allowed := map[string]bool{k.name: true}
		for _, o := range keys {
			if o.table != k.table || o.ref == "orgs" {
				continue
			}
			for _, c := range o.linkCols() {
				if indexOf(k.linkCols(), c) >= 0 {
					allowed[o.name] = true
				}
			}
		}
		var over []any
		for _, c := range k.linkCols() {
			over = append(over, c, parentIn(t, b, k.table, c))
		}
		err := inOrg(t, app, a.Org, false, func(tx pgx.Tx) error {
			_, err := ins(ctx, tx, k.table, base(t, tx, k.table, a))
			return err
		})
		if err != nil {
			t.Errorf("%s: the same row naming A's parent is refused, so the case proves nothing: %v", k.name, err)
			continue
		}
		err = inOrg(t, app, a.Org, false, func(tx pgx.Tx) error {
			_, err := ins(ctx, tx, k.table, base(t, tx, k.table, a).with(over...))
			return err
		})
		if pe := wantCode(t, err, "23503", k.name+": an org-A row naming B's parent"); pe != nil && !allowed[pe.ConstraintName] {
			t.Errorf("%s: refused by %s, not by this key or one sharing its columns", k.name, pe.ConstraintName)
		}
		exercised++
	}
	if exercised != wantForeignKeys {
		t.Errorf("exercised %d foreign keys; the catalog holds %d not to orgs", exercised, wantForeignKeys)
	}
}

// grants is hub_app's privileges in schema hub, from the ACLs themselves:
// "table:PRIV" for a table's, "table.column:PRIV" for a column's.
func grants(t *testing.T, q *pgx.Conn, grantee string) []string {
	t.Helper()
	rows, err := q.Query(ctxT(t), `
SELECT c.relname || ':' || a.privilege_type
FROM pg_class c CROSS JOIN LATERAL aclexplode(c.relacl) a
WHERE c.relnamespace = 'hub'::regnamespace AND a.grantee = $1::regrole::oid
UNION ALL
SELECT c.relname || '.' || att.attname || ':' || a.privilege_type
FROM pg_attribute att JOIN pg_class c ON c.oid = att.attrelid CROSS JOIN LATERAL aclexplode(att.attacl) a
WHERE c.relnamespace = 'hub'::regnamespace AND NOT att.attisdropped AND a.grantee = $1::regrole::oid
ORDER BY 1`, grantee)
	if err != nil {
		t.Fatalf("read %s's grants: %v", grantee, err)
	}
	out, err := pgx.CollectRows(rows, pgx.RowTo[string])
	if err != nil {
		t.Fatalf("read %s's grants: %v", grantee, err)
	}
	return out
}

// diff is what want has that got lacks, and what got has that want lacks.
func diff(want, got []string) (missing, extra []string) {
	w, g := map[string]bool{}, map[string]bool{}
	for _, x := range want {
		w[x] = true
	}
	for _, x := range got {
		g[x] = true
		if !w[x] {
			extra = append(extra, x)
		}
	}
	for _, x := range want {
		if !g[x] {
			missing = append(missing, x)
		}
	}
	return missing, extra
}

// hubAppGrants is contract §10.5's list, and nothing else. SELECT is on every
// table, schema_migrations included (it exists before 0001 runs, and /readyz
// reads it as hub_app). audit_heads and orgs are SELECT only (A18, A19).
func hubAppGrants() []string {
	var out []string
	for _, tb := range append([]string{"orgs", "schema_migrations"}, appTables...) {
		out = append(out, tb+":SELECT")
	}
	for _, tb := range insertable {
		out = append(out, tb+":INSERT")
	}
	for _, tb := range []string{"team_members", "project_remotes", "blobs", "project_blobs", "idempotency_keys"} {
		out = append(out, tb+":DELETE")
	}
	for tb, cols := range map[string][]string{
		"teams":            {"name"},
		"team_members":     {"role"},
		"team_invites":     {"state", "bound_user_id", "bound_at", "revoked_at"},
		"projects":         {"name"},
		"briefs":           {"branch_ref", "head_revision", "latest_complete_revision", "archived_at", "updated_at"},
		"releases":         {"state", "pipeline_run", "confirmed_at", "abandoned_at", "abandon_reason", "abandon_message", "expired_at", "environment"},
		"idempotency_keys": {"status", "response"},
	} {
		for _, c := range cols {
			out = append(out, tb+"."+c+":UPDATE")
		}
	}
	sort.Strings(out)
	return out
}

func TestHubAppGrantsAreExactlyTheList(t *testing.T) {
	d := hubtest.Migrated(t)
	conn := d.OwnerConn(t)
	missing, extra := diff(hubAppGrants(), grants(t, conn, "hub_app"))
	if len(missing) > 0 || len(extra) > 0 {
		t.Errorf("hub_app's grants differ from §10.5's list:\n missing %v\n extra   %v", missing, extra)
	}
	var usage, create bool
	if err := conn.QueryRow(ctxT(t), "SELECT has_schema_privilege('hub_app', 'hub', 'USAGE'), has_schema_privilege('hub_app', 'hub', 'CREATE')").
		Scan(&usage, &create); err != nil || !usage || create {
		t.Errorf("hub_app on schema hub: USAGE %v, CREATE %v (%v); want USAGE only", usage, create, err)
	}
	var public int
	if err := conn.QueryRow(ctxT(t), `SELECT count(*) FROM pg_class c CROSS JOIN LATERAL aclexplode(c.relacl) a
		WHERE c.relnamespace = 'hub'::regnamespace AND a.grantee = 0`).Scan(&public); err != nil || public != 0 {
		t.Errorf("PUBLIC holds %d table privileges in hub (%v); want 0", public, err)
	}
}
