//go:build integration

package migrate_test

// Task 07b's database tests: the guards and the owner functions appended to
// sql/0001_init.sql, each on a database seeded by seedOrg (schema_integration_test.go).

import (
	"context"
	"fmt"
	"regexp"
	"sort"
	"strings"
	"testing"
	"time"

	"github.com/jackc/pgx/v5"

	"github.com/kpmg-au-emu/panacea-app/control-plane/internal/hub/hubtest"
)

// exec runs sql on q in its own transaction, rolled back, and returns its error.
func exec(t *testing.T, conn *pgx.Conn, org, sql string, args ...any) error {
	t.Helper()
	return inOrg(t, conn, org, false, func(tx pgx.Tx) error {
		_, err := tx.Exec(ctxT(t), sql, args...)
		return err
	})
}

// wantRefused is wantCode with the message holding msg too: several layers
// answer 42501, and the message says which one did.
func wantRefused(t *testing.T, err error, code, msg, what string) {
	t.Helper()
	if pe := wantCode(t, err, code, what); pe != nil && !strings.Contains(pe.Message, msg) {
		t.Errorf("%s: refused with %q, want a message holding %q", what, pe.Message, msg)
	}
}

// appendOnly is the six tables forbid_change holds.
var appendOnly = []string{"design_approvals", "design_approval_blobs", "session_links",
	"release_deployments", "evidence", "audit_events"}

func TestAppendOnlyTriggersHoldEvenForTheOwner(t *testing.T) {
	_, owner, _, _, _ := seeded(t)
	for _, tb := range appendOnly {
		msg := "hub: " + tb + " is append-only"
		wantRefused(t, exec(t, owner, "", "UPDATE hub."+tb+" SET org_id = org_id"), "42501", msg, "owner UPDATE "+tb)
		wantRefused(t, exec(t, owner, "", "DELETE FROM hub."+tb), "42501", msg, "owner DELETE "+tb)
		// CASCADE, so a table another references reaches its own trigger rather
		// than the foreign-key refusal (0A000); the named table's trigger fires first.
		wantRefused(t, exec(t, owner, "", "TRUNCATE hub."+tb+" CASCADE"), "42501", msg, "owner TRUNCATE "+tb)
	}
}

func TestOnlyEraseBriefCanEraseARevision(t *testing.T) {
	d, owner, app, a, _ := seeded(t)
	admin := adminConn(t, d)
	erase := "UPDATE hub.brief_revisions SET canonical = NULL, erased_at = now() WHERE brief_id = $1"
	if err := exec(t, owner, "", erase, a.Brief); err != nil {
		t.Errorf("the owner's erasure-shaped UPDATE: %v; guard_revision lets hub_owner erase", err)
	}
	wantRefused(t, exec(t, admin, "", erase, a.Brief), "42501", "immutable", "the administrator's erasure (not hub_owner)")
	for what, set := range map[string]string{
		"status":           "status = 'open'",
		"digest":           "digest = sha256('x')",
		"new content":      "canonical = 'x', digest = sha256('x')",
		"erasure+saved_at": "canonical = NULL, erased_at = now(), saved_at = saved_at + interval '1 second'",
		"erasure+status":   "canonical = NULL, erased_at = now(), status = 'open'",
	} {
		wantRefused(t, exec(t, owner, "", "UPDATE hub.brief_revisions SET "+set+" WHERE brief_id = $1", a.Brief),
			"42501", "immutable", "owner changes a revision's "+what)
	}
	wantRefused(t, exec(t, owner, "", "DELETE FROM hub.brief_revisions WHERE brief_id = $1", a.Brief),
		"42501", "never deleted", "owner DELETE of a revision")
	wantRefused(t, exec(t, owner, "", "TRUNCATE hub.brief_revisions CASCADE"), "42501", "append-only", "owner TRUNCATE brief_revisions")
	err := inOrg(t, app, a.Org, true, func(tx pgx.Tx) error {
		var n int
		if err := tx.QueryRow(ctxT(t), "SELECT hub.erase_brief($1)", a.Brief).Scan(&n); err != nil {
			return err
		}
		if n != 1 {
			t.Errorf("erase_brief = %d, want 1 (one revision held content)", n)
		}
		return nil
	})
	if err != nil {
		t.Fatalf("erase_brief as hub_app: %v", err)
	}
	var withContent int
	var archived, erased bool
	if err := owner.QueryRow(ctxT(t), `SELECT (SELECT count(*) FROM hub.brief_revisions WHERE brief_id = $1 AND canonical IS NOT NULL),
		archived_at IS NOT NULL, erased_at IS NOT NULL FROM hub.briefs WHERE id = $1`, a.Brief).Scan(&withContent, &archived, &erased); err != nil ||
		withContent != 0 || !archived || !erased {
		t.Errorf("after erase_brief: %d revisions keep content, archived %v, erased %v (%v)", withContent, archived, erased, err)
	}
	// An erasure is never undone or re-dated, even by the owner; the brief is
	// archived too, but only guard_brief's erased_at clause refuses these (PS-2).
	for what, set := range map[string]string{
		"un-erase": "erased_at = NULL",
		"re-date":  "erased_at = erased_at + interval '1 second'",
	} {
		wantRefused(t, exec(t, owner, "", "UPDATE hub.briefs SET "+set+" WHERE id = $1", a.Brief),
			"42501", "illegal change", "the owner's "+what+" of an erased brief")
	}
}

