import type { JSX } from "react";
import { useShallow } from "zustand/react/shallow";
import { startSessionOnWorktree } from "./SessionPane";
import { selectResolvedDefaults, selectWorktrees } from "../../store/selectors";
import { useCockpit } from "../../store/store";
import type { Branch } from "../../store/types";

/** Three zero states, not one. docs/DESIGN-SYSTEM.md's floor is that every state is designed,
 *  and "empty" here is three different situations that deserve three different sentences: no
 *  branch is a navigation state whose answer is the rail; a branch with no worktrees is the
 *  one §10.3 calls out, because POST /v1/sessions needs an existing worktreeId and worktree
 *  creation is Phase 3, so it must say so in WORDS rather than offer a button that 404s; and
 *  a worktree with no sessions is the only one with actions. No spinner and no skeleton in
 *  any of them — §7.7 allows none that shifts a pixel. */
export function EmptyDevelop({ branch }: { branch: Branch | undefined }): JSX.Element {
  // §3.7: selectWorktrees returns a stable empty-array constant when the branch is absent,
  // so this needs no useShallow and no `?? []`.
  const worktrees = useCockpit(selectWorktrees(branch?.id ?? ""));
  const selectedWorktreeId = useCockpit((s) => s.selection.worktreeId);
  // useShallow is mandatory: resolveDefaults builds a fresh SessionDefaults on every call, so
  // a bare read re-renders this on every 2 s poll (task 17's amendment to §3.7).
  const defaults = useCockpit(useShallow(selectResolvedDefaults));
  const harnesses = useCockpit((s) => s.harnesses);

  if (branch === undefined) {
    return (
      <div className="pane-empty">
        <p className="pane-empty-line">Select a branch in the rail to start working on it.</p>
      </div>
    );
  }

  if (worktrees.length === 0) {
    return (
      <div className="pane-empty">
        <p className="pane-empty-line">
          {branch.name} has no worktrees yet, and a session runs inside one.
        </p>
        <p className="pane-empty-note">
          Creating a worktree arrives in Phase 3. Until then, check out this branch in a
          worktree from the command line and it will appear in the rail.
        </p>
      </div>
    );
  }

  // The same rule §6.9 gives ⌘⏎: the selected worktree, falling back to the branch's first.
  // Two surfaces that resolved it differently is how a user ends up with a session on a
  // worktree they were not looking at.
  const target = worktrees.find((w) => w.id === selectedWorktreeId) ?? worktrees[0];
  // Labelled from the RESOLVED defaults — "＋ Claude Code" when that is what would start,
  // "＋ Codex" when it is not. A hard-coded label lies the first time a user changes their
  // default, and §6.4's rule that the gate is the descriptor and never `harness ===
  // "claude-code"` applies to the wording as much as to the logic.
  const primary = harnesses.find((h) => h.id === defaults.harness);
  const shell = harnesses.find((h) => h.id === "shell");

  return (
    <div className="pane-empty">
      <p className="pane-empty-line">No sessions on {target.name}.</p>
      <div className="pane-empty-actions">
        <button
          type="button"
          className="pane-empty-go"
          onClick={() => void startSessionOnWorktree(target.id, defaults)}
        >
          {`＋ ${primary?.name ?? defaults.harness}`}
        </button>
        {shell === undefined ? null : (
          <button
            type="button"
            className="pane-empty-go"
            onClick={() =>
              // permissionMode: "ask" is passed and toCreateRequest strips it, because
              // shell.permissionModes is empty — §3.6's rule, not a special case here.
              // Passing "" instead would be a lie about a legal PermissionMode.
              void startSessionOnWorktree(target.id, {
                harness: shell.id,
                model: "",
                effort: "",
                permissionMode: "ask",
              })
            }
          >
            Open a shell
          </button>
        )}
      </div>
    </div>
  );
}
