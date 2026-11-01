package api

import (
	"context"
	"math"
	"sort"
	"time"

	"github.com/lcfrai/luci/control-plane/internal/management"
	"github.com/lcfrai/luci/control-plane/internal/product"
)

func (a *API) cloudPoolCapacity(ctx context.Context, ref management.ExecutionActorRef, budget int64) (product.CoordinatorCloudCapacity, error) {
	pool := a.deps.CloudTasks.Pool
	pool.mu.Lock()
	hosts := pool.allHosts()
	journal, err := pool.store.read()
	if err != nil {
		pool.mu.Unlock()
		return product.CoordinatorCloudCapacity{SetupNeeds: []string{"The durable cloud placement ledger is unavailable. Existing original host bindings remain held; no new cloud allocation is authorized."}}, nil
	}
	snapshots := pool.snapshots()
	err = pool.refreshLocked(&journal, snapshots)
	usage := pool.aggregateUsage(journal, snapshots, ref, time.Now().UTC())
	pool.mu.Unlock()
	l := pool.config.Limits
	view := product.CoordinatorCloudCapacity{PlacementDigest: pool.catalogueDigest(), Provider: "cloud-pool", Region: "approved host pool", ConfiguredGlobalSlots: l.MaxConcurrent, ConfiguredProjectSlots: l.MaxPerProject, EstimatedMonthMicros: usage.ActorMonthMicros, MonthlyBudgetMicros: budget, MaxTaskSeconds: l.MaxTaskSeconds, RetainedAttempts: []product.CoordinatorCloudAttempt{}, Hosts: []product.CoordinatorCloudHost{}, ObservedAt: time.Now().UTC().Format(time.RFC3339Nano), Limitation: "Configured capacity only. Placement uses only the reviewed host allow-list and fresh host checks; a recorded original is never moved or recreated after host loss. Shared reservations and uncertain original VMs count once; charges remain estimates, not provider bills."}
	totalSlots := 0
	for id, host := range pool.hosts {
		limits, template, hostUsage := host.Runtime.Runtime.Controller.ConfiguredUsage()
		slots := minimumCloudSlots(limits.MaxConcurrent-hostUsage.Active, int((limits.MemoryMiB-hostUsage.MemoryMiB)/template.MemoryMiB), int((limits.CPUMillis-hostUsage.CPUMillis)/template.CPUMillis))
		if host.Draining {
			slots = 0
		}
		view.Hosts = append(view.Hosts, product.CoordinatorCloudHost{ID: id, Name: host.Name, Provider: host.Runtime.Runtime.Controller.Provider(), Region: host.Runtime.Runtime.Controller.Region(), TemplateID: template.ID, Default: id == pool.config.DefaultHostID, Draining: host.Draining, ConfiguredAvailableSlots: slots, EstimatedTaskCostMicros: host.Runtime.EstimatedCostMicros, MaxTaskSeconds: host.Runtime.MaxSeconds})
		totalSlots += slots
		if host.Runtime.EstimatedCostMicros > view.EstimatedTaskCostMicros {
			view.EstimatedTaskCostMicros = host.Runtime.EstimatedCostMicros
		}
	}
	if pool.elastic != nil {
		for _, host := range pool.elasticCandidates(ref, nil) {
			if _, base := pool.hosts[host.ID]; !base {
				limits, template, u := host.Runtime.Runtime.Controller.ConfiguredUsage()
				totalSlots += minimumCloudSlots(limits.MaxConcurrent-u.Active, int((limits.MemoryMiB-u.MemoryMiB)/template.MemoryMiB), int((limits.CPUMillis-u.CPUMillis)/template.CPUMillis))
			}
		}
	}
	sort.Slice(view.Hosts, func(i, j int) bool { return view.Hosts[i].ID < view.Hosts[j].ID })
	view.ConfiguredAvailableSlots = minimumCloudSlots(totalSlots, l.MaxConcurrent-usage.Active, l.MaxPerProject-usage.ProjectActive)
	seen := map[string]bool{}
	for _, snapshot := range snapshots {
		for _, row := range snapshot.Records {
			identity := row.Request.Identity
			seen[cloudOriginalKey(identity.OwnerID, identity.OperationID)] = true
			if identity.ProjectID != ref.ProjectID || !row.ResourcesHeld {
				continue
			}
			view.RetainedAttempts = append(view.RetainedAttempts, product.CoordinatorCloudAttempt{HostID: snapshot.HostID, RunID: identity.RunID, TaskID: identity.TaskID, AttemptID: identity.AttemptID, OperationID: identity.OperationID, State: string(row.State), ResourcesHeld: true, RecoveryPending: cloudSnapshotRecoveryPending(hosts, snapshot.HostID, identity.OperationID), EstimatedCostMicros: row.Request.EstimatedCostMicros, UpdatedAt: row.UpdatedAt.UTC().Format(time.RFC3339Nano)})
		}
	}
	for _, row := range journal.Rows {
		if row.Actor.ProjectID != ref.ProjectID || !row.Admitted || row.Retired || seen[cloudOriginalKey(row.Request.Identity.OwnerID, row.Request.Identity.OperationID)] {
			continue
		}
		identity := row.Request.Identity
		view.RetainedAttempts = append(view.RetainedAttempts, product.CoordinatorCloudAttempt{HostID: row.HostID, RunID: identity.RunID, TaskID: identity.TaskID, AttemptID: identity.AttemptID, OperationID: identity.OperationID, State: "placement-held", ResourcesHeld: true, RecoveryPending: true, EstimatedCostMicros: row.Request.EstimatedCostMicros, UpdatedAt: row.CreatedAt.UTC().Format(time.RFC3339Nano)})
	}
	sort.Slice(view.RetainedAttempts, func(i, j int) bool {
		return view.RetainedAttempts[i].OperationID < view.RetainedAttempts[j].OperationID
	})
	view.RetainedProjectVMs = len(view.RetainedAttempts)
	if len(view.RetainedAttempts) > 20 {
		view.MoreRetainedAttempts = len(view.RetainedAttempts) - 20
		view.RetainedAttempts = view.RetainedAttempts[:20]
	}
	if err != nil || usage.Unknown || usage.ActorMonthMicros > budget || budget < 1 || view.EstimatedTaskCostMicros > budget-usage.ActorMonthMicros || usage.MemoryMiB >= l.MemoryMiB || usage.CPUMillis >= l.CPUMillis || usage.EstimateMicros > l.EstimatedCostMicros-view.EstimatedTaskCostMicros {
		view.ConfiguredAvailableSlots = 0
		view.SetupNeeds = []string{"The shared pool's original allocations, storage or current spending limit cannot admit another task. Unknown originals remain charged; reconcile their exact hosts."}
	}
	if _, policy, present := management.ActorFromContext(ctx); !present || !policy.AllowCloudProvisioning || policy.MonthlyBudgetUSD <= 0 {
		view = cloudCapacityBlocked(view, "Current cloud provisioning and a positive spending allowance are required.")
	}
	if a.deps.AuthorizeExecutionControl == nil || a.deps.AuthorizeExecutionControl(ctx, ref) != nil {
		return product.CoordinatorCloudCapacity{}, management.ErrExecutionAuthority
	}
	if _, err = a.authorizeExecution(ctx, ref, "codex", true); err != nil {
		if _, err = a.authorizeExecution(ctx, ref, "claude-code", true); err != nil {
			view = cloudCapacityBlocked(view, "Current execution, model and secret authority is required. Original owned attempts remain available for recovery.")
		}
	}
	return view, nil
}
func cloudPolicyBudget(value float64) int64 {
	if math.IsNaN(value) || math.IsInf(value, 0) || value <= 0 || value > float64(1<<50)/1_000_000 {
		return 0
	}
	return int64(math.Floor(value * 1_000_000))
}

func cloudSnapshotRecoveryPending(hosts map[string]CloudTaskHost, hostID, operationID string) bool {
	host, ok := hosts[hostID]
	if !ok || host.Runtime == nil || host.Runtime.Runtime == nil || host.Runtime.Runtime.Controller == nil {
		return true
	}
	return host.Runtime.Runtime.Controller.RestoredPending(operationID)
}
