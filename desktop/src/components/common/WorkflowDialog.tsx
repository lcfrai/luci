import { Activity, useRef, useState, type ReactNode } from "react";
import { useDismissable } from "../../hooks/useDismissable";
import { Dialog, DialogContent, DialogDescription, DialogHeader, DialogTitle } from "../fluid/components/dialog";
import { useWorkspaceOverlayAdmission } from "./WorkspaceOverlayScope";
import { Button } from "../fluid/components/button";
import "./workflow-dialog.css";

export interface WorkflowDialogReadAccess {
  canRead: boolean; loading: boolean; error: string;
}
type WorkflowDialogProps = {
  /** Exact owner permission state; retained canRead alone is not fresh admission. */
  readAccess?: WorkflowDialogReadAccess;
  /** Generic owner-owned close/discard advice without private draft contents. */
  unavailableContent?: ReactNode;
  /** Owner-authored generic scope guidance; never a captured private value. */
  unavailableMessage?: string;
  title: string; description: string; busy?: boolean; onClose(): void; children: ReactNode;
  getCloseFocusTarget?(openingControl: HTMLElement | null): HTMLElement | null;
};

/** Retain modal-owned drafts while disconnected workspace access suspends presentation. */
export function WorkflowDialog(props: WorkflowDialogProps) {
  const workspace = useWorkspaceOverlayAdmission();
  const read = props.readAccess;
  const freshRead = !read || read.canRead && !read.loading && !read.error;
  const [readSuspended, setReadSuspended] = useState(!workspace.available || !!read && !read.canRead);
  const denied = !workspace.available || !!read && !read.canRead;
  if (denied && !readSuspended) setReadSuspended(true);
  else if (!denied && readSuspended && freshRead) setReadSuspended(false);
  // Routine permission refreshes keep an admitted immutable review mounted.
  // Once denied, retained canRead is insufficient: exact fresh authority must
  // settle before that same review becomes visible again.
  const available = workspace.available && !denied && !readSuspended;
  const [retrying, setRetrying] = useState(false);
  const latestAvailable = useRef(available); latestAvailable.current = available;
  const [returnFocusTo] = useState<HTMLElement | null>(() =>
    typeof document !== "undefined" && document.activeElement instanceof HTMLElement ? document.activeElement : null,
  );
  // Activity also hides React portals and disconnects modal effects, while
  // retaining nested form state. CSS on the host alone cannot do either.
  // C056 reopens this scope only after current membership and fresh project /
  // session catalogues have been applied; their owners still control mounting.
  return <>
    {workspace.available && !available && <section className="workflow-unavailable" aria-label="Review access">
      <p role="status">{read?.loading ? "Checking access for this review…" : props.unavailableMessage ?? "This review is unavailable with the current project access. Restore access before continuing."}</p>
      {props.unavailableContent}
      <div className="workflow-actions">
        {workspace.retry && <Button variant="secondary" disabled={retrying || read?.loading} onClick={() => {
          setRetrying(true); void workspace.retry!().catch(() => {}).finally(() => setRetrying(false));
        }}>Retry review access</Button>}
        <Button variant="secondary" disabled={props.busy} onClick={props.onClose}>Close review</Button>
      </div>
    </section>}
    <Activity mode={available ? "visible" : "hidden"}>
      <WorkflowDialogModal {...props} returnFocusTo={returnFocusTo} mayRestoreFocus={() => latestAvailable.current} />
    </Activity>
  </>;
}

function WorkflowDialogModal({ title, description, busy = false, onClose, getCloseFocusTarget, children, returnFocusTo, mayRestoreFocus }: WorkflowDialogProps & {
  returnFocusTo: HTMLElement | null; mayRestoreFocus(): boolean;
}) {
  const panel = useDismissable<HTMLDivElement>(true, () => { if (!busy) onClose(); }, false);
  const restore = useRef(mayRestoreFocus); restore.current = mayRestoreFocus;
  return <Dialog open onOpenChange={open => { if (!open && !busy) onClose(); }}>
    <DialogContent ref={panel} size="lg" showCloseButton={false} aria-modal="true"
      onCloseAutoFocus={event => {
        event.preventDefault();
        if (!restore.current()) return;
        const target = getCloseFocusTarget ? getCloseFocusTarget(returnFocusTo) : returnFocusTo;
        if (target?.isConnected && !target.closest("[hidden], [inert], [aria-hidden='true']")) target.focus();
      }}
      onEscapeKeyDown={event => { if (busy) event.preventDefault(); }}
      onPointerDownOutside={event => { if (busy) event.preventDefault(); }}>
      <DialogHeader><DialogTitle>{title}</DialogTitle><DialogDescription>{description}</DialogDescription></DialogHeader>
      {children}
    </DialogContent>
  </Dialog>;
}
