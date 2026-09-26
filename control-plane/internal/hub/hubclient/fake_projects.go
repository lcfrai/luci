package hubclient

import (
	"cmp"
	"context"
	"slices"
	"strings"
	"unicode/utf8"

	"github.com/kpmg-au-emu/panacea-app/control-plane/internal/hub/hubapi"
	"github.com/kpmg-au-emu/panacea-app/control-plane/internal/ids"
)

// The fake's teams, invites and projects (contract §5.3, §7.2–§7.4). Every
// check runs in the Hub's order: the 404 for what the caller cannot see, then
// the 403 for a role too low, then the 400 for the body, then the 409s.

type fakeTeam struct {
	id, name  string
	createdAt string
	createdBy string                 // a user id
	members   map[string]*fakeMember // by user id
	seq       int64
}

type fakeMember struct {
	role    hubapi.Role
	addedAt string
	addedBy string // a user id; "" is null (the team's creator)
}

type fakeInvite struct {
	id, teamID, email string
	role              hubapi.Role
	state             hubapi.InviteState
	createdAt         string
	createdBy         string // a user id
	boundUser         string // a user id once bound
	seq               int64
}

type fakeProject struct {
	id, teamID, name string
	remotes          []hubapi.Remote
	createdAt        string
	createdBy        string // a user id
	seq              int64
}

func (t *fakeTeam) order() int64    { return t.seq }
func (i *fakeInvite) order() int64  { return i.seq }
func (p *fakeProject) order() int64 { return p.seq }

// repoLinkedMessage is contract §7.4's sentence, which the desktop renders verbatim.
const repoLinkedMessage = "Another team already shares this repository. Ask someone in that team to invite you."

// ── lookups: what the caller can see ────────────────────────────────────────

// teamOf is teamID's team and the caller's role in it. A team the caller is not
// in, an unknown id and a malformed one are the same 404 (contract §4.2; task 15).
func (w *fakeWorld) teamOf(teamID, me string) (*fakeTeam, hubapi.Role, error) {
	if t := w.teams[teamID]; t != nil {
		if m := t.members[me]; m != nil {
			return t, m.role, nil
		}
	}
	return nil, "", problem(404, hubapi.CodeTeamNotFound, "There is no such team that you belong to.", "teamId", teamID)
}

// projectOf is projectID's project and the caller's role in its team; a project
// the caller cannot read is 404, as if it did not exist (contract §5.3).
func (w *fakeWorld) projectOf(projectID, me string) (*fakeProject, hubapi.Role, error) {
	if p := w.projects[projectID]; p != nil {
		if m := w.teams[p.teamID].members[me]; m != nil {
			return p, m.role, nil
		}
	}
	return nil, "", problem(404, hubapi.CodeProjectNotFound, "There is no such project that you can see.", "projectId", projectID)
}

func (w *fakeWorld) canRead(p *fakeProject, me string) bool {
	return w.teams[p.teamID].members[me] != nil
}

// linked is the project anywhere in the org with repoKey as a remote, or nil.
func (w *fakeWorld) linked(repoKey string) *fakeProject {
	for _, p := range w.projects {
		for _, r := range p.remotes {
			if r.RepoKey == repoKey {
				return p
			}
		}
	}
	return nil
}

// alreadyLinked names the other project only to a caller who can read it (§4.2).
func (w *fakeWorld) alreadyLinked(repoKey string, other *fakeProject, me string) error {
	kv := []string{"repoKey", repoKey}
	if w.canRead(other, me) {
		kv = append(kv, "projectId", other.id)
	}
	return problem(409, hubapi.CodeRepoAlreadyLinked, repoLinkedMessage, kv...)
}

// ── rendering ───────────────────────────────────────────────────────────────

func (w *fakeWorld) userRef(id string) *hubapi.User {
	if id == "" {
		return nil
	}
	u := w.users[id]
	return &u
}

func (w *fakeWorld) renderTeam(t *fakeTeam, me string) hubapi.Team {
	return hubapi.Team{ID: t.id, Name: t.name, Role: t.members[me].role, CreatedAt: t.createdAt, CreatedBy: w.users[t.createdBy]}
}

func (w *fakeWorld) renderMember(t *fakeTeam, userID string) hubapi.Member {
	m := t.members[userID]
	return hubapi.Member{User: w.users[userID], Role: m.role, AddedAt: m.addedAt, AddedBy: w.userRef(m.addedBy)}
}

