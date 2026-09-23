# Dependency graph and phase-order recommendation — phases 3, 4, 5, 7

**Date:** 2026-09-23
**Inputs:** the three surveys in this directory (`survey-phase-2-substrate.md`, `survey-phase-4-discover.md`, `survey-phase-7-deploy.md`), plus direct reads of the tree at `feat/phase-2-cockpit`.
**Deliverable class:** a planning artifact, not an implementation plan. `CLAUDE.md`'s gate — "the product owner reviews prototypes before implementation plans, and plans before code" — means nothing below authorises writing a task.

---

## 0. Evidence standard, and the gap in the inputs

Every edge below is labelled with how it is known:

| Label | Meaning |
|---|---|
| **VERIFIED** | I read the file and line named. Cited with `path:line`. |
| **ASSERTED** | A survey claims it; I did not open the file. Carried through with the survey's own citation. |
| **INFERRED** | Neither survey states it; I reasoned it from two facts that are each verified. Flagged in place. |

I ran no build, no `go test`, no `npm test`. Nothing here is behavioural evidence.

**The gap, stated first because it constrains section 3.** The task brief said four surveys. Three arrived. **There is no Phase 5 — Design canvas survey.** Everything this document knows about Phase 5 comes from four lines in the substrate survey plus `PROGRAM.md:94` and `CLAUDE.md`. That is a material asymmetry: phases 4 and 7 were surveyed to the line number, phase 5 was not surveyed at all. A recommendation that moved phase 5 would be a recommendation made on one-third of the evidence used to leave it alone. Section 3 says so plainly rather than pretending the three surveys cover the question they were asked.

---

## 1. The dependency graph

### 1.1 The picture

```
                    ┌───────────────────────────────────────────┐
                    │  PHASE 2 — substrate (IN FLIGHT: tasks 23, 24 open)   │
                    │  Go side CLOSED · desktop side MOVING     │
                    └───────────────────────────────────────────┘
                       │              │              │        │
        ┌──────────────┘     ┌────────┘       ┌──────┘        └────────┐
        │ 11 symbols         │ 7 symbols      │ 2 symbols              │ 1 symbol
        ▼                    ▼                ▼                        ▼
   ┌─────────┐          ┌─────────┐      ┌─────────┐             ┌──────────┐
   │ PHASE 4 │          │ PHASE 7 │      │ PHASE 5 │             │ PHASE 3  │
   │ Discover│          │ Deploy  │      │ Design  │             │ Git/merge│
   └─────────┘          └─────────┘      └─────────┘             └──────────┘
        │  provenance.Ref     ▲                ▲                        │
        ├─────────────────────┘                │                        │
        │  provenance.Ref                      │                        │
        └──────────────────────────────────────┘                        │
                                                                        │
        ▲  Criterion state evidence (SOFT — degrades to all-Open)       │
        └────────────────────────────────────────────────────────────────┤
                                                                        │
             ▲  push/PR so the platform has a commit to build (INFERRED)│
             └───────────────────────────────────────────────────────────┘
                  (Phase 7 WRITE path only; read path is independent)

   §10.5 event stream  ──► required by PHASE 5 (multiplayer), optional for PHASE 7
   AGPL-3.0 clearance  ──► gates PHASE 5 — §13 network use, not only distribution (see PROGRAM.md Phase 5)
```

### 1.2 The three edges that actually matter

Ranked by what breaks if the edge is ignored.

**Edge 1 — Phase 4 → Phase 7 (and → Phase 5): `provenance.Ref`. HARD. Two independent surveys agree.**
Phase 4 mints `provenance.Ref` and serves it from `GET /api/v1/discover/branches/{branchID}/provenance`, which never 404s (absent brief ⇒ `Present:false`). Phase 7's survey lists it under `provenanceNeeds.reads` and states the direction explicitly: *"Phase 7 (Deploy) depends on Phase 4 and not the reverse, since ShippedManifest is built here and mounted there."* Phase 5 inherits the same edge from `CLAUDE.md` line 13 — "the spec written in Define is visibly referenced in Design". **ASSERTED** by both surveys; the type does not exist yet so it cannot be verified. Consequence: **Phase 4's provenance package must merge before Phase 5 or Phase 7 writes a line of read-surface code**, or they each invent a local shape and the spine forks into three.

