package api

import (
	"context"
	"errors"
	"net/http"
	"strings"

	"github.com/lcfrai/luci/control-plane/internal/backend"
	"github.com/lcfrai/luci/control-plane/internal/harness"
	"github.com/lcfrai/luci/control-plane/internal/httpx"
	"github.com/lcfrai/luci/control-plane/internal/product"
	"github.com/lcfrai/luci/control-plane/internal/registry"
)

func (a *API) validateProjectRunSessionRequest(w http.ResponseWriter, req *sessionCreateRequest) bool {
	req.ProjectRunID = strings.TrimSpace(req.ProjectRunID)
	req.ProjectRunOperationID = strings.TrimSpace(req.ProjectRunOperationID)
	if (req.ProjectRunID == "") != (req.ProjectRunOperationID == "") {
		httpx.WriteProblem(w, http.StatusBadRequest, httpx.CodeBadRequest, "projectRunId and projectRunOperationId must be supplied together.", nil)
		return false
	}
	if req.ProjectRunID != "" && (a.deps.Product == nil || req.AuthOnly) {
		httpx.WriteProblem(w, http.StatusConflict, httpx.CodeConflict, "A project run requires its reserved coding operation in the project workspace.", nil)
		return false
	}
	if req.ProjectRunID != "" && strings.TrimSpace(req.Resume) != "" {
		httpx.WriteProblem(w, http.StatusConflict, httpx.CodeConflict, "A reserved project run operation starts a fresh coding conversation. Keep the earlier attempt in history and continue with the saved reviewed inputs.", nil)
		return false
	}
	return true
}

func validateProjectRunResume(w http.ResponseWriter, req sessionCreateRequest, source registry.Session) bool {
	if source.ProjectRunID == "" && source.ProjectRunOperationID == "" {
		return true
	}
	httpx.WriteProblem(w, http.StatusConflict, httpx.CodeConflict, "Reserve a fresh coding operation in this project run to continue its reviewed inputs. Earlier conversations stay with their original attempts.", nil)
	return false
}

// Repeated dispatch reads its already allocated child before resolving mutable
// defaults or checking whether that child's subscription account is busy. A
// missing child after dispatch began is an unknown outcome, never a new create.
func (a *API) reuseProjectRunSession(w http.ResponseWriter, r *http.Request, req sessionCreateRequest, projectID string) bool {
	if req.ProjectRunID == "" {
		return false
	}
	existing, err := a.deps.Product.FindProjectRunSession(projectID, req.ProjectRunID, req.ProjectRunOperationID, a.deps.Store.Get)
	if err != nil {
		httpx.WriteProblem(w, http.StatusConflict, httpx.CodeConflict, err.Error(), nil)
		return true
	}
	if existing == nil {
		return false
	}
	if !projectRunSessionRequestMatches(req, *existing) {
		httpx.WriteProblem(w, http.StatusConflict, httpx.CodeConflict, "This coding operation already names a session with different reviewed inputs or model settings. Keep its saved selection or reserve a new operation.", nil)
		return true
	}
	w.Header().Set("Location", sessionBasePath+"/sessions/"+existing.ID)
	httpx.WriteJSON(w, http.StatusOK, a.sessionJSON(*existing, sessionBasePath))
	return true
}

func projectRunSessionRequestMatches(req sessionCreateRequest, existing registry.Session) bool {
	harnessID := strings.TrimSpace(req.Harness)
	if harnessID == "" {
		harnessID = string(harness.ClaudeCode)
	}
	permission := strings.TrimSpace(req.PermissionMode)
	if permission == "" {
		permission = string(harness.PermissionAsk)
	}
	if req.ProjectRunID != existing.ProjectRunID || req.ProjectRunOperationID != existing.ProjectRunOperationID || strings.TrimSpace(req.WorktreeID) != existing.WorktreeID || harnessID != existing.Harness || permission != existing.PermissionMode || strings.TrimSpace(req.Effort) != existing.Effort || req.AuthOnly {
		return false
	}
	if req.AgentConnectionID != nil && *req.AgentConnectionID != existing.AgentConnectionID {
		return false
	}
	model := strings.TrimSpace(req.Model)
	if model != "" && model != "default" && model != existing.Model && !(existing.Harness == "opencode" && "luci-gateway/"+model == existing.Model) {
		return false
	}
	if req.ImplementationContext == nil || existing.ImplementationContext == nil || !sameImplementationSelection(req.ImplementationContext, product.ImplementationContextSelection(existing.ImplementationContext)) {
		return false
	}
	prompts := []string{product.ImplementationOpeningPrompt(existing.ImplementationContext), product.PreviousCapturedImplementationOpeningPrompt(existing.ImplementationContext)}
	if existing.ReviewedContext == nil {
		prompts = append(prompts, product.LegacyImplementationOpeningPrompt(existing.ImplementationContext))
	}
	for _, prompt := range prompts {
		if strings.TrimSpace(req.OpeningPrompt) != "" {
			prompt += "\n\nAdditional direction supplied for this run:\n" + req.OpeningPrompt
		}
		if prompt == existing.OpeningPrompt {
			return true
		}
	}
	return false
}

