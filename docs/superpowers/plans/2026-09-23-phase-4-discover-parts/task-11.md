### Task 11: `discoverapi` — the brief routes, the relay rule, the route audit

The renderer's only door to the Brief. Every route is on the control plane (loopback), under `/v1`
with the `/api/v1` alias, mounted through task 10's `RouteMounter` so the origin guard, the alias
and phase 2's §10a audit all reach it. The control plane calls the Hub behind these routes as the
signed-in user; the renderer never sees a Hub token (ADR-H1 §5.4). **Kept from the 2026-09-18
plan's task 9** (`:3596-4480`): one package, JSON in and out through `httpx`, no route ever 404s
for an absent brief. **Changed:** every route is new or reshaped (the table below); the
materialise route is gone (C7); errors follow the Hub's relay rule.

**Files:**
- `control-plane/internal/discoverapi/api.go` — `Config`, `Handler`, `New`, `Mount`, `relay` (new)
- `control-plane/internal/discoverapi/briefs.go` + `briefs_test.go` (new)
- `control-plane/internal/discoverapi/relay_test.go` (new)
- `control-plane/internal/httpx/problem.go` — three codes (modify)
- `control-plane/internal/api/routes_audit_test.go` — one line per GET, and the audit's `Deps`
  mount a real `discoverapi.Handler` over the fake Hub (modify)

**The routes this task adds:**

| Method | Path | Body | 200 | Errors |
|---|---|---|---|---|
| GET | `/v1/discover/projects/{projectId}/binding` | — | `Binding` | 404 `project_not_found` |
| POST | `/v1/discover/projects/{projectId}/binding` | `{"teamId":"tm-…"}` | `Binding` | 409 `project_not_shared` (`details.state` = `no_remote`); relayed Hub codes |
| GET | `/v1/discover/projects/{projectId}/brief` | — | `View` (project scope) | 404 `project_not_found` |
| GET | `/v1/discover/projects/{projectId}/branches/{branchId}/brief` | — | `View` (branch scope) | 404 `project_not_found`, `branch_not_found` |
| POST | `/v1/discover/projects/{projectId}/brief/ops` | `OpsRequest` | `OpsResponse` | below |
| POST | `/v1/discover/projects/{projectId}/branches/{branchId}/brief/ops` | `OpsRequest` | `OpsResponse` | below |
| GET | `/v1/discover/briefs/{briefId}/revisions?after=&limit=` | — | `History` | relayed `brief_not_found` |

