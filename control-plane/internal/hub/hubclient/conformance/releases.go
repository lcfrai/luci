package conformance

import (
	"strings"
	"testing"
	"time"

	"github.com/kpmg-au-emu/panacea-app/control-plane/internal/hub/hubapi"
	"github.com/kpmg-au-emu/panacea-app/control-plane/internal/hub/hubclient"
)

// The releases group: server tasks 23 and 24 (contract §7.9, and task 00's
// HA7-1, HA7-2, HA7-5). Rows marked clock need Env.Advance.
var releaseRows = []row{
	{name: "pin_ship_head", fn: pinShipHead},
	{name: "pin_falls_back_to_project_brief", fn: pinFallsBackToProjectBrief},
	{name: "pin_no_brief", fn: pinNoBrief},
	{name: "pin_no_complete_revision", fn: pinNoCompleteRevision},
	{name: "pin_does_not_move", fn: pinDoesNotMove},
	{name: "reserve_refusals", fn: reserveRefusals},
	{name: "confirm_by_reserver_only", fn: confirmByReserverOnly},
	{name: "confirm_idempotent", fn: confirmIdempotent},
	{name: "confirm_other_run_409", fn: confirmOtherRun409},
	{name: "one_release_per_run", fn: oneReleasePerRun},
	{name: "abandon_from_reserved_only", fn: abandonFromReservedOnly},
	{name: "expiry", clock: true, fn: expiry},
	{name: "join_rules", fn: joinRules},
	{name: "rollback_inherits", fn: rollbackInherits},
	{name: "source_unknown", fn: sourceUnknown},
	{name: "source_unpinned", fn: sourceUnpinned},
	{name: "by_deployment_id", fn: byDeploymentID},
	{name: "environment_null_until_joined", fn: environmentNullUntilJoined},
	{name: "environment_null_abandons_and_expires", clock: true, fn: environmentNullAbandonsAndExpires},
	{name: "rollback_confirm_names_no_source", fn: rollbackConfirmNamesNoSource},
	{name: "kind_filter", fn: kindFilter},
	{name: "list_filters_and_order", fn: releaseListFiltersAndOrder},
}

const shipEnv = "evidence-upload"

// newPanacea is a Panacea project name no other row uses: (panaceaProject,
// pipelineRun) and (panaceaProject, deploymentId) are unique in the org.
func newPanacea() string { return "pp-" + uniq() }

func newCommit() string { return strings.Repeat("0", 24) + uniq() }

func newRun() string { return "run-" + uniq() }

func newDeployment() string { return "d-" + uniq() }

// runOn is a run on ref of pp to shipEnv, at a fresh commit.
func runOn(pp, ref string) hubapi.ReserveRequest {
	return hubapi.ReserveRequest{Kind: hubapi.KindRun, PanaceaProject: pp, Environment: sp(shipEnv), BranchRef: sp(ref), Commit: sp(newCommit())}
}

// rollbackOf is a rollback of pp's deployment source, to shipEnv.
func rollbackOf(pp, source string) hubapi.ReserveRequest {
	return hubapi.ReserveRequest{Kind: hubapi.KindRollback, PanaceaProject: pp, Environment: sp(shipEnv), SourceDeploymentID: sp(source)}
}

// reserve is h reserving req in projectID, which must succeed.
func reserve(t *testing.T, e Env, h hubclient.Hub, projectID string, req hubapi.ReserveRequest) hubapi.Release {
	t.Helper()
	r := must(h.Reserve(bg, projectID, req))(t)
	if r.State != hubapi.StateReserved || r.ProjectID != projectID || r.Kind != req.Kind || !hubapi.ValidID(hubapi.PrefixRelease, r.ID) ||
		r.PipelineRun != nil || r.ConfirmedAt != nil || r.Deployments == nil || len(r.Deployments) != 0 {
		t.Fatalf("Reserve = %+v, want a reserved %s of this project with no run and no deployments", r, req.Kind)
	}
	at, err1 := time.Parse(time.RFC3339Nano, r.ReservedAt)
	exp, err2 := time.Parse(time.RFC3339Nano, r.ExpiresAt)
	if err1 != nil || err2 != nil || e.ReservationTTL != 0 && exp.Sub(at) != e.ReservationTTL {
		t.Errorf("reservedAt %s, expiresAt %s: want expiresAt = reservedAt + %s", r.ReservedAt, r.ExpiresAt, e.ReservationTTL)
	}
	return r
}

// confirmRun confirms a run release with a fresh pipeline run.
func confirmRun(t *testing.T, h hubclient.Hub, r hubapi.Release) hubapi.Release {
	t.Helper()
	got := must(h.Confirm(bg, r.ID, hubapi.ConfirmRequest{PipelineRun: sp(newRun())}))(t)
	if got.State != hubapi.StateConfirmed || got.ConfirmedAt == nil || got.PipelineRun == nil {
		t.Fatalf("Confirm = %+v, want confirmed with its run", got)
	}
	return got
}

