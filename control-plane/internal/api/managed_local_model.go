package api

import (
	"context"
	"errors"
	"strings"

	"github.com/lcfrai/luci/control-plane/internal/backend"
	"github.com/lcfrai/luci/control-plane/internal/managedmodel"
	"github.com/lcfrai/luci/control-plane/internal/management"
	"github.com/lcfrai/luci/control-plane/internal/product"
	"github.com/lcfrai/luci/control-plane/internal/registry"
)

func (a *API) holdManagedModel(ctx context.Context, c product.AgentConnectionReceipt, lease string) (func(), *managedmodel.Binding, error) {
	if c.ManagedLocalModel == nil {
		return func() {}, nil, nil
	}
	if a.execution == nil || a.execution.scope == "" || lease == "" || a.deps.Backend.Kind() != backend.KindDocker {
		return nil, nil, errors.New("The managed model requires an owned local execution admission.")
	}
	unlock, o, err := managedmodel.Hold(ctx, c.ManagedLocalModel, c.Endpoint, c.Model)
	if err != nil {
		return nil, nil, err
	}
	return unlock, &managedmodel.Binding{Boot: o.Boot, Generation: o.Status.Generation, Revision: o.Revision, RuntimeScope: a.execution.scope, LeaseID: lease, Endpoint: c.Endpoint, Model: c.Model}, nil
}
func (a *API) checkManagedBinding(b *managedmodel.Binding) error {
	if b == nil {
		return nil
	}
	if a.execution == nil || b.RuntimeScope != a.execution.scope || b.LeaseID == "" {
		return errors.New("managed model runtime identity changed")
	}
	o, e := managedmodel.Check(&managedmodel.Reference{Revision: b.Revision}, b.Endpoint, b.Model)
	if e != nil {
		return e
	}
	if o.Boot != b.Boot || o.Status.Generation != b.Generation {
		return errors.New("managed model generation changed")
	}
	return nil
}

// Read-only assistants use the existing worker admission and durable launch
// journal too. This is scoped only to managed references; BYO is unchanged.
func (a *API) reserveManagedAssistant(ctx context.Context, row registry.Session, c product.AgentConnectionReceipt, resources backend.Resources) (func(), *managedmodel.Binding, error) {
	if c.ManagedLocalModel == nil {
		return func() {}, nil, nil
	}
	id := "model-" + row.ID
	c.Model = strings.TrimPrefix(c.Model, "luci-gateway/")
	release, binding, e := a.holdManagedModel(ctx, c, id)
	if e != nil {
		return nil, nil, e
	}
	fail := func(e error) (func(), *managedmodel.Binding, error) { release(); return nil, nil, e }
	actor, e := a.captureQueuedActor(ctx, row.ProjectID)
	if e != nil {
		return fail(e)
	}
	authority, e := a.authorizeExecution(ctx, actor, row.Harness, true)
	if e != nil {
		return fail(e)
	}
	if actor.UserID != "" {
		ctx = management.WithExecutionAuthority(ctx, authority)
	}
	policy, e := a.deps.Product.SwarmSchedulerPolicy()
	if e != nil {
		return fail(e)
	}
	provider, limit := providerCapacity(product.ProjectRunRoute{Harness: row.Harness, Connection: c}, policy)
	decision, e := a.execution.reserve(ctx, id, row.ProjectID, provider, limit, actor, authority, policy)
	if e != nil {
		return fail(e)
	}
	if decision.State != "admitted" {
		return fail(errors.New(decision.Message))
	}
	if decision.Resources != resources {
		_ = a.execution.release(ctx, id)
		return fail(errors.New("The assistant resource policy changed. Refresh the current allocation before trying again."))
	}
	return release, binding, nil
}
func (a *API) bindManagedAssistant(ctx context.Context, row registry.Session, b *managedmodel.Binding) error {
	if b == nil {
		return nil
	}
	if e := a.execution.bind(ctx, b.LeaseID, row.ID); e != nil {
		return e
	}
	return a.execution.mutate(ctx, func(l *executionLeaseLedger) error {
		for i := range l.Leases {
			v := &l.Leases[i]
			if v.Scope == a.execution.scope && v.ID == b.LeaseID && v.SessionID == row.ID && v.State == "allocating" {
				copy := *b
				v.ManagedLocalModel = &copy
				return nil
			}
		}
		return errors.New("managed assistant lease changed")
	})
}