**Edge 2 — Phase 2 → Phase 4: `Session.ContextRef`, plumbed but unrendered. HARD, and half-built.**
**VERIFIED** end to end: `control-plane/internal/registry/session.go:72-74` persists it with the comment "Phase 2 stores and returns it and never interprets it"; `control-plane/internal/api/sessions.go:65,96` read it and `:152,301` write it; `desktop/src/api/client.ts:96` carries it on create and `:359,381` propagate it through resume and fork; `desktop/src/store/defaults.ts:110,127` omit it when empty; `desktop/src/store/types.ts:163` types it. And `grep -rn contextRef desktop/src/components` returns **0** — **VERIFIED**. The carrier exists at every layer and the thread is visible on zero surfaces. It is also opaque by contract at both ends, so nothing today can turn a ref into a readable brief title.

**Edge 3 — Phase 2 → Phase 4: the Materialise hook that does not exist. BLOCKER.**
`grep -rn Materialis control-plane desktop` returns **nothing** — **VERIFIED absent**. `api.Deps` (`control-plane/internal/api/api.go:66`) has `Version`, `Backend`, `BackendReady`, `Store` and more, and no `Materialiser`. Go contract §5.1 reserved the space between worktree-resolve and `backend.Create` **in prose only**. Phase 4's own plan's only Materialise trigger is the desktop calling `POST /briefs/{id}/materialise`. So the ordering guarantee "CLAUDE.md is on disk before the PTY starts" is **client-enforced only**, today, with no error path. The failure is silent: a session starts on an un-materialised worktree, the agent inherits nothing, and the feature looks built. This is the one edge that blocks *writing* a Phase 4 task, not just executing one.

### 1.3 Full edge table

| From → To | What crosses | Evidence | Hardness |
|---|---|---|---|
| P2 → P4 | `worktree.Provider.Path` / `worktree.Static` / `project.Scanner.Path` | **VERIFIED** `worktree/worktree.go:33,43`; `project/project.go:339` | hard, satisfied |
| P2 → P4 | `Scanner.ProjectName` / `BranchName` | **VERIFIED** `project/project.go:172,175` | hard, satisfied |
| P2 → P4/P7 | `statepath.Dir(sub)` | **VERIFIED** package present | hard, satisfied |
| P2 → P4/P7 | `/api/v1/` StripPrefix alias | **VERIFIED** `api/api.go:274` (+ comment at `:269-273`) | hard, satisfied |
| P2 → P4/P7 | `Session.ContextRef` | **VERIFIED** (see Edge 2) | hard, satisfied as a carrier |
| P2 → P4 | `api.Deps` Materialiser field | **VERIFIED ABSENT** | **BLOCKER** |
| P2 → P4 | `registry.writeAtomic` five-retry helper | **VERIFIED** `registry/filestore.go:460`, lowercase ⇒ unreachable | hard, unsatisfied |
| P2 → P4 | `LensRouter` discover arm | **VERIFIED** `AppShell.tsx:26-33` | hard, satisfied |
| P2 → P4 | `useCockpit` store needs a `discoverSlice` | **VERIFIED** store exists; slice does not | hard, unsatisfied |
| P2 → P4 | `SessionPaneHeader` as the ContextChip mount | **ASSERTED** (survey; UI contract §11) | hard, satisfied |
| P2 → P4 | 28 token names in `tokens.css` | **ASSERTED** (UI contract §5.4) | hard, satisfied |
| P2 → P4 | `desktop/vitest.config.ts` must not be overwritten | **VERIFIED** file exists with `setupFiles`, `clearMocks`, `css:{include:[/\.css\?raw$/]}` | hazard, see §2.3 |
| P2 → P7 | `LensRouter` deploy arm | **VERIFIED** `AppShell.tsx:58-65` — and it says `phase={5}` | hard, satisfied; label wrong |
| P2 → P7 | recorded route table (`routes.Handle`) | **ASSERTED** `api/api.go:126-172` | hard, satisfied |
| P2 → P7 | `httpx` has no outbound client | **ASSERTED** | soft — P7 adds the first one |
| P2 → P5 | `LensRouter` design arm | **VERIFIED** `AppShell.tsx:34-41` | hard, satisfied |
| P2 → P5 | **no event stream exists** (§10.5 requests one) | **VERIFIED** `api/poller.ts:1-3`; contract §10.5 at ui-contract`:2291` | **hard, unsatisfied** |
| **P4 → P7** | `provenance.Ref` + never-404 endpoint | **ASSERTED**, both surveys, consistent | **hard** |
| **P4 → P5** | `provenance.Ref` (brief visible in Design) | **ASSERTED** from `CLAUDE.md` spine | hard |
| P3 → P4 | `Criterion.State` Settled/Contested evidence | **ASSERTED** (P4 survey risk 7) | **SOFT — degrades to all-Open** |
| P3 → P4 | real worktree create/destroy behind `Provider.Path` | **VERIFIED** substitutable — `worktree.Static` + `Scanner.Path` satisfy it today | **SOFT — substituted** |
| P3 → P7 | a pushed commit the platform can build | **INFERRED** — no survey states it | hard for the **Ship** path only |
| P6 → any | nothing | — | none found |

