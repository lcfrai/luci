package management

import (
	"encoding/json"
	"math"
	"net/http"
	"strings"
)

func requireAdmin(w http.ResponseWriter, r *http.Request, super bool) (User, bool) {
	u, _, ok := ActorFromContext(r.Context())
	if !ok || u.Disabled || (u.Role != "super-admin" && (super || u.Role != "admin")) {
		fail(w, 403, "permission_denied", "This action requires an administrator with the appropriate role.")
		return u, false
	}
	return u, true
}
func (g *Gateway) listUsers(w http.ResponseWriter, r *http.Request) {
	if _, ok := requireAdmin(w, r, false); !ok {
		return
	}
	g.mu.Lock()
	defer g.mu.Unlock()
	jsonReply(w, 200, map[string]any{"users": g.userListLocked()})
}
func (g *Gateway) createUser(w http.ResponseWriter, r *http.Request) {
	actor, ok := requireAdmin(w, r, false)
	if !ok {
		return
	}
	var req struct {
		Username    string `json:"username"`
		DisplayName string `json:"displayName"`
		Password    string `json:"password"`
		Role        string `json:"role"`
	}
	if !decode(w, r, &req) {
		return
	}
	if req.Role == "" {
		req.Role = "member"
	}
	if !validIdentity(req.Username, req.DisplayName, req.Password) || !validRole(req.Role) {
		fail(w, 400, "invalid_identity", "Use a valid username, display name, role and password of at least 12 characters.")
		return
	}
	if actor.Role != "super-admin" && req.Role != "member" {
		fail(w, 403, "permission_denied", "Only a Super Admin can create an administrator.")
		return
	}
	if !g.passwordSlot(w) {
		return
	}
	defer g.releasePasswordSlot()
	hash := passwordHash(req.Password)
	g.mu.Lock()
	defer g.mu.Unlock()
	if current, exists := g.state.Users[actor.ID]; !exists || current.User.Disabled || current.User.Role != actor.Role {
		fail(w, 403, "identity_changed", "Your role changed. Sign in again before administering users.")
		return
	}
	if len(g.state.Users) >= 1000 {
		fail(w, 409, "user_limit", "This instance supports at most 1,000 users.")
		return
	}
	for _, u := range g.state.Users {
		if strings.EqualFold(u.User.Username, req.Username) {
			fail(w, 409, "username_exists", "Choose another username.")
			return
		}
	}
	before := g.state
	g.state = cloneIdentityState(before)
	u := User{ID: opaqueID("user-"), Username: strings.ToLower(req.Username), DisplayName: strings.TrimSpace(req.DisplayName), Role: req.Role}
	g.state.Users[u.ID] = userRecord{User: u, PasswordHash: hash}
	g.auditLocked(actor.ID, "user.create", u.ID, "succeeded")
	if g.persistLocked() != nil {
		g.state = before
		fail(w, 500, "save_failed", "The user could not be saved.")
		return
	}
	jsonReply(w, 201, u)
}
func (g *Gateway) updateUser(w http.ResponseWriter, r *http.Request) {
	actor, ok := requireAdmin(w, r, false)
	if !ok {
		return
	}
	var req struct {
		Role     *string `json:"role"`
		Disabled *bool   `json:"disabled"`
		Password *string `json:"password"`
	}
	if !decode(w, r, &req) {
		return
	}
	if req.Role == nil && req.Disabled == nil && req.Password == nil {
		fail(w, 400, "empty_update", "Choose a role, enabled state or password to update.")
		return
	}
	if req.Role != nil && !validRole(*req.Role) {
		fail(w, 400, "invalid_role", "Choose member, admin or super-admin.")
		return
	}
	if req.Password != nil && (len(*req.Password) < 12 || len(*req.Password) > 1024) {
		fail(w, 400, "invalid_password", "Use a password of 12–1,024 characters.")
		return
	}
	var hash string
	if req.Password != nil {
		if !g.passwordSlot(w) {
			return
		}
		defer g.releasePasswordSlot()
		hash = passwordHash(*req.Password)
	}
	g.mu.Lock()
	if current, exists := g.state.Users[actor.ID]; !exists || current.User.Disabled || current.User.Role != actor.Role {
		g.mu.Unlock()
		fail(w, 403, "identity_changed", "Your role changed. Sign in again before administering users.")
		return
	}
	row, exists := g.state.Users[r.PathValue("id")]
	if !exists {
		g.mu.Unlock()
		fail(w, 404, "unknown_user", "This user does not exist.")
		return
	}
	if actor.Role != "super-admin" && (row.User.Role != "member" || (req.Role != nil && *req.Role != "member")) {
		g.mu.Unlock()
		fail(w, 403, "permission_denied", "Only a Super Admin can manage administrator roles or accounts.")
		return
	}
	updated := row
	if req.Role != nil {
		updated.User.Role = *req.Role
	}
	if req.Disabled != nil {
		updated.User.Disabled = *req.Disabled
	}
	if req.Password != nil {
		updated.PasswordHash = hash
	}
	if row.User.Role == "super-admin" && !row.User.Disabled && (updated.User.Disabled || updated.User.Role != "super-admin") {
		count := 0
		for _, u := range g.state.Users {
			if u.User.Role == "super-admin" && !u.User.Disabled {
				count++
			}
		}
		if count <= 1 {
			g.mu.Unlock()
			fail(w, 409, "last_super_admin", "Keep at least one enabled Super Admin.")
			return
		}
	}
	before := g.state
	g.state = cloneIdentityState(before)
	g.state.Users[row.User.ID] = updated
	g.revokeSessionsLocked(row.User.ID)
	g.auditLocked(actor.ID, "user.update", row.User.ID, "succeeded")
	if g.persistLocked() != nil {
		g.state = before
		g.mu.Unlock()
		fail(w, 500, "save_failed", "The user update could not be saved.")
		return
	}
	g.revokeConnectionsLocked(row.User.ID)
	rt := g.runtimes[row.User.ID]
	g.mu.Unlock()
	if rt != nil {
		rt.stop()
	}
	jsonReply(w, 200, updated.User)
}
func validatePolicy(p Policy) error {
	if p.SessionMemoryMiB < 128 || p.SessionMemoryMiB > 32768 || p.CheckMemoryMiB < 128 || p.CheckMemoryMiB > 32768 || p.SessionCPUMillis < 100 || p.SessionCPUMillis > 16000 || p.CheckCPUMillis < 100 || p.CheckCPUMillis > 16000 {
		return &policyError{}
	}
	if p.MaxSessionsPerUser < 1 || p.MaxSessionsPerUser > 100 || p.MaxConcurrentChecksPerUser < 1 || p.MaxConcurrentChecksPerUser > 20 || math.IsNaN(p.MonthlyBudgetUSD) || math.IsInf(p.MonthlyBudgetUSD, 0) || p.MonthlyBudgetUSD < 0 || p.MonthlyBudgetUSD > 100000 || len(strings.TrimSpace(p.InstanceName)) < 1 || len(p.InstanceName) > 120 || (p.BackingMode != "docker" && p.BackingMode != "aws" && p.BackingMode != "azure") || len(p.AllowedHarnesses) < 1 || len(p.AllowedHarnesses) > 4 {
		return &policyError{}
	}
	seen := map[string]bool{}
	for _, id := range p.AllowedHarnesses {
		if (id != "claude-code" && id != "codex" && id != "opencode" && id != "shell") || seen[id] {
			return &policyError{}
		}
		seen[id] = true
	}
	return nil
}

