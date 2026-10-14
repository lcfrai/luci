### Task 12: `discoverapi` — refs, the head reading, materialisations

The two provenance endpoints of contract §3.1 — **two endpoints, one type** — and the per-worktree
drift view. Every surface in every phase that shows a brief resolves it here: `ContextChip` and
`ShippedManifest` by key, `DefinitionOfDone` and Discover by the head reading.

**Files:**
- `control-plane/internal/discoverapi/provenance.go` + `provenance_test.go` (new)
- `control-plane/internal/api/routes_audit_test.go` — four lines (modify)

**Routes:**

| Method | Path | 200 | Errors |
|---|---|---|---|
| GET | `/v1/discover/refs?key=…&key=…` (1–100) | `RefsResponse` | 400 `bad_request` (`field: "key"`) for 0 or more than 100; **never** for an unknown or malformed key |
| GET | `/v1/discover/projects/{projectId}/provenance` | `HeadReading` — what a session on a detached worktree would inherit, and its criterion states | 404 `project_not_found` |
| GET | `/v1/discover/projects/{projectId}/branches/{branchId}/provenance` | `HeadReading` — what a session on this branch would inherit, and its criterion states | 404 `project_not_found`, `branch_not_found` |
| GET | `/v1/discover/projects/{projectId}/branches/{branchId}/materialisations` | `Materialisations` | 404 as above |

**Wire shapes — complete:**

```go
// control-plane/internal/discoverapi/provenance.go
package discoverapi

// RefsResponse is one Ref per requested key, in request order (a repeated
// key is answered in each slot), plus the time-varying heads of every brief
// named, plus the time-varying criterion states of every resolved key. The
// desktop caches refs by key for ever and heads per brief (contract §6.4) -
// except slots whose resolution is "unavailable" - and OVERWRITES a key's
// criterion states on every answer that carries them (review I14).
type RefsResponse struct {
	Refs            []provenance.Ref                         `json:"refs"`
	Heads           map[string]provenance.Head               `json:"heads"`
	CriterionStates map[string][]provenance.CriterionReading `json:"criterionStates"` // by key; resolved slots only
}

// HeadReading is a head reading: the Ref, plus its criterion states beside it
// (never inside it), from the same HeadRef call (task 07). DefinitionOfDone is
// the one surface that shows states, and it reads them from here, so they do
// not depend on some session having stamped the same key (re-review b06-1
// R-I3). CriterionStates is [] when the Ref is not resolved, never null.
type HeadReading struct {
	Ref             provenance.Ref                `json:"ref"`
	CriterionStates []provenance.CriterionReading `json:"criterionStates"`
}

// Materialisations is every worktree of the branch and the state of its
// managed block, for Discover's "who inherits this" and "edited by hand"
// views (prototype states 4, 8 and 11).
type Materialisations struct {
	ExpectedKey string                    `json:"expectedKey"` // what a start would write now; "" for none
	Worktrees   []contextwrite.Inspection `json:"worktrees"`   // in the scanner's order
}
```

```jsonc
// GET /v1/discover/refs?key=brief:bf-…a2@5&key=none:declined&key=brief:bf-…40@5&key=
// 200
{ "refs": [
    { "key": "brief:bf-…a2@5", "resolution": "resolved", "noneReason": "", "present": true,
      "briefId": "bf-…a2", "revision": 5, "digest": "sha256:…", "scope": "branch",
      "projectId": "pj-…", "branchRef": "refs/heads/evidence-upload", "status": "complete",
      "headRevision": 6, "latestCompleteRevision": 6,
      "inherits": { "key": "brief:bf-…b7@5", "briefId": "bf-…b7", "revision": 5, "digest": "sha256:…", "erased": false },
      "title": "Branch brief — evidence-upload",
      "headline": "Let client users upload evidence files against an open request.",
      "audience": "KPMG staff broadly", "constraints": "Existing uploads and their download links",
      "outOfScope": "Bulk upload; the reviewer side of evidence",
      "sections": [ … ], "criteria": [ { "id": "c-…", "text": "Tests pass", "digest": "sha256:…" } ],
      "savedAt": "…", "savedBy": "Dylan Mackie", "erasedAt": "" },
    { "key": "none:declined", "resolution": "none", "noneReason": "declined", "present": false, … },
    { "key": "brief:bf-…40@5", "resolution": "missing", "present": false, … },
    { "key": "", "resolution": "none", "noneReason": "", "present": false, … } ],
  "heads": { "bf-…a2": { "headRevision": 6, "latestCompleteRevision": 6, "archived": false },
             "bf-…b7": { "headRevision": 5, "latestCompleteRevision": 5, "archived": false } },
  "criterionStates": { "brief:bf-…a2@5": [ { "id": "c-…", "state": "open", "because": "" } ] } }
```

