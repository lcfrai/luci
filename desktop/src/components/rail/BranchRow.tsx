// Contract §6.2: one branch row. Click selects the branch AND sets lens "develop". Its dot is
// the highest-attention status among its sessions, and clicking the dot navigates to the first
// session carrying that status — AGENTIC-UX §10 rule 7, every aggregate names its fact.
//
// TASK 22 NOTE — this file is in task 22's modify list (shared-shape §1). Its right-click
// dispatch is already complete: every row kind goes through openContextMenu and the one
// <ContextMenu/>, so task 22's verbs land in menu/ContextMenu.tsx and NOT here.
import type { JSX } from "react";
import { useShallow } from "zustand/react/shallow";
import { useCockpit } from "../../store/store";
import { selectBranchAttentionSessionId, selectWorktrees } from "../../store/selectors";
import { StatusDot } from "../common/StatusDot";
import { WorktreeRow } from "./WorktreeRow";
import type { Branch } from "../../store/types";
import "./rail.css";

export function BranchRow(props: {
  branch: Branch;
  selected: boolean;
  expanded: boolean;
}): JSX.Element {
  const { branch, selected, expanded } = props;
  const worktrees = useCockpit(useShallow(selectWorktrees(branch.id)));
  const attentionId = useCockpit(selectBranchAttentionSessionId(branch.id));
  const attentionStatus = useCockpit((s) =>
    attentionId === null ? undefined : s.sessions[attentionId]?.status,
  );
  const selectBranch = useCockpit((s) => s.selectBranch);
  const toggleBranch = useCockpit((s) => s.toggleBranch);
  const focusSession = useCockpit((s) => s.focusSession);
  const openContextMenu = useCockpit((s) => s.openContextMenu);
  const expandedWorktrees = useCockpit((s) => s.expandedWorktrees);

  return (
    <div>
      <div
        role="treeitem"
        tabIndex={-1}
        aria-level={2}
        aria-expanded={worktrees.length > 0 ? expanded : undefined}
        aria-selected={selected}
        data-kind="branch"
        data-id={branch.id}
        className="rail-row rail-level-2"
        onClick={() => {
          selectBranch(branch.projectId, branch.id);
          // Guarded exactly as WorktreeRow guards its own toggle. A childless branch that
          // toggled anyway would accumulate expandedBranches[id] = true and render an empty
          // <div role="group"> — an expand affordance over nothing, which is the control that
          // lies that Rail.test.tsx's "draws no chevron for a worktree with nothing under it"
          // argues against. Selection still happens either way; only expansion is gated.
          if (worktrees.length > 0) toggleBranch(branch.id);
        }}
        onContextMenu={(event) => {
          event.preventDefault();
          openContextMenu({ kind: "branch", id: branch.id, x: event.clientX, y: event.clientY });
        }}
      >
        <span
          className={worktrees.length > 0 ? "rail-chev" : "rail-chev rail-chev-empty"}
          aria-hidden="true"
        >
          {expanded ? "▾" : "▸"}
        </span>
        {attentionStatus === undefined ? null : (
          <button
            type="button"
            className="rail-dot-btn"
            onClick={(event) => {
              event.stopPropagation();
              if (attentionId !== null) focusSession(attentionId);
            }}
          >
            <StatusDot
              status={attentionStatus}
              live={attentionStatus === "running"}
              label={`${attentionStatus} · ${branch.name}`}
            />
          </button>
        )}
        <span className="rail-name">{branch.name}</span>
        {branch.isHead ? <span className="rail-badge">HEAD</span> : null}
      </div>
      {expanded ? (
        <div role="group">
          {worktrees.map((worktree) => (
            <WorktreeRow
              key={worktree.id}
              worktree={worktree}
              expanded={expandedWorktrees[worktree.id] === true}
            />
          ))}
        </div>
      ) : null}
    </div>
  );
}
