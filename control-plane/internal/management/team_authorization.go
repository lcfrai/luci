package management

import (
	"net/http"
	"path"
	"strings"
)

// TeamGrants are explicit capabilities, separate from permission to edit shared
// context. In particular, making somebody an editor never grants paid execution,
// secret management, deployment, or membership administration.
type TeamGrants struct {
	Execute bool `json:"execute"`
	Secrets bool `json:"secrets"`
	Deploy  bool `json:"deploy"`
	Admin   bool `json:"admin"`
}

type TeamPermissions struct {
	Read    bool `json:"read"`
	Edit    bool `json:"edit"`
	Execute bool `json:"execute"`
	Secrets bool `json:"secrets"`
	Deploy  bool `json:"deploy"`
	Admin   bool `json:"admin"`
}

func PermissionsForTeam(role string, grants TeamGrants) TeamPermissions {
	if role != "owner" && role != "editor" && role != "viewer" {
		return TeamPermissions{}
	}
	return TeamPermissions{Read: true, Edit: role == "owner" || role == "editor", Execute: grants.Execute, Secrets: grants.Secrets, Deploy: grants.Deploy, Admin: grants.Admin}
}

// IntersectTeamPermissions limits a project's grants by current team membership.
// A project grant can narrow team authority; it cannot create a new capability.
func IntersectTeamPermissions(team, project TeamPermissions) TeamPermissions {
	return TeamPermissions{Read: team.Read && project.Read, Edit: team.Edit && project.Edit, Execute: team.Execute && project.Execute, Secrets: team.Secrets && project.Secrets, Deploy: team.Deploy && project.Deploy, Admin: team.Admin && project.Admin}
}

// TeamRouteScope tells the gateway how to resolve ownership before admission.
// Identifiers from the request are selectors, never proof of membership. Session
// IDs and body worktree IDs must be resolved in the selected private team worker;
// collections must be filtered before a response reaches a project-scoped member.
type TeamRouteScope struct {
	ProjectID        string
	SessionID        string
	WorktreeID       string
	Collection       string
	WorktreeFromBody bool
	CreatesProject   bool
}

type teamCapability uint8

const (
	teamRead teamCapability = 1 << iota
	teamEdit
	teamExecute
	teamSecrets
	teamDeploy
	teamAdmin
)

type teamRouteRule struct {
	pattern  string
	required teamCapability
}

