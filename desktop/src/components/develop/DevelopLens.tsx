import "./develop.css";
import type { JSX } from "react";
import { useShallow } from "zustand/react/shallow";
import { CanvasLayout } from "./CanvasLayout";
import { ColumnsLayout } from "./ColumnsLayout";
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
  const layout = useCockpit((s) => s.layout);
  const branch = useCockpit(selectSelectedBranch);
  const projectId = useCockpit((s) => s.selection.projectId);

  /* Canvas renders the pinned working set (§3.7 selectCanvasSessionIds); Columns renders every
     session on the branch (§3.7 selectPaneSessionIds). The empty case above is decided on the
     pane list, so Canvas is never reached with nothing to show, and CanvasLayout seeds an empty
     pin list from the pane order rather than rendering a bare plane.
     The ternary stays broken across lines: task 21's guard is
     `grep -cE '^[[:space:]]*<CanvasLayout'` and it counts the element only when the element
     STARTS A LINE, so collapsing this onto one line leaves that guard reading 0 against wired
     code (shared-shape rule 3). */
  return (
    <div className="develop-workspace">
      {projectId && <ProjectWork key={projectId} projectId={projectId} />}
      <div className="develop-sessions">
        {sessionIds.length === 0 ? (
          <div className="cols-empty">
            <EmptyDevelop branch={branch} />
          </div>
        ) : layout === "canvas" ? (
          <CanvasLayout sessionIds={canvasIds} />
        ) : (
          <ColumnsLayout sessionIds={sessionIds} />
        )}
      </div>
    </div>
  );
}
