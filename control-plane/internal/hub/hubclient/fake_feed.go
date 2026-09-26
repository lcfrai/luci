package hubclient

import (
	"context"
	"maps"
	"strconv"
	"strings"
	"unicode/utf8"

	"github.com/kpmg-au-emu/panacea-app/control-plane/internal/hub/hubapi"
	"github.com/kpmg-au-emu/panacea-app/control-plane/internal/ids"
)

// The fake's audit log and the change feed read from it (contract §7.11, §8).
// Every write appends its events here, in the write's own critical section,
// so the log is gapless and in commit order, as the Hub's is (§10.2).

type fakeEvent struct {
	seq       int64
	at        string
	kind      string
	teamID    string // "" is null
	projectID string // "" is null
	subject   string
	actor     string // a user id; "" is null (the janitor)
	data      map[string]any
	feed      bool // §8's last column
}

// fakeEvidence is one judgement (§7.10): append-only, the repo key verbatim.
type fakeEvidence struct {
	rec hubapi.EvidenceRecord // recordedBy rendered at read, from recordedBy
	by  string                // a user id
	seq int64
}

func (e *fakeEvidence) order() int64 { return e.seq }

// auditOnly is §8's kinds that never reach the feed. The fake writes
// auth.user_provisioned and blob.uploaded; it has no audit route, no janitor
// collecting blobs and no 403 audit, so the other three never occur in it.
var auditOnly = map[string]bool{
	"auth.user_provisioned": true, "auth.forbidden": true,
	"blob.uploaded": true, "blob.deleted": true, "audit.read": true,
}

// emit appends one event to the org's log. kv is data, as "key", value, …;
// every value a string, an int or a bool (§8).
func (w *fakeWorld) emit(actor, kind, teamID, projectID, subject string, kv ...any) {
	data := make(map[string]any, len(kv)/2)
	for i := 0; i+1 < len(kv); i += 2 {
		data[kv[i].(string)] = kv[i+1]
	}
	w.events = append(w.events, &fakeEvent{seq: int64(len(w.events)) + 1, at: w.stamp(), kind: kind,
		teamID: teamID, projectID: projectID, subject: subject, actor: actor, data: data, feed: !auditOnly[kind]})
}

// emitIn is emit for a record of project p: its team and itself.
func (w *fakeWorld) emitIn(actor, kind string, p *fakeProject, subject string, kv ...any) {
	w.emit(actor, kind, p.teamID, p.id, subject, kv...)
}

// visible is whether the caller sees ev in the feed now: a feed kind whose
// team the caller is in, or whose project is a project of such a team.
func (w *fakeWorld) visible(ev *fakeEvent, me string) bool {
	if !ev.feed {
		return false
	}
	if t := w.teams[ev.teamID]; t != nil && t.members[me] != nil {
		return true
	}
	p := w.projects[ev.projectID]
	return p != nil && w.canRead(p, me)
}

// ── evidence (§7.10, task 25) ───────────────────────────────────────────────

// RecordEvidence is route 41, in task 25's order: the fields (400); the repo a
// remote of a project the caller can read (422 repo_not_linked, one answer
// whether none has it or one the caller cannot see); member (403); the brief
// key a revision of that project's (422 key_unresolvable). recordedBy is the
// caller; the repo key is stored verbatim.
func (f *Fake) RecordEvidence(_ context.Context, req hubapi.EvidenceRequest) (hubapi.EvidenceRecord, error) {
	w := f.w
	w.mu.Lock()
	defer w.mu.Unlock()
	me, err := f.enter("RecordEvidence", req)
	if err != nil {
		return hubapi.EvidenceRecord{}, err
	}
	because := utf8.RuneCountInString(strings.TrimSpace(req.Because))
	switch _, host := hubapi.HostOfRepoKey(req.Repo); {
	case !host:
		return hubapi.EvidenceRecord{}, badRequest("repo", "That is not a repository key of either host.")
	case utf8.RuneCountInString(req.CriterionID) < 1 || utf8.RuneCountInString(req.CriterionID) > 128:
		return hubapi.EvidenceRecord{}, badRequest("criterionId", "A criterion id is 1 to 128 characters.")
	case !commitRe.MatchString(req.Commit):
		return hubapi.EvidenceRecord{}, badRequest("commit", "A commit is 40 lowercase hex.")
	case req.Verdict != hubapi.VerdictSettled && req.Verdict != hubapi.VerdictContested:
		return hubapi.EvidenceRecord{}, badRequest("verdict", "A verdict is settled or contested.")
	case because < 1 || because > 2000:
		return hubapi.EvidenceRecord{}, badRequest("because", "A reason is 1 to 2,000 characters.")
	}
	p := w.linked(req.Repo)
	if p == nil || !w.canRead(p, me) {
		return hubapi.EvidenceRecord{}, problem(422, hubapi.CodeRepoNotLinked, "No project you can see shares that repository.", "repo", req.Repo)
	}
	if err := need(w.teams[p.teamID].members[me].role, hubapi.RoleMember); err != nil {
		return hubapi.EvidenceRecord{}, err
	}
	if _, rev := w.revisionIn(p.id, req.BriefKey); rev == nil {
		return hubapi.EvidenceRecord{}, problem(422, hubapi.CodeKeyUnresolvable, "That brief revision is not one of this project's.", "key", req.BriefKey)
	}
	ev := &fakeEvidence{rec: hubapi.EvidenceRecord{ID: ids.Wide(hubapi.PrefixEvidence), Repo: req.Repo, ProjectID: p.id,
		BriefKey: req.BriefKey, CriterionID: req.CriterionID, CriterionDigest: req.CriterionDigest, Commit: req.Commit,
		Verdict: req.Verdict, Because: req.Because, RecordedAt: w.stamp()}, by: me, seq: w.next()}
	w.evidence[ev.rec.ID] = ev
	w.emitIn(me, "evidence.recorded", p, ev.rec.ID, "briefKey", req.BriefKey, "criterionId", req.CriterionID,
		"verdict", string(req.Verdict), "commit", req.Commit)
	return w.renderEvidence(ev), nil
}