// shipped is a run of pp on ref reserved, confirmed and joined to a fresh
// deployment, which it returns.
func shipped(t *testing.T, e Env, projectID, pp, ref string) (hubapi.Release, string) {
	t.Helper()
	r := confirmRun(t, e.Member, reserve(t, e, e.Member, projectID, runOn(pp, ref)))
	d := newDeployment()
	must(e.Member.JoinDeployment(bg, r.ID, hubapi.DeploymentJoinRequest{DeploymentID: d}))(t)
	return must(e.Member.Release(bg, r.ID))(t), d
}

// wantPin asserts a pin: of rev with headRevisionAtShip head, or of nothing
// when rev is nil.
func wantPin(t *testing.T, got hubapi.Pin, status hubapi.PinStatus, reason hubapi.PinReason, rev *hubapi.BriefRevision, head int) {
	t.Helper()
	if got.Status != status || got.Reason != reason {
		t.Errorf("pin = %s/%s, want %s/%s", got.Status, got.Reason, status, reason)
	}
	if rev == nil {
		if got.BriefKey != nil || got.BriefDigest != nil || got.HeadRevisionAtShip != nil {
			t.Errorf("an empty pin carries key %v, digest %v, head %v; want all null", got.BriefKey, got.BriefDigest, got.HeadRevisionAtShip)
		}
		return
	}
	if got.BriefKey == nil || *got.BriefKey != rev.Key || got.BriefDigest == nil || *got.BriefDigest != rev.Digest ||
		got.HeadRevisionAtShip == nil || *got.HeadRevisionAtShip != head {
		t.Errorf("pin = key %v, digest %v, head %v; want %s, %s, %d", got.BriefKey, got.BriefDigest, got.HeadRevisionAtShip, rev.Key, rev.Digest, head)
	}
}

// the branch brief's latest complete revision, not its head, with the head now.
func pinShipHead(t *testing.T, e Env) {
	p := newProject(t, e)
	pb := newBrief(t, e.Member, p.ID, "")
	write(t, e.Member, p.ID, pb.ID, 0, hubapi.StatusComplete)
	b := newBrief(t, e.Member, p.ID, featRef)
	write(t, e.Member, p.ID, b.ID, 0, hubapi.StatusComplete)
	r2 := write(t, e.Member, p.ID, b.ID, 1, hubapi.StatusComplete)
	write(t, e.Member, p.ID, b.ID, 2, hubapi.StatusOpen)
	r := reserve(t, e, e.Member, p.ID, runOn(newPanacea(), featRef))
	wantPin(t, r.Pin, hubapi.PinPinned, hubapi.PinShipHead, &r2, 3)
	if r.ReservedBy.ID != meOf(t, e.Member).User.ID || r.BranchRef != featRef || r.Environment == nil || *r.Environment != shipEnv {
		t.Errorf("the release = %+v, want the member's, on %s, to %s", r, featRef, shipEnv)
	}
}

// no live branch brief for the ref → the live project brief; an explicit
// briefId wins over both.
func pinFallsBackToProjectBrief(t *testing.T, e Env) {
	p := newProject(t, e)
	pb := newBrief(t, e.Member, p.ID, "")
	pr := write(t, e.Member, p.ID, pb.ID, 0, hubapi.StatusComplete)
	gone := newBrief(t, e.Member, p.ID, featRef)
	write(t, e.Member, p.ID, gone.ID, 0, hubapi.StatusComplete)
	must(e.Member.Archive(bg, gone.ID))(t)
	other := newBrief(t, e.Member, p.ID, "refs/heads/other")
	or := write(t, e.Member, p.ID, other.ID, 0, hubapi.StatusComplete)
	wantPin(t, reserve(t, e, e.Member, p.ID, runOn(newPanacea(), featRef)).Pin, hubapi.PinPinned, hubapi.PinShipHead, &pr, 1)
	wantPin(t, reserve(t, e, e.Member, p.ID, runOn(newPanacea(), "")).Pin, hubapi.PinPinned, hubapi.PinShipHead, &pr, 1)
	req := runOn(newPanacea(), featRef)
	req.BriefID = sp(other.ID)
	wantPin(t, reserve(t, e, e.Member, p.ID, req).Pin, hubapi.PinPinned, hubapi.PinShipHead, &or, 1)
}

// no brief at all → none/no_brief, and the reservation still succeeds.
func pinNoBrief(t *testing.T, e Env) {
	p := newProject(t, e)
	wantPin(t, reserve(t, e, e.Member, p.ID, runOn(newPanacea(), featRef)).Pin, hubapi.PinNone, hubapi.PinNoBrief, nil, 0)
}

