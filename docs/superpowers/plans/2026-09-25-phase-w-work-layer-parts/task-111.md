### Task 111: named risk facts on lanes; the repository proposal; Orchestrator-filed issues into `triage`

Stream **W3** (agents on the work), about 3 hours (an estimate, not a measurement). Depends on
task 100 (the W3 Hub guards: *"an issue created by a delegated caller is forced to `triage` with
`source: orchestrator`"*, `tasks-81-100.md:485-486`). Uses phase 3's lane index and changed paths
(`control-plane/internal/converge`, P3 tasks 07-09; skeleton delta row `:282`; **UNVERIFIED until
merge**), phase 3's Merge lens `LaneCard.tsx` (P3 task 19; **UNVERIFIED until merge**), task 48's
`IssueAgentLine.tsx`, task 42's `TriageView.tsx`, task 101's `verbs/issue.go`, task 103's
`AgentByline`, task 107's `AgentTaskCard`, and TM-05 §6.3's task scope globs and leases.

Three rows of 05 §1, each of which Linear prices as a number and we price as a fact:

1. **L-174** (`05-design.md:336`): *"Named risk facts, not a score (AX anti-pattern 12): touches
   prod config, runs a migration, edits outside declared scope (AX §7.6)"*.
2. **L-192** (`:359`): *"A project already knows its repos; with several, the Orchestrator proposes
   one, with no confidence number unless calibrated"*.
3. **L-193** (`:360`): *"An Orchestrator may file an issue (`source: orchestrator`, 04 §2.4); it
   lands in `triage` for a person"*.

**Decisions made here, so nobody re-derives them:**

1. **Three names, closed.** `touches-prod-config`, `runs-migration`, `outside-declared-scope`.
   Each fact carries the paths that raised it and one `because` line the product assembles from
   the glob that matched (*"matches `deploy/prod/**` in this project's prod-config globs"*). It is
   never a model's words. A fourth name is a code change with its own test.
2. **Mechanical only** (AX §7 point 6, `AGENTIC-UX.md:194`: *"Detected mechanically as divergence
   between declared intent and observed action"*). A fact is a glob match on a changed path, or a
   changed path outside every scope glob and lease of the task. No weighting, no ranking between
   facts, no *high/medium/low*, no colour scale, no number (`AGENTIC-UX.md:247`, anti-pattern 12).
   The facts render in the fixed order of decision 1, not by severity.
3. **Derived, never stored.** The facts are computed on read from the lane's changed paths and
   the globs, on the local control plane, and are never written to the issue or the Hub (skeleton
   rule 3). A saved view cannot filter by them in W3.
4. **The globs belong to the project.** Prod-config and migration globs come from the project's
   work settings; **no default globs are shipped**, so an unconfigured project shows no
   `touches-prod-config` or `runs-migration` fact, and the lane card says *"no risk globs set for
   this project"* once, as a fact. Where the settings live on the Hub is **UNVERIFIED** (gap 1).
5. **Declared scope exists only where a task declares it.** `outside-declared-scope` needs the
   task's scope globs and its leases (TM-05 §6.3), so it appears on team projects only; a solo
   lane has no declared scope and never carries it.
6. **Glob syntax is `doublestar`'s** (`github.com/bmatcuk/doublestar/v4`, MIT), a new dependency
   of the control-plane module, `**` included, matched on forward-slash repository paths. If
   Phase T's lease globs (TM-05 §6.3) land with a different matcher, this part switches to it so
   one glob means one thing across the product.
7. **The repository proposal is a field of the Orchestrator's task, dropped on a one-repo
   project.** With one bound remote there is nothing to propose and Fleet drops the field. With
   several, the proposal names one remote the project binds and a `because` (`text·A`, at most
   300 characters). A `confidence` value is kept **only** with a `calibrationId` that resolves;
   no calibration record exists in W3, so in W3 every confidence is dropped (anti-pattern 12).
   Whether a Hub project binds several remotes today is **UNVERIFIED** (gap 2).
8. **An Orchestrator files through one added verb, `issue.file`**, beside task 101's three. The
   verb has no `state`, `assignee`, `priority` or `cycle` argument at all, so an Orchestrator
   cannot ask for them; the Hub forces `triage` and `source: orchestrator` whatever the body says
   (task 100). The scope's *"create outside `triage` is refused"* is therefore tested as *stored
   in `triage`*, task 100's behaviour, and the difference is named (gap 3).
9. **An Orchestrator never assigns, starts or moves an issue.** No verb offers it (dispatch
   refuses an unknown verb), and the Hub refuses a delegated caller's assign and state move; the
   Hub row is asked of task 100 (gap 4) and run here as conformance rows against the fake and the
   real Hub.
10. **Triage shows who filed it.** An Orchestrator-filed row carries task 103's byline
    (`alice · orchestrator (agent)`), the `text·A` mark on title and description, and the
    Orchestrator's `because`. Accepting it is task 42's accept, done by the person; the revision
    stays `text·A` until a person edits it.

**Files:**
- `control-plane/internal/work/riskfacts.go` (+ `_test.go`) — the derivation (new; **path
  UNVERIFIED**, not in the file map)
- `control-plane/internal/api/workrisk.go` (+ `_test.go`), `api.go` ⚑, `routes_audit_test.go` ⚑ —
  `GET /v1/work/lanes/{lane}/risk` on loopback (new, modified)
- `control-plane/go.mod` ⚑, `go.sum` ⚑ — `doublestar/v4`, MIT (modified)
- `desktop/src/components/work/RiskFacts.tsx` (+ test) — the facts line (new)
- phase 3's `desktop/src/components/lens/merge/LaneCard.tsx` ⚑ and task 48's
  `desktop/src/components/issue/IssueAgentLine.tsx` ⚑ — mount `RiskFacts` (modified)
- `control-plane/internal/fleet/work/repo.go` (+ `_test.go`) — the repository-choice rule (new)
- `desktop/src/components/work/RepoProposal.tsx` (+ test), mounted in task 107's
  `AgentTaskCard.tsx` ⚑ (new, modified)
- `control-plane/internal/fleet/verbs/issue.go` ⚑ (task 101's), TM-19's dispatch table ⚑,
  `testdata/panacea-control/v1/issue.file.*.json` — the `issue.file` verb (modified, new)
- `desktop/src/components/work/TriageView.tsx` ⚑ (task 42's) — the Orchestrator-filed row (modified)
- `control-plane/internal/hub/hubclient/conformance/work_agents.go` ⚑ (task 99's) — three rows,
  appended under rule H1 (modified)
