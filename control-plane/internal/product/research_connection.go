package product

import (
	"errors"
	"net/http"
	"os"

	"github.com/lcfrai/luci/control-plane/internal/management"
	"github.com/lcfrai/luci/control-plane/internal/project"
)

// This credential belongs to the research connector, never application runtime
// configuration. Only the private disk representation contains its value.
type privateResearchConnection struct {
	Version      int    `json:"version"`
	AccountScope string `json:"accountScope"`
	ProjectScope string `json:"projectScope"`
	ProjectID    string `json:"projectId"`
	TeamID       string `json:"teamId"`
	Revision     int    `json:"revision"`
	Key          string `json:"key"`
}
type ResearchConnection struct {
	Provider   string `json:"provider"`
	Revision   int    `json:"revision"`
	Configured bool   `json:"configured"`
}

func validResearchKey(key string) bool {
	if key == "" {
		return true
	}
	if len(key) < 10 || len(key) > 4096 {
		return false
	}
	for _, r := range key {
		if r < 33 || r > 126 {
			return false
		}
	}
	return true
}
func researchConnectionMetadata(row privateResearchConnection) ResearchConnection {
	return ResearchConnection{"brave", row.Revision, row.Key != ""}
}

// Caller holds s.mu. Scope is the same private account/project namespace as
// other project connectors, plus the current team identity. No ambient key.
func (s *Server) researchConnectionLocked(p project.Project, ref management.ExecutionActorRef) (string, privateResearchConnection, error) {
	scope, err := s.approvalScopes(p.Path, p.ID)
	if err != nil {
		return "", privateResearchConnection{}, err
	}
	path, err := privateApplicationStatePathForRoot(s.config.ProjectsDir, s.stateDir, "research-connection-"+reviewDigest([]string{p.ID, ref.TeamID}))
	if err != nil {
		return "", privateResearchConnection{}, err
	}
	empty := privateResearchConnection{Version: 1, AccountScope: scope.AccountScope, ProjectScope: scope.ProjectScope, ProjectID: p.ID, TeamID: ref.TeamID}
	var saved privateResearchConnection
	err = externalReadJSON(path, 16<<10, &saved)
	if errors.Is(err, os.ErrNotExist) {
		return path, empty, nil
	}
	if err != nil || saved.Version != 1 || saved.AccountScope != empty.AccountScope || saved.ProjectScope != empty.ProjectScope || saved.ProjectID != p.ID || saved.TeamID != ref.TeamID || saved.Revision < 1 || saved.Revision >= 1_000_000_000 || !validResearchKey(saved.Key) {
		return "", empty, errResearch
	}
	return path, saved, nil
}
func (s *Server) researchConnection(w http.ResponseWriter, r *http.Request) {
	p, ok := s.requestProject(w, r)
	if !ok {
		return
	}
	ref, err := s.researchAccess(r.Context(), p.ID, true)
	if err != nil {
		problem(w, 403, "research_access", "Current project Edit and Secrets access are required.")
		return
	}
	if r.Method == http.MethodGet || r.Method == http.MethodHead {
		s.mu.RLock()
		_, row, e := s.researchConnectionLocked(p, ref)
		s.mu.RUnlock()
		if e != nil || s.researchAuthorize(r.Context(), ref, true) != nil {
			problem(w, 403, "research_access", "Current research connection is unavailable.")
			return
		}
		journeyJSON(w, 200, researchConnectionMetadata(row))
		return
	}
	var req struct {
		ExpectedRevision int    `json:"expectedRevision"`
		Key              string `json:"key"`
		Remove           bool   `json:"remove"`
	}
	if !journeyDecode(w, r, &req) {
		return
	}
	if req.ExpectedRevision < 0 || req.Remove && req.Key != "" || !req.Remove && req.Key == "" || !validResearchKey(req.Key) {
		problem(w, 400, "research_connection", "Provide one explicit key update or removal and its current revision.")
		return
	}
	s.mu.Lock()
	path, row, err := s.researchConnectionLocked(p, ref)
	if err == nil {
		if row.Revision != req.ExpectedRevision {
			err = ErrOrchestratorConflict
		} else if s.researchAuthorize(r.Context(), ref, true) != nil {
			err = management.ErrExecutionAuthority
		} else {
			row.Revision++
			row.Key = req.Key
			err = atomicJSON(path, row)
		}
	}
	s.mu.Unlock()
	if err != nil {
		problem(w, 409, "research_connection_changed", "Read current connection metadata before changing its private key.")
		return
	}
	if s.researchAuthorize(r.Context(), ref, true) != nil {
		problem(w, 403, "research_access", "The connection update may be retained; current access changed. Read its metadata after access is restored.")
		return
	}
	journeyJSON(w, 200, researchConnectionMetadata(row))
}
