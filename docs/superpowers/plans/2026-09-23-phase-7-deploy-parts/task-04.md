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

// autoDeployDTO mirrors GetBranchAutoDeploy's data (handlers/projects.go:4136-4142) and
// SetBranchAutoDeploy's (:4192-4198): the same five keys in a data envelope. On the PUT,
// enabled is the bool just written (*body.Enabled, :4194), never null (review M6).
type autoDeployDTO struct {
	Branch             string `json:"branch"`
	Enabled            *bool  `json:"enabled"` // GET: null = no explicit label; PUT: always set
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

// setPostureResponseDTO mirrors SetBranchAccessMode's data (handlers/project_security.go:824-845).
// The envelope's message is carried only when enforced is true ("access mode set to …", :835).
// When enforced is false Panacea's sentence embeds the reconcile error's err.Error() (:818-821,
// :837-839), so it is logged at warn and never carried; SetPosture writes notEnforcedSentence.
type setPostureResponseDTO struct {
	Branch       string   `json:"branch"`
	Env          string   `json:"env"`
	AccessMode   string   `json:"accessMode"`
	Enforced     bool     `json:"enforced"`
	BypassTokens []string `json:"bypassTokens"` // present only when minted; returned ONCE (:830-833)
}

// accessBindingDTO mirrors models.AccessBinding (models/models.go:2004-2015) as ListProjectAccess
// fills it (clients/keycloak.go:143-159): a person's row has group == grantee; a legacy row has
// group = the Keycloak group's name and grantee = its panacea-group attribute. The model's
// comment says a legacy grantee is "" (:2011); the code never sends that (review M30).
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
| `SetAutoDeploy` | `PUT B/auto-deploy` (1309) | 200 | response is `data` with the GET's five keys, read from source (`projects.go:4192-4198`, review M6), decoded into `autoDeployDTO`; `enabled` is a bool, the value just written (`:4194`), never null. A 409 *"branch has no environment yet — …"* (`:4178`) is `KindConflict` with the sentence |
| `Posture` | `GET B/access` (1430) | 200 | `Mode, ModeKnown = deploy.ParsePostureMode(AccessMode)` |
| `SetPosture` | `PUT B/access` (1431) | 200 | `Password` sent only for `PosturePassword`; `Enforced` carried through. `Message` is the envelope `message` when `enforced` is true. When it is false, `Message` is **our** sentence, built from the structured `enforced` and the mode and branch this call sent: *"Access mode saved as `<mode>` for `<branch>`, but not yet enforced: the route could not be updated, so traffic still meets the previous mode. Re-apply, and do not treat the new mode as active until it succeeds."* Panacea's own sentence is logged at warn, never carried, because it embeds `reconcileErr`, an `err.Error()` (`project_security.go:818-821`, `:837-839`; review M7) |
| `Access` | `GET P/access` (1407) | 200 | role word → domain: `viewer`→`RoleViewer`, `editor` or `developer`→`RoleDeveloper`, `owner`→`RoleOwner`, else `RoleUnknown` (`models/project_groups.go:9-12`). `Group` and `Grantee` copied verbatim; `Legacy = group != grantee` (review M30) |
| `Grant` | `POST P/access` (1408) | **201** (`handlers/access.go:61`) | domain → wire: `RoleDeveloper` → **`"editor"`**; `RoleUnknown` refused before sending. Single-role on Panacea: `AddProjectAccess` first drops the grantee from the project's other role groups (`clients/keycloak.go:198-214`), so a grant to a current member **is** the role change, one request. That drop is best-effort (a failure is logged at debug, `:207-210`), so the caller re-reads (task 11) |
| `Revoke` | `DELETE P/access/{group}` (1409) | 200 | takes the row's **`Group`**, path-escaped (an email contains `@`). For a person's row that is their identity, and Panacea removes them from **every** role group (`keycloak.go:245-260`); for a legacy row it is the group's name, and Panacea deletes that group (`:262-267`). A legacy row's `Grantee` (an attribute value such as `team-old`) is no group's name, so unless a user happens to have that name Panacea answers 200 having removed nothing (`:268`); and if one does, it removes that user instead. It is never sent (review M30) |
| `SetURLPrefix` | `PUT B/url-prefix` (1313) | 200 | a 409 carries *"url prefix already taken: …"* (`projects.go:4321`) verbatim |
| `ClearURLPrefix` | `DELETE B/url-prefix` (1314) | 200 | clearing a branch with no prefix is success (`projects.go:4380-4382`); the body is `data {branch, routeReverted}` plus a `message` (`:4448-4451`), read from source (b07), and only the status is used |

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
`autodeploy.json`, `autodeploy-409-noenv.json` (`:4133`), `autodeploy-set.json` (the PUT's `data`,
`:4192-4198`, `enabled` a bool), `posture.json`,
`posture-set-not-enforced.json` (the message of `project_security.go:836-839`, `enforced: false`,
branch `feat/fix(ui)` — git allows the `(`, `git check-ref-format --branch` accepts it — and a
stand-in `reconcileErr` that itself holds a `(`), `posture-set-minted.json` (`enforced: true`), `posture-409-locked.json` (`:733-735`), `access.json` (one person grant, its `group` equal to its `grantee`,
one legacy group binding shaped as Panacea's own test pins it, `{"group":"project-legacy-viewer",
"role":"viewer","grantee":"team-old"}`, `clients/keycloak_access_test.go:349-351`; review M30),
`urlprefix.json`, `urlprefix-409.json`.

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
| `TestSetAutoDeployDecodesPut` | `SetAutoDeploy(ctx, p, "feat/x", true)` against `autodeploy-set.json` → the recorded body is `{"enabled":true}`, and the result has `*Explicit == true` (non-nil), `ProjectDefault`, `Effective` and `ProductionBranch` from the file | a decoder written for a different PUT shape (the response was once marked unverified, review M6), or `enabled` read back as "no explicit label" |
| `TestPostureNotEnforced` | `posture-set-not-enforced.json` (its `reconcileErr` a stand-in `dial tcp 10.0.0.9:6443: i/o timeout (retry 3)`, on branch `feat/fix(ui)`) → `Enforced == false`, `Message` equal to our sentence for that mode and branch, containing neither `10.0.0.9` nor `retry`; the captured warn log carries Panacea's sentence; `posture-set-minted.json` (`enforced: true`) keeps Panacea's *"access mode set to …"* | reporting a saved-but-unapplied posture as done; an internal error shown to the user (review M7); a cut at the first `(`, which the branch's own `(` defeats |
| `TestPostureMintedTokenReturnedNotLogged` | `posture-set-minted.json` → `BypassTokens` has the token; the captured log lacks it | a token in the control plane's log |
| `TestPasswordOnlyForPasswordMode` | `SetPosture(Mode: kpmg-sso, Password: "x")` sends no `password` key | sending a stale password field that Panacea might persist |
| `TestGrantEditorSpelling` | `Grant(RoleDeveloper)` body has `"role":"editor"` | the domain word leaking onto the wire |
| `TestGrantUnknownUserIsRejected` | a 500 `{"error":"failed to add access binding: resolve grantee \"x\": … / no Keycloak user found for grantee \"x\""}` → `KindRejected`, `Message` *"No Panacea user matches that name or email."*; any other 500 → `KindUnavailable`, `Message ""`. The test quotes `handlers/access.go:57` and `clients/keycloak.go:182`, `:189` | a typo shown as an outage — or, the opposite mutant, any 500 body shown to the user (plan 7P-12 names this as the single parsed sentence; task 11 explains the route; review M8) |
| `TestRevokeEscapesEmail` | `Revoke("a.b@kpmg.com.au")` requests `/access/a.b%40kpmg.com.au` | an unescaped path segment |
| `TestAccessLegacyGroup` | `access.json`'s legacy row → `Legacy == true`, `Group "project-legacy-viewer"`, `Grantee "team-old"`; the person row → `Legacy == false`, `Group == Grantee` | drawing a group binding as a person; a legacy test keyed on `Grantee == ""`, which Panacea's code never sends (review M30) |
| `TestRevokeLegacyByGroup` | `Revoke` with the legacy row's `Group` requests `/access/project-legacy-viewer`, and the recorded requests hold no `/access/team-old` | revoking a legacy row by its grantee: Panacea answers 200 and removes nothing (`keycloak.go:268`) (review M30) |
| `TestNoWriteIsRetried` | every method of this task against a server answering 503 once, then 200: each saw one request | — the whole file's rule |

#### Steps

- [ ] **Re-read before writing the decoders:** `handlers/projects.go:4192-4198`
  (`SetBranchAutoDeploy`'s response) and `:4448-4451` (`ClearBranchURLPrefix`'s). Both are
  recorded in the table above, read from source at `b02424691` (review M6); if either has moved,
  correct the table first.
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

grep -niE '(secretscan|dast|sast|checks|iacscan|scascan|licensescan|modelscan|nuclei|schemathesis)mode' control-plane/internal/deploy/panacea/wire.go
→ (nothing): no field and no tag carries any of TriggerDeployOptions' ten scan-mode spellings
  (models/models.go:2356-2372 at b33d15acb). Anchored to those ten, not to `Mode"`, which matches
  the posture DTOs' json:"accessMode" (review I1). -i on purpose: encoding/json matches keys
  case-insensitively, so an untagged `DASTMode` field would still reach Panacea as dastMode. The
  shipRequestDTO comment spells "scan-mode", which the pattern does not match.
```

#### What this task does NOT claim

- That a live deploy, rollback or access change works. Nothing here reaches dev Panacea; task 20 is
  the owner-batched live run (OD-12).
- That Panacea accepts `developer` as a role spelling (plan §12). It sends `editor` and never tests
  the other.
- Any Hub interaction. Ship's reserve/confirm is task 09.