// revision is a new revision n of brief (scope, ref) in s's org, hashing right.
func revision(t *testing.T, s *orgSeed, brief string, n int, scope, ref string) trow {
	c := []byte(fmt.Sprintf("revision %d of %s %s", n, brief, hx(t, 4)))
	return trow{"org_id": s.Org, "brief_id": brief, "revision": n, "project_id": s.Project, "scope": scope,
		"branch_ref": ref, "status": "open", "canonical": c, "digest": sum(c), "saved_by": s.User}
}

func TestAHeadMovesOneStepOntoARealRevision(t *testing.T) {
	_, owner, app, a, _ := seeded(t)
	head := "UPDATE hub.briefs SET head_revision = $2 WHERE id = $1"
	wantRefused(t, exec(t, app, a.Org, head, a.Brief, 3), "42501", "illegal change", "head 1 -> 3")
	wantRefused(t, exec(t, app, a.Org, head, a.Brief, 2), "42501", "illegal change", "head 1 -> 2 with no revision 2")
	err := inOrg(t, app, a.Org, false, func(tx pgx.Tx) error {
		if _, err := ins(ctxT(t), tx, "brief_revisions", revision(t, a, a.Brief, 2, "project", "")); err != nil {
			return err
		}
		_, err := tx.Exec(ctxT(t), head, a.Brief, 2)
		return err
	})
	if err != nil {
		t.Errorf("head 1 -> 2 after revision 2 exists: %v", err)
	}
	wantRefused(t, exec(t, app, a.Org, "UPDATE hub.briefs SET latest_complete_revision = 0 WHERE id = $1", a.Brief),
		"42501", "illegal change", "latest_complete_revision 1 -> 0")
	// Revisions 2 and 3 exist, so the real-revision clause passes each move
	// below and the one-step rule alone refuses it (mutant PS-1).
	for what, steps := range map[string][]int{"head 1 -> 3, onto a real revision 3": {3}, "head 1 -> 2 -> 1": {2, 1}} {
		err := inOrg(t, app, a.Org, false, func(tx pgx.Tx) error {
			for _, n := range []int{2, 3} {
				if _, err := ins(ctxT(t), tx, "brief_revisions", revision(t, a, a.Brief, n, "project", "")); err != nil {
					return fmt.Errorf("insert revision %d: %w", n, err)
				}
			}
			for _, h := range steps {
				if _, err := tx.Exec(ctxT(t), head, a.Brief, h); err != nil {
					return err
				}
			}
			return nil
		})
		wantRefused(t, err, "42501", "illegal change", what)
	}
	// A brief's identity never changes, even for the owner, who holds every
	// column (mutant M19: guard_brief's identity tuple dropped).
	for what, set := range map[string]string{
		"project":    "project_id = project_id || 'x'",
		"scope":      "scope = 'branch', branch_ref = 'refs/heads/x'",
		"created_by": "created_by = NULL",
		"created_at": "created_at = created_at - interval '1 day'",
	} {
		wantRefused(t, exec(t, owner, "", "UPDATE hub.briefs SET "+set+" WHERE id = $1", a.Brief),
			"42501", "illegal change", "the owner changes a brief's "+what)
	}
}

