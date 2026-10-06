package management

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"regexp"
	"sort"
	"strings"
	"time"
)

// Team identity and authorization live only in private management state.
// Neither project files nor a worker-supplied actor can create membership.
type Team struct {
	ID        string `json:"id"`
	Name      string `json:"name"`
	Revision  int    `json:"revision"`
	CreatedAt string `json:"createdAt"`
	UpdatedAt string `json:"updatedAt"`
}
type TeamMember struct {
	UserID    string     `json:"userId"`
	Role      string     `json:"role"`
	Grants    TeamGrants `json:"grants"`
	Revision  int        `json:"revision"`
	JoinedAt  string     `json:"joinedAt"`
	UpdatedAt string     `json:"updatedAt"`
}
type TeamProjectGrant struct {
	UserID   string     `json:"userId"`
	Role     string     `json:"role"`
	Grants   TeamGrants `json:"grants"`
	Revision int        `json:"revision"`
}
type teamRecord struct {
	Billing       *TeamBilling                           `json:"billing,omitempty"`
	Team          Team                                   `json:"team"`
	Service       TeamService                            `json:"service,omitempty"`
	DataLifecycle TeamDataLifecycle                      `json:"dataLifecycle,omitempty"`
	Members       map[string]TeamMember                  `json:"members"`
	Projects      map[string]map[string]TeamProjectGrant `json:"projects"`
}
type TeamAccess struct {
	Team            Team            `json:"team"`
	Member          TeamMember      `json:"member"`
	Permissions     TeamPermissions `json:"permissions"`
	ProjectID       string          `json:"projectId,omitempty"`
	ProjectRevision int             `json:"projectRevision,omitempty"`
}
type teamAccessKey struct{}

func WithTeamAccess(ctx context.Context, access TeamAccess) context.Context {
	return context.WithValue(ctx, teamAccessKey{}, access)
}
func TeamAccessFromContext(ctx context.Context) (TeamAccess, bool) {
	access, ok := ctx.Value(teamAccessKey{}).(TeamAccess)
	return access, ok
}

var teamIDPattern = regexp.MustCompile(`^team-[A-Za-z0-9_-]{22}$`)
var teamProjectIDPattern = regexp.MustCompile(`^[A-Za-z0-9_-]{1,128}$`)
var errTeamAccess = errors.New("This team or project is unavailable to your account. Refresh your workspaces or ask its owner for access.")

