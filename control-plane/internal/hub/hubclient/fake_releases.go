package hubclient

import (
	"context"
	"regexp"
	"slices"
	"time"
	"unicode/utf8"

	"github.com/kpmg-au-emu/panacea-app/control-plane/internal/hub/hubapi"
	"github.com/kpmg-au-emu/panacea-app/control-plane/internal/ids"
)

// The fake's releases (contract §7.9; server tasks 23 and 24; task 00's
// HA7-1, HA7-2, HA7-5). The pin is taken at reserve and never again.

type fakeRelease struct {
	id, projectID      string
	kind               hubapi.ReleaseKind
	state              hubapi.ReleaseState
	panaceaProject     string
	environment        *string // null until a join names it (HA7-1)
	branchRef          string
	commit             *string
	pipelineRun        *string
	sourceDeploymentID *string
	sourceReleaseID    *string
	pin                hubapi.Pin
	designKey          *string
	deployments        []hubapi.DeploymentJoin
	reservedAt         string
	reservedBy         string // a user id
	expires            time.Time
	confirmedAt        *string
	abandonedAt        *string
	abandonReason      *hubapi.AbandonReason
	abandonMessage     *string
	seq                int64
}

func (r *fakeRelease) order() int64 { return r.seq }

var (
	panaceaNameRe = regexp.MustCompile(`^[a-z0-9-]{1,63}$`)   // panaceaProject, environment
	commitRe      = regexp.MustCompile(`^[0-9a-f]{40}$`)      // a commit
	pipelineRunRe = regexp.MustCompile(`^[a-z0-9.-]{1,253}$`) // a PipelineRun
)

// sweep is the janitor (task 24, §12.5 step 1), run at every call rather than
// every 60 s: a reserved release past expiresAt becomes expired, with one
// release.expired whose actor is null. Through the interface the two are the
// same: no caller ever reads a reservation it can no longer confirm as live.
func (w *fakeWorld) sweep() {
	now := w.now()
	for _, r := range inOrder(w.releases) {
		if r.state == hubapi.StateReserved && now.After(r.expires) {
			r.state = hubapi.StateExpired
			w.emitIn("", "release.expired", w.projects[r.projectID], r.id)
		}
	}
}

// clone is a copy of p, so no caller holds the fake's own pointer.
func clone[T any](p *T) *T {
	if p == nil {
		return nil
	}
	v := *p
	return &v
}

func (w *fakeWorld) renderRelease(r *fakeRelease) hubapi.Release {
	pin := r.pin
	pin.BriefKey, pin.BriefDigest, pin.HeadRevisionAtShip = clone(pin.BriefKey), clone(pin.BriefDigest), clone(pin.HeadRevisionAtShip)
	return hubapi.Release{ID: r.id, ProjectID: r.projectID, Kind: r.kind, State: r.state, PanaceaProject: r.panaceaProject,
		Environment: clone(r.environment), BranchRef: r.branchRef, Commit: clone(r.commit), PipelineRun: clone(r.pipelineRun),
		SourceDeploymentID: clone(r.sourceDeploymentID), SourceReleaseID: clone(r.sourceReleaseID), Pin: pin,
		DesignKey: clone(r.designKey), Deployments: append([]hubapi.DeploymentJoin{}, r.deployments...),
		ReservedAt: r.reservedAt, ReservedBy: w.users[r.reservedBy], ExpiresAt: hubapi.FormatTime(r.expires),
		ConfirmedAt: clone(r.confirmedAt), AbandonedAt: clone(r.abandonedAt), AbandonReason: clone(r.abandonReason),
		AbandonMessage: clone(r.abandonMessage)}
}

// joined is the release of the org holding (panaceaProject, deploymentID), or nil.
func (w *fakeWorld) joined(panaceaProject, deploymentID string) *fakeRelease {
	for _, r := range w.releases {
		if r.panaceaProject == panaceaProject && slices.ContainsFunc(r.deployments, func(d hubapi.DeploymentJoin) bool {
			return d.DeploymentID == deploymentID
		}) {
			return r
		}
	}
	return nil
}

