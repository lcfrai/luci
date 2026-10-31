package product

import (
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/lcfrai/luci/control-plane/internal/management"
	"github.com/lcfrai/luci/control-plane/internal/registry"
)

func TestOutcomeTaskClockRequiresExactOriginalTerminalFact(t *testing.T) {
	at := time.Date(2026, 10, 31, 1, 0, 0, 0, time.UTC)
	digest := strings.Repeat("a", 64)
	row := registry.Session{ID: "s-original", Status: registry.StatusExited, Command: "/usr/local/bin/node", Args: []string{"/usr/local/lib/luci/codex-task-result.cjs", digest, "--", "bounded task"}, CreatedAt: at, SandboxRef: "original-container", TaskTerminalImage: "sha256:" + strings.Repeat("b", 64), TaskTerminalCreatedAt: at.Add(time.Second), TaskTerminalStartedAt: at.Add(2 * time.Second)}
	if _, err := taskRuntimeTiming(row); err == nil {
		t.Fatal("generic exited state became measured execution")
	}
	row.TaskTerminalResult = &registry.TaskTerminalResult{Method: registry.TaskTerminalResultMethod, SessionCreatedAt: at, Ref: row.SandboxRef, Image: row.TaskTerminalImage, ContractDigest: digest, CreatedAt: row.TaskTerminalCreatedAt, StartedAt: row.TaskTerminalStartedAt, FinishedAt: at.Add(8 * time.Second), NormalZero: true, ContainerExitCode: 0}
	timing, err := taskRuntimeTiming(row)
	if err != nil || timing.ElapsedMilliseconds == nil || *timing.ElapsedMilliseconds != 6000 || timing.OriginalIdentityDigest != reviewDigest(row.TaskTerminalResult) {
		t.Fatal(timing, err)
	}
	row.SandboxRef = "replacement"
	if _, err = taskRuntimeTiming(row); err == nil {
		t.Fatal("replacement original identity was measured")
	}
}

func TestOutcomeControlActionsOnlyAppendOnSuccessfulCASMutation(t *testing.T) {
	store, id, path, input := projectRunStoreFixture(t)
	run := createStoredRun(t, store, id, path, input)
	action := runHumanAction{ActorID: "alice", Action: "cancel", RequestDigest: strings.Repeat("c", 64)}
	changed, err := store.Cancel(id, path, run.ID, run.Revision, action)
	if err != nil || changed.Measurements == nil || len(changed.Measurements.HumanActions) != 1 {
		t.Fatal(changed, err)
	}
	replay, err := store.Cancel(id, path, changed.ID, changed.Revision, action)
	if err != nil || replay.Revision != changed.Revision || len(replay.Measurements.HumanActions) != 1 {
		t.Fatal("no-op duplicated an intervention", replay, err)
	}
	if _, err = store.Cancel(id, path, run.ID, run.Revision, action); !errors.Is(err, ErrProjectRunConflict) {
		t.Fatal("stale control bypassed CAS", err)
	}
}

func TestOutcomeLegacyUnknownTimingIsNotZeroOrProviderSpend(t *testing.T) {
	store, id, path, input := projectRunStoreFixture(t)
	run := createStoredRun(t, store, id, path, input)
	run, _, _, err := store.ReserveOperation(id, path, run.ID, run.Revision, "outcome-reserve-123456789", "session", strings.Repeat("d", 64))
	if err != nil {
		t.Fatal(err)
	}
	run.Measurements = nil
	view := summarizeRunMeasurements([]ProjectRun{run})
	if view.TaskRuntimeMilliseconds != nil || view.UnknownTaskIntervals != 1 || view.RecordedHumanActions != nil || view.RunsWithEarlierUninstrumentedActions != 1 || view.InputTokens != nil || view.OutputTokens != nil {
		t.Fatal(view)
	}
}

func TestOutcomeBillTotalsKeepCurrenciesSeparateAndUnknownUsage(t *testing.T) {
	ledger := outcomeCostLedger{Rows: []OutcomeCostReceipt{{OutcomeCostInput: OutcomeCostInput{Currency: "USD", AmountMicros: "1234567"}}, {OutcomeCostInput: OutcomeCostInput{Currency: "AUD", AmountMicros: "7654321"}}, {OutcomeCostInput: OutcomeCostInput{Currency: "USD", AmountMicros: "1"}}}}
	view := outcomeCostView(management.ExecutionActorRef{UserID: "alice", ProjectID: "p-one"}, ledger)
	if len(view.Totals) != 2 || view.Totals[0].Currency != "AUD" || view.Totals[0].AmountMicros != "7654321" || view.Totals[1].Currency != "USD" || view.Totals[1].AmountMicros != "1234568" || view.Totals[1].ReceiptRows != 2 || view.ProviderVerified || view.InputTokens != nil || view.OutputTokens != nil {
		t.Fatal(view)
	}
}
