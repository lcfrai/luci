import { useEffect, useId, useRef, useState } from "react";
import { coordinatorIntentsApi, type CoordinatorIntent, type CoordinatorIntentsView, type ResolveCoordinatorIntent } from "../../api/coordinatorIntents";
import { errorText, ProductApiError } from "../../api/product";
import { Button } from "../fluid/components/button";

export interface CoordinatorTeamIntentsProps {
  projectId: string; scope: string; actorId: string; contextDigest: string;
  canRead: boolean; canEdit: boolean; blocked?: string;
  /** Focus/navigation only; the parent retains its existing dirty-draft guard. */
  onDiscuss?: (turnId: string) => void;
  onRefreshContext?: () => void;
}
type Draft = { intentId: string; supersededBy: string; reason: string };
type Pending = { intentId: string; input: ResolveCoordinatorIntent };
type Retained = { version: 1; projectId: string; actorId: string; draft?: Draft; pending?: Pending };
const labels = { specification: "Discover context", "design-plan": "Design", plan: "Implementation plan" };
const digest = (value: unknown) => typeof value === "string" && /^[a-f0-9]{64}$/.test(value);
const id = (value: unknown) => typeof value === "string" && value.startsWith("intent-") && value.length <= 128;
function readRetained(key: string, projectId: string, actorId: string) {
  const empty: Retained = { version: 1, projectId, actorId };
  try {
    const raw = localStorage.getItem(key);
    if (raw === null) return { value: empty, raw, error: "" };
    const value = JSON.parse(raw) as Retained;
    if (raw.length > 16000 || value.version !== 1 || value.projectId !== projectId || value.actorId !== actorId ||
        (value.draft && (!id(value.draft.intentId) || typeof value.draft.supersededBy !== "string" || value.draft.supersededBy.length > 128 || typeof value.draft.reason !== "string" || value.draft.reason.length > 2000)) ||
        (value.pending && (!id(value.pending.intentId) || !value.pending.input || !id(value.pending.input.supersededBy) ||
          !Number.isSafeInteger(value.pending.input.expectedRevision) || value.pending.input.expectedRevision < 0 || !digest(value.pending.input.expectedContextDigest) ||
          typeof value.pending.input.reason !== "string" || !value.pending.input.reason.trim() || value.pending.input.reason.length > 2000))) {
      throw new Error("The retained team decision could not be read. Keep this browser record for recovery; new decisions are paused.");
    }
    return { value, raw, error: "" };
  } catch (cause) { return { value: empty, raw: null, error: errorText(cause) }; }
}
function confirms(view: CoordinatorIntentsView, pending: Pending, actorId: string) {
  return view.intents.some(intent => intent.id === pending.intentId && intent.actor.id === actorId && intent.status === "superseded" &&
    intent.supersededBy === pending.input.supersededBy && intent.resolutionReason === pending.input.reason);
}
function conflictText(reason: string) {
  return ({ "opposing-intent": "Decision needed: these requests change the same context.", "current-context-conflict": "The current context conflicts with this request.",
    "dependency-unresolved": "Waiting for a prerequisite request to be applied.", "origin-unavailable": "The original proposal cannot currently be verified." })[reason] ?? "Coordination needs attention.";
}

/** Scope changes destroy display state; unresolved mutations remain in that person's browser record. */
export function CoordinatorTeamIntents(props: CoordinatorTeamIntentsProps) {
  if (!props.canRead || !props.actorId) return null;
  return <TeamIntents key={JSON.stringify([props.scope, props.projectId, props.actorId])} {...props} />;
}

