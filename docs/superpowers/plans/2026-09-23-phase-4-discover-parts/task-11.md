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
| POST | `/v1/discover/projects/{projectId}/binding` | `{"teamId":"tm-…","name":"…"}` (`name` optional: absent or blank is task 07's `DefaultName`) | `Binding` | 409 `project_not_shared` (`details.state` = `no_remote`); 409 `project_name_taken` (b10 re-review I-2); relayed Hub codes |
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
	Name   string `json:"name,omitempty"` // "" → discoverhub.DefaultName (b10 re-review I-2); 1-120 characters after trimming otherwise, else 400 bad_request field "name"
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
            "binding": { "state": "bound", "hubProjectId": "pj-…", "hubProjectName": "Engagement Portal", "repoKeys": ["github.com/…"], "teams": [], "matches": [], "defaultName": "" },
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
| `*hubclient.Error` 401 after the client's own refresh, or `signin.ErrSignedOut` | 401 `signed_out` (Hub task C1's code) | `reason` |
| `hubclient.ErrUnavailable` | 503 `upstream_unavailable` (R4's code) | `upstream: "hub"` |
| `discoverhub.ErrHubOff` on a write | 503 `hub_not_configured` (C1's code) | — |
| `discoverhub.ErrNotBound`, `ErrNoRemote` on a write | 409 `project_not_shared` (**new**) | `state`: `unbound` · `no_remote` |
| `*discoverhub.FanoutTooLargeError` | 409 `fanout_too_large` (**new**), with one message per cause (b10 re-review M-3), the numbers written with thousands separators: the count, "Nothing was saved. This change would update {needed} branch briefs, and the Hub takes {limit} in one save."; the body, "Nothing was saved. This change would send {bytes} bytes to the Hub, and it takes {limitBytes} in one save."; one branch, "Nothing was saved. The brief for branch {branchRef} is too large to carry this change ({bytes} bytes; the Hub takes {limitBytes}). Its answers must be shortened first." | `needed`, `limit`; `bytes`, `limitBytes` when a size is what failed (task 08, review M6); `branchRef` when one branch brief's own revision is what failed (b10 re-review M-2) |
| `*discoverhub.NameTakenError` (task 07, b10 re-review I-2), checked before the Hub-relay row | 409 `project_name_taken` (**new**), message "This team already has a project called {name}. Choose another name." | `field: "name"`, `name` |
| `discoverhub.ErrErasedRestore` | 409 `conflict` | `reason: "erased"` |
| `discover.ErrEmptyAnswer`, `ErrUnknownKey`, `ErrNotAnswered`, `ErrNotDrafted`, `ErrNotFollowUp`, `ErrBadOp`, a disallowed `op.kind` | 400 `bad_request` | `field` |
| `discover.ErrReservedText`, matched with `errors.Is`; the key from `errors.As(*discover.ReservedTextError)` (task 03 rule 12, plan §4 Security, S3) | 400 `bad_request`, message "Answers cannot contain the text panacea:discover:." | `field: "text"`, `key`: the question key |
| `discover.ErrTooLong`, matched with `errors.Is`; key and limit from `errors.As(*discover.TooLongError)` (task 03 rule 13, plan §4 Security, S4, review M6) | 400 `bad_request`, message "This answer is too long. Keep it to 4,000 characters." (`limit` 300: "This question is too long. Keep it to 300 characters.") | `field: "text"`, `key`, `limit` |
| `*discoverhub.RevisionTooLargeError` (task 08, S4) | 400 `bad_request`, message "This brief is too long to share. Shorten an answer and try again." | `field: "text"`, `key`: the op's question key, or `""` for an op with none (`restore`, `propose_follow_up`: the brief as a whole, b10 re-review M-2; `reinherit` is a server op, reached only as a fan-out item, which is `fanout_too_large`), `bytes`, `limit` |
| `project.ErrUnknownProject` | 404 `project_not_found` | `projectId` |
| a branch id not among the project's branches | 404 `branch_not_found` (**new**) | `branchId` |
| the route's own deadline expired (`Budgets`, below) while a Hub call was in flight: the route's `ctx.Err() == context.DeadlineExceeded`, whatever error type the Hub call returned | 503 `upstream_unavailable` | `upstream: "hub"` |
| `discoverhub.ErrDigestDisagreement`, anything else | 500 `internal`, generic sentence; the cause to the log | — |

**Budgets — every Discover route runs under a deadline below the desktop's `timeoutMs` (review
I4).** A black-holed Hub (VPN down, a proxy that swallows packets) does not refuse a connection:
`hubclient` waits 10 s per attempt, with two retries after 250 ms and 1 s (Hub contract §9.2,
`hub-contract.md:1337-1339` at hub `2f84662`), so one read can take about 31 s. The desktop's
default is 10 s (`client.ts:33`). Without a server deadline the person sees the desktop's
`timeout`, not the typed `upstream_unavailable` that the lens words as "the Hub cannot be reached".
The rule: **server deadline < desktop `timeoutMs`, for every Discover call.**

```go
// control-plane/internal/discoverapi/api.go
// Budgets are the route deadlines. Config.Budgets zero values take these
// defaults; tests set them in milliseconds.
type Budgets struct {
	Default  time.Duration // 8 s: every GET, and ops other than propose_follow_up (desktop 10 s)
	FollowUp time.Duration // 28 s: ops with op.kind propose_follow_up (desktop 30 s); discover.FollowUpBudget (20 s) is inside it
	Analyse  time.Duration // 55 s: POST …/analyse (desktop 60 s, task 13); repoanalysis.DraftBudget (35 s) is inside it
}
```

Each handler wraps `r.Context()` in `context.WithTimeout` from `Config.Budgets` before its first Hub
call. A model call that outlasts its own inner budget degrades (`model_failed`) and is never this
row. The row above classifies by the route's own `ctx.Err()`, never by the error's type.

**Back-off that cannot be interrupted.** The Hub's transport design (Hub task 33 at `2f84662`,
`phase-h-hub-parts/task-33.md:29-31` and `:52-53`) sleeps with `time.Sleep`, which no `ctx` can
cut short, and retries a timeout, including the caller's own `context.DeadlineExceeded`. A deadline
that expires during an attempt therefore costs up to **1.25 s** more: 250 ms of sleep, an attempt
that fails at once, 1 s of sleep, and a third that fails at once. Every budget here is sized to
absorb that. Each is at least 2 s under its desktop `timeoutMs`: `Default` 8 s against 10 s,
`FollowUp` 28 s against 30 s, `Analyse` 55 s against 60 s, and `Materialise`'s Hub budget 5 s
(task 09) inside session start's 10 s. So the typed 503 still arrives at least 0.75 s before the
desktop gives up. Of Hub ask **HA4-10**'s three changes (plan §9), only **(a)** must land for I4 to
hold: each attempt's context derives from the caller's `ctx`. Without (a), a black-holed read runs
its full ~31 s whatever the budget. (b), no retry once the caller's `ctx` is done, and (c), a
`ctx`-aware `Sleep`, only remove the 1.25 s. If the Hub stream refuses (a), `discoverhub` takes
the fallback HA4-10 declares: each Hub call runs in a goroutine and the route returns on
`ctx.Done()`. Unverified until task 23 step 10 runs against the real `hubclient`.

