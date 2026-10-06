import { useEffect, useRef, useState } from 'react';
import { api } from '../../api';
import { errorText, productApi, type CheckRun, type Release } from '../../api/product';
import { platformApi } from '../../api/platform';
import { projectRunsApi, runSessionRequest, type CreateProjectRun, type ProjectRun, type RunOperation } from '../../api/projectRuns';
import type { CreateSessionRequest } from '../../api/client';
import type { ImplementationContextRequest } from '../../store/types';
import { useCockpit } from '../../store/store';
import { useAuth } from '../auth/AuthProvider';
import { useSetup } from '../setup/SetupProvider';
import { Button } from '../fluid/components/button';
import { WorkflowDialog } from '../common/WorkflowDialog';
import { SessionPicker } from '../topbar/SessionPicker';
import './project-runs.css';

export interface ProjectRunDraft { implementationContext: ImplementationContextRequest; worktreeId: string }
type Dialog = 'join-check' | 'join-release' | 'accept' | 'cancel';
type ReleaseChoice = Pick<Release, 'id' | 'projectId' | 'commit' | 'status' | 'target'>;
const releaseKey = (release: ReleaseChoice) => JSON.stringify([release.target, release.id]);
const labels: Record<string, string> = {
  ready: 'Ready to code', coding: 'Coding', 'awaiting-checks': 'Awaiting verification',
  'awaiting-release': 'Awaiting release evidence', blocked: 'Needs attention', cancelled: 'Cancelled', complete: 'Accepted outcome',
};
const when = (value: string) => new Date(value).toLocaleString();
const scope = (values: unknown[]) => JSON.stringify(values);

export function ProjectRuns(props: { projectId: string; draft?: ProjectRunDraft; onDraftClose(): void; onReviewPlan(): void }) {
  const auth = useAuth(), setup = useSetup();
  const identity = scope([productApi.baseUrl, auth?.profile.user.id, setup?.workspace, setup?.status?.access?.user, props.projectId]);
  const prepared = useRef({ draft: props.draft, identity });
  if (prepared.current.draft !== props.draft) prepared.current = { draft: props.draft, identity };
  // Discard transient requests, errors, selection and confirmations on account/workspace changes.
  return <RunWorkspace key={identity} {...props} draft={prepared.current.identity === identity ? props.draft : undefined} />;
}

