// Contract §6.4: the bar. Layout toggle, Authority, Run locally, the split New-session button.
import type { JSX } from "react";
import { useCockpit } from "../../store/store";
import { LayoutToggle } from "./LayoutToggle";
import { NewSessionButton } from "./NewSessionButton";
import { SessionPicker } from "./SessionPicker";
import { NavigationIcon } from "../common/NavigationIcon";
import "./topbar.css";
import { Button } from "../fluid/components/button";
import { ORCHESTRATOR_OPEN } from "../orchestrator/OrchestratorPanel";

export function TopBar({
  projectsExpanded,
  onToggleProjects,
}: {
  projectsExpanded?: boolean;
  onToggleProjects?: () => void;
} = {}): JSX.Element {
  const layout = useCockpit((s) => s.layout);
  const lens = useCockpit((s) => s.lens);
  const surface = useCockpit(s => s.workspaceSurface);
  const showLuci = useCockpit(s => s.showLuci);
  const projectName = useCockpit(s => s.projects.find(project => project.id === s.selection.projectId)?.name ?? "");
  const authOnly = useCockpit(s => !!s.selection.sessionId && !!s.sessions[s.selection.sessionId]?.authOnly);
  const inTools = surface === "tools" || authOnly;
  const projectId = useCockpit((s) => s.selection.projectId);
  const setLens = useCockpit((s) => s.setLens);
  const openBranchManager = useCockpit(s => s.openBranchManager);
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
      <Button variant={inTools ? "ghost" : "secondary"} size="compact" aria-current={!inTools ? "page" : undefined} onClick={() => {
        showLuci();
        if (projectId) window.dispatchEvent(new CustomEvent(ORCHESTRATOR_OPEN, { detail: { projectId } }));
        else requestAnimationFrame(() => { const state = useCockpit.getState(); if (!state.selection.projectId && state.workspaceSurface === "luci") document.querySelector<HTMLElement>(".luci-start h1")?.focus(); });
      }}>Luci</Button>
      {!inTools && projectId ? <span className="top-title" title={projectName} aria-label={`Current project: ${projectName || "Selected project"}`}>{projectName || "Selected project"}</span> : projectId === null ? (
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
            ? "Discover"
            : lens === "design"
              ? "Design"
              : "Deploy"}
        </span>
      )}
      <span className="top-spacer" />
      {inTools && projectId !== null && (lens === "develop" || lens === "files" || lens === "merge") && <button type="button" className="top-btn" aria-haspopup="dialog" onClick={() => openBranchManager({ projectId, action: "switch" })}>Branches</button>}
      {inTools && projectId !== null && lens === "develop" && (
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
