package api

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/lcfrai/luci/control-plane/internal/backend/e2bbe"
	"github.com/lcfrai/luci/control-plane/internal/management"
)

func placementFixture(id, host, user, team string) cloudPlacement {
	actor := management.ExecutionActorRef{UserID: user, TeamID: team, ProjectID: "project-one"}
	request := e2bbe.Request{Identity: e2bbe.Identity{OwnerID: "workspace-owner", ProjectID: actor.ProjectID, RunID: "run-" + strings.Repeat("a", 24), TaskID: "build", AttemptID: "attempt-" + id, OperationID: "child-" + id}, BillingUserID: user, BillingTeamID: team, SourceDigest: strings.Repeat("a", 64), ContextDigest: strings.Repeat("b", 64), BundleDigest: strings.Repeat("c", 64), MaxSeconds: 60, EstimatedCostMicros: 100}
	return cloudPlacement{QueueID: "queue-" + id, Actor: actor, Request: request, RequestDigest: cloudPlacementDigest(request), RunInputDigest: strings.Repeat("d", 64), HostID: host, HostFingerprint: strings.Repeat("e", 64), Template: e2bbe.Template{ID: "template", BuildID: "build", BuildDigest: strings.Repeat("f", 64), MemoryMiB: 512, CPUMillis: 1000}, CreatedAt: time.Now().UTC(), Admitted: true}
}
func TestCloudPoolCountsOriginalReservationAndMaterializedVMExactlyOnce(t *testing.T) {
	a, b := placementFixture("a", "host-a", "alice", "team-one"), placementFixture("b", "host-b", "bob", "team-one")
	record := e2bbe.Record{Request: a.Request, RequestDigest: a.RequestDigest, Template: a.Template, State: e2bbe.Allocated, ResourcesHeld: true, SandboxID: "vm-a", CreatedAt: a.CreatedAt}
	usage := cloudAggregateUsage(map[string]cloudPlacement{a.Request.Identity.OperationID: a, b.Request.Identity.OperationID: b}, []cloudHostSnapshot{{HostID: "host-a", Records: []e2bbe.Record{record}}}, a.Actor, time.Now().UTC())
	if usage.Unknown || usage.Active != 2 || usage.ProjectActive != 2 || usage.MemoryMiB != 1024 || usage.CPUMillis != 2000 || usage.ActorMonthMicros != 200 || usage.EstimateMicros != 200 {
		t.Fatalf("pool doubled a materialized original or lost pending/team charges: %+v", usage)
	}
	changed := record
	changed.RequestDigest = strings.Repeat("0", 64)
	if !cloudAggregateUsage(map[string]cloudPlacement{a.Request.Identity.OperationID: a}, []cloudHostSnapshot{{HostID: "host-a", Records: []e2bbe.Record{changed}}}, a.Actor, time.Now()).Unknown {
		t.Fatal("changed original was treated as a healthy matching reservation")
	}
}
func TestCloudPoolRetainsLostHostChargesAndDistinguishesUnadmittedIntent(t *testing.T) {
	original := placementFixture("a", "missing-host", "alice", "")
	pending := placementFixture("b", "host-b", "alice", "")
	pending.Admitted = false
	rows := map[string]cloudPlacement{original.Request.Identity.OperationID: original, pending.Request.Identity.OperationID: pending}
	usage := cloudAggregateUsage(rows, nil, original.Actor, time.Now())
	if usage.Active != 1 || usage.ActorMonthMicros != 100 || usage.EstimateMicros != 100 {
		t.Fatal("missing host released its owned charge or an unadmitted intent invented a VM", usage)
	}
	original.Retired = true
	rows[original.Request.Identity.OperationID] = original
	usage = cloudAggregateUsage(rows, nil, original.Actor, time.Now())
	if usage.Active != 0 || usage.ActorMonthMicros != 100 || usage.EstimateMicros != 100 {
		t.Fatal("positive VM retirement was mistaken for a refunded bill", usage)
	}
	original.NoProviderEffect = true
	rows[original.Request.Identity.OperationID] = original
	usage = cloudAggregateUsage(rows, nil, original.Actor, time.Now())
	if usage.Active != 0 || usage.ActorMonthMicros != 0 || usage.EstimateMicros != 0 {
		t.Fatal("positively unmaterialized allocation remained charged as provider spend", usage)
	}
}
func TestCloudPoolLegacyAndCompactedSpendRemainIncluded(t *testing.T) {
	row := placementFixture("legacy", "legacy-host", "alice", "")
	record := e2bbe.Record{Request: row.Request, RequestDigest: row.RequestDigest, Template: row.Template, CreatedAt: row.CreatedAt, State: e2bbe.Unknown, ResourcesHeld: true}
	snapshots := []cloudHostSnapshot{{HostID: "legacy-host", Records: []e2bbe.Record{record}, Settled: []e2bbe.SettledSpend{{OperationID: "older", RequestDigest: strings.Repeat("f", 64), BillingUserID: "alice", CreatedAt: time.Now(), EstimatedCostMicros: 50, SandboxCreated: true}}}}
	usage := cloudAggregateUsage(map[string]cloudPlacement{}, snapshots, row.Actor, time.Now())
	if usage.Unknown || usage.Active != 1 || usage.ActorMonthMicros != 150 || usage.EstimateMicros != 150 {
		t.Fatal("pool reset legacy original or settled costs", usage)
	}
	actor := row.Actor
	actor.TeamID = "team-one"
	if !cloudAggregateUsage(map[string]cloudPlacement{}, snapshots, actor, time.Now()).Unknown {
		t.Fatal("legacy tombstone silently acquired an invented team attribution")
	}
}
func TestCloudPoolAdmissionAndFinalEffectUseOneAggregateCeiling(t *testing.T) {
	first, second := placementFixture("a", "host-a", "alice", "team-one"), placementFixture("b", "host-b", "bob", "team-one")
	usage := cloudAggregateUsage(map[string]cloudPlacement{first.Request.Identity.OperationID: first, second.Request.Identity.OperationID: second}, nil, first.Actor, time.Now())
	limits := e2bbe.Limits{MaxConcurrent: 2, MaxPerProject: 2, MemoryMiB: 1024, CPUMillis: 2000, EstimatedCostMicros: 200, MaxTaskSeconds: 60}
	if !cloudPoolAllows(usage, limits, 200, 60) {
		t.Fatal("exactly reserved shared capacity refused")
	}
	for _, test := range []struct {
		name    string
		limit   e2bbe.Limits
		budget  int64
		seconds int
	}{
		{"global", e2bbe.Limits{MaxConcurrent: 1, MaxPerProject: 2, MemoryMiB: 1024, CPUMillis: 2000, EstimatedCostMicros: 200, MaxTaskSeconds: 60}, 200, 60},
		{"project", e2bbe.Limits{MaxConcurrent: 2, MaxPerProject: 1, MemoryMiB: 1024, CPUMillis: 2000, EstimatedCostMicros: 200, MaxTaskSeconds: 60}, 200, 60},
		{"memory", e2bbe.Limits{MaxConcurrent: 2, MaxPerProject: 2, MemoryMiB: 512, CPUMillis: 2000, EstimatedCostMicros: 200, MaxTaskSeconds: 60}, 200, 60},
		{"cpu", e2bbe.Limits{MaxConcurrent: 2, MaxPerProject: 2, MemoryMiB: 1024, CPUMillis: 1000, EstimatedCostMicros: 200, MaxTaskSeconds: 60}, 200, 60},
		{"pool-spend", e2bbe.Limits{MaxConcurrent: 2, MaxPerProject: 2, MemoryMiB: 1024, CPUMillis: 2000, EstimatedCostMicros: 100, MaxTaskSeconds: 60}, 200, 60},
		{"actor-team-spend", limits, 100, 60}, {"deadline", limits, 200, 61},
	} {
		t.Run(test.name, func(t *testing.T) {
			if cloudPoolAllows(usage, test.limit, test.budget, test.seconds) {
				t.Fatal("aggregate admission escaped a current ceiling")
			}
		})
	}
	usage.Unknown = true
	if cloudPoolAllows(usage, limits, 200, 60) {
		t.Fatal("unknown host evidence authorized a new allocation")
	}
}
func TestCloudPlacementPrivateStoreSurvivesRestartAndHoldsMissingOriginalHost(t *testing.T) {
	dir := filepath.Join(t.TempDir(), "pool")
	store, err := openCloudPlacementStore(dir, "workspace-one")
	if err != nil {
		t.Fatal(err)
	}
	journal, err := store.read()
	if err != nil {
		t.Fatal(err)
	}
	cutover := journal.CreatedAt
	row := placementFixture("a", "host-a", "alice", "")
	journal.Rows[row.Request.Identity.OperationID] = row
	if err = store.write(journal); err != nil {
		t.Fatal(err)
	}
	if err = store.Close(); err != nil {
		t.Fatal(err)
	}
	reopened, err := openCloudPlacementStore(dir, "workspace-one")
	if err != nil {
		t.Fatal(err)
	}
	next, err := reopened.read()
	if err != nil || !next.CreatedAt.Equal(cutover) || cloudPlacementDigest(next.Rows[row.Request.Identity.OperationID]) != cloudPlacementDigest(row) {
		t.Fatal("restart lost exact original/cutover", err)
	}
	pool := &CloudTaskPool{store: reopened, hosts: map[string]CloudTaskHost{}}
	if _, err = pool.runtime(row); !errors.Is(err, e2bbe.ErrUnknown) {
		t.Fatal("missing original host was replaced")
	}
	if known, err := pool.neverMaterialized("project-one", row.Request.Identity.RunID, row.Request.Identity.AttemptID, row.Request.Identity.OperationID, time.Now().Format(time.RFC3339Nano), true); known || err == nil {
		t.Fatal("missing host inferred original absence")
	}
	if err = reopened.Close(); err != nil {
		t.Fatal(err)
	}
	if err = os.Remove(filepath.Join(dir, "placements.json")); err != nil {
		t.Fatal(err)
	}
	if _, err = openCloudPlacementStore(dir, "workspace-one"); err == nil {
		t.Fatal("missing initialized ledger silently reset all original ownership")
	}
}