// the governing brief has no complete revision → none/no_complete_revision,
// even when the project brief has one: the branch brief governs its ref.
func pinNoCompleteRevision(t *testing.T, e Env) {
	p := newProject(t, e)
	pb := newBrief(t, e.Member, p.ID, "")
	write(t, e.Member, p.ID, pb.ID, 0, hubapi.StatusComplete)
	b := newBrief(t, e.Member, p.ID, featRef)
	write(t, e.Member, p.ID, b.ID, 0, hubapi.StatusOpen)
	write(t, e.Member, p.ID, b.ID, 1, hubapi.StatusSkipped)
	wantPin(t, reserve(t, e, e.Member, p.ID, runOn(newPanacea(), featRef)).Pin, hubapi.PinNone, hubapi.PinNoCompleteRevision, nil, 0)
}

// the pin is taken once: a later complete revision, and the confirm, leave it.
func pinDoesNotMove(t *testing.T, e Env) {
	p := newProject(t, e)
	b := newBrief(t, e.Member, p.ID, featRef)
	r1 := write(t, e.Member, p.ID, b.ID, 0, hubapi.StatusComplete)
	r := reserve(t, e, e.Member, p.ID, runOn(newPanacea(), featRef))
	write(t, e.Member, p.ID, b.ID, 1, hubapi.StatusComplete)
	wantPin(t, must(e.Viewer.Release(bg, r.ID))(t).Pin, hubapi.PinPinned, hubapi.PinShipHead, &r1, 1)
	wantPin(t, confirmRun(t, e.Member, r).Pin, hubapi.PinPinned, hubapi.PinShipHead, &r1, 1)
}

// §7.9's reserve refusals: the shapes (task 23), the design key (never
// inferred), the explicit brief (this project's), the roles.
func reserveRefusals(t *testing.T, e Env) {
	a, ap := approve(t, e)
	pp := newPanacea()
	bad := func(field string, mutate func(r *hubapi.ReserveRequest), base hubapi.ReserveRequest) {
		t.Helper()
		mutate(&base)
		_, err := e.Member.Reserve(bg, a.p.ID, base)
		wantErr(t, err, 400, hubapi.CodeBadRequest, "field", field)
	}
	run, rb := runOn(pp, featRef), rollbackOf(pp, newDeployment())
	bad("kind", func(r *hubapi.ReserveRequest) { r.Kind = "x" }, run)
	bad("panaceaProject", func(r *hubapi.ReserveRequest) { r.PanaceaProject = "Bad_Name" }, run)
	bad("environment", func(r *hubapi.ReserveRequest) { r.Environment = sp("") }, run)
	bad("commit", func(r *hubapi.ReserveRequest) { r.Commit = nil }, run)
	bad("commit", func(r *hubapi.ReserveRequest) { r.Commit = sp("A" + newCommit()[1:]) }, run)
	bad("commit", func(r *hubapi.ReserveRequest) { r.Commit = sp(newCommit()[:39]) }, run)
	bad("branchRef", func(r *hubapi.ReserveRequest) { r.BranchRef = sp("feat") }, run)
	bad("sourceDeploymentId", func(r *hubapi.ReserveRequest) { r.SourceDeploymentID = nil }, rb)
	bad("commit", func(r *hubapi.ReserveRequest) { r.Commit = sp(newCommit()) }, rb)
	bad("branchRef", func(r *hubapi.ReserveRequest) { r.BranchRef = sp(featRef) }, rb)
	bad("briefId", func(r *hubapi.ReserveRequest) { r.BriefID = sp(a.rev.BriefID) }, rb)
	bad("designKey", func(r *hubapi.ReserveRequest) { r.DesignKey = sp(ap.DesignKey) }, rb)

	unapproved := runOn(pp, featRef)
	unapproved.DesignKey = sp(newDesignKey())
	_, err := e.Member.Reserve(bg, a.p.ID, unapproved)
	wantErr(t, err, 422, hubapi.CodeDesignNotApproved, "designKey", *unapproved.DesignKey)
	foreign := runOn(pp, featRef)
	foreign.BriefID = sp(newBrief(t, e.Member, newProject(t, e).ID, featRef).ID)
	_, err = e.Member.Reserve(bg, a.p.ID, foreign)
	wantErr(t, err, 422, hubapi.CodeKeyUnresolvable)
	_, err = e.Viewer.Reserve(bg, a.p.ID, run)
	wantErr(t, err, 403, hubapi.CodeForbidden, "required", "member")
	for _, s := range strangers(t, e) {
		_, err := s.hub.Reserve(bg, a.p.ID, run)
		wantErr(t, err, 404, hubapi.CodeProjectNotFound, "projectId", a.p.ID)
	}
	if page := must(e.Viewer.Releases(bg, a.p.ID, hubapi.ReleaseQuery{}))(t); len(page.Items) != 0 {
		t.Errorf("after the refusals the project has %d releases, want 0", len(page.Items))
	}
	withDesign := runOn(pp, featRef)
	withDesign.DesignKey = sp(ap.DesignKey)
	if r := reserve(t, e, e.Member, a.p.ID, withDesign); r.DesignKey == nil || *r.DesignKey != ap.DesignKey {
		t.Errorf("an approved design key is stored as %v, want %s", r.DesignKey, ap.DesignKey)
	}
}