A `View` never 404s for an absent brief: `brief: null` is the answer (the old plan's rule, kept).

**Wire shapes — complete** (`View`, `Binding`, `History` are task 07's structs, marshalled as
declared there; `discover.Op`'s wire tags are task 03's):

```go
// control-plane/internal/discoverapi/briefs.go
package discoverapi

type BindRequest struct {
	TeamID string `json:"teamId"`
}

// OpsRequest is one change at the head the person saw.
type OpsRequest struct {
	BaseRevision int         `json:"baseRevision"`
	Op           discover.Op `json:"op"`
}

// The op kinds a client may send. reinherit, draft and add_follow_up are the
// server's own and are 400 bad_request, details.field = "op.kind".
var clientOps = map[discover.OpKind]bool{
	discover.OpRecord: true, discover.OpEdit: true, discover.OpSkip: true,
	discover.OpAccept: true, discover.OpDismissFollowUp: true, discover.OpRestore: true,
	discoverhub.OpProposeFollowUp: true,
}

type OpsResponse struct {
	View    discoverhub.View       `json:"view"`
	Written []string               `json:"written"` // keys written, this brief's first; [] when nothing changed
	Assist  discover.AssistOutcome `json:"assist"`  // "" unless op.kind was propose_follow_up
}
```

```jsonc
// POST /v1/discover/projects/p-3f2a91c0/branches/b-3f2a91c0-evidence-upload/brief/ops
{ "baseRevision": 5, "op": { "kind": "edit", "key": "done", "text": "Upload works for PDF, XLSX and images\nA virus scan runs before any file is visible\nAn interrupted upload resumes\nTests pass" } }
// 200
{ "view": { "scope": "branch", "projectId": "p-3f2a91c0", "branchId": "b-3f2a91c0-evidence-upload",
            "binding": { "state": "bound", "hubProjectId": "pj-…", "hubProjectName": "Engagement Portal", "repoKeys": ["github.com/…"], "teams": [] },
            "brief": { "id": "bf-…a2", "scope": "branch", "headRevision": 6, "headKey": "brief:bf-…a2@6", "status": "complete",
                       "latestCompleteRevision": 6, "latestCompleteKey": "brief:bf-…a2@6", "questions": [ … ], "answers": [ … ],
                       "inherits": { "key": "brief:bf-…b7@5", "digest": "sha256:…" }, "change": { "kind": "edit", "key": "done", "restores": 0 },
                       "savedAt": "2026-09-23T04:02:11.123Z", "savedBy": "Dylan Mackie" },
            "inherited": { "key": "brief:bf-…b7@5", "briefId": "bf-…b7", "revision": 5, "questions": [ … ], "answers": [ … ] },
            "step": { "kind": "complete", "ordinal": 0, "total": 0, "key": "", "prompt": "", "help": "", "options": [], "because": "", "pending": [] },
            "preview": { "key": "brief:bf-…a2@6", "block": "<!-- panacea:discover:begin brief=bf-…a2 rev=6 -->\n…" } },
  "written": ["brief:bf-…a2@6"], "assist": "" }
```

**Errors — the whole mapping, in one function per source:**

| Source | Status, code | `details` |
|---|---|---|
| `*hubclient.Error` 4xx except 401 | **unchanged** status, code, message, details (Hub contract §4.3) | the Hub's |
| `*hubclient.Error` 401 after the client's own refresh, or `entraauth.ErrSignedOut` | 401 `signed_out` (Hub task C1's code) | `reason` |
| `hubclient.ErrUnavailable` | 503 `upstream_unavailable` (R4's code) | `upstream: "hub"` |
| `discoverhub.ErrHubOff` on a write | 503 `hub_not_configured` (C1's code) | — |
| `discoverhub.ErrNotBound`, `ErrNoRemote` on a write | 409 `project_not_shared` (**new**) | `state`: `unbound` · `no_remote` |
| `*discoverhub.FanoutTooLargeError` | 409 `fanout_too_large` (**new**) | `needed`, `limit` |
| `discoverhub.ErrErasedRestore` | 409 `conflict` | `reason: "erased"` |
| `discover.ErrEmptyAnswer`, `ErrUnknownKey`, `ErrNotAnswered`, `ErrNotDrafted`, `ErrNotFollowUp`, `ErrBadOp`, a disallowed `op.kind` | 400 `bad_request` | `field` |
| `discover.ErrReservedText`, matched with `errors.Is`; the key from `errors.As(*discover.ReservedTextError)` (task 03 rule 12, plan §4 Security, S3) | 400 `bad_request`, message "Answers cannot contain the text panacea:discover:." | `field: "text"`, `key`: the question key |
| `project.ErrUnknownProject` | 404 `project_not_found` | `projectId` |
| a branch id not among the project's branches | 404 `branch_not_found` (**new**) | `branchId` |
| `discoverhub.ErrDigestDisagreement`, anything else | 500 `internal`, generic sentence; the cause to the log | — |

```go
// control-plane/internal/httpx/problem.go — three constants, beside CodeBriefNotWritten
	CodeProjectNotShared = "project_not_shared" // 409 the project has no Hub project (Discover)
	CodeFanoutTooLarge   = "fanout_too_large"   // 409 a project edit would re-revision more branch briefs than one Hub batch holds
	CodeBranchNotFound   = "branch_not_found"   // 404
```

`signed_out`, `hub_not_configured` and `upstream_unavailable` are **not** added here: they belong
to the Hub's task C1 and to whichever of phases 5 and 7 lands R4 first. If this task lands before
them, it adds them with the Hub contract's exact spelling (§9.3, §4.3) in a separate commit named
for that ask — never a variant spelling.

`relay` implements the first three rows once, in `api.go`, and is deleted when Hub ask **HA4-5**
(`hubclient.Relay`) lands.

**The route audit.** `catalogReadOnlyRoutes` gains, each with its reason:

```go
"GET /v1/discover/projects/{projectId}/binding":                   "Hub reads (a project lookup per repo key and /v1/me), memoised in memory for 60 s. Creates nothing; POST beside it binds.",
"GET /v1/discover/projects/{projectId}/brief":                     "Hub reads only. Never creates a brief: the first op does.",
"GET /v1/discover/projects/{projectId}/branches/{branchId}/brief": "Hub reads only, as above.",
"GET /v1/discover/briefs/{briefId}/revisions":                     "One Hub history read.",
```

**Tests first:**

| Test | Asserts | Mutant it kills |
|---|---|---|
| `TestRoutesAreTheTableAndNothingElse` | `Mount` registers exactly the seven patterns above plus task 12's and 13's | an unaudited route |
| `TestEveryGetIsInTheReadOnlyMap` (in `routes_audit_test.go`) | the audit, run over a `Deps` with a real `discoverapi.Handler`, passes; removing one line fails it | a Discover GET the audit never saw |
| `TestAbsentBriefIsNull` | a bound project with no brief → 200, `brief: null`, step 1 of 5 | a 404 that the lens has to special-case |
| `TestHubOffIsAViewNotAnError` | nil Hub → GET brief 200 with `binding.state: "hub_off"`; POST ops 503 `hub_not_configured` | Develop's rail throwing on a Discover read |
| `TestOpsRejectsServerKinds` | `reinherit`, `draft`, `add_follow_up` → 400, `details.field: "op.kind"` | a client forging a fan-out |
| `TestOpsRefusesReservedText` | a `record` whose text holds `<!-- panacea:discover:end -->` → 400 `bad_request`, `details.field: "text"`, `details.key` the recorded question's key; the fake Hub received no revision write | the marker rule enforced in `Apply` but lost at the route, or mapped to a 500 (review C2) |
| `TestOpsEchoesWrittenKeys` | a record at base 4 → `written: ["brief:…@5"]` | a desktop that has to guess the new key |
| `TestRelayTable` — one test per row | status, code, message and details for each source row | a Hub `revision_conflict` rewritten as `conflict`, losing `headRevision` |
| `TestRelayNeverLeaksATransportError` | `ErrUnavailable` wrapping a dial error to `127.0.0.1:8750` → the message is the generic sentence; the address is only in the log | a Hub address or path in the problem body |
| `TestUnknownFieldIsInvalidJSON` | an extra top-level field → 400 `invalid_json` (`httpx.DecodeJSON`) | a lenient decoder that accepts a typo'd `baseRevison` as 0 |
| `TestAliasReachesDiscover` | `/api/v1/discover/…/brief` answers as `/v1/…` | a mount outside the guarded mux |

**Steps:**

- [ ] **Step 1:** Write the tests; confirm they fail.
- [ ] **Step 2:** `api.go` (`New`, `Mount`, `relay`, the error mapping), `briefs.go`.
- [ ] **Step 3:** Add the three codes; add the audit lines and wire the audit's `Deps`.
- [ ] **Step 4:** Run the acceptance commands; commit by pathspec.

**Acceptance:**

```
$ go test ./control-plane/internal/discoverapi/... ./control-plane/internal/api/... -race -count=1
ok   github.com/kpmg-au-emu/panacea-app/control-plane/internal/discoverapi
ok   github.com/kpmg-au-emu/panacea-app/control-plane/internal/api
$ go list -deps ./control-plane/internal/api | grep -c "discover\|contextwrite\|provenance"
0
```
(`api` imports nothing of phase 4 — R1's reason. The reverse is allowed: `discoverapi` reaches
`api` through `contextwrite`, and `routes_audit_test.go` is the external package `api_test`, so
importing `discoverapi` there is not a cycle.)

**Does NOT claim:** provenance routes (task 12); analyse (task 13); any real Hub call.
