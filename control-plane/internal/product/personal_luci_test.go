package product

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"os"
	"strings"
	"testing"

	"github.com/lcfrai/luci/control-plane/internal/management"
)

func personalAllow(ctx context.Context, _ string, control bool) error {
	if control {
		return management.ErrExecutionAuthority
	}
	return ctx.Err()
}
func personalTestPreferences(projectID string) PersonalPreferences {
	return PersonalPreferences{ComputeProjectID: projectID, Harness: "codex", Model: "gpt-5.4", ConnectionID: "connection-one", ProjectIDs: []string{projectID}, GoalIDs: []string{}}
}

func TestPersonalConversationPersistsPrivatelyAndResolvesWithoutRepeatingInference(t *testing.T) {
	options := Options{StateDir: t.TempDir(), ProjectsDir: t.TempDir()}
	s, err := New(options)
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()
	mux := http.NewServeMux()
	s.Register(mux)
	p := createJourneyProject(t, mux, "personal-conversation")
	ctx := homeActor("alice")
	view, err := s.SavePersonalPreferences(ctx, 0, personalTestPreferences(p.ID), personalAllow)
	if err != nil {
		t.Fatal(err)
	}
	req := PersonalTurnRequest{ExpectedRevision: view.Revision, ExpectedContextDigest: view.Context.Digest, IdempotencyKey: "personal-original-request-123456", Message: "Help me decide what needs attention"}
	turn, existing, err := s.ReservePersonalTurn(ctx, req, "assist-original", personalAllow)
	if err != nil || existing {
		t.Fatal(turn, existing, err)
	}
	repeated, existing, err := s.ReservePersonalTurn(ctx, req, "assist-do-not-create", personalAllow)
	if err != nil || !existing || repeated.ID != turn.ID || repeated.JobID != "assist-original" {
		t.Fatal("duplicate request acquired a second job identity", repeated, err)
	}
	if resolved, err := s.ResolvePersonalRequest(ctx, req, personalAllow); err != nil || resolved == nil || resolved.JobID != turn.JobID {
		t.Fatal("original resolution changed identity", resolved, err)
	}
	answer := &PersonalAnswer{Summary: "Review the saved project scope first.", Options: []string{}, Discussion: &PersonalDiscussion{ProjectID: p.ID, Message: "What is the next decision for this project?"}}
	scope, _ := s.PersonalScope(ctx)
	if err := s.CompletePersonalTurn(scope, turn.ID, turn.JobID, "succeeded", answer); err != nil {
		t.Fatal(err)
	}
	reopened, err := New(options)
	if err != nil {
		t.Fatal(err)
	}
	defer reopened.Close()
	loaded, err := reopened.PersonalView(ctx, 0, personalAllow)
	if err != nil || len(loaded.Turns) != 1 || loaded.Turns[0].Answer == nil || loaded.Turns[0].Answer.Summary != answer.Summary {
		t.Fatal("private conversation did not survive restart", loaded, err)
	}
	other, err := reopened.PersonalView(homeActor("bob"), 0, personalAllow)
	if err != nil || other.Revision != 0 || len(other.Turns) != 0 {
		t.Fatal("conversation crossed actor scope", other, err)
	}
	team := management.WithExecutionAuthority(context.Background(), management.ExecutionAuthority{Actor: management.User{ID: "alice", Username: "alice"}, Team: &management.TeamAccess{Team: management.Team{ID: "different-team"}}})
	other, err = reopened.PersonalView(team, 0, personalAllow)
	if err != nil || len(other.Turns) != 0 {
		t.Fatal("conversation crossed workspace/team", other, err)
	}
	req.Message = "Different message under the old request key"
	if _, _, err := s.ReservePersonalTurn(ctx, req, "assist-conflict", personalAllow); !errors.Is(err, ErrPersonalChanged) {
		t.Fatal("changed request reused original admission", err)
	}
}

