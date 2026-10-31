package product

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"reflect"
	"strings"
	"testing"
	"time"

	"github.com/lcfrai/luci/control-plane/internal/management"
	"github.com/lcfrai/luci/control-plane/internal/project"
)

type fixtureFeedbackRunner struct {
	page   GitHubIssuePage
	during func()
	reads  int
}

func (f *fixtureFeedbackRunner) Identity(context.Context) (GitHubIssueCLIIdentity, error) {
	return GitHubIssueCLIIdentity{Path: "fixture-only-gh", Bytes: 1, SHA256: strings.Repeat("a", 64)}, nil
}
func (f *fixtureFeedbackRunner) ReadIssues(_ context.Context, _ GitHubIssueCLIIdentity, repo string) (GitHubIssuePage, error) {
	f.reads++
	if repo != "owner/repo" {
		return GitHubIssuePage{}, ErrProjectFeedback
	}
	if f.during != nil {
		f.during()
	}
	return f.page, nil
}
func feedbackFixture(t *testing.T) (*Server, project.Project, context.Context, management.ExecutionActorRef, *fixtureFeedbackRunner, *string, *bool) {
	t.Helper()
	s, mux := journeyFixture(t)
	p := createJourneyProject(t, mux, "Feedback project")
	ctx := orchestratorActor(t, "feedback-owner")
	actor := management.ExecutionActorRef{UserID: "feedback-owner", ProjectID: p.ID}
	origin := "https://github.com/owner/repo.git"
	prior := s.runner
	s.runner = func(ctx context.Context, name string, args []string, dir string) (string, error) {
		if strings.Contains(strings.Join(args, " "), "config --get-all remote.origin.url") {
			return origin + "\n", nil
		}
		return prior(ctx, name, args, dir)
	}
	allowed := true
	s.SetProjectFeedbackAuthorizer(func(_ context.Context, ref management.ExecutionActorRef, _ bool) error {
		if !allowed || ref != actor {
			return management.ErrExecutionAuthority
		}
		return nil
	})
	runner := &fixtureFeedbackRunner{page: GitHubIssuePage{Fetched: 1, Issues: []GitHubIssue{{Number: 7, Title: "Preserve a failed save", Body: "Keep typed input after a failed request.", State: "open", UpdatedAt: "2026-10-31T01:00:00Z"}}}}
	s.githubIssuesRunner = runner
	return s, p, ctx, actor, runner, &origin, &allowed
}
func feedbackPreviewForTest(t *testing.T, s *Server, ctx context.Context, actor management.ExecutionActorRef) FeedbackPreview {
	t.Helper()
	view, err := s.ProjectFeedback(ctx, actor)
	if err != nil || view.Source == nil {
		t.Fatalf("source missing: %v", err)
	}
	preview, err := s.PreviewProjectFeedback(ctx, actor, *view.Source)
	if err != nil {
		t.Fatal(err)
	}
	return preview
}

func TestFeedbackPreviewRechecksCurrentSecretsAndAttachedOrigin(t *testing.T) {
	for _, kind := range []string{"revoked access", "changed origin"} {
		t.Run(kind, func(t *testing.T) {
			s, p, ctx, actor, runner, origin, allowed := feedbackFixture(t)
			view, err := s.ProjectFeedback(ctx, actor)
			if err != nil || view.Source == nil {
				t.Fatal(err)
			}
			runner.during = func() {
				if kind == "revoked access" {
					*allowed = false
				} else {
					*origin = "https://github.com/owner/other"
				}
			}
			if _, err = s.PreviewProjectFeedback(ctx, actor, *view.Source); err == nil {
				t.Fatal("changed authority/source accepted")
			}
			_, ledger, err := s.readFeedbackPreviewsLocked(p, actor)
			if err != nil || len(ledger.Previews) != 0 {
				t.Fatal("unconfirmed preview retained", err)
			}
			if runner.reads != 1 {
				t.Fatal("unexpected retry")
			}
		})
	}
}

