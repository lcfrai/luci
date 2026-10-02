import { NavigationIcon } from "../common/NavigationIcon";
// Contract §6.2, with one departure argued in step 11: the chevron is drawn when
// sessionCount > 0 OR the store already holds session ids for this worktree, so a worktree
// whose sessions have all exited stays expandable and its transcripts stay reachable.
// The count itself is NEVER printed (AGENTIC-UX §10 rule 10).
//
// Task 22's right-click VERBS are not here and never were: every row kind goes through
// openContextMenu and the one <ContextMenu/>. What task 22 did add is the ready MARK below —
// the visible effect of §10.2's "Mark ready", without which the verb writes a flag with nowhere
// to show. Task 18 reserved no point for it, so it carries its own anchored guard.
import type { JSX } from "react";
import { useShallow } from "zustand/react/shallow";
import { useCockpit } from "../../store/store";
import { selectSessionIdsForWorktree } from "../../store/selectors";
import { useCreateSession } from "../../hooks/useCreateSession";
import { SessionRow } from "./SessionRow";
import type { Worktree } from "../../store/types";
import "./rail.css";

export function WorktreeRow(props: {
  worktree: Worktree;
  expanded: boolean;
}): JSX.Element {
  const { worktree, expanded } = props;
  const sessionIds = useCockpit(
    useShallow(selectSessionIdsForWorktree(worktree.id)),
  );
  const toggleWorktree = useCockpit((s) => s.toggleWorktree);
  const selectWorktree = useCockpit((s) => s.selectWorktree);
  const openContextMenu = useCockpit((s) => s.openContextMenu);
  const focusedSessionId = useCockpit((s) => s.selection.sessionId);
  const createSession = useCreateSession();

  const hasChildren = worktree.sessionCount > 0 || sessionIds.length > 0;

  return (
    <div>
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
          <NavigationIcon name={expanded ? "chevron-down" : "chevron-right"} />
        </span>
        <span className="rail-name">{worktree.name}</span>
        {/* "Mark ready" writes a flag the rail is the only place to see, and §10.2's whole
            point is that it survives a restart — a user who marks a worktree ready, quits and
            comes back must be able to tell that they did. A glyph with a title, not a count:
            AGENTIC-UX §10 rule 10 bans counts outside the queue and a tick is not one.
            `.rail-ready` deliberately has no rule in task 18's rail.css: an unstyled span
            inherits the row's type and colour, which is right for a mark that must not compete
            with the status dot. The class name exists so a later task can style it without
            re-finding the element. */}
        {worktree.ready === true ? (
          <span
            className="rail-ready"
            title="Marked ready to merge"
            aria-label="ready"
          >
            <NavigationIcon name="check" />
          </span>
        ) : null}
        <button
          type="button"
          className="rail-add"
          aria-label={`New session in ${worktree.name}`}
          onClick={(event) => {
            event.stopPropagation();
            void createSession(worktree.id);
          }}
        >
          <NavigationIcon name="add" />
        </button>
      </div>
      {expanded ? (
        <div role="group">
          {sessionIds.map((id) => (
            <SessionRow
              key={id}
              sessionId={id}
              selected={id === focusedSessionId}
            />
          ))}
        </div>
      ) : null}
    </div>
  );
}