**The finding in that table:** there is no hard edge from Phase 3 into Phase 4. Both candidate edges are soft. Phase 4's plan (`plans/2026-09-18-phase-4-discover.md`, lines 42 and 2139) assigns the CLAUDE.md-before-PTY guarantee to Phase 3 — that assignment is **wrong against the code**, because Phase 3 does not own session start. Phase 2's `handleSessionCreate` does, and it has shipped.

**The edge nobody wrote down:** P3 → P7 for the Ship path. `panacea-api` builds a *commit on a branch it can reach*. The cockpit's branch is a local worktree. Getting worktree → pushed remote is Phase 3's PR/push work. Phase 7's read path (status, versions, scans, `LiveReadyReplicas`) has no such dependency. **INFERRED, not verified — I made no network call and read no push code.** It is the reason section 4 splits Phase 7 in two.

---

## 2. The shared-file collision surface

### 2.1 Counted

"Phases" counts *distinct phases that must edit the file*, Phase 2 included because it is live.

| File | P2 | P3 | P4 | P5 | P7 | **n** | Merge character |
|---|:--:|:--:|:--:|:--:|:--:|:--:|---|
| `desktop/src/components/AppShell.tsx` | ● | ● | ● | ● | ● | **5** | **SERIALISING** |
| `desktop/src/components/AppShell.test.tsx` | ● | ● | ● | ● | ● | **5** | **SERIALISING** |
| `docs/superpowers/contracts/…phase-2-ui-contract.md` | ● | ○ | ● | ● | ● | **4** | **SERIALISING** (verbatim-diff gate) |
| `desktop/src/store/types.ts` | ● | ○ | ● | ● | ● | **4** | **SERIALISING** (gated by contract §2) |
| `control-plane/internal/api/api.go` | ● | ● | ● | ○ | ● | **4** | **SERIALISING** (`Deps` struct) / append (routes) |
| `desktop/src/store/selectors.ts` | ● | ● | ● | ● | ● | **5** | append-only — parallel |
| `desktop/src/api/client.ts` | ● | ● | ● | ● | ● | **5** | append-only — parallel |
| `desktop/src/components/palette/commands.ts` | ● | ● | ● | ● | ● | **5** | append to one array — semi |
| `desktop/src/components/subrail/SubRail.tsx` | ● | ○ | ◐ | ◐ | ◐ | **2–5** | one shared array — semi, optional |
| `desktop/src/styles/tokens.css` | ● | ○ | ◐ | ◐ | ◐ | **1–4** | check-first ⇒ read-only in practice |
| `docs/PROGRAM.md` | ● | ● | ● | ● | ● | **5** | prose — semi |
| `control-plane/cmd/api/main.go` | ● | ● | ● | ○ | ● | **4** | dep wiring — semi |
| `control-plane/internal/api/routes_audit_test.go` | ● | ● | ● | ○ | ● | **4** | two maps — append |
| `control-plane/internal/statepath/statepath.go` | ● | ● | ● | ○ | ● | **4** | new subkeys — parallel |
| `desktop/src/store/store.ts` + `slices/` | ● | ● | ● | ● | ● | **5** | new file per slice — parallel; root-key adds are **SERIALISING** |
| `control-plane/internal/api/sessions.go` | ● | ○ | ● | ○ | ○ | **2** | **SERIALISING** — the Materialise call site |
| `control-plane/internal/registry/filestore.go` | ● | ● | ● | ○ | ○ | **3** | `writeAtomic` lift — one-shot |
| `desktop/vitest.config.ts` | ● | ○ | ● | ○ | ○ | **2** | **HAZARD — whole-file overwrite** |
| `desktop/package.json` | ● | ○ | ● | ● | ○ | **3** | deps — semi |
| `desktop/src/components/develop/SessionPaneHeader.tsx` | ● | ○ | ● | ○ | ○ | **2** | one insertion — sequence it |
| `control-plane/internal/registry/session.go` | ● | ○ | ○ | ○ | ○ (read) | **1** | no collision |
| `desktop/src/App.tsx` | ● | ○ | ○ | ○ | ○ | **1** | Phase 2 task 24 only |