func cloneTeamRecords(source map[string]teamRecord) map[string]teamRecord {
	out := make(map[string]teamRecord, len(source))
	for id, row := range source {
		copy := row
		copy.Billing = cloneTeamBilling(row.Billing)
		copy.Members = make(map[string]TeamMember, len(row.Members))
		for user, member := range row.Members {
			copy.Members[user] = member
		}
		copy.Projects = make(map[string]map[string]TeamProjectGrant, len(row.Projects))
		for project, grants := range row.Projects {
			copy.Projects[project] = make(map[string]TeamProjectGrant, len(grants))
			for user, grant := range grants {
				copy.Projects[project][user] = grant
			}
		}
		out[id] = copy
	}
	return out
}
func validTeamRole(role string) bool { return role == "owner" || role == "editor" || role == "viewer" }
func validateTeams(state storedState) error {
	if len(state.Teams) > 64 {
		return errors.New("invalid team workspace state")
	}
	for id, row := range state.Teams {
		if err := validateTeamBilling(row.Billing); err != nil {
			return err
		}
		if err := validateTeamDataLifecycle(row.DataLifecycle); err != nil {
			return err
		}
		if err := validateTeamService(row.Service); err != nil {
			return err
		}
		if !teamIDPattern.MatchString(id) || row.Team.ID != id || row.Team.Revision < 1 || strings.TrimSpace(row.Team.Name) == "" || len(row.Team.Name) > 100 || len(row.Members) == 0 || len(row.Members) > 128 || row.Projects == nil || len(row.Projects) > 256 {
			return errors.New("invalid team workspace state")
		}
		owners := 0
		for user, member := range row.Members {
			if _, exists := state.Users[user]; !exists || member.UserID != user || !validTeamRole(member.Role) || member.Revision < 1 || member.Revision > row.Team.Revision {
				return errors.New("invalid team membership state")
			}
			if member.Role == "owner" {
				if !member.Grants.Admin {
					return errors.New("team owner must retain administration")
				}
				if !state.Users[user].User.Disabled {
					owners++
				}
			}
		}
		if owners == 0 {
			return errors.New("team must retain an enabled owner")
		}
		for project, grants := range row.Projects {
			if !teamProjectIDPattern.MatchString(project) || len(grants) > 128 {
				return errors.New("invalid team project permissions")
			}
			for user, grant := range grants {
				if _, exists := row.Members[user]; !exists || grant.UserID != user || (grant.Role != "editor" && grant.Role != "viewer") || grant.Revision < 1 || grant.Revision > row.Team.Revision {
					return errors.New("invalid team project permissions")
				}
			}
		}
	}
	return validateTeamInvitations(state)
}
func intersectTeamPermissions(a, b TeamPermissions) TeamPermissions {
	return TeamPermissions{Read: a.Read && b.Read, Edit: a.Edit && b.Edit, Execute: a.Execute && b.Execute, Secrets: a.Secrets && b.Secrets, Deploy: a.Deploy && b.Deploy, Admin: a.Admin && b.Admin}
}
func (g *Gateway) resolveTeamAccessLocked(teamID, userID, projectID string) (TeamAccess, error) {
	if g.managedTeamID != "" && teamID != g.managedTeamID {
		return TeamAccess{}, errTeamAccess
	}
	user, ok := g.state.Users[userID]
	if !ok || user.User.Disabled || g.closed {
		return TeamAccess{}, errTeamAccess
	}
	row, ok := g.state.Teams[teamID]
	if !ok {
		return TeamAccess{}, errTeamAccess
	}
	member, ok := row.Members[userID]
	if !ok {
		return TeamAccess{}, errTeamAccess
	}
	access := TeamAccess{Team: row.Team, Member: member, Permissions: PermissionsForTeam(member.Role, member.Grants), ProjectID: projectID}
	if projectID != "" && !access.Permissions.Admin {
		grant, ok := row.Projects[projectID][userID]
		if !ok {
			return TeamAccess{}, errTeamAccess
		}
		access.Permissions = intersectTeamPermissions(access.Permissions, PermissionsForTeam(grant.Role, grant.Grants))
		access.ProjectRevision = grant.Revision
	}
	if !access.Permissions.Read {
		return TeamAccess{}, errTeamAccess
	}
	return access, nil
}
func (g *Gateway) resolveTeamAccess(teamID, userID, projectID string) (TeamAccess, error) {
	g.mu.Lock()
	defer g.mu.Unlock()
	return g.resolveTeamAccessLocked(teamID, userID, projectID)
}
func (g *Gateway) teamAccessCurrentLocked(teamID, userID, projectID string, memberRevision, projectRevision int) bool {
	access, err := g.resolveTeamAccessLocked(teamID, userID, projectID)
	return err == nil && access.Member.Revision == memberRevision && access.ProjectRevision == projectRevision
}

func (g *Gateway) registerTeams(mux *http.ServeMux) {
	g.registerTeamBilling(mux)
	g.registerTeamDataLifecycle(mux)
	g.registerTeamAudit(mux)
	g.registerTeamService(mux)
	g.registerTeamBrowser(mux)
	g.registerTeamInvitations(mux)
	mux.HandleFunc("GET /v1/teams", g.listTeams)
	mux.HandleFunc("POST /v1/teams", g.createTeam)
	mux.HandleFunc("GET /v1/teams/{teamId}", g.getTeam)
	mux.HandleFunc("PATCH /v1/teams/{teamId}", g.renameTeam)
	mux.HandleFunc("PUT /v1/teams/{teamId}/members/{userId}", g.saveTeamMember)
	mux.HandleFunc("DELETE /v1/teams/{teamId}/members/{userId}", g.deleteTeamMember)
	mux.HandleFunc("POST /v1/teams/{teamId}/members", g.addTeamMember)
	mux.HandleFunc("GET /v1/teams/{teamId}/projects/{projectId}/members", g.listTeamProjectMembers)
	mux.HandleFunc("PUT /v1/teams/{teamId}/projects/{projectId}/members/{userId}", g.saveTeamProjectMember)
	mux.HandleFunc("DELETE /v1/teams/{teamId}/projects/{projectId}/members/{userId}", g.deleteTeamProjectMember)
	mux.HandleFunc("/v1/teams/{teamId}/workspace/", g.routeTeamWorkspace)
}

