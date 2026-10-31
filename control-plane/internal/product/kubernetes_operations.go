package product

import (
	"context"
	"encoding/json"
	"errors"
	"os"
	"strings"
	"time"

	"github.com/lcfrai/luci/control-plane/internal/deploymentkube"
	"github.com/lcfrai/luci/control-plane/internal/management"
)

type KubernetesTempoSave struct {
	ExpectedRevision int                          `json:"expectedRevision"`
	Name             string                       `json:"name"`
	Connection       deploymentkube.ExistingTempo `json:"connection"`
}
type KubernetesTempoView struct {
	TargetID          string `json:"targetId"`
	TargetFingerprint string `json:"targetFingerprint"`
	Revision          int    `json:"revision"`
	Configured        bool   `json:"configured"`
	Name              string `json:"name"`
	Endpoint          string `json:"endpoint"`
	UpdatedAt         string `json:"updatedAt"`
}
type kubernetesTempoRecord struct {
	Version      int                          `json:"version"`
	AccountScope string                       `json:"accountScope"`
	ProjectScope string                       `json:"projectScope"`
	Actor        management.ExecutionActorRef `json:"actor"`
	Target       KubernetesTargetDescriptor   `json:"target"`
	Revision     int                          `json:"revision"`
	Name         string                       `json:"name"`
	Connection   deploymentkube.ExistingTempo `json:"connection"`
	UpdatedAt    string                       `json:"updatedAt"`
}

func (s *Server) operationalReview(ctx context.Context, actor management.ExecutionActorRef, id string) (KubernetesReview, error) {
	if !validKubeReceipt(id) || s.kubernetesAuthority(ctx, actor, false, true, 0) != nil {
		return KubernetesReview{}, management.ErrExecutionAuthority
	}
	s.kubernetesReviewMu.Lock()
	_, ledger, err := s.readKubernetesReviews(actor.ProjectID)
	var found *KubernetesReview
	if err == nil {
		for _, row := range ledger.Reviews {
			if row.ID == id && kubeActor(row.Request.Identity) == actor && row.Applied {
				copy := row
				found = &copy
				break
			}
		}
	}
	s.kubernetesReviewMu.Unlock()
	if err != nil || found == nil || s.authorizeKubernetesReview(ctx, *found, false, false) != nil {
		return KubernetesReview{}, ErrKubernetesReview
	}
	return *found, nil
}

func (s *Server) operationalConnector(row KubernetesReview) *deploymentkube.Connector {
	return s.NewKubernetesDeploymentConnector(s.resolveKubernetesTarget, func(ctx context.Context, request deploymentkube.Request, mutation bool) error {
		if mutation || reviewDigest(request) != reviewDigest(row.Request) {
			return ErrKubernetesReview
		}
		return s.authorizeKubernetesTelemetryRead(ctx, row)
	})
}

func (s *Server) ReadKubernetesOperationalPods(ctx context.Context, actor management.ExecutionActorRef, id, componentID string) (deploymentkube.OperationalPods, error) {
	ctx = WithKubernetesTargetResolution(ctx)
	row, err := s.operationalReview(ctx, actor, id)
	if err != nil {
		return deploymentkube.OperationalPods{}, err
	}
	result, err := s.operationalConnector(row).ReadOperationalPods(ctx, row.Request.Identity, componentID)
	if err != nil || s.authorizeKubernetesReview(ctx, row, false, false) != nil {
		return deploymentkube.OperationalPods{}, ErrKubernetesReview
	}
	return result, nil
}

type KubernetesLogRead struct {
	Query                    deploymentkube.LogQuery `json:"query"`
	AcknowledgeSensitiveLogs bool                    `json:"acknowledgeSensitiveLogs"`
}
type KubernetesLogResult struct {
	Observation deploymentkube.OperationalLogs `json:"observation"`
	Evidence    OperationalEvidence            `json:"evidence"`
}

func (s *Server) ReadKubernetesOperationalLogs(ctx context.Context, actor management.ExecutionActorRef, id string, input KubernetesLogRead) (KubernetesLogResult, error) {
	ctx = WithKubernetesTargetResolution(ctx)
	if !input.AcknowledgeSensitiveLogs {
		return KubernetesLogResult{}, ErrKubernetesReview
	}
	row, err := s.operationalReview(ctx, actor, id)
	if err != nil {
		return KubernetesLogResult{}, err
	}
	result, err := s.operationalConnector(row).ReadOperationalLogs(ctx, row.Request.Identity, input.Query)
	if err != nil || s.authorizeKubernetesReview(ctx, row, false, false) != nil {
		return KubernetesLogResult{}, ErrKubernetesReview
	}
	// Retain identity/count/hash only. The bounded log text is never written to
	// the journal or handed to a model/automatic finding by this method.
	evidence := OperationalEvidence{Version: 1, ProjectID: actor.ProjectID, Actor: actor, ReviewID: id, Kind: "logs", PlanDigest: row.Request.Identity.PlanDigest, SourceCommit: row.Request.Blueprint.Source.Commit, TargetID: row.Target.ID, TargetFingerprint: row.Target.Fingerprint, NamespaceUID: result.NamespaceUID, ComponentID: result.Pod.ComponentID, PodUID: result.Pod.UID, DeploymentUID: result.Pod.DeploymentUID, ServiceUID: result.Pod.ServiceUID, ObservedAt: result.ObservedAt, State: result.State, PayloadDigest: result.TextSHA256, ObservationDigest: result.EvidenceDigest, LineCount: result.LineCount, Partial: true, LogQuery: &result.Query}
	saved, err := s.retainOperationalEvidence(ctx, row, evidence)
	if err != nil || s.authorizeKubernetesReview(ctx, row, false, false) != nil {
		return KubernetesLogResult{}, ErrKubernetesReview
	}
	return KubernetesLogResult{Observation: result, Evidence: saved}, nil
}