// This is an allowlist, not an "all GETs are safe" rule. Terminal attachment and
// application browser bridges accept writes over GET/upgrade, and application
// data can contain secrets. A newly added API remains unavailable to teams until
// its actual behavior and ownership scope have been classified here.
var teamRouteRules = []teamRouteRule{
	{"GET /v1/healthz", teamRead},
	{"GET /v1/machines", teamRead},
	{"GET /v1/harnesses", teamRead},
	{"GET /v1/projects", teamRead},
	{"GET /v1/projects/{projectId}/branches", teamRead},
	{"GET /v1/projects/{projectId}/worktrees", teamRead},
	{"PATCH /v1/projects/{projectId}/worktrees/{worktreeId}", teamEdit},
	{"GET /v1/sessions", teamRead},
	{"POST /v1/sessions", teamEdit | teamExecute},
	{"GET /v1/sessions/{sessionId}", teamRead},
	{"GET /v1/sessions/{sessionId}/reviewed-context", teamRead},
	{"PATCH /v1/sessions/{sessionId}", teamEdit},
	{"DELETE /v1/sessions/{sessionId}", teamEdit | teamExecute},
	{"POST /v1/sessions/{sessionId}/restart", teamEdit | teamExecute},
	{"GET /v1/sessions/{sessionId}/attach", teamEdit | teamExecute},
	{"POST /v1/product/projects", teamEdit},
	{"GET /v1/product/project-imports/{attemptId}", teamRead},
	{"POST /v1/product/projects/restore", teamEdit},
	{"GET /v1/product/projects/{projectId}/brief", teamRead},
	{"POST /v1/product/projects/{projectId}/brief", teamEdit},
	{"GET /v1/product/projects/{projectId}/design", teamRead},
	{"POST /v1/product/projects/{projectId}/design", teamEdit},
	{"GET /v1/product/projects/{projectId}/design/export", teamRead},
	{"GET /v1/product/projects/{projectId}/files", teamRead},
	{"GET /v1/product/projects/{projectId}/context-history", teamRead},
	{"GET /v1/product/projects/{projectId}/context", teamRead},
	{"GET /v1/product/projects/{projectId}/external-handoffs", teamRead},
	{"POST /v1/product/projects/{projectId}/external-handoffs", teamEdit},
	{"GET /v1/product/projects/{projectId}/external-handoffs/{handoffId}", teamRead},
	{"GET /v1/product/projects/{projectId}/external-handoffs/{handoffId}/bundle", teamRead},
	{"GET /v1/product/projects/{projectId}/external-handoffs/{handoffId}/review", teamRead},
	{"POST /v1/product/projects/{projectId}/external-handoffs/{handoffId}/returns", teamEdit},
	{"GET /v1/product/projects/{projectId}/context/archive", teamRead},
	{"POST /v1/product/projects/{projectId}/context/exports/repair", teamEdit},
	{"POST /v1/product/projects/{projectId}/context/archive/prune", teamEdit},
	{"GET /v1/product/projects/{projectId}/specification", teamRead},
	{"POST /v1/product/projects/{projectId}/specification", teamEdit},
	{"GET /v1/product/projects/{projectId}/specification/history", teamRead},
	{"POST /v1/product/projects/{projectId}/specification/approve", teamEdit},
	{"GET /v1/product/projects/{projectId}/design-plan", teamRead},
	{"POST /v1/product/projects/{projectId}/design-plan", teamEdit},
	{"GET /v1/product/projects/{projectId}/design-plan/history", teamRead},
	{"POST /v1/product/projects/{projectId}/design-plan/approve", teamEdit},
	{"GET /v1/product/projects/{projectId}/plan", teamRead},
	{"GET /v1/product/projects/{projectId}/overview", teamRead},
	{"GET /v1/product/projects/{projectId}/swarm-queue", teamRead},
	{"POST /v1/product/projects/{projectId}/plan", teamEdit},
	{"GET /v1/product/projects/{projectId}/assist", teamRead},
	{"POST /v1/product/projects/{projectId}/assist", teamEdit | teamExecute | teamSecrets},
	{"POST /v1/product/projects/{projectId}/assist/repository", teamEdit | teamExecute | teamSecrets},
	{"GET /v1/product/projects/{projectId}/assist/repository/requests/{requestId}", teamRead},
	{"POST /v1/product/projects/{projectId}/assist/orchestrator", teamEdit | teamExecute | teamSecrets},
	{"GET /v1/product/projects/{projectId}/orchestrator", teamRead},
	{"GET /v1/product/projects/{projectId}/orchestrator/export", teamRead},
	{"POST /v1/product/projects/{projectId}/orchestrator/rotate", teamEdit},
	// Actor-bound request recovery fences only that actor's absent request; it
	// cannot change shared context, lease credentials or dispatch/repair jobs.
	{"POST /v1/product/projects/{projectId}/orchestrator/resolve", teamRead},
	{"POST /v1/product/projects/{projectId}/orchestrator/turns/{turnId}/apply", teamEdit},
	{"GET /v1/product/projects/{projectId}/assist/{jobId}", teamRead},
	{"POST /v1/product/projects/{projectId}/assist/{jobId}/cancel", teamEdit | teamExecute},
	{"POST /v1/product/projects/{projectId}/assist/{jobId}/reconcile", teamEdit},
	{"POST /v1/product/projects/{projectId}/assist/{jobId}/apply", teamEdit},
	{"GET /v1/product/projects/{projectId}/checks", teamRead},
	{"POST /v1/product/projects/{projectId}/checks", teamEdit | teamExecute},
	{"GET /v1/product/projects/{projectId}/checks/{runId}", teamRead},
	{"POST /v1/product/projects/{projectId}/checks/{runId}/cancel", teamEdit},
	{"GET /v1/product/projects/{projectId}/checks/{runId}/recovery", teamRead},
	{"POST /v1/product/projects/{projectId}/checks/{runId}/recovery", teamEdit},
	{"GET /v1/product/projects/{projectId}/runs", teamRead},
	{"POST /v1/product/projects/{projectId}/runs", teamEdit},
	{"GET /v1/product/projects/{projectId}/runs/{runId}", teamRead},
	{"POST /v1/product/projects/{projectId}/runs/{runId}/operations", teamEdit},
	{"POST /v1/product/projects/{projectId}/runs/{runId}/reconcile", teamEdit},
	{"POST /v1/product/projects/{projectId}/runs/{runId}/checks", teamEdit},
	{"POST /v1/product/projects/{projectId}/runs/{runId}/releases", teamEdit},
	{"POST /v1/product/projects/{projectId}/runs/{runId}/accept", teamEdit},
	{"POST /v1/product/projects/{projectId}/runs/{runId}/cancel", teamEdit},
	{"POST /v1/product/projects/{projectId}/runs/{runId}/swarm", teamEdit | teamExecute},
	{"POST /v1/product/projects/{projectId}/runs/{runId}/swarm/queue", teamEdit | teamExecute},
	{"POST /v1/product/projects/{projectId}/runs/{runId}/swarm/tasks/{taskId}/queue", teamEdit | teamExecute},
	{"POST /v1/product/projects/{projectId}/swarm-queue/{queueId}/pause", teamEdit},
	{"POST /v1/product/projects/{projectId}/swarm-queue/{queueId}/cancel", teamEdit},
	{"POST /v1/product/projects/{projectId}/swarm-queue/{queueId}/priority", teamEdit},
	{"POST /v1/product/projects/{projectId}/swarm-queue/{queueId}/resume", teamEdit | teamExecute},
	{"POST /v1/product/projects/{projectId}/runs/{runId}/swarm/pause", teamEdit},
	{"POST /v1/product/projects/{projectId}/runs/{runId}/swarm/resume", teamEdit | teamExecute},
	{"POST /v1/product/projects/{projectId}/runs/{runId}/swarm/cancel", teamEdit},
	{"POST /v1/product/projects/{projectId}/runs/{runId}/swarm/reconcile", teamEdit},
	{"POST /v1/product/projects/{projectId}/runs/{runId}/swarm/tasks/{taskId}/reserve", teamEdit | teamExecute},
	{"GET /v1/product/projects/{projectId}/runs/{runId}/swarm/tasks/{taskId}/review", teamRead},
	{"POST /v1/product/projects/{projectId}/runs/{runId}/swarm/tasks/{taskId}/integrate", teamEdit | teamExecute},
	{"POST /v1/product/projects/{projectId}/runs/{runId}/swarm/tasks/{taskId}/abandon", teamEdit},
	{"GET /v1/product/projects/{projectId}/runs/{runId}/swarm/operations/{operationId}/workspace", teamRead},
	{"GET /v1/product/projects/{projectId}/runs/{runId}/swarm/operations/{operationId}/workspace/file", teamRead},
	{"GET /v1/product/projects/{projectId}/runs/{runId}/swarm/operations/{operationId}/recovery", teamRead | teamEdit},
	{"GET /v1/product/projects/{projectId}/runs/{runId}/swarm/operations/{operationId}/checkpoint", teamRead},
	{"POST /v1/product/projects/{projectId}/runs/{runId}/swarm/operations/{operationId}/checkpoint", teamEdit | teamExecute},
	{"GET /v1/product/projects/{projectId}/runs/{runId}/swarm/operations/{operationId}/handoff", teamRead},
	{"POST /v1/product/projects/{projectId}/runs/{runId}/swarm/operations/{operationId}/handoff/collect", teamEdit},
	{"POST /v1/product/projects/{projectId}/runs/{runId}/swarm/operations/{operationId}/handoff/questions/{questionId}/answer", teamEdit},
	{"GET /v1/product/projects/{projectId}/git", teamRead},
	{"POST /v1/product/projects/{projectId}/git/commit", teamEdit},
	{"GET /v1/product/projects/{projectId}/git/branches", teamRead},
	{"POST /v1/product/projects/{projectId}/git/branches", teamEdit},
	{"POST /v1/product/projects/{projectId}/git/switch", teamEdit},
	{"GET /v1/product/projects/{projectId}/git/branch-operations/{operationId}", teamRead},
	{"GET /v1/product/projects/{projectId}/git/delivery", teamRead},
	{"POST /v1/product/projects/{projectId}/git/remote", teamEdit | teamDeploy},
	{"POST /v1/product/projects/{projectId}/git/github", teamEdit | teamDeploy},
	{"POST /v1/product/projects/{projectId}/git/push", teamEdit | teamDeploy},
	{"GET /v1/product/projects/{projectId}/configuration", teamRead},
	{"POST /v1/product/projects/{projectId}/configuration", teamEdit | teamSecrets},
	{"POST /v1/product/projects/{projectId}/backups", teamEdit},
	{"GET /v1/product/projects/{projectId}/prototype-releases", teamRead},
	{"GET /v1/product/projects/{projectId}/releases", teamRead},
	{"POST /v1/product/projects/{projectId}/releases", teamEdit | teamDeploy},
	{"POST /v1/product/projects/{projectId}/releases/cloud", teamEdit | teamDeploy},
	{"POST /v1/product/projects/{projectId}/releases/pages", teamEdit | teamDeploy},
	{"POST /v1/product/projects/{projectId}/releases/deploy-key", teamEdit | teamDeploy | teamSecrets},
	{"POST /v1/product/projects/{projectId}/releases/{releaseId}/stop", teamEdit | teamDeploy},
	{"POST /v1/product/projects/{projectId}/releases/{releaseId}/cancel", teamEdit | teamDeploy},
	{"POST /v1/product/projects/{projectId}/releases/{releaseId}/refresh", teamEdit | teamDeploy},
	{"POST /v1/product/projects/{projectId}/releases/{releaseId}/rollback", teamEdit | teamDeploy},
	{"GET /v1/setup/status", teamRead},
	{"POST /v1/setup/check", teamEdit | teamAdmin},
	{"GET /v1/setup/agents", teamRead},
	{"GET /v1/setup/agent-connections", teamRead},
	{"POST /v1/setup/agent-connections", teamEdit | teamSecrets},
	{"POST /v1/setup/agents/anthropic", teamEdit | teamSecrets},
	{"POST /v1/setup/agents/openai", teamEdit | teamSecrets},
	{"POST /v1/setup/agents/local", teamEdit | teamSecrets},
	{"GET /v1/setup/agents/codex/account", teamRead},
	{"POST /v1/setup/agents/codex/account", teamEdit | teamSecrets},
	{"DELETE /v1/setup/agents/codex/account", teamEdit | teamSecrets},
	{"POST /v1/setup/agents/codex/sign-in", teamEdit | teamExecute | teamSecrets},
	{"GET /v1/setup/git/identity", teamRead},
	{"POST /v1/setup/git/identity", teamEdit | teamAdmin},
	{"GET /v1/setup/github/login", teamSecrets},
	{"POST /v1/setup/github/login", teamEdit | teamSecrets},
	{"POST /v1/setup/github/check", teamEdit | teamSecrets},
	{"GET /v1/setup/targets", teamDeploy},
	{"POST /v1/setup/targets", teamEdit | teamAdmin | teamSecrets | teamDeploy},
	{"GET /v1/platform/status", teamRead},
	{"GET /v1/platform/projects/{projectId}", teamRead},
	{"POST /v1/platform/projects/{projectId}/deployments", teamEdit | teamDeploy},
	{"POST /v1/platform/projects/{projectId}/deployments/{releaseId}/{action}", teamEdit | teamDeploy},
	{"GET /v1/platform/projects/{projectId}/deployments/{releaseId}/logs", teamDeploy | teamSecrets},
	{"POST /v1/platform/projects/{projectId}/database", teamEdit | teamDeploy | teamSecrets},
	{"GET /v1/platform/projects/{projectId}/backups/{backupId}", teamDeploy | teamSecrets},
	{"POST /v1/platform/projects/{projectId}/backups/import", teamEdit | teamDeploy | teamSecrets},
	{"GET /v1/platform/projects/{projectId}/data", teamDeploy | teamSecrets},
	{"POST /v1/platform/projects/{projectId}/data", teamEdit | teamDeploy | teamSecrets},
	{"GET /v1/platform/projects/{projectId}/data/backups/{backupId}", teamDeploy | teamSecrets},
}

