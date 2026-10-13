package api

import (
	"strings"
	"testing"
	"time"

	"github.com/lcfrai/luci/control-plane/internal/backend/e2bbe"
)

func TestCloudCapacityAdviceNamesUnconfiguredServiceWithoutClaimingProviderSlots(t *testing.T) {
	view, err := (&API{}).cloudTaskCapacity(orchestratorExecutionActor(t), "p-one", "u-reviewed-task-fixture")
	if err != nil || view.Provider != "" || view.ConfiguredAvailableSlots != 0 || len(view.SetupNeeds) != 1 || !strings.Contains(view.SetupNeeds[0], "not configured") {
		t.Fatal("unconfigured service was presented as usable cloud capacity", view, err)
	}
}

func TestCloudCapacityAdviceKeepsHeldChargeAndUsesConfiguredResourceLimits(t *testing.T) {
	now := time.Date(2026, time.October, 13, 12, 0, 0, 0, time.UTC)
	row := func(user string, at time.Time, held bool, sandbox string, cost int64) e2bbe.Record {
		return e2bbe.Record{Request: e2bbe.Request{Identity: e2bbe.Identity{ProjectID: "p-one", RunID: "run-one", TaskID: "task-one", AttemptID: "attempt-one", OperationID: "op-" + sandbox}, BillingUserID: user, EstimatedCostMicros: cost}, CreatedAt: at, UpdatedAt: at, ResourcesHeld: held, SandboxID: sandbox}
	}
	limits := e2bbe.Limits{MaxConcurrent: 8, MaxPerProject: 6, MemoryMiB: 8192, CPUMillis: 8000}
	template := e2bbe.Template{MemoryMiB: 2048, CPUMillis: 2000}
	usage := e2bbe.Usage{Active: 2, MemoryMiB: 4096, CPUMillis: 4000}
	rows := []e2bbe.Record{
		row("alice", now.Add(-time.Hour), false, "retired-vm", 100),
		row("alice", now.AddDate(0, -1, 0), true, "still-held", 150),
		row("bob", now.Add(-time.Hour), true, "other-user", 250),
		row("alice", now.Add(-time.Hour), false, "", 300), // proved no Create
	}
	rows[2].Request.Identity.ProjectID = "p-other"
	view := cloudCapacityView(limits, template, usage, 1, rows, nil, map[string]bool{"op-still-held": true}, "alice", "p-one", "us-east-1", 200, 60, 1000, now)
	if view.ConfiguredAvailableSlots != 2 || view.EstimatedMonthMicros != 250 || view.MonthlyBudgetMicros != 1000 || view.EstimatedTaskCostMicros != 200 || view.Region != "us-east-1" ||
		view.RetainedProjectVMs != 1 || len(view.RetainedAttempts) != 1 || view.RetainedAttempts[0].OperationID != "op-still-held" || !view.RetainedAttempts[0].RecoveryPending || view.MoreRetainedAttempts != 0 || view.RetiredBillingUnconfirmed != 1 {
		t.Fatal("configured capacity or exact retained estimate was misreported", view)
	}
	blocked := cloudCapacityBlocked(view, "Current execution authority was revoked.")
	if blocked.ConfiguredAvailableSlots != 0 || len(blocked.SetupNeeds) != 1 || blocked.RetainedAttempts[0] != view.RetainedAttempts[0] || blocked.EstimatedMonthMicros != view.EstimatedMonthMicros || blocked.RetiredBillingUnconfirmed != view.RetiredBillingUnconfirmed {
		t.Fatal("revoked admission hid or released an owned VM or its estimate", blocked)
	}
	newer := row("alice", now.Add(time.Nanosecond), true, "newer-vm", 50)
	ordered := append(append([]e2bbe.Record{}, rows...), row("alice", now, true, "same-second", 50), newer)
	view = cloudCapacityView(limits, template, usage, 1, ordered, nil, nil, "alice", "p-one", "us-east-1", 200, 60, 1000, now)
	if view.RetainedProjectVMs != 3 || view.RetainedAttempts[0].OperationID != "op-newer-vm" {
		t.Fatal("newest exact held attempt was not first", view.RetainedAttempts)
	}
	rows = append(rows, row("", now.Add(-time.Hour), true, "legacy-held", 100))
	view = cloudCapacityView(limits, template, usage, 1, rows, nil, nil, "alice", "p-one", "us-east-1", 200, 60, 1000, now)
	if view.ConfiguredAvailableSlots != 0 || !strings.Contains(view.Limitation, "unavailable") || len(view.SetupNeeds) != 1 {
		t.Fatal("unattributed held allocation exposed apparent budget", view)
	}
	view = cloudCapacityView(limits, template, usage, 1, rows[:4], nil, nil, "alice", "p-one", "us-east-1", 800, 60, 1000, now)
	if view.ConfiguredAvailableSlots != 0 || len(view.SetupNeeds) != 1 {
		t.Fatal("new task cost exceeded this actor's retained monthly estimate", view)
	}
	settled := []e2bbe.SettledSpend{{OperationID: "op-archived", BillingUserID: "alice", CreatedAt: now, EstimatedCostMicros: 700, SandboxCreated: true}}
	view = cloudCapacityView(limits, template, usage, 1, rows[:4], settled, nil, "alice", "p-one", "us-east-1", 200, 60, 1000, now)
	if view.EstimatedMonthMicros != 950 || view.ConfiguredAvailableSlots != 0 || view.RetainedProjectVMs != 1 || view.RetiredBillingUnconfirmed != 2 {
		t.Fatal("archived spend was omitted or misreported as a live VM", view)
	}
}
