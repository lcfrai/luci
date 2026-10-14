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
`task-21.md:357-387` at `feat/env` `42397d8e`). So in the cloud the phase 3 `remote.Fetcher` runs **in the broker**, over
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

**Interfaces:**
- Consumes: phase 3's `remote.Auth`, `Fetcher`, `Push` (`task-12.md:28-126`), `credstore.Credential`,
  `Store`, `Chain` and `githost.Remote`, `ParseRemote`, `GitAuthHeader` (`task-13.md:28-175`),
  `githost.Host` (`task-14.md:57-66`); phase 8 task 21's `POST /v1/git` and its credential-`Env`
  rule (`task-21.md:222`, `:383-387` at `42397d8e`); task 17's delegation check for Fleet on broker routes.
- Produces: `credstore.SourceProjectFetch`; `githost.Binding`, `FetchSource`, `GitHubApp`,
  `AzureDevOpsIdentity`, `FetchEnv`, `ProveAccess`; `remote.FetchOnly`, `ErrReadOnlyAuth`,
  `Fetcher.Authorize`; the broker's `FetchCredential`, two routes and audit line.
- Required by: task 31 (`rebase` calls the fetch route), task 22 (the Orchestrator's view of
  `origin` is the last fetch), task 44 (leavers' proofs), task 57 (the live check), task 59 (the
  lane mirror fetches with the same `FetchCredential`).

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

var (
	ErrNoAccess              = errors.New("githost: this person cannot read the repository")
	ErrNotAPersonsCredential = errors.New("githost: a re-proof needs the person's own credential")
)
```

`ProveAccess` refuses a credential whose `Source` is `SourceProjectFetch` with
`ErrNotAPersonsCredential`, before any request: the proof must be hers, or it proves nothing.

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
`task-01.md:242`). The repoKey must equal the project's binding; a mismatch is 422.

**`POST /v1/machines/{machineId}/projects/{projectId}/fetch`**: Fleet under the machine owner's
live delegation (task 17's check, the class `orchestrator` or `worker`), or the owner's desktop.
No body. The broker runs one `Fetcher.Fetch` for that project on that machine and answers phase
3's `FetchResult` (`task-12.md:75-78`), or 409 `fetch_proof_required` (no fresh proof), 404
`no_binding`, 502 `fetch_failed` with git's sentence. Task 31's `rebase` directive and the
Orchestrator's view call it; nothing else does.

```go
// control-plane/cmd/broker/fetch.go (new, UNVERIFIED until phase 8's broker merges)

// FetchCredential is the skeleton's FetchCredential(project): the one place the
// broker turns a binding into a token. It refuses without a fresh proof for the
// machine's owner, and it writes the audit line whatever the outcome.
func (b *Broker) FetchCredential(ctx context.Context, machine MachineRef, projectID string) (credstore.Credential, error)

// fetchLoop is phase 3's slow timer, moved to the broker for team projects:
// every PANACEA_APP_FETCH_INTERVAL (phase 3's key and default, 10m, min 1m), for
// each awake machine, one remote.Fetcher over that machine's gitx.Machine
// (phase 8 task 21's remote runner), with Projects = the team projects of the
// machine that have a binding AND a fresh proof, and Authorize = FetchEnv of
// FetchCredential. A sleeping machine is skipped: its clones cannot move.
// Singleton: it runs on the replica holding phase 8's broker lease.
func (b *Broker) fetchLoop(ctx context.Context)
```

**The audit line.** Every `FetchCredential` call, issued or refused, writes one line through
phase 8's broker audit logger (task 17's `TestAuditLine`, shape **UNVERIFIED until merge**), which
phase 8 ships to Log Analytics. That is "every fetch audited to the project": the project id is a
field, and a Sentinel query per project lists them.

```json
{ "event": "fetch_credential", "projectId": "pj-…", "machineId": "…", "ownerOid": "…",
  "repoKey": "github.com/o/r", "provider": "github-app" | "azure-devops-identity",
  "via": "timer" | "fleet" | "desktop" | "mirror", "delegationId": "dl-…" | "",
  "result": "issued" | "no_proof" | "proof_failed" | "no_binding" | "too_wide" | "host_error",
  "expiresAt": "…" | "" }
