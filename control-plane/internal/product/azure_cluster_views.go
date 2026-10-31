package product

import (
	"context"
	"sort"

	"github.com/lcfrai/luci/control-plane/internal/management"
)

type AzureClusterJobView struct {
	CloudJob
	ProjectID string `json:"projectId"`
}

func (s *Server) ListAzureClusterJobs(ctx context.Context, actor management.ExecutionActorRef) ([]AzureClusterJobView, error) {
	if s.kubernetesAuthority(ctx, actor, false, true, 0) != nil {
		return nil, management.ErrExecutionAuthority
	}
	s.mu.RLock()
	jobs := []CloudJob{}
	for _, j := range s.jobs {
		if j.Kind == "aks" && j.Plan.Foundation != nil && j.Plan.Foundation.Actor == actor {
			jobs = append(jobs, j)
		}
	}
	s.mu.RUnlock()
	if len(jobs) > 128 {
		return nil, errAzureCluster
	}
	rows := []AzureClusterJobView{}
	for _, j := range jobs {
		if !validFoundationPlan(j.Plan) {
			return nil, errAzureCluster
		}
		rows = append(rows, AzureClusterJobView{CloudJob: j, ProjectID: actor.ProjectID})
	}
	sort.Slice(rows, func(i, j int) bool { return rows[i].CreatedAt.After(rows[j].CreatedAt) })
	if s.kubernetesAuthority(ctx, actor, false, true, 0) != nil {
		return nil, management.ErrExecutionAuthority
	}
	return rows, nil
}