func (s *Server) tempoRecord(actor management.ExecutionActorRef, targetID string) (string, kubernetesTempoRecord, error) {
	if targetID == "" || len(targetID) > 128 {
		return "", kubernetesTempoRecord{}, ErrKubernetesReview
	}
	_, targets, err := s.readKubernetesTargets(actor.ProjectID)
	if err != nil {
		return "", kubernetesTempoRecord{}, err
	}
	var descriptor *KubernetesTargetDescriptor
	for _, target := range targets.Targets {
		if target.ID == targetID {
			copy := target.KubernetesTargetDescriptor
			descriptor = &copy
		}
	}
	if descriptor == nil {
		return "", kubernetesTempoRecord{}, ErrKubernetesReview
	}
	key := reviewDigest([]string{actor.UserID, actor.TeamID, actor.ProjectID, targetID})
	path, err := privateApplicationStatePathForRoot(s.config.ProjectsDir, s.stateDir, "tempo-query-"+key)
	empty := kubernetesTempoRecord{Version: 1, AccountScope: targets.AccountScope, ProjectScope: targets.ProjectScope, Actor: actor, Target: *descriptor}
	if err != nil {
		return "", empty, err
	}
	var saved kubernetesTempoRecord
	err = externalReadJSON(path, 512<<10, &saved)
	if errors.Is(err, os.ErrNotExist) {
		return path, empty, nil
	}
	if err != nil || saved.Version != 1 || saved.AccountScope != empty.AccountScope || saved.ProjectScope != empty.ProjectScope || saved.Actor != actor || saved.Target.ID != descriptor.ID || saved.Target.ProjectID != actor.ProjectID || !operationalDigest.MatchString(saved.Target.Fingerprint) || saved.Revision < 1 || saved.Revision > 1e9 || !validArtifactStoredText(saved.Name) || len(saved.Name) < 1 || len(saved.Name) > 120 {
		return "", empty, ErrKubernetesReview
	}
	// Shape/TLS validation only: this private reader can never issue a request.
	if _, err = deploymentkube.NewTempoReader(saved.Connection, func(context.Context) error { return ErrKubernetesReview }); err != nil {
		return "", empty, ErrKubernetesReview
	}
	if _, err = time.Parse(time.RFC3339Nano, saved.UpdatedAt); err != nil {
		return "", empty, ErrKubernetesReview
	}
	return path, saved, nil
}
func tempoView(record kubernetesTempoRecord) KubernetesTempoView {
	return KubernetesTempoView{TargetID: record.Target.ID, TargetFingerprint: record.Target.Fingerprint, Revision: record.Revision, Configured: record.Revision > 0, Name: record.Name, Endpoint: record.Connection.Endpoint, UpdatedAt: record.UpdatedAt}
}
func (s *Server) KubernetesTempo(ctx context.Context, actor management.ExecutionActorRef, targetID string) (KubernetesTempoView, error) {
	if s.kubernetesAuthority(ctx, actor, false, true, 0) != nil {
		return KubernetesTempoView{}, management.ErrExecutionAuthority
	}
	s.kubernetesReviewMu.Lock()
	_, record, err := s.tempoRecord(actor, targetID)
	s.kubernetesReviewMu.Unlock()
	if err != nil || s.kubernetesAuthority(ctx, actor, false, true, 0) != nil {
		return KubernetesTempoView{}, ErrKubernetesReview
	}
	return tempoView(record), nil
}
func (s *Server) SaveKubernetesTempo(ctx context.Context, actor management.ExecutionActorRef, targetID string, input KubernetesTempoSave) (KubernetesTempoView, error) {
	if s.kubernetesAuthority(ctx, actor, false, true, 0) != nil {
		return KubernetesTempoView{}, management.ErrExecutionAuthority
	}
	if input.ExpectedRevision < 0 || input.ExpectedRevision >= 1e9 || len(strings.TrimSpace(input.Name)) < 1 || len(input.Name) > 120 || !validArtifactStoredText(input.Name) {
		return KubernetesTempoView{}, ErrKubernetesReview
	}
	if _, err := deploymentkube.NewTempoReader(input.Connection, func(context.Context) error { return ErrKubernetesReview }); err != nil {
		return KubernetesTempoView{}, ErrKubernetesReview
	}
	s.kubernetesReviewMu.Lock()
	path, record, err := s.tempoRecord(actor, targetID)
	if err != nil || record.Revision != input.ExpectedRevision {
		s.kubernetesReviewMu.Unlock()
		return KubernetesTempoView{}, ErrKubernetesReview
	}
	_, targets, err := s.readKubernetesTargets(actor.ProjectID)
	matched := false
	if err == nil {
		for _, target := range targets.Targets {
			if target.ID == targetID {
				record.Target = target.KubernetesTargetDescriptor
				matched = true
				break
			}
		}
	}
	if !matched {
		s.kubernetesReviewMu.Unlock()
		return KubernetesTempoView{}, ErrKubernetesReview
	}
	record.Revision++
	record.Name = input.Name
	record.Connection = input.Connection
	record.UpdatedAt = time.Now().UTC().Format(time.RFC3339Nano)
	raw, marshalErr := json.MarshalIndent(record, "", "  ")
	if marshalErr != nil || len(raw)+1 > 512<<10 || s.kubernetesAuthority(ctx, actor, false, true, 0) != nil {
		s.kubernetesReviewMu.Unlock()
		return KubernetesTempoView{}, ErrKubernetesReview
	}
	err = durableRunJSON(path, record)
	s.kubernetesReviewMu.Unlock()
	if err != nil || s.kubernetesAuthority(ctx, actor, false, true, 0) != nil {
		return KubernetesTempoView{}, ErrKubernetesReview
	}
	return tempoView(record), nil
}

