package product

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"reflect"
	"slices"
	"strings"
	"time"

	"github.com/lcfrai/luci/control-plane/internal/management"
	"github.com/lcfrai/luci/control-plane/internal/project"
)

const feedbackHistoryLimit = 200
const feedbackSelectionLimit = 10
const feedbackLedgerLimit = 8 << 20

type ProjectFeedbackAuthorizer func(context.Context, management.ExecutionActorRef, bool) error
type FeedbackSource struct {
	Repository    string `json:"repository"`
	ContextDigest string `json:"contextDigest"`
}
type FeedbackObservation struct {
	ID                  string       `json:"id"`
	Digest              string       `json:"digest"`
	Repository          string       `json:"repository"`
	Issue               GitHubIssue  `json:"issue"`
	ObservedAt          string       `json:"observedAt"`
	ImportedAt          string       `json:"importedAt"`
	Actor               ContextActor `json:"actor"`
	SourceContextDigest string       `json:"sourceContextDigest"`
}
type ProjectFeedbackView struct {
	ProjectID          string                `json:"projectId"`
	Revision           int                   `json:"revision"`
	Source             *FeedbackSource       `json:"source"`
	Observations       []FeedbackObservation `json:"observations"`
	SelectedIDs        []string              `json:"selectedIds"`
	PrunedObservations int                   `json:"prunedObservations"`
}
type FeedbackPreview struct {
	ID           string         `json:"id"`
	ProjectID    string         `json:"projectId"`
	Source       FeedbackSource `json:"source"`
	CreatedAt    string         `json:"createdAt"`
	ExpiresAt    string         `json:"expiresAt"`
	Fetched      int            `json:"fetched"`
	Issues       []GitHubIssue  `json:"issues"`
	CLIAuthScope string         `json:"cliAuthScope"`
}
type storedFeedbackPreview struct {
	FeedbackPreview
	Actor management.ExecutionActorRef `json:"actor"`
	CLI   GitHubIssueCLIIdentity       `json:"cli"`
}
type feedbackPreviewLedger struct {
	Version      int                          `json:"version"`
	ProjectID    string                       `json:"projectId"`
	AccountScope string                       `json:"accountScope"`
	ProjectScope string                       `json:"projectScope"`
	Actor        management.ExecutionActorRef `json:"actor"`
	Previews     []storedFeedbackPreview      `json:"previews"`
}
type feedbackImportReceipt struct {
	KeyDigest     string   `json:"keyDigest"`
	RequestDigest string   `json:"requestDigest"`
	IDs           []string `json:"ids"`
}
type feedbackLedger struct {
	Version            int                     `json:"version"`
	ProjectID          string                  `json:"projectId"`
	AccountScope       string                  `json:"accountScope"`
	ProjectScope       string                  `json:"projectScope"`
	Revision           int                     `json:"revision"`
	Observations       []FeedbackObservation   `json:"observations"`
	SelectedIDs        []string                `json:"selectedIds"`
	PrunedObservations int                     `json:"prunedObservations"`
	Imports            []feedbackImportReceipt `json:"imports"`
}
type FeedbackImportRequest struct {
	PreviewID          string `json:"previewId"`
	ExpectedRevision   int    `json:"expectedRevision"`
	IssueNumbers       []int  `json:"issueNumbers"`
	AcknowledgeSharing bool   `json:"acknowledgeSharing"`
	IdempotencyKey     string `json:"idempotencyKey"`
}
type FeedbackImportResult struct {
	View        ProjectFeedbackView `json:"view"`
	ImportedIDs []string            `json:"importedIds"`
	Replayed    bool                `json:"replayed"`
}

func (s *Server) SetProjectFeedbackAuthorizer(authorize ProjectFeedbackAuthorizer) {
	s.feedbackAuthorize = authorize
}

func (s *Server) feedbackAccess(ctx context.Context, actor management.ExecutionActorRef, credentialed bool) error {
	if ctx.Err() != nil || actor.Local || actor.UserID == "" || actor.ProjectID == "" || s.feedbackAuthorize == nil {
		return management.ErrExecutionAuthority
	}
	return s.feedbackAuthorize(ctx, actor, credentialed)
}

