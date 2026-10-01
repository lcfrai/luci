package conformance

import (
	"strconv"
	"testing"
	"time"

	"github.com/kpmg-au-emu/panacea-app/control-plane/internal/hub/hubapi"
	"github.com/kpmg-au-emu/panacea-app/control-plane/internal/hub/hubclient"
)

// The feed group: server task 26 (contract §7.11, §8).
var feedRows = []row{
	{name: "latest_then_new_events", fn: latestThenNewEvents},
	{name: "outsider_sees_nothing", fn: outsiderSeesNothing},
	{name: "more_when_limit_cuts", fn: moreWhenLimitCuts},
	{name: "audit_only_kinds_never_appear", fn: auditOnlyKindsNeverAppear},
	{name: "every_write_is_its_events", fn: everyWriteIsItsEvents},
}

// head is h's cursor now: Changes("latest")'s next.
func head(t *testing.T, h hubclient.Hub) string {
	t.Helper()
	page := must(h.Changes(bg, "latest", 0))(t)
	if len(page.Events) != 0 || page.More {
		t.Errorf("Changes(latest) = %d events, more %v; want none", len(page.Events), page.More)
	}
	if n, err := strconv.ParseInt(page.Next, 10, 64); err != nil || n < 0 || strconv.FormatInt(n, 10) != page.Next {
		t.Fatalf("Changes(latest).next = %q, want a decimal seq", page.Next)
	}
	return page.Next
}

// since is every event h can see after cursor, page by page, in seq order.
func since(t *testing.T, h hubclient.Hub, after string) []hubapi.Event {
	t.Helper()
	var out []hubapi.Event
	for range 1000 {
		page := must(h.Changes(bg, after, 500))(t)
		out = append(out, page.Events...)
		after = page.Next
		if !page.More {
			return out
		}
	}
	t.Fatalf("the feed after %s never ended", after)
	return nil
}

// count is the events of kind about subject ("" = any subject).
func count(evs []hubapi.Event, kind, subject string) int {
	n := 0
	for _, ev := range evs {
		if ev.Kind == kind && (subject == "" || ev.Subject == subject) {
			n++
		}
	}
	return n
}

func str(p *string) string {
	if p == nil {
		return "<null>"
	}
	return *p
}

// latest is no events and the head; after a revision write, the next page
// holds brief.revised with §8's data and the writer as actor.
func latestThenNewEvents(t *testing.T, e Env) {
	p := newProject(t, e)
	b := newBrief(t, e.Member, p.ID, featRef)
	cur := head(t, e.Viewer)
	r := write(t, e.Member, p.ID, b.ID, 0, hubapi.StatusComplete)
	evs := since(t, e.Viewer, cur)
	if len(evs) != 1 || evs[0].Kind != "brief.revised" {
		t.Fatalf("after one revision the viewer's feed holds %+v, want one brief.revised", evs)
	}
	ev := evs[0]
	me := meOf(t, e.Member).User
	if ev.Subject != b.ID || str(ev.TeamID) != e.TeamID || str(ev.ProjectID) != p.ID || ev.Actor == nil || ev.Actor.ID != me.ID || ev.At == "" {
		t.Errorf("brief.revised = %+v, want subject %s, team T, project %s, actor the writer", ev, b.ID, p.ID)
	}
	c, _ := strconv.ParseInt(cur, 10, 64)
	if ev.Seq <= c {
		t.Errorf("the event's seq %d is not after the cursor %s", ev.Seq, cur)
	}
	for k, want := range map[string]string{"key": r.Key, "revision": "1", "digest": r.Digest.String(), "status": hubapi.StatusComplete} {
		if got := sprint(ev.Data[k]); got != want {
			t.Errorf("data.%s = %q, want %q", k, got, want)
		}
	}
}

