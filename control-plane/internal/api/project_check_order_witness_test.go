// Controlled production HTTP/ledger regression, using controlled backend replies.
// It adds a separately charged workspace to the original ordered recovery test;
// it does not establish installed Docker, native UI or real-provider acceptance.
package api

import (
	"bytes"
	"context"
	"os"
	"path/filepath"
	"reflect"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/lcfrai/luci/control-plane/internal/backend"
	"github.com/lcfrai/luci/control-plane/internal/management"
	"github.com/lcfrai/luci/control-plane/internal/product"
)

type orderedRecoveryWitnessExecutor struct {
	*httpCheckExecutor
	entered chan backend.CheckIdentity
}

func (b *orderedRecoveryWitnessExecutor) CreateCheck(ctx context.Context, prepared backend.PreparedCheck) (string, error) {
	// Production has already persisted creating before invoking this boundary.
	select {
	case b.entered <- prepared.Spec.Identity:
	case <-ctx.Done():
		return "", ctx.Err()
	}
	return b.httpCheckExecutor.CreateCheck(ctx, prepared)
}

func TestProjectCheckOriginalReviewPreservesSeparatePendingWorkspace(t *testing.T) {
	left, right := newSwarmSessionFixtureTasks(t), newSwarmSessionFixtureTasks(t)
	shared := filepath.Join(t.TempDir(), "shared-capacity")
	leftBase, rightBase := newHTTPCheckExecutor(left.env.be), newHTTPCheckExecutor(right.env.be)
	leftBase.createErr = backend.ErrUnavailable
	wireHTTPCheckFixture(t, left, leftBase, shared, "user-recovery", 12)
	wireHTTPCheckFixture(t, right, rightBase, shared, "user-witness", 12)
	if left.env.api.deps.ObserveTaskCompletions || right.env.api.deps.ObserveTaskCompletions {
		t.Fatal("ordered fixture must not start an unobserved periodic ticker")
	}
	if left.env.api.execution.scope == right.env.api.execution.scope {
		t.Fatal("witness must have an independent workspace scope")
	}
	leftFence := &httpCheckFenceExecutor{httpCheckExecutor: leftBase}
	left.env.api.deps.Backend = leftFence
	var denied atomic.Bool
	left.env.api.deps.AuthorizeExecutionControl = func(_ context.Context, actor management.ExecutionActorRef) error {
		if denied.Load() || actor.UserID != "user-recovery" || actor.ProjectID != left.owner.ID {
			return management.ErrExecutionAuthority
		}
		return nil
	}
	started := checkHTTPStart(t, left, "user-recovery")
	target := checkHTTPWait(t, left, "user-recovery", started.ID, func(row product.ProjectCheckRun) bool {
		return row.Status == "unknown" && row.ReasonCode == "check_create_unconfirmed"
	})
	checkHTTPIdle(t, left, target)
	// Neither GET invokes the observer; this precedes the first explicit tick.
	target = checkHTTPRead(t, left, "user-recovery", target.ID)
	originalReview := checkHTTPRecovery(t, left, "user-recovery", target.ID)
	if originalReview.State != "available" || originalReview.RecoveryDigest == nil || originalReview.Method == nil || *originalReview.Method != "fence-launch" {
		t.Fatal("original exact unknown-create review is unavailable")
	}
	release := make(chan struct{})
	var releaseOnce sync.Once
	defer releaseOnce.Do(func() { close(release) })
	rightBase.createGate = release
	rightWitness := &orderedRecoveryWitnessExecutor{httpCheckExecutor: rightBase, entered: make(chan backend.CheckIdentity, 1)}
	right.env.api.deps.Backend = rightWitness
	witnessStarted := checkHTTPStart(t, right, "user-witness")
	var witnessIdentity backend.CheckIdentity
	select {
	case witnessIdentity = <-rightWitness.entered:
	case <-time.After(10 * time.Second):
		t.Fatal("bounded witness did not reach its real controlled Create boundary")
	}
	witness := checkHTTPRead(t, right, "user-witness", witnessStarted.ID)
	if witnessIdentity.CheckID != witness.ID || witnessIdentity.ProjectID != right.owner.ID || witness.ResourcesHeld == nil || !*witness.ResourcesHeld {
		t.Fatal("unrelated witness is not an exact charged check")
	}
	before, err := left.env.api.execution.readFenceLedger()
	if err != nil || len(before.Checks) != 2 || len(before.CheckFences) != 0 {
		t.Fatal("both HTTP-created reservations must exist without seeded ledger data", err)
	}
	find := func(rows []checkExecutionLease, id string) checkExecutionLease {
		t.Helper()
		for _, row := range rows {
			if row.ID == id {
				return row
			}
		}
		t.Fatalf("missing exact lease %s", id)
		return checkExecutionLease{}
	}
	targetLease := find(before.Checks, target.Commands[0].AllocationID)
	witnessLease := find(before.Checks, witnessIdentity.AllocationID)
	if targetLease.State != "creating" || witnessLease.State != "creating" || targetLease.Ref != "" || witnessLease.Ref != "" || witnessLease.Identity != witnessIdentity {
		t.Fatal("unknown target and pending witness are not separately bound")
	}
	for _, lease := range []checkExecutionLease{targetLease, witnessLease} {
		if lease.Resources.MemoryMiB != 1024 || lease.Resources.CPUMillis != 1000 || lease.CreateDiskBytes == 0 {
			t.Fatal("each real pending claim must retain its full requested budget")
		}
	}
	targetHistory := filepath.Join(left.options.StateDir, "project-checks", target.ProjectID+".json")
	witnessHistory := filepath.Join(right.options.StateDir, "project-checks", witness.ProjectID+".json")
	read := func(path string) []byte {
		t.Helper()
		value, e := os.ReadFile(path)
		if e != nil {
			t.Fatal(e)
		}
		return value
	}
	targetBytes, witnessBytes := read(targetHistory), read(witnessHistory)
	for tick := 0; tick < 4; tick++ {
		if tick == 2 {
			left.reopen(t)
		}
		if left.env.api.deps.ObserveTaskCompletions {
			t.Fatal("reopen enabled an uncontrolled observer")
		}
		if err = left.env.api.ObserveChecksOnce(context.Background()); err != nil {
			t.Fatal(err)
		}
		current, e := left.env.api.execution.readFenceLedger()
		if e != nil || !reflect.DeepEqual(current, before) {
			t.Fatal("observation mutated either charged lease", e)
		}
		if !reflect.DeepEqual(checkHTTPRecovery(t, left, "user-recovery", target.ID), originalReview) {
			t.Fatal("first observation or reopen invalidated unchanged review")
		}
		if !bytes.Equal(read(targetHistory), targetBytes) || !bytes.Equal(read(witnessHistory), witnessBytes) {
			t.Fatal("observation changed exact retained history bytes")
		}
	}
	denied.Store(true)
	checkHTTPRecover(t, left, "user-recovery", target.ID, originalReview, 403)
	denied.Store(false)
	unchanged, err := left.env.api.execution.readFenceLedger()
	if err != nil || !reflect.DeepEqual(unchanged, before) {
		t.Fatal("revoked recovery changed retained ownership", err)
	}
	retired := checkHTTPRecover(t, left, "user-recovery", target.ID, originalReview, 200)
	if retired.Status != "cancelled" || retired.EvidenceStatus != "unverified" || retired.ResourcesHeld == nil || *retired.ResourcesHeld || !retired.Commands[0].CleanupConfirmed || retired.Commands[0].ExitCode != nil || retired.Commands[0].Output != target.Commands[0].Output {
		t.Fatal("recovery invented success or failed to settle only its target")
	}
	after, err := left.env.api.execution.readFenceLedger()
	if err != nil || len(after.Checks) != 1 || !reflect.DeepEqual(after.Checks[0], witnessLease) || !reflect.DeepEqual(after.Leases, before.Leases) || len(after.CheckFences) != 1 || after.CheckFences[0].Identity != targetLease.Identity || after.CheckFences[0].State != "retired" {
		t.Fatal("recovery changed the peer charge or failed to retain exact fence proof", err)
	}
	if !reflect.DeepEqual(checkHTTPRead(t, right, "user-witness", witness.ID), witness) || !bytes.Equal(read(witnessHistory), witnessBytes) {
		t.Fatal("recovery changed unrelated pending history")
	}
	_, leftCreates, leftStarts, leftLogs, leftRemoves := leftBase.counts()
	if leftCreates != 1 || leftStarts+leftLogs+leftRemoves != 0 {
		t.Fatal("target was replayed or invented execution")
	}
	_, rightCreates, rightStarts, _, _ := rightBase.counts()
	// The entered channel is sent immediately before the embedded adapter's
	// counter increments, so do not impose an unsupported scheduling order.
	if rightCreates > 1 || rightStarts != 0 {
		t.Fatal("witness must still be its single pending Create")
	}
	releaseOnce.Do(func() { close(release) })
	done := checkHTTPWait(t, right, "user-witness", witness.ID, func(row product.ProjectCheckRun) bool {
		return row.Status == "passed" && row.ResourcesHeld != nil && !*row.ResourcesHeld
	})
	checkHTTPIdle(t, right, done)
	checkHTTPCompletionAcknowledged(t, right, done)
	if done.Commands[0].ExitCode == nil || *done.Commands[0].ExitCode != 0 || done.Commands[0].Output != rightBase.output || !done.Commands[0].CleanupConfirmed {
		t.Fatal("witness did not retain its own controlled result")
	}
	final, err := left.env.api.execution.readFenceLedger()
	if err != nil || len(final.Checks) != 0 || !reflect.DeepEqual(final.CheckFences, after.CheckFences) {
		t.Fatal("final witness settlement lost target fence or retained a charge", err)
	}
	_, rightCreates, rightStarts, rightLogs, rightRemoves := rightBase.counts()
	if rightCreates != 1 || rightStarts != 1 || rightLogs != 1 || rightRemoves != 1 {
		t.Fatal("witness must run once after one barrier release")
	}
}