// only the reserver confirms or abandons: another member is 403 not_reserver,
// a viewer 403 forbidden, a stranger 404 release_not_found.
func confirmByReserverOnly(t *testing.T, e Env) {
	p := newProject(t, e)
	r := reserve(t, e, e.Admin, p.ID, runOn(newPanacea(), featRef))
	conf := hubapi.ConfirmRequest{PipelineRun: sp(newRun())}
	aband := hubapi.AbandonRequest{Reason: hubapi.AbandonCancelled}
	_, err := e.Member.Confirm(bg, r.ID, conf)
	wantErr(t, err, 403, hubapi.CodeNotReserver, "releaseId", r.ID)
	_, err = e.Member.Abandon(bg, r.ID, aband)
	wantErr(t, err, 403, hubapi.CodeNotReserver, "releaseId", r.ID)
	_, err = e.Viewer.Confirm(bg, r.ID, conf)
	wantErr(t, err, 403, hubapi.CodeForbidden, "required", "member")
	_, err = e.Viewer.Abandon(bg, r.ID, aband)
	wantErr(t, err, 403, hubapi.CodeForbidden, "required", "member")
	for _, s := range strangers(t, e) {
		for _, err := range []error{second(s.hub.Confirm(bg, r.ID, conf)), second(s.hub.Abandon(bg, r.ID, aband)), second(s.hub.Release(bg, r.ID))} {
			wantErr(t, err, 404, hubapi.CodeReleaseNotFound)
		}
	}
	if got := must(e.Member.Release(bg, r.ID))(t); got.State != hubapi.StateReserved {
		t.Errorf("after the refusals the release is %s, want reserved", got.State)
	}
	confirmRun(t, e.Admin, r)
}

// second is a call's error alone.
func second[T any](_ T, err error) error { return err }

// the same run again → 200, unchanged, one release.confirmed; a rollback's
// {} twice → 200 both times.
func confirmIdempotent(t *testing.T, e Env) {
	p := newProject(t, e)
	before := head(t, e.Member)
	r := reserve(t, e, e.Member, p.ID, runOn(newPanacea(), featRef))
	req := hubapi.ConfirmRequest{PipelineRun: sp(newRun())}
	c1 := must(e.Member.Confirm(bg, r.ID, req))(t)
	c2 := must(e.Member.Confirm(bg, r.ID, req))(t)
	if c1.State != hubapi.StateConfirmed || c2.State != hubapi.StateConfirmed || *c2.PipelineRun != *req.PipelineRun || *c2.ConfirmedAt != *c1.ConfirmedAt {
		t.Errorf("confirm twice = %+v then %+v, want one confirmation, unchanged", c1, c2)
	}
	if n := count(since(t, e.Member, before), "release.confirmed", r.ID); n != 1 {
		t.Errorf("%d release.confirmed events for one run confirmed twice, want 1", n)
	}
	rb := reserve(t, e, e.Member, p.ID, rollbackOf(newPanacea(), newDeployment()))
	for range 2 {
		if got := must(e.Member.Confirm(bg, rb.ID, hubapi.ConfirmRequest{}))(t); got.State != hubapi.StateConfirmed || got.PipelineRun != nil {
			t.Errorf("a rollback's {} confirm = %+v, want confirmed with no run", got)
		}
	}
}

// a confirmed run named with another run → 409 release_state; a run's
// confirm without a run, or a run outside the grammar → 400 pipelineRun.
func confirmOtherRun409(t *testing.T, e Env) {
	p := newProject(t, e)
	r := reserve(t, e, e.Member, p.ID, runOn(newPanacea(), featRef))
	for _, bad := range []*string{nil, sp(""), sp("Bad Run")} {
		_, err := e.Member.Confirm(bg, r.ID, hubapi.ConfirmRequest{PipelineRun: bad})
		wantErr(t, err, 400, hubapi.CodeBadRequest, "field", "pipelineRun")
	}
	c := confirmRun(t, e.Member, r)
	_, err := e.Member.Confirm(bg, r.ID, hubapi.ConfirmRequest{PipelineRun: sp(newRun())})
	wantErr(t, err, 409, hubapi.CodeReleaseState, "releaseId", r.ID, "state", "confirmed")
	if got := must(e.Member.Release(bg, r.ID))(t); *got.PipelineRun != *c.PipelineRun {
		t.Errorf("the refused run replaced %s with %s", *c.PipelineRun, *got.PipelineRun)
	}
}

