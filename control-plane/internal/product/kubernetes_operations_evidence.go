package product

import (
	"context"
	"encoding/json"
	"errors"
	"os"
	"regexp"
	"sort"
	"time"

	"github.com/lcfrai/luci/control-plane/internal/deploymentkube"
	"github.com/lcfrai/luci/control-plane/internal/management"
)

// Only closed observation metadata is retained. There is intentionally no log
// text, span name, attribute, stacktrace, backend token or CA field here.
type OperationalEvidence struct {
	OwnedBackend      *deploymentkube.StoredTelemetrySource `json:"ownedBackend,omitempty"`
	Version           int                                   `json:"version"`
	ID                string                                `json:"id"`
	Revision          int                                   `json:"revision"`
	ProjectID         string                                `json:"projectId"`
	Actor             management.ExecutionActorRef          `json:"actor"`
	ReviewID          string                                `json:"reviewId"`
	Kind              string                                `json:"kind"`
	PlanDigest        string                                `json:"planDigest"`
	SourceCommit      string                                `json:"sourceCommit"`
	TargetID          string                                `json:"targetId"`
	TargetFingerprint string                                `json:"targetFingerprint"`
	NamespaceUID      string                                `json:"namespaceUid"`
	ComponentID       string                                `json:"componentId"`
	PodUID            string                                `json:"podUid"`
	DeploymentUID     string                                `json:"deploymentUid"`
	ServiceUID        string                                `json:"serviceUid"`
	ObservedAt        string                                `json:"observedAt"`
	State             string                                `json:"state"`
	PayloadDigest     string                                `json:"payloadDigest"`
	ObservationDigest string                                `json:"observationDigest"`
	LineCount         int                                   `json:"lineCount"`
	TraceCount        int                                   `json:"traceCount"`
	SpanCount         int                                   `json:"spanCount"`
	ErrorSpans        int                                   `json:"errorSpans"`
	Partial           bool                                  `json:"partial"`
	BackendRevision   int                                   `json:"backendRevision"`
	BackendName       string                                `json:"backendName"`
	StartAt           string                                `json:"startAt"`
	EndAt             string                                `json:"endAt"`
	LogQuery          *deploymentkube.LogQuery              `json:"logQuery,omitempty"`
	Selected          bool                                  `json:"selected"`
	SelectedAt        string                                `json:"selectedAt"`
}
type operationalEvidenceLedger struct {
	Version      int                          `json:"version"`
	AccountScope string                       `json:"accountScope"`
	ProjectScope string                       `json:"projectScope"`
	Actor        management.ExecutionActorRef `json:"actor"`
	Rows         []OperationalEvidence        `json:"rows"`
}

var operationalDigest = regexp.MustCompile(`^[a-f0-9]{64}$`)
var operationalEvidenceID = regexp.MustCompile(`^ops-evidence-[a-f0-9]{64}$`)

