### Task 14: `githost` — GitHub and Azure DevOps behind one interface, and the PR route

OD-4: phase 3's PR path supports **both** hosts from v1, behind one host interface. A PR is how a
product branch reaches trunk; the remote's server merges it and writes its own merge commit,
possibly a squash, so phase 3 cannot put a trailer there (survey §7.3). What it can do is put the
brief key in the PR's description **as text, labelled as text** — a reference a reviewer can
read, not a provenance record — beside the list of lands the PR carries, each of which does carry
its merge link in git.

Opening a PR does not push. A branch that is not on the remote as it is here is refused with
`not_pushed`, and the Merge lens offers Push first (task 20). Merging a PR, reading its review, or
re-implementing any of the host's review surface is out of scope (brief §3).

**Files:**
- `control-plane/internal/githost/githost.go` — `Host`, `OpenRequest`, `PullRequest`, `HTTPError`, `For` (new)
- `control-plane/internal/githost/github.go` + test (new)
- `control-plane/internal/githost/azuredevops.go` + test (new)
- `control-plane/internal/api/pullrequests.go` + test — the route, the description builder (new)
- `control-plane/internal/api/gitdeps.go`, `gitroutes.go` — `// ── task 14 ──`
- `control-plane/cmd/api/main.go` — one `// ── task 14 ──` line, `gitDeps.Fetcher.AfterFetch = gitDeps.Lander.RefreshProject` (review I5). Task 14 is the first task after both 09 (`Lander`) and 12 (`Fetcher`), so no new edge is needed; task 04's `TestServeWiresGit` gains one assertion, that `AfterFetch` is non-nil, and `TestServeFetchRefreshesClones` below proves it does the work (a non-nil `func(...){}` passes the first; re-review b05 issue 8)
- `control-plane/cmd/api/main_phase3_test.go` — `TestServeFetchRefreshesClones` (new)

#### Interfaces — complete

```go
package githost

// OpenRequest is one PR to open.
type OpenRequest struct {
	Source string // branch name, no refs/heads/
	Target string
	Title  string
	Body   string // plain text
	Draft  bool
}

// PullRequest is an open PR on the host.
type PullRequest struct {
	Number   int    `json:"number"` // GitHub's number; Azure DevOps' pullRequestId
	WebURL   string `json:"webUrl"`
	Title    string `json:"title"`
	Source   string `json:"source"`
	Target   string `json:"target"`
	Draft    bool   `json:"draft"`
	Existing bool   `json:"existing"` // one was already open for this source and target, and this is it
}

// HTTPError is the host's API refusing. Message is the host's own text (GitHub's "message",
// Azure DevOps' "message"), capped at 2 KiB; the cockpit shows it as details.hostSays.
type HTTPError struct {
	Status  int
	Message string
}

func (e *HTTPError) Error() string

// Host is one git host's PR API.
type Host interface {
	Kind() Kind
	// OpenPullRequest opens a PR, or returns the one already open for the same source and
	// target with Existing set. The token goes only in the Authorization header.
	OpenPullRequest(ctx context.Context, cred credstore.Credential, r Remote, req OpenRequest) (PullRequest, error)
}

// For returns the implementation for a remote's kind.
func For(k Kind, client *http.Client) Host
```

