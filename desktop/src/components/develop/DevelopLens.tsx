import "./develop.css";
import { useCallback, useRef, useState, type JSX } from "react";
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
import { useProjectDraftScope } from "../../hooks/useProjectDraft";

export function DevelopLens(): JSX.Element {
  // useShallow is mandatory here: this selector builds a new array every call (§3.7). Without
  // it, zustand 5's Object.is sees a fresh array on every 2 s poll and re-renders the whole
  // lens thirty times a minute — invisible until ten panes are open. selectSelectedBranch
  // needs none: it returns a stored object.
  const sessionIds = useCockpit(useShallow(selectPaneSessionIds));
  const canvasIds = useCockpit(useShallow(selectCanvasSessionIds));
  const branch = useCockpit(selectSelectedBranch);
  const projectId = useCockpit((s) => s.selection.projectId);
  const branchId = useCockpit((s) => s.selection.branchId);
  const scope = useProjectDraftScope(projectId ?? "");
  const identity = JSON.stringify([scope, projectId, branchId]);
  const currentIdentity = useRef(identity); currentIdentity.current = identity;
  const [chrome, setChrome] = useState({ identity, focused: false, reveal: 0 });
  // Layout changes must not remount ProjectWork or lose its unsaved plan. A
  // callback from a previous branch/account cannot hide this scope's controls.
  const onFocusModeChange = useCallback((focused: boolean) => {
    if (currentIdentity.current !== identity) return;
    setChrome(previous => previous.identity === identity && previous.focused === focused
      ? previous : { identity, focused, reveal: previous.identity === identity ? previous.reveal : 0 });
  }, [identity]);
  const revealProjectControls = useCallback(() => {
    if (currentIdentity.current !== identity) return;
    setChrome(previous => ({ identity, focused: false, reveal: (previous.identity === identity ? previous.reveal : 0) + 1 }));
  }, [identity]);
  const controlsHidden = sessionIds.length > 0 && chrome.identity === identity && chrome.focused;
  const revealRequest = chrome.identity === identity ? chrome.reveal : 0;

  return (
    <div className={`develop-workspace${sessionIds.length === 0 ? " develop-workspace--planning" : ""}`}>
      {projectId && <ProjectWork key={projectId} projectId={projectId} expanded={sessionIds.length === 0}
        controlsHidden={controlsHidden} onReveal={revealProjectControls} />}
      <div className="develop-sessions">
        {sessionIds.length === 0 ? (
          <div className="cols-empty">
            <EmptyDevelop branch={branch} />
          </div>
        ) : (
          <DevelopWorkspace projectId={projectId} sessionIds={sessionIds} canvasIds={canvasIds}
            onFocusModeChange={onFocusModeChange} revealRequest={revealRequest} />
        )}
      </div>
    </div>
  );
}