// a stranger's feed after T's writes holds none of them, and its cursor still
// moves past them (task 26: next is the head, not the last visible). The org
// admin in no team is a stranger here too (task 05 review m2; contract §5.3
// gives it the outsider's answer on every route but audit and erasure).
func outsiderSeesNothing(t *testing.T, e Env) {
	ss := strangers(t, e)
	curs := make([]string, len(ss))
	for i, s := range ss {
		curs[i] = head(t, s.hub)
	}
	p := newProject(t, e)
	b := newBrief(t, e.Member, p.ID, featRef)
	write(t, e.Member, p.ID, b.ID, 0, hubapi.StatusOpen)
	revoke(t, e, invite(t, e, freshEmail(), hubapi.RoleViewer).ID)
	for i, s := range ss {
		for _, ev := range since(t, s.hub, curs[i]) {
			if str(ev.TeamID) == e.TeamID || str(ev.ProjectID) == p.ID {
				t.Errorf("the %s's feed holds T's %s about %s", s.name, ev.Kind, ev.Subject)
			}
		}
		mine := since(t, e.Admin, curs[i])
		if len(mine) == 0 {
			t.Fatalf("T's admin sees none of T's writes")
		}
		page := must(s.hub.Changes(bg, curs[i], 500))(t)
		next, _ := strconv.ParseInt(page.Next, 10, 64)
		if last := mine[len(mine)-1].Seq; !page.More && next < last {
			t.Errorf("the %s's next is %d, behind T's last event %d: it would rescan them for ever", s.name, next, last)
		}
	}
	// Fix round 1 PS6-1: being in some team is not being in T. The outsider
	// makes a team of its own, and still sees none of T's writes.
	//
	// Task 05 review-1 I1: the writes below are one project event AND one pair
	// of team events, because "team_id is a team the caller is in" is a
	// separate branch of the rule from the project one. Without an invite here,
	// a Hub whose team branch checks only that the caller is in SOME team
	// passes, and every team's invitee emails leak on route 43.
	newTeam(t, e.Outsider)
	cur := head(t, e.Outsider)
	write(t, e.Member, p.ID, b.ID, 1, hubapi.StatusOpen)
	revoke(t, e, invite(t, e, freshEmail(), hubapi.RoleViewer).ID)
	for _, ev := range since(t, e.Outsider, cur) {
		if str(ev.TeamID) == e.TeamID || str(ev.ProjectID) == p.ID {
			t.Errorf("a member of another team sees T's %s about %s", ev.Kind, ev.Subject)
		}
	}
}

// limit cuts a page and says more; following next collects each event once,
// in seq order; bad parameters are 400 naming them.
func moreWhenLimitCuts(t *testing.T, e Env) {
	p := newProject(t, e)
	b := newBrief(t, e.Member, p.ID, featRef)
	cur := head(t, e.Member)
	for base := range 5 {
		write(t, e.Member, p.ID, b.ID, base, hubapi.StatusOpen)
	}
	var seqs []int64
	for i, want := range []struct {
		n    int
		more bool
	}{{2, true}, {2, true}, {1, false}} {
		page := must(e.Member.Changes(bg, cur, 2))(t)
		if len(page.Events) != want.n || page.More != want.more {
			t.Fatalf("page %d = %d events, more %v; want %d, %v", i, len(page.Events), page.More, want.n, want.more)
		}
		if page.More && page.Next != strconv.FormatInt(page.Events[len(page.Events)-1].Seq, 10) {
			t.Errorf("page %d: next %s, want the last event's seq", i, page.Next)
		}
		for _, ev := range page.Events {
			seqs = append(seqs, ev.Seq)
		}
		cur = page.Next
	}
	for i := 1; i < len(seqs); i++ {
		if seqs[i] <= seqs[i-1] {
			t.Errorf("seqs %v are not strictly increasing", seqs)
		}
	}
	// A cursor past the head selects nothing (task-26.md:26, after < seq <= H)
	// and next is the head: no events, no more, next below the cursor.
	h, _ := strconv.ParseInt(head(t, e.Member), 10, 64)
	past := strconv.FormatInt(h+1000, 10)
	page := must(e.Member.Changes(bg, past, 10))(t)
	if n, err := strconv.ParseInt(page.Next, 10, 64); len(page.Events) != 0 || page.More || err != nil || n < h || n >= h+1000 {
		t.Errorf("Changes(%s) past the head %d = %d events, more %v, next %q; want none, false, the head", past, h, len(page.Events), page.More, page.Next)
	}
	for _, after := range []string{"-1", "abc", ""} {
		wantErr(t, second(e.Member.Changes(bg, after, 10)), 400, hubapi.CodeBadRequest, "field", "after")
	}
	for _, limit := range []int{-1, 501} {
		wantErr(t, second(e.Member.Changes(bg, "0", limit)), 400, hubapi.CodeBadRequest, "field", "limit")
	}
}

// a blob upload and a refused write leave no audit-only kind in any feed.
func auditOnlyKindsNeverAppear(t *testing.T, e Env) {
	cur := head(t, e.Admin)
	p := newProject(t, e)
	upload(t, e.Member, p.ID, htmlType, fresh("audited"))
	wantErr(t, second(e.Viewer.CreateBrief(bg, p.ID, hubapi.CreateBriefRequest{Scope: hubapi.ScopeProject})), 403, hubapi.CodeForbidden, "required", "member")
	for _, w := range []who{{"admin", e.Admin}, {"member", e.Member}, {"viewer", e.Viewer}, {"outsider", e.Outsider}} {
		for _, ev := range since(t, w.hub, cur) {
			switch ev.Kind {
			case "auth.user_provisioned", "auth.forbidden", "blob.uploaded", "blob.deleted", "audit.read":
				t.Errorf("%s's feed holds the audit-only %s", w.name, ev.Kind)
			}
		}
	}
}

