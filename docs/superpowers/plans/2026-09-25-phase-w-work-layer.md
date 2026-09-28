# Phase W: The Work Layer (Linear parity-plus)

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking. Each task's full text is in `2026-09-25-phase-w-work-layer-parts/task-NN.md`; this file is the skeleton those parts are written from.

**Goal:** A team moves its backlog into PanaceaApp and never goes back. Issues are **Hub objects on
the provenance thread**: each one points at the brief criteria it serves, so the chain reads
brief → criterion → design frame → issue → session → land → evidence → release, in one place.
Everything Linear does that we price as MATCH, EXCEED or PARTIAL is built (W0 to W5, `OD-26`), in
the design-language uplift (`02`, `05` §4.4) with Commit Mono and Segoe UI Variable (`OD-28`). An
agent takes any row with `Shift+R`, shows a first sign of work within 10 seconds, and the issue
turns `done` by itself only when the platform **observed** its own evidence at its own landed
commit. A hand-set done reads *done, not proven*, and sync can never write it. Progress is proof
per criterion and plain counts per group; no percentage, bar, ETA or "N of M" anywhere (`OD-27`).
From W3, per-user Orchestrators negotiate on issues across owners, and nobody steers another
person's agent (`OD-14` to `OD-17`).

**Delivery shape.** Phase W runs **alongside** team mode on its own streams (`OD-25`). W1 is the
first adoptable slice and its demo runs in **local mode** first (`OD-20`), then moves to the cloud
when team mode's gateway stream (phase 6 stream A) and environment stream (phase 8 stream C) land.
Team mode's critical path is untouched: no Phase W task edits a team-mode task, and Phase W's Hub
deltas are new Hub tasks numbered after Hub task 43, with the same sync-merge rule team mode uses
(Global constraints, rule H1).

**The demo W1 must pass (05 §5.3).** Import a Linear backlog or a CSV; it lands in Triage as *not
tied · imported*. Press `T` on EP-142 and attach the criterion it serves from a text-match
suggestion. Press `Shift+R`; within 10 seconds a local-mode agent is working and its one live
line is on the row. Teammates comment. It lands; the row flashes `landed` in place and does not
move. The platform watched the resume test pass at the landed commit, so EP-142 turns `done` with
no click. The last frame is the Desk: *"Nothing needs you. EP-142 landed and proven at 14:02."*
W1b adds *"EP-142 shipped to production at 14:02."*

**Tech Stack:** Go 1.23 (the Hub service in the `feat/hub` contract; `control-plane` and the
phase 3 and phase 4 packages), React 19 + TypeScript 6 + Vite 8, `zustand` 5, Vitest 5 +
`@testing-library/react` 16 + jsdom 30, Tauri 2. No new runtime dependency without a licence line
in the task (MIT, Apache-2.0, BSD, ISC or OFL only; `OD-11`).

**Spec:** `docs/superpowers/planning/2026-09-25-linear-parity/05-design.md` (revision 2) and
`00-summary.md`; the parity matrix `01-linear-feature-inventory.md` (257 items); the design craft
`02-linear-design-language.md`; the critiques `06` and `07`; `docs/DESIGN-SYSTEM.md`;
`docs/AGENTIC-UX.md` with the four `Q13` amendments (task 01); `prototype/work-layer.html`.

---

## Architecture

No new service. The work layer is **a new Hub aggregate** (`Work`), **a relay and a binding** in
the local control plane, **small deltas to phases 3 and 4** that put the issue on the thread, and
**a new cockpit lens plus the Desk frame** on the desktop. Fleet (Phase T) joins in W3 only.

```
desktop (the portal)                 control plane (loopback, per user)       the Hub (Phase H + Work)
┌───────────────────────────┐ REST  ┌───────────────────────────────────┐ REST ┌────────────────────────┐
│ Work lens · Plan · issue  │ ────► │ /v1/hub/work/* relay (contract    │ ───► │ issues is-, revisions  │
│ view · Desk frame         │       │   §4.3 relay rule, unchanged)     │      │ comments ic-, features │
│ replica: in memory only,  │ ◄──── │ Panacea-Hub-Cursor on /v1/sessions│ ◄─── │ cycles, views, … lp-   │
│   cursor on /v1/changes   │       │ session ⇄ issue binding (IssueRef)│      │ one event per write    │
│ optimistic write + Undo   │       │ Materialiser: brief key + issue   │      │ authz rows, erase      │
└───────────────────────────┘       │ phase 3: Panacea-Issue trailer,   │      │ janitor: cycles, loops │
                                    │   land → landed, done evaluator   │      └───────────┬────────────┘
                                    └───────────────┬───────────────────┘                  │ W3: Fleet reads
                                                    │ local mode (OD-20) now;              ▼ issues, writes
                                                    ▼ cloud when phase 6 A + phase 8 C land   notes as owner
                                              sandbox · agent CLI (never parsed, OD-8)
```

Seven rules shape every task. A task that breaks one is wrong however good it is alone.

1. **The issue is on the thread.** An issue head `is-` is mutable; an issue revision
   `issue:is-…@r` is immutable and digested, and its `serves [{briefKey, criterionId}]` is what
   joins it to Define, as its `implements [design:…]` joins it to Design (81b, 96b). `EP-142` is a display alias from `project.key` and a number, **never a
   key**, because an admin can change the project key (05 §3.6).
2. **Done means proven.** `started`, `in_review`, `landed` and `done` are **facts**, written as the
   person whose act caused them. `done` needs the issue's **own** evidence, at or after its **own**
   land commit, for each criterion it serves; observed evidence (a watched test run's exit code at
   the landed commit) closes it with no click. A hand-set `done` needs a one-line reason and reads
   *done, not proven*. **No sync or integration token may write a fact state or `done`**, and an
   authorisation row plus a test pins it (05 §3.3, §3.6).
3. **Three homes, never mixed** (05 §3.1). A decision or accountability fact is a Hub record; a fact
   about now (who is on an issue, which files, the live line) is Fleet or control-plane state,
   derived and never stored on the issue; a preference is the desktop's. **The desktop keeps no Hub
   data on disk** (`ADR-H1:467-474`), so the replica is in memory (Q5).
4. **Rows never move under the reader.** Arrivals and moves are held (`arrivals.hold` on `N`,
   `moves.hold`), a landed row flashes in place, and a row carries **one** agent line, the reader's
   own; a teammate shows as a person and the files they touch, never as dots (05 §4.4).
5. **Proof per criterion, no predictions** (`OD-27`). Surfaces show which criteria are proven and
   plain counts per group. No percentage, progress bar, ETA or "N of M" completion figure appears
   anywhere; every surface task's tests call the shared `expectNoPrediction` helper (task 20).
6. **Agent CLIs are never modified, wrapped or parsed** (`OD-8`). The live line and every status
   come from mechanically observed facts (session status, lane events, observed files, exit codes),
   never from PTY bytes. Issue text enters an agent's managed block only as spec-class text its
   owner adopted (Start or Delegate), after the Hub's schema and character checks; comments and
   requests are never rendered into it (05 §3.4, `Q3`).
7. **Nobody steers another person's agent** (`OD-16`). An edit to someone's in-flight issue reaches
   its owner as a Desk *inform*, never their agents; a reassign is a *decide* for the owner; a
   cross-owner relation is a proposal from the editor's own Orchestrator (W3) (05 §3.7).

### Streams, and where each slice can start

Phase W runs on its own streams beside Phase T (`OD-25`, Q11 option B). The Hub stream reviews with
team mode's stream B; the desktop streams sit beside its stream F and **share one Desk frame**
(05 §5.6): team mode adds rows to the frame task 62 builds, never a second Desk.

| Stream | Tasks | Starts after | Lands in |
|---|---|---|---|
| **WH, Hub deltas** | 02-17 (W1), then each slice's Hub tasks | Hub task 43 merged into `feat/hub`, synced here (rule H1) | `feat/work-layer`, merged into `feat/hub` by the orchestrator |
| **WP, phases 3 and 4** | 18-19, 21-25 | phase 4 tasks 06, 09-10, 16-17; phase 3 tasks 05, 07-09 | `feat/work-layer`, each file's owner phase wins (rule H2) |
| **WD1, look and Work lens** | 20, 26-45 | task 01; the look needs no Hub | `feat/work-layer` |
| **WD2, issue view, keys, replica, Desk** | 46-70 | tasks 05 and 20 (the fake is enough to start; task 16 brings the real relay) | `feat/work-layer` |
| **W1b** | 71 | task 22 and phase 7 task 19 | `feat/work-layer` |
| **W2-W5** | 72-174, with 81b and 96b in W2 | the slice before, per the table | `feat/work-layer` |
| **WL, the design's *later* rows** | 175-187 | W5's proof (task 174); nothing depends on WL | `feat/work-layer` |

