package product

import (
	"context"
	"errors"
	"io"
	"os"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/lcfrai/luci/control-plane/internal/management"
)

// Only durable identity is retained. The runtime must re-resolve current
// account, team, project, harness and secret authority on every continuation.
type CoordinatorReauthorizer func(context.Context, management.ExecutionActorRef, string, bool) (context.Context, error)
type coordinatorBackground struct {
	mu          sync.Mutex
	tick        sync.Mutex
	authorize   CoordinatorReauthorizer
	dispatch    CoordinatorFindingDispatcher
	recipients  management.ProjectRecipientLister
	readFinding management.ProjectReadAuthorizer
	cancel      context.CancelFunc
	done        chan struct{}
	wake        chan struct{}
	cursor      string
	// One refused draft fingerprint per saved actor/project. A 4xx cannot be
	// treated as an absent mutation or sent again on every two-second tick.
	draftRefusals  map[string]string
	reviewRefusals map[string]string
}

func (s *Server) SetCoordinatorRuntime(authorize CoordinatorReauthorizer) {
	b := &s.coordinatorBackground
	b.mu.Lock()
	if b.cancel != nil {
		b.mu.Unlock()
		return
	}
	b.authorize = authorize
	if authorize == nil {
		b.mu.Unlock()
		return
	}
	ctx, cancel := context.WithCancel(s.workspaceContext)
	b.cancel = cancel
	b.done = make(chan struct{})
	b.wake = make(chan struct{}, 1)
	done, wake := b.done, b.wake
	b.mu.Unlock()
	go func() {
		defer close(done)
		timer := time.NewTicker(2 * time.Second)
		defer timer.Stop()
		for {
			select {
			case <-ctx.Done():
				return
			case <-wake:
			case <-timer.C:
			}
			bounded, stop := context.WithTimeout(ctx, 30*time.Second)
			_ = s.RunCoordinatorOnce(bounded)
			stop()
		}
	}()
	s.WakeCoordinator()
}

func (s *Server) SetCoordinatorFindingDispatcher(dispatch CoordinatorFindingDispatcher) {
	b := &s.coordinatorBackground
	b.mu.Lock()
	b.dispatch = dispatch
	b.mu.Unlock()
	s.WakeCoordinator()
}

func (s *Server) SetCoordinatorFindingRecipients(list management.ProjectRecipientLister, read management.ProjectReadAuthorizer) {
	b := &s.coordinatorBackground
	b.mu.Lock()
	b.recipients, b.readFinding = list, read
	b.mu.Unlock()
	s.WakeCoordinator()
}
func (s *Server) WakeCoordinator() {
	b := &s.coordinatorBackground
	b.mu.Lock()
	wake := b.wake
	b.mu.Unlock()
	if wake != nil {
		select {
		case wake <- struct{}{}:
		default:
		}
	}
}
func (s *Server) closeCoordinator() {
	b := &s.coordinatorBackground
	b.mu.Lock()
	cancel, done := b.cancel, b.done
	b.mu.Unlock()
	if cancel == nil {
		return
	}
	cancel()
	select {
	case <-done:
	case <-time.After(5 * time.Second):
	}
	// A timed-out operation retains its pending journal. Closing never certifies
	// its side effect absent, nor makes its identity replayable.
}

