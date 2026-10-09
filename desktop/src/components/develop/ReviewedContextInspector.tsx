import { useEffect, useRef, useState } from "react";
import { api, type ReviewedContextDocument, type SessionReviewedContext } from "../../api";
import { errorText } from "../../api/product";
import { useProjectAccess } from "../../hooks/useProjectAccess";
import { useProjectDraftScope } from "../../hooks/useProjectDraft";
import type { Session } from "../../store/types";
import { DraftDocumentReview } from "../common/DraftDocumentReview";
import { WorkflowDialog } from "../common/WorkflowDialog";
import { Button } from "../fluid/components/button";
import "../common/draft-recovery.css";
import "./reviewed-context.css";

const documents: { kind: ReviewedContextDocument; label: string }[] = [
  { kind: "brief", label: "Brief" }, { kind: "definition", label: "Definition" },
  { kind: "design", label: "Design handoff" }, { kind: "plan", label: "Implementation plan" },
];
const digestPattern = /^[a-f0-9]{64}$/;

function matchesSession(value: SessionReviewedContext, session: Session, document: ReviewedContextDocument): boolean {
  const dependency = session.implementationContext?.dependencyContextDigest;
  const returnedDependency = value?.implementationContext?.dependencyContextDigest;
  return !!value && value.sessionId === session.id && value.projectId === session.projectId && value.worktreeId === session.worktreeId &&
    (value.version === 1 || value.version === 2) && value.version === session.reviewedContextDelivery?.version &&
    (value.version === 1 ? !dependency && !returnedDependency : digestPattern.test(dependency ?? '') && returnedDependency === dependency) &&
    value.digest === session.reviewedContextDelivery?.digest && digestPattern.test(value.digest) &&
    value.document?.kind === document && digestPattern.test(value.document.sha256) && Number.isSafeInteger(value.document.bytes) && value.document.bytes >= 0 &&
    !!value.document.value && typeof value.document.value === "object" && !Array.isArray(value.document.value);
}

/** Mounted only by an explicit inspection action. Each read verifies the entire retained bundle. */
export function ReviewedContextInspector({ session, scope, onClose }: { session: Session; scope: string; onClose(): void }) {
  const currentScope = useProjectDraftScope(session.projectId);
  const access = useProjectAccess(session.projectId);
  const canRead = currentScope === scope && access.canRead;
  const canRequest = canRead && !access.loading && !access.error;
  const [document, setDocument] = useState<ReviewedContextDocument>("definition");
  const [retry, setRetry] = useState(0);
  const requestKey = JSON.stringify([scope, session.projectId, session.worktreeId, session.id, session.reviewedContextDelivery?.version, session.reviewedContextDelivery?.digest, session.implementationContext?.dependencyContextDigest, document, retry]);
  const [result, setResult] = useState<{ key: string; value?: SessionReviewedContext; error?: string } | null>(null);
  const latest = useRef({ requestKey, canRead }); latest.current = { requestKey, canRead };
  const attempt = useRef({ key: "", generation: 0 });
  useEffect(() => () => { attempt.current = { key: "", generation: attempt.current.generation + 1 }; }, []);
  useEffect(() => {
    if (!canRead) {
      attempt.current = { key: "", generation: attempt.current.generation + 1 };
      setResult(null); return;
    }
    // Permission rechecks gate new reads without replacing an already viewed
    // immutable document or repeating its completed explicit request.
    if (!canRequest || attempt.current.key === requestKey) return;
    const generation = attempt.current.generation + 1;
    attempt.current = { key: requestKey, generation };
    const currentAttempt = () => attempt.current.generation === generation && latest.current.canRead && latest.current.requestKey === requestKey;
    setResult(null);
    void api.getSessionReviewedContext(session.id, document).then(value => {
      if (!currentAttempt()) return;
      if (!matchesSession(value, session, document)) {
        setResult({ key: requestKey, error: "The response does not match this session's captured context. Close this view and reopen the session before trying again." });
      } else setResult({ key: requestKey, value });
    }).catch(cause => {
      if (currentAttempt()) setResult({ key: requestKey, error: errorText(cause) });
    });
  }, [canRead, canRequest, requestKey, session.id, document]);
  const current = canRead && result?.key === requestKey ? result : null;
  const context = session.implementationContext!;
  const label = documents.find(item => item.kind === document)!.label;
  return <div onPointerDown={event => event.stopPropagation()}>
    <WorkflowDialog readAccess={{...access, canRead}} title="Reviewed context" description="The original inputs captured for this coding session. Later project amendments are separate. This view does not change the project or run any work." onClose={onClose}>
      <div className="reviewed-context-inspector">
        {canRead && <section aria-label="Captured session scope">
          <h3>{session.title}</h3>
          <p className="reviewed-context-session">Session {session.id}</p>
          <p>Brief {context.briefRevision} · Definition {context.specificationRevision} · Design {context.designPlanRevision} · Plan {context.planRevision}</p>
          <h4>Selected tasks for this session</h4>
          <ul>{context.taskIds.map((id, index) => <li key={id}>{context.taskTitles[index]} <span className="draft-record-id">{id}</span></li>)}</ul>
          <p className="journey-muted">The documents provide supporting context. Only these selected tasks were assigned; captured inputs do not prove implementation or checks passed.</p>
        </section>}
        <div className="workflow-form"><label>Captured document
          <select value={document} disabled={!canRequest} onChange={event => setDocument(event.target.value as ReviewedContextDocument)}>
            {documents.map(item => <option key={item.kind} value={item.kind}>{item.label}</option>)}
          </select>
        </label></div>
        {canRead && !canRequest && <p role="status">{access.message}</p>}
        {!canRead ? <p role="status">{currentScope !== scope ? "The account or workspace changed. Close this view and open the session in its workspace." : access.message}</p>
          : !current ? <p role="status">Loading captured {label.toLowerCase()}…</p>
            : current.error ? <div role="alert"><p>{current.error}</p><p>No current project document has been substituted.</p><Button variant="secondary" disabled={!canRequest} onClick={() => setRetry(value => value + 1)}>Retry captured document</Button></div>
              : current.value && <section aria-label={`Captured ${label.toLowerCase()}`}>
                <h3>{label} · original captured version</h3>
                <DraftDocumentReview key={requestKey} value={current.value.document.value} metadataDescription="These identify this session's original captured document. Later amendments and current approvals are separate." />
              </section>}
        <div className="workflow-actions"><Button variant="secondary" onClick={onClose}>Close reviewed context</Button></div>
      </div>
    </WorkflowDialog>
  </div>;
}
