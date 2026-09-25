### Task 48: Phase 3 delta: the read-only per-project fetch credential (OD-22)

**Slice TM1. Depends on: 17, P3-12, P3-13, P3-14. About 3 hours. Seam: none new.**

OD-22 answers the design's Q11 (`05-design.md:1333`): the cloud **may fetch** a project's
repository while nobody is signed in, **read-only, per project**; every push still uses a
signed-in person's own credential (`2026-09-23-owner-decisions.md:150`). Three things need it: the
slow-timer fetch that keeps lanes fresh (phase 3 task 12, `task-12.md:16-17`), a typed `rebase`
directive (task 31), and the Orchestrator's view of `origin` (tasks 22, 25). Phase 3 today fetches
**on the laptop**, with the machine's own git credential helper (`task-12.md:4-10`), which is
exactly the case where nobody is there. This task is a delta on `feat/phase-3-git` (worktree
`.claude/worktrees/phase-3`, read at `9935278`) and on phase 8's broker; **none of the files it
extends is built** on either branch (phase 3 has no `remote`, `credstore` or `githost` code;
phase 8 has no broker), so every path is **UNVERIFIED until merge** and cites its plan row.

**The one thing this task must not get wrong is that a read credential can never write.** The
token a host issues is read-only (`contents: read`; an Azure DevOps identity with Read on one
repository), so the host would refuse a push anyway. That is the second wall, not the first. The
first is in our code: a fetch credential is a distinct type of `remote.Auth` and
`credstore.Credential`, and **`remote.Push` and `githost.OpenPullRequest` refuse it before any
process starts or any request is sent**. A test counts zero git calls and zero HTTP requests.

