import { useEffect, useRef, useState } from "react";
import { coordinatorPolicyApi, coordinatorTargets, type CoordinatorPolicy as Policy, type CoordinatorTarget } from "../../api/coordinatorPolicy";
import { orchestratorApi, type OrchestratorAdvanceRequest, type OrchestratorView } from "../../api/orchestrator";
import { errorText, ProductApiError } from "../../api/product";
import { ApiError } from "../../api/client";
import { Button } from "../fluid/components/button";

const labels = { specification: "Definition", "design-plan": "Design", plan: "Implementation plan" };
type PolicyWrite = { expectedRevision: number; mode: Policy["mode"]; reviewMode: Policy["reviewMode"]; allowedTargets: CoordinatorTarget[]; rejected?: true };
type Advance = { turnId: string; actorId: string; contextDigest: string; expectedContextDigest?: string; revision: number; policyRevision: number; rejected?: true };
type Retained = { version: 1; advance?: Advance; policyWrite?: PolicyWrite };
function readRetained(key: string): { value: Retained; raw: string | null; error: string } {
  try {
    const raw = localStorage.getItem(key);
    if (!raw) return { value: { version: 1 }, raw, error: "" };
    const value = JSON.parse(raw) as Retained;
    if (raw.length > 16000 || value.version !== 1 ||
        (value.advance && (!value.advance.turnId || !value.advance.actorId || !value.advance.contextDigest || (value.advance.expectedContextDigest !== undefined && (typeof value.advance.expectedContextDigest !== "string" || !value.advance.expectedContextDigest)) || !Number.isSafeInteger(value.advance.revision) || !Number.isSafeInteger(value.advance.policyRevision) || (value.advance.rejected !== undefined && value.advance.rejected !== true))) ||
        (value.policyWrite && (!Number.isSafeInteger(value.policyWrite.expectedRevision) || !["review", "auto-drafts"].includes(value.policyWrite.mode) || !["manual", "approved-amendments"].includes(value.policyWrite.reviewMode) || !Array.isArray(value.policyWrite.allowedTargets) || value.policyWrite.allowedTargets.some(target => !coordinatorTargets.includes(target)) || (value.policyWrite.rejected !== undefined && value.policyWrite.rejected !== true)))) {
      throw new Error("The retained coordinator request cannot be read. Automatic draft saving is paused; keep this browser record for recovery.");
    }
    return { value, raw, error: "" };
  } catch (cause) { return { value: { version: 1 }, raw: null, error: errorText(cause) }; }
}
function applied(view: OrchestratorView, request: Advance) {
  return view.turns.some(turn => turn.id === request.turnId && turn.actor.id === request.actorId && turn.contextDigest === request.contextDigest && !!turn.appliedRevision);
}
// Plain ApiError 409 can mean the client discarded a response after workspace
// switching. Only a parsed server refusal can establish a rejected mutation.
const refused = (cause: unknown) => cause instanceof ProductApiError && [400, 401, 403, 409].includes(cause.status);

