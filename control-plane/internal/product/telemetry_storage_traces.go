package product

import (
	"context"
	"github.com/lcfrai/luci/control-plane/internal/deploymentkube"
	"github.com/lcfrai/luci/control-plane/internal/management"
	"time"
)

func (s *Server) readOwnedKubernetesTraces(ctx context.Context, actor management.ExecutionActorRef, id string, input deploymentkube.TraceQuery) (KubernetesTraceResult, error) {
	ctx = WithKubernetesTargetResolution(ctx)
	ctx, cancel := context.WithTimeout(ctx, 90*time.Second)
	defer cancel()
	row, err := s.operationalReview(ctx, actor, id)
	if err != nil {
		return KubernetesTraceResult{}, err
	}
	result, err := s.operationalConnector(row).ReadOwnedOperationalTraces(ctx, row.Request.Identity, input)
	if err != nil || result.Storage == nil || s.authorizeKubernetesReview(ctx, row, false, false) != nil {
		return KubernetesTraceResult{}, ErrKubernetesReview
	}
	count, errors := 0, 0
	for _, trace := range result.Traces {
		count += trace.SpanCount
		errors += trace.ErrorSpans
	}
	// BackendRevision1 denotes this closed storage profile, not a fabricated
	// external connection revision. Exact plan/target/owned Pod are sealed below.
	name := "Owned Tempo " + result.Storage.ServiceID
	evidence := OperationalEvidence{OwnedBackend: result.Storage, Version: 1, ProjectID: actor.ProjectID, Actor: actor, ReviewID: id, Kind: "traces", PlanDigest: row.Request.Identity.PlanDigest, SourceCommit: row.Request.Blueprint.Source.Commit, TargetID: row.Target.ID, TargetFingerprint: row.Target.Fingerprint, NamespaceUID: result.NamespaceUID, ComponentID: result.Pod.ComponentID, PodUID: result.Pod.UID, DeploymentUID: result.Pod.DeploymentUID, ServiceUID: result.Pod.ServiceUID, ObservedAt: result.ObservedAt, State: result.State, PayloadDigest: reviewDigest(result.Traces), ObservationDigest: result.EvidenceDigest, TraceCount: len(result.Traces), SpanCount: count, ErrorSpans: errors, Partial: result.Partial, BackendRevision: 1, BackendName: name, StartAt: result.StartAt, EndAt: result.EndAt}
	saved, err := s.retainOperationalEvidence(ctx, row, evidence)
	if err != nil || s.authorizeKubernetesReview(ctx, row, false, false) != nil {
		return KubernetesTraceResult{}, ErrKubernetesReview
	}
	backend := KubernetesTempoView{TargetID: row.Target.ID, TargetFingerprint: row.Target.Fingerprint, Revision: 1, Configured: true, Name: name, Endpoint: "Owned cluster storage; private mTLS tunnel", UpdatedAt: result.ObservedAt}
	return KubernetesTraceResult{Observation: result, Evidence: saved, Backend: backend}, nil
}