● edits · ◐ may edit · ○ reads or untouched. P3 and P5 columns are **INFERRED** — neither has a survey; they are scored from what a phase of that shape must touch.

### 2.2 The one that is genuinely serialising: `AppShell.tsx`

**VERIFIED** by reading the file. Five phases converge on 106 lines:

- Phase 2 still owns two reserved insertion points — `:76-81` (`useKeyboardMap()`, task 23) and `:100-103` (`<CommandPalette />`, task 23).
- Phases 3, 4, 5 and 7 each replace a different `case` body in the same `switch` and each add one `import` at the top.

The `case` bodies auto-merge. **The import block does not** — four phases adding a line to the same contiguous region is four conflicts, and `AppShell.test.tsx` has the identical property because it pins the placeholder strings each phase deletes.

**This is avoidable in one 30-minute task.** Before any later phase takes a worktree, do a single **lens-seam** task that replaces all four remaining placeholder arms with four thin stub components — `DiscoverLens`, `DesignLens`, `FilesLens`, `MergeLens`, `DeployLens` — each in its own new file, each rendering the `LensPlaceholder` it renders today, with the phase label corrected. After that, `AppShell.tsx` is closed for the programme and every later phase owns one file nobody else opens. It converts a certain four-way conflict into one small sequenced change. The substrate survey proposes the same thing in its open questions; this is an endorsement, not a new idea.

### 2.3 Two hazards that are not conflicts, they are silent deletions

**`desktop/vitest.config.ts` — VERIFIED.** The file exists and carries three settings Phase 2 depends on: `setupFiles: ["./src/test/setup.ts"]`, `clearMocks: true`, and `css: { include: [/\.css\?raw$/] }`. That last one has a 9-line comment explaining that without it Vitest rewrites `tokens.css?raw` to `export default ""` and **eight token assertions pass against an empty string**. Phase 4's task 10 step 3 creates this file outright. That is not a merge conflict — Git takes the new file cleanly and the test suite goes green against nothing. The UI contract §11 flagged this and marked it "Flagged for whoever finalises the Phase 4 plan"; nobody has actioned it. Fix is one word: *check* first, do not *create*.

**`store.ts` root keys.** `zustand`'s persist shallow-merges at the root, so a nested slice silently drops keys added after a snapshot. Any phase adding a root key must bump `PERSIST_VERSION` and write a migration. Two phases doing that concurrently produce two version-2 migrations. **ASSERTED** from the substrate survey; not verified.

### 2.4 Collision verdict

- **Serialising (needs an owner and a sequence):** `AppShell.tsx`, `AppShell.test.tsx`, `types.ts` + the UI contract it is diffed against, `api.Deps` inside `api.go`, `sessions.go`, `vitest.config.ts`, root-key additions to `store.ts`. **Seven.**
- **Parallelisable (append-only or new-file):** `selectors.ts`, `client.ts`, `statepath.go`, new store slices, `routes_audit_test.go`, `tokens.css` under check-first. **Six.**
- **Non-issues:** `session.go`, `project.go`, `worktree.go`, `App.tsx`.

