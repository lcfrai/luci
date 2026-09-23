### Task 00: S0 absorbs the sibling asks — every interface the siblings code to, decided before it freezes

**`feat/hub`, before task 01; read with task 44a, which folds these answers into the contract.
No code of its own: each row below is built by the task it names. About 0.5 hours to re-read.**

S0 (tasks 01–05) is the first thing phases 3, 4, 5 and 7 code against, and this plan promises that
*"nothing below S0 changes an interface they code to"*. The sibling plans name their Hub asks
precisely (phase 4 plan `:475-483`, HA4-1…HA4-9; phase 5 plan `:458-462`, HA5-1…HA5-5; phase 7 plan
`:540-616`, HA7-1…HA7-5). Three of them are **not** additive — HA5-1 changes a parameter type,
HA7-1 a column, a trigger and a field type, and HA7-1 again the signature of `JoinDeployment` — so
they are decided here, before task 01, and every S0 task builds the answer. A later ask is
additive only if it adds a method, a field or a route; anything else reopens this part. An added
method is additive **because siblings consume these interfaces and `NewFake()`; they never
implement them**. A sibling that needs a double uses `NewFake()` or embeds the interface, never a
hand-written type that satisfies it. (Checked on 2026-09-24 by grepping the phase 3, 4, 5 and 7
plans for a type, a `var _` assertion or a stub that implements a `hubclient` interface: none.)

The sibling line numbers were read on 2026-09-24 from the sibling worktrees
(`.claude/worktrees/phase-4`, `phase-5`, `phase-7`, `docs/superpowers/plans/`); they move as those
plans are edited, so each row quotes the ask's id, which does not.

#### Every ask and its answer