---

## The decisions this plan rests on

Owner decisions are in `docs/superpowers/planning/2026-09-23-owner-decisions.md`; the newer win.

| Id | What it fixes for Phase W | Where it bites |
|---|---|---|
| `OD-25` | Built **alongside** team mode on its own streams; W1 demos in local mode first; team mode's critical path is untouched | the streams table; no task edits a Phase T task |
| `OD-26` | **Everything, W1 to W5**: every MATCH, EXCEED and PARTIAL item of the 257 is built; the 27 SKIPs stand (Q6) | the coverage map; the SKIP list |
| `OD-27` | **Proof per criterion and plain counts; no percentage, bar, ETA or "N of M"**; `AGENTIC-UX.md` gets Q13's four one-line amendments | rule 5; task 20's `expectNoPrediction`; task 01 writes the amendments |
| `OD-28` | **Commit Mono** (OFL) for every id, path and time; **Segoe UI Variable** for the sans | task 27 (Commit Mono bundled as files, its licence recorded; Segoe UI Variable is the system face, never bundled) |
| `OD-1` | The review gate is waived for phases 3, 4, 5 and 7 **only**; Phase W keeps it: the in-app prototype (task 38) is shown before the Work surfaces are sized, and `feat/work-layer` is not merged onward until the owner has looked. Q3, Q4, Q5, Q7, Q8, Q9, Q11 and Q12 take their recommended answers under it | tasks 38-39; merge policy |
| `OD-3`, `OD-5` | Shared state lives in **the Hub**, our own server; issues are Hub records from day one | stream WH |
| `OD-6` | The desktop signs in with Entra PKCE; every Hub write is as a person | authorisation rows, tasks 06-07 |
| `OD-2`, `OD-4` | Worktrees are local clones; **Azure DevOps and GitHub both**, so outside-PR linking, webhooks and sync cover both hosts | tasks in W5 |
| `OD-8` | Agent CLIs are never parsed; a harness transcript may be read only through a pinned adapter, and nothing here needs one | rule 6 |
| `OD-11` | Clean room: no Linear source, CSS, icon or copy is copied; our glyphs are drawn; every dependency is MIT, Apache-2.0, BSD, ISC or OFL | tasks 28-29 licence checks |
| `OD-13` | No Panacea source is copied; phase 7 stays behind its interface, so W1b and W4's release rows read the manifest through phase 7 | W1b, W4 |
| `OD-14`-`OD-17` | Team mode: per-user Orchestrators are the only cross-user channel; **nobody steers another's agent** | rule 7; W3 |
| `OD-19`, `OD-21` | Inference only through the LiteLLM gateway, Global or AU; W4's AI assists need phase 6 | W4 assists gate on phase 6 |
| `OD-20` | Local mode stays; W1 runs there first | task 70's demo |
| `OD-24` | A project's data classification is set at creation; AU-only projects route AU and branch names carry no title slug (Q8) | tasks 21, 24 |

The design's own questions, as answered: **Q1** → `OD-27`; **Q2** → `OD-26`; **Q3** issue text
enters the agent's context with guards (rule 6); **Q4** the Desk frame is in W1 (task 62);
**Q5** the replica is in memory (tasks 55-58); **Q6** the skips stand; **Q7** Linear's project is
our *feature*; **Q8** no title slug in AU-only branch names; **Q9** build our own, sync one way
in (W5); **Q10** → `OD-28`; **Q11** option B (`OD-25`); **Q12** cross-team in W4; **Q13** the
four amendments (task 01); **Q14** Segoe UI Variable (`OD-28`).

---

## Global constraints

**Merging and sibling files**

- **H1, the sync-merge rule for Hub deltas** (the rule team mode's Hub deltas follow, as Phase T
  words it at `2026-09-25-phase-t-team-mode.md:118-125` on `feat/team-mode`, read at `9a12da6`:
  *"A delta task starts only after the sync-merge of its sibling branch"*). The
  Hub route table says *"Nothing else is routed"* (`hub-contract.md:609`), so the work layer arrives
  as **new Hub tasks after Hub task 43, never edits to a Hub task in flight** (05 §5.5). Before a
  WH task starts, the orchestrator **sync-merges** the current `feat/hub` into `feat/work-layer`;
  the implementer never merges. A WH task is **additive only**: new files, a new `Work` interface
  embedded in `hubclient.Hub` (`hub-contract.md:1329`), new rows appended to the route table and
  the authorisation matrix, new wire types, and migrations taking the **next free number at sync
  time** (renumbered on a clash; they are append-only and applied nowhere shared until merged).
  On an add/add or edit conflict **the Hub plan's file wins whole**; this plan's change is re-applied
  on top of it and its tests re-run against it, and a failure is a finding against our change.
  The orchestrator merges finished WH tasks into `feat/hub` in task order.
- **H2, deltas to phases 3 and 4** (tasks 18-19, 21-25). Each extends a file its owner phase
  builds (`provenance/key.go`, `Session`, the Materialiser, the commit hook, land, the evaluator).
  If the owner's file has not landed on the sync base, the task **waits** (its `dependsOn` names the
  owner task); it never writes a second copy. The owner phase wins every conflict.
- **W0's two Hub-contract edits go as a proposed edit** to the `feat/hub` owner, not a direct change
  (05 §5.2), because that contract is under live edit. Task 01 writes the proposal.
- **UI branches stay unmerged until the owner has looked** (`OD-1`, `OD-12`); visual acceptance is
  the owner's eyes, never a screenshot or a self-report (`CLAUDE.md`).

**Go**

- Module path `github.com/kpmg-au-emu/panacea-app/control-plane`; the `go` directive stays
  `go 1.23` in `go.work` and every `go.mod`. Check `git diff` on them before every commit.
- **`go test ./...` is invalid at the workspace root.** Run
  `go test ./control-plane/... -race -count=1`; the Hub's integration rows run with
  `-tags integration` against `control-plane/cmd/hub/compose.yaml`.
- The Hub's boundary test (`hub-contract.md:78-82`) holds: the control plane imports only
  `hubapi` and `hubclient`, never `internal/hub/server` or `internal/hub/store`.
- **Every Hub write appends one metadata-only event in the same transaction**
  (`hub-contract.md:1131`); every new route gets its authorisation-matrix rows in the same task;
  every new object's text gets the brief's erase (org admin only; keys, digests and revision
  numbers kept, resolving `erased`).
- New ids use `ids.Wide(prefix)` (`hub-contract.md:93-100`): `is-`, `ic-`, `ft-`, `ml-`, `cy-`,
  `in-`, `rq-`, `lb-`, `tp-`, `vw-`, `lp-`, `df-` (05 §3.6; checked against the Hub's and TM-05's
  prefixes). `nt-` is TM-05's and is reused for status updates.

**Desktop**

- Read `docs/DESIGN-SYSTEM.md`, `docs/AGENTIC-UX.md` and `prototype/work-layer.html` before any
  UI task. No indigo or violet, no gradient hero, no blanket glass; elevation is a 1px ring plus
  layered shadow; radius by element size; motion only on `transform`/`opacity`, never
  `transition: all`; **one heartbeat** (`desktop/src/hooks/heartbeat.ts`), never a spinner per row;
  live numbers `tabular-nums`.
- **Faces** (`OD-28`): Commit Mono for every id, key, path, time, cost and duration, at `--t-fact`
  12px (11px mono only for `kbd` hints); Segoe UI Variable for the sans. `--ink-3` (3.43:1,
  `tokens.css:47`) never carries a fact. `--mono` must stay a real monospace for xterm.js
  (`tokens.css:149-152`).
- **Keys** (05 §4.3): single keys and `G`/`O`/`M` sequences fire only with focus in a list, by
  extending `isCockpitChord` (`desktop/src/hooks/useKeyboardMap.ts`); agent keys test
  `event.code` (`KeyR` with `shiftKey`), never `event.key`; Linear's `Shift+D`, `Shift+E`,
  `Shift+P`, `Shift+M` and `Shift+S` do nothing; `OD-9`'s terminal toggle is respected.
- **No completion prediction** (`OD-27`): every surface task's tests call `expectNoPrediction`
  (task 20), which fails on a `%`, a `progressbar` role, an ETA or an "N of M" string.
- The desktop persists **no Hub data**: no `localStorage`, IndexedDB or file write of any Hub
  record; a test in task 55 greps for it.
- Tests: `npm --prefix desktop test` (`vitest run`) and `npm --prefix desktop run typecheck`.

**Everywhere**

