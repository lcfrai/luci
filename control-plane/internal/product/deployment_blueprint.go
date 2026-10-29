package product

import (
	"context"
	"fmt"

	"github.com/lcfrai/luci/control-plane/internal/deploymentplan"
)

// DeploymentBlueprint performs pure compilation after API-level project access
// checks. It does not resolve credentials, inspect Git, persist the plan, or
// enter an existing deployment executor. Supplied source is intent, not proof.
func (s *Server) DeploymentBlueprint(ctx context.Context, projectID string, request deploymentplan.Blueprint) (deploymentplan.Plan, error) {
	if projectID == "" || request.ProjectID != projectID {
		return deploymentplan.Plan{}, fmt.Errorf("blueprint must belong to the requested project")
	}
	return (deploymentplan.ManifestPlanner{}).Plan(ctx, request)
}