function TeamIntents({ projectId, scope, actorId, contextDigest, canRead, canEdit, blocked = "", onDiscuss, onRefreshContext }: CoordinatorTeamIntentsProps) {
  const key = `luci.team-decision.v1:${JSON.stringify([scope, projectId, actorId])}`;
  const [initial] = useState(() => readRetained(key, projectId, actorId));
  const [retained, setRetained] = useState(initial.value), retainedRef = useRef(initial.value), rawRef = useRef(initial.raw);
  const [storageError, setStorageError] = useState(initial.error);
  const [view, setView] = useState<CoordinatorIntentsView | null>(null), [fresh, setFresh] = useState(false);
  const [error, setError] = useState(""), [notice, setNotice] = useState("");
  const [busy, setBusy] = useState(false), [consent, setConsent] = useState(false);
  const alive = useRef(true), generation = useRef(0), working = useRef(false);
  const displayRead = useRef<AbortController | null>(null);
  const latest = useRef({ canRead, canEdit, blocked, contextDigest }); latest.current = { canRead, canEdit, blocked, contextDigest };
  const fieldId = useId();
  function persist(value: Retained) {
    try {
      if (storageError || localStorage.getItem(key) !== rawRef.current) throw new Error("Another window changed this team decision, or browser recovery is unavailable. Reopen the project to inspect the retained decision before changing it.");
      const raw = value.draft || value.pending ? JSON.stringify(value) : null;
      if (raw === null) localStorage.removeItem(key); else localStorage.setItem(key, raw);
      rawRef.current = raw; retainedRef.current = value; setRetained(value);
    } catch (cause) { setStorageError(errorText(cause)); throw cause; }
  }
  function accept(next: CoordinatorIntentsView) {
    setView(next); setFresh(true); setError("");
    const pending = retainedRef.current.pending;
    if (pending && confirms(next, pending, actorId)) {
      persist({ version: 1, projectId, actorId });
      setNotice("Your recorded decision is confirmed. No request was sent again; implementation and delivery retain their own checks.");
    }
  }
  useEffect(() => { alive.current = true; return () => { alive.current = false; generation.current++; displayRead.current?.abort(); }; }, []);
  useEffect(() => {
    const ticket = ++generation.current, controller = new AbortController();
    displayRead.current?.abort(); displayRead.current = controller;
    setFresh(false); setConsent(false);
    void coordinatorIntentsApi.get(projectId, controller.signal).then(next => {
      if (alive.current && ticket === generation.current && !controller.signal.aborted && latest.current.canRead) accept(next);
    }).catch(cause => { if (alive.current && ticket === generation.current && !controller.signal.aborted) setError(errorText(cause)); });
    return () => controller.abort();
  }, [projectId, canRead, canEdit, contextDigest]);

  async function refresh() {
    if (working.current || !latest.current.canRead) return;
    working.current = true; const ticket = ++generation.current; setBusy(true); setFresh(false); setConsent(false);
    displayRead.current?.abort(); const controller = new AbortController(); displayRead.current = controller;
    try {
      const next = await coordinatorIntentsApi.get(projectId, controller.signal);
      if (alive.current && ticket === generation.current && !controller.signal.aborted && latest.current.canRead) accept(next);
    } catch (cause) { if (alive.current && ticket === generation.current && !controller.signal.aborted && latest.current.canRead) setError(errorText(cause)); }
    finally { working.current = false; if (alive.current) setBusy(false); }
  }
  function updateDraft(draft?: Draft) {
    if (retainedRef.current.pending || working.current || !latest.current.canEdit || storageError) return;
    setConsent(false); setNotice("");
    try { persist({ version: 1, projectId, actorId, ...(draft ? { draft } : {}) }); } catch { /* Displayed by persist. */ }
  }
  async function resolve() {
    const draft = retainedRef.current.draft;
    if (!view || !draft || !fresh || !consent || working.current || retainedRef.current.pending || storageError ||
        !latest.current.canRead || !latest.current.canEdit || latest.current.blocked || latest.current.contextDigest !== view.contextDigest ||
        !view.allowedActions.includes("supersede-own-intent")) return;
    const intent = view.intents.find(row => row.id === draft.intentId);
    const replacement = view.intents.find(row => row.id === draft.supersededBy);
    if (intent?.actor.id !== actorId || intent.status !== "pending" || !replacement || replacement.status === "superseded" || replacement.id === intent.id || replacement.target !== intent.target || replacement.dependsOn.includes(intent.id) || !draft.reason.trim()) return;
    const pending: Pending = { intentId: intent.id, input: { expectedRevision: view.revision, expectedContextDigest: view.contextDigest, supersededBy: replacement.id, reason: draft.reason.trim() } };
    try { persist({ ...retainedRef.current, pending }); } catch { return; }
    working.current = true; const ticket = ++generation.current; setBusy(true); setFresh(false); setConsent(false); setError("");
    try {
      await coordinatorIntentsApi.resolve(projectId, actorId, intent.id, pending.input);
      if (!alive.current || ticket !== generation.current || !latest.current.canRead || !latest.current.canEdit) return;
      persist({ version: 1, projectId, actorId });
      setNotice("Your request was superseded by the selected request. This records your decision; it does not claim agreement from anyone else or completed delivery.");
      const next = await coordinatorIntentsApi.get(projectId);
      if (alive.current && ticket === generation.current && latest.current.canRead) accept(next);
    } catch (cause) {
      if (!alive.current || ticket !== generation.current || !latest.current.canRead) return;
      // Only an authoritative refusal settles a submitted mutation without a
      // receipt. Transport/schema failures keep the exact request across restart.
      if (cause instanceof ProductApiError && [400, 401, 403, 409, 422].includes(cause.status)) {
        try { persist({ version: 1, projectId, actorId, draft: retainedRef.current.draft }); } catch { /* Preserve conflict barrier. */ }
        setError(`${errorText(cause)} Refresh coordination and review your decision again.`);
      } else setError(`${errorText(cause)} Read coordination to establish the exact outcome; the request will not be sent again automatically.`);
    } finally { working.current = false; if (alive.current) setBusy(false); }
  }

  const pendingCount = view?.intents.filter(intent => intent.status === "pending").length ?? 0;
  const selected = view?.intents.find(intent => intent.id === retained.draft?.intentId);
  const current = fresh && !!view && view.contextDigest === contextDigest;
  const editable = current && canEdit && !blocked && !busy && !retained.pending && !storageError && !!view?.allowedActions.includes("supersede-own-intent");
  const candidates = view?.intents.filter(intent => intent.id !== selected?.id && intent.target === selected?.target && intent.status !== "superseded" && !intent.dependsOn.includes(selected?.id ?? "")) ?? [];
  const affects = (intent: CoordinatorIntent) => view?.conflicts.filter(conflict => conflict.intentId === intent.id || (conflict.reason === "opposing-intent" && conflict.otherIntentId === intent.id)) ?? [];
  function status(intent: CoordinatorIntent) {
    if (intent.status === "applied") return `Context saved · revision ${intent.appliedRevision}`;
    if (intent.status === "superseded") return "Superseded · original retained";
    return affects(intent).length ? "Needs coordination" : "Pending context application";
  }
  return <section className="orchestrator-agent" aria-label="Team coordination">
    <div className="orchestrator-actions"><strong>Team coordination</strong><span>{view ? `${pendingCount} pending · ${view.conflicts.length} coordination ${view.conflicts.length === 1 ? "issue" : "issues"}` : "Reading shared requests…"}</span>
      <Button variant="ghost" disabled={busy} onClick={() => void refresh()}>Refresh coordination</Button></div>
    {error && <p role="alert">{error}</p>}{storageError && <p role="alert">{storageError}</p>}{notice && <p role="status">{notice}</p>}
    {retained.pending && <p role="status">A team decision is awaiting its exact outcome. Other decisions in this browser are paused; refreshing only reads the result.</p>}
    {view && !current && <p role="status">{fresh ? "Shared context changed. Refresh the conversation before making a decision." : "Checking current coordination; decisions are paused."}{fresh && onRefreshContext && <Button variant="ghost" onClick={onRefreshContext}>Refresh conversation</Button>}</p>}
    {blocked && <p role="status">{blocked}</p>}
    {view && !view.intents.length && <p>No shared requests recorded yet. Luci records completed proposals with their author and original context.</p>}
    {view && view.intents.length > 0 && <details><summary>Requests and context history ({view.intents.length})</summary>
      <p>Compatible changes use the project’s context policy. Conflicting requests stay explicit. A saved context revision is separate from implementation, checks and delivery.</p>
      <ol className="orchestrator-turns">{view.intents.map(intent => <li key={intent.id}>
        <strong>{intent.actor.displayName}’s Luci · {labels[intent.target]}</strong><p>{status(intent)}</p><p>{intent.requestedChange}</p>
        {affects(intent).map((conflict, index) => <p key={index}>{conflictText(conflict.reason)}{conflict.paths.length ? ` Affected: ${conflict.paths.join(", ")}.` : ""}</p>)}
        <p>{intent.affectsAllTasks ? "The whole task plan may be affected." : intent.taskIds.length ? `Affected tasks: ${intent.taskIds.join(", ")}.` : "No task impact recorded."}{intent.dependsOn.length > 0 && ` Depends on ${intent.dependsOn.join(", ")}.`}</p>
        {intent.status === "superseded" && <p>Replaced by {intent.supersededBy}. {intent.resolutionReason}</p>}
        <details><summary>Version and ownership</summary><dl><dt>Request</dt><dd>{intent.id}</dd><dt>Person</dt><dd>{intent.actor.id}</dd><dt>Luci</dt><dd>{intent.luciId}</dd><dt>Original context</dt><dd style={{ overflowWrap: "anywhere" }}>{intent.baseContextDigest}</dd>{intent.acceptedContextDigest && <><dt>Accepted context</dt><dd style={{ overflowWrap: "anywhere" }}>{intent.acceptedContextDigest}</dd></>}</dl></details>
        <div className="orchestrator-actions">{onDiscuss && <Button variant="ghost" onClick={() => onDiscuss(intent.turnId)}>Open this conversation</Button>}
          {intent.actor.id === actorId && intent.status === "pending" && <Button variant="secondary" disabled={!editable || !!retained.draft} onClick={() => updateDraft({ intentId: intent.id, supersededBy: "", reason: "" })}>Resolve my request</Button>}</div>
      </li>)}</ol>
      <details><summary>Coordination event history ({view.events.length})</summary><ol>{view.events.map(event => <li key={event.id}>{event.sequence}. {event.actor.displayName}: {event.kind} {event.intentId}{event.causes.length > 0 ? `; follows ${event.causes.join(", ")}` : ""}</li>)}</ol></details>
    </details>}
    {retained.draft && canEdit && <form aria-label="Resolve my team request" onSubmit={event => { event.preventDefault(); void resolve(); }}>
      <fieldset style={{ minInlineSize: 0, border: 0, padding: 0 }} disabled={!editable || selected?.actor.id !== actorId || selected.status !== "pending"}>
        <legend>Supersede your request</legend><p>Keep your original request and follow a retained replacement. This does not approve another person’s intent on their behalf.</p>
        <p>{selected?.requestedChange ?? "The original request is not in the current projection. Refresh before continuing."}</p>
        <label className="luci-field" htmlFor={`${fieldId}-replacement`}>Replacement request<select id={`${fieldId}-replacement`} value={retained.draft.supersededBy} onChange={event => updateDraft({ ...retained.draft!, supersededBy: event.target.value })}><option value="">Choose a retained request</option>{candidates.map(intent => <option key={intent.id} value={intent.id}>{intent.actor.displayName}: {intent.requestedChange.slice(0, 120)} ({labels[intent.target]})</option>)}</select></label>
        <label className="luci-field" htmlFor={`${fieldId}-reason`}>Why this resolves your request<textarea id={`${fieldId}-reason`} maxLength={2000} value={retained.draft.reason} onChange={event => updateDraft({ ...retained.draft!, reason: event.target.value })} /></label>
        <label className="orchestrator-ack"><input type="checkbox" checked={consent} onChange={event => setConsent(event.target.checked)} />Supersede only my request with this replacement and retain my reason.</label>
        <Button type="submit" variant="secondary" disabled={!consent || !retained.draft.reason.trim() || !candidates.some(intent => intent.id === retained.draft!.supersededBy)}>Record my decision</Button>
      </fieldset>
      <Button type="button" variant="ghost" disabled={busy || !!retained.pending || !!storageError} onClick={() => updateDraft()}>Discard decision draft</Button>
      <p>Your decision draft is retained for this person and project in this browser.</p>
    </form>}
    {busy && <p role="status">Reading or recording the exact team decision…</p>}
  </section>;
}
