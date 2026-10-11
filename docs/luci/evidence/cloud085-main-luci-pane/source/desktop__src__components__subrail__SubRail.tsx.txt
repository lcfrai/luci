// Contract §6.3. Six rows, none disabled, none numbered, none marked complete —
// docs/AGENTIC-UX.md §4 rule 12 and §9 item 1.
import type { JSX } from "react";
import { selectSelectedBranch } from "../../store/selectors";
import { useCockpit } from "../../store/store";
import type { Lens } from "../../store/types";
import {
  NavigationIcon,
  type NavigationIconName,
} from "../common/NavigationIcon";
import "./subrail.css";

export const LENSES: ReadonlyArray<{
  id: Lens;
  label: string;
  icon: NavigationIconName;
}> = [
  { id: "discover", label: "Discover", icon: "discover" },
  { id: "design", label: "Design", icon: "design" },
  { id: "develop", label: "Develop", icon: "develop" },
  { id: "deploy", label: "Deploy", icon: "deploy" },
];

export function SubRail({ inert }: { inert?: boolean } = {}): JSX.Element {
  const lens = useCockpit((s) => s.lens);
  const inTools = useCockpit(s => s.workspaceSurface === "tools");
  const setLens = useCockpit((s) => s.setLens);
  const projectId = useCockpit((s) => s.selection.projectId);
  const selectProject = useCockpit((s) => s.selectProject);
  // Shared-shape §4: a derived read is a selector, never an inline computation, so that two
  // components cannot derive the same thing differently. Task 20 added `selectSelectedBranch`
  // for exactly this derivation and this line was the second copy of it.
  const branchName = useCockpit(selectSelectedBranch)?.name ?? "";
  const projectName = useCockpit(
    (s) => s.projects.find((p) => p.id === projectId)?.name ?? "",
  );

  return (
    <aside className="sub" aria-label="Phases" inert={inert}>
      <div className="sub-head">
        <button
          type="button"
          className="sub-back"
          title={projectName}
          onClick={() => selectProject(projectId)}
        >
          <NavigationIcon name="back" />
          <span>{projectName}</span>
        </button>
        <span className="sub-branch">{branchName}</span>
      </div>
      <nav className="sub-list" aria-label="Project journey">
        {LENSES.map((entry) => (
          <button
            key={entry.id}
            type="button"
            className="sub-row"
            aria-current={
              inTools && (lens === entry.id ||
              (entry.id === "develop" && (lens === "files" || lens === "merge")))
                ? "page"
                : undefined
            }
            onClick={() => setLens(entry.id)}
          >
            <span className="sub-glyph" aria-hidden="true">
              <NavigationIcon name={entry.icon} />
            </span>
            {entry.label}
          </button>
        ))}
      </nav>
    </aside>
  );
}
