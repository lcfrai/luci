package deploymentplan

import "context"

// ManifestPlanner is an actual Plan-only connector. It cannot observe a cluster
// or apply manifests and therefore deliberately does not implement Connector.
type ManifestPlanner struct{}

var _ Planner = ManifestPlanner{}

func (ManifestPlanner) Plan(ctx context.Context, blueprint Blueprint) (Plan, error) {
	if err := ctx.Err(); err != nil {
		return Plan{}, err
	}
	plan, err := Compile(blueprint)
	if err != nil {
		return Plan{}, err
	}
	if err := ctx.Err(); err != nil {
		return Plan{}, err
	}
	return plan, nil
}