func validOperationalEvidence(row OperationalEvidence) bool {
	if row.OwnedBackend != nil {
		v := row.OwnedBackend
		if row.Kind != "traces" || v.Profile != "tempo-2.10.10-azure" || !validConnectionToken(v.ServiceID, 63, false) || !validConnectionToken(v.PodUID, 128, false) || !validConnectionToken(v.DeploymentUID, 128, false) || !validConnectionToken(v.ServiceUID, 128, false) || !validConnectionToken(v.ConfigUID, 128, false) || !validConnectionToken(v.ConfigVersion, 128, false) || row.BackendName != "Owned Tempo "+v.ServiceID || row.BackendRevision != 1 {
			return false
		}
	}
	if row.Version != 1 || !operationalEvidenceID.MatchString(row.ID) || row.Revision < 1 || row.Revision > 1e9 || row.Actor.UserID == "" || row.Actor.Local || row.Actor.ProjectID != row.ProjectID || !validKubeReceipt(row.ReviewID) || row.Kind != "logs" && row.Kind != "traces" || !operationalDigest.MatchString(row.PlanDigest) || !operationalDigest.MatchString(row.PayloadDigest) || !operationalDigest.MatchString(row.ObservationDigest) || !operationalDigest.MatchString(row.TargetFingerprint) || row.SourceCommit == "" || row.ComponentID == "" || len(row.ComponentID) > 63 || row.PodUID == "" || len(row.PodUID) > 128 || row.DeploymentUID == "" || len(row.DeploymentUID) > 128 || row.NamespaceUID == "" || len(row.NamespaceUID) > 128 {
		return false
	}
	if row.State != "recorded" && row.State != "empty" && row.State != "partial" && row.State != "no-data" {
		return false
	}
	if row.LineCount < 0 || row.LineCount > 201 || row.TraceCount < 0 || row.TraceCount > 4 || row.SpanCount < 0 || row.SpanCount > 1024 || row.ErrorSpans < 0 || row.ErrorSpans > row.SpanCount || row.BackendRevision < 0 || len(row.BackendName) > 120 {
		return false
	}
	if row.Kind == "logs" && (row.TraceCount != 0 || row.SpanCount != 0 || row.BackendRevision != 0 || row.BackendName != "" || row.StartAt != "" || row.EndAt != "" || row.LogQuery == nil || row.LogQuery.ComponentID != row.ComponentID || row.LogQuery.PodUID != row.PodUID || row.LogQuery.TailLines < 1 || row.LogQuery.TailLines > 200 || row.LogQuery.SinceSeconds < 1 || row.LogQuery.SinceSeconds > 3600 || !row.Partial) {
		return false
	}
	if row.Kind == "traces" && (row.LineCount != 0 || row.BackendRevision < 1 || row.BackendName == "" || row.LogQuery != nil) {
		return false
	}
	if _, err := time.Parse(time.RFC3339Nano, row.ObservedAt); err != nil {
		return false
	}
	if row.Selected {
		if _, err := time.Parse(time.RFC3339Nano, row.SelectedAt); err != nil {
			return false
		}
	} else if row.SelectedAt != "" {
		return false
	}
	return true
}
func (s *Server) readOperationalEvidence(actor management.ExecutionActorRef) (string, operationalEvidenceLedger, error) {
	_, targets, err := s.readKubernetesTargets(actor.ProjectID)
	empty := operationalEvidenceLedger{Version: 1, Actor: actor, Rows: []OperationalEvidence{}}
	if err != nil {
		return "", empty, err
	}
	empty.AccountScope, empty.ProjectScope = targets.AccountScope, targets.ProjectScope
	path, err := privateApplicationStatePathForRoot(s.config.ProjectsDir, s.stateDir, "operational-evidence-"+reviewDigest([]string{actor.UserID, actor.TeamID, actor.ProjectID}))
	if err != nil {
		return "", empty, err
	}
	var saved operationalEvidenceLedger
	err = externalReadJSON(path, 512<<10, &saved)
	if errors.Is(err, os.ErrNotExist) {
		return path, empty, nil
	}
	if err != nil || saved.Version != 1 || saved.Actor != actor || saved.AccountScope != empty.AccountScope || saved.ProjectScope != empty.ProjectScope || saved.Rows == nil || len(saved.Rows) > 64 {
		return "", empty, ErrKubernetesReview
	}
	seen := map[string]bool{}
	selected := 0
	for _, row := range saved.Rows {
		if !validOperationalEvidence(row) || row.Actor != actor || seen[row.ID] {
			return "", empty, ErrKubernetesReview
		}
		seen[row.ID] = true
		if row.Selected {
			selected++
		}
	}
	if selected > 16 {
		return "", empty, ErrKubernetesReview
	}
	return path, saved, nil
}
func (s *Server) writeOperationalEvidence(path string, ledger operationalEvidenceLedger) error {
	raw, err := json.MarshalIndent(ledger, "", "  ")
	if err != nil || len(raw)+1 > 512<<10 {
		return ErrKubernetesReview
	}
	return durableRunJSON(path, ledger)
}
func (s *Server) retainOperationalEvidence(ctx context.Context, review KubernetesReview, row OperationalEvidence) (OperationalEvidence, error) {
	if s.authorizeKubernetesTelemetryRead(ctx, review) != nil {
		return OperationalEvidence{}, ErrKubernetesReview
	}
	row.ID = "ops-evidence-" + reviewDigest([]string{row.ReviewID, row.ObservationDigest, row.Actor.UserID, row.Actor.TeamID})
	row.Revision = 1
	if !validOperationalEvidence(row) {
		return OperationalEvidence{}, ErrKubernetesReview
	}
	s.kubernetesReviewMu.Lock()
	path, ledger, err := s.readOperationalEvidence(row.Actor)
	if err != nil {
		s.kubernetesReviewMu.Unlock()
		return OperationalEvidence{}, err
	}
	for _, old := range ledger.Rows {
		if old.ID == row.ID {
			s.kubernetesReviewMu.Unlock()
			if reviewDigest(old) != reviewDigest(row) {
				return OperationalEvidence{}, ErrKubernetesReview
			}
			return old, nil
		}
	}
	ledger.Rows = append(ledger.Rows, row)
	if len(ledger.Rows) > 64 {
		removed := false
		for i, old := range ledger.Rows {
			if !old.Selected && old.ID != row.ID {
				ledger.Rows = append(append([]OperationalEvidence{}, ledger.Rows[:i]...), ledger.Rows[i+1:]...)
				removed = true
				break
			}
		}
		if !removed {
			s.kubernetesReviewMu.Unlock()
			return OperationalEvidence{}, ErrKubernetesReview
		}
	}
	if s.kubernetesAuthority(ctx, row.Actor, false, true, 0) != nil {
		s.kubernetesReviewMu.Unlock()
		return OperationalEvidence{}, management.ErrExecutionAuthority
	}
	err = s.writeOperationalEvidence(path, ledger)
	s.kubernetesReviewMu.Unlock()
	if err != nil || s.authorizeKubernetesTelemetryRead(ctx, review) != nil {
		return OperationalEvidence{}, ErrKubernetesReview
	}
	return row, nil
}

