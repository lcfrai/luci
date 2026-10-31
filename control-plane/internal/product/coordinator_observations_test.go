package product

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/lcfrai/luci/control-plane/internal/deploymentkube"
	"github.com/lcfrai/luci/control-plane/internal/management"
)

func TestObservationSignalsIgnoreReadTimestampButKeepCounterAndOwnerChanges(t *testing.T) {
	a := deploymentkube.TelemetryObservation{ObservedAt: "2026-10-31T00:00:00Z", EvidenceDigest: strings.Repeat("a", 64), Continuity: "baseline", CollectorPodUID: "pod-one", State: "recorded", Samples: []deploymentkube.ProductEventSample{{ServiceID: "app", EventID: "activation", Outcome: "success", ReportedCount: 2}}}
	b := a
	b.ObservedAt = "2026-10-31T00:01:00Z"
	b.EvidenceDigest = strings.Repeat("b", 64)
	b.Continuity = "continued"
	if reviewDigest(stableTelemetrySignal(a)) != reviewDigest(stableTelemetrySignal(b)) {
		t.Fatal("same saved counts bought another automatic turn")
	}
	b.CollectorPodUID = "replacement"
	if reviewDigest(stableTelemetrySignal(a)) == reviewDigest(stableTelemetrySignal(b)) {
		t.Fatal("changed original owner was ignored")
	}
	b = a
	b.Samples = append([]deploymentkube.ProductEventSample{}, a.Samples...)
	b.Samples[0].ReportedCount = 3
	if reviewDigest(stableTelemetrySignal(a)) == reviewDigest(stableTelemetrySignal(b)) {
		t.Fatal("new cumulative outcome was omitted")
	}
}

