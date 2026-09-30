# 03 · Define: the brief, the interview, the spec — adopt, do not build

**Date:** 2026-09-30 · **Status:** complete; revised for the owner's second direction (relaunch, section 0)
**Question:** which existing product or project can stand in for Define (phase 4, Discover, and the
OD-36 glossary and decision record) so that we build glue, not the capability?

## 0. Revised for the owner's second direction

The first run (stopped 09:47) stands at its core: **Spec Kit is still the Define engine.** Panacea's one
interview-to-spec step (Forge's) runs in Forge's own agent loop, outside our sandbox and without
brownfield, glossary or decisions; Coolify and Supabase have none. Five things changed, and why:

1. **The Define session is an Orca worktree.** Orca (MIT; the owner's cockpit pick, verified in
   `02-fleet-cockpit.md`) starts an agent in a new worktree with an opening prompt
   (`orca worktree create --name <n> --agent claude --prompt "<task>"`) and renders and edits the
   markdown the interview writes (its rich markdown editor, with annotations on rendered text). That
   absorbs most of G2 and G5: their glue falls from 2.5 to 1.75 days (section 4.11).
2. **Panacea supplies two rules our spec lacked** (read-only look at `Projects/Panacea`). Forge's
   `SPEC.md` (`services/forge-orchestrator/models/spec.go`) decides **access before interface** (who
   may read and change each entity; every route carries an auth decision) and **refuses criteria a
   browser cannot check** (3 to 8, each with a testable verb). Both go into the preset and into
   Publish's validator, as snippets with attribution (OD-13 relaxed). Forge itself is not adopted:
   its agent loop is not the unmodified CLI, and its files live in Forge's store, not in the worktree
   (section 4.10).
3. **The data class is set in Define, in Panacea's words.** OD-24 requires it on creation. Panacea's
   stage ladder already enforces a ceiling of `none < synthetic < protected`, read from the
   `panacea.io/data-classification` annotation (`services/panacea-api/handlers/deployment_stages.go`).
   The project and brief record it, and Deploy carries it to Panacea: new glue G7, 0.5 day. Panacea's
   risk questionnaire and assessment document are the next step, not the first (section 4.10).
4. **Supabase and Coolify change nothing here.** Coolify is a deploy tool with no Define capability.
   Supabase could back the Hub's records, but Define reaches the Hub only through the built
   `discoverhub` client, so where the Hub keeps them is not decided here (section 4.12).
5. **One new owner call** (section 7, call 4): which data classes a customer picks from at creation.

**Net:** still **about 8 stream-days**. Orca saves 0.75 day; the Panacea rules and the data class add
0.75 day. The brief gets stronger for regulated customers: access decided up front, criteria a
browser can test, and the data class that Deploy enforces.

## 1. Answer (revised for the owner's second direction)

**Adopt GitHub Spec Kit** (MIT, GitHub's own organisation, v1.0.13 on 2026-09-29) **as the Define engine,
run as an Orca worktree session**: Claude Code in our sandbox, started with Spec Kit's command as the
opening prompt. One Panacea preset carries the brief's five sections, **Panacea Forge's access table and
testable-criteria rule**, the project's **data class in Panacea's vocabulary**, and the aihero `grill-me`
and `grill-with-docs` skills (MIT) for the glossary and decisions. The built provenance thread stays (Hub
brief revisions, `brief:bf-…@n` keys, the Materialiser) and is pointed at the spec Spec Kit writes.

- **Integration: about 8 stream-days, about 5 calendar days on two parallel streams** (unchanged).
- **Replaces** 12 of phase 4's 22 unbuilt tasks (13, 18, 19, 21, and OD-36's 24, 25, 27-32) and most of the
  Hub's OD-36 share: **about 71 of the about 109 hours left**, before fix rounds.
- **Still ours:** 7.5 days of glue (section 6, G1-G7) plus ten kept thread tasks. The four desktop ones
  (15, 16, 17, 20) move into Orca's renderer if `02-fleet-cockpit.md` re-seats the cockpit there.
- **From Panacea:** Forge's spec rules as attributed snippets, and the data-class ceiling its Deploy
  already enforces. Not Forge's agent loop, and no Panacea service for Define: grep finds no brief,
  glossary or decision store in `services/panacea-api` or `panacea-mcp`.
- **No buy.** No commercial product passes both AU residency and "brief in our Hub": Kiro has no AU region,
  ChatPRD is blocked by the KPMG proxy, and Confluence moves the brief out of the Hub. Taskmaster's
  Commons Clause is a restricted licence.
- **Owner calls:** four, all customer-visible, none about money (section 7).

**First step:** G1, the vendored `specify-cli` and the `panacea` preset in `images/sandbox/Dockerfile`
(1.25 days).

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

**What is already built.** Tasks 01-10: each part file carries an as-built record (the orchestrator's
summary counts nine). They are `statepath.WriteAtomic` and the Go packages `modelclient`, `discover`,
`claudemd`, `provenance`, `discoverhub` and `contextwrite` (about 17,000 lines with tests, in the phase-4
worktree), plus the `api` Materialiser seam (task 10). They cover N4 and N5, the provenance thread, and
they stay. Only task 04's model follow-up and the stepper side of task 03's ops lose their caller in
the recommendation below. The content model stays as the brief's storage.

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
the web, checked on the upstream page on 2026-09-30. **The relaunch searched again:** still nothing for
*specification*, *requirement*, *backstage* or *techdocs*; the catalogue lists Supabase (Apache-2.0,
110.9k stars) as a whole product. The Panacea rows come from Panacea's source, read-only; the Orca row
from Orca's documentation.

| Candidate | What it is | Licence | Deploys as | Maturity | Verdict |
|---|---|---|---|---|---|
| **GitHub Spec Kit** | Spec-driven workflow as agent skills: idea assessment, `specify`, `clarify`, `plan`, `tasks`, `converge` | **MIT** | Python CLI + templates, installed into the sandbox; runs inside the Claude Code session we already host; offline wheel install | 139.4k stars; v1.0.13 on 2026-09-29, a release every 1-3 days; GitHub's own org | **Adopt** |
| **mattpocock/skills** (aihero) | `grill-me`, `grill-with-docs` (writes `GLOSSARY.md` and ADRs), `domain-modeling`, `to-spec` | **MIT** | Skill files in the sandbox | 272.1k stars; assessed skill by skill on 2026-09-28 | **Integrate** (inside our Spec Kit preset) |
| **Panacea Forge's spec** (`forge-orchestrator`) | A short discovery (purpose, data, access, routes, acceptance) and a validated `SPEC.md` that Forge's browser test grades against | KPMG's own; snippets allowed with attribution | Inside Panacea: Forge sessions API, Forge's own agent loop | Built and tested in Panacea (`spec_tool_test.go`) | **Integrate** (two rules as snippets, not the service) |
| **Panacea data class and risk** (`panacea-api`) | A stage ladder with a data-class ceiling; a per-project risk questionnaire and a generated assessment document | KPMG's own | Panacea API, Keycloak `panacea` realm | Built; the ceiling is an attestation until DLP enforces | **Integrate** (data class now, risk questionnaire next) |
| **Orca** (the cockpit pick) | A worktree plus an agent with an opening prompt; a rich markdown editor in the worktree | **MIT** | Electron desktop; `orca serve` | 81.6k stars, daily releases (`02-fleet-cockpit.md`) | **Integrate** (hosts the Define session) |
| **Backstage** (in Panacea) | Scaffolder and TechDocs are installed; no ADR plugin | **Apache-2.0** | Panacea's Backstage | CNCF project | **Reference** |
| **Supabase** | Postgres platform: REST, realtime, auth, storage | **Apache-2.0** | Self-host (Docker Compose) or Supabase Cloud, Sydney region | 110.9k stars | **Reference** (a Hub-store question) |
| **Coolify** | Self-hosted deploy platform | **Apache-2.0** | Docker on a VM | `06-deploy.md` §3.1 | **Reject** for Define (no Define capability) |
| **OpenSpec** (Fission AI) | Specs as source of truth plus change proposals with ADDED/MODIFIED/REMOVED deltas | **MIT** | npm CLI + slash commands in the agent | 70.7k stars; v1.13.2, September 2026; small start-up | **Reference** (runner-up, brownfield) |
| **BMAD-METHOD** | A team of agent personas: analyst brief, PRD, UX, architecture, epics, stories | **MIT**; "BMad" is a trademark of BMad Code, LLC | Skills or Claude Code plugin in the agent | 53.6k stars; v6.12.0, September 2026 | **Reference** |
| **adrkit** | Typed, validated ADRs; Spec Kit extension (`context`, `check`, `draft`) and an MCP server | **Apache-2.0** (schema CC0) | npm packages, Node 22+ | 18 stars; v0.15.0; pinned to Spec Kit <1.1 | **Reference** (optional add-on later) |
| **speckit-grill-me** (yoshi1220) | `grill-me` ported into Spec Kit: rounds by frontier, then `docs/domain_knowledge/` | **MIT** | Spec Kit extension | 2 stars | **Reference** (we build the same from the MIT original) |
| **GSD** (`open-gsd/gsd-core`) | New-project interview, research agents, requirements with ids, roadmap, phase execution | **MIT** | npm installer into the agent | 10.0k stars (old repo 64.4k, archived 2026-06-26); v1.7.0 | **Reference** |
| **Spec Kitty** | Spec Kit derivative adding worktrees, a kanban dashboard and multi-agent missions | **MIT** | CLI + local dashboard | 1.7k stars; 4.0.0rc2 | **Reject** (duplicates the cockpit, now Orca) |
| **Taskmaster** (`claude-task-master`) | PRD parsed into dependent tasks, complexity report, MCP | **MIT + Commons Clause: restricted** | npm CLI / MCP | 28.1k stars | **Reject** (licence) |
| **AWS Kiro** | Agentic IDE, CLI and web with requirements/design/tasks specs (EARS) | Proprietary | AWS SaaS; $0-$200 per user per month; Enterprise SSO via IAM Identity Center | GA November 2025 | **Reject** as a part; **reference** the format |
| **Tessl** | Now an "agent enablement platform": skills registry, governance loops | Proprietary | SaaS; free tier 1,000 credits a month | Pivoted away from specs | **Reject** |
| **Traycer** | Desktop planning and task-tracking app that hands work to coding agents | Proprietary | Desktop app + cloud sync, $16 per user per month | SOC 2 Type 2 | **Reject** (a second cockpit beside Orca) |
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
  the project moved repositories on 2026-06-26, a continuity risk. **Effort** about 10 stream-days, most
  of it switching its execution half off. **Verdict: reference.**

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

### 4.10 Panacea — integrate its rules and its data class, not a service

Read-only grep of `Projects/Panacea` on 2026-09-30. **Panacea has no brief, glossary or decision store**:
nothing in `services/panacea-api/routes/routes.go`, and none of `panacea-mcp`'s tools (projects, deploys,
env, logs, pods, security status, rollback) touches Define. It has three things Define should use.