func TestPersonalHistoryWithholdsRevokedSourcesAndKeepsExplicitContextSelection(t *testing.T) {
	s, mux := overviewFixture(t)
	p := createJourneyProject(t, mux, "personal-read-fence")
	ctx := homeActor("alice")
	view, err := s.SavePersonalPreferences(ctx, 0, personalTestPreferences(p.ID), personalAllow)
	if err != nil {
		t.Fatal(err)
	}
	turn, _, err := s.ReservePersonalTurn(ctx, PersonalTurnRequest{ExpectedRevision: view.Revision, ExpectedContextDigest: view.Context.Digest, IdempotencyKey: "private-sensitive-original-12345", Message: "PRIVATE GOAL TEXT"}, "assist-sensitive", personalAllow)
	if err != nil {
		t.Fatal(err)
	}
	scope, _ := s.PersonalScope(ctx)
	if err := s.CompletePersonalTurn(scope, turn.ID, turn.JobID, "succeeded", &PersonalAnswer{Summary: "PRIVATE PROJECT RESPONSE", Options: []string{}}); err != nil {
		t.Fatal(err)
	}
	denied := func(context.Context, string, bool) error { return management.ErrExecutionAuthority }
	hidden, err := s.PersonalView(ctx, 0, denied)
	data, _ := json.Marshal(hidden)
	if err != nil || len(hidden.Turns) != 1 || !hidden.Turns[0].Withheld || strings.Contains(string(data), "PRIVATE") || strings.Contains(string(data), p.ID) {
		t.Fatal("revoked source survived redaction", string(data), err)
	}
	current, err := s.PersonalView(ctx, 0, personalAllow)
	if err != nil || current.Turns[0].Withheld {
		t.Fatal("restored access lost the retained record", err)
	}
	// Context capture succeeds first, then access is revoked while the same
	// response reads history. Neither the old turn nor the captured context may
	// survive the final response boundary.
	reads := 0
	lateRevoke := func(ctx context.Context, _ string, _ bool) error {
		reads++
		if reads >= 5 {
			return management.ErrExecutionAuthority
		}
		return ctx.Err()
	}
	late, err := s.PersonalView(ctx, 0, lateRevoke)
	lateRaw, _ := json.Marshal(late)
	if err != nil || len(late.Turns) != 1 || !late.Turns[0].Withheld || late.Preferences.ComputeProjectID != "" || late.Context.Digest != "" || strings.Contains(string(lateRaw), p.ID) {
		t.Fatal("late revocation exposed earlier captured context", string(lateRaw), err)
	}
	prefs := current.Preferences
	prefs.ProjectIDs = []string{}
	current, err = s.SavePersonalPreferences(ctx, current.Revision, prefs, personalAllow)
	if err != nil || len(current.Context.Projects) != 0 || len(current.Context.Goals) != 0 {
		t.Fatal("context selection implicitly copied history/goals", current, err)
	}
	if err := ValidatePersonalAnswer(current.Context, PersonalAnswer{Summary: "Unselected destination", Options: []string{}, Discussion: &PersonalDiscussion{ProjectID: p.ID, Message: "Do this"}}); err == nil {
		t.Fatal("model selected an unincluded project")
	}
}

func TestPersonalHandoffPersistsOneReviewedMessageAndUnknownNeverReplays(t *testing.T) {
	s, mux := overviewFixture(t)
	p := createJourneyProject(t, mux, "private-handoff")
	ctx := homeActor("alice")
	view, err := s.SavePersonalPreferences(ctx, 0, personalTestPreferences(p.ID), personalAllow)
	if err != nil {
		t.Fatal(err)
	}
	turn, _, err := s.ReservePersonalTurn(ctx, PersonalTurnRequest{ExpectedRevision: view.Revision, ExpectedContextDigest: view.Context.Digest, IdempotencyKey: "personal-handoff-turn-request-123", Message: "PRIVATE HISTORY; suggest a project discussion"}, "assist-discussion", personalAllow)
	if err != nil {
		t.Fatal(err)
	}
	scope, _ := s.PersonalScope(ctx)
	if err := s.CompletePersonalTurn(scope, turn.ID, turn.JobID, "succeeded", &PersonalAnswer{Summary: "Discuss the next step", Options: []string{}}); err != nil {
		t.Fatal(err)
	}
	view, err = s.PersonalView(ctx, 0, personalAllow)
	if err != nil {
		t.Fatal(err)
	}
	req := PersonalHandoffRequest{ExpectedRevision: view.Revision, ProjectID: p.ID, Message: "ONLY REVIEWED MESSAGE", ProjectRevision: 1, ProjectContextDigest: strings.Repeat("a", 64), IdempotencyKey: "reviewed-message-request-123456"}
	reserved, existing, err := s.ReservePersonalHandoff(ctx, turn.ID, req, personalAllow)
	if err != nil || existing || reserved.Handoff.Message != req.Message {
		t.Fatal(reserved, err)
	}
	if err := s.CompletePersonalHandoff(ctx, turn.ID, reserved.Handoff.ID, "", "", personalAllow); err != nil {
		t.Fatal(err)
	}
	retained, existing, err := s.ReservePersonalHandoff(ctx, turn.ID, req, personalAllow)
	if err != nil || !existing || retained.Handoff.Status != "unknown" || retained.Handoff.ProjectRequestKey != reserved.Handoff.ProjectRequestKey {
		t.Fatal("uncertainty enabled a second dispatch", retained, err)
	}
	req.Message = "Different message"
	if _, _, err := s.ReservePersonalHandoff(ctx, turn.ID, req, personalAllow); !errors.Is(err, ErrPersonalChanged) {
		t.Fatal("reviewed text changed under the retained effect", err)
	}
}