func TestAnArchivedBriefIsFrozen(t *testing.T) {
	_, _, app, a, _ := seeded(t)
	ref := "refs/heads/feat-" + hx(t, 4)
	br := base(t, app, "briefs", a).with("branch_ref", ref)
	err := inOrg(t, app, a.Org, true, func(tx pgx.Tx) error {
		ctx := ctxT(t)
		if _, err := ins(ctx, tx, "briefs", br); err != nil {
			return err
		}
		if _, err := ins(ctx, tx, "brief_revisions", revision(t, a, br["id"].(string), 1, "branch", ref)); err != nil {
			return err
		}
		if _, err := ins(ctx, tx, "brief_revisions", revision(t, a, br["id"].(string), 2, "branch", ref)); err != nil {
			return err
		}
		_, err := tx.Exec(ctx, "UPDATE hub.briefs SET head_revision = 1 WHERE id = $1", br["id"])
		if err == nil {
			_, err = tx.Exec(ctx, "UPDATE hub.briefs SET archived_at = now() WHERE id = $1", br["id"])
		}
		return err
	})
	if err != nil {
		t.Fatalf("seed an archived branch brief: %v", err)
	}
	for what, set := range map[string]string{
		"head":       "head_revision = 2",
		"ref":        "branch_ref = 'refs/heads/other'",
		"unarchive":  "archived_at = NULL",
		"re-archive": "archived_at = archived_at + interval '1 second'",
	} {
		wantRefused(t, exec(t, app, a.Org, "UPDATE hub.briefs SET "+set+" WHERE id = $1", br["id"]),
			"42501", "illegal change", "an archived brief's "+what)
	}
	if err := exec(t, app, a.Org, "DELETE FROM hub.briefs WHERE id = $1", br["id"]); pgErr(err) == nil || pgErr(err).Code != "42501" {
		t.Errorf("DELETE of a brief: %v, want 42501", err)
	}
	live := base(t, app, "briefs", a).with("branch_ref", ref)
	if err := inOrg(t, app, a.Org, false, func(tx pgx.Tx) error { _, err := ins(ctxT(t), tx, "briefs", live); return err }); err != nil {
		t.Errorf("a new live brief on the archived one's ref: %v", err)
	}
}

func TestAPinNeverMoves(t *testing.T) {
	_, owner, app, a, _ := seeded(t)
	pin := "UPDATE hub.releases SET pin_brief_revision = 2 WHERE id = $1"
	wantRefused(t, exec(t, app, a.Org, pin, a.Release), "42501", "permission denied", "hub_app moves a pin (the grant)")
	wantRefused(t, exec(t, owner, "", pin, a.Release), "42501", "pin never changes", "the owner moves a pin (the trigger)")
	wantRefused(t, exec(t, owner, "", "DELETE FROM hub.releases WHERE id = $1", a.Release), "42501", "never deleted", "owner DELETE of a release")
}

// release inserts and returns a new run release in s's org with extra set, as hub_app.
func release(t *testing.T, app *pgx.Conn, s *orgSeed, kv ...any) trow {
	t.Helper()
	r := base(t, app, "releases", s).with("source_release_id", nil).with(kv...)
	if err := inOrg(t, app, s.Org, true, func(tx pgx.Tx) error { _, err := ins(ctxT(t), tx, "releases", r); return err }); err != nil {
		t.Fatalf("insert a release: %v", err)
	}
	return r
}

func TestOnlyAReservedReleaseMoves(t *testing.T) {
	_, _, app, a, _ := seeded(t)
	r := release(t, app, a)
	confirm := "UPDATE hub.releases SET state = 'confirmed', pipeline_run = $2, confirmed_at = now() WHERE id = $1"
	if err := inOrg(t, app, a.Org, true, func(tx pgx.Tx) error { _, err := tx.Exec(ctxT(t), confirm, r["id"], "run-1"); return err }); err != nil {
		t.Fatalf("confirm a reserved release: %v", err)
	}
	for what, set := range map[string]string{
		"re-confirm with another run": "pipeline_run = 'run-2', confirmed_at = now()",
		"abandon":                     "state = 'abandoned', confirmed_at = NULL, abandoned_at = now(), abandon_reason = 'cancelled'",
		"expire":                      "state = 'expired', confirmed_at = NULL, expired_at = now()",
		"back to reserved":            "state = 'reserved', confirmed_at = NULL",
	} {
		wantRefused(t, exec(t, app, a.Org, "UPDATE hub.releases SET "+set+" WHERE id = $1", r["id"]),
			"42501", "not reserved", "a confirmed release: "+what)
	}
}

// move runs one UPDATE hub.releases SET <set> WHERE id = $1 as hub_app, committed.
func move(t *testing.T, app *pgx.Conn, s *orgSeed, id any, set string) error {
	return inOrg(t, app, s.Org, true, func(tx pgx.Tx) error {
		_, err := tx.Exec(ctxT(t), "UPDATE hub.releases SET "+set+" WHERE id = $1", id)
		return err
	})
}