Every `Ref` field is present in every slot — empty strings, zeroes, `null` for `inherits`, `[]`
for lists — so the desktop's type has no optional fields.

`heads` names the pinned project brief (`bf-…b7`) as well as the slot's own: `Resolve` sends every
pin to the Hub, cached or not, and the Hub names every brief a resolved key names (Hub contract
§7.5 :829; task 07). A pin's brief is absent only when the call that carried it failed (task 07
rule 5), and then the desktop's rule 4 decides nothing until it arrives (re-review b06-1 R-M4).

**Rules:**

1. `refs` delegates to `discoverhub.Service.Resolve` (task 07), which **never returns a Hub
   error** — it has no error result (task 07 rule 5, review I6). The one rule, restated: a slot
   whose pieces are all in hand stays `resolved` (a branch key's pieces are its revision **and**
   its pinned project revision, each from the cache or a call that succeeded; re-review b05-1
   R-M1); every other brief slot is `unavailable` on any Hub error,
   on `signed_out`, and when the Hub is not configured (`hub_off`, a nil `discoverhub.Config.Hub`); `missing`
   comes only from the Hub's own answer, never from its absence. The response is 200 in every one
   of those cases. Only a failure to parse the query is a 4xx.
2. The head readings delegate to `Service.HeadRef`. A Hub that cannot be reached is 503
   `upstream_unavailable` here — a head is never served from cache, because it moves. A Hub that
   is **not configured** is not "cannot be reached": `HeadRef` answers a `none` Ref with no error
   (task 07 rule 6), so the reading is 200 `none` — a session started now inherits nothing, and
   Develop is not blocked for a Hub it was never given (Hub contract §9.5; re-review b05-1 N2).
3. `materialisations` lists the branch's worktrees through the scanner and calls
   `contextwrite.Materialiser.Inspect` for each. It **reads files and writes nothing**.
