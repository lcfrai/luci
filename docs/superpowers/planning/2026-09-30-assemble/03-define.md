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

## 5. Recommendation

_Pending._

## 6. What we still build (glue)

_Pending._

## 7. Calls only the owner can make

_Pending._

## 8. What this study could not verify

_Pending._

## 9. Sources

_Pending._
