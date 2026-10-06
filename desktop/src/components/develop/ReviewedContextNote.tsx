import type { Session } from "../../store/types";

/** Describe recorded delivery without treating revision metadata as captured bytes. */
export function ReviewedContextNote({ session, className = "journey-muted" }: { session: Session; className?: string }) {
  if (!session.implementationContext) return null;
  const delivery = session.reviewedContextDelivery;
  const captured = delivery?.status === "captured" && delivery.version === 1 && /^[a-f0-9]{64}$/.test(delivery.digest ?? "");
  return <p role="note" aria-label="Agent context" className={className}>
    {captured
      ? "Reviewed inputs were captured at start. Later amendments are separate."
      : <>Context delivery is unconfirmed for this session. It may read newer project context. {session.status === "exited" ? "Start a new reviewed run to use captured inputs." : "Stop it and start a new reviewed run before continuing."}</>}
  </p>;
}