// Callers hold s.mu. No provider work or recursive projectRoot acquisition.
func (s *Server) feedbackSourceLocked(ctx context.Context, p project.Project) (FeedbackSource, error) {
	state, err := s.readApprovedJourney(p.Path, p.ID)
	if err != nil {
		return FeedbackSource{}, ErrProjectFeedback
	}
	skills, err := orchestratorSkillsContext(state)
	if err != nil {
		return FeedbackSource{}, ErrProjectFeedback
	}
	standards, err := s.teamStandardsContext(ctx, p, state, false)
	if err != nil {
		return FeedbackSource{}, ErrProjectFeedback
	}
	_, gtm, err := s.readGTMLedger(p)
	if err != nil {
		return FeedbackSource{}, ErrProjectFeedback
	}
	gtmRevision, gtmDigest := 0, ""
	if len(gtm.Revisions) > 0 {
		last := gtm.Revisions[len(gtm.Revisions)-1]
		gtmRevision, gtmDigest = last.Revision, last.Digest
	}
	bounded, cancel := context.WithTimeout(context.WithValue(ctx, journeyGitRootContextKey{}, s.config.ProjectsDir), 3*time.Second)
	defer cancel()
	raw, err := s.journeyGit(bounded, p.Path, "config", "--get-all", "remote.origin.url")
	if err != nil {
		return FeedbackSource{}, ErrProjectFeedback
	}
	repository, err := canonicalGitHubOrigin(strings.TrimSpace(raw))
	if err != nil {
		return FeedbackSource{}, err
	}
	digest := reviewDigest(struct {
		ProjectID     string
		Brief         Brief
		Specification ProjectSpecification
		Design        ProjectDesignPlan
		Plan          ProjectPlan
		Skills        *OrchestratorSkillsContext
		Standards     *OrchestrationStandards
		GTMRevision   int
		GTMDigest     string
	}{p.ID, state.Brief, currentSpecification(state), currentDesignPlan(state), currentPlan(state, p.ID), skills, standards, gtmRevision, gtmDigest})
	return FeedbackSource{Repository: repository, ContextDigest: digest}, nil
}

func feedbackObservationDigest(repository string, issue GitHubIssue) string {
	return reviewDigest(struct {
		Repository string
		Issue      GitHubIssue
	}{repository, issue})
}
func validFeedbackObservation(row FeedbackObservation) bool {
	return validFeedbackRepository(row.Repository) && row.Digest == feedbackObservationDigest(row.Repository, row.Issue) && row.ID == "feedback-"+row.Digest && validGitHubIssuePage(GitHubIssuePage{Fetched: 1, Issues: []GitHubIssue{row.Issue}}) && hexDigest(row.SourceContextDigest) && validConnectionToken(row.Actor.ID, 128, false) && row.Actor.DisplayName != "" && len(row.Actor.DisplayName) <= 512 && validArtifactStoredText(row.Actor.DisplayName) && feedbackTimestamp(row.ObservedAt) && feedbackTimestamp(row.ImportedAt)
}
func feedbackTimestamp(value string) bool {
	if len(value) > 64 {
		return false
	}
	_, err := time.Parse(time.RFC3339Nano, value)
	return err == nil
}
func feedbackIDs(ids []string, observations []FeedbackObservation) bool {
	if ids == nil || len(ids) > feedbackSelectionLimit || !slices.IsSorted(ids) {
		return false
	}
	known := map[string]bool{}
	for _, row := range observations {
		known[row.ID] = true
	}
	seen := map[string]bool{}
	for _, id := range ids {
		if !known[id] || seen[id] {
			return false
		}
		seen[id] = true
	}
	return true
}

