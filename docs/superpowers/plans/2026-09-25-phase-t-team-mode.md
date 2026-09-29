# Phase T: Team mode (Fleet, Orchestrators across owners)

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking. Each task's full text is in `2026-09-25-phase-t-team-mode-parts/task-NN.md`.

**Goal:** Two people share one project, and each runs their own agents in the cloud from their own
desktop. Every agent can read what the others are doing: their task, the brief section it serves,
its status, the files the platform observed it touching, and its leases. An overlapping plan is
caught **before** a worker is spawned. Each person's Orchestrator settles it with the other
person's Orchestrator through typed messages, or escalates to its own human, to **inform** or to
**decide**. Every instruction to an agent is a `dv-…` record that answers *"who asked for this?"*.
No user steers another user's agent (OD-16), no personal subscription is used (OD-19), and the
desktop is the portal (OD-20).

It is delivered in three slices, in order:

- **TM1-α** (about two weeks, the first slice of TM1): the three parts nothing has run yet, thin,
  on the real cluster. The `panacea-orchestrator` runner in a diskless Orchestrator pod; the
  **policy mechanism** (OD-23), by which the control plane puts a versioned, audited Claude Code
  managed-settings file into every cloud sandbox at session start; and Fleet as a delegated Hub
  caller. Proven by one scripted conflict between two identities, through to one `inform` and one
  `decide`, plus the pod-kill test. **If TM1-α fails, TM1 is re-planned before its remaining tasks
  start** (`05-design.md` §12.0).
- **TM1**: two people, one repo, agents that know each other. Done when `05-design.md` §12.1's
  eight criteria pass on the real environment with two real people.
- **TM2**: one document. The shared lane mirror in its own per-project Kata pod, tier M forecasts,
  one team land queue, `counter` / `split` / `build-on`, the phase 3 lenses by principal, the CLI
  bump job, team and project quotas, and phase 7's release attribution.

TM3 (a live Orchestrator, the watch view, speculative builds, an MCP front) is **not** in this plan.

**Architecture:** one new managed service, Fleet, beside the Hub; the desktop is a client of both.

```
desktop (portal)                                our environment (AKS, Australia East only)
┌──────────────────────────┐                   ┌────────────────────────────────────────────────┐
│ cockpit: team strip,     │  REST + ws        │ fleetd (2 stateless replicas)                  │
│ Orchestrator panel, Desk │ ───────────────►  │  core: cards, leases, threads, escalations,    │
│ local control plane      │  (fleetbe behind  │        taint rule, budgets, forecaster P/H(/M) │
│  └ fleetbe (Backend)     │  backend.Backend) │  relay: attach handler, /control stream        │
└──────────┬───────────────┘                   │  Postgres schema: leases, turn queues,         │
           │ Entra PKCE (OD-6)                 │    threads, policy versions, Hub outbox        │
           ▼                                   └──┬────────────┬──────────────┬─────────────────┘
┌──────────────────────────┐  delegated caller    │ broker     │ /control     │ per-session token
│ Hub: records, feed,      │ ◄────────────────────┘ (2nd caller│ relay        ▼
│ audit → Sentinel         │  (names a dl-…, azp  │ delegation │        ┌──────────────────┐
│ task, dv, ot, hm, lease… │   = Fleet identity)  │ checked    │        │ LiteLLM gateway  │
└──────────────────────────┘                      │ ≤ 60 s)    │        │ (phase 6, OD-21) │
                                                  ▼            ▼        │ Global │ AU      │
                           ┌─────────────────────────────────────────┐  └────▲─────────────┘
                           │ Kata pods, one owner each               │       │ private path
                           │  orchestrator class: diskless, runner   │───────┘ one-hour,
                           │  worker class: machine disk, agent CLI  │         pod-bound token
                           │  sandboxd: /attach /control /healthz    │
                           │  panacea binary, Bash(panacea *) only   │
                           │  managed-settings.json from policy v-N  │
                           └──────────────┬──────────────────────────┘
                                          │ machined sweep, outside the VM, every 5-10 s
                                          ▼
                                   Fleet activity table (observed files, never agent words)
```

Four rules shape every task:

1. **Agents see facts, not each other's words.** Files touched are observed by `machined` outside
   the sandbox. Workers never read a teammate's free text; Orchestrators read it only as data.
2. **Turn origin governs authority.** When Fleet, not the owner, started an Orchestrator turn, the
   Orchestrator may issue only the **typed** verbs at its own workers — pause, resume, narrow,
   wait-for, rebase, none of which can make a worker do something new — and anything else (free
   text) becomes a `decide` to its human (`05-design.md` §7.2). One definition, task 03's
   `DirectiveKind.Typed()`, serves both Fleet's check and the Hub's (fix batch b07, G31-1).
3. **Fleet holds no Hub record durably.** It rebuilds Hub-owned state from the Hub after a restart;
   its own schema holds Fleet state and the outbox of Hub-bound writes, nothing else
   (`05-design.md` §11.1, `ADR-H1:467-474`).
4. **Agent CLIs are never modified, wrapped or parsed.** Policy reaches Claude Code only through
   the configuration files it already reads; PTY bytes carry no meaning to any component (OD-8).

**Tech stack:** as phase 2 (Go 1.23, `github.com/coder/websocket`, React 19 + TypeScript, Vitest),
plus phase 8's AKS, Helm and Terraform, and phase 6's LiteLLM gateway (OD-21).

**Spec:** `docs/superpowers/planning/2026-09-24-team-collaboration/05-design.md` (revision 3, its
§15 critique map), `00-summary.md`, `docs/PROGRAM.md`, `docs/AGENTIC-UX.md`, `docs/DESIGN-SYSTEM.md`.

---

## The decisions this plan rests on

The owner decisions in `docs/superpowers/planning/2026-09-23-owner-decisions.md` win over the
design, and the design wins over every older plan. Where the design and a later decision
disagree, the decision wins and the task that carries it says so.

| Id | What it fixes for this plan | Where it lands |
|---|---|---|
| **OD-14** | Team agents run centrally, as managed Kata sessions Fleet coordinates; the desktop is a client. Reverses ADR-H1's "sessions stay local" for team projects and phase 8's owner-only broker visibility. | 11, 12, 17, 19–21 |
| **OD-15** | Agent awareness first; humans get a calm summary (who, one line, files). No teammate terminal by default. | 22, 35, 51–53 |
| **OD-16** | One Orchestrator per user is the only cross-user channel; it settles or escalates to **inform** or **decide**. No user steers another's agent. | 26–31, 62 |
| **OD-17** | Design now, build next; the Hub, phase 8 and phase 3 are re-scoped where team mode touches them. In-flight streams stay foundations. | every delta task |
| **OD-18** | Azure. Phase 8 resumes unchanged in shape. | 15, 17, 32, 47 |
| **OD-19** | Inference through the org gateway, Global (Anthropic, default) or AU (Bedrock Sydney), a scoped per-session token per sandbox, never an org key, never a personal subscription. | 20, 32, 47, 56 |
| **OD-20** | The desktop is the portal; local mode stays; the cloud must track the latest Claude Code CLI. | 21, 65 |
| **OD-21** | The gateway is **LiteLLM**. It resolves the design's Q1: this plan calls phase 6's key-minting API and builds no gateway. | 20, 37 |
| **OD-22** | A **read-only, per-project fetch credential**, used only for fetches; every push uses a signed-in person's own credential. It resolves the design's Q11. | 48, 58 |
| **OD-23** | **The mechanism, not the policy.** The control plane supplies Claude Code managed settings per org, and per project where needed, into every cloud sandbox at session start, versioned and audited. A minimal default ships; no InfoSec policy document is drafted. It **supersedes** the design's "product `managed-settings.json` derived from the tenant's live file and submitted to KPMG InfoSec" (`05-design.md` §2.5, §11.2, Q12) as a TM0 item and TM1 gate. | 13–15, 33 |
| **OD-24** | Budgets on by default; escalation defaults only tighten; opening prompts private; a data class **set at project creation**, updatable, driving AU vs Global and the deploy scan set. It resolves the design's Q2–Q5. | 22, 30, 37, 43, 56 |
| **OD-25** | Phase W runs alongside; team mode's critical path is untouched by it. Nothing here waits on W. | — |
| **OD-27** | Proof per criterion, no predictions: no percentages, progress bars, ETAs or "N of M" in any team surface. | 51–56, 67 |
| **OD-28** | Ids, paths and times in Commit Mono; sans Segoe UI Variable. | 51–56, 67 |
| **OD-2, OD-4** | One clone per worktree; Azure DevOps and GitHub both, so the fetch credential has two providers. | 48, 64 |
| **OD-3, OD-6** | Durable team records live in the Hub; the desktop signs in with Entra PKCE. | 01–08, 21 |
| **OD-8** | No component derives meaning from PTY bytes; agent CLIs are never modified, wrapped or parsed. The only PTY write Fleet makes is a fixed template (task 38). | 13, 27, 38 |
| **OD-11** | Clean room: no canvas or Linear source is opened or copied. Phase 5's artboard lease is out of scope here. | — |
| **OD-13** | No Panacea source is copied; patterns only. | every task |

---

## Global constraints

Every task inherits these. A part that breaks one must say so and say why.

**Where the code is built.** Phase T is written on `feat/team-mode`. Three of its groups extend code
that lives, today, on a sibling branch that is not merged: the Hub (`feat/hub`, worktree
`.claude/worktrees/hub`), phase 8 (`env`, worktree `.claude/worktrees/env`) and phase 3
(`.claude/worktrees/phase-3`). **A delta task starts only after the sync-merge of its sibling
branch into `feat/team-mode`**, and it names, in its part file, the exact sibling file it extends.
Where that file does not exist yet on the sibling (for example phase 8's broker, `machined` and
`remotebe`, tasks 17–22, unbuilt; phase 3's `gitx` and `lanes`, unbuilt), the part cites the
sibling **plan** row and marks the path **UNVERIFIED until merge**. No delta task copies sibling
code onto this branch ahead of the merge.

**Records and state (`05-design.md` §3.2).**

1. If losing it would lose a decision or an accountability fact, it is a **Hub** record:
   revisioned, one metadata-only audit event each, on the change feed. If it is a fact about now,
   it is **Fleet** state in Fleet's own Postgres schema. Activity and presence are the only
   in-memory state.
2. Fleet persists no Hub record (`ADR-H1:467-474`). Its schema holds leases, turn queues, open
   thread working state, policy versions and the Hub outbox. After a restart it rebuilds the rest
   from the Hub.
3. The owner comes from the token or the delegation, **never from a request body**. Only an
   owner's own agents write that owner's rows. Nobody writes another person's task, lease,
   message, directive or escalation, whatever their role (§3.3 rule 5). **Fix batch b06
   (S02-C6):** a `dl-…` *named in a request body* is a name, not proof of anything. Whoever
   resolves one does it through task 11's `Feed.Adopt` (b06 round 2: task 20's `Launch` today, task 24's spawn next), which checks the delegation's own owner, org, project and
   **acting** class against the authenticated caller before acting under it or caching it (a spawned worker acts under the `orchestrator` class; a worker delegation never covers an Orchestrator), because the Hub
   makes the delegation's owner the principal (task 05) and route 47 hands Fleet any delegation it
   presents (`task-01.md:314`). Otherwise naming someone else's id *is* becoming them. The attempt is audited: `Adopt` names the caller in route 47's `for`, and the Hub records `delegation.named_by_another`.
4. Records carry `ag-…` ids. No machine-local session id, host path, config key or URL reaches
   the Hub. Paths are repo-relative globs, validated server-side.
5. Every free-text field is bounded (the limits in §3.3), hygiene-checked, and carries its author
   class, `text·A` or `text·H`.

**Trust.**

6. **Agents see facts, not words.** Activity is observed by `machined` outside the VM. A worker
   never receives a teammate's free text; an Orchestrator receives it only as quoted data.
7. **Turn origin governs authority** (§7.2). In a Fleet-started turn an Orchestrator may issue only
   the five typed verbs at its own workers — pause, resume, narrow, wait-for, rebase; free text
   becomes a `decide`. The check is in Fleet's verb handler (task 31), not in the prompt, and the
   Hub re-checks it (task 07) against the **same** `DirectiveKind.Typed()`, so the two layers cannot
   drift (b07, G31-1/S01-I9).
8. **No git credential enters a sandbox.** Fetch uses the project's read-only credential (OD-22),
   held by the broker. Push and PR use the lane owner's own token, per act, in memory only, and stay
   human actions.
9. **No org key enters a sandbox.** A sandbox holds only a one-hour, pod-bound gateway token,
   fetched through `apiKeyHelper` (`panacea token`), never an environment variable.
10. **Policy reaches Claude Code only through files it already reads** (OD-23, OD-8). The control
    plane writes the managed-settings file before the harness starts; it never wraps, patches or
    parses the CLI, and no component reads meaning from PTY bytes.
11. Fleet is a network service: every WebSocket upgrade is authorised per request, per owner, and
    every socket is revocable within 60 seconds (§7.4). Phase 2's loopback attach handler is not
    reused for it.

**Region and routing.** Fleet, its Postgres, the lane mirror, the gateway and its usage store, and
every sandbox run in **Australia East** (§2.5 invariant). An AU project never makes a Global call,
and fails closed if the AU route is down.

**Engineering.**

- Go 1.23 (the `go` directive does not move), `github.com/coder/websocket` for every socket, the
  standard library `net/http` mux, `pgx/v5` for Fleet's Postgres (**UNVERIFIED** that the Hub
  branch already pins it; if so, match its version).
- Ids come from phase 2's `control-plane/internal/ids` (`crypto/rand`), with the prefixes in §3.3.
- Every error body is phase 2's `httpx` problem shape. Every new route joins the Z5 role-matrix
  test (§3.4) in the task that adds it.
- Tests first. Each test names the mutant it kills. `go test ./...` and `go vet ./...` pass in
  every module a task touches; the desktop's `npm run typecheck` and `npm test` pass for UI tasks.
