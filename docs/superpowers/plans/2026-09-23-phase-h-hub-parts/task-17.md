### Task 17: projects and their git remotes — routes 13–18 (correction A1)

**Lane A. Depends on: 16. About 2.5 hours. Contract task H3.**

A Hub project is keyed to its git remotes on **both** hosts (`OD-4`), because a local project id
is a hash of a path on one laptop and two teammates' ids never match (ADR-H1 §1.5). The control
plane maps a local project to a Hub project by asking `GET /v1/projects?repo=<repoKey>` with phase
3's `githost.RepoKey` (task 35). A repository is **one** project's, org-wide. Correction **A1**:
the Azure DevOps key form is phase 3's `azure-devops/<org>/<project>/<repo>`, validated by
`hubapi.ValidRepoKey` (task 01), never the contract's old `…visualstudio.com/…` regex.

**Files:**
- `control-plane/internal/hub/server/projects.go` + `projects_integration_test.go` (new)
- `control-plane/internal/hub/store/projects.go` (new)
- `control-plane/internal/hub/server/authz.go` — `ProjectRole` (modify)
- `control-plane/internal/hub/server/matrix_integration_test.go` — `rowsTask17`; the fixture gains `ProjectID`, `RepoKey` (modify)
- `control-plane/internal/hub/server/server.go` — `s.registerProjects()` (modify: one line)

#### Contract — the routes, complete

| # | Route | Role | Success | Refusals | Events |
|---|---|---|---|---|---|
| 13 | `GET /v1/projects?repo=&teamId=&after=&limit=` | any | `200 Page[Project]`, by `lower(name)` — **only projects of the caller's teams**; `repo=` gives zero or one item | 400 `field: repo` (not a valid key for either host) | — |
| 14 | `POST /v1/projects` `CreateProjectRequest` | M of `teamId` | `201 Project` | 404 `team_not_found` (not a member); 403 `forbidden` `required: member` (a viewer); 400 `field: name` (1–120 after trimming) or `field: remotes` (a key invalid for its host, a duplicate in the list, more than 20); 409 `conflict` `field: name` (unique per team, case-insensitive); 409 `repo_already_linked` | `project.created {"name"}`; one `project.remote_added {"repoKey","host"}` per remote |
| 15 | `GET /v1/projects/{projectId}` | V | `200 Project` | 404 `project_not_found` | — |
| 16 | `PATCH /v1/projects/{projectId}` `RenameProjectRequest` | A | `200 Project` | 404; 403 `required: admin`; 400/409 as 14 | `project.updated {"name"}` |
| 17 | `POST /v1/projects/{projectId}/remotes` `RemoteRequest` | A | `201 Remote` | 404; 403; 400 `field: repoKey` or `field: host`; 409 `repo_already_linked` | `project.remote_added {"repoKey","host"}` |
| 18 | `DELETE /v1/projects/{projectId}/remotes?repoKey=` | A | `204`; a key that is not this project's remote is also `204`, with no event (absent is not an error) | 404; 403; 400 `field: repoKey` | `project.remote_removed {"repoKey"}` |

- **`repo_already_linked`** (contract §4.2): `details.repoKey` always; `details.projectId` **only when
  the caller can read** the project that holds it — otherwise the answer would reveal a project id
  across teams. Its `message` is exactly *"Another team already shares this repository. Ask someone
  in that team to invite you."* — the desktop renders it verbatim (task 42), so it is written for
  the person, not the developer.