func (a *API) beginProjectRunSession(w http.ResponseWriter, r *http.Request, req sessionCreateRequest, candidate registry.Session, capture *product.ReviewedContextCapture) (product.ProjectRunSessionAdmission, bool) {
	assignment := ""
	if capture != nil {
		assignment = capture.ReviewAssignmentDigest
	}
	admission, err := a.deps.Product.BeginProjectRunSessionWithAssignment(candidate.ProjectID, req.ProjectRunID, req.ProjectRunOperationID, candidate, assignment, a.deps.Store.Get)
	if err != nil {
		httpx.WriteProblem(w, http.StatusConflict, httpx.CodeConflict, err.Error(), nil)
		return product.ProjectRunSessionAdmission{}, false
	}
	if admission.Release == nil || admission.SessionID == "" || admission.Existing == nil && admission.Revalidate == nil {
		if admission.Release != nil {
			admission.Release()
		}
		a.internal(w, r, "admit reserved project run session", errors.New("incomplete project run admission"))
		return product.ProjectRunSessionAdmission{}, false
	}
	if admission.Existing != nil {
		defer admission.Release()
		if !projectRunSessionRequestMatches(req, *admission.Existing) {
			httpx.WriteProblem(w, http.StatusConflict, httpx.CodeConflict, "This coding operation already names a session with different reviewed inputs or model settings. Keep its saved selection or reserve a new operation.", nil)
			return product.ProjectRunSessionAdmission{}, false
		}
		w.Header().Set("Location", sessionBasePath+"/sessions/"+admission.SessionID)
		httpx.WriteJSON(w, http.StatusOK, a.sessionJSON(*admission.Existing, sessionBasePath))
		return product.ProjectRunSessionAdmission{}, false
	}
	return admission, true
}