// wanted is one expected event: kind, subject, team and project ("" = null),
// and data, every key of it.
type wanted struct {
	kind, subject, team, project string
	data                         map[string]string
}

// everyWriteIsItsEvents: every write of both parts appends §8's events, one
// per changed record, in order, with §8's subject and data (and no more keys).
func everyWriteIsItsEvents(t *testing.T, e Env) {
	admin, out := meOf(t, e.Admin).User, meOf(t, e.Outsider).User
	T, cur := e.TeamID, head(t, e.Admin)
	var exp []wanted
	add := func(kind, subject, team, project string, kv ...string) {
		w := wanted{kind: kind, subject: subject, team: team, project: project, data: map[string]string{}}
		for i := 0; i+1 < len(kv); i += 2 {
			w.data[kv[i]] = kv[i+1]
		}
		exp = append(exp, w)
	}
	u := newTeam(t, e.Admin)
	add("team.created", u.ID, u.ID, "", "name", u.Name)
	add("team.member_set", admin.ID, u.ID, "", "role", "admin")
	inv := invite(t, e, out.Email, hubapi.RoleMember)
	add("team.invite_created", inv.ID, T, "", "email", out.Email, "role", "member")
	meOf(t, e.Outsider)
	add("team.invite_bound", inv.ID, T, "", "userId", out.ID, "role", "member")
	add("team.member_set", out.ID, T, "", "role", "member")
	must(e.Admin.SetMember(bg, T, out.ID, hubapi.RoleViewer))(t)
	add("team.member_set", out.ID, T, "", "role", "viewer")
	ok(t, e.Admin.RemoveMember(bg, T, out.ID))
	add("team.member_removed", out.ID, T, "")
	inv2 := invite(t, e, freshEmail(), hubapi.RoleViewer)
	add("team.invite_created", inv2.ID, T, "", "email", inv2.Email, "role", "viewer")
	revoke(t, e, inv2.ID)
	add("team.invite_revoked", inv2.ID, T, "")
	p := newProject(t, e)
	add("project.created", p.ID, T, p.ID, "name", p.Name)
	add("project.remote_added", p.ID, T, p.ID, "repoKey", p.Remotes[0].RepoKey, "host", "github")
	repo2 := newRepo()
	must(e.Admin.AddRemote(bg, p.ID, hubapi.RemoteRequest{RepoKey: repo2, Host: hubapi.HostGitHub}))(t)
	add("project.remote_added", p.ID, T, p.ID, "repoKey", repo2, "host", "github")
	b := newBrief(t, e.Member, p.ID, featRef)
	add("brief.created", b.ID, T, p.ID, "scope", "branch", "branchRef", featRef)
	r1 := write(t, e.Member, p.ID, b.ID, 0, hubapi.StatusComplete)
	add("brief.revised", b.ID, T, p.ID, "key", r1.Key, "revision", "1", "digest", r1.Digest.String(), "status", "complete")
	const renamed = "refs/heads/renamed"
	must(e.Member.Rebind(bg, b.ID, renamed))(t)
	add("brief.rebound", b.ID, T, p.ID, "branchRef", renamed, "previousBranchRef", featRef)
	ap := must(e.Member.Approve(bg, p.ID, approvableOn(t, e, p, r1).req))(t) // its uploads are audit only
	add("design.approved", ap.ID, T, p.ID, "designKey", ap.DesignKey, "briefKey", r1.Key)
	lr := linkOf(ap)
	l := must(e.Member.LinkSession(bg, p.ID, lr))(t)
	add("session_link.created", l.ID, T, p.ID, "designKey", ap.DesignKey, "briefKey", r1.Key, "sessionId", lr.SessionID)
	everyReleaseWrite(t, e, p, renamed, r1, add)
	evr := j0(p, r1).evidenceOn(newCriterion())
	ev := must(e.Member.RecordEvidence(bg, evr))(t)
	add("evidence.recorded", ev.ID, T, p.ID, "briefKey", r1.Key, "criterionId", evr.CriterionID, "verdict", "settled", "commit", evr.Commit)
	must(e.Member.Archive(bg, b.ID))(t)
	add("brief.archived", b.ID, T, p.ID)
	if e.Erase != nil {
		ok(t, e.Erase(b.ID, "Conformance: the erased event carries no reason"))
		add("brief.erased", b.ID, T, p.ID, "erasedRevisions", "1")
	} else {
		t.Log("conformance: this Env has no Erase; brief.erased is not run")
	}
	matchFeed(t, since(t, e.Admin, cur), exp, T, u.ID)
}