func (w *fakeWorld) renderInvite(i *fakeInvite) hubapi.Invite {
	return hubapi.Invite{ID: i.id, TeamID: i.teamID, Email: i.email, Role: i.role, State: i.state,
		CreatedAt: i.createdAt, CreatedBy: w.users[i.createdBy], BoundUser: w.userRef(i.boundUser)}
}

func (w *fakeWorld) renderProject(p *fakeProject) hubapi.Project {
	return hubapi.Project{ID: p.id, TeamID: p.teamID, Name: p.name, Remotes: append([]hubapi.Remote{}, p.remotes...),
		CreatedAt: p.createdAt, CreatedBy: w.users[p.createdBy]}
}

// teamsOf is the caller's teams, by lower(name) (task 15), then id.
func (w *fakeWorld) teamsOf(me string) []hubapi.Team {
	out := []hubapi.Team{}
	for _, t := range w.teams {
		if t.members[me] != nil {
			out = append(out, w.renderTeam(t, me))
		}
	}
	slices.SortFunc(out, func(a, b hubapi.Team) int {
		return cmp.Or(strings.Compare(strings.ToLower(a.Name), strings.ToLower(b.Name)), strings.Compare(a.ID, b.ID))
	})
	return out
}

// name120 is a team or project name: 1–120 characters after trimming (§7.3, §7.4).
func name120(raw string) (string, error) {
	name := strings.TrimSpace(raw)
	if n := utf8.RuneCountInString(name); n < 1 || n > 120 || !utf8.ValidString(name) {
		return "", badRequest("name", "A name is 1 to 120 characters.")
	}
	return name, nil
}

// sameName is the case-insensitive uniqueness of team and project names.
func sameName(a, b string) bool { return strings.ToLower(a) == strings.ToLower(b) }

// ── /v1/me (§7.2) ───────────────────────────────────────────────────────────

// Me binds every pending invite in the org for the caller's email: the caller
// joins with the invite's role, or keeps a higher one already held.
func (f *Fake) Me(context.Context) (hubapi.Me, error) {
	w := f.w
	w.mu.Lock()
	defer w.mu.Unlock()
	me, err := f.enter("Me")
	if err != nil {
		return hubapi.Me{}, err
	}
	bound := 0
	for _, inv := range inOrder(w.invites) {
		if inv.state != hubapi.InvitePending || inv.email != w.users[me].Email {
			continue
		}
		inv.state, inv.boundUser = hubapi.InviteBound, me
		bound++
		t := w.teams[inv.teamID]
		// Task 16a: team.invite_bound per invite, then team.member_set when the
		// membership changed.
		w.emit(me, "team.invite_bound", t.id, "", inv.id, "userId", me, "role", string(inv.role))
		switch m := t.members[me]; {
		case m == nil:
			t.members[me] = &fakeMember{role: inv.role, addedAt: w.stamp(), addedBy: inv.createdBy}
			w.emit(me, "team.member_set", t.id, "", me, "role", string(inv.role))
		case rank(inv.role) > rank(m.role):
			m.role = inv.role
			w.emit(me, "team.member_set", t.id, "", me, "role", string(inv.role))
		}
	}
	return hubapi.Me{User: w.users[me], Org: w.org, OrgAdmin: f.as.OrgAdmin, Teams: w.teamsOf(me), InvitesBound: bound}, nil
}

// ── teams and members (§7.3) ────────────────────────────────────────────────

func (f *Fake) Teams(context.Context) ([]hubapi.Team, error) {
	w := f.w
	w.mu.Lock()
	defer w.mu.Unlock()
	me, err := f.enter("Teams")
	if err != nil {
		return nil, err
	}
	return w.teamsOf(me), nil
}

// CreateTeam makes the caller the new team's admin.
func (f *Fake) CreateTeam(_ context.Context, req hubapi.CreateTeamRequest) (hubapi.Team, error) {
	w := f.w
	w.mu.Lock()
	defer w.mu.Unlock()
	me, err := f.enter("CreateTeam", req)
	if err != nil {
		return hubapi.Team{}, err
	}
	name, err := name120(req.Name)
	if err != nil {
		return hubapi.Team{}, err
	}
	for _, t := range w.teams {
		if sameName(t.name, name) {
			return hubapi.Team{}, problem(409, hubapi.CodeConflict, "A team in this organisation already has that name.", "field", "name")
		}
	}
	now := w.stamp()
	t := &fakeTeam{id: ids.Wide(hubapi.PrefixTeam), name: name, createdAt: now, createdBy: me,
		members: map[string]*fakeMember{me: {role: hubapi.RoleAdmin, addedAt: now}}, seq: w.next()}
	w.teams[t.id] = t
	w.emit(me, "team.created", t.id, "", t.id, "name", name)
	w.emit(me, "team.member_set", t.id, "", me, "role", string(hubapi.RoleAdmin))
	return w.renderTeam(t, me), nil
}