```

No token, no header and no key id is ever a field; a test greps the captured log for the token.

**Bindings and key material.** `PANACEA_APP_BROKER_FETCH_BINDINGS` names a JSON file, `[]Binding`,
mounted from the broker chart's values (`deploy/values/<env>/broker.yaml`, `fetchBindings:`),
validated at start-up (one invalid binding stops the broker, naming the project). The GitHub
App's private key is one Key Vault secret, `github-fetch-app-key`, with **Key Vault Secrets
User** granted on that secret to the broker's identity alone (phase 8 task 05's module takes a
per-secret reader list; **UNVERIFIED** until merge), mounted by the Secrets Store CSI driver at
`/var/run/panacea/fetch/github-app.pem` and read at each mint, so rotation needs no restart. Azure
DevOps needs no secret: `stacks/workloads` creates one user-assigned identity per entry of a new
`ado_fetch_projects` map, each with one federated credential trusting the broker's service account
subject. Granting it Read on one repository is done in Azure DevOps by an admin (the runbook),
because `guards.sh` G9 admits only the `azurerm`, `azuread` and `azapi` providers
(`infra/scripts/guards.sh:33`, built), not an Azure DevOps provider.

**The sign-in proof** (`control-plane/internal/api/signin_proof.go`, the local control plane):
after the Hub sign-in completes (Hub task 33's hook, **UNVERIFIED until merge**), for each team
project of hers with a cloud machine, the control plane reads the binding's `repoKey`, asks phase 3
task 13's `Credentials` chain for **her** credential for that host, calls `ProveAccess`, and `PUT`s
the result. `ErrNoCredential` or `ErrNoAccess` → `ok: false`; any other error → nothing is sent,
and the previous proof ages out on its own TTL. The cockpit shows a false proof on the project's
row as one sentence: *"Fetches into your cloud clones are paused: your access to <repo> could not
be confirmed. Sign in again, or ask the repository's admin."*

#### Tests to write first

The first two are the scope paragraph's (`2026-09-25-phase-t-team-mode.md:1071-1072`).

| Test | Asserts | Mutant it kills |
|---|---|---|
| `TestPushWithTheFetchCredentialIsRefusedBeforeAnyCall` (`remote`) | `Push(…, FetchOnly(env))` → `ErrReadOnlyAuth`; phase 3's fake `gitx.Runner` recorded **0** calls and the project lock was never taken | the check placed after `Locks.Lock` or after a `rev-parse`, so a read credential reaches git |
| `TestAFailedReproofStopsTheTimer` (`cmd/broker`) | a fresh proof → one tick fetches the project (fake machine saw one `fetch`); `PUT … ok:false` → the next tick sees no `fetch` for it, and the audit line says `proof_failed`; a proof 24 h 1 s old → no `fetch` either (`no_proof`) | the timer listing projects by binding alone; the TTL never checked |
| `TestOpenPullRequestRefusesTheFetchCredential` (`githost`) | GitHub and Azure DevOps `OpenPullRequest` with a `SourceProjectFetch` credential → `ErrFetchCredentialNotForWrite`; the `httptest` server saw **0** requests | the host's own 403 relied on as the only wall |
| `TestGitHubAppAsksForOneRepoContentsRead` | the stand-in received the exact body `{"repositories":["r"],"permissions":{"contents":"read"}}` and a JWT whose `iss` is the app id, `exp − iat ≤ 10m` | a token for the whole installation |
| `TestAWiderTokenIsRevokedAndRefused` | the stand-in answers `contents: write`, or two repositories → `ErrCredentialTooWide`, and it then received `DELETE /installation/token` with that token | trusting the answer; leaving a wide token alive |
| `TestFetchEnvIsHostScopedAndReadOnly` | `FetchEnv` → `GIT_CONFIG_KEY_0 = http.https://github.com/.extraHeader`, `ReadOnly() == true`; with a person's credential → error | a fetch header valid for every host; a person's token laundered as read-only |
| `TestProveAccessUsesHerCredentialAndTreatsOutagesAsUnknown` | 200 + `pull: true` → nil; 404 → `ErrNoAccess`; 503 → the error, not `ErrNoAccess`; a `SourceProjectFetch` credential → `ErrNotAPersonsCredential`, 0 requests | a GitHub outage pausing every member's fetches; the project credential proving itself |
| `TestSigninSendsFalseOnNoCredentialAndNothingOnOutage` (`api`) | no credential → `PUT ok:false`; `ErrNoAccess` → `ok:false`; a 503 → no `PUT` | an outage recorded as a failed proof |
| `TestProofRouteIsTheOwnersDesktopOnly` | Fleet under her delegation → 403; Grace's desktop on Linus's machine → 404; a `repoKey` other than the binding's → 422 | Fleet vouching for a person's access |
| `TestTheFetchRouteNeedsAFreshProof` | no proof → 409 `fetch_proof_required`, no machine call; a fresh one → 200 `FetchResult` | the route bypassing the proof that the timer honours |
| `TestTheAuditLineNeverHoldsTheToken` | after an issue, the captured audit output does not contain the token, the header or `x-access-token` | a debug field left in |
| `TestNoSessionSandboxEverReceivesTheCredential` | for every create path of task 17's fake cluster, no pod spec, Secret or env of a `component: sandbox` pod holds the token or the PEM path | the credential handed to a worker "for convenience" |
| `TestBindingsValidateAtStart` | a GitHub binding on an Azure DevOps `repoKey`, both providers, neither → start-up fails naming the project | a silently unused binding |

#### Steps

- [ ] **Step 1: Read after the merges.** Phase 3 tasks 12–14 as built (`remote`, `credstore`,
  `githost`), phase 8 tasks 17–21 and 29 as built (the broker, `machined`'s `/v1/git`, the broker
  chart), phase 8 tasks 05 and 10's modules. Rename anything this part named provisionally.
