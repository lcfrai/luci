### Task 04: Panacea acts and governance calls

**Half:** 7a (the calls exist before 7b wires Ship to them). **Hours:** 3. **Depends on:** 02, 03.

`deploy.Actor` and `deploy.Governor` on `*panacea.Client`. Every method here is a **write**, so
none is retried (task 02's `do`) and every one is tested for exactly one request.

**Files:**
- `control-plane/internal/deploy/panacea/wire.go` (modified) — the act DTOs below
- `control-plane/internal/deploy/panacea/acts.go` (new) — `Ship`, `Cancel`, `Rollback`
- `control-plane/internal/deploy/panacea/governance.go` (new) — the nine `Governor` methods
- `control-plane/internal/deploy/panacea/panaceatest/server.go` (modified) — answers the write routes
- `control-plane/internal/deploy/panacea/testdata/*.json` (new files listed below)
- `control-plane/internal/deploy/panacea/acts_test.go`, `governance_test.go` (new)
- `control-plane/internal/deploy/deploytest/conformance.go` (modified) — the rows below

#### Wire DTOs — complete (`wire.go`, write half)

```go
// shipRequestDTO is the subset of models.TriggerDeployOptions phase 7 sends
// (models/models.go:2342-2383). It has NO scan-mode field on purpose (brief B2): turning a
// scanner off from a desktop is a security-posture change, and Panacea's rule is "off means
// skip, never fabricate". NoCache is a pointer exactly as Panacea's is (:2349-2352).
// TriggerDeploy decodes leniently (handlers/projects.go:2967-2968), so an unknown field would be
// silently dropped — which is why no speculative "metadata" field is sent (seam S7-N2).
type shipRequestDTO struct {
	Branch  string `json:"branch"`
	Commit  string `json:"commit"`
	NoCache *bool  `json:"noCache,omitempty"`
}

// shipResponseDTO mirrors TriggerDeploy's 202 data (handlers/projects.go:2998-3005).
type shipResponseDTO struct {
	Project     string `json:"project"`
	Status      string `json:"status"`
	PipelineRun string `json:"pipelineRun"`
}

// cancelResponseDTO mirrors CancelDeploy's data (handlers/projects.go:3670-3687).
type cancelResponseDTO struct {
	Status  string `json:"status"` // "cancelled" | "already_completed"
	Message string `json:"message"`
}

// rollbackResponseDTO is Rollback's data (handlers/projects.go:1636-1639): the Deployment
// RollbackDeployment returns, enriched with image/commit/branch/environment (:1619-1623).
// Its ID is the deployment rolled back TO (clients/kubernetes.go:5982, :6030).
type rollbackResponseDTO = deploymentDTO

// autoDeployDTO mirrors GetBranchAutoDeploy's data (handlers/projects.go:4136-4142).
type autoDeployDTO struct {
	Branch             string `json:"branch"`
	Enabled            *bool  `json:"enabled"` // null = no explicit label
	ProjectDefault     bool   `json:"projectDefault"`
	Effective          bool   `json:"effective"`
	IsProductionBranch bool   `json:"isProductionBranch"`
}

// setAutoDeployDTO is SetBranchAutoDeploy's body (handlers/projects.go:4159-4167).
type setAutoDeployDTO struct {
	Enabled *bool `json:"enabled"` // required; always non-nil when sent
}

// postureDTO mirrors GetBranchAccessMode's data (handlers/project_security.go:668-677).
type postureDTO struct {
	Branch           string   `json:"branch"`
	Env              string   `json:"env"`
	AccessMode       string   `json:"accessMode"`
	IPWhitelist      []string `json:"ipWhitelist"`
	PasswordSet      bool     `json:"passwordSet"`
	BypassTokenCount int      `json:"bypassTokenCount"`
}

// setPostureDTO mirrors branchAccessModeRequest (handlers/project_security.go:681-691).
// BypassTokens (caller-supplied raw tokens) is deliberately NOT declared: the cockpit never
// supplies a token, it only asks Panacea to mint one.
type setPostureDTO struct {
	AccessMode         string `json:"accessMode"`
	Password           string `json:"password,omitempty"`
	RotateBypassTokens bool   `json:"rotateBypassTokens,omitempty"`
}

// setPostureResponseDTO mirrors SetBranchAccessMode's data (handlers/project_security.go:824-845);
// the envelope's message is the outcome sentence, including "SAVED … BUT NOT YET ENFORCED".
type setPostureResponseDTO struct {
	Branch       string   `json:"branch"`
	Env          string   `json:"env"`
	AccessMode   string   `json:"accessMode"`
	Enforced     bool     `json:"enforced"`
	BypassTokens []string `json:"bypassTokens"` // present only when minted; returned ONCE (:830-833)
}

// accessBindingDTO mirrors models.AccessBinding (models/models.go:2004-2015).
type accessBindingDTO struct {
	Group   string `json:"group"`
	Role    string `json:"role"`
	Grantee string `json:"grantee"`
}

// grantDTO mirrors models.AddAccessRequest (models/models.go:2016-2019). "group" carries a
// username or email: AddProjectAccess resolves it to one Keycloak user (clients/keycloak.go:180-190).
type grantDTO struct {
	Group string `json:"group"`
	Role  string `json:"role"`
}

// urlPrefixDTO is setBranchURLPrefixRequest (handlers/projects.go:4263-4265).
type urlPrefixDTO struct {
	Prefix string `json:"prefix"`
}

// urlPrefixResponseDTO mirrors SetBranchURLPrefix's data (handlers/projects.go:4364-4371).
type urlPrefixResponseDTO struct {
	Branch string `json:"branch"`
	Prefix string `json:"prefix"`
	Host   string `json:"host"`
}
```

#### The routes (`acts.go`, `governance.go`)

`P` as in task 03; `B` is `P + "/branches/" + url.PathEscape(branch)`.

| Method | Route (`routes.go`) | Expect | Mapping |
|---|---|---|---|
| `Ship` | `POST P/deploy` (1286) | **202** | refuses before sending when `!deploy.ValidCommit(req.Commit)` or `req.Branch == ""` (`KindRejected`, no request); `Run = PipelineRun`; an empty `pipelineRun` on a 202 is `KindUnavailable` (*"Panacea accepted the deploy but did not name the run"*) |
| `Cancel` | `POST P/pipelines/{run}/cancel` (1291) | 200 | `already_completed` → `AlreadyFinished` |
| `Rollback` | `POST P/deployments/{id}/rollback` (1026) | 200 | `SourceDeploymentID = data.ID`; if `data.ID != id` → `KindUnavailable` and a log line, because 7P-4's premise no longer holds and the Hub confirm must not guess |
| `AutoDeploy` | `GET B/auto-deploy` (1308) | 200 | — |
| `SetAutoDeploy` | `PUT B/auto-deploy` (1309) | 200 | response is `data` like the GET — **unverified**, read the handler tail at `projects.go:4168`ff before writing the decoder |
| `Posture` | `GET B/access` (1430) | 200 | `Mode, ModeKnown = deploy.ParsePostureMode(AccessMode)` |
| `SetPosture` | `PUT B/access` (1431) | 200 | `Password` sent only for `PosturePassword`; `Enforced` and the envelope `message` carried through |
| `Access` | `GET P/access` (1407) | 200 | role word → domain: `viewer`→`RoleViewer`, `editor` or `developer`→`RoleDeveloper`, `owner`→`RoleOwner`, else `RoleUnknown` (`models/project_groups.go:9-12`) |
| `Grant` | `POST P/access` (1408) | **201** (`handlers/access.go:61`) | domain → wire: `RoleDeveloper` → **`"editor"`**; `RoleUnknown` refused before sending |
| `Revoke` | `DELETE P/access/{group}` (1409) | 200 | the grantee path-escaped (an email contains `@`) |
| `SetURLPrefix` | `PUT B/url-prefix` (1313) | 200 | a 409 carries *"url prefix already taken: …"* (`projects.go:4321`) verbatim |
| `ClearURLPrefix` | `DELETE B/url-prefix` (1314) | 200 | clearing a branch with no prefix is success (`projects.go:4380-4382`) |

**Never logged, anywhere in this file:** `PostureChange.Password`, any `bypassTokens` value, the
token. `do` already omits bodies; these two methods also pass `logBody: false` explicitly and a test
asserts the captured log.

#### Testdata added

`ship-202.json`, `ship-500-timeout.json` (`{"error":"failed to start deployment: context deadline
exceeded"}`, the shape of `projects.go:2991`), `ship-402-budget.json` (status 402, `{"error":"new
deploys are paused: this project has reached its monthly hosting budget; new deploys are paused.
Raise it on the project Cost tab."}` — `projects.go:2856` joined to `budget_resolve.go:240`),
`ship-402-risk.json` (status 402, the frame of `projects.go:2916-2918` — *"production deploy blocked
by the project's risk assessment: <reason>. Resolve the failing risk requirements on the project
Risk page, then re-deploy."* — with a stand-in reason; the real reason is the assessment's own text
or the fallback built at `handlers/risk_capability_apply.go:536-539`, whose label was not read),
`cancel-cancelled.json`, `cancel-already.json`,
`rollback-warm.json`, `rollback-cold.json`, `rollback-409-pruned.json` (the sentence of
`projects.go:1603-1604`), `rollback-400-noimage.json` (`:1575`), `rollback-404.json` (`:1570`),
`autodeploy.json`, `autodeploy-409-noenv.json` (`:4133`), `posture.json`,
`posture-set-not-enforced.json` (the message of `project_security.go:836-839`, `enforced: false`),
`posture-set-minted.json`, `posture-409-locked.json` (`:733-735`), `access.json` (one person grant,
one legacy group binding with empty `grantee`), `urlprefix.json`, `urlprefix-409.json`.

#### Conformance rows added

| Row | Assertion | Mutant it kills |
|---|---|---|
| `RollbackSourceIsArgument` | `Rollback(p, "d1").SourceDeploymentID == "d1"` for warm and cold | a connector that reports a new id, which task 10 would send to the Hub and 409 |
| `CancelFinishedIsNotAnError` | cancelling a succeeded run → `AlreadyFinished`, `err == nil` | an error toast for an undo that raced the build's end |
| `GrantDeveloperRoundTrips` | `Grant(RoleDeveloper)` then `Access` → that grantee with `RoleDeveloper` | sending `"developer"` where Panacea's writer spells `"editor"` — **unverified whether it would be refused**, so the connector never finds out |

#### Tests to write first

| Test | Assertion | Mutant it kills |
|---|---|---|
| `TestShipSendsExactlyBranchCommit` | the recorded body is `{"branch":"feat/x","commit":"<40 hex>"}` — no `noCache` key when nil, no scan field, nothing else | a scan-mode override leaking in; `noCache:false` sent where Panacea distinguishes absent |
| `TestShipRefusesSymbolicCommit` | `Commit: "main"` → `KindRejected`, the server saw **zero** requests | shipping whatever the branch holds at clone time |
| `TestShipOneAttemptOn503` | a 503 → `KindUnavailable`, exactly one request | a retried deploy |
| `TestShip500IsUnavailableWithoutSentence` | `ship-500-timeout.json` → `KindUnavailable`, `Message == ""` | rendering Panacea's internal error text; the caller (task 09) writes its own sentence |
| `TestShip402IsARefusalWithSentence` | `ship-402-budget.json` and `ship-402-risk.json` → each `KindConflict`, `Status == 402`, `Message` equal to the file's sentence, exactly one request | a paused budget or a risk block read as `KindUnavailable` with `Message ""`: the lens says Panacea is offline, and task 09's abandon branches never match, so the reservation lingers 15 minutes and the reason is lost (review I2) |
| `TestRollbackCarriesWarmth` | warm → `WarmthWarm`, 0 s; cold → `WarmthCold`, the seconds | dropping the only hint of how long the flip takes |
| `TestRollbackRefusalsVerbatim` | the 409, 400 and 404 bodies → `KindConflict`, `KindRejected`, `KindNotFound`, each `Message` equal to the file's sentence | a generic "rollback failed", which hides *"redeploy from source instead"* |
| `TestPostureNotEnforced` | `posture-set-not-enforced.json` → `Enforced == false`, `Message` is the full sentence | reporting a saved-but-unapplied posture as done |
| `TestPostureMintedTokenReturnedNotLogged` | `posture-set-minted.json` → `BypassTokens` has the token; the captured log lacks it | a token in the control plane's log |
| `TestPasswordOnlyForPasswordMode` | `SetPosture(Mode: kpmg-sso, Password: "x")` sends no `password` key | sending a stale password field that Panacea might persist |
| `TestGrantEditorSpelling` | `Grant(RoleDeveloper)` body has `"role":"editor"` | the domain word leaking onto the wire |
| `TestGrantUnknownUserIsRejected` | a 500 `{"error":"failed to add access binding: resolve grantee \"x\": … / no Keycloak user found for grantee \"x\""}` → `KindRejected`, `Message` *"No Panacea user matches that name or email."*; any other 500 → `KindUnavailable`, `Message ""`. The test quotes `handlers/access.go:57` and `clients/keycloak.go:182`, `:189` | a typo shown as an outage — or, the opposite mutant, any 500 body shown to the user (task 11 explains the one exception) |
| `TestRevokeEscapesEmail` | `Revoke("a.b@kpmg.com.au")` requests `/access/a.b%40kpmg.com.au` | an unescaped path segment |
| `TestAccessLegacyGroup` | the legacy row → `Grantee == ""`, `Group` set | drawing a group name as a person |
| `TestNoWriteIsRetried` | every method of this task against a server answering 503 once, then 200: each saw one request | — the whole file's rule |

#### Steps

- [ ] **Read first:** `handlers/projects.go:4168-4262` (the rest of `SetBranchAutoDeploy`) and
  `:4383-4420` (`ClearBranchURLPrefix`), and record their response envelopes in this part's
  table before writing the decoders. The table above marks the one it did not read.
- [ ] Write the testdata and the tests; they fail to compile.
- [ ] Write the DTOs, `acts.go`, `governance.go`; extend `panaceatest.Server`. Its
  `GET B/access` already answers `env` from task 03's `BranchEnvironment` table; `PostureWord`
  adds the posture fields to the **same** body, and the route fails only for a branch seeded in
  neither table, with `env` omitted when only a posture word is seeded (task 03's
  `panaceatest.Server` paragraph; b02 round 2, R3). Otherwise `UnknownPostureFailsClosed`, which
  seeds only `PostureWord`, would hit the unseeded-branch `t.Errorf` by design.
- [ ] Add the conformance rows; run everything green; commit by pathspec.

#### Acceptance

```
go test ./control-plane/internal/deploy/... -race -count=1
→ ok

grep -nE 'Mode"|ScanMode|dastMode|sastMode' control-plane/internal/deploy/panacea/wire.go
→ only the comment on shipRequestDTO
```

#### What this task does NOT claim

- That a live deploy, rollback or access change works. Nothing here reaches dev Panacea; task 20 is
  the owner-batched live run (OD-12).
- That Panacea accepts `developer` as a role spelling (plan §12). It sends `editor` and never tests
  the other.
- Any Hub interaction. Ship's reserve/confirm is task 09.
