package conformance

import (
	"testing"

	"github.com/kpmg-au-emu/panacea-app/control-plane/internal/hub/hubapi"
)

// The projects group: server task 17 (contract §7.4). The last row is
// carried/task-05.md item 13's projects half.
var projectRows = []row{
	{name: "repo_linked_once", fn: repoLinkedOnce},
	{name: "repo_key_forms", fn: repoKeyForms},
	{name: "for_repo_is_readable_only", fn: forRepoIsReadableOnly},
	{name: "org_admin_is_a_non_member", fn: orgAdminProjects},
}

// a second project with the same repo key → 409 repo_already_linked; the
// other project is named only to a caller who can read it.
func repoLinkedOnce(t *testing.T, e Env) {
	p := newProject(t, e)
	repo := p.Remotes[0]
	_, err := e.Admin.CreateProject(bg, hubapi.CreateProjectRequest{TeamID: e.TeamID, Name: "Second " + uniq(),
		Remotes: []hubapi.RemoteRequest{{RepoKey: repo.RepoKey, Host: repo.Host}}})
	wantErr(t, err, 409, hubapi.CodeRepoAlreadyLinked, "repoKey", repo.RepoKey, "projectId", p.ID)
	u := newTeam(t, e.Outsider)
	_, err = e.Outsider.CreateProject(bg, hubapi.CreateProjectRequest{TeamID: u.ID, Name: "Theirs " + uniq(),
		Remotes: []hubapi.RemoteRequest{{RepoKey: repo.RepoKey, Host: repo.Host}}})
	if he := wantErr(t, err, 409, hubapi.CodeRepoAlreadyLinked, "repoKey", repo.RepoKey); he.Problem.Details["projectId"] != "" {
		t.Errorf("a caller who cannot read the project was told its id %q", he.Problem.Details["projectId"])
	}
	_, err = e.Admin.AddRemote(bg, p.ID, hubapi.RemoteRequest{RepoKey: repo.RepoKey, Host: repo.Host})
	wantErr(t, err, 409, hubapi.CodeRepoAlreadyLinked, "repoKey", repo.RepoKey)
}

// phase 3's Azure DevOps form is accepted; the contract's old form is 400 (A1).
func repoKeyForms(t *testing.T, e Env) {
	p := newProject(t, e)
	ado := hubapi.RemoteRequest{RepoKey: "azure-devops/kpmgcomau/panacea app/r-" + uniq(), Host: hubapi.HostAzureDevOps}
	if r := must(e.Admin.AddRemote(bg, p.ID, ado))(t); r.RepoKey != ado.RepoKey || r.Host != hubapi.HostAzureDevOps || r.AddedAt == "" {
		t.Errorf("AddRemote(ado) = %+v, want the key and host as sent, with addedAt", r)
	}
	if got, found, err := e.Member.ProjectForRepo(bg, ado.RepoKey); err != nil || !found || got.ID != p.ID {
		t.Errorf("ProjectForRepo(ado) = %s %v %v, want the project", got.ID, found, err)
	}
	old := hubapi.RemoteRequest{RepoKey: "kpmgcomau.visualstudio.com/p/r-" + uniq(), Host: hubapi.HostAzureDevOps}
	_, err := e.Admin.AddRemote(bg, p.ID, old)
	wantErr(t, err, 400, hubapi.CodeBadRequest, "field", "repoKey")
	_, err = e.Admin.CreateProject(bg, hubapi.CreateProjectRequest{TeamID: e.TeamID, Name: "Old " + uniq(), Remotes: []hubapi.RemoteRequest{old}})
	wantErr(t, err, 400, hubapi.CodeBadRequest, "field", "remotes")
	_, _, err = e.Admin.ProjectForRepo(bg, old.RepoKey)
	wantErr(t, err, 400, hubapi.CodeBadRequest, "field", "repo")
}

// ProjectForRepo answers only a project the caller can read: ok false, no error.
func forRepoIsReadableOnly(t *testing.T, e Env) {
	p := newProject(t, e)
	repo := p.Remotes[0].RepoKey
	for _, w := range []who{{"admin", e.Admin}, {"member", e.Member}, {"viewer", e.Viewer}} {
		if got, found, err := w.hub.ProjectForRepo(bg, repo); err != nil || !found || got.ID != p.ID {
			t.Errorf("%s: ProjectForRepo = %s %v %v, want the project", w.name, got.ID, found, err)
		}
	}
	for _, s := range strangers(t, e) {
		if got, found, err := s.hub.ProjectForRepo(bg, repo); err != nil || found || got.ID != "" {
			t.Errorf("%s: ProjectForRepo = %+v %v %v, want ok false and no error", s.name, got, found, err)
		}
	}
	if _, found, err := e.Admin.ProjectForRepo(bg, newRepo()); err != nil || found {
		t.Errorf("an unlinked key = %v %v, want ok false", found, err)
	}
}

// carried item 13, projects' half: Project and AddRemote are 404, and
// ProjectForRepo not found, to an org admin in no team as to the outsider.
func orgAdminProjects(t *testing.T, e Env) {
	p := newProject(t, e)
	for _, s := range strangers(t, e) {
		t.Run(s.name, func(t *testing.T) {
			_, err := s.hub.Project(bg, p.ID)
			wantErr(t, err, 404, hubapi.CodeProjectNotFound, "projectId", p.ID)
			_, err = s.hub.AddRemote(bg, p.ID, hubapi.RemoteRequest{RepoKey: newRepo(), Host: hubapi.HostGitHub})
			wantErr(t, err, 404, hubapi.CodeProjectNotFound, "projectId", p.ID)
			if _, found, err := s.hub.ProjectForRepo(bg, p.Remotes[0].RepoKey); err != nil || found {
				t.Errorf("ProjectForRepo(T's repo) = %v %v, want not found", found, err)
			}
		})
	}
	if got := must(e.Admin.Project(bg, p.ID))(t); len(got.Remotes) != 1 {
		t.Errorf("the project has %d remotes after the strangers' calls, want 1", len(got.Remotes))
	}
}