type teamMemberView struct {
	TeamMember
	User User `json:"user"`
}

func (g *Gateway) teamViewLocked(access TeamAccess) map[string]any {
	row := g.state.Teams[access.Team.ID]
	members := make([]teamMemberView, 0, len(row.Members))
	for id, member := range row.Members {
		members = append(members, teamMemberView{member, g.state.Users[id].User})
	}
	sort.Slice(members, func(i, j int) bool { return members[i].User.DisplayName < members[j].User.DisplayName })
	return map[string]any{"team": row.Team, "membership": access.Member, "permissions": access.Permissions, "members": members, "executionBoundary": "A separate team workspace on this execution host. Managed teams require a dedicated host or equivalent verified boundary."}
}
func (g *Gateway) listTeams(w http.ResponseWriter, r *http.Request) {
	u, _, _ := ActorFromContext(r.Context())
	g.mu.Lock()
	defer g.mu.Unlock()
	rows := []map[string]any{}
	for id := range g.state.Teams {
		if access, err := g.resolveTeamAccessLocked(id, u.ID, ""); err == nil {
			rows = append(rows, map[string]any{"team": access.Team, "membership": access.Member, "permissions": access.Permissions})
		}
	}
	sort.Slice(rows, func(i, j int) bool { return rows[i]["team"].(Team).Name < rows[j]["team"].(Team).Name })
	result := map[string]any{"teams": rows}
	if g.managedTeamID != "" {
		result["managedTeamId"] = g.managedTeamID
	}
	jsonReply(w, 200, result)
}
func (g *Gateway) createTeam(w http.ResponseWriter, r *http.Request) {
	if g.managedTeamID != "" {
		fail(w, 403, "managed_team_fixed", "This installation serves its assigned team. Use a separate installation for another team.")
		return
	}
	var req struct {
		Name string `json:"name"`
	}
	if !decode(w, r, &req) {
		return
	}
	req.Name = strings.TrimSpace(req.Name)
	if req.Name == "" || len(req.Name) > 100 {
		fail(w, 400, "invalid_team", "Choose a team name of 1–100 characters.")
		return
	}
	u, _, _ := ActorFromContext(r.Context())
	g.mu.Lock()
	defer g.mu.Unlock()
	if g.closed || g.state.Users[u.ID].User.Disabled {
		fail(w, 403, "team_unavailable", errTeamAccess.Error())
		return
	}
	if len(g.state.Teams) >= 64 {
		fail(w, 409, "team_limit", "This installation has reached its 64-team limit.")
		return
	}
	before := g.state
	g.state = cloneIdentityState(before)
	now := time.Now().UTC().Format(time.RFC3339Nano)
	team := Team{ID: opaqueID("team-"), Name: req.Name, Revision: 1, CreatedAt: now, UpdatedAt: now}
	member := TeamMember{UserID: u.ID, Role: "owner", Grants: TeamGrants{Execute: true, Secrets: true, Deploy: true, Admin: true}, Revision: 1, JoinedAt: now, UpdatedAt: now}
	g.state.Teams[team.ID] = teamRecord{Team: team, Members: map[string]TeamMember{u.ID: member}, Projects: map[string]map[string]TeamProjectGrant{}}
	g.auditLocked(u.ID, "team.create", team.ID, "success")
	if err := g.persistLocked(); err != nil {
		g.state = before
		fail(w, 500, "state_write_failed", "Team creation could not be saved. No workspace was started.")
		return
	}
	access, _ := g.resolveTeamAccessLocked(team.ID, u.ID, "")
	jsonReply(w, 201, g.teamViewLocked(access))
}
func (g *Gateway) getTeam(w http.ResponseWriter, r *http.Request) {
	u, _, _ := ActorFromContext(r.Context())
	g.mu.Lock()
	defer g.mu.Unlock()
	access, err := g.resolveTeamAccessLocked(r.PathValue("teamId"), u.ID, "")
	if err != nil {
		fail(w, 404, "team_unavailable", err.Error())
		return
	}
	jsonReply(w, 200, g.teamViewLocked(access))
}
func (g *Gateway) requireTeamAdminLocked(w http.ResponseWriter, r *http.Request, revision int) (TeamAccess, bool) {
	u, _, _ := ActorFromContext(r.Context())
	access, err := g.resolveTeamAccessLocked(r.PathValue("teamId"), u.ID, "")
	if err != nil || !access.Permissions.Admin {
		fail(w, 403, "team_admin_required", "Only this team's administrators can change its access.")
		return TeamAccess{}, false
	}
	if revision < 1 || access.Team.Revision != revision {
		fail(w, 409, "team_revision_conflict", "Team access changed. Reload the team before saving.")
		return TeamAccess{}, false
	}
	return access, true
}
func (g *Gateway) commitTeamLocked(w http.ResponseWriter, before storedState, actor, action, teamID string) bool {
	if err := validateTeams(g.state); err != nil {
		g.state = before
		fail(w, 409, "invalid_team_change", err.Error())
		return false
	}
	g.auditLocked(actor, action, teamID, "success")
	if err := g.persistLocked(); err != nil {
		g.state = before
		fail(w, 500, "state_write_failed", "Team access could not be saved. Previous access remains in effect.")
		return false
	}
	return true
}
func bumpTeam(row *teamRecord) {
	row.Team.Revision++
	row.Team.UpdatedAt = time.Now().UTC().Format(time.RFC3339Nano)
}
func (g *Gateway) renameTeam(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Name             string `json:"name"`
		ExpectedRevision int    `json:"expectedRevision"`
	}
	if !decode(w, r, &req) {
		return
	}
	req.Name = strings.TrimSpace(req.Name)
	if req.Name == "" || len(req.Name) > 100 {
		fail(w, 400, "invalid_team", "Choose a team name of 1–100 characters.")
		return
	}
	g.mu.Lock()
	defer g.mu.Unlock()
	access, ok := g.requireTeamAdminLocked(w, r, req.ExpectedRevision)
	if !ok {
		return
	}
	before := g.state
	g.state = cloneIdentityState(before)
	row := g.state.Teams[access.Team.ID]
	bumpTeam(&row)
	row.Team.Name = req.Name
	g.state.Teams[row.Team.ID] = row
	if !g.commitTeamLocked(w, before, access.Member.UserID, "team.rename", row.Team.ID) {
		return
	}
	access.Team = row.Team
	jsonReply(w, 200, g.teamViewLocked(access))
}