- **Forge's spec** (`services/forge-orchestrator`). Forge is Panacea's own app-building agent. On an empty
  project its prompt runs a discovery "in this order": purpose, data, **access** ("for each entity, who
  may read it and who may change it … BEFORE any screen"), routes with their auth decision, then
  acceptance through a `write_spec` tool, one step at a time (`orchestrator/prompts.go:73-89`).
  `models/spec.go` refuses a spec that cannot serve as an oracle: every route needs an auth decision from
  a fixed set (`public`, `user`, `owner`, `project-role:<role>`), and there must be **3 to 8 criteria,
  each testable** (a verb such as *can*, *cannot*, *shows*, *returns*, *refuses*, *persists*). Forge's
  browser test then checks the running app against those criteria.
  - **Gaps as a Define service.** Its agent loop is Forge's own, not the unmodified CLI in our sandbox;
    the files land in Forge's session store (`SaveFile`), not in the worktree; greenfield web apps only;
    criteria are numbered, not given stable ids; no glossary or decisions. Driving it would mean a chat
    client for `/api/v1/forge/ws` in the cockpit and an import step: about 5 days, for less than
    Spec Kit gives.
  - **Verdict: integrate as snippets.** The access table goes into the preset's spec template
    (routes optional, since not every app serves routes). The validator logic (`Validate`, the
    testable-verb rule, the strict routes-table parse) goes into Publish, copied with attribution in the
    file header. About 0.25 day in G1 and none extra in G3. **Trade-off:** Forge's spec is stricter and
    tied to Panacea's own probes; Spec Kit's is richer and runs in the CLI we host, so take Spec Kit
    and graft Forge's two rules.
- **The data-class ceiling** (`services/panacea-api/handlers/deployment_stages.go:40-76`). Panacea's
  stage ladder runs innovation → secure-test ("enforced scans + SSO + synthetic data + assessed risk")
  → production ("approved risk + DLP"), against a declared ceiling of `none < synthetic < protected` in
  the `panacea.io/data-classification` annotation. It is enforced today as an attestation; the DLP
  gate in front of the AI gateway reads the same annotation (`handlers/ai_governance_dlp.go`). OD-24
  says the class is set on creation and decides the scans. **Verdict: integrate.** Define asks for it
  once, in these three words, with the residency path (AU or Global, OD-19) beside it; the project
  record and the brief carry it; Deploy hands it to Panacea (G7). **Gap:** no non-test code in
  `panacea-api` writes the annotation (grep); it is read in four places. So the hand-over is either a
  new Panacea route (Panacea's owner) or an admin step, and it belongs to `06-deploy.md`. Panacea also
  has a second vocabulary, a team policy's `OFFICIAL | PROTECTED` (`models/models.go:1098-1102`):
  owner call 4.
- **Risk requirements and the assessment document** (`routes.go:1175-1239`, team templates at
  `1997-2020`). Per project: the team template's requirements, answers, a solution status, supporting
  documents (ClamAV-scanned) and a generated assessment as DOCX or PDF. **Verdict: integrate next, not
  first.** A Define session can later draft the answers from the spec for the person to confirm, which
  is where the brief meets Deploy's secure-test gate. It is not on the path to the first shipped brief.
  The call would use a user's `pnc_` token (`00-panacea.md` §5.2), since `panacea-api` refuses a
  Keycloak realm token on REST today.
- **Backstage** (`backstage/packages`): Scaffolder and TechDocs are installed; no ADR plugin.
  **Reference.** TechDocs could show the spec to readers outside the cockpit later; today Orca renders
  it and the Hub holds the revisions.

### 4.11 Orca — integrate, as the place the Define session runs

Orca is the owner's cockpit pick; `02-fleet-cockpit.md` verifies it (MIT, 81.6k stars, v1.4.217 on
2026-09-29, Stably AI). This section covers only what Define uses, from Orca's docs on 2026-09-30.

- **Start.** `orca worktree create --name <n> --agent claude --prompt "<task>" --json` creates a
  worktree and starts Claude Code with the task (CLI reference). The Define action is that call with
  `/speckit-specify <idea>`, or `/speckit-assess-intake` from nothing. This replaces G2's session start.
- **Read and edit.** The rich markdown editor edits worktree files in place: a rendered view with a
  slash menu, tables and Mermaid, a raw Monaco toggle, and annotations on rendered text. So the person
  reads `spec.md`, `GLOSSARY.md` and the ADRs rendered, and can correct them before Publish. This
  replaces G5's read-only view.
- **Hand-off to Develop.** Orca's orchestration (runs, tasks, workers on child worktrees, decision
  gates) can turn Spec Kit's `tasks.md` into parallel worktrees, each inheriting the brief. That is
  `02`'s and `05-work-team.md`'s step, named here so the thread is visible.
