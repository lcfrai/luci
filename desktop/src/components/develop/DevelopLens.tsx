import "./develop.css";
import type { JSX } from "react";
import { useShallow } from "zustand/react/shallow";
import { DevelopWorkspace } from "./DevelopWorkspace";
import { EmptyDevelop } from "./EmptyDevelop";
import { ProjectWork } from "../journey/ProjectWork";
import {
  selectCanvasSessionIds,
  selectPaneSessionIds,
  selectSelectedBranch,
} from "../../store/selectors";
import { useCockpit } from "../../store/store";

export function DevelopLens(): JSX.Element {
  // useShallow is mandatory here: this selector builds a new array every call (§3.7). Without
  // it, zustand 5's Object.is sees a fresh array on every 2 s poll and re-renders the whole
  // lens thirty times a minute — invisible until ten panes are open. selectSelectedBranch
  // needs none: it returns a stored object.
  const sessionIds = useCockpit(useShallow(selectPaneSessionIds));
  const canvasIds = useCockpit(useShallow(selectCanvasSessionIds));
  const branch = useCockpit(selectSelectedBranch);
  const projectId = useCockpit((s) => s.selection.projectId);

  return (
    <div className={`develop-workspace${sessionIds.length === 0 ? " develop-workspace--planning" : ""}`}>
      {projectId && <ProjectWork key={projectId} projectId={projectId} expanded={sessionIds.length === 0} />}
      <div className="develop-sessions">
        {sessionIds.length === 0 ? (
          <div className="cols-empty">
            <EmptyDevelop branch={branch} />
          </div>
        ) : (
          <DevelopWorkspace projectId={projectId} sessionIds={sessionIds} canvasIds={canvasIds} />
        )}
      </div>
    </div>
  );
}
