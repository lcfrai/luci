package conformance

import (
	"strings"
	"testing"

	"github.com/kpmg-au-emu/panacea-app/control-plane/internal/hub/hubapi"
)

// The evidence group: server task 25 (contract §7.10, phase 3's H-2).
var evidenceRows = []row{
	{name: "repo_not_linked", fn: repoNotLinked},
	{name: "viewer_forbidden", fn: viewerForbidden},
	{name: "evidence_checks", fn: evidenceChecks},
	{name: "oldest_first", fn: oldestFirst},
	{name: "unreadable_repo_is_empty", fn: unreadableRepoIsEmpty},
}

// judged is a project of T with a complete brief revision to judge against.
type judged struct {
	p    hubapi.Project
	repo string
	rev  hubapi.BriefRevision
}

func newJudged(t *testing.T, e Env) judged {
	t.Helper()
	p := newProject(t, e)
	b := newBrief(t, e.Member, p.ID, featRef)
	return judged{p: p, repo: p.Remotes[0].RepoKey, rev: write(t, e.Member, p.ID, b.ID, 0, hubapi.StatusComplete)}
}

// evidenceOn is a settled judgement of criterion digest crit in j's repo.
func (j judged) evidenceOn(crit hubapi.Digest) hubapi.EvidenceRequest {
	return hubapi.EvidenceRequest{Repo: j.repo, BriefKey: j.rev.Key, CriterionID: "done:2", CriterionDigest: crit,
		Commit: newCommit(), Verdict: hubapi.VerdictSettled, Because: "Upload of a 40 MB PDF passes in test/upload.spec.ts"}
}

func newCriterion() hubapi.Digest { return hubapi.DigestOf([]byte("criterion " + uniq())) }

// a repo no project has, one only another team's project has, and T's repo
// to a stranger: the same 422 repo_not_linked, naming the repo alone.
func repoNotLinked(t *testing.T, e Env) {
	j := newJudged(t, e)
	u := newTeam(t, e.Outsider)
	theirs := newProjectIn(t, e.Outsider, u.ID).Remotes[0].RepoKey
	var msgs []string
	for _, c := range []struct {
		name string
		w    who
		repo string
	}{
		{"unlinked", who{"member", e.Member}, newRepo()},
		{"another team's", who{"member", e.Member}, theirs},
	} {
		req := j.evidenceOn(newCriterion())
		req.Repo = c.repo
		_, err := c.w.hub.RecordEvidence(bg, req)
		he := wantErr(t, err, 422, hubapi.CodeRepoNotLinked, "repo", c.repo)
		msgs = append(msgs, he.Problem.Message)
		if len(he.Problem.Details) != 1 {
			t.Errorf("%s: details %v, want the repo alone", c.name, he.Problem.Details)
		}
	}
	for _, s := range strangers(t, e) {
		_, err := s.hub.RecordEvidence(bg, j.evidenceOn(newCriterion()))
		msgs = append(msgs, wantErr(t, err, 422, hubapi.CodeRepoNotLinked, "repo", j.repo).Problem.Message)
	}
	for _, m := range msgs[1:] {
		if m != msgs[0] {
			t.Errorf("repo_not_linked messages differ (%q, %q): the difference tells which repos are linked", msgs[0], m)
		}
	}
}

// a viewer of the repo's project → 403 forbidden, required member.
func viewerForbidden(t *testing.T, e Env) {
	j := newJudged(t, e)
	_, err := e.Viewer.RecordEvidence(bg, j.evidenceOn(newCriterion()))
	wantErr(t, err, 403, hubapi.CodeForbidden, "required", "member")
}

// the body's fields (400 naming each), and the brief key (this project's).
func evidenceChecks(t *testing.T, e Env) {
	j := newJudged(t, e)
	other := newJudged(t, e)
	for field, mutate := range map[string]func(r *hubapi.EvidenceRequest){
		"repo":        func(r *hubapi.EvidenceRequest) { r.Repo = "not a repository key" },
		"criterionId": func(r *hubapi.EvidenceRequest) { r.CriterionID = "" },
		"commit":      func(r *hubapi.EvidenceRequest) { r.Commit = "A" + newCommit()[1:] },
		"verdict":     func(r *hubapi.EvidenceRequest) { r.Verdict = "maybe" },
		"because":     func(r *hubapi.EvidenceRequest) { r.Because = " \t\n " },
	} {
		req := j.evidenceOn(newCriterion())
		mutate(&req)
		_, err := e.Member.RecordEvidence(bg, req)
		wantErr(t, err, 400, hubapi.CodeBadRequest, "field", field)
	}
	long := j.evidenceOn(newCriterion())
	long.CriterionID, long.Because = strings.Repeat("c", 129), "ok"
	wantErr(t, second(e.Member.RecordEvidence(bg, long)), 400, hubapi.CodeBadRequest, "field", "criterionId")
	long = j.evidenceOn(newCriterion())
	long.Because = strings.Repeat("é", 2001)
	wantErr(t, second(e.Member.RecordEvidence(bg, long)), 400, hubapi.CodeBadRequest, "field", "because")
	long.Because = "  " + strings.Repeat("é", 2000) + "  "
	must(e.Member.RecordEvidence(bg, long))(t)
	foreign := j.evidenceOn(newCriterion())
	foreign.BriefKey = other.rev.Key
	wantErr(t, second(e.Member.RecordEvidence(bg, foreign)), 422, hubapi.CodeKeyUnresolvable, "key", other.rev.Key)
	for _, digests := range [][]string{{}, make([]string, 101), {"sha256:XYZ"}} {
		for i := range digests {
			if digests[i] == "" {
				digests[i] = newCriterion().String()
			}
		}
		_, err := e.Member.EvidenceByDigest(bg, j.repo, digests)
		wantErr(t, err, 400, hubapi.CodeBadRequest, "field", "digest")
	}
}