// ── reserve (task 23 route 35) ──────────────────────────────────────────────

// checkReserve is route 35's 400s, in task 23's field order.
func checkReserve(req hubapi.ReserveRequest) error {
	run := req.Kind == hubapi.KindRun
	source := req.SourceDeploymentID != nil
	switch {
	case !run && req.Kind != hubapi.KindRollback:
		return badRequest("kind", "A release is a run or a rollback.")
	case !panaceaNameRe.MatchString(req.PanaceaProject):
		return badRequest("panaceaProject", "A Panacea project is 1 to 63 of a-z, 0-9 and hyphen.")
	case req.Environment == nil && (!run || source):
		return badRequest("environment", "Only a run from no source may leave its environment to the deploy.")
	case req.Environment != nil && !panaceaNameRe.MatchString(*req.Environment):
		return badRequest("environment", "An environment is 1 to 63 of a-z, 0-9 and hyphen.")
	case source && (len(*req.SourceDeploymentID) < 1 || len(*req.SourceDeploymentID) > 128), !run && !source:
		return badRequest("sourceDeploymentId", "A rollback names its source deployment, 1 to 128 characters.")
	}
	if !run {
		for _, c := range []struct {
			field string
			set   bool
		}{{"commit", req.Commit != nil}, {"branchRef", req.BranchRef != nil}, {"briefId", req.BriefID != nil}, {"designKey", req.DesignKey != nil}} {
			if c.set {
				return badRequest(c.field, "A rollback takes its "+c.field+" from its source; it sends none.")
			}
		}
		return nil
	}
	switch {
	case req.Commit == nil && !source, req.Commit != nil && !commitRe.MatchString(*req.Commit):
		return badRequest("commit", "A run names its commit, 40 lowercase hex.")
	case req.BranchRef != nil && *req.BranchRef != "" && !hubapi.ValidBranchRef(*req.BranchRef):
		return badBranchRef()
	case req.BriefID != nil && !hubapi.ValidID(hubapi.PrefixBrief, *req.BriefID):
		return badRequest("briefId", "A brief id is bf- and 16 hex.")
	case req.DesignKey != nil && second(hubapi.ParseDesignKey(*req.DesignKey)) != nil:
		return badRequest("designKey", "A design key is design:<canvas id>@<revision>.")
	}
	return nil
}

// shipPin is §7.9's first case: the brief, then its latest complete revision.
func shipPin(b *fakeBrief) hubapi.Pin {
	switch {
	case b == nil:
		return hubapi.Pin{Status: hubapi.PinNone, Reason: hubapi.PinNoBrief}
	case b.latestComplete == 0:
		return hubapi.Pin{Status: hubapi.PinNone, Reason: hubapi.PinNoCompleteRevision}
	}
	key := hubapi.BriefKey{BriefID: b.id, Revision: b.latestComplete}.String()
	digest, head := b.revisions[b.latestComplete-1].digest, len(b.revisions)
	return hubapi.Pin{Status: hubapi.PinPinned, Reason: hubapi.PinShipHead, BriefKey: &key, BriefDigest: &digest, HeadRevisionAtShip: &head}
}

// inheritedPin is §7.9's second case, from the source release (nil: unknown).
func inheritedPin(src *fakeRelease) hubapi.Pin {
	switch {
	case src == nil:
		return hubapi.Pin{Status: hubapi.PinNone, Reason: hubapi.PinSourceUnknown}
	case src.pin.Status != hubapi.PinPinned:
		return hubapi.Pin{Status: hubapi.PinNone, Reason: hubapi.PinSourceUnpinned}
	}
	return hubapi.Pin{Status: hubapi.PinPinned, Reason: hubapi.PinInherited, BriefKey: clone(src.pin.BriefKey),
		BriefDigest: clone(src.pin.BriefDigest), HeadRevisionAtShip: clone(src.pin.HeadRevisionAtShip)}
}

