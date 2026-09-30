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

*(to be filled)*

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