func (s *Server) readFeedbackLocked(p project.Project) (string, feedbackLedger, error) {
	scope, err := s.approvalScopes(p.Path, p.ID)
	if err != nil {
		return "", feedbackLedger{}, err
	}
	path, err := privateApplicationStatePathForRoot(s.config.ProjectsDir, s.stateDir, "project-feedback-"+p.ID)
	empty := feedbackLedger{Version: 1, ProjectID: p.ID, AccountScope: scope.AccountScope, ProjectScope: scope.ProjectScope, Observations: []FeedbackObservation{}, SelectedIDs: []string{}, Imports: []feedbackImportReceipt{}}
	if err != nil {
		return "", empty, err
	}
	var saved feedbackLedger
	err = externalReadJSON(path, feedbackLedgerLimit, &saved)
	if errors.Is(err, os.ErrNotExist) {
		return path, empty, nil
	}
	if err != nil || saved.Version != 1 || saved.ProjectID != p.ID || saved.AccountScope != empty.AccountScope || saved.ProjectScope != empty.ProjectScope || saved.Revision < 0 || saved.Revision > 1_000_000_000 || saved.PrunedObservations < 0 || saved.PrunedObservations > 1_000_000_000 || saved.Revision == 0 && len(saved.Observations) > 0 || saved.Observations == nil || len(saved.Observations) > feedbackHistoryLimit || saved.Imports == nil || len(saved.Imports) > 32 || !feedbackIDs(saved.SelectedIDs, saved.Observations) {
		return "", empty, ErrProjectFeedback
	}
	seen := map[string]bool{}
	for _, row := range saved.Observations {
		if !validFeedbackObservation(row) || seen[row.ID] {
			return "", empty, ErrProjectFeedback
		}
		seen[row.ID] = true
	}
	keys := map[string]bool{}
	for _, receipt := range saved.Imports {
		ids := append([]string(nil), receipt.IDs...)
		slices.Sort(ids)
		if !hexDigest(receipt.KeyDigest) || !hexDigest(receipt.RequestDigest) || keys[receipt.KeyDigest] || len(ids) == 0 || !feedbackIDs(ids, saved.Observations) {
			return "", empty, ErrProjectFeedback
		}
		keys[receipt.KeyDigest] = true
	}
	return path, saved, nil
}

func (s *Server) readFeedbackPreviewsLocked(p project.Project, actor management.ExecutionActorRef) (string, feedbackPreviewLedger, error) {
	scope, err := s.approvalScopes(p.Path, p.ID)
	if err != nil {
		return "", feedbackPreviewLedger{}, err
	}
	path, err := privateApplicationStatePathForRoot(s.config.ProjectsDir, s.stateDir, "feedback-previews-"+p.ID+"-"+reviewDigest(actor)[:24])
	empty := feedbackPreviewLedger{Version: 1, ProjectID: p.ID, AccountScope: scope.AccountScope, ProjectScope: scope.ProjectScope, Actor: actor, Previews: []storedFeedbackPreview{}}
	if err != nil {
		return "", empty, err
	}
	var saved feedbackPreviewLedger
	err = externalReadJSON(path, feedbackLedgerLimit, &saved)
	if errors.Is(err, os.ErrNotExist) {
		return path, empty, nil
	}
	if err != nil || saved.Version != 1 || saved.ProjectID != p.ID || saved.AccountScope != empty.AccountScope || saved.ProjectScope != empty.ProjectScope || saved.Actor != actor || saved.Previews == nil || len(saved.Previews) > 2 {
		return "", empty, ErrProjectFeedback
	}
	seen := map[string]bool{}
	for _, row := range saved.Previews {
		created, e1 := time.Parse(time.RFC3339Nano, row.CreatedAt)
		expiry, e2 := time.Parse(time.RFC3339Nano, row.ExpiresAt)
		if row.Actor != actor || row.ProjectID != p.ID || !strings.HasPrefix(row.ID, "feedback-preview-") || !prototypeReleaseID.MatchString(row.ID) || seen[row.ID] || !validFeedbackRepository(row.Source.Repository) || !hexDigest(row.Source.ContextDigest) || e1 != nil || e2 != nil || !expiry.Equal(created.Add(10*time.Minute)) || row.CLIAuthScope != "workspace-cli-not-personal-identity" || !hexDigest(row.CLI.SHA256) || row.CLI.Bytes <= 0 || !validGitHubIssuePage(GitHubIssuePage{Fetched: row.Fetched, Issues: row.Issues}) {
			return "", empty, ErrProjectFeedback
		}
		seen[row.ID] = true
	}
	return path, saved, nil
}

