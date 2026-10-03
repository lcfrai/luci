// Contract §6.4: the bar. Layout toggle, Authority, Run locally, the split New-session button.
import type { JSX } from "react";
import { useCockpit } from "../../store/store";
import { LayoutToggle } from "./LayoutToggle";
import { NewSessionButton } from "./NewSessionButton";
import { SessionPicker } from "./SessionPicker";
import { NavigationIcon } from "../common/NavigationIcon";
import "./topbar.css";

export function TopBar({
  projectsExpanded,
  onToggleProjects,
}: {
  projectsExpanded?: boolean;
  onToggleProjects?: () => void;
} = {}): JSX.Element {
  const layout = useCockpit((s) => s.layout);
  const lens = useCockpit((s) => s.lens);
  const projectId = useCockpit((s) => s.selection.projectId);
  const setLens = useCockpit((s) => s.setLens);
  const setLayout = useCockpit((s) => s.setLayout);
  const pickerOpen = useCockpit((s) => s.pickerOpen);
  const closePicker = useCockpit((s) => s.closePicker);
  const branchId = useCockpit((s) => s.selection.branchId);
  // §6.9's ⌘⏎ rule, written once: the selected worktree, else the branch's first.
  const worktreeId = useCockpit(
    (s) =>
      s.selection.worktreeId ??
      (branchId === null
        ? null
        : ((s.worktreesByBranch[branchId] ?? [])[0]?.id ?? null)),
  );

  return (
    <header className="top">
      <button
        type="button"
        className="top-projects"
        aria-label="Projects"
        aria-controls="project-navigation"
        aria-expanded={projectsExpanded}
        data-overlay-trigger
        onClick={onToggleProjects}
      >
        <NavigationIcon name="projects" />
        <span>Projects</span>
      </button>
      {projectId === null ? (
        <span className="top-title">Projects</span>
      ) : lens === "develop" || lens === "files" || lens === "merge" ? (
        <>
          <nav className="top-tools" aria-label="Develop tools">
            {(
              [
                ["develop", "Sessions"],
                ["files", "Files"],
                ["merge", "Git"],
              ] as const
            ).map(([id, name]) => (
              <button
                type="button"
                key={id}
                className="top-btn"
                aria-current={lens === id ? "page" : undefined}
                onClick={() => setLens(id)}
              >
                {name}
              </button>
            ))}
          </nav>
          {lens === "develop" && (
            <LayoutToggle layout={layout} onChange={setLayout} />
          )}
        </>
      ) : (
        <span className="top-title">
          {lens === "discover"
            ? "Define"
            : lens === "design"
              ? "Design"
              : "Deploy"}
        </span>
      )}
      <span className="top-spacer" />
      {projectId !== null && lens === "develop" && (
        <NewSessionButton worktreeId={worktreeId} />
      )}
      <SessionPicker
        open={pickerOpen}
        worktreeId={worktreeId}
        onClose={closePicker}
      />
    </header>
  );
}