The serialising set is small and four of the seven are closed by the single lens-seam task plus one "check, don't create" wording fix.

---

## 3. Recommendation

### **Keep the order. It is right.**

`1 live session · 2 cockpit · 3 Git/worktrees/convergence · 4 Discover · 5 Design canvas · 6 Capacity · 7 Deploy connector · 8 Kata · 9 hardening.`

That is a confirmation, stated plainly, and it is the answer. Three amendments follow, and **none of them renumbers anything.**

### 3.1 Why the no-spend constraint does not discriminate here

The order was built by "how much can be proven without spending money". Check it against the table: phases 3, 4 and 5 are "No"; phase 7 is "Needs the Panacea API, not Azure directly" — and the Panacea dev estate already exists and is already funded, so phase 7's *marginal* spend is zero too. **Phases 3, 4, 5 and 7 are all inside the no-spend envelope.** The constraint that built the order is silent on how they are sequenced among themselves. So reordering within that set costs nothing against the constraint — which means the constraint cannot be used to defend the current order either. The defence has to come from the tiebreak: risk retired per unit of effort.

### 3.2 Why risk-first also says keep it

The tempting move is to pull Deploy (7) in front of Design (5), because Deploy terminates the provenance spine and the spine is the product. It does not survive contact with the two surveys:

1. **Phase 7 is the cheap one.** Its own survey: *"a Go HTTPS client plus a polling loop against an API that already enforces all RBAC… far cheaper than expected on the read path."* Every capability the brief names already exists as a route. Low risk, low effort — that is a *late* position under risk-first, not an early one.
2. **Phase 5 is the expensive unknown.** Adopting a third-party AGPL codebase into our Tauri shell, with MCP agent-editing and multiplayer, and getting a handoff bundle out the other side, is the largest unretired technical risk left in the no-spend envelope. Risk-first puts it before Deploy, which is where it is.
3. **Phase 7's blockers do not get better by moving it.** All four are decisions or cross-repo asks: no client-settable deploy metadata, `sanitizeBranchEnv` truncating to 20 chars, no public/PKCE Keycloak client, an unresolved base URL. Not one of them is unblocked by building sooner. They are unblocked by *asking* sooner — which is amendment 3, and it is free.
4. **I cannot responsibly move Phase 5 on one-third of the evidence.** There is no Phase 5 survey. Phases 4 and 7 were surveyed to the line number and 5 was not surveyed at all. Moving the unsurveyed phase on the strength of surveys that do not cover it is the failure mode this planning pass exists to avoid.

### 3.3 The one real constraint violation, and why I am not fixing it

By the order's own rule, **phase 7 should precede phase 6.** Phase 7 can be proven end to end with zero new spend; phase 6 cannot — its quota tracking and rotation need real subscriptions, which is why its column says "Partly". A strict reading of "ordered by how much can be proven without spending money" puts Deploy at 6 and Capacity at 7.

I recommend **not** acting on it now, and recording the reason so it is not re-litigated:

- The benefit lands at a phase that is not being planned in this pass, and neither 6 nor 7 starts for some time.
- The cost is a second renumber of Deploy within 24 hours of `6bcc9af`, plus a sweep of `PROGRAM.md`'s table *and* prose, `AppShell.tsx`'s five `phase={n}` props, and the titles and filenames of three survey documents written today.
- Churn of exactly this kind is what produced the two-orderings defect `6bcc9af` just repaired.

**Decide it at the phase-5-close checkpoint**, in one change that moves the table and the prose together. Until then the deviation is *known and accepted*, which is different from *undetected*.

### 3.4 The three amendments

**Amendment 1 — run phases 3 and 4 concurrently, not in sequence.**
There is no hard edge from 3 into 4 (§1.3). Phase 4's only worktree need, `Provider.Path`, is **VERIFIED** satisfied today by `worktree.Static` and `project.Scanner.Path`. Phase 4 never touches git and never reads a CLI. Its one genuine 3-shaped need — `Criterion.State` reaching Settled or Contested — is already designed to degrade to all-Open, labelled and deliberate. The `3 → 4` arrow in the existing Phase 4 plan is an artefact of that plan misassigning session-start ownership to Phase 3. This is the single largest schedule win available and it changes no phase number.

