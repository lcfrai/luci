import type { JSX } from "react";
import type { Worktree } from "../../store/types";

/** The one sentence, exported so a test names it rather than re-typing it.
 *
 *  It names the fact that justifies it — docs/AGENTIC-UX.md §2 property 7 — and the fact is
 *  mechanical: `isPrimary: false` from the read-only enumeration means a LINKED worktree, and a
 *  linked worktree's `.git` is a file holding `gitdir: C:/…`, a host path that does not exist
 *  inside the Linux sandbox. So every git command there dies with "not a git repository". The
 *  primary worktree's git works (the sandbox git fix): it converts line endings as the host's git
 *  does in that worktree, and it cannot prune the linked worktrees' metadata (round 1), which
 *  makes "not primary" exactly the set without working git. The managed branch switch flow
 *  operates the primary project checkout. */
export const LINKED_WORKTREE_GIT_NOTE =
  "Git is unavailable in this worktree's sessions because it is a linked worktree, whose .git " +
  "points to a path on this machine the sandbox cannot see; open this branch in the main project " +
  "checkout to use Git.";

/** Where a user starts a session in a worktree, or reads one, and that worktree is linked.
 *
 *  Nothing for the primary worktree, and nothing while the worktree is unknown: a worktree the
 *  store has not loaded proves nothing either way, and a notice claiming a fact the cockpit has
 *  not seen is the element property 7 says must not exist. */
export function LinkedWorktreeNote({
  worktree,
}: {
  worktree: Worktree | undefined;
}): JSX.Element | null {
  if (worktree === undefined || worktree.isPrimary) return null;
  return (
    <p className="pane-gitnote" role="note" data-linked-worktree-note="">
      {LINKED_WORKTREE_GIT_NOTE}
    </p>
  );
}