- **Must stay off: artifacts.** Orca shares "HTML or Markdown as public view links through the signed-in
  Orca account", opt-in under Settings → Artifacts. A spec is customer data; our build removes it.
- **Where Publish lives.** On the person's side of the trust boundary: a button in Orca's window,
  calling our control plane with the person's token. Never a command in the sandbox terminal, which
  the agent can type into too. Orca documents **no plugin API**, so the button, the drift line and the
  brief chip are changes in our Orca fork.
- **Gaps.** No plugin API; the CLI needs the running Orca app; the docs do not say whether the markdown
  editor works on SSH or remote worktrees, which is how our sandboxes would appear (`02` decides the
  transport). **Effort for Define:** G2 0.25 day, G5 1.5 days. **Verdict: integrate.**

### 4.12 Supabase and Coolify — reference and reject, for Define

- **Supabase** (Apache-2.0, 110.9k stars). Postgres with REST, realtime, auth and storage. Self-hosts
  with Docker Compose; Supabase Cloud has a Sydney region (`ap-southeast-2`); its Auth service takes
  Keycloak as an OIDC provider, so sign-in stays Entra through Keycloak (OD-30). For Define it could
  hold brief revisions, terms and decisions in three tables with row-level security, and push changes
  to teammates live. **But Define reaches the Hub only through the built `discoverhub` client and the
  `hubapi` contract**, so Supabase is a way to implement the Hub, not a Define choice. **Trade-off:**
  Supabase gives REST, realtime and storage on day one; our Hub gives one audited API beside the
  provenance ledger that Deploy's release record needs (`06-deploy.md`). **Verdict: reference.**