**Amendment 2 — Phase 4's `provenance` package is slice zero, merged before 5 or 7 starts.**
`provenance.Ref`, the never-404 endpoint, and the "a new field goes into `provenance.Ref`, never computed locally" contract line. Small, self-contained, and it is the shared type on Edge 1. Merging it first means Design and Deploy build read surfaces against a real type instead of three stubs that drift.

**Amendment 3 — raise Phase 7's external asks now, at planning time.**
They have other people's lead time attached and consume none of ours:
1. An additive `metadata map[string]string` on `TriggerDeployOptions` in `panacea-api` — without it the spine's last rung is held in our state dir, invisible in the Panacea portal, lost if the directory is lost.
2. A public/PKCE Keycloak client in realm `panacea` — without it Phase 7 is capped at paste-a-token onboarding.
3. AGPL-3.0 clearance for Paperdesign — **corrected 2026-09-23: not a gate on distribution alone.** Under the locked we-host model, **§13 Remote Network Interaction** can oblige us to offer source the moment a user's browser interacts with our modified copy, with no distribution involved. The question for legal is §13 under a hosted model. It may therefore gate phase 5 **shipping to any user**, not only external sale — and the answer takes calendar time, so ask now.
4. One unauthenticated `GET /api/v1/version` to settle `api.dev.panacea.kpmg` against `dev.panacea.kpmg/api`. Two written sources disagree; no network call was made by anyone in this pass.

### 3.5 The decision that must be made before any Phase 4 task is written

**Who calls Materialise.** Edge 3. Three options, ranked, matching the Phase 4 survey:

1. **Server-side `Materialiser` dep in `api.Deps`, called between worktree-resolve and `backend.Create` in `handleSessionCreate`.** Structural — the guarantee cannot be bypassed by any client. Costs: one field, one validation branch in `api.New`, tests, and it inverts the package dependency unless a thin `Materialiser` interface is *declared in `api`* and *satisfied by `contextwrite`*. Declare it in `api`. **Recommended.**
2. Treat `contextRef` as the precondition — materialise when the managed block is absent or stale. Also structural; more state to reason about.
3. Leave it client-enforced plus an honest warning on the session header. Cheapest, and it leaves the documented silent failure in place.

Options 1 and 2 put `sessions.go` under Phase 4's pen. Because tasks 23 and 24 touch **zero** control-plane files (**VERIFIED**: `grep -c "control-plane/"` returns 0 in both task docs), that file is already quiet — Phase 4 can take it without contending with Phase 2.

---

## 4. What can genuinely run in parallel

### 4.1 Gate zero, before any worktree is cut

| # | Action | Owner | Why it is first |
|---|---|---|---|
| 0a | Phase 2 tasks 23 and 24 land, and task 24's **first live run** happens | Phase 2 | §5 — task 24 is the first time anything reads from the control plane |
| 0b | **Lens-seam task** on the integration branch: five placeholder arms → five stub components, `AppShell.tsx` closed, `phase={5}` on deploy corrected | Phase 2 closeout | Kills the only certain four-way conflict (§2.2) |
| 0c | Materialise ownership decided (§3.5) | orchestrator | Blocks writing Phase 4 tasks at all |
| 0d | `writeAtomic` lifted into `statepath` or duplicated — decide which | orchestrator | Decides whether Phase 4 edits a Phase 2 file |
| 0e | Phase 4 task 10 step 3 reworded from *create* to *check* for `vitest.config.ts` | whoever finalises the Phase 4 plan | §2.3 — silent test deletion |

0b and 0e are each under an hour. 0c is a decision, not work.

### 4.2 The worktree plan

