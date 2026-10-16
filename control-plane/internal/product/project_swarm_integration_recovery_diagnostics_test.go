package product

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/lcfrai/luci/control-plane/internal/registry"
)

func TestSwarmIntegrationDiagnosticPublicationReadbackRecovery(t *testing.T) {
	s, mux, p, run, base, _ := swarmHTTPFixture(t)
	s.SetProjectRunSessionObserver(func(context.Context, string, string, string) ([]registry.Session, error) { return nil, nil })
	run = swarmReserveHTTP(t, mux, base, run, "task-a", "recovery-diagnostic-123456789")
	op := run.Operations[0]
	if op.Allocation != "ready" {
		t.Fatal("fixture allocation is unconfirmed")
	}
	swarmCommitFixture(t, mustSwarmPathFixture(t, s, p.ID, run.ID, op.ID), "alpha.txt", "reviewed recovery\n")
	run = swarmFinishFixture(t, s, p, run, op.ID)
	review := swarmReviewHTTP(t, mux, base, "task-a")
	if !review.CanIntegrate {
		t.Fatal("fixture review is unavailable")
	}
	realRunner := s.runner
	published, readbackRefused, mutations := false, false, 0
	s.runner = func(ctx context.Context, exe string, args []string, dir string) (string, error) {
		joined := " " + strings.Join(args, " ") + " "
		if published && dir == p.Path && strings.Contains(joined, " rev-parse HEAD ") && !readbackRefused {
			readbackRefused = true
			return "", errors.New("controlled target readback failure")
		}
		if strings.Contains(joined, " fetch ") || strings.Contains(joined, " worktree add ") || strings.Contains(joined, " merge ") {
			mutations++
		}
		out, err := realRunner(ctx, exe, args, dir)
		if dir == p.Path && strings.Contains(joined, " merge --ff-only ") && err == nil {
			published = true
		}
		return out, err
	}
	run = projectRunHTTPResponse(t, journeyRequest(t, mux, "POST", base+"/tasks/task-a/integrate", map[string]any{"expectedRevision": run.Revision, "reviewDigest": review.ReviewDigest}, 200)).ProjectRun
	in := swarmTask(&run, "task-a").Integration
	if !published || !readbackRefused || in == nil || in.State != "unknown" || in.Stage != "target-readback" || in.FailureCode != "operation-unconfirmed" || in.Commit == "" {
		t.Fatal("fixture did not reach published-but-unconfirmed readback")
	}
	commit, beforeMutations := in.Commit, mutations
	reopened, err := NewProjectRunStore(s.stateDir)
	if err != nil {
		t.Fatal(err)
	}
	s.projectRuns = reopened
	run, err = reopened.Get(p.ID, p.Path, run.ID)
	if err != nil || swarmTask(&run, "task-a").Integration.Stage != "target-readback" {
		t.Fatal("restart lost the retained diagnostic", err)
	}
	// A matching HEAD alone is insufficient: the existing clean-source proof
	// must still refuse a dirty target, without discarding the failure detail.
	dirty := filepath.Join(p.Path, "unreviewed-recovery.txt")
	if err := os.WriteFile(dirty, []byte("unreviewed\n"), 0600); err != nil {
		t.Fatal(err)
	}
	run = projectRunHTTPResponse(t, journeyRequest(t, mux, "POST", base+"/reconcile", map[string]int{"expectedRevision": run.Revision}, 200)).ProjectRun
	if swarmTask(&run, "task-a").Integration.State != "unknown" || swarmTask(&run, "task-a").Integration.Stage != "target-readback" {
		t.Fatal("dirty target was promoted or diagnostic lost")
	}
	if err := os.Remove(dirty); err != nil {
		t.Fatal(err)
	}
	run = projectRunHTTPResponse(t, journeyRequest(t, mux, "POST", base+"/reconcile", map[string]int{"expectedRevision": run.Revision}, 200)).ProjectRun
	in = swarmTask(&run, "task-a").Integration
	if in.State != "integrated" || in.Commit != commit || in.Stage != "" || in.FailureCode != "" || in.SourceCommit != review.SourceCommit || in.TargetCommit != review.TargetCommit || in.ReviewDigest != review.ReviewDigest || mutations != beforeMutations || run.Acceptance != nil {
		t.Fatal("exact recovery failed, replayed Git or changed reviewed identity")
	}
	retained, err := reopened.Get(p.ID, p.Path, run.ID)
	if err != nil || swarmTask(&retained, "task-a").Integration.State != "integrated" {
		t.Fatal("recovered receipt did not persist", err)
	}
}

