package api

import (
	"context"
	"math"
	"sort"
	"time"

	"github.com/lcfrai/luci/control-plane/internal/backend/e2bbe"
	"github.com/lcfrai/luci/control-plane/internal/management"
	"github.com/lcfrai/luci/control-plane/internal/product"
)

func minimumCloudSlots(values ...int) int {
	minimum := values[0]
	for _, value := range values[1:] {
		if value < minimum {
			minimum = value
		}
	}
	if minimum < 0 {
		return 0
	}
	return minimum
}

// cloudTaskCapacity is read-only advice shown in the normal Develop flow.
// Unknown/legacy attributed spend is never presented as remaining budget.
func (a *API) cloudTaskCapacity(ctx context.Context, projectID, actorID string) (product.CoordinatorCloudCapacity, error) {
	actor, policy, present := management.ActorFromContext(ctx)
	if !present || actor.ID != actorID {
		return product.CoordinatorCloudCapacity{}, nil
	}
	cloud := a.deps.CloudTasks
	if cloud == nil {
		return product.CoordinatorCloudCapacity{SetupNeeds: []string{"The managed microVM service is not configured for this workspace. An operator must connect a protected E2B Embed host before cloud tasks can start."}}, nil
	}
	// A person who may control the exact owned task must still see its retained
	// identity and cost after Execute, model access or provisioning is revoked.
	// Those permissions only govern another Create; they cannot make an
	// existing allocation disappear from the recovery view.
	ref, err := management.CaptureExecutionControlActor(ctx, projectID)
	if err != nil || ref.Local {
		return product.CoordinatorCloudCapacity{SetupNeeds: []string{"Current project edit access and a managed identity are required to inspect or recover owned cloud tasks."}}, nil
	}
	if a.deps.AuthorizeExecutionControl == nil {
		return product.CoordinatorCloudCapacity{SetupNeeds: []string{"Cloud task recovery authority is unavailable; refresh after the managed project controller is configured."}}, nil
	}
	if err = a.deps.AuthorizeExecutionControl(ctx, ref); err != nil {
		return product.CoordinatorCloudCapacity{SetupNeeds: []string{"Current project edit access is required to inspect or recover owned cloud tasks."}}, nil
	}
	budget := int64(math.Floor(policy.MonthlyBudgetUSD * 1_000_000))
	if budget < 0 {
		budget = 0
	}
	limits, template, usage, projectActive, rows, settled := cloud.Runtime.Controller.Capacity(projectID)
	pending := make(map[string]bool)
	for _, row := range rows {
		if row.Request.Identity.ProjectID == projectID && cloud.Runtime.Controller.RestoredPending(row.Request.Identity.OperationID) {
			pending[row.Request.Identity.OperationID] = true
		}
	}
	view := cloudCapacityView(limits, template, usage, projectActive, rows, settled, pending, actorID, projectID, cloud.Runtime.Controller.Region(), cloud.EstimatedCostMicros, cloud.MaxSeconds, budget, time.Now().UTC())
	if cloud.Runtime.Controller.Provider() == "azure" {
		view.Provider = "azure-e2b-embed"
	}
	if !policy.AllowCloudProvisioning || policy.MonthlyBudgetUSD <= 0 {
		return cloudCapacityBlocked(view, "Cloud provisioning and a positive monthly spending budget must be enabled by the current account or team policy."), nil
	}
	if _, err = a.authorizeExecution(ctx, ref, "codex", true); err != nil {
		if _, err = a.authorizeExecution(ctx, ref, "claude-code", true); err != nil {
			return cloudCapacityBlocked(view, "Current project execution, secret access or an allowed Codex/Claude Code harness connection is missing. Existing owned attempts remain visible for exact recovery."), nil
		}
	}
	return view, nil
}

func cloudCapacityBlocked(view product.CoordinatorCloudCapacity, reason string) product.CoordinatorCloudCapacity {
	view.ConfiguredAvailableSlots = 0
	view.SetupNeeds = append(view.SetupNeeds, reason)
	return view
}

