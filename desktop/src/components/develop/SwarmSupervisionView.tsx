import { useEffect, useId, useRef, useState, type ReactNode } from 'react';
import type { ProjectRun } from '../../api/projectRuns';
import { errorText } from '../../api/product';
import { checkedSupervision, mergeSwarmMessages, SWARM_MESSAGE_KINDS, swarmSupervisionApi, type SwarmMessage, type SwarmMessageKind, type SwarmSupervision } from '../../api/swarmSupervision';
import { useProjectDraftScope } from '../../hooks/useProjectDraft';
import { useTeamWorkspace } from '../teams/TeamWorkspaceProvider';
import { Button } from '../fluid/components/button';
import { SwarmReviewPanel } from './SwarmReviewPanel';
import { archiveRejectedMessageRecovery, clearMessageRecovery, readMessageRecovery, retainMessageRecovery, type PendingSwarmMessage } from './swarmMessageRecovery';
import './swarm-supervision.css';

interface Props {
  run: ProjectRun; canRead: boolean; canEdit: boolean; paused: boolean;
  onOpenTask(id: string): void;
  /** Existing run owner serializes writes and refreshes its own run after a successful write. */
  onWrite(action: () => Promise<void>): Promise<void>;
}
interface QuestionResolutionDraft {
  question: SwarmMessage; operationId: string; childId: string; worktreeId: string;
  projectId: string; runId: string; scope: string; runRevision: number; ownerRevision: number; stale: boolean;
}
export function SwarmSupervisionView(props: Props) {
  const draftScope = useProjectDraftScope(props.run.projectId), team = useTeamWorkspace();
  const scope = draftScope ? JSON.stringify([draftScope, team?.teamId ?? '', props.run.id]) : null;
  return <SwarmSupervisionPanel key={scope ?? `${props.run.projectId}:${props.run.id}:unscoped`} {...props} recoveryScope={scope} />;
}
/** One owner, explicit bounded pagination; no worker polling or controller actions. */
export function SwarmSupervisionPanel(props: Props & {recoveryScope: string | null}) {
  const {run, canRead, canEdit, paused, recoveryScope, onOpenTask, onWrite} = props;
  const [view, setView] = useState<SwarmSupervision | null>(null), [messages, setMessages] = useState<SwarmMessage[]>([]);
  const [windowStart, setWindowStart] = useState(0);
  const [error, setError] = useState(''), [notice, setNotice] = useState(''), [reading, setReading] = useState(false), [writing, setWriting] = useState(false);
  const [recovery] = useState(() => {
    try { return {pending: recoveryScope ? readMessageRecovery(recoveryScope) : null, error: ''}; }
    catch (cause) { return {pending: null, error: errorText(cause)}; }
  });
  const [pending, setPending] = useState<PendingSwarmMessage | null>(recovery.pending);
  const [retirable, setRetirable] = useState(false);
  const latest = useRef(props); latest.current = props;
  const live = useRef(false), readRequest = useRef<AbortController | null>(null), writeRequest = useRef<AbortController | null>(null);
  const readingRef = useRef(false), writingRef = useRef(false), readEpoch = useRef(0);
  const messageWindow = useRef(messages); messageWindow.current = messages;
  const pendingRef = useRef(pending); pendingRef.current = pending;
  const viewRef = useRef(view); viewRef.current = view;
  const confirmedResolutionRevision = useRef(0);
  const compose = useRef<HTMLDetailsElement>(null), claimInput = useRef<HTMLTextAreaElement>(null);
  const [senderId, setSenderId] = useState(''), [recipientId, setRecipientId] = useState('');
  const [kind, setKind] = useState<SwarmMessageKind>('discovery'), [summary, setSummary] = useState(''), [causalId, setCausalId] = useState('');
  const [resolution, setResolution] = useState<QuestionResolutionDraft | null>(null);
  const draftRef = useRef({resolution, senderId, recipientId, kind, causalId});
  draftRef.current = {resolution, senderId, recipientId, kind, causalId};
  const formId = useId();
  const tasks = new Map((run.swarm?.tasks ?? []).map(task => [task.id, task]));
  const operations = new Map(run.operations.map(operation => [operation.id, operation]));
  const currentOperations = (run.swarm?.tasks ?? []).flatMap(task => {
    const operation = operations.get(task.operationId ?? '');
    return operation?.taskId === task.id && operation.childId && operation.worktreeId ? [operation] : [];
  });
  const admitted = canRead && canEdit && !paused && !reading && !writing && !!recoveryScope && !recovery.error && !!view && view.runRevision >= run.revision && view.runRevision >= confirmedResolutionRevision.current;
  const currentOperationIdentity = JSON.stringify(currentOperations.map(operation => [operation.id, operation.taskId, operation.childId, operation.worktreeId, operation.allocation]));
  useEffect(() => {
    live.current = true;
    return () => { live.current = false; readEpoch.current++; readRequest.current?.abort(); writeRequest.current?.abort(); };
  }, []);
  useEffect(() => { if (!canRead || !canEdit) writeRequest.current?.abort(); }, [canRead, canEdit]);
  useEffect(() => {
    setResolution(value => value ? {...value, stale: true} : null);
  }, [run.projectId, run.id, run.revision, recoveryScope, canRead, canEdit, paused, currentOperationIdentity, view?.context.freshness, view?.context.contractDigest]);
  function resolutionReady() {
    const draft = draftRef.current, intent = draft.resolution, owner = latest.current, snapshot = viewRef.current;
    if (!intent || intent.stale || !owner.canRead || !owner.canEdit || owner.paused || owner.recoveryScope !== intent.scope ||
        owner.run.projectId !== intent.projectId || owner.run.id !== intent.runId || owner.run.revision !== intent.ownerRevision ||
        !snapshot?.questionResolutions || snapshot.context.freshness !== 'current' || snapshot.runRevision !== intent.runRevision || snapshot.runRevision < owner.run.revision || snapshot.runRevision < confirmedResolutionRevision.current ||
        snapshot.tasks.find(item => item.taskId === intent.question.recipientTaskId)?.operationId !== intent.operationId ||
        snapshot.questionResolutions.some(item => item.questionId === intent.question.id) ||
        draft.kind !== 'decision' || draft.causalId !== intent.question.id || draft.senderId !== intent.operationId || draft.recipientId !== intent.question.sender.taskId) return false;
    const task = owner.run.swarm?.tasks.find(item => item.id === intent.question.recipientTaskId);
    const operation = owner.run.operations.find(item => item.id === intent.operationId);
    return !!task && !!operation && task.operationId === intent.operationId && operation.taskId === task.id && operation.childId === intent.childId &&
      operation.worktreeId === intent.worktreeId && operation.allocation === 'ready';
  }
  function settleAck(value: SwarmSupervision) {
    const retained = pendingRef.current;
    if (!recoveryScope || retained?.type !== 'ack') return;
    const message = value.messages.find(item => item.id === retained.messageId);
    const operation = operations.get(retained.request.recipientOperationId);
    if (!message?.ack || !operation || message.ack.operationId !== operation.id || message.ack.childId !== operation.childId) return;
    clearMessageRecovery(recoveryScope, retained); pendingRef.current = null; setPending(null); setRetirable(false);
    setNotice('Delivery receipt confirmed in saved history. This does not approve or execute work.');
  }
  async function read(after: number, replace: boolean, duringWrite = false) {
    if (!latest.current.canRead || !duringWrite && latest.current.paused || readingRef.current) return;
    const controller = new AbortController(), epoch = ++readEpoch.current;
    readRequest.current?.abort(); readRequest.current = controller; readingRef.current = true; setReading(true); setError('');
    try {
      const value = await swarmSupervisionApi.get(run.projectId, run.id, after, controller.signal);
      if (!live.current || !latest.current.canRead || controller.signal.aborted || epoch !== readEpoch.current) return;
      if (viewRef.current && value.runRevision < viewRef.current.runRevision) throw new Error('The supervision response predates the retained run snapshot. Its newer history was preserved.');
      if (value.runRevision < confirmedResolutionRevision.current) throw new Error('The supervision response predates a confirmed resolution receipt. Refresh saved messages before recording another decision.');
      const merged = mergeSwarmMessages(replace ? [] : messageWindow.current, value.messages);
      checkedSupervision(run.projectId, run.id, after, value, merged);
      setMessages(merged); if (replace) setWindowStart(after); setView(value); settleAck(value);
    } catch (cause) {
      if (live.current && latest.current.canRead && !controller.signal.aborted && epoch === readEpoch.current) setError(errorText(cause));
    } finally {
      if (readRequest.current === controller) { readRequest.current = null; readingRef.current = false; if (live.current) setReading(false); }
    }
  }
  useEffect(() => {
    if (canRead && !paused) void read(0, true);
    return () => { readEpoch.current++; readRequest.current?.abort(); readRequest.current = null; readingRef.current = false; };
  }, [run.revision, canRead, paused]);
  async function send(value: PendingSwarmMessage) {
    if (!admitted || writingRef.current || !recoveryScope) return;
    // The parent may refuse a stale/busy write. Retain only inside its admitted action.
    await onWrite(async () => {
      if (!live.current || !latest.current.canRead || !latest.current.canEdit || writingRef.current) return;
      if (value.type === 'post' && value.request.resolvesQuestionId && (!resolutionReady() ||
          value.request.resolvesQuestionId !== draftRef.current.resolution?.question.id || value.operationId !== draftRef.current.senderId ||
          value.request.kind !== draftRef.current.kind || value.request.recipientTaskId !== draftRef.current.recipientId || value.request.causalMessageId !== draftRef.current.causalId ||
          value.request.expectedRevision !== viewRef.current?.runRevision)) {
        setError('This resolution draft no longer matches the current question, attempt or access. Refresh saved messages and select the question again. No write was sent.');
        return;
      }
      const controller = new AbortController(); writeRequest.current = controller; writingRef.current = true; setWriting(true); setError(''); setNotice('');
      try {
        retainMessageRecovery(recoveryScope, value); pendingRef.current = value; setPending(value); setRetirable(false);
        const result = value.type === 'post'
          ? await swarmSupervisionApi.post(run.projectId, run.id, value.operationId, value.request, controller.signal)
          : await swarmSupervisionApi.ack(run.projectId, run.id, value.messageId, value.request, controller.signal);
        if (!live.current || !latest.current.canRead || !latest.current.canEdit || controller.signal.aborted) return;
        if (value.type === 'post' && value.request.resolvesQuestionId && !resolutionReady()) return;
        const sender = operations.get(result.message.sender.operationId);
        if (!sender || sender.taskId !== result.message.sender.taskId || sender.childId !== result.message.sender.childId || sender.worktreeId !== result.message.sender.worktreeId) throw new Error('The returned sender does not match a saved attempt. Inspect history before continuing.');
        if (value.type === 'ack' && result.message.ack?.childId !== operations.get(value.request.recipientOperationId)?.childId) throw new Error('The returned delivery receipt does not match the saved recipient attempt.');
        if (value.type === 'post' && value.request.resolvesQuestionId) confirmedResolutionRevision.current = Math.max(confirmedResolutionRevision.current, result.runRevision);
        clearMessageRecovery(recoveryScope, value); pendingRef.current = null; setPending(null); setRetirable(false);
        setSummary(''); setCausalId(''); setResolution(null);
        setNotice(value.type === 'post' ? value.request.resolvesQuestionId ? 'Question resolution recorded. This records the coordinator’s decision; answer correctness and worker delivery are not verified.' : 'Attributed message recorded. No worker was started or instructed to execute.' : 'Delivery receipt recorded. This does not approve, verify or integrate work.');
        await read(0, true, true);
      } catch (cause) {
        if (live.current && latest.current.canRead) setError(`${errorText(cause)} Inspect saved messages; any retained request remains unconfirmed. No automatic retry occurred.`);
      } finally {
        writingRef.current = false; writeRequest.current = null; if (live.current) setWriting(false);
      }
    });
  }
  async function reconcilePending() {
    const retained = pendingRef.current;
    if (!retained || !recoveryScope || !latest.current.canRead || writingRef.current) return;
    const controller = new AbortController(); writeRequest.current = controller; writingRef.current = true; setWriting(true); setError(''); setNotice('');
    try {
      if (retained.type === 'post') {
        const result = await swarmSupervisionApi.disposition(run.projectId, run.id, retained.operationId, retained.request, controller.signal);
        if (!live.current || controller.signal.aborted || !latest.current.canRead) return;
        if (result.state === 'recorded') {
          if (retained.request.resolvesQuestionId) {
            confirmedResolutionRevision.current = Math.max(confirmedResolutionRevision.current, result.runRevision);
            setResolution(null);
          }
          clearMessageRecovery(recoveryScope, retained); pendingRef.current = null; setPending(null); setRetirable(false);
          setNotice('The exact message is recorded in saved history. No worker was started.');
          if (retained.request.resolvesQuestionId) await read(0, true, true);
        } else if (result.state === 'rejected' || result.state === 'key-conflict') {
          archiveRejectedMessageRecovery(recoveryScope, retained, result.state, result.runRevision); pendingRef.current = null; setPending(null); setRetirable(false);
          setNotice('The original rejected request was preserved for inspection. You may draft a new request against the current run.');
        } else { setRetirable(true); setNotice('The exact request can still be in flight. Its outcome remains unconfirmed; no retry was sent. You can explicitly close its original revision.'); }
      } else {
        const result = await swarmSupervisionApi.message(run.projectId, run.id, retained.messageId, controller.signal);
        if (!live.current || controller.signal.aborted || !latest.current.canRead) return;
        const recipient = operations.get(retained.request.recipientOperationId);
        if (recipient && result.message.ack?.operationId === recipient.id && result.message.ack.childId === recipient.childId) {
          clearMessageRecovery(recoveryScope, retained); pendingRef.current = null; setPending(null); setRetirable(false);
          setNotice('The exact delivery receipt is recorded in saved history.');
        } else if (result.message.ack || result.runRevision > retained.request.expectedRevision) {
          archiveRejectedMessageRecovery(recoveryScope, retained, result.message.ack ? 'key-conflict' : 'rejected', result.runRevision); pendingRef.current = null; setPending(null); setRetirable(false);
          setNotice('The original delivery request was preserved as rejected. Review the saved message before recording a new receipt.');
        } else { setRetirable(true); setNotice('The delivery request can still be in flight. No retry was sent. You can explicitly close its original revision.'); }
      }
    } catch (cause) { if (live.current && !controller.signal.aborted) setError(errorText(cause)); }
    finally { writingRef.current = false; writeRequest.current = null; if (live.current) setWriting(false); }
  }
  async function retirePending() {
    const retained = pendingRef.current;
    if (!retained || !retirable || !recoveryScope || !latest.current.canEdit || writingRef.current) return;
    await onWrite(async () => {
      const controller = new AbortController(); writeRequest.current = controller; writingRef.current = true; setWriting(true); setError('');
      try {
        const result = retained.type === 'post'
          ? await swarmSupervisionApi.retirePost(run.projectId, run.id, retained.operationId, retained.request, controller.signal)
          : await swarmSupervisionApi.retireAck(run.projectId, run.id, retained.messageId, retained.request, controller.signal);
        if (!live.current || controller.signal.aborted || !latest.current.canRead) return;
        archiveRejectedMessageRecovery(recoveryScope, retained, 'rejected', result.runRevision); pendingRef.current = null; setPending(null); setRetirable(false);
        setNotice('The exact original request was fenced and preserved as rejected. A new request may use the current run revision.');
        await read(0, true, true);
      } catch (cause) { if (live.current && !controller.signal.aborted) setError(`${errorText(cause)} The original request remains retained; read back its outcome before new work.`); }
      finally { writingRef.current = false; writeRequest.current = null; if (live.current) setWriting(false); }
    });
  }
  if (!canRead) return <section className="swarm-supervision" aria-label="Swarm messages"><p>Messages are unavailable until current project read access is confirmed.</p></section>;
  function taskLink(id: string) {
    const task = tasks.get(id);
    return task ? <Button variant="tertiary" onClick={() => onOpenTask(id)}>Inspect task {task.title} ({id})</Button> : <span>Saved task unavailable ({id})</span>;
  }
  const labels = (value: string, allowed: string[]) => allowed.includes(value) ? value : `unknown (${value})`;
  return <section className="swarm-supervision" aria-label="Swarm messages">
    <div className="project-swarm-heading"><h4>Team messages and attention</h4><Button variant="tertiary" disabled={paused || reading || writing} onClick={() => void read(0, true)}>Refresh saved messages</Button></div>
    <p className="swarm-supervision-note">Messages are attributed claims. Delivery receipts are not approval, correctness, integration or deployment authority.</p>
    {(error || recovery.error) && <p role="alert">{error || recovery.error}</p>}{notice && <p role="status">{notice}</p>}
    {reading && <p role="status">Reading saved supervision…</p>}
    {view ? <>
      <p>Activity: {labels(view.activityPhase, ['ready', 'queued', 'working', 'paused', 'blocked', 'stopped', 'unknown'])} · Quality: {labels(view.qualityPhase, ['not-started', 'needs-review', 'repair-required', 'awaiting-checks', 'checked', 'unknown'])} · Settlement: {labels(view.settlement, ['confirmed', 'uncertain'])}</p>
      <p className="swarm-supervision-note">Run revision {view.runRevision} · Queue revision {view.queueRevision} · Context {labels(view.context.freshness, ['current', 'changed', 'unavailable'])} · {view.context.contractDigest ? 'Captured execution contract' : 'Legacy: contract unassessed'} · {view.observedAt || 'Observation time not reported'}</p>
      {view.runRevision < run.revision && <p role="status">This message snapshot predates the current run. Refresh before writing.</p>}
      {view.runRevision < confirmedResolutionRevision.current && <p role="status">A confirmed resolution receipt is newer than this message snapshot. Refresh saved messages before recording another decision.</p>}
      <p className="swarm-supervision-note">{Object.entries(view.metrics).filter(([name]) => ['routineDecisions', 'latency', 'quality', 'throughput', 'cost'].includes(name)).map(([name, value]) => `${name}: ${typeof value === 'number' && Number.isFinite(value) ? `${value} reported; units and cohort unavailable` : value === null ? 'not measured' : 'unknown measurement shape'}`).join(' · ')}. No autonomy percentage is inferred.</p>
      <Attention key={`${run.id}:${view.runRevision}`} value={view} taskLink={taskLink} />
      <SwarmReviewPanel run={run} view={view} canRead={canRead} canEdit={canEdit} paused={paused} recoveryScope={recoveryScope} onWrite={onWrite} onRefresh={() => read(0, true, true)} onOpenTask={onOpenTask} messages={messages} />
      <ol className="swarm-message-list" aria-label="Saved attributed messages">{messages.map(message => {
        const sender = operations.get(message.sender.operationId), recipient = currentOperations.find(operation => operation.taskId === message.recipientTaskId);
        const knownSender = sender?.taskId === message.sender.taskId && sender.childId === message.sender.childId && sender.worktreeId === message.sender.worktreeId;
        const recordedResolution = view.questionResolutions?.find(item => item.questionId === message.id);
        return <li key={message.id}>
          <p><strong>{SWARM_MESSAGE_KINDS.includes(message.kind as SwarmMessageKind) ? message.kind : `Unknown message kind (${message.kind})`}</strong> · #{message.sequence} · {message.createdAt}</p>
          <p>{message.attribution === 'coordinator-recorded' ? 'Coordinator recorded this attributed claim for' : 'Owned worker reported this claim from'} task {message.sender.taskId} · attempt {message.sender.operationId}{!knownSender ? ' · saved sender evidence unavailable' : ''}</p>
          <p className="swarm-message-claim">{message.summary}</p>
          <p>To task {message.recipientTaskId}{message.causalMessageId ? ` · In reply to ${message.causalMessageId}` : ''}</p>
          {message.resolvesQuestionId && <p>Explicit decision resolving question {message.resolvesQuestionId}. This grants no execution authority.</p>}
          {message.kind === 'question' && <p>{view.questionResolutions === undefined ? 'Question resolution status unavailable from this server.' : recordedResolution ? `Resolution recorded by decision ${recordedResolution.resolutionMessageId}. The decision may be outside this message page.` : 'No explicit resolution recorded in this saved run snapshot.'}</p>}
          <p className="swarm-supervision-note">Evidence reference: {message.evidence.kind} {message.evidence.id || '(none)'} {message.evidence.digest}. References are not independently verified here.</p>
          {message.ack ? <p>Delivery receipt: {message.ack.actorId} · attempt {message.ack.operationId} · {message.ack.at}. Approval is separate.</p> : <p>No delivery receipt recorded.</p>}
          <div className="journey-actions">{taskLink(message.sender.taskId)}{taskLink(message.recipientTaskId)}
            {canEdit && <Button variant="tertiary" disabled={!admitted || !!pending || !recipient || !knownSender} onClick={() => {
              if (!recipient) return;
              setResolution(null);
              setSenderId(recipient.id); setRecipientId(message.sender.taskId); setCausalId(message.id);
              if (compose.current) compose.current.open = true;
              claimInput.current?.focus();
            }}>Draft reply to message {message.sequence}</Button>}
            {canEdit && message.kind === 'question' && !recordedResolution && <Button variant="secondary" disabled={view.questionResolutions === undefined || view.context.freshness !== 'current' || !admitted || !!pending || !recipient || recipient.allocation !== 'ready' || view.tasks.find(item => item.taskId === message.recipientTaskId)?.operationId !== recipient.id || !knownSender} onClick={() => {
              if (!recipient || !recoveryScope || view.questionResolutions === undefined) return;
              setResolution({question: message, operationId: recipient.id, childId: recipient.childId, worktreeId: recipient.worktreeId!, projectId: run.projectId, runId: run.id, scope: recoveryScope, runRevision: view.runRevision, ownerRevision: run.revision, stale: false});
              setSenderId(recipient.id); setRecipientId(message.sender.taskId); setKind('decision'); setCausalId(message.id); setSummary('');
              if (compose.current) compose.current.open = true;
              claimInput.current?.focus();
            }}>Resolve question {message.sequence}</Button>}
            {canEdit && !message.ack && <Button variant="secondary" disabled={!admitted || !!pending || !recipient || !knownSender} onClick={() => recipient && void send({type: 'ack', messageId: message.id, request: {expectedRevision: view.runRevision, recipientOperationId: recipient.id}})}>Record delivery receipt for message {message.sequence}</Button>}
          </div>
        </li>;
      })}</ol>
      {!messages.length && <p>No messages in this saved page. This does not establish completion or lack of attention.</p>}
      <p className="swarm-supervision-note">{messages.length} messages in this bounded window after #{windowStart} · {view.page.omitted} later messages. {view.page.hasMore ? 'More saved messages are available.' : 'No later page reported.'}</p>
      {view.page.hasMore && <Button variant="tertiary" disabled={paused || reading || writing} onClick={() => void read(view.page.nextCursor, messages.length + 50 > 250)}>Load next message page</Button>}
      {windowStart > 0 && <Button variant="tertiary" disabled={paused || reading || writing} onClick={() => void read(0, true)}>Return to first message page</Button>}
    </> : !reading && <p>No supervision snapshot available.</p>}
    {pending && <div className="swarm-supervision-pending"><p role="status">Exact {pending.type === 'post' ? 'message' : 'delivery'} request retained. Its outcome may be unconfirmed; new writes are blocked.</p>
      {pending.type === 'post' ? <p className="swarm-message-claim">Retained claim: {pending.request.summary}</p> : <p>Delivery for saved message {pending.messageId} requires exact readback, including when it is outside this page.</p>}
      <Button variant="secondary" disabled={writing || !canRead} onClick={() => void reconcilePending()}>Check exact saved outcome</Button>
      {retirable && canEdit && <Button variant="tertiary" disabled={writing} onClick={() => void retirePending()}>Close original request revision</Button>}
    </div>}
    {!canEdit ? <p className="swarm-supervision-note">Read-only access. Recording claims and delivery receipts requires current project edit authority.</p> : <details ref={compose} className="swarm-message-compose"><summary>Record an attributed message</summary>
      <p>This records a claim for a saved attempt. It does not send terminal input, answer an upstream permission prompt, or start an agent.</p>
      {!recoveryScope && <p>Account and workspace identity are required to retain a write safely.</p>}
      <label htmlFor={`${formId}-sender`}>Sender attempt<select id={`${formId}-sender`} value={senderId} disabled={!!pending || writing} onChange={event => { setSenderId(event.target.value); setResolution(value => value ? {...value, stale: true} : null); }}><option value="">Choose saved current attempt</option>{currentOperations.map(operation => <option key={operation.id} value={operation.id}>{tasks.get(operation.taskId!)?.title} · {operation.id}</option>)}</select></label>
      <label htmlFor={`${formId}-recipient`}>Recipient task<select id={`${formId}-recipient`} value={recipientId} disabled={!!pending || writing} onChange={event => { setRecipientId(event.target.value); setResolution(value => value ? {...value, stale: true} : null); }}><option value="">Choose saved task</option>{[...tasks.values()].map(task => <option key={task.id} value={task.id}>{task.title} ({task.id})</option>)}</select></label>
      <label htmlFor={`${formId}-kind`}>Claim kind<select id={`${formId}-kind`} value={kind} disabled={!!pending || writing} onChange={event => { setKind(event.target.value as SwarmMessageKind); setResolution(value => value ? {...value, stale: true} : null); }}>{SWARM_MESSAGE_KINDS.map(value => <option key={value} value={value}>{value}</option>)}</select></label>
      {causalId && <p>Reply to saved message {causalId}. This is a new attributed claim.</p>}
      {resolution && <p role="status">{resolutionReady() ? `Resolution draft for question ${resolution.question.id}. Record resolution explicitly to close its attention item; this does not verify the answer or deliver it to a running worker.` : 'Resolution draft is stale. Refresh saved messages and select the question again before recording a resolution.'}</p>}
      <label htmlFor={`${formId}-summary`}>Attributed claim<textarea ref={claimInput} id={`${formId}-summary`} value={summary} maxLength={1000} rows={4} disabled={!!pending || writing} onChange={event => setSummary(event.target.value)} /></label>
      <p className="swarm-supervision-note">{kind === 'review-finding' ? 'A review finding requires the selected owned reviewer attempt’s saved handoff. Resolution still checks the exact obligation and reported checks.' : 'No evidence receipt attached. A message alone cannot approve or verify work.'}</p>
      <p className="swarm-supervision-note">{new TextEncoder().encode(summary.trim()).length}/1,000 UTF-8 bytes</p>
      <Button variant="secondary" disabled={!admitted || !!pending || !!resolution && !resolutionReady() || !currentOperations.some(operation => operation.id === senderId) || !tasks.has(recipientId) || !summary.trim() || new TextEncoder().encode(summary.trim()).length > 1000 || kind === 'review-finding' && !operations.get(senderId)?.handoff?.digest} onClick={() => view && void send({type: 'post', operationId: senderId, request: {expectedRevision: view.runRevision, idempotencyKey: crypto.randomUUID(), recipientTaskId: recipientId, kind, causalMessageId: causalId, ...(resolution ? {resolvesQuestionId: resolution.question.id} : {}), evidence: kind === 'review-finding' ? {kind: 'handoff', id: senderId, digest: operations.get(senderId)!.handoff!.digest} : {kind: 'none', id: '', digest: ''}, summary: summary.trim()}})}>{resolution ? 'Record resolution' : 'Record message claim'}</Button>
    </details>}
  </section>;
}
function Attention({value, taskLink}: {value: SwarmSupervision; taskLink(id: string): ReactNode}) {
  const [page, setPage] = useState(0), current = Math.min(page, Math.max(0, Math.ceil(value.attention.length / 20) - 1));
  return <div className="swarm-supervision-attention"><h5>Recorded attention ({value.attention.length})</h5>
    <ul>{value.attention.slice(current * 20, current * 20 + 20).map((item, index) => <li key={`${item.id}:${index}`}>{['question', 'review', 'repair', 'unknown', 'blocked'].includes(item.kind) ? item.kind : 'Unknown attention kind'} · {item.id} {taskLink(item.taskId)}</li>)}</ul>
    {value.attention.length > 20 && <div className="journey-actions"><Button variant="tertiary" disabled={!current} onClick={() => setPage(current - 1)}>Previous attention page</Button><span>Page {current + 1}; other attention entries omitted from this page</span><Button variant="tertiary" disabled={(current + 1) * 20 >= value.attention.length} onClick={() => setPage(current + 1)}>Next attention page</Button></div>}
    {!value.attention.length && <p>No attention entries reported in this snapshot. Quality and settlement remain separate.</p>}
  </div>;
}
