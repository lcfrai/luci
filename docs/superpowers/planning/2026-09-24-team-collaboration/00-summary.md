# 00 — Team mode, on one page

**Read first:** `05-design.md` §0 (the design in ten lines), then §12 (what gets built, in what
order) and §13 (the ten owner questions). Research behind it: `01` inventory, `02` prior art, `03`
techniques, `04` enterprise constraints. **Nothing here is built.** Binding owner decisions:
OD-14 to OD-20 (`../2026-09-23-owner-decisions.md:114-141`).

## The ask, and the answer

The owner wants to share a project with a team, see each member's work and agents and their focus,
let each member spin up their own agents on the same repo, and have every agent understand what the
others are doing, *"like a Word Document on Sharepoint being edited by multiple people"*.

**The answer: team agents run centrally, in one managed service called Fleet, on AKS with Kata
sandboxes. The desktop is the portal, and no user needs Docker.** Every agent reaches the model
through one org-level, zero-data-retention gateway, on a token scoped to its session and attributed
to its owner, team and project. Global (the Anthropic API) is the default; AU (Bedrock Sydney) is
forced by a project's classification or chosen by the user. No personal subscription is used.

## The four capabilities

| | Capability | How | Where in `05` |
|---|---|---|---|
| **C1** | share a project | the Hub's planned teams, invites and Share popover, unchanged; Share now also moves the project into Fleet | §3.1, §10.1 |
| **C2** | see teammates' work | a calm **team strip**: who, one assembled line each, the files touched, one heartbeat dot. No teammate terminal by default | §9 |
| **C3** | run my own agents on the same repo | from my own desktop: my own cloud machine, my own clone and lane, my own git credential, my gateway quota | §10.2 |
| **C4** | agents understand each other | every agent has a **task card** (task, reason linked to the brief, status, observed files, leases), read through a `panacea` command in every sandbox | §5 |

## The model

1. **Every user has one Orchestrator per team project.** It plans, spawns and directs only its own
   user's workers. It writes no code and holds no git credential (§4).
2. **Orchestrators are the only cross-user speakers.** They use a typed protocol: ask, answer,
   inform, propose, accept, reject. Only typed resolutions have effects: split, sequence, yield,
   build-on, proceed-both (§6).
3. **Fleet detects conflicts itself,** from observed paths and hunks, and opens a thread between
   the two Orchestrators before a worker is spawned where it can (§8).
4. **Each Orchestrator escalates only to its own human:** *inform* (a line in the handover) or
   *decide* (a Desk item with options and a deadline), under rules that human set in advance.
   No user steers another user's agent (§6.5).
5. **Coordination text is untrusted input.** Facts are observed by the platform; agent text is
   typed, bounded, attributed and quoted, and never enters `CLAUDE.md`. An Orchestrator that has
   read peer text needs its human's decision for anything wider than its task (§7).

## What it changes in the plans in flight (§11)

- **Hub:** new durable records (task, lease changes, conflict, thread, message, escalation, note,
  team policy, usage roll-up), Fleet as a second caller under a user-created delegation, and audit
  anchored in Sentinel. About 9.5 days.
- **Phase 8:** stays on Azure. A second broker caller, an `orchestrator` sandbox class, a gateway
  network allow, Fleet and gateway charts, a project disk, a CLI image-bump job. About 9.5 days.
- **Phase 3:** finished as planned; its forecast and queue become a library Fleet can run, and
  declared claims return as leases in Fleet. About 4 days.
- **Phase 6** becomes the inference gateway. **Phases 4, 5 and 7** get small additions.

## Delivery (§12)

- **TM1, two people and one repo:** about 53.5 focused days, roughly 5 working weeks on four
  parallel streams, after the current wave. Done when an overlap is caught before any code is
  written, the Orchestrators settle or escalate it, hostile peer text causes zero out-of-scope
  directives, usage is attributed correctly, and an AU-only project never calls Global.
- **TM2, one document:** the shared lane mirror, exact merge forecasts, one team land queue, the
  CLI bump job and quotas. About 14 days.
- **TM3:** opt-in extras (live Orchestrator, watch view, speculative builds, MCP front, memory).

## Decide before TM1 (§13.1)

1. **Q1:** which gateway. Recommendation: a one-day spike each on LiteLLM and Anthropic's Claude
   apps gateway.
2. **Q2:** a required classification click at Share. Recommendation: yes.
3. **Q3:** quota ceilings. Recommendation: org admin per person; team admin per project, inside it.
4. **Q4:** accept §6.5's Orchestrator authority defaults. Recommendation: yes.
5. **Q5:** keep opening prompts private. Recommendation: yes.

**Biggest unknown:** whether KPMG's managed policy reaches a sandbox that signs in with a gateway
token rather than a claude.ai account. The check is `/status` in OD-12's hands-on session (§14.1).
