package platform

import (
	"context"
	"errors"
	"strings"
	"time"
)

// ProjectRunDeploymentObservation is a fresh read of a saved deployment. It
// contains no resource control, private environment or provider credentials.
type ProjectRunDeploymentObservation struct {
	Deployment  Deployment
	ObservedAt  string
	Limitations string
}

// ObserveProjectRunDeployment never repairs or launches a resource. Historical
// ready rows require current ownership, exact source identity and actual health.
func (s *Service) ObserveProjectRunDeployment(ctx context.Context, projectID, releaseID string) (ProjectRunDeploymentObservation, error) {
	var out ProjectRunDeploymentObservation
	if !identifier.MatchString(projectID) || !identifier.MatchString(releaseID) || s.opts.ResolveProject == nil {
		return out, errors.New("invalid native release selection")
	}
	p, err := s.opts.ResolveProject(projectID)
	if err != nil || p.ID != projectID {
		return out, errors.New("native release project is unavailable")
	}
	d, err := s.findDeployment(projectID, releaseID)
	if err != nil || d.ProjectID != projectID || d.ID != releaseID || d.Status != "ready" {
		return out, errors.New("native release has no recorded ready outcome")
	}
	bounded, cancel := context.WithTimeout(ctx, 8*time.Second)
	defer cancel()
	switch d.Provider {
	case "docker":
		var info containerInfo
		var err error
		if d.Runtime != nil {
			info, err = s.inspectAdmittedDeployment(bounded, d)
		} else {
			info, err = s.inspect(bounded, projectID, d.ID, d.Container)
		}
		if err != nil || !info.State.Running || d.Image == "" || info.Image != d.Image || (d.Runtime == nil && info.Config.Image != "luci-platform/"+s.instance+":"+d.ID) {
			return out, errors.New("native release running container and image ownership changed")
		}
		address, err := loopbackURL(info, d.Port)
		if err != nil || address != d.URL || !validHealthPath(d.HealthPath) || !s.healthy(bounded, address, d.HealthPath) {
			return out, errors.New("native release port or current health is unverified")
		}
		if err := s.checkDeploymentDatabaseBinding(projectID, d, info); err != nil {
			return out, errors.New("native release database binding changed; redeploy against current data")
		}
		out.Limitations = "Owned native Docker image, loopback binding and application health were observed. This is not public publication or full behavior acceptance."
	case "panacea":
		if err := s.observeProjectRunPanacea(bounded, d); err != nil {
			return out, err
		}
		out.Limitations = "The owned provider connection, exact deployment commit, live ready replicas and HTTPS health were observed. Full browser behavior and inbox delivery are not established."
	default:
		return out, errors.New("native provider has no supported ownership and health observation")
	}
	if err := bounded.Err(); err != nil {
		return out, err
	}
	out.Deployment, out.ObservedAt = d, time.Now().UTC().Format(time.RFC3339Nano)
	return out, nil
}

func (s *Service) observeProjectRunPanacea(ctx context.Context, d Deployment) error {
	c, err := s.connection()
	if err != nil || c.binding() != d.ProviderBinding || !remoteName.MatchString(d.RemoteProject) || !identifier.MatchString(d.PipelineRun) {
		return errors.New("native provider connection or deployment ownership changed")
	}
	var history []struct {
		ID                string `json:"id"`
		Commit            string `json:"commit"`
		Status            string `json:"status"`
		Image             string `json:"image"`
		LiveReadyReplicas *int   `json:"liveReadyReplicas"`
	}
	if err := s.provider(ctx, c, "GET", "/api/v1/projects/"+d.RemoteProject+"/deployments", nil, &history); err != nil {
		return errors.New("native provider deployment history is unavailable")
	}
	// Multiple live revisions cannot establish which source answers this URL.
	found := false
	for _, item := range history {
		if item.LiveReadyReplicas == nil || *item.LiveReadyReplicas < 1 {
			continue
		}
		if found || item.ID != d.PipelineRun || item.Commit != d.Commit || item.Image != d.Image || !strings.EqualFold(item.Status, "succeeded") {
			return errors.New("native provider has a different or ambiguous live deployment")
		}
		found = true
	}
	if !found {
		return errors.New("native provider has not confirmed live replicas of the exact release")
	}
	var remote RemoteProject
	if err := s.provider(ctx, c, "GET", "/api/v1/projects/"+d.RemoteProject, nil, &remote); err != nil || remote.Name != d.RemoteProject {
		return errors.New("native provider project identity could not be confirmed")
	}
	address, err := connectionURL(remote.URL)
	if err != nil || address != d.URL || !s.healthy(ctx, address, "/") {
		return errors.New("native provider release has no current matching HTTPS health")
	}
	return nil
}