// records are the caller's, the repo verbatim; ByDigest answers oldest first,
// only the digests asked for.
func oldestFirst(t *testing.T, e Env) {
	j := newJudged(t, e)
	d1, d2, d3 := newCriterion(), newCriterion(), newCriterion()
	var made []hubapi.EvidenceRecord
	for _, c := range []struct {
		h hubapi.Digest
		w who
	}{{d1, who{"member", e.Member}}, {d2, who{"admin", e.Admin}}, {d1, who{"admin", e.Admin}}, {d3, who{"member", e.Member}}} {
		r := must(c.w.hub.RecordEvidence(bg, j.evidenceOn(c.h)))(t)
		if r.RecordedBy.ID != meOf(t, c.w.hub).User.ID || r.Repo != j.repo || r.ProjectID != j.p.ID || r.BriefKey != j.rev.Key ||
			r.CriterionDigest != c.h || !hubapi.ValidID(hubapi.PrefixEvidence, r.ID) || r.RecordedAt == "" {
			t.Errorf("RecordEvidence by the %s = %+v, want theirs, verbatim", c.w.name, r)
		}
		made = append(made, r)
		tick(e)
	}
	ids := func(ds ...hubapi.Digest) string {
		var in, out []string
		for _, d := range ds {
			in = append(in, d.String())
		}
		for _, r := range must(e.Viewer.EvidenceByDigest(bg, j.repo, in))(t) {
			out = append(out, r.ID)
		}
		return strings.Join(out, ",")
	}
	if got, want := ids(d2, d1), made[0].ID+","+made[1].ID+","+made[2].ID; got != want {
		t.Errorf("ByDigest(d2, d1) = %s, want %s: oldest first, whatever order was asked", got, want)
	}
	if got, want := ids(d1), made[0].ID+","+made[2].ID; got != want {
		t.Errorf("ByDigest(d1) = %s, want %s", got, want)
	}
	// ByDigest is by repo key, not by project (hub-contract.md:1753: the index
	// is org, repo_key, criterion_digest): the project's second remote answers
	// none of the first remote's records, only its own.
	alt := newRepo()
	must(e.Admin.AddRemote(bg, j.p.ID, hubapi.RemoteRequest{RepoKey: alt, Host: hubapi.HostGitHub}))(t)
	if got := must(e.Viewer.EvidenceByDigest(bg, alt, []string{d1.String()}))(t); got == nil || len(got) != 0 {
		t.Errorf("ByDigest(%s, d1) = %+v, want []: those records are %s's", alt, got, j.repo)
	}
	req := j.evidenceOn(d1)
	req.Repo = alt
	r := must(e.Member.RecordEvidence(bg, req))(t)
	if got := must(e.Viewer.EvidenceByDigest(bg, alt, []string{d1.String()}))(t); len(got) != 1 || got[0].ID != r.ID || got[0].Repo != alt {
		t.Errorf("ByDigest(%s, d1) = %+v, want %s alone", alt, got, r.ID)
	}
	if got, want := ids(d1), made[0].ID+","+made[2].ID; got != want {
		t.Errorf("after a record under %s, ByDigest(%s, d1) = %s, want %s", alt, j.repo, got, want)
	}
}

// a repo the caller cannot read, or none has, lists nothing — never a 404.
func unreadableRepoIsEmpty(t *testing.T, e Env) {
	j := newJudged(t, e)
	crit := newCriterion()
	must(e.Member.RecordEvidence(bg, j.evidenceOn(crit)))(t)
	for _, s := range append(strangers(t, e), who{"member, unlinked repo", e.Member}) {
		repo := j.repo
		if s.hub == e.Member {
			repo = newRepo()
		}
		got, err := s.hub.EvidenceByDigest(bg, repo, []string{crit.String()})
		if err != nil || got == nil || len(got) != 0 {
			t.Errorf("%s: EvidenceByDigest = %v, %v; want [] and no error", s.name, got, err)
		}
	}
}