func TestPersonalDiscussionPurposeSurvivesSerializationAndCannotApplyOrAutoAdvance(t *testing.T) {
	s, mux := journeyFixture(t)
	p := createJourneyProject(t, mux, "discussion-only")
	ctx := PersonalDiscussionContext(orchestratorActor(t, "alice"))
	turn, _ := reserveOrchestratorTest(t, s, p.ID, ctx, "ONLY REVIEWED MESSAGE", "")
	if !turn.IsDiscussionOnly() || !strings.HasPrefix(turn.ID, "turn-discussion-") {
		t.Fatal("controller did not retain restrictive purpose", turn.ID)
	}
	answer := &OrchestratorAnswer{Summary: "Discuss the saved plan", Options: []string{}, Impact: []string{}}
	if err := s.CompleteOrchestratorTurn(p.ID, turn.ID, turn.JobID, "succeeded", answer); err != nil {
		t.Fatal(err)
	}
	view, err := s.OrchestratorView(ctx, p.ID, 0)
	if err != nil {
		t.Fatal(err)
	}
	stored, err := s.OrchestratorTurn(p.ID, turn.ID)
	if err != nil || !stored.IsDiscussionOnly() {
		t.Fatal("purpose lost after durable read", err)
	}
	raw, _ := json.Marshal(stored)
	var restored OrchestratorTurn
	if json.Unmarshal(raw, &restored) != nil || !restored.IsDiscussionOnly() {
		t.Fatal("purpose lost on serialization")
	}
	restored.DiscussionOnly = false
	if !restored.IsDiscussionOnly() {
		t.Fatal("missing newer field downgraded canonical controller purpose")
	}
	if err := s.applyOrchestratorProposalGuarded(ctx, p.ID, turn.ID, view.Revision, view.Context.Digest, nil); !errors.Is(err, ErrOrchestratorInvalid) {
		t.Fatal("discussion entered the shared Apply/Advance mutation gate", err)
	}
	malicious := *answer
	malicious.Proposal = &OrchestratorProposal{Target: "specification", Summary: "Unrequested amendment"}
	if err := s.CompleteOrchestratorTurn(p.ID, turn.ID, turn.JobID, "succeeded", &malicious); !errors.Is(err, ErrOrchestratorInvalid) {
		t.Fatal("model output widened discussion purpose", err)
	}
}