// Reserve is route 35: member; the shapes; the explicit brief (this
// project's, 422 key_unresolvable) and design key (approved here, 422
// design_not_approved; never inferred); then the pin, once.
func (f *Fake) Reserve(_ context.Context, projectID string, req hubapi.ReserveRequest) (hubapi.Release, error) {
	w := f.w
	w.mu.Lock()
	defer w.mu.Unlock()
	me, err := f.enter("Reserve", projectID, req)
	if err != nil {
		return hubapi.Release{}, err
	}
	p, mine, err := w.projectOf(projectID, me)
	if err != nil {
		return hubapi.Release{}, err
	}
	if err := need(mine, hubapi.RoleMember); err != nil {
		return hubapi.Release{}, err
	}
	if err := checkReserve(req); err != nil {
		return hubapi.Release{}, err
	}
	if req.BriefID != nil {
		if b := w.briefs[*req.BriefID]; b == nil || b.projectID != p.id {
			return hubapi.Release{}, problem(422, hubapi.CodeKeyUnresolvable, "That brief is not one of this project's.", "briefId", *req.BriefID)
		}
	}
	if req.DesignKey != nil && w.approvalIn(p.id, *req.DesignKey) == nil {
		return hubapi.Release{}, problem(422, hubapi.CodeDesignNotApproved, "That design revision is not approved in this project.", "designKey", *req.DesignKey)
	}
	now := w.now()
	r := &fakeRelease{id: ids.Wide(hubapi.PrefixRelease), projectID: p.id, kind: req.Kind, state: hubapi.StateReserved,
		panaceaProject: req.PanaceaProject, environment: clone(req.Environment), commit: clone(req.Commit),
		sourceDeploymentID: clone(req.SourceDeploymentID), designKey: clone(req.DesignKey),
		reservedAt: hubapi.FormatTime(now), reservedBy: me, expires: now.Add(w.ttl), seq: w.next()}
	if req.BranchRef != nil {
		r.branchRef = *req.BranchRef
	}
	if req.SourceDeploymentID != nil {
		// The confirmed release in this project joined to (panaceaProject, source).
		src := w.joined(req.PanaceaProject, *req.SourceDeploymentID)
		if src != nil && (src.projectID != p.id || src.state != hubapi.StateConfirmed) {
			src = nil
		}
		r.pin = inheritedPin(src)
		if src != nil {
			r.sourceReleaseID = &src.id
			if r.kind == hubapi.KindRollback {
				r.commit, r.branchRef, r.designKey = clone(src.commit), src.branchRef, clone(src.designKey)
			}
		}
	} else {
		var b *fakeBrief
		switch {
		case req.BriefID != nil:
			b = w.briefs[*req.BriefID]
		case r.branchRef != "":
			b = w.liveBrief(p.id, hubapi.ScopeBranch, r.branchRef)
		}
		if b == nil && req.BriefID == nil {
			b = w.liveBrief(p.id, hubapi.ScopeProject, "")
		}
		r.pin = shipPin(b)
	}
	w.releases[r.id] = r
	w.emitIn(me, "release.reserved", p, r.id, "kind", string(r.kind), "environment", deref(r.environment),
		"pinStatus", string(r.pin.Status), "briefKey", deref(r.pin.BriefKey))
	return w.renderRelease(r), nil
}

// deref is *p, or "" for null: how §8's data spells a null (task 23).
func deref(p *string) string {
	if p == nil {
		return ""
	}
	return *p
}

// ── confirm, abandon, join (task 24 routes 38–40) ───────────────────────────

// releaseOf is releaseID's release and the caller's role in its project; one
// the caller cannot read is 404, as if it did not exist.
func (w *fakeWorld) releaseOf(releaseID, me string) (*fakeRelease, hubapi.Role, error) {
	if r := w.releases[releaseID]; r != nil {
		if m := w.teams[w.projects[r.projectID].teamID].members[me]; m != nil {
			return r, m.role, nil
		}
	}
	return nil, "", problem(404, hubapi.CodeReleaseNotFound, "There is no such release that you can see.", "releaseId", releaseID)
}