- **No project deletion** in v1 (provenance contract P13): `DELETE /v1/projects/{projectId}` is 405
  `method_not_allowed` (the chassis's rule, task 08).
- **The same repository in two orgs** is two projects: the unique index is `(org_id, repo_key)`
  (task 07). Each org is a separate Entra tenant.

```go
// control-plane/internal/hub/server/authz.go — filled in by this task.

// ProjectRole returns the caller's role in the team that owns projectID; ok=false
// when the caller cannot read it (not a member, another org, or no such project —
// indistinguishable, all 404 project_not_found).
func ProjectRole(ctx context.Context, tx *Tx, projectID string) (role hubapi.Role, teamID string, ok bool, err error)
```

#### Tests to write first — `projects_integration_test.go`

| Test | Asserts | Mutant it kills |
|---|---|---|
| `TestCreateAProjectWithRemotes` | Grace creates P with `github.com/lcfrai/panaceaapp` and `azure-devops/kpmgcomau/panacea app/panaceaapp` → 201; three events in order | a project whose remotes are not audited |
| `TestRepoKeysAreValidatedPerHost` | the old `kpmgcomau.visualstudio.com/p/r` → 400 `field: remotes`; `github.com/LCFRAI/x` → 400; a GitHub key sent as `azure-devops` → 400 | correction A1 regressing to the contract's regex |
| `TestARepositoryIsOneProjectsPerOrg` | a second project with the same key → 409 `repo_already_linked`; Xavier (org 2) linking the same key in his org → 201 | two projects claiming one repository; one org blocking another |
| `TestAlreadyLinkedHidesAnUnreadableProject` | Out Sider's team links a key Grace's team holds → 409 with `repoKey` and **no** `projectId`; Grace's own second attempt → `projectId` present | a cross-team leak of project ids |
| `TestProjectForRepoIsZeroOrOne` | `?repo=` of the linked key → one item for Grace, **zero** for Out Sider; an unlinked key → zero | the control plane mapping a local project to a project its user cannot see |
| `TestProjectNamesArePerTeam` | "Portal" twice in T → 409 `field: name`; "Portal" in another team → 201 | — |
| `TestAViewerCannotCreateAProject` | Vera → 403 `required: member` | a viewer writing |
| `TestRemovingARemote` | Grace removes one → 204 and an event; removing it again → 204, no event; `?repo=` no longer finds P | a phantom link after removal |
| `TestThereIsNoProjectDeletion` | `DELETE /v1/projects/{id}` → 405 with `Allow: GET, PATCH` | a release orphaned by a deleted project (P13) |

The matrix gains `rowsTask17` — from contract §5.3:

| Route | V | M | A | O | N | X |
|---|---|---|---|---|---|---|
| `GET /v1/projects?repo={RepoKey}` | 200 (1 item) | 200 (1) | 200 (1) | 200 (0) | 200 (0) | 200 (0) |
| `POST /v1/projects` (team T) | 403 | 201 | 201 | 404 | 404 | 404 |
| `GET /v1/projects/{ProjectID}` | 200 | 200 | 200 | 404 | 404 | 404 |
| `PATCH /v1/projects/{ProjectID}` | 403 | 403 | 200 | 404 | 404 | 404 |
| `POST /v1/projects/{ProjectID}/remotes` | 403 | 403 | 201 | 404 | 404 | 404 |
| `DELETE /v1/projects/{ProjectID}/remotes?repoKey=…` | 403 | 403 | 204 | 404 | 404 | 404 |

(The harness gains a `wantItems map[column]int` for list rows, so "200 with nothing" and "200 with
the project" are different expectations.)

#### Steps

- [ ] **Step 1:** Re-read contract §5.3, §7.4, §8; phase 3 task 13's `RepoKey` comment.
- [ ] **Step 2:** Tests and matrix rows; they fail. Then `store/projects.go`, `ProjectRole`,
  `projects.go`, the registration line.
- [ ] **Step 3:** Commit by pathspec.

#### Acceptance

```bash
go test -tags integration ./control-plane/internal/hub/server/ -race -count=1 -v \
  -run 'TestCreateAProject|TestRepoKeysAre|TestARepositoryIs|TestAlreadyLinked|TestProjectForRepo|TestProjectNamesAre|TestAViewerCannotCreate|TestRemovingARemote|TestThereIsNoProject|TestAuthorisationMatrix|TestEveryRoute' \
  2>&1 | grep -c "^--- PASS"                                                                  # 11
```

#### What this task does not claim

- That phase 3's `RepoKey()` canonicalises every remote form the way `ValidRepoKey` expects;
  phase 3 task 13 owns that and the conformance row `projects/repo_key_forms` checks the pair.
- Anything about local projects: the control plane's `p-…` ids never reach the Hub.