// two releases of one Panacea project confirming one run → the second 409 conflict.
func oneReleasePerRun(t *testing.T, e Env) {
	p := newProject(t, e)
	pp, run := newPanacea(), newRun()
	r1 := reserve(t, e, e.Member, p.ID, runOn(pp, featRef))
	r2 := reserve(t, e, e.Member, p.ID, runOn(pp, featRef))
	must(e.Member.Confirm(bg, r1.ID, hubapi.ConfirmRequest{PipelineRun: sp(run)}))(t)
	_, err := e.Member.Confirm(bg, r2.ID, hubapi.ConfirmRequest{PipelineRun: sp(run)})
	wantErr(t, err, 409, hubapi.CodeConflict)
	if got := must(e.Member.Release(bg, r2.ID))(t); got.State != hubapi.StateReserved {
		t.Errorf("the refused release is %s, want still reserved", got.State)
	}
	r3 := reserve(t, e, e.Member, p.ID, runOn(newPanacea(), featRef))
	must(e.Member.Confirm(bg, r3.ID, hubapi.ConfirmRequest{PipelineRun: sp(run)}))(t) // another Panacea project's run
}

// abandon from reserved only, with its reason and message; never revived.
func abandonFromReservedOnly(t *testing.T, e Env) {
	p := newProject(t, e)
	r := reserve(t, e, e.Member, p.ID, runOn(newPanacea(), featRef))
	_, err := e.Member.Abandon(bg, r.ID, hubapi.AbandonRequest{Reason: "x"})
	wantErr(t, err, 400, hubapi.CodeBadRequest, "field", "reason")
	_, err = e.Member.Abandon(bg, r.ID, hubapi.AbandonRequest{Reason: hubapi.AbandonCancelled, Message: strings.Repeat("m", 501)})
	wantErr(t, err, 400, hubapi.CodeBadRequest, "field", "message")
	msg := "Panacea refused the deploy: quota."
	a := must(e.Member.Abandon(bg, r.ID, hubapi.AbandonRequest{Reason: hubapi.AbandonDeployFailed, Message: msg}))(t)
	if a.State != hubapi.StateAbandoned || a.AbandonedAt == nil || a.AbandonReason == nil || *a.AbandonReason != hubapi.AbandonDeployFailed ||
		a.AbandonMessage == nil || *a.AbandonMessage != msg {
		t.Errorf("Abandon = %+v, want abandoned with its reason and message", a)
	}
	_, err = e.Member.Abandon(bg, r.ID, hubapi.AbandonRequest{Reason: hubapi.AbandonCancelled})
	wantErr(t, err, 409, hubapi.CodeReleaseState, "releaseId", r.ID, "state", "abandoned")
	_, err = e.Member.Confirm(bg, r.ID, hubapi.ConfirmRequest{PipelineRun: sp(newRun())})
	wantErr(t, err, 409, hubapi.CodeReleaseState, "releaseId", r.ID, "state", "abandoned")
	c := confirmRun(t, e.Member, reserve(t, e, e.Member, p.ID, runOn(newPanacea(), featRef)))
	_, err = e.Member.Abandon(bg, c.ID, hubapi.AbandonRequest{Reason: hubapi.AbandonCancelled})
	wantErr(t, err, 409, hubapi.CodeReleaseState, "releaseId", c.ID, "state", "confirmed")
}

// ⏱ past expiresAt a reservation reads expired and cannot be confirmed; a
// second before, it can.
func expiry(t *testing.T, e Env) {
	p := newProject(t, e)
	early := reserve(t, e, e.Member, p.ID, runOn(newPanacea(), featRef))
	late := reserve(t, e, e.Member, p.ID, runOn(newPanacea(), featRef))
	e.Advance(e.ReservationTTL - time.Second)
	confirmRun(t, e.Member, early)
	e.Advance(2 * time.Second)
	if got := must(e.Viewer.Release(bg, late.ID))(t); got.State != hubapi.StateExpired {
		t.Errorf("an overdue reservation reads %s, want expired", got.State)
	}
	_, err := e.Member.Confirm(bg, late.ID, hubapi.ConfirmRequest{PipelineRun: sp(newRun())})
	wantErr(t, err, 409, hubapi.CodeReleaseExpired, "releaseId", late.ID)
	_, err = e.Member.Abandon(bg, late.ID, hubapi.AbandonRequest{Reason: hubapi.AbandonCancelled})
	wantErr(t, err, 409, hubapi.CodeReleaseState, "releaseId", late.ID, "state", "expired")
	if got := must(e.Viewer.Release(bg, early.ID))(t); got.State != hubapi.StateConfirmed {
		t.Errorf("a confirmed release past its expiresAt reads %s, want confirmed", got.State)
	}
}