// reserverOf is releaseOf, then member (403 forbidden), then the reserver
// (403 not_reserver): routes 38 and 39.
func (w *fakeWorld) reserverOf(releaseID, me string) (*fakeRelease, error) {
	r, role, err := w.releaseOf(releaseID, me)
	if err != nil {
		return nil, err
	}
	if err := need(role, hubapi.RoleMember); err != nil {
		return nil, err
	}
	if r.reservedBy != me {
		return nil, problem(403, hubapi.CodeNotReserver, "Only whoever reserved a release confirms or abandons it.", "releaseId", r.id)
	}
	return r, nil
}

func releaseState(r *fakeRelease) error {
	return problem(409, hubapi.CodeReleaseState, "The release is "+string(r.state)+".", "releaseId", r.id, "state", string(r.state))
}

func badDeploymentID(id string) bool { return len(id) < 1 || len(id) > 128 }

// checkConfirm is route 38's body: a run's pipelineRun; a rollback's {} or
// deploymentId.
func checkConfirm(r *fakeRelease, req hubapi.ConfirmRequest) error {
	if r.kind == hubapi.KindRun {
		if req.PipelineRun == nil || !pipelineRunRe.MatchString(*req.PipelineRun) {
			return badRequest("pipelineRun", "A run is confirmed with its PipelineRun, 1 to 253 of a-z, 0-9, dot and hyphen.")
		}
		if req.DeploymentID != nil {
			return badRequest("deploymentId", "A run's deployment is joined by the deploy poller, not the confirm.")
		}
		return nil
	}
	if req.PipelineRun != nil {
		return badRequest("pipelineRun", "A rollback creates no PipelineRun.")
	}
	if req.DeploymentID != nil && badDeploymentID(*req.DeploymentID) {
		return badRequest("deploymentId", "A deployment id is 1 to 128 characters.")
	}
	return nil
}

// holds reports whether r is joined to deploymentID.
func holds(r *fakeRelease, deploymentID string) bool {
	return slices.ContainsFunc(r.deployments, func(d hubapi.DeploymentJoin) bool { return d.DeploymentID == deploymentID })
}

// join joins r to deploymentID now, with its event.
func (w *fakeWorld) join(me string, r *fakeRelease, deploymentID string) hubapi.DeploymentJoin {
	j := hubapi.DeploymentJoin{DeploymentID: deploymentID, JoinedAt: w.stamp()}
	r.deployments = append(r.deployments, j)
	w.emitIn(me, "release.deployment_joined", w.projects[r.projectID], r.id, "deploymentId", deploymentID, "environment", deref(r.environment))
	return j
}