// Team is the team and its members, by lower(display name) (task 15), then id.
func (f *Fake) Team(_ context.Context, teamID string) (hubapi.TeamDetail, error) {
	w := f.w
	w.mu.Lock()
	defer w.mu.Unlock()
	me, err := f.enter("Team", teamID)
	if err != nil {
		return hubapi.TeamDetail{}, err
	}
	t, _, err := w.teamOf(teamID, me)
	if err != nil {
		return hubapi.TeamDetail{}, err
	}
	members := []hubapi.Member{}
	for id := range t.members {
		members = append(members, w.renderMember(t, id))
	}
	slices.SortFunc(members, func(a, b hubapi.Member) int {
		return cmp.Or(strings.Compare(strings.ToLower(a.User.DisplayName), strings.ToLower(b.User.DisplayName)), strings.Compare(a.User.ID, b.User.ID))
	})
	return hubapi.TeamDetail{Team: w.renderTeam(t, me), Members: members}, nil
}

func (t *fakeTeam) admins() int {
	n := 0
	for _, m := range t.members {
		if m.role == hubapi.RoleAdmin {
			n++
		}
	}
	return n
}

func lastAdmin(teamID string) error {
	return problem(409, hubapi.CodeLastAdmin, "That would leave the team with no admin.", "teamId", teamID)
}

// SetMember adds a user of the org (addedBy the caller) or changes their role.
func (f *Fake) SetMember(_ context.Context, teamID, userID string, role hubapi.Role) (hubapi.Member, error) {
	w := f.w
	w.mu.Lock()
	defer w.mu.Unlock()
	me, err := f.enter("SetMember", teamID, userID, role)
	if err != nil {
		return hubapi.Member{}, err
	}
	t, mine, err := w.teamOf(teamID, me)
	if err != nil {
		return hubapi.Member{}, err
	}
	if err := need(mine, hubapi.RoleAdmin); err != nil {
		return hubapi.Member{}, err
	}
	if rank(role) == 0 {
		return hubapi.Member{}, badRequest("role", "A role is viewer, member or admin.")
	}
	if _, ok := w.users[userID]; !ok {
		return hubapi.Member{}, problem(404, hubapi.CodeMemberNotFound, "There is no such user in this organisation.", "userId", userID)
	}
	switch m := t.members[userID]; {
	case m == nil:
		t.members[userID] = &fakeMember{role: role, addedAt: w.stamp(), addedBy: me}
	case m.role == hubapi.RoleAdmin && role != hubapi.RoleAdmin && t.admins() == 1:
		return hubapi.Member{}, lastAdmin(teamID)
	case m.role == role:
		return w.renderMember(t, userID), nil // no record changed, so no event (§8)
	default:
		m.role = role
	}
	w.emit(me, "team.member_set", t.id, "", userID, "role", string(role))
	return w.renderMember(t, userID), nil
}

// RemoveMember: an admin removes anyone; anyone removes themself.
func (f *Fake) RemoveMember(_ context.Context, teamID, userID string) error {
	w := f.w
	w.mu.Lock()
	defer w.mu.Unlock()
	me, err := f.enter("RemoveMember", teamID, userID)
	if err != nil {
		return err
	}
	t, mine, err := w.teamOf(teamID, me)
	if err != nil {
		return err
	}
	if userID != me {
		if err := need(mine, hubapi.RoleAdmin); err != nil {
			return err
		}
	}
	m := t.members[userID]
	if m == nil {
		return problem(404, hubapi.CodeMemberNotFound, "That user is not a member of this team.", "userId", userID)
	}
	if m.role == hubapi.RoleAdmin && t.admins() == 1 {
		return lastAdmin(teamID)
	}
	delete(t.members, userID)
	w.emit(me, "team.member_removed", t.id, "", userID)
	return nil
}

// ── invites (§7.3) ──────────────────────────────────────────────────────────

