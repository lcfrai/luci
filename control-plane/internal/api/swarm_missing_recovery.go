package api

import (
	"context"
	"encoding/hex"
	"errors"
	"time"

	"github.com/lcfrai/luci/control-plane/internal/backend"
	"github.com/lcfrai/luci/control-plane/internal/management"
	"github.com/lcfrai/luci/control-plane/internal/product"
	"github.com/lcfrai/luci/control-plane/internal/registry"
)

// Retirements contain ownership and evidence only, never files or credentials.
// They remain after capacity is released so a delayed allocator cannot reuse
// the exact lease, child or reviewed operation.
type executionRetirement struct {
	Scope       string                            `json:"scope"`
	LeaseID     string                            `json:"leaseId"`
	ProjectID   string                            `json:"projectId"`
	RunID       string                            `json:"runId"`
	OperationID string                            `json:"operationId"`
	ChildID     string                            `json:"childId"`
	QueueID     string                            `json:"queueId,omitempty"`
	Proof       product.ProjectSwarmRecoveryProof `json:"proof"`
	RetiredAt   time.Time                         `json:"retiredAt"`
}

func validExecutionRetirement(v executionRetirement) bool {
	scope, e := hex.DecodeString(v.Scope)
	if e != nil || len(scope) != 32 || v.LeaseID == "" || len(v.LeaseID) > 200 || v.ProjectID == "" || len(v.ProjectID) > 200 || v.RunID == "" || len(v.RunID) > 200 || v.OperationID == "" || len(v.OperationID) > 200 || v.ChildID == "" || len(v.ChildID) > 200 || len(v.QueueID) > 200 || v.RetiredAt.IsZero() || v.Proof.LeaseID != v.LeaseID || v.Proof.ChildID != v.ChildID || v.Proof.QueueID != v.QueueID || (v.QueueID != "" && v.QueueID != v.LeaseID) || v.Proof.Backend == "" || len(v.Proof.Backend) > 64 {
		return false
	}
	observed, e1 := time.Parse(time.RFC3339Nano, v.Proof.ObservedAt)
	inventory, e2 := time.Parse(time.RFC3339Nano, v.Proof.InventoryObservedAt)
	return e1 == nil && e2 == nil && !observed.Before(inventory) && !v.RetiredAt.Before(observed)
}
func retiredExecutionMatches(v executionRetirement, req product.ProjectSwarmRecoveryRequest) bool {
	return v.ProjectID == req.ProjectID && v.RunID == req.RunID && v.OperationID == req.OperationID && v.ChildID == req.ChildID && v.QueueID == req.QueueID
}

var errMissingRecoveryUnproved = errors.New("This exact child cannot be proved absent before backend launch. Its operation, files and capacity remain retained; no replacement was started.")

func (a *API) authorizeExecutionRetirement(ctx context.Context, projectID string) error {
	actor, err := management.CaptureExecutionControlActor(ctx, projectID)
	if err != nil {
		return err
	}
	if a.deps.AuthorizeExecutionControl != nil {
		return a.deps.AuthorizeExecutionControl(ctx, actor)
	}
	if actor.Local && !a.deps.ManagedExecution && a.deps.AuthorizeExecution == nil {
		return ctx.Err()
	}
	return management.ErrExecutionAuthority
}

func (g *executionAdmission) recoveryLease(ctx context.Context, req product.ProjectSwarmRecoveryRequest) (executionLease, *executionRetirement, error) {
	var found executionLease
	var retired *executionRetirement
	err := g.mutate(ctx, func(l *executionLeaseLedger) error {
		for _, v := range l.Retirements {
			if v.Scope == g.scope && retiredExecutionMatches(v, req) {
				copy := v
				retired = &copy
				return nil
			}
		}
		for _, v := range l.Leases {
			if v.Scope != g.scope || v.ProjectID != req.ProjectID {
				continue
			}
			if req.QueueID != "" {
				if v.ID != req.QueueID {
					continue
				}
			} else if v.SessionID != req.ChildID {
				continue
			}
			if found.ID != "" {
				return errMissingRecoveryUnproved
			}
			if v.LaunchGuard != 1 || (v.State != "reserved" && v.State != "allocating") || (v.SessionID != "" && v.SessionID != req.ChildID) || (v.SandboxID != "" && v.SandboxID != req.ChildID) {
				return errMissingRecoveryUnproved
			}
			found = v
		}
		if found.ID == "" {
			return errMissingRecoveryUnproved
		}
		return nil
	})
	return found, retired, err
}