- [ ] **Step 2: `remote` and `credstore`.** The push test first; it fails; `readOnly`,
  `FetchOnly`, `Authorize`, the refusal; `go test ./internal/remote/` passes.
- [ ] **Step 3: `githost`.** The six `githost` tests against `httptest` stand-ins; they fail;
  `fetchcred.go`, `prove.go`, the `OpenPullRequest` refusal; they pass.
- [ ] **Step 4: The broker.** The five broker tests on phase 8's fake cluster; they fail; the proof
  store, the two routes, `FetchCredential`, `fetchLoop`, the audit line; they pass.
- [ ] **Step 5: Sign-in.** The `api` test; the hook; it passes.
- [ ] **Step 6: Infrastructure.** The Key Vault secret and its one reader, the `ado_fetch_projects`
  map and federated credentials, the broker chart's CSI mount and `fetchBindings`;
  `bash infra/scripts/validate.sh` and `helm unittest deploy/charts/broker` pass.
- [ ] **Step 7: Runbook.** `docs/runbooks/fetch-credential.md`: install the App on one repository
  with *Contents: read* only; add the project's identity to the Azure DevOps organisation with no
  licence beyond Stakeholder or Basic as the org requires, and grant it *Read* on the one
  repository only (both portal steps from knowledge, **UNVERIFIED**).
- [ ] **Step 8: Commit** each group with `git commit -F <msg> -- <paths>`.

#### Acceptance

```bash
cd control-plane
go test ./internal/remote/ -run 'TestPushWithTheFetchCredential' -count=1
# ok  	github.com/kpmg-au-emu/panacea-app/control-plane/internal/remote	…
go test ./internal/githost/ -run 'TestOpenPullRequestRefuses|TestGitHubApp|TestAWiderToken|TestFetchEnv|TestProveAccess' -count=1
# ok  	…/internal/githost	…
go test ./cmd/broker/ -run 'TestAFailedReproof|TestProofRoute|TestTheFetchRoute|TestTheAuditLine|TestNoSessionSandbox|TestBindingsValidate' -count=1
# ok  	…/cmd/broker	…
go test ./internal/api/ -run TestSigninSends -count=1 && go vet ./...
cd .. && bash infra/scripts/guards.sh | tail -1      # guards: 11 of 11 passed (task 47's count)
```

The live half, a real installation token fetching one lane on the real environment while its
owner is signed out and a push with it refused by our code, is **deferred to task 57**: it needs
the GitHub App installed and an Azure DevOps grant, both owner actions (OD-12).

#### What this task does not claim

- **Not that the host enforces read-only as we read it.** That an installation token narrowed by
  `repositories` and `permissions` cannot push is GitHub's documented behaviour as remembered, and
  an Azure DevOps Read grant's scope is the org admin's setting; both are **UNVERIFIED** until
  task 57's live push is refused by the host as well as by us.
- **Not revocation within seconds.** A token already minted lives about an hour (GitHub's
  installation-token life, from knowledge); a failed proof stops the *next* mint. A member whose
  repository access is removed keeps fetch-only reach into her own clones until the next sign-in
  or the proof TTL, which is the design's stated bound (`05-design.md:1061-1062`).
- **Not fetching for solo projects.** A solo cloud project keeps phase 3's own path (the person
  signed in). OD-22 is per team project.
- **Not the Orchestrator reading `origin` itself.** An Orchestrator has no machine disk (task 15);
  its view of `origin` is what the owner's machine last fetched, through task 22's read verbs.
- **Not push or PR from the cloud.** Both stay human acts with the lane owner's own token, per act,
  in memory only (phase 3 task 13; global rule 8).

#### Gaps found in the skeleton

1. **Where the timer runs.** The skeleton says the credential is "used only by `machined` runners
   (phase 3 task 12's slow-timer fetch…)". `machined` serves routes and holds only the token the
   broker presents (phase 8 `task-20.md:24`); it calls nobody. So the timer runs **in the broker**,
   and `machined` runs the one git call. The fetch still executes on the machine, as the scope
   intends; only the loop moves.
2. **The audit sink.** "Every fetch audited to the project" has no Hub route in task 01's table;
   this part uses the broker's own audit line to Log Analytics. If the owner wants fetches in the
   Hub's chain, it is a new Hub route and task 45's stream carries it.
3. **Per-project Azure DevOps identities** mean one user-assigned identity per team project in
   Terraform, plus a manual Azure DevOps grant each; fine for TM1's one repository, not for many.
   A provisioning service is not planned anywhere.
4. **The sign-in hook.** Hub task 33 owns the desktop's sign-in completion; whether it exposes an
   after-sign-in hook is **UNVERIFIED**. If not, the proof runs on the control plane's first team
   read after a token refresh.
5. **Leavers.** Task 44 should delete a leaver's proofs (`DELETE` of the annotation entry) so her
   machine stops fetching at once rather than at the TTL; task 44's scope does not say so.