```go
package githost

// GitHub is the REST API, version 2022-11-28.
//   POST {BaseURL}/repos/{owner}/{repo}/pulls
//        Authorization: Bearer <token>; Accept: application/vnd.github+json; X-GitHub-Api-Version: 2022-11-28
//        {"title","head":<source>,"base":<target>,"body","draft"}
//   201 → number, html_url, title, draft, head.ref, base.ref
//   422 whose errors[].message contains "A pull request already exists" →
//   GET  {BaseURL}/repos/{owner}/{repo}/pulls?head={owner}:{source}&base={target}&state=open → the first, Existing
//   any other non-2xx → *HTTPError
type GitHub struct {
	BaseURL string // "" means "https://api.github.com"
	Client  *http.Client
}

// AzureDevOps is the REST API, api-version 7.1.
//   POST {base}/{project}/_apis/git/repositories/{repo}/pullrequests?api-version=7.1
//        Authorization: Bearer <token> (Scheme Bearer) or Basic base64(":"+token) (BasicPAT)
//        {"sourceRefName":"refs/heads/<source>","targetRefName":"refs/heads/<target>",
//         "title","description","isDraft"}          — description cut at 4000 characters, the API's limit
//   201 → pullRequestId, title, isDraft, repository.webUrl; WebURL = webUrl + "/pullrequest/" + id
//   409 whose message contains "TF401179" (an active PR exists) →
//   GET  {base}/{project}/_apis/git/repositories/{repo}/pullrequests
//        ?searchCriteria.sourceRefName=refs/heads/<source>&searchCriteria.targetRefName=refs/heads/<target>
//        &searchCriteria.status=active&api-version=7.1 → value[0], Existing
// base is "https://dev.azure.com/{org}" for a dev.azure.com remote and "https://{org}.visualstudio.com"
// for a visualstudio.com remote; {project} and {repo} are path-escaped.
type AzureDevOps struct {
	BaseURL func(r Remote) string // nil means the rule above; tests point it at httptest
	Client  *http.Client
}
```

```go
// gitdeps.go
	// ── task 14 ──
	Hosts func(k githost.Kind) githost.Host // githost.For with the production client
```

**Wire: `POST /v1/projects/{projectId}/branches/{branchId}/pull-requests`**

```json
// request — every field optional
{ "target": "main", "title": "Evidence upload", "draft": false, "briefKey": "brief:b-42@6" }

// 201 Created (a new PR) or 200 OK (one was already open)
{ "number": 7, "webUrl": "https://github.com/lcfrai/panaceaapp/pull/7", "title": "Evidence upload",
  "source": "evidence-upload", "target": "main", "draft": false, "existing": false }
```