func TestFeedbackImportIsReviewedVersionedAndIdempotent(t *testing.T) {
	s, p, ctx, actor, runner, _, _ := feedbackFixture(t)
	preview := feedbackPreviewForTest(t, s, ctx, actor)
	req := FeedbackImportRequest{PreviewID: preview.ID, IssueNumbers: []int{7}, ExpectedRevision: 0, IdempotencyKey: "feedback-import-version-first-123456"}
	if _, err := s.ImportProjectFeedback(ctx, actor, req); err == nil {
		t.Fatal("missing sharing acknowledgement accepted")
	}
	req.AcknowledgeSharing = true
	saved, err := s.ImportProjectFeedback(ctx, actor, req)
	if err != nil || saved.View.Revision != 1 || len(saved.View.Observations) != 1 || len(saved.View.SelectedIDs) != 0 {
		t.Fatal("import selected model context or lost evidence", err)
	}
	original := saved.View.Observations[0]
	replay, err := s.ImportProjectFeedback(ctx, actor, req)
	if err != nil || !replay.Replayed || replay.View.Revision != 1 || !reflect.DeepEqual(replay.ImportedIDs, saved.ImportedIDs) {
		t.Fatal("retry duplicated import", err)
	}
	req.IssueNumbers = []int{8}
	if _, err = s.ImportProjectFeedback(ctx, actor, req); err == nil {
		t.Fatal("key reused for different issues")
	}
	runner.page.Issues[0].Body = "A later, distinct observation"
	runner.page.Issues[0].UpdatedAt = "2026-10-31T02:00:00Z"
	next := feedbackPreviewForTest(t, s, ctx, actor)
	second, err := s.ImportProjectFeedback(ctx, actor, FeedbackImportRequest{PreviewID: next.ID, ExpectedRevision: 1, IssueNumbers: []int{7}, AcknowledgeSharing: true, IdempotencyKey: "feedback-import-version-second-123456"})
	if err != nil || len(second.View.Observations) != 2 || second.View.Observations[0] != original {
		t.Fatal("prior observation replaced", err)
	}
	selected, err := s.SelectProjectFeedback(ctx, actor, second.View.Revision, saved.ImportedIDs)
	if err != nil || !reflect.DeepEqual(selected.SelectedIDs, saved.ImportedIDs) {
		t.Fatal("explicit selection failed", err)
	}
	if _, err = s.SelectProjectFeedback(ctx, actor, second.View.Revision, []string{}); err == nil {
		t.Fatal("stale selection cleared newer state")
	}
	_, ledger, err := s.readFeedbackLocked(p)
	if err != nil || len(ledger.Observations) != 2 || ledger.Observations[0].Actor.ID != actor.UserID {
		t.Fatal("private ledger attribution missing", err)
	}
}

func TestFeedbackPreviewExpiryAndSourceCASDoNotPartiallyImport(t *testing.T) {
	s, p, ctx, actor, _, origin, _ := feedbackFixture(t)
	preview := feedbackPreviewForTest(t, s, ctx, actor)
	req := FeedbackImportRequest{PreviewID: preview.ID, ExpectedRevision: 0, IssueNumbers: []int{7}, AcknowledgeSharing: true, IdempotencyKey: "feedback-import-expiring-preview-123456"}
	*origin = "https://github.com/owner/changed"
	if _, err := s.ImportProjectFeedback(ctx, actor, req); err == nil {
		t.Fatal("changed repository imported")
	}
	*origin = "https://github.com/owner/repo.git"
	path, previews, err := s.readFeedbackPreviewsLocked(p, actor)
	if err != nil {
		t.Fatal(err)
	}
	at := time.Now().UTC().Add(-11 * time.Minute)
	previews.Previews[0].CreatedAt = at.Format(time.RFC3339Nano)
	previews.Previews[0].ExpiresAt = at.Add(10 * time.Minute).Format(time.RFC3339Nano)
	if err = saveFeedbackJSON(path, previews); err != nil {
		t.Fatal(err)
	}
	if _, err = s.ImportProjectFeedback(ctx, actor, req); err == nil {
		t.Fatal("expired preview imported")
	}
	_, ledger, err := s.readFeedbackLocked(p)
	if err != nil || ledger.Revision != 0 || len(ledger.Observations) != 0 {
		t.Fatal("failed import partially persisted", err)
	}
}