- UI tasks read `docs/DESIGN-SYSTEM.md` and `docs/AGENTIC-UX.md` first. One global heartbeat
  clock; `tabular-nums`; no spinners per row; no percentages, bars, ETAs or "N of M" (OD-27);
  ids, paths and times in Commit Mono (OD-28). **A prototype is owner-reviewed before its
  implementation task starts** (`CLAUDE.md` working agreements).
- Budgets are on by default, escalation floors only tighten, opening prompts are private, and a
  project has a data class from creation (OD-24).
- State what is unverified. A task that cannot run on the real cluster says which acceptance step
  is deferred to task 33 (TM1-α), 57 (TM1) or 69 (TM2), and why.

---

## File map

`cp/` is `control-plane/` (module `github.com/kpmg-au-emu/panacea-app/control-plane`); `sbx/` is
`sandboxd/` (its own module, vendored). `go.work` joins the two, and **neither imports the other**
(checked: no `panacea-app/sandboxd` import under `control-plane/`), so a shape both speak is written
twice and pinned by one set of golden fixtures that both modules' tests read.

### New on `feat/team-mode`

| Path | Responsibility | Task |
|---|---|---|
| `cp/internal/teamtext/{text.go,hygiene.go,glob.go}` | Bounded text with its author class (`text·A`, `text·H`); G3 hygiene (C0/C1, ANSI, bidi, U+E0000–E007F, whitespace runs in names, secret-shaped text); repo-relative glob validation, the 200-character path cap and the 10% breadth check | 02 |
| `cp/internal/fleet/model/*.go` | Fleet's domain: `Class`, `Agent`, `Lease`, `Event`, `Turn`, the per-class verb table | 09 |
| `cp/internal/fleet/wire/control.go`, `sbx/internal/control/wire.go`, `testdata/panacea-control/v1/*.json` | The `panacea` control wire v1, one copy per module, one fixture set | 09 |
| `cp/internal/fleet/store/{store.go,memstore.go,pgstore.go}`, `.../store/storetest/suite.go`, `.../store/migrations/0001_fleet.sql` | Fleet's Postgres schema and the `Store` both implementations pass | 10 |
| `cp/internal/fleet/outbox/outbox.go` | The durable Hub outbox, idempotent delivery, the 10-minute negotiation pause | 11 |
| `cp/internal/fleet/delegation/{client.go,feed.go,rebuild.go}` | Fleet as a delegated Hub caller; the per-org feed; revocation within 60 s; rebuild after restart | 11 |
| `cp/cmd/fleetd/{main.go,config.go}`, `cp/internal/fleet/server/{server.go,authz.go}` | The chassis: `PANACEA_APP_FLEET_*`, Entra per request, role-to-verb, singleton loops | 12 |
| `cp/internal/policy/{policy.go,merge.go,digest.go}`, `.../policy/defaults/{worker,orchestrator,mirror}.json` | **OD-23's mechanism**: layers, merge, invariants, digest; the minimal defaults | 13 |
| `cp/internal/policy/store.go`, `cp/internal/fleet/server/policy.go` | Versioned, append-only policy per org and project; the admin routes; the audit event | 14 |
| `sbx/internal/policycheck/policycheck.go` | `sandboxd` refuses to start the harness unless the mounted managed file matches its digest | 15 |
| `sbx/internal/control/{server.go,socket.go,snapshot.go}` | `/control`, the in-pod Unix socket that stamps the calling session, the `team.json` tmpfs snapshot | 16 |
| `sbx/internal/panaceacli/*.go` | Every `panacea` verb but `token`, over the socket; the unavailable answer | 18 |
| `cp/internal/fleet/control/{streams.go,dispatch.go}` | One control stream per sandbox through the broker; verb dispatch by class and owner | 19 |
| `cp/internal/fleet/sessions/{sessions.go,tokens.go}` | Launch keyed by owner over the broker; `ag-…` ids; gateway tokens with their tags | 20 |
| `cp/internal/backend/fleetbe/fleetbe.go`, `cp/internal/api/teamlaunch.go`, `cp/internal/api/teamrelay.go`, `cp/internal/registry/teamdelegations.go` | The desktop's `fleetbe` behind `backend.Backend`; the team launch that creates the worker **and** the per-project orchestrator delegation (b07); the four `/v1/team/*` human relays (b07) | 21 |
| `cp/internal/fleet/verbs/{team.go,check.go,files.go}` | Read verbs, filtered by class | 22 |
| `cp/internal/fleet/leases/{leases.go,rightofway.go}`, `cp/internal/fleet/verbs/lease.go` | The lease engine | 23, 35 |
| `cp/internal/fleet/verbs/{task.go,worker.go,report.go}` | Task, worker and report verbs | 24 |
| `cp/internal/fleet/forecast/{tierp.go,tierh.go,tierm.go,coedit.go}` | The forecaster, tier by tier | 25, 36, 60 |
| `cp/internal/fleet/turns/{queue.go,origin.go,template.go}` | The per-Orchestrator event queue, turn origin, the fixed turn template | 26 |
| `sbx/cmd/panacea-orchestrator/main.go`, `sbx/internal/runner/{runner.go,rotate.go,dive.go}` | The Orchestrator runner | 27, 40 |
| `cp/internal/fleet/protocol/{messages.go,inbox.go,ask.go,limits.go,resolve.go}`, `cp/internal/fleet/verbs/{peer.go,inbox.go}` | Threads, message kinds, the templated `ask`, limits, resolutions | 28, 29, 62 |
| `cp/internal/fleet/escalate/{escalate.go,rules.go}`, `cp/internal/fleet/verbs/human.go` | Inform and decide, the §6.5 rows, Fleet-set defaults and deadlines | 30 |
| `cp/internal/fleet/directive/{directive.go,taint.go}`, `cp/internal/fleet/verbs/direct.go` | Directives, the taint rule, piggyback delivery | 31 |
| `deploy/charts/fleet/**` | Fleet's chart: two replicas, PDB, workload identity, NetworkPolicy | 32 |
| `infra/smoke/team/*.sh`, `docs/verification/2026-09-25-phase-t-alpha-results.md` | TM1-α's scripted run on the real cluster, and its record | 33 |
| `cp/internal/fleet/activity/{activity.go,presence.go}` | The 45-second activity table, presence, holds from observed edits | 35 |
| `cp/internal/fleet/budget/{budget.go,counterpart.go}` | Turn budgets, human bypass, per-counterpart cap and digest, the spend split | 37 |
| `cp/internal/fleet/ptywriter/ptywriter.go`, `cp/internal/fleet/delivery/ladder.go` | The template-only PTY writer and the delivery ladder | 38 |
| `cp/internal/fleet/relay/{attach.go,revoke.go}` | Fleet's own attach handler and socket revocation | 39 |
| `cp/internal/fleet/instructions/{orchestrator.md,worker-cloud.md}`, `cp/internal/fleet/eval/*`, `testdata/team-events/*.json` | The Orchestrator's instructions and the replay eval | 41 |
| `cp/internal/fleet/g9/{corpus/*.txt,g9_test.go,run.go}` | G9's corpus and the scripted two-Orchestrator run | 42 |
| `prototype/team-mode.html` | The owner-reviewed prototype of every team surface | 51 |
| `cp/internal/api/team.go`, `desktop/src/api/team.ts`, `desktop/src/store/slices/teamSlice.ts`, `desktop/src/store/types.ts` | Team read models and actions on the local control plane; the desktop client. It **extends** task 21's four `/v1/team/*` relays (b07), which exist from TM1-α: it widens the answer relay's `200` to `DecideItem` and registers none of the four again; it no longer introduces the first of them | 52 |
| `desktop/src/components/team/*` | The team strip and click-in | 53 |
| `desktop/src/components/desk/team*.tsx` | The four Desk row kinds | 54 |
| `desktop/src/components/orchestrator/*` | The Orchestrator panel | 55 |
| `desktop/src/components/team/{RouteBadge,ShareLine,SpendLine}.tsx` | Route badge, Share line, spend split | 56 |
| `docs/verification/2026-09-25-phase-t-tm1-results.md` | TM1's two-person proof | 57 |
| `cp/internal/fleet/mirror/{client.go,result.go}`, `sbx/cmd/panacea-mirror/main.go` | The lane mirror's typed results and its in-pod runner | 59 |
| `cp/internal/fleet/landq/landq.go` | The team land queue over the mirror | 61 |
| `cp/internal/api/lanesmove.go` | "Move my lanes to the cloud" | 64 |
| `desktop/src/components/team/QuotaAdmin.tsx` | Quotas admin view | 67 |
| `docs/verification/2026-09-25-phase-t-tm2-results.md` | TM2's proof | 69 |

### Deltas on sibling code, built here after that branch's sync-merge

"Built" means the file exists on the sibling branch as read on 2026-09-25. "Planned" means only
the sibling **plan** names it; the part cites the plan row and marks the path **UNVERIFIED until
merge**.

| Sibling | Path it extends | State on 2026-09-25 | What this plan adds | Task |
|---|---|---|---|---|
| Hub (`feat/hub`) | `docs/superpowers/adr/ADR-H1-hub-architecture.md` (`:252-253`, `:370-372`, `:434-435`, `:467-474`) | built (doc) | the "delegated callers" amendment | 01 |
| Hub | `docs/superpowers/contracts/2026-09-23-hub-contract.md` §5.3, §7 (a new §7.13 onward), §8 | built (doc) | the team records' routes, matrix rows and events | 01 |
| Hub | `cp/internal/hub/hubapi/{types.go,codes.go,keys.go}` | built (Hub tasks 01–03) | `team.go`: every team record and request; the new id prefixes and codes | 03 |
| Hub | `cp/internal/hub/migrate/sql/0001_init.sql` | planned (Hub task 07a/07b) | `00NN_team.sql`, numbered after the Hub's last migration at merge | 04 |
| Hub | `cp/internal/hub/server/auth.go` (Hub task 13), `cp/internal/devissuer/*` (tasks 11–12) | planned | delegated-caller authentication; a client-credentials grant | 05 |
| Hub | `cp/internal/hub/server/*.go`, `cp/internal/hub/store/*.go` (the Hub task 14 write shape) | planned | `delegations.go`, `teamtasks.go`, `agents.go`, `leaseevents.go`, `conflicts.go`, `threads.go`, `escalations.go`, `directives.go`, `turns.go`, `humanmsgs.go`, `notes.go`, `teampolicy.go` | 06, 07, 43, 46 |
| Hub | `cp/internal/hub/store/changes.go` (Hub task 26) | planned | the new feed kinds and a per-org subscription for Fleet | 08 |
| Hub | `cp/internal/hub/hubclient/{hubclient.go,fake*.go,conformance/conformance.go}` | built (tasks 04–05), HTTP half planned (33–34) | team methods, the fake's team half, conformance rows | 08 |
| Hub | `cp/internal/hub/server/matrix_integration_test.go` (Hub task 16b) | planned | a row per team route, per principal, incl. *auditor* and *compliance* | 43 |
| Hub | Hub task 27's audit route and chain | planned | the Sentinel stream and the anchored chain head | 45 |
| Hub | Hub tasks 23–24's release records | planned | per-lane attribution on a team release | 68 |
| Phase 8 (`feat/env`) | `cp/internal/sandbox/manager.go` (`Spec` at `:69-83`) | built (task 14, `09ae70a`) | `Spec.Class`; the `orchestrator` and `mirror` shapes; the policy Secret mount; the control token | 15, 58 |
| Phase 8 | `sbx/internal/server/server.go` (`:85-86` serve `/attach`, `/healthz` only) | built (phase 2) | the `/control` route beside them | 16 |
| Phase 8 | `cp/cmd/broker/*` (tasks 17–19) | planned | Fleet as a second caller under a delegation; the `control` relay kind; each pod's source address | 17 |
| Phase 8 | `deploy/charts/platform/templates/*` (task 11), `deploy/charts/{broker,hub}` (task 29) | planned | the sandbox→gateway allow; the Orchestrator class's egress; the AU allowlist; the mirror pod's zero egress | 32, 47, 58 |
| Phase 8 | `infra/terraform/modules/keyvault` (task 05), `stacks/workloads` (task 10) | planned | Fleet's identity; the fetch credential's secret, readable by the broker only | 32, 48 |
| Phase 8 | `cp/cmd/machined/*` (tasks 20–21) | planned | the activity sweep; lane bundle export to the mirror | 34, 60 |
| Phase 8 | `pipelines/images.yml` (task 27) | planned | the CLI bump job and its four gates | 65 |
| Phase 8 | `infra/smoke/kata/*` (task 32) | planned | the five added probes (§11.2 last row) | 47 |
| Phase 3 (`feat/phase-3-git`) | `cp/internal/converge/{claims.go,forecast.go,land.go}` (tasks 07–09) | planned; no `gitx` or `lanes` code yet | the sweep as a library; the forecast as a library; the team land review | 34, 49, 59 |
| Phase 3 | `cp/internal/remote/*` (task 12), `cp/internal/githost/*` (tasks 13–14) | planned | the read-only fetch credential in the cloud | 48 |
| Phase 3 | `cp/internal/changes/*` (task 16), `desktop/src/components/{merge,files}/*` (tasks 19, 21) | planned | conflicts in the land review; lenses by Hub principal | 49, 63 |
| Phase 4 (`feat/phase-4-discover`) | `cp/internal/claudemd/render.go` | built | the fixed team pointer paragraph | 50 |
| Phase 6 (`feat/phase-6-gateway`) | `sbx/cmd/panacea/main.go`, `images/sandbox/managed-settings.gateway.json` (task 14, seam S6.6) | planned, plan in progress | the policy layer merges the fragment; every other verb joins the binary | 13, 18 |
| Phase 6 | `cp/internal/gatewayclient/client.go` (task 15), keysmith's API (task 08) | planned | Fleet mints per-session tokens with team tags | 20 |
| Phase 6 | `cp/internal/budget/*`, `cp/internal/usage/*` (tasks 06, 09), `desktop/src/components/inference/*` (task 22) | planned | per-counterpart tags; team quotas; the route badge reuses the panel's state | 37, 56, 66 |

---

## Task list