- **Coolify** (Apache-2.0). A self-hosted deploy platform; `06-deploy.md` rejects it for Deploy (no
  Kata, no AKS). It has no Define capability. **Verdict: reject.**

## 5. Recommendation (revised for the owner's second direction)

**Adopt GitHub Spec Kit as the Define engine, run as an Orca worktree session on our sandbox, with one
Panacea preset that carries the brief's five sections, Forge's access table and testable-criteria rule,
the data class, and the aihero grilling and glossary skills. Keep the built provenance thread (Hub brief
revisions, keys, the Materialiser) and point it at the spec Spec Kit writes.**

**What the person does.**

1. Presses **Define** on a project or branch and types the idea; a new project also asks for its data
   class (`none`, `synthetic` or `protected`) and AU or Global. The cockpit calls
   `orca worktree create --name <n> --agent claude --prompt "/speckit-specify <idea>"` (from nothing:
   `/speckit-assess-intake` first). If `02` keeps our own shell, the phase 2 harness already takes an
   opening prompt (`control-plane/internal/harness/argv.go`, `OpeningPrompt`).
2. Answers the agent in the pane: one question at a time with a recommended answer (`clarify`),
   grilling for words and decisions (`grill-with-docs`), and Forge's access step: who may read and who
   may change each thing the app stores.
