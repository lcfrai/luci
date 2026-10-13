package main

import (
	"time"

	"github.com/lcfrai/luci/control-plane/internal/backend/e2bbe"
)

// The controller journals allocations before the second authority check, so
// the exact request may already be present. Count it once in that case and
// prospectively once during the initial check. Retired owned VMs still count;
// a proven no-Create refusal does not become model/provider spend. An older
// current-month allocation with no saved billing owner makes admission
// uncertain and fails closed until it is settled or the period ends.
func cloudActorMonthlyCostFits(rows []e2bbe.Record, request e2bbe.Request, now time.Time, limit int64) bool {
	if request.BillingUserID == "" || limit <= 0 || request.EstimatedCostMicros <= 0 || request.EstimatedCostMicros > limit {
		return false
	}
	month := now.UTC().Format("2006-01")
	total := int64(0)
	found := false
	for _, row := range rows {
		if !row.ResourcesHeld && row.SandboxID == "" || row.CreatedAt.UTC().Format("2006-01") != month && !row.ResourcesHeld {
			continue
		}
		if row.Request.BillingUserID == "" {
			return false
		}
		if row.Request.BillingUserID != request.BillingUserID {
			continue
		}
		if row.Request.Identity.OperationID == request.Identity.OperationID {
			if row.Request != request {
				return false
			}
			found = true
		}
		cost := row.Request.EstimatedCostMicros
		if cost < 1 || cost > limit-total {
			return false
		}
		total += cost
	}
	return found || request.EstimatedCostMicros <= limit-total
}
