import { useEffect, useRef, useState } from 'react';
import { coordinatorContinuationsApi, type CoordinatorContinuation, type CoordinatorContinuationRequest } from '../../api/coordinatorContinuations';
import { coordinatorPolicyApi, type CoordinatorPolicy } from '../../api/coordinatorPolicy';
import type { OrchestratorView } from '../../api/orchestrator';
import type { CreateSessionRequest } from '../../api/client';
import { ApiError } from '../../api/client';
import { errorText, ProductApiError } from '../../api/product';
import { MAX_SWARM_PARALLEL_TASKS } from '../../api/swarmLimits';
import { useCockpit } from '../../store/store';
import { selectWorktreeById } from '../../store/selectors';
import { Button } from '../fluid/components/button';
import { SessionPicker } from '../topbar/SessionPicker';

type Pending = { actorId: string; projectId: string; rejected?: true } & (
  { kind: 'create'; input: CoordinatorContinuationRequest } |
  { kind: 'advance'; id: string; revision: number } |
  { kind: 'policy'; revision: number }
);
const refused = (error: unknown) => error instanceof ProductApiError && [400, 401, 403, 409].includes(error.status);
const denied = (error: unknown) => error instanceof ApiError && [401, 403].includes(error.status);
const states = { ready: 'Ready to continue', continuing: 'Continuing', queued: 'Tasks queued', checking: 'Independent checks running', 'checks-verified': 'Independent checks verified', blocked: 'Needs attention', unknown: 'Outcome unconfirmed', refused: 'Needs attention' };
const steps = { 'create-run': 'Reviewed build', 'prepare-tasks': 'Task preparation', 'queue-tasks': 'Task dispatch' };
const keyFor = (scope: string) => `luci.coordinator-execution.v1:${scope}`;
function initialRequest(key: string, projectId: string, actorId: string) {
  try {
    const raw = localStorage.getItem(key);
    if (!raw) return { raw, value: null as Pending | null, error: '' };
    const value = JSON.parse(raw) as Pending;
    if (raw.length > 64000 || value.actorId !== actorId || value.projectId !== projectId || (value.rejected !== undefined && value.rejected !== true) || !['create', 'advance', 'policy'].includes(value.kind) ||
        value.kind === 'create' && (!value.input?.idempotencyKey || value.input.run?.implementationContext?.projectId !== projectId) ||
        value.kind === 'advance' && (!value.id || !Number.isSafeInteger(value.revision)) || value.kind === 'policy' && !Number.isSafeInteger(value.revision)) throw new Error('The retained continuation request cannot be read. Keep it for recovery; new work is paused.');
    return { raw, value, error: '' };
  } catch (cause) { return { raw: null, value: null as Pending | null, error: errorText(cause) }; }
}
/** Mount keyed to account/workspace/project scope; no credentials or approval grants are saved here. */
export function CoordinatorExecution({ projectId, scope, actorId, view, canRead, canEdit, canExecute, blocked, runOn, onInspect, onNavigate, policyEpoch = 0, onPolicyUpdated }: {
  projectId: string; scope: string; actorId: string; view: OrchestratorView;
  canRead: boolean; canEdit: boolean; canExecute: boolean; blocked: string; runOn: string;
  policyEpoch?: number; onPolicyUpdated?(): void;
  onInspect(runId: string): void; onNavigate(lens: OrchestratorView['nextAction']['lens']): void;
}) {
  const key = keyFor(scope), [initial] = useState(() => initialRequest(key, projectId, actorId));
  const [pending, setPending] = useState(initial.value), pendingRef = useRef(initial.value), rawRef = useRef(initial.raw);
  const [storageError, setStorageError] = useState(initial.error);
  const [rows, setRows] = useState<CoordinatorContinuation[]>([]), [selected, setSelected] = useState('');
  const [policy, setPolicy] = useState<CoordinatorPolicy | null>(null), [fresh, setFresh] = useState(false);
  const [error, setError] = useState(''), [notice, setNotice] = useState(''), [busy, setBusy] = useState(''), [readDenied, setReadDenied] = useState(false);
  const [consent, setConsent] = useState(false), [picker, setPicker] = useState<CoordinatorContinuationRequest | null>(null);
  const [parallel, setParallel] = useState(2), [outcome, setOutcome] = useState<'preview' | 'publish'>('preview');
  const worktreeId = useCockpit(state => state.selection.worktreeId), worktree = useCockpit(selectWorktreeById(worktreeId ?? ''));
  const alive = useRef(true), generation = useRef(0), working = useRef(false), readController = useRef<AbortController | null>(null);
  const latest = useRef({ canRead, canEdit, canExecute, blocked, view, worktree }); latest.current = { canRead, canEdit, canExecute, blocked, view, worktree };
  function persist(value: Pending | null) {
    try {
      if (storageError || localStorage.getItem(key) !== rawRef.current) throw new Error('Another window changed the retained continuation. Reopen this project before starting more work.');
      const raw = value ? JSON.stringify(value) : null;
      if (raw) localStorage.setItem(key, raw); else localStorage.removeItem(key);
      rawRef.current = raw; pendingRef.current = value; setPending(value);
    } catch (cause) { setStorageError(errorText(cause)); throw cause; }
  }
  const current = (ticket: number) => alive.current && latest.current.canRead && ticket === generation.current;
  function remember(value: CoordinatorContinuation) {
    setRows(previous => [value, ...previous.filter(row => row.id !== value.id)]); setSelected(value.id);
  }
  async function load(explicit: boolean) {
    if (!latest.current.canRead || working.current) return;
    readController.current?.abort();
    const controller = new AbortController(); readController.current = controller; const signal = controller.signal;
    const ticket = ++generation.current; setFresh(false); setConsent(false); setBusy('read');
    try {
      const [nextPolicy, nextRows] = await Promise.all([coordinatorPolicyApi.get(projectId, signal), coordinatorContinuationsApi.list(projectId, actorId, signal)]);
      if (!current(ticket) || signal?.aborted) return;
      const retained = pendingRef.current;
      if (explicit && retained) {
        const match = retained.kind === 'create' ? nextRows.find(row => row.idempotencyKey === retained.input.idempotencyKey && row.originTurnId === retained.input.originTurnId && row.contextDigest === retained.input.expectedContextDigest && row.policyRevision === retained.input.expectedPolicyRevision) : retained.kind === 'advance' ? nextRows.find(row => row.id === retained.id && row.revision > retained.revision && !row.actions.some(action => action.status === 'pending' || action.status === 'unknown')) : undefined;
        if (retained.rejected || match || retained.kind === 'policy' && nextPolicy.revision > retained.revision) { persist(null); setNotice(match ? 'The exact retained request was observed. It was not submitted again.' : 'Current authoritative state was read. The retained request was not replayed.'); }
      }
      setPolicy(nextPolicy); setRows(nextRows); setSelected(previous => nextRows.some(row => row.id === previous) ? previous : nextRows[nextRows.length - 1]?.id ?? '');
      setFresh(true); setReadDenied(false); setError('');
    } catch (cause) { if (current(ticket) && !signal?.aborted) { setError(errorText(cause)); if (denied(cause)) { setRows([]); setPolicy(null); setReadDenied(true); } } }
    finally { if (readController.current === controller) readController.current = null; if (current(ticket)) setBusy(''); }
  }
  useEffect(() => { alive.current = true; return () => { alive.current = false; generation.current++; readController.current?.abort(); }; }, []);
  useEffect(() => {
    generation.current++; setConsent(false); setPicker(null); setFresh(false);
    if (!canRead) { setRows([]); setPolicy(null); return; }
    void load(false);
    return () => readController.current?.abort();
  }, [projectId, actorId, canRead, canEdit, canExecute, view.context.digest, policyEpoch]);
  async function mutate<T>(label: string, record: Pending, operation: () => Promise<T>, accept: (value: T) => void) {
    if (!fresh || !latest.current.canRead || !latest.current.canEdit || !latest.current.canExecute || working.current || pendingRef.current || storageError) return false;
    try { persist(record); } catch { return false; }
    working.current = true; const ticket = ++generation.current; setBusy(label); setConsent(false); setError('');
    try {
      const value = await operation();
      if (!current(ticket) || !latest.current.canEdit) return false;
      persist(null); accept(value); return true;
    } catch (cause) {
      if (current(ticket)) {
        if (refused(cause)) { try { persist({ ...record, rejected: true }); } catch { /* Keep uncertainty. */ } }
        setError(`${errorText(cause)} Read the exact retained outcome before another request; this action will not be replayed automatically.`);
      }
      return false;
    } finally { working.current = false; if (alive.current) setBusy(''); }
  }
  async function enableExecution() {
    if (!policy || !consent || !latest.current.canExecute) return;
    await mutate('policy', { kind: 'policy', actorId, projectId, revision: policy.revision }, () => coordinatorPolicyApi.save(projectId, { expectedRevision: policy.revision, mode: policy.mode, allowedTargets: policy.allowedTargets, executionMode: 'reviewed-tasks' }), value => { setPolicy(value); onPolicyUpdated?.(); setNotice('Reviewed-task continuation is enabled. Saved context approvals, current access and resource limits still apply. Deployment remains separately controlled.'); });
  }
  const ownTurn = [...view.turns].reverse().find(turn => turn.actor.id === actorId);
  const tasks = view.context.plan.tasks.filter(task => task.status !== 'done');
  const issue = blocked || !canEdit ? blocked || 'Editing access is required to continue implementation.' : !canExecute ? 'Execution access is required to continue implementation.' : !fresh ? 'Read current continuation and policy status first.' : !ownTurn || ownTurn.status !== 'succeeded' || !ownTurn.answer || ownTurn.answer.blocking || ownTurn.answer.proposal && !ownTurn.appliedRevision ? 'Complete and save this conversation’s amendment before implementation.' : view.blockingQuestions.length ? 'Resolve the material project questions first.' : view.context.specification.status !== 'approved' || view.context.designPlan.status !== 'approved' ? 'Review and approve the current definition and design first.' : view.context.plan.briefRevision !== view.context.brief.revision || view.context.plan.specificationRevision !== view.context.specification.revision ? 'Review the implementation plan against the current approved context.' : !tasks.length ? 'No unfinished saved tasks are selected for this amendment.' : !worktree?.isPrimary || worktree.projectId !== projectId ? 'Choose this project’s main checkout before configuring continued work.' : policy?.executionMode !== 'reviewed-tasks' ? 'Enable reviewed-task continuation before starting work.' : rows.some(row => row.originTurnId === ownTurn.id) ? 'This conversation already has a retained continuation. Inspect its recorded work instead of starting it again.' : '';
  const row = rows.find(value => value.id === selected);
  const start = () => {
    if (issue || pendingRef.current || working.current || storageError || !policy || !ownTurn || !worktree) return;
    setParallel(Math.min(2, tasks.length)); setOutcome('preview');
    setPicker({ idempotencyKey: crypto.randomUUID(), originTurnId: ownTurn.id, expectedContextDigest: view.context.digest, expectedPolicyRevision: policy.revision, maxParallel: Math.min(2, tasks.length), run: { worktreeId: worktree.id, harness: '', model: '', permissionMode: 'ask', outcome: 'preview', implementationContext: { projectId, briefRevision: view.context.brief.revision, planRevision: view.context.plan.revision, specificationRevision: view.context.specification.revision, designPlanRevision: view.context.designPlan.revision, taskIds: tasks.map(task => task.id) } } });
  };
  async function create(selection: CreateSessionRequest) {
    if (!picker || issue || latest.current.blocked || picker.expectedContextDigest !== latest.current.view.context.digest || !selection.implementationContext || JSON.stringify(selection.implementationContext) !== JSON.stringify(picker.run.implementationContext)) return { ok: false as const, message: 'The reviewed selection changed. Close this picker and read the current context before continuing.' };
    if (!Number.isInteger(parallel) || parallel < 1 || parallel > Math.min(MAX_SWARM_PARALLEL_TASKS, tasks.length)) return { ok: false as const, message: 'Choose a concurrency limit within the selected task count.' };
    const input = { ...picker, maxParallel: parallel, run: { ...picker.run, worktreeId: selection.worktreeId, harness: selection.harness ?? '', model: selection.model ?? '', permissionMode: selection.permissionMode ?? 'ask', ...(selection.effort ? { effort: selection.effort } : {}), ...(selection.agentConnectionId !== undefined ? { agentConnectionId: selection.agentConnectionId } : {}), outcome } };
    const ok = await mutate('create', { kind: 'create', projectId, actorId, input }, () => coordinatorContinuationsApi.create(projectId, actorId, input), value => { remember(value); setPicker(null); setNotice('The continuation is retained. Queued work is not yet verified or deployed.'); });
    return ok ? { ok: true as const } : { ok: false as const, message: 'The continuation outcome needs readback in Luci. Close this picker and refresh continued work; do not submit it again.' };
  }
  async function advance() {
    if (!row || !canExecute || blocked) return;
    await mutate('advance', { kind: 'advance', actorId, projectId, id: row.id, revision: row.revision }, () => coordinatorContinuationsApi.advance(projectId, actorId, row.id, row.revision), remember);
  }
  if (!canRead) return null;
  return <section aria-label="Continued work" className="orchestrator-agent">
    <h3>Continue with Luci</h3>
    <p>Run on: {runOn}. Luci uses the selected upstream harness and the project’s reviewed tasks. Deploy to is chosen separately.</p>
    {error && <p role="alert">{error}</p>}{storageError && <p role="alert">{storageError}</p>}{notice && <p role="status">{notice}</p>}
    {pending && <p role="status">A continuation request is retained. New requests are paused until its exact outcome is read back.</p>}
    {readDenied ? <p role="alert">Current access was refused. Retained private work is hidden.</p> : <>
      {rows.length > 0 && <label className="luci-field">Retained continuation<select value={selected} onChange={event => setSelected(event.target.value)}>{rows.map(value => <option value={value.id} key={value.id}>{states[value.status]} · {value.originTurnId}</option>)}</select></label>}
      {row && <article aria-label="Selected continuation"><p role="status"><strong>{states[row.status]}</strong> · {row.message}</p>
        {row.contextDigest !== view.context.digest && <p>The current context observation differs from this work’s captured snapshot. Luci rechecks the exact reviewed selection before continuing; it does not replace captured context.</p>}
        <ol>{row.actions.map(action => <li key={action.id}>{steps[action.kind]} · {action.status}{action.message ? ` — ${action.message}` : ''}</li>)}</ol>
        {row.queueIds.length > 0 && <p>{row.queueIds.length} queued task reservations. Completion, independent checks and release evidence remain separate.</p>}
        {row.delivery && <section aria-label="Independent check evidence"><p>{row.delivery.status} · {row.delivery.message}</p><p>Check {row.delivery.checkId} · source {row.delivery.commit.slice(0, 12)}. Preview, release and deployment remain separate.</p></section>}
        {row.runId && <Button variant="secondary" onClick={() => { if (latest.current.canRead && !readDenied) onInspect(row.runId!); }}>Inspect this build</Button>}
        {row.status !== 'checks-verified' && <Button variant="secondary" disabled={!canEdit || !canExecute || !fresh || !!busy || !!pending || !!storageError || !!blocked} onClick={() => void advance()}>Continue retained work</Button>}
      </article>}
      {policy && policy.executionMode !== 'reviewed-tasks' && <details><summary>Implementation permission</summary><p>Allow Luci to prepare and queue reviewed tasks with your chosen upstream route. Current access, approvals, resource and provider limits still apply. This does not authorize deployment or change draft policy.</p><label className="orchestrator-ack"><input type="checkbox" checked={consent} disabled={!canEdit || !canExecute || !fresh || !!busy || !!pending} onChange={event => setConsent(event.target.checked)} />Allow reviewed-task continuation in this project.</label><Button variant="secondary" disabled={!consent || !canEdit || !canExecute || !fresh || !!busy || !!pending || !!storageError} onClick={() => void enableExecution()}>Enable reviewed-task continuation</Button></details>}
      {issue && <p>{issue}</p>}
      <Button variant="primary" disabled={!!issue || !!pending || !!busy || !!storageError} onClick={start}>Configure continued work</Button>
      {(view.context.specification.status !== 'approved' || view.context.designPlan.status !== 'approved') && <Button variant="ghost" onClick={() => onNavigate(view.context.specification.status !== 'approved' ? 'discover' : 'design')}>Review current context</Button>}
    </>}
    <Button variant="ghost" disabled={!!busy} onClick={() => void load(true)}>Refresh continued work</Button>
    {busy && <p role="status">{busy === 'read' ? 'Reading retained continuation and current policy…' : busy === 'policy' ? 'Saving implementation permission…' : 'Continuing exact reviewed work…'}</p>}
    <SessionPicker open={!!picker && canRead && canEdit && canExecute && !readDenied} worktreeId={picker?.run.worktreeId ?? null} implementationContext={picker?.run.implementationContext} onClose={() => setPicker(null)} onSubmit={create} submitLabel="Start reviewed continuation" progressLabel="Recording this continuation and preparing its reviewed tasks. Resources may be allocated under the current limits." failureGuidance="Read the exact retained request outcome in Luci before another start." onCreated={() => setPicker(null)}>
      <p>Run on: {runOn}. This starts only the selected saved tasks; successful deployment is not implied.</p>
      <label className="luci-field">Maximum parallel tasks<input type="number" min={1} max={Math.min(MAX_SWARM_PARALLEL_TASKS, picker?.run.implementationContext.taskIds.length ?? 1)} value={parallel} onChange={event => setParallel(Number(event.target.value))} /></label>
      <label className="luci-field">Intended delivery outcome<select value={outcome} onChange={event => setOutcome(event.target.value as typeof outcome)}><option value="preview">Working preview</option><option value="publish">Published application</option></select></label>
      <p>Choosing an outcome does not approve deployment, grant cloud spend or bypass release checks.</p>
    </SessionPicker>
  </section>;
}
