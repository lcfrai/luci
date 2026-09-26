# 06 — Critique: feasibility and integration of `05-design.md`

**Role:** adversarial critic, feasibility and integration. **Date:** 2026-09-25.
**Reviewed:** `05-design.md` and `00-summary.md` in this folder, against the real plans and code:
the Hub contract and ADR (`.claude/worktrees/hub`), the phase 3 plan and R4
(`.claude/worktrees/phase-3`), the phase 8 plan and broker (`.claude/worktrees/env`), and phase 2's
control plane (`control-plane/`). Binding: OD-1 to OD-20
(`../2026-09-23-owner-decisions.md`). Line numbers into the worktrees may drift, because those trees
are live. **UNVERIFIED** marks every claim this file did not check against a source or a run.

**Verdict: sound in shape, not yet buildable as written.** The model implements OD-14 to OD-17
(the table near the end of this file). But three Critical gaps sit exactly where the first build
starts: the Orchestrator's runtime (C1, C2) and the `panacea` channel's permission policy (C3).
Fix those, plus I1 to I5, before TM1 is planned in tasks. **3 Critical, 11 Important, 8 Minor.**

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

## Important

### I1. "Observed" activity is computed inside the agent's own process tree

- **Claim:** activity is *"observed, never reported"*, and *"the `sandboxd` watcher runs outside
  the agent's process tree"* (`05-design.md:244-246`). G1 lists `sandboxd` as a writer of facts
  (`05-design.md:551`).