// The parent supplies only boot/generation. Exact session/CID identities come
// from this private runtime's durable ledger, never a browser stop request.
func (a *API) ManagedModelDependents(ctx context.Context, boot, generation string, stop bool) error {
	rows, err := a.deps.Store.List()
	if err != nil {
		return err
	}
	for _, row := range rows {
		b := row.ManagedLocalModel
		if b == nil || b.Boot != boot || b.Generation != generation {
			continue
		}
		if a.execution == nil || b.RuntimeScope != a.execution.scope {
			return errors.New("managed model dependent runtime identity differs")
		}
		if row.SandboxID == "" {
			return errors.New("managed model dependent allocation identity unavailable")
		}
		if !sessionRestarts.acquire(row.ID) {
			return errors.New("managed model dependent lifecycle is in progress")
		}
		err = func() error {
			defer sessionRestarts.release(row.ID)
			current, e := a.deps.Store.Get(row.ID)
			if e != nil {
				return e
			}
			if current.SandboxID != row.SandboxID || current.SandboxRef != row.SandboxRef || !current.CreatedAt.Equal(row.CreatedAt) || current.ManagedLocalModel == nil || *current.ManagedLocalModel != *b || current.Backend != string(backend.KindDocker) || current.MachineID != LocalMachineID {
				return errors.New("managed model dependent allocation changed")
			}
			if current.Status == registry.StatusExited {
				absent, e := a.exactBackendTerminal(ctx, current.SandboxID, true)
				if e != nil {
					return e
				}
				if absent {
					return a.execution.releaseSession(ctx, current.ID)
				}
			}
			if !stop {
				return errors.New("managed model still has dependent work")
			}
			phase, e := a.managedModelLease(current, b)
			if e != nil {
				return e
			}
			if phase == "allocating" {
				absent, e := a.exactBackendTerminal(ctx, current.SandboxID, true)
				if e != nil || !absent {
					return errors.New("unstarted managed allocation is not positively absent")
				}
				_, e = a.deps.Store.Update(current.ID, func(s *registry.Session) error {
					if s.SandboxID != current.SandboxID || s.ManagedLocalModel == nil || *s.ManagedLocalModel != *b {
						return sessionErrMoved
					}
					s.Status = registry.StatusExited
					s.ExitReason = "Luci cancelled this pending managed-model allocation while closing."
					return nil
				})
				if e != nil {
					return e
				}
				return a.execution.releaseSession(ctx, current.ID)
			}
			if b.AssistanceJobID != "" {
				job, e := a.assistanceStore().Read(b.AssistanceJobID)
				if e != nil || job.SessionID != row.ID || row.WorktreeID != assistWorktree {
					return errors.New("managed assistant ownership unavailable")
				}
				a.assistMu.Lock()
				cancel := a.assistCancels[b.AssistanceJobID]
				a.assistMu.Unlock()
				if cancel != nil {
					cancel()
				}
			}
			if e = a.deps.Backend.Destroy(ctx, current.SandboxID); e != nil && !errors.Is(e, backend.ErrNotFound) {
				return e
			}
			absent, e := a.exactBackendTerminal(ctx, current.SandboxID, true)
			if e != nil || !absent {
				return errors.New("managed model dependent exact container removal is unconfirmed")
			}
			_, e = a.deps.Store.Update(current.ID, func(s *registry.Session) error {
				if s.SandboxID != current.SandboxID || s.SandboxRef != current.SandboxRef || s.ManagedLocalModel == nil || *s.ManagedLocalModel != *b {
					return sessionErrMoved
				}
				s.Status = registry.StatusExited
				if s.ExitReason == "" {
					s.ExitReason = "Luci stopped this session while closing its managed local model."
				}
				s.Endpoint = ""
				s.AttachToken = ""
				return nil
			})
			if e != nil {
				return e
			}
			return a.execution.releaseSession(ctx, current.ID)
		}()
		if err != nil {
			return err
		}
	}
	return nil
}
func (a *API) managedModelLease(row registry.Session, b *managedmodel.Binding) (string, error) {
	ledger, e := a.execution.readFenceLedger()
	if e != nil {
		return "", e
	}
	for _, lease := range ledger.Leases {
		if lease.Scope == a.execution.scope && lease.ID == b.LeaseID && lease.SessionID == row.ID && lease.ProjectID == row.ProjectID && lease.SandboxID == row.SandboxID && (lease.State == "backend-settled" || lease.State == "allocating") && lease.LaunchGuard == 1 && lease.ManagedLocalModel != nil && *lease.ManagedLocalModel == *b {
			return lease.State, nil
		}
	}
	return "", errors.New("managed model dependent execution lease is unavailable")
}