Each task's full text is in `2026-09-25-phase-t-team-mode-parts/task-NN.md`. A focused day is
8 hours. **dependsOn** names this plan's tasks by number and sibling tasks by prefix: `H` the Hub
plan (`.claude/worktrees/hub/docs/superpowers/plans/2026-09-23-phase-h-hub.md:401-446`), `P8-` phase
8 (`.claude/worktrees/env/docs/superpowers/plans/2026-09-23-phase-8-own-environment.md:553-622`),
`P3-` phase 3 (`.claude/worktrees/phase-3/docs/superpowers/plans/2026-09-23-phase-3-git-convergence.md:591-614`),
`P6-` phase 6 (`.claude/worktrees/phase-6/docs/superpowers/plans/2026-09-25-phase-6-gateway.md:159-182`,
still being written), `P4` phase 4's merged Materialiser. A sibling task is a dependency only once its
branch is sync-merged here.

**Seams** this plan defines, so parallel writers agree without reading each other:

| Seam | What is fixed | Defined in | Consumed by |
|---|---|---|---|
| **ST.1** | the `panacea` control wire v1: request, response and event JSON, verb names, the error envelope, the fixture set | 09 | 16, 18, 19, 22–31, 38 |
| **ST.2** | the Hub's team records: ids, fields, bounds, routes, feed kinds, audit event names | 01, 03 | 04–08, 11, 43, 46 |
| **ST.3** | delegation: the `X-Panacea-Delegation` header, the actor rule, the `ag-…` owner check | 01, 05 | 11, 17, 20, 21, 44 |
| **ST.4** | the policy layers, `policy.Render`, the digest and `PANACEA_APP_POLICY_SHA256` | 13 | 14, 15, 41 |
| **ST.5** | the turn: the fixed template, `turn.begin` / `turn.end` on the control stream, the session id rule | 26 | 27, 40 |
| **ST.6** | the activity sample `machined` sends: paths, hunks, base blob, last commit, all capped | 34 | 35, 36, 60 |
| **ST.7** | the mirror's typed result: tier, paths, line ranges, basis shas | 59 | 60, 61 |
| **ST.8** | the team read model the cockpit renders, and its author-class labels | 52 | 53–56, 63, 67 |

Sibling seams used: **S8.1** `backend.Backend` unchanged (phase 8); **S6.6** the managed-settings
fragment and **S6.3** the mint API (phase 6); **S6.2** budgets; **P8-D4** and **P8-D12**.

### TM1-α: the three untried parts, thin, on the real cluster

| Id | Title | Slice | Focused days | dependsOn | Seams |
|---|---|---|---|---|---|
| 01 | ADR-H1 "delegated callers" amendment and the team-records contract | TM1-α | 0.375 (3 h) | Hub merge | ST.2, ST.3 |
| 02 | `teamtext`: bounded text, author class, G3 hygiene, globs and breadth | TM1-α | 0.3 (2.5 h) | — | — |
| 03 | Hub wire types for team records; id prefixes and codes | TM1-α | 0.3 (2.5 h) | 01, 02, H01–H03 | ST.2 |
| 04 | Hub migration: team tables, RLS, append-only guards, tombstone columns | TM1-α | 0.375 (3 h) | 03, H06, H07a, H07b | ST.2 |
| 05 | Hub delegated-caller authentication; `devissuer` client-credentials grant | TM1-α | 0.375 (3 h) | 04, H11–H13 | ST.3 |
| 06 | Hub routes, part 1: delegations, tasks, agents, lease events, conflicts; route 87, a project's mode (b01); route 88, the principal read, route 47's broker caller and the five `auth.go`/`delegation.go` deltas that gate them (b02) | TM1-α | 0.5 (4 h) | 05, H14, H17 | ST.2, ST.3 |
| 07 | Hub routes, part 2: threads, messages, escalations, directives, turns, human messages | TM1-α | 0.375 (3 h) | 06 | ST.2 |
| 08 | Hub feed kinds, Fleet's per-org subscription; `hubclient` team methods, fake, conformance | TM1-α | 0.375 (3 h) | 07, H26, H33, H34 | ST.2 |
| 09 | Fleet model and the `panacea` control wire v1 | TM1-α | 0.3 (2.5 h) | 02 | ST.1 |
| 10 | Fleet's Postgres schema and `Store` | TM1-α | 0.375 (3 h) | 09 | — |
| 11 | Fleet as a delegated Hub caller: outbox, delegation client, feed, rebuild | TM1-α | 0.375 (3 h) | 08, 10 | ST.3 |
| 12 | `cmd/fleetd`: config, Entra per request, role-to-verb, singleton loops | TM1-α | 0.375 (3 h) | 01, 06, 08 (b02: the principal read), 10, 11, P8-16 | — |
| 13 | OD-23 mechanism, part 1: layers, merge, invariants, digest, minimal defaults | TM1-α | 0.375 (3 h) | — (P6-14's fragment once merged) | ST.4, S6.6 |
| 14 | OD-23 mechanism, part 2: versioned store, admin routes, audit, version on agent start | TM1-α | 0.4 (3.5 h) | 06, 08, 12, 13 | ST.4 |
| 15 | Phase 8 delta: sandbox classes (task 14b), the policy mount, `sandboxd`'s digest check | TM1-α | 0.375 (3 h) | 13, P8-14, P8-15 | ST.4 |
| 16 | Phase 8 delta: `sandboxd` `/control`, the in-pod socket, the `team.json` snapshot | TM1-α | 0.375 (3 h) | 09 | ST.1 |
| 17 | Phase 8 delta: the broker's second caller, the control relay, pod source addresses | TM1-α | 0.375 (3 h) | 06 (b02: route 47's broker caller), 16, P8-17, P8-18 | ST.3, P8-D4 |
| 18 | The `panacea` verbs in the sandbox binary | TM1-α | 0.3 (2.5 h) | 16, P6-14 | ST.1 |
| 19 | Fleet's control streams and verb dispatch | TM1-α | 0.375 (3 h) | 12, 17 | ST.1 |
| 20 | Fleet sessions over the broker; `ag-…` ids; gateway tokens with team tags | TM1-α | 0.375 (3 h) | 11, 14, 15, 19, P6-15, P8-22 | S6.3, S8.1 |
| 21 | `fleetbe` and the team launch on the local control plane | TM1-α | 0.5625 (4.5 h; b07 added the orchestrator delegation and the four human relays) | 06, 07 (b07: the four relays' Hub routes), 08, 20, H35 (b01) | S8.1, ST.3 |
| 22 | Read verbs: `team`, `check`, `files`, `show`, filtered by class | TM1-α | 0.375 (3 h) | 19, 20 | ST.1 |
| 23 | Lease engine: intent, right of way, breadth cap, request and release | TM1-α | 0.375 (3 h) | 10, 22 | ST.1 |
| 24 | Task, worker and report verbs; intent leases at plan time | TM1-α | 0.3 (2.5 h) | 23 | ST.1 |
| 25 | Tier-P forecaster; conflict to thread | TM1-α | 0.3 (2.5 h) | 07, 24 | — |
| 26 | The turn queue, turn origin and the fixed template | TM1-α | 0.375 (3 h) | 10, 25 | ST.5 |
| 27 | The `panacea-orchestrator` runner, and the pod-kill test | TM1-α | 0.375 (3 h) | 15, 16, 18, 26 | ST.5 |
| 28 | Protocol messages: `peer`, `inbox`, the templated `ask`, limits | TM1-α | 0.375 (3 h) | 25, 26 | ST.1 |
| 29 | Resolutions `yield`, `sequence`, `proceed-both`; DECIDED WITHOUT YOU | TM1-α | 0.3 (2.5 h) | 23, 28 | — |
| 30 | Escalations: inform and decide, the §6.5 rows, Fleet-set default and deadline | TM1-α | 0.375 (3 h) | 28, 29 | — |
| 31 | Directives and the taint rule; piggyback delivery | TM1-α | 0.375 (3 h) | 26, 30 | ST.1 |
| 32 | Phase 8 delta: Fleet's chart, the private gateway path, the Orchestrator's egress | TM1-α | 0.375 (3 h) | 12, P8-11, P8-29, P6-19 | P8-D12 |
| 33 | TM1-α on the real cluster: the scripted conflict, pod kill, hook and policy checks; the replan gate | TM1-α | 0.375 (3 h) | 01–32 | — |

**If task 33 fails, TM1 is re-planned before any task from 34 on starts** (`05-design.md` §12.0).

### TM1: two people, one repo, agents that know each other

| Id | Title | Slice | Focused days | dependsOn | Seams |
|---|---|---|---|---|---|
| 34 | Phase 3 delta: `machined`'s activity sweep | TM1 | 0.375 (3 h) | 17, P3-07, P8-20, P8-21 | ST.6 |
| 35 | Activity and presence; holds from observed edits, bulk, per-path expiry, releases | TM1 | 0.375 (3 h) | 23, 34 | ST.6 |
| 36 | Tier-H forecasts and co-edit down-weighting | TM1 | 0.25 (2 h) | 25, 35 | — |
| 37 | Budgets: turn budget, human bypass, per-counterpart cap and digest, the spend split | TM1 | 0.375 (3 h) | 20, 26, P6-09 | S6.2 |
| 38 | The template-only PTY writer and the delivery ladder | TM1 | 0.3 (2.5 h) | 19, 31 | — |
| 39 | Fleet's own attach handler and socket revocation | TM1 | 0.375 (3 h) | 11, 12, 19 | — |
| 40 | Runner rotation and DIVE | TM1 | 0.3 (2.5 h) | 27, 39 | ST.5 |
| 41 | The Orchestrator's instructions and the replay eval | TM1 | 0.375 (3 h) | 13, 31 | ST.4 |
| 42 | G9: the injection corpus and the scripted two-Orchestrator run | TM1 | 0.375 (3 h) | 31, 38, 41 | — |
| 43 | Hub delta: role-to-verb, *auditor* and *compliance*, team policy, Z5 rows | TM1 | 0.375 (3 h) | 07, H16b | — |
| 44 | Hub delta: leavers, member removal, delegation revocation end to end | TM1 | 0.3 (2.5 h) | 06, 11, 39 | ST.3 |
| 45 | Hub delta: audit to Sentinel; the anchored chain head | TM1 | 0.3 (2.5 h) | H27, 06 (b01) | — |
| 46 | Hub delta: notes and erasure tombstones | TM1 | 0.25 (2 h) | 07, H27 | ST.2 |
| 47 | Phase 8 delta: AU egress allowlist, the Australia East guard, task 32's five probes | TM1 | 0.375 (3 h) | 32, P8-32 | P8-D12 |
| 48 | Phase 3 delta: the read-only per-project fetch credential (OD-22) | TM1 | 0.375 (3 h) | 17, P3-12, P3-13, P3-14 | — |
| 49 | Phase 3 delta: the team land review: conflicts listed, a second approver for agent-config paths | TM1 | 0.375 (3 h) | 07, P3-09, P3-16, P3-19 | — |
| 50 | Phase 4 delta: the fixed team pointer paragraph | TM1 | 0.125 (1 h) | P4 | — |
| 51 | The prototype of every team surface, owner-reviewed | TM1 | 0.375 (3 h) | — | ST.8 |
| 52 | Team read models and actions on the local control plane; the desktop client | TM1 | 0.375 (3 h) | 21, 43, 51 | ST.8 |
| 53 | The team strip and click-in | TM1 | 0.375 (3 h) | 51 (approved), 52 | ST.8 |
| 54 | Desk rows: HORIZON, NEEDS YOU, SINCE YOU LAST LOOKED, DECIDED WITHOUT YOU | TM1 | 0.375 (3 h) | 52, 53 | ST.8 |
| 55 | The Orchestrator panel | TM1 | 0.375 (3 h) | 39, 40, 52 | ST.8 |
| 56 | Route badge, Share line, spend line; the data class at creation | TM1 | 0.3 (2.5 h) | 43, 52, P6-22 | ST.8 |
| 57 | TM1 proof: two people on the real environment, the eight criteria, the first-teammate gate | TM1 | 0.375 (3 h) | 33–56 | — |

### TM2: one document

| Id | Title | Slice | Focused days | dependsOn | Seams |
|---|---|---|---|---|---|
| 58 | Phase 8 delta: the lane mirror's per-project Kata pod | TM2 | 0.375 (3 h) | 15, 48 | P8-D12 |
| 59 | Phase 3 delta: the forecast as a library; the mirror runner and its typed result | TM2 | 0.375 (3 h) | 58, P3-08 | ST.7 |
| 60 | Tier-M forecasts; lane bundles into the mirror | TM2 | 0.3 (2.5 h) | 34, 36, 59 | ST.6, ST.7 |
| 61 | The team land queue over the mirror | TM2 | 0.375 (3 h) | 49, 60 | ST.7 |
| 62 | Resolutions `counter`, `split`, `build-on` | TM2 | 0.375 (3 h) | 29, 30, 60 | — |
| 63 | Phase 3 delta: merge and files lenses by Hub principal; teammates' lanes from the mirror | TM2 | 0.375 (3 h) | 60, P3-16, P3-19, P3-21 | ST.8 |
| 64 | "Move my lanes to the cloud" | TM2 | 0.3 (2.5 h) | 21, P3-24 | — |
| 65 | Phase 8 delta: the CLI bump job and its four gates | TM2 | 0.375 (3 h) | 42, P8-27, P8-27a | — |
| 66 | Team and project quotas and the spend split, server side | TM2 | 0.3 (2.5 h) | 37, 43, P6-06, P6-09 | S6.2 |
| 67 | The quotas admin view | TM2 | 0.3 (2.5 h) | 56, 66 | ST.8 |
| 68 | Release attribution on a team project (phase 7's thread) | TM2 | 0.25 (2 h) | 61, H23, H24 | — |
| 69 | TM2 proof | TM2 | 0.3 (2.5 h) | 58–68 | — |

**Totals:** TM1-α 33 tasks, 95 hours, about 12 focused days; TM1 24 tasks, 65.5 hours, about 8;
TM2 12 tasks, 32.5 hours, about 4. These are this plan's own tasks only. They exclude the sibling plans' tasks they depend on and
phase 6's gateway, and they are estimates, not measurements. The design's ≈ 95.5 and ≈ 16 focused
days (`05-design.md` §12.1, §12.2) count those sibling streams too, so the two figures do not
compare directly.

**TM1-α waves** (a task starts when everything it depends on is merged, sibling tasks included):
1 — 02, 13; 2 — 01, 09; 3 — 03, 10, 16; 4 — 04, 18; 5 — 05; 6 — 06; 7 — 07; 8 — 08; 9 — 11;
10 — 12; 11 — 14, 15, 17, 32; 12 — 19; 13 — 20; 14 — 21, 22; 15 — 23; 16 — 24; 17 — 25; 18 — 26;
19 — 27, 28; 20 — 29; 21 — 30; 22 — 31; 23 — 33. The Hub chain (01–08) is serial and gates
task 11; Fleet tasks 22–31 test against task 08's fake and task 10's `memstore`, so they need no
running Hub. Task 51 (the prototype) has no dependency and starts in wave 1, so the owner's review
is not on the critical path. TM1 and TM2 waves follow the same rule from their dependsOn columns.

**Wave 23, task 33, reads its entry conditions as checks** (fix batch b07, X-C2). Every condition
E2 to E4 is now built by a task in an earlier wave — E2 and E3 by task 21 (wave 14), E4 by tasks 07
(wave 7), 12 (wave 10) and 32 (wave 11) — so the gate verifies merged work. Before b07 those five
conditions lived in other parts' gap lists and in no task's scope, which meant the gate could only
answer BLOCKED and §12.0's "TM1 is re-planned" could never fire on evidence. The one condition that
is still a human act is E6: the operator prepares each worker's clone by hand (task 20's Gap S20-3,
open by decision, the lane mount being task 47's). E1 and E5 were always human or cross-phase acts.

---

## Task scopes

One paragraph each. A part writer works from its paragraph, this file's constraints, and the design
sections it names; it does not need another part. `§` means `05-design.md`.

### TM1-α

**01 — ADR-H1 "delegated callers" amendment and the team-records contract.** Docs only, on the Hub
branch after its sync-merge: a new ADR-H1 section and new contract sections, written **before any
TM1 code** (§11.1 row 4, `06` I4). The amendment changes, each with the line it replaces: (a) `azp`
(`ADR-H1:370`): Fleet's workload identity joins the allowed callers **only** on requests that name
a delegation in `X-Panacea-Delegation: dl-…`; (b) actor (`:372`, `:434-435`): the actor is the
delegation's owner, never a body field, recorded as `actor = owner, via = fleet, delegation = dl-…`,
and the Hub refuses a write whose `ag-…` does not belong to that owner; (c) RLS (`:253`): the Hub
sets `org_id` per request from the delegation's org; (d) the feed (`:252`): a per-org subscription
for Fleet, team projects only; (e) `devissuer` gains a client-credentials grant. The contract gains
§7.13 onward: every record of §3.3 that lives in the Hub (delegation, task, agent start/end, lease
state change, conflict, thread, message, escalation, directive, orchestrator turn, human message,
note, team policy, usage roll-up) with its route, fields, bounds, author class, audit event name
and feed kind, plus §5.3 matrix rows. **Fix batch b02** adds (f) a **second** app-only identity in
§5.10 rule 1, the sandbox broker (role `Hub.DelegationReader`), held by **rule 10** to route 47
alone — every other path 403 before the route runs, **rule 2's exemptions included**, tested by
role and never by "is this token app-only" (b02 round 1, C2) — rule 9 is left reserved and empty
for task 66's gateway caller, which has already drafted
its text against that number — and route 47's answer to that caller alone carrying `ownerTid` and
`ownerOid` (§7.13); and (g) a **second** named delegation exemption in rule 2, `GET /v1/principals`,
**row 88** and new **§7.28**, whose answer's `orgId` and three org flags are the **project's** org,
never the caller's. §7.27 and rows 80 to 86 are other tasks' (14, 46, 49), which is why b02 took
88 and §7.28. States the trusted-deputy risk (§2.4 item 5) in the ADR's risk list. Acceptance: every design field in §3.3 appears once; a grep for `actor` in a request body
schema returns nothing. Does not claim the Hub accepts any of it; tasks 03–08 build it.

**02 — `teamtext`.** `cp/internal/teamtext`, standard library only, used by the Hub deltas and
Fleet alike. `type Author string` (`"agent"`, `"human"`); `type Text struct { Body string; Author
Author; AuthorAgent string; Owner string; At time.Time }`; `func Bound(kind Kind, s string) error`
with the limits of §3.3 as named constants (title 140, why-line 200, message 600, reason 300,
question 300, directive 600, human message 2000, note 1000, brief title 120, path 200);
`func Clean(s string) error` refusing C0/C1 controls, ANSI/CSI/OSC, bidi overrides and isolates,
U+E0000–U+E007F, whitespace runs in names, and secret-shaped text (the patterns list is a fixed
table in the part); `func ValidGlob(g string) error` (repo-relative, no `..`, no absolute, no
backslash, ≤ 200 characters) and `func Breadth(globs []string, tracked []string) float64`. Tests
first, one per rule, each naming its mutant (for example, dropping the tag-block range lets
`U+E0041` through). Does not claim to catch persuasive text (§7.3).

**03 — Hub wire types for team records.** `cp/internal/hub/hubapi/team.go` on the merged Hub
branch, beside `types.go` (Hub task 03), with the complete Go struct and JSON tag for every record
and request task 01 fixed; id prefixes `dl`, `tk`, `ag`, `ls`, `cf`, `th`, `ms`, `es`, `dv`, `ot`,
`hm`, `nt` added to `keys.go`'s grammar through `ids.Wide`; enums (task state, lease mode and state,
conflict tier and state, thread state, message kind, resolution kind, escalation kind, directive
kind, turn origin, data class) as typed strings with `Valid()`; new problem codes in `codes.go`
(`delegation_required`, `delegation_expired`, `agent_not_owner`, `text_refused`, `glob_refused`,
`breadth_exceeded`). Every `text·A`/`text·H` field is a `teamtext.Text`. Golden JSON tests per
type. Does not add routes.

**04 — Hub migration for team records.** `cp/internal/hub/migrate/sql/00NN_team.sql`, numbered
after the Hub's last migration at merge (Hub task 07a/07b is `0001_init.sql`, **UNVERIFIED until
merge**). Tables for every Hub-owned record of §3.3, each with `org_id` under the Hub's RLS
pattern, owner columns `(tid, oid)`, the author class on every text column, `CHECK` bounds that
mirror task 02's constants, append-only guards (grants **and** triggers, as G4) on `message`,
`directive` and `human_message`, `tombstoned_at` on note, message and human message (erasure keeps
the audit event), and `ag_id` never a session id. Integration tests on Hub task 06's harness: a
second org cannot read a row; an `UPDATE` on a message is refused; a 601-character message is
refused by the database, not only by Go. Does not claim the Hub's erasure route covers these yet
(task 46).