3. Reads and corrects the result in Orca's markdown editor: `specs/<n>-<name>/spec.md` with the five
   sections and the access table, `GLOSSARY.md`, `docs/adr/*.md`.
4. Presses **Publish brief** in Orca's window. The control plane checks Forge's two rules (3 to 8
   testable criteria; an access decision for every entity), then writes a brief revision to the Hub
   through the built `discoverhub`, with terms and decisions as records beside it (OD-36).
5. Every later session on that branch inherits it through the built Materialiser and shows the chip.
   Deploy reads the data class from the project.

**Why this one.**

- It covers N1-N3 with the best-maintained permissive option: MIT, GitHub's own organisation, 139.4k stars,
  a release yesterday.
- It adds no infrastructure: no server, no account, no new model endpoint. It runs inside the sandbox we
  already run, through the gateway, installed offline. N6 holds by construction.
- It writes the spec as plain files that any coding agent reads (Spec Kit itself installs into about 60
  agents), so Develop sessions read `spec.md` directly as well as inheriting the brief.
- It keeps the spine: the Hub revision and its key stay ours, and they are the part already built.
- Presets override templates, commands and terminology, so we fit it to our brief without forking it.
- **It is the strongest brief for a regulated customer on offer:** Spec Kit's structure, plus Panacea's
  two tested rules (access before interface; criteria a browser can check), plus the data class that
  Panacea's Deploy already enforces. No other candidate has any of the three.
- It uses Orca as the owner chose. Orca's own features start the session and render the files; the
  only Orca changes are the Publish button, the drift line and the chip.

**Runner-up: OpenSpec** (MIT), if brownfield change deltas turn out to matter more than the interview.
**Panacea Forge as a service is not the runner-up:** it would add a second chat surface and a second
agent loop, and still lack brownfield, the glossary and the decisions.
**Not a buy:** no commercial product passes both AU residency and "brief in our Hub".

**What it replaces**, in the plans' own hours before fix rounds:

| Replaced | Tasks | Hours |
|---|---|---|
| Phase 4, planned and unbuilt | 13 repository analysis, 18 lens asking, 19 lens states, 21 import a repository | 10.5 |
| Phase 4, OD-36 round 2 (`11-design-r2.md` §7.3) | 24 prototype, 25 term and decision model, 27 Hub reads and writes, 28 `RenderInherited`, 29 three files, 30 suggestion round, 31 lens areas, 32 relay routes | 37.5 |
| Hub, OD-36 share | five new tasks and seven amendments, shrunk to one record kind (G4, about 8 h) | 31 → 8 |
| **Total** | **12 of phase 4's 22 unbuilt tasks, most of the Hub's OD-36 share** | **about 71 of the about 109 h left** |

**Kept** (unbuilt, still needed for the thread): 11 (trimmed to publish plus reads), 12, 14, 15, 16
(trimmed), 17, 20, 22, 23 and 26, about 23.5 h. **Built and kept:** tasks 01-10 (section 2).
**OD-36 stands:** terms and decisions stay beside the brief. Only round 2's own mechanics (rounds,
suggestion states, bespoke lens states) are not built.

