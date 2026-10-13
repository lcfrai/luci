import { useCallback, useEffect, useId, useRef, useState } from "react";
import { orchestratorApi, type OrchestratorAdvanceRequest, type OrchestratorJob, type OrchestratorRequest, type OrchestratorTurn, type OrchestratorView } from "../../api/orchestrator";
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
import { WorkflowDialog } from "../common/WorkflowDialog";
import { AmendmentReview } from "./AmendmentReview";
import { ProjectUnderstanding } from "./ProjectUnderstanding";
import { ConversationStorage } from "./ConversationStorage";
import { CoordinatorPolicy } from "./CoordinatorPolicy";
import { CoordinatorExecution } from "./CoordinatorExecution";
import { CoordinatorTeamIntents } from "./CoordinatorTeamIntents";
import { CoordinatorContextReview } from "./CoordinatorContextReview";
import { useAuth } from "../auth/AuthProvider";
import { readAgentPreference, writeAgentPreference, type OrchestratorAgent } from "./agentPreference";
import { finishComposerRequest, isComposerRecoveryKey, readComposerRecovery, retainComposerRequest, saveComposerDraft } from "./composerRecovery";
import "./orchestrator.css";

export const ORCHESTRATOR_OPEN = "luci:orchestrator-open";
export interface OrchestratorOpenDetail {
  projectId: string; message?: string;
  scope?: string; destination?: "decisions"; turnId?: string;
}
const compactOverviewQuery = "(max-width: 1100px)";
const stageLabels = { discover: "Discover", design: "Design", develop: "Develop", deploy: "Deploy" };
const amendmentEditors = { specification: "definition", "design-plan": "design", plan: "plan" } as const;
const amendmentLabels = { specification: "definition", "design-plan": "design", plan: "implementation plan" };
const pendingStatus = (status: string) => ["reserved", "queued", "running", "unknown"].includes(status);
const readAccessDenied = (cause: unknown) => cause instanceof ApiError && (cause.status === 401 || cause.status === 403);
const statuses: Record<OrchestratorTurn["status"], string> = {
  reserved: "Request reserved · outcome not yet confirmed", queued: "Waiting for the agent", running: "Luci is working",
  succeeded: "Response ready", failed: "Request failed", cancelled: "Request cancelled", unknown: "Outcome not confirmed",
};

export function OrchestratorPanel({ projectId, primary = false }: { projectId: string; primary?: boolean }) {
  const scope = useProjectDraftScope(projectId);
  if (!scope) return null;
  return <ProjectConversation key={scope} projectId={projectId} scope={scope} primary={primary} />;
}

