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
| **OD-14** | Team agents run centrally, as managed Kata sessions Fleet coordinates; the desktop is a client. Reverses ADR-H1's "sessions stay local" for team projects and phase 8's owner-only broker visibility. | 06, 11, 12, 13 |
| **OD-15** | Agent awareness first; humans get a calm summary (who, one line, files). No teammate terminal by default. | 16, 17, 35, 36 |
| **OD-16** | One Orchestrator per user is the only cross-user channel; it settles or escalates to **inform** or **decide**. No user steers another's agent. | 18, 19, 27–31 |
| **OD-17** | Design now, build next; the Hub, phase 8 and phase 3 are re-scoped where team mode touches them. In-flight streams stay foundations. | every delta task |
| **OD-18** | Azure. Phase 8 resumes unchanged in shape. | 11–15 |
| **OD-19** | Inference through the org gateway, Global (Anthropic, default) or AU (Bedrock Sydney), a scoped per-session token per sandbox, never an org key, never a personal subscription. | 14, 15, 20 |
| **OD-20** | The desktop is the portal; local mode stays; the cloud must track the latest Claude Code CLI. | 34, 44 |
| **OD-21** | The gateway is **LiteLLM**. It resolves the design's Q1: this plan calls phase 6's virtual-key API and builds no gateway. | 20 |
| **OD-22** | A **read-only, per-project fetch credential**, used only for fetches; every push uses a signed-in person's own credential. | 04, 15, 24 |
| **OD-23** | **The mechanism, not the policy.** The control plane supplies Claude Code managed settings per org, and per project where needed, into every cloud sandbox at session start, versioned and audited. A minimal default ships; no InfoSec policy document is drafted. It **supersedes** the design's "product `managed-settings.json` submitted to KPMG InfoSec" (`05-design.md` §2.5, §11.2, Q12) as a TM1 gate. | 07, 08, 09, 13 |
| **OD-24** | Budgets on by default; escalation defaults only tighten; opening prompts private; a data class **set at project creation**, updatable, driving AU vs Global and the deploy scan set. | 03, 20, 29, 31 |
| **OD-25** | Phase W runs alongside; team mode's critical path is untouched by it. Nothing here waits on W. | — |
| **OD-27** | Proof per criterion, no predictions: no percentages, progress bars, ETAs or "N of M" in any team surface. | 35–37 |
| **OD-28** | Ids, paths and times in Commit Mono; sans Segoe UI Variable. | 35–37 |
| **OD-2, OD-4** | One clone per worktree; Azure DevOps and GitHub both, so the fetch credential has two providers. | 04, 24 |
| **OD-3, OD-6** | Durable team records live in the Hub; the desktop signs in with Entra PKCE. | 01–05, 34 |
| **OD-8** | No component derives meaning from PTY bytes; agent CLIs are never modified, wrapped or parsed. The only PTY write Fleet makes is a fixed template (task 23). | 07, 18, 23 |
| **OD-11** | Clean room: no canvas or Linear source is opened or copied. Phase 5's artboard lease is out of scope here. | — |
| **OD-13** | No Panacea source is copied; patterns only. | 11, 14 |

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
  is deferred to task 45 or 46, and why.

---