4. Criterion states come from `Config.Evaluate` (phase 3's seam, S4-P3.4); nil → all `open`.
   They are **not** in any `Ref`: `Resolve`'s `states` becomes `criterionStates`, keyed by key,
   one entry per `resolved` slot whose readings were taken, recomputed on every call (review
   I14), and none for a branch slot whose binding task 07 rule 8 could not read (task 06 fix
   round 2, re-review 1 R1-I1): "binding unknown" is not sent as `open`, so the desktop keeps
   its last reading. The route copies `states` as it is and adds no entry. `criterionStates`
   is `{}`, never `null`, when no slot has one. The head readings serve `HeadRef`'s `states`
   as `HeadReading.criterionStates`, `[]` when the `Ref` is not resolved (re-review b06-1 R-I3).
5. **Every route here runs under task 11's `Budgets.Default` (8 s; review I4)**, below the
   desktop's 10 s. For the head readings, the deadline's expiry is task 11's route-deadline row
   (503 `upstream_unavailable`). For `refs`, it is "unreachable" in rule 1's sense: the cache
   answers what it can, the rest are `unavailable`, and the response is still 200, so a
   black-holed Hub never turns the chip into the desktop's `timeout`. For `materialisations`, the
   expiry is task 11's route-deadline row too (503 `upstream_unavailable`, `upstream: "hub"`),
   classified by the route's `ctx.Err()`. That can name an expiry during `Inspect`'s local git and
   file reads as the Hub's, the misnaming task 09 avoids by budgeting only its Hub step. It is
   accepted here, and stated: those reads take milliseconds per worktree, so the 8 s can run out
   during one only after `Inspect`'s `Source` reads have spent almost all of it, and then the Hub
   is the cause (re-review b04-1, nit 3).

**The route audit** gains four lines:

```go
"GET /v1/discover/refs":                                                   "Hub reads through an in-memory cache of immutable revisions. Writes nothing.",
"GET /v1/discover/projects/{projectId}/provenance":                        "One Hub head read.",
"GET /v1/discover/projects/{projectId}/branches/{branchId}/provenance":    "One Hub head read.",
"GET /v1/discover/projects/{projectId}/branches/{branchId}/materialisations": "Reads each worktree's CLAUDE.md to compare it with a fresh render (contract §3.3's one permitted read-back). Writes no file.",
```

**Tests first:**

| Test | Asserts | Mutant it kills |
|---|---|---|
| `TestRefsAnswersEverySlotInOrder` | four keys in, four refs out, in order, with a repeat answered twice | a map-ordered response the desktop cannot zip |
| `TestRefsNeverFailsTheBatchForABadKey` | `b-42`, `brief:bf-…@0`, `""` among good keys → 200, `invalid`/`none` in their slots | one typo blanking twelve chips |
| `TestRefsLimits` | 0 keys and 101 keys → 400 `field: "key"` | an unbounded fan-out to the Hub |
| `TestRefsServesCachedSlotsWhenTheHubIsDown` | resolve once; fake goes unavailable; resolve the same key plus a new one → 200, first `resolved`, second `unavailable` | a whole Develop column of chips blanked by one unreachable Hub (ADR-H1 §5.4) |
| `TestRefsWithTheHubOffOrSignedOutIsUnavailable` (review I6) | a `discoverhub.Service` built with a nil `discoverhub.Config.Hub`, and one whose fake returns `signin.ErrSignedOut` → 200 (not 401, not 503), every uncached brief slot `unavailable`, **none** `missing` | the route relaying a `Resolve` error, or `hub_off` shown as data loss on every chip (task 07's `TestHubOffIsUnavailableNotMissing` is the unit half) |
| `TestRefsJSONHasEveryFieldInEverySlot` | each slot's JSON object has the same key set as the golden `Ref` of task 06 | optional fields the desktop has to guard |
| `TestRefsServesCriterionStatesBesideTheRefs` (review I14) | a resolved key, a `missing` one and `""` → `criterionStates` has exactly the resolved key, with one `{id, state, because}` per `criteria` item of its slot, same ids and order; no slot's `criteria` item has `state` or `because`; zero resolved slots → `"criterionStates": {}`; with `Config.Evaluate` set, a branch key resolved once on a healthy fake and again under `SetUnavailable(true)`, so it is served from the cache while `List` fails → its slot `resolved` and **no** `criterionStates` entry for it, never one of `open` readings (task 07 rule 8; task 06 fix round 2, re-review 1 R1-I1) | a verdict inside the cached `Ref`, or a `null` the desktop has to guard; a route that fills a missing entry with `open`, which overwrites the desktop's `settled` for the whole outage |
| `TestHeadReadingIsNeverCached` | two reads with the head moved between them return different revisions | a stale `DefinitionOfDone` |
| `TestHeadReadingOfAnUnreachableHubIs503` | `upstream_unavailable`, `upstream: "hub"` | a head served from memory |
| `TestHeadReadingServesCriterionStates` (re-review b06-1 R-I3) | a branch whose head key no session stamped; `Config.Evaluate` answers every criterion `settled` with `Because` `{"CI run 88 passed"}` → 200, `ref` has no `state` or `because` under `criteria`, and `criterionStates` has one `settled` reading per `ref.criteria` item, same ids and order | a head reading with no states, which leaves `DefinitionOfDone` showing `open` until phase 3 happens to mark that exact key |
| `TestHeadReadingWithTheHubOffIsNone` (re-review b05-1 N2) | a `discoverhub.Service` built with a nil `discoverhub.Config.Hub` → 200, `ref.resolution: "none"`, every `Ref` field present, `criterionStates: []` | a nil Hub mapped to 503, which puts an error on Develop's `DefinitionOfDone` for a Hub nobody configured |
| `TestRefsAgainstAHubThatNeverAnswersIs200WithinTheBudget` | a fake Hub that blocks until `ctx.Done()`, `Budgets{Default: 200ms}`, one key cached and one not → 200 within 1 s, the cached slot `resolved`, the other `unavailable` | a `refs` with no deadline (the desktop's `timeout` on every chip), or a deadline expiry mapped to 503 when rule 1 promises 200 (review I4) |
| `TestMaterialisationsWritesNothing` | every worktree's file bytes and mtimes are unchanged after the call | a GET with a side effect (§10a) |
| `TestMaterialisationsListsTheBranchsWorktreesOnly` | two branches, three worktrees: only this branch's two | a cross-branch leak into "who inherits this" |

**Steps:**

- [ ] **Step 1:** Write the tests; confirm they fail.
- [ ] **Step 2:** `provenance.go`; add the four routes to `Mount`; add the audit lines.
- [ ] **Step 3:** Run the acceptance commands; commit by pathspec.

**Acceptance:**

```
$ go test ./control-plane/internal/discoverapi/... ./control-plane/internal/api/... -race -count=1
ok   github.com/kpmg-au-emu/panacea-app/control-plane/internal/discoverapi
ok   github.com/kpmg-au-emu/panacea-app/control-plane/internal/api
```

**Does NOT claim:** `design:` key resolution for phase 5 (the Hub's `/v1/refs` answers those; this
route passes them through as `resolved`/`missing` with phase 4's `Ref` fields empty — phase 5 adds
its own fields or its own surface, S4-P5.1); criterion evaluation (phase 3).
