#!/usr/bin/env bash
# Task 25. A throwaway project for the Phase 2 end-to-end run: one repository with two
# branches, and one linked worktree on the second. Linked worktrees live OUTSIDE the
# projects root, because project.go enumerates every child directory holding a .git
# entry and a linked worktree's .git is a file — inside the root it would be a project.
#
# Usage: bash desktop/e2e/fixture.sh <empty-directory>
# Prints the Windows form of the projects root, for LUCI_PROJECTS_DIR.
set -euo pipefail

root="${1:?usage: fixture.sh <empty-directory>}"
[ -z "$(ls -A "$root" 2>/dev/null)" ] || { echo "fixture: $root is not empty" >&2; exit 1; }

mkdir -p "$root/projects/e2e-demo" "$root/worktrees" "$root/state"
cd "$root/projects/e2e-demo"
git init -q -b main
git -c user.name=e2e -c user.email=e2e@invalid commit -q --allow-empty -m "e2e: root"
git branch feat/e2e
git worktree add -q "$root/worktrees/e2e-demo-feat" feat/e2e

cygpath -w "$root/projects"
