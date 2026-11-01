package api

import (
	"math"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/lcfrai/luci/control-plane/internal/management"
)

func elasticFixture() (CloudElasticPolicy, AzureElasticOperator, AzureElasticHost) {
	now := time.Now().UTC()
	actor := management.ExecutionActorRef{UserID: "alice", ProjectID: "project-one"}
	op := AzureElasticOperator{ParentHostID: "primary", ProfileDigest: strings.Repeat("a", 64), BootstrapSeconds: 600, MinHostSeconds: 900, MaxHosts: 3, HostCostMicrosPerHour: 3_600_000, MaxHostSeconds: 7200, BudgetMicros: 50_000_000}
	policy := CloudElasticPolicy{ID: elasticPolicyKey(actor), Actor: actor, Harness: "codex", Revision: 1, ProfileDigest: op.ProfileDigest, CatalogueDigest: strings.Repeat("b", 64), MaxHosts: 2, HostSeconds: 1800, IdleSeconds: 60, BudgetMicros: 10_000_000, ExpiresAt: now.Add(4 * time.Hour), Enabled: true}
	h := AzureElasticHost{ID: "host-" + strings.Repeat("c", 32), PolicyID: policy.ID, Actor: actor, PolicySnapshot: policy, OperatorSnapshot: op, OperatorDigest: op.ProfileDigest, OwnershipMarker: strings.Repeat("d", 32), CreatedAt: now, ExpiresAt: now.Add(1800 * time.Second), EstimateMicros: 1_800_000, State: "reserved"}
	return policy, op, h
}

func TestElasticHostReservesCostBeforeCreateAndRetainsUnknownAndSettledEstimates(t *testing.T) {
	policy, _, h := elasticFixture()
	pool := &CloudTaskPool{}
	j := cloudPlacementJournal{Elastic: &cloudElasticLedger{Policies: map[string]CloudElasticPolicy{policy.ID: policy}, Hosts: map[string]AzureElasticHost{h.ID: h}}}
	u := pool.aggregateUsage(j, nil, h.Actor, h.CreatedAt)
	if u.EstimateMicros != h.EstimateMicros || u.ActorMonthMicros != h.EstimateMicros || u.Unknown {
		t.Fatal("reserved capacity was not charged before the first write", u)
	}
	h.NetworkAttempted = true
	j.Elastic.Hosts[h.ID] = h
	u = pool.aggregateUsage(j, nil, h.Actor, h.ExpiresAt.Add(time.Second))
	if !u.Unknown || u.EstimateMicros != h.EstimateMicros {
		t.Fatal("expired unknown original released its reservation", u)
	}
	h.DeleteAttempted = true
	h.Retired = true
	j.Elastic.Hosts[h.ID] = h
	u = pool.aggregateUsage(j, nil, h.Actor, h.ExpiresAt.Add(time.Second))
	if u.Unknown || u.EstimateMicros != h.EstimateMicros {
		t.Fatal("settled physical absence erased retained estimates", u)
	}
	h.NetworkRejected = true
	j.Elastic.Hosts[h.ID] = h
	u = pool.aggregateUsage(j, nil, h.Actor, h.ExpiresAt)
	if u.EstimateMicros != 0 || u.ActorMonthMicros != 0 {
		t.Fatal("proven rejected no-effect request stayed billed", u)
	}
}

func TestElasticHostCostRejectsOverflowAndRoundsUp(t *testing.T) {
	if _, ok := elasticHostCost(2, math.MaxInt64); ok {
		t.Fatal("overflowed host budget was admitted")
	}
	if got, ok := elasticHostCost(1, 1); !ok || got != 1 {
		t.Fatal("fractional micro cost rounded down", got, ok)
	}
	if got, ok := elasticHostCost(3600, 1_234_567); !ok || got != 1_234_567 {
		t.Fatal("exact hourly estimate changed", got, ok)
	}
}

func TestElasticPrivateRestartKeepsOriginalReviewAndRejectsCorruptedReview(t *testing.T) {
	policy, _, h := elasticFixture()
	store, err := openCloudPlacementStore(filepath.Join(t.TempDir(), "pool"), "workspace-one")
	if err != nil {
		t.Fatal(err)
	}
	j, err := store.read()
	if err != nil {
		t.Fatal(err)
	}
	review := cloudElasticReview{ReviewID: "capacity-" + strings.Repeat("e", 32), Policy: policy, Digest: cloudPlacementDigest(policy)}
	j.Elastic = &cloudElasticLedger{Policies: map[string]CloudElasticPolicy{policy.ID: policy}, Hosts: map[string]AzureElasticHost{h.ID: h}, Reviews: map[string]cloudElasticReview{review.ReviewID: review}}
	if err = store.write(j); err != nil {
		t.Fatal(err)
	}
	dir := store.dir
	if err = store.Close(); err != nil {
		t.Fatal(err)
	}
	store, err = openCloudPlacementStore(dir, "workspace-one")
	if err != nil {
		t.Fatal(err)
	}
	defer store.Close()
	restored, err := store.read()
	if err != nil || cloudPlacementDigest(restored.Elastic.Hosts[h.ID]) != cloudPlacementDigest(h) {
		t.Fatal("restart lost original ownership/billing window", err)
	}
	bad := review
	bad.Policy.HostSeconds = 0
	restored.Elastic.Reviews[bad.ReviewID] = bad
	if store.write(restored) == nil {
		t.Fatal("corrupted retained review was accepted")
	}
}

func TestElasticRetainedReviewCannotExceedCurrentOperatorOrChangeActor(t *testing.T) {
	policy, op, _ := elasticFixture()
	if !elasticPolicyFits(policy, op) {
		t.Fatal("valid reviewed policy refused")
	}
	policy.MaxHosts = op.MaxHosts + 1
	if elasticPolicyFits(policy, op) {
		t.Fatal("stored review bypassed current operator host ceiling")
	}
	policy, op, _ = elasticFixture()
	op.HostCostMicrosPerHour++
	op.ProfileDigest = strings.Repeat("f", 64)
	if elasticPolicyFits(policy, op) {
		t.Fatal("changed operator price retained old capacity authority")
	}
	policy, _, _ = elasticFixture()
	policy.Actor.UserID = "mallory"
	if validElasticPolicy(policy) {
		t.Fatal("actor substitution retained another original's key")
	}
}