// No provider observation is performed on this source check. The returned
// evidence remains explicitly historical, never current runtime availability.
func (s *Server) operationalEvidenceReview(ctx context.Context, row OperationalEvidence) (KubernetesReview, error) {
	if s.kubernetesAuthority(ctx, row.Actor, false, true, 0) != nil {
		return KubernetesReview{}, management.ErrExecutionAuthority
	}
	s.kubernetesReviewMu.Lock()
	_, ledger, err := s.readKubernetesReviews(row.ProjectID)
	var review *KubernetesReview
	if err == nil {
		for _, candidate := range ledger.Reviews {
			if candidate.ID == row.ReviewID {
				copy := candidate
				review = &copy
				break
			}
		}
	}
	s.kubernetesReviewMu.Unlock()
	if err != nil || review == nil || !review.Applied || kubeActor(review.Request.Identity) != row.Actor || review.Request.Identity.PlanDigest != row.PlanDigest || review.Target.ID != row.TargetID || review.Target.Fingerprint != row.TargetFingerprint || review.Target.NamespaceUID != row.NamespaceUID || review.Request.Blueprint.Source.Commit != row.SourceCommit || s.authorizeKubernetesTelemetryRead(ctx, *review) != nil {
		return KubernetesReview{}, ErrKubernetesReview
	}
	return *review, nil
}
func (s *Server) operationalEvidenceSource(ctx context.Context, review KubernetesReview) error {
	actor := kubeActor(review.Request.Identity)
	input := KubernetesReviewInput{TargetID: review.Target.ID, BlueprintRevision: review.BlueprintRevision, BlueprintDigest: review.BlueprintDigest, RunID: review.RunID, CheckID: review.CheckID, MonthlyBudgetUSD: review.MonthlyBudgetUSD}
	saved, contextDigest, runDigest, checkDigest, err := s.kubernetesSource(ctx, actor, input)
	if err != nil || reviewDigest(saved.Blueprint) != reviewDigest(review.Request.Blueprint) || saved.PlanDigest != review.Request.Identity.PlanDigest || contextDigest != review.ContextDigest || runDigest != review.RunInputDigest || checkDigest != review.CheckDigest {
		return ErrKubernetesReview
	}
	return s.authorizeKubernetesTelemetryRead(ctx, review)
}
func (s *Server) operationalEvidenceCurrent(ctx context.Context, row OperationalEvidence) error {
	review, err := s.operationalEvidenceReview(ctx, row)
	if err != nil {
		return err
	}
	return s.operationalEvidenceSource(ctx, review)
}

type OperationalEvidenceSelect struct {
	ExpectedRevision int  `json:"expectedRevision"`
	Selected         bool `json:"selected"`
}

