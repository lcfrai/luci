import { useRef, useState, type JSX } from "react";
import { useShallow } from "zustand/react/shallow";
import { api } from "../../api";
import type { ApiError } from "../../api/client";
import { classify } from "../../api/problems";
import { toCreateRequest } from "../../store/defaults";
import { selectResolvedDefaults, selectWorktrees } from "../../store/selectors";
import { useCockpit } from "../../store/store";
import type { Branch, SessionDefaults, WorktreeId } from "../../store/types";
import { LinkedWorktreeNote } from "./LinkedWorktreeNote";
import { Button } from "../fluid/components/button";
import { useProjectAccess } from "../../hooks/useProjectAccess";

/** The create path for this surface's two named actions. It lived in `SessionPane.tsx` while
 *  that file's Restart was also a create; Restart is now `api.restartSession`, so these two
 *  buttons are the only callers and it belongs where they are.
 *
 *  DO NOT COLLAPSE THIS INTO task 18's `useCreateSession`, and the reason is not the caller
 *  count. That hook RE-RESOLVES its arguments against the live store — right for "＋ New
 *  session", which should honour whatever the user's defaults say today — whereas this one
 *  sends the descriptor it was handed. Merging them would make one of the two wrong, silently,
 *  at whichever call site lost. This comment is the only thing standing between a later task
 *  and a helpful merge, which is why it travels with the code rather than staying in the file
 *  the code left.
 *
 *  It is still the second `api.createSession` call site in the tree, and task 24's audit
 *  counts on exactly two. */
export async function startSessionOnWorktree(
  worktreeId: WorktreeId,
  defaults: SessionDefaults,
): Promise<void> {
  const before = useCockpit.getState();
  const harness = before.harnesses.find((h) => h.id === defaults.harness);
  if (harness === undefined) {
    // Not a crash and not silence: the catalogue has not loaded, or the harness was removed
    // server-side between the render and the click.
    before.pushToast(`The ${defaults.harness} harness is not in the catalogue.`);
    return;
  }

  try {
    // toCreateRequest strips every field this harness does not accept (§3.6); sending
    // `effort` to a shell is a 400 the client could have avoided.
    const session = await api.createSession(toCreateRequest(worktreeId, defaults, harness));
    const after = useCockpit.getState();
    after.upsertSession(session);
    after.focusSession(session.id);
  } catch (e) {
    // Every client method rejects with ApiError and nothing else (§4).
    useCockpit.getState().pushToast(sentenceForCreate(e as ApiError));
  }
}

/** The prose is local because its tail names THIS act; the discrimination is `classify` because
 *  §4.2's non-collapse rule is one rule and three copies of it drift.
 *
 *  LOAD-BEARING SHAPE: declared return type, switch over `ProblemKind`, no `default:`. A fourth
 *  kind must fail this function with TS2366 rather than fall through a catch-all. */
function sentenceForCreate(error: ApiError): string {
  switch (classify(error)) {
    case "control-plane-down":
      return "The control plane is not running. Start it, then try again.";
    case "backend-down":
      return "Docker is not running, so a sandbox cannot be started.";
    case "server-said":
      // Including the sixteenth code, non_loopback_publish, whose sandbox is RUNNING.
      return error.message;
  }
}

/** Help the user select a branch, open its checkout or start the chosen real CLI. */
export function EmptyDevelop({ branch }: { branch: Branch | undefined }): JSX.Element {
  const access = useProjectAccess(branch?.projectId ?? "");
  // §3.7: selectWorktrees returns a stable empty-array constant when the branch is absent,
  // so this needs no useShallow and no `?? []`.
  const worktrees = useCockpit(selectWorktrees(branch?.id ?? ""));
  const selectedWorktreeId = useCockpit((s) => s.selection.worktreeId);
  // useShallow is mandatory: resolveDefaults builds a fresh SessionDefaults on every call, so
  // a bare read re-renders this on every 2 s poll (task 17's amendment to §3.7).
  const defaults = useCockpit(useShallow(selectResolvedDefaults));
  const harnesses = useCockpit((s) => s.harnesses);
  const openBranchManager = useCockpit(s => s.openBranchManager);
  const [pending, setPending] = useState("");
  const starting = useRef(false);
  async function start(worktreeId: WorktreeId, descriptor: SessionDefaults) {
    if (starting.current || !access.canExecute || descriptor.harness !== "shell" && !access.canAssist) return;
    starting.current = true; setPending(descriptor.harness);
    try { await startSessionOnWorktree(worktreeId, descriptor); }
    finally { starting.current = false; setPending(""); }
  }

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
          Open {branch.name} in your project to start working.
        </p>
        <p className="pane-empty-note">
          Luci can switch the main project checkout to this branch after you commit changes and stop active sessions.
        </p>
        {access.message && <p className="pane-empty-note">{access.message}</p>}
        <Button disabled={!access.canEdit} onClick={() => openBranchManager({ projectId: branch.projectId, action: "switch", branchId: branch.id })}>Switch to {branch.name}</Button>
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
      {/* Before the buttons, so the user reads what a session here cannot do before starting one. */}
      <LinkedWorktreeNote worktree={target} />
      {access.message && <p className="pane-empty-note">{access.message}</p>}
      <div className="pane-empty-actions">
        <Button
          type="button"
          loading={pending === defaults.harness}
          disabled={!!pending || !access.canExecute || defaults.harness !== "shell" && !access.canAssist}
          title={defaults.harness !== "shell" && !access.canAssist ? "Credential access is required to run a coding agent." : undefined}
          onClick={() => void start(target.id, defaults)}
        >
          {`＋ ${primary?.name ?? defaults.harness}`}
        </Button>
        {shell === undefined ? null : (
          <Button
            type="button"
            variant="secondary"
            loading={pending === shell.id}
            disabled={!!pending || !access.canExecute}
            onClick={() =>
              // permissionMode: "ask" is passed and toCreateRequest strips it, because
              // shell.permissionModes is empty — §3.6's rule, not a special case here.
              // Passing "" instead would be a lie about a legal PermissionMode.
              void start(target.id, {
                harness: shell.id,
                model: "",
                effort: "",
                permissionMode: "ask",
              })
            }
          >
            Open a shell
          </Button>
        )}
      </div>
    </div>
  );
}
