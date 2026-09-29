# 03 · Define: the brief, the interview, the spec — adopt, do not build

**Date:** 2026-09-30 · **Status:** in progress (skeleton; sections land one commit at a time)
**Question:** which existing product or project can stand in for Define (phase 4, Discover, and the
OD-36 glossary and decision record) so that we build glue, not the capability?

## 1. Answer

_Pending: filled once the candidates below are assessed._

## 2. What Define has to do (the bar every candidate is scored against)

Seven needs, from the phase 4 plan (`.claude/worktrees/phase-4/docs/superpowers/plans/2026-09-23-phase-4-discover.md`
§1-§2), OD-3, OD-36 and the hard constraints. **N1-N3 are the capability a product can supply. N4-N5 are
the provenance thread, which is ours by definition. N6-N7 are the conditions for shipping it.**

| # | Need | Where it comes from |
|---|---|---|
| N1 | **Interview.** Turn an idea into a brief by asking one question at a time, with a recommended answer, from nothing (greenfield) or drafted from an existing repository (brownfield). The model helps and never gates | plan §1; tasks 03, 04, 13, 18, 21 |
| N2 | **Spec with acceptance criteria**, each with a stable id, in a form a coding agent reads and phase 3 can check before merge (the definition of done) | plan §1; tasks 03, 06, 17 |
| N3 | **Glossary and decision record beside the brief**, each its own record that references a brief revision; a decision is recorded only if it is hard to reverse, surprising, and a real trade-off | OD-36; `2026-09-28-discovery-skills/11-design-r2.md` §2-§3 |
| N4 | **Shared and revisioned server-side** (the Hub), so a teammate sees the same brief; a key such as `brief:bf-…@n` names immutable text and resolves to one `provenance.Ref` on every surface: pane chip, merge list, release | OD-3; plan §2 rules 1-3; tasks 06-08, 11-12 |
| N5 | **Inherited, not advised.** The brief (plus terms and decisions under OD-36) is written into the worktree's `CLAUDE.md` before the agent process starts, and the session is stamped with the key | plan §1; tasks 09, 10, 14 |
| N6 | **Enterprise conditions.** Azure (OD-18); Entra through Keycloak (OD-30); model calls only through the LiteLLM gateway, zero retention, AU path on Bedrock Sydney (OD-19, OD-24); permissive licence for anything deployed into a tenancy; nothing that makes an untrusted agent more trusted | owner decisions file |
| N7 | **Usability.** One obvious start, no wall of configuration; the whole Define step fits inside the cockpit the user already has | `CLAUDE.md` "Why this exists" |

**What is already built and stays.** Nine of phase 4's tasks: the Go packages `modelclient`, `discover`,
`claudemd`, `provenance`, `discoverhub` and `contextwrite` (about 17,000 lines with tests, in the phase-4
worktree) and the `api` Materialiser seam (task 10). They cover N4 and N5, the provenance thread.

**What is not built, and is the target of this study.** Tasks 11-23 (routes, repository analysis, wiring,
all seven desktop tasks, contracts, end-to-end): **32.5 h** in the plan's own table. OD-36's design round 2:
**45.25 h** in phase 4 (nine new tasks 24-32, seven amended) plus **31 h** in the Hub, **about 76.75 h**
before fix rounds (`11-design-r2.md` §7.5). **Together about 109 h before fix rounds, about 140 h with the
ledger's one fix round per task.** Most of it is N1-N3 and the screens that show them.

## 3. Candidates at a glance

**Glasses first.** The offline catalogue and its MCP server (both dated 2026-09-29) hold **no entry** for
Spec Kit, Kiro, BMAD, Taskmaster, Tessl, Traycer, ChatPRD, OpenSpec or GSD, and zero matches for *spec*,
*prd*, *requirements*, *glossary* or *decision record*. The nearest entries are off target and restricted:
Outline (knowledge base, **BSL-1.1**) and Plane (issue tracker, **AGPL-3.0**). Everything below came from
the web, checked on the upstream page on 2026-09-30.

