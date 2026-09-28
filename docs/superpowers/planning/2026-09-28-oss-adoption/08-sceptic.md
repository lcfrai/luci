# 08 — The sceptic: what the adoption research got wrong

**Date:** 2026-09-28. **Target:** `00-summary.md`, and the seven domain and seven licence files
beside it. **Read-only run:** no source changed, no test run, nothing started.

**Method, and it matters.** The summary says its ground truth is *"the task tables in the seven live
plans"*. Those plans are **not on `feat/phase-2-cockpit`** — `docs/superpowers/plans/` on this
branch holds three files. They live on the feature-branch worktrees under `.claude/worktrees/`
(`team`, `work`, `phase-5`, `hub`, `env`, `phase-3`, `phase-6`). I read the **individual part
files** the summary cites by number, not the summary tables. That is where most of what follows
came from, and it is the check the summary did not make for its own top tier.

---

## The verdict in one paragraph

The **domain and licence research is good** — better than the summary it produced. The licence
auditors caught things the decision document then lost. The failure is concentrated in
`00-summary.md` itself, in three places: **Tier 1 is not safe** (three of its six items are
neutral-to-negative once you read the code or the registry), **the two largest recommendations rest
on an effort estimate the plans contradict by an order of magnitude**, and **the summary dropped
the single most important governance warning its own licence auditor wrote** — about NATS, its
biggest saving. Nothing here is a reason to abandon the exercise. Two items are genuinely strong
and should be acted on this week. The rest needs the numbers redone against the part files.

The verdict line is at the foot of this document.

---

## 1. Overstated savings

### 1.1 Tier 1 is billed "cheap, certain, no architecture change". Three of six are not.

Tier 1 is the part of the report a reader is most likely to act on without checking, because the
document tells them to act *"regardless of any other decision"*. That is exactly where I found the
weakest claims.

#### #5 `@floating-ui/react` — the "only genuine deletion of existing code" is about fifteen lines

The summary calls this *"the only genuine deletion of existing code in the whole report"*.

`desktop/src/components/common/Popover.tsx` is **55 lines in total**. The viewport-edge geometry is
one `useLayoutEffect` — a `Math.max`/`Math.min` clamp on `left` and a single below-or-above flip on
`top`. Call it **fifteen lines**. `desktop/src/components/menu/ContextMenu.tsx` does not have its
own geometry at all; line 75 records that it *"anchors to a POINT, so the rect is zero-sized.
Popover's clamp then does…"*. So the deletion is fifteen lines from one file, not two files' worth.

Against that: `@floating-ui/react@0.27.20` (MIT, verified at `registry.npmjs.org/@floating-ui/react`
on 2026-09-28) pulls `@floating-ui/react-dom`, `@floating-ui/utils`, **plus `tabbable` and
`aria-hidden`** — two packages the summary's "all five packages in its shipped tree" does not
enumerate. Both are MIT, so the verdict holds; the *count* does not, and the summary's own §6 rule
is "check the dependency's own dependencies where it matters".

And there is a test cost the summary does not see. The Popover comment records a deliberate
decision: *"jsdom reports every rect as zero, which is fine: the clamp is a no-op there and the
tests assert content, not geometry."* floating-ui's `autoUpdate` positions **after** paint and needs
`ResizeObserver` shimming under jsdom; adopting it makes a currently-inert code path into one the
tests must mock. `AGENTIC-UX.md` §10 rule 7 — which `ContextMenu.tsx:87` cites by name — bans layout
shift, and post-paint repositioning is the shape of that bug.

**Net: five to seven new packages and a test rewrite, to delete fifteen lines.** This is not Tier 1.
It is not anything. Drop it.

#### #1 The six `@xterm/*` addons — the report's own freshness bar excludes them

Billed *"1–2 days… No new vendor — same monorepo we already depend on."* From
`registry.npmjs.org`, read 2026-09-28:

