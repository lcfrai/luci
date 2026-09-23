# Survey — Phase 7, the Deploy connector (product journey: **Deploy**)

Written 2026-09-23. Read-only pass. No code, no plan, no git write, no network probe.
Scope: what `panacea-api` actually exposes, what a Go client must speak to it, how the SSO
chain works, and where the phase-7 brief and the real API disagree.

**Sibling repo present and readable:** `~/Projects/Panacea` — `services/panacea-api` (Go),
`services/panacea-mcp` (TypeScript), `deploy/`. Everything below is read from source unless
explicitly marked unverified.

---

## Verdict, first

**The API is already richer than phase 7 needs. This phase is a typed Go client plus a
polling loop, not new platform work. But four things in the brief are wrong against the
real API, and one of them breaks the product spine.**

| | |
|---|---|
| **Cheaper than expected** | Every capability the brief names — branch environments, versions, rollback, auto-deploy, build and scan visibility — already exists as a route. No new `panacea-api` endpoint is required for the read path. |
| **Wrong — blocker** | **The spec cannot be attached to the release through the API.** `TriggerDeployOptions` has no client-settable metadata field; `DeployedBy` and `Trigger` are `json:"-"`. The provenance thread must be held locally. See §6. |
| **Wrong — high** | **"Access control through EntraID groups" is not what per-branch access is.** Branch access is a five-value posture, not a group binding. Group-based RBAC exists only at *project* level. See §4. |
| **Wrong — high** | **"Branch environments with arbitrary names" is false.** `sanitizeBranchEnv` truncates to 20 characters, so two long branches silently share one namespace. See §3.2. |
| **Wrong — high** | **No native OIDC client exists.** Both Keycloak clients are confidential with a web redirect URI. A desktop PKCE login is impossible without a platform-side change we do not own. See §5.3. |
| **Wrong — low, but it is in live code** | `desktop/src/components/AppShell.tsx:59` labels the `deploy` lens `phase={5}`. Under the corrected order (`6bcc9af`) Deploy is **phase 7**. `docs/PROGRAM.md:57` agrees it is 7. One-token fix, owned by Phase 2. |
| **Reuse claim needs correcting** | `CLAUDE.md` names `services/panacea-mcp` as reuse. It is **TypeScript**, 645 lines, `src/index.ts`. It is not a Go library and cannot be imported. It is an excellent *reference implementation* of the auth and error-shaping behaviour, and nothing more. |

---

## 1. What this phase is, in product terms

A branch in the cockpit gets a **Ship** action. Pressing it starts a real Tekton pipeline on
Panacea, and the Deploy lens becomes the honest account of what happened: which commit, which
image, which environment, whether the build passed, whether the security scans passed, whether
the pods are actually serving, and — the part that matters for the spine — **which brief this
release satisfies**.