| Candidate | What it is | Licence | Deploys as | Maturity | Verdict |
|---|---|---|---|---|---|
| **GitHub Spec Kit** | Spec-driven workflow as agent skills: idea assessment, `specify`, `clarify`, `plan`, `tasks`, `converge` | **MIT** | Python CLI + templates, installed into the sandbox; runs inside the Claude Code session we already host; offline wheel install | 139.4k stars; v1.0.13 on 2026-09-29, a release every 1-3 days; GitHub's own org | **Adopt** |
| **mattpocock/skills** (aihero) | `grill-me`, `grill-with-docs` (writes `GLOSSARY.md` and ADRs), `domain-modeling`, `to-spec` | **MIT** | Skill files in the sandbox | 272.1k stars; assessed skill by skill on 2026-09-28 | **Integrate** (inside our Spec Kit preset) |
| **OpenSpec** (Fission AI) | Specs as source of truth plus change proposals with ADDED/MODIFIED/REMOVED deltas | **MIT** | npm CLI + slash commands in the agent | 70.7k stars; v1.13.2, September 2026; small start-up | **Reference** (runner-up, brownfield) |
| **BMAD-METHOD** | A team of agent personas: analyst brief, PRD, UX, architecture, epics, stories | **MIT**; "BMad" is a trademark of BMad Code, LLC | Skills or Claude Code plugin in the agent | 53.6k stars; v6.12.0, September 2026 | **Reference** |
| **adrkit** | Typed, validated ADRs; Spec Kit extension (`context`, `check`, `draft`) and an MCP server | **Apache-2.0** (schema CC0) | npm packages, Node 22+ | 18 stars; v0.15.0; pinned to Spec Kit <1.1 | **Reference** (optional add-on later) |
| **speckit-grill-me** (yoshi1220) | `grill-me` ported into Spec Kit: rounds by frontier, then `docs/domain_knowledge/` | **MIT** | Spec Kit extension | 2 stars | **Reference** (we build the same from the MIT original) |
| **GSD** (`open-gsd/gsd-core`) | New-project interview, research agents, requirements with ids, roadmap, phase execution | **MIT** | npm installer into the agent | 10.0k stars (old repo 64.4k, archived 2026-06-26); v1.7.0 | **Reference** |
| **Spec Kitty** | Spec Kit derivative adding worktrees, a kanban dashboard and multi-agent missions | **MIT** | CLI + local dashboard | 1.7k stars; 4.0.0rc2 | **Reject** (duplicates our cockpit) |
| **Taskmaster** (`claude-task-master`) | PRD parsed into dependent tasks, complexity report, MCP | **MIT + Commons Clause: restricted** | npm CLI / MCP | 28.1k stars | **Reject** (licence) |
| **AWS Kiro** | Agentic IDE, CLI and web with requirements/design/tasks specs (EARS) | Proprietary | AWS SaaS; $0-$200 per user per month; Enterprise SSO via IAM Identity Center | GA November 2025 | **Reject** as a part; **reference** the format |
| **Tessl** | Now an "agent enablement platform": skills registry, governance loops | Proprietary | SaaS; free tier 1,000 credits a month | Pivoted away from specs | **Reject** |
| **Traycer** | Desktop planning and task-tracking app that hands work to coding agents | Proprietary | Desktop app + cloud sync, $16 per user per month | SOC 2 Type 2 | **Reject** (a competing cockpit) |
| **ChatPRD** | PRD writer for product managers | Proprietary | SaaS only | The KPMG proxy blocks it (302 to KPMG's acceptable-use policy) | **Reject** |
| **BrainGrid** | SaaS planner that writes a spec with acceptance criteria and verifies the build | Proprietary | SaaS + MCP; price not published | Unverified | **Reject** |
| **Confluence + Rovo** | Wiki with AI drafting; AU data residency, Rovo included | Proprietary | Atlassian Cloud; SSO through Atlassian Guard | Mature | **Reject** (a *buy* that moves the brief out of the Hub) |

## 4. Candidate assessments

### 4.1 GitHub Spec Kit — adopt

- **Does.** Turns an idea into a spec through agent skills. `speckit-assess-*` (intake → research → define
  → shape → decide) takes a raw idea to a go, clarify or stop decision under `.specify/assessments/`.
  `speckit-specify` writes `specs/<n>-<name>/spec.md` plus a quality checklist, with at most three
  `[NEEDS CLARIFICATION]` markers. `speckit-clarify` asks **up to five questions, exactly one at a time,
  each with 2-5 options and a recommended one first**, and writes every answer into the spec under
  `## Clarifications` / `### Session <date>`; its scan includes "Terminology & Consistency". Then `plan`,
  `tasks`, `implement`, and `converge`, which checks the build against the spec until "Converged".
- **Spec format.** User stories with priorities, **Given/When/Then** acceptance scenarios, edge cases,
  functional requirements `FR-001…`, key entities, success criteria, assumptions (`templates/spec-template.md`).
- **Licence.** MIT. **Deploys.** No server. A Python CLI (`uv tool install specify-cli`, or an offline wheel
  bundle for air-gapped hosts) writes templates and skills into the repository; for Claude Code the
  integration key is `claude` and it installs under `.claude/skills`. `specify init` uses bundled assets
  and needs no network. **Presets** override templates, commands *and terminology*, stack by priority, and
  install from a local directory (`specify preset add --dev <path>`) or a URL.
- **Enterprise fit.** Nothing to sign in to and nothing to host: identity, audit and residency are those of
  the session it runs in, which are already ours (Keycloak → Entra, the Hub, the gateway's AU path). It
  adds no model endpoint: every model call is the agent's own, through LiteLLM. Upstream names no
  telemetry (not verified by reading code). The community catalogue (130+ extensions, 70+ authors, "not
  an audit or endorsement") must be switched off or self-hosted.
- **Maturity.** 139.4k stars, GitHub's own organisation, v1.0.13 on 2026-09-29, releases every 1-3 days.
- **Gaps.** N1 and N2 are covered. **N3 partly:** no glossary and no decision record of its own. **N4 not
  at all:** files in git only, no sharing, no revision numbers, no provenance keys, which is exactly the
  part we have built. **N7 risk:** a menu of slash commands is the wall of configuration our bar forbids,
  so the cockpit must hide it behind one start. **Churn:** commands were renamed in the 1.0 line, so pin a
  version and canary the sections we parse.
- **Effort.** About 8 stream-days with the glue in section 6. **Verdict: adopt.**

### 4.2 mattpocock/skills (aihero) — integrate, inside the Spec Kit preset

- **Does.** `grill-me` interviews "until every branch of the design tree is resolved"; `grill-with-docs`
  does the same while writing **`GLOSSARY.md` and ADRs**; `domain-modeling` sharpens the terms.
- **Licence.** MIT, 272.1k stars. **Deploys, fit.** Skill files; no server, no calls of their own.
- **Gaps.** No spec template or acceptance-criteria structure (Spec Kit supplies both). The 2026-09-28
  study (`2026-09-28-discovery-skills/00-summary.md` §1) already judged these skill by skill; this study
  adds only where they sit: inside our preset, so glossary and decisions land as **files**, not as a
  transcript that OD-8 forbids us to depend on.
- **Effort.** 1 day, inside G1. **Verdict: integrate.**

### 4.3 OpenSpec — reference, the runner-up

- **Does.** `openspec/specs/` is the source of truth; `openspec/changes/` holds a proposal, design and tasks
  with ADDED/MODIFIED/REMOVED deltas, merged into the specs on archive. `/opsx:explore` is a thinking
  partner, then `propose`, `apply`, `archive`. Works with 30+ tools.
- **Licence.** MIT. **Deploys, fit.** An npm CLI plus commands in the agent; same fit as Spec Kit.
- **Maturity.** 70.7k stars, v1.13.2 in September 2026, backed by Fission AI, a small company.
- **Gaps.** The best brownfield model of the set, but its interview is an open-ended explore, not
  Spec Kit's structured one-question-with-a-recommendation; no glossary, no decision record; smaller backer.
- **Effort.** About 9 stream-days (the same glue plus an interview skill of our own). **Verdict: reference.**
  Revisit if per-feature spec folders pile up in long-lived repositories.

### 4.4 BMAD-METHOD — reference

- **Does.** A set of agent personas across the lifecycle: brainstorming and a product brief, a PRD, UX,
  architecture, epics and stories. v6 installs as skills or a Claude Code plugin.
- **Licence.** MIT for the code; **"BMad" and "BMAD-METHOD" are trademarks of BMad Code, LLC**, so none of
  our surfaces may carry the name. **Maturity.** 53.6k stars, v6.12.0 in September 2026.
- **Gaps.** The richest from-nothing elicitation of the set, and the most ceremony: one comparison counts
  21 agents. It fails N7, and its UX, architecture and story steps cross into Design and the work layer,
  which breaks our phase boundaries.
- **Effort.** About 12 stream-days, mostly trimming. **Verdict: reference.** Borrow its elicitation
  techniques into the preset if clarify plus grill proves too thin.

### 4.5 AWS Kiro — reject as a part, reference the format

- **Does.** An agentic IDE, CLI and web app whose specs are `requirements.md` (user stories with acceptance
  criteria in EARS notation), `design.md` and `tasks.md`, as feature specs or bugfix specs.
- **Licence, price.** Proprietary. Free, Pro $20, Pro+ $40, Pro Max $100, Power $200 per user per month;
  Enterprise adds SAML/SCIM through **AWS IAM Identity Center**, not our Keycloak.
- **Residency: fails OD-24.** Individual data is stored in US East; enterprise data in the profile's
  region; inference regions listed are US, Europe and GovCloud only, **no Australia**.
- **Gaps.** It is its own agent and editor, not an unmodified CLI in our sandbox, on AWS not Azure: adopting
  it replaces the cockpit, not Define. **Verdict: reject.** Its EARS sentence patterns are available to
  Spec Kit as a community extension (`spec-kit-ears`) if acceptance criteria need a stricter grammar.

### 4.6 Taskmaster — reject on licence

- **Licence: MIT with Commons Clause, restricted.** The clause removes the right to sell the software or a
  hosted service whose value comes substantially from it; a we-host SaaS (OD-5) that runs it for customers
  is that service. A Commons Clause licence is not open source, so it fails our permissive rule outright.
- **Gaps.** It only turns a PRD into dependent tasks, which is the work layer's step, not Define's.
  Spec Kit's `tasks` and its bundled GitHub `taskstoissues` extension cover the same step under MIT.

### 4.7 GSD (`open-gsd/gsd-core`) — reference

- **Does.** `/gsd-new-project` interviews, runs research subagents, writes requirements with ids and a
  roadmap, and keeps `STATE.md` and `CONTEXT.md`. MIT, 10.0k stars, v1.7.0.
- **Gaps.** A whole execution engine (phases, roadmap, state) that overlaps our Orchestrator and work layer;
  the project moved repositories on 2026-06-26, a continuity risk. **Verdict: reference.**

### 4.8 The SaaS products — reject

| Product | Why not |
|---|---|
| **Tessl** | Has pivoted to a skills registry and governance loops; SaaS; audit logs still "rolling out". No longer a Define product |
| **Traycer** | A desktop planner with cloud sync ($16 per user per month, SOC 2 Type 2) that hands work to agents: a second cockpit, with no residency statement |
| **ChatPRD** | SaaS only, for product managers. **The KPMG proxy blocks `chatprd.ai`** (302 to KPMG's acceptable-use policy page), so it cannot even be evaluated from here |
| **BrainGrid** | SaaS planner that verifies a build against its acceptance criteria. The idea is right and is already ours: phase 3's criterion evaluator. No published price |
| **Confluence + Rovo** | The one credible *buy*: AU data residency (Rovo included), SSO through Atlassian Guard, per-user licences. It moves the brief into a wiki outside the Hub and outside the customer's cloud, so the provenance key has nothing immutable to name. It breaks the spine |

### 4.9 Small parts — reference

- **adrkit** (Apache-2.0, 18 stars, v0.15.0): typed ADR front matter with an `affects` field, a validator
  and an MCP server, plus Spec Kit commands to pull governing decisions into a plan and check it. The best
  format for OD-36's decision record, but young and pinned to Spec Kit below 1.1. Add later, not now.
- **speckit-grill-me** (MIT, 2 stars): shows the port of `grill-me` into Spec Kit works. Too small to depend
  on; we build the same from the MIT original (G1).
- **Spec Kitty** (MIT, 1.7k stars, 4.0.0rc2): worktrees, a kanban dashboard, multi-agent missions. It
  duplicates phases 2 and 3. **Reject.**
- **Spec Kit community integrations** for Azure DevOps (MIT, 15 stars, two commits), Jira, Linear,
  Confluence and a requirement-to-test matrix (`spec-kit-trace`) exist. None is needed for Define; phase 3
  owns the git hosts. Read their source before any use.

## 5. Recommendation

**Adopt GitHub Spec Kit as the Define engine, run as a Define session in the cockpit we already have,
with one Panacea preset that carries the brief's five sections and the aihero grilling and glossary
skills. Keep the built provenance thread (Hub brief revisions, keys, the Materialiser) and point it at
the spec Spec Kit writes.**

**What the person does.**

1. Presses **Define** on a project or branch and types the idea. The cockpit starts a Claude Code session
   on that worktree with an opening prompt such as `/speckit-specify <idea>` (from nothing:
   `/speckit-assess-intake` first). The phase 2 harness already takes an opening prompt
   (`control-plane/internal/harness/argv.go`, `OpeningPrompt`).
2. Answers the agent in the pane: one question at a time with a recommended answer (`clarify`), and
   grilling for words and decisions (`grill-with-docs`).
3. Reads the result, rendered: `specs/<n>-<name>/spec.md` with the five sections, `GLOSSARY.md`,
   `docs/adr/*.md`.
4. Presses **Publish brief**. The control plane writes a brief revision to the Hub through the built
   `discoverhub`, with terms and decisions as records beside it (OD-36).
5. Every later session on that branch inherits it through the built Materialiser and shows the chip.

**Why this one.**

- It covers N1-N3 with the best-maintained permissive option: MIT, GitHub's own organisation, 139.4k stars,
  a release yesterday.
- It adds no infrastructure: no server, no account, no new model endpoint. It runs inside the sandbox we
  already run, through the gateway, installed offline. N6 holds by construction.
- It writes the spec in the form coding agents already read (about 60 integrations use this format), so
  Develop sessions read `spec.md` directly as well as inheriting the brief.
- It keeps the spine: the Hub revision and its key stay ours, and they are the part already built.
- Presets override templates, commands and terminology, so we fit it to our brief without forking it.

**Runner-up: OpenSpec** (MIT), if brownfield change deltas turn out to matter more than the interview.
**Not a buy:** no commercial product passes both AU residency and "brief in our Hub".

**What it replaces**, in the plans' own hours before fix rounds:

| Replaced | Tasks | Hours |
|---|---|---|
| Phase 4, planned and unbuilt | 04 model follow-up, 13 repository analysis, 18 lens asking, 19 lens states, 21 import a repository | 12 |
| Phase 4, OD-36 round 2 (`11-design-r2.md` §7.3) | 24 prototype, 25 term and decision model, 27 Hub reads and writes, 28 `RenderInherited`, 29 three files, 30 suggestion round, 31 lens areas, 32 relay routes | 37.5 |
| Hub, OD-36 share | five new tasks and seven amendments, shrunk to one record kind (G4, about 8 h) | 31 → 8 |
| **Total** | **13 phase 4 tasks, most of the Hub's OD-36 share** | **about 72 of the about 109 h left** |

**Kept** (unbuilt, still needed for the thread): 11 (trimmed to publish plus reads), 12, 14, 15, 16
(trimmed), 17, 20, 22, 23 and 26, about 23.5 h. **Built and unchanged:** the nine tasks in section 2.
**OD-36 stands:** terms and decisions stay beside the brief. Only round 2's own mechanics (rounds,
suggestion states, bespoke lens states) are not built.

**Integration: about 8 stream-days** (section 6), **about 5 calendar days on two parallel streams**:
control plane (G1-G4 with tasks 12, 14, 20, 22, 26) and desktop (G5 with tasks 15-17).

## 6. What we still build (glue)

_Pending._

## 7. Calls only the owner can make

_Pending._

## 8. What this study could not verify

_Pending._

## 9. Sources

_Pending._