**05 — Hub delegated-caller authentication.** Extends `cp/internal/hub/server/auth.go` (Hub task
13, **UNVERIFIED until merge**): a request whose `azp` is Fleet's workload identity is refused
unless it carries `X-Panacea-Delegation`; the delegation must exist, be unexpired and unrevoked,
and list the route; the principal becomes the delegation's owner with `via` set; `org_id` for RLS
comes from the delegation. The write transaction (Hub task 14) checks the body's `ag-…` against
the owner's agents. `cp/internal/devissuer` gains a client-credentials grant so the Hub's offline
proof (Hub task 43) can mint a Fleet token. Tests: a Fleet token with no header gets 401
`delegation_required`; an expired delegation 401 `delegation_expired`; an `ag-…` of another owner
403 `agent_not_owner`; a desktop token with the header is refused (the header is Fleet-only).
Does not create delegations (task 06).

**06 — Hub routes, part 1.** On the Hub's server and store (Hub task 14's write shape):
`POST /v1/delegations` (the user's own call; body: project, class, routes; 24-hour expiry),
`POST /v1/delegations/{id}/renew`, `DELETE /v1/delegations/{id}`; `POST /v1/projects/{p}/tasks`,
`PATCH …/tasks/{tk}` (state, scope, reason; owner only); `POST …/agents` (start: role, parent,
harness and pinned version, task, lane ref, policy version and digest) and `POST
…/agents/{ag}/end` (exit class); `POST …/leases/{ls}/events` (state changes only; Fleet stays
authoritative); `POST …/conflicts`, `PATCH …/conflicts/{cf}`. Solo projects keep `session_link`;
a team record carrying a session id is refused. Each write is one metadata-only audit event and a
feed row. Tests per route: owner, other member, viewer, Fleet with and without delegation. Does
not add the Z5 matrix rows (task 43). **Fix batch b01 (X-C1):** also route 87, `POST
/v1/projects/{p}/mode` `{"mode":"team"}`, the one writer of `projects.mode`: the team admin's
desktop only, no route name, one way, one `project.mode_set` event on Fleet's feed, and gated
before any write on `teamModeGate` (503 `audit_not_streaming`), open under TM1-α only by the
dev-only `PANACEA_APP_HUB_TEAM_MODE_UNSTREAMED` and filled by task 45 with `TeamModeAllowed`.
Route 46 refuses a solo project (409 `not_team_project`), so every team record sits behind it.
Task 21 wraps it as a local route, task 56's Share calls it, task 33's E5 runs it. **b01 round 2:**
Hub task 17's `store/projects.go` project-row reader selects `mode`, so routes 13 to 16 answer
`"mode":"solo"` or `"team"`, never `""` (`TestProjectReadsCarryMode`); every team harness sets
`PANACEA_APP_HUB_AUDIT_SINK=fake` beside the dev flag, so task 45's gate still opens for it.
**Fix batch b02 (S02-C2, S02-C5):** also the two Hub reads no task owned. **Route 88**,
`GET /v1/principals?tid=&oid=&projectId=`, contract §7.28 — Fleet with **no delegation**, §5.10
rule 2's second and last exemption, the read every Fleet route's `authorize` is built on (task
12). Its `orgId` is the **project's** org and its three flags are that org's, so an auditor of
another org arrives with `role` `""` and all three false; an unprovisioned `(tid, oid)` is 404
`principal_not_found`, an unknown or unseeable project 404 `route_not_found`. And **route 47's
third caller**, the broker (app-only, role `Hub.DelegationReader`, §5.10 rule 10): route 47 and
nothing else, any delegation of its own org whatever its state, and to that caller alone the
answer carries `ownerTid` and `ownerOid` — gap S17-1 closed, so task 17's broker derives a pod's
owner label itself instead of taking Fleet's word. Both reads are audited (`principal.read`,
`delegation.read_by_broker`), on neither feed; since fix batch b06 round 2, so is a Fleet launch read of route 47 whose `?for=` is not the delegation's owner (`delegation.named_by_another`). **The gate comes with the identity** (b02 rounds 1
and 3): this task, not task 05, owns the **five** deltas in task 05's files that keep the callers
apart — `Claims.AppRole` and `Claims.BrokerRead`, `isExemptPath`, `delegationHeader`'s role arms,
`resolveCaller`'s broker branch, and `Claims.PrincipalRead` with its own `resolveCaller` arm. Until
they land the broker is 401 `delegation_required` before route 47 and a second Fleet on route 78,
the org-wide feed, and Fleet is 401 on route 88: an exempt route needs its own arm.

**07 — Hub routes, part 2.** Threads (`POST`, `PATCH` state and round count), messages
(append-only, kind and typed fields per §6.2, text ≤ 600), escalations (`POST` by Fleet under the
Orchestrator owner's delegation, or by Fleet for itself; `POST …/escalations/{es}/answer` by that
human only, `text·H`), directives (append-only, turn origin, preceding peer ids, approving
`es-…`), orchestrator turns (an audit event only, metadata: origin, event ids, issued ids, tokens),
human messages (append-only, owner writes, ≤ 2000). Read rules: escalations, directives and human
messages are readable by their owner and an org *auditor* only; thread messages by parties only
(§3.4). Tests: Bob cannot read Alice's escalation; a directive `PATCH` is refused; an escalation
answered by someone else is 403. Does not implement notes (task 46).
**Fix batch b07 (G31-1, S01-I9):** the Hub's second copy of the taint rule holds no verb set of its
own. `TaintAllows` calls task 03's `DirectiveKind.Typed()`, so the five typed kinds (pause, resume,
narrow, wait-for, rebase) pass in a Fleet-started turn and only free text needs an answered decide.
Two copies of one rule drifted — the Hub's held three kinds against Fleet's five — and a Fleet-turn
`resume` died in the outbox on a 403. One definition cannot drift.

**08 — Hub feed kinds, Fleet's subscription, and `hubclient`.** Extends the change feed (Hub task
26) with a feed kind per new record and `GET /v1/orgs/{org}/changes?kinds=…` for Fleet's workload
identity, team projects only. Extends `cp/internal/hub/hubclient` (Hub tasks 04–05, built): a
`Team` interface with one method per task 06–07 route, the fake's team half with every observable
rule of those routes, and conformance rows that run against the fake and the real Hub. The fake is
what Fleet's tests use. **Fix batch b02:** also `Team.Principal` over route 88 (S02-C2), which
task 12 was written against as a proposal and is now real; `hubapi.Principal`; an `AsBroker()`
view and `Delegation`'s two broker-only fields (S02-C5); and five conformance rows, for which
`Env` gains a **second org** — "the principal answer's flags are the project's org" has no other
witness. Tests: the conformance suite, 39 team rows (b04's `team-feed/by-subject-reads-the-past`
is the 39th); a revoked delegation appears on the feed
within one poll. Does not deliver anything from Fleet (task 11).

**09 — Fleet model and the `panacea` control wire v1.** `cp/internal/fleet/model` (`Class`
`orchestrator`·`worker`·`mirror`; `Agent`; `Lease`; `Event` with its kinds from §4.2; `Turn`;
the per-class verb table of §5.2 as data), and the wire: one request envelope `{ "v": 1, "id",
"verb", "args" }`, one response `{ "id", "ok", "result" | "error": { "code", "message", "since" } }`,
and server-pushed `{ "event": "turn.begin" | "turn.end" | "nudge" | "stop" }`, in
`cp/internal/fleet/wire/control.go` and `sbx/internal/control/wire.go`, both tested against the same
`testdata/panacea-control/v1/*.json`. Every verb's `args` and `result` shape is written out,
including `unavailable` with `since`. Tests: round-trip of every fixture in both modules; an unknown
field is refused (G2). Does not carry transport.