func cloudCapacityView(limits e2bbe.Limits, template e2bbe.Template, usage e2bbe.Usage, projectActive int, rows []e2bbe.Record, settled []e2bbe.SettledSpend, pending map[string]bool,
	actorID, projectID, region string, taskCost int64, maxSeconds int, budget int64, now time.Time) product.CoordinatorCloudCapacity {
	month := now.UTC().Format("2006-01")
	committed, unknown := int64(0), false
	retiredBillingUnconfirmed := 0
	type heldAttempt struct {
		view product.CoordinatorCloudAttempt
		at   time.Time
	}
	retained := make([]heldAttempt, 0)
	for _, row := range rows {
		if row.ResourcesHeld && row.Request.Identity.ProjectID == projectID {
			identity := row.Request.Identity
			retained = append(retained, heldAttempt{view: product.CoordinatorCloudAttempt{RunID: identity.RunID, TaskID: identity.TaskID,
				AttemptID: identity.AttemptID, OperationID: identity.OperationID, State: string(row.State),
				ResourcesHeld: true, RecoveryPending: pending[identity.OperationID], EstimatedCostMicros: row.Request.EstimatedCostMicros,
				UpdatedAt: row.UpdatedAt.UTC().Format(time.RFC3339Nano)}, at: row.UpdatedAt})
		}
		if (!row.ResourcesHeld && row.SandboxID == "") || (row.CreatedAt.UTC().Format("2006-01") != month && !row.ResourcesHeld) {
			continue
		}
		if row.Request.BillingUserID == "" {
			unknown = true
			continue
		}
		if row.Request.BillingUserID == actorID {
			if !row.ResourcesHeld && row.SandboxID != "" && !row.ProviderBillingSettled && row.CreatedAt.UTC().Format("2006-01") == month {
				retiredBillingUnconfirmed++
			}
			cost := row.Request.EstimatedCostMicros
			if cost <= 0 || cost > math.MaxInt64-committed {
				unknown = true
				continue
			}
			committed += cost
		}
	}
	for _, row := range settled {
		if !row.SandboxCreated || row.CreatedAt.UTC().Format("2006-01") != month {
			continue
		}
		if row.BillingUserID == "" {
			unknown = true
			continue
		}
		if row.BillingUserID == actorID {
			retiredBillingUnconfirmed++ // compacted rows retain no provider bill receipt
			if row.EstimatedCostMicros < 1 || row.EstimatedCostMicros > math.MaxInt64-committed {
				unknown = true
				continue
			}
			committed += row.EstimatedCostMicros
		}
	}
	available := minimumCloudSlots(limits.MaxConcurrent-usage.Active, limits.MaxPerProject-projectActive,
		int((limits.MemoryMiB-usage.MemoryMiB)/template.MemoryMiB), int((limits.CPUMillis-usage.CPUMillis)/template.CPUMillis))
	limitation := "Configured limits only. Provider health, current authority, real quota, actual billing and model access are checked separately; an observed VM remains charged until exact retirement."
	if unknown || committed > budget || taskCost > budget-committed {
		available = 0
		limitation = "The current person's cloud estimate is unavailable or over the configured monthly budget. New VMs remain refused until current authority and retained spending are reconciled."
	}
	sort.Slice(retained, func(i, j int) bool {
		if retained[i].at.Equal(retained[j].at) {
			return retained[i].view.OperationID < retained[j].view.OperationID
		}
		return retained[i].at.After(retained[j].at)
	})
	count, more := len(retained), 0
	if len(retained) > 20 {
		more = len(retained) - 20
		retained = retained[:20]
	}
	attempts := make([]product.CoordinatorCloudAttempt, 0, len(retained))
	for _, row := range retained {
		attempts = append(attempts, row.view)
	}
	advice := product.CoordinatorCloudCapacity{Provider: "e2b-embed", Region: region,
		ConfiguredGlobalSlots: limits.MaxConcurrent, ConfiguredProjectSlots: limits.MaxPerProject,
		ConfiguredAvailableSlots: available, EstimatedTaskCostMicros: taskCost,
		EstimatedMonthMicros: committed, MonthlyBudgetMicros: budget, MaxTaskSeconds: maxSeconds,
		RetiredBillingUnconfirmed: retiredBillingUnconfirmed,
		RetainedProjectVMs:        count, RetainedAttempts: attempts, MoreRetainedAttempts: more,
		ObservedAt: now.Format(time.RFC3339Nano), Limitation: limitation}
	if unknown || committed > budget || taskCost > budget-committed {
		advice.SetupNeeds = []string{"The saved cloud spending estimate cannot authorize another VM. Reconcile retained charges or update the current budget before activation."}
	}
	return advice
}