func TestANullEnvironmentReleaseConfirmsAbandonsAndExpires(t *testing.T) {
	_, owner, app, a, _ := seeded(t)
	for what, set := range map[string]string{
		"confirmed": "state = 'confirmed', pipeline_run = 'run-1', confirmed_at = now()",
		"abandoned": "state = 'abandoned', abandoned_at = now(), abandon_reason = 'deploy_failed'",
		"expired":   "state = 'expired', expired_at = now()",
	} {
		r := release(t, app, a, "environment", nil)
		if err := move(t, app, a, r["id"], set); err != nil {
			t.Errorf("a NULL-environment run -> %s: %v", what, err)
			continue
		}
		var env *string
		if err := owner.QueryRow(ctxT(t), "SELECT environment FROM hub.releases WHERE id = $1", r["id"]).Scan(&env); err != nil || env != nil {
			t.Errorf("after -> %s: environment %v (%v), want NULL", what, env, err)
		}
	}
}

func TestTheEnvironmentIsSetOnce(t *testing.T) {
	_, _, app, a, _ := seeded(t)
	r := release(t, app, a, "environment", nil)
	if err := move(t, app, a, r["id"], "state = 'confirmed', pipeline_run = 'run-1', confirmed_at = now()"); err != nil {
		t.Fatalf("confirm: %v", err)
	}
	if err := move(t, app, a, r["id"], "environment = 'feat-x'"); err != nil {
		t.Errorf("a confirmed NULL-environment run learns feat-x at the join: %v", err)
	}
	wantRefused(t, move(t, app, a, r["id"], "environment = 'feat-y'"), "42501", "set once", "feat-x -> feat-y")
	wantRefused(t, move(t, app, a, r["id"], "environment = NULL"), "42501", "set once", "feat-x -> NULL")
	ab := release(t, app, a, "environment", nil)
	if err := move(t, app, a, ab["id"], "state = 'abandoned', abandoned_at = now(), abandon_reason = 'cancelled'"); err != nil {
		t.Fatalf("abandon: %v", err)
	}
	wantRefused(t, move(t, app, a, ab["id"], "environment = 'feat-x'"), "42501", "set once", "an abandoned run learns an environment")
	rs := release(t, app, a) // reserved, environment 'prod'
	wantRefused(t, move(t, app, a, rs["id"], "environment = 'feat-x'"), "42501", "set once", "a reserved run's prod -> feat-x")
}

var (
	distinctPair = regexp.MustCompile(`\(\s*(NEW\.\w+(?:\s*,\s*NEW\.\w+)*)\s*\)\s*IS\s+DISTINCT\s+FROM\s*\(\s*(OLD\.\w+(?:\s*,\s*OLD\.\w+)*)\s*\)`)
	envPair      = regexp.MustCompile(`NEW\.environment\s+IS\s+DISTINCT\s+FROM\s+OLD\.environment`)
	fieldName    = regexp.MustCompile(`(?:NEW|OLD)\.(\w+)`)
)

func names(list string) []string {
	var out []string
	for _, m := range fieldName.FindAllStringSubmatch(list, -1) {
		out = append(out, m[1])
	}
	return out
}

func TestReleaseGuardCoversEveryColumn(t *testing.T) {
	d := hubtest.Migrated(t)
	conn := d.OwnerConn(t)
	var def string
	if err := conn.QueryRow(ctxT(t), "SELECT pg_get_functiondef('hub.guard_release'::regproc)").Scan(&def); err != nil {
		t.Fatalf("read the installed guard_release: %v", err)
	}
	pairs := distinctPair.FindAllStringSubmatch(def, -1)
	if len(pairs) != 2 {
		t.Fatalf("guard_release holds %d (NEW…) IS DISTINCT FROM (OLD…) pairs, want 2 (immutable, mutable)", len(pairs))
	}
	var lists [2][]string
	for i, p := range pairs {
		nw, old := names(p[1]), names(p[2])
		if strings.Join(nw, ",") != strings.Join(old, ",") {
			t.Errorf("pair %d: NEW (%v) and OLD (%v) differ, name for name", i, nw, old)
		}
		lists[i] = nw
	}
	if n := len(envPair.FindAllString(def, -1)); n != 1 {
		t.Errorf("NEW.environment IS DISTINCT FROM OLD.environment appears %d times, want 1", n)
	}
	seen := map[string]int{"environment": 1}
	for _, l := range lists {
		for _, c := range l {
			seen[c]++
		}
	}
	rows, err := conn.Query(ctxT(t), "SELECT column_name::text FROM information_schema.columns WHERE table_schema = 'hub' AND table_name = 'releases'")
	if err != nil {
		t.Fatal(err)
	}
	cols, err := pgx.CollectRows(rows, pgx.RowTo[string])
	if err != nil {
		t.Fatal(err)
	}
	isCol := map[string]bool{}
	for _, c := range cols {
		isCol[c] = true
		if seen[c] != 1 {
			t.Errorf("column releases.%s appears %d times across the immutable, mutable and environment lists, want 1", c, seen[c])
		}
	}
	for c := range seen {
		if !isCol[c] {
			t.Errorf("the guard names %s, which is not a column of releases", c)
		}
	}
	var updatable []string
	for _, g := range grants(t, conn, "hub_app") {
		if strings.HasPrefix(g, "releases.") && strings.HasSuffix(g, ":UPDATE") {
			updatable = append(updatable, strings.TrimSuffix(strings.TrimPrefix(g, "releases."), ":UPDATE"))
		}
	}
	mutable := append(append([]string{}, lists[1]...), "environment")
	sort.Strings(mutable)
	sort.Strings(updatable)
	if strings.Join(mutable, ",") != strings.Join(updatable, ",") {
		t.Errorf("mutable + environment = %v; hub_app's UPDATE columns on releases = %v", mutable, updatable)
	}
}