func (s *Server) SelectOperationalEvidence(ctx context.Context, actor management.ExecutionActorRef, id string, input OperationalEvidenceSelect) (OperationalEvidence, error) {
	if !operationalEvidenceID.MatchString(id) || input.ExpectedRevision < 1 || input.ExpectedRevision >= 1e9 || s.kubernetesAuthority(ctx, actor, false, true, 0) != nil {
		return OperationalEvidence{}, management.ErrExecutionAuthority
	}
	s.kubernetesReviewMu.Lock()
	_, ledger, err := s.readOperationalEvidence(actor)
	var before *OperationalEvidence
	if err == nil {
		for _, row := range ledger.Rows {
			if row.ID == id {
				copy := row
				before = &copy
				break
			}
		}
	}
	s.kubernetesReviewMu.Unlock()
	if err != nil || before == nil || before.Revision != input.ExpectedRevision || input.Selected && s.operationalEvidenceCurrent(ctx, *before) != nil {
		return OperationalEvidence{}, ErrKubernetesReview
	}
	s.kubernetesReviewMu.Lock()
	path, current, err := s.readOperationalEvidence(actor)
	if err != nil {
		s.kubernetesReviewMu.Unlock()
		return OperationalEvidence{}, err
	}
	var result *OperationalEvidence
	selected := 0
	for _, row := range current.Rows {
		if row.Selected {
			selected++
		}
	}
	if input.Selected && !before.Selected && selected >= 16 {
		s.kubernetesReviewMu.Unlock()
		return OperationalEvidence{}, ErrKubernetesReview
	}
	for i := range current.Rows {
		row := &current.Rows[i]
		if row.ID == id {
			if reviewDigest(*row) != reviewDigest(*before) {
				s.kubernetesReviewMu.Unlock()
				return OperationalEvidence{}, ErrKubernetesReview
			}
			row.Selected = input.Selected
			row.SelectedAt = ""
			if row.Selected {
				row.SelectedAt = time.Now().UTC().Format(time.RFC3339Nano)
			}
			row.Revision++
			copy := *row
			result = &copy
			break
		}
	}
	if result == nil || s.kubernetesAuthority(ctx, actor, false, true, 0) != nil {
		s.kubernetesReviewMu.Unlock()
		return OperationalEvidence{}, ErrKubernetesReview
	}
	err = s.writeOperationalEvidence(path, current)
	s.kubernetesReviewMu.Unlock()
	if err != nil || s.kubernetesAuthority(ctx, actor, false, true, 0) != nil || input.Selected && s.operationalEvidenceCurrent(ctx, *result) != nil {
		return OperationalEvidence{}, ErrKubernetesReview
	}
	return *result, nil
}

// SelectedOperationalEvidence is an opt-in structured source for ordinary Luci
// findings. It performs no provider requests and never returns raw logs/spans.
// Missing/stale evidence is withheld; callers must not turn it into a healthy0.
func (s *Server) SelectedOperationalEvidence(ctx context.Context, actor management.ExecutionActorRef) ([]OperationalEvidence, error) {
	if s.kubernetesAuthority(ctx, actor, false, true, 0) != nil {
		return nil, management.ErrExecutionAuthority
	}
	s.kubernetesReviewMu.Lock()
	_, ledger, err := s.readOperationalEvidence(actor)
	s.kubernetesReviewMu.Unlock()
	if err != nil {
		return nil, err
	}
	result := []OperationalEvidence{}
	reviews := map[string]KubernetesReview{}
	for i := len(ledger.Rows) - 1; i >= 0 && len(result) < 16; i-- {
		row := ledger.Rows[i]
		if !row.Selected {
			continue
		}
		review, err := s.operationalEvidenceReview(ctx, row)
		if err != nil {
			return nil, ErrKubernetesReview
		}
		key := reviewDigest(struct {
			Target                                                                                  KubernetesTargetDescriptor
			BlueprintRevision                                                                       int
			BlueprintDigest, RunID, CheckID, ContextDigest, RunInputDigest, CheckDigest, PlanDigest string
		}{review.Target, review.BlueprintRevision, review.BlueprintDigest, review.RunID, review.CheckID, review.ContextDigest, review.RunInputDigest, review.CheckDigest, review.Request.Identity.PlanDigest})
		if _, seen := reviews[key]; !seen {
			if s.operationalEvidenceSource(ctx, review) != nil {
				return nil, ErrKubernetesReview
			}
			reviews[key] = review
		}
		result = append(result, row)
		if ctx.Err() != nil {
			return nil, ErrKubernetesReview
		}
	}
	// Per-call successful-source memoization avoids repeating Git/check reads for
	// several samples of one review. Recheck each distinct binding at publication;
	// there is no cross-request cache or weaker current-context assertion.
	for _, review := range reviews {
		if s.operationalEvidenceSource(ctx, review) != nil {
			return nil, ErrKubernetesReview
		}
	}
	for _, row := range result {
		if _, err := s.operationalEvidenceReview(ctx, row); err != nil {
			return nil, ErrKubernetesReview
		}
	}
	s.kubernetesReviewMu.Lock()
	_, latest, readErr := s.readOperationalEvidence(actor)
	s.kubernetesReviewMu.Unlock()
	if readErr != nil || operationalSelection(actor, latest).Digest != operationalSelection(actor, ledger).Digest {
		return nil, ErrKubernetesReview
	}
	sort.Slice(result, func(i, j int) bool { return result[i].ObservedAt > result[j].ObservedAt })
	if s.kubernetesAuthority(ctx, actor, false, true, 0) != nil {
		return nil, management.ErrExecutionAuthority
	}
	return result, nil
}