var teamRouteMux, teamRulesByPattern = buildTeamRouteRules()

func buildTeamRouteRules() (*http.ServeMux, map[string]teamRouteRule) {
	rules := append([]teamRouteRule(nil), teamRouteRules...)
	for _, suffix := range []string{"users", "tables", "tables/{table}/rows", "tables/{table}/rls", "buckets", "buckets/{bucket}/objects", "buckets/{bucket}/objects/{path...}", "realtime", "email", "domain", "functions", "functions/{function}"} {
		rules = append(rules, teamRouteRule{"GET /v1/platform/projects/{projectId}/data/" + suffix, teamDeploy | teamSecrets})
	}
	for _, suffix := range []string{"users", "tables", "tables/{table}/rows", "tables/{table}/rls", "buckets", "buckets/{bucket}/objects/{path...}", "realtime", "email", "domain", "functions/{function}/action", "functions/{function}/invoke"} {
		rules = append(rules, teamRouteRule{"POST /v1/platform/projects/{projectId}/data/" + suffix, teamEdit | teamDeploy | teamSecrets})
	}
	rules = append(rules, teamRouteRule{"POST /v1/platform/projects/{projectId}/data/functions/{function}/source", teamEdit})
	// Browser bridges, remote workspace selection/auth/admin, backing provisioning
	// and host setup are intentionally absent. They need team-scoped propagation
	// before a saved remote account may act on behalf of an individual member.
	mux, byPattern := http.NewServeMux(), map[string]teamRouteRule{}
	for _, rule := range rules {
		// Setup mutations change shared team state, including credentials used by
		// projects the caller may not be allowed to see. A project secret grant
		// alone cannot replace a team-wide model or deployment connection.
		if strings.HasPrefix(rule.pattern, "POST /v1/setup/") || strings.HasPrefix(rule.pattern, "DELETE /v1/setup/") {
			rule.required |= teamAdmin
		}
		mux.HandleFunc(rule.pattern, func(http.ResponseWriter, *http.Request) {})
		byPattern[rule.pattern] = rule
	}
	return mux, byPattern
}