// head is s's audit head as the owner reads it.
func head(t *testing.T, owner *pgx.Conn, s *orgSeed) (int64, []byte) {
	t.Helper()
	var seq int64
	var h []byte
	if err := owner.QueryRow(ctxT(t), "SELECT seq, hash FROM hub.audit_heads WHERE org_id = $1", s.Org).Scan(&seq, &h); err != nil {
		t.Fatalf("read %s's audit head: %v", s.Label, err)
	}
	return seq, h
}

// appendEvent inserts audit event r as hub_app in s's org, committed.
func appendEvent(t *testing.T, app *pgx.Conn, s *orgSeed, r trow) error {
	return inOrg(t, app, s.Org, true, func(tx pgx.Tx) error { _, err := ins(ctxT(t), tx, "audit_events", r); return err })
}

func TestTheAuditChainIsGaplessAndHashed(t *testing.T) {
	_, owner, app, a, _ := seeded(t)
	if seq, _ := head(t, owner, a); seq != 1 {
		t.Fatalf("after the seed's one event, A's head is at %d, want 1: audit_append does not advance it", seq)
	}
	// base chains on the tip as its connection sees it: the owner's, since
	// hub_app outside a transaction has no hub.org_id and sees no event.
	for want := int64(2); want <= 3; want++ {
		if err := appendEvent(t, app, a, base(t, owner, "audit_events", a)); err != nil {
			t.Fatalf("event %d, chained right: %v", want, err)
		}
		if seq, h := head(t, owner, a); seq != want {
			t.Errorf("after event %d the head is at %d (%x)", want, seq, h)
		}
	}
	good := base(t, owner, "audit_events", a) // seq 4 on the head
	bad := []byte("not the head's hash, 32 bytes...")
	for what, r := range map[string]trow{
		"seq 4 with the wrong prev_hash": good.with("prev_hash", bad, "hash", sum(append(append([]byte{}, bad...), good["canonical"].([]byte)...))),
		"seq 5, skipping 4":              good.with("seq", int64(5)),
	} {
		wantRefused(t, appendEvent(t, app, a, r), "40001", "out of order", what)
	}
	if pe := wantCode(t, appendEvent(t, app, a, good.with("hash", sum([]byte("x")))), "23514", "a hash that is not sha256(prev_hash || canonical)"); pe != nil &&
		pe.ConstraintName != "audit_events_hash_chains" {
		t.Errorf("refused by %s, want audit_events_hash_chains", pe.ConstraintName)
	}
	if seq, _ := head(t, owner, a); seq != 3 {
		t.Errorf("after three refusals the head is at %d, want 3", seq)
	}
	if err := appendEvent(t, app, a, good); err != nil {
		t.Errorf("seq 4 chained right after the refusals: %v", err)
	}
}

type principal struct {
	org, user string
	created   bool
}

func resolve(t *testing.T, app *pgx.Conn, tenant, oid, name, email string) []principal {
	t.Helper()
	rows, err := app.Query(ctxT(t), "SELECT org_id, user_id, created FROM hub.resolve_principal($1::text::uuid, $2::text::uuid, $3, $4, $5)",
		tenant, oid, name, email, newID(t, "u"))
	if err != nil {
		t.Fatalf("resolve_principal: %v", err)
	}
	out, err := pgx.CollectRows(rows, func(r pgx.CollectableRow) (principal, error) {
		var p principal
		return p, r.Scan(&p.org, &p.user, &p.created)
	})
	if err != nil {
		t.Fatalf("resolve_principal: %v", err)
	}
	return out
}