func TestPersonalConversationContinuesWithoutLosingOriginalRequestsOrUncertainHandoffs(t *testing.T) {
	s, mux := overviewFixture(t)
	p := createJourneyProject(t, mux, "private-rollover")
	ctx := homeActor("alice")
	view, err := s.SavePersonalPreferences(ctx, 0, personalTestPreferences(p.ID), personalAllow)
	if err != nil {
		t.Fatal(err)
	}
	req := PersonalTurnRequest{ExpectedRevision: view.Revision, ExpectedContextDigest: view.Context.Digest, IdempotencyKey: "rollover-original-request-123456", Message: "Keep this original request"}
	turn, _, err := s.ReservePersonalTurn(ctx, req, "assist-rollover", personalAllow)
	if err != nil {
		t.Fatal(err)
	}
	current, err := s.PersonalView(ctx, 0, personalAllow)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := s.ContinuePersonalConversation(ctx, current.Revision, current.ConversationID, personalAllow); !errors.Is(err, ErrOrchestratorBusy) {
		t.Fatal("in-flight job was detached from its original part", err)
	}
	scope, _ := s.PersonalScope(ctx)
	if err := s.CompletePersonalTurn(scope, turn.ID, turn.JobID, "succeeded", &PersonalAnswer{Summary: "Retained answer", Options: []string{}}); err != nil {
		t.Fatal(err)
	}
	current, _ = s.PersonalView(ctx, 0, personalAllow)
	handoff, _, err := s.ReservePersonalHandoff(ctx, turn.ID, PersonalHandoffRequest{ExpectedRevision: current.Revision, ProjectID: p.ID, Message: "Only reviewed text", ProjectRevision: 1, ProjectContextDigest: strings.Repeat("a", 64), IdempotencyKey: "rollover-reviewed-project-message"}, personalAllow)
	if err != nil {
		t.Fatal(err)
	}
	if err := s.CompletePersonalHandoff(ctx, turn.ID, handoff.Handoff.ID, "", "", personalAllow); err != nil {
		t.Fatal(err)
	}
	// Seed additional valid terminal records through the same private writer;
	// no provider is involved in this persistence-boundary regression.
	s.personalMu.Lock()
	row, err := s.readPersonal(scope)
	if err == nil {
		for i := 1; i < personalTurnLimit; i++ {
			copy := row.Turns[0]
			copy.ID = fmt.Sprintf("personal-turn-retained-%d", i)
			copy.JobID = fmt.Sprintf("assist-retained-%d", i)
			copy.Sequence = i + 1
			copy.Handoff = nil
			copy.KeyDigest = reviewDigest(fmt.Sprintf("key-%d", i))
			copy.RequestDigest = reviewDigest(fmt.Sprintf("request-%d", i))
			row.Turns = append(row.Turns, copy)
		}
		err = s.savePersonal(&row)
	}
	s.personalMu.Unlock()
	if err != nil {
		t.Fatal(err)
	}
	current, _ = s.PersonalView(ctx, 0, personalAllow)
	next, err := s.ContinuePersonalConversation(ctx, current.Revision, current.ConversationID, personalAllow)
	if err != nil || next.ConversationID == current.ConversationID || len(next.Turns) != 0 || len(next.Archives) != 1 || next.Archives[0].Turns != 64 {
		t.Fatal("rollover lost part identity/history", next, err)
	}
	path, _ := s.personalArchivePath(scope, current.ConversationID, false)
	original, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	archived, err := s.PersonalArchiveView(ctx, -1, current.ConversationID, personalAllow)
	if err != nil || !archived.Archived || len(archived.Turns) != 64 || archived.Turns[0].Handoff.Status != "unknown" {
		t.Fatal("saved history unavailable", archived, err)
	}
	duplicate, existing, err := s.ReservePersonalTurn(ctx, req, "assist-never-create-again", personalAllow)
	if err != nil || !existing || duplicate.ID != turn.ID || duplicate.JobID != turn.JobID {
		t.Fatal("archive enabled inference replay", duplicate, err)
	}
	if err := s.CompletePersonalHandoff(ctx, turn.ID, handoff.Handoff.ID, "turn-discussion-original", "assist-project-original", personalAllow); err != nil {
		t.Fatal(err)
	}
	resolved, err := s.PersonalTurn(ctx, turn.ID, personalAllow)
	if err != nil || resolved.Handoff.Status != "forwarded" || resolved.Handoff.ProjectTurnID != "turn-discussion-original" {
		t.Fatal("archived original acknowledgement did not resolve", resolved, err)
	}
	after, _ := os.ReadFile(path)
	if string(after) != string(original) {
		t.Fatal("resolving a receipt rewrote the immutable archived conversation")
	}
	denied := func(context.Context, string, bool) error { return management.ErrExecutionAuthority }
	hidden, err := s.PersonalArchiveView(ctx, -1, current.ConversationID, denied)
	if err != nil || !hidden.Turns[0].Withheld {
		t.Fatal("archive bypassed current source read grants", err)
	}
}
