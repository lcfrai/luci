package platform

import (
	"context"
	"errors"
	"reflect"
	"time"
)

// ApplicationPrototypeRelease contains source provenance only. Opening the app
// still requires the existing owned browser bridge; private ports never leave it.
type ApplicationPrototypeRelease struct {
	ID                    string
	ProjectID             string
	Commit                string
	ConfigurationRevision int
	CreatedAt             string
}

func (s *Service) ApplicationPrototypeReleases(ctx context.Context, projectID string) ([]ApplicationPrototypeRelease, error) {
	p, err := s.opts.ResolveProject(projectID)
	if err != nil || p.ID != projectID {
		return nil, errors.New("the application project is unavailable")
	}
	s.mu.Lock()
	state, err := s.load(projectID)
	busy := s.busy[projectID]
	s.mu.Unlock()
	if err != nil {
		return nil, errors.New("application deployment history is unavailable")
	}
	result := []ApplicationPrototypeRelease{}
	if busy {
		return result, nil
	}
	ctx, cancel := context.WithTimeout(ctx, 8*time.Second)
	defer cancel()
	seen := map[string]bool{}
	for _, d := range state.Deployments {
		if d.Provider != "docker" || d.Status != "ready" {
			continue
		}
		recorded := d
		if d.Runtime != nil {
			snapshot := *d.Runtime
			recorded.Runtime = &snapshot
		}
		if d.ProjectID != projectID || !identifier.MatchString(d.ID) || seen[d.ID] || !commitID.MatchString(d.Commit) || d.Port < 1 || d.Port > 65535 || !validHealthPath(d.HealthPath) {
			return nil, errors.New("application deployment provenance is invalid")
		}
		seen[d.ID] = true
		if err := ctx.Err(); err != nil {
			return nil, err
		}
		if d.Runtime != nil {
			observed, err := s.observeAdmittedDeployment(ctx, d)
			if err != nil {
				continue
			}
			d = observed
		}
		if !identifier.MatchString(d.Container) || d.Image == "" {
			return nil, errors.New("application deployment provenance is invalid")
		}
		info, err := s.inspect(ctx, projectID, d.ID, d.Container)
		if err != nil || !info.State.Running || info.Image != d.Image || info.State.Health.Status == "unhealthy" || info.State.Health.Status == "starting" {
			continue
		}
		address, err := loopbackURL(info, d.Port)
		if err != nil || !s.healthy(ctx, address, d.HealthPath) {
			continue
		}
		// A stop/restart admitted while HTTP health was in flight invalidates
		// this observation. Do not offer an old ready record after it changed.
		s.mu.Lock()
		latest, stateErr := s.load(projectID)
		unchanged := !s.busy[projectID]
		s.mu.Unlock()
		if stateErr != nil {
			return nil, errors.New("application deployment history is unavailable")
		}
		matching := false
		for _, current := range latest.Deployments {
			if current.ID == recorded.ID && reflect.DeepEqual(current, recorded) {
				matching = true
			}
		}
		if !unchanged || !matching {
			continue
		}
		result = append(result, ApplicationPrototypeRelease{ID: d.ID, ProjectID: projectID, Commit: d.Commit, ConfigurationRevision: d.ConfigurationRevision, CreatedAt: d.CreatedAt})
	}
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	return result, nil
}