function RunWorkspace({ projectId, draft, onDraftClose, onReviewPlan }: {
  projectId: string; draft?: ProjectRunDraft; onDraftClose(): void; onReviewPlan(): void;
}) {
  const [runs, setRuns] = useState<ProjectRun[]>([]), [selected, setSelected] = useState('');
  const [loading, setLoading] = useState(true), [busy, setBusy] = useState('');
  const [error, setError] = useState(''), [notice, setNotice] = useState('');
  const [dispatchUncertain, setDispatchUncertain] = useState(false);
  const [dialog, setDialog] = useState<Dialog | null>(null), [choice, setChoice] = useState('');
  const [checks, setChecks] = useState<CheckRun[]>([]), [releases, setReleases] = useState<ReleaseChoice[]>([]);
  const [evidenceNotice, setEvidenceNotice] = useState('');
  const [accepted, setAccepted] = useState(false), [outcome, setOutcome] = useState<'preview' | 'publish'>('preview');
  const alive = useRef(true), working = useRef(false), request = useRef(0);
  const creation = useRef<{ signature: string; key: string } | null>(null);
  const operationKeys = useRef(new Map<string, string>());
  const run = runs.find(item => item.id === selected) ?? runs[0];
  const setLens = useCockpit(state => state.setLens);
  const allowed = (action: string) => !!run?.allowedActions?.includes(action) && !(dispatchUncertain && (action === 'start-coding' || action === 'continue-coding'));
  const remember = (value: ProjectRun) => {
    if (!alive.current) return;
    if (value.projectId !== projectId || value.input.implementation.projectId !== projectId) throw new Error('The run belongs to another project. Refresh this project.');
    setRuns(current => [value, ...current.filter(item => item.id !== value.id)].sort((a, b) => b.createdAt.localeCompare(a.createdAt)));
    setSelected(value.id);
  };
  async function load() {
    const mine = ++request.current;
    setLoading(true);
    try {
      const values = await projectRunsApi.list(projectId);
      if (!alive.current || mine !== request.current) return;
      if (values.some(value => value.projectId !== projectId || value.input.implementation.projectId !== projectId)) throw new Error('The run history belongs to another project. Refresh this project.');
      setRuns(values); setError(''); setDispatchUncertain(false);
    } catch (cause) { if (alive.current && mine === request.current) setError(errorText(cause)); }
    finally { if (alive.current && mine === request.current) setLoading(false); }
  }
  useEffect(() => {
    alive.current = true; void load();
    return () => { alive.current = false; request.current++; };
  }, [projectId]);
  useEffect(() => { if (draft) { setOutcome('preview'); creation.current = null; } }, [draft]);

  async function operate(label: string, action: () => Promise<void>) {
    if (working.current) return;
    working.current = true; request.current++; setLoading(false); setBusy(label); setError(''); setNotice('');
    try { await action(); }
    catch (cause) {
      if (alive.current) setError(`${errorText(cause)} Refresh history and reconcile the recorded run before retrying an uncertain operation.`);
    } finally { working.current = false; if (alive.current) setBusy(''); }
  }
  async function create(selection: CreateSessionRequest) {
    if (!draft || !selection.implementationContext || selection.implementationContext.projectId !== projectId) return { ok: false as const, message: 'Review this project’s saved plan first.' };
    if (working.current) return { ok: false as const, message: 'A run operation is already in progress.' };
    const input: Omit<CreateProjectRun, 'idempotencyKey'> = {
      worktreeId: selection.worktreeId, harness: selection.harness || '', model: selection.model || '',
      permissionMode: selection.permissionMode || 'ask', ...(selection.effort ? { effort: selection.effort } : {}),
      ...(selection.agentConnectionId !== undefined ? { agentConnectionId: selection.agentConnectionId } : {}),
      outcome, implementationContext: selection.implementationContext,
    };
    const signature = JSON.stringify(input);
    if (creation.current?.signature !== signature) creation.current = { signature, key: crypto.randomUUID() };
    working.current = true;
    try {
      const value = await projectRunsApi.create(projectId, { ...input, idempotencyKey: creation.current.key });
      if (!alive.current) return { ok: false as const, message: 'The workspace changed. Open its run history to inspect the saved result.' };
      request.current++; setLoading(false); remember(value);
      setNotice('Reviewed run saved. Start coding when ready; no session or deployment has started.');
      return { ok: true as const };
    } catch (cause) { return { ok: false as const, message: errorText(cause) }; }
    finally { working.current = false; }
  }
  async function startCoding() {
    if (!run || !allowed('start-coding')) return;
    const keyScope = `${run.id}:${run.revision}`;
    let key = operationKeys.current.get(keyScope);
    if (!key) { key = crypto.randomUUID(); operationKeys.current.set(keyScope, key); }
    const reserved = await projectRunsApi.reserve(projectId, run.id, run.revision, key);
    if (!alive.current) return; // Persisted reservation remains observable; never dispatch after switching scope.
    remember(reserved.run);
    await dispatch(reserved.run, reserved.operation);
  }
  async function dispatch(saved: ProjectRun, operation: RunOperation) {
    setDispatchUncertain(true);
    const session = await api.createSession(runSessionRequest(saved, operation));
    if (!alive.current) return;
    if (session.id !== operation.childId || session.projectId !== projectId || session.projectRunId !== saved.id || session.projectRunOperationId !== operation.id) throw new Error('The returned session did not match its reservation. No further coding operation was requested.');
    useCockpit.getState().upsertSession(session);
    const value = await projectRunsApi.get(projectId, saved.id);
    if (alive.current) { remember(value); setDispatchUncertain(false); setNotice('Coding session started. Session exit and task progress do not verify the outcome.'); }
  }
  async function continueCoding() {
    if (!run || !allowed('continue-coding')) return;
    const reserved = run.operations.filter(item => item.kind === 'session' && item.status === 'reserved');
    if (reserved.length !== 1) throw new Error('The reserved operation is ambiguous. Reconcile this run before continuing.');
    await dispatch(run, reserved[0]);
  }
  async function openSession() {
    const child = run?.observation?.childId || run?.operations[run.operations.length - 1]?.childId;
    if (!child || !run) return;
    const session = await api.getSession(child);
    if (!alive.current) return;
    if (session.projectId !== projectId || session.projectRunId !== run.id || !run.operations.some(item => item.id === session.projectRunOperationId && item.childId === session.id)) throw new Error('This coding session does not belong to the selected run.');
    const state = useCockpit.getState();
    state.upsertSession(session); state.selectBranch(projectId, session.branchId);
    const current = useCockpit.getState();
    if (current.selection.projectId === projectId && current.selection.branchId === session.branchId) { current.selectWorktree(session.worktreeId); current.focusSession(session.id); }
  }
  async function chooseEvidence(kind: 'join-check' | 'join-release') {
    if (!allowed(kind)) return;
    setChoice(''); setEvidenceNotice('');
    if (kind === 'join-check') {
      const value = await productApi.checks(projectId);
      if (!alive.current) return;
      if (value.runs.some(item => item.projectId !== projectId)) throw new Error('Check history belongs to another project.');
      setChecks(value.runs);
    } else {
      const [standard, native] = await Promise.allSettled([productApi.releases(projectId), platformApi.project(projectId)]);
      if (!alive.current) return;
      if (standard.status === 'rejected' && native.status === 'rejected') throw new Error(`Release history is unavailable: ${errorText(standard.reason)} ${errorText(native.reason)}`);
      const values: ReleaseChoice[] = standard.status === 'fulfilled' ? [...standard.value.releases] : [];
      if (native.status === 'fulfilled') {
        if (native.value.project.projectId !== projectId) throw new Error('Deployment history belongs to another project.');
        values.push(...native.value.project.deployments.map(item => ({ id: item.id, projectId: item.projectId, commit: item.commit, status: item.status, target: `native-${item.provider}` })));
      }
      if (values.some(item => item.projectId !== projectId)) throw new Error('Release history belongs to another project.');
      setReleases(values);
      if (standard.status === 'rejected' || native.status === 'rejected') setEvidenceNotice('Some deployment history could not be loaded. Available results remain selectable; close and reopen this chooser to retry.');
    }
    setDialog(kind);
  }
  async function confirm() {
    if (!run || !dialog || !allowed(dialog)) return;
    let value: ProjectRun;
    if (dialog === 'accept') {
      if (!accepted || !run.observation?.valid) return;
      value = await projectRunsApi.accept(projectId, run.id, run.revision);
    } else if (dialog === 'cancel') value = await projectRunsApi.cancel(projectId, run.id, run.revision);
    else if (dialog === 'join-check') {
      if (!checks.some(item => item.id === choice)) return;
      value = await projectRunsApi.joinCheck(projectId, run.id, run.revision, choice);
    } else {
      const release = releases.find(item => releaseKey(item) === choice);
      if (!release) return;
      value = await projectRunsApi.joinRelease(projectId, run.id, run.revision, release.id, release.target);
    }
    if (alive.current) { remember(value); setDialog(null); }
  }
  const primary = allowed('accept') && run?.observation?.valid ? 'accept'
    : allowed('continue-coding') ? 'continue-coding'
      : run?.state === 'ready' && allowed('start-coding') ? 'start-coding'
      : run?.state === 'awaiting-checks' && allowed('join-check') ? 'join-check'
        : run?.state === 'awaiting-release' && allowed('join-release') ? 'join-release'
          : allowed('reconcile') ? 'reconcile' : '';
  function action(kind: string) {
    if (!run || !allowed(kind)) return;
    if (kind === 'start-coding') void operate('Starting coding', startCoding);
    else if (kind === 'continue-coding') void operate('Continuing reserved coding', continueCoding);
    else if (kind === 'reconcile') void operate('Reconciling run', async () => { remember(await projectRunsApi.reconcile(projectId, run.id, run.revision)); if (alive.current) setDispatchUncertain(false); });
    else if (kind === 'join-check' || kind === 'join-release') void operate('Reading available evidence', () => chooseEvidence(kind));
    else if (kind === 'accept' || kind === 'cancel') { setAccepted(false); setDialog(kind); }
  }
  const actionLabel: Record<string, string> = { 'start-coding': 'Start coding', 'continue-coding': 'Continue reserved coding', 'join-check': 'Select check evidence', 'join-release': 'Select release evidence', reconcile: 'Reconcile run', accept: 'Review and accept outcome', cancel: 'Cancel run' };
  return <section className="project-runs" aria-label="Project runs" aria-busy={!!busy || loading}>
    <div className="project-runs-heading"><h2>Project run</h2><Button variant="tertiary" disabled={!!busy || loading} onClick={() => void load()}>{loading ? 'Reading history…' : 'Refresh history'}</Button></div>
    {error && <p role="alert" className="journey-error">{error}</p>}
    {notice && <p role="status" className="journey-muted">{notice}</p>}
    {!run ? <><p className="journey-muted">Keep reviewed tasks, coding, checks and a verified release together. Creating a run saves its inputs; each next operation remains your choice.</p><Button variant="secondary" onClick={onReviewPlan}>Review implementation plan</Button></> : <>
      <div className="project-run-next"><strong>{labels[run.state] || run.state}</strong><span>{run.input.outcome === 'preview' ? 'Workspace preview' : 'Public deployment'}</span><p>{run.reason}</p>
        {run.acceptance && <p>Accepted {when(run.acceptance.acceptedAt)}. This historical acceptance is retained separately from current validity.</p>}
        {run.observation ? <p className={run.observation.valid ? 'journey-muted' : 'journey-warning'}>{run.observation.valid ? 'Last observation valid' : 'Current evidence needs attention'} · {when(run.observation.observedAt)}. {run.observation.reason}</p> : <p className="journey-muted">No observation recorded yet. Refreshing history does not run or reconcile operations.</p>}
        <div className="journey-actions">
          {primary && <Button disabled={!!busy || loading} onClick={() => action(primary)}>{busy || actionLabel[primary]}</Button>}
          {!!run.operations.length && <Button variant="secondary" disabled={!!busy} onClick={() => void operate('Opening coding session', openSession)}>Open coding session</Button>}
          {allowed('reconcile') && primary !== 'reconcile' && <Button variant="secondary" disabled={!!busy || loading} onClick={() => action('reconcile')}>Reconcile run</Button>}
          {allowed('join-check') && primary !== 'join-check' && <Button variant="tertiary" disabled={!!busy || loading} onClick={() => action('join-check')}>Select check evidence</Button>}
        </div>
        {run.state === 'awaiting-checks' && <p className="journey-muted">Run reviewed commands in the checks panel below, then select their evidence here. A CLI exit or task marked Done does not complete this run.</p>}
        {allowed('continue-coding') && <p className="journey-muted">A coding operation was reserved before dispatch. Continue uses that same child identity; it does not reserve another attempt. Reconcile first if its result is uncertain.</p>}
        {run.state === 'awaiting-release' && <Button variant="tertiary" disabled={!!busy} onClick={() => setLens('deploy')}>Open deployment controls</Button>}
        {run.state === 'cancelled' && <p className="journey-muted">Future dispatch is cancelled. Running children retain their separate stop controls.</p>}
      </div>
      <details className="project-run-receipts"><summary>Reviewed inputs and evidence</summary>
        <p>{run.input.implementation.taskTitles.join('; ')}</p><p>Brief {run.input.implementation.briefRevision} · Plan {run.input.implementation.planRevision} · Definition {run.input.implementation.specificationRevision} · Design {run.input.implementation.designPlanRevision}</p>
        <p>{run.input.route.harness} · {run.input.route.model} · {run.input.route.connection.id || 'Direct agent settings'} · connection revision {run.input.route.connection.revision}</p>
        <p>Starting commit {run.input.implementation.sourceCommit || 'not committed'}{run.input.implementation.sourceDirty ? ' · included uncommitted changes' : ''}</p>
        {run.input.requirements.map(item => <p key={item.id}><strong>{item.id} · {item.title}</strong><br />{item.acceptance}</p>)}
        <h3>Coding attempts</h3>{!run.operations.length && <p>No coding operation reserved.</p>}
        <ul>{run.operations.map(item => <li key={item.id}>Attempt {item.attempt} · {item.status} · {item.childId}<p>{item.message}</p></li>)}</ul>
        {allowed('start-coding') && primary !== 'start-coding' && <Button variant="secondary" disabled={!!busy || loading} onClick={() => action('start-coding')}>Start another coding attempt</Button>}
        <h3>Check evidence</h3>{!run.checks.length && <p>No accepted check evidence.</p>}
        {run.checks.map(item => <div key={item.checkId}><p>{item.checkId} · {item.commit.slice(0, 12)} · {item.execution}</p>{!item.originalAvailable && <p className="journey-muted">Original check history is unavailable; the saved evidence snapshot is retained.</p>}<ul>{item.commands.map((command, index) => <li key={index}>{command.label} · {command.status} · exit {command.exitCode}</li>)}</ul></div>)}
        <h3>Release evidence</h3>{!run.releases.length && <p>No accepted release evidence.</p>}
        {run.releases.map(item => <p key={item.releaseId}>{item.releaseId} · {item.target} · {item.commit.slice(0, 12)}<br />Observed {when(item.observedAt)}. {item.limitations}</p>)}
        {allowed('cancel') && <Button variant="tertiary" disabled={!!busy || loading} onClick={() => action('cancel')}>Cancel run</Button>}
      </details>
    </>}
    {runs.length > 0 && <details className="project-run-history"><summary>Run history · {runs.length}</summary><ul>{runs.map(item => <li key={item.id}><Button variant={item.id === run?.id ? 'secondary' : 'tertiary'} disabled={!!busy} aria-pressed={item.id === run?.id} onClick={() => { setSelected(item.id); setDialog(null); setError(''); setNotice(''); }}>{labels[item.state] || item.state} · {when(item.createdAt)}</Button><span>{item.input.outcome === 'preview' ? 'Workspace preview' : 'Public deployment'}</span></li>)}</ul></details>}
    <SessionPicker open={!!draft} worktreeId={draft?.worktreeId ?? null} implementationContext={draft?.implementationContext} onClose={onDraftClose} onSubmit={create} onCreated={onDraftClose} submitLabel="Create project run">
      <p className="pick-note">Save the reviewed inputs and model route. Coding and deployment start only through their separate controls.</p><label className="pick-field">Intended outcome<select value={outcome} onChange={event => setOutcome(event.target.value as typeof outcome)}><option value="preview">Workspace preview</option><option value="publish">Public deployment</option></select></label>
    </SessionPicker>
    {dialog && run && <WorkflowDialog title={actionLabel[dialog]} busy={!!busy} onClose={() => setDialog(null)} description={dialog === 'cancel' ? 'Cancel future dispatch for this run. This does not stop an existing coding session, check or application; use its separate stop control.' : dialog === 'accept' ? 'Review the selected behavior yourself. Acceptance records that review only after the server revalidates matching technical evidence.' : 'Choose an existing result. The server validates its project, current reviews, requirements and exact source before joining it. This does not execute commands or deploy anything.'}>
      {error && <p role="alert" className="journey-error">{error}</p>}
      {evidenceNotice && <p role="status" className="journey-muted">{evidenceNotice}</p>}
      {dialog === 'accept' ? <label className="setup-check"><input type="checkbox" checked={accepted} onChange={event => setAccepted(event.target.checked)} />I reviewed the selected {run.input.outcome === 'preview' ? 'workspace preview' : 'public deployment'} and accept this run’s outcome.</label> : dialog === 'join-check' || dialog === 'join-release' ? <label className="luci-field">{dialog === 'join-check' ? 'Check result' : 'Release result'}<select value={choice} onChange={event => setChoice(event.target.value)}><option value="">Choose recorded evidence</option>{(dialog === 'join-check' ? checks : releases).map(item => <option key={'target' in item ? releaseKey(item) : item.id} value={'target' in item ? releaseKey(item) : item.id}>{item.id} · {item.status} · {'target' in item ? `${item.target} · ` : ''}{item.commit ? item.commit.slice(0, 12) : 'No commit'}</option>)}</select>{!(dialog === 'join-check' ? checks : releases).length && <small>No results recorded yet. Run checks or prepare a release through its existing controls first.</small>}</label> : null}
      <div className="workflow-actions"><Button variant="secondary" disabled={!!busy} onClick={() => setDialog(null)}>Keep reviewing</Button><Button disabled={!!busy || (dialog === 'accept' ? !accepted || !run.observation?.valid : dialog !== 'cancel' && !choice)} onClick={() => void operate(actionLabel[dialog], confirm)}>{busy || (dialog === 'accept' ? 'Accept reviewed outcome' : dialog === 'cancel' ? 'Cancel future dispatch' : 'Join selected evidence')}</Button></div>
    </WorkflowDialog>}
  </section>;
}
