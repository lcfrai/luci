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
   Orchestrator may only pause, narrow or sequence its own workers; anything else becomes a
   `decide` to its human (`05-design.md` §7.2).
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
   message, directive or escalation, whatever their role (§3.3 rule 5).
4. Records carry `ag-…` ids. No machine-local session id, host path, config key or URL reaches
   the Hub. Paths are repo-relative globs, validated server-side.
5. Every free-text field is bounded (the limits in §3.3), hygiene-checked, and carries its author
   class, `text·A` or `text·H`.

**Trust.**

6. **Agents see facts, not words.** Activity is observed by `machined` outside the VM. A worker
   never receives a teammate's free text; an Orchestrator receives it only as quoted data.
7. **Turn origin governs authority** (§7.2). In a Fleet-started turn an Orchestrator may only
   pause, narrow or sequence its own workers; any other verb becomes a `decide`. The check is in
   Fleet's verb handler, not in the prompt.
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
| `cp/internal/backend/fleetbe/fleetbe.go`, `cp/internal/api/teamlaunch.go` | The desktop's `fleetbe` behind `backend.Backend`; the team launch that creates the delegation | 21 |
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
| `cp/internal/api/team.go`, `desktop/src/api/team.ts`, `desktop/src/store/slices/teamSlice.ts`, `desktop/src/store/types.ts` | Team read models and actions on the local control plane; the desktop client | 52 |
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
| 06 | Hub routes, part 1: delegations, tasks, agents, lease events, conflicts | TM1-α | 0.375 (3 h) | 05, H14, H17 | ST.2, ST.3 |
| 07 | Hub routes, part 2: threads, messages, escalations, directives, turns, human messages | TM1-α | 0.375 (3 h) | 06 | ST.2 |
| 08 | Hub feed kinds, Fleet's per-org subscription; `hubclient` team methods, fake, conformance | TM1-α | 0.375 (3 h) | 07, H26, H33, H34 | ST.2 |
| 09 | Fleet model and the `panacea` control wire v1 | TM1-α | 0.3 (2.5 h) | 02 | ST.1 |
| 10 | Fleet's Postgres schema and `Store` | TM1-α | 0.375 (3 h) | 09 | — |
| 11 | Fleet as a delegated Hub caller: outbox, delegation client, feed, rebuild | TM1-α | 0.375 (3 h) | 08, 10 | ST.3 |
| 12 | `cmd/fleetd`: config, Entra per request, role-to-verb, singleton loops | TM1-α | 0.375 (3 h) | 10, 11, P8-16 | — |
| 13 | OD-23 mechanism, part 1: layers, merge, invariants, digest, minimal defaults | TM1-α | 0.375 (3 h) | — (P6-14's fragment once merged) | ST.4, S6.6 |
| 14 | OD-23 mechanism, part 2: versioned store, admin routes, audit, version on agent start | TM1-α | 0.3 (2.5 h) | 06, 12, 13 | ST.4 |
| 15 | Phase 8 delta: sandbox classes (task 14b), the policy mount, `sandboxd`'s digest check | TM1-α | 0.375 (3 h) | 13, P8-14, P8-15 | ST.4 |
| 16 | Phase 8 delta: `sandboxd` `/control`, the in-pod socket, the `team.json` snapshot | TM1-α | 0.375 (3 h) | 09 | ST.1 |
| 17 | Phase 8 delta: the broker's second caller, the control relay, pod source addresses | TM1-α | 0.375 (3 h) | 16, P8-17, P8-18 | ST.3, P8-D4 |
| 18 | The `panacea` verbs in the sandbox binary | TM1-α | 0.3 (2.5 h) | 16, P6-14 | ST.1 |
| 19 | Fleet's control streams and verb dispatch | TM1-α | 0.375 (3 h) | 12, 17 | ST.1 |
| 20 | Fleet sessions over the broker; `ag-…` ids; gateway tokens with team tags | TM1-α | 0.375 (3 h) | 11, 14, 15, 19, P6-15, P8-22 | S6.3, S8.1 |
| 21 | `fleetbe` and the team launch on the local control plane | TM1-α | 0.375 (3 h) | 06, 20 | S8.1, ST.3 |
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

---
