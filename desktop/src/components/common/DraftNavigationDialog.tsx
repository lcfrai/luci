import { useEffect } from "react";
import { useShallow } from "zustand/react/shallow";
import { useCockpit } from "../../store/store";
import { Button } from "../fluid/components/button";
import { useProjectAccess } from "../../hooks/useProjectAccess";
import { WorkflowDialog } from "./WorkflowDialog";

// Compact navigation may close its rail while this guarded decision stays open.
// Resolve the destination only after the modal trap is removed; a connected but
// inert opener cannot receive focus. Keep ordinary desktop navigation unchanged.
function draftNavigationCloseTarget(openingControl: HTMLElement | null): HTMLElement | null {
  const usable = (target: HTMLElement | null): target is HTMLElement => {
    if (!target?.isConnected || target.closest('[inert], [hidden], [aria-hidden="true"]') || target.matches(':disabled')) return false;
    const style = window.getComputedStyle(target);
    return style.display !== "none" && style.visibility !== "hidden" && style.visibility !== "collapse";
  };
  if (usable(openingControl) && openingControl !== document.body) return openingControl;
  const projects = document.querySelector<HTMLButtonElement>('button[aria-controls="project-navigation"]');
  return usable(projects) ? projects : null;
}

export function DraftNavigationDialog() {
  const pending = useCockpit(state => state.pendingNavigation);
  const sourceProjectId = useCockpit(state => state.selection.projectId) ?? "";
  const readAccess = useProjectAccess(sourceProjectId);
  const labels = useCockpit(useShallow(state => [...new Set(Object.values(state.unsavedProjectDrafts)
    .filter(draft => draft.projectId === state.selection.projectId).map(draft => draft.label))]));
  const retained = useCockpit(state => Object.values(state.unsavedProjectDrafts)
    .filter(draft => draft.projectId === state.selection.projectId).every(draft => draft.retained));
  const localSetup = useCockpit(state => Object.values(state.unsavedProjectDrafts)
    .some(draft => draft.projectId === state.selection.projectId && draft.purpose === "verification-setup"));
  const cancel = useCockpit(state => state.cancelNavigation);
  const proceed = useCockpit(state => state.discardDraftsAndNavigate);
  useEffect(() => { if (pending && labels.length === 0) proceed(); }, [pending, labels.length, proceed]);
  if (!pending || !labels.length) return null;
  const actions = <div className="workflow-actions"><Button variant="secondary" onClick={() => proceed()}>Discard edits and continue</Button>{retained && <Button variant="secondary" onClick={() => proceed(true)}>Keep drafts and continue</Button>}<Button onClick={cancel}>Keep editing</Button></div>;
  return <WorkflowDialog unavailableContent={actions} readAccess={{...readAccess, canRead: !!sourceProjectId && readAccess.canRead}} title="Keep your changes?" description={`${labels.join(" and ")} has unsaved edits. Stay here to ${localSetup ? "continue editing" : "save them"}${retained ? ", keep the browser drafts for later," : ""} or discard those edits before continuing.`} onClose={cancel} getCloseFocusTarget={draftNavigationCloseTarget}>
    {actions}
  </WorkflowDialog>;
}