type teamMemberRequest struct {
	ExpectedRevision int        `json:"expectedRevision"`
	Username         string     `json:"username,omitempty"`
	Role             string     `json:"role"`
	Grants           TeamGrants `json:"grants"`
}

func (g *Gateway) addTeamMember(w http.ResponseWriter, r *http.Request) {
	g.changeTeamMember(w, r, true)
}
func (g *Gateway) saveTeamMember(w http.ResponseWriter, r *http.Request) {
	g.changeTeamMember(w, r, false)
}
func (g *Gateway) changeTeamMember(w http.ResponseWriter, r *http.Request, adding bool) {
	var req teamMemberRequest
	if !decode(w, r, &req) {
		return
	}
	if !validTeamRole(req.Role) || req.Role == "owner" && !req.Grants.Admin {
		fail(w, 400, "invalid_team_member", "Choose owner, editor or viewer and explicit permissions; an owner retains administration.")
		return
	}
	g.mu.Lock()
	defer g.mu.Unlock()
	access, ok := g.requireTeamAdminLocked(w, r, req.ExpectedRevision)
	if !ok {
		return
	}
	userID := r.PathValue("userId")
	if adding {
		for id, user := range g.state.Users {
			if strings.EqualFold(user.User.Username, strings.TrimSpace(req.Username)) {
				userID = id
				break
			}
		}
	}
	user, exists := g.state.Users[userID]
	if !exists || user.User.Disabled {
		fail(w, 404, "team_user_unavailable", "Choose an active account on this Luci installation.")
		return
	}
	old, exists := g.state.Teams[access.Team.ID].Members[userID]
	if adding && exists {
		fail(w, 409, "already_team_member", "This account is already a member. Edit its existing permissions.")
		return
	}
	if !adding && !exists {
		fail(w, 404, "team_member_unavailable", "This team member is unavailable.")
		return
	}
	if (old.Role == "owner" || req.Role == "owner") && access.Member.Role != "owner" {
		fail(w, 403, "team_owner_required", "Only a team owner can assign or change an owner.")
		return
	}
	if adding && len(g.state.Teams[access.Team.ID].Members) >= 128 {
		fail(w, 409, "team_member_limit", "This team has reached its 128-member limit.")
		return
	}
	before := g.state
	g.state = cloneIdentityState(before)
	row := g.state.Teams[access.Team.ID]
	bumpTeam(&row)
	joined := old.JoinedAt
	if joined == "" {
		joined = row.Team.UpdatedAt
	}
	row.Members[userID] = TeamMember{UserID: userID, Role: req.Role, Grants: req.Grants, Revision: row.Team.Revision, JoinedAt: joined, UpdatedAt: row.Team.UpdatedAt}
	g.state.Teams[row.Team.ID] = row
	if !g.commitTeamLocked(w, before, access.Member.UserID, "team.member.save", row.Team.ID) {
		return
	}
	g.revokeTeamAccessLocked(row.Team.ID, userID, "")
	current, err := g.resolveTeamAccessLocked(row.Team.ID, access.Member.UserID, "")
	if err != nil {
		jsonReply(w, 200, map[string]any{"team": row.Team})
		return
	}
	jsonReply(w, 200, g.teamViewLocked(current))
}
func (g *Gateway) deleteTeamMember(w http.ResponseWriter, r *http.Request) {
	var req struct {
		ExpectedRevision int `json:"expectedRevision"`
	}
	if !decode(w, r, &req) {
		return
	}
	g.mu.Lock()
	defer g.mu.Unlock()
	access, ok := g.requireTeamAdminLocked(w, r, req.ExpectedRevision)
	if !ok {
		return
	}
	userID := r.PathValue("userId")
	old, exists := g.state.Teams[access.Team.ID].Members[userID]
	if !exists {
		fail(w, 404, "team_member_unavailable", "This team member is unavailable.")
		return
	}
	if old.Role == "owner" && access.Member.Role != "owner" {
		fail(w, 403, "team_owner_required", "Only a team owner can remove an owner.")
		return
	}
	before := g.state
	g.state = cloneIdentityState(before)
	row := g.state.Teams[access.Team.ID]
	bumpTeam(&row)
	delete(row.Members, userID)
	for project, grants := range row.Projects {
		delete(grants, userID)
		if len(grants) == 0 {
			delete(row.Projects, project)
		}
	}
	g.state.Teams[row.Team.ID] = row
	if !g.commitTeamLocked(w, before, access.Member.UserID, "team.member.remove", row.Team.ID) {
		return
	}
	g.revokeTeamAccessLocked(row.Team.ID, userID, "")
	jsonReply(w, 200, map[string]any{"team": row.Team, "removed": true, "runningWork": "Shared sessions and releases are retained. Authorized team members can stop them explicitly."})
}
func (g *Gateway) listTeamProjectMembers(w http.ResponseWriter, r *http.Request) {
	u, _, _ := ActorFromContext(r.Context())
	g.mu.Lock()
	defer g.mu.Unlock()
	access, err := g.resolveTeamAccessLocked(r.PathValue("teamId"), u.ID, r.PathValue("projectId"))
	if err != nil {
		fail(w, 404, "team_project_unavailable", errTeamAccess.Error())
		return
	}
	grants := []TeamProjectGrant{}
	for user, grant := range g.state.Teams[access.Team.ID].Projects[access.ProjectID] {
		if access.Permissions.Admin || user == u.ID {
			grants = append(grants, grant)
		}
	}
	sort.Slice(grants, func(i, j int) bool { return grants[i].UserID < grants[j].UserID })
	jsonReply(w, 200, map[string]any{"team": access.Team, "projectId": access.ProjectID, "permissions": access.Permissions, "grants": grants})
}
func (g *Gateway) saveTeamProjectMember(w http.ResponseWriter, r *http.Request) {
	g.changeTeamProjectMember(w, r, false)
}
func (g *Gateway) deleteTeamProjectMember(w http.ResponseWriter, r *http.Request) {
	g.changeTeamProjectMember(w, r, true)
}
func (g *Gateway) changeTeamProjectMember(w http.ResponseWriter, r *http.Request, remove bool) {
	var req teamMemberRequest
	if !decode(w, r, &req) {
		return
	}
	if !remove && (req.Role != "editor" && req.Role != "viewer") {
		fail(w, 400, "invalid_project_role", "Choose editor or viewer for project access.")
		return
	}
	u, _, _ := ActorFromContext(r.Context())
	teamID, projectID := r.PathValue("teamId"), r.PathValue("projectId")
	if !teamProjectIDPattern.MatchString(projectID) {
		fail(w, 400, "invalid_project", "Choose an owned team project.")
		return
	}
	// Resolve real ownership outside the identity lock; a guessed foreign ID
	// cannot create a project grant in another runtime.
	access, err := g.resolveTeamAccess(teamID, u.ID, "")
	if err != nil || !access.Permissions.Admin {
		fail(w, 403, "team_admin_required", "Only a team administrator can change project access.")
		return
	}
	if err = g.teamProjectExists(r.Context(), access, projectID); err != nil {
		fail(w, 404, "team_project_unavailable", errTeamAccess.Error())
		return
	}
	g.mu.Lock()
	defer g.mu.Unlock()
	access, ok := g.requireTeamAdminLocked(w, r, req.ExpectedRevision)
	if !ok {
		return
	}
	userID := r.PathValue("userId")
	if _, exists := g.state.Teams[teamID].Members[userID]; !exists {
		fail(w, 404, "team_member_unavailable", "Add the account to this team first.")
		return
	}
	before := g.state
	g.state = cloneIdentityState(before)
	row := g.state.Teams[teamID]
	if _, exists := row.Projects[projectID]; !exists {
		if len(row.Projects) >= 256 && !remove {
			g.state = before
			fail(w, 409, "project_grant_limit", "This team has reached its 256-project permission limit.")
			return
		}
		row.Projects[projectID] = map[string]TeamProjectGrant{}
	}
	bumpTeam(&row)
	if remove {
		delete(row.Projects[projectID], userID)
	} else {
		row.Projects[projectID][userID] = TeamProjectGrant{UserID: userID, Role: req.Role, Grants: req.Grants, Revision: row.Team.Revision}
	}
	if len(row.Projects[projectID]) == 0 {
		delete(row.Projects, projectID)
	}
	g.state.Teams[teamID] = row
	if !g.commitTeamLocked(w, before, u.ID, "team.project.access", teamID) {
		return
	}
	g.revokeTeamAccessLocked(teamID, userID, "")
	jsonReply(w, 200, map[string]any{"team": row.Team, "projectId": projectID, "saved": true})
}

func scopedTeamActor(u User, access TeamAccess) User {
	if access.Permissions.Admin {
		u.Role = "admin"
	} else {
		u.Role = "member"
	}
	return u
}
func teamPolicy(policy Policy) Policy { policy.AllowHostCommands = false; return policy }

func teamRevisionError(access TeamAccess) error {
	return fmt.Errorf("team access revision %d is no longer current", access.Member.Revision)
}