- Clean room (`OD-11`): no Linear source, CSS, icon, screenshot or copy is copied; Linear facts come
  only from `01` and `02`. Every new dependency names its licence (MIT, Apache-2.0, BSD, ISC, OFL).
- No secrets in source; configuration is `PANACEA_APP_*`; the control plane stays on loopback.
- State what is unverified. Every size in this plan is an estimate; none is measured.
- Every shell command goes through `~/.claude/tools/claudezsh/claudezsh` (device rule).
- Commit messages end with `Co-Authored-By: Claude Opus 5.5 (1M context) <noreply@anthropic.com>`.

---

## File map

Every file the phase creates or modifies, with the task that owns it. Paths are relative to the
repository root. **Shared files**, touched by more than one task here or owned by a sibling plan,
are marked ⚑; the tasks that touch them run in task order, never in parallel worktrees. Hub server
files (`server/`, `store/`, `migrate/`) are **planned by the Hub plan and not yet on `feat/hub`**
at this writing (only `hubapi/` and `hubclient/` exist there); their paths follow the Hub plan's
file map (`2026-09-23-phase-h-hub.md:260-362`) and a WH task that finds them moved follows the
move (rule H1). Where a slice adds to a W1 file, the later task is listed in the same row.

### The Hub: `control-plane/internal/hub/...` (stream WH)