type KubernetesTraceResult struct {
	Observation deploymentkube.OperationalTraces `json:"observation"`
	Evidence    OperationalEvidence              `json:"evidence"`
	Backend     KubernetesTempoView              `json:"backend"`
}

func (s *Server) ReadKubernetesOperationalTraces(ctx context.Context, actor management.ExecutionActorRef, id string, input deploymentkube.TraceQuery) (KubernetesTraceResult, error) {
	if input.OwnedBackend {
		return s.readOwnedKubernetesTraces(ctx, actor, id, input)
	}
	ctx = WithKubernetesTargetResolution(ctx)
	ctx, cancel := context.WithTimeout(ctx, 90*time.Second)
	defer cancel()
	row, err := s.operationalReview(ctx, actor, id)
	if err != nil {
		return KubernetesTraceResult{}, err
	}
	s.kubernetesReviewMu.Lock()
	_, record, err := s.tempoRecord(actor, row.Target.ID)
	s.kubernetesReviewMu.Unlock()
	if err != nil || record.Revision == 0 || record.Target != row.Target {
		return KubernetesTraceResult{}, ErrKubernetesReview
	}
	current := func(current context.Context) error {
		if s.authorizeKubernetesTelemetryRead(current, row) != nil {
			return ErrKubernetesReview
		}
		s.kubernetesReviewMu.Lock()
		_, latest, err := s.tempoRecord(actor, row.Target.ID)
		s.kubernetesReviewMu.Unlock()
		if err != nil || reviewDigest(latest) != reviewDigest(record) {
			return ErrKubernetesReview
		}
		return s.authorizeKubernetesTelemetryRead(current, row)
	}
	reader, err := deploymentkube.NewTempoReader(record.Connection, current)
	if err != nil {
		return KubernetesTraceResult{}, ErrKubernetesReview
	}
	result, err := s.operationalConnector(row).ReadOperationalTraces(ctx, row.Request.Identity, input, reader)
	if err != nil || current(ctx) != nil || s.authorizeKubernetesReview(ctx, row, false, false) != nil {
		return KubernetesTraceResult{}, ErrKubernetesReview
	}
	count, errors := 0, 0
	for _, trace := range result.Traces {
		count += trace.SpanCount
		errors += trace.ErrorSpans
	}
	evidence := OperationalEvidence{Version: 1, ProjectID: actor.ProjectID, Actor: actor, ReviewID: id, Kind: "traces", PlanDigest: row.Request.Identity.PlanDigest, SourceCommit: row.Request.Blueprint.Source.Commit, TargetID: row.Target.ID, TargetFingerprint: row.Target.Fingerprint, NamespaceUID: result.NamespaceUID, ComponentID: result.Pod.ComponentID, PodUID: result.Pod.UID, DeploymentUID: result.Pod.DeploymentUID, ServiceUID: result.Pod.ServiceUID, ObservedAt: result.ObservedAt, State: result.State, PayloadDigest: reviewDigest(result.Traces), ObservationDigest: result.EvidenceDigest, TraceCount: len(result.Traces), SpanCount: count, ErrorSpans: errors, Partial: result.Partial, BackendRevision: record.Revision, BackendName: record.Name, StartAt: result.StartAt, EndAt: result.EndAt}
	saved, err := s.retainOperationalEvidence(ctx, row, evidence)
	if err != nil || current(ctx) != nil || s.authorizeKubernetesReview(ctx, row, false, false) != nil {
		return KubernetesTraceResult{}, ErrKubernetesReview
	}
	return KubernetesTraceResult{Observation: result, Evidence: saved, Backend: tempoView(record)}, nil
}
