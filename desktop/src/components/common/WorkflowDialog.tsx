import { useState, type ReactNode } from "react";
import { useDismissable } from "../../hooks/useDismissable";
import { Dialog, DialogContent, DialogDescription, DialogHeader, DialogTitle } from "../fluid/components/dialog";
import "./workflow-dialog.css";

/** Fluid's Radix modal supplies focus trapping; the shared overlay stack owns app shortcuts. */
export function WorkflowDialog({ title, description, busy = false, onClose, children }: {
  title: string; description: string; busy?: boolean; onClose(): void; children: ReactNode;
}) {
  const [returnFocusTo] = useState<HTMLElement | null>(() =>
    typeof document !== "undefined" && document.activeElement instanceof HTMLElement ? document.activeElement : null,
  );
  const panel = useDismissable<HTMLDivElement>(true, () => { if (!busy) onClose(); });
  return <Dialog open onOpenChange={open => { if (!open && !busy) onClose(); }}>
    <DialogContent ref={panel} size="lg" showCloseButton={false} aria-modal="true"
      onCloseAutoFocus={event => {
        // This controlled dialog has no Radix Trigger. Restore the opening
        // control after its focus trap is removed, rather than during cleanup.
        event.preventDefault();
        if (returnFocusTo?.isConnected) returnFocusTo.focus();
      }}
      onEscapeKeyDown={event => { if (busy) event.preventDefault(); }}
      onPointerDownOutside={event => { if (busy) event.preventDefault(); }}>
      <DialogHeader><DialogTitle>{title}</DialogTitle><DialogDescription>{description}</DialogDescription></DialogHeader>
      {children}
    </DialogContent>
  </Dialog>;
}