func classifyTeamRoute(r *http.Request) (TeamRouteScope, teamCapability, bool) {
	var scope TeamRouteScope
	if r.URL == nil || (r.URL.RawPath != "" && !canonicalStorageObjectPath(r)) || strings.ContainsAny(r.URL.Path, "\\\x00") || path.Clean(r.URL.Path) != r.URL.Path {
		return scope, 0, false
	}
	_, pattern := teamRouteMux.Handler(r)
	rule, ok := teamRulesByPattern[pattern]
	if !ok {
		return scope, 0, false
	}
	parts := strings.Split(r.URL.Path, "/")
	_, route, _ := strings.Cut(pattern, " ")
	for i, part := range strings.Split(route, "/") {
		if i >= len(parts) {
			return TeamRouteScope{}, 0, false
		}
		switch part {
		case "{projectId}":
			scope.ProjectID = parts[i]
		case "{sessionId}":
			scope.SessionID = parts[i]
		case "{worktreeId}":
			scope.WorktreeID = parts[i]
		}
	}
	switch pattern {
	case "GET /v1/projects":
		scope.Collection = "projects"
	case "GET /v1/sessions":
		scope.Collection = "sessions"
	case "GET /v1/machines":
		scope.Collection = "machines"
	case "GET /v1/harnesses":
		scope.Collection = "harnesses"
	case "POST /v1/sessions":
		scope.WorktreeFromBody = true
	case "POST /v1/product/projects", "POST /v1/product/projects/restore":
		scope.CreatesProject = true
	}
	return scope, rule.required | teamRead, true
}

func ClassifyTeamRoute(r *http.Request) (TeamRouteScope, bool) {
	scope, _, ok := classifyTeamRoute(r)
	return scope, ok
}

func authorizeTeamRoute(w http.ResponseWriter, r *http.Request, permissions TeamPermissions) bool {
	_, required, known := classifyTeamRoute(r)
	if !known {
		fail(w, http.StatusForbidden, "team_route_unavailable", "This operation is not available in a team workspace.")
		return false
	}
	for _, check := range []struct {
		flag    teamCapability
		allowed bool
	}{
		{teamRead, permissions.Read}, {teamEdit, permissions.Edit}, {teamExecute, permissions.Execute},
		{teamSecrets, permissions.Secrets}, {teamDeploy, permissions.Deploy}, {teamAdmin, permissions.Admin},
	} {
		if required&check.flag != 0 && !check.allowed {
			fail(w, http.StatusForbidden, "team_permission_denied", "Your current team and project permissions do not allow this operation.")
			return false
		}
	}
	return true
}
