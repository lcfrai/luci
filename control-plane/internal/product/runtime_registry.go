package product

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"os"
	"strings"

	"github.com/distribution/reference"
	"github.com/docker/docker/api/types/registry"
	"github.com/lcfrai/luci/control-plane/internal/management"
)

var ErrRuntimePreparation = errors.New("The reviewed runtime preparation is unavailable, changed or unconfirmed")

type RuntimeRegistry struct {
	ID          string `json:"id"`
	ProjectID   string `json:"projectId"`
	Name        string `json:"name"`
	Repository  string `json:"repository"`
	Revision    int    `json:"revision"`
	Fingerprint string `json:"fingerprint"`
}
type RuntimeRegistrySave struct {
	ID               string `json:"id,omitempty"`
	Name             string `json:"name"`
	Repository       string `json:"repository"`
	ExpectedRevision int    `json:"expectedRevision"`
	Username         string `json:"username"`
	Password         string `json:"password"`
}
type privateRuntimeRegistry struct {
	RuntimeRegistry
	Username string `json:"username"`
	Password string `json:"password"`
}
type runtimeRegistryLedger struct {
	Version      int                      `json:"version"`
	ProjectID    string                   `json:"projectId"`
	AccountScope string                   `json:"accountScope"`
	ProjectScope string                   `json:"projectScope"`
	Registries   []privateRuntimeRegistry `json:"registries"`
}