func (w *fakeWorld) renderEvidence(ev *fakeEvidence) hubapi.EvidenceRecord {
	out := ev.rec
	out.RecordedBy = w.users[ev.by]
	return out
}

// EvidenceByDigest is route 42: 1–100 digests; the repo's project must be
// readable, else no items (never a 404 confirming the link); oldest first.
func (f *Fake) EvidenceByDigest(_ context.Context, repo string, digests []string) ([]hubapi.EvidenceRecord, error) {
	w := f.w
	w.mu.Lock()
	defer w.mu.Unlock()
	me, err := f.enter("EvidenceByDigest", repo, digests)
	if err != nil {
		return nil, err
	}
	if _, ok := hubapi.HostOfRepoKey(repo); !ok {
		return nil, badRequest("repo", "That is not a repository key of either host.")
	}
	if len(digests) < 1 || len(digests) > 100 {
		return nil, badRequest("digest", "Ask for 1 to 100 digests.")
	}
	want := map[hubapi.Digest]bool{}
	for _, s := range digests {
		d, err := hubapi.ParseDigest(s)
		if err != nil {
			return nil, badRequest("digest", "A digest is sha256: and 64 lowercase hex characters.")
		}
		want[d] = true
	}
	out := []hubapi.EvidenceRecord{}
	p := w.linked(repo)
	if p == nil || !w.canRead(p, me) {
		return out, nil
	}
	for _, ev := range inOrder(w.evidence) {
		if ev.rec.Repo == repo && ev.rec.ProjectID == p.id && want[ev.rec.CriterionDigest] {
			out = append(out, w.renderEvidence(ev))
		}
	}
	return out, nil
}

// ── the feed (§7.11, task 26) ───────────────────────────────────────────────

func (w *fakeWorld) renderEvent(ev *fakeEvent) hubapi.Event {
	out := hubapi.Event{Seq: ev.seq, At: ev.at, Kind: ev.kind, Subject: ev.subject, Actor: w.userRef(ev.actor), Data: maps.Clone(ev.data)}
	if ev.teamID != "" {
		id := ev.teamID
		out.TeamID = &id
	}
	if ev.projectID != "" {
		id := ev.projectID
		out.ProjectID = &id
	}
	return out
}

// Changes is route 43. after is "latest" or a non-negative decimal seq (400
// field after); limit 1–500, 0 the default 200 (400 field limit). "latest" is
// no events and the head. Otherwise the events after the cursor the caller
// can see now, at most limit; more when a limit+1-th exists; next is the last
// event's seq when more, else the head — so a caller who sees none of a busy
// org's events still moves its cursor on (task 26).
func (f *Fake) Changes(_ context.Context, after string, limit int) (hubapi.ChangesPage, error) {
	w := f.w
	w.mu.Lock()
	defer w.mu.Unlock()
	me, err := f.enter("Changes", after, limit)
	if err != nil {
		return hubapi.ChangesPage{}, err
	}
	var from int64
	if after != "latest" {
		n, err := strconv.ParseInt(after, 10, 64)
		if err != nil || n < 0 || strconv.FormatInt(n, 10) != after {
			return hubapi.ChangesPage{}, badRequest("after", "A cursor is latest or a sequence number the feed gave.")
		}
		from = n
	}
	switch {
	case limit == 0:
		limit = 200
	case limit < 0 || limit > 500:
		return hubapi.ChangesPage{}, badRequest("limit", "A limit is 1 to 500.")
	}
	head := int64(len(w.events))
	out := hubapi.ChangesPage{Events: []hubapi.Event{}, Next: strconv.FormatInt(head, 10)}
	if after == "latest" {
		return out, nil
	}
	for _, ev := range w.events[min(from, head):] {
		if !w.visible(ev, me) {
			continue
		}
		if len(out.Events) == limit {
			out.More, out.Next = true, strconv.FormatInt(out.Events[limit-1].Seq, 10)
			break
		}
		out.Events = append(out.Events, w.renderEvent(ev))
	}
	return out, nil
}