| Ask | What it needs | The answer (what this plan builds) | Built by |
|---|---|---|---|
| **HA4-1** | the `hubapi` Go structs, field by field | task 03 writes every §7 object and request as a Go struct; S0 freezes them. Phase 4's assumed shapes differ only in names, and it fixes those mechanically | 03 |
| **HA4-2** | `Briefs.List(ctx, projectID, BriefQuery) (Page[BriefHead], error)`; phase 4's task 08 is blocked on it | **built as asked**, with one change: `BriefQuery` carries `Page PageRequest` rather than flat `After`/`Limit`, the shape of every other query type (`ReleaseQuery`, `ApprovalQuery`). Route 19 already exists (task 18) | 03, 04, 05, 34 |
| **HA4-3** | a revision batch of **64**, the route's body cap **4 MiB**, `hubapi.MaxRevisionBatch` | **built as asked**: `hubapi.MaxRevisionBatch = 64`, `hubapi.MaxRevisionBatchBytes = 4 << 20`. Route 26's body cap is that constant (task 14's `WriteLimit`); every other route keeps `httpx.MaxBodyBytes`. The reply cap in `hubclient` is raised to 8 MiB. Both caps hold only under the *Wire rule* (no HTML escaping on either side; see *Sizes* below) | 03, 04, 08, 14, 19, 33, 34 |
| **HA4-4** | `latestComplete` beside `revision` in `ForBranch` | **built as asked**: `BriefWithRevision.LatestComplete *BriefRevision` (`"latestComplete"`), `null` exactly when `LatestCompleteRevision` is 0, and the same revision as `revision` when the head is complete. One type, so `GET /v1/briefs/{id}` carries it too | 03, 04, 18 |
| **HA4-5** | `Relay(err) (status int, p httpx.Problem, ok bool)` in `hubclient` | **built as asked**. `WriteError` becomes `Relay` plus `httpx.WriteProblem`; `ok` is false for an error the rule does not classify, which `WriteError` answers 500 `internal`. `Error` gains `RetryAfter string` so a caller of `Relay` can copy a 429's header — `WriteError` could not, before, because `Error` did not carry it | 04, 33 |
| **HA4-6** | one Azure DevOps repo-key form | = **A1**: the Hub accepts `azure-devops/<org>/<project>/<repo>`, lowercased | 01, 17 |
| **HA4-7** | the fake follows `content.status` exactly and exposes `latestCompleteRevision` on heads and in `/v1/refs`'s `heads` | **confirmed**: task 04 rule 4 (`latestCompleteRevision` moves only on `complete`) and task 03's `HeadInfo` | 04 (no change) |
| **HA4-8** | `Subscribe(func(hubapi.Event)) (cancel func())`, in commit order | **this plan's shape stands, confirmed**: `Subscribe(kindPrefix string, fn func(hubapi.Event)) (unsubscribe func())`. It is phase 4's shape plus a prefix; `Subscribe("", fn)` is exactly the ask. Delivery is in `seq` order, which is commit order (the org's audit lock serialises every write, task 14) | 36 (no change) |
| **HA4-9** | `BriefRevisionDigest(briefID, revision, projectID, scope, branchRef, content)` in `hubapi` | **met, confirmed**: `hubapi.BriefEnvelope{BriefID, Revision, ProjectID, Scope, BranchRef, Content}.Digest()` (task 02) is exported and is the ask with named fields. No second spelling: two functions for one digest is how they drift | 02 (no change) |
| **HA5-1** | `PutBlob(…, body io.ReadSeeker)`, because a retry cannot replay a consumed reader | **built as asked**: the parameter is `io.ReadSeeker`; the transport seeks to 0 before each attempt, and a failed `Seek` ends the call with that error, never a retry with a short body | 04, 05, 33, 34 |
| **HA5-2** | §9.2's `Designs` amended to this plan's seven methods | **yes**: 44a writes the seven into §9.2 | 44a |
| **HA5-3** | the fake models §7.7's checks 1–6, §7.8, `digest_mismatch`, `quota_exceeded` | **confirmed**: task 05 rules 1–3 and the `designs` rows | 05 (no change) |
| **HA5-4** | ADR-H1 R5's *"audited with its session context"* has no field to carry | **the Hub records what it can verify**: the principal and the time. No client-asserted `surface` field: the Hub cannot check it, and an audit row that records an unverifiable claim reads as evidence. Deleting the ADR's phrase is the owner's (an owner question); 44a states the contract's reading | 44a; owner |
| **HA5-5** | v2: canvases in the Hub | **reserved**: seam S-CANVAS; nothing in v1 | — |
| **HA7-1** | `environment` may be `null` at reserve, set by the deployment join | **built** — with two corrections to the review's shape, below | 03, 04, 05, 07, 23, 24, 34 |
| **HA7-2** | §7.9's rollback confirm is `{}`; a rollback confirm naming its **source's** deployment is 409 | **built**: the fake and the Hub refuse a rollback confirm whose `deploymentId` equals its `sourceDeploymentId` with 409 `conflict`; 44a writes the sentence | 05, 24, 44a |
| **HA7-3** | a shared deploy target on the project | **not in v1**, additive later; phase 7's local file stands. Recorded as seam S-DEPLOY-TARGET (reserved) | — |
| **HA7-4** | one generic cursor mechanism: `CursorSource`, one `Panacea-<Name>-Cursor` header per source | **control plane: built as asked** (task 36). **Desktop: built with one declared difference** (task 40), as HA4-2's `Page` is. In place of `startPolling(…, onCursor?: (name, value) => void)`, called once per changed header, there is one `onHubHeaders(h)` hook. `h.cursors` is keyed by the **lowercased** name (`hub`, `deploy`, never `Deploy`), and each consumer compares its own entry with the last value it saw. `onCursor` is **not** built. Phase 7 adds no second hook to `poller.ts`: it passes its feed's `noteHeaders` through `App.tsx`'s one function and reads `h.cursors.deploy` | 36, 40 |
| **HA7-5** | a `kind` filter on the release list | **built**: `ReleaseQuery.Kind` and `&kind=run\|rollback` on route 36 | 03, 05, 23, 34 |

#### HA7-1, exactly — and why the review's shape was corrected

The review (C1) proposed `CHECK (environment IS NOT NULL OR state = 'reserved')` and letting
`guard_release` allow `environment: NULL → value` **on a reserved release**. Both fail on this
plan's facts:

- **The join happens after the confirm.** Route 40 accepts **confirmed** releases only (task 24),
  and phase 7's watcher joins the Deployment it later sees. A rule that lets the environment move
  only while `reserved` would never fire, and the null would stay for ever.
- **The CHECK would break three writes.** A null-environment run could not be **confirmed** (the
  join has not happened yet), **abandoned** (Panacea refused the deploy before any environment
  existed) or **expired** by the janitor: each moves `state` off `reserved` with `environment`
  still null, and Postgres would refuse the `UPDATE` as a check violation — a 500 to the user.

What is built instead:

```sql
-- task 07, releases: the column, and who may leave it null
environment  text  CHECK (environment ~ '^[a-z0-9-]{1,63}$'),        -- nullable (HA7-1)
CHECK (environment IS NOT NULL OR (kind = 'run' AND source_deployment_id IS NULL)),
```

A rollback, and a run with a `sourceDeploymentId`, always name the environment (Panacea knows it
for a deployment that already exists). `guard_release` drops `environment` from the immutable tuple
and adds exactly one allowed change: **`environment` NULL → a value, once, while `state` is
`reserved` or `confirmed`, every other column unchanged**. A value never changes and never returns
to null. Every other rule of the guard stands.

```go
// task 03 — hubapi
type ReserveRequest struct { …; Environment *string `json:"environment"` }   // null: a run without sourceDeploymentId only
type Release struct        { …; Environment *string `json:"environment"` }   // null until the join names it
type DeploymentJoinRequest struct {
	DeploymentID string  `json:"deploymentId"`
	Environment  *string `json:"environment,omitempty"` // sets Release.environment when it is null
}
// task 04 — hubclient.Releases: the join takes the request, not a bare id
JoinDeployment(ctx context.Context, releaseID string, req hubapi.DeploymentJoinRequest) (hubapi.DeploymentJoin, error)
```

Route 40 (task 24): a join naming an environment **equal** to the release's is a no-op on the
column; naming one on a null-environment release sets it in the join's transaction; naming a
**different** one from a non-null value is 409 `conflict`, `details.field=environment`, and nothing
is joined. `release.deployment_joined` gains `data.environment`: the release's environment
**after** the join, `""` while it is still null (task 24). A join that names none on a release
whose environment is `feat-x` emits `"feat-x"`.
Route 35 (task 23): `environment` absent or `null` is 400 `field: environment` for a rollback or a
run with `sourceDeploymentId`; `release.reserved`'s `data.environment` is `""` when null.

#### Sizes — the batch, and the reply cap it exposed

| Constant | Value | Why |
|---|---|---|
| `hubapi.MaxRevisionBatch` | 64 | HA4-3; phase 4's fan-out re-revisions every inheriting branch brief in one batch |
| `hubapi.MaxRevisionBatchBytes` | 4 MiB | 64 × the 64 KiB envelope cap (task 02, `ErrEnvelopeTooLarge`). The cap is on **wire** bytes and the item cap is on **canonical** bytes, so the plan fixes the wire encoding (the *Wire rule* below). Under it each item is `{"briefId","baseRevision","content"}` with `content` byte-for-byte its canonical form — fewer bytes than the canonical envelope, whose other members are `v`, `kind`, `briefId`, `revision`, `projectId`, `scope` and `branchRef` — so 64 maximum-size revisions fit, whatever characters their content holds |
| `hubclient`'s JSON reply cap | 8 MiB (was 1 MiB) | the two largest bodies the Hub sends: `/v1/refs`, 100 keys × a 64 KiB revision (≈ 6.3 MiB), and route 26's reply, 64 revisions echoed with content (≈ 4.1 MiB). Both figures hold **only** under the *Wire rule*: with the default encoder a content of `<` characters is six times its canonical size, and neither is a bound. At 1 MiB, a **committed** batch of 16 near-maximum revisions — already legal before HA4-3 — was reported `ErrUnavailable`, and its idempotent replay again |

**The wire rule (R1 of the b01 re-review).** Canonical form writes `<`, `>`, `&`, U+2028 and
U+2029 as raw UTF-8 bytes (task 02). Go's default JSON encoding does not. `json.Marshal`, and a
`json.Encoder` left at its default, write each of `<`, `>` and `&` as the six bytes `<`, and
U+2028 and U+2029 (three bytes each) as six. They do this even inside a `json.RawMessage`: go1.26
`encoding/json/encode.go:487` compacts a marshaller's output through `appendCompact(…,
opts.escapeHTML)`, and `indent.go:57-71` escapes those five characters only when that flag is set.
`json.Compact` would not help either. It removes whitespace and keeps every escape the writer sent,
so a `é` stays six bytes for a two-byte `é`. The rule is therefore:

1. **Requests.** `hubclient` encodes every JSON request body once per call, with
   `json.NewEncoder(&buf)` and `SetEscapeHTML(false)`, and never with `json.Marshal` (task 33).
   `WriteRevisions` replaces each `content` with `hubapi.Canonical(content)` before it encodes
   (task 34), so the wire carries exactly the bytes the Hub will digest.
2. **Replies.** The Hub's `server.WriteJSON`, which writes every success body, encodes with
   `SetEscapeHTML(false)` itself and no longer calls `httpx.WriteJSON` (task 08).
   `httpx.WriteJSON` is shared with the control plane and is unchanged. Turning off the escape is
   safe because every Hub response carries `Content-Type: application/json` and
   `X-Content-Type-Options: nosniff` (task 08). A `content` in a reply is then its stored
   canonical bytes.
3. **What still expands.** A Go `string` field is escaped as before: U+2028 and U+2029 always
   (`encode.go:1054`), control characters as `\u00xx`. These are ids, names and times. All of them
   are bounded and none is `content`, so they fall inside the fixed per-item wrapper that the
   ≈ figures above already include.

#### HA7-4, exactly — the cursor headers

```go
// task 36 — control-plane/internal/api
// CursorSource is one change cursor the desktop learns from the poll it already
// makes. Every response to an allowed caller carries "Panacea-<Name>-Cursor: <Cursor()>"
// for each source whose Cursor() is not "".
type CursorSource interface {
	CursorName() string // one token of [A-Za-z], 1–32 long, unique among the sources ignoring case: "Hub", "Deploy"
	Cursor() string     // opaque; "" means "do not set the header"
}
// Deps.CursorSources []CursorSource — api.New refuses a bad name, or a duplicate compared case-insensitively.
```

`hubfeed.Poller` is the first source (`"Hub"`); phase 7 appends its own in its own wiring. The
`Access-Control-Expose-Headers` value is built once, in `api.New`, from the sources' names plus
`Panacea-Auth-State`, and set by the same wrap — so `httpx/origin.go` holds no Hub or phase-7 name.
The desktop half (task 40) is **not** phase 7's `onCursor(name, value)`. It is a declared
difference. `startPolling`'s one `onHubHeaders` hook receives `{ cursors: Record<string, string>;
authState }`, with one entry per `Panacea-<Name>-Cursor` header, keyed by the **lowercased** `<Name>`
(`fetch` lowercases header names, and task 40 matches `^panacea-([a-z]{1,32})-cursor$`). So the Hub's
entry is `cursors.hub`, phase 7's is `cursors.deploy`, and `cursors.Deploy` is always `undefined`.
`App.tsx` passes one function, and each consumer's `noteHeaders` is called from it. Each consumer
compares its own entry with the last value it saw, which does the work `onCursor`'s "changed" filter
would have done.

#### Acceptance

```bash
P=docs/superpowers/plans/2026-09-23-phase-h-hub-parts
grep -c "MaxRevisionBatch" $P/task-03.md $P/task-19.md $P/task-04.md       # each ≥ 1
grep -n "LatestComplete" $P/task-03.md                                      # the BriefWithRevision field
grep -n "io.ReadSeeker" $P/task-04.md $P/task-34.md                         # PutBlob
grep -n "func Relay" $P/task-33.md                                          # present
grep -n "CursorSource" $P/task-36.md                                        # present
grep -n "environment IS NOT NULL OR" $P/task-07.md                          # the HA7-1 check
grep -rn "1–16" $P/ --exclude=task-00.md                                   # none left
```

#### What this part does not claim

- That the siblings have re-read these answers: 44a publishes them in the contract, and the
  orchestrator announces 44a to phases 3, 4, 5 and 7 with its sha and the `git show
  <44a-sha>:docs/superpowers/contracts/2026-09-23-hub-contract.md` read path (task 44a, step 3).
- HA5-4's ADR text: deleting R5's phrase is the owner's decision; until then the contract says
  what the Hub records.
