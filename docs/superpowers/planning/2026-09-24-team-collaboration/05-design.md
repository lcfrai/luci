# 05 — Design: team mode, centrally managed (OD-14 to OD-17)

**Status:** design, 2026-09-24, **rewritten after OD-14 to OD-17**. The earlier draft of this file
(commits `28a65d1` to `7b094c5`) assumed each teammate's agents ran on that teammate's laptop and
that only facts met in the Hub. OD-14 reverses that: team agents run centrally. That draft is
superseded wholesale. What survives from it and from `01`–`04` (the fences, the conflict tiers, the
Hub's team model) is carried in here and marked. **Nothing here is built.** Repo claims are
`path:line`. `01`–`04` are the research files in this folder, cited by section. Line numbers into the
`hub`, `phase-3` and `env` worktrees may drift, because those trees are being edited live.
**UNVERIFIED** marks every claim not checked against a source or a run. No web source was re-read
for this file; external claims carry the read dates given in `02`–`04`.

**The ask** (owner, verbatim): *"share a project with my team, and everyone can not only see the
work I am doing on it, the agents I have running, and what their focus is, but spin up their own
agents in the same repo and the agents understand what each other are doing and what they are
working on."*

**The owner's direction** (`2026-09-23-owner-decisions.md:116-121`): *"Aren't we managing all of the
agents centrally, rather than them being on each machine? I am thinking like a Word Document on
Sharepoint being edited by multiple people, where each agent can see other agents doing work,
understand what the agent is currently working on and the task they have and the reason they are
updating that document, they are able to communicate and understand each other."*

| Key | Capability | Before OD-14 (`01` §2) | In this design |
|---|---|---|---|
| **C1** | share a project with a team | planned in the Hub, not built | the Hub plan, unchanged, plus a team project hosted by Fleet (§2, §3) |
| **C2** | everyone sees each member's work and each agent's focus | absent by design (`ADR-H1:88`) | agents see every task card; humans see a calm team strip (§5, §9) |
| **C3** | each member runs their own agents on the same repo | per machine; fleets never met | every member's agents run in one managed Fleet, each on its owner's subscription (§4, §10) |
| **C4** | agents understand each other | absent; phase 3 cut claims (A9) | task cards, leases and an orchestrator protocol (§5, §6) |

## 0. The design in ten lines

1. **One managed control plane per org, called Fleet,** runs every team agent as a managed session in
   a Kata sandbox in our environment. The desktop becomes a client of Fleet. Laptop Docker stays as
   the solo, offline and development mode (OD-14).
2. **Every user has one Orchestrator per team project.** It is an unmodified Claude Code session on
   that user's own subscription, in its own sandbox. It plans the work and spawns and directs only
   that user's **workers** (OD-16).
3. **Every agent has a task card**: its task; its reason, which is a link to the brief section the
   task serves (the provenance thread); its status; the files it is touching, as observed by the
   platform; and the leases it holds. Every agent on the project can read every card (OD-15).
4. **Agents reach the platform through one channel that survives the KPMG policy:** a `panacea`
   command inside each sandbox, answered by `sandboxd` from Fleet. It needs no MCP and no hooks. An
   MCP front can be added later for a customer whose admin allowlists it (§5.2).
5. **Orchestrators are the only cross-user speakers.** They talk through a typed protocol on Fleet:
   announce, ask, answer, propose, accept, reject, yield. Fleet detects conflicts itself and opens a
   **conflict thread** between the orchestrators involved (§6).
6. **Each Orchestrator escalates only to its own human,** either to **inform** (a line in the
   RETURN handover) or to **ask for a decision** (an ADJUDICATE item), under a policy that human
   sets in advance. No human steers another human's agent (OD-16, §6.5).
7. **Coordination text is untrusted input.** Facts are observed by the platform. Agent text is typed,
   bounded, attributed and quoted. A teammate's free text reaches only orchestrators, and an
   orchestrator that has read it may give its workers only typed directives without asking its
   human. The sandbox and the verb set enforce this, not the prompt (§7).
8. **Conflicts are predicted centrally.** Paths and hunks come from observed activity. Exact
   `merge-tree` forecasts run over a per-project **lane mirror** that Fleet holds, so no unfinished
   lane has to be pushed to the customer's remote. Leases are soft; the reviewed land is the
   backstop (§8).
9. **Humans get a calm team strip:** who is on the repo, one line each, and the files touched.
   Conflicts appear only when an Orchestrator escalates. There is no teammate terminal by default
   (OD-15, §9).
10. **The minimum lovable slice (TM1)** is about 30 focused days after the current wave. It
    re-scopes the Hub, phase 8 and phase 3 in the exact ways listed in §11 (OD-17, §12).