// Invites is the team's invites, newest first; revoked and bound rows are kept.
func (f *Fake) Invites(_ context.Context, teamID string) ([]hubapi.Invite, error) {
	w := f.w
	w.mu.Lock()
	defer w.mu.Unlock()
	me, err := f.enter("Invites", teamID)
	if err != nil {
		return nil, err
	}
	t, _, err := w.teamOf(teamID, me)
	if err != nil {
		return nil, err
	}
	out := []hubapi.Invite{}
	for _, inv := range slices.Backward(inOrder(w.invites)) {
		if inv.teamID == t.id {
			out = append(out, w.renderInvite(inv))
		}
	}
	return out, nil
}

// CreateInvite: the email lowercased, 3–254 characters, exactly one "@".
func (f *Fake) CreateInvite(_ context.Context, teamID string, req hubapi.CreateInviteRequest) (hubapi.Invite, error) {
	w := f.w
	w.mu.Lock()
	defer w.mu.Unlock()
	me, err := f.enter("CreateInvite", teamID, req)
	if err != nil {
		return hubapi.Invite{}, err
	}
	t, mine, err := w.teamOf(teamID, me)
	if err != nil {
		return hubapi.Invite{}, err
	}
	if err := need(mine, hubapi.RoleAdmin); err != nil {
		return hubapi.Invite{}, err
	}
	email := strings.ToLower(req.Email)
	if n := utf8.RuneCountInString(email); n < 3 || n > 254 || strings.Count(email, "@") != 1 {
		return hubapi.Invite{}, badRequest("email", "An email is 3 to 254 characters with exactly one @.")
	}
	if rank(req.Role) == 0 {
		return hubapi.Invite{}, badRequest("role", "A role is viewer, member or admin.")
	}
	for id := range t.members {
		if w.users[id].Email == email {
			return hubapi.Invite{}, problem(409, hubapi.CodeConflict, "That person is already a member of this team.", "field", "email")
		}
	}
	for _, inv := range w.invites {
		if inv.teamID == t.id && inv.state == hubapi.InvitePending && inv.email == email {
			return hubapi.Invite{}, problem(409, hubapi.CodeConflict, "That email already has a pending invite to this team.")
		}
	}
	inv := &fakeInvite{id: ids.Wide(hubapi.PrefixInvite), teamID: t.id, email: email, role: req.Role,
		state: hubapi.InvitePending, createdAt: w.stamp(), createdBy: me, seq: w.next()}
	w.invites[inv.id] = inv
	w.emit(me, "team.invite_created", t.id, "", inv.id, "email", email, "role", string(req.Role))
	return w.renderInvite(inv), nil
}

// RevokeInvite keeps the row as revoked; a bound invite cannot be revoked.
func (f *Fake) RevokeInvite(_ context.Context, teamID, inviteID string) error {
	w := f.w
	w.mu.Lock()
	defer w.mu.Unlock()
	me, err := f.enter("RevokeInvite", teamID, inviteID)
	if err != nil {
		return err
	}
	t, mine, err := w.teamOf(teamID, me)
	if err != nil {
		return err
	}
	if err := need(mine, hubapi.RoleAdmin); err != nil {
		return err
	}
	inv := w.invites[inviteID]
	if inv == nil || inv.teamID != t.id {
		return problem(404, hubapi.CodeInviteNotFound, "There is no such invite to this team.", "inviteId", inviteID)
	}
	if inv.state == hubapi.InviteBound {
		return problem(409, hubapi.CodeConflict, "That invite has been accepted and cannot be revoked.")
	}
	if inv.state == hubapi.InvitePending { // a second revoke changes nothing and emits nothing
		inv.state = hubapi.InviteRevoked
		w.emit(me, "team.invite_revoked", t.id, "", inv.id)
	}
	return nil
}

// ── projects and remotes (§7.4) ─────────────────────────────────────────────

// maxRemotes bounds one CreateProject's remotes (task 17).
const maxRemotes = 20

// ProjectForRepo is GET /v1/projects?repo=: only the caller's teams' projects.
func (f *Fake) ProjectForRepo(_ context.Context, repoKey string) (hubapi.Project, bool, error) {
	w := f.w
	w.mu.Lock()
	defer w.mu.Unlock()
	me, err := f.enter("ProjectForRepo", repoKey)
	if err != nil {
		return hubapi.Project{}, false, err
	}
	if _, ok := hubapi.HostOfRepoKey(repoKey); !ok {
		return hubapi.Project{}, false, badRequest("repo", "That is not a repository key of either host.")
	}
	if p := w.linked(repoKey); p != nil && w.canRead(p, me) {
		return w.renderProject(p), true, nil
	}
	return hubapi.Project{}, false, nil
}

