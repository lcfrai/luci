package product

// ProjectRunSelectionRequest is the public selection resolved by the session
// API before a run is saved. It carries no path, credential or receipt.
type ProjectRunSelectionRequest struct {
	ProjectID         string  `json:"projectId"`
	WorktreeID        string  `json:"worktreeId"`
	Harness           string  `json:"harness"`
	Model             string  `json:"model"`
	PermissionMode    string  `json:"permissionMode"`
	Effort            string  `json:"effort"`
	AgentConnectionID *string `json:"agentConnectionId"`
}

// ProjectRunSelection is an internal resolution result, not a run DTO. Public
// connection identity may be saved in the run; the path and private Codex
// generation must stay in controller-owned storage and never enter a prompt.
type ProjectRunSelection struct {
	ProjectID               string                 `json:"projectId"`
	WorktreeID              string                 `json:"worktreeId"`
	Path                    string                 `json:"-"`
	Harness                 string                 `json:"harness"`
	Model                   string                 `json:"model"`
	PermissionMode          string                 `json:"permissionMode"`
	Effort                  string                 `json:"effort"`
	AgentConnectionID       string                 `json:"agentConnectionId"`
	AgentConnectionRevision int                    `json:"agentConnectionRevision"`
	AgentConnectionKind     string                 `json:"agentConnectionKind"`
	AgentConnectionProtocol string                 `json:"agentConnectionProtocol"`
	AuthMode                string                 `json:"authMode"`
	CodexConnectionID       string                 `json:"-"`
	Connection              AgentConnectionReceipt `json:"connection"`
}
