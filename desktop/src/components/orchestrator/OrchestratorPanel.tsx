import { useCallback, useEffect, useId, useRef, useState } from "react";
import { orchestratorApi, type OrchestratorJob, type OrchestratorRequest, type OrchestratorTurn, type OrchestratorView } from "../../api/orchestrator";
import { errorText } from "../../api/product";
import { ApiError } from "../../api/client";
import { PROJECT_CONTEXT_CHANGED, requestProjectContextEdit, type ContextEditor, type ProjectContextIdentity } from "../../api/projectContextEvents";
import { useProjectDraftScope } from "../../hooks/useProjectDraft";
import { useProjectAccess } from "../../hooks/useProjectAccess";
import { agentConnectionIssue, configuredAgentModel, selectedAgentConnection, useAgentConnections } from "../../hooks/useAgentConnections";
import { useCockpit } from "../../store/store";
import { useSetup } from "../setup/SetupProvider";
import { AgentConnectionSelect } from "../setup/AgentConnectionSelect";
import { Button } from "../fluid/components/button";
import { AmendmentReview } from "./AmendmentReview";
import { ProjectUnderstanding } from "./ProjectUnderstanding";
import { ConversationStorage } from "./ConversationStorage";
import { readAgentPreference, writeAgentPreference, type OrchestratorAgent } from "./agentPreference";
import { finishComposerRequest, isComposerRecoveryKey, readComposerRecovery, retainComposerRequest, saveComposerDraft } from "./composerRecovery";
import "./orchestrator.css";

export const ORCHESTRATOR_OPEN = "luci:orchestrator-open";
const compactOverviewQuery = "(max-width: 1100px)";
const stageLabels = { discover: "Define", design: "Design", develop: "Develop", deploy: "Deploy" };
const pendingStatus = (status: string) => ["reserved", "queued", "running", "unknown"].includes(status);
const statuses: Record<OrchestratorTurn["status"], string> = {
  reserved: "Request reserved · outcome not yet confirmed", queued: "Waiting for the agent", running: "Luci is working",
  succeeded: "Response ready", failed: "Request failed", cancelled: "Request cancelled", unknown: "Outcome not confirmed",
};

export function OrchestratorPanel({ projectId }: { projectId: string }) {
  const scope = useProjectDraftScope(projectId);
  if (!scope) return null;
  return <ProjectConversation key={scope} projectId={projectId} scope={scope} />;
}

