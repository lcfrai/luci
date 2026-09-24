# 00 — Team mode, on one page

**Revision 3, 2026-09-25, after both critiques** (`06` feasibility, `07` security). Every Critical
and Important finding is fixed in `05-design.md` or rebutted there; `05` §15 maps each one.
**Nothing is built.** Binding: OD-14 to OD-20 (`../2026-09-23-owner-decisions.md:114-141`).

## The four capabilities

| | Capability | How | `05` |
|---|---|---|---|
| **C1** | share a project with a team | the Hub's planned teams, invites and Share, unchanged; Share moves the project into Fleet | §3.1, §10.1 |
| **C2** | see each member's work and agents | a calm **team strip**: who, one assembled line each, the files touched. No teammate terminal | §9 |
| **C3** | run my own agents on the same repo | from my own desktop, in the cloud: my clone and lane, my git credential for writes, my gateway quota | §10.2 |
| **C4** | agents understand each other | a **task card** per agent (task, brief section, status, observed files, leases), read through a `panacea` command | §5 |

## The design, in plain words

1. **All team agents run centrally, in one managed service (Fleet), in Kata sandboxes on Azure.**
   The desktop is the portal: the cockpit plus one thin local process that holds your sign-in.
   **No user needs Docker** (§10.3).
2. **No personal subscriptions.** Every agent uses the org's zero-data-retention gateway: Global
   (Anthropic API) by default, AU (Bedrock Sydney) by project. Tokens last an hour, are bound to
   their own sandbox (mechanism **unverified**, probed in phase 8 task 32), and bill to their
   owner, team and project (§2.5).
3. **Each person has one Orchestrator per project.** It runs their workers, writes no code, and is
   the only thing that talks to other people's Orchestrators, through typed messages (§4, §6).
4. **Agents see facts, not each other's words.** Files touched are observed by the platform from
   outside the sandbox. Workers never read a teammate's free text; Orchestrators read it as data.
   When Fleet, not you, started a turn, your Orchestrator may only pause, narrow or sequence your
   workers; anything else asks you (§5.1, §7.2).
5. **You hear only from your own side:** *inform* or *decide*, under rules you set. Fleet itself
   tells both people about every conflict settled without them. Every instruction to an agent is
   recorded, with who caused it (§6.5, §3.3).

## Delivery, with rough effort (§12)

| Slice | What | Effort |
|---|---|---|
| **TM1-α** | the three untested parts, thin, on the real cluster: the Orchestrator runner, the product's own Claude Code policy file, Fleet as a Hub caller; one scripted conflict | 2 weeks (the first slice of TM1) |
| **TM1** | two people, one repo, agents that know each other; done when an overlap is caught before code, settled or escalated, hostile peer text produces zero unapproved free-text directives, and a killed Orchestrator loses nothing | ≈ 95 focused days (was 53.5), about 7 to 8 weeks on five streams |
| **TM2** | one document: shared lane mirror (isolated), exact merge forecasts, one team land queue, CLI bump job, quota views | ≈ 16 days, about 3.5 weeks |
| **TM3** | opt-in: live Orchestrator, watch view, speculative builds, MCP front | later |

TM0, the current wave, is further off than it reads: phase 3 has no lanes code yet, and phase 8's
broker, `machined` and first apply are unbuilt (§12).

## What it changes in the plans in flight (§11)

- **Hub (≈ 16.5 days):** new records, including `directive`, Orchestrator-turn and human-message
  records; an ADR-H1 **"delegated callers"** amendment; a role-to-verb table; leaver checks.
- **Phase 8 (≈ 18 days):** task 14b (a small, diskless Orchestrator class); a `/control` route and
  relay; per-class policy files; a private gateway path; the mirror's own Kata pod; Australia East
  for everything.
- **Phase 3 (≈ 8 days):** observed activity from `machined`; land review lists conflicts and needs
  a second approver for agent-config files; a read-only fetch credential.
- **Phase 6** becomes the inference gateway (≈ 8.5 days). **Phases 4, 5, 7:** small additions.

## Owner questions, with recommendations (§13.1)

**Decide now (they block TM1-α):**

1. **Q0 Azure or AWS?** Stay on **Azure**. AWS now runs nested KVM on ordinary instances (AWS
   notice read 2026-09-25; Kata itself unconfirmed), but Kata there is self-managed, where AKS
   manages it; AU inference reaches Bedrock Sydney through the gateway either way.
2. **Q1 Which gateway?** A one-day spike each on LiteLLM and Anthropic's Claude apps gateway.
3. **Q11 May the cloud fetch the repo while nobody is signed in?** Yes, read-only, per project.
4. **Q12 Sponsor our Claude Code policy file to KPMG InfoSec?** Yes: cloud sessions do not get
   KPMG's server-managed policy, so this file governs them.

**Decide before TM1 proper:** Q2 require a data classification at Share (yes); Q3 org ceilings,
team budgets, non-zero defaults (yes); Q4 accept the escalation defaults, loosening never (yes);
Q5 keep opening prompts private (yes).