// j0 is p's first repo judged against rev.
func j0(p hubapi.Project, rev hubapi.BriefRevision) judged {
	return judged{p: p, repo: p.Remotes[0].RepoKey, rev: rev}
}

// everyReleaseWrite is everyWriteIsItsEvents' releases: reserve, confirm, join,
// a null-environment reserve and its abandon, and (⏱) an expiry, whose actor is null.
func everyReleaseWrite(t *testing.T, e Env, p hubapi.Project, ref string, pinned hubapi.BriefRevision,
	add func(kind, subject, team, project string, kv ...string)) {
	t.Helper()
	T := e.TeamID
	rel := reserve(t, e, e.Member, p.ID, runOn(newPanacea(), ref))
	add("release.reserved", rel.ID, T, p.ID, "kind", "run", "environment", shipEnv, "pinStatus", "pinned", "briefKey", pinned.Key)
	c := confirmRun(t, e.Member, rel)
	add("release.confirmed", rel.ID, T, p.ID, "pipelineRun", *c.PipelineRun)
	d := newDeployment()
	must(e.Member.JoinDeployment(bg, rel.ID, hubapi.DeploymentJoinRequest{DeploymentID: d}))(t)
	add("release.deployment_joined", rel.ID, T, p.ID, "deploymentId", d, "environment", shipEnv)
	req := runOn(newPanacea(), ref)
	req.Environment = nil
	r2 := reserve(t, e, e.Member, p.ID, req)
	add("release.reserved", r2.ID, T, p.ID, "kind", "run", "environment", "", "pinStatus", "pinned", "briefKey", pinned.Key)
	must(e.Member.Abandon(bg, r2.ID, hubapi.AbandonRequest{Reason: hubapi.AbandonCancelled}))(t)
	add("release.abandoned", r2.ID, T, p.ID, "reason", "cancelled")
	if e.Advance == nil {
		t.Log("conformance: this Env has no Advance; release.expired is not run")
		return
	}
	r3 := reserve(t, e, e.Member, p.ID, runOn(newPanacea(), ref))
	add("release.reserved", r3.ID, T, p.ID, "kind", "run", "environment", shipEnv, "pinStatus", "pinned", "briefKey", pinned.Key)
	e.Advance(e.ReservationTTL + time.Second)
	if got := must(e.Member.Release(bg, r3.ID))(t); got.State != hubapi.StateExpired {
		t.Fatalf("an overdue reservation reads %s, want expired", got.State)
	}
	add("release.expired", r3.ID, T, p.ID)
}

// matchFeed compares the events about teams (the rest are other rows') with exp.
func matchFeed(t *testing.T, evs []hubapi.Event, exp []wanted, teams ...string) {
	t.Helper()
	var got []hubapi.Event
	for _, ev := range evs {
		for _, tm := range teams {
			if str(ev.TeamID) == tm {
				got = append(got, ev)
				break
			}
		}
	}
	for i := 0; i < len(got) || i < len(exp); i++ {
		switch {
		case i >= len(got):
			t.Errorf("event %d: missing, want %s about %s", i, exp[i].kind, exp[i].subject)
			continue
		case i >= len(exp):
			t.Errorf("event %d: %s about %s, want no more", i, got[i].Kind, got[i].Subject)
			continue
		}
		g, w := got[i], exp[i]
		if g.Kind != w.kind || g.Subject != w.subject || str(g.TeamID) != w.team || str(g.ProjectID) != orNull(w.project) {
			t.Errorf("event %d = %s about %s (team %s, project %s), want %s about %s (team %s, project %s)", i,
				g.Kind, g.Subject, str(g.TeamID), str(g.ProjectID), w.kind, w.subject, w.team, orNull(w.project))
			continue
		}
		if (g.Actor == nil) != (w.kind == "release.expired") {
			t.Errorf("event %d (%s): actor %v, want null only for the janitor's release.expired", i, g.Kind, g.Actor)
		}
		if len(g.Data) != len(w.data) || g.Data == nil {
			t.Errorf("event %d (%s): data %v, want exactly the keys of %v", i, g.Kind, g.Data, w.data)
		}
		for k, v := range w.data {
			if sprint(g.Data[k]) != v {
				t.Errorf("event %d (%s): data.%s = %v, want %q", i, g.Kind, k, g.Data[k], v)
			}
		}
	}
}

func orNull(s string) string {
	if s == "" {
		return "<null>"
	}
	return s
}