type policyError struct{}

func (*policyError) Error() string {
	return "Choose valid limits, allowed harnesses, instance name and Docker/AWS/Azure backing."
}
func (g *Gateway) getPolicy(w http.ResponseWriter, r *http.Request) {
	if _, ok := requireAdmin(w, r, false); !ok {
		return
	}
	g.mu.Lock()
	defer g.mu.Unlock()
	jsonReply(w, 200, g.state.Policy)
}
func (g *Gateway) patchPolicy(w http.ResponseWriter, r *http.Request) {
	actor, ok := requireAdmin(w, r, true)
	if !ok {
		return
	}
	var req map[string]json.RawMessage
	if !decode(w, r, &req) {
		return
	}
	allowed := map[string]bool{"allowHostCommands": true, "allowCloudProvisioning": true, "allowPublicPublishing": true, "maxSessionsPerUser": true, "maxConcurrentChecksPerUser": true, "sessionMemoryMiB": true, "sessionCPUMillis": true, "checkMemoryMiB": true, "checkCPUMillis": true, "monthlyBudgetUsd": true, "allowedHarnesses": true, "backingMode": true, "instanceName": true}
	for key, value := range req {
		if !allowed[key] || string(value) == "null" {
			fail(w, 400, "invalid_policy", "Unknown or null policy field.")
			return
		}
	}
	g.mu.Lock()
	defer g.mu.Unlock()
	data, _ := json.Marshal(g.state.Policy)
	fields := map[string]json.RawMessage{}
	_ = json.Unmarshal(data, &fields)
	for key, value := range req {
		fields[key] = value
	}
	data, _ = json.Marshal(fields)
	var policy Policy
	if json.Unmarshal(data, &policy) != nil || validatePolicy(policy) != nil {
		fail(w, 400, "invalid_policy", (&policyError{}).Error())
		return
	}
	before := g.state
	g.state = cloneIdentityState(before)
	g.state.Policy = policy
	g.auditLocked(actor.ID, "policy.update", "instance", "succeeded")
	if g.persistLocked() != nil {
		g.state = before
		fail(w, 500, "save_failed", "The policy could not be saved.")
		return
	}
	jsonReply(w, 200, policy)
}
func (g *Gateway) audit(w http.ResponseWriter, r *http.Request) {
	if _, ok := requireAdmin(w, r, false); !ok {
		return
	}
	g.mu.Lock()
	defer g.mu.Unlock()
	events := append([]AuditEvent{}, g.state.Events...)
	jsonReply(w, 200, map[string]any{"events": events})
}