- **Reality:** `sandboxd` is the agent's **parent**. The entrypoint `exec`s `sandboxd`, which
  starts the harness in its PTY (`images/sandbox/entrypoint.sh:13-17`,
  `sandboxd/cmd/sandboxd/main.go:39-69`). Both run as `dev`, uid 1001
  (`images/sandbox/Dockerfile:19`, `:24`). A hijacked worker can stop the watcher, exhaust its
  inotify watches, or write to the `panacea` socket directly. Dirty hunks need `git diff` in the
  lane, and phase 3 has **dropped** the read-only `.git/config` bind, so *"the agent's `git
  config` writes work"* (`.claude/worktrees/phase-3/docs/superpowers/plans/2026-09-23-phase-3-git-convergence.md:447`).
  A `core.fsmonitor` the agent sets would then run inside `sandboxd`, the process that stamps
  facts.
- **Also:** "files **read**" from inotify (`05-design.md:221`) is noise. One `rg` or `grep -r`
  opens every file in the tree.
- **Fix:** compute activity **outside the VM**. `machined` mounts the same disk, and it already
  runs git under phase 3's §HGH hardening (P8-D7, `phase-8-own-environment.md:136`). A 5 to 10 s
  hardened `status` / `diff --numstat` sweep there feeds Fleet, and it is also phase 3 task 07's
  observed-claims path. Keep `sandboxd` for process facts only. Drop "files read" from TM1.

### I2. Awareness still hangs on the model obeying one sentence, when the cloud permits a hook

- **Claim:** workers run `panacea check <path>` before a first edit because the managed block tells
  them to (`05-design.md:301-303`, `:394-395`). Hooks are rejected under P8 (`05-design.md:88`,
  `:421`), citing the laptop policy.
- **Reality:** `03` already says this kind of instruction is *"advisory: whether the model re-reads
  on cue is behaviour, not a guarantee"* (`03-techniques.md:138`). Its C6 run was never done
  (`03-techniques.md:613`). By C3, the laptop policy that strips hooks does not reach a
  gateway-token sandbox. The product's own `managed-settings.json` governs there, and can define a
  `PreToolUse` hook on `Edit|Write|MultiEdit`. That hook runs `panacea check`, returns directives
  as context, and makes the hot path deterministic.
- **Fix:** keep "no hook **dependency**". Add the hook as a cloud-only enhancer, behind the same
  InfoSec approval as C3. The observed-edit `hold` lease stays the backstop
  (`05-design.md:470-472`), once I1 makes it trustworthy. Add C6 to §14.2 as a measured rate:
  of N first edits, how many were preceded by a check.

### I3. No git credential exists for anything the cloud does while the human is away

- **Claim:** Fleet keeps no user refresh token (`05-design.md:141-142`). Push is a human act with a
  per-push token from the desktop; Azure DevOps uses on-behalf-of (`05-design.md:186-191`).
- **Reality:** the design needs **fetch** with nobody present. The Orchestrator's view is at the
  integration head *"refreshed by `machined`"* (`05-design.md:258`), a tainted Orchestrator may
  order `rebase` onto upstream (`05-design.md:567`), and phase 3 fetches *"on a slow timer"*
  (phase 3 plan task 12, `2026-09-23-phase-3-git-convergence.md:561`). Phase 3's credential is Git Credential Manager and Windows
  Credential Manager on the owner's laptop
  (`.claude/worktrees/phase-3/docs/superpowers/plans/2026-09-23-phase-3-git-convergence-parts/task-12.md:7`,
  `task-13.md:3`, `:32`). On-behalf-of needs a live user assertion, so it fails in exactly the
  "away" case too.
- **Fix:** an owner decision before TM1. Recommended: a **read-only**, project-scoped fetch
  credential (a GitHub App installation token; an Azure DevOps read-only identity), with every
  fetch audited to the project. **Write** (push, PR) stays the user's own token, handed over per
  act. Add it to §13.1 as Q6.

### I4. Fleet as a delegated Hub caller inverts three Hub invariants, and it is costed at 2 days

- **Claim:** Fleet calls the Hub as its own workload identity, names a delegation, and the Hub
  records *"actor = the owner, via = Fleet"* (`05-design.md:135-140`), at ~2 days
  (`05-design.md:811`). The delegation client is built *"from Hub task 30's `entraauth`"*
  (`05-design.md:865`).
- **What it collides with in ADR-H1** (`.claude/worktrees/hub/docs/superpowers/adr/ADR-H1-hub-architecture.md`):
  - `azp` must be in `PANACEA_APP_HUB_ALLOWED_CLIENTS`, *"the desktop's client id. A token some
    other app obtained for our API is refused"* (`:370`).
  - The principal is `(tid, oid)` from the token (`:372`), and *"no request body carries an actor"*
    (`:434-435`). A delegation id **is** an actor carried in the request.
  - Tenancy is Postgres RLS on `org_id` (G4, `:253`). Fleet is one multi-tenant caller, so the Hub
    must set the org per request from the delegation, not from the token.
  - Revocation *"within 60 seconds"* (`05-design.md:143-146`) needs Fleet to read **every org's**
    change feed (G3, `:252`), which is a cross-tenant read the Hub has no role for.
- **And in the Hub plan:** task 30's `entraauth` is the desktop's public-client PKCE flow
  (`.claude/worktrees/hub/docs/superpowers/plans/2026-09-23-phase-h-hub.md:431`). On-behalf-of is
  a confidential-client exchange, and nothing of it exists. `devissuer` does code + PKCE and
  refresh only (`phase-h-hub.md:412`), so Hub task 43's offline proof cannot exercise Fleet.
- **Fix:** write an ADR-H1 amendment, "delegated callers", before TM1. It covers the token shape
  Fleet presents, the `azp` list, org selection under RLS, a per-org feed subscription for Fleet,
  and `devissuer` gaining the jwt-bearer grant. Re-cost the row at about 4 to 5 days.

### I5. The desktop has no transport to Fleet, and the "portal" still runs a local control plane

- **Claim:** for a team project the cockpit *"talks to Fleet"* (`05-design.md:98`). Stream F is UI
  only (`05-design.md:907`).
- **Reality:** phase 8 says *"The desktop keeps talking to `127.0.0.1:8730` only"*, through
  `remotebe` in the laptop's control plane (`phase-8-own-environment.md:81-83`). The user's tokens
  live in that local Go process (HA-2, `:250`; `phase-h-hub.md:15`). So either the local control
  plane proxies Fleet, or the renderer holds tokens, which breaks the Hub's custody. The design
  picks neither. A team session's PTY then crosses desktop → local control plane → Fleet →
  broker → `sandboxd`: three relays, none measured.
- **Fix:** add a `fleetbe` backend in the local control plane, behind the unchanged
  `backend.Backend` seam (S8.1, `phase-8-own-environment.md:657`), plus the team routes. Cost it at
  about 3 to 4 days, and add it to stream D or F. Say plainly in §10.3 that the portal is the
  cockpit plus a thin local control plane: no Docker, but one local Go process. Add PTY
  round-trip latency to task 32.

### I6. `waitingOn: human` has no lawful source

- **Claim:** agent status includes `waiting`, with `waitingOn` `human`·`lease`·`peer`
  (`05-design.md:220`). The strip shows *"waiting on Priya"* (`05-design.md:659`, `:670`).
- **Reality:** phase 2 refuses to invent this state. BLOCKED-ON-YOU *"needs the agent-runtime
  signal"* and *"must not be guessed from PTY bytes"*
  (`control-plane/internal/registry/session.go:10-13`). A permission prompt or a question to the
  user shows only in the PTY (OD-8 forbids reading it) or in the optional transcript adapter.
- **Fix:** `lease` and `peer` are Fleet's own facts, so keep them. Make `human` come only from
  `panacea report blocked --on human` (agent text, labelled so) or from the cloud-only
  `Notification` hook of I2. When neither is present, show `idle`, as phase 2 does.

### I7. Fleet is one in-memory process, and "agents pause" when it dies has no mechanism

- **Claim:** Fleet is one multi-tenant deployment (`05-design.md:107-110`). Its state is *"in
  memory with a snapshot for restart"* (`05-design.md:208-209`), and the live lease table is
  authoritative there (`05-design.md:222`). *"If Fleet goes down, agents pause and no record is
  lost"* (`05-design.md:117`).
- **Reality:** nothing pauses a worker. It is an interactive CLI in its own pod, and it keeps
  editing when Fleet is gone. Its `panacea` calls fail, and it falls back to a stale
  `/panacea/team.json` (`05-design.md:398-400`). Every Fleet rollout, which will be weekly in
  early build, drops every control stream, turn queue and lease at once, for every org. Phase 8
  runs even the Hub as two stateless replicas (`phase-8-own-environment.md:264`).
- **Fix:** keep leases, turn queues and thread state in Fleet's own Postgres schema, so a restart
  or a second replica loses nothing. Split the PTY relay from the coordination core. Define
  `panacea` behaviour while Fleet is unreachable: `check` answers *"team view unavailable since
  <t>"*, and the worker's block says to report `blocked` and not to edit a path it holds no lease
  on. Replace "agents pause" with what actually happens.

### I8. The negotiation timings make the 15-minute deadline the normal way threads end

- **Claim:** events coalesce for up to 60 s per turn (`05-design.md:271-272`). A thread has
  3 rounds per side and a 15-minute deadline, and running out forces `decide` on **both** humans
  (`05-design.md:486-488`).
- **Arithmetic** (**UNVERIFIED**, turn length not measured): one hop is up to 60 s of coalescing
  plus one headless turn. The turn resumes a growing context, then makes at least one
  `panacea inbox` call and one `panacea peer` call, so assume 20 to 60 s. That is 80 to 120 s a
  hop, and 3 rounds per side is 6 hops, or 8 to 12 minutes, before any `inbox`/`team` re-reads.
  So a thread that uses its rounds lands on the deadline. Each one then puts a `decide` on two
  Desks, the reverse of OD-15's "calm" (`owner-decisions.md:125`).
- **Fix:** deliver thread events with no coalescing, keeping 60 s for everything else. Set the
  default deadline to 30 minutes. Measure hop time in G9's scripted run, and add a TM1 done
  criterion: the median thread resolves without a deadline escalation.

### I9. A long-lived headless Orchestrator's context, and so its cost, grows without bound

- **Claim:** one Orchestrator session per user per project, resumed turn after turn
  (`05-design.md:276-278`). *"An idle Orchestrator costs nothing"* (`05-design.md:283`).
- **Reality:** every `-p --resume` turn re-sends the whole conversation. After a few days of peer
  threads, that is compaction territory, and each compaction is an agent-written summary of peer
  text, which is exactly what G1 and G7 try to keep out of authority. When a turn arrives after
  the prompt cache has expired, it pays full input price. The idle **pod** still holds a slot
  (C2).
- **Fix:** rotate Orchestrator sessions, one per day or per task. Seed each new session only from
  Fleet's typed state (`panacea team`, open threads, the owner's policy), never from the old
  transcript. Its taint then resets for free (`05-design.md:563-565`). Budget coordination spend
  per turn, not only per hour (`05-design.md:319`).

### I10. Azure is the orchestrator's call, and one of its stated reasons is out of date

- **Claim:** *"The cloud is Azure (AKS + Kata), so phase 8 stands"* (`05-design.md:3-4`, `:822`).
- **The record:** OD-18's answer is *"Orchestrator's call, on that criterion: Azure"*. Part of its
  reason is *"Kata on EKS needs metal instances"* (`owner-decisions.md:139`). The owner's words
  relayed into this run lean the other way: *"We would look at hosting this probably in AWS, as
  that would allow Bedrock for AU inferencing with ZDR"*.
- **Out of date:** AWS announced nested virtualization on **virtual** EC2 instances (C8i, M8i,
  R8i, all commercial regions) on 2026-02-16
  (https://aws.amazon.com/about-aws/whats-new/2026/02/amazon-ec2-nested-virtualization-on-virtual
  and https://www.infoq.com/news/2026/03/aws-ec2-nested-virtualization/, both seen as search
  results on 2026-09-25; the pages themselves were not fetched, so **UNVERIFIED** in detail). So
  Kata on EKS no longer needs metal. It is still **self-managed**, whereas AKS Pod Sandboxing is
  managed, and phase 8's other reasons (written and part-built for AKS, Entra, Panacea on Azure)
  still stand.
- **Also:** the AU route stays onshore only if Fleet, the gateway and the sandboxes all run in
  Australia East. Phase 8 recommends that region (`phase-8-own-environment.md:43`, `:317`), but
  `05` never makes it a requirement of the AU route. And Azure → Bedrock is a cross-cloud hop with
  an identity federation still to be designed (`05-design.md:835-837`).
- **Fix:** put the corrected trade-off back to the owner as one line before TM1 (it is not a
  design defect). Make "Australia East for every team-mode component" a stated invariant of the AU
  route. List which parts are cloud-bound (Kata Pod Sandboxing, workload identity, Key Vault,
  Sentinel) so an AWS move later is a known cost.

### I11. TM1's "about 5 weeks" leaves out the pieces that fail first, and TM0 is further off than it reads

- **Claim:** TM0 *"are the current wave, not new work"*, and TM1 takes ≈ 53.5 focused days with a
  23-day critical path (`05-design.md:889-913`).
- **The current wave's state, read 2026-09-25:** phase 3 has **no** `gitx` or `lanes` package yet
  (`.claude/worktrees/phase-3/control-plane/internal/` holds only phase 2's packages). Phase 8's
  latest code commit is task 14 (`09ae70a`, `36b5c25` on the `env` branch). The broker, `machined`
  and `remotebe` (tasks 17-22) are unbuilt, and first apply needs owner-run steps (Entra, DNS
  delegation, the Postgres bootstrap; `phase-8-own-environment.md:317-329`). Task 32 may yet flip
  the disk model to NFS with no affinity (P8-D6, `:135`), which changes C2.
- **Missing from the 53.5 days:** the Orchestrator runner and its transcript home (C1, ~3); the
  product `managed-settings.json` and its InfoSec approval (C3); the `fleetbe` transport (I5,
  ~3-4); the Hub delegation amendment (I4, +2-3); Fleet's persistence (I7, ~2); the Orchestrator's
  instructions and an eval loop to tune them. G9 is assumed to pass *"first time"*
  (`05-design.md:913`), and nothing is budgeted for when it does not.
- **Order:** D (Fleet, 19 days) cannot be integration-tested until B's delegation and C's second
  broker caller exist, so the real critical path is max(B, C) + D's integration + G. Stream E is in
  none of the four streams (`05-design.md:906`, `:911`).
- **Fix:** re-plan TM1 at about 65 to 70 focused days (7 to 8 weeks on four streams). Put C1, C3 and
  I4 first, as a two-week **TM1-α** spike: one Orchestrator and one worker per person, two people,
  and a scripted conflict, on the real cluster.

## Minor

1. **The ask list is 49 rules, not 63** (`05-design.md:377`; `~/.claude/remote-settings.json`,
   read 2026-09-25). And being off an `ask` list is not the same as being allowed (C3).
2. **§11.2 omits two phase 8 changes the `panacea` channel needs.** `sandboxd` serves only
   `/attach` and `/healthz` (`sandboxd/internal/server/server.go:85-86`), and the broker relays
   only attach (task 18, `phase-8-own-environment.md:585`). The control stream needs a new
   `sandboxd` route and a second broker relay kind. §11.4's 3 days half-counts them, and
   `05-design.md:355-359` reads as if nothing changes.
3. **The Orchestrator class re-scopes phase 8 task 14, which is already built** (`09ae70a` on the
   `env` branch; `05-design.md:827`). Name it as a reopened task, or as a new task 14b.
4. **Tier H needs the same base blob** (`05-design.md:606`). After any land, members' bases
   diverge, so in TM1 (no mirror, `05-design.md:915-917`) H mostly collapses to P. Say so in §8.2.
5. **A 7-day delegation outlives an Entra disable** (`05-design.md:137-138`). Removing a member
   revokes it (`:143-146`), but HR disabling the account does not remove team membership. Use 24
   hours, or a daily Graph `accountEnabled` check.
6. **P2 cites `ADR-H1:372` and `:434-435` as its source** (`05-design.md:82`), but §2.4 amends
   both. Label it "amended, §11.1", so a reader does not take the ADR as already satisfied.
7. **"Egress to the gateway only"** (`05-design.md:259-260`). The CLI makes non-inference calls
   too: update checks, telemetry, feature flags (**UNVERIFIED** which ones at 2.1.27x). Set
   `CLAUDE_CODE_DISABLE_NONESSENTIAL_TRAFFIC=1` in the image, or they fail noisily.
8. **A directive typed into a worker's PTY** (`05-design.md:310-312`) arrives as a user turn, with
   the human's authority. Prefix every typed directive with a fixed `[your Orchestrator]` tag, so
   the transcript and the owner can tell whose words they were.

## The two things most likely to fail in the first build

1. **The Orchestrator does not run on phase 8's sandbox as designed.** It has no process model for
   per-turn `claude -p` (C1), no transcript home (C1), and it takes a worker slot on a machine
   node that has about three (C2). **Prevented by the design? No.** §11.2 and §11.4 cost the class
   and the loop without meeting `sandboxd`'s one-command contract or P8-D6's affinity. The first
   two-person run would hit `machine_full`, or a loop with nowhere to exec.
2. **The awareness channel is not called, not permitted, or not truthful.** The `panacea` verb has
   no allow rule in any policy (C3). The first-edit check rests on model obedience, with C6
   unmeasured (I2). The "observed" facts come from a process the agent owns (I1). **Prevented by
   the design? Partly.** The observed-edit `hold` lease (`05-design.md:470-472`) and the reviewed
   land (P7) catch a collision late, but only once I1 is fixed. Nothing catches a denied
   `panacea` call in a headless Orchestrator: it just does nothing.

## Does `05` implement OD-14 to OD-17, not the old per-user fleet?

| OD | Verdict | Evidence |
|---|---|---|
| OD-14 central | **Yes, in shape.** One Fleet, Kata sandboxes, the desktop as client (`05-design.md:43-45`, `:94-110`). **Residue:** per-user machines and per-user git credentials remain (`:102`, `:186-191`); that is right, but C2 and I3 are the single-user assumptions they carry | `05` §2 |
| OD-15 agent-first, calm humans | **Yes.** Assembled lines, observed files, no teammate terminal (`05-design.md:650-689`). **At risk** from I8: deadline escalations flood two Desks | `05` §9 |
| OD-16 orchestrators only | **Yes.** Workers cannot cross users (`05-design.md:294`), resolutions move only one's own workers (`:462-463`), inform vs decide (`:491-513`). **Replaced correctly:** "never pool" becomes attribution plus quotas, per OD-19 (`:174-177`) | `05` §4, §6 |
| OD-17 buildable next, re-scopes named | **Partly.** §11 names changes per plan, but misses the `sandboxd`/broker control relay (Minor 2), the reopened task 14 (Minor 3), the Hub's `azp`/RLS/feed changes (I4) and the desktop transport (I5). TM0 is further off than "the current wave" implies (I11) | `05` §11-12 |

**What holds up:** the placement rule (§3.2); the typed protocol and resolutions (§6.2); the taint
rule enforced on verbs rather than prompts (§7.2); the `panacea` binary as the harness-neutral
channel (§5.2); the lane mirror, so no WIP is pushed to the customer's remote (§8.2); the
CLAUDE.md pointer being fixed text, so `03`'s re-read question does not matter (`03-techniques.md:113-125`).
