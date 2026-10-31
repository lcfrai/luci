package product

import (
	"context"
	"github.com/lcfrai/luci/control-plane/internal/management"
	"github.com/lcfrai/luci/control-plane/internal/previewruntime"
	"strings"
	"testing"
	"time"
)

func recordedImageBatch(t *testing.T) (*Server, context.Context, RuntimePreparation) {
	t.Helper()
	s, ctx, actor, row := runtimePreparationFixture(t)
	file, private, err := s.readRuntimePreparation(actor, row.Plan.ID)
	if err != nil {
		t.Fatal(err)
	}
	source := RuntimeBuildAttempt{ServiceID: "app", RunID: "run-original", CheckID: "check-original", Commit: strings.Repeat("a", 40), SourceDigest: strings.Repeat("b", 64), RunInputDigest: strings.Repeat("c", 64), CheckDigest: strings.Repeat("d", 64), Attempted: true}
	private.Builds = []RuntimeBuildAttempt{source}
	private.Execution = &RuntimePreparationExecution{RunID: source.RunID, CheckID: source.CheckID, Commit: source.Commit, SourceDigest: source.SourceDigest, RunInputDigest: source.RunInputDigest, CheckDigest: source.CheckDigest, StartedAt: time.Now().UTC().Format(time.RFC3339Nano), State: "running"}
	private.Revision++
	if err = durableRunJSON(file, private); err != nil {
		t.Fatal(err)
	}
	return s, ctx, private.RuntimePreparation
}
func TestRuntimeImageBatchRestartCannotReplayReservedService(t *testing.T) {
	s, ctx, row := recordedImageBatch(t)
	calls := 0
	s.SetPreviewRuntime(previewruntime.Runtime{BuildAndPublish: func(context.Context, previewruntime.Request, previewruntime.Publication, func(previewruntime.Outcome) error) (previewruntime.Outcome, error) {
		calls++
		return previewruntime.Outcome{}, nil
	}})
	request := RuntimePreparationStart{ExpectedRevision: row.Revision - 1, PreparationDigest: row.Plan.Digest, RunID: row.Execution.RunID, CheckID: row.Execution.CheckID}
	replay, err := s.StartRuntimePreparation(ctx, row.Plan.Actor, row.Plan.ID, request)
	if err != nil || replay.Revision != row.Revision || calls != 0 {
		t.Fatal("lost acknowledgement replayed image build", err, calls)
	}
	if _, err = s.ObserveRuntimePreparation(ctx, row.Plan.Actor, row.Plan.ID); err == nil || calls != 0 {
		t.Fatal("unknown original acquired replacement", err, calls)
	}
	request.CheckID = "different-check"
	if _, err = s.StartRuntimePreparation(ctx, row.Plan.Actor, row.Plan.ID, request); err == nil || calls != 0 {
		t.Fatal("batch adopted different checked source", err, calls)
	}
}
func TestRuntimeImageBatchRejectsUncheckedWorkBeforeOriginalReservation(t *testing.T) {
	s, ctx, actor, row := runtimePreparationFixture(t)
	if _, err := s.StartRuntimePreparation(ctx, actor, row.Plan.ID, RuntimePreparationStart{ExpectedRevision: row.Revision, PreparationDigest: row.Plan.Digest, RunID: "missing-run", CheckID: "missing-check"}); err == nil {
		t.Fatal("unverified work admitted")
	}
	_, after, err := s.readRuntimePreparation(actor, row.Plan.ID)
	if err != nil || after.Execution != nil || len(after.Builds) != 0 || after.Revision != row.Revision {
		t.Fatal("unchecked source reserved a build", err)
	}
}
func TestRuntimeImageBatchRequiresEveryExactServiceAndPositiveFinalization(t *testing.T) {
	_, _, row := recordedImageBatch(t)
	if !validRuntimePreparationExecution(row) {
		t.Fatal("original batch rejected")
	}
	bad := row
	bad.Builds = nil
	if validRuntimePreparationExecution(bad) {
		t.Fatal("missing service admitted")
	}
	bad = row
	bad.Builds = append([]RuntimeBuildAttempt{}, row.Builds...)
	bad.Builds[0].Commit = strings.Repeat("e", 40)
	if validRuntimePreparationExecution(bad) {
		t.Fatal("mixed checked commits admitted")
	}
	bad = row
	execution := *row.Execution
	bad.Execution = &execution
	execution.State = "complete"
	execution.FinishedAt = time.Now().UTC().Format(time.RFC3339Nano)
	if validRuntimePreparationExecution(bad) {
		t.Fatal("completion without saved blueprint")
	}
	bad.SavedBlueprintRevision = bad.Plan.BlueprintRevision + 1
	if !validRuntimePreparationExecution(bad) {
		t.Fatal("exact finalization rejected")
	}
}
func TestRuntimeImagePermitIsConsumedEvenWhenOriginalExecutionCannotProceed(t *testing.T) {
	s, ctx, row := recordedImageBatch(t)
	permit := &runtimeImagePermit{planDigest: row.Plan.Digest, source: row.Builds[0]}
	permit.used.Store(true)
	if _, err := s.executeRuntimePreparationService(ctx, row.Plan.Actor, row.Plan.ID, "app", row.Execution.RunID, row.Execution.CheckID, permit); err == nil {
		t.Fatal("consumed original permit executed")
	}
}

func TestRuntimeImageBatchActiveReadRechecksCurrentAuthorityBeforeResponse(t *testing.T) {
	s, ctx, row := recordedImageBatch(t)
	s.coordinatorActions = map[string]bool{"preparation-" + row.Plan.ID: true}
	original := s.coordinatorDeploymentAuthorize
	calls := 0
	s.coordinatorDeploymentAuthorize = func(c context.Context, actor management.ExecutionActorRef, allocate bool) (management.ExecutionAuthority, error) {
		calls++
		if calls == 3 {
			return management.ExecutionAuthority{}, management.ErrExecutionAuthority
		}
		return original(c, actor, allocate)
	}
	got, err := s.ObserveRuntimePreparation(ctx, row.Plan.Actor, row.Plan.ID)
	if err == nil || got.Plan.ID != "" || calls != 3 {
		t.Fatal("revoked post-read authority leaked private batch", err, calls)
	}
}
