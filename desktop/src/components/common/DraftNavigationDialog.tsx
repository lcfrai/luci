import { useEffect } from "react";
import { useShallow } from "zustand/react/shallow";
import { useCockpit } from "../../store/store";
import { Button } from "../fluid/components/button";
import { WorkflowDialog } from "./WorkflowDialog";

export function DraftNavigationDialog() {
  const pending = useCockpit(state => state.pendingNavigation);
  const labels = useCockpit(useShallow(state => [...new Set(Object.values(state.unsavedProjectDrafts)
    .filter(draft => draft.projectId === state.selection.projectId).map(draft => draft.label))]));
  const cancel = useCockpit(state => state.cancelNavigation);
  const proceed = useCockpit(state => state.discardDraftsAndNavigate);
  useEffect(() => { if (pending && labels.length === 0) proceed(); }, [pending, labels.length, proceed]);
  if (!pending || !labels.length) return null;
  return <WorkflowDialog title="Keep your changes?" description={`${labels.join(" and ")} has unsaved edits. Stay here to save them, or discard those edits before continuing.`} onClose={cancel}>
    <div className="workflow-actions"><Button variant="secondary" onClick={proceed}>Discard edits and continue</Button><Button onClick={cancel}>Keep editing</Button></div>
  </WorkflowDialog>;
}