| Worktree | Contents | Starts | Contends on |
|---|---|---|---|
| **A** | **Phase 3** — worktree lifecycle, claims, diff, merge, PR | after gate zero | `api.go` route table, `cmd/api/main.go`, `selectors.ts` — all append-only |
| **B** | **Phase 4** — Discover, with `provenance` as slice zero | after gate zero, **concurrent with A** | `api.go` `Deps` struct, `sessions.go`, `filestore.go`, `SessionPaneHeader.tsx` |
| **C** | **Phase 7a** — read path only: the Go client, status/versions/scans, the Deploy lens as a read surface | after B's slice zero merges | `client.ts`, `statepath.go` — append-only |
| **D** | **Phase 5** — Design canvas | **blocked**, see 4.4 | unknown — unsurveyed |
| — | **Phase 7b** — the Ship write path | after A merges | the inferred P3→P7 edge (§1.3) |

A and B genuinely do not contend once `AppShell.tsx` is closed. Their only overlap is `api.go` — and it splits cleanly: Phase 3 appends routes to the recorded table, Phase 4 adds one field to `Deps`. Give each phase a marked, contiguous block in both the route table and the `Deps` struct and the conflicts become mechanical.

C is the interesting one. Phase 7's read path needs nothing from Phase 3 and, once amendment 2 lands, nothing further from Phase 4. It is the cheapest phase in the programme and it can be absorbed into a gap without disturbing the order, because *running work in parallel is not the same as reordering phases.* Recommend C is the overflow lane, not a promotion.

### 4.3 Merge order

```
gate zero (0a–0e, on the integration branch)
        │
        ├─► B slice 0 : provenance.Ref + never-404 endpoint     ── merge FIRST
        │
        ├─► A : phase 3          ─┐
        ├─► B : rest of phase 4  ─┤ either order; api.go conflicts are mechanical
        │                         │
        ├─► C : phase 7a read path  (after B slice 0)
        │
        ├─► phase 7b ship path      (after A)
        │
        └─► D : phase 5             (after its two gates clear — 4.4)
```

`provenance` merging first is the load-bearing constraint. Everything else is a preference.

### 4.4 What cannot parallelise, and why

- **Phase 5 cannot take a worktree yet.** Two gates, neither technical-in-the-canvas: (i) AGPL-3.0 clearance — `PROGRAM.md:94`, legal, not engineering; (ii) **there is no event stream**. `poller.ts:1-3` states it: "There is no event stream in the Go contract — §10.5 asks for one — so Phase 2 polls." Multiplayer is push-shaped. Either §10.5 lands in the Go contract *before* Phase 5, or the app runs two transports in one window with two reconnect ladders and two failure vocabularies. That is a control-plane decision, and it must be made by whoever owns the Go contract, not by whoever adopts the canvas. **VERIFIED** the stream is absent; **ASSERTED** that multiplayer needs it.
- **Phase 7b cannot precede Phase 3** if the inferred P3→P7 edge holds. Verify it cheaply before planning 7b: does shipping a cockpit branch require our own push, or does the platform build from a branch it already tracks? One reading of the Phase 3 scope answers it.
- **Anything touching `types.ts` cannot parallelise**, because it is gated by a verbatim diff against UI contract §2 — so a domain field addition is a contract amendment in the same change, and two concurrent amendments to one contract section is a manual merge every time. Budget the amendment into any task that touches the model, and apply shared-shape §3D's sweep rule: grep every mention, re-read every *cell* of a row you touch, and check the fenced code block under the table, which is a second copy of the row.
- **Phase 6 is not analysed here.** No survey, not in scope, no edges found into 3, 4, 5 or 7.

---

## 5. When Phase 2's contract is settled enough to build against

Split the answer, because the two halves have different answers.

### 5.1 The Go control plane is settled **now**

**VERIFIED**, and this is the most actionable finding in the document: `grep -c "control-plane/"` returns **0** in both `task-23.md` and `task-24.md`. **No remaining Phase 2 task touches a Go file.**

Every Go symbol phases 4 and 7 consume is therefore final:

| Symbol | File | Status |
|---|---|---|
| `worktree.Provider.Path` / `Static` | `worktree/worktree.go:33,43` | settled |
| `Scanner.ProjectName` / `BranchName` / `Path` | `project/project.go:172,175,339` | settled |
| `statepath.Dir` | `statepath/statepath.go` | settled |
| `/api/v1/` alias | `api/api.go:274` | settled — and carries a DO-NOT-DELETE comment |
| `Session.ContextRef` | `registry/session.go:72-74` | settled; pinned by `filestore_test.go`'s persisted-key set |
| `registry.writeAtomic` | `registry/filestore.go:460` | settled but **unexported** — decision 0d |

