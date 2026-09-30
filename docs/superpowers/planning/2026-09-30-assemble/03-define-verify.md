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

## 3. Alternative 1

*(to be filled)*

## 4. Alternative 2

*(to be filled)*

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
