import { NavigationIcon } from "../common/NavigationIcon";
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
import type { HomeProject } from "../../api/projectHome";
import "./rail.css";

export function ProjectRow(props: {
  project: Project;
  selected: boolean;
  expanded: boolean;
  activity?: HomeProject;
  onOpen?: (id: string) => void;
}): JSX.Element {
  const { project, selected, expanded, activity, onOpen } = props;
  const branches = useCockpit(useShallow(selectBranches(project.id)));
  const toggleProject = useCockpit((s) => s.toggleProject);
  const selectProject = useCockpit((s) => s.selectProject);
  const openContextMenu = useCockpit((s) => s.openContextMenu);
  const selectedBranchId = useCockpit((s) => s.selection.branchId);
  const expandedBranches = useCockpit((s) => s.expandedBranches);

  return (
    <div>
      <div
        role="treeitem"
        tabIndex={-1}
        aria-label={project.name}
        aria-describedby={activity ? `project-activity-${project.id}` : undefined}
        aria-level={1}
        aria-expanded={expanded}
        aria-selected={selected}
        data-kind="project"
        data-id={project.id}
        className="rail-row"
        onClick={() => onOpen ? onOpen(project.id) : selectProject(project.id)}
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
        <button
          type="button"
          className="rail-chev rail-project-toggle"
          tabIndex={-1}
          aria-label={`${expanded ? "Collapse" : "Expand"} ${project.name}`}
          aria-expanded={expanded}
          onClick={(event) => {
            // Disclosure is independent of navigation. In particular, selecting a cold
            // project must not move the next project when its branch list arrives.
            event.stopPropagation();
            event.currentTarget.closest<HTMLElement>('[role="treeitem"]')?.focus();
            toggleProject(project.id);
          }}
        >
          <NavigationIcon name={expanded ? "chevron-down" : "chevron-right"} />
        </button>
        <span className="rail-name">{project.name}</span>
        {activity && <span id={`project-activity-${project.id}`} className="rail-activity" aria-label={`Activity for ${project.name}`}>
          {activity.working && <span title="Working" aria-label="Working">◔</span>}
          {activity.needsInput && <span title="Needs input" aria-label="Needs input">!</span>}
          {activity.queued && <span title="Queued" aria-label="Queued">…</span>}
          {activity.stops.length > 0 && <span title="Stopped" aria-label="Stopped">■</span>}
          {activity.unknown && <span title="Some activity unconfirmed" aria-label="Some activity unconfirmed">?</span>}
        </span>}
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
