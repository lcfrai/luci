package product

import "strings"

// A saved route is a person's explicit nonsecret preference. It contains no
// approved revisions, tasks, authority snapshot or credentials.
type CoordinatorRoute struct {
	WorktreeID        string  `json:"worktreeId"`
	Harness           string  `json:"harness"`
	Model             string  `json:"model"`
	PermissionMode    string  `json:"permissionMode"`
	Effort            string  `json:"effort,omitempty"`
	AgentConnectionID *string `json:"agentConnectionId,omitempty"`
	Outcome           string  `json:"outcome"`
}
type CoordinatorSavedRoute struct {
	Revision    int                   `json:"revision"`
	CheckPlan   *CoordinatorCheckPlan `json:"checkPlan,omitempty"`
	Route       CoordinatorRoute      `json:"route"`
	MaxParallel int                   `json:"maxParallel"`
	UpdatedAt   string                `json:"updatedAt"`
}

func coordinatorRoute(req projectRunCreateRequest) CoordinatorRoute {
	out := CoordinatorRoute{WorktreeID: strings.TrimSpace(req.WorktreeID), Harness: strings.TrimSpace(req.Harness), Model: strings.TrimSpace(req.Model), PermissionMode: strings.TrimSpace(req.PermissionMode), Effort: strings.TrimSpace(req.Effort), Outcome: req.Outcome}
	if req.AgentConnectionID != nil {
		value := strings.TrimSpace(*req.AgentConnectionID)
		out.AgentConnectionID = &value
	}
	return out
}
func validCoordinatorRoute(saved *CoordinatorSavedRoute) bool {
	if saved == nil {
		return true
	}
	if validateCoordinatorCheckPlan(saved.CheckPlan) != nil {
		return false
	}
	r := saved.Route
	if saved.Revision < 1 || saved.MaxParallel < 1 || saved.MaxParallel > MaxSwarmParallelTasks || !validConnectionToken(r.WorktreeID, 200, false) || !validConnectionToken(r.Harness, 128, false) || !validConnectionToken(r.Model, 200, true) || !validConnectionToken(r.PermissionMode, 128, false) || !validConnectionToken(r.Effort, 128, true) || (r.Outcome != "preview" && r.Outcome != "publish") {
		return false
	}
	return r.AgentConnectionID == nil || validConnectionToken(*r.AgentConnectionID, 128, true)
}