```go
// control-plane/internal/httpx/problem.go — four constants, beside CodeBriefNotWritten
	CodeProjectNotShared = "project_not_shared" // 409 the project has no Hub project (Discover)
	CodeFanoutTooLarge   = "fanout_too_large"   // 409 a project edit one Hub batch cannot carry: too many branch briefs, too many bytes, or one branch brief too large
	CodeBranchNotFound   = "branch_not_found"   // 404
	CodeProjectNameTaken = "project_name_taken" // 409 Share under a name the team already holds (b10 re-review I-2)
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
| `TestRelayTable` — one test per row | status, code, message and details for each source row; `fanout_too_large` gets one per cause (count, body, branch), each asserting its own message and that `branchRef` is present only for the branch cause (b10 re-review M-3); `RevisionTooLargeError` with `Key ""` sends `key: ""` | a Hub `revision_conflict` rewritten as `conflict`, losing `headRevision` |
| `TestRelayNeverLeaksATransportError` | `ErrUnavailable` wrapping a dial error to `127.0.0.1:8750` → the message is the generic sentence; the address is only in the log | a Hub address or path in the problem body |
| `TestAHubThatNeverAnswersIs503BeforeTheDesktopGivesUp` | a fake Hub whose every call blocks until `ctx.Done()`, `Budgets{Default: 200ms, FollowUp: 300ms}`: GET brief, a `record` op and a `propose_follow_up` op each return 503 `upstream_unavailable`, `upstream: "hub"`, within 1 s | a route with no deadline, which a black-holed Hub turns into the desktop's `timeout` (review I4) |
| `TestTheDefaultBudgetsAreBelowTheDesktopTimeouts` | `New(Config{})` fills `Budgets{8s, 28s, 55s}`, each below task 15's 10 s, 30 s and 60 s | a budget raised past the desktop's, which re-opens I4 |
| `TestBindPassesTheName` (b10 re-review I-2) | `{"teamId":"tm-1"}` and `{"teamId":"tm-1","name":"  "}` → `Bind` called with `name == ""`; `"name":"Portal fork"` → `Bind` called with it; a 121-character name → 400 `bad_request`, `details.field: "name"`, no Hub call; `*NameTakenError` → 409 `project_name_taken` with `details.name` | a name dropped on the way, so every fork Shares under the directory name; a 120-character limit left to the Hub's 400, which names no field |
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