// Confirm is route 38, by task 24's state table. A rollback's deploymentId
// also joins, but never its own source deployment (HA7-2: 409 conflict).
func (f *Fake) Confirm(_ context.Context, releaseID string, req hubapi.ConfirmRequest) (hubapi.Release, error) {
	w := f.w
	w.mu.Lock()
	defer w.mu.Unlock()
	me, err := f.enter("Confirm", releaseID, req)
	if err != nil {
		return hubapi.Release{}, err
	}
	r, err := w.reserverOf(releaseID, me)
	if err != nil {
		return hubapi.Release{}, err
	}
	if err := checkConfirm(r, req); err != nil {
		return hubapi.Release{}, err
	}
	switch r.state {
	case hubapi.StateConfirmed:
		if r.kind == hubapi.KindRun && *r.pipelineRun == *req.PipelineRun ||
			r.kind == hubapi.KindRollback && (req.DeploymentID == nil || holds(r, *req.DeploymentID)) {
			return w.renderRelease(r), nil // 200, unchanged, no event
		}
		return hubapi.Release{}, releaseState(r)
	case hubapi.StateAbandoned:
		return hubapi.Release{}, releaseState(r)
	case hubapi.StateExpired: // enter's sweep has expired every overdue reservation
		return hubapi.Release{}, problem(409, hubapi.CodeReleaseExpired, "The reservation expired before it was confirmed.", "releaseId", r.id)
	}
	if d := req.DeploymentID; d != nil {
		if *d == deref(r.sourceDeploymentID) {
			return hubapi.Release{}, problem(409, hubapi.CodeConflict, "That deployment is the rollback's source, not its result.", "field", "deploymentId")
		}
		if w.joined(r.panaceaProject, *d) != nil {
			return hubapi.Release{}, problem(409, hubapi.CodeConflict, "Another release already holds that deployment.", "field", "deploymentId")
		}
	}
	if run := req.PipelineRun; run != nil {
		for _, o := range w.releases {
			if o.panaceaProject == r.panaceaProject && o.pipelineRun != nil && *o.pipelineRun == *run {
				return hubapi.Release{}, problem(409, hubapi.CodeConflict, "Another release already confirmed that run.", "field", "pipelineRun")
			}
		}
	}
	now := w.stamp()
	r.state, r.confirmedAt, r.pipelineRun = hubapi.StateConfirmed, &now, clone(req.PipelineRun)
	w.emitIn(me, "release.confirmed", w.projects[r.projectID], r.id, "pipelineRun", deref(r.pipelineRun))
	if req.DeploymentID != nil {
		w.join(me, r, *req.DeploymentID)
	}
	return w.renderRelease(r), nil
}

// Abandon is route 39: from reserved (and not overdue) only.
func (f *Fake) Abandon(_ context.Context, releaseID string, req hubapi.AbandonRequest) (hubapi.Release, error) {
	w := f.w
	w.mu.Lock()
	defer w.mu.Unlock()
	me, err := f.enter("Abandon", releaseID, req)
	if err != nil {
		return hubapi.Release{}, err
	}
	r, err := w.reserverOf(releaseID, me)
	if err != nil {
		return hubapi.Release{}, err
	}
	if req.Reason != hubapi.AbandonDeployFailed && req.Reason != hubapi.AbandonCancelled {
		return hubapi.Release{}, badRequest("reason", "A release is abandoned because the deploy failed or was cancelled.")
	}
	if utf8.RuneCountInString(req.Message) > 500 {
		return hubapi.Release{}, badRequest("message", "A message is at most 500 characters.")
	}
	if r.state != hubapi.StateReserved {
		return hubapi.Release{}, releaseState(r)
	}
	now, reason := w.stamp(), req.Reason
	r.state, r.abandonedAt, r.abandonReason = hubapi.StateAbandoned, &now, &reason
	if req.Message != "" {
		r.abandonMessage = clone(&req.Message)
	}
	w.emitIn(me, "release.abandoned", w.projects[r.projectID], r.id, "reason", string(reason))
	return w.renderRelease(r), nil
}