function ProjectConversation({ projectId, scope }: { projectId: string; scope: string }) {
  const access = useProjectAccess(projectId), setup = useSetup();
  const { canRead, canEdit, canAssist, canExecute } = access;
  const [compact, setCompact] = useState(() => window.matchMedia(compactOverviewQuery).matches);
  // The responsive default applies only until an explicit choice. Keep the
  // body mounted when collapsed so discussion, review and drafts are retained.
  const [openChoice, setOpenChoice] = useState<boolean | null>(null);
  const open = openChoice ?? !compact;
  const [view, setView] = useState<OrchestratorView | null>(null);
  const [conversationOpen, setConversationOpen] = useState(false);
  const [earlier, setEarlier] = useState<OrchestratorTurn[]>([]), [page, setPage] = useState<{ before: number; hasEarlier: boolean } | null>(null);
  const [message, setMessage] = useState(""), [replyTo, setReplyTo] = useState("");
  const [localRecovery, setLocalRecovery] = useState(() => readComposerRecovery(scope));
  const [recoveredDraft, setRecoveredDraft] = useState(localRecovery.draft);
  const savedDraft = useRef(localRecovery.draft);
  const pendingRequest = localRecovery.pending[0];
  const [reviewedRequest, setReviewedRequest] = useState("");
  const [storageConflict, setStorageConflict] = useState(false);
  const [error, setError] = useState(""), [notice, setNotice] = useState("");
  const [loading, setLoading] = useState(false), [busy, setBusy] = useState("");
  const [contextRefreshQueued, setContextRefreshQueued] = useState(false);
  const [uncertain, setUncertain] = useState(false), [pendingJob, setPendingJob] = useState<OrchestratorJob | null>(null);
  const [missingJobId, setMissingJobId] = useState("");
  const [review, setReview] = useState<{ turn: OrchestratorTurn; revision: number; digest: string } | null>(null);
  const [refreshEditor, setRefreshEditor] = useState(false);
  const [agent, setAgent] = useState<OrchestratorAgent>(() => readAgentPreference(scope, projectId));
  const { catalogue, error: connectionError } = useAgentConnections(undefined, canRead);
  const connection = selectedAgentConnection(catalogue, agent.agentConnectionId);
  const model = configuredAgentModel(connection, agent.harness) || agent.model;
  const routeIssue = agentConnectionIssue(connection, agent.harness, agent.agentConnectionId ?? catalogue?.activeConnectionId)
    || (!catalogue ? connectionError || "Loading agent connections…" : !connection && !model.trim() ? "Choose a saved connection, or enter the exact model for your own agent settings." : "");
  const sourceRevision = useCockpit(state => state.projectSourceRevisions[projectId] ?? 0);
  const lens = useCockpit(state => state.lens), setLens = useCockpit(state => state.setLens);
  const drafts = useCockpit(state => Object.values(state.unsavedProjectDrafts).filter(draft => draft.projectId === projectId).map(draft => draft.label).join(", "));
  const draftWarning = drafts ? `${drafts} has unsaved edits. Save or discard those edits in the editor before sending or applying; Luci uses saved context.` : "";
  const alive = useRef(true), generation = useRef(0), working = useRef(false), latest = useRef(view);
  const permission = useRef(access); permission.current = access;
  const messageRef = useRef(message); messageRef.current = message;
  const composer = useRef<HTMLTextAreaElement>(null), conversationEnd = useRef<HTMLDivElement>(null);
  const overviewBody = useRef<HTMLDivElement>(null);
  const contentId = useId();
  const cancelEditRequest = useRef<(() => void) | null>(null);
  latest.current = view;

  useEffect(() => {
    const media = window.matchMedia(compactOverviewQuery);
    const resize = () => {
      // Do not hide the control someone is using when the viewport narrows.
      // No focus is moved; an explicit open/close still takes precedence.
      if (media.matches && overviewBody.current?.contains(document.activeElement)) setOpenChoice(current => current ?? true);
      setCompact(media.matches);
    };
    resize();
    media.addEventListener("change", resize);
    return () => media.removeEventListener("change", resize);
  }, []);

  const remember = useCallback((value: OrchestratorView) => {
    if (!alive.current || !permission.current.canRead || value.projectId !== projectId || value.revision < (latest.current?.revision ?? -1)) return;
    latest.current = value; setView(value);
  }, [projectId]);
  const refresh = useCallback(async (explicit = false) => {
    if (!permission.current.canRead || working.current) return;
    const ticket = ++generation.current;
    if (explicit || !latest.current) setLoading(true);
    try {
      const value = await orchestratorApi.get(projectId);
      if (!alive.current || ticket !== generation.current) return;
      remember(value); setError("");
      if (explicit) setUncertain(false); // A retained request remains blocked independently until its exact receipt is resolved.
    } catch (cause) { if (alive.current && ticket === generation.current) setError(errorText(cause)); }
    finally { if (alive.current && ticket === generation.current) setLoading(false); }
  }, [projectId, remember]);

  useEffect(() => {
    alive.current = true;
    return () => { alive.current = false; generation.current++; cancelEditRequest.current?.(); };
  }, []);
  useEffect(() => {
    if (!canRead) { generation.current++; working.current = false; latest.current = null; setBusy(""); setLoading(false); setView(null); setEarlier([]); setPendingJob(null); setMissingJobId(""); setReview(null); return; }
    void refresh();
  }, [canRead, sourceRevision, lens, refresh]);
  useEffect(() => {
    const changed = (event: Event) => {
      const detail = (event as CustomEvent<ProjectContextIdentity>).detail;
      if (detail?.scope === scope && detail.projectId === projectId && permission.current.canRead) setContextRefreshQueued(true);
    };
    window.addEventListener(PROJECT_CONTEXT_CHANGED, changed);
    return () => window.removeEventListener(PROJECT_CONTEXT_CHANGED, changed);
  }, [scope, projectId]);
  useEffect(() => {
    // Never invalidate an in-flight mutation's ownership. Once it completes,
    // refresh with the normal generation fence so old reads cannot win.
    if (!contextRefreshQueued || busy || working.current) return;
    setContextRefreshQueued(false);
    void refresh(true);
  }, [contextRefreshQueued, busy, refresh]);
  useEffect(() => {
    if (!catalogue || agent.agentConnectionId !== undefined) return;
    // Resolve "active" once. A later catalogue refresh must not silently reroute a conversation.
    const next = { ...agent, agentConnectionId: catalogue.activeConnectionId || "" };
    setAgent(next); writeAgentPreference(scope, next);
  }, [catalogue, agent, scope]);
  const chooseAgent = (next: OrchestratorAgent) => { setAgent(next); writeAgentPreference(scope, next); };

  const editComposer = (nextMessage: string, nextReply: string) => {
    if (!permission.current.canAssist || pendingRequest || recoveredDraft || storageConflict) {
      setNotice("Restore or resolve your saved message before starting another one."); return;
    }
    setMessage(nextMessage); setReplyTo(nextReply);
    try {
      savedDraft.current = saveComposerDraft(scope, savedDraft.current, nextMessage, nextReply);
      const state = readComposerRecovery(scope); setLocalRecovery(state);
      if (!savedDraft.current) setRecoveredDraft(state.draft);
    } catch (cause) { setError(errorText(cause)); }
  };
  const editComposerRef = useRef(editComposer); editComposerRef.current = editComposer;
  const restoreDraft = () => {
    if (!permission.current.canAssist || pendingRequest) return;
    const state = readComposerRecovery(scope); setLocalRecovery(state);
    if (state.error || !state.draft) return;
    savedDraft.current = state.draft; setMessage(state.draft.message); setReplyTo(state.draft.replyTo);
    setRecoveredDraft(null); setError(""); setConversationOpen(true);
    setNotice("Your unsent message was restored in this browser. Nothing has been sent.");
  };
  const discardDraft = () => {
    if (!permission.current.canAssist || pendingRequest) return;
    try {
      savedDraft.current = saveComposerDraft(scope, recoveredDraft, "", "");
      const state = readComposerRecovery(scope); savedDraft.current = state.draft;
      setRecoveredDraft(state.draft); setMessage(""); setReplyTo(""); setLocalRecovery(state); setError("");
    } catch (cause) { setError(errorText(cause)); }
  };
  useEffect(() => {
    const changed = (event: StorageEvent) => {
      if (!isComposerRecoveryKey(scope, event.key)) return;
      const state = readComposerRecovery(scope); setLocalRecovery(state);
      if (JSON.stringify(state.draft) !== JSON.stringify(savedDraft.current)) {
        setRecoveredDraft(state.draft); setStorageConflict(true);
      }
    };
    window.addEventListener("storage", changed);
    return () => window.removeEventListener("storage", changed);
  }, [scope]);

  useEffect(() => {
    const receive = (event: Event) => {
      const detail = (event as CustomEvent<{ projectId?: string; message?: string }>).detail;
      if (detail?.projectId !== projectId || !permission.current.canRead) return;
      setOpenChoice(true);
      setConversationOpen(true);
      if (typeof detail.message === "string" && detail.message.trim()) {
        if (!messageRef.current.trim()) editComposerRef.current(detail.message.slice(0, 16000), "");
        else setNotice("Your unsent message is still here. Send or clear it before starting another question.");
      }
      requestAnimationFrame(() => { if (alive.current) composer.current?.focus(); });
    };
    window.addEventListener(ORCHESTRATOR_OPEN, receive);
    return () => window.removeEventListener(ORCHESTRATOR_OPEN, receive);
  }, [projectId]);

  const activeTurn = view?.turns.find(turn => pendingStatus(turn.status));
  const activeJobId = pendingJob && pendingStatus(pendingJob.status) ? pendingJob.id : activeTurn?.jobId;
  const completedUnrecorded = !!activeTurn && pendingJob?.id === activeTurn.jobId && !pendingStatus(pendingJob.status);
  const missingReservedJob = !!activeTurn && activeTurn.status === "reserved" && missingJobId === activeTurn.jobId;
  useEffect(() => {
    if (!canRead) return;
    let stopped = false, polling = false;
    const tick = async () => {
      if (stopped || polling || working.current || document.visibilityState === "hidden") return;
      polling = true;
      const ticket = generation.current;
      try {
        if (activeJobId) {
          const job = await orchestratorApi.job(projectId, activeJobId);
          if (stopped || !alive.current || !permission.current.canRead || ticket !== generation.current) return;
          setPendingJob(job); setMissingJobId("");
        }
        if (!stopped && ticket === generation.current) await refresh();
      } catch (cause) {
        if (!stopped && alive.current && ticket === generation.current) {
          setError(errorText(cause));
          if (activeJobId && activeTurn?.status === "reserved" && cause instanceof ApiError && cause.status === 404) setMissingJobId(activeJobId);
        }
      }
      finally { polling = false; }
    };
    const timer = setInterval(() => void tick(), activeJobId ? 2500 : 10000);
    return () => { stopped = true; clearInterval(timer); };
  }, [canRead, activeJobId, activeTurn?.status, projectId, refresh]);
  useEffect(() => {
    if (!refreshEditor || drafts) return;
    setRefreshEditor(false);
    useCockpit.setState(state => ({ projectSourceRevisions: { ...state.projectSourceRevisions, [projectId]: (state.projectSourceRevisions[projectId] ?? 0) + 1 } }));
  }, [refreshEditor, drafts, projectId]);

  async function dispatchRequest(input: OrchestratorRequest) {
    working.current = true; const ticket = ++generation.current; setBusy("send"); setLoading(false); setError(""); setNotice("");
    let accepted = false;
    try {
      const job = await orchestratorApi.send(projectId, input);
      // A verified acknowledgment can clear ONLY its own durable identity even
      // after navigation. It must never update another account/project's UI.
      finishComposerRequest(scope, input);
      if (!alive.current || !permission.current.canRead || ticket !== generation.current) return;
      accepted = true;
      const state = readComposerRecovery(scope); setLocalRecovery(state); savedDraft.current = state.draft;
      setPendingJob(job); setMessage(current => current.trim() === input.message ? "" : current); setReplyTo(""); setReviewedRequest(""); setRecoveredDraft(state.draft);
      const value = await orchestratorApi.get(projectId);
      if (alive.current && ticket === generation.current) { remember(value); conversationEnd.current?.scrollIntoView({ block: "nearest" }); }
    } catch (cause) {
      if (alive.current && permission.current.canRead && ticket === generation.current) {
        setError(`${errorText(cause)} ${accepted ? "Your request was accepted, but its conversation could not be refreshed. Refresh to observe the recorded outcome." : "The outcome is unconfirmed. Check the original request before sending another message."}`);
        setUncertain(true); setLocalRecovery(readComposerRecovery(scope));
      }
    } finally { if (ticket === generation.current) { working.current = false; if (alive.current) { setBusy(""); setLoading(false); } } }
  }
  async function send() {
    if (!permission.current.canAssist || !view || !message.trim() || drafts || routeIssue || busy || loading || contextRefreshQueued || working.current || activeTurn || activeJobId || uncertain || pendingRequest || recoveredDraft || storageConflict || localRecovery.error) return;
    const input: OrchestratorRequest = { expectedRevision: view.revision, expectedContextDigest: view.context.digest, message: message.trim(),
      ...(replyTo ? { replyToTurnId: replyTo } : {}), harness: agent.harness, ...(model.trim() ? { model: model.trim() } : {}), agentConnectionId: agent.agentConnectionId, idempotencyKey: crypto.randomUUID() };
    try {
      retainComposerRequest(scope, input); setLocalRecovery(readComposerRecovery(scope)); setReviewedRequest(input.idempotencyKey);
    } catch (cause) { setError(errorText(cause)); setLocalRecovery(readComposerRecovery(scope)); return; }
    await dispatchRequest(input);
  }
  async function retryOriginal() {
    if (!permission.current.canAssist || !pendingRequest || reviewedRequest !== pendingRequest.request.idempotencyKey || drafts || working.current || busy) return;
    // Never rebase on a newer revision, context, active connection or model.
    await dispatchRequest(pendingRequest.request);
  }
  async function checkOriginal() {
    if (!permission.current.canRead || !pendingRequest || working.current || busy) return;
    const input = pendingRequest.request;
    working.current = true; const ticket = ++generation.current; setBusy("resolve"); setError("");
    try {
      const receipt = await orchestratorApi.resolveRequest(projectId, input);
      if (receipt.status === "unknown") {
        if (alive.current && permission.current.canRead && ticket === generation.current) setError(receipt.message);
        return;
      }
      // 'not-sent' is a durable server fence against delayed original delivery,
      // never an inference from missing history or a generic HTTP error.
      finishComposerRequest(scope, input, receipt.status === "not-sent");
      if (!alive.current || !permission.current.canRead || ticket !== generation.current) return;
      const state = readComposerRecovery(scope); setLocalRecovery(state); savedDraft.current = state.draft;
      setReviewedRequest(""); setUncertain(false); setMessage(""); setReplyTo(""); setRecoveredDraft(state.draft);
      if (receipt.status === "accepted") {
        setPendingJob(receipt.job!); setNotice("The original request was accepted. It has not been sent again.");
      } else {
        // Return to an editable LOCAL draft only after the original key has
        // been permanently fenced. New context/routing requires a new Send.
        setNotice("The original request was not sent and can no longer start. Restore your message, review the current context and send when ready.");
      }
      const value = await orchestratorApi.get(projectId);
      if (alive.current && permission.current.canRead && ticket === generation.current) remember(value);
    } catch (cause) { if (alive.current && permission.current.canRead && ticket === generation.current) setError(errorText(cause)); }
    finally { if (ticket === generation.current) { working.current = false; if (alive.current) setBusy(""); } }
  }
  async function cancel() {
    if (!activeJobId || !canExecute || working.current) return;
    working.current = true; const ticket = ++generation.current; setBusy("cancel"); setError("");
    try {
      const job = await orchestratorApi.cancel(projectId, activeJobId);
      if (!alive.current || !permission.current.canRead || ticket !== generation.current) return;
      setPendingJob(job); const value = await orchestratorApi.get(projectId);
      if (ticket === generation.current) remember(value);
    } catch (cause) { if (alive.current && ticket === generation.current) setError(errorText(cause)); }
    finally { if (ticket === generation.current) { working.current = false; if (alive.current) setBusy(""); } }
  }
  async function reconcile() {
    if (!activeTurn?.jobId || !canEdit || working.current || !(completedUnrecorded || missingReservedJob)) return;
    working.current = true; const ticket = ++generation.current; setBusy("reconcile"); setError("");
    try {
      const job = await orchestratorApi.reconcile(projectId, activeTurn.jobId);
      if (!alive.current || !permission.current.canRead || ticket !== generation.current) return;
      setPendingJob(job); setMissingJobId(""); const value = await orchestratorApi.get(projectId);
      if (ticket === generation.current) remember(value);
    } catch (cause) { if (alive.current && ticket === generation.current) setError(errorText(cause)); }
    finally { if (ticket === generation.current) { working.current = false; if (alive.current) setBusy(""); } }
  }
  async function older() {
    if (!view || working.current) return;
    const before = page?.before ?? view.beforeSequence;
    const ticket = generation.current;
    working.current = true; setBusy("earlier"); setError("");
    try {
      const value = await orchestratorApi.get(projectId, before);
      if (!alive.current || ticket !== generation.current || !permission.current.canRead) return;
      setEarlier(current => [...new Map([...value.turns, ...current].map(turn => [turn.id, turn])).values()]);
      setPage({ before: value.beforeSequence, hasEarlier: value.hasEarlier });
    } catch (cause) { if (alive.current) setError(errorText(cause)); }
    finally { working.current = false; if (alive.current) setBusy(""); }
  }
  async function apply() {
    const current = latest.current;
    const unsaved = Object.values(useCockpit.getState().unsavedProjectDrafts).some(draft => draft.projectId === projectId);
    if (!canEdit || !review || !current || unsaved || working.current || review.revision !== current.revision || review.digest !== current.context.digest) return;
    working.current = true; const ticket = ++generation.current; setBusy("apply"); setError("");
    try {
      const value = await orchestratorApi.apply(projectId, review.turn.id, { expectedRevision: review.revision, expectedContextDigest: review.digest });
      if (!alive.current || !permission.current.canRead || ticket !== generation.current) return;
      remember(value); setReview(null); setRefreshEditor(true);
      setNotice("Amendment saved as a draft. Review the updated context and any affected approvals before implementation.");
    } catch (cause) { if (alive.current && ticket === generation.current) setError(`${errorText(cause)} Keep this proposal, refresh the conversation and review the current context before applying again.`); }
    finally { if (ticket === generation.current) { working.current = false; if (alive.current) setBusy(""); } }
  }
  const openComposer = (text?: string) => {
    setOpenChoice(true); setConversationOpen(true);
    if (text && !messageRef.current.trim()) editComposer(text.slice(0, 16000), replyTo);
    requestAnimationFrame(() => { if (alive.current) composer.current?.focus(); });
  };
  const ask = (text?: string) => { if (!pendingRequest && !recoveredDraft) editComposer(messageRef.current || text?.slice(0, 16000) || "", ""); openComposer(); };
  const edit = (editor: ContextEditor) => {
    if (!permission.current.canEdit) return;
    cancelEditRequest.current?.();
    cancelEditRequest.current = requestProjectContextEdit(scope, projectId, editor);
    setLens(editor === "definition" ? "discover" : "design");
  };
  const reply = (turnId: string, text = "") => { editComposer(text || messageRef.current, turnId); openComposer(); };
  const inspect = (runId: string) => {
    if (!permission.current.canRead) return;
    useCockpit.setState({ projectRunInspection: { scope, projectId, runId } });
    setLens("develop");
  };
  const turns = [...new Map([...earlier, ...(view?.turns ?? [])].map(turn => [turn.id, turn])).values()].sort((a, b) => a.sequence - b.sequence);
  const replyQuestion = view?.blockingQuestions.find(question => question.turnId === replyTo)?.question || turns.find(turn => turn.id === replyTo)?.answer?.question;
  const reviewBlock = draftWarning || (!canEdit ? "Editing access is required to apply an amendment." : review && view && review.revision !== view.revision ? "The conversation changed during review. Close this review, refresh and review the amendment again." : "");

  return <aside className={`orchestrator${open ? " orchestrator-open" : ""}`} aria-label="Project understanding">
    <header className="orchestrator-header"><div><h2>Project understanding</h2>{open ? <p>What we are building and what happens next.</p> : <p className="orchestrator-collapsed-stage">{canRead && view ? `${stageLabels[view.nextAction.lens]} · Saved context` : "Saved project context"}</p>}</div><Button variant="secondary" size="compact" aria-expanded={open} aria-controls={contentId} onClick={() => setOpenChoice(!open)}>{open ? "Hide project overview" : "Show project overview"}</Button></header>
    <div className="orchestrator-body" id={contentId} ref={overviewBody} hidden={!open}>
      {access.message && <p role="status">{access.message}</p>}
      {canRead && <>
        <div className="orchestrator-toolbar"><span className="orchestrator-muted">Saved shared context</span><Button variant="ghost" size="compact" disabled={!!busy || loading} onClick={() => void refresh(true)}>Refresh project understanding</Button></div>
        {loading && !view && <p role="status">Loading project understanding…</p>}
        {error && <p className="orchestrator-error" role="alert">{error}</p>}
        {notice && <p role="status">{notice}</p>}
        {(loading || contextRefreshQueued) && view && <p role="status">Updating saved project understanding…</p>}
        {view && <ProjectUnderstanding view={view} onAsk={ask} onReply={reply} onNavigate={setLens} onInspect={inspect} onEdit={edit} canEdit={canEdit} />}
        <details className="orchestrator-discussion" open={conversationOpen} onToggle={event => setConversationOpen(event.currentTarget.open)}><summary>Discuss or request a change</summary>
        <div hidden={!conversationOpen}>
        {view && <>
          <div className="orchestrator-toolbar"><h3>Luci</h3><Button variant="ghost" size="compact" disabled={!!busy || loading} onClick={() => void refresh(true)}>Refresh conversation</Button></div>
          {(page?.hasEarlier ?? view.hasEarlier) && <Button variant="secondary" size="compact" disabled={!!busy} onClick={() => void older()}>Show earlier conversation</Button>}
          {!turns.length && <p className="orchestrator-welcome">Describe your idea, ask about the current work, or discuss a change. Luci can propose an amendment for you to review.</p>}
          <ol className="orchestrator-turns" aria-label="Conversation history">{turns.map(turn => <li key={turn.id}>
            <div className="orchestrator-turn-meta"><strong>{turn.actor.displayName}</strong><time dateTime={turn.createdAt}>{new Date(turn.createdAt).toLocaleString()}</time></div>
            <p className="orchestrator-message">{turn.message}</p>
            {turn.replyToTurnId && <p className="orchestrator-muted">Reply to an earlier question</p>}
            <p className="orchestrator-muted">{statuses[turn.status]}{turn.appliedRevision ? ` · Applied as draft revision ${turn.appliedRevision}` : ""}</p>
            {turn.answer && <div className="orchestrator-answer"><strong>Luci</strong><p>{turn.answer.summary}</p>
              {turn.answer.question && <section className="orchestrator-question" aria-label="Luci question"><p><strong>{turn.answer.blocking && !turn.questionResolvedBy ? "Decision needed: " : "Question: "}</strong>{turn.answer.question}</p><p>{turn.answer.rationale}</p>
                {!!turn.answer.impact?.length && <div><strong>Why it matters</strong><ul>{turn.answer.impact.map((impact, index) => <li key={index}>{impact}</li>)}</ul></div>}
                {turn.questionResolvedBy ? <p>Answered in a later turn.</p> : <div className="orchestrator-actions">{turn.answer.options.map((option, index) => <Button key={index} variant="secondary" size="compact" disabled={!canAssist || !!busy} onClick={() => reply(turn.id, option)}>{option}</Button>)}<Button variant="ghost" size="compact" disabled={!canAssist || !!busy} onClick={() => reply(turn.id)}>Reply to this question</Button></div>}
              </section>}
              {!turn.answer.question && turn.answer.rationale && <p>{turn.answer.rationale}</p>}
              {turn.answer.proposal && <div className="orchestrator-proposal"><p>{turn.answer.proposal.summary}</p><Button variant="secondary" disabled={!!busy || !!turn.appliedRevision} onClick={() => { setReview({ turn, revision: view.revision, digest: view.context.digest }); setError(""); }}>Review amendment</Button></div>}
              {turn.answer.nextAction && <p className="orchestrator-muted">Suggested next step: {turn.answer.nextAction}</p>}
            </div>}
            {(turn.status === "unknown" || turn.status === "reserved") && <p>The request is unconfirmed. Refresh to observe it; Luci will not start a duplicate request automatically.</p>}
            <details className="orchestrator-provenance"><summary>Context used for this turn</summary><p>Definition {turn.context.specification.revision} · Design {turn.context.designPlan.revision} · Plan {turn.context.plan.revision}</p><p>{turn.context.sourceKnown ? `Source ${turn.context.sourceCommit.slice(0, 10) || "without a commit"}${turn.context.sourceDirty ? " with local changes" : ""}` : "Source state was unavailable"}</p></details>
          </li>)}</ol>
          <div ref={conversationEnd} />
          {(completedUnrecorded || missingReservedJob) && <div role="status"><p>{missingReservedJob ? "This request was reserved, but its agent job could not be found. Recover the interrupted request without starting an agent." : "The agent request finished, but its response has not been recorded in this conversation. Recover the recorded response without running the agent again."}</p><Button variant="secondary" disabled={!canEdit || !!busy} onClick={() => void reconcile()}>Recover recorded response</Button></div>}
          {!!view.blockingQuestions.length && <section className="orchestrator-question" aria-label="Unresolved decisions"><strong>Resolve before implementation</strong>{view.blockingQuestions.map(question => <div key={question.turnId}><p>{question.question}</p><p>{question.rationale}</p><Button variant="secondary" size="compact" disabled={!canAssist || !!busy} onClick={() => reply(question.turnId)}>Answer this decision</Button></div>)}</section>}
          <ConversationStorage projectId={projectId} view={view} canEdit={canEdit} busy={!!busy} onBusy={value => { working.current = value; generation.current++; setLoading(false); setBusy(value ? "history" : ""); }} onView={(value, rotated) => { remember(value); if (rotated) { setEarlier([]); setPage(null); } }} />
        </>}
        {localRecovery.error && <p role="alert">{localRecovery.error}</p>}
        {canAssist && storageConflict && <section aria-label="Message changed in another tab"><p>The saved message changed in another tab. Keep a copy of any text you need before reloading it; sending is paused to avoid repeating a request.</p><Button variant="secondary" onClick={() => { const state = readComposerRecovery(scope); setLocalRecovery(state); savedDraft.current = state.draft; setRecoveredDraft(state.draft); setMessage(""); setReplyTo(""); setReviewedRequest(""); setStorageConflict(false); setError(""); }}>Reload saved message</Button></section>}
        {canAssist && recoveredDraft && !pendingRequest && !storageConflict && <section aria-label="Unsent message recovery"><p>An unsent message is saved for this account and project in this browser. It is kept for 30 days.</p><Button variant="secondary" onClick={restoreDraft}>Restore unsent message</Button><Button variant="ghost" onClick={discardDraft}>Discard unsent message</Button></section>}
        {canRead && pendingRequest && <section aria-label="Unconfirmed message recovery"><p>The original request may already have been accepted. Sending a new copy is blocked until its outcome is checked. Nothing is retried automatically.</p>
          <Button variant="secondary" disabled={!!busy} onClick={() => void checkOriginal()}>Check original request</Button>
          {localRecovery.pending.length > 1 && <p>{localRecovery.pending.length} requests need to be checked, one at a time.</p>}
          {canAssist && (reviewedRequest !== pendingRequest.request.idempotencyKey ? <Button variant="secondary" onClick={() => { setMessage(pendingRequest.request.message); setReplyTo(pendingRequest.request.replyToTurnId ?? ""); setReviewedRequest(pendingRequest.request.idempotencyKey); setConversationOpen(true); }}>Review original request</Button> : <><p>Original agent: {pendingRequest.request.harness}{pendingRequest.request.model ? ` · ${pendingRequest.request.model}` : ""}. Retry uses its saved context and original request identity; model usage may be charged if it was never accepted.</p><Button variant="secondary" disabled={!!busy || !!drafts} onClick={() => void retryOriginal()}>Retry original request</Button></>)}
        </section>}
        <form className="orchestrator-composer" onSubmit={event => { event.preventDefault(); void send(); }}>
          <details className="orchestrator-agent"><summary>Agent and model</summary><fieldset disabled={!!busy || !canAssist || !!pendingRequest}>
            <label className="luci-field">Luci agent<select value={agent.harness} onChange={event => chooseAgent({ ...agent, harness: event.target.value as OrchestratorAgent["harness"], model: "" })}><option value="codex">Codex</option><option value="claude-code">Claude Code</option><option value="opencode">OpenCode</option></select></label>
            <AgentConnectionSelect catalogue={catalogue} value={agent.agentConnectionId} label="Luci connection" onChange={id => chooseAgent({ ...agent, agentConnectionId: id === undefined ? catalogue?.activeConnectionId || "" : id })} />
            <label className="luci-field">Luci model<input value={model} readOnly={!!configuredAgentModel(connection, agent.harness)} onChange={event => chooseAgent({ ...agent, model: event.target.value })} placeholder="Exact model, or subscription default" maxLength={512} /></label>
          </fieldset></details>
          <p className="orchestrator-muted">{connection ? `Using ${connection.name}${model ? ` · ${model}` : ""}.` : "Using your own agent settings."} Saved context and this conversation are sent to the selected provider. Model usage may be charged.</p>
          {routeIssue && <p role="status">{routeIssue}</p>}
          {connectionError && <p role="status">Agent settings could not be refreshed: {connectionError}</p>}
          {setup && <Button type="button" variant="ghost" size="compact" onClick={() => setup.openAgents()}>Agent connections</Button>}
          {draftWarning && <p role="status">{draftWarning}</p>}
          {!canAssist && canRead && <p role="status">Sending requires editing, execution and secret access. You can still read the conversation and saved context.</p>}
          {refreshEditor && drafts && <p role="status">The amendment is saved. Your newer unsaved edits remain in the editor; save or discard them to refresh its context.</p>}
          {canAssist && replyTo && <div className="orchestrator-actions"><span>{replyQuestion ? `Replying to: ${replyQuestion}` : "Replying to a project decision"}</span><Button type="button" variant="ghost" size="compact" disabled={!!pendingRequest || !canAssist} onClick={() => editComposer(message, "")}>Clear reply</Button></div>}
          <label className="luci-field">Message Luci<textarea ref={composer} rows={3} value={canAssist ? message : ""} maxLength={16000} disabled={!canAssist || busy === "send" || !!pendingRequest || !!recoveredDraft || storageConflict} onChange={event => editComposer(event.target.value, replyTo)} placeholder="What should we build, clarify or change?" /></label>
          <div className="orchestrator-actions"><Button type="submit" disabled={!canAssist || !view || !message.trim() || !!drafts || !!routeIssue || !!busy || loading || contextRefreshQueued || !!activeTurn || !!activeJobId || uncertain || !!pendingRequest || !!recoveredDraft || storageConflict || !!localRecovery.error}>{busy === "send" ? "Sending…" : "Send to Luci"}</Button>{activeJobId && !completedUnrecorded && !missingReservedJob && <Button type="button" variant="secondary" disabled={!canExecute || !!busy} onClick={() => void cancel()}>Cancel Luci request</Button>}</div>
          <p className="orchestrator-muted">Messages can propose changes. You review amendments and start coding, checks or deployment explicitly.</p>
        </form>
        </div>
        </details>
      </>}
    </div>
    {review && view && <AmendmentReview turn={review.turn} view={view} busy={busy === "apply"} blocked={reviewBlock} error={error} onClose={() => setReview(null)} onApply={() => void apply()} />}
  </aside>;
}