// joins: confirmed only; a repeat answers the join; one release per
// (panaceaProject, deploymentId) in the org; a viewer 403, a stranger 404.
func joinRules(t *testing.T, e Env) {
	p := newProject(t, e)
	pp, d := newPanacea(), newDeployment()
	r := reserve(t, e, e.Member, p.ID, runOn(pp, featRef))
	join := hubapi.DeploymentJoinRequest{DeploymentID: d}
	_, err := e.Member.JoinDeployment(bg, r.ID, join)
	wantErr(t, err, 409, hubapi.CodeReleaseState, "releaseId", r.ID, "state", "reserved")
	confirmRun(t, e.Member, r)
	for _, id := range []string{"", strings.Repeat("d", 129)} {
		_, err := e.Member.JoinDeployment(bg, r.ID, hubapi.DeploymentJoinRequest{DeploymentID: id})
		wantErr(t, err, 400, hubapi.CodeBadRequest, "field", "deploymentId")
	}
	_, err = e.Viewer.JoinDeployment(bg, r.ID, join)
	wantErr(t, err, 403, hubapi.CodeForbidden, "required", "member")
	for _, s := range strangers(t, e) {
		wantErr(t, second(s.hub.JoinDeployment(bg, r.ID, join)), 404, hubapi.CodeReleaseNotFound)
	}
	j1 := must(e.Admin.JoinDeployment(bg, r.ID, join))(t) // any member joins: the poller runs as whoever is signed in
	j2 := must(e.Member.JoinDeployment(bg, r.ID, join))(t)
	if j1.DeploymentID != d || j1.JoinedAt == "" || j2 != j1 {
		t.Errorf("join then repeat = %+v, %+v; want %s once, the repeat answering the join", j1, j2, d)
	}
	other := confirmRun(t, e.Member, reserve(t, e, e.Member, p.ID, runOn(pp, featRef)))
	_, err = e.Member.JoinDeployment(bg, other.ID, join)
	wantErr(t, err, 409, hubapi.CodeConflict)
	elsewhere := confirmRun(t, e.Member, reserve(t, e, e.Member, p.ID, runOn(newPanacea(), featRef)))
	must(e.Member.JoinDeployment(bg, elsewhere.ID, join))(t) // another Panacea project's deployment
	if got := must(e.Member.Release(bg, r.ID))(t).Deployments; len(got) != 1 || got[0] != j1 {
		t.Errorf("deployments = %+v, want the one join", got)
	}
	if got := must(e.Member.Release(bg, other.ID))(t).Deployments; len(got) != 0 {
		t.Errorf("the refused release holds %+v, want no join", got)
	}
}

// a rollback of a confirmed, joined, pinned release copies its pin as
// inherited, and its commit, branch and design key; a run from the same
// source copies the pin alone.
func rollbackInherits(t *testing.T, e Env) {
	a, ap := approve(t, e)
	pp := newPanacea()
	req := runOn(pp, featRef)
	req.DesignKey = sp(ap.DesignKey)
	src := confirmRun(t, e.Member, reserve(t, e, e.Member, a.p.ID, req))
	d := newDeployment()
	must(e.Member.JoinDeployment(bg, src.ID, hubapi.DeploymentJoinRequest{DeploymentID: d}))(t)
	write(t, e.Member, a.p.ID, a.rev.BriefID, 1, hubapi.StatusComplete) // the brief moves on; the rollback does not
	rb := reserve(t, e, e.Member, a.p.ID, rollbackOf(pp, d))
	wantPin(t, rb.Pin, hubapi.PinPinned, hubapi.PinInherited, &a.rev, 1)
	if rb.SourceReleaseID == nil || *rb.SourceReleaseID != src.ID || rb.SourceDeploymentID == nil || *rb.SourceDeploymentID != d ||
		rb.Commit == nil || *rb.Commit != *src.Commit || rb.BranchRef != featRef || rb.DesignKey == nil || *rb.DesignKey != ap.DesignKey {
		t.Errorf("the rollback = %+v, want source %s's commit, branch and design key", rb, src.ID)
	}
	redeploy := hubapi.ReserveRequest{Kind: hubapi.KindRun, PanaceaProject: pp, Environment: sp(shipEnv), SourceDeploymentID: sp(d)}
	rr := reserve(t, e, e.Member, a.p.ID, redeploy)
	wantPin(t, rr.Pin, hubapi.PinPinned, hubapi.PinInherited, &a.rev, 1)
	if rr.SourceReleaseID == nil || *rr.SourceReleaseID != src.ID || rr.Commit != nil || rr.DesignKey != nil {
		t.Errorf("a run from the source = %+v, want the pin and source copied, no commit or design key", rr)
	}
}

// a source no confirmed release of this project is joined to → none/
// source_unknown, recorded, not refused; a reserved-only join counts for nothing.
func sourceUnknown(t *testing.T, e Env) {
	p := newProject(t, e)
	pp := newPanacea()
	rb := reserve(t, e, e.Member, p.ID, rollbackOf(pp, newDeployment()))
	wantPin(t, rb.Pin, hubapi.PinNone, hubapi.PinSourceUnknown, nil, 0)
	if rb.SourceReleaseID != nil || rb.Commit != nil {
		t.Errorf("an unknown source's rollback = %+v, want no source release and no commit", rb)
	}
	_, d := shipped(t, e, newProject(t, e).ID, pp, featRef) // another project's deployment
	wantPin(t, reserve(t, e, e.Member, p.ID, rollbackOf(pp, d)).Pin, hubapi.PinNone, hubapi.PinSourceUnknown, nil, 0)
}