Go-side Phase 4 work — `discoverstore`, `contextwrite`, `claudemd.Merge`, the `provenance` package, the three route handlers — can be planned against today's tree with no stale-citation risk. That is amendment 2's slice zero, and it is unblocked apart from decision 0c.

### 5.2 The desktop is **not** settled, and the gate is task 24's first live run

Task 24's plan names, among others: `App.tsx`, `App.css` (deleted), `api/client.ts`, `api/index.ts`, `api/poller.ts`, `api/attachRegistry.ts`, `store/store.ts`, `store/slices/prefsSlice.ts`, `components/AppShell.tsx`, `components/develop/*`, `components/palette/*`, `hooks/useKeyboardMap.ts`, plus two new audit suites. Every one of those is a surface phases 4, 5 and 7 cite.

The reason the gate is the **run**, not the merge, is the substrate survey's own top risk and it is correct: **the cockpit has never run.** `App.tsx` is still the placeholder saying "The cockpit mounts here in task 24". Nothing mounts `AppShell`, nothing starts the poller, nothing loads the catalogue. Task 24's plan says so itself — nothing before it ever *lists* a machine, project, branch, worktree or harness. Every quality claim about the substrate is `tsc` plus `vitest`. Per `CLAUDE.md`, that is not evidence it works.

Planning against a substrate whose first live run has not happened means one integration defect lands on three phases at once. The volatility is measured, not theoretical: commits touching `desktop/src` landed at 08:59, 09:56, 10:21, 10:47, 11:19, 11:35, 12:14, 12:23, 12:46, 13:22, 13:47 and 13:54 **today** — roughly one every thirty minutes through the working day.

### 5.3 The gate, in one line each

| Surface | Safe to build against | Condition |
|---|---|---|
| Go control plane | **now** | none — tasks 23 and 24 touch no `.go` file (**VERIFIED**) |
| `AppShell.tsx` | after the lens-seam task (0b) | which itself is after task 23 |
| `types.ts`, `store.ts`, `selectors.ts`, `client.ts`, `poller.ts` | after task 24 **runs** | not after it merges |
| `components/develop/*`, `SessionPaneHeader.tsx` | after task 24 runs | moved four times today |
| `tokens.css` | now, **read-only** | check-first; adding a token is a contract amendment |
| the whole substrate, as a claim | after task 25's real-window E2E | the only behavioural evidence in the programme |

**Recommended gate:** plan Go-side Phase 4 now; hold every desktop-side task for phases 4, 5 and 7 until task 24 has run against a live control plane and task 25 exists as a plan. Task 25 is referenced (`task-05.md:1457` defers Docker questions Q3 and Q4 to it) but **no `task-25.md` exists** — `phase-2-parts/` holds tasks 05 through 24 only. Nobody has written the verification task that everything downstream is waiting on. **No definition of a task 26 was found anywhere in `docs/`**, so the "26 tasks" figure is still unreconciled.

---

## 6. Open questions this pass could not close

1. **Who calls Materialise** (§3.5) — blocks writing any Phase 4 task. Recommend option 1.
2. **Is there a Phase 5 survey coming?** Section 3's confidence in leaving Design at position 5 is bounded by its absence.
3. **Does §10.5 (the event stream) land before Phase 5?** If yes, it is a slice in front of 5 and it is a Go-contract change. If no, Phase 5 ships a second transport.
4. **P3 → P7 (Ship path)** — INFERRED, unverified. Settle it before planning 7b.
5. **Which prototype is owner-approved?** `prototype/` holds seven variants (`app`, `index`, `v2`–`v6`), five containing Discover, all committed 2026-09-17. Phase 4's plan cites `app.html:964-987` as the approved Discover interaction. If `app.html` is not the approved one, that spec reference is wrong — and `CLAUDE.md`'s gate is "the product owner reviews prototypes before implementation plans". **ASSERTED** from the Phase 4 survey; I did not open `prototype/`.
6. **Tasks 25 and 26** — 25 is referenced and unwritten, 26 is unfound. Does either gate Phase 3?
