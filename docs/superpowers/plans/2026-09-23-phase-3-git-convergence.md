# Phase 3 — Git, worktrees, coordination and convergence — implementation plan

> **For agentic workers:** execute task by task, in the order of the task list at the end. Each
> task's full specification is its part file in
> `docs/superpowers/plans/2026-09-23-phase-3-git-convergence-parts/task-NN.md`. The part file is
> the contract for that task; this file is the map. Steps use checkbox (`- [ ]`) syntax.

**Status: DRAFT, being written.** This line is removed when the task list is complete.

## Goal

Make a Panacea worktree **real git that works inside its sandbox**, and converge several of them
into their product branch with provenance. Concretely, when this plan is done:

1. The cockpit creates and removes worktrees. A worktree is a **local clone** of the project
   repository (OD-2), checked out on the product branch's own name, so an agent inside the sandbox
   has a working git whose world matches the rail. The linked-worktree git failure that phase 2's
   sandbox fix could only *announce* is gone for every worktree the cockpit creates.
2. Every agent commit carries `Panacea-Session: <id>`, added by a git hook the image ships and the
   control plane enables through environment, chained to the repository's own hook.
3. The control plane fetches each clone's work back into the project repository under
   `refs/panacea/lanes/<worktree-id>` and computes, from git alone, what each worktree changed, what
   it has not committed, and whether it will land clean against the branch **as it will be** after
   the worktrees above it in the queue.
4. A human lands worktrees into their branch one at a time, in the order they set, by plumbing with
   compare-and-swap. The merge commit carries the brief key the human saw as a trailer. A land can be
   undone exactly until it is pushed or built upon, and by a revert commit after.
5. The branch pushes from the control plane with the user's own credentials, and a PR opens on
   **Azure DevOps or GitHub** behind one host interface (OD-4). The sandbox never holds a git
   credential.
6. Each diff hunk links to the commit and session that wrote it (tier 1, always), and, for Claude
   Code, to the tool call that wrote it through a version-pinned, canary-tested transcript adapter
   that degrades to tier 1 (OD-8).
7. The **Merge** and **Files** lenses show all of the above, and nothing a human cannot trace to a
   git command or a recorded fact.

## Architecture

<!-- piece 2 -->