| Package | Latest | Published | `peerDependencies` |
|---|---|---|---|
| `@xterm/addon-webgl` | **0.19.0** | **2024-04-22** | `@xterm/xterm: ^5.0.0` |
| `@xterm/addon-fit` | 0.11.0 | 2024-04 | `@xterm/xterm: ^5.0.0` |

`desktop/package.json` declares **`"@xterm/xterm": "^6.0.0"`** and `"@xterm/addon-fit": "^0.11.0"`.

Two things follow. First, the repo is **already** running an addon outside its declared peer range,
so this will not block an install — I checked, and I am not claiming it will. Second, and this is
the real point: the addons' last publish is **about twenty-nine months ago**, and they are pinned to
the previous major of the core. The summary rejected `cmdk` in part because *"last release
2025-03-14, 18 months"*, and its own §6 mitigation is *"prefer projects with a release in the last
12 months"*. **The same bar, applied consistently, excludes the report's number-one item at 29
months — eleven months staler than the thing it rejected.**

That does not make them unusable. It makes "cheap, certain, no architecture change" wrong. Adding
five addons across an unreleased major boundary on the renderer path is a **compatibility spike**,
not a one-day drop-in, and the `webgl` one has a second problem (§2.1).

#### #6 `virtua` / TanStack Virtual — this one survives, and it is one of the two I endorse

Verified against the tree. `docs/DESIGN-SYSTEM.md:90` — *"Virtualise any list over 50 rows"*.
`:65` — *"Log panes batch appends at ~10Hz and never scroll-jump when the user has scrolled up."*
A search of `desktop/src/` for `scrollTop`, `stickToBottom` or `atBottom` returns **nothing**. The
gap is real, both rules are binding, and neither library brings a look. Keep it.

### 1.2 Phase 5: the two paused tasks are 3 hours and 2.5 hours

This is the clearest arithmetic error in the report, and it is the one that has already cost a
running build.

The summary pauses phase 5 tasks 25 and 14, and puts "answer D-OSS-1/2/3, then resume" at order
position 7 with **2–3 days** of spike attached. Read the part files:

- `.../2026-09-23-phase-5-design-canvas-parts/task-25.md` header: **"Hours: 3"**.
  Title: *"The editor: viewport, selection, marquee, move and resize, snapping, canvas keys."*
- `.../task-14.md` header: **"Hours: 2.5"**. Title: *"The MCP protocol layer."*

The summary quotes `02-canvas.md` calling task 25 *"the classic two weeks that became six"*. The
plan's own estimate for that task is **three focused hours**. The summary attaches **~2 days** of
integration cost to the Moveable + Selecto + `@use-gesture` + `rbush` bundle that replaces it. On
the document's own numbers that is **a net loss of about a day and a half**, plus four new
dependencies, plus a paused build — to avoid three hours of work.

One of those two numbers is wrong. Neither was checked against the other. Until someone reconciles
them, the phase 5 pause is unjustified and should be lifted.

### 1.3 Phase H: three of the six adoptions target tasks that are half-hour or already built

| Summary claim | What the part file says |
|---|---|
| #15 `go-oidc` + `x/oauth2`: *"Phase H task 10 largely **removed**"* | `task-10.md`: **"About 0.5 hours."** And it is not even unbuilt — *"Phase 8 task 16 **BUILT** it as `control-plane/internal/entra` on `feat/env` (`669204b`..`1772f25`)"*. I read it: eight files, `entra.go:2` says **"Standard library only: RS256 over `crypto/rsa`, JWKS over `net/http`"**, with `entra_test.go` asserting tenant-confusion rejection. **This recommendation proposes replacing working, reviewed, merged, dependency-free security code to save half an hour.** |
| #17 `goose`: *"Phase H task 06 largely **removed**"* | `task-06.md`: **"About 3 hours"**, and the runner is only part of it — it is also `roles.sql`, the integration harness every later test uses, and pinning `pgx`. The runner's defining requirement is ADR-H1 §5.6: each migration *"recorded with the SHA-256 of its text so an edited, already-applied migration is refused"*. **`goose` records a version, not a checksum.** The requirement that makes the task worth writing is the part `goose` does not do. |
| #18 `go-keyring`: *"**+1 new task**: the 2560-byte Credential Manager limit"* | `task-28.md` already specifies *"**one chunking rule** for tokens larger than a credential blob"*. The plan solved this before the report added a task for it. |

