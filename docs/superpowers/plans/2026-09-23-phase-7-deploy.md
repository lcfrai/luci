# Phase 7 — the Deploy connector: implementation plan

> **For agentic workers:** execute task by task with `superpowers:subagent-driven-development` or
> `superpowers:executing-plans`. Every task's part file is in
> `docs/superpowers/plans/2026-09-23-phase-7-deploy-parts/task-NN.md`. Steps use `- [ ]` checkboxes.
> Every task is test-first.

**Status:** written 2026-09-23 on branch `feat/phase-7-deploy` (worktree
`.claude/worktrees/phase-7`, based on `3303fd6`). Owner-reviewable under `OD-1`; adversarially
reviewed before code. **Nothing in this plan was compiled or run.** Every citation below was opened
in this pass unless it is marked *unverified*; §10 lists what was not verified.

---

## 1. Goal

A branch in the cockpit gets a **Ship** action. Ship reserves a release in the Hub (which fixes the
brief pin), starts a real Tekton pipeline on Panacea through `panacea-api`, and confirms the run
against the reservation. The **Deploy lens** then becomes the honest account of what happened, per
environment: which commit, which image, which version, whether the build passed, whether the scans
passed, whether pods are actually serving, and **which brief revision the release was shipped
against**. Cancel, rollback and redeploy are the three other acts; auto-deploy, project access,
branch access posture and the URL prefix are the four governance controls, each drawn as the API
really offers it.

It is the last rung of the provenance thread (`CLAUDE.md`, *"attached to the release in Deploy"*):
the pin a release carries is taken by the **Hub** at reservation, never asserted by the caller after
the fact, and never inferred from what the poller observes.

**What is not new:** every Panacea capability used here already exists as a `panacea-api` route
(§4). Phase 7 adds no Panacea endpoint and copies no Panacea source (`OD-13`). It is a typed Go
client behind a `Deployer` interface, a Ship orchestrator, a deploy poller on the control plane's
existing clock, a set of control-plane routes, and the lens.

