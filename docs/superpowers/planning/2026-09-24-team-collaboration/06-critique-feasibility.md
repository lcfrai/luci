# 06 — Critique: feasibility and integration of `05-design.md`

**Role:** adversarial critic, feasibility and integration. **Date:** 2026-09-25.
**Reviewed:** `05-design.md` and `00-summary.md` in this folder, against the real plans and code:
the Hub contract and ADR (`.claude/worktrees/hub`), the phase 3 plan and R4
(`.claude/worktrees/phase-3`), the phase 8 plan and broker (`.claude/worktrees/env`), and phase 2's
control plane (`control-plane/`). Binding: OD-1 to OD-20
(`../2026-09-23-owner-decisions.md`). Line numbers into the worktrees may drift, because those trees
are live. **UNVERIFIED** marks every claim this file did not check against a source or a run.

**Status:** in progress. Findings are appended below as they are checked.

## Critical

### C1. The Orchestrator's headless turn loop has no process to run in

- **Claim:** each Orchestrator turn is `claude -p --resume <session>`, *"one turn at a time"*, in
  the Orchestrator's own Kata sandbox (`05-design.md:276-279`); Fleet sizes the loop at ~3 days as
  "new" (`05-design.md:868`).
- **Reality:** `sandboxd` starts **one** command in **one** PTY for the pod's whole life
  (`sandboxd/cmd/sandboxd/main.go:39-69`) and serves only `/attach` and `/healthz`
  (`sandboxd/internal/server/server.go:85-86`). Phase 8 keeps that contract unchanged
  (`.claude/worktrees/env/docs/superpowers/plans/2026-09-23-phase-8-own-environment.md:84-88`)
  and bans `pods/exec` outright (P8-D7, `:136`). So nothing can start a second `claude -p` in a
  running pod. The options are all undesigned: a new pod per turn (Kata start per event batch, and
  a transcript that must outlive the pod), a new `sandboxd` verb (breaks phase 8's "does not
  change"), or a product-owned loop process as the PTY command.
- **Second gap, same place:** `--resume` needs the transcript on disk. Phase 8 keeps `~/.claude` as
  an `emptyDir` and only `~/.claude/projects` on the machine disk, under
  `conversations/<worktree id>/` (`phase-8-own-environment.md:675`, P8-D6 `:135`). The Orchestrator
  has no worktree (`05-design.md:257-259`), so it has no conversation directory.
- **Third gap:** phase 2's Resume deliberately **forks** (`--fork-session`), because reusing an id
  puts two processes on one conversation (`control-plane/internal/harness/argv.go:91-100`). §4.2's
  DIVE (`05-design.md:285-287`) opens `claude --resume` on the loop's session. Built on phase 2's
  harness it forks, and the loop's next turn resumes the old id and loses what the owner did.
- **Fix:** specify the `orchestrator` class as a product-owned runner (`panacea-orchestrator`) as
  the PTY command. It blocks on the control stream, and per batch runs
  `claude -p --resume <fixed id> --output-format stream-json` as a child, with no `--fork-session`.
  Its conversation directory is `conversations/orch-<project>/`. DIVE attaches to the same id
  only while the runner is parked. Add it to §11.2 and §11.4 at about +3 days, and add a TM1 test
  that kills the pod between two turns and resumes.

### C2. Orchestrators eat the worker slots on each user's machine node

- **Claim:** one Orchestrator per user **per team project** (`05-design.md:46`), in its own Kata
  sandbox, with a read-only view of the project *"refreshed by `machined`"* (`05-design.md:257-259`).
  The Orchestrator class is costed at 1.5 days and given no size (`05-design.md:827`).
- **Reality:** P8-D6 puts every session pod of a machine on the **same node** as its machine pod,
  by required pod affinity, because they mount sub-paths of one RWO disk. That gives *"about three
  sessions per machine on an otherwise empty node"*, and the broker answers `409 machine_full`
  past that. A session's size is broker policy, not the caller's, so an Orchestrator gets a full
  worker's 2 CPU / 4 GiB by default (`phase-8-own-environment.md:135`).
- **Failure:** the Orchestrator's view comes from the owner's machine disk, so it takes one of
  about three slots. Dylan on two team projects has two Orchestrators and **one** worker slot left.
  TM1's done test (`05-design.md:919-927`) needs an Orchestrator plus at least one worker per
  person; a third launch returns `machine_full`. `claude -p` idles at almost zero CPU, but it
  holds a full slot while the loop waits.
- **Fix:** give the `orchestrator` class **no machine disk and no affinity**. Its view of the
  project comes through `panacea` verbs, answered by Fleet from `machined` over the broker (in
  TM1) or from the lane mirror (TM2). Give it its own small size (for example 250m / 512 MiB; to
  be measured in task 32's P07). Or make the Orchestrator one per user, spanning all of that
  user's team projects. Either way, write the size and placement into phase 8 task 14.

### C3. Whichever way `/status` answers, the `panacea` channel has no designed policy

- **Claim:** the biggest unknown is whether KPMG's managed policy reaches a gateway-token sandbox,
  settled by running `/status` (`00-summary.md:75-76`, `05-design.md:975`). If it does not, the
  image carries a product `managed-settings.json` *"at least as strict as the tenant's"*
  (`05-design.md:181-185`). The `panacea` verb is judged safe because *"it is not on the laptop's
  63-rule `ask` list"* (`05-design.md:376-377`).
- **The live policy**, read from `~/.claude/remote-settings.json` on 2026-09-25:
  `forceLoginMethod: "claudeai"`, `forceRemoteSettingsRefresh: true`,
  `allowManagedPermissionRulesOnly: true`, `permissions.defaultMode: "auto"`,
  `disableBypassPermissionsMode: "disable"`, 144 `allow` rules with **no `Bash(panacea …)`**, 49
  `ask` rules (not 63), `allowedMcpServers: []`, `allowManagedMcpServersOnly: true`, and
  `strictPluginOnlyCustomization: ["hooks", "mcp"]`.
- **If the policy did reach the sandbox:** `forceLoginMethod: "claudeai"` may refuse a
  gateway-token session outright (**UNVERIFIED**). Local allow rules are ignored, so `panacea`
  is decided by auto mode's classifier on every call. A headless `-p` Orchestrator cannot answer a
  prompt, so a call the classifier does not approve is a denial, and the Orchestrator cannot act.
  Phase 2's `bypassPermissions` choice (`control-plane/internal/harness/argv.go:18-21`) is
  disabled too.
- **The docs already answer the question, and the answer is "it does not reach it".** Server-managed
  settings are *"bypassed"* when a session sets *"a non-default `ANTHROPIC_BASE_URL`"*, and *"Claude
  Code skips the settings fetch"* for such a session. Only *"the managed settings file in the runner
  image"* applies to a self-hosted environment
  (https://code.claude.com/docs/en/server-managed-settings, read 2026-09-25). §2.5's sandbox sets
  exactly that variable (`05-design.md:163-166`). So the cloud agents sit outside the governance
  behind *"All activity is logged"* (the policy's `companyAnnouncements`). A copy "at least as
  strict" cannot carry `forceLoginMethod: claudeai` without breaking itself. What governs the cloud
  is then a **KPMG InfoSec approval** of the product's own file, not an engineering test. KPMG's
  own tenant is **UNVERIFIED**; the docs are not.
- **What that opens up:** the same page says a self-hosted **Claude apps gateway** *"provides the
  equivalent remote managed-settings delivery: gateway-signed-in clients fetch managed settings
  from the gateway"*. That is a strong point for the Anthropic gateway in Q1 (`05-design.md:953`),
  and §13 does not weigh it. A managed `claudeMd` key also delivers instruction text without
  approval (same page). It is the natural home for the Orchestrator's instructions, which §4.1
  never places (C1).
- **Why it is critical:** C4's whole channel is a Bash call (`05-design.md:353-385`). Being absent
  from an `ask` list does not mean allowed: an unlisted Bash command falls to the default mode.
- **Fix:** decide now, as a TM0 item, that the cloud image ships a product
  `managed-settings.json` derived from the tenant's. It allows `Bash(panacea *)`, pins
  `permissions.defaultMode` for each class (the Orchestrator's headless mode must never need a
  prompt), and leaves `forceLoginMethod` unset. Put it to KPMG InfoSec with the gateway's audit
  export as the replacement trail. Keep `/status` as the check that the product file, and nothing
  else, is in force.
