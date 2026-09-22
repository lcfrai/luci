// Contract §6.4 and §0.1: the LEFT half creates with resolved defaults and opens no dialog;
// the caret opens the picker. The prototype opened a dialog for both and the spec supersedes it.
import type { JSX } from "react";
import { useCockpit } from "../../store/store";
import { useCreateSession } from "../../hooks/useCreateSession";
import type { WorktreeId } from "../../store/types";
import "./topbar.css";

export function NewSessionButton(props: { worktreeId: WorktreeId | null }): JSX.Element {
  const { worktreeId } = props;
  const createSession = useCreateSession();
  const pickerOpen = useCockpit((s) => s.pickerOpen);
  const openPicker = useCockpit((s) => s.openPicker);
  const closePicker = useCockpit((s) => s.closePicker);

  return (
    <div className="top-split">
      <button
        type="button"
        className="top-split-main"
        disabled={worktreeId === null}
        title={
          worktreeId === null
            ? "Select a worktree in the rail first — a session runs in one."
            : undefined
        }
        onClick={() => {
          if (worktreeId !== null) void createSession(worktreeId);
        }}
      >
        ＋ New session
      </button>
      <button
        type="button"
        className="top-split-caret"
        aria-label="Session options"
        aria-haspopup="dialog"
        aria-expanded={pickerOpen}
        data-overlay-trigger=""
        onClick={() => (pickerOpen ? closePicker() : openPicker())}
      >
        ▾
      </button>
    </div>
  );
}