type OperationalSelectionRow struct {
	ID          string `json:"id"`
	Revision    int    `json:"revision"`
	Kind        string `json:"kind"`
	ComponentID string `json:"componentId"`
	ObservedAt  string `json:"observedAt"`
}
type OperationalSelection struct {
	ProjectID  string                    `json:"projectId"`
	Digest     string                    `json:"digest"`
	Historical bool                      `json:"historical"`
	Rows       []OperationalSelectionRow `json:"rows"`
}

func operationalSelection(actor management.ExecutionActorRef, ledger operationalEvidenceLedger) OperationalSelection {
	rows := []OperationalSelectionRow{}
	for _, row := range ledger.Rows {
		if row.Selected {
			rows = append(rows, OperationalSelectionRow{ID: row.ID, Revision: row.Revision, Kind: row.Kind, ComponentID: row.ComponentID, ObservedAt: row.ObservedAt})
		}
	}
	return OperationalSelection{ProjectID: actor.ProjectID, Digest: reviewDigest(rows), Historical: true, Rows: rows}
}

// History can be read/cleared even when a source is stale. It is not a runtime
// observation or model source. The findings getter above independently checks
// current source before including any selected metadata.
func (s *Server) ReadOperationalSelection(ctx context.Context, actor management.ExecutionActorRef) (OperationalSelection, error) {
	if s.kubernetesAuthority(ctx, actor, false, true, 0) != nil {
		return OperationalSelection{}, management.ErrExecutionAuthority
	}
	s.kubernetesReviewMu.Lock()
	_, ledger, err := s.readOperationalEvidence(actor)
	s.kubernetesReviewMu.Unlock()
	if err != nil || s.kubernetesAuthority(ctx, actor, false, true, 0) != nil {
		return OperationalSelection{}, ErrKubernetesReview
	}
	return operationalSelection(actor, ledger), nil
}
func (s *Server) ClearOperationalSelection(ctx context.Context, actor management.ExecutionActorRef, expected string) (OperationalSelection, error) {
	if !hexDigest(expected) || s.kubernetesAuthority(ctx, actor, false, true, 0) != nil {
		return OperationalSelection{}, management.ErrExecutionAuthority
	}
	s.kubernetesReviewMu.Lock()
	path, ledger, err := s.readOperationalEvidence(actor)
	if err != nil || operationalSelection(actor, ledger).Digest != expected {
		s.kubernetesReviewMu.Unlock()
		return OperationalSelection{}, ErrKubernetesReview
	}
	changed := false
	for i := range ledger.Rows {
		row := &ledger.Rows[i]
		if !row.Selected {
			continue
		}
		if row.Revision >= 1e9 {
			s.kubernetesReviewMu.Unlock()
			return OperationalSelection{}, ErrKubernetesReview
		}
		row.Selected = false
		row.SelectedAt = ""
		row.Revision++
		changed = true
	}
	if s.kubernetesAuthority(ctx, actor, false, true, 0) != nil {
		s.kubernetesReviewMu.Unlock()
		return OperationalSelection{}, management.ErrExecutionAuthority
	}
	if changed {
		err = s.writeOperationalEvidence(path, ledger)
	}
	s.kubernetesReviewMu.Unlock()
	if err != nil || s.kubernetesAuthority(ctx, actor, false, true, 0) != nil {
		return OperationalSelection{}, ErrKubernetesReview
	}
	return operationalSelection(actor, ledger), nil
}