function ProjectConversation({ projectId, scope, primary }: { projectId: string; scope: string; primary: boolean }) {
  const access = useProjectAccess(projectId), setup = useSetup();
  const actorId = useAuth()?.profile.user.id ?? "";
  const [policyEpoch, setPolicyEpoch] = useState(0);
  const { canRead, canEdit, canAssist, canExecute } = access;
  const [compact, setCompact] = useState(() => window.matchMedia(compactOverviewQuery).matches);
  // The responsive default applies only until an explicit choice. Keep the
  // body mounted when collapsed so discussion, review and drafts are retained.
  const [openChoice, setOpenChoice] = useState<boolean | null>(null);
  const open = primary || (openChoice ?? !compact);
  const [view, setView] = useState<OrchestratorView | null>(null);
  // Defer the hidden discussion until it is requested. Once opened, preserve
  // its controls and focus targets through closing within this project scope.
  // Reads, request observation and draft recovery remain owned by this parent.
  const [conversation, setConversation] = useState({ open: primary, mounted: primary });
  const conversationOpen = primary || conversation.open;
  const setConversationOpen = useCallback((open: boolean) => {
    setConversation(current => current.open === open ? current : { open, mounted: current.mounted || open });
  }, []);
  useEffect(() => { if (primary) setConversationOpen(true); }, [primary, setConversationOpen]);
  const [earlier, setEarlier] = useState<OrchestratorTurn[]>([]), [page, setPage] = useState<{ before: number; hasEarlier: boolean } | null>(null);
  const [message, setMessage] = useState(""), [replyTo, setReplyTo] = useState("");
  const [suggestedReply, setSuggestedReply] = useState<{ turnId: string; question: string; answer: string; message: string; replyTo: string; previousQuestion: string } | null>(null);
  const choiceFocus = useRef<{ choice: NonNullable<typeof suggestedReply>; message: string; replyTo: string; composer: boolean } | null>(null);
  const [localRecovery, setLocalRecovery] = useState(() => readComposerRecovery(scope));
  const [recoveredDraft, setRecoveredDraft] = useState(localRecovery.draft);
  const savedDraft = useRef(localRecovery.draft);
  const pendingRequest = localRecovery.pending[0];
  const [reviewedRequest, setReviewedRequest] = useState("");
  const [storageConflict, setStorageConflict] = useState(false);
  const [error, setError] = useState(""), [notice, setNotice] = useState("");
  const [decisionNotice, setDecisionNotice] = useState("");
  const [decisionFocus, setDecisionFocus] = useState<{ ticket: number; turnId?: string } | null>(null);
  const [loading, setLoading] = useState(false), [busy, setBusy] = useState("");
  const [contextRefreshQueued, setContextRefreshQueued] = useState(false);
  const [contextStale, setContextStale] = useState(false);
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
  const projectName = useCockpit(state => state.projects.find(project => project.id === projectId)?.name ?? "Selected project");
  const drafts = useCockpit(state => Object.values(state.unsavedProjectDrafts).filter(draft => draft.projectId === projectId).map(draft => draft.label).join(", "));
  const contextDrafts = useCockpit(state => Object.values(state.unsavedProjectDrafts).filter(draft => draft.projectId === projectId && draft.purpose !== "verification-setup" && draft.purpose !== "coordinator-setup").map(draft => draft.label).join(", "));
  const checkSetupDraft = useCockpit(state => Object.values(state.unsavedProjectDrafts).some(draft => draft.projectId === projectId && (draft.purpose === "verification-setup" || draft.purpose === "coordinator-setup")));
  const otherCheckSetup = useCockpit(state => Object.values(state.unsavedProjectDrafts).some(draft => draft.projectId === projectId && draft.purpose === "verification-setup"));
  const checkSetupRetained = useCockpit(state => Object.values(state.unsavedProjectDrafts).filter(draft => draft.projectId === projectId && (draft.purpose === "verification-setup" || draft.purpose === "coordinator-setup")).every(draft => draft.retained));
  const draftWarning = contextDrafts ? `${contextDrafts} has unsaved edits. Save or discard those edits in the editor before sending or applying; Luci uses saved context.` : "";
  const setupChangeWarning = checkSetupDraft ? checkSetupRetained
    ? "Keep your verification setup for later by changing phase and choosing Keep drafts and continue, or discard the setup before applying this amendment."
    : "Your verification setup is not retained in this browser. Copy the command text you need, then discard the setup before applying this amendment." : "";
  const alive = useRef(true), generation = useRef(0), viewRead = useRef(0), working = useRef(false), latest = useRef(view);
  // Only display refreshes own this signal. Sends, jobs and post-write readback
  // keep their existing durable request and observation lifecycles.
  const refreshController = useRef<AbortController | null>(null);
  const abortRefresh = useCallback(() => {
    const controller = refreshController.current; refreshController.current = null;
    controller?.abort();
  }, []);
  const permission = useRef(access); permission.current = access;
  const messageRef = useRef(message); messageRef.current = message;
  const replyToRef = useRef(replyTo); replyToRef.current = replyTo;
  const composer = useRef<HTMLTextAreaElement>(null), conversationEnd = useRef<HTMLDivElement>(null);
  const overviewBody = useRef<HTMLDivElement>(null);
  const contentId = useId();
  const cancelEditRequest = useRef<(() => void) | null>(null);
  const editRequestNeedsWrite = useRef(true);
  const decisionRequest = useRef(0), decisionNoticeNode = useRef<HTMLParagraphElement>(null);
  const decisionNodes = useRef(new Map<string, HTMLDivElement>());
  const conversationNodes = useRef(new Map<string, HTMLLIElement>());
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
    latest.current = value; setView(value); setContextStale(false);
  }, [projectId]);
  const forgetPrivateRead = useCallback(() => {
    // A confirmed read refusal supersedes cached permissions and every pending read.
    // Keep the user's unsent local draft and durable paid-request identity intact.
    generation.current++; viewRead.current++; decisionRequest.current++; abortRefresh();
    latest.current = null; working.current = false;
    setView(null); setEarlier([]); setPage(null); setReview(null); setSuggestedReply(null);
    setPendingJob(null); setMissingJobId(""); setDecisionFocus(null); setDecisionNotice("");
    setBusy(""); setLoading(false); setContextStale(true);
    cancelEditRequest.current?.(); cancelEditRequest.current = null;
  }, [abortRefresh]);
  const refresh = useCallback(async (explicit = false) => {
    if (!permission.current.canRead || working.current) return;
    const ticket = ++generation.current;
    const read = ++viewRead.current;
    abortRefresh();
    const controller = new AbortController(); refreshController.current = controller;
    // Routine observation keeps the last recorded view stable. Explicit refreshes
    // and known context changes disable derived actions until the new read lands.
    if (explicit || !latest.current) { setContextStale(true); setLoading(true); }
    try {
      const value = await orchestratorApi.get(projectId, undefined, controller.signal);
      if (!alive.current || !permission.current.canRead || controller.signal.aborted || ticket !== generation.current || read !== viewRead.current) return;
      remember(value); setError("");
      if (explicit) setUncertain(false); // A retained request remains blocked independently until its exact receipt is resolved.
    } catch (cause) { if (alive.current && !controller.signal.aborted && ticket === generation.current && read === viewRead.current) { setError(errorText(cause)); setContextStale(true); if (readAccessDenied(cause)) forgetPrivateRead(); } }
    finally {
      if (refreshController.current === controller) refreshController.current = null;
      if (alive.current && !controller.signal.aborted && ticket === generation.current) setLoading(false);
    }
  }, [projectId, remember, forgetPrivateRead, abortRefresh]);

  useEffect(() => {
    alive.current = true;
    return () => { alive.current = false; generation.current++; abortRefresh(); cancelEditRequest.current?.(); };
  }, [abortRefresh]);
  useEffect(() => {
    if (!canRead || (!canEdit && editRequestNeedsWrite.current)) { cancelEditRequest.current?.(); cancelEditRequest.current = null; }
  }, [canEdit, canRead]);
  useEffect(() => {
    if (!canRead || !canAssist || pendingRequest || recoveredDraft || storageConflict) setSuggestedReply(null);
  }, [canRead, canAssist, pendingRequest, recoveredDraft, storageConflict]);
  useEffect(() => {
    if (!canRead) { forgetPrivateRead(); return; }
    setContextStale(true);
    void refresh();
  }, [canRead, sourceRevision, lens, refresh, forgetPrivateRead]);
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
      const detail = (event as CustomEvent<OrchestratorOpenDetail>).detail;
      if (detail?.projectId !== projectId || !permission.current.canRead) return;
      if (detail.destination === "decisions" && detail.scope !== scope) return;
      const ticket = ++decisionRequest.current;
      setDecisionFocus(null); setDecisionNotice("");
      setOpenChoice(true);
      setConversationOpen(true);
      if (detail.destination === "decisions") {
        // This destination only reveals an existing decision. It must never
        // replace an unsent message, reply target or retained paid request.
        setDecisionNotice("Reading current project decisions…");
        const contextGeneration = generation.current;
        // Fence older GETs too, without changing an in-flight mutation's
        // generation/ownership. Ledger revision alone cannot order context edits.
        const read = ++viewRead.current;
        const contextChanged = () => {
          if (contextGeneration === generation.current && read === viewRead.current) return false;
          setDecisionNotice("The project changed while its decisions were loading. Refresh run history and open the current decision again; your message has not been changed.");
          return true;
        };
        void orchestratorApi.get(projectId).then(value => {
          if (!alive.current || !permission.current.canRead || ticket !== decisionRequest.current) return;
          if (contextChanged()) return;
          if (value.projectId !== projectId) throw new Error("The project decision response belongs to another project.");
          remember(value);
          const current = latest.current;
          if (detail.turnId && current?.blockingQuestions.some(question => question.turnId === detail.turnId)) {
            setDecisionNotice(""); setDecisionFocus({ ticket, turnId: detail.turnId });
          } else {
            setDecisionNotice(detail.turnId ? "This decision is no longer listed as unresolved. Review the current project understanding and refresh run history; your message has not been changed." : "Project decisions refreshed. Review the unresolved decisions below, if any, then refresh run history. Your message has not been changed.");
            setDecisionFocus({ ticket });
          }
        }).catch(cause => {
          if (!alive.current || !permission.current.canRead || ticket !== decisionRequest.current) return;
          if (contextChanged()) return;
          if (readAccessDenied(cause)) { setError(errorText(cause)); forgetPrivateRead(); return; }
          setContextStale(true);
          setDecisionNotice(`Project decisions could not be refreshed: ${errorText(cause)} Your message has not been changed.`);
          setDecisionFocus({ ticket });
        });
        return;
      }
      if (typeof detail.message === "string" && detail.message.trim()) {
        if (!messageRef.current.trim()) editComposerRef.current(detail.message.slice(0, 16000), "");
        else setNotice("Your unsent message is still here. Send or clear it before starting another question.");
      }
      requestAnimationFrame(() => { if (alive.current) composer.current?.focus(); });
    };
    window.addEventListener(ORCHESTRATOR_OPEN, receive);
    return () => window.removeEventListener(ORCHESTRATOR_OPEN, receive);
  }, [projectId, scope, remember, forgetPrivateRead]);
  useEffect(() => () => { decisionRequest.current++; }, [lens]);
  useEffect(() => {
    if (busy) { decisionRequest.current++; setDecisionFocus(null); }
  }, [busy]);
  useEffect(() => {
    if (!canRead || !open || !conversationOpen) { decisionRequest.current++; setDecisionFocus(null); }
  }, [canRead, open, conversationOpen]);
  useEffect(() => {
    if (!decisionFocus || !open || !conversationOpen || !canRead) return;
    const target = decisionFocus;
    const frame = requestAnimationFrame(() => {
      if (!alive.current || !permission.current.canRead || target.ticket !== decisionRequest.current) return;
      if (target.turnId && !latest.current?.blockingQuestions.some(question => question.turnId === target.turnId)) {
        setDecisionNotice("This decision is no longer listed as unresolved. Refresh run history to review the current guidance.");
        setDecisionFocus({ ticket: target.ticket });
        return;
      }
      const group = target.turnId ? decisionNodes.current.get(target.turnId) : decisionNoticeNode.current;
      const answer = group?.querySelector<HTMLButtonElement>("button");
      const node = answer && !answer.disabled ? answer : group;
      node?.scrollIntoView?.({ block: "nearest" }); node?.focus();
      setDecisionFocus(null);
    });
    return () => cancelAnimationFrame(frame);
  }, [decisionFocus, open, conversationOpen, canRead]);

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
          if (readAccessDenied(cause)) { forgetPrivateRead(); return; }
          if (activeJobId && activeTurn?.status === "reserved" && cause instanceof ApiError && cause.status === 404) setMissingJobId(activeJobId);
        }
      }
      finally { polling = false; }
    };
    const timer = setInterval(() => void tick(), activeJobId ? 2500 : 10000);
    return () => { stopped = true; clearInterval(timer); };
  }, [canRead, activeJobId, activeTurn?.status, projectId, refresh, forgetPrivateRead]);
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
    if (!permission.current.canAssist || !view || !message.trim() || contextDrafts || routeIssue || busy || loading || contextStale || contextRefreshQueued || working.current || activeTurn || activeJobId || uncertain || pendingRequest || recoveredDraft || storageConflict || localRecovery.error) return;
    const input: OrchestratorRequest = { expectedRevision: view.revision, expectedContextDigest: view.context.digest, message: message.trim(),
      ...(replyTo ? { replyToTurnId: replyTo } : {}), harness: agent.harness, ...(model.trim() ? { model: model.trim() } : {}), agentConnectionId: agent.agentConnectionId, idempotencyKey: crypto.randomUUID() };
    try {
      retainComposerRequest(scope, input); setLocalRecovery(readComposerRecovery(scope)); setReviewedRequest(input.idempotencyKey);
    } catch (cause) { setError(errorText(cause)); setLocalRecovery(readComposerRecovery(scope)); return; }
    await dispatchRequest(input);
  }
  async function retryOriginal() {
    if (!permission.current.canAssist || !pendingRequest || reviewedRequest !== pendingRequest.request.idempotencyKey || contextDrafts || working.current || busy) return;
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
    } catch (cause) { if (alive.current && ticket === generation.current) { setError(errorText(cause)); if (readAccessDenied(cause)) forgetPrivateRead(); } }
    finally { working.current = false; if (alive.current) setBusy(""); }
  }
  async function apply() {
    const current = latest.current;
    const unsaved = Object.values(useCockpit.getState().unsavedProjectDrafts).some(draft => draft.projectId === projectId);
    if (!canEdit || !review || !current || contextStale || unsaved || working.current || review.revision !== current.revision || review.digest !== current.context.digest) return;
    working.current = true; const ticket = ++generation.current; setBusy("apply"); setError("");
    try {
      const value = await orchestratorApi.apply(projectId, review.turn.id, { expectedRevision: review.revision, expectedContextDigest: review.digest });
      if (!alive.current || !permission.current.canRead || ticket !== generation.current) return;
      remember(value); setReview(null); setRefreshEditor(true);
      setNotice("Amendment saved as a draft. Review the updated context and any affected approvals before implementation.");
    } catch (cause) { if (alive.current && ticket === generation.current) setError(`${errorText(cause)} Keep this proposal, refresh the conversation and review the current context before applying again.`); }
    finally { if (ticket === generation.current) { working.current = false; if (alive.current) setBusy(""); } }
  }
  async function advanceDraft(turnId: string, input: OrchestratorAdvanceRequest): Promise<OrchestratorView> {
    const current = latest.current;
    const unsaved = Object.values(useCockpit.getState().unsavedProjectDrafts).some(draft => draft.projectId === projectId);
    if (!permission.current.canEdit || permission.current.loading || permission.current.error || !current || current.revision !== input.expectedRevision || current.context.digest !== input.expectedContextDigest || contextStale || contextRefreshQueued || unsaved || working.current || pendingRequest || uncertain) {
      throw new Error("The current project is not ready for automatic draft saving.");
    }
    working.current = true; const ticket = ++generation.current; setBusy("auto-draft"); setError("");
    try {
      const value = await orchestratorApi.advance(projectId, turnId, input);
      if (alive.current && permission.current.canRead && permission.current.canEdit && ticket === generation.current) {
        remember(value); setRefreshEditor(true);
      }
      return value;
    } finally { if (ticket === generation.current) { working.current = false; if (alive.current) setBusy(""); } }
  }
  const openComposer = (text?: string) => {
    const origin = document.activeElement;
    const ticket = generation.current;
    setOpenChoice(true); setConversationOpen(true);
    if (text && !messageRef.current.trim()) editComposer(text.slice(0, 16000), replyTo);
    requestAnimationFrame(() => {
      if (!alive.current || !permission.current.canRead || permission.current.loading || permission.current.error || ticket !== generation.current ||
          document.activeElement !== origin && document.activeElement !== document.body) return;
      // Opening the coordinator is navigation only. Retained requests/drafts
      // still require their existing explicit recovery controls before sending.
      const field = composer.current;
      const target = field?.disabled
        ? overviewBody.current?.querySelector<HTMLButtonElement>('[aria-label="Unsent message recovery"] button, [aria-label="Unconfirmed message recovery"] button')
        : field;
      target?.scrollIntoView?.({ block: "nearest" }); target?.focus();
    });
  };
  const openConversationTurn = (turnId: string) => {
    if (!permission.current.canRead || permission.current.loading || permission.current.error) return;
    const origin = document.activeElement, ticket = generation.current;
    setOpenChoice(true); setConversationOpen(true);
    requestAnimationFrame(() => {
      if (!alive.current || !permission.current.canRead || permission.current.loading || permission.current.error || ticket !== generation.current ||
          document.activeElement !== origin && document.activeElement !== document.body) return;
      const node = conversationNodes.current.get(turnId);
      if (node) { node.scrollIntoView?.({ block: "nearest" }); node.focus(); }
      else setNotice("This request is in earlier conversation history. Show earlier conversation to find it. Your message and reply target have not changed.");
    });
  };
  const ask = (text?: string) => {
    // Opening a generic discussion must not detach an answer already in progress.
    // Clear reply is the explicit way to use that message for a separate topic.
    if (!messageRef.current.trim() && !pendingRequest && !recoveredDraft) editComposer(messageRef.current || text?.slice(0, 16000) || "", "");
    openComposer();
  };
  const edit = (editor: ContextEditor, questionId?: string) => {
    const permitted = () => permission.current.canRead && (editor === "plan" || permission.current.canEdit);
    if (!permitted()) return;
    cancelEditRequest.current?.();
    editRequestNeedsWrite.current = editor !== "plan";
    const destination = editor === "definition" ? "discover" : editor === "plan" ? "develop" : "design";
    const request = () => {
      if (!alive.current || !permitted()) return;
      const cancel = requestProjectContextEdit(scope, projectId, editor, questionId);
      // Exact destinations transfer focus, so compact screens can give their
      // workspace back. Generic editors keep the overview and opener visible.
      const transfersFocus = editor === "plan" || (editor === "definition" && !!questionId);
      if (transfersFocus && window.matchMedia(compactOverviewQuery).matches) setOpenChoice(false);
      const unsubscribe = useCockpit.subscribe(state => {
        if (state.lens === destination) return;
        cancel(); unsubscribe();
      });
      cancelEditRequest.current = () => { cancel(); unsubscribe(); };
    };
    setLens(destination);
    const current = useCockpit.getState();
    if (current.lens === destination) request();
    else if (current.pendingNavigation?.kind === "lens" && current.pendingNavigation.lens === destination) {
      // Publish only after guarded navigation succeeds. Cancelling must not
      // leave a context target waiting for an unrelated later visit.
      const unsubscribe = useCockpit.subscribe(state => {
        if (state.pendingNavigation?.kind === "lens" && state.pendingNavigation.lens === destination) return;
        unsubscribe();
        cancelEditRequest.current = null;
        if (state.lens === destination && state.selection.projectId === current.selection.projectId) request();
      });
      cancelEditRequest.current = unsubscribe;
    }
  };
  const reply = (turnId: string, text = "") => { editComposer(text || messageRef.current, turnId); openComposer(); };
  const suggestReply = (turn: OrchestratorTurn, answer: string) => {
    if (!permission.current.canAssist || pendingRequest || recoveredDraft || storageConflict) {
      setNotice("Restore or resolve your saved message before starting another one."); return;
    }
    if (messageRef.current.trim() && (messageRef.current !== answer || replyTo !== turn.id)) {
      setSuggestedReply({ turnId: turn.id, question: turn.answer?.question || "", answer, message: messageRef.current, replyTo, previousQuestion: replyQuestion || "An earlier project decision" });
      return;
    }
    reply(turn.id, answer);
  };
  const suggestedReplyCloseTarget = (selected: NonNullable<typeof suggestedReply>, openingControl: HTMLElement | null) => {
    // Resolve this explicit choice at the modal's actual close lifecycle. Frame
    // callbacks can run before Radix restores its opener and lose the handoff.
    const intent = choiceFocus.current;
    if (!intent || intent.choice !== selected) return null;
    choiceFocus.current = null;
    if (!alive.current || !permission.current.canRead || !permission.current.canAssist ||
        messageRef.current !== intent.message || replyToRef.current !== intent.replyTo) return null;
    const active = document.activeElement;
    if (active !== document.body && active !== openingControl && active !== composer.current) return null;
    return intent.composer || !openingControl?.isConnected ? composer.current : openingControl;
  };
  const closeSuggestedReply = () => {
    const selected = suggestedReply;
    if (selected) choiceFocus.current = { choice: selected, message: messageRef.current, replyTo, composer: false };
    setSuggestedReply(null);
  };
  const replaceWithSuggestion = () => {
    if (!suggestedReply || !permission.current.canRead || !permission.current.canAssist || pendingRequest || recoveredDraft || storageConflict || busy) return;
    const selected = suggestedReply;
    choiceFocus.current = { choice: selected, message: messageRef.current, replyTo, composer: true };
    setSuggestedReply(null);
    if (messageRef.current !== selected.message || replyTo !== selected.replyTo) {
      setNotice("Your unsent message changed while this choice was open. It has been kept; choose the suggested answer again if you want to replace it.");
      return;
    }
    const question = turns.find(turn => turn.id === selected.turnId);
    if (!question || question.questionResolvedBy || !question.answer || question.answer.question !== selected.question || !question.answer.options.includes(selected.answer)) {
      setNotice("This question changed while this choice was open. Your unsent message has been kept. Review the current question before answering.");
      return;
    }
    editComposer(selected.answer, selected.turnId);
    setOpenChoice(true); setConversationOpen(true);
    choiceFocus.current = { choice: selected, message: selected.answer, replyTo: selected.turnId, composer: true };
  };
  const inspect = (runId: string) => {
    if (!permission.current.canRead || contextStale) return;
    useCockpit.setState({ projectRunInspection: { scope, projectId, runId, refresh: true } });
    setLens("develop");
  };
  const discussFinding = (work: NonNullable<OrchestratorView["activity"]>["work"][number]) => {
    if (!permission.current.canAssist || contextStale || pendingRequest || recoveredDraft || storageConflict || uncertain || busy) return;
    if (messageRef.current.trim()) {
      setNotice("Your unsent message has been kept. Include the recorded finding when you ask Luci about it.");
    } else {
      editComposer(`Review the retained ${work.status} finding for run ${work.runId || "without a run"}, context ${work.contextDigest}${work.finding?.checkId ? `, check ${work.finding.checkId}` : ""}. Explain any necessary Discover or Design change before continuing. Do not retry an unconfirmed external effect.`, "");
    }
    openComposer();
  };
  const turns = [...new Map([...earlier, ...(view?.turns ?? [])].map(turn => [turn.id, turn])).values()].sort((a, b) => a.sequence - b.sequence);
  const replyQuestion = view?.blockingQuestions.find(question => question.turnId === replyTo)?.question || turns.find(turn => turn.id === replyTo)?.answer?.question;
  const reviewBlock = draftWarning || setupChangeWarning || (contextStale ? "Refresh project understanding before applying an amendment." : !canEdit ? "Editing access is required to apply an amendment." : review && view && review.revision !== view.revision ? "The conversation changed during review. Close this review, refresh and review the amendment again." : "");

  const understanding = view && <ProjectUnderstanding view={view} stale={contextStale || contextRefreshQueued} onAsk={ask} onReply={reply} onNavigate={setLens} onInspect={inspect} onEdit={edit} canEdit={canEdit} />;
  return <aside className={`orchestrator${open ? " orchestrator-open" : ""}${primary ? " orchestrator-primary" : ""}`} role={primary ? "region" : undefined} aria-label={primary ? "Luci conversation" : "Project understanding"}>
    <header className="orchestrator-header"><div><h2>{primary ? "Luci" : "Project understanding"}</h2>{primary ? <p>Working in {projectName}. Discover, Design, Develop and Deploy are your project tools.</p> : open ? <p>One conversation across your project.</p> : <p className="orchestrator-collapsed-stage">{canRead && view ? `${stageLabels[view.nextAction.lens]} · Saved context` : "Saved project context"}</p>}</div><Button variant="primary" size="compact" disabled={!canRead || access.loading || !!access.error} onClick={() => { useCockpit.getState().showLuci(); ask(); }}>Talk to Luci</Button>{primary ? <Button variant="secondary" size="compact" onClick={() => setLens(lens)}>Return to {stageLabels[lens as keyof typeof stageLabels] ?? "Develop"}</Button> : <Button variant="secondary" size="compact" aria-expanded={open} aria-controls={contentId} onClick={() => setOpenChoice(!open)}>{open ? "Hide project overview" : "Show project overview"}</Button>}</header>
    <div className="orchestrator-body" id={contentId} ref={overviewBody} hidden={!open}>
      {access.message && <p role="status">{access.message}</p>}
      {canRead && <>
        <div className="orchestrator-toolbar"><span className="orchestrator-muted">Saved shared context</span><Button variant="ghost" size="compact" disabled={!!busy || loading} onClick={() => void refresh(true)}>Refresh project understanding</Button></div>
        {loading && !view && <p role="status">Loading project understanding…</p>}
        {error && <p className="orchestrator-error" role="alert">{error}</p>}
        {notice && <p role="status">{notice}</p>}
        {decisionNotice && <p ref={decisionNoticeNode} role="status" tabIndex={-1}>{decisionNotice}</p>}
        {(loading || contextRefreshQueued) && view && <p role="status">Updating saved project understanding…</p>}
        {!primary && understanding}
        <details className="orchestrator-discussion" open={conversationOpen} onToggle={event => { if (!primary) setConversationOpen(event.currentTarget.open); }}><summary>Discuss or request a change</summary>
        {conversation.mounted && <div hidden={!conversationOpen}>
        {view && <>
          <div className="orchestrator-toolbar"><h3>Luci</h3><Button variant="ghost" size="compact" disabled={!!busy || loading} onClick={() => void refresh(true)}>Refresh conversation</Button></div>
          {view.activity && (!!view.activity.accepted.length || !!view.activity.work.length || !!view.activity.findings?.length || !!view.activity.recovery?.length || !!view.blockingQuestions.length) &&
            <section aria-label="Shared changes and affected work" className="orchestrator-provenance">
              <h4>Shared changes and affected work</h4>
              {!!view.blockingQuestions.length && <p>{view.blockingQuestions.length} decision{view.blockingQuestions.length === 1 ? "" : "s"} {view.blockingQuestions.length === 1 ? "needs" : "need"} an answer before affected work can continue. Answer below; Luci will recheck current context, authority and policy before acting.</p>}
              {view.activity.accepted.map(event => <p key={event.eventId}>
                {event.actor.displayName}’s {event.target} change was accepted at target <code>{event.appliedTargetDigest}</code>.
                {event.acceptedContextDigest && <> Accepted context <code>{event.acceptedContextDigest}</code>.</>}
                {event.affectsAllTasks ? " All tasks may be affected." : event.taskIds.length ? ` Affected tasks: ${event.taskIds.join(", ")}.` : ""}
                {" "}Event <code>{event.eventId}</code> · intent <code>{event.intentId}</code> · conversation <code>{event.originTurnId}</code>
              </p>)}
              {view.activity.work.map(work => <p key={work.id}>
                Affected work: {work.message} · {work.status} · context <code>{work.contextDigest}</code>
                {work.acceptedEvents.length > 0 && <> · accepted event{work.acceptedEvents.length === 1 ? "" : "s"} {work.acceptedEvents.map(event => event.eventId).join(", ")}</>}
                {work.finding && <> · checks {work.finding.status}{work.finding.checkId && <> <code>{work.finding.checkId}</code></>}{work.finding.commit && <> · source <code>{work.finding.commit}</code></>}
                  {work.finding.previewStatus && <> · preview {work.finding.previewStatus}{work.finding.previewReleaseId && <> <code>{work.finding.previewReleaseId}</code></>}</>}
                  {work.finding.deployStatus && <> · deployment {work.finding.deployStatus}{work.finding.deployReleaseId && <> <code>{work.finding.deployReleaseId}</code></>}</>}
                </>}
                {work.runId && <> · run <code>{work.runId}</code> <Button variant="secondary" size="compact" disabled={!canRead || contextStale} onClick={() => inspect(work.runId!)}>Inspect run</Button></>}
                {(["blocked", "unknown", "refused", "preview-blocked", "deployment-blocked"].includes(work.status)) && <Button variant="secondary" size="compact" disabled={!canAssist || !!busy || contextStale || uncertain || !!pendingRequest || !!recoveredDraft || storageConflict} onClick={() => discussFinding(work)}>Discuss finding with Luci</Button>}
                {work.status === "deployment-blocked" && <Button variant="secondary" size="compact" disabled={!canRead || !access.permissions.deploy} onClick={() => setLens("deploy")}>Open Deploy recovery</Button>}
              </p>)}
              {view.activity.recovery?.map(recovery => <p key={recovery.releaseId}>
                Historical release recovery <code>{recovery.releaseId}</code> for release <code>{recovery.rollbackOf}</code> · {recovery.status} · commit <code>{recovery.commit}</code>. {recovery.message}
                {' '}This separate action does not change the original delivery receipt.
                <Button variant="secondary" size="compact" disabled={!canRead || !access.permissions.deploy} onClick={() => setLens("deploy")}>Open Deploy recovery</Button>
              </p>)}
              {view.activity.findings?.map(finding => <p key={finding.id} role={finding.status === 'question' || finding.status === 'unknown' ? 'status' : undefined}>
                Luci follow-up for {finding.sourceStatus} {finding.eventId ? 'event' : 'work'} <code>{finding.eventId || finding.continuationId}</code>, recorded digest <code>{finding.contextDigest}</code>:
                {' '}{finding.status === 'pending' ? 'Saved for this person. Automatic discussion waits for an explicitly saved provider route, model-spending choice and current project policy.' :
                  finding.status === 'reserved' || finding.status === 'inflight' ? 'One exact model turn is reserved or running; its outcome is not yet confirmed.' :
                  finding.status === 'question' ? 'A consequential question needs an answer in this conversation.' :
                  finding.status === 'resumed' ? 'The recorded question was answered; affected work is being rechecked against the accepted context.' :
                  finding.status === 'observed' ? 'This verified check or preview stage is recorded for this person. Inspect the exact run before treating it as delivered behavior.' :
                  finding.status === 'unknown' ? 'The model outcome is unconfirmed. Inspect the saved turn before another request.' :
                  'Luci’s result is ready for review. A proposal is still only a draft.'}
                {' '}Finding <code>{finding.id}</code>{finding.turnId && <> · turn <code>{finding.turnId}</code></>}.
              </p>)}
              {view.activity.earlierAvailable && <p>Earlier shared changes and work remain in the project’s saved coordination history.</p>}
            </section>}
          {(page?.hasEarlier ?? view.hasEarlier) && <Button variant="secondary" size="compact" disabled={!!busy} onClick={() => void older()}>Show earlier conversation</Button>}
          {!turns.length && <p className="orchestrator-welcome">Describe your idea, ask about the current work, or discuss a change. Luci can propose an amendment for you to review.</p>}
          <ol className="orchestrator-turns" aria-label="Conversation history">{turns.map(turn => <li key={turn.id} tabIndex={-1} ref={node => { if (node) conversationNodes.current.set(turn.id, node); else conversationNodes.current.delete(turn.id); }}>
            <div className="orchestrator-turn-meta"><strong>{turn.actor.displayName}</strong><time dateTime={turn.createdAt}>{new Date(turn.createdAt).toLocaleString()}</time></div>
            <p className="orchestrator-message">{turn.message}</p>
            {turn.replyToTurnId && <p className="orchestrator-muted">Reply to an earlier question</p>}
            <p className="orchestrator-muted">{statuses[turn.status]}{turn.appliedRevision ? ` · Applied as draft revision ${turn.appliedRevision}` : ""}</p>
            {turn.answer && <div className="orchestrator-answer"><strong>Luci</strong><p>{turn.answer.summary}</p>
              {turn.answer.question && <section className="orchestrator-question" aria-label="Luci question"><p><strong>{turn.answer.blocking && !turn.questionResolvedBy ? "Decision needed: " : "Question: "}</strong>{turn.answer.question}</p><p>{turn.answer.rationale}</p>
                {!!turn.answer.impact?.length && <div><strong>Why it matters</strong><ul>{turn.answer.impact.map((impact, index) => <li key={index}>{impact}</li>)}</ul></div>}
                {turn.questionResolvedBy ? <p>Answered in a later turn.</p> : <div className="orchestrator-actions">{turn.answer.options.map((option, index) => <Button key={index} variant="secondary" size="compact" disabled={!canAssist || !!busy} onClick={() => suggestReply(turn, option)}>{option}</Button>)}<Button variant="ghost" size="compact" disabled={!canAssist || !!busy} onClick={() => reply(turn.id)}>Reply to this question</Button></div>}
              </section>}
              {!turn.answer.question && turn.answer.rationale && <p>{turn.answer.rationale}</p>}
              {turn.answer.proposal && <div className="orchestrator-proposal"><p>{turn.answer.proposal.summary}</p>{turn.appliedRevision ? <section aria-label="Continue amendment review">
                <p>This amendment saved {amendmentLabels[turn.answer.proposal.target]} draft revision {turn.appliedRevision}. Review the current document and its affected approvals before preparing more work.</p>
                <p className="orchestrator-muted">Existing builds retain the context they started with. Reviewing this document does not stop or change those agents.</p>
                <Button variant="secondary" disabled={!!busy || contextStale || contextRefreshQueued || !canRead || (turn.answer.proposal.target !== "plan" && !canEdit)} onClick={() => edit(amendmentEditors[turn.answer!.proposal!.target])}>Open current {amendmentLabels[turn.answer.proposal.target]}</Button>
              </section> : <Button variant="secondary" disabled={!!busy} onClick={() => { setReview({ turn, revision: view.revision, digest: view.context.digest }); setError(""); }}>Review amendment</Button>}</div>}
              {turn.answer.nextAction && <p className="orchestrator-muted">Suggested next step: {turn.answer.nextAction}</p>}
            </div>}
            {(turn.status === "unknown" || turn.status === "reserved") && <p>The request is unconfirmed. Refresh to observe it; Luci will not start a duplicate request automatically.</p>}
            <details className="orchestrator-provenance"><summary>Context used for this turn</summary><p>Definition {turn.context.specification.revision} · Design {turn.context.designPlan.revision} · Plan {turn.context.plan.revision}</p><p>{turn.context.sourceKnown ? `Source ${turn.context.sourceCommit.slice(0, 10) || "without a commit"}${turn.context.sourceDirty ? " with local changes" : ""}` : "Source state was unavailable"}</p></details>
          </li>)}</ol>
          <div ref={conversationEnd} />
          {(completedUnrecorded || missingReservedJob) && <div role="status"><p>{missingReservedJob ? "This request was reserved, but its agent job could not be found. Recover the interrupted request without starting an agent." : "The agent request finished, but its response has not been recorded in this conversation. Recover the recorded response without running the agent again."}</p><Button variant="secondary" disabled={!canEdit || !!busy} onClick={() => void reconcile()}>Recover recorded response</Button></div>}
          {!!view.blockingQuestions.length && <section className="orchestrator-question" aria-label="Unresolved decisions"><strong>Resolve before implementation</strong>{view.blockingQuestions.map(question => <div key={question.turnId} role="group" aria-label={question.question} tabIndex={-1} ref={node => { if (node) decisionNodes.current.set(question.turnId, node); else decisionNodes.current.delete(question.turnId); }}><p>{question.question}</p><p>{question.rationale}</p><Button variant="secondary" size="compact" disabled={!canAssist || !!busy} onClick={() => reply(question.turnId)}>Answer this decision</Button></div>)}</section>}
          <ConversationStorage projectId={projectId} view={view} canEdit={canEdit} busy={!!busy} onBusy={value => { working.current = value; generation.current++; setLoading(false); setBusy(value ? "history" : ""); }} onView={(value, rotated) => { remember(value); if (rotated) { setEarlier([]); setPage(null); } }} />
        </>}
        {localRecovery.error && <p role="alert">{localRecovery.error}</p>}
        {canAssist && storageConflict && <section aria-label="Message changed in another tab"><p>The saved message changed in another tab. Keep a copy of any text you need before reloading it; sending is paused to avoid repeating a request.</p><Button variant="secondary" onClick={() => { const state = readComposerRecovery(scope); setLocalRecovery(state); savedDraft.current = state.draft; setRecoveredDraft(state.draft); setMessage(""); setReplyTo(""); setReviewedRequest(""); setStorageConflict(false); setError(""); }}>Reload saved message</Button></section>}
        {canAssist && recoveredDraft && !pendingRequest && !storageConflict && <section aria-label="Unsent message recovery"><p>An unsent message is saved for this account and project in this browser. It is kept for 30 days.</p><Button variant="secondary" onClick={restoreDraft}>Restore unsent message</Button><Button variant="ghost" onClick={discardDraft}>Discard unsent message</Button></section>}
        {canRead && pendingRequest && <section aria-label="Unconfirmed message recovery"><p>The original request may already have been accepted. Sending a new copy is blocked until its outcome is checked. Nothing is retried automatically.</p>
          <Button variant="secondary" disabled={!!busy} onClick={() => void checkOriginal()}>Check original request</Button>
          {localRecovery.pending.length > 1 && <p>{localRecovery.pending.length} requests need to be checked, one at a time.</p>}
          {canAssist && (reviewedRequest !== pendingRequest.request.idempotencyKey ? <Button variant="secondary" onClick={() => { setMessage(pendingRequest.request.message); setReplyTo(pendingRequest.request.replyToTurnId ?? ""); setReviewedRequest(pendingRequest.request.idempotencyKey); setConversationOpen(true); }}>Review original request</Button> : <><p>Original agent: {pendingRequest.request.harness}{pendingRequest.request.model ? ` · ${pendingRequest.request.model}` : ""}. Retry uses its saved context and original request identity; model usage may be charged if it was never accepted.</p><Button variant="secondary" disabled={!!busy || !!contextDrafts} onClick={() => void retryOriginal()}>Retry original request</Button></>)}
        </section>}
        <form className="orchestrator-composer" onSubmit={event => { event.preventDefault(); void send(); }}>
          {view && <section className="orchestrator-composer-context" aria-label="Conversation context">
            <p>Current saved context · Definition {view.context.specification.revision} · Design {view.context.designPlan.revision} · Plan {view.context.plan.revision}</p>
            <p className="orchestrator-muted">Describe an idea or amendment from any activity. Luci can ask about ambiguity and propose a draft for your review. A running agent keeps its original captured scope.</p>
            {canAssist && <Button type="button" variant="ghost" size="compact" disabled={!!busy || !!pendingRequest || !!recoveredDraft || storageConflict || !!view.blockingQuestions.length} onClick={() => ask('Help break the approved goal into small dependency-aware tasks for upstream coding agents. Give each task scoped write paths, requirement references and observable acceptance. Ask me about any material gap first. I prefer one cloud microVM per agent when the configured provider, capacity and spending policy permit; do not treat this request as execution or deployment consent.')}>Plan parallel tasks with Luci</Button>}
          </section>}
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
          {checkSetupDraft && <p role="status">Your verification setup is a browser draft and is not included automatically. Paste the relevant commands or unfinished arguments into your message to ask for help. Sending does not execute them.</p>}
          {!canAssist && canRead && <p role="status">Sending requires editing, execution and secret access. You can still read the conversation and saved context.</p>}
          {refreshEditor && drafts && <p role="status">The amendment is saved. {contextDrafts ? "Your newer unsaved edits remain in the editor; save or discard them to refresh its context." : checkSetupRetained ? "Your verification setup remains in the editor. Change phase and choose Keep drafts and continue, or discard the setup, to refresh saved context." : "Your verification setup remains in the editor but is not retained. Copy the command text you need, then discard the setup to refresh saved context."}</p>}
          {canAssist && replyTo && <div className="orchestrator-actions"><span>{replyQuestion ? `Replying to: ${replyQuestion}` : "Replying to a project decision"}</span><Button type="button" variant="ghost" size="compact" disabled={!!pendingRequest || !canAssist} onClick={() => editComposer(message, "")}>Clear reply</Button></div>}
          <label className="luci-field">Message Luci<textarea ref={composer} rows={3} value={canAssist ? message : ""} maxLength={16000} disabled={!canAssist || busy === "send" || !!pendingRequest || !!recoveredDraft || storageConflict} onChange={event => editComposer(event.target.value, replyTo)} placeholder="What should we build, clarify or change?" /></label>
          <div className="orchestrator-actions"><Button type="submit" disabled={!canAssist || !view || !message.trim() || !!contextDrafts || !!routeIssue || !!busy || loading || contextStale || contextRefreshQueued || !!activeTurn || !!activeJobId || uncertain || !!pendingRequest || !!recoveredDraft || storageConflict || !!localRecovery.error}>{busy === "send" ? "Sending…" : "Send to Luci"}</Button>{activeJobId && !completedUnrecorded && !missingReservedJob && <Button type="button" variant="secondary" disabled={!canExecute || !!busy} onClick={() => void cancel()}>Cancel Luci request</Button>}</div>
          <p className="orchestrator-muted">Luci proposes context changes and continues within your configured policy. Unresolved decisions, current approvals and deployment controls still apply.</p>
        </form>
        </div>}
        </details>
        {primary && <section aria-label="Saved project understanding">{understanding}</section>}
        {view && <CoordinatorPolicy projectId={projectId} scope={scope} actorId={actorId} view={view} policyEpoch={policyEpoch} onPolicyUpdated={() => setPolicyEpoch(value => value + 1)}
          canRead={canRead && !access.loading && !access.error} canEdit={canEdit}
          blocked={draftWarning || setupChangeWarning || (contextStale || contextRefreshQueued || loading ? "Refresh saved project understanding first." : busy || pendingRequest || uncertain ? "Resolve the current conversation request first." : "")}
          onAdvance={advanceDraft} beginObservation={() => {
            const observed = latest.current, read = viewRead.current, ticket = generation.current;
            const current = !contextStale && !contextRefreshQueued && !loading && !working.current;
            return value => { if (current && alive.current && !working.current && permission.current.canRead && !permission.current.loading && !permission.current.error && latest.current === observed && viewRead.current === read && generation.current === ticket) remember(value); };
          }} />}
        {view && <CoordinatorContextReview projectId={projectId} scope={scope} actorId={actorId} view={view}
          canRead={canRead && !access.loading && !access.error} canEdit={canEdit && !access.loading && !access.error}
          blocked={draftWarning || setupChangeWarning || (contextStale || contextRefreshQueued || loading ? "Refresh saved project understanding first." : busy || pendingRequest || uncertain ? "Resolve the current conversation request first." : "")}
          onReviewed={() => void refresh(true)} />}
        {view && <CoordinatorExecution projectId={projectId} scope={scope} actorId={actorId} view={view} policyEpoch={policyEpoch} onPolicyUpdated={() => setPolicyEpoch(value => value + 1)}
          canRead={canRead && !access.loading && !access.error} canEdit={canEdit && !access.loading && !access.error} canExecute={access.canExecute && !access.loading && !access.error} canDeploy={access.permissions.deploy && !access.loading && !access.error}
          blocked={draftWarning || (otherCheckSetup ? setupChangeWarning : "") || (contextStale || contextRefreshQueued || loading ? "Refresh saved project understanding first." : busy || pendingRequest || uncertain ? "Resolve the current conversation request first." : "")}
          runOn={setup?.workspace?.mode === "cloud" ? "Connected workspace · Docker compatibility" : "This computer · Docker"}
          onInspect={inspect} onNavigate={setLens} />}
        {view && <CoordinatorTeamIntents projectId={projectId} scope={scope} actorId={actorId} contextDigest={view.context.digest}
          canRead={canRead && !access.loading && !access.error} canEdit={canEdit}
          blocked={draftWarning || setupChangeWarning || (contextStale || contextRefreshQueued || loading ? "Refresh saved project understanding first." : busy || pendingRequest || uncertain ? "Resolve the current conversation request first." : "")}
          onDiscuss={openConversationTurn} onRefreshContext={() => void refresh(true)} />}
      </>}
    </div>
    {suggestedReply && canRead && canAssist && <WorkflowDialog readAccess={access} title="Replace your unsent message?" description="Using this suggested answer replaces your current message and its saved draft in this browser. Nothing is sent until you choose Send to Luci." onClose={closeSuggestedReply} getCloseFocusTarget={openingControl => suggestedReplyCloseTarget(suggestedReply, openingControl)}>
      <details><summary>Current unsent message</summary><p className="orchestrator-message">{suggestedReply.message}</p></details>
      {suggestedReply.replyTo && suggestedReply.replyTo !== suggestedReply.turnId && <>
        <p>Replacing this message also switches your reply to the new question.</p>
        <p><strong>Current question: </strong>{suggestedReply.previousQuestion}</p>
      </>}
      <p><strong>{suggestedReply.replyTo && suggestedReply.replyTo !== suggestedReply.turnId ? "New question: " : "Reply will answer: "}</strong>{suggestedReply.question}</p><p className="orchestrator-message">{suggestedReply.answer}</p>
      <div className="workflow-actions"><Button variant="secondary" onClick={closeSuggestedReply}>Keep current message</Button><Button disabled={!!busy || !!pendingRequest || !!recoveredDraft || storageConflict} onClick={replaceWithSuggestion}>Replace with suggested answer</Button></div>
    </WorkflowDialog>}
    {review && view && <AmendmentReview readAccess={access} turn={review.turn} view={view} busy={busy === "apply"} blocked={reviewBlock} error={error} onClose={() => setReview(null)} onApply={() => void apply()} />}
  </aside>;
}