func TestSavedObservationFindingDeduplicatesAndRetainsOriginalUnknown(t *testing.T) {
	s, p, _, ctx, req, _ := coordinatorContinuationFixture(t)
	row := coordinatorReadyFixture(t, s, p, ctx, req)
	s.SetResearchAuthorizer(func(context.Context, management.ExecutionActorRef, bool) error { return nil })
	rows, err := s.readContinuations(p, "alice")
	if err != nil {
		t.Fatal(err)
	}
	rows.SavedRoute = &CoordinatorSavedRoute{Revision: 1, Route: coordinatorRoute(req.Run), MaxParallel: 1, UpdatedAt: time.Now().UTC().Format(time.RFC3339Nano), AutoDiscussFindings: true, Observations: &CoordinatorObservationRoute{Kinds: []string{"research"}, TelemetryIDs: []string{}, Remaining: 1, Last: map[string]string{"research": strings.Repeat("b", 64)}}}
	if err = s.saveContinuations(rows); err != nil {
		t.Fatal(err)
	}
	source := CoordinatorObservation{Kind: "research", Digest: strings.Repeat("a", 64), Revision: 1, Summary: "Selected saved research changed; propose a reviewed roadmap amendment."}
	changed, err := s.appendObservationFinding(ctx, p, row.ExecutionActor, 1, source, true)
	if err != nil || !changed {
		t.Fatal("new selected source not retained", err)
	}
	if changed, err = s.appendObservationFinding(ctx, p, row.ExecutionActor, 1, source, true); err != nil || changed {
		t.Fatal("same exact source replayed", err)
	}
	rows, err = s.readContinuations(p, "alice")
	if err != nil || len(rows.Findings) != 1 || rows.SavedRoute.Observations.Remaining != 0 {
		t.Fatal("budget/dedup not durable", err)
	}
	id := rows.Findings[0].ID
	if _, err = s.setCoordinatorFindingStatus(p, "alice", id, "pending", "reserved", ""); err != nil {
		t.Fatal(err)
	}
	reopened, err := New(Options{StateDir: s.stateDir, ProjectsDir: s.projectRoot()})
	if err != nil {
		t.Fatal(err)
	}
	defer reopened.Close()
	reopened.SetResearchAuthorizer(func(context.Context, management.ExecutionActorRef, bool) error { return nil })
	if err = reopened.settleInterruptedCoordinatorFindings(p, "alice"); err != nil {
		t.Fatal(err)
	}
	rows, err = reopened.readContinuations(p, "alice")
	if err != nil || rows.Findings[0].Status != "unknown" || rows.Findings[0].RequestKey == "" {
		t.Fatal("unknown original lost", err)
	}
	if changed, err = reopened.appendObservationFinding(ctx, p, row.ExecutionActor, 1, source, false); err != nil || changed {
		t.Fatal("explicit same-source request replayed unknown", err)
	}
}
func TestObservationRouteRejectsUnknownKindsAndFreshRevocation(t *testing.T) {
	route := &CoordinatorObservationRoute{Kinds: []string{"research"}, TelemetryIDs: []string{}, Remaining: 4, Last: map[string]string{}}
	if !validObservationRoute(route) {
		t.Fatal("closed route refused")
	}
	route.Kinds = []string{"research", "research"}
	if validObservationRoute(route) {
		t.Fatal("duplicate source accepted")
	}
	route.Kinds = []string{"logs"}
	if validObservationRoute(route) {
		t.Fatal("raw logs acquired model authority")
	}
	s, p, _, ctx, req, _ := coordinatorContinuationFixture(t)
	row := coordinatorReadyFixture(t, s, p, ctx, req)
	s.SetResearchAuthorizer(func(context.Context, management.ExecutionActorRef, bool) error { return errors.New("revoked") })
	source := CoordinatorObservation{Kind: "metrics", Digest: strings.Repeat("c", 64), Revision: 1, Summary: "Saved imported aggregate rows changed."}
	if changed, err := s.appendObservationFinding(ctx, p, row.ExecutionActor, 1, source, false); err == nil || changed {
		t.Fatal("revoked actor retained a new paid proposal")
	}
	rows, err := s.readContinuations(p, "alice")
	if err != nil || len(rows.Findings) != 0 {
		t.Fatal("refusal changed finding ledger", err)
	}
}
func TestObservationTransitionsRetainReversionIdentityAndManualBaseline(t *testing.T) {
	s, p, _, ctx, req, _ := coordinatorContinuationFixture(t)
	row := coordinatorReadyFixture(t, s, p, ctx, req)
	s.SetResearchAuthorizer(func(context.Context, management.ExecutionActorRef, bool) error { return nil })
	a := CoordinatorObservation{"research", strings.Repeat("a", 64), 1, "Saved state A"}
	b := CoordinatorObservation{"research", strings.Repeat("b", 64), 2, "Saved state B"}
	rows, err := s.readContinuations(p, "alice")
	if err != nil {
		t.Fatal(err)
	}
	rows.SavedRoute = &CoordinatorSavedRoute{Revision: 1, Route: coordinatorRoute(req.Run), MaxParallel: 1, UpdatedAt: time.Now().UTC().Format(time.RFC3339Nano), AutoDiscussFindings: true, Observations: &CoordinatorObservationRoute{Kinds: []string{"research"}, TelemetryIDs: []string{}, Remaining: 2, Last: map[string]string{"research": a.Digest}}}
	if err = s.saveContinuations(rows); err != nil {
		t.Fatal(err)
	}
	if changed, err := s.appendObservationFinding(ctx, p, row.ExecutionActor, 1, b, false); err != nil || !changed {
		t.Fatal("manual source not retained", err)
	}
	rows, err = s.readContinuations(p, "alice")
	if err != nil || rows.SavedRoute.Observations.Last["research"] != b.Digest || rows.SavedRoute.Observations.Remaining != 2 {
		t.Fatal("manual source failed to advance baseline without budget debit", err)
	}
	originalID := rows.Findings[0].ID
	if _, err = s.setCoordinatorFindingStatus(p, "alice", originalID, "pending", "reserved", ""); err != nil {
		t.Fatal(err)
	}
	if err = s.settleInterruptedCoordinatorFindings(p, "alice"); err != nil {
		t.Fatal(err)
	}
	// A retained original can coexist with an older baseline (e.g. interrupted
	// legacy bookkeeping). Repair only the baseline, not the model request.
	rows, err = s.readContinuations(p, "alice")
	if err != nil {
		t.Fatal(err)
	}
	rows.SavedRoute.Observations.Last["research"] = a.Digest
	if err = s.saveContinuations(rows); err != nil {
		t.Fatal(err)
	}
	if changed, err := s.appendObservationFinding(ctx, p, row.ExecutionActor, 1, b, true); err != nil || !changed {
		t.Fatal("dedupe did not settle baseline", err)
	}
	rows, err = s.readContinuations(p, "alice")
	if err != nil || len(rows.Findings) != 1 || rows.SavedRoute.Observations.Remaining != 2 || rows.SavedRoute.Observations.Last["research"] != b.Digest {
		t.Fatal("dedupe bought a second model turn", err)
	}
	for _, source := range []CoordinatorObservation{a, b} {
		if changed, err := s.appendObservationFinding(ctx, p, row.ExecutionActor, 1, source, true); err != nil || !changed {
			t.Fatal("real reversion was lost", err)
		}
	}
	rows, err = s.readContinuations(p, "alice")
	if err != nil || len(rows.Findings) != 3 || rows.SavedRoute.Observations.Remaining != 0 || rows.SavedRoute.ObservationSequence != 3 {
		t.Fatal("transition identity/budget not durable", err)
	}
	if rows.Findings[0].ID != originalID || rows.Findings[0].Status != "unknown" || rows.Findings[0].RequestKey == "" || rows.Findings[2].ID == originalID {
		t.Fatal("reversion replayed or overwrote original unknown")
	}
	if changed, err := s.appendObservationFinding(ctx, p, row.ExecutionActor, 1, b, false); err != nil || changed {
		t.Fatal("unchanged latest state replayed", err)
	}
}
func TestReadableClearedObservationSourcesAreExplicitNotUnavailable(t *testing.T) {
	s, p, _, ctx, req, _ := coordinatorContinuationFixture(t)
	row := coordinatorReadyFixture(t, s, p, ctx, req)
	s.SetResearchAuthorizer(func(context.Context, management.ExecutionActorRef, bool) error { return nil })
	sources, unavailable, _, err := s.projectObservationSources(ctx, p, row.ExecutionActor, []string{})
	if err != nil {
		t.Fatal(err)
	}
	for _, kind := range []string{"research", "feedback", "metrics"} {
		found := false
		for _, source := range sources {
			if source.Kind == kind {
				found = true
				if !hexDigest(source.Digest) || !strings.Contains(source.Summary, "empty") {
					t.Fatal("known empty source misrepresented", source)
				}
			}
		}
		for _, value := range unavailable {
			if value == kind {
				t.Fatal("readable empty source was unavailable", kind)
			}
		}
		if !found {
			t.Fatal("clearing the last source removed the change signal", kind)
		}
	}
	s.SetResearchAuthorizer(func(context.Context, management.ExecutionActorRef, bool) error {
		return management.ErrExecutionAuthority
	})
	if values, _, _, err := s.projectObservationSources(ctx, p, row.ExecutionActor, []string{}); err == nil || values != nil {
		t.Fatal("authority loss became a fabricated empty source")
	}
}