func TestResolvePrincipal(t *testing.T) {
	_, owner, app, a, _ := seeded(t)
	if got := resolve(t, app, uuidOf(t), uuidOf(t), "N", "n@x.test"); len(got) != 0 {
		t.Errorf("an unknown tenant: %v, want no row", got)
	}
	oid := uuidOf(t)
	first := resolve(t, app, a.Tenant, oid, "First Name", "First@X.test")
	if len(first) != 1 || first[0].org != a.Org || !first[0].created {
		t.Fatalf("a new oid in A's tenant: %v, want one created row in %s", first, a.Org)
	}
	again := resolve(t, app, a.Tenant, oid, "First Name", "first@x.test")
	if len(again) != 1 || again[0].user != first[0].user || again[0].created {
		t.Errorf("the same oid again: %v, want %s, created false", again, first[0].user)
	}
	renamed := resolve(t, app, a.Tenant, oid, "New Name", "new@x.test")
	var name, email string
	if err := owner.QueryRow(ctxT(t), "SELECT display_name, email FROM hub.users WHERE id = $1", first[0].user).Scan(&name, &email); err != nil ||
		len(renamed) != 1 || renamed[0].user != first[0].user || name != "New Name" || email != "new@x.test" {
		t.Errorf("a changed name: %v; stored %q %q (%v), want New Name, new@x.test", renamed, name, email, err)
	}
}

func TestEnsureOrgIsIdempotent(t *testing.T) {
	d := hubtest.Migrated(t)
	app, owner := appConn(t, d), d.OwnerConn(t)
	tenant, id1, id2 := uuidOf(t), newID(t, "org"), newID(t, "org")
	for i, id := range []string{id1, id2} {
		var got string
		if err := app.QueryRow(ctxT(t), "SELECT hub.ensure_org($1::text::uuid, $2, $3)", tenant, fmt.Sprintf("Org %d", i), id).Scan(&got); err != nil || got != id1 {
			t.Errorf("ensure_org call %d with id %s = %q, %v; want the first id %s", i+1, id, got, err, id1)
		}
	}
	var orgs, heads int
	var name string
	if err := owner.QueryRow(ctxT(t), `SELECT (SELECT count(*) FROM hub.orgs), (SELECT count(*) FROM hub.audit_heads WHERE org_id = $1),
		(SELECT display_name FROM hub.orgs WHERE id = $1)`, id1).Scan(&orgs, &heads, &name); err != nil || orgs != 1 || heads != 1 || name != "Org 1" {
		t.Errorf("after two calls: %d orgs, %d heads, name %q (%v); want 1, 1, Org 1", orgs, heads, name, err)
	}
}

func TestEraseBriefChecksTheOrg(t *testing.T) {
	_, owner, app, a, b := seeded(t)
	wantCode(t, exec(t, app, b.Org, "SELECT hub.erase_brief($1)", a.Brief), "P0002", "erase A's brief from B's session")
	wantCode(t, exec(t, app, "", "SELECT hub.erase_brief($1)", a.Brief), "P0002", "erase A's brief with no org set")
	var n int
	if err := owner.QueryRow(ctxT(t), "SELECT count(*) FROM hub.brief_revisions WHERE brief_id = $1 AND canonical IS NOT NULL", a.Brief).Scan(&n); err != nil || n != 1 {
		t.Errorf("A's revisions with content after the refusals: %d (%v), want 1", n, err)
	}
}

// storedKeys is every idempotency_keys row, key to the row's whole text form.
func storedKeys(t *testing.T, owner *pgx.Conn) map[string]string {
	t.Helper()
	rows, err := owner.Query(ctxT(t), "SELECT k.key, k::text FROM hub.idempotency_keys k")
	if err != nil {
		t.Fatal(err)
	}
	out := map[string]string{}
	for rows.Next() {
		var k, v string
		if err := rows.Scan(&k, &v); err != nil {
			t.Fatal(err)
		}
		out[k] = v
	}
	if err := rows.Err(); err != nil {
		t.Fatal(err)
	}
	return out
}

