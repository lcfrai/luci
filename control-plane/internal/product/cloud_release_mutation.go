package product

import (
	"context"
	"errors"
	"net/http"
	"strings"

	"github.com/lcfrai/luci/control-plane/internal/management"
	"github.com/lcfrai/luci/control-plane/internal/project"
)

// Manual operations retain the request's original actor and connection. This
// installs no new provider action and grants nothing: each existing mutating
// Coolify request must pass fresh authority before AND after ownership reads.
// A later refusal leaves already-issued changes/intent retained, never rolled
// back or replayed. These checks are not an atomic provider authorization lease.
func (s *Server) bindManualCloudReleaseMutations(ctx context.Context, p project.Project, target DeploymentTarget, row *CloudRelease, allocate bool, client *CoolifyClient) error {
	actor, err := management.CaptureDeploymentActor(ctx, p.ID)
	if err != nil || s.coordinatorDeploymentAuthorize == nil || row == nil || client == nil {
		return management.ErrExecutionAuthority
	}
	authorize := func(current context.Context) error {
		if current.Err() != nil {
			return current.Err()
		}
		authority, err := s.coordinatorDeploymentAuthorize(current, actor, allocate)
		if err != nil {
			return management.ErrExecutionAuthority
		}
		// The existing unmanaged-local authorizer may explicitly permit Local;
		// do not turn its empty authority into a synthetic authenticated user.
		if actor.Local {
			if actor.UserID != "" || actor.TeamID != "" || authority.Actor.ID != "" || authority.Team != nil {
				return management.ErrExecutionAuthority
			}
		} else {
			refreshed := management.WithExecutionAuthority(current, authority)
			actual, err := management.CaptureDeploymentActor(refreshed, p.ID)
			if err != nil || actual != actor || allocate && !authority.Policy.AllowPublicPublishing {
				return management.ErrExecutionAuthority
			}
		}
		return current.Err()
	}
	if err := authorize(ctx); err != nil {
		return err
	}
	client.beforeMutation = func(current context.Context) error {
		if err := authorize(current); err != nil {
			return err
		}
		rows, err := s.validateManualCloudReleaseSnapshot(p, target, *row, allocate)
		if err != nil {
			return err
		}
		if err := validateManualCloudApplication(current, client, *row, rows); err != nil {
			return err
		}
		// Provider reads may block. Recapture the SAME actor's current grants
		// and reread the original connection/configuration immediately before
		// handing control back to the one existing HTTP mutation.
		if err := authorize(current); err != nil {
			return err
		}
		_, err = s.validateManualCloudReleaseSnapshot(p, target, *row, allocate)
		if err != nil {
			return err
		}
		return current.Err()
	}
	return nil
}

func (s *Server) validateManualCloudReleaseSnapshot(p project.Project, target DeploymentTarget, row CloudRelease, allocate bool) ([]CloudRelease, error) {
	actual, err := s.resolveProject(p.ID)
	if err != nil || actual.ID != p.ID || actual.Path != p.Path {
		return nil, ErrOrchestratorConflict
	}
	if row.ProjectID != p.ID || row.TargetID != target.ID || row.TargetFingerprint != targetFingerprint(target) || row.ServerUUID != target.ServerUUID {
		return nil, ErrOrchestratorConflict
	}
	s.mu.RLock()
	current, exists := s.targets[target.ID]
	rows, err := s.readCloudReleases(p.ID)
	s.mu.RUnlock()
	// Fingerprints intentionally tolerate tunnel ports across REQUESTS. This
	// in-flight client cannot adopt a replacement endpoint/token/server.
	if err != nil || !exists || current.ID != target.ID || current.ServerUUID != target.ServerUUID || current.BaseURL != target.BaseURL || current.Token != target.Token {
		return nil, ErrOrchestratorConflict
	}
	found := false
	for _, saved := range rows {
		if saved.ID == row.ID {
			found = saved.CloudReleaseRequest == row.CloudReleaseRequest && saved.Release.Commit == row.Release.Commit && saved.ConfigurationRevision == row.ConfigurationRevision && saved.TargetFingerprint == row.TargetFingerprint && saved.ServerUUID == row.ServerUUID && saved.ApplicationID == row.ApplicationID && saved.ProviderProjectID == row.ProviderProjectID && saved.DataDomainRevision == row.DataDomainRevision && saved.DataBindingFingerprint == row.DataBindingFingerprint && saved.CoordinatorOriginDigest == row.CoordinatorOriginDigest
			break
		}
	}
	if !found {
		return nil, ErrOrchestratorConflict
	}
	// Stop remains possible after configuration changed or allocations were
	// suspended. A new build must keep the exact selected configuration.
	if allocate {
		config, err := s.applicationConfigurationSnapshot(p.ID)
		if err != nil || config.Revision != row.ConfigurationRevision {
			return nil, ErrOrchestratorConflict
		}
	}
	return rows, nil
}

func validateManualCloudApplication(ctx context.Context, client *CoolifyClient, row CloudRelease, rows []CloudRelease) error {
	if row.ApplicationID == "" {
		return nil
	}
	var app struct {
		UUID       string `json:"uuid"`
		ServerUUID string `json:"server_uuid"`
		Repository string `json:"git_repository"`
		Branch     string `json:"git_branch"`
		Commit     string `json:"git_commit_sha"`
		Domain     string `json:"fqdn"`
	}
	if err := client.request(ctx, http.MethodGet, "/applications/"+row.ApplicationID, nil, nil, &app); err != nil {
		return err
	}
	repository := row.Repository
	if row.Source == "private" {
		owner, name, err := githubDeploymentRepository(repository)
		if err != nil {
			return err
		}
		repository = "git@github.com:" + owner + "/" + name + ".git"
	}
	if app.UUID != row.ApplicationID || app.ServerUUID != row.ServerUUID || app.Repository != repository || strings.TrimSuffix(app.Domain, "/") != row.Domain {
		return errors.New("The provider application no longer matches this exact owned release.")
	}
	known := app.Branch == row.Branch && app.Commit == row.Release.Commit
	for _, prior := range rows {
		if prior.ApplicationID == row.ApplicationID && prior.TargetFingerprint == row.TargetFingerprint && prior.Repository == row.Repository && prior.Branch == app.Branch && prior.Domain == row.Domain && prior.Status == "ready" && prior.Release.Commit == app.Commit {
			known = true
		}
	}
	if !known {
		return errors.New("The provider application source no longer matches retained owned release history.")
	}
	return ctx.Err()
}