// An exact backend not-found alone is insufficient. The independently sampled
// complete inventory must be newer than this probe and the durable claim. No
// transport failure, stale sample or missing registry is interpreted as zero.
func (a *API) observeMissingExecution(ctx context.Context, req product.ProjectSwarmRecoveryRequest) (time.Time, error) {
	if a.deps.ResourceSnapshot == nil {
		return time.Time{}, errMissingRecoveryUnproved
	}
	if _, err := a.deps.Store.Get(req.ChildID); !errors.Is(err, registry.ErrNotFound) {
		return time.Time{}, errMissingRecoveryUnproved
	}
	started := time.Now().UTC()
	claimed, err := time.Parse(time.RFC3339Nano, req.UpdatedAt)
	if err != nil {
		return time.Time{}, errMissingRecoveryUnproved
	}
	bounded, cancel := context.WithTimeout(ctx, 3*time.Second)
	defer cancel()
	_, err = a.deps.Backend.Get(bounded, req.ChildID)
	if !errors.Is(err, backend.ErrNotFound) || errors.Is(err, backend.ErrUnavailable) || errors.Is(err, context.DeadlineExceeded) || errors.Is(err, context.Canceled) || bounded.Err() != nil {
		return time.Time{}, errMissingRecoveryUnproved
	}
	ticker := time.NewTicker(50 * time.Millisecond)
	defer ticker.Stop()
	for {
		snapshot := a.deps.ResourceSnapshot().Execution
		now := time.Now().UTC()
		if snapshot.InventoryComplete && snapshot.Fresh(now, 5*time.Second) && !snapshot.ObservedAt.Before(started) && !snapshot.ObservedAt.Before(claimed) {
			for _, row := range snapshot.Containers {
				if row.SandboxID == req.ChildID || row.ID == req.ChildID {
					return time.Time{}, errMissingRecoveryUnproved
				}
			}
			if _, err := a.deps.Store.Get(req.ChildID); !errors.Is(err, registry.ErrNotFound) {
				return time.Time{}, errMissingRecoveryUnproved
			}
			return snapshot.ObservedAt, nil
		}
		select {
		case <-bounded.Done():
			return time.Time{}, errMissingRecoveryUnproved
		case <-ticker.C:
		}
	}
}
func (a *API) proveSwarmMissingChild(ctx context.Context, req product.ProjectSwarmRecoveryRequest) (product.ProjectSwarmRecoveryProof, error) {
	if err := a.authorizeExecutionRetirement(ctx, req.ProjectID); err != nil {
		return product.ProjectSwarmRecoveryProof{}, err
	}
	lease, retired, err := a.execution.recoveryLease(ctx, req)
	if err != nil {
		a.sessionLog().Debug("missing child lease could not be verified", "err", err)
		return product.ProjectSwarmRecoveryProof{}, errMissingRecoveryUnproved
	}
	if retired != nil {
		return retired.Proof, nil
	}
	observed, err := a.observeMissingExecution(ctx, req)
	if err != nil {
		return product.ProjectSwarmRecoveryProof{}, err
	}
	// The product caller holds its dispatch mutex, but recheck durable ownership
	// after the observation too so any independent allocator marker wins safely.
	current, _, err := a.execution.recoveryLease(ctx, req)
	if err != nil || current.ID != lease.ID || current.UpdatedAt != lease.UpdatedAt {
		return product.ProjectSwarmRecoveryProof{}, errMissingRecoveryUnproved
	}
	return product.ProjectSwarmRecoveryProof{ChildID: req.ChildID, QueueID: req.QueueID, LeaseID: lease.ID, Backend: string(a.deps.Backend.Kind()), ObservedAt: time.Now().UTC().Format(time.RFC3339Nano), InventoryObservedAt: observed.Format(time.RFC3339Nano)}, nil
}
func (a *API) finalizeSwarmMissingChild(ctx context.Context, req product.ProjectSwarmRecoveryRequest, proof product.ProjectSwarmRecoveryProof) error {
	if err := a.authorizeExecutionRetirement(ctx, req.ProjectID); err != nil {
		return err
	}
	if a.deps.Product == nil {
		return errMissingRecoveryUnproved
	}
	retained, err := a.deps.Product.ProjectSwarmRetirement(req.ProjectID, req.RunID, req.OperationID, req.ChildID, req.QueueID)
	if err != nil || retained == nil || *retained != proof {
		return errMissingRecoveryUnproved
	}
	lease, retired, err := a.execution.recoveryLease(ctx, req)
	if err != nil {
		a.sessionLog().Debug("missing child finalization lease could not be verified", "err", err)
		return errMissingRecoveryUnproved
	}
	if retired != nil {
		if retired.Proof == proof {
			return nil
		}
		return errMissingRecoveryUnproved
	}
	if lease.ID != proof.LeaseID {
		return errMissingRecoveryUnproved
	}
	if _, err = a.observeMissingExecution(ctx, req); err != nil {
		return err
	}
	if err = a.authorizeExecutionRetirement(ctx, req.ProjectID); err != nil {
		return err
	}
	err = a.execution.mutate(ctx, func(l *executionLeaseLedger) error {
		for _, v := range l.Retirements {
			if v.Scope == a.execution.scope && v.LeaseID == proof.LeaseID {
				if retiredExecutionMatches(v, req) && v.Proof == proof {
					return nil
				}
				return errMissingRecoveryUnproved
			}
		}
		if len(l.Retirements) >= 4096 {
			return errors.New("Execution retirement history is full; capacity remains held until its history can be archived safely.")
		}
		if _, err := a.deps.Store.Get(req.ChildID); !errors.Is(err, registry.ErrNotFound) {
			return errMissingRecoveryUnproved
		}
		at := -1
		for i, v := range l.Leases {
			if v.Scope == a.execution.scope && v.ID == proof.LeaseID {
				if v != lease {
					return errMissingRecoveryUnproved
				}
				at = i
				break
			}
		}
		if at < 0 {
			return errMissingRecoveryUnproved
		}
		l.Retirements = append(l.Retirements, executionRetirement{Scope: a.execution.scope, LeaseID: proof.LeaseID, ProjectID: req.ProjectID, RunID: req.RunID, OperationID: req.OperationID, ChildID: req.ChildID, QueueID: req.QueueID, Proof: proof, RetiredAt: time.Now().UTC()})
		l.Leases = append(l.Leases[:at], l.Leases[at+1:]...)
		return nil
	})
	if err != nil {
		a.sessionLog().Debug("missing child finalization could not be persisted", "err", err)
		return errMissingRecoveryUnproved
	}
	return nil
}