The summary applied exactly the right test to `cmdk` and `tinykeys` — *already built and under
test, do not replace* — and then failed to apply it to `internal/entra`. Its own §7 explains why:
*"the UI researcher read the tree"*. The Hub researcher read `feat/hub`, not `feat/env`.

### 1.4 The two "largest savings" rest on a plan-edit estimate the plans contradict

§5 calls the Phase T fold-in *"the largest single saving in the report"* and prices it at **1–2 days
of plan edit**, on the grounds that it is *"only available while the plan is still a document"* and
must land *"before the part files freeze"*. Phase W gets the same framing.

The part files are not unfrozen drafts. Counted on the worktrees today:

| Plan | Part files | Cross-file citations of the form `task-NN.md:LINE` |
|---|---|---|
| Phase T | 69 | **1,009**, present in **65 of 69 files** |
| Phase W | 187 | **1,678** |

`task-19.md` is the shape of the problem. It *"produces `control.Dispatcher`, `control.Handler`,
`control.Caller` and `control.Streams`, which tasks 22–31 and 38 are **already written
against**"* — and it proves it by citing `task-22.md:49-52`, `task-23.md:63`, `task-26.md:102`,
`:501-503`, `:650-652`, `:810-812`. Swapping the transport under task 19 invalidates those Go type
names in ten downstream files, and every line citation into the sixteen edited files shifts.

**1–2 days is not an estimate of that work; it is the absence of one.** The summary marks its
estimates unmeasured in §7 and then uses them as the ordering key in §5. Re-price both fold-ins
before either is scheduled, or the "act while it is cheap" argument inverts: it is already not cheap.

---

## 2. Architecture collisions

### 2.1 WebGL: Columns' nine panes and Canvas' ten tiles is the case where per-terminal WebGL breaks

The summary's headline argument for `@xterm/addon-webgl` is *"the renderer answer to Columns' 9
panes and Canvas' 10 tiles, each an independent live terminal"*.