// JoinDeployment is route 40: member; confirmed only; one release per
// (panaceaProject, deploymentId) in the org; the join's environment sets a
// null one, is a no-op on an equal one, and is 409 on a different one,
// nothing joined (HA7-1).
func (f *Fake) JoinDeployment(_ context.Context, releaseID string, req hubapi.DeploymentJoinRequest) (hubapi.DeploymentJoin, error) {
	w := f.w
	w.mu.Lock()
	defer w.mu.Unlock()
	me, err := f.enter("JoinDeployment", releaseID, req)
	if err != nil {
		return hubapi.DeploymentJoin{}, err
	}
	r, role, err := w.releaseOf(releaseID, me)
	if err != nil {
		return hubapi.DeploymentJoin{}, err
	}
	if err := need(role, hubapi.RoleMember); err != nil {
		return hubapi.DeploymentJoin{}, err
	}
	if badDeploymentID(req.DeploymentID) {
		return hubapi.DeploymentJoin{}, badRequest("deploymentId", "A deployment id is 1 to 128 characters.")
	}
	if req.Environment != nil && !panaceaNameRe.MatchString(*req.Environment) {
		return hubapi.DeploymentJoin{}, badRequest("environment", "An environment is 1 to 63 of a-z, 0-9 and hyphen.")
	}
	if r.state != hubapi.StateConfirmed {
		return hubapi.DeploymentJoin{}, releaseState(r)
	}
	if holder := w.joined(r.panaceaProject, req.DeploymentID); holder != nil && holder != r {
		return hubapi.DeploymentJoin{}, problem(409, hubapi.CodeConflict, "Another release already holds that deployment.", "field", "deploymentId")
	}
	if r.environment != nil && req.Environment != nil && *req.Environment != *r.environment {
		return hubapi.DeploymentJoin{}, problem(409, hubapi.CodeConflict, "The release was deployed to another environment.",
			"field", "environment", "environment", *r.environment)
	}
	for _, j := range r.deployments {
		if j.DeploymentID == req.DeploymentID {
			return j, nil // 200: this release already holds it
		}
	}
	if r.environment == nil && req.Environment != nil {
		r.environment = clone(req.Environment)
	}
	return w.join(me, r, req.DeploymentID), nil
}

// ── reads (task 23 routes 36–37) ────────────────────────────────────────────

func (f *Fake) Release(_ context.Context, releaseID string) (hubapi.Release, error) {
	w := f.w
	w.mu.Lock()
	defer w.mu.Unlock()
	me, err := f.enter("Release", releaseID)
	if err != nil {
		return hubapi.Release{}, err
	}
	r, _, err := w.releaseOf(releaseID, me)
	if err != nil {
		return hubapi.Release{}, err
	}
	return w.renderRelease(r), nil
}

// Releases is route 36: newest reservedAt first; environment= never matches
// a null environment; kind= is run or rollback (HA7-5).
func (f *Fake) Releases(_ context.Context, projectID string, q hubapi.ReleaseQuery) (hubapi.Page[hubapi.Release], error) {
	w := f.w
	w.mu.Lock()
	defer w.mu.Unlock()
	me, err := f.enter("Releases", projectID, q)
	if err != nil {
		return hubapi.Page[hubapi.Release]{}, err
	}
	p, _, err := w.projectOf(projectID, me)
	if err != nil {
		return hubapi.Page[hubapi.Release]{}, err
	}
	switch q.Kind {
	case "", hubapi.KindRun, hubapi.KindRollback:
	default:
		return hubapi.Page[hubapi.Release]{}, badRequest("kind", "A kind is run or rollback.")
	}
	switch q.State {
	case "", hubapi.StateReserved, hubapi.StateConfirmed, hubapi.StateAbandoned, hubapi.StateExpired:
	default:
		return hubapi.Page[hubapi.Release]{}, badRequest("state", "A state is reserved, confirmed, abandoned or expired.")
	}
	limit, err := pageLimit(q.Page.Limit)
	if err != nil {
		return hubapi.Page[hubapi.Release]{}, err
	}
	rows, err := newestFirst(w.releases, func(r *fakeRelease) string { return r.projectID }, p.id, q.Page.After, func(r *fakeRelease) bool {
		return (q.Environment == "" || r.environment != nil && *r.environment == q.Environment) &&
			(q.PanaceaProject == "" || r.panaceaProject == q.PanaceaProject) &&
			(q.DeploymentID == "" || holds(r, q.DeploymentID)) &&
			(q.PipelineRun == "" || r.pipelineRun != nil && *r.pipelineRun == q.PipelineRun) &&
			(q.Kind == "" || r.kind == q.Kind) && (q.State == "" || r.state == q.State)
	})
	if err != nil {
		return hubapi.Page[hubapi.Release]{}, err
	}
	return pageOf(rows, limit, func(r *fakeRelease) string { return r.id }, w.renderRelease), nil
}