It is also the last rung of the provenance thread. The spec written in Discover, carried
through Develop, reviewed in Merge, must appear *attached to the release* in Deploy. That is
the claim `CLAUDE.md` makes about the product ("the spec written in Define is visibly
referenced in Design, carried into Develop, and attached to the release in Deploy"), and §6
explains why the API cannot hold up its end of it.

Everything is over HTTPS to an API that already enforces RBAC. We add no permission logic of
our own — the same discipline `panacea-mcp`'s README states for itself: *"You can do exactly
what your token can do — nothing more."*

---

## 2. The routes that matter, verified by reading `routes.go`

`services/panacea-api/routes/routes.go` is 3355 lines and registers several hundred routes on
a stdlib `http.ServeMux` with Go 1.22 method+pattern syntax. The phase-7-relevant subset:

### 2.1 Deploy and release

| Route | Guard | Notes |
|---|---|---|
| `POST /api/v1/projects/{name}/deploy` | developer + `RequireSectionWritable("service")` | routes.go:1286. The Ship button. Body is `TriggerDeployOptions`. Returns **202** with `{project, status, pipelineRun}` — the run name comes back synchronously so a client can poll immediately. Handler: projects.go:3024. |
| `GET /api/v1/projects/{name}/deployments` | membership | routes.go:1024. History. Union of live PipelineRuns and the durable store. |
| `GET /api/v1/projects/{name}/deploys/latest` | membership | routes.go:1025. Handler projects.go:1603. Newest of that same union; 404 only when genuinely empty. Attaches provisioning, live readiness and scan status. **This is the single best polling endpoint for the lens header.** |
| `POST /api/v1/projects/{name}/deployments/{id}/rollback` | developer | routes.go:1026. Handler projects.go:1629. |
| `POST /api/v1/projects/{name}/promote` | developer | routes.go:1027. |
| `POST /api/v1/projects/{name}/environments/{env}/redeploy` | developer | routes.go:1028. |
| `GET /api/v1/projects/{name}/pipelines/{run}` | membership | routes.go:1290. Status of the run name returned by the deploy call. |
| `POST /api/v1/projects/{name}/pipelines/{run}/cancel` | developer | routes.go:1291. |

### 2.2 Environments and branches

| Route | Guard | Notes |
|---|---|---|
| `GET /api/v1/projects/{name}/environments` | membership | routes.go:1126. Handler projects.go:2079. Resolves **real** environments from live namespaces — not a preset dev/stage/prod triple. Degrades to production-only with no K8s client. |
| `GET /api/v1/projects/{name}/branches` | membership | routes.go:1170. Handler projects.go:2246. **Careful: this lists remote *git* branches** (`ListRemoteBranches`), not deployed environments. Two different lists; the lens needs both and must not conflate them. |
| `POST …/branches/{branch}/archive` · `/unarchive` · `/set-primary` · `/pause` · `/resume` | developer | routes.go:1299–1304. |
| `DELETE …/branches/{branch}` | **owner** | routes.go:1302. |
| `GET` / `PUT …/branches/{branch}/auto-deploy` | **owner** | routes.go:1308–1309. Auto-deploy is an owner-level control, not developer. Worth knowing before the UI offers it to everyone. |
| `PUT` / `DELETE …/branches/{branch}/url-prefix` | **owner** | routes.go:1313–1314. |

### 2.3 Build and scan visibility

| Route | Guard | Notes |
|---|---|---|
| `GET /api/v1/projects/{name}/security-status` | membership | routes.go:1043. Returns `models.SecurityStatus` (security.go:205). Accepts `?deployId=` to scope to one deploy. |
| `GET /api/v1/projects/{name}/security-status/{scan}` | membership | routes.go:1044. Per-scanner detail. |
| `GET /api/v1/projects/{name}/runtime-security` | membership | routes.go:1066. |
| `GET /api/v1/projects/{name}/logs` | membership | routes.go:1091. |
| `GET /api/v1/projects/{name}/logs/stream` | membership | routes.go:1109. **SSE**, live application-log follow. |
| `GET …/environments/{env}/readiness/stream` | membership | routes.go:2286. **SSE**, live ready-pod count. |
| `GET …/environments/{env}/pods` · `/events` · `/diagnose` · `/quota` | membership | routes.go:1148–1155. The "why is it broken" drill-in. |

Two SSE streams exist and no WebSocket. That matters for the cockpit: `CLAUDE.md`'s
frontend rule says many concurrent live processes share **one global heartbeat clock**, never
a spinner per row. Two SSE connections per visible environment would violate the spirit of
that. Polling `deploys/latest` on the existing heartbeat is the cheaper and more consistent
default; SSE is an opt-in for the one environment the user has drilled into.

### 2.4 Access

| Route | Guard | Notes |
|---|---|---|
| `GET /api/v1/projects/{name}/access` | membership | routes.go:1407. Lists Keycloak group→role bindings. |
| `POST /api/v1/projects/{name}/access` | **owner** | routes.go:1408. Body `{group, role}`. |
| `DELETE /api/v1/projects/{name}/access/{group}` | **owner** | routes.go:1409. |
| `GET` / `PUT …/branches/{branch}/access` | membership / developer | routes.go:1430–1431. **Not a group binding** — see §4. |

---

## 3. Wire types a Go client must mirror

### 3.1 `TriggerDeployOptions` — models.go:2342

```go
Branch   string            `json:"branch"`
Commit   string            `json:"commit"`
BuildEnv map[string]string `json:"-"`      // handler-populated from Vault
NoCache  *bool             `json:"noCache,omitempty"`
// eleven scan-mode overrides, all omitempty, all "off|warn|enforce"
SecretScanMode, DASTMode, SASTMode, ChecksMode,
IacScanMode, ScaScanMode, LicenseScanMode, ModelScanMode,
NucleiMode, SchemathesisMode string
DeployedBy string `json:"-"`   // handler sets from SSO caller
Trigger    string `json:"-"`   // handler sets: manual|redeploy|promote|rollback
```

Two things to read off this. **`NoCache` is a pointer on purpose** — absent means "use the
default", and a Go client that sends `false` is making a different statement from one that
omits it. Copy the pointer. And **there is no metadata, annotation or free-text field**. That
is §6.

### 3.2 Branch → environment, and the collision

`sanitizeBranchEnv` (projects.go:3896) is the whole of the naming rule:

```go
env := strings.ToLower(branch)
env = strings.ReplaceAll(env, "/", "-")
env = strings.ReplaceAll(env, "_", "-")
if len(env) > 20 { env = env[:20] }
env = strings.TrimRight(env, "-")
if env == "" { return "dev" }
```

**Twenty characters, then truncate.** So:

```
feat/phase-7-deploy-connector-a  ->  feat-phase-7-deploy  (20 chars, trailing "-" trimmed)
feat/phase-7-deploy-connector-b  ->  feat-phase-7-deploy  (identical)
```

Two branches, one namespace. The second deploy overwrites the first with no error and no
warning — this is exactly the *silent-override failure class* the Panacea programme has
already paid for repeatedly. The brief's phrase "branch environments with **arbitrary
names**" is not what the platform does, and a cockpit that lets a user name a branch freely
and then shows them one environment for two branches is worse than one that says the rule out
loud.

`mapBranchToEnvWithProd` (projects.go:3920) sits above it: with a configured production
branch, only that branch maps to `prod` and `main`/`master` do **not**; with no configured
production branch, `main`/`master` → `prod`. The comment notes this must stay byte-identical
to `clients.environmentForBranch` and to the webhook receiver's `TargetEnvironment`. A fourth
implementation in our Go client would be a fourth thing to drift. **Derive the env name by
asking `GET …/environments`, do not re-implement the mapping.**

### 3.3 `Deployment` — models.go:665

`ID`, `ProjectName`, `Revision`, `Status`, `Image`, `Version`, `Commit`, `Author`, `Branch`,
`Environment`, `Duration`, `FailedStep`, `ErrorMessage`, `BuildLogs`, `CreatedAt`, plus:

- `Version` — the human-facing release identity: image tag when known, else commit short-SHA.
  Derived, never fabricated; **empty when neither is recorded**. The UI must render an empty
  version as "unknown", not as blank-looking-like-zero.
- `RollbackWarmth` — `"warm"` (sub-second flip) or `"cold"`, with `ColdStartSeconds` advisory.
  Set **only** on the object `Rollback` returns, empty on history rows. This is a genuinely
  good affordance: the lens can tell the user "instant" vs "~20s" *before* they commit.
- `LiveReadyReplicas *int` — and the pointer is the whole point. `nil` = not measured,
  `0` = measured and down, `>0` = serving. The model's own comment says the UI "must say
  nothing rather than imply health" on nil. A Go client that flattens this to `int` destroys
  the distinction and the lens will lie.
- Only the **newest** deploy per environment carries `LiveReadyReplicas`. Historical rows must
  not be painted with today's health.

`Status` is the *build pipeline's* outcome. A crash-looping app reports `Succeeded` while its
URL is down. Build status and serving status are two different facts and the lens must show
both, or it will show a green tick over a dead site.

### 3.4 `SecurityStatus` — security.go:205

`Overall` is one of `FAIL | INCOMPLETE | WARN | PASS | SKIPPED`, with `Blocking[]` and
`Incomplete[]` both naming *which* scans and *why*. The `Incomplete` list exists specifically
so a caller can distinguish "the scanner crashed" from "the scanner was never enabled" —
which is the same honesty requirement the Panacea owner has already set: every scan works,
off means skip, never fabricate. `DetailsRetained *bool` and `DetailsSource`
(`live|snapshot|mixed|unverified|none`) tell you whether findings are attributable to *this*
deploy. Render `unverified` as unverified.

---

## 4. "EntraID groups" — the brief and the API disagree

There are **two different access systems** and the brief conflates them.

**Project-level RBAC is group-based** and is what `authedProject` / `authedProjectDeveloper` /
`authedProjectOwner` enforce. Bindings are Keycloak groups (Entra-federated), managed at
routes.go:1407–1409, handler `handlers/access.go` delegating straight to
`Keycloak.{List,Add,Remove}ProjectAccess`. Roles are viewer / developer / owner. The
middleware wrappers are at routes.go:3226–3355 and the chain is always
`ApiKey → Auth → [RBAC] → Audit → Handler`.

**Branch-environment access is NOT group-based.** `handlers/access_mode.go:25–31` defines
exactly five values:

```
panacea-sso   — platform SSO + the project's own app-user allowlist
kpmg-sso      — platform SSO; any authenticated KPMG identity (SAFE DEFAULT)
password      — one shared Basic-Auth password
bypass-token  — SSO-gated, but a valid bypass token skips the redirect
none          — public
```

`canonicalAccessMode` never maps an unknown mode to `none` — an unrecognised value falls back
to `kpmg-sso`. Fail-closed, deliberately. There is also a documented intentional divergence in
`clients.inheritCanonicalAccessMode`: an inherited `password` base downgrades to `kpmg-sso`,
because a branch does not inherit the password secret and propagating it verbatim would leave
the branch **public**. The comment says "Do not 'resync' that away." A Go client that models
access mode as a free string and echoes it back is one refactor away from re-opening that.

**What this means for phase 7.** "Give this branch environment to these Entra groups" is not
an operation the API offers. The closest truthful UI is: a project-level group/role grid
(three roles, real groups) *plus* a per-branch posture selector (five values). Presenting one
merged "who can see this" control would be inventing a capability.

One related trap, from `middleware/groups_header.go`: a single `X-Auth-Request-Groups` value
`"Finance,Restricted"` is read as **two** groups, and nothing on the wire distinguishes that
from one group whose display name contains a comma. Entra and Keycloak both permit commas in
group display names. If our UI ever renders or round-trips a group name, it must not join a
list with `","`.

---

## 5. The SSO chain, and what the desktop app can actually do

### 5.1 The chain as deployed

`oauth2-proxy` → Keycloak realm `panacea` → Entra ID, exactly as `CLAUDE.md` describes.
Verified in `deploy/portal/setup-keycloak.sh`: realm `panacea`, a `backstage` confidential
client (line 139), a `panacea-oauth2-proxy` confidential client (line 320), and a gated
Entra OIDC identity-provider broker whose redirect is
`https://auth.<DOMAIN>/realms/panacea/broker/entra/endpoint`. The `groups` client scope is
attached to the proxy client so SSO tokens carry group membership.

`GET /auth/verify` (routes.go:624) is the ForwardAuth endpoint tenant apps use. It is not a
login endpoint for us.

### 5.2 How `panacea-api` decides who you are — `middleware/auth.go`

Five labelled identity paths, and the labels are load-bearing because they are written into
the audit trail:

- `verified-identity-token` — `X-Backstage-Identity` JWT, signature-verified against JWKS.
- `unverified-identity-token` — the same JWT **decoded without verifying the signature**.
- `backend-secret-header` — raw `X-Auth-Request-*` headers trusted on a `BACKEND_SECRET`
  match. Not signature-verified.
- `service-token` — `BACKEND_SECRET` with no identity at all.
- `legacy-project-key`.

Two things the source says plainly and that we should quote rather than soften. First,
`IdentityAssertedFromForgeableHeader` exists because the header path is *"an UNVERIFIED claim
about the END USER"* — a caller holding the shared secret can name `platform-admins` in the
group header and be minted a SuperAdmin. Second, a long comment block in `middleware/jwks.go`
states that when `BACKSTAGE_JWKS_URL` is **empty**, `resolve` returns an unverified decode **in
every environment**, and describes this as *"a larger fail-open than the bounded one F2/H-3
closed"*, raised as a LEAD? item on 2026-08-07. Neither path is one a desktop client would use
— we would never hold `BACKEND_SECRET` — but both are context the owner should have before
anyone proposes "just forward the headers".

### 5.3 There is no native OIDC client — this is the blocker

Both Keycloak clients are:

```
"publicClient": false,
"standardFlowEnabled": true,
"directAccessGrantsEnabled": false,
"redirectUris": ["https://<DOMAIN>/oauth2/callback"]
```

Confidential, no direct grant, and a **web** redirect URI. There is no public client, no PKCE
client, and no loopback (`http://127.0.0.1:<port>/callback`) redirect registered. A native
desktop OIDC login against realm `panacea` is therefore **not possible today**, and creating
the client is a change to the Panacea platform's Keycloak realm — another repo, another owner,
another change window. It is not in PanaceaApp's gift.

### 5.4 What the desktop app should do instead — `pnc_` tokens

`middleware/api_key.go` implements a first-class personal-token path:

- Prefix `pnc_`, sent as `Authorization: Bearer pnc_…`.
- `ApiKey` runs **before** `Auth` in every chain, and on a match short-circuits the SSO header
  parser entirely (auth.go: *"If an earlier middleware already resolved an authenticated user,
  skip the SSO header parser"*).
- The token carries a frozen group snapshot which, when a `LiveGroupResolver` is wired, is
  **intersected with the owner's current Keycloak membership on every request**, cached 60s,
  failing **closed**. Offboarding therefore takes effect on tokens already issued, within a
  minute. `intersectGroups` can only ever narrow.
- An unscoped token owned by a `platform-admins` member acts as SuperAdmin; a normal user's
  token auto-scopes to their own projects.

This is the mechanism `panacea-mcp` uses, and it is the right one for us. **The user pastes a
token once; the desktop app stores it in the OS keychain and sends it as a bearer.**

One constraint to be honest about: the app **cannot mint its own token**.
`POST /api/v1/me/api-keys` (routes.go:3050) is wrapped in `authedUnscoped` plus
`RequirePortalGrant()`, so it needs an existing browser SSO session — and `authedUnscoped`
explicitly rejects a scoped token, so a token cannot bootstrap another token. The onboarding
flow is therefore: open the portal in the system browser → Account → API keys → copy → paste.
That is one extra step, it is honest, and it works today with zero platform change.

`middleware.RequirePortalGrant` (rbac.go:374) also means a user awaiting team approval gets a
specific 403 — *"no access yet: your account is waiting for team approval"*. Surface that
string; it is actionable and a generic "forbidden" is not.

### 5.5 Unverified: the API base URL

`panacea-mcp`'s README and its code default consistently to `https://api.dev.panacea.kpmg`
(README lines 47, 60, 75, 86, 115, 127; `src/index.ts:32`). My own working notes from earlier
dev-estate work record the dev API as `https://dev.panacea.kpmg/api` — explicitly *not* the
`api.dev` form. **These contradict, and I did not probe the network to resolve it.** Treat the
base URL as configurable and unverified; a single live `GET /api/v1/version` (routes.go:630,
unauthenticated) settles it in one call. Do not hard-code either form.

Also from the MCP code: `PANACEA_TLS_INSECURE=1` exists because of KPMG corporate TLS
interception. A Go client will hit the same wall and needs a deliberate, opt-in, loudly-logged
escape hatch — or better, corporate-root-CA loading, which is the root-cause fix rather than
the workaround.

---

## 6. The provenance blocker — read this one carefully

`CLAUDE.md`: *"the spec written in Define is visibly referenced in Design, carried into
Develop, and attached to the release in Deploy. Same artifact, one thread, shown throughout.
Any feature that breaks that thread is wrong regardless of how good it is in isolation."*

**`panacea-api` accepts no client-supplied metadata on a deploy.** `TriggerDeployOptions`
(models.go:2342) has `Branch`, `Commit`, `NoCache` and eleven scan modes. `DeployedBy` and
`Trigger` are `json:"-"` — the handler stamps them from the SSO caller (projects.go:3049–3051)
and a JSON client cannot set them. Those two land on the PipelineRun as
`panacea.io/deployed-by` and `panacea.io/trigger` annotations, and there is no third,
general-purpose annotation the client controls.

So there is no way to push "this release satisfies brief `<ref>`" into Panacea. Three options,
ranked:

1. **Hold the link locally** (recommended). Our control plane stores
   `(project, pipelineRun) -> provenance.Ref` at the moment we call `POST …/deploy`, since the
   202 response hands back the run name synchronously. `pipelineRun` then joins to the
   `Deployment.ID` we poll. The thread stays unbroken in our UI. Cost: the link is ours, not
   the platform's, so it is invisible in the Panacea portal and lost if our state directory
   is lost. It must be stated as a known limit, not quietly shipped as if the platform knew.
2. **Encode the ref in `Commit`.** Do not. `Commit` is a git ref that the pipeline resolves.
3. **Ask for an API change** — a `metadata map[string]string` on `TriggerDeployOptions`,
   annotated onto the PipelineRun. Small, additive, backward-compatible. But it is a
   cross-repo change with its own owner and review, and it must not block phase 7.

This is the single most important thing for the orchestrator to decide, because option 1 is a
design commitment that phase 3 (worktrees, where the branch identity lives) and phase 4
(Discover, which mints the ref) both have to agree with.

---

## 7. Where the reuse map is right, and where it is wrong

`CLAUDE.md` says reuse `services/panacea-api/routes/routes.go` and `services/panacea-mcp`.

- **`routes.go` — right, as a contract.** It is the authoritative list of what exists. It is
  not code we import; it is a different module (`github.com/panacea-platform/panacea/...`) in
  a different repo. Our client re-declares the handful of DTOs it needs. Copying the full
  `models` package would be copying thousands of lines to use twenty.
- **`panacea-mcp` — wrong as stated.** It is TypeScript (`src/index.ts`, 645 lines,
  `package.json`, `build.mjs`). There is no Go to reuse. What it *does* give us, and it is
  genuinely valuable, is a proven behavioural spec: bearer-token auth, the `pnc_` prefix
  warning, TLS-insecure opt-in, and a set of hand-written error hints (401 → "mint a fresh
  `pnc_` token"; network/TLS → "set `PANACEA_TLS_INSECURE=1`"). Port the *error shaping*, not
  the code.

Our own substrate, read from this repo:

- `control-plane/go.mod` is `go 1.23` with `coder/websocket` and `k8s.io/client-go`.
  `golang.org/x/oauth2 v0.21.0` is already present as an **indirect** dependency, so an OIDC
  flow later would not need a new direct module — but §5.3 says that flow has nowhere to go
  yet.
- `control-plane/internal/httpx/` holds `origin.go` (origin policy) and `problem.go`. It is
  server-side; there is no outbound HTTP client helper yet. Phase 7 introduces the first one.
- `control-plane/internal/api/api.go:126–172` uses a **route registration table** rather than
  bare `mux.HandleFunc`, with a recorded comment that nine routes once compiled, served, and
  were never recorded. Any route phase 7 adds must go through `routes.Handle`.
- `control-plane/internal/api/api.go:274` mounts `/api/v1/` as a `StripPrefix` alias of `/v1/`.
  Phase 7's own routes get both spellings for free.

---

## 8. Risks, ranked

1. **Provenance cannot round-trip through the platform** (§6). Blocker for the spine claim;
   needs an orchestrator decision before any plan is written.
2. **20-character branch-env truncation silently collides** (§3.2). Two branches, one
   namespace, no error. Needs a product answer: refuse long branch names, warn, or show the
   collision honestly.
3. **No native OIDC client** (§5.3). Caps phase 7 at paste-a-token onboarding unless the
   platform team adds a public client. Not our repo.
4. **"EntraID groups" per branch does not exist** (§4). The brief promises a control the API
   does not offer; the UI must be redrawn around project-groups + branch-posture.
5. **Build-green vs serving-dead** (§3.3). `Status: Succeeded` with `LiveReadyReplicas: 0` is
   a normal, common state. A lens that shows only build status will report success over a
   dead site.
6. **Owner-only auto-deploy** (§2.2). `PUT …/auto-deploy` is `authedProjectOwner`. A developer
   pressing it gets a 403. Design for that, do not discover it live.
7. **API base URL unresolved** (§5.5). Two documented forms contradict. One unauthenticated
   `GET /api/v1/version` settles it; I did not run it.

---

## 9. What I did not verify

- **No network call was made.** No endpoint was probed, no token was used, no response body
  was observed. Every shape above is read from Go source and, for the base URL, from a README.
- **The dev/prod base URL** (§5.5) is an unresolved contradiction between two written sources.
- **Whether `LiveGroupResolver` is actually wired** in the dev and prod deployments. The
  intersection behaviour in §5.4 is real in code but is conditional on `deps.Keycloak`
  satisfying the interface; `liveGroupResolverFrom` (routes.go:3207) returns nil when it does
  not, which silently restores frozen-snapshot behaviour. I read the fallback, not the live
  wiring.
- **Whether the phase-2 `deploy` lens placeholder is still `phase={5}`** by the time this is
  read. Phase 2 is live in this tree and another agent is editing `desktop/`. It was
  `AppShell.tsx:59` when I read it.
- I did not read the Tekton pipeline definitions, so how a scan mode becomes a task parameter
  is inferred from `TriggerDeployOptions` comments rather than observed.