func TestSwarmIntegrationCandidateWritePreservesAcknowledgedRevision(t *testing.T) {
	for _, mode := range []string{"before-write", "lost-ack-zero", "lost-ack-returned", "acknowledged"} {
		t.Run(mode, func(t *testing.T) {
			s, _, p, run, _, _ := swarmHTTPFixture(t)
			s.SetProjectRunSessionObserver(func(context.Context, string, string, string) ([]registry.Session, error) { return nil, nil })
			// This is a synthetic durable receipt at the real fixture HEAD, not
			// evidence that an integration command or external worker ran.
			candidate := strings.TrimSpace(confinedFixtureGit(t, p.Path, "rev-parse", "HEAD"))
			var err error
			run, err = s.projectRuns.mutateSwarm(p.ID, p.Path, run.ID, run.Revision, func(current *ProjectRun) error {
				swarmTask(current, "task-a").Integration = &ProjectSwarmIntegration{State: "integrating", SourceCommit: candidate, TargetCommit: candidate, ReviewDigest: strings.Repeat("a", 64)}
				return nil
			})
			if err != nil {
				t.Fatal(err)
			}
			acknowledged, calls := run, 0
			writeErr := errors.New("controlled candidate write acknowledgment failure")
			run, err = recordSwarmIntegrationCandidate(run, func() (ProjectRun, error) {
				calls++
				if mode == "before-write" {
					return ProjectRun{}, writeErr
				}
				updated, saveErr := s.projectRuns.mutateSwarm(p.ID, p.Path, run.ID, run.Revision, func(current *ProjectRun) error {
					swarmTask(current, "task-a").Integration.Commit = candidate
					return nil
				})
				if saveErr != nil {
					t.Fatal(saveErr)
				}
				if mode == "acknowledged" {
					return updated, nil
				}
				if mode == "lost-ack-returned" {
					return updated, writeErr
				}
				return ProjectRun{}, writeErr
			})
			if calls != 1 || run.ID != acknowledged.ID || run.ProjectID != acknowledged.ProjectID {
				t.Fatal("candidate write replayed or confirmed identity lost")
			}
			if mode == "acknowledged" {
				if err != nil || run.Revision != acknowledged.Revision+1 || swarmTask(&run, "task-a").Integration.Commit != candidate {
					t.Fatal("acknowledged write did not advance")
				}
				return
			}
			if !errors.Is(err, writeErr) || run.Revision != acknowledged.Revision || swarmTask(&run, "task-a").Integration.Commit != "" {
				t.Fatal("uncertain write replaced last acknowledged receipt")
			}
			final, finalErr := s.projectRuns.mutateSwarm(p.ID, p.Path, run.ID, run.Revision, func(current *ProjectRun) error {
				in := swarmTask(current, "task-a").Integration
				in.State, in.Stage, in.FailureCode = "unknown", "candidate-record", "persistence-unconfirmed"
				current.State = "blocked"
				current.Reason = swarmIntegrationFailureReason(in.Stage, in.FailureCode)
				return nil
			})
			reopened, openErr := NewProjectRunStore(s.stateDir)
			if openErr != nil {
				t.Fatal(openErr)
			}
			retained, readErr := reopened.Get(p.ID, p.Path, run.ID)
			if readErr != nil {
				t.Fatal(readErr)
			}
			in := swarmTask(&retained, "task-a").Integration
			if mode == "before-write" {
				if finalErr != nil || final.ID != acknowledged.ID || retained.Revision != acknowledged.Revision+1 || in.State != "unknown" || in.Commit != "" || in.Stage != "candidate-record" || in.FailureCode != "persistence-unconfirmed" {
					t.Fatal("confirmed final unknown write was lost", finalErr)
				}
				return
			}
			if !errors.Is(finalErr, ErrProjectRunConflict) || retained.Revision != acknowledged.Revision+1 || in.State != "integrating" || in.Commit != candidate || in.Stage != "" || in.FailureCode != "" {
				t.Fatal("lost acknowledgment was overwritten or final failed save claimed durable diagnostics", finalErr)
			}
			// Only the existing clean-HEAD proof may recover this retained write.
			s.projectRuns = reopened
			dirty := filepath.Join(p.Path, "unreviewed-lost-ack.txt")
			if err := os.WriteFile(dirty, []byte("dirty\n"), 0600); err != nil {
				t.Fatal(err)
			}
			retained, err = s.observeProjectSwarm(context.Background(), p.Path, retained)
			if err != nil || swarmTask(&retained, "task-a").Integration.State != "integrating" {
				t.Fatal("dirty lost-ACK candidate was promoted", err)
			}
			if err := os.Remove(dirty); err != nil {
				t.Fatal(err)
			}
			retained, err = s.observeProjectSwarm(context.Background(), p.Path, retained)
			if err != nil || swarmTask(&retained, "task-a").Integration.State != "integrated" || calls != 1 {
				t.Fatal("exact lost-ACK recovery failed or replayed candidate write", err)
			}
		})
	}
}
