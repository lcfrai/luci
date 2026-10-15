import { useRef, useState } from 'react';
import type { ProjectRun } from '../../api/projectRuns';
import { errorText } from '../../api/product';
import { projectSwarmApi, type SwarmReview } from '../../api/projectSwarm';
import { swarmHandoffApi, type TaskHandoffView } from '../../api/swarmHandoff';
import { swarmReviewObligationsApi } from '../../api/swarmReviewObligations';
import type { SwarmMessage, SwarmReviewObligation, SwarmSupervision } from '../../api/swarmSupervision';
import { Button } from '../fluid/components/button';
import { archiveSwarmReviewRecovery, clearSwarmReviewRecovery, readSwarmReviewRecovery, retainSwarmReviewRecovery, type PendingSwarmReview } from './swarmReviewRecovery';

interface Props {
  run: ProjectRun; view: SwarmSupervision; messages: SwarmMessage[];
  canRead: boolean; canEdit: boolean; paused: boolean; recoveryScope: string | null;
  onWrite(action: () => Promise<void>): Promise<void>; onRefresh(): Promise<void>; onOpenTask(id: string): void;
}
/** All decisions name saved attempts and receipts. Rendering never contacts a worker. */
export function SwarmReviewPanel({run, view, messages, canRead, canEdit, paused, recoveryScope, onWrite, onRefresh, onOpenTask}: Props) {
  const [candidateTaskId, setCandidateTaskId] = useState(''), [reviewerOperationId, setReviewerOperationId] = useState('');
  const [review, setReview] = useState<SwarmReview | null>(null), [selectedId, setSelectedId] = useState('');
  const [handoff, setHandoff] = useState<TaskHandoffView | null>(null), [findingId, setFindingId] = useState('');
  const [outcome, setOutcome] = useState<'passed' | 'repair-required'>('repair-required');
  const [busy, setBusy] = useState(false), [error, setError] = useState(''), [notice, setNotice] = useState('');
  const [retirable, setRetirable] = useState(false);
  const [initial] = useState(() => { try { return {pending: recoveryScope ? readSwarmReviewRecovery(recoveryScope) : null, error: ''}; } catch (cause) { return {pending: null, error: errorText(cause)}; } });
  const [pending, setPending] = useState<PendingSwarmReview | null>(initial.pending);
  const pendingRef = useRef(pending); pendingRef.current = pending;
  const tasks = run.swarm?.tasks ?? [], operations = run.operations;
  const candidate = tasks.find(task => task.id === candidateTaskId && task.role !== 'reviewer');
  const candidateOperation = operations.find(operation => operation.id === candidate?.operationId && operation.taskId === candidateTaskId);
  const reviewers = tasks.filter(task => task.role === 'reviewer' && task.reviewOfTaskId === candidateTaskId).flatMap(task => {
    const op = operations.find(operation => operation.id === task.operationId && operation.taskId === task.id);
    return op ? [{task, op}] : [];
  });
  const reviewer = reviewers.find(item => item.op.id === reviewerOperationId);
  const selected = view.reviewObligations.find(item => item.id === selectedId) ?? null;
  const selectedReviewer = selected && operations.find(operation => operation.id === selected.reviewerOperationId && operation.childId === selected.reviewerChildId);
  const findings = selected && selectedReviewer ? messages.filter(message => message.kind === 'review-finding' && message.sender.operationId === selectedReviewer.id &&
    message.recipientTaskId === selected.taskId && message.evidence.kind === 'handoff' && message.evidence.digest === selectedReviewer.handoff?.digest) : [];
  const ready = canRead && canEdit && !paused && !busy && !pending && !initial.error && !!recoveryScope && view.runRevision >= run.revision;
  async function inspectCandidate() {
    if (!candidate || !canRead || busy) return;
    setBusy(true); setError(''); setReview(null);
    try { setReview(await projectSwarmApi.review(run.projectId, run, candidate.id)); }
    catch (cause) { setError(errorText(cause)); }
    finally { setBusy(false); }
  }
  async function inspectObligation(item: SwarmReviewObligation) {
    if (!canRead || busy) return;
    setBusy(true); setError(''); setHandoff(null); setSelectedId(item.id); setFindingId('');
    try {
      const exact = await swarmReviewObligationsApi.get(run.projectId, run.id, item.id);
      if (exact.obligation.candidateOperationId !== item.candidateOperationId || exact.obligation.reviewerOperationId !== item.reviewerOperationId) throw new Error('The review identities changed. Refresh this run.');
      const op = operations.find(operation => operation.id === item.reviewerOperationId && operation.childId === item.reviewerChildId);
      if (op?.handoff) {
        const result = await swarmHandoffApi.get(run.projectId, run.id, op);
        if (result.digest !== op.handoff.digest) throw new Error('The reviewer result differs from the saved attempt. Refresh before resolving.');
        setHandoff(result);
      }
      setNotice('Exact saved review inspected. Worker reports remain attributed claims; combined checks are separate.');
    } catch (cause) { setError(errorText(cause)); }
    finally { setBusy(false); }
  }
  async function write(value: PendingSwarmReview) {
    if (!ready || !recoveryScope) return;
    await onWrite(async () => {
      setBusy(true); setError(''); setNotice('');
      try {
        retainSwarmReviewRecovery(recoveryScope, value); pendingRef.current = value; setPending(value); setRetirable(false);
        if (value.type === 'create') await swarmReviewObligationsApi.create(run.projectId, run.id, value.taskId, value.request);
        else await swarmReviewObligationsApi.resolve(run.projectId, run.id, value.obligationId, value.request);
        clearSwarmReviewRecovery(recoveryScope, value); pendingRef.current = null; setPending(null); setRetirable(false);
        setNotice(value.type === 'create' ? 'Exact independent review assignment saved. Start its reserved reviewer through normal consent.' : 'Reviewer finding recorded. Integration and checked completion remain separate.');
        await onRefresh();
      } catch (cause) { setError(`${errorText(cause)} The exact request remains retained until its outcome is read back. No automatic retry occurred.`); }
      finally { setBusy(false); }
    });
  }
  async function reconcile() {
    const value = pendingRef.current;
    if (!value || !recoveryScope || busy || !canRead) return;
    setBusy(true); setError(''); setNotice('');
    try {
      let state: 'recorded' | 'rejected' | 'key-conflict' | 'unconfirmed' = 'unconfirmed';
      let revision = value.request.expectedRevision;
      if (value.type === 'create') {
        const readback = await swarmReviewObligationsApi.disposition(run.projectId, run.id, value.taskId, value.request);
        state = readback.state; revision = readback.runRevision;
      } else {
        const readback = await swarmReviewObligationsApi.get(run.projectId, run.id, value.obligationId);
        revision = readback.runRevision;
        const receipt = readback.obligation;
        if (receipt.state === value.request.outcome && receipt.reviewerHandoffDigest === value.request.reviewerHandoffDigest && receipt.findingMessageId === value.request.findingMessageId) state = 'recorded';
        else if (receipt.state !== 'pending') state = 'key-conflict';
        else if (revision > value.request.expectedRevision) state = 'rejected';
      }
      if (state === 'recorded') {
        clearSwarmReviewRecovery(recoveryScope, value); pendingRef.current = null; setPending(null); setRetirable(false);
        setNotice('The exact review request is recorded in saved history.');
        await onRefresh();
      } else if (state === 'rejected' || state === 'key-conflict') {
        archiveSwarmReviewRecovery(recoveryScope, value, state, revision); pendingRef.current = null; setPending(null); setRetirable(false);
        setNotice('The original rejected request was preserved. Inspect current evidence before preparing a new request.');
        await onRefresh();
      } else { setRetirable(true); setNotice('The exact request can still be in flight. No replay was sent. You can explicitly close its original revision.'); }
    } catch (cause) { setError(errorText(cause)); }
    finally { setBusy(false); }
  }
  async function retirePending() {
    const value = pendingRef.current;
    if (!value || !retirable || !canEdit || !recoveryScope || busy) return;
    await onWrite(async () => {
      setBusy(true); setError('');
      try {
        const result = value.type === 'create'
          ? await swarmReviewObligationsApi.retire(run.projectId, run.id, value.taskId, value.request)
          : await swarmReviewObligationsApi.retireResolution(run.projectId, run.id, value.obligationId, value.request);
        archiveSwarmReviewRecovery(recoveryScope, value, 'rejected', result.runRevision); pendingRef.current = null; setPending(null); setRetirable(false);
        setNotice('The exact original review request was fenced and preserved as rejected. Inspect current evidence before a new request.');
        await onRefresh();
      } catch (cause) { setError(`${errorText(cause)} The original request remains retained; read back its outcome before new work.`); }
      finally { setBusy(false); }
    });
  }
  const validPass = !!handoff?.receipt?.reported && handoff.receipt.reported.reportedChecks.length > 0 &&
    handoff.receipt.reported.reportedChecks.every(check => check.outcome === 'passed') &&
    handoff.receipt.reported.questions.every(question => !question.blocking);
  const selectedHandoffDigest = handoff?.digest ?? '';
  return <div className="swarm-supervision-reviews"><h5>Independent review and repair</h5>
    <p className="swarm-supervision-note">A reviewer must be an explicit task in the approved execution contract. Its report and finding are evidence to inspect, not permission to integrate or deploy.</p>
    {(error || initial.error) && <p role="alert">{error || initial.error}</p>}{notice && <p role="status">{notice}</p>}
    {pending && <div className="swarm-supervision-pending"><p role="status">An exact {pending.type} request is retained with an unconfirmed outcome. New review writes are blocked.</p><Button variant="secondary" disabled={busy || !canRead} onClick={() => void reconcile()}>Check exact saved review outcome</Button>{retirable && canEdit && <Button variant="tertiary" disabled={busy} onClick={() => void retirePending()}>Close original review request revision</Button>}</div>}
    <details><summary>Assign an independent reviewer</summary>
      <label>Candidate coding task<select value={candidateTaskId} disabled={busy || !!pending} onChange={event => {setCandidateTaskId(event.target.value); setReviewerOperationId(''); setReview(null);}}><option value="">Choose a saved candidate</option>{tasks.filter(task => task.role !== 'reviewer').map(task => <option key={task.id} value={task.id}>{task.title}</option>)}</select></label>
      <label>Reserved reviewer attempt<select value={reviewerOperationId} disabled={busy || !!pending} onChange={event => setReviewerOperationId(event.target.value)}><option value="">Choose an approved reviewer</option>{reviewers.map(item => <option key={item.op.id} value={item.op.id}>{item.task.title} · {item.op.status} · {item.op.id}</option>)}</select></label>
      <div className="journey-actions"><Button variant="tertiary" disabled={!candidateOperation || busy || !canRead} onClick={() => void inspectCandidate()}>Inspect exact candidate diff</Button>{candidate && <Button variant="tertiary" onClick={() => onOpenTask(candidate.id)}>Open candidate task</Button>}</div>
      {review && <details><summary>Captured source diff · {review.sourceCommit.slice(0, 12)} · {review.independentReviewState ?? 'review state unknown'}</summary><p>{review.reason}</p><pre className="swarm-message-claim">{review.patch}</pre></details>}
      <Button variant="secondary" disabled={!ready || !reviewer || reviewer.op.status !== 'reserved' || reviewer.op.allocation !== 'ready' || !candidateOperation || !review || review.operationId !== candidateOperation.id || !review.reviewDigest || !review.patch || !view.context.contractDigest} onClick={() => review && reviewer && candidate && void write({type: 'create', taskId: candidate.id, request: {expectedRevision: view.runRevision, idempotencyKey: crypto.randomUUID(), candidateOperationId: review.operationId, reviewerOperationId: reviewer.op.id, reviewDigest: review.reviewDigest}})}>Save exact review assignment</Button>
    </details>
    <ul>{view.reviewObligations.map(item => <li key={item.id}><p><strong>{item.state}</strong> · candidate {item.taskId} · attempt {item.candidateOperationId} · reviewer {item.reviewerOperationId} · repairs {item.repairsConsumed}/{item.repairLimit}</p>
      <p className="swarm-supervision-note">Candidate {item.candidateCommit.slice(0, 12)} · context {item.contextDigest.slice(0, 12)} · review {item.reviewDigest.slice(0, 12)}{item.repairOperationId ? ` · repair attempt ${item.repairOperationId}` : ''}</p>
      <Button variant="tertiary" disabled={busy} onClick={() => void inspectObligation(item)}>Inspect exact review and result</Button><Button variant="tertiary" onClick={() => onOpenTask(item.taskId)}>Open candidate task</Button></li>)}</ul>
    {!view.reviewObligations.length && <p>No independent review obligation is saved for this run.</p>}
    {selected && <div><h6>Review {selected.id}</h6><p>Status {selected.state} · reviewer {selected.reviewerOperationId} · candidate {selected.candidateOperationId}</p>
      {handoff?.receipt?.reported ? <><p>Reviewer report: {handoff.receipt.reported.summary}</p><ul>{handoff.receipt.reported.reportedChecks.map((check, index) => <li key={index}>{check.outcome}: {check.command} · {check.detail}</li>)}</ul>{handoff.receipt.reported.questions.filter(question => question.blocking).map((question, index) => <p key={index}>Blocking question: {question.question}</p>)}</> : <p>The exact reviewer result is not yet saved or available. Resolve only after inspecting its handoff.</p>}
      {selected.state === 'pending' && selectedHandoffDigest !== '' && selectedHandoffDigest === selectedReviewer?.handoff?.digest && <><label>Finding linked to reviewer result<select value={findingId} onChange={event => setFindingId(event.target.value)}><option value="">Choose a saved finding</option>{findings.map(message => <option key={message.id} value={message.id}>{message.summary.slice(0, 100)} · #{message.sequence}</option>)}</select></label>
        {!findings.length && <p>Record a review-finding message from this reviewer using its saved handoff, then load the message page containing it.</p>}
        <label>Reviewer outcome<select value={outcome} onChange={event => setOutcome(event.target.value as 'passed' | 'repair-required')}><option value="repair-required">Repair required</option><option value="passed">Checks reported passed</option></select></label>
        <Button variant="secondary" disabled={!ready || !findingId || outcome === 'passed' && !validPass} onClick={() => void write({type: 'resolve', obligationId: selected.id, request: {expectedRevision: view.runRevision, outcome, reviewerHandoffDigest: selectedHandoffDigest, findingMessageId: findingId}})}>Record inspected reviewer outcome</Button>
      </>}
    </div>}
  </div>;
}