**The second thing is that the project credential never outlives a person's own access by more
than one sign-in.** Fetches go into *her* clones on *her* machine disk. So the broker issues the
project credential for her machine only while it holds a fresh **proof**: at each sign-in, her
own desktop checks her own repository access with her own token (phase 3 task 13's chain) and
reports the result. A failed or missing proof stops every fetch into her clones, the timer's
included (`05-design.md:1059-1062`).

**Where it runs.** The broker holds the credential's source material and mints a token per fetch;
the fetch itself runs in `machined` through phase 8 task 21's `POST /v1/git` with the header in
`Env`, host-scoped exactly as phase 3's `PushEnv` scopes a push's (`task-13.md:168-175`; phase 8
`task-21.md:168-197`). So in the cloud the phase 3 `remote.Fetcher` runs **in the broker**, over
the remote `gitx.Machine` of each machine, and `machined` never stores a token. The lane mirror
(task 59) is the only other caller. **No credential enters a session sandbox** (global rule 8).

**Files:**
- `control-plane/internal/credstore/credstore.go` (phase 3 task 13, planned): `SourceProjectFetch`
  (modify)
- `control-plane/internal/githost/fetchcred.go` + test: `Binding`, `FetchSource`, `GitHubApp`,
  `AzureDevOpsIdentity`, `FetchEnv` (new, **UNVERIFIED until merge**: the package is phase 3's)
- `control-plane/internal/githost/prove.go` + test: `ProveAccess` (new)
- `control-plane/internal/githost/githost.go` (phase 3 task 14): `OpenPullRequest`'s
  implementations refuse a project-fetch credential (modify)
- `control-plane/internal/remote/fetch.go`, `push.go` (phase 3 task 12): `Auth.readOnly`,
  `FetchOnly`, `Fetcher.Authorize`, `Push`'s refusal (modify)
- `control-plane/cmd/broker/fetch.go` + test: the proof store, the per-machine fetch loop, the two
  routes, the audit line (new, **UNVERIFIED until merge**: phase 8 tasks 17–19)
- `control-plane/internal/api/signin_proof.go` + test: after sign-in, prove and report (new; the
  local control plane's sign-in hook is the Hub's, Hub task 33 **UNVERIFIED until merge**)
- `infra/terraform/modules/keyvault/*`, `infra/terraform/stacks/workloads/*` (phase 8 tasks 05, 10,
  planned): the GitHub App key secret, readable by the broker's identity only; one user-assigned
  identity per Azure DevOps team project, federated to the broker's service account (modify)
- `deploy/charts/broker/templates/*` (phase 8 task 29, planned): the key's CSI mount, the bindings
  file (modify)
- `docs/runbooks/fetch-credential.md`: installing the GitHub App; granting an Azure DevOps
  identity Read on one repository (new)

#### Interfaces — complete

```go
// control-plane/internal/credstore/credstore.go (modify: one constant; Credential unchanged)

// SourceProjectFetch marks the read-only per-project credential of OD-22. It is
// never a person's, never in a Store chain, and remote.Push and every githost
// write refuse it by this value before any process or request (Phase T task 48).
const SourceProjectFetch = "project-fetch"
```

```go
// control-plane/internal/githost/fetchcred.go
package githost

// Binding says how one project's repository is fetched without a person. It
// holds no secret: an installation id or a client id is an identifier, and the
// key material stays in Key Vault, mounted into the broker only.
type Binding struct {
	ProjectID string `json:"projectId"` // pj-…
	RepoKey   string `json:"repoKey"`   // Remote.RepoKey(): "github.com/o/r" or "azure-devops/org/p/r"
	// Exactly one of the two, matching RepoKey's host.
	GitHub *struct {
		AppID          int64 `json:"appId"`
		InstallationID int64 `json:"installationId"`
	} `json:"github,omitempty"`
	AzureDevOps *struct {
		ClientID string `json:"clientId"` // the project's user-assigned identity, federated to the broker
		TenantID string `json:"tenantId"`
	} `json:"azureDevOps,omitempty"`
}

// Validate refuses a binding whose provider does not match its RepoKey, has
// both or neither provider, or names a RepoKey ParseRemote would not produce.
func (b Binding) Validate() error

// FetchSource mints a fetch credential for one binding, valid about an hour.
// Every Credential it returns has Source SourceProjectFetch.
type FetchSource interface {
	FetchCredential(ctx context.Context, b Binding, r Remote) (credstore.Credential, error)
}

// GitHubApp mints an installation token narrowed to one repository and one
// permission (GitHub REST, version 2022-11-28, from knowledge, UNVERIFIED):
//   POST {BaseURL}/app/installations/{installationId}/access_tokens
//        Authorization: Bearer <app JWT, RS256, iss=appId, iat=now-60s, exp=now+9m>
//        {"repositories":["<repo>"],"permissions":{"contents":"read"}}
//   201 → {"token","expires_at","permissions","repositories":[{"full_name"}]}
// The answer is checked, not trusted: permissions must be exactly
// {"contents":"read"} (a "metadata":"read" GitHub adds is allowed) and
// repositories exactly the one; anything wider is ErrCredentialTooWide and the
// token is revoked (DELETE {BaseURL}/installation/token) before returning.
type GitHubApp struct {
	BaseURL string            // "https://api.github.com"
	Key     func() ([]byte, error) // the PEM, read from the CSI mount at each call
	Client  *http.Client
	Now     func() time.Time
}

// AzureDevOpsIdentity gets an Entra token for Azure DevOps' resource
// ("499b84ac-1321-427f-aa17-267ca6975798/.default", from knowledge, UNVERIFIED)
// as the binding's ClientID, by client assertion with the broker's own
// projected service-account token (workload identity). Which repository it can
// read is decided in Azure DevOps by the runbook's one grant, not here.
type AzureDevOpsIdentity struct {
	TokenURL  string // "https://login.microsoftonline.com/{tenant}/oauth2/v2.0/token"
	Assertion func() (string, error) // the file AZURE_FEDERATED_TOKEN_FILE names
	Client    *http.Client
}

var (
	ErrNoBinding          = errors.New("githost: the project has no fetch binding")
	ErrCredentialTooWide  = errors.New("githost: the host issued more than contents:read on one repository")
	ErrFetchCredentialNotForWrite = errors.New("githost: a project fetch credential cannot push or open a pull request")
)

// FetchEnv is PushEnv's twin for a fetch: the same host-scoped extraHeader,
// wrapped by remote.FetchOnly so it can never be passed to Push.
func FetchEnv(r Remote, cred credstore.Credential) (remote.Auth, error) // error unless Source == SourceProjectFetch
```

`githost` importing `remote` adds no cycle: `remote` imports `gitx` only, and `cmd/api/main.go`
already joins the two for `PushAuth` (`task-13.md:187-189`). If phase 3's merged graph says
otherwise, `FetchEnv` moves to `remote` and takes the header string.

```go
// control-plane/internal/remote/fetch.go and push.go (modify)

type Auth struct {
	Env      map[string]string
	readOnly bool // set only by FetchOnly; the zero Auth and PushEnv's are writable
}

// FetchOnly wraps a fetch header's Env. Push refuses the result.
func FetchOnly(env map[string]string) Auth { return Auth{Env: env, readOnly: true} }

// ReadOnly reports whether a is a project fetch credential's.
func (a Auth) ReadOnly() bool { return a.readOnly }

var ErrReadOnlyAuth = errors.New("remote: a read-only fetch credential cannot push")

// Push, first statement:  if auth.readOnly { return PushResult{}, ErrReadOnlyAuth }
// before the lock, before any gitx call.

// Fetcher gains one field. When Authorize is set it replaces AuthFor: an error
// is recorded as the project's LastError and no git runs for that project.
	Authorize func(ctx context.Context, p ProjectRef) (Auth, error)
```

```go
// control-plane/internal/githost/prove.go

// ProveAccess checks, with a person's OWN credential, that she can read the
// repository. It is the §10.2 re-proof, run by her desktop's control plane at
// each sign-in. It sends one request and changes nothing:
//   GitHub:       GET {BaseURL}/repos/{owner}/{repo}  → 200 and permissions.pull == true
//   Azure DevOps: GET https://dev.azure.com/{org}/{project}/_apis/git/repositories/{repo}?api-version=7.1 → 200
// 401, 403 and 404 are ErrNoAccess (a private repository she cannot see is 404
// on both hosts, from knowledge); any other failure is returned as it is, and
// is NOT a failed proof: "GitHub is down" must not stop her fetches.
func ProveAccess(ctx context.Context, h Host, cred credstore.Credential, r Remote) error

var ErrNoAccess = errors.New("githost: this person cannot read the repository")
```

`ProveAccess` refuses a credential whose `Source` is `SourceProjectFetch`
(`ErrFetchCredentialNotForWrite` is the wrong word; it returns `ErrNotAPersonsCredential`): the
proof must be hers, or it proves nothing.

#### The broker's half — complete wire

Two routes on phase 8's broker (tasks 17–18, planned), beside task 17's Fleet routes.

**`PUT /v1/machines/{machineId}/fetch-proofs/{projectId}`**: the machine owner's **desktop** token
only (Fleet → 403 `forbidden`; another person → 404, as every broker machine route). Body:

```jsonc
{ "repoKey": "github.com/o/r", "ok": true | false }   // ok=false carries no reason text
// 204. The broker writes the proof and emits one audit line.
```

The broker keeps proofs on the machine's `PersistentVolumeClaim` as the annotation
`panacea-app/fetch-proofs`, a JSON object `{"pj-…": {"repoKey", "ok", "at"}}`, so a sleeping
machine (no pod) and a restarted broker keep them (phase 8's machine disk is that claim, P8-D6,
**UNVERIFIED** until merge). A proof is **fresh** while `ok` is true and `at` is within
`PANACEA_APP_BROKER_FETCH_PROOF_TTL` (default `24h`, the Hub's delegation ceiling,
`task-01.md:237`). The repoKey must equal the project's binding; a mismatch is 422.

**`POST /v1/machines/{machineId}/projects/{projectId}/fetch`**: Fleet under the machine owner's
live delegation (task 17's check, the class `orchestrator` or `worker`), or the owner's desktop.
No body. The broker runs one `Fetcher.Fetch` for that project on that machine and answers phase
3's `FetchResult` (`task-12.md:75-78`), or 409 `fetch_proof_required` (no fresh proof), 404
`no_binding`, 502 `fetch_failed` with git's sentence. Task 31's `rebase` directive and the
Orchestrator's view call it; nothing else does.
