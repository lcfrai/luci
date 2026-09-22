// Contract §6.2: one project row + its chevron. Renders BranchRow children when expanded.
//
// TASK 22 NOTE — this file is in task 22's modify list (shared-shape §1). Its right-click
// dispatch is already complete: every row kind goes through openContextMenu and the one
// <ContextMenu/>, so task 22's verbs land in menu/ContextMenu.tsx and NOT here.
import type { JSX } from "react";
import { useShallow } from "zustand/react/shallow";
import { useCockpit } from "../../store/store";
import { selectBranches } from "../../store/selectors";
import { BranchRow } from "./BranchRow";
import type { Project } from "../../store/types";
import "./rail.css";

export function ProjectRow(props: {
  project: Project;
  selected: boolean;
  expanded: boolean;
}): JSX.Element {
  const { project, selected, expanded } = props;
  const branches = useCockpit(useShallow(selectBranches(project.id)));
  const toggleProject = useCockpit((s) => s.toggleProject);
  const selectProject = useCockpit((s) => s.selectProject);
  const openContextMenu = useCockpit((s) => s.openContextMenu);
  const selectedBranchId = useCockpit((s) => s.selection.branchId);
  const expandedBranches = useCockpit((s) => s.expandedBranches);

  return (
    <div className="rail-node">
      <div
        role="treeitem"
        tabIndex={-1}
        aria-level={1}
        aria-expanded={expanded}
        aria-selected={selected}
        data-kind="project"
        data-id={project.id}
        className="rail-row"
        onClick={() => {
          selectProject(project.id);
          toggleProject(project.id);
        }}
        onContextMenu={(event) => {
          event.preventDefault();
          openContextMenu({
            kind: "project",
            id: project.id,
            x: event.clientX,
            y: event.clientY,
          });
        }}
      >
        <span className="rail-chev" aria-hidden="true">
          {expanded ? "▾" : "▸"}
        </span>
        <span className="rail-name">{project.name}</span>
      </div>
      {expanded ? (
        <div role="group">
          {branches.map((branch) => (
            <BranchRow
              key={branch.id}
              branch={branch}
              selected={branch.id === selectedBranchId}
              expanded={expandedBranches[branch.id] === true}
            />
          ))}
        </div>
      ) : null}
    </div>
  );
}
