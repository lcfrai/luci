import { NavigationIcon } from "../common/NavigationIcon";
// Contract §6.4 and §0.1: the LEFT half creates with resolved defaults and opens no dialog;
// the caret opens the picker. The prototype opened a dialog for both and the spec supersedes it.
import type { JSX } from "react";
import { useCockpit } from "../../store/store";
import { useCreateSession } from "../../hooks/useCreateSession";
import { useProjectAccess } from "../../hooks/useProjectAccess";
import { selectResolvedDefaults, selectWorktreeById } from "../../store/selectors";
import type { WorktreeId } from "../../store/types";
import "./topbar.css";

export function NewSessionButton(props: {
  worktreeId: WorktreeId | null;
}): JSX.Element {
  const { worktreeId } = props;
  const projectId = useCockpit(state => selectWorktreeById(worktreeId ?? "")(state)?.projectId ?? "");
  const access = useProjectAccess(projectId);
  const harness = useCockpit(state => selectResolvedDefaults(state).harness);
  const canStart = access.canExecute && (harness === "shell" || access.canAssist);
  const createSession = useCreateSession(projectId);
  const pickerOpen = useCockpit((s) => s.pickerOpen);
  const openPicker = useCockpit((s) => s.openPicker);
  const closePicker = useCockpit((s) => s.closePicker);

  return (
    <div className="top-split">
      <button
        type="button"
        className="top-split-main"
        disabled={worktreeId === null || !canStart}
        title={
          worktreeId === null
            ? "Select a worktree in the rail first — a session runs in one."
            : access.message || (!canStart ? "Credential access is required for a coding agent. Session options can start a shell." : undefined)
        }
        onClick={() => {
          if (worktreeId !== null && canStart) void createSession(worktreeId);
        }}
      >
        <NavigationIcon name="add" /> New session
      </button>
      <button
        type="button"
        className="top-split-caret"
        aria-label="Session options"
        aria-haspopup="dialog"
        aria-expanded={pickerOpen}
        data-overlay-trigger=""
        disabled={!access.canExecute}
        title={access.message || undefined}
        onClick={() => (pickerOpen ? closePicker() : openPicker())}
      >
        <NavigationIcon name="chevron-down" />
      </button>
    </div>
  );
}