// RunCoordinatorOnce advances at most one eligible saved continuation. Its
// cursor advances BEFORE any slow authority/source operation so a pending
// project cannot indefinitely starve another. No browser request is retained.
func (s *Server) RunCoordinatorOnce(ctx context.Context) error {
	b := &s.coordinatorBackground
	b.tick.Lock()
	defer b.tick.Unlock()
	if err := ctx.Err(); err != nil {
		return err
	}
	b.mu.Lock()
	authorize, dispatch, recipients, readFinding, cursor := b.authorize, b.dispatch, b.recipients, b.readFinding, b.cursor
	b.mu.Unlock()
	if authorize == nil {
		return nil
	}
	root, err := guardedProjectPath(s.stateDir, "coordinator-continuations", false)
	if errors.Is(err, os.ErrNotExist) {
		return nil
	}
	if err != nil {
		return err
	}
	f, err := os.Open(root)
	if errors.Is(err, os.ErrNotExist) {
		return nil
	}
	if err != nil {
		return err
	}
	entries, err := f.ReadDir(4097)
	_ = f.Close()
	if err != nil && !errors.Is(err, io.EOF) {
		return err
	}
	if len(entries) > 4096 {
		return ErrOrchestratorCapacity
	}
	names := []string{}
	for _, e := range entries {
		if !e.IsDir() && strings.HasSuffix(e.Name(), ".json") {
			names = append(names, e.Name())
		}
	}
	sort.Strings(names)
	// Only bounded private metadata is read; each full row is discarded before
	// moving to another actor/project. Historical terminal rows are not rerun.
	type candidate struct{ name, id string }
	var next *candidate
	scanned := 0
	for pass := 0; pass < 2 && next == nil; pass++ {
		for _, name := range names {
			if ctx.Err() != nil {
				return ctx.Err()
			}
			if scanned >= 128 {
				return nil
			}
			if (pass == 0 && name+"~" <= cursor) || (pass == 1 && name > cursor) {
				continue
			}
			scanned++
			b.mu.Lock()
			b.cursor = name + "~"
			b.mu.Unlock()
			path, e := guardedProjectPath(root, name, false)
			if e != nil {
				return e
			}
			var header coordinatorContinuations
			if e = externalReadJSON(path, coordinatorContinuationBytes, &header); e != nil {
				return e
			}
			header.Rows = nil
			p, e := s.resolveProject(header.ProjectID)
			if e != nil {
				continue
			}
			expected, e := s.continuationPath(header.ProjectID, header.ActorID, false)
			if e != nil || expected != path {
				return ErrOrchestratorInvalid
			}
			s.mu.RLock()
			rows, e := s.readContinuations(p, header.ActorID)
			s.mu.RUnlock()
			if e != nil {
				return e
			}
			if e = s.settleInterruptedCoordinatorFindings(p, header.ActorID); e != nil {
				return e
			}
			s.mu.RLock()
			rows, e = s.readContinuations(p, header.ActorID)
			s.mu.RUnlock()
			if e != nil {
				return e
			}
			// A delivery can be persisted just before a crash. Reconstruct its
			// per-person notification from the exact durable row, once, before
			// advancing any subsequent affected work.
			if dispatch != nil {
				seeded, e := s.seedCoordinatorFindingRecipients(ctx, p, rows, recipients, readFinding)
				if e != nil {
					return e
				}
				if seeded {
					return nil
				}
				accepted, e := s.deliverAcceptedFinding(ctx, p, rows, authorize, readFinding)
				if e != nil {
					return e
				}
				if accepted {
					return nil
				}
				published, e := s.deliverCoordinatorFindings(ctx, p, rows, root, names, authorize, readFinding)
				if e != nil {
					return e
				}
				if published {
					return nil
				}
				followed, e := s.continueCoordinatorFinding(ctx, p, rows, authorize, dispatch)
				if e != nil {
					return e
				}
				if followed {
					return nil
				}
			}
			advanced, e := s.continueSavedCoordinatorDraft(ctx, p, rows, authorize)
			if e != nil {
				return e
			}
			if advanced {
				return nil
			}
			reviewed, e := s.continueSavedCoordinatorReview(ctx, p, rows, authorize)
			if e != nil {
				return e
			}
			if reviewed {
				return nil
			}
			created, e := s.continueAcceptedCoordinatorEvent(ctx, p, rows, authorize)
			if e != nil {
				return e
			}
			if created {
				return nil
			}
			sort.Slice(rows.Rows, func(i, j int) bool { return rows.Rows[i].ID < rows.Rows[j].ID })
			for _, row := range rows.Rows {
				key := name + "/" + row.ID
				if (pass == 0 && key <= cursor) || (pass == 1 && key > cursor) {
					continue
				}
				if row.ExecutionActor.UserID == "" || row.Status == "refused" || row.Status == "blocked" || row.Status == "deployed" || row.Status == "deployment-blocked" || row.Status == "preview-blocked" {
					continue
				}
				if row.Status == "preview-ready" {
					if row.DeploymentPlan == nil || row.Run.Outcome != "publish" {
						continue
					}
					s.mu.RLock()
					policy, policyErr := s.readCoordinatorPolicy(p)
					s.mu.RUnlock()
					if policyErr != nil {
						return policyErr
					}
					if policy.DeploymentMode != "existing-target" {
						continue
					}
				}
				if row.Status == "checks-verified" {
					if row.PreviewPlan == nil {
						continue
					}
					s.mu.RLock()
					policy, policyErr := s.readCoordinatorPolicy(p)
					s.mu.RUnlock()
					if policyErr != nil {
						return policyErr
					}
					if policy.PreviewMode != "checked-source" {
						continue
					}
				}
				next = &candidate{name, row.ID}
				b.mu.Lock()
				b.cursor = key
				b.mu.Unlock()
				var fresh context.Context
				if coordinatorRetainedDeploymentObservation(row) {
					fresh, e = s.reauthorizeCoordinatorDeploymentActor(ctx, p, row, false)
				} else {
					fresh, e = authorize(ctx, row.ExecutionActor, row.Run.Harness, row.Run.AgentConnectionID != nil && *row.Run.AgentConnectionID != "")
				}
				if e != nil {
					return e
				}
				actor, ok := authenticatedContextActor(fresh)
				if !ok || actor.ID != row.Actor.ID {
					return errCoordinatorAuthority
				}
				stale, staleErr := s.blockStaleCoordinatorRow(p, row)
				if staleErr != nil {
					return staleErr
				}
				if stale {
					return nil // retain prior results; the accepted event owns replan
				}
				assigned, assignmentErr := s.continueCoordinatorIndependentReview(fresh, p, row)
				if assignmentErr != nil {
					return assignmentErr
				}
				if assigned {
					return nil
				}
				reviewedOutcome, reviewErr := s.continueCoordinatorReviewOutcome(fresh, p, row)
				if reviewErr != nil {
					return reviewErr
				}
				if reviewedOutcome {
					return nil
				}
				repair, repairErr := s.continueCoordinatorRepair(fresh, p, row)
				if repairErr != nil {
					return repairErr
				}
				if repair {
					return nil
				}
				integrated, integrationErr := s.continueCoordinatorCandidateIntegration(fresh, p, row)
				if integrationErr != nil {
					return integrationErr
				}
				if integrated {
					return nil
				}
				_, e = s.continueCoordinator(fresh, p, row.ID, row.Actor.ID, 0)
				return e
			}
		}
	}
	b.mu.Lock()
	b.cursor = ""
	b.mu.Unlock()
	return nil
}