**10 — Fleet's Postgres schema and `Store`.** `cp/internal/fleet/store`: `Store` interface (agents
and status, leases and lease paths, turn queue events, open-thread working state, the Hub outbox,
policy versions, per-counterpart counters), `memstore`, `pgstore` over `pgx/v5` (version matched to
the Hub's pin), and `storetest.Suite` both pass (phase 2's `registrytest` shape). Migration
`0001_fleet.sql` in its own schema `fleet`, keyed by `org_id` and owner. No Hub record is stored
except as an outbox payload. Tests: the suite; two concurrent `ClaimTurn` calls for one Orchestrator
yield one winner (the two-replica property). Does not start a server.

**11 — Fleet as a delegated Hub caller.** `cp/internal/fleet/outbox`: every Hub-bound write goes
to the outbox in the same transaction as the Fleet state change, then a delivery loop sends it with
an idempotency key (the Hub's contract §10.6 shape) and marks it sent; after 10 minutes of Hub
failure Fleet pauses negotiation (threads stop advancing; workers keep running) and resumes on
recovery (§2.2 item 3). `cp/internal/fleet/delegation`: Fleet's workload-identity token source,
a delegation cache keyed by owner and project, `X-Panacea-Delegation` on every call; `feed.go`
refreshes the tracked orgs and live delegations from Fleet's own store every tick, so both
replicas hold what either one adopted rather than only the launches they served, and it
consumes task 08's per-org subscription and, on a revoked or lapsed delegation, calls a
`Revoker` interface (stop sessions, close sockets, withdraw leases, mark cards *ended: access
removed*) within 60 seconds; `rebuild.go` reloads Hub-owned state into memory at start (Fleet
persists no Hub record, `ADR-H1:467-474`). The outbox reads a **different** row of that same
store — the latest agent on the key, ended ones included — because a launch's last write is its
`agents.end` and a restart's backlog is all ended agents; that read never acts, so delivery can
never destroy a sandbox, and a failure of it is transient, so one store blip kills no queue. The
poll set is therefore **orgs with a live agent**, never "orgs Fleet serves", and an org first
tracked at a launch is polled from that launch's head: a consumer whose trigger is a person
rather than an agent gets nothing from `Feed.Also` and must read the Hub directly, which is why
task 14's policy activation does (fix batch b04, S02-C4). `Feed.Adopt` is a launch's **only** resolver (fix batch b06 round 2): it reads the org's head, then the named record with the caller named in route 47's `for`, and checks that record against the caller's own org, owner, project and acting class before its one `Put`. Task 20 calls it, and so will task 24's spawn path.
Tests on task 08's fake: a Hub outage of 11 minutes pauses negotiation and loses no write, ended
agents and all; a revocation reaches the `Revoker` inside 60 s of fake time, on the replica that
did **not** serve the launch, proven with two `Feed`s over one store.

**12 — `cmd/fleetd` chassis.** `cp/cmd/fleetd/{main.go,config.go}` (every `PANACEA_APP_FLEET_*`
variable parsed once, a bad value fails start with its name) and `cp/internal/fleet/server`:
the mux, phase 2's problem body, `/healthz`, `/readyz` (Postgres and Hub reachable), and
`authz.go`: every route validates an Entra token with phase 8's `internal/entra` (built on
`feat/env`, task 16), audience Fleet, `azp` the desktop's client id, principal `(tid, oid)`;
membership and role come from the Hub (cached ≤ 60 s) and are checked against §3.4's table per
request. **Fix batch b02 (S02-C2):** that Hub read is route 88, `GET /v1/principals`, contract
§7.28, built by tasks 01, 06 and 08 — it was a gap this task raised and nobody owned, so `go run
./cmd/fleetd` did not build. `Member.OrgID` is the **project's** org from §7.28's answer, and
every org-level grant is bound to it. Singleton loops (outbox delivery, deadline sweeper, feed consumer) take a Postgres
advisory lock so two replicas never both run one. Tests: a viewer's launch is 403; a token for
another audience is 401; two `fleetd` processes on one database run one feed consumer.
**Fix batch b07 (G32-3, G32-4):** two more variables, because task 32's chart sets both and this
binary read neither. `PANACEA_APP_FLEET_MIGRATE` is `start` (default, today's behaviour) or `job`,
where a separate migration Job owns the schema, this process migrates nothing, and it refuses to
serve a schema behind its code rather than failing on a DML-only role's permissions; that Job runs `fleetd migrate`, which migrates and exits (b07 round 2).
`PANACEA_APP_FLEET_PG_AUTH` is `password` (default) or `entra`, where the pool's password is an
Entra token refreshed from the same assertion source the Hub client uses. Both fail the start by
name on any other value, so a chart typo is a named error, not a silent default.

**13 — OD-23 mechanism, part 1: the policy layers.** `cp/internal/policy`: a policy is Claude Code
managed settings, as JSON. `Render(layers Layers, class model.Class) (file []byte, digest string,
err error)` merges, in order: the embedded minimal default for the class (`defaults/*.json`), phase
6's gateway fragment (S6.6, `images/sandbox/managed-settings.gateway.json`, read as input once
merged), the org's current version, the project's current version, then **class invariants that no
layer can remove**: `permissions.allow` contains `Bash(panacea *)`; `apiKeyHelper` is `panacea
token`; `allowManagedHooksOnly` and `allowManagedPermissionRulesOnly` are true; the worker class's
`PreToolUse` hook on `Edit|Write|MultiEdit` runs `panacea check`; `env` sets
`CLAUDE_CODE_DISABLE_NONESSENTIAL_TRAFFIC=1`; `permissions.defaultMode` is pinned per class; the
Orchestrator's allowed tools are `Bash(panacea *)` and `Read`; `forceLoginMethod` is absent. A
later layer may add rules and tighten; an attempt to drop an invariant is an error naming it. **The
two layers are not equally trusted**, because the org layer is an org admin's and the project layer
is a *team* admin's (`task-12.md:354`): the project layer is a tighten-only allowlist —
`permissions.deny`, `permissions.ask` and a closed list of non-command scalars, every other key
refused by name (**I14**) — and no layer of either scope may give the **orchestrator** class a
command-bearing key (`hooks`, `statusLine`, `mcpServers`, `aws*`, any `*Helper`) or an unlisted
`env` key (**I15**), because that pod's socket speaks as its owner (OD-16; fix batch b05, S02-C3).
An org admin's hook must sit in a `worker` section, not `all`, because `all` is rendered for the
orchestrator class too. The minimal defaults are those invariants and nothing else: **no drafted
policy** (OD-23). Digest is
SHA-256 over the canonical bytes. Tests: each invariant has a mutant (a project layer that removes
`Bash(panacea *)` must fail); rendering is deterministic. Which `env` keys a managed file can lock
is **UNVERIFIED** (§7.3).

**14 — OD-23 mechanism, part 2: versions, routes, audit.** `cp/internal/policy/store.go` over
task 10's store: append-only versions `pv-…` per org and per project, each with author, time,
bytes and digest; `cp/internal/fleet/server/policy.go`: `GET`/`POST /v1/orgs/{org}/policy` (org
admin) and `/v1/projects/{p}/policy` (team admin), a `POST` creating version n+1 after `Render`
succeeds for every class. A version is stored **pending** and governs nothing until the Hub holds
its metadata-only audit event `policy.versioned` (id, scope, seq, digest), written by **the
admin's own client** on Hub route 80 — not through Fleet's outbox, which would need a delegation
an admin editing policy does not hold (`task-14.md:44-70`). Fleet then **pulls**: the client calls
`POST …/policy/{pv}/activate`, Fleet reads that event back by id (route 78 with `subject=`) and
activates only on a match, and a 30 s `SweepPendingPolicy` asks the same question for any client
that died in between. Activation deliberately does **not** ride task 11's feed: that feed polls
orgs with live agents, and TM1-α's admin writes the org policy before anyone launches, so the
layer would have stayed pending forever (fix batch b04, S02-C4). Every agent-start record (task
06) carries the rendered digest, so *"which policy governed this agent"* is answerable from the
Hub. Tests: a member's `POST` is 403; a layer that fails `Render` creates no version; the same
bytes twice give one digest; a policy posted before any launch still activates. Does not deliver
files into pods (task 15).

