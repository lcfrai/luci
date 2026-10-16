package product

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os/exec"
	"strings"
	"testing"

	"github.com/lcfrai/luci/control-plane/internal/registry"
)

func TestSwarmIntegrationDiagnosticPreservesLegacyJSON(t *testing.T) {
	for _, state := range []string{"integrating", "integrated", "unknown"} {
		in := ProjectSwarmIntegration{State: state, SourceCommit: "source", TargetCommit: "target", ReviewDigest: "review"}
		data, err := json.Marshal(in)
		want := `{"state":"` + state + `","sourceCommit":"source","targetCommit":"target","reviewDigest":"review"}`
		if err != nil || string(data) != want || !validSwarmIntegrationDiagnostic(in) {
			t.Fatal("legacy integration JSON changed", state, err)
		}
	}
}

func TestSwarmIntegrationDiagnosticRejectsMalformedPairs(t *testing.T) {
	for stage := range swarmIntegrationStages {
		if !validSwarmIntegrationDiagnostic(ProjectSwarmIntegration{State: "unknown", Stage: stage, FailureCode: "operation-unconfirmed"}) {
			t.Fatal("known unknown-outcome stage refused", stage)
		}
	}
	for _, in := range []ProjectSwarmIntegration{
		{State: "unknown", Stage: "candidate-import"},
		{State: "unknown", FailureCode: "command-failed"},
		{State: "unknown", Stage: "private/path", FailureCode: "command-failed"},
		{State: "unknown", Stage: "candidate-import", FailureCode: "private detail"},
		{State: "integrated", Stage: "candidate-import", FailureCode: "command-failed"},
		{State: "unknown", Stage: "candidate-import", FailureCode: "target-changed"},
		{State: "unknown", Stage: "candidate-import", FailureCode: "persistence-unconfirmed"},
	} {
		if validSwarmIntegrationDiagnostic(in) {
			t.Fatal("malformed diagnostic accepted")
		}
	}
}

func TestSwarmIntegrationDiagnosticDoesNotInventCommandOrigin(t *testing.T) {
	for _, err := range []error{errors.New("private native detail"), &exec.ExitError{}} {
		if swarmIntegrationFailureCode(err) != "operation-unconfirmed" {
			t.Fatal("unproven command origin was asserted")
		}
	}
	for _, err := range []error{context.Canceled, fmt.Errorf("wrapped: %w", context.DeadlineExceeded)} {
		if swarmIntegrationFailureCode(err) != "interrupted" {
			t.Fatal("typed interruption lost")
		}
	}
}

// These source fixtures retain actual Git execution under the future owned
// test Job. The native-exit arm deliberately invokes a fixed invalid Git flag;
// it is not a fabricated ProcessState or upstream-agent execution receipt.
func TestSwarmIntegrationDiagnosticActualHandlerRetainsUnknown(t *testing.T) {
	for _, nativeExit := range []bool{false, true} {
		t.Run(fmt.Sprintf("native-exit-%t", nativeExit), func(t *testing.T) {
			s, mux, p, run, base, _ := swarmHTTPFixture(t)
			s.SetProjectRunSessionObserver(func(context.Context, string, string, string) ([]registry.Session, error) { return nil, nil })
			run = swarmReserveHTTP(t, mux, base, run, "task-a", "diagnostic-reserve-123456789")
			op := run.Operations[0]
			if op.Allocation != "ready" {
				t.Fatal("fixture allocation is unconfirmed")
			}
			swarmCommitFixture(t, mustSwarmPathFixture(t, s, p.ID, run.ID, op.ID), "alpha.txt", "reviewed alpha\n")
			run = swarmFinishFixture(t, s, p, run, op.ID)
			review := swarmReviewHTTP(t, mux, base, "task-a")
			if !review.CanIntegrate {
				t.Fatal("fixture review is unavailable")
			}
			realRunner, calls := s.runner, 0
			s.runner = func(ctx context.Context, exe string, args []string, dir string) (string, error) {
				for _, arg := range args {
					if arg == "fetch" {
						calls++
						if nativeExit {
							return realRunner(ctx, exe, []string{"--luci-fixed-invalid-diagnostic-flag"}, dir)
						}
						return "", errors.New("PRIVATE_NATIVE_DETAIL C:/private/token")
					}
				}
				return realRunner(ctx, exe, args, dir)
			}
			run = projectRunHTTPResponse(t, journeyRequest(t, mux, "POST", base+"/tasks/task-a/integrate", map[string]any{"expectedRevision": run.Revision, "reviewDigest": review.ReviewDigest}, 200)).ProjectRun
			in := swarmTask(&run, "task-a").Integration
			wantCode := "operation-unconfirmed"
			if nativeExit {
				wantCode = "command-failed"
			}
			if in == nil || in.State != "unknown" || in.Stage != "candidate-import" || in.FailureCode != wantCode || in.SourceCommit != review.SourceCommit || in.TargetCommit != review.TargetCommit || in.ReviewDigest != review.ReviewDigest || run.State != "blocked" || calls != 1 || strings.Contains(run.Reason, "PRIVATE_NATIVE_DETAIL") || strings.Contains(run.Reason, "C:/") || strings.Contains(run.Reason, "invalid-diagnostic-flag") {
				t.Fatal("unknown receipt or safe diagnostic lost")
			}
			journeyRequest(t, mux, "POST", base+"/tasks/task-a/integrate", map[string]any{"expectedRevision": run.Revision, "reviewDigest": review.ReviewDigest}, 409)
			if calls != 1 {
				t.Fatal("unknown integration was replayed")
			}
			reopened, err := NewProjectRunStore(s.stateDir)
			if err != nil {
				t.Fatal(err)
			}
			retained, err := reopened.Get(p.ID, p.Path, run.ID)
			task := swarmTask(&retained, "task-a")
			if err != nil || task == nil || task.Integration == nil || task.Integration.Stage != "candidate-import" {
				t.Fatal("diagnostic did not survive readback", err)
			}
		})
	}
}