| Path | Responsibility | Status | Task |
|---|---|---|---|
| `hubapi/keys.go` ⚑ | the `issue:` case in the key grammar the Hub mints and accepts (Hub task 01's file; W0's proposal) | modified | 02 |
| `hubapi/workids.go` | the Work id prefixes (`is-` … `df-`); `DisplayAlias(projectKey, n)`, which is never parsed as a key | new | 02 |
| `hubapi/workcanonical.go`, `hubapi/testdata/issue-revision/*.json` | the `issue-revision` canonical kind; golden vectors | new | 02 |
| `hubapi/work.go` | W1 wire types: issue head and revision, `serves`, states, thread state, relations, labels, policies, comments, subscriptions, imports | new | 03 |
| `hubapi/work_plan.go`, `work_agents.go`, `work_roadmap.go`, `work_intake.go` | W2, W3, W4 and W5 wire types | new | 72, 99, 114, 143 |
| `hubclient/work.go` | the `Work` interface, one sub-interface per slice | new | 04; 72, 99, 114, 143 |
| `hubclient/hubclient.go` ⚑ | one line: `Hub` embeds `Work` (`hub-contract.md:1329`) | modified | 04 |
| `hubclient/fake_work*.go` | the fake's Work rules, in memory | new | 04, 05; 73, 99, 115, 144 |
| `hubclient/conformance/work*.go` | Work conformance rows, run against the fake and the real Hub | new | 05; 73, 99, 115, 144 |
| `hubclient/methods_work*.go` | the Work methods over HTTP, with the digest re-check | new | 16; 82, 100, 124, 168 |
| `migrate/sql/NNNN_work_*.sql` | one migration per slice, **next free number at sync time**: tables, RLS, grants, guards | new | 06, 07; 74, 100, 116, 145 |
| `store/work_*.go`, `server/work_*.go` | the store and routes, one pair per aggregate | new | 08-15; W2-W5 Hub tasks (81b's `work_implements.go` among them) |
| `server/matrix_integration_test.go` ⚑ | authorisation-matrix rows for every Work route | modified | 07 and every WH route task |
| `server/changes.go` ⚑ | the change feed carries Work kinds, filtered per caller | modified | 13 |
| `server/erase.go` ⚑ | erase for issues, comments, requests and notes | modified | 14; 146 |
| `server/janitor.go` ⚑ | cycle rollover and closure proposals (W2); recurring issues and reminders (W4); loop triggers (W5) | modified | 77; 121; 162 |
| `work_e2e_integration_test.go` | W1's Hub end to end through `devissuer` | new | 17 |
| `worksync/{engine,links,boards,github,jira}.go` | tracker sync, run by the Hub as an integration principal that can never write a fact state or `done` | new | 150-153 |
| `server/work_hooks.go` | host webhooks for outside PRs and commits, both hosts (`OD-4`) | new | 154, 155 |
| `server/work_forms.go`, `server/work_pages.go` | the Entra-protected intake form; the read-only issue page (W2's https link target) | new | 148; 80 |
| `mailer/{mailer,graph,fake}.go` | the outbound mail seam (digest, requester updates) and inbound team addresses | new | 81; 149 |
| `webhooks/{deliver,sign}.go` | HMAC webhooks built on the change feed, metadata only | new | 156 |
| `control-plane/cmd/workmcp/` | the Work MCP server, read-only mode, behind Entra OAuth | new | 161 |

### The control plane (loopback, per user) and the tools

| Path | Responsibility | Status | Task |
|---|---|---|---|
| `control-plane/internal/api/work.go` (+ test) | the `/v1/hub/work/*` relay (contract §4.3, §9.3 pattern) | new | 16; 82, 100, 124, 168 |
| `control-plane/internal/api/api.go` ⚑, `routes_audit_test.go` ⚑ | `Deps.Work`, `Deps.WorkStart`, `Deps.WorkImport`; registrations; catalog rows | modified | 16, 24, 25 |
| `control-plane/cmd/api/{config,main}.go` ⚑ | wiring and `PANACEA_APP_WORK_*` variables | modified | 16, 24, 25 |
| `control-plane/internal/workstart/{start,undo}.go`, `internal/api/workstart.go` | Start from an issue: bind, worktree, session, *Undo Start* until the first file write | new | 24 |
| `control-plane/internal/workimport/{linear,csv,batch}.go`, `internal/api/workimport.go` | the Linear public-API pager and the CSV parser, both producing SW.6's batch | new | 25 |
| `control-plane/cmd/panacea-release/`, `integrations/{azure-pipelines-task,github-action}/` | the release CLI and the two CI wrappers | new | 164 |
| `sdk/{ts,go}/` | public-API clients generated from the contract | new | 158 |

### Deltas on sibling code (rule H2; the owner phase's file wins)

"Planned" means only the sibling **plan** names the file; the part cites the plan row and marks the
path **UNVERIFIED until merge**, as Phase T does.

| Sibling | Path it extends | State on 2026-09-25 | What this plan adds | Task |
|---|---|---|---|---|
| phase 4 | `control-plane/internal/provenance/{key,ref}.go` (P4 task 06; plan `:312-313`) | planned | the `issue:` case; `Ref.Kind` `"issue"` | 18 |
| phase 4 | `control-plane/internal/contextwrite/materialise.go` (P4 task 09; `:319`), `internal/api/sessions.go` ⚑ (P4 task 10; `:346`) | planned; `sessions.go` built | the managed block's issue section; `Session.IssueRef` | 19 |
| phase 4 | `desktop/src/provenance/{loader.ts,ContextChip.tsx}` (P4 tasks 16-17; `:330-332`) | planned | the loader's second key source; the issue chip | 19, 67 |
| phase 3 | `images/sandbox/githooks/` (P3 task 05; plan `:145`), `control-plane/internal/gitx` (`:135`) | planned | the `Panacea-Issue` trailer | 21 |
| phase 3 | `control-plane/internal/converge` (P3 tasks 07-09; `:137`) | planned | the lane index names its issue; land trailers per issue; the fact moves | 21, 22 |
| phase 3 | `control-plane/internal/evidence` (P3 task 15; `:142`) | planned | the done evaluator; observed evidence; *self-attested* | 23; 104 |
| phase 3 | the Merge lens (P3 tasks 19-20; paths **UNVERIFIED**) | planned | lane rows name their issue; proposed evidence and *Record* | 67; 104 |
| phase 7 | the manifest (P7 task 19) and its drawer (P7 task 17; paths **UNVERIFIED**) | planned | issues from `Panacea-Issue` trailers; *shipped to* | 71; 131 |
| phase 6 | the per-session gateway token (plan still being written) | planned | the token carries the issue id | 135 |
| Phase T | Fleet's task record, events and verbs (ST.1, ST.2; plan `:195-243`) | planned | `task.issueId`, two event kinds, three verbs | 98, 101 |
| Phase T | its Desk rows `desktop/src/components/desk/team*.tsx` (TM task 54) | planned | nothing: they plug into SW.17, task 62's frame | 62 |
| Phase H + phase 5 | `control-plane/internal/hub/server/approvals.go` ⚑ (**Hub task 22**; hub plan `:304`, `:425`) — the close's call site; `desktop/src/design/{DesignLens,DesignSide}.tsx` (P5 task 28; `:420`). Phase 5's `internal/designapi/approve.go` (P5-21; `:397`) is that route's **caller**, not the writer, and is untouched | Hub's built; phase 5 planned, read 2026-09-28, paths **UNVERIFIED until merge** | the approval write also lands the issues whose revision `implements` that exact key, in its own transaction (04 §7.7; 05 `:1082`, `:1305`); the canvas's list of those issues; the issue view's *Implements* and its picker | 81b, 96b |

### The desktop: `desktop/`

| Path | Responsibility | Status | Task |
|---|---|---|---|
| `src/test/expectNoPrediction.ts`, `src/test/workFixtures.ts` | the `OD-27` guard every surface test calls; issue, criterion and revision fixtures | new | 20 |
| `src/styles/tokens.css` ⚑, `tokens.test.ts` ⚑, `designsystem.test.ts` ⚑ | 02 D2 step 1's tokens and `--t-fact`; the provisional `paper` block; generated themes later | modified | 26, 30, 34; 95, 96 |
| `src/assets/fonts/CommitMono-*.woff2`, `src/assets/fonts/OFL-CommitMono.txt`, `src/styles/fonts.css` | Commit Mono as files with its licence; the sans stack on Segoe UI Variable | new | 27 |
| `src/components/glyphs/{StateGlyph,AgentGlyph,MarkGlyph}.tsx`, `glyphs.css` | our drawn glyphs, SVG in `currentColor`, 14 and 16 px | new | 28, 29 |
| `src/components/common/ViewHeader.tsx`, `src/styles/app.css` ⚑, `rail/rail.css` ⚑ | one header with the route badge; `chrome.dim` | new, modified | 31 |
| `src/lib/flags.ts`, `src/components/devtools/{FlagCompare,StressSets}.tsx` | `flag.compare` and `stress.sets` in the dev toolbar | new | 32, 33 |
| `src/styles/forced-colors.css`, `src/styles/grain.css` | the Windows contrast layer; grain, off in `-hc` | new | 34, 35 |
| `src/components/thread/{ThreadNode,ThreadLine,ThreadTrace}.tsx`, `useThreadLit.ts` | the signature primitives | new | 36 |
| `src/api/work.ts` (+ test), `src/api/client.ts` ⚑ | Work wire types and relay calls | new, modified | 37; 83, 125, 169 |
| `src/store/slices/workSlice.ts`, `src/store/types.ts` ⚑, `src/store/store.ts` ⚑ | the non-persisted Work view model; `Lens` gains `"work"` (`types.ts:173`) | new, modified | 37 |
| `src/work/replica/{replica,ops,undo,visibility,search}.ts` | the in-memory Hub replica on `/v1/changes` | new | 55-58 |
| `src/components/work/{WorkLens,CriterionGroup,IssueRow,AgentLine,HeldLine}.tsx`, `work.css` | the Work lens and its row | new | 40, 41 |
| `src/components/work/{TriageView,BoardView,DisplayMenu,BulkBar,PlanSurface,Peek,StartChip,NewIssue,ImportDialog}.tsx` | Triage, board, display, bulk, Plan, peek, Start, create, import | new | 42-45, 60, 61, 68, 69 |
| `src/components/issue/{IssueView,IssueHead,Revisions,Serves,Implements,Relations,IssueAgentLine,Comments,Attachments}.tsx` | the one issue view | new | 46-48, 51, 52; 96b |
| `src/components/editor/*`, `desktop/package.json` ⚑ | the one Markdown editor on `prosemirror-*` (MIT) | new, modified | 49, 50; 92 |
| `src/hooks/useKeyboardMap.ts` ⚑, `src/work/keys/{keymap,sequences}.ts`, `src/components/work/ShortcutSheet.tsx` | `keys.list`: `isCockpitChord` extended, `G`/`O`/`M` sequences, the `?` sheet | modified, new | 53, 54 |
| `src/components/palette/commands.ts` ⚑, `CommandPalette.tsx` ⚑ | work verbs with chords; prefixes; opens beside a row | modified | 59 |
| `src/components/desk/{Desk,DeskSection,DeskZero,deskModel,deskRows}.tsx`, `desk.css` | **the one Desk frame** (§5.6); team mode's rows plug in | new | 62-65 |
| `src/components/common/LiveRegion.tsx` | one polite live region per window | new | 66 |
| `src/components/develop/SessionPaneHeader.tsx` ⚑, `src/components/AppShell.tsx` ⚑ | the issue chip; the Work, Plan and Desk mounts | modified | 67; 40, 45, 62 |
| `src/components/{plan,roadmap,intake,admin}/*` | W2's features, cycles, views; W4's roadmap and insights; W5's intake and admin | new | 84-96; 126-141; 169-173 |

### Proof and documents

| Path | Responsibility | Status | Task |
|---|---|---|---|
| `docs/superpowers/planning/2026-09-25-linear-parity/w0-hub-contract-proposal.md` | W0's two Hub-contract edits as a proposal to the `feat/hub` owner | new | 01 |
| `docs/PROGRAM.md` ⚑, `docs/FUNCTIONAL-SPEC.md` ⚑, `docs/AGENTIC-UX.md` ⚑, `…/2026-09-25-linear-parity/02-linear-design-language.md` ⚑ | Phase W's row; Feature and Issue in the object model; Q13's four amendments; 02's three corrections | modified | 01 |
| `docs/verification/2026-09-25-phase-w-results.md` | per slice: what was run, what was observed, the parity rows it closes, what stays unverified | new | 70, 97, 113, 142, 174, 187 |
| `docs/verification/2026-09-25-phase-w-prototype-review.md` | the owner's review of the in-app prototype, in the owner's words | new | 39 |
| `docs/work/agent-rules.md`, `docs/work/method.md` | our published agent interaction rules; the user-facing method page | new | 109; 134 |

---

## Task list

Each task's full text is in `2026-09-25-phase-w-work-layer-parts/task-NN.md` (three digits from
`task-100.md`), written from its scope paragraph in
`docs/superpowers/plans/2026-09-25-phase-w-work-layer-scopes/`, one file per 20 ids (the list is
under *Task scopes* at the end of this file). A focused day is 8 hours, and **every task is sized
at 1 to 3 hours**: the plan's estimate, not a measurement. The design's slice sizes (05 §5.1, about
138 focused days) are for one human implementer and are **not reconciled** with these; neither is
measured.

**Totals, summed from the tables below** (an estimate, not a measurement):

| Slice | Ids | Tasks | Hours | Focused days |
|---|---|---|---|---|
| W0 | 01 | 1 | 2 | 0.25 |
| W1 | 02-70 | 69 | 192 | 24 |
| W1b | 71 | 1 | 3 | 0.375 |
| W2 | 72-97, with 81b and 96b | 28 | 76.5 | 9.6 |
| W3 | 98-113 | 16 | 46 | 5.75 |
| W4 | 114-142 | 29 | 79.5 | 9.9 |
| W5 | 143-174 | 32 | 91 | 11.4 |
| WL | 175-187 | 13 | 33 | 4.1 |
| **All** | **01-187, 81b, 96b** | **189** | **523** | **65.4** |

**dependsOn** names this plan's tasks by number and sibling tasks by prefix: `H` the Hub plan
(`.claude/worktrees/hub/docs/superpowers/plans/2026-09-23-phase-h-hub.md:396-446`), `P3-` phase 3
(`.claude/worktrees/phase-3/docs/superpowers/plans/2026-09-23-phase-3-git-convergence.md:591-614`),
`P4-` phase 4 (`.claude/worktrees/phase-4/docs/superpowers/plans/2026-09-23-phase-4-discover.md:394-406`), `P5-` phase 5 (`.claude/worktrees/phase-5/docs/superpowers/plans/2026-09-23-phase-5-design-canvas.md:442-472`, its task table read 2026-09-28),
`P6-` phase 6 (its plan is still being written, so every `P6-` id is **UNVERIFIED**), `P7-` phase 7
(`.claude/worktrees/phase-7/docs/superpowers/plans/2026-09-23-phase-7-deploy.md:645-647`) and `TM-`
Phase T (`.claude/worktrees/team/docs/superpowers/plans/2026-09-25-phase-t-team-mode.md:281-406`).
A sibling task is a dependency only once its branch is sync-merged here (rules H1, H2).

**Start lines.**

| Start line | After | Who can start | What they can do |
|---|---|---|---|
| **WS0, code against the Work fake** | 01-05 (about 13 h) | streams WP, WD1, WD2 | build every Work-backed handler and surface against `hubclient.Work` and `NewFake()`; SW.1 and SW.2 are frozen here, and a later change that is not an added method, field or route reopens task 05 and is announced |
| **WS1, the real Hub** | 06-17 | task 70 | the local-mode demo against the local Hub stack through `devissuer` |
| **W1 in the cloud** | 70, then phase 6 stream A and phase 8 stream C | the owner | the same demo on managed sessions; no Phase W task is needed for it |

**Seams** this plan defines, so parallel writers agree without reading each other:

| Seam | What is fixed | Defined in | Consumed by |
|---|---|---|---|
| **SW.0** | the W0 proposal: the `issue:` key case and the `c-` example | 01 | 02, 18 |
| **SW.1** | the Work wire, W1: id prefixes, the `issue:` key, `DisplayAlias`, the `issue-revision` canonical form and golden vectors — which already carry `implements: [design:…]` (`task-02.md:243`, 04 §7.7) — every W1 type and request body | 02, 03 | 04-19, 24, 25, 37, 81b |
| **SW.2** | `hubclient.Work` (W1 methods), `NewFake()`'s Work rules, the relay paths `/v1/hub/work/*` | 04, 05, 16 | 19, 21-25, 37, 55, 98 |
| **SW.3** | who may write what: the authorisation rows and the guards (fact states, `done`, the sync principal) | 07 | 10, 22, 23, 99, 145, 150, 159 |
| **SW.4** | the state move: `StateMove{to, cause, reason}`, the done rule's inputs and its result words | 10 | 22, 23, 48, 63-65, 71, 81b |
| **SW.5** | the Work feed kinds and their metadata-only payloads | 13 | 55, 62, 98, 123, 156 |
| **SW.6** | the import batch: one normalised item per external issue, idempotent by `(source, externalId)` | 15 | 25, 69, 150-153 |
| **SW.7** | the managed block's issue section: what an agent is told, byte for byte | 19 | 24, 101 |
| **SW.8** | `expectNoPrediction(container)` and the Work fixtures | 20 | every desktop task |
| **SW.9** | the `Panacea-Issue: is-…` trailer grammar and its placement | 21 | 22, 71, 131, 154 |
| **SW.10** | Start from an issue: the route, its result, *Undo Start* and *Stop and discard* | 24 | 61, 102, 163 |
| **SW.11** | the tokens and faces: `--t-fact`, `--row-h*`, `--dur-flash`, `--undo-window`, `--mono`, `--sans` | 26, 27 | every UI task |
| **SW.12** | `<StateGlyph state priority notProven/>`, `<AgentGlyph status/>`, `<MarkGlyph kind/>` | 28, 29 | every surface task |
| **SW.13** | `<ViewHeader title route badge actions/>` | 31 | 38-48, 62, 85-91, 126-132 |
| **SW.14** | `<ThreadNode/>`, `<ThreadLine/>`, `<ThreadTrace/>`, `useThreadLit(criterionId)` | 36 | 40, 47, 67, 85 |
| **SW.15** | the desktop Work types and `workSlice` | 37 | 38-69, 83, 125 |
| **SW.16** | the replica: `useWork(selector)`, `mutate(op) → {undo}`, `onRefused`, `search(q)` | 55-58 | 40-69, 83, 125 |
| **SW.17** | the Desk frame's row-kind registry: `registerDeskRow(kind, section, render, rank)` | 62 | 63-65, 71, TM-54 |
| **SW.18** | the Work wire, W2 (`WorkPlan`) | 72 | 73-96 |
| **SW.19** | the W3 records: comment kinds `note`, `evidence_proposal`, `negotiation`; guidance; the kill switch | 99 | 100-112 |
| **SW.20** | the Work wire, W4 (`WorkRoadmap`) | 114 | 115-141 |
| **SW.21** | the Work wire, W5 (`WorkIntake`) | 143 | 144-173 |
| **SW.22** | the outbound webhook payload: event metadata only, HMAC-SHA256 signature header | 156 | 157, 158, 178, 182 |

### W0 and W1: the first slice a team can adopt

| Id | Task | Slice | Focused days | dependsOn | Seams |
|---|---|---|---|---|---|
| 01 | W0 and the documents: the Hub-contract proposal; PROGRAM, FUNCTIONAL-SPEC, AGENTIC-UX's four amendments, 02's corrections | W0 | 0.25 (2 h) | — | SW.0 |
| 02 | `hubapi`: Work id prefixes, the `issue:` key, `DisplayAlias`, the `issue-revision` canonical kind and golden vectors | W1 | 0.3 (2.5 h) | 01; H01-H03 | SW.1 |
| 03 | `hubapi`: the W1 wire types and request bodies | W1 | 0.375 (3 h) | 02 | SW.1 |
| 04 | `hubclient.Work` and the fake, part 1: issues, revisions, numbers, the project key, states | W1 | 0.375 (3 h) | 03; H04-H05 | SW.2 |
| 05 | the fake, part 2, and the Work conformance rows: relations, labels, policies, comments, subscriptions, imports, search, erase (**WS0**) | W1 | 0.375 (3 h) | 04 | SW.2 |
| 06 | migration, part 1: the Work tables, row-level security and grants | W1 | 0.375 (3 h) | 03; H06-H07b | — |
| 07 | migration, part 2: the guards, and the authorisation-matrix rows for every W1 Work route | W1 | 0.375 (3 h) | 06; H16b | SW.3 |
| 08 | issue heads, numbers and `project.key`: routes and store, one event per write | W1 | 0.375 (3 h) | 07; H14, H17 | SW.1 |
| 09 | issue revisions, `serves`, restore and attachments; `/v1/refs` resolves `issue:` | W1 | 0.375 (3 h) | 08; H19-H21 | SW.1 |
| 10 | states, thread state and the done rule on the server | W1 | 0.375 (3 h) | 09; H25 | SW.4 |
| 11 | relations, team labels, `triage` and team policies | W1 | 0.375 (3 h) | 08 | — |
| 12 | comments `ic-`: mentions, reactions, anchors, resolve | W1 | 0.375 (3 h) | 08 | — |
| 13 | subscriptions; the change feed's Work kinds, filtered per caller | W1 | 0.3 (2.5 h) | 11, 12; H26 | SW.5 |
| 14 | erase for issues and comments; title search | W1 | 0.3 (2.5 h) | 12; H27 | — |
| 15 | the import route: a normalised batch into `triage`, *not tied · imported* | W1 | 0.3 (2.5 h) | 11 | SW.6 |
| 16 | `hubclient`'s Work methods over HTTP; the control plane's `/v1/hub/work/*` relay | W1 | 0.375 (3 h) | 05, 15; H33-H34 | SW.2 |
| 17 | W1's Hub end to end through `devissuer` | W1 | 0.375 (3 h) | 10, 13, 14, 16; H43 | — |
| 18 | phase 4 delta: `issue:` in `provenance/key.go`; `Ref.Kind` `"issue"` | W1 | 0.25 (2 h) | 02; P4-06 | SW.1 |
| 19 | phase 4 delta: `Session.IssueRef`; the Materialiser's issue section; the loader's second key | W1 | 0.375 (3 h) | 05, 18; P4-09, P4-10, P4-16 | SW.7 |
| 20 | desktop test kit: `expectNoPrediction` and the Work fixtures | W1 | 0.25 (2 h) | — | SW.8 |
| 21 | phase 3 delta: the `Panacea-Issue` trailer; the lane index names its issue; AU-only branch names | W1 | 0.375 (3 h) | 19; P3-05, P3-07, P3-08 | SW.9 |
| 22 | phase 3 delta: the `started`, `in_review` and `landed` fact moves, written as the person | W1 | 0.375 (3 h) | 05, 21; P3-09 | SW.4 |
| 23 | phase 3 delta: the done evaluator; observed evidence at the landed commit; *self-attested* | W1 | 0.375 (3 h) | 22; P3-15 | SW.4 |
| 24 | control plane: Start from an issue, *Undo Start*, the data-class route | W1 | 0.375 (3 h) | 16, 19, 21 | SW.10 |
| 25 | control plane: the Linear public-API and CSV import adapters | W1 | 0.375 (3 h) | 15, 16 | SW.6 |
| 26 | tokens: 02 D2 step 1, `--t-fact`, the provisional `paper` block | W1 | 0.25 (2 h) | — | SW.11 |
| 27 | faces: Commit Mono bundled with its licence; Segoe UI Variable as the sans | W1 | 0.25 (2 h) | 26 | SW.11 |
| 28 | drawn glyphs, part 1: the nine issue states, *done, not proven*, the priority notch | W1 | 0.375 (3 h) | 26 | SW.12 |
| 29 | drawn glyphs, part 2: agent shapes, selection, needs-you, `thread.node`; the licence check | W1 | 0.375 (3 h) | 28 | SW.12 |
| 30 | `voice.split` across Develop and the session pane (02 D2 step 3) | W1 | 0.3 (2.5 h) | 27 | — |
| 31 | `chrome.dim` and one `ViewHeader` with the route badge (02 D2 step 2) | W1 | 0.375 (3 h) | 26 | SW.13 |
| 32 | `flag.compare` in the dev toolbar: old and new look, the three sans candidates (02 D2 step 7) | W1 | 0.25 (2 h) | 27, 31 | — |
| 33 | `stress.sets` with its three new screens | W1 | 0.375 (3 h) | 29, 32 | — |
| 34 | the `forced-colors` layer; the reduced-motion running shape and static tick | W1 | 0.25 (2 h) | 29, 33 | — |
| 35 | grain, measured at 150 % in WebView2 | W1 | 0.25 (2 h) | 31 | — |
| 36 | the thread primitives: node, line, lit, trace | W1 | 0.375 (3 h) | 29, 30 | SW.14 |
| 37 | desktop Work wire types, relay calls and `workSlice`; `Lens` gains `"work"` | W1 | 0.3 (2.5 h) | 05, 20 | SW.15 |
| 38 | prototype, part 1: the Work lens and Plan in the app, behind `flag.compare`, on fixtures | W1 | 0.375 (3 h) | 32, 36, 37 | — |
| 39 | prototype, part 2: the issue view and the Desk; **the owner's review**, recorded | W1 | 0.375 (3 h) | 38 | — |
| 40 | the Work lens: the list grouped by criterion, one agent line, coarse liveness | W1 | 0.375 (3 h) | 39 | — |
| 41 | `arrivals.hold`, `moves.hold` and departures; `N` applies | W1 | 0.375 (3 h) | 40 | — |
| 42 | Triage: the view, number keys, `T` with a text-match criterion suggestion | W1 | 0.375 (3 h) | 40 | — |
| 43 | the board, and drag that offers a fact instead of faking it | W1 | 0.375 (3 h) | 40 | — |
| 44 | display options and the bulk bar, with *Start N worktrees* and `confirm.inplace` | W1 | 0.375 (3 h) | 40 | — |
| 45 | Plan (`G P`): Triage, Backlog, criteria with their issues, *My issues* | W1 | 0.375 (3 h) | 42 | — |
| 46 | the issue view, part 1: head fields and their pickers | W1 | 0.375 (3 h) | 37, 39 | — |
| 47 | the issue view, part 2: revisions, restore, `serves` with `thread.node`, revision-pinned links | W1 | 0.375 (3 h) | 36, 46 | — |
| 48 | the issue view, part 3: relations, the live agent line, *done, not proven* and its reason | W1 | 0.3 (2.5 h) | 46 | — |
| 49 | the editor, part 1: one Markdown editor on `prosemirror-*` (MIT), author class kept | W1 | 0.375 (3 h) | 46 | — |
| 50 | the editor, part 2: the slash menu and tables | W1 | 0.375 (3 h) | 49 | — |
| 51 | comments: the thread, mentions, reactions, anchors, resolve | W1 | 0.375 (3 h) | 49 | — |
| 52 | attachments by blob digest; subscribe and unsubscribe | W1 | 0.3 (2.5 h) | 51 | — |
| 53 | keys, part 1: `keys.list`, `isCockpitChord` extended, `event.code` for agent keys, the dead Linear chords, the single-key switch | W1 | 0.375 (3 h) | 40 | — |
| 54 | keys, part 2: `G`, `O` and `M` sequences, the generated `?` sheet, chords in menus, the safe triangle | W1 | 0.375 (3 h) | 53 | — |
| 55 | the replica, part 1: in memory, full load, the cursor on `/v1/changes`; the no-persistence test | W1 | 0.375 (3 h) | 37 | SW.16 |
| 56 | the replica, part 2: optimistic writes, rollback on refusal, `optimistic.undo`, `U` | W1 | 0.375 (3 h) | 55 | SW.16 |
| 57 | the replica, part 3: only what the caller may read; resync on a gap; erase | W1 | 0.3 (2.5 h) | 55 | SW.16 |
| 58 | the replica, part 4: `/` search and `O I` open by number | W1 | 0.3 (2.5 h) | 53, 55 | SW.16 |
| 59 | the palette: every work verb with its chord, the four prefixes, opens beside the row | W1 | 0.3 (2.5 h) | 54, 58 | — |
| 60 | peek on `Space`: `peek.span`, `peek.hunk`, `peek.person` | W1 | 0.375 (3 h) | 40, 55 | — |
| 61 | Start on the desktop: `Shift+R`, the *Undo Start* chip, *Stop and discard*, the 10 s first sign | W1 | 0.375 (3 h) | 24, 56 | — |
| 62 | **the Desk frame**: five fixed sections, never live while open, the row-kind registry | W1 | 0.375 (3 h) | 37 | SW.17 |
| 63 | the Desk: issue rows in NEEDS YOU, aging, *blocking someone's agent* | W1 | 0.375 (3 h) | 55, 62 | — |
| 64 | the Desk: FAILED with *Restart from checkpoint* and *Abandon*; LANDED; RUNNING's fold | W1 | 0.375 (3 h) | 62 | — |
| 65 | the Desk: §3.7's informs and the reassign *decide*; answer in place; the zero state | W1 | 0.375 (3 h) | 63, 64 | — |
| 66 | the accessibility floor: one live region, `aria-hidden` liveness, grid roles | W1 | 0.3 (2.5 h) | 41, 62 | — |
| 67 | issue chips in Develop and Merge | W1 | 0.25 (2 h) | 48; P4-17, P3-19 | — |
| 68 | create an issue from any list, a Desk row, a span or a hunk | W1 | 0.3 (2.5 h) | 46, 53 | — |
| 69 | the import dialog; `first.landing`'s rehearsal as the real issue EP-1 | W1 | 0.3 (2.5 h) | 25, 42 | — |
| 70 | **W1's local-mode demo**, end to end, and its results | W1 | 0.375 (3 h) | 01-69 | — |
| 71 | shipped: issues in the release manifest; *shipped to production, 14:02* | W1b | 0.375 (3 h) | 22, 64; P7-17, P7-19 | — |

### W2: planning

| Id | Task | Slice | Focused days | dependsOn | Seams |
|---|---|---|---|---|---|
| 72 | `hubapi` and `hubclient.WorkPlan`: the W2 wire types and methods | W2 | 0.375 (3 h) | 70 | SW.18 |
| 73 | the fake's W2 rules and conformance rows | W2 | 0.375 (3 h) | 72 | SW.18 |
| 74 | the W2 migration, its guards and matrix rows | W2 | 0.375 (3 h) | 72 | — |
| 75 | custom states inside the human bands | W2 | 0.3 (2.5 h) | 74 | — |
| 76 | features `ft-`: lead, dates, status, branch reservation, *promote to feature* | W2 | 0.375 (3 h) | 74; P4-18 | — |
| 77 | cycles `cy-`: cadence, the janitor's creation and rollover, scope log, iCal; closure proposals and archive | W2 | 0.375 (3 h) | 74 | — |
| 78 | sub-issues and the parent rule; templates `tp-`; label groups and org labels | W2 | 0.375 (3 h) | 74 | — |
| 79 | estimates, due dates, rank, move with redirect, the resolved-blocker rule | W2 | 0.3 (2.5 h) | 74 | — |
| 80 | views `vw-`, drafts `df-`, https links and the read-only page, CSV export | W2 | 0.375 (3 h) | 74 | — |
| 81 | the digest builder and the outbound mail seam | W2 | 0.3 (2.5 h) | 13, 74 | — |
| 81b | `implements`: the write and read paths, and the design approval that closes a design issue (04 §7.7) | W2 | 0.375 (3 h) | 10, 23, 74; P5-21 | SW.1, SW.4 |
| 82 | `hubclient`'s W2 methods over HTTP, the relay rows, W2's Hub end to end | W2 | 0.375 (3 h) | 73, 75-81, 81b | SW.18 |
| 83 | desktop W2 wire types and replica kinds | W2 | 0.3 (2.5 h) | 55, 73 | — |
| 84 | custom states in team settings and on the row | W2 | 0.25 (2 h) | 83 | — |
| 85 | the features board and feature tabs; status beside the facts | W2 | 0.375 (3 h) | 83 | — |
| 86 | *promote to feature* and the `B` key | W2 | 0.25 (2 h) | 85 | — |
| 87 | cycles: convergence per criterion, the scope log, cadence settings, `Y` | W2 | 0.375 (3 h) | 83 | — |
| 88 | sub-issues and *split into issues* | W2 | 0.3 (2.5 h) | 83 | — |
| 89 | templates, estimates, due dates (`D`), manual order and move | W2 | 0.375 (3 h) | 83 | — |
| 90 | label groups, `Group/Label` shorthand, org labels | W2 | 0.25 (2 h) | 83 | — |
| 91 | views: the editor, save, favourites, set as default, ad hoc lists, search prefixes, a person's page | W2 | 0.375 (3 h) | 83 | — |
| 92 | drafts, callouts and *copy as Markdown* with the Hub key | W2 | 0.3 (2.5 h) | 50, 83 | — |
| 93 | https links and the prefill deep link | W2 | 0.25 (2 h) | 80, 83 | — |
| 94 | the digest, defer with a reason (`H`), CSV export | W2 | 0.3 (2.5 h) | 65, 81 | — |
| 95 | themes, part 1: `tokens.gen.ts` from three LCH seeds; `graphite` and `paper` generated (02 D2 step 5) | W2 | 0.375 (3 h) | 33 | SW.11 |
| 96 | themes, part 2: the high-contrast pair; personal preferences | W2 | 0.3 (2.5 h) | 95 | — |
| 96b | the Design lens's issue list; *implements* on the issue view | W2 | 0.25 (2 h) | 83, 81b; P5-27, P5-28 | SW.14, SW.16 |
| 97 | W2's proof and results | W2 | 0.3 (2.5 h) | 72-96, 81b, 96b | — |

### W3: agents on the work (ships with Phase T's TM1)

| Id | Task | Slice | Focused days | dependsOn | Seams |
|---|---|---|---|---|---|
| 98 | Phase T delta: `task.issueId`; the `issue.delegated` and `blocker.landed` Orchestrator events | W3 | 0.375 (3 h) | 70; TM-03, TM-06, TM-10 | SW.5 |
| 99 | the W3 Hub wire, fake and conformance: comment kinds, guidance records, the org kill switch | W3 | 0.375 (3 h) | 97, 98 | SW.19 |
| 100 | the W3 migration, routes, matrix rows and HTTP methods | W3 | 0.375 (3 h) | 99; TM-05 | SW.19 |
| 101 | the verbs: `panacea issue show`, `panacea issue note`, `panacea evidence propose`; `panacea team` names each agent's issue | W3 | 0.375 (3 h) | 100; TM-18, TM-19, TM-22 | SW.7 |
| 102 | `Shift+O` delegate and `Shift+A` ask; the team strip's line is the issue title | W3 | 0.375 (3 h) | 101; TM-53, TM-55 | SW.10 |
| 103 | Orchestrator notes on the issue, one voice per owner; agent comments name owner, agent, model, session; `@me:orchestrator` | W3 | 0.3 (2.5 h) | 100 | — |
| 104 | proposed evidence under each criterion in the Merge lens; *Record* as the person | W3 | 0.375 (3 h) | 23, 100; P3-20 | — |
| 105 | Fleet: §3.7 rules 2 and 3 through Orchestrators; cross-owner `blocks` seeds a `sequence` proposal | W3 | 0.375 (3 h) | 98; TM-29, TM-30 | — |
| 106 | the negotiation row on both issues: a `⇄` line until seen, then a glyph | W3 | 0.3 (2.5 h) | 105 | — |
| 107 | the agent's task-card page; *my agents* and *Alice's agents* filters; `who.label` | W3 | 0.375 (3 h) | 98; TM-52 | — |
| 108 | `step.fold` on the issue; one ephemeral line; session, lane and preview links; the first-activity budget | W3 | 0.375 (3 h) | 107 | — |
| 109 | guidance and the kill switch in settings; our published agent rules | W3 | 0.3 (2.5 h) | 100; TM-14 | — |
| 110 | the read-only *ask* worker: answers that link files and lines | W3 | 0.375 (3 h) | 101; TM-21 | — |
| 111 | named risk facts on lanes; the repository proposal; Orchestrator-filed issues into `triage` | W3 | 0.375 (3 h) | 100 | — |
| 112 | the same verbs in solo mode through phase 3's runner (**UNVERIFIED** that it can host them) | W3 | 0.375 (3 h) | 101 | — |
| 113 | W3's proof and results | W3 | 0.3 (2.5 h) | 98-112 | — |

### W4: roadmap, release and reporting

| Id | Task | Slice | Focused days | dependsOn | Seams |
|---|---|---|---|---|---|
| 114 | `hubapi` and `hubclient.WorkRoadmap`: the W4 wire types and methods | W4 | 0.375 (3 h) | 97 | SW.20 |
| 115 | the fake's W4 rules and conformance rows | W4 | 0.375 (3 h) | 114 | SW.20 |
| 116 | the W4 migration, its guards and matrix rows | W4 | 0.375 (3 h) | 114 | — |
| 117 | cross-team planning: a feature shared with a second team; initiatives across teams | W4 | 0.375 (3 h) | 116 | — |
| 118 | initiatives `in-`: owner, target, status, the health roll-up | W4 | 0.3 (2.5 h) | 116 | — |
| 119 | status updates `nt-`: health word, the pinned brief revision, the facts they cite | W4 | 0.3 (2.5 h) | 118; TM-46 | — |
| 120 | milestones `ml-` as criterion subsets; feature dependencies; feature and brief templates | W4 | 0.375 (3 h) | 116 | — |
| 121 | the janitor, W4: recurring issues, update reminders, triage rotation | W4 | 0.3 (2.5 h) | 78, 116 | — |
| 122 | insights queries: measured counts and durations, percentiles for the scatter | W4 | 0.375 (3 h) | 116 | — |
| 123 | read cursors for Work objects: the input to *since you last looked* | W4 | 0.3 (2.5 h) | 116; TM-54 | SW.5 |
| 124 | `hubclient`'s W4 methods over HTTP, the relay rows, W4's Hub end to end | W4 | 0.375 (3 h) | 115, 117-123 | SW.20 |
| 125 | desktop W4 wire types and replica kinds | W4 | 0.25 (2 h) | 83, 115 | — |
| 126 | cross-team sharing in the feature's header | W4 | 0.3 (2.5 h) | 125 | — |
| 127 | initiatives and the update composer | W4 | 0.375 (3 h) | 125 | — |
| 128 | the timeline, part 1: layout, day-to-quarter zoom, the cycle band, grouping | W4 | 0.375 (3 h) | 125 | — |
| 129 | the timeline, part 2: drag to reschedule with Undo; dependencies, a violated order in signal | W4 | 0.375 (3 h) | 128 | — |
| 130 | milestones: the criterion-subset picker and convergence | W4 | 0.25 (2 h) | 125 | — |
| 131 | phase 7 delta: notes by criterion, freeze per environment, path filters, *shipped* on the issue | W4 | 0.375 (3 h) | 71 | SW.9 |
| 132 | the insights panel and scatter; every point opens its issue | W4 | 0.375 (3 h) | 122, 125 | — |
| 133 | *since you last looked* names issues: four delta classes, a handover that links every claim | W4 | 0.375 (3 h) | 123; TM-54 | — |
| 134 | recurring issues, triage rotation, the method page | W4 | 0.3 (2.5 h) | 121 | — |
| 135 | phase 6 delta: the session token carries the issue id; spend per issue and per loop | W4 | 0.25 (2 h) | 24; P6 (UNVERIFIED id) | — |
| 136 | assists, part 1: triage suggestions with `serves` and a "why"; the calibrated dial | W4 | 0.375 (3 h) | 124; P6 gateway | — |
| 137 | assists, part 2: duplicates, composer hints, guidance layers | W4 | 0.375 (3 h) | 136 | — |
| 138 | assists, part 3: natural-language filters as editable chips | W4 | 0.3 (2.5 h) | 91, 136 | — |
| 139 | drafts, part 1: drafted updates, thread summaries as link indexes, the property-change digest | W4 | 0.375 (3 h) | 127, 136 | — |
| 140 | drafts, part 2: *propose issues per criterion*; *propose the next cycle* | W4 | 0.375 (3 h) | 87, 136 | — |
| 141 | an agent's edit to an issue or brief is a proposal, shown as a diff | W4 | 0.3 (2.5 h) | 47, 136 | — |
| 142 | W4's proof and results | W4 | 0.3 (2.5 h) | 114-141 | — |

### W5: intake and the ecosystem

| Id | Task | Slice | Focused days | dependsOn | Seams |
|---|---|---|---|---|---|
| 143 | `hubapi` and `hubclient.WorkIntake`: the W5 wire types and methods | W5 | 0.375 (3 h) | 97 | SW.21 |
| 144 | the fake's W5 rules and conformance rows | W5 | 0.375 (3 h) | 143 | SW.21 |
| 145 | the W5 migration, its guards and matrix rows: no sync principal or app writes a fact state or `done` | W5 | 0.375 (3 h) | 07, 143 | SW.3 |
| 146 | requests `rq-`: the requester, the verbatim quote as `text·H`, *important*, accept or link in `triage`, erase | W5 | 0.375 (3 h) | 14, 145; H27 | — |
| 147 | intake rules, first match wins with the author on the event; form defaults for members and non-members | W5 | 0.3 (2.5 h) | 146 | — |
| 148 | intake forms with typed, required fields; the Entra-protected Hub web form | W5 | 0.375 (3 h) | 147 | — |
| 149 | mail: an intake address per team; the requester hears *shipped*, with the release name, where they asked | W5 | 0.375 (3 h) | 81, 131, 146 | — |
| 150 | tracker sync, part 1: the engine, the integration principal, links by `(source, externalId)`, one way in to `triage` | W5 | 0.375 (3 h) | 15, 145 | SW.6 |
| 151 | tracker sync, part 2: Azure Boards, one way in | W5 | 0.375 (3 h) | 150 | — |
| 152 | tracker sync, part 3: GitHub Issues, one way in | W5 | 0.3 (2.5 h) | 150 | — |
| 153 | tracker sync, part 4: Jira one way in; then two way for comments only, on all three | W5 | 0.375 (3 h) | 151, 152 | — |
| 154 | outside PRs, part 1: host webhooks in from Azure DevOps and GitHub; commit links; closing and non-closing magic words | W5 | 0.375 (3 h) | 21, 22, 145 | SW.9 |
| 155 | outside PRs, part 2: linkbacks and autolinks; a PR-body magic word files an issue; copy `ep-142-slug`, the first push is `started` | W5 | 0.375 (3 h) | 154 | — |
| 156 | outbound webhooks: metadata-only payloads on the change feed, actor and previous values, HMAC-SHA256, retries, the delivery log | W5 | 0.375 (3 h) | 13, 145 | SW.22 |
| 157 | the Teams app, part 1: intake from a Teams message with its thread synced to the request; a channel per feature; view changes to a channel | W5 | 0.375 (3 h) | 146, 156 | — |
| 158 | the public API: published docs; TypeScript and Go clients generated from the contract | W5 | 0.375 (3 h) | 143, 156 | — |
| 159 | apps: Entra OAuth apps that act as a person, their manifests, admin approval | W5 | 0.375 (3 h) | 145 | SW.3 |
| 160 | keys and limits: personal and team-scoped API keys, admin-revocable; per-token rate limits with headers | W5 | 0.3 (2.5 h) | 159 | — |
| 161 | the Work MCP server on the Hub behind Entra OAuth, with read-only mode; the MCP connector allow-list | W5 | 0.375 (3 h) | 159, 160 | — |
| 162 | loops `lp-`, part 1: the record and its versions; event and schedule triggers in the janitor, as turns of the owner's Orchestrator within their grid, billed to them | W5 | 0.375 (3 h) | 145; TM-26, TM-37 | — |
| 163 | loops, part 2: the run log and *run now*; the first-pass triage loop, "on accept, Start" for a label | W5 | 0.375 (3 h) | 24, 162 | SW.10 |
| 164 | CI outside us: `panacea release`, an Azure Pipelines task and a GitHub Action | W5 | 0.375 (3 h) | 131, 160; P7-08 (a release-create route is **UNVERIFIED**) | — |
| 165 | the Teams app, part 2: the Desk digest, at most once per 15 minutes; ask **your own** Orchestrator from Teams | W5 | 0.375 (3 h) | 94, 157; TM-07, TM-28 | — |
| 166 | the per-team harness allow-list, Claude Code first; team skills in the sandbox image or a team library, the harness unmodified | W5 | 0.3 (2.5 h) | 145; TM-13, TM-15 | — |
| 167 | audit: each event says person or agent, and whose agent; streaming to Microsoft Sentinel | W5 | 0.3 (2.5 h) | 145; H27 | — |
| 168 | `hubclient`'s W5 methods over HTTP, the relay rows, W5's Hub end to end | W5 | 0.375 (3 h) | 144, 146-167 | SW.21 |
| 169 | desktop W5 wire types and replica kinds | W5 | 0.25 (2 h) | 125, 144 | — |
| 170 | requests in Triage: the quote, *important*, accept or link, the criterion suggestion; the form and rules editors | W5 | 0.375 (3 h) | 42, 169 | — |
| 171 | trackers on the desktop: connect Azure Boards, GitHub Issues or Jira; *not tied · imported*; the sync log | W5 | 0.3 (2.5 h) | 169 | — |
| 172 | loops on the desktop: the editor, versions, the run log, *run now* | W5 | 0.3 (2.5 h) | 169 | — |
| 173 | admin on the desktop: apps, keys, webhooks, MCP connectors, the harness allow-list | W5 | 0.375 (3 h) | 169 | — |
| 174 | W5's proof and results | W5 | 0.3 (2.5 h) | 143-173 | — |

### WL: the design's *later* rows

05 §1 marks twelve MATCH or EXCEED items *later*: designed for, not scheduled. `OD-26` commits all
257 items less the 27 skips, so each has a task here. Nothing in W0 to W5 depends on WL.

| Id | Task | Slice | Focused days | dependsOn | Seams |
|---|---|---|---|---|---|
| 175 | *Copy from team…* on templates, views and policies (L-7) | WL | 0.3 (2.5 h) | 174 | — |
| 176 | personal auto-assign rules in settings (L-40) | WL | 0.25 (2 h) | 174 | — |
| 177 | dashboards: a secondary surface under Work, never the landing page; measured counts only (L-152) | WL | 0.375 (3 h) | 132, 174 | — |
| 178 | warehouse and spreadsheet sync through the public API and webhooks; the first named connector (L-154) | WL | 0.3 (2.5 h) | 156, 158, 174 | SW.22 |
| 179 | syntax-aware diff and file categories through a permissively licensed differ, behind the licence gate (L-171) | WL | 0.375 (3 h) | 174; P3-16, P3-21 | — |
| 180 | guided review: a large change grouped by the criterion each change serves, core first, each hunk with its span (L-172) | WL | 0.375 (3 h) | 179; P3-19 | — |
| 181 | signed land commits for agent sessions (L-176; **UNVERIFIED** whether phase 3 signs today) | WL | 0.3 (2.5 h) | 174; P3-09 | — |
| 182 | Slack, after Teams: intake, feature feeds and the Desk digest (L-206) | WL | 0.375 (3 h) | 157, 165, 174 | SW.22 |
| 183 | alert intake into `triage`: Azure Monitor first, then Sentry (L-209) | WL | 0.3 (2.5 h) | 146, 174 | — |
| 184 | triage rotation imported from PagerDuty or Opsgenie (L-218) | WL | 0.25 (2 h) | 134, 174 | — |
| 185 | Entra group to team mapping (L-238; **UNVERIFIED** whether the Hub plan has it) | WL | 0.3 (2.5 h) | 174 | — |
| 186 | the in-app changelog; making it public is KPMG's call (L-254) | WL | 0.25 (2 h) | 174 | — |
| 187 | WL's proof and results | WL | 0.3 (2.5 h) | 175-186 | — |

---

## Task scopes

Each task's scope paragraph lives outside this file, in
`docs/superpowers/plans/2026-09-25-phase-w-work-layer-scopes/`, one file per group of 20 ids. A
part `task-NN.md` is written from its scope paragraph and this skeleton's row; where the two
disagree, the row's id, dependsOn and seams win and the scope is corrected.

| Scope file | Ids | Slices |
|---|---|---|
| `tasks-01-20.md` | 01-20 | W0, W1 (the Hub, phase 4 deltas, the test kit) |
| `tasks-21-40.md` | 21-40 | W1 (phase 3 deltas, Start, import, the look, the Work lens) |
| `tasks-41-60.md` | 41-60 | W1 (the lens, Triage, the issue view, the editor, keys, the replica, peek) |
| `tasks-61-80.md` | 61-80 | W1, W1b, W2 (Start on the desktop, the Desk, the demo, *shipped*, W2's Hub) |
| `tasks-81-100.md` | 81-100, 81b, 96b | W2, W3 (W2's Hub and desktop, W2's proof, W3's Hub) |
| `tasks-101-120.md` | 101-120 | W3, W4 (the verbs and Fleet on issues, W3's proof, W4's Hub) |
| `tasks-121-140.md` | 121-140 | W4 (janitor, insights, the timeline, release, assists) |
| `tasks-141-160.md` | 141-160 | W4, W5 (W4's proof; requests, sync, outside PRs, webhooks, the API) |
| `tasks-161-180.md` | 161-180 | W5, WL (MCP, loops, CI, Teams, audit, W5's desktop and proof; the first *later* rows) |
| `tasks-181-187.md` | 181-187 | WL (the remaining *later* rows and WL's proof) |