**15 — Phase 8 delta: sandbox classes and the policy mount.** Extends `cp/internal/sandbox/manager.go`
on `feat/env` (built, task 14, `Spec` at `:69-83`) with `Spec.Class` and `Spec.PolicyFile`:
the `orchestrator` class mounts **no machine disk and sets no affinity** (task 14b), has its own
requests (proposed 250m / 512 MiB, measured in task 47), and its PTY command is
`panacea-orchestrator`; every class gets a per-pod Secret `<pod>-policy` mounted read-only at
`/etc/claude-code/` holding `managed-settings.json` (the Linux managed path, **UNVERIFIED** at the
pinned CLI version; task 33's `/status` check settles it) and `PANACEA_APP_POLICY_SHA256` in the
env. `sbx/internal/policycheck` makes `sandboxd` refuse to start the harness if the file is absent
or its digest differs. Tests on the fake clientset: an orchestrator pod has no volume from the
machine disk and no affinity; a worker pod keeps phase 8 task 15's; `sandboxd` with a tampered file
exits non-zero before the PTY starts.

**16 — Phase 8 delta: `sandboxd` `/control`.** `sbx/internal/control`: a `/control` WebSocket route
beside `/attach` and `/healthz` (`sbx/internal/server/server.go:85-86`, built), bearer-authorised
with a control token distinct from the attach token; one Unix socket at `/run/panacea/control.sock`
(mode 0600, the agent's uid) that the `panacea` binary dials; every request on it is stamped with
the pod's session id and class before it is forwarded up the control stream, so the credential is
the channel (§3.3 rule 1). `snapshot.go` writes `/panacea/team.json` (worker-visible fields only,
with `generatedAt`) to a read-only `tmpfs` mount outside the worktree whenever Fleet pushes one.
When the stream is down every request is answered `unavailable` with `since`. Tests: a request with
a forged session id in its body is re-stamped; the snapshot never contains a `text·A` field.

**17 — Phase 8 delta: the broker's second caller and control relay.** Extends `cp/cmd/broker`
(phase 8 tasks 17–19, **UNVERIFIED until merge**; the broker is unbuilt on 2026-09-25). OD-14
reverses owner-only visibility: Fleet's workload identity may create, list, get, destroy and relay
for an owner **only** under a live delegation, which the broker checks against the Hub (cached
≤ 60 s) and writes into its audit line as `actor = owner, via = fleet, delegation = dl-…`. Every
object keeps its owner label (P8-D4). A second relay kind, `control`, joins attach (phase 8 task
18) and reaches `sandboxd`'s `/control` on 8080 (P8-D12's ingress rule, unchanged). The broker
records each pod's source address at ready and exposes it to phase 6's keysmith for token binding
(P6-D7). **Fix batch b02 (S02-C5):** the broker reads route 47 **as itself**, an app-only caller
with role `Hub.DelegationReader` on that one route (§5.10 rule 10), and the answer to it alone
carries `ownerTid` and `ownerOid`, which is what `OwnerLabelValue` takes. Gap S17-1 is closed, and
task 06 owns both halves. Tests: Fleet without a delegation is 403; a delegation for Alice cannot
list Bob's pods; a revoked delegation fails within the cache window; the audit line names the
delegation.

**18 — The `panacea` verbs in the sandbox binary.** Extends phase 6's `sbx/cmd/panacea/main.go`
(task 14, `token` only, **UNVERIFIED until merge**) with `sbx/internal/panaceacli`: `team`,
`check <path>`, `files`, `show <path>`, `lease request|release <globs>`, `report
done|blocked|assumed <text>` (with `blocked --on human`), `task …`, `worker spawn|stop`, `direct
<ag-id> <kind> …`, `inbox`, `peer <kind> …`, `human inform|ask …`, `lane <ag-id>`, each a thin
client over task 16's socket, printing the result JSON in the fixed quoted envelope (G4) and
exiting non-zero with *"team view unavailable since <t>"* when Fleet is down. The binary is static
(`CGO_ENABLED=0`, P8-D8) and vendored. The class's verb set is enforced by Fleet, not here; the CLI
shows the verbs it was built with. Tests: every verb against a fake socket, from task 09's
fixtures; exit codes; `token` behaviour unchanged.

**19 — Fleet's control streams and verb dispatch.** `cp/internal/fleet/control`: Fleet opens one
`control` relay per live sandbox through the broker (task 17), keeps it open with reconnect and
backoff, and dispatches each stamped request to a handler registered per verb. `dispatch.go`
checks, before any handler runs: the stamped session exists, its class may call this verb (task 09's
table), and every object the args name belongs to the session's owner (§3.3 rule 1). Unknown verbs,
unknown fields and oversized args are refused with ST.1's error codes. Tests: a worker calling
`peer` is refused; Alice's Orchestrator calling `worker stop` on Bob's worker is refused; a dropped
stream reconnects and the sandbox sees `unavailable` meanwhile.

**20 — Fleet sessions over the broker.** `cp/internal/fleet/sessions`: `Launch(owner, project,
class, harness, delegation)` creates the sandbox through the broker (the `remotebe` client shape,
phase 8 task 22), mints an `ag-…` id, writes agent start (and later end) to the Hub through the
outbox, and renders the class's policy file (task 13) into the spec (task 15). The Orchestrator
is created on a `member`'s first team launch and is one per user per project; a `viewer` gets
none. `tokens.go` mints the session's gateway key through phase 6's `gatewayclient` (P6 task 15,
S6.3) with tags `tid`, `oid`, team, project, agent id, route and, for peer-triggered turns,
counterpart, and a fresh key per Orchestrator turn; an AU project with the AU route down fails
closed (P6-D5). Tests on a fake broker and fake keysmith: a second Orchestrator launch returns the
first; a viewer's launch is 403; an AU project never mints on Global.
**Fix batch b06 (S02-C6):** the `dl-…` in a launch request is a **name, not a capability**. Fleet
resolves it — task 11's cache keyed by the principal, else task 11's `Adopt` (b06 round 2: org head first, then the Hub's record) — and the answer is
the caller's own org, user, project and acting class on both paths (an entry under the caller's key is the caller's by construction; `Adopt` checks before its one `Put`);
anything else is refused, is never cached, drops or revokes nothing of the real owner's, and is audited by the Hub (`delegation.named_by_another`). Without it a cache miss let any
member launch under any delegation id: task 05 makes the delegation's owner the principal, so the
agent row, the pod's owner label and the gateway mint would all have carried the other person's
identity and budget. Tests: `TestLaunchUnderAnotherOwnersDelegationIsRefused` on a cold cache,
`TestARefusalLeavesTheOwnersDelegationAlone` (b06 r2, replacing `TestARefusedDelegationIsNotCached`, which its own mutant passed), and task 11's `TestAdoptRefusesAnotherOwnersDelegation`.

**21 — `fleetbe` and the team launch on the local control plane.** `cp/internal/backend/fleetbe`
implements phase 2's `backend.Backend` (S8.1, unchanged) over Fleet's REST and attach routes, with
the user's Fleet token from the Hub's token custody (`entraauth` `TokenSource`, Hub task 31).
`cp/internal/api/teamlaunch.go`: on a team project, a launch first calls the Hub **as the user** to
create the delegation (task 06), then Fleet with it; the desktop renews delegations silently while
signed in. A session still running locally on a team project is labelled *outside the team*: no
leases, visible to others only once pushed (§2.3). Tests: create/list/destroy/attach through a fake
Fleet; a launch with the Hub down fails with the Hub's code, not a Fleet one; the session JSON leaks
no token (phase 2's `sessionJSON` rule). **Fix batch b01 (X-C1):** `TeamProjects` reads
`hubapi.Project.Mode` (task 03) and gains `MakeTeam`, behind the local
`POST /v1/projects/{projectId}/team-mode` over Hub route 87 (task 08's `SetProjectMode`, Hub task
35's S-LINK); a launch never calls it, so no launch turns a project team behind Share's consent.
On the Hub's 200, `MakeTeam` calls S-LINK's `Resolver.Invalidate(localProjectID)`, so the next
`IsTeam` does not read the 5-minute cache's `solo` (b01 round 2).
**Fix batch b07 (X-C2, S02-C7/S03-C1, G33-1/S04-I8)** adds two things this task alone can do,
because route 46 and Hub routes 65 and 70 take only the owner's own desktop token (66 and 68 also admit Fleet·O, but show a person only their own rows).
(1) The member's first team launch in a project creates the owner's **`orchestrator`-class
delegation** as well as the session's worker one, keeps it per project in the registry's
`TeamDelegations` record, renews both on one timer, and sends its id to Fleet on
`CreateAgentRequest`; without it task 20 answered `awaiting-delegation` for ever and TM1-α had no
Orchestrator at all. (2) Four **`/v1/team/*` human relays** — list escalations, answer one, post a
human message, list directives — each adding the caller's own token and nothing else, passing the Hub's page through unchanged, and refusing
any owner named in a body. Task 52 extends both; it no longer has to exist first. Together they
close task 33's E2 and E3.

**22 — Read verbs.** `cp/internal/fleet/verbs/{team.go,check.go,files.go}`: `team` returns §5.1's
table for every agent on the project, **filtered by the caller's class**: a worker gets owner,
role, brief section id and title, status, observed files, leases and conflicts, and never a task
title, why-line, commit subject or thread text; an Orchestrator also gets those, quoted and
attributed. `check <path>` returns the path's holders, editors, open conflicts (the other side's
owner, brief section and paths only, for a worker) and the caller's own pending directives first.
`files`/`show` answer from the lane owner's `machined` over the broker (TM1) at the integration
head, read-only. Every string passes task 02 before it leaves Fleet. Tests: a worker's `team`
contains no `text·A` field, for every fixture; `check` on an unleased path returns empty, not an
error; OD-24's private opening prompt never appears anywhere.

**23 — Lease engine.** `cp/internal/fleet/leases` and `verbs/lease.go`: modes `intent`, `hold`,
`bulk`; states of §3.3; `Request(holder, globs, mode)` validates every glob (task 02) and refuses a
glob matching more than 10% of tracked files at the integration head (the tree list from `machined`,
task 22's path) with `breadth_exceeded`; overlap with any live lease or observed edit is returned
as a typed overlap for the forecaster (task 25). `rightofway.go`: the earlier **and narrower** lease
holds; between an earlier broad and a later narrow lease, the narrower wins on the paths it names;
`bulk` never has right of way; the *contested worker* is the one whose edit to the path is newer,
else the one without right of way (§6.3 items 5–6). Every lease is released when its task or agent
ends; each state change goes to the Hub (task 06) through the outbox. Advisory only: Fleet never
refuses a write (P7). Tests: one per right-of-way rule, each with the mutant that flips it; `**`
is refused; ending a task releases its leases.

**24 — Task, worker and report verbs.** `verbs/{task.go,worker.go,report.go}`: `task create|update
|abandon` for the caller's own owner only, with title (`text·H` if relayed from the owner's own
message, else `text·A`), `briefRef` (a brief section id the Hub resolves) and why-line, and
intended scope (≤ 20 globs); **creating a task takes `intent` leases on its scope at once**, so an
overlap raises a conflict before any worker exists (§6.3 item 1, §8.1). `worker spawn` launches a
worker under the caller's own owner and task through task 20; `worker stop` stops only the caller's
own. `report done|blocked|assumed` records a worker's report as `text·A` for its own Orchestrator's
queue (task 26); `blocked --on human` is the only source of `waitingOn: human` besides the hook's
`Notification` (§3.3 agent row). Tests: an Orchestrator cannot create a task for another owner; a
task whose scope overlaps a teammate's intent produces the overlap before `worker spawn` is called.

**25 — Tier-P forecaster; conflict to thread.** `cp/internal/fleet/forecast/tierp.go`: a set
intersection over intent globs, hold paths and (from task 35) observed edits and committed paths,
per pair of agents with different owners. A new overlap raises `cf-…` (tier `P`, paths, basis
shas, state `raised`) in the Hub and opens `th-…` between the two Orchestrators with the typed
conflict attached (both tasks, both brief sections), and puts one thread event on each
Orchestrator's queue **with no coalescing** (§4.2, §6.4). A pair already in an open thread raises
no second conflict. Tests: two intents on `auth/**` and `auth/session.go` raise one conflict and
one thread; same-owner overlap raises nothing; the thread event reaches both queues in one call.

**26 — The turn queue, origin and template.** `cp/internal/fleet/turns`: one queue per
Orchestrator in Fleet's Postgres; events of §4.2; thread events (peer message, conflict opened) are
deliverable at once, every other kind coalesces for up to 60 s; one turn at a time per
Orchestrator (task 10's `ClaimTurn`); a batch's **origin** is `human` if it holds a message from
the Orchestrator's own human, else `fleet` (§7.2). The turn input is a fixed product template
(*"3 team events. Run `panacea inbox` to read them."*) and never carries event text. `turn.begin`
goes down the control stream (ST.5); `turn.end` comes back with the turn's token use; each turn
writes an `ot-…` audit event (origin, event ids, issued ids, tokens). Tests: a peer event is not
delayed by coalescing; a human message in a batch makes the origin `human`; the template contains
no byte of any event.

**27 — The `panacea-orchestrator` runner and the pod-kill test.** `sbx/cmd/panacea-orchestrator`
and `sbx/internal/runner`: the orchestrator class's one PTY command. It blocks on `turn.begin`,
then runs the harness as a **child**: the first turn `claude -p --session-id <fixed uuid> …`, every
later turn `claude -p --resume <fixed uuid> …`, never `--fork-session` (phase 2's Resume forks at
`cp/internal/harness/argv.go:91-100`; this path must not reuse it). The working directory is an
empty folder so no project settings or `CLAUDE.md` load (a `--setting-sources`-style flag is
**UNVERIFIED** at the pinned version; task 33 checks). The transcript lives in the pod's `emptyDir`
as a cache. For a human-origin turn the child's stdout is kept **verbatim and bounded** as the
turn's `text·A` reply; nothing in the runner or Fleet reads fields out of it (OD-8), so the part
picks `--output-format text`. It sends `turn.end` and blocks again. The pod-kill test (on a fake
broker and `memstore`) kills the pod between two turns, lets Fleet relaunch it, and checks that no
lease, thread or pending directive is lost and that the next turn starts a new session.
`--session-id` and `--fork-session` are listed by `claude --help` 2.1.274 (`argv.go:91-97`); that
they behave this way with `-p` at the image's pinned version is **UNVERIFIED**, and the OD-8 canary
covers it.

**28 — Protocol messages.** `cp/internal/fleet/protocol` and `verbs/{peer.go,inbox.go}`: `peer`
kinds `ask`, `answer`, `inform`, `propose`, `accept`, `reject`, `counter`, `withdraw` with the typed
fields and text limits of §6.2, schema-checked (G2) and cleaned (G3) before storage; `inbox`
returns unread messages in a fixed envelope naming each author as an agent and its owner (G4).
**Fleet answers every `ask` itself, at once, from typed card fields** (task, brief section, status,
files, leases) as a template; a free-text `answer` is accepted only under the sender's human's
standing permission and lands in that human's SINCE YOU LAST LOOKED. Limits: 3 rounds per side, a
30-minute deadline (team policy may lengthen, never shorten), 20 messages an hour per pair, and a
thread closes `escalated` after two rounds with no new typed content (§6.6). Tests: one per limit;
an `ask` gets a Fleet answer with no Orchestrator turn; a free-text `answer` without permission is
refused.

**29 — Resolutions `yield`, `sequence`, `proceed-both`; DECIDED WITHOUT YOU.**
`protocol/resolve.go`: when every party has accepted a proposal, Fleet applies its typed
parameters and nothing else: `yield` releases the sender's leases on the named globs; `sequence`
records who goes first as a land-order hint (consumed by phase 3's queue in TM1, task 61's in TM2)
and turns the second side's lease to `intent` until the first lands; `proceed-both` marks the
conflict `accepted` (tier P or S only here; H and M are always a decide, task 30). Each Orchestrator
then directs **its own** workers (task 31); neither side can move the other's. For every accepted
resolution Fleet itself writes a DECIDED WITHOUT YOU item for **both** humans from the thread's
typed state, whether or not either Orchestrator informs (§6.2, `07` I1). Tests: `yield` releases
only the named globs; a resolution cannot touch the other side's worker; both humans get the item
when neither Orchestrator calls `human inform`.

**30 — Escalations.** `cp/internal/fleet/escalate` and `verbs/human.go`: `human inform` and `human
ask` write `es-…` records to the Orchestrator's own human only; **Fleet sets every decide's
default (always the safe one) and deadline (≥ 30 minutes)**, whatever the Orchestrator asked
for. `rules.go` holds §6.5's table as data, keyed on typed action, with each user's rows (tighten
only) and team-admin floors (add asking only); the always-decide rows (`build-on`, `proceed-both`
on H or M, free text in a Fleet-started turn, deadline exhaustion) cannot be loosened (OD-24).
**A `peer accept` on a decide row becomes the escalation itself**, so an Orchestrator cannot skip
it. On rounds or deadline exhausted, Fleet writes a decide to both humans with the thread's typed
state. When a decide's deadline passes unanswered, the safe default applies: the contested worker
(task 23) pauses; nothing is deleted or reverted. Tests: an Orchestrator's `default: continue,
deadline: 60s` is overridden; accepting a split that moves edited paths becomes a decide; an expired
decide pauses exactly the contested worker.

**31 — Directives and the taint rule.** `cp/internal/fleet/directive` and `verbs/direct.go`:
`direct <ag-id> <kind>` with kinds `pause`, `resume`, `narrow` (globs), `wait-for` (a lease),
`rebase`, `free` (text ≤ 600 `text·A`), to the caller's **own** workers only. Every directive is a
`dv-…` Hub record **before** delivery, carrying turn origin, the ids of peer messages and cards the
session read since its human last spoke, and the approving `es-…` if any. `taint.go`: on a team
project every Orchestrator is tainted from its first turn; in a `fleet`-origin turn only `pause`,
`resume`, `narrow` inside the task's scope, `wait-for` and `rebase` pass; `free`, a new task, a
wider scope, out-of-scope paths, `build-on` or abandoning the task become a **decide written by
`direct` itself**, showing the human the peer text read since they last spoke, with authors. In a
`human`-origin turn free text passes and is shown inline in the panel as issued. Delivery rung 1
(piggyback): every `panacea` call a worker makes returns its pending directives first, as tool
output, the only way directive text reaches a worker. Tests: a `free` in a Fleet turn produces a
decide and no directive; a `narrow` outside scope is refused; a directive is readable in the Hub
before the worker sees it.

**32 — Phase 8 delta: Fleet's chart and the private gateway path.** `deploy/charts/fleet/**`, new,
beside phase 8's `broker` and `hub` charts (task 29, **UNVERIFIED until merge**): two replicas, a
PDB, the Fleet workload identity (phase 8 task 10's module), its Postgres schema on the Hub's
Flexible Server under its own Entra role (P8-D10), Australia East only. Phase 8's platform chart
(task 11) gains: one allow from sandbox pods to the gateway's service port and nothing else on the
service CIDR; the `orchestrator` class's egress to the gateway and Fleet's control route only; no
public ingress to the gateway (P8-D12, P6-D1). Chart tests: `helm template` output checked by phase
8's `netpolcover` test; a sandbox pod cannot reach any other service port. Patterns only from
Panacea (OD-13). Does not apply anything to a cluster (task 33 does, by the owner's runbook).

**33 — TM1-α on the real cluster.** `infra/smoke/team/*.sh` and
`docs/verification/2026-09-25-phase-t-alpha-results.md`: on the real environment, the project made
a team project by its admin through route 87, never assumed (M0, `05-team-mode.sh`, fix batch
b01); one Orchestrator and one worker for each of two identities; a scripted overlapping plan (two tasks whose scopes
overlap) raises a tier-P conflict **before** either worker is spawned; the Orchestrators settle it
with `yield` or `sequence`; one human gets an `inform`, the other a `decide`; the pod-kill test
(task 27) on a real pod; `/status` in a worker and in the Orchestrator shows the rendered managed
file, and nothing else, in force (OD-23); whether the `PreToolUse` hook in a file-based managed
file raises an approval dialog in an interactive worker (§5.3); whether an empty working directory
keeps project settings out (§4.1); the PTY round trip through `fleetbe`; a first G9 pass. The
record states each observation, what was not observed, and the verdict. **If it fails, TM1 is
re-planned before task 34 starts.** A screenshot is not evidence; the record says what ran.
**Fix batch b07 (X-C2):** its entry conditions are checks on merged work, not waits on gaps. E2 and
E3 verify task 21's orchestrator delegation and four human relays; E4 verifies task 07's `Typed()`
taint rule and task 12's two new variables; E6 states TM1-α's one accepted manual step, the
operator's worker clones (task 20's Gap S20-3, open by decision). Before b07 all five named gaps in
other parts' gap lists that no task owned, so the gate could only answer BLOCKED.

### TM1

**34 — Phase 3 delta: `machined`'s activity sweep.** Phase 3 task 07's observed claims become a
library (`cp/internal/converge/claims.go`, **UNVERIFIED until merge**; phase 3 has no `gitx` or
`lanes` code yet) that `machined` (phase 8 tasks 20–21) runs **outside the agent's VM**, every 5 to
10 s, over each live worktree: `git status --porcelain=v2 -z` and `git diff --numstat -z` plus
dirty hunk ranges on the base blob and the last commit's sha and changed paths, under phase 3's
§HGH hardening with `core.fsmonitor`, `core.hooksPath` and every other config-driven command forced
off on the command line (phase 3 lets the agent write `.git/config`). Output is ST.6's sample,
every path through task 02's hygiene and 200-character cap, sent to Fleet over the broker for a team
project and to the owner's control plane for a solo one. No "files read" (§3.3 activity row).
Tests: a repo whose `.git/config` sets `core.fsmonitor` to a script never runs it; a path with a
bidi override is dropped and counted. Whether a guest write is visible within one sweep is
**UNVERIFIED** (task 47 probes).

**35 — Activity, presence and holds.** `cp/internal/fleet/activity`: the activity table, in memory
with a 45-second TTL, fed by ST.6 samples; presence (cockpit online, project open) from the
desktop. On the first observed edit to a path a worker gets a `hold` lease whether or not it asked;
more than 25 files in one minute becomes one flagged `bulk` lease, reported to the owner's
Orchestrator as an event; a hold lapses 30 minutes after the last edit **to that path**; every
lease is released when its task or agent ends, its delegation lapses or its owner leaves; a team
admin may release a stale lease, audited (§6.3). Rebuilt within one sweep after a restart. Tests:
a formatter run over 40 files yields one bulk lease with no right of way; editing another path
does not extend a hold.

**36 — Tier-H forecasts and co-edit down-weighting.** `forecast/{tierh.go,coedit.go}`: overlapping
dirty or committed line ranges on the **same base blob** raise tier `H`; in TM1 this mostly
collapses to P, since bases diverge after any land (§8.2, `06` Minor 4), and the part says so. A
per-project co-edit rate down-weights files often edited together: a lockfile or generated file
raises nothing at tier P, only at H or M. HORIZON items (task 54) come from here and stay until the
conflict is `resolved` or `cleared`. Tests: two hunks on different base blobs raise nothing at H;
`package-lock.json` raises nothing at P.

**37 — Budgets.** `cp/internal/fleet/budget`: each Orchestrator's turn budget (default 30 turns an
hour) and per-turn token cap, set by its owner within team policy; **a human's own message skips
the budget and the queue**, and a human `pause` is executed by Fleet directly, with no turn;
peer-triggered cost is metered per counterpart from the gateway's usage (phase 6 task 09) using the
counterpart tag task 20 sets; past 10 turns an hour from one counterpart, that counterpart's events
fold into one digest turn every 15 minutes; when the budget is spent, non-human events queue and the
owner is told. The spend split, *workers* against *coordination*, is served for task 56. Tests: a
peer flood cannot delay a human message; the eleventh counterpart turn in an hour becomes a digest.

**38 — The template-only PTY writer and the delivery ladder.** `cp/internal/fleet/ptywriter`: the
**only** Fleet code that writes to a PTY. Its API takes a template id, not bytes, and it writes the
product-written templates byte for byte (*"[Panacea] Your Orchestrator sent you a directive. Run
`panacea check`."*, *"Team update for you. Run `panacea check`."*). A unit test proves it refuses
every other byte sequence, including `!`, `/`, `ESC [ Z` and any control byte (`07` C3).
`delivery/ladder.go`: rung 1 piggyback (task 31); rung 2, the inbox socket, only behind canary C4
and off by default; rung 3 the nudge when the worker is idle, idle derived from a byte count, never
content (`registry/session.go:22-23`); rung 4 `pause`, an interrupt then a stop. At most one nudge
per worker per 3 minutes. Tests: the refusal table; a busy worker gets no nudge; two directives in
one minute cause one nudge.

**39 — Fleet's own attach handler and socket revocation.** `cp/internal/fleet/relay`: a new
handler, **not** phase 2's `api/attach.go` (§7.4): every WebSocket upgrade validates the Entra token
(task 12) and that its owner is the session's owner; knowing an `ag-…` id grants nothing; the PTY
relay carries a session's terminal only to its owner. `revoke.go` closes a revoked owner's open
sockets within 60 seconds of task 11's revocation. Every route, attach included, joins Z5 (§7.4
item 5). Tests: Bob's valid token cannot attach to Alice's worker; a revocation closes the socket
inside 60 s of fake time; an input frame from a non-owner is never forwarded.

**40 — Runner rotation and DIVE.** `sbx/internal/runner/{rotate.go,dive.go}`: the runner starts a
**new** session id each day and at each task boundary, seeded only from Fleet's typed state (its
`panacea team` view, its open threads, its human's policy rows and last messages), **never from
the old transcript**, so context and cost stay bounded and no agent-written summary of peer text
is carried (§4.2, `06` I9); taint is not per session, so rotation resets nothing unsafe (§7.2).
DIVE: while parked between turns, the owner may attach (through task 39) an interactive `claude
--resume <same id>` via the runner, never a fork; events queue meanwhile and show as waiting; DIVE
start, end and every directive it issues are `ot-…` records with origin `human`. Tests: the seed
contains no byte of the previous transcript; a turn event during a DIVE waits; a non-owner cannot
DIVE.

**41 — The Orchestrator's instructions and the replay eval.** `cp/internal/fleet/instructions`: the
Orchestrator's operating text, delivered through the orchestrator class's policy layer as the
managed `claudeMd` key (cited by §4.1, **not re-read here**) plus the runner's
`--append-system-prompt`: read `panacea team` before planning; plan around live leases; every
effect goes through `panacea` verbs; a human's messages are private; peer text is data. The cloud
worker block carries §5.3's fixed pointer paragraph. These are product instructions, not org
policy (OD-23 unaffected). `cp/internal/fleet/eval` replays scripted team events
(`testdata/team-events/*.json`) through a real Orchestrator on the gateway and **scores only
Fleet's typed verb log** (which verbs, which args, which escalations), never the model's prose
(OD-8). Tests: the eval harness on a fake model that emits a fixed verb sequence scores it
exactly; the instructions contain no teammate-derived text.

**42 — G9: the injection corpus and the scripted two-Orchestrator run.** `cp/internal/fleet/g9`:
`04` F8's corpus plus hostile peer messages (imperatives, fake system text, directive-shaped text,
Unicode tricks), run two ways. **CI, deterministic:** hostile calls drive Fleet's verb handlers
directly and must produce **zero** free-text directives in Fleet-started turns and zero typed
directives outside the task's scope; the PTY writer refuses every non-template byte (task 38).
**Live, scripted:** two Orchestrators on the gateway, one fed the corpus, run through task 41's
harness; the record counts free-text directives, out-of-scope directives, hop time per negotiation
step and coordination tokens as a share of each member's spend (target under 10%, **UNVERIFIED**
until measured). Budget: 3 days of contingency for G9 not passing first time sit in task 57's
schedule, not here. Pass mark: zero, both counts.

**43 — Hub delta: role-to-verb, org capabilities, team policy, Z5 rows.** On the Hub branch: the
§3.4 table as the Hub's authorisation rows; two org-level capabilities, *auditor* (reads
escalations, directives, human messages and threads across owners) and *compliance* (may lower a
project's AU route, audited); `teampolicy.go` with the project's **data class, set at project
creation and updatable** (OD-24; if the Hub's project record already carries it from phase 6's
asks, this reuses it, **UNVERIFIED until merge**), the route (raised by any member, lowered only by
*compliance*), non-zero default quotas per member and per project, the Orchestrator turn budget,
escalation floors (an admin may only add asking) and message budgets. Every team route of tasks
06–07 and this one joins the matrix test (`server/matrix_integration_test.go`, Hub task 16b):
nobody writes another person's task, lease, message, directive or escalation, whatever their role.
Tests: the matrix; a member lowering the route is 403; a floor that removes asking is refused.

**44 — Hub delta: leavers and revocation end to end.** Removing a member revokes their delegations
for the project (Hub) and, through task 11's feed, Fleet stops their sessions there, closes their
sockets (task 39), withdraws their leases and marks their cards *ended: access removed*, within 60
seconds. A leaver disabled in Entra but not removed is caught by the 24-hour delegation lapse and a
daily account-state check at renewal; which Graph read KPMG's tenant permits is **UNVERIFIED**
(§14.1), so the part ships the check behind an interface with a fake and names the read to confirm.
Tests: removal to stopped session under 60 s of fake time; a lapsed delegation pauses sessions and
withdraws leases. **Fix batch b01 (I-A):** its world turns P `team` through route 87 before it opens a delegation, on a Hub with the dev flag beside sink `fake` — since b01 a delegation exists only on a team project.

**45 — Hub delta: audit to Sentinel.** Extends Hub task 27's audit route and chain: every audit
event streams to Sentinel (Log Analytics, phase 8 task 04's workspace) and the chain head is
anchored there on a schedule, so a hash chain inside the Hub's own Postgres is not its only proof
(§11.1, `ADR-H1:163`). Part of the gate before the first real teammate. Tests: a tampered row is
detected by comparing against the anchored head; the stream carries metadata only, no text body.
**Fix batch b01 (X-C1):** fills task 06's `teamModeGate` with `TeamModeAllowed`, so route 87 is
503 `audit_not_streaming` until the sink is `logs` (the dev-only flag opens it beside `fake` only).

**46 — Hub delta: notes and erasure tombstones.** `notes.go`: any `member` writes a note (≤ 1000,
`text·H`, author from token) on a project, task or person; a note reaches an Orchestrator **only**
through `panacea inbox`, quoted with its author's name, and never a worker (`07` M2). Hub task 27's
erasure gains tombstones for notes, messages and human messages, keeping each audit event. Tests:
an erased note returns a tombstone and its audit event remains; a worker's `team` never contains a
note.

**47 — Phase 8 delta: AU egress, the region guard, task 32's probes.** Phase 8's platform chart
(task 11) gains, for sandboxes of an AU project, an egress allowlist of package registries and the
git host only, since AU governs inference, not `curl` (§2.5). A guard in phase 8's `guards.sh`
(task 01) fails any Terraform or chart that places Fleet, its Postgres, the lane mirror, the
gateway or its usage store, or a sandbox pool outside Australia East. Phase 8 task 32's checklist
(`infra/smoke/kata/*`, **UNVERIFIED until merge**) gains five probes, each a script tested on canned
output: the Orchestrator pod's real size under TM1-α load; whether a guest write reaches
`machined`'s sweep within one sweep; whether a pod's source address survives to the gateway, or
mTLS is needed; team launch latency from the warm pool (target under 10 s); the PTY round trip
through `fleetbe` (target 150 ms p95 from Australia). Tests: the guard rejects a fixture in
`australiasoutheast`; each probe's parser on canned output.

**48 — Phase 3 delta: the read-only per-project fetch credential (OD-22).** Extends phase 3's
`cp/internal/githost` (tasks 13–14) with `FetchCredential(project) (Credential, error)`: a GitHub
App installation token with `contents: read` on the one repository, or an Azure DevOps identity
with read on the one repo (OD-4: both). It is held by the broker (task 17), stored in Key Vault
readable by the broker's identity only (phase 8 tasks 05, 10), used only by `machined` runners
(phase 3 task 12's slow-timer fetch, a typed `rebase`, the Orchestrator's view) and the lane mirror,
and every fetch is audited to the project. **Push and PR keep the lane owner's own token, per act,
in memory only** (phase 3 task 13). A member's own repository access is re-proven with her token
at each sign-in; a failed re-check stops fetches into her clones (§10.2 item 2). Tests: a push with
the fetch credential is refused before any network call; a failed re-proof stops the timer.

**49 — Phase 3 delta: the team land review.** Extends phase 3's land (task 09) and the merge lens
(task 19), **UNVERIFIED until merge**: on a team project, the land review lists every conflict the
lane was party to, with its resolution (from the Hub's `cf-…` records), and a lane that changes an
agent-config path (`CLAUDE.md`, `AGENTS.md`, `.claude/**`, `.mcp.json`, git-hook files,
package-script entries) needs a **second member's approval** to land (§8.3, `07` I7). The approval
is a Hub record by the approver as themselves. Tests: a lane touching `.claude/settings.json`
cannot land on its owner's approval alone; a package.json change that alters only `dependencies`
needs no second approver, one that alters `scripts` does.

**50 — Phase 4 delta: the team pointer paragraph.** Extends `cp/internal/claudemd/render.go` on
`feat/phase-4-discover` (built): for a team project, the managed block gains §5.3's one fixed
paragraph, byte for byte, and **nothing from any teammate is ever rendered into it** (`04` F4).
Solo projects are unchanged. Tests: the team render contains the paragraph once and is
deterministic; a solo render is byte-identical to before.

**51 — The prototype of every team surface.** `prototype/team-mode.html`, one self-contained file,
after reading `docs/DESIGN-SYSTEM.md` and `docs/AGENTIC-UX.md`: the team strip (at most 5 rows,
zero state *"Only you on this project."*), click-in, the four Desk row kinds, the Orchestrator
panel with inline directives and thread list, the Share line, the route badge (`Global`, `AU`,
`AU · required`) and the spend line. One heartbeat clock, no spinner per row, `tabular-nums`, Commit
Mono for ids, paths and times, Segoe UI Variable for text (OD-28), no percentages, bars, ETAs or
"N of M" (OD-27), author class beside every agent-written line. **The owner reviews it before tasks
53–56 start**; visual acceptance is the owner's eyes (`CLAUDE.md`).

**52 — Team read models on the local control plane; the desktop client.** `cp/internal/api/team.go`
relays through `fleetbe` (task 21) and the Hub: the strip read model (per person: display name,
assembled line = brief section title + task title + agent count, three latest observed paths and a
count, one status per row), a person's detail, Desk items (escalations, informs, DECIDED WITHOUT
YOU, HORIZON), the Orchestrator conversation (human messages out, kept turn replies in), escalation
answers, notes and route raise. `desktop/src/api/team.ts`, `store/slices/teamSlice.ts` and
`store/types.ts` mirror the JSON field for field (ST.8), with every `text·A` field typed as such.
Tests: the read model never includes a teammate's terminal, transcript or prompt; the client's
types match the fixtures.

**53 — The team strip and click-in.** `desktop/src/components/team/*`, as the approved prototype:
one band at the top of a team project's view and nothing added to the rail (*"navigation only"*,
`docs/AGENTIC-UX.md:144`); one row per person with a live agent, at most 5, then *"+N more"*; the
assembled line (never a model summary); observed files with a 1px ring when someone else also holds
the path; `●` working, `○` waiting, *"waiting on <name>"* only from `blocked --on human` or the
hook's `Notification`, else `idle`; a change flashes and decays over about 600 ms, `transform` and
`opacity` only. Click-in expands in place: tasks with their brief-section links (the provenance
thread), agents as STATE / MEANING / NEXT cards, files and leases, and one action, *"Ask my
Orchestrator about this"*, which opens **your own** panel with the subject attached. No control that
steers a teammate's agent exists. Tests (Vitest + RTL): six people render five rows and *"+1
more"*; no row has its own timer; the design-system audit test passes.

**54 — Desk rows.** `desktop/src/components/desk/team*.tsx`: in the Desk's fixed sections, no new
section: HORIZON (Fleet's forecasts, kept until `resolved` or `cleared`, never dropped on
`accepted`), NEEDS YOU (a decide: question, up to 4 options with consequences, the Fleet-set
deadline in `tabular-nums`, the safe default, answered in place without navigating), SINCE YOU LAST
LOOKED (informs and each free-text `answer` your Orchestrator sent, batched, never a toast),
DECIDED WITHOUT YOU (every conflict closed without you, with Undo where reversible). A teammate's
agent never appears on your Desk by itself. Tests: answering a decide calls the answer route once;
informs batch; no toast is created.

**55 — The Orchestrator panel.** `desktop/src/components/orchestrator/*`: a conversation with **your**
Orchestrator (you type a human message; its kept reply is shown verbatim as plain text); every
free-text directive issued in a turn you started is shown inline as *"→ told <worker>: …"*; beside
it, open threads as typed state (the other party, the subject, *"round 2 of 3"*, the deadline);
opening a thread shows its messages with the author class on every line (*"Alice's Orchestrator
(agent)"*). `text·A` is rendered as plain text: no markdown links, no HTML, no terminal rendering
(§7.3). DIVE opens the Orchestrator's own terminal for its owner only. Opened from the project view
or ⌘K. Tests: a message containing a markdown link renders as text; the author class is on every
line; DIVE is absent for a non-owner.

**56 — Route badge, Share line, spend line; the data class at creation.**
`desktop/src/components/team/{RouteBadge,ShareLine,SpendLine}.tsx`: the badge in the project header
(`Global`, `AU`, `AU · required` with no switch for an AU-only class); on a solo project *"Use AU
for this project"*, on a team project *"Use AU for everyone on this project"*, a one-way ratchet the
strip announces, lowered only by *compliance*; reuses phase 6's inference panel state (task 22). The
Hub's Share popover (Hub task 42) gains one line: *"Team agents run in Panacea's cloud. Your
teammates' agents will see your agents' tasks and files."* After a linked share it turns the
project team through task 21's local route over Hub route 87, and says a 403 or a 503
`audit_not_streaming` in words without undoing the share (fix batch b01). The data class is **chosen when the
project is created** and can be changed later (OD-24), in the project-create flow. The spend line,
in project settings, splits *workers* and *coordination* against the user's quota, in words and
figures, never a bar. Tests: an AU-only project renders no switch; the ratchet button is absent once
AU is set.

**57 — TM1 proof.** `docs/verification/2026-09-25-phase-t-tm1-results.md`: two real people, one repo,
the real environment, and §12.1's eight criteria, each with its evidence or its failure: (1) an
overlapping plan raises tier P before either worker spawns; (2) `yield` or `sequence` settles it,
one human gets an inform and the other a decide, and DECIDED WITHOUT YOU reaches both; (3) G9's
live run gives zero free-text directives in Fleet-started turns and zero out-of-scope directives,
and the PTY writer refuses every non-template byte; (4) the median thread resolves without a
deadline escalation, with hop time; (5) usage rows attribute every token to person, project and
counterpart, and overhead is measured; (6) an AU project never makes a Global call and a token
copied off its pod is refused; (7) a killed Orchestrator pod loses no lease, thread or directive;
(8) every directive is a `dv-…` record that answers *"who asked for this?"*. Plus the C6 rate and
the gate before the first real teammate: Z5 passes, attach tests pass, audit streams to Sentinel.
Holds the 3-day G9 contingency.

### TM2

**58 — Phase 8 delta: the lane mirror's Kata pod.** Extends `cp/internal/sandbox/manager.go` (task
15's `Spec.Class`) with the `mirror` class: one pod per team project, in Kata, with its own project
disk, **no credential beyond the read-only fetch credential** (task 48) and no other egress (a
platform-chart NetworkPolicy with Fleet's control route as its only ingress, P8-D12), git pinned by
digest in its image, no sandbox mounting its disk (§8.2, `07` I11). The mirror's disk and lanes are
deleted when the project goes, and a member's lanes when they leave (Q7's recommendation, not an
owner decision: the part marks it so). Tests on the fake clientset: the mirror pod mounts no
machine disk, has no gateway egress and no token Secret.

**59 — Phase 3 delta: the forecast as a library; the mirror runner.** Phase 3 task 08's chained
forecast (`cp/internal/converge/forecast.go`, **UNVERIFIED until merge**) becomes a library the
owner's control plane runs for a solo project and the mirror runs for a team one.
`sbx/cmd/panacea-mirror` (the mirror's PTY-less command) and `cp/internal/fleet/mirror`: for each
incoming lane bundle, `git bundle verify`, then fetch with `transfer.fsckObjects=true` under phase
3's §HGH hardening into `refs/panacea/lanes/<owner-hash>/<wt-id>`, then `git merge-tree
--write-tree` for the pairs asked, returning **only** ST.7's typed result (tier, paths, line ranges,
basis shas) to Fleet. Nothing under `refs/panacea/*` is ever pushed to the customer's remote. Tests:
a bundle that fails `verify` is dropped and reported; a corrupt object fails `fsckObjects`; the
result carries no file content.

**60 — Tier-M forecasts; lane bundles into the mirror.** After each commit on a team lane, the lane
owner's `machined` exports the lane since its merge base as a bundle (phase 3 ruling R4) to the
mirror through the broker; Fleet asks the mirror for tier M **only for pairs P or H flagged**
(§8.2), so work grows with activity, not the square of the team; the forecaster raises or updates
`cf-…` at tier `M`. Tests: an unflagged pair never reaches `merge-tree`; a clean textual merge on a
P-flagged pair still yields a conflict record if paths overlap, marked mergeable.

**61 — The team land queue over the mirror.** `cp/internal/fleet/landq`: one queue per team
project for every member's lanes, ordered by task 29's `sequence` hints; each land is still its
owner's reviewed act (task 49's review); the integration head is built in the mirror, handed as a
bundle to the lane owner's `machined`, and pushed with **that owner's own credential** (phase 3
task 13); every member's repository then fetches the new head from the mirror. Tests: a `sequence`
hint orders two lands; a land with an unresolved agent-config approval is refused; the push uses the
lane owner's token and never the fetch credential.

**62 — Resolutions `counter`, `split`, `build-on`.** Extends task 29: `counter` replaces the
proposal and counts as a round; `split` re-issues each side's leases to its own globs; `build-on`
brings the named lane into the other side's worktree as a bundle through the mirror and is
**always a decide** (task 30), with every agent-config path it changes on its own line (§6.5,
§8.3). Tests: a `counter` past the round cap escalates; a `build-on` accept without the human's
answer applies nothing; the decide lists `.claude/settings.json` when the lane changes it.

**63 — Phase 3 delta: lenses by principal.** Phase 3's merge and files lenses (tasks 16, 19, 21,
**UNVERIFIED until merge**) name lane owners by Hub principal, never git author (`04` §7.3), and
show teammates' lanes from the mirror (paths and line counts only). Tests: two lanes with the same
git author and different principals show two names; no file content from a teammate's lane is
fetched for display.

**64 — "Move my lanes to the cloud".** `cp/internal/api/lanesmove.go`: on a shared project, the
owner's explicit click exports each unpushed laptop lane as a bundle (phase 3 task 24's runner, R4)
into the owner's cloud machine, where Fleet takes it over; nothing moves without that click
(§10.1 item 4). Not costed in `05-design.md` §12; this plan places it in TM2 and says so. Tests: no
lane moves without the call; a lane already on the machine is not duplicated.

**65 — Phase 8 delta: the CLI bump job.** Extends phase 8's `pipelines/images.yml` (task 27, with
task 27a's canary, **UNVERIFIED until merge**): a job that learns of a new Claude Code release
(how is **UNVERIFIED**, §10.4), builds a new sandbox image by digest, runs four gates (the OD-8
canary; the `panacea` channel tests, including that a worker still runs `panacea check` before an
edit; G9's corpus, task 42; a gateway compatibility call through phase 6's gatekeeper), and
promotes the digest to the warm pool. New launches and every Restart get it; a running session
keeps its version and its card shows it. Proposed target: the warm pool within 24 hours of a
release. Tests: a failing gate blocks promotion; the promoted reference is a digest, never a tag.

**66 — Team and project quotas, server side.** Reads and writes the gateway's budgets (phase 6
tasks 06 and 09, S6.2: person → LiteLLM user budget, project → team budget, team → keysmith at
mint) from the Hub's team policy (task 43): an org admin sets a ceiling per person, a team admin
sets project budgets inside it, both with non-zero defaults (OD-24, Q3). Fleet's spend split
(task 37) is served per member and per counterpart. Tests: a project budget above the org ceiling
is refused; an unset quota resolves to the non-zero default, never to unlimited.

**67 — The quotas admin view.** `desktop/src/components/team/QuotaAdmin.tsx`, from the approved
prototype's language: per-member and per-project budgets in words and `tabular-nums` figures, the
split *workers* and *coordination*, coordination per counterpart (*"coordination with Alice: 41k
tokens today"*); editable by a team admin within the org ceiling. No percentages, bars or
projections (OD-27). Tests: a non-admin sees read-only values; no element renders a progress bar.

**68 — Release attribution on a team project.** Extends the Hub's release records (Hub tasks 23–24,
**UNVERIFIED until merge**): a release in a team project lists, per lane landed into it, the person
whose agent made it, the `tk-…` task and the brief section it served, so the provenance thread
reaches the release with its people (§11.5). Phase 7, which calls Panacea's API, reads it; phase 7
has no plan on disk yet, so the part defines the Hub side only. Tests: a release with two lanes
from two people lists both with their brief sections.

**69 — TM2 proof.** `docs/verification/2026-09-25-phase-t-tm2-results.md`: the mirror pod's
isolation (no egress, no credential beyond fetch, no sandbox mount); a tier-M forecast on a real
pair; a team land through the mirror pushed with the owner's credential; a `build-on` decide
listing an agent-config path; the bump job's dry run through its four gates; quotas enforced at
the gateway. Each item with what was and was not observed.

---

## What this plan does not claim

- That anything is built. Every sibling path marked planned is **UNVERIFIED until merge**, and
  task 33 may still re-plan TM1.
- That the fences stop a determined attacker. Persuasive, well-formed text still passes (§7.3); G9
  measures, it does not prove.
- **KPMG InfoSec approval is no longer a gate.** Under OD-23 the mechanism ships with minimal
  defaults and no drafted policy; the design's Q12 and its TM1 gate are superseded. When an org
  policy is written, it enters through task 14's routes as a new version, with no code change.
- The Linux managed-settings path, `-p` with `--session-id` then `--resume` (the flags exist at
  2.1.274, `control-plane/internal/harness/argv.go:91-97`), a `--setting-sources` flag, whether a
  file-based managed hook prompts, and which `env` keys a managed file locks are **UNVERIFIED** at
  the pinned CLI version; tasks 27 and 33 and the OD-8 canary settle them.
- Pod-source token binding, guest-write visibility to `machined`, Orchestrator pod size, warm-pool
  launch time and the PTY round trip are **UNVERIFIED** until task 47's probes run.
- Which Graph read KPMG's tenant permits for the leaver check (task 44), and the gateway's
  Bedrock federation (phase 6's), are **UNVERIFIED**.
- TM3 (a live Orchestrator over the inbox socket, the watch view, tier S builds, an MCP front) and
  phase 5's canvas lease are not in this plan.