/** Scope-keyed by the parent; review is a separate explicit opt-in. */
export function CoordinatorPolicy({ projectId, scope, actorId, view, canRead, canEdit, blocked, onAdvance, beginObservation, policyEpoch = 0, onPolicyUpdated }: {
  projectId: string; scope: string; actorId: string; view: OrchestratorView;
  canRead: boolean; canEdit: boolean; blocked: string;
  onAdvance(turnId: string, input: OrchestratorAdvanceRequest): Promise<OrchestratorView>;
  beginObservation(): (view: OrchestratorView) => void;
  policyEpoch?: number; onPolicyUpdated?(): void;
}) {
  const key = `luci.coordinator-request.v1:${scope}`;
  const [initial] = useState(() => readRetained(key));
  const [retained, setRetained] = useState(initial.value), retainedRef = useRef(initial.value), rawRef = useRef(initial.raw);
  const [storageError, setStorageError] = useState(initial.error);
  const [policy, setPolicy] = useState<Policy | null>(null);
  const [mode, setMode] = useState<Policy["mode"]>("review"), [targets, setTargets] = useState<CoordinatorTarget[]>(coordinatorTargets);
  const [reviewMode, setReviewMode] = useState<Policy["reviewMode"]>("manual");
  const [consent, setConsent] = useState(false), [busy, setBusy] = useState("");
  const [error, setError] = useState(""), [notice, setNotice] = useState("");
  const [fresh, setFresh] = useState(false);
  const [readDenied, setReadDenied] = useState(false);
  const alive = useRef(true), epoch = useRef(0), inFlight = useRef(false);
  const attempted = useRef(new Set<string>());
  const latest = useRef({ canRead, canEdit, blocked, view, actorId, onAdvance, beginObservation });
  latest.current = { canRead, canEdit, blocked, view, actorId, onAdvance, beginObservation };
  function persist(value: Retained) {
    try {
      if (storageError || localStorage.getItem(key) !== rawRef.current) throw new Error("Another window changed the coordinator request, or browser recovery is unavailable. Automatic changes are paused; reopen this project to read the retained request.");
      const raw = value.advance || value.policyWrite ? JSON.stringify(value) : null;
      if (raw === null) localStorage.removeItem(key); else localStorage.setItem(key, raw);
      rawRef.current = raw; retainedRef.current = value; setRetained(value);
    } catch (cause) { setStorageError(errorText(cause)); throw cause; }
  }
  function acceptPolicy(value: Policy) {
    setPolicy(value); setMode(value.mode); setReviewMode(value.reviewMode); setTargets(value.allowedTargets); setConsent(false); setFresh(true); setReadDenied(false);
  }
  useEffect(() => { alive.current = true; return () => { alive.current = false; epoch.current++; }; }, []);
  useEffect(() => {
    const ticket = ++epoch.current, controller = new AbortController();
    setFresh(false); setConsent(false);
    if (!canRead) { setPolicy(null); return; }
    void coordinatorPolicyApi.get(projectId, controller.signal).then(value => {
      if (!alive.current || ticket !== epoch.current || controller.signal.aborted || !latest.current.canRead) return;
      acceptPolicy(value); setError("");
    }).catch(cause => { if (alive.current && ticket === epoch.current && !controller.signal.aborted) {
      setError(errorText(cause));
      if (cause instanceof ApiError && [401, 403].includes(cause.status)) { setPolicy(null); setReadDenied(true); }
    } });
    return () => controller.abort();
  }, [projectId, canRead, canEdit, policyEpoch]);

  async function refresh() {
    if (!latest.current.canRead || inFlight.current) return;
    const observedView = latest.current.view, publish = latest.current.beginObservation();
    inFlight.current = true; const ticket = ++epoch.current; setBusy("refresh"); setFresh(false); setConsent(false);
    try {
      const nextPolicy = await coordinatorPolicyApi.get(projectId);
      const nextView = await orchestratorApi.get(projectId);
      if (!alive.current || ticket !== epoch.current || !latest.current.canRead) return;
      const pending = retainedRef.current;
      // A policy revision beyond the submitted CAS fences any delayed original
      // write. An unchanged revision does not establish that it cannot arrive.
      const policySettled = pending.policyWrite && (pending.policyWrite.rejected || nextPolicy.revision > pending.policyWrite.expectedRevision);
      const advanceApplied = pending.advance && applied(nextView, pending.advance);
      const advanceSettled = pending.advance && (pending.advance.rejected || advanceApplied);
      if (advanceSettled && pending.advance) attempted.current.add(`${pending.advance.turnId}:${pending.advance.contextDigest}`);
      if (policySettled || advanceSettled) persist({ version: 1,
        ...(!policySettled && pending.policyWrite ? { policyWrite: pending.policyWrite } : {}),
        ...(!advanceSettled && pending.advance ? { advance: pending.advance } : {}),
      });
      acceptPolicy(nextPolicy);
      // Exact request settlement above remains valid historical evidence even
      // when another reader has advanced the parent's displayed context.
      if (latest.current.view === observedView) publish(nextView);
      setError("");
      setNotice(advanceApplied ? "The retained amendment was saved. No request was sent again." : "Current policy and conversation read. Unconfirmed requests remain paused until their outcome is established.");
    } catch (cause) { if (alive.current && ticket === epoch.current && latest.current.canRead) {
      setError(errorText(cause));
      if (cause instanceof ApiError && [401, 403].includes(cause.status)) { setPolicy(null); setReadDenied(true); }
    } }
    finally { inFlight.current = false; if (alive.current) setBusy(""); }
  }
  async function save() {
    if (!latest.current.canEdit || !policy || !fresh || inFlight.current || retainedRef.current.policyWrite || retainedRef.current.advance || storageError || !consent) return;
    const input = { expectedRevision: policy.revision, mode, reviewMode: mode === "review" ? "manual" as const : reviewMode, allowedTargets: targets };
    if (mode === "auto-drafts" && !targets.length) return;
    try { persist({ version: 1, policyWrite: input }); } catch { return; }
    inFlight.current = true; const ticket = ++epoch.current; setBusy("save"); setFresh(false); setError("");
    try {
      const value = await coordinatorPolicyApi.save(projectId, input);
      if (!alive.current || ticket !== epoch.current || !latest.current.canRead || !latest.current.canEdit) return;
      persist({ version: 1 }); acceptPolicy(value); onPolicyUpdated?.(); setNotice("Context policy saved. Build and deployment permissions have not changed.");
    } catch (cause) { if (alive.current && ticket === epoch.current && latest.current.canRead) {
      if (refused(cause)) { try { persist({ version: 1, policyWrite: { ...input, rejected: true } }); } catch { /* Retain the conservative unknown record. */ } }
      setError(`${errorText(cause)} Refresh policy and conversation to observe the saved request; it will not be resent automatically.`);
    } }
    finally { inFlight.current = false; if (alive.current) setBusy(""); }
  }

  useEffect(() => {
    if (!canRead || !canEdit || !fresh || policy?.mode !== "auto-drafts" || blocked || inFlight.current || storageError || retained.advance || retained.policyWrite) return;
    const turn = [...view.turns].reverse().find(candidate => candidate.actor.id === actorId);
    if (!turn || turn.actor.id !== actorId || turn.status !== "succeeded" || !turn.answer?.proposal || turn.answer.blocking || turn.appliedRevision ||
        view.blockingQuestions.length || !policy.allowedTargets.includes(turn.answer.proposal.target)) return;
    const identity = `${turn.id}:${turn.contextDigest}`;
    if (attempted.current.has(identity)) return;
    const request: Advance = { turnId: turn.id, actorId, contextDigest: turn.contextDigest, expectedContextDigest: view.context.digest, revision: view.revision, policyRevision: policy.revision };
    try { persist({ version: 1, advance: request }); } catch { return; }
    attempted.current.add(identity);
    inFlight.current = true; const ticket = ++epoch.current; setBusy("advance"); setError("");
    // The server alone decides whether a retained old-base team proposal can
    // be rebased compatibly. Always submit the exact CURRENT context CAS.
    void latest.current.onAdvance(turn.id, { expectedRevision: view.revision, expectedContextDigest: view.context.digest, expectedPolicyRevision: policy.revision }).then(value => {
      if (!alive.current || ticket !== epoch.current || !latest.current.canRead || !latest.current.canEdit) return;
      if (!applied(value, request)) throw new Error("The draft outcome was not confirmed in the returned conversation.");
      persist({ version: 1 }); setNotice("Luci saved the proposed context draft under your policy. Review affected approvals before implementation.");
    }).catch(cause => { if (alive.current && ticket === epoch.current && latest.current.canRead) {
      if (refused(cause)) { try { persist({ version: 1, advance: { ...request, rejected: true } }); } catch { /* Retain the conservative unknown record. */ } }
      setError(`${errorText(cause)} Automatic draft saving is paused. Refresh to read the exact outcome; nothing will be replayed.`);
    } })
      .finally(() => { inFlight.current = false; if (alive.current) setBusy(""); });
  }, [actorId, blocked, canEdit, canRead, fresh, policy, retained, storageError, view]);

  if (!canRead) return null;
  if (readDenied) return <section aria-label="Coordinator policy unavailable"><p role="alert">Current access to the coordinator policy was refused. Saved requests remain retained.</p><Button variant="secondary" disabled={!!busy} onClick={() => void refresh()}>Refresh policy and conversation</Button></section>;
  const pending = !!retained.advance || !!retained.policyWrite;
  const disabled = !canEdit || !fresh || !!busy || pending || !!storageError;
  return <details className="orchestrator-agent"><summary>Context policy · {!policy ? "Checking policy" : policy.mode === "auto-drafts" ? "Automatic saving" : "Review first"}</summary>
    <p>Luci can save completed, unambiguous proposals as drafts. Separately, you can allow routine changes to an already approved definition or design to be reviewed under this exact policy. Neither option starts a build, spends on a new model request or deploys an application.</p>
    {!canEdit && <p>Editing access is required to change this policy.</p>}
    {error && <p role="alert">{error}</p>}{storageError && <p role="alert">{storageError}</p>}{notice && <p role="status">{notice}</p>}
    {pending && <p role="status">A coordinator request is retained. Automatic changes are paused until its exact outcome is read back.</p>}
    {blocked && policy?.mode === "auto-drafts" && <p role="status">Automatic draft saving is paused: {blocked}</p>}
    <fieldset disabled={disabled}>
      <legend>Saving context proposals</legend>
      <label className="luci-field">Draft policy<select value={mode} onChange={event => { setMode(event.target.value as Policy["mode"]); setConsent(false); }}><option value="review">Review each amendment</option><option value="auto-drafts">Automatically save context drafts</option></select></label>
      {mode === "auto-drafts" && <label className="luci-field">Routine amendment review<select value={reviewMode} onChange={event => { setReviewMode(event.target.value as Policy["reviewMode"]); setConsent(false); }}><option value="manual">Review each saved draft yourself</option><option value="approved-amendments">Approve valid changes to previously approved definition and design</option></select></label>}
      {coordinatorTargets.map(target => <label className="orchestrator-ack" key={target}><input type="checkbox" checked={targets.includes(target)} onChange={event => { setTargets(coordinatorTargets.filter(value => value === target ? event.target.checked : targets.includes(value))); setConsent(false); }} />{labels[target]}</label>)}
      <label className="orchestrator-ack"><input type="checkbox" checked={consent} onChange={event => setConsent(event.target.checked)} />Apply this context policy to this project. An initial definition/design review and unresolved human questions still need me; build and deployment permissions stay unchanged.</label>
      <Button variant="secondary" disabled={!consent || (mode === "auto-drafts" && !targets.length)} onClick={() => void save()}>Save draft policy</Button>
    </fieldset>
    <Button variant="ghost" disabled={!!busy} onClick={() => void refresh()}>Refresh policy and conversation</Button>
    {busy && <p role="status">{busy === "advance" ? "Saving the exact proposed context draft…" : busy === "save" ? "Saving draft policy…" : "Reading policy and conversation…"}</p>}
  </details>;
}