**Integration: about 8 stream-days** (section 6), **about 5 calendar days on two parallel streams**:
control plane (G1-G4 and G7, with tasks 11, 12, 14, 22 and 26) and cockpit (G5, with the desktop tasks
15, 16, 17 and 20); task 23 is G6. *Revised:* the first run put task 20 on the control plane, but the
plan lists it as a desktop task. Orca removes no further phase 4 task: if `02` re-seats the cockpit on
Orca, the four desktop tasks (12 h) move into Orca's renderer; they do not disappear.

## 6. What we still build (glue)

| # | Glue | Days |
|---|---|---|
| G1 | **Sandbox image and the `panacea` preset.** Add `python3` and a vendored `specify-cli` wheel bundle to `images/sandbox/Dockerfile` (the proxy already breaks Go module TLS there, so vendor as `sandboxd` does), or render `specify init --integration claude` in a build stage. The preset: the spec template gains the five brief sections (*Purpose, Audience, Must not break, Definition of done* as `SC-` lines, *Out of scope*) beside Spec Kit's own; `grill-me`, `grill-with-docs` and `domain-modeling` vendored with their MIT notice; our words through preset terminology; the community catalogue off. Pin both versions. *Revised:* the template also gains **Forge's access table** (each entity: who may read, who may change; routes with their auth decision where the app serves routes) and its **testable-criteria rule** in the checklist, and the spec header states the **data class**, so the agent plans for it (for example, synthetic test data only) | 1.25 |
| G2 | **Define session start.** *Revised:* a *Define* action calls `orca worktree create --name <n> --agent claude --prompt "/speckit-specify <idea>"`, or uses the built `OpeningPrompt` if `02` keeps our own shell. `.specify/` and the Spec Kit skills are placed in the worktree and listed in `.git/info/exclude` by the built `contextwrite` exclude logic, so Panacea still writes no tracked file | 0.25 |
| G3 | **Publish.** A control-plane route reads the **committed blobs** (spec, `GLOSSARY.md`, `docs/adr/*.md`) at the lane's head through host git, never the working file. It treats them as untrusted text: size caps, parse only, nothing executed. It maps the five sections onto the built `discover.Content` (answers `Author: model_accepted`, `DraftedFrom: ["<path>@<blob>"]`, criteria from the `SC-` lines, so phase 3 keeps its keys) and writes a revision through the built `discoverhub`; terms and decisions go as records beside it. *Revised:* before writing, it applies **Forge's rules**, adapted from `services/forge-orchestrator/models/spec.go` with attribution in the file header: 3 to 8 criteria, each testable; an access decision for every entity; the strict routes-table parse. A refusal names the fix, as Forge's does | 2 |
| G4 | **One Hub record kind** for a term or a decision that references a brief revision (OD-36), instead of round 2's Hub tasks. Hub-stream work | 1 |
| G5 | **Cockpit.** *Revised for Orca:* Orca's markdown editor is the read-and-correct view, so no rendered view of our own. In our Orca fork: one **Publish brief** button in Orca's window (never a command in the sandbox terminal), one drift line when the branch's spec blob differs from the published one or has uncommitted edits, and task 17's chip, definition of done and shipped manifest on the worktree header. Orca's artifact sharing is removed from the build | 1.5 |
| G6 | **Canary and end-to-end.** A golden spec parsed in CI, pinned to the Spec Kit version; a scripted run from idea to published brief to a new session that inherits it and shows the chip. The real-model interview joins the owner's batched session (OD-12) and stays unverified until then. *Revised:* the scripted run starts through `orca worktree create` | 1 |
| G7 | **Data class on creation (OD-24), new.** The create flow asks for the class in Panacea's words (`none`, `synthetic`, `protected`) and the AI path (AU or Global, OD-19). The project record and the brief carry it; the Materialiser writes it into `CLAUDE.md`. Deploy hands it to Panacea's `panacea.io/data-classification` annotation: that step is `06-deploy.md`'s, because no `panacea-api` route writes the annotation today | 0.5 |
| | **Glue total** | **7.5** |

The kept tasks (section 5, about 23.5 h) overlap G3 and G5, so the whole comes to **about 8 stream-days**.

### Risks, and the answer to each

1. **Upstream churn.** Commands were renamed inside the 1.0 line, and releases land every 1-3 days. Pin
   the version; G6's canary fails on any change to a section we parse; upgrade on purpose.
2. **Interview quality is the model's.** `clarify` stops at five questions a run; `grill-me` goes deeper.
   Not measured: the first owner session is the test.