func saveFeedbackJSON(path string, value any) error {
	raw, err := json.MarshalIndent(value, "", "  ")
	if err != nil || len(raw)+1 > feedbackLedgerLimit {
		return ErrProjectFeedback
	}
	return durableRunJSON(path, value)
}
func feedbackView(p project.Project, row feedbackLedger, source *FeedbackSource) ProjectFeedbackView {
	return ProjectFeedbackView{ProjectID: p.ID, Revision: row.Revision, Source: source, Observations: row.Observations, SelectedIDs: row.SelectedIDs, PrunedObservations: row.PrunedObservations}
}

func (s *Server) ProjectFeedback(ctx context.Context, actor management.ExecutionActorRef) (ProjectFeedbackView, error) {
	if err := s.feedbackAccess(ctx, actor, false); err != nil {
		return ProjectFeedbackView{}, err
	}
	p, err := s.resolveProject(actor.ProjectID)
	if err != nil {
		return ProjectFeedbackView{}, err
	}
	s.mu.RLock()
	_, row, err := s.readFeedbackLocked(p)
	source, sourceErr := s.feedbackSourceLocked(ctx, p)
	s.mu.RUnlock()
	if err != nil {
		return ProjectFeedbackView{}, err
	}
	if err = s.feedbackAccess(ctx, actor, false); err != nil {
		return ProjectFeedbackView{}, err
	}
	if sourceErr != nil {
		return feedbackView(p, row, nil), nil
	}
	return feedbackView(p, row, &source), nil
}

func (s *Server) PreviewProjectFeedback(ctx context.Context, actor management.ExecutionActorRef, expected FeedbackSource) (FeedbackPreview, error) {
	if err := s.feedbackAccess(ctx, actor, true); err != nil {
		return FeedbackPreview{}, err
	}
	p, err := s.resolveProject(actor.ProjectID)
	if err != nil {
		return FeedbackPreview{}, err
	}
	s.mu.RLock()
	source, err := s.feedbackSourceLocked(ctx, p)
	s.mu.RUnlock()
	if err != nil || source != expected || s.githubIssuesRunner == nil {
		return FeedbackPreview{}, ErrProjectFeedback
	}
	// No application/state lock spans credentialed external I/O.
	identity, err := s.githubIssuesRunner.Identity(ctx)
	if err != nil {
		return FeedbackPreview{}, ErrProjectFeedback
	}
	if identity.Path == "" || identity.Bytes <= 0 || !hexDigest(identity.SHA256) {
		return FeedbackPreview{}, ErrProjectFeedback
	}
	if err = s.feedbackAccess(ctx, actor, true); err != nil {
		return FeedbackPreview{}, err
	}
	page, err := s.githubIssuesRunner.ReadIssues(ctx, identity, source.Repository)
	if err != nil {
		return FeedbackPreview{}, err
	}
	if !validGitHubIssuePage(page) {
		return FeedbackPreview{}, ErrProjectFeedback
	}
	if err = s.feedbackAccess(ctx, actor, true); err != nil {
		return FeedbackPreview{}, err
	}
	s.mu.RLock()
	defer s.mu.RUnlock()
	s.feedbackMu.Lock()
	defer s.feedbackMu.Unlock()
	current, err := s.feedbackSourceLocked(ctx, p)
	if err != nil || current != source {
		return FeedbackPreview{}, ErrProjectFeedback
	}
	path, ledger, err := s.readFeedbackPreviewsLocked(p, actor)
	if err != nil {
		return FeedbackPreview{}, err
	}
	now := time.Now().UTC()
	preview := FeedbackPreview{ID: newID("feedback-preview-"), ProjectID: p.ID, Source: source, CreatedAt: now.Format(time.RFC3339Nano), ExpiresAt: now.Add(10 * time.Minute).Format(time.RFC3339Nano), Fetched: page.Fetched, Issues: page.Issues, CLIAuthScope: "workspace-cli-not-personal-identity"}
	retained := []storedFeedbackPreview{}
	for _, old := range ledger.Previews {
		expiry, _ := time.Parse(time.RFC3339Nano, old.ExpiresAt)
		if now.Before(expiry) {
			retained = append(retained, old)
		}
	}
	if len(retained) > 1 {
		retained = retained[len(retained)-1:]
	}
	ledger.Previews = append(retained, storedFeedbackPreview{FeedbackPreview: preview, Actor: actor, CLI: identity})
	if err = s.feedbackAccess(ctx, actor, true); err != nil {
		return FeedbackPreview{}, err
	}
	if err = saveFeedbackJSON(path, ledger); err != nil {
		return FeedbackPreview{}, err
	}
	if err = s.feedbackAccess(ctx, actor, true); err != nil {
		return FeedbackPreview{}, err
	}
	return preview, nil
}