func (f *Fake) Project(_ context.Context, projectID string) (hubapi.Project, error) {
	w := f.w
	w.mu.Lock()
	defer w.mu.Unlock()
	me, err := f.enter("Project", projectID)
	if err != nil {
		return hubapi.Project{}, err
	}
	p, _, err := w.projectOf(projectID, me)
	if err != nil {
		return hubapi.Project{}, err
	}
	return w.renderProject(p), nil
}

// CreateProject needs member of the team. Every remote is phase 3's key form
// for its host (A1), distinct, at most 20, and linked nowhere else in the org.
func (f *Fake) CreateProject(_ context.Context, req hubapi.CreateProjectRequest) (hubapi.Project, error) {
	w := f.w
	w.mu.Lock()
	defer w.mu.Unlock()
	me, err := f.enter("CreateProject", req)
	if err != nil {
		return hubapi.Project{}, err
	}
	t, mine, err := w.teamOf(req.TeamID, me)
	if err != nil {
		return hubapi.Project{}, err
	}
	if err := need(mine, hubapi.RoleMember); err != nil {
		return hubapi.Project{}, err
	}
	name, err := name120(req.Name)
	if err != nil {
		return hubapi.Project{}, err
	}
	if len(req.Remotes) > maxRemotes {
		return hubapi.Project{}, badRequest("remotes", "A project starts with at most 20 remotes.")
	}
	seen := map[string]bool{}
	for _, r := range req.Remotes {
		if !hubapi.ValidRepoKey(r.Host, r.RepoKey) || seen[r.RepoKey] {
			return hubapi.Project{}, badRequest("remotes", "Each remote is a distinct repository key of its host.")
		}
		seen[r.RepoKey] = true
	}
	for _, p := range w.projects {
		if p.teamID == t.id && sameName(p.name, name) {
			return hubapi.Project{}, problem(409, hubapi.CodeConflict, "A project in this team already has that name.", "field", "name")
		}
	}
	for _, r := range req.Remotes {
		if other := w.linked(r.RepoKey); other != nil {
			return hubapi.Project{}, w.alreadyLinked(r.RepoKey, other, me)
		}
	}
	now := w.stamp()
	p := &fakeProject{id: ids.Wide(hubapi.PrefixProject), teamID: t.id, name: name, createdAt: now, createdBy: me, seq: w.next()}
	for _, r := range req.Remotes {
		p.remotes = append(p.remotes, hubapi.Remote{RepoKey: r.RepoKey, Host: r.Host, AddedAt: now})
	}
	w.projects[p.id] = p
	w.emitIn(me, "project.created", p, p.id, "name", name)
	for _, r := range p.remotes {
		w.emitIn(me, "project.remote_added", p, p.id, "repoKey", r.RepoKey, "host", string(r.Host))
	}
	return w.renderProject(p), nil
}

// AddRemote needs admin; the key is phase 3's form for its host (A1).
func (f *Fake) AddRemote(_ context.Context, projectID string, req hubapi.RemoteRequest) (hubapi.Remote, error) {
	w := f.w
	w.mu.Lock()
	defer w.mu.Unlock()
	me, err := f.enter("AddRemote", projectID, req)
	if err != nil {
		return hubapi.Remote{}, err
	}
	p, mine, err := w.projectOf(projectID, me)
	if err != nil {
		return hubapi.Remote{}, err
	}
	if err := need(mine, hubapi.RoleAdmin); err != nil {
		return hubapi.Remote{}, err
	}
	if req.Host != hubapi.HostGitHub && req.Host != hubapi.HostAzureDevOps {
		return hubapi.Remote{}, badRequest("host", "A host is github or azure-devops.")
	}
	if !hubapi.ValidRepoKey(req.Host, req.RepoKey) {
		return hubapi.Remote{}, badRequest("repoKey", "That is not a repository key of its host.")
	}
	if other := w.linked(req.RepoKey); other != nil {
		return hubapi.Remote{}, w.alreadyLinked(req.RepoKey, other, me)
	}
	r := hubapi.Remote{RepoKey: req.RepoKey, Host: req.Host, AddedAt: w.stamp()}
	p.remotes = append(p.remotes, r)
	w.emitIn(me, "project.remote_added", p, p.id, "repoKey", r.RepoKey, "host", string(r.Host))
	return r, nil
}
