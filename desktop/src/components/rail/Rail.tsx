// Contract §6.2: the scroll container, the role="tree" wrapper, and keyboard traversal within
// the tree. Traversal reads the DOM rather than a parallel flat list, because a second copy of
// "which rows are visible" is a second thing to keep in step with the render — and defect 10's
// whole class is an index taken against one list and applied to another.
import type { JSX, KeyboardEvent } from "react";
import { useShallow } from "zustand/react/shallow";
import { useCockpit } from "../../store/store";
import { selectDefaultMachine } from "../../store/selectors";
import { MachineChip } from "./MachineChip";
import { ProjectRow } from "./ProjectRow";
import { RailFooter } from "./RailFooter";
import { useSetup } from "../setup/SetupProvider";
import "./rail.css";

export function Rail(): JSX.Element {
  const setup = useSetup();
  const projects = useCockpit(useShallow((s) => s.projects));
  const machine = useCockpit(selectDefaultMachine);
  const expandedProjects = useCockpit((s) => s.expandedProjects);
  const selectedProjectId = useCockpit((s) => s.selection.projectId);
  const toggleProject = useCockpit((s) => s.toggleProject);
  const toggleBranch = useCockpit((s) => s.toggleBranch);
  const toggleWorktree = useCockpit((s) => s.toggleWorktree);

  const toggle = (kind: string, id: string): void => {
    if (kind === "project") toggleProject(id);
    else if (kind === "branch") toggleBranch(id);
    else if (kind === "worktree") toggleWorktree(id);
  };

  const onKeyDown = (event: KeyboardEvent<HTMLDivElement>): void => {
    // tsconfig has lib ["ES2020","DOM"] and no DOM.Iterable, so a NodeList cannot be spread.
    const rows = Array.from(
      event.currentTarget.querySelectorAll<HTMLElement>('[role="treeitem"]'),
    );
    const active = document.activeElement as HTMLElement | null;
    const index = active === null ? -1 : rows.indexOf(active);
    if (index < 0) return;
    const row = rows[index];
    const kind = row.dataset.kind ?? "";
    const id = row.dataset.id ?? "";
    const expanded = row.getAttribute("aria-expanded");
    const level = Number(row.getAttribute("aria-level") ?? "1");

    switch (event.key) {
      case "ArrowDown":
        event.preventDefault();
        rows[Math.min(index + 1, rows.length - 1)]?.focus();
        break;
      case "ArrowUp":
        event.preventDefault();
        rows[Math.max(index - 1, 0)]?.focus();
        break;
      case "ArrowRight":
        event.preventDefault();
        // Three states, not two: "false" (collapsed, so expand), "true" (expanded, so move to
        // the first child) and null (a LEAF — a session row, or a childless worktree or
        // branch, which render no aria-expanded at all). ARIA APG says ArrowRight on an end
        // node does nothing, so the null case must fall through to neither branch. Without
        // the `=== "true"` guard it reached the else and moved focus down, which is ArrowDown
        // wearing another key's clothes.
        if (expanded === "false") toggle(kind, id);
        else if (expanded === "true") rows[index + 1]?.focus();
        break;
      case "ArrowLeft":
        event.preventDefault();
        if (expanded === "true") {
          toggle(kind, id);
        } else {
          for (let i = index - 1; i >= 0; i -= 1) {
            if (Number(rows[i].getAttribute("aria-level") ?? "1") < level) {
              rows[i].focus();
              break;
            }
          }
        }
        break;
      case "Enter":
      case " ":
        event.preventDefault();
        row.click();
        break;
      default:
        break;
    }
  };

  return (
    <nav className="rail" aria-label="Projects">
      <div><div className="rail-brand"><strong>Luci</strong><button type="button" aria-label="Create project" onClick={() => setup?.openProject()}>＋</button></div>{setup?.workspace?.mode === "cloud" && <button className="rail-workspace" onClick={()=>setup.openSettings()}><strong>Cloud workspace</strong><span>{setup.workspace.host}</span></button>}<MachineChip machine={machine} /></div>
      <div className="rail-scroll" role="tree" aria-label="Projects" onKeyDown={onKeyDown}>
        {projects.length === 0 ? (
          <p className="rail-empty">
            No projects yet. Create a project above, or choose your projects folder in Settings.
          </p>
        ) : (
          projects.map((project) => (
            <ProjectRow
              key={project.id}
              project={project}
              selected={project.id === selectedProjectId}
              expanded={expandedProjects[project.id] === true}
            />
          ))
        )}
      </div>
      <RailFooter />
    </nav>
  );
}