func TestEraseBriefLeavesEveryStoredResponse(t *testing.T) {
	_, owner, app, a, b := seeded(t)
	ref := "refs/heads/two-" + hx(t, 4)
	a2 := base(t, app, "briefs", a).with("branch_ref", ref)
	a1, a2id := a.Brief, a2["id"].(string)
	batch := fmt.Sprintf(`{"revisions":[{"briefKey":"brief:%s@1","content":null},{"briefKey":"brief:%s@1","content":null}]}`, a1, a2id)
	approval := fmt.Sprintf(`{"designKey":%q,"briefKey":"brief:%s@1"}`, a.DesignKey, a1)
	err := inOrg(t, app, a.Org, true, func(tx pgx.Tx) error {
		ctx := ctxT(t)
		for _, step := range []func() error{
			func() error { _, err := ins(ctx, tx, "briefs", a2); return err },
			func() error {
				_, err := ins(ctx, tx, "brief_revisions", revision(t, a, a2id, 1, "branch", ref))
				return err
			},
			func() error {
				_, err := tx.Exec(ctx, "UPDATE hub.briefs SET head_revision = 1 WHERE id = $1", a2id)
				return err
			},
			func() error {
				_, err := ins(ctx, tx, "idempotency_keys", base(t, tx, "idempotency_keys", a).with("key", "r1", "status", 200,
					"path", "/v1/projects/"+a.Project+"/briefs/revisions", "response", []byte(batch)))
				return err
			},
			func() error {
				_, err := ins(ctx, tx, "idempotency_keys", base(t, tx, "idempotency_keys", a).with("key", "r2", "status", 201,
					"path", "/v1/projects/"+a.Project+"/designs/approvals", "response", []byte(approval)))
				return err
			},
			func() error {
				_, err := ins(ctx, tx, "idempotency_keys", base(t, tx, "idempotency_keys", a).with("key", "r3"))
				return err
			},
		} {
			if err := step(); err != nil {
				return err
			}
		}
		return nil
	})
	if err != nil {
		t.Fatalf("seed A's stored responses: %v", err)
	}
	if err := inOrg(t, app, b.Org, true, func(tx pgx.Tx) error {
		_, err := ins(ctxT(t), tx, "idempotency_keys", base(t, tx, "idempotency_keys", b).with("key", "r4", "status", 200, "response", []byte("names "+a1)))
		return err
	}); err != nil {
		t.Fatalf("seed B's stored response: %v", err)
	}
	before := storedKeys(t, owner)
	if err := inOrg(t, app, a.Org, true, func(tx pgx.Tx) error { _, err := tx.Exec(ctxT(t), "SELECT hub.erase_brief($1)", a1); return err }); err != nil {
		t.Fatalf("erase_brief(A1): %v", err)
	}
	after := storedKeys(t, owner)
	for _, k := range []string{"r1", "r2", "r3", "r4"} {
		if before[k] == "" || after[k] != before[k] {
			t.Errorf("stored response %s: before %q, after %q; want it kept byte for byte", k, before[k], after[k])
		}
	}
	if len(after) != len(before) {
		t.Errorf("idempotency_keys held %d rows before erase_brief and %d after", len(before), len(after))
	}
}

// begin opens a live transaction on conn with hub.org_id = org (none when ""),
// rolled back at cleanup unless the test ends it.
func begin(t *testing.T, ctx context.Context, conn *pgx.Conn, org string) pgx.Tx {
	t.Helper()
	tx, err := conn.Begin(ctx)
	if err != nil {
		t.Fatalf("begin: %v", err)
	}
	t.Cleanup(func() { tx.Rollback(context.Background()) })
	if org != "" {
		if _, err := tx.Exec(ctx, "SELECT set_config('hub.org_id', $1, true)", org); err != nil {
			t.Fatalf("set hub.org_id: %v", err)
		}
	}
	return tx
}

func TestAuditAppendChecksTheOrgFirst(t *testing.T) {
	d, owner, _, a, b := seeded(t)
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	seqB, hashB := head(t, owner, b)
	// Connection 2 goes first, and holds B's head in a live transaction.
	holder := begin(t, ctx, appConn(t, d), b.Org)
	if _, err := holder.Exec(ctx, "SELECT * FROM hub.lock_audit_head()"); err != nil {
		t.Fatalf("connection 2 locks B's head: %v", err)
	}
	// Connection 1, in A's session, appends B's next event, chained right.
	c1 := appConn(t, d)
	tx := begin(t, ctx, c1, a.Org)
	if _, err := tx.Exec(ctx, "SET LOCAL lock_timeout = '2s'"); err != nil {
		t.Fatal(err)
	}
	ev := base(t, owner, "audit_events", b) // the owner sees B's tip: seq seqB+1 on hashB
	start := time.Now()
	_, err := ins(ctx, tx, "audit_events", ev)
	took := time.Since(start)
	wantRefused(t, err, "42501", "not this session's", "A's session appends B's event")
	// The code and message are the assertion; the time is evidence only (a
	// trigger that reached B's lock first would wait lock_timeout, 2 s).
	t.Logf("the refusal took %v", took)
	tx.Rollback(ctx)
	holder.Rollback(ctx)
	if seq, h := head(t, owner, b); seq != seqB || string(h) != string(hashB) {
		t.Errorf("B's head moved: %d %x, was %d %x", seq, h, seqB, hashB)
	}
}

