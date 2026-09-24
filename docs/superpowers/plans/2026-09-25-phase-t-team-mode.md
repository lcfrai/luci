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
