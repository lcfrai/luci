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
	cloud := a.deps.CloudTasks
	if cloud == nil {
		return product.CoordinatorCloudCapacity{}, nil
	}
	actor, policy, present := management.ActorFromContext(ctx)
	if !present || actor.ID != actorID || !policy.AllowCloudProvisioning || policy.MonthlyBudgetUSD <= 0 {
		return product.CoordinatorCloudCapacity{}, nil
	}
	ref, err := management.CaptureExecutionActor(ctx, projectID)
	if err != nil || ref.Local {
		return product.CoordinatorCloudCapacity{}, nil
	}
	if _, err = a.authorizeExecution(ctx, ref, "codex", true); err != nil {
		if _, err = a.authorizeExecution(ctx, ref, "claude-code", true); err != nil {
			return product.CoordinatorCloudCapacity{}, nil
		}
	}
	budget := int64(math.Floor(policy.MonthlyBudgetUSD * 1_000_000))
	if budget < 0 {
		budget = 0
	}
	limits, template, usage, projectActive, rows := cloud.Runtime.Controller.Capacity(projectID)
	return cloudCapacityView(limits, template, usage, projectActive, rows, actorID, projectID, cloud.Runtime.Controller.Region(), cloud.EstimatedCostMicros, cloud.MaxSeconds, budget, time.Now().UTC()), nil
}

func cloudCapacityView(limits e2bbe.Limits, template e2bbe.Template, usage e2bbe.Usage, projectActive int, rows []e2bbe.Record,
	actorID, projectID, region string, taskCost int64, maxSeconds int, budget int64, now time.Time) product.CoordinatorCloudCapacity {
	month := now.UTC().Format("2006-01")
	committed, unknown := int64(0), false
	retained := make([]product.CoordinatorCloudAttempt, 0)
	for _, row := range rows {
		if row.ResourcesHeld && row.Request.Identity.ProjectID == projectID {
			identity := row.Request.Identity
			retained = append(retained, product.CoordinatorCloudAttempt{RunID: identity.RunID, TaskID: identity.TaskID,
				AttemptID: identity.AttemptID, OperationID: identity.OperationID, State: string(row.State),
				ResourcesHeld: true, EstimatedCostMicros: row.Request.EstimatedCostMicros,
				UpdatedAt: row.UpdatedAt.UTC().Format(time.RFC3339Nano)})
		}
		if (!row.ResourcesHeld && row.SandboxID == "") || (row.CreatedAt.UTC().Format("2006-01") != month && !row.ResourcesHeld) {
			continue
		}
		if row.Request.BillingUserID == "" {
			unknown = true
			continue
		}
		if row.Request.BillingUserID == actorID {
			cost := row.Request.EstimatedCostMicros
			if cost <= 0 || cost > math.MaxInt64-committed {
				unknown = true
				continue
			}
			committed += cost
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
		if retained[i].UpdatedAt == retained[j].UpdatedAt {
			return retained[i].OperationID < retained[j].OperationID
		}
		return retained[i].UpdatedAt > retained[j].UpdatedAt
	})
	count, more := len(retained), 0
	if len(retained) > 20 {
		more = len(retained) - 20
		retained = retained[:20]
	}
	return product.CoordinatorCloudCapacity{Provider: "e2b-embed", Region: region,
		ConfiguredGlobalSlots: limits.MaxConcurrent, ConfiguredProjectSlots: limits.MaxPerProject,
		ConfiguredAvailableSlots: available, EstimatedTaskCostMicros: taskCost,
		EstimatedMonthMicros: committed, MonthlyBudgetMicros: budget, MaxTaskSeconds: maxSeconds,
		RetainedProjectVMs: count, RetainedAttempts: retained, MoreRetainedAttempts: more,
		ObservedAt: now.Format(time.RFC3339Nano), Limitation: limitation}
}