func TestTheHeadLockNeedsTheFunction(t *testing.T) {
	_, owner, app, a, _ := seeded(t)
	ctx := ctxT(t)
	direct := "SELECT seq, hash FROM hub.audit_heads WHERE org_id = $1 FOR UPDATE"
	wantCode(t, exec(t, app, a.Org, direct, a.Org), "42501", "hub_app's own SELECT … FOR UPDATE on audit_heads")
	tx := begin(t, ctx, app, a.Org)
	var seq int64
	var h []byte
	if err := tx.QueryRow(ctx, "SELECT seq, hash FROM hub.lock_audit_head()").Scan(&seq, &h); err != nil || seq != 1 || len(h) != 32 {
		t.Fatalf("lock_audit_head in A = %d %x, %v; want A's head, seq 1", seq, h, err)
	}
	nowait := "SELECT 1 FROM hub.audit_heads WHERE org_id = $1 FOR UPDATE NOWAIT"
	wantCode(t, exec(t, owner, "", nowait, a.Org), "55P03", "the owner's NOWAIT while hub_app holds A's head")
	if err := tx.Commit(ctx); err != nil {
		t.Fatal(err)
	}
	if err := exec(t, owner, "", nowait, a.Org); err != nil {
		t.Errorf("the owner's NOWAIT after the holder committed: %v", err)
	}
	wantCode(t, exec(t, app, "", "SELECT * FROM hub.lock_audit_head()"), "P0002", "lock_audit_head with hub.org_id unset")
}

func TestOrgIdsListsEveryOrgAndOnlyIds(t *testing.T) {
	_, _, app, a, b := seeded(t)
	var n int
	if err := app.QueryRow(ctxT(t), "SELECT count(*) FROM hub.orgs").Scan(&n); err != nil || n != 0 {
		t.Errorf("hub_app with no org: orgs shows %d rows (%v), want 0", n, err)
	}
	rows, err := app.Query(ctxT(t), "SELECT * FROM hub.org_ids()")
	if err != nil {
		t.Fatal(err)
	}
	if f := rows.FieldDescriptions(); len(f) != 1 {
		t.Errorf("org_ids returns %d columns, want 1 (the id)", len(f))
	}
	got, err := pgx.CollectRows(rows, pgx.RowTo[string])
	want := []string{a.Org, b.Org}
	sort.Strings(want)
	if err != nil || strings.Join(got, ",") != strings.Join(want, ",") {
		t.Errorf("org_ids = %v (%v), want %v in id order", got, err, want)
	}
}

func TestHubAppExecutesExactlyTheOwnerFunctions(t *testing.T) {
	d := hubtest.Migrated(t)
	conn := d.OwnerConn(t)
	rows, err := conn.Query(ctxT(t), `SELECT routine_name::text FROM information_schema.role_routine_grants
		WHERE grantee = 'hub_app' AND routine_schema = 'hub' AND privilege_type = 'EXECUTE' ORDER BY 1`)
	if err != nil {
		t.Fatal(err)
	}
	got, err := pgx.CollectRows(rows, pgx.RowTo[string])
	want := "ensure_org,erase_brief,lock_audit_head,org_ids,resolve_principal"
	if err != nil || strings.Join(got, ",") != want {
		t.Errorf("hub_app executes %v (%v), want %s", got, err, want)
	}
	var public []string
	rows, err = conn.Query(ctxT(t), `SELECT p.proname::text FROM pg_proc p
		CROSS JOIN LATERAL aclexplode(coalesce(p.proacl, acldefault('f', p.proowner))) a
		WHERE p.pronamespace = 'hub'::regnamespace AND a.grantee = 0 AND a.privilege_type = 'EXECUTE'
		  AND p.proname IN ('ensure_org', 'resolve_principal', 'erase_brief', 'lock_audit_head', 'org_ids')`)
	if err == nil {
		public, err = pgx.CollectRows(rows, pgx.RowTo[string])
	}
	if err != nil || len(public) != 0 {
		t.Errorf("PUBLIC executes %v (%v), want none of the owner functions", public, err)
	}
}
