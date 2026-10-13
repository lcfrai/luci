import { useEffect, useRef, useState } from "react";
import { definitionApi } from "../../api/definition";
import { designPlanApi } from "../../api/designPlan";
import type { OrchestratorView } from "../../api/orchestrator";
import { ProductApiError, errorText } from "../../api/product";
import { Button } from "../fluid/components/button";

type ReviewTarget = "specification" | "design-plan";
type Pending = { version: 1; projectId: string; actorId: string; target: ReviewTarget; revision: number; briefRevision: number; rejected?: true };
function readPending(key: string, projectId: string, actorId: string): { pending: Pending | null; raw: string | null; error: string } {
  try {
    const raw = localStorage.getItem(key);
    if (raw === null) return { pending: null, raw, error: "" };
    const pending = JSON.parse(raw) as Pending;
    if (raw.length > 2048 || pending.version !== 1 || pending.projectId !== projectId || pending.actorId !== actorId ||
        !["specification", "design-plan"].includes(pending.target) || !Number.isSafeInteger(pending.revision) || pending.revision < 1 ||
        !Number.isSafeInteger(pending.briefRevision) || pending.briefRevision < 0 || (pending.rejected !== undefined && pending.rejected !== true)) {
      throw new Error("The retained context review cannot be verified. Preserve this browser record for recovery.");
    }
    return { pending, raw, error: "" };
  } catch (cause) { return { pending: null, raw: null, error: errorText(cause) }; }
}

export function CoordinatorContextReview({ projectId, scope, actorId, view, canRead, canEdit, blocked, onReviewed }: {
  projectId: string; scope: string; actorId: string; view: OrchestratorView;
  canRead: boolean; canEdit: boolean; blocked: string; onReviewed(): void;
}) {
  if (!canRead || !actorId) return null;
  return <ContextReview key={JSON.stringify([scope, projectId, actorId])} projectId={projectId} scope={scope} actorId={actorId}
    view={view} canEdit={canEdit} blocked={blocked} onReviewed={onReviewed} />;
}