// ResolveProjectRunSelection reuses session route and harness resolution without
// allocating a child, leasing an account or rewriting private provider state.
// Actual session admission repeats these checks at the time of allocation.
func (a *API) ResolveProjectRunSelection(ctx context.Context, req product.ProjectRunSelectionRequest) (product.ProjectRunSelection, error) {
	var result product.ProjectRunSelection
	if err := ctx.Err(); err != nil {
		return result, err
	}
	if a.deps.Product == nil {
		return result, errors.New("Project runs are unavailable in this workspace.")
	}
	req.ProjectID = strings.TrimSpace(req.ProjectID)
	req.WorktreeID = strings.TrimSpace(req.WorktreeID)
	if req.WorktreeID == "" || req.ProjectID == "" {
		return result, errors.New("Choose the project worktree before saving a run.")
	}
	release, err := a.deps.Product.AcquireGitSessionAdmission(ctx, req.ProjectID)
	if err != nil {
		return result, err
	}
	defer release()
	path, err := a.deps.Worktrees.Path(ctx, req.WorktreeID)
	if err != nil {
		return result, errors.New("The selected worktree is unavailable in this workspace.")
	}
	projectID, _ := a.sessionLocate(ctx, req.WorktreeID)
	if projectID != req.ProjectID {
		return result, errors.New("The selected worktree belongs to a different project.")
	}
	harnessID := harness.ID(strings.TrimSpace(req.Harness))
	if harnessID == "" {
		harnessID = harness.ClaudeCode
	}
	desc, known := harness.Lookup(harnessID)
	if !known || harnessID == harness.Shell {
		return result, errors.New("Choose a coding agent for the project run.")
	}
	model := strings.TrimSpace(req.Model)
	if model == "" {
		model = desc.DefaultModel
	}
	mode := harness.PermissionMode(strings.TrimSpace(req.PermissionMode))
	if mode == "" {
		mode = harness.PermissionAsk
	}
	effort := harness.Effort(strings.TrimSpace(req.Effort))
	connection, env, boundModel, err := a.resolveSessionAgent(string(harnessID), req.AgentConnectionID, registry.Session{}, strings.TrimSpace(req.Model))
	if err != nil {
		return result, err
	}
	if boundModel != "" {
		model = boundModel
	}
	if harnessID == harness.OpenCode && env["OPENCODE_CONFIG_CONTENT"] == "" {
		return result, errors.New("Configure an explicit local or compatible model in Settings before starting OpenCode.")
	}
	_, _, err = harness.Build(harness.Request{DockerBoundary: a.deps.Backend.Kind() == backend.KindDocker, ProjectBrief: true, Harness: harnessID, Model: model, Effort: effort, Permission: mode})
	if err != nil {
		return result, err
	}
	authMode, generation := connection.AuthMode, ""
	if harnessID == harness.Codex && (connection.Kind == "legacy" || connection.Kind == "codex-subscription") {
		a.codexAccountMu.Lock()
		defer a.codexAccountMu.Unlock()
		account, err := a.readCodexConnection()
		if err != nil {
			return result, err
		}
		if account == nil && connection.Kind == "codex-subscription" {
			return result, errCodexDisconnected
		}
		if account != nil {
			authMode, generation = "chatgpt", account.ID
		}
	}
	connection.AuthMode = authMode
	return product.ProjectRunSelection{
		ProjectID: projectID, WorktreeID: req.WorktreeID, Path: path,
		Harness: string(harnessID), Model: model, PermissionMode: string(mode), Effort: string(effort),
		AgentConnectionID: connection.ID, AgentConnectionRevision: connection.Revision,
		AgentConnectionKind: connection.Kind, AgentConnectionProtocol: connection.Protocol,
		AuthMode: authMode, CodexConnectionID: generation, Connection: connection,
	}, nil
}

// ObserveProjectRunSessions inspects the private registry and existing backend
// without starting or repairing anything. Conflicting ownership is returned to
// the product adapter for rejection rather than hidden by a filtering query.
func (a *API) ObserveProjectRunSessions(ctx context.Context, projectID, runID, operationID string) ([]registry.Session, error) {
	var rows []registry.Session
	var err error
	if indexed, ok := a.deps.Store.(interface {
		ProjectRunSessions(string, string) ([]registry.Session, error)
	}); ok {
		rows, err = indexed.ProjectRunSessions(runID, operationID)
	} else {
		rows, err = a.deps.Store.List()
	}
	if err != nil {
		return nil, err
	}
	result := []registry.Session{}
	for _, row := range rows {
		if row.ProjectRunOperationID != operationID && !(row.ProjectRunID == runID && row.ProjectRunOperationID == "") {
			continue
		}
		if len(result) >= 2 {
			return nil, errors.New("multiple coding sessions claim this run operation")
		}
		if row.Status != registry.StatusExited && row.SandboxID == "" {
			row.Status, row.ExitReason = registry.StatusExited, "project-run-outcome-unknown"
		} else if row.Status != registry.StatusExited {
			sandbox, err := a.deps.Backend.Get(ctx, row.SandboxID)
			if errors.Is(err, backend.ErrNotFound) || err == nil && sandbox.State == backend.StateGone {
				row.Status = registry.StatusExited
				row.ExitReason = "project-run-outcome-unknown"
			} else if err == nil && sandbox.State == backend.StateStopped {
				row.Status = registry.StatusExited
				if row.ExitReason == "" {
					row.ExitReason = "exited"
				}
			} else if err != nil {
				return nil, err
			} else if sandbox.ID != row.SandboxID {
				return nil, errors.New("the coding backend returned a different sandbox identity")
			}
		}
		result = append(result, row)
	}
	return result, nil
}
