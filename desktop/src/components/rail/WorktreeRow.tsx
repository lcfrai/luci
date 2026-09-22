// Contract §6.2, with one departure argued in step 11: the chevron is drawn when
// sessionCount > 0 OR the store already holds session ids for this worktree, so a worktree
// whose sessions have all exited stays expandable and its transcripts stay reachable.
// The count itself is NEVER printed (AGENTIC-UX §10 rule 10).
//
// TASK 22 NOTE — this file is in task 22's modify list (shared-shape §1). Its right-click
// dispatch is already complete: every row kind goes through openContextMenu and the one
// <ContextMenu/>, so task 22's verbs land in menu/ContextMenu.tsx and NOT here.
import type { JSX } from "react";
import { useShallow } from "zustand/react/shallow";
import { useCockpit } from "../../store/store";
import { selectSessionIdsForWorktree } from "../../store/selectors";
import { useCreateSession } from "../../hooks/useCreateSession";
import { SessionRow } from "./SessionRow";
import type { Worktree } from "../../store/types";
import "./rail.css";

export function WorktreeRow(props: { worktree: Worktree; expanded: boolean }): JSX.Element {
  const { worktree, expanded } = props;
  const sessionIds = useCockpit(useShallow(selectSessionIdsForWorktree(worktree.id)));
  const toggleWorktree = useCockpit((s) => s.toggleWorktree);
  const selectWorktree = useCockpit((s) => s.selectWorktree);
  const openContextMenu = useCockpit((s) => s.openContextMenu);
  const focusedSessionId = useCockpit((s) => s.selection.sessionId);
  const createSession = useCreateSession();

  const hasChildren = worktree.sessionCount > 0 || sessionIds.length > 0;

  return (
    <div className="rail-node">
      <div
        role="treeitem"
        tabIndex={-1}
        aria-level={3}
        aria-expanded={hasChildren ? expanded : undefined}
        data-kind="worktree"
        data-id={worktree.id}
        className="rail-row rail-level-3"
        onClick={() => {
          selectWorktree(worktree.id);
          if (hasChildren) toggleWorktree(worktree.id);
        }}
        onContextMenu={(event) => {
          event.preventDefault();
          openContextMenu({
            kind: "worktree",
            id: worktree.id,
            x: event.clientX,
            y: event.clientY,
          });
        }}
      >
        <span
          className={hasChildren ? "rail-chev" : "rail-chev rail-chev-empty"}
          aria-hidden="true"
        >
          {expanded ? "▾" : "▸"}
        </span>
        <span className="rail-name">{worktree.name}</span>
        <button
          type="button"
          className="rail-add"
          aria-label={`New session in ${worktree.name}`}
          onClick={(event) => {
            event.stopPropagation();
            void createSession(worktree.id);
          }}
        >
          ＋
        </button>
      </div>
      {expanded ? (
        <div role="group">
          {sessionIds.map((id) => (
            <SessionRow key={id} sessionId={id} selected={id === focusedSessionId} />
          ))}
        </div>
      ) : null}
    </div>
  );
}