func (s *Server) ImportProjectFeedback(ctx context.Context, actor management.ExecutionActorRef, req FeedbackImportRequest) (FeedbackImportResult, error) {
	fail := func(err error) (FeedbackImportResult, error) { return FeedbackImportResult{}, err }
	if err := s.feedbackAccess(ctx, actor, true); err != nil {
		return fail(err)
	}
	if !req.AcknowledgeSharing || !validRunKey(req.IdempotencyKey) || req.ExpectedRevision < 0 || len(req.IssueNumbers) == 0 || len(req.IssueNumbers) > feedbackSelectionLimit {
		return fail(ErrProjectFeedback)
	}
	numbers := append([]int(nil), req.IssueNumbers...)
	slices.Sort(numbers)
	for i, n := range numbers {
		if n <= 0 || i > 0 && numbers[i-1] == n {
			return fail(ErrProjectFeedback)
		}
	}
	p, err := s.resolveProject(actor.ProjectID)
	if err != nil {
		return fail(err)
	}
	person, ok := authenticatedContextActor(ctx)
	if !ok || person.ID != actor.UserID {
		return fail(management.ErrExecutionAuthority)
	}
	s.mu.RLock()
	defer s.mu.RUnlock()
	s.feedbackMu.Lock()
	defer s.feedbackMu.Unlock()
	path, row, err := s.readFeedbackLocked(p)
	if err != nil {
		return fail(err)
	}
	key := reviewDigest([]string{"github-feedback-import-v1", actor.UserID, actor.TeamID, req.IdempotencyKey})
	request := reviewDigest(struct {
		Preview string
		Numbers []int
	}{req.PreviewID, numbers})
	source, sourceErr := s.feedbackSourceLocked(ctx, p)
	for _, prior := range row.Imports {
		if prior.KeyDigest == key {
			if prior.RequestDigest != request || sourceErr != nil {
				return fail(ErrProjectFeedback)
			}
			if err = s.feedbackAccess(ctx, actor, true); err != nil {
				return fail(err)
			}
			return FeedbackImportResult{View: feedbackView(p, row, &source), ImportedIDs: prior.IDs, Replayed: true}, nil
		}
	}
	if sourceErr != nil || req.ExpectedRevision != row.Revision || row.Revision >= 1_000_000_000 {
		return fail(ErrProjectFeedback)
	}
	_, previews, err := s.readFeedbackPreviewsLocked(p, actor)
	if err != nil {
		return fail(err)
	}
	var preview *FeedbackPreview
	for i := range previews.Previews {
		if previews.Previews[i].ID == req.PreviewID {
			preview = &previews.Previews[i].FeedbackPreview
		}
	}
	if preview == nil || preview.Source != source {
		return fail(ErrProjectFeedback)
	}
	expiry, _ := time.Parse(time.RFC3339Nano, preview.ExpiresAt)
	if !time.Now().Before(expiry) {
		return fail(ErrProjectFeedback)
	}
	issues := map[int]GitHubIssue{}
	for _, issue := range preview.Issues {
		issues[issue.Number] = issue
	}
	known := map[string]bool{}
	for _, old := range row.Observations {
		known[old.ID] = true
	}
	ids := []string{}
	at := time.Now().UTC().Format(time.RFC3339Nano)
	for _, number := range numbers {
		issue, present := issues[number]
		if !present {
			return fail(ErrProjectFeedback)
		}
		digest := feedbackObservationDigest(source.Repository, issue)
		id := "feedback-" + digest
		ids = append(ids, id)
		if !known[id] {
			row.Observations = append(row.Observations, FeedbackObservation{ID: id, Digest: digest, Repository: source.Repository, Issue: issue, ObservedAt: preview.CreatedAt, ImportedAt: at, Actor: person, SourceContextDigest: source.ContextDigest})
			known[id] = true
		}
	}
	slices.Sort(ids)
	protected := map[string]bool{}
	for _, id := range append(append([]string{}, row.SelectedIDs...), ids...) {
		protected[id] = true
	}
	for len(row.Observations) > feedbackHistoryLimit {
		removed := false
		for i, old := range row.Observations {
			if !protected[old.ID] {
				row.Observations = append(row.Observations[:i], row.Observations[i+1:]...)
				row.PrunedObservations++
				removed = true
				break
			}
		}
		if !removed {
			return fail(ErrProjectFeedback)
		}
	}
	receipts := []feedbackImportReceipt{}
	for _, prior := range row.Imports {
		sorted := append([]string(nil), prior.IDs...)
		slices.Sort(sorted)
		if feedbackIDs(sorted, row.Observations) {
			receipts = append(receipts, prior)
		}
	}
	if len(receipts) > 31 {
		receipts = receipts[len(receipts)-31:]
	}
	row.Imports = append(receipts, feedbackImportReceipt{KeyDigest: key, RequestDigest: request, IDs: ids})
	row.Revision++
	if err = s.feedbackAccess(ctx, actor, true); err != nil {
		return fail(err)
	}
	if err = saveFeedbackJSON(path, row); err != nil {
		return fail(err)
	}
	if err = s.feedbackAccess(ctx, actor, true); err != nil {
		return fail(err)
	}
	return FeedbackImportResult{View: feedbackView(p, row, &source), ImportedIDs: ids}, nil
}

