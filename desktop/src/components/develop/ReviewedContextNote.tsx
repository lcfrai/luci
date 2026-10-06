import { useEffect, useState } from "react";
import { useProjectDraftScope } from "../../hooks/useProjectDraft";
import type { Session } from "../../store/types";
import { Button } from "../fluid/components/button";
import { ReviewedContextInspector } from "./ReviewedContextInspector";
import "./reviewed-context.css";

/** Describe recorded delivery without treating revision metadata as captured bytes. */
export function ReviewedContextNote({ session, canInspect, readPending = false, className = "journey-muted" }: { session: Session; canInspect: boolean; readPending?: boolean; className?: string }) {
  const scope = useProjectDraftScope(session.projectId);
  if (!session.implementationContext) return null;
  const identity = JSON.stringify([scope, session.projectId, session.worktreeId, session.id, session.reviewedContextDelivery]);
  return <ScopedContextNote key={identity} session={session} scope={scope} canInspect={canInspect} readPending={readPending} className={className} />;
}

function ScopedContextNote({ session, scope, canInspect, readPending, className }: { session: Session; scope: string | null; canInspect: boolean; readPending: boolean; className: string }) {
  const [open, setOpen] = useState(false);
  useEffect(() => { if (!canInspect) setOpen(false); }, [canInspect]);
  const delivery = session.reviewedContextDelivery;
  const captured = delivery?.status === "captured" && delivery.version === 1 && /^[a-f0-9]{64}$/.test(delivery.digest ?? "");
  return <div className={`reviewed-context-note ${className}`}><p role="note" aria-label="Agent context">
    {captured
      ? "Reviewed inputs were captured at start. Later amendments are separate."
      : "Context delivery is unconfirmed for this session; no verified captured copy is recorded. Its agent may read newer project context. A new reviewed run is needed to use captured inputs."}
  </p>{captured && <Button type="button" variant="secondary" disabled={!canInspect || readPending || !scope} onClick={() => setOpen(true)}>View reviewed context</Button>}
    {open && canInspect && scope && <ReviewedContextInspector session={session} scope={scope} onClose={() => setOpen(false)} />}
  </div>;
}
