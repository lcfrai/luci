# 03 — Define (the brief, the interview, the spec): the sceptic's verification

**Status:** IN PROGRESS (skeleton). Sections are filled one at a time and committed after each.
**Date:** 2026-09-30. **Verifies:** `03-define.md` (the researcher's recommendation).
**Role:** sceptic. Try to refute the recommendation (GitHub Spec Kit as the Define engine, run
as an Orca worktree session, with a Panacea preset) and its two best alternatives at source:
licence, the vendor's own docs, paid-tier gates, maintenance, and whether the days fit OUR
architecture (Go control plane `Backend`, Tauri/React desktop or Orca, Keycloak, LiteLLM, Azure).
Desk research only: nothing installed or run.

## 0. Verdict first

*(to be filled)*

## 1. What was checked, and how

All reads on 2026-09-30 (from 00:50 UTC). Nothing installed, built or run; no third-party code executed.

- **The report under test:** `03-define.md`, all 529 lines.
- **GitHub API: one call left, then 403** ("API rate limit exceeded", shared address). So every
  licence was read from the **`LICENSE` file itself** (`raw.githubusercontent.com/<repo>/HEAD/LICENSE`),
  every release date from the repo's **`releases.atom` feed** (dates carry the year), and stars from
  the repo page's `repo-stars-counter-star` title. This is stronger than the report's summarising
  fetch of HTML pages.
- **Spec Kit at tag `v1.0.13`, raw:** `README.md`, `pyproject.toml`, `CHANGELOG.md` (3,052 lines,
  grepped), `templates/commands/specify.md`, `templates/commands/clarify.md`,
  `templates/spec-template.md`, `src/specify_cli/__init__.py`. Docs pages `reference/presets.html`
  and `installation.html` through a summarising fetch.
- **OpenSpec `v1.13.2`, raw:** `README.md`, `package.json`, `LICENSE`, releases feed.
- **mattpocock/skills `main`, raw:** `README.md` and the `SKILL.md` of `grilling`, `grill-me`,
  `grill-with-docs`, `domain-modeling` and `setup-matt-pocock-skills`.
- **Orca `main`, raw docs source:** `docs/site/content/docs/cli/reference.mdx`,
  `editing/markdown.mdx`, `ways-to-run.mdx`, `remote-servers.mdx`, `meta.json`; and
  `02-fleet-cockpit.md` §0 and R2-R4, which read Orca's **source** at `59b746ff`.
- **Panacea, read-only** (reads and `git grep` only): `services/forge-orchestrator/models/spec.go`
  (all 304 lines), `orchestrator/spec_tool.go`, `orchestrator/prompts.go:55-104`,
  `routes/routes.go`, `orchestrator/orchestrator.go` and `config/config.go` by grep;
  `services/panacea-api/handlers/deployment_stages.go:36-76, 655-690, 1190-1225`,
  `clients/branch_data_policy.go`, `clients/branch_data_ops.go`, `models/models.go:1096-1103`,
  `routes/routes.go` at the cited lines; a **whole-repository** `git grep data-classification`.
- **This repository:** `images/sandbox/Dockerfile`, `control-plane/internal/harness/argv.go`, the
  phase 4 plan's task table (`.claude/worktrees/phase-4/…/2026-09-23-phase-4-discover.md:402-414`),
  `2026-09-28-discovery-skills/11-design-r2.md` §7.5.

## 2. The recommendation: GitHub Spec Kit, in an Orca session, with a Panacea preset

**Verdict: UPHELD as the Define engine.** Spec Kit's own claims hold at source almost word for word.
Three findings the report missed change how G1 builds it (2.4). **The Orca half does not survive as
written** (2.5): it contradicts `02-fleet-cockpit.md`'s source-read ruling in two places.

### 2.1 Licence at source

MIT. `LICENSE` at `HEAD`: "MIT License / Copyright GitHub, Inc.", the standard 21-line text, no
trademark or field-of-use clause. Nothing restricted; shipping it inside our sandbox image is fine
with the notice kept.

### 2.2 Maintenance

- **139,453 stars** (repo page counter). GitHub's own organisation.
- Releases (Atom feed): 1.0.13 on 09-29, 1.0.12 09-25, 1.0.11 09-24, 1.0.10 09-23, 1.0.9 09-21,
  1.0.8 09-18. **1.0.0 was 2026-08-21**, 40 days ago; 0.12.8 was 07-08. About 40 releases in 12
  weeks. "Every 1-3 days" holds (gaps of 1 to 4 days).
- **Patch releases carry features**, which the report's churn risk understates: 1.0.4 lets a preset
  declare a required extension, 1.0.9 added the first-party `assess` and `bugfix` bundles, 1.0.10 and
  1.0.12 refactored the preset CLI. Pin the exact version and read every note before a bump.

### 2.3 The claims, against the source

| # | Claim in `03` | At source (v1.0.13) | Holds? |
|---|---|---|---|
| 1 | `clarify`: up to 5 questions, exactly one at a time, 2-5 options, recommended first; answers under `## Clarifications` / `### Session <date>`; scans terminology | `clarify.md:2, 113, 129-186, 233`, near verbatim: "Present EXACTLY ONE question at a time", "`**Recommended:** Option [X]`" | **Holds** |
| 2 | `clarify`'s status unclear (README omits it) | It ships in core; README:91 lists it as an optional gate ("Add clarification, checklists, and consistency analysis") | **Holds; resolved** |
| 3 | Spec: prioritised stories, Given/When/Then, edge cases, `FR-001`, key entities, `SC-001`, assumptions; at most 3 `[NEEDS CLARIFICATION]` | `spec-template.md:11-120`; `specify.md:128` | **Holds** |
| 4 | `/speckit-assess-intake` when starting from nothing | `assess` is a **bundled, opt-in extension**: `specify extension add assess`, then five skills (intake → research → define → shape → decide) ending *go / needs-clarification / kill* (README:128-153). It became a first-party bundle **in 1.0.9, nine days ago** (#4504) | **Holds, with a cost:** G1 must install it, and it means five commands before `specify` |
| 5 | Installs offline; `init` needs no network | `pyproject.toml:28-58`: the wheel bundles templates, commands, scripts, the extensions `git`, `agent-context`, `assess`, `bug` and `github`, and catalog snapshots, "so `specify init` works without network access (air-gapped / enterprise)" | **Holds** |
| 6 | Claude Code skills under `.claude/skills`, invoked as `/speckit-*` | README:62-66 ("agent skills, not terminal commands"); CHANGELOG 0.14.2, #3708 | **Holds** |
| 7 | Presets override templates, commands and terminology, stack by priority, install with `--dev <path>` | Presets reference page, verbatim on all three. Renaming or hiding a `/speckit-*` command: not documented | **Holds** |
| 8 | No telemetry | Eight runtime dependencies (typer, click, rich, readchar, pyyaml, packaging, pathspec, json5): no HTTP or analytics client. `specify self check` runs only when invoked | **Holds as far as read**; submodules not read |
| 9 | "Commands were renamed in the 1.0 line" | The renames (Claude moved to `.claude/skills`, `/speckit.x` became `/speckit-x`) date from 0.14, in July. The 1.0 churn is presets, bundles and the catalog | **Imprecise.** The risk and its fix (pin, canary) stand |
| 10 | Python needed in the sandbox | Needs Python ≥ 3.11 (`pyproject.toml:6`). `images/sandbox/Dockerfile` is `node:20-bookworm-slim` with no python3 | **Holds, as a G1 cost** |

### 2.4 Three findings the report missed

**A. Spec Kit's success criteria are the kind Forge's rule refuses.** G3 takes the *Definition of
done* from the `SC-` lines and applies Forge's testable rule. But `specify.md:318-339` tells the agent
that success criteria must be "Measurable: include specific metrics (time, percentage, count, rate)"
and technology-agnostic, with examples such as "Task completion rate improves by 40%". That sentence
matches none of Forge's verbs, so Publish would refuse the spec Spec Kit was told to write, and the
person would see a refusal for doing what the agent asked. **Fix:** the preset adds its own section,
*Acceptance criteria* (`AC-001…`, 3 to 8, each testable), for the Definition of done, and leaves
`SC-` as outcomes. Phase 3 keys on `AC-`. A new template section alone is weak, because the
command's checklist does not know it. So the preset also **overrides the `specify` command's
criteria guidance**, and the canary covers that command file too. About +0.25 day in G1.

**B. The `git` and `agent-context` extensions must stay off, and a test must say so.** Both are
opt-in (#2873 at 0.10.0; #3097). Branch creation happens only in a `before_specify` hook of the
`git` extension (`specify.md:74-78`). CLAUDE.md edits happen only in `agent-context`. Installed,
the first would switch branches inside our lane, and the second would write the `CLAUDE.md` that
the built Materialiser owns (N5). The report never names either. It is a constraint on G1, and it
costs nothing if written down.

**C. The agent must not hold the `specify` CLI.** G1 puts `specify-cli` into the runtime image.
Then the untrusted agent can run `specify extension add <id>` against the community catalog (130+
entries, "not an audit or endorsement", per the report) and install third-party instructions into
its own skills. The report says only that the catalogue "must be switched off". **Fix:** run
`specify init --integration claude`, `preset add --dev` and `extension add assess` in a
**build stage**. Copy only `.specify/` and `.claude/skills/speckit-*` into the runtime image, and
keep sandbox egress closed to GitHub and PyPI. The commands call their helper scripts as `sh`,
`ps` or `py` variants (`clarify.md:7-10`). Choosing the `sh` variant might drop python3 from the
runtime image altogether (not verified). The report offered the build-stage render as an option.
Make it the rule.

### 2.5 The Orca half: refuted as written, rebuilt on `02`'s ruling

`02-fleet-cockpit.md` read Orca's **source** and ruled: **adopt Orca unforked**. Each lane runs
in our Kata sandbox through one Panacea recipe, and one Panacea plugin carries the brief (02 §0,
R3). `03` read Orca's docs only, and it conflicts with that ruling in two places.

1. **The start command runs outside our sandbox.** `orca worktree create --agent claude --prompt`
   is real: "`--agent` launches the selected agent in the first terminal. `--prompt` sends initial
   work to that agent" (`cli/reference.mdx:98-113`). But the CLI "talks to a running Orca runtime",
   and without a recipe it makes a worktree on that runtime's host, the laptop. A lane in our
   sandbox is a recipe workspace. **"Orca's CLI cannot create a recipe workspace"** (02 R3, from
   source). So the report's command runs Claude Code on the laptop, which has no per-session
   gateway token and no AU path. Then "identity, audit and AU residency hold by construction" is
   false for exactly the command the report names. **Fix:** Define starts through 02's recipe. The
   create-lane route carries a *pending opening prompt*, which the sandbox hands to the agent: the
   same two-step pattern 02 uses for fork. G2 goes from 0.25 to 0.75 day, and it waits on 02's
   recipe.
2. **"Orca documents no plugin API, so fork it" is wrong.** Plugin API v1 exists. It is
   experimental, with panels, commands and events, and 02 rules out a fork for v1. But the plugin
   API has **no `net:fetch` yet** (02 R3), so a plugin button cannot call our control plane. And
   `03`'s own rule forbids sending Publish through the sandbox terminal. **Fix:** Publish, the drift
   line and the definition of done go on a **Hub web page opened in Orca's browser tab**, which is
   02 R4's direction for the Define lens. The plugin's sidebar panel shows the chip. Tasks 15, 16,
   17 and 20 move to Hub web pages, not "into Orca's renderer". **Customer-visible change:**
   unforked, there is no chip on each pane header (task 17's "chip on every pane header"). The
   brief shows in the Panacea panel and on the Hub page instead.
3. **Publish would accept text the person never saw. This is the sharpest gap.** In the report the
   person reads the **working copy** in Orca's editor, and then Publish reads the **committed
   blob**. The agent controls both. It can commit one text and leave another on screen. Also, under
   02 the person's own corrections are working-copy edits in the sandbox, and they reach the lane
   endpoint only after Orca's *Commit and Push*: a step the report's flow leaves out. **Fix:** the
   Hub page renders **the exact pushed blob** it will publish, beside Publish, and says "edits not
   pushed" when the working copy differs. The accept gesture then covers the published bytes.
4. **Editing a remote lane: resolved, from the docs.** For a Remote Orca Server, "Terminals, agent
   processes, files, worktrees, and session state live on the server computer", and you "use Orca
   normally" (`remote-servers.mdx:101`). For SSH, "the editor, diff, and UI stay on your laptop"
   (`ways-to-run.mdx:31`). The rich editor has annotations on rendered text (`markdown.mdx:23-25`).
   This is documented, not run.
5. **Artifact sharing cannot be "removed from our build", because there is no build of ours.**
   "Share as artifact" publishes markdown "as a public view link through your signed-in Orca
   account". It is off by default, and **"Agents and scripts can use `orca artifacts share`"**
   (`markdown.mdx:52-54`). So the control has to hold against the agent too: no Orca account sign-in
   on the laptop or on the sandbox's `orca serve`, the setting kept off, and egress to Orca's cloud
   blocked from sandboxes. Risk 8's "a test asserts the setting is absent" becomes a check of the
   sandbox's Orca settings plus an egress test.

## 3. Alternative 1: OpenSpec (the report's runner-up)

**Verdict: UPHELD as reference and runner-up, with one fact the report missed: telemetry is on by
default.**

- **Licence at source:** MIT. `LICENSE` reads "Copyright (c) 2024 OpenSpec Contributors";
  `package.json` says `"license": "MIT"`. Holds.
- **Maintenance:** 70,691 stars. Releases v1.13.2 on 2026-09-23, v1.13.1 on 09-17, v1.13.0 on
  09-09, so about weekly. `MAINTAINERS.md` names **two core maintainers plus a bot**
  (`@alfred-openspec`). The repo sits under the `Fission-AI` organisation, but no company backing
  is stated in the file. The report's "small company" is, if anything, generous: the bus factor
  is two.
- **Claims:** `/opsx:explore` ("a no-stakes thinking partner"), `/opsx:propose`, `apply` and
  `archive`, with deltas archived under `openspec/changes/archive/`, and "30+ tools". All hold
  (README:39-75, 150-158). There is still no structured one-question-with-a-recommendation interview
  and no glossary or ADR, as the report says.
- **Missed: telemetry is on unless switched off.** "OpenSpec collects anonymous usage stats … only
  command names and version … unset means on", "Automatically disabled in CI". It is disabled by
  `OPENSPEC_TELEMETRY=0` or `DO_NOT_TRACK=1` (README:246-254). Its runtime dependencies name no
  analytics SDK, so the call is plain `fetch` to an endpoint the README does not name. That is
  fine in our sandbox once the image sets the variable and egress is closed. It is still one more
  thing to prove than Spec Kit, which names no telemetry.
- **Fit:** an npm CLI that needs Node ≥ 20.19.0. Our image is `node:20`, so it fits if the tag
  resolves to 20.19 or later (not checked). No paid tier, and none is needed.
- **Days:** the report's 9 stream-days are plausible, and the Orca and Publish corrections in 2.5
  apply equally. **It does not overturn the recommendation.** Spec Kit has the structured `clarify`,
  a larger maintainer base and GitHub behind it. OpenSpec wins only if change deltas in long-lived
  repositories matter more than the interview.

## 4. Alternative 2: Panacea Forge's spec as a Define service (the owner's default, on merit)

**Verdict: the report's rejection is UPHELD, and strengthened at source.** Forge's rules are
worth taking (section 5). Forge as a Define service is not.

- **Licence:** KPMG's own. Snippets are allowed with attribution (OD-13 relaxed). No issue.
- **Its own agent loop, confirmed.** `orchestrator.go` speaks the Anthropic Messages shape
  (`/v1/messages`, `anthropic-version`, around `:690-731`) with its own tools (`write_spec`
  at `:1548` and `:1777`, `create_file`). Its models are `claude-sonnet-4-6-global` and
  `claude-opus-4-8-global` (`config.go:171-172`). So the unmodified CLI is not in our sandbox,
  and `OD-8`'s harness contract does not apply.
- **Files in Forge's store, confirmed.** `writeSpecTool` validates, then calls
  `store.SaveFile(ctx, sessionID, "SPEC.md", …)` (`spec_tool.go:34-47`). Nothing lands in a
  worktree.
- **Greenfield only, confirmed.** The discovery runs only `if len(files) == 0`
  (`prompts.go:72`). Otherwise it is build mode on Forge's own project. No path imports a
  customer repository: the only clone is in `ship/`.
- **Residency does not switch per project.** The AI provider comes from env or from one
  platform-wide `platform_settings` row, `ai_providers` (`config.go:36-42, 300-306`), with
  `-global` model names. `OD-19` wants AU or Global per project.
- **Reaching it:** Forge accepts exactly one credential, "an `Authorization: Bearer
  <BACKEND_SECRET>`" (`middleware.go:98-116`). Users come through panacea-api's forge proxy
  (`routes.go:3106-3108`), which needs a user's `pnc_` token, since realm tokens are refused
  (`00-panacea.md` §5.2). The report left this unchecked.
- **Days:** "about 5 days" for a websocket chat client plus an import step is realistic. It would
  still give a second chat surface, a second agent loop outside the sandbox, no brownfield and
  no glossary or decisions. **Rejected on merit, not on origin.**

## 5. The parts grafted in: aihero skills, Forge's spec rules, Panacea's data class

*(to be filled)*

## 6. The integration days, against our architecture

*(to be filled)*

## 7. Corrected recommendation

*(to be filled)*

## 8. What this verification could not verify

*(to be filled)*

## 9. Sources

*(to be filled)*