func (s *Server) SelectProjectFeedback(ctx context.Context, actor management.ExecutionActorRef, expected int, ids []string) (ProjectFeedbackView, error) {
	if err := s.feedbackAccess(ctx, actor, true); err != nil {
		return ProjectFeedbackView{}, err
	}
	p, err := s.resolveProject(actor.ProjectID)
	if err != nil {
		return ProjectFeedbackView{}, err
	}
	s.mu.RLock()
	defer s.mu.RUnlock()
	s.feedbackMu.Lock()
	defer s.feedbackMu.Unlock()
	path, row, err := s.readFeedbackLocked(p)
	if err != nil {
		return ProjectFeedbackView{}, err
	}
	selected := append([]string{}, ids...)
	slices.Sort(selected)
	if expected != row.Revision || !feedbackIDs(selected, row.Observations) || row.Revision >= 1_000_000_000 {
		return ProjectFeedbackView{}, ErrProjectFeedback
	}
	if !reflect.DeepEqual(selected, row.SelectedIDs) {
		row.SelectedIDs = selected
		if _, err = feedbackContextForLedger(p.ID, row); err != nil {
			return ProjectFeedbackView{}, err
		}
		row.Revision++
		if err = s.feedbackAccess(ctx, actor, true); err != nil {
			return ProjectFeedbackView{}, err
		}
		if err = saveFeedbackJSON(path, row); err != nil {
			return ProjectFeedbackView{}, err
		}
	}
	source, sourceErr := s.feedbackSourceLocked(ctx, p)
	if err = s.feedbackAccess(ctx, actor, true); err != nil {
		return ProjectFeedbackView{}, err
	}
	if sourceErr != nil {
		return feedbackView(p, row, nil), nil
	}
	return feedbackView(p, row, &source), nil
}

func feedbackSourceURL(row FeedbackObservation) string {
	return fmt.Sprintf("https://github.com/%s/issues/%d", row.Repository, row.Issue.Number)
}