// a source release with no pin → none/source_unpinned, naming the source.
func sourceUnpinned(t *testing.T, e Env) {
	p := newProject(t, e)
	pp := newPanacea()
	src, d := shipped(t, e, p.ID, pp, featRef) // no brief: none/no_brief
	rb := reserve(t, e, e.Member, p.ID, rollbackOf(pp, d))
	wantPin(t, rb.Pin, hubapi.PinNone, hubapi.PinSourceUnpinned, nil, 0)
	if rb.SourceReleaseID == nil || *rb.SourceReleaseID != src.ID || rb.Commit == nil || *rb.Commit != *src.Commit {
		t.Errorf("the rollback = %+v, want source %s and its commit", rb, src.ID)
	}
}

// Releases(deploymentId) finds the joined release; an unjoined id finds none.
func byDeploymentID(t *testing.T, e Env) {
	p := newProject(t, e)
	src, d := shipped(t, e, p.ID, newPanacea(), featRef)
	confirmRun(t, e.Member, reserve(t, e, e.Member, p.ID, runOn(newPanacea(), featRef)))
	page := must(e.Viewer.Releases(bg, p.ID, hubapi.ReleaseQuery{DeploymentID: d}))(t)
	if len(page.Items) != 1 || page.Items[0].ID != src.ID {
		t.Errorf("Releases(deploymentId %s) = %d items, want %s", d, len(page.Items), src.ID)
	}
	if page := must(e.Viewer.Releases(bg, p.ID, hubapi.ReleaseQuery{DeploymentID: newDeployment()}))(t); len(page.Items) != 0 || page.Items == nil {
		t.Errorf("an unjoined deployment id lists %d releases, want [] (no brief is recorded)", len(page.Items))
	}
}

// HA7-1: a first Ship reserves with a null environment, confirms still null,
// and its first join names it; a different one later is 409, nothing joined;
// a rollback, or a run from a source, may not be null.
func environmentNullUntilJoined(t *testing.T, e Env) {
	p := newProject(t, e)
	pp := newPanacea()
	req := runOn(pp, featRef)
	req.Environment = nil
	r := reserve(t, e, e.Member, p.ID, req)
	if r.Environment != nil {
		t.Errorf("reserved with a null environment, the release says %q", *r.Environment)
	}
	if c := confirmRun(t, e.Member, r); c.Environment != nil {
		t.Errorf("a confirm wrote environment %q", *c.Environment)
	}
	d1, d2 := newDeployment(), newDeployment()
	must(e.Member.JoinDeployment(bg, r.ID, hubapi.DeploymentJoinRequest{DeploymentID: d1, Environment: sp("feat-x")}))(t)
	if got := must(e.Member.Release(bg, r.ID))(t); got.Environment == nil || *got.Environment != "feat-x" {
		t.Errorf("after the join the environment is %v, want feat-x", got.Environment)
	}
	_, err := e.Member.JoinDeployment(bg, r.ID, hubapi.DeploymentJoinRequest{DeploymentID: d2, Environment: sp("feat-y")})
	wantErr(t, err, 409, hubapi.CodeConflict, "field", "environment", "environment", "feat-x")
	if got := must(e.Member.Release(bg, r.ID))(t); len(got.Deployments) != 1 || got.Deployments[0].DeploymentID != d1 || *got.Environment != "feat-x" {
		t.Errorf("after the refused join: %+v, env %v; want only %s, feat-x", got.Deployments, got.Environment, d1)
	}
	must(e.Member.JoinDeployment(bg, r.ID, hubapi.DeploymentJoinRequest{DeploymentID: d2}))(t) // none named: joins
	rb := rollbackOf(pp, d1)
	rb.Environment = nil
	_, err = e.Member.Reserve(bg, p.ID, rb)
	wantErr(t, err, 400, hubapi.CodeBadRequest, "field", "environment")
	redeploy := hubapi.ReserveRequest{Kind: hubapi.KindRun, PanaceaProject: pp, SourceDeploymentID: sp(d1)}
	_, err = e.Member.Reserve(bg, p.ID, redeploy)
	wantErr(t, err, 400, hubapi.CodeBadRequest, "field", "environment")
}

// ⏱ HA7-1: a null environment refuses neither the abandon nor the expiry.
func environmentNullAbandonsAndExpires(t *testing.T, e Env) {
	p := newProject(t, e)
	req := runOn(newPanacea(), featRef)
	req.Environment = nil
	r1, r2 := reserve(t, e, e.Member, p.ID, req), reserve(t, e, e.Member, p.ID, req)
	if a := must(e.Member.Abandon(bg, r1.ID, hubapi.AbandonRequest{Reason: hubapi.AbandonDeployFailed}))(t); a.State != hubapi.StateAbandoned || a.Environment != nil {
		t.Errorf("a null-environment abandon = %s, env %v; want abandoned, null", a.State, a.Environment)
	}
	e.Advance(e.ReservationTTL + time.Second)
	if got := must(e.Member.Release(bg, r2.ID))(t); got.State != hubapi.StateExpired || got.Environment != nil {
		t.Errorf("a null-environment reservation past expiresAt = %s, env %v; want expired, null", got.State, got.Environment)
	}
}