func TestFeedbackRollingRetentionPreservesSelectedEvidence(t *testing.T) {
	s, _, ctx, actor, runner, _, _ := feedbackFixture(t)
	revision := 0
	var protected string
	for batch := 0; batch < 21; batch++ {
		runner.page = GitHubIssuePage{Fetched: 10, Issues: []GitHubIssue{}}
		numbers := []int{}
		for i := 0; i < 10; i++ {
			number := batch*10 + i + 1
			numbers = append(numbers, number)
			runner.page.Issues = append(runner.page.Issues, GitHubIssue{Number: number, Title: fmt.Sprintf("Issue %d", number), Body: "Recorded feedback", State: "open", UpdatedAt: "2026-10-31T01:00:00Z"})
		}
		preview := feedbackPreviewForTest(t, s, ctx, actor)
		saved, err := s.ImportProjectFeedback(ctx, actor, FeedbackImportRequest{PreviewID: preview.ID, ExpectedRevision: revision, IssueNumbers: numbers, AcknowledgeSharing: true, IdempotencyKey: fmt.Sprintf("feedback-retention-batch-%06d-key", batch)})
		if err != nil {
			t.Fatal(err)
		}
		revision = saved.View.Revision
		if batch == 0 {
			protected = saved.ImportedIDs[0]
			selected, err := s.SelectProjectFeedback(ctx, actor, revision, []string{protected})
			if err != nil {
				t.Fatal(err)
			}
			revision = selected.Revision
		}
	}
	view, err := s.ProjectFeedback(ctx, actor)
	if err != nil || len(view.Observations) != 200 || view.PrunedObservations != 10 || !reflect.DeepEqual(view.SelectedIDs, []string{protected}) {
		t.Fatal("retention became a permanent-full ledger or lost selection", err)
	}
}

func TestFeedbackCapturedContextIsOptionalBoundedAndUntrusted(t *testing.T) {
	var legacy struct {
		Selected *OrchestratorFeedbackContext `json:"selectedFeedback,omitempty"`
	}
	raw, _ := json.Marshal(legacy)
	if string(raw) != "{}" {
		t.Fatal("legacy encoding changed")
	}
	if value, err := feedbackContextForLedger("p-feedback", feedbackLedger{}); err != nil || value != nil {
		t.Fatal("empty selection changed legacy context")
	}
	issue := GitHubIssue{Number: 1, Title: strings.Repeat("é", 100), Body: strings.Repeat("é", 300), State: "open", UpdatedAt: "2026-10-31T00:00:00Z"}
	digest := feedbackObservationDigest("owner/repo", issue)
	row := FeedbackObservation{ID: "feedback-" + digest, Digest: digest, Repository: "owner/repo", Issue: issue, ObservedAt: issue.UpdatedAt, ImportedAt: issue.UpdatedAt, SourceContextDigest: strings.Repeat("b", 64), Actor: ContextActor{ID: "person", DisplayName: "Person"}}
	ledger := feedbackLedger{Observations: []FeedbackObservation{row}, SelectedIDs: []string{row.ID}}
	value, err := feedbackContextForLedger("p-feedback", ledger)
	if err != nil || !value.EvidenceOnly || len(value.Items[0].BodyExcerpt) != 256 || value.OmittedBodyBytes != 344 || value.Items[0].TitleBytesOmitted != 72 {
		t.Fatal("bounded Unicode projection lost omissions", err)
	}
	value.EvidenceOnly = false
	if validOrchestratorFeedback(value, "p-feedback") {
		t.Fatal("feedback promoted to authority")
	}
	ledger.SelectedIDs = []string{"feedback-" + strings.Repeat("c", 64)}
	if _, err = feedbackContextForLedger("p-feedback", ledger); !errors.Is(err, ErrProjectFeedback) {
		t.Fatal("unknown selected evidence accepted")
	}
}