**Chromium caps live WebGL contexts at about 16 per page and silently evicts the oldest when you
exceed it** — the console says *"Too many active WebGL contexts. Oldest context will be lost."*
(Chromium issue 40543269 / 771792, "Make WebGL context limit configurable"; the same behaviour is
reported against OpenLayers #16118 and PixiJS #8215; Firefox's limit is 8, Bugzilla 790138.)
WebView2 is Chromium, so the desktop inherits it.

One WebGL context per terminal means the Canvas layout at ten tiles sits **five contexts under a
hard cap** that a fork, a resume or a second window crosses. The failure is not a crash — it is the
oldest terminal going blank while the newest keeps painting. That is the worst possible failure mode
for a cockpit whose entire premise is watching many agents at once.

The summary lists *"the WebGL context-loss fallback, and making it an acceptance test"* as the
leftover. It is not a leftover. It is the design question: per-pane WebGL with an eviction policy,
or one shared context drawn into the other canvases, or DOM/canvas rendering for inactive panes.
**Move #1 from "cheap, certain, no architecture change" to "needs a decision", and put the context
count in the acceptance test.** The other five addons (`serialize`, `search`, `unicode11`,
`web-links`, `clipboard`) carry none of this and stay cheap.

### 2.2 The MCP Go SDK forces a Go-toolchain bump on the whole workspace

`modelcontextprotocol/go-sdk@main/go.mod` declares **`go 1.25.0`** (read 2026-09-28).

In this repo: `go.work` → `go 1.23`; `control-plane/go.mod` → `go 1.23`; `sandboxd/go.mod` →
`go 1.23`; `canvas/go.mod` (on `feat/phase-5-canvas`) → `go 1.23`. A `go` directive is a hard floor,
and the workspace directive must be at least the maximum of its members — `phase-5/go.work` uses
`./canvas`, `./control-plane`, `./sandboxd` and `./tools/licencegate`. So adopting the SDK inside
`canvas/` drags **every module in the workspace, the CI image and the sandbox image** to Go 1.25.

Phase 5's plan already saw this. `task-14.md` records **plan decision P5-A**: five JSON-RPC methods
over Streamable HTTP POST, *"small enough to own, and owning it keeps the authority free of
dependencies and of **the SDK's Go-version question**"* — and it already built the escape hatch:
*"swapping in `modelcontextprotocol/go-sdk` later touches this package alone"*.

**The summary recommends reversing a recorded plan decision without citing it, and without the fact
that decision turned on.** The SDK also brings `golang-jwt/jwt/v5`, `google/jsonschema-go`,
`segmentio/encoding`, `yosida95/uritemplate/v3`, `x/oauth2`, `x/time` and `x/tools` into a package
whose stated purpose is to have no dependencies. For 2.5 hours. Reject #14 as written; keep P5-A.

### 2.3 Phase T is Postgres-and-Hub shaped, not bus shaped — and NATS would be a third policy engine

The five-subsystem NATS claim does not survive the part files.

- `task-26.md` (the turn queue): an Orchestrator *"is a **queue of typed events in Fleet's
  Postgres**, drained one batch at a time"*.
- `task-23.md` (leases) depends on task 10's `Store`/`Tx` and task 11's `outbox.Enqueue` — there is
  **already a transactional outbox**, so a Hub record and its emission commit together.
- `task-28.md`: Orchestrator messages are *"typed messages into threads that **Fleet holds and the
  Hub records**"*.
- `task-26.md` (Hub, separate plan): *"The Hub **pushes nothing in v1**; the control plane polls one
  ordered feed per user every 5 s."*

Adding NATS does not delete Postgres from any of that — the audit trail and the provenance thread
have to be in the Hub's database. It adds a **second durable store that must stay consistent with
the first**, and loses the one property the outbox exists to provide. That is a cost, not a saving,
and it is the summary's §6 "component dictates our data model" risk landing on its own top pick.

The lease claim is the weakest. `task-23.md` is *"Lease engine: intent, right of way, breadth cap,
request and release"*, and leases are **advisory** — *"Fleet never refuses a write"*. `task-35.md`
adds holds inferred from **observed edits**, bulk, and per-path expiry. NATS KV gives atomic
create-or-fail with a TTL. It gives nothing for right of way, nothing for the breadth cap, nothing
for inferred holds. The saving is one primitive inside one of two tasks.

**And the authorisation argument inverts.** The summary wants `OD-16` written *"as NATS subject
permissions, so the security review has something enforceable"*. But `task-43.md` already makes team
authorisation *"**one table and one test**"* in the Hub — and the summary separately recommends
**Cedar** for tasks 06, 07, 43 and 44. Taken together the report proposes **three** authorisation
surfaces for Phase T: the Hub's role-to-verb table (designed), Cedar (#20), and NATS subject
permissions (#7). Three policy engines is a security regression, not an enforceable rule. Pick one.

### 2.4 centrifuge is a third WebSocket stack in a desktop that already has two

#21 puts centrifuge on Phase T tasks 52–53 at **4–6 days** saved. Those tasks are the team read
model and the team strip — a React surface over data the Hub already publishes through task 08's
feed kinds and the control plane's 5-second poll (Hub `task-26.md`). The desktop already carries
`desktop/src/api/attachRegistry.ts` (**683 lines**) for the PTY relay, with reconnect, backpressure
and a session model that `@xterm/addon-attach` was correctly rejected for lacking.

centrifuge brings its own client protocol and its own JS SDK. Adopting it means the cockpit speaks
three real-time protocols: the PTY relay, the Hub feed, and centrifuge. Before taking it, someone
has to say which of the other two it **replaces** — and if the answer is neither, the 4–6 days are
spent buying a third.

### 2.5 `agent-sandbox`: the strongest architecture case in the report, and "no architecture change" is still wrong

Credit where due. Phase 8 `task-12.md` says warm capacity is node-warm-first because *"a pod-level
pool (pre-booted pods claimed later) **is not expressible today**"*. `SandboxWarmPool` makes it
expressible. That is a genuine new capability, not just a saving, and the summary is right to want
the spike.

But "sits **behind the existing `Backend` interface** with no architecture change" is not right.
`agent-sandbox` is a CRD plus a controller: the control plane stops creating Pods and starts
creating `Sandbox` objects. Phase 8 `task-14.md` hardens the pod `sandbox.Manager` builds
(`control-plane/internal/sandbox/manager.go:139-210`) and `task-15.md` is seam S8.3 — the worktree
host mount that `katabe.Create` deliberately refuses (`katabe/kata.go:84-89`). Every one of those
controls has to be re-expressed through someone else's pod template, and reaping/TTL moves from our
reconciler to theirs. Spike it as the summary says — but scope the spike to **tasks 14 and 15's
controls surviving the CR**, not only to warm-pool latency.

---

## 3. Licence — I re-checked the three the summary leans on hardest

All three checks are mine, fetched from the primary source on **2026-09-28**, not read from the
summary or from a badge.

| # | What I fetched | Result | Verdict on the summary |
|---|---|---|---|
| **#2 Tauri plugins** | `raw.githubusercontent.com/tauri-apps/plugins-workspace/v2/Cargo.toml` and `.../v2/plugins/deep-link/Cargo.toml` | The plugin declares `license = { workspace = true }`; the workspace root declares **`license = "Apache-2.0 OR MIT"`**, `rust-version = "1.90"`, `edition = "2024"` | **CONFIRMED.** Note the per-plugin file inherits — anyone re-checking one plugin in isolation sees no licence at all. Also note **MSRV 1.90** as a build-agent floor |
| **#24 OpenTofu / azurerm** | `raw.githubusercontent.com/hashicorp/terraform-provider-azurerm/main/LICENSE` | **Mozilla Public License 2.0**, `Copyright (c) 2017 HashiCorp, Inc.`; no Change Date, no Change License, no BUSL heading | **CONFIRMED.** The provider is still MPL even though HashiCorp's own products went BUSL. This is the report's single most valuable licence finding and it holds |
| **#1 xterm addons** | `registry.npmjs.org/@xterm/addon-webgl`, `/@xterm/addon-fit` | `"license": "MIT"` on both | **CONFIRMED on licence.** The problem with #1 is freshness and the WebGL cap, not the licence |

### 3.1 The one the summary lost: NATS

`licence-agent-fleet.md` did its job. Row 2 carries the verdict *"**yes** — with a governance note
**the researcher omitted**"*, and §2.2 spells it out: *"Synadia announced it would move
`nats-server` to the Business Source License; the CNCF filed a USPTO petition over the NATS
trademark; the dispute was settled on 2025-05-01 with Synadia agreeing **not** to fork under BUSL,
to keep `nats-server` Apache-2.0, and to transfer the trademarks to the Linux Foundation."* It even
gives the operating rule: *"pin NATS and re-read its `LICENSE` on every major upgrade."*

I confirmed the history independently (CNCF announcement 2025-05-01, *"CNCF and Synadia Align on
Securing the Future of the NATS.io Project"*; The Register 2025-05-02; `nats-io/nats-server` issue
#6832). The settlement also records that **if Synadia forks for a proprietary offering it must do so
under a new name** — which is the residual risk: the dominant contributor retains an incentive to
put its effort somewhere other than the CNCF tree.

**`00-summary.md` carries none of it.** Row #7's licence cell reads *"Apache-2.0 (server,
`nats.go`, `nkeys`)"*, full stop. §6's relicence-risk row names HashiCorp, Meilisearch, Langfuse and
the MCP Go SDK — and **not NATS**, which is the same row's largest recommendation. On a program
whose owner has already had to withdraw a locked decision over a licence, dropping the one live
near-miss from the decision document is the most serious editorial failure in the report.

**Fix:** restore §2.2 verbatim into the summary row and into §6, with the pin rule.

### 3.2 Unresolved, and it should not be ranked at all: TanStack DB

The summary's own #9 says `@tanstack/db-ivm` is *"a fork of Apache-2.0 `@electric-sql/d2ts`
declared MIT **with no licence file**"*, and makes "resolve the question" a decision to take later.
Repo facts I confirmed: `TanStack/db` is MIT, created 2025-03-11, 3,921 stars, 167 open issues, last
push 2026-09-28 (`api.github.com/repos/TanStack/db`, read 2026-09-28) — healthy, and beside the
point.

**The claimed saving is ~2.5 days.** Spending an unresolved-provenance dependency at the centre of
the desktop's data model to save two and a half days, on this program, is the wrong side of the
trade. Demote #9 to *do not adopt until `db-ivm` ships a LICENSE file*, and write the `replica/`
interface as planned. If the file appears, adopting later is the swap the interface was for.

---

## 4. Maintenance reality — the pattern the report never names

§6's treadmill row counts *names* and checks *last release*. It never counts **maintainers**. Run
that test across the list and a pattern appears that changes several rankings:

| Component | Where it sits | Bus factor |
|---|---|---|
| `virtua` | Tier 1 #6 — the log pane | effectively one (inokawa) |
| Moveable, Selecto, `@use-gesture` | #13 — four of the canvas deps | Moveable + Selecto are one maintainer (daybrush) |
| centrifuge | #21 — the whole team strip transport | effectively one (FZambia) |
| uFuzzy | #26 | one (leeoniya) |
| `participle` | #25 — the saved-view grammar | effectively one (alecthomas) |
| `bluekeyes/go-gitdiff` | Tier 1 #3 | one, low activity |
| `ygo` | phase 5 critical path if Yjs wins | ~55 stars — the summary does flag this one |

None of these is disqualifying on its own. Together they say the tier-2 list is **mostly single-
maintainer projects**, and the report's mitigation ("prefer a release in the last 12 months") does
not detect that at all. Add "how many people can cut a release" to the gate, and prefer TanStack
Virtual over `virtua` where the two are interchangeable, on that ground alone.

**Churn, the other direction.** `06-platform.md` records `agent-sandbox` at **v1.0.2 (2026-09-11),
v1.0.3 (2026-09-17), v1.0.4 (2026-09-24)** — three releases in thirteen days, on a project created
2025-08-12 with 206 open issues. The domain file calls that *"activity and also churn"* and gives a
pin. The summary's row #8 mentions neither the pin nor the cadence.

**The good news the summary buries:** `06-platform.md` produced actual pin targets — LiteLLM
`v1.102.1` (2026-09-23, with upstream recommending a commit-hash pin over a tag), `agent-sandbox`
`v1.0.4`. §6 says "pin by version and digest" and then names no pins. Lift the pin table into the
summary; it is the difference between a rule and a practice.

---

## 5. What was missed

Two kinds. Most of what looks missing was **found by a domain researcher and lost by the summary** —
that is a five-minute fix and worth more than new research. A smaller set was never considered.

### 5.1 Found, then dropped between the domain file and the decision

| Item | Where it was found | Why losing it matters |
|---|---|---|
| **OpenTelemetry GenAI semantic conventions** (Apache-2.0) — *"adopt the vocabulary"* | `06-platform.md:471-487` | This is the one that hurts. The researcher's point is a **provenance** point, not a telemetry one: LiteLLM's Prometheus labels carry key, user, team, model — *"what they do not carry is **project**, **brief**, **lane** and **task** — the provenance thread. Put those on the virtual key's `metadata` at mint time, emit them as OTel GenAI attributes… and the cockpit can answer 'what did this brief cost' rather than only 'what did this key cost'. That is the difference between telemetry and provenance, and **the spine rule makes it non-optional**."* The summary's phase 6 section says *"Nothing new — this research ADDS work"* and lists two new tasks, neither of them this one. **The report's own first-order test — does the thread survive — is applied to everything except the one recommendation that serves it.** |
| **`sqlc-dev/sqlc`** (MIT) — *"adopt, optional"* for the ledger and revision-log scan code | `05-hub-identity.md:77`, `licence-hub-identity.md:289` | Hand-written `rows.Scan` across the ledger is exactly the commodity this exercise exists to find, and it is compile-time-checked SQL, which suits an append-only ledger |
| **`Nerzal/gocloak`** (Apache-2.0) — *"adopt if we automate realm config"* | `05-hub-identity.md:71` region | `OD-30` puts a Keycloak realm on the critical path and phase 8 has to stand one up. Realm/client/mapper configuration is either automated or it is a runbook |
| **`secure-systems-lab/dsse`** as a format, and **`spdx/tools-golang`**'s `Apache-2.0 OR GPL-2.0-or-later` dual trap | `05-hub-identity.md` rows 24, 30 | The summary carries CycloneDX and the in-toto DSSE envelope but not the dual-licence warning that sits one row away |
| **OpenBao as an adoption**, not only as "the thing that is not Vault" | §3a | It appears only in the do-not-adopt table's replacement column. It is a phase 8 runtime component with an MPL obligation and nobody owns it |

### 5.2 Never considered anywhere in the seven domain files

I grepped all fourteen files for each of these. Zero hits.

1. **A typed API contract across the Go/TypeScript boundary** — `buf` + **ConnectRPC** (Apache-2.0),
   or `oapi-codegen` (Apache-2.0) over OpenAPI. This is the biggest genuine omission. The monorepo
   has a Go control plane, a Go Hub, a Go Fleet and a TypeScript desktop, and Hub `task-33.md`
   describes a hand-written `hubclient` with hand-written retry, idempotency and refresh rules plus
   a hand-written fake and a hand-written conformance suite (task 08). Generated clients, a
   generated fake and a wire-compatibility check are the single most boring, most commoditised,
   most repeated-per-phase thing in the program, and no researcher looked at it.
2. **`testcontainers-go`** (MIT). Hub `task-06.md` builds *"the integration harness every later
   integration test uses"* by hand, against Docker Postgres. This is a solved problem, and the same
   harness will be wanted again for NATS, LiteLLM and MinIO-alikes.
3. **`sigstore/cosign`** (Apache-2.0) for signing. The report adopts in-toto and SLSA as *formats*
   and DSSE as an *envelope*, and then has nothing that **signs**. `sigstore/rekor` was assessed and
   rejected for the Hub — correctly, it is a public log — but cosign is a signing tool, not a log,
   and it is what phase 9's ISM evidence and the sandbox-image bump pipeline (`OD-20`) need. Note
   `06-platform.md:1163` records that LiteLLM upstream already signs its images with cosign, so we
   will be verifying cosign signatures whether or not we produce them.
4. **Windows ConPTY for local mode.** `creack/pty` is mentioned once in passing
   (`07-ui-primitives.md:210`, `licence-platform.md:154`) and never assessed. `OD-20` keeps local
   mode; a Windows host PTY is not `creack/pty`, and the answer is a different library.
5. **`prefers-reduced-motion`, `forced-colors` and the accessibility floor outside phase W.** #10
   scopes React Aria to Phase W tasks 34/46–48/66. `DESIGN-SYSTEM.md:90` requires
   `prefers-reduced-motion` variants **on everything**, and phase 2's cockpit is already built
   without a primitives layer. Either the decision is program-wide or W1 gets an accessibility floor
   the rest of the product does not have.

---

## 6. Act on these two this week. Nothing else.

Both are chosen on the same test: **the cost of being wrong is near zero, and the cost of delay is
not.** Neither needs a spike, a new runtime component, or a plan edit that touches a cross-cited
part file.

### ONE — Decide OpenTofu, and record it before a line of phase 8 Terraform exists

Summary order item 3, and the only item where I verified the underlying licence myself and found the
report exactly right. HashiCorp Terraform is BUSL-1.1; `terraform-provider-azurerm/main/LICENSE` is
**MPL-2.0** (fetched 2026-09-28, quoted in §3). So the engine must change and the providers need
not — which is why this is hours of work now and a rewrite of every pipeline later. Phase 8's
Terraform is *written and planned, never applied* (`OD-5`), so the decision is free today and the
cost curve is vertical from the first `terraform` invocation in a pipeline. Same ruling for
**OpenBao** in place of Vault, and give OpenBao an owner while you are there (§5.1).

**Effort: hours. Blocking: phase 8 tasks 01–10, 28, 30.**

### TWO — Write phase 6's AU-residency `count_tokens` canary, then rebuild the LiteLLM image

Summary order item 2, and the only item in the report whose absence is a **compliance finding**
rather than a schedule item. `OD-19` and `OD-24` promise a project classified "AU data only" reaches
Bedrock in Sydney and nothing else. LiteLLM ignoring a custom `api_base` on
`/v1/messages/count_tokens` — which Claude Code calls on the hot path — makes that promise silently
false, and the fix is a test that fails today, before any phase 6 code exists to argue with.
Separately, the stock `berriai/litellm` image copies `enterprise/` (whose licence forbids
distribution) into an image we would publish into a customer's own tenancy; rebuild from the MIT
tree with a CI layer check. Pin **`v1.102.1`** by commit hash, per `06-platform.md`'s pin table.

**Effort: about a day. Blocking: phase 6 task 19 and the first customer-hosted deploy.**

### Deliberately not this week

- **Do not pause phase 5.** The two paused tasks are 3 and 2.5 hours (§1.2), the MCP swap reverses a
  recorded plan decision and forces the workspace to Go 1.25 (§2.2), and tasks 05 and 06 — two of
  the five the Yjs fork would rewrite — **landed today** (`568c8d1a`…`24e42f24`, 2026-09-28), so the
  D-OSS-2 window the summary describes as open has been closing while the report was written.
- **Do not fold NATS / centrifuge / River / Cedar / CloudEvents into phase T yet.** Re-price it
  first: 1,009 line-level cross-citations across 65 of 69 part files is not a 1–2 day edit (§1.4),
  and the proposal currently lands three authorisation surfaces on one phase (§2.3).
- **Do not take `@floating-ui/react` or `go-oidc` at all** (§1.1, §1.3). Both are net-negative.
- **The six xterm addons are still worth having** — but take the five that are not `webgl` as the
  cheap win, and treat `webgl` as its own decision with the 16-context cap in the acceptance test
  (§2.1).

---

## 7. What I could not verify, and where I could be wrong

- **npm registry facts came through `WebFetch`**, which summarises with a small model. I calibrated
  it: it reported `@xterm/addon-fit@0.11.0`, peer `^5.0.0`, April 2024 — which matches
  `desktop/package.json` exactly, so the data is real. It is still second-hand, and the addon
  versions must be re-read directly at pin time.
- **Bus-factor figures in §4 are my judgement from maintainer identity**, not commit-graph analysis.
  Treat them as a prompt to check, not as measurements.
- **I did not re-audit every licence** — only the three the summary leans on hardest, plus NATS.
  The other domain licence files looked careful and I found no error in the ones I sampled.
- **The effort numbers I criticise are the plans' own** and are equally unmeasured. My argument is
  not that three hours is correct; it is that **two documents in the same directory differ by a
  factor of thirty on the same task and nobody reconciled them**.
- **`~/Projects/Paperdesign`, `kgoedecke/doop` and every AGPL/GPL canvas were not opened, not read,
  not searched.** Phase 5 stays a clean room.
- **Tooling note:** `~/.claude/tools/claudezsh` took over 120 seconds per invocation on this machine
  today and had to be backgrounded; the reads above used allow-listed commands and the dedicated
  file tools instead. Worth someone looking at — an audit wrapper that slow gets dropped.

---

*Prepared 2026-09-28 as the adversarial review of `00-summary.md`. Findings only — no source was
changed, no test was run, nothing was started.*

**Recommendation quality: Needs fixes**