// HA7-2: a rollback confirmed naming its own source deployment → 409
// conflict, nothing joined, still reserved; {} → confirmed.
func rollbackConfirmNamesNoSource(t *testing.T, e Env) {
	p := newProject(t, e)
	src := newDeployment()
	rb := reserve(t, e, e.Member, p.ID, rollbackOf(newPanacea(), src))
	_, err := e.Member.Confirm(bg, rb.ID, hubapi.ConfirmRequest{DeploymentID: sp(src)})
	wantErr(t, err, 409, hubapi.CodeConflict, "field", "deploymentId")
	if got := must(e.Member.Release(bg, rb.ID))(t); got.State != hubapi.StateReserved || len(got.Deployments) != 0 {
		t.Errorf("after the refusal: %s with %+v, want reserved, no join", got.State, got.Deployments)
	}
	if got := must(e.Member.Confirm(bg, rb.ID, hubapi.ConfirmRequest{}))(t); got.State != hubapi.StateConfirmed || len(got.Deployments) != 0 {
		t.Errorf("confirm {} = %s with %+v, want confirmed, no join", got.State, got.Deployments)
	}
	other := reserve(t, e, e.Member, p.ID, rollbackOf(newPanacea(), newDeployment()))
	d := newDeployment()
	if got := must(e.Member.Confirm(bg, other.ID, hubapi.ConfirmRequest{DeploymentID: sp(d)}))(t); len(got.Deployments) != 1 || got.Deployments[0].DeploymentID != d {
		t.Errorf("a rollback confirmed with its deployment = %+v, want it joined", got.Deployments)
	}
}

// HA7-5: Kind filters runs from rollbacks in one environment; an unknown kind is 400.
func kindFilter(t *testing.T, e Env) {
	p := newProject(t, e)
	pp := newPanacea()
	src, d := shipped(t, e, p.ID, pp, featRef)
	rb := must(e.Member.Confirm(bg, reserve(t, e, e.Member, p.ID, rollbackOf(pp, d)).ID, hubapi.ConfirmRequest{}))(t)
	for kind, want := range map[hubapi.ReleaseKind]string{hubapi.KindRollback: rb.ID, hubapi.KindRun: src.ID} {
		page := must(e.Viewer.Releases(bg, p.ID, hubapi.ReleaseQuery{Environment: shipEnv, Kind: kind}))(t)
		if len(page.Items) != 1 || page.Items[0].ID != want {
			t.Errorf("Kind %s = %d items, want only %s", kind, len(page.Items), want)
		}
	}
	_, err := e.Viewer.Releases(bg, p.ID, hubapi.ReleaseQuery{Kind: "x"})
	wantErr(t, err, 400, hubapi.CodeBadRequest, "field", "kind")
}

// Releases filters by environment (never matching a null one), Panacea
// project, run and state, newest reservedAt first; strangers see no project.
func releaseListFiltersAndOrder(t *testing.T, e Env) {
	p := newProject(t, e)
	pp, other := newPanacea(), newPanacea()
	r1 := confirmRun(t, e.Member, reserve(t, e, e.Member, p.ID, runOn(pp, featRef)))
	tick(e)
	q2 := runOn(other, featRef)
	q2.Environment = sp("staging")
	r2 := reserve(t, e, e.Member, p.ID, q2)
	tick(e)
	q3 := runOn(pp, featRef)
	q3.Environment = nil
	r3 := reserve(t, e, e.Member, p.ID, q3)
	ids := func(q hubapi.ReleaseQuery) []string {
		t.Helper()
		var out []string
		for _, r := range must(e.Viewer.Releases(bg, p.ID, q))(t).Items {
			out = append(out, r.ID)
		}
		return out
	}
	for name, c := range map[string]struct {
		q    hubapi.ReleaseQuery
		want []string
	}{
		"all, newest first": {hubapi.ReleaseQuery{}, []string{r3.ID, r2.ID, r1.ID}},
		"environment":       {hubapi.ReleaseQuery{Environment: shipEnv}, []string{r1.ID}},
		"panaceaProject":    {hubapi.ReleaseQuery{PanaceaProject: pp}, []string{r3.ID, r1.ID}},
		"pipelineRun":       {hubapi.ReleaseQuery{PipelineRun: *r1.PipelineRun}, []string{r1.ID}},
		"state":             {hubapi.ReleaseQuery{State: hubapi.StateReserved}, []string{r3.ID, r2.ID}},
	} {
		if got := ids(c.q); strings.Join(got, ",") != strings.Join(c.want, ",") {
			t.Errorf("Releases %s = %v, want %v", name, got, c.want)
		}
	}
	for _, s := range strangers(t, e) {
		wantErr(t, second(s.hub.Releases(bg, p.ID, hubapi.ReleaseQuery{})), 404, hubapi.CodeProjectNotFound, "projectId", p.ID)
	}
}