3. **Two copies of the spec** (the branch's file and the Hub revision). The Hub revision is the one that
   counts: Publish records the blob it read, and G5 shows drift.
4. **An untrusted agent writes the text a person publishes.** Publish is the accept gesture, after the
   person has seen the rendered spec; the route parses and never executes.
5. **No model, no interview.** The prototype degraded to five plain questions. Here the preset's
   template is filled by hand and published, since Publish needs no model; Develop needs the model anyway.
6. **The Hub is not ready.** `discoverhub` runs against the Hub fake until HA4-2 lands. This
   recommendation neither adds to nor removes that dependency. If the plan backs the Hub with Supabase
   or Panacea's Postgres to get there sooner, Define is unchanged: it speaks only `hubapi`.
7. **Orca has no plugin API** *(new)*. The Publish button, the drift line and the chip are changes in our
   Orca fork, carried across Orca's daily releases. Keep them in one module and rebase on a fixed
   cadence; `02-fleet-cockpit.md` owns the fork policy.
8. **Orca's artifact sharing could put a spec on a public link** *(new)*. It is opt-in upstream; our build
   removes it, and a test asserts the setting is absent.
9. **Panacea has no route that writes the data class** *(new)*. Define records it from day one; until
   Panacea gains a route, Deploy carries it to Panacea by an admin step (`06-deploy.md`).
10. **Forge's rules can over-refuse** *(new)*. A 3-to-8 limit and a verb list suit web apps; a data pipeline
    may need other verbs. The refusal names the fix, and the verb list is one table in G3 that we tune
    from the first real briefs.

## 7. Calls only the owner can make

All four are about what customers see. **There is no money call:** every part is MIT or Apache-2.0 with
no seats, Orca is MIT and Panacea is our own, and the interview's model tokens run on the existing
gateway budget (cost per Define session not measured).

1. **Define becomes a conversation, not a form.** Customers get an agent-led interview in an Orca pane,
   and a brief that is a Spec Kit spec carrying our five sections, instead of the five-question stepper in
   `prototype/phase-4-discover.html` that the owner reviewed. Under OD-1 the owner looks before it
   merges. **Recommended: accept.** It is the change that removes most of the remaining build.
2. **Spec files in customer repositories.** By default the branch commits `specs/<n>-<name>/spec.md`,
   `GLOSSARY.md` and `docs/adr/`, the convention coding agents read; Spec Kit's own tooling stays
   untracked. **Recommended: commit them.** A project setting can keep them untracked later.
3. **Whose names are on screen.** Our words on buttons and in the preset's templates and terminology,
   with Spec Kit credited in the licence notices. The commands a person may see typed in the pane keep
   Spec Kit's `/speckit-*` names unless a preset can rename them (not verified). **Recommended: our
   words wherever we control the surface.**
4. **The data classes a customer picks from at creation** *(new)*. OD-24 requires one. Panacea's Deploy
   enforces three ceilings, `none`, `synthetic` and `protected`; a Panacea team policy also carries
   `OFFICIAL | PROTECTED`, the Australian Government's classification words. Customers see whichever set
   we choose, and it decides their scans. **Recommended: Panacea's three ceilings, in plain words**
   ("no sensitive data", "synthetic data only", "protected data"), because they are the ones Deploy's
   stage ladder enforces.

## 8. What this study could not verify

- **Nothing third-party was run** (the study's rule). That Spec Kit's interview works in our PTY pane, that
  a preset can carry the five sections, that `specify init` works offline in our image, and that it suits
  Claude Code 2.1.274 all rest on upstream documents, not on a run.
- **`clarify`'s status.** It is on `main` as `templates/commands/clarify.md`, but the README's command list
  omits it. Confirm it on the version we pin.
- **File locations the glue parses.** `GLOSSARY.md` is named by the aihero catalogue; the ADR directory
  (`docs/adr/` here) is an assumption until the vendored skill is read, and G1 may set both paths.
- **The GitHub API was rate-limited** (0 calls left, the address is shared). Stars, releases and licences
  were read from GitHub's HTML pages through a summarising fetch, not from `LICENSE` files. Release years
  are inferred, because GitHub omits the current year.
- **No telemetry** is what upstream pages say; the code was not read.
- **Products not reachable or not priced.** ChatPRD is blocked by the KPMG proxy. BrainGrid and Tessl
  publish no paid prices. Kiro's regions come from its data-protection page.
- **Estimates.** Task hours are the plans' own. Glue days are this study's estimate, not a measurement;
  the fix-round multiplier is the ledger's judgement.
- **Hub readiness** is unchanged by this study: the real Hub is behind HA4-2.
- **Orca** *(relaunch)*. `--prompt`, the markdown editor, artifacts and orchestration come from Orca's
  docs through a summarising fetch, not from a run or its code. The docs do not say whether the
  markdown editor works on SSH or remote worktrees, and the cost of carrying fork changes across daily
  releases is not measured.
- **Panacea** *(relaunch)*. Read by grep only; Forge's rules were read, not run. "No route writes the
  data-class annotation" rests on a grep of non-test Go in `services/panacea-api`, not on other
  services or runbooks. For `panacea-api` (the risk module, later), `00-panacea.md` §5.2 names the
  path: a user's `pnc_` token; a Keycloak realm token gets 401 on REST today. How a caller reaches
  Forge's own API was not checked; this study uses no Forge endpoint.
- **Supabase** *(relaunch)*. The Sydney region and the Keycloak provider come from Supabase's docs
  through search; self-hosting on AKS was not checked.

## 9. Sources

Upstream pages, read 2026-09-30:

- Spec Kit: <https://github.com/github/spec-kit>, its releases, `README.md`,
  `templates/spec-template.md`, `templates/commands/clarify.md` and `templates/commands/specify.md` on
  `main`; docs <https://github.github.io/spec-kit/> (air-gapped install, integrations, presets, community
  extensions); PR #4504 (the assess bundle).
- <https://github.com/mattpocock/skills> · <https://github.com/Fission-AI/OpenSpec> (and releases) ·
  <https://github.com/bmad-code-org/BMAD-METHOD> (and releases) ·
  <https://github.com/eyaltoledano/claude-task-master> · <https://github.com/open-gsd/gsd-core> and the
  archived `gsd-build/get-shit-done` · <https://github.com/spec-kitty/spec-kitty> ·
  <https://github.com/mbeacom/adrkit> · <https://github.com/yoshi1220/speckit-grill-me> ·
  <https://github.com/pragya247/spec-kit-azure-devops> · <https://github.com/outline/outline>
- Kiro: <https://kiro.dev/pricing/>, <https://kiro.dev/docs/specs/>,
  <https://kiro.dev/docs/privacy-and-security/data-protection/> · Tessl: <https://tessl.io/> ·
  Traycer: <https://traycer.ai/pricing> · BrainGrid: its comparison article on braingrid.ai ·
  ChatPRD: `chatprd.ai` (blocked) · Atlassian data residency and Rovo: support.atlassian.com, atlassian.com
- Comparisons used for orientation only: MarkTechPost (2026-05-08), `RSMuthu/SDD-comparison`.
- Glasses: `.superpowers/glasses/gsearch.py` and `https://lcfr.ai/api/mcp`, catalogue of 2026-09-29.
- *Relaunch:* Orca <https://github.com/stablyai/orca> and <https://www.onorca.dev/docs> (CLI reference,
  CLI overview, orchestration, skills registry and MCP, worktrees, rich markdown editor) · Supabase
  <https://supabase.com/docs/guides/platform/regions>,
  <https://supabase.com/docs/guides/self-hosting/self-hosted-oauth>.
- *Relaunch, Panacea, read-only* (`Projects/Panacea`): `services/forge-orchestrator/models/spec.go`,
  `orchestrator/spec_tool.go`, `orchestrator/prompts.go`, `routes/routes.go`;
  `services/panacea-api/handlers/deployment_stages.go`, `handlers/ai_governance_dlp.go`,
  `models/models.go`, `routes/routes.go`; `services/panacea-mcp/src/index.ts`;
  `backstage/packages/app/package.json` and `backstage/packages/backend/package.json`.
- *Relaunch, this repository:* `02-fleet-cockpit.md` (Orca verified), `06-deploy.md` §3.1 (Coolify),
  `01-sandboxes.md` (where the session runs).

In this repository: `docs/superpowers/planning/2026-09-23-owner-decisions.md` (OD-3, OD-19, OD-24, OD-30,
OD-36); `docs/superpowers/planning/2026-09-28-discovery-skills/00-summary.md` and `11-design-r2.md`
§7; `.claude/worktrees/phase-4/docs/superpowers/plans/2026-09-23-phase-4-discover.md` §1, §2, §6;
`.claude/worktrees/phase-4/control-plane/internal/discover/content.go`;
`control-plane/internal/harness/argv.go`; `images/sandbox/Dockerfile`.