function ContextReview({ projectId, scope, actorId, view, canEdit, blocked, onReviewed }: {
  projectId: string; scope: string; actorId: string; view: OrchestratorView;
  canEdit: boolean; blocked: string; onReviewed(): void;
}) {
  const key = `luci.context-review.v1:${JSON.stringify([scope, projectId, actorId])}`;
  const [initial] = useState(() => readPending(key, projectId, actorId));
  const [pending, setPending] = useState(initial.pending), [raw, setRaw] = useState(initial.raw), [storageError, setStorageError] = useState(initial.error);
  const [busy, setBusy] = useState(false), [consent, setConsent] = useState(false), [error, setError] = useState(""), [notice, setNotice] = useState("");
  const rawRef = useRef(raw), alive = useRef(true), editable = useRef(canEdit);
  editable.current = canEdit;
  useEffect(() => { alive.current = true; return () => { alive.current = false; }; }, []);
  const target: ReviewTarget | null = view.context.specification.status === "draft" ? "specification" : view.context.designPlan.status === "draft" ? "design-plan" : null;
  if (!target && !pending && !storageError) return null;
  const label = target === "specification" ? "definition" : "design";
  const selected = target === "specification" ? view.context.specification : view.context.designPlan;
  function persist(value: Pending | null) {
    try {
      if (storageError || localStorage.getItem(key) !== rawRef.current) throw new Error("Another window changed this review request. Reopen the project and read its exact outcome.");
      const next = value ? JSON.stringify(value) : null;
      if (next === null) localStorage.removeItem(key); else localStorage.setItem(key, next);
      rawRef.current = next; setRaw(next); setPending(value);
    } catch (cause) { setStorageError(errorText(cause)); throw cause; }
  }
  async function approve() {
    if (!canEdit || !target || blocked || !consent || busy || pending || storageError || selected.status !== "draft") return;
    const request: Pending = { version: 1, projectId, actorId, target, revision: selected.revision, briefRevision: view.context.brief.revision };
    try { persist(request); } catch { return; }
    setBusy(true); setError(""); setNotice(""); setConsent(false);
    try {
      const result = target === "specification"
        ? await definitionApi.approve(projectId, request.revision, request.briefRevision)
        : await designPlanApi.approve(projectId, request.revision);
      if (!alive.current || !editable.current) return; // Retain the exact request for later authorized readback.
      if (result.projectId !== projectId || result.status !== "approved" || result.revision !== request.revision + 1 || !/^[a-f0-9]{64}$/.test(result.digest)) {
        throw new Error("The returned approval did not match the exact requested revision. Read its saved outcome before another review.");
      }
      persist(null); setNotice("The exact context review is recorded. Luci can continue affected work under its separate execution policy."); onReviewed();
    } catch (cause) {
      if (!alive.current) return;
      if (cause instanceof ProductApiError && [400, 401, 403, 409, 422].includes(cause.status)) {
        try { persist({ ...request, rejected: true }); } catch { /* Keep the conservative retained request. */ }
      }
      setError(`${errorText(cause)} Read the saved review outcome before submitting another approval.`);
    } finally { if (alive.current) setBusy(false); }
  }
  async function readOutcome() {
    if (!pending || busy) return;
    setBusy(true); setError("");
    try {
      const current = pending.target === "specification" ? await definitionApi.get(projectId) : await designPlanApi.get(projectId);
      if (!alive.current) return;
      if (current.projectId !== projectId) throw new Error("The returned review belongs to another project.");
      if (current.status === "approved" && current.revision === pending.revision + 1 && /^[a-f0-9]{64}$/.test(current.digest)) {
        persist(null); setNotice("The retained review was confirmed from saved context. No approval was sent again."); onReviewed();
      } else if (pending.rejected && current.revision === pending.revision && current.status === "draft") {
        persist(null); setNotice("The server refused that exact review. Inspect the current draft before deciding again."); onReviewed();
      } else {
        setNotice("This approval is still uncertain or context changed. The retained request stays blocked; inspect saved context before any new review.");
      }
    } catch (cause) { if (alive.current) setError(errorText(cause)); }
    finally { if (alive.current) setBusy(false); }
  }
  return <section className="orchestrator-agent" aria-label="Review conversation context">
    <strong>Review the current {label}</strong>
    <p>Approve this saved revision here after checking its content and open questions. Approval is separate from the model proposal, coding, checks and deployment.</p>
    {target && <details><summary>Saved {label} · revision {selected.revision}</summary>
      <p>{target === "specification" ? view.context.specification.purpose : view.context.designPlan.implementationNotes}</p>
      {target === "specification" && <><p>{view.context.specification.requirements.length} requirements · {view.context.specification.questions.filter(item => !item.answer.trim()).length} unanswered questions</p><ul>{view.context.specification.requirements.map(item => <li key={item.id}>{item.title}: {item.acceptance}</li>)}</ul></>}
      {target === "design-plan" && <ul>{view.context.designPlan.screens.map(item => <li key={item.id}>{item.name}: {item.purpose}</li>)}</ul>}
    </details>}
    {error && <p role="alert">{error}</p>}{storageError && <p role="alert">{storageError}</p>}{notice && <p role="status">{notice}</p>}
    {blocked && <p role="status">{blocked}</p>}
    {pending ? <><p role="status">An exact review request is retained. It will not be sent again without its outcome.</p><Button variant="secondary" disabled={busy} onClick={() => void readOutcome()}>Read saved review outcome</Button></>
      : target && <><label className="orchestrator-ack"><input type="checkbox" checked={consent} disabled={!canEdit || !!blocked || busy || !!storageError} onChange={event => setConsent(event.target.checked)} />I reviewed this saved {label} revision and its open decisions.</label>
        <Button variant="secondary" disabled={!canEdit || !!blocked || busy || !!storageError || !consent} onClick={() => void approve()}>Approve saved {label}</Button></>}
  </section>;
}