func runtimeRegistryRepository(v string) bool {
	n, err := reference.ParseNormalizedNamed(v)
	return err == nil && len(v) <= 240 && n.String() == v && reference.IsNameOnly(n) && strings.Contains(v, "/") && reference.Domain(n) != "localhost" && (strings.Contains(reference.Domain(n), ".") || strings.Contains(reference.Domain(n), ":"))
}
func runtimeRegistryFingerprint(r RuntimeRegistry) string { r.Fingerprint = ""; return reviewDigest(r) }
func validRuntimeRegistry(r privateRuntimeRegistry) bool {
	return strings.HasPrefix(r.ID, "registry-") && len(r.ID) == 41 && validConnectionToken(r.ID, 41, false) && r.ProjectID != "" && r.Revision >= 1 && r.Revision <= 1_000_000_000 && r.Name != "" && len(r.Name) <= 120 && validArtifactStoredText(r.Name) && runtimeRegistryRepository(r.Repository) && r.Fingerprint == runtimeRegistryFingerprint(r.RuntimeRegistry) && len(r.Username) <= 256 && len(r.Password) <= 8192 && strings.ContainsAny(r.Username+r.Password, "\x00\r\n") == false && (r.Username == "") == (r.Password == "")
}
func (s *Server) readRuntimeRegistries(projectID string) (string, runtimeRegistryLedger, error) {
	p, err := s.resolveProject(projectID)
	if err != nil {
		return "", runtimeRegistryLedger{}, err
	}
	s.mu.RLock()
	scope, err := s.approvalScopes(p.Path, p.ID)
	s.mu.RUnlock()
	if err != nil {
		return "", runtimeRegistryLedger{}, err
	}
	path, err := privateApplicationStatePathForRoot(s.config.ProjectsDir, s.stateDir, "runtime-registries-"+projectID)
	empty := runtimeRegistryLedger{Version: 1, ProjectID: projectID, AccountScope: scope.AccountScope, ProjectScope: scope.ProjectScope, Registries: []privateRuntimeRegistry{}}
	if err != nil {
		return "", empty, err
	}
	var saved runtimeRegistryLedger
	err = externalReadJSON(path, 4<<20, &saved)
	if errors.Is(err, os.ErrNotExist) {
		return path, empty, nil
	}
	if err != nil || saved.Version != 1 || saved.ProjectID != projectID || saved.AccountScope != empty.AccountScope || saved.ProjectScope != empty.ProjectScope || saved.Registries == nil || len(saved.Registries) > 32 {
		return "", empty, ErrRuntimePreparation
	}
	ids := map[string]bool{}
	for _, r := range saved.Registries {
		if !validRuntimeRegistry(r) || r.ProjectID != projectID || ids[r.ID] {
			return "", empty, ErrRuntimePreparation
		}
		ids[r.ID] = true
	}
	return path, saved, nil
}
func (s *Server) ListRuntimeRegistries(ctx context.Context, actor management.ExecutionActorRef) ([]RuntimeRegistry, error) {
	if err := s.kubernetesAuthority(ctx, actor, false, true, 0); err != nil {
		return nil, err
	}
	_, ledger, err := s.readRuntimeRegistries(actor.ProjectID)
	if err != nil {
		return nil, err
	}
	out := []RuntimeRegistry{}
	for _, r := range ledger.Registries {
		out = append(out, r.RuntimeRegistry)
	}
	if err = s.kubernetesAuthority(ctx, actor, false, true, 0); err != nil {
		return nil, err
	}
	return out, nil
}
func (s *Server) SaveRuntimeRegistry(ctx context.Context, actor management.ExecutionActorRef, input RuntimeRegistrySave) (RuntimeRegistry, error) {
	if err := s.kubernetesAuthority(ctx, actor, false, true, 0); err != nil {
		return RuntimeRegistry{}, err
	}
	id := input.ID
	if id == "" {
		if input.ExpectedRevision != 0 {
			return RuntimeRegistry{}, ErrRuntimePreparation
		}
		var err error
		id, err = kubeOpaqueID("registry-")
		if err != nil {
			return RuntimeRegistry{}, err
		}
	}
	row := privateRuntimeRegistry{RuntimeRegistry: RuntimeRegistry{ID: id, ProjectID: actor.ProjectID, Name: strings.TrimSpace(input.Name), Repository: input.Repository, Revision: input.ExpectedRevision + 1}, Username: input.Username, Password: input.Password}
	row.Fingerprint = runtimeRegistryFingerprint(row.RuntimeRegistry)
	if !validRuntimeRegistry(row) {
		return RuntimeRegistry{}, ErrRuntimePreparation
	}
	s.runtimePreparationMu.Lock()
	defer s.runtimePreparationMu.Unlock()
	path, ledger, err := s.readRuntimeRegistries(actor.ProjectID)
	if err != nil {
		return RuntimeRegistry{}, err
	}
	found := -1
	for i, r := range ledger.Registries {
		if r.ID == id {
			found = i
			if r.Revision != input.ExpectedRevision {
				return RuntimeRegistry{}, ErrRuntimePreparation
			}
		}
	}
	if found < 0 && (input.ExpectedRevision != 0 || input.ID != "" || len(ledger.Registries) >= 32) {
		return RuntimeRegistry{}, ErrRuntimePreparation
	}
	if found < 0 {
		ledger.Registries = append(ledger.Registries, row)
	} else {
		ledger.Registries[found] = row
	}
	raw, err := json.MarshalIndent(ledger, "", "  ")
	if err != nil || len(raw)+1 > 4<<20 {
		return RuntimeRegistry{}, ErrRuntimePreparation
	}
	if err = s.kubernetesAuthority(ctx, actor, false, true, 0); err != nil {
		return RuntimeRegistry{}, err
	}
	if err = durableRunJSON(path, ledger); err != nil {
		return RuntimeRegistry{}, err
	}
	if err = s.kubernetesAuthority(ctx, actor, false, true, 0); err != nil {
		return RuntimeRegistry{}, err
	}
	return row.RuntimeRegistry, nil
}
func (s *Server) runtimeRegistryCredentials(projectID string, want RuntimeRegistry) (string, error) {
	_, ledger, err := s.readRuntimeRegistries(projectID)
	if err != nil {
		return "", err
	}
	for _, row := range ledger.Registries {
		if row.ID == want.ID {
			if row.RuntimeRegistry != want {
				return "", ErrRuntimePreparation
			}
			// Explicit saved credentials only, never $HOME/.docker/config.json,
			// credential helpers, process environment or the coding agent's login.
			auth := registry.AuthConfig{Username: row.Username, Password: row.Password, ServerAddress: reference.Domain(mustRuntimeRegistryName(row.Repository))}
			raw, err := json.Marshal(auth)
			if err != nil {
				return "", err
			}
			return base64.URLEncoding.EncodeToString(raw), nil
		}
	}
	return "", ErrRuntimePreparation
}
func mustRuntimeRegistryName(v string) reference.Named {
	n, _ := reference.ParseNormalizedNamed(v)
	return n
}