The handler, in order: `origin`'s URL (`no_remote`) → `ParseRemote` (`unknown_host`) →
`remote.ReadState` (`not_pushed`, `details.why` `never_pushed` or `ahead`) → `target`, defaulting
to `symbolic-ref --short refs/remotes/origin/HEAD` minus `origin/` (unknown → 400 "Name the target
branch") → `GitDeps.Credentials.Credential(remote.Host())` (`no_credential`, `details.host`) →
the description → `Hosts(kind).OpenPullRequest` (`*HTTPError` → 502 `host_failed` with
`details.hostSays` and `details.status`). `title` defaults to the branch name. `briefKey` must pass
`gitx.Message`'s value charset (400 otherwise) and is copied verbatim. Every git call the handler
makes (`origin`'s URL, `symbolic-ref`, and the ancestry check below) is `gitx.Read` in the project
with Scope Project (task-02 §HGH).

The description, plain text, built by one function so a test pins it:

```
Opened from Panacea.

Brief, as text (a reference, not a provenance record): brief:b-42@6

Lands on evidence-upload not yet on main, newest first:
- Land upload-ui into evidence-upload (d5f0a17) — reviewed against brief:b-42@6
- Land upload-api into evidence-upload (e19b7c2) — reviewed against brief:b-42@5
```

The land lines come from task 10's `History`, taking `kind: "land"` entries until the first one
reachable from `refs/remotes/origin/<target>`, at most 20. The "Brief, as text" line is absent
when `briefKey` is empty, and a land with no brief says "no brief was shown when this landed".

#### The tests to write first

`httptest.NewTLSServer` stands in for each host; the client under test trusts its certificate.

| Test | Asserts | Mutant it kills |
|---|---|---|
| `TestGitHubOpensAPR` | method, path `/repos/lcfrai/panaceaapp/pulls`, the three headers, and the JSON body exactly; 201 → the `PullRequest` fields | a request GitHub rejects |
| `TestGitHubReturnsTheExistingPR` | 422 "A pull request already exists…" then the GET → `Existing: true`, that PR's number | a second PR, or an error, for a PR that exists |
| `TestGitHubRefusalIsAnHTTPError` | 403 `{"message":"Resource not accessible by integration"}` → `*HTTPError{403, that text}` | the host's reason lost |
| `TestAzureDevOpsOpensAPR` | `POST /KPMGAU-BUS-Panacea/_apis/git/repositories/Panacea/pullrequests?api-version=7.1` against the visualstudio.com base; `refs/heads/` on both refs; WebURL built from `repository.webUrl` | the legacy `visualstudio.com` form unsupported |
| `TestAzureDevOpsPathEscapesTheProject` | project `My Project` → `/My%20Project/_apis/…` | a 404 for any project with a space |
| `TestAzureDevOpsReturnsTheExistingPR` | 409 TF401179 then the search GET → `Existing: true` | as GitHub's |
| `TestAzureDevOpsAuthPerScheme` | Bearer credential → `Authorization: Bearer x`; BasicPAT → `Basic base64(":x")` | an ADO PAT sent as a bearer token |
| `TestAzureDevOpsCutsTheDescription` | a 5000-character body → 4000 on the wire | a 400 from the API's limit |
| `TestTokenOnlyInTheAuthorizationHeader` | both hosts: the token appears in no URL, no body, no error | a token in a URL a proxy logs |
| `TestPRRouteRefusesAnUnpushedBranch` | never pushed → `not_pushed`/`never_pushed`; ahead → `/ahead`; no host call made | a PR that shows the remote's old commits |
| `TestPRRouteWithNoCredential` | `credstore.None` → 409 `no_credential`, `details.host` the authority | a 500 for "not signed in" |
| `TestPRRouteMapsTheHostsRefusal` | a fake Host returning `*HTTPError{403,…}` → 502 `host_failed`, `hostSays`, `status: "403"` | a host refusal shown as ours |
| `TestPRDescriptionIsPinned` | two lands (rev 6 and rev 5), a key → the text above, byte for byte; no key → no "Brief, as text" line | the key presented as a provenance record |
| `TestPRRouteDefaultsTargetFromOriginHead` | `refs/remotes/origin/HEAD` → `origin/main` → target `main`; unset and no `target` → 400 | a PR against the wrong base |
| `TestPRRouteRefusesAnInjectedKey` | a key with a newline → 400 | text injection into the description |
| `TestServeFetchRefreshesClones` (cmd/api; re-review b05 issue 8) | the Deps `serve` builds, over a temp state dir: a project whose `origin` is a bare repository, one cockpit worktree on `main` created through `Git.Creator`; a second clone of the bare repository pushes one commit to `main`; `Git.Fetcher.Fetch(ctx, <the project's ProjectRef>)` → the worktree clone's `refs/remotes/upstream/main` equals that commit, read through `lanegittest.Fake` as the lane's runner (task 24's seam: `GitDeps.LaneGit.Dial` set to `lanegittest.Fakes.Dial`, with `lanegittest.Backend`; review r4-1 I-3) or a `docker exec` into the lane's sandbox, never host git in the clone (R4.2); and the clone's `.git/config` bytes, read as a file before and after, are unchanged | `AfterFetch` wired to a no-op, or not wired: a teammate's push that no lane can see until the next land |

#### Steps

- [ ] **Step 1: Preflight.** Tasks 10 and 13 merged (`githost.Remote`, `credstore`; `Lander.RefreshProject` and `Fetcher.AfterFetch` for the one wiring line).
- [ ] **Step 2:** tests; `githost.go`, `github.go`, `azuredevops.go`; the route.
- [ ] **Step 3:** run; commit by pathspec.

#### Acceptance

```
go test ./control-plane/internal/githost/ ./control-plane/internal/api/ -race -count=1 → PASS
```

Manual, on the owner's laptop, recorded or marked not run: one draft PR on
`github.com/lcfrai/panaceaapp` from a throwaway branch, then closed by hand; one on an Azure DevOps
repository the owner names. **Neither is run by an agent without the owner's say-so**: both
create objects other people can see.

#### What this task does not claim

- **Neither host API was called while writing this plan.** Paths, headers and the two "already
  exists" signals are from the hosts' public documentation as recalled, and the `httptest` fakes
  encode that recollection. The manual check above is what verifies them.
- Go's HTTP client does **not** read the Windows system proxy. If the owner's network requires a
  proxy for `api.github.com` or `*.visualstudio.com`, the control plane needs `HTTPS_PROXY` in its
  environment; git itself uses its own `http.proxy`. Not measured.
- GitHub Enterprise Server and Azure DevOps Server are `unknown_host` in v1.
- A PR's review, checks and merge happen on the host, not in the cockpit.
