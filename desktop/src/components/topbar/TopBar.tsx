// Contract §6.4: the bar. Layout toggle, Authority, Run locally, the split New-session button.
import type { JSX } from "react";
import { useCockpit } from "../../store/store";
import { LayoutToggle } from "./LayoutToggle";
import { NewSessionButton } from "./NewSessionButton";
import { SessionPicker } from "./SessionPicker";
import "./topbar.css";

export function TopBar(): JSX.Element {
  const layout = useCockpit((s) => s.layout);
  const setLayout = useCockpit((s) => s.setLayout);
  const pickerOpen = useCockpit((s) => s.pickerOpen);
  const closePicker = useCockpit((s) => s.closePicker);
  const branchId = useCockpit((s) => s.selection.branchId);
  // §6.9's ⌘⏎ rule, written once: the selected worktree, else the branch's first.
  const worktreeId = useCockpit((s) =>
    s.selection.worktreeId ??
    (branchId === null ? null : ((s.worktreesByBranch[branchId] ?? [])[0]?.id ?? null)),
  );

  return (
    <header className="top">
      <LayoutToggle layout={layout} onChange={setLayout} />
      <span className="top-spacer" />
      <button
        type="button"
        className="top-btn"
        disabled
        title="Authority needs the agent-runtime signal (docs/AGENTIC-UX.md §7). It arrives in Phase 3."
      >
        Authority
      </button>
      <button
        type="button"
        className="top-btn"
        disabled
        title="Run locally needs the deploy connector. It arrives in Phase 7."
      >
        Run locally
      </button>
      <NewSessionButton worktreeId={worktreeId} />
      <SessionPicker open={pickerOpen} worktreeId={worktreeId} onClose={closePicker} />
    </header>
  );
}
