import { useEffect, useRef, useState } from 'react';
import { api } from '../../api';
import { errorText, productApi, type CheckRun, type Release, type ImplementationPlan } from '../../api/product';
import { platformApi } from '../../api/platform';
import { notifyProjectContextChanged } from '../../api/projectContextEvents';
import { projectRunsApi, runHasTaskReview, runSessionRequest, type CreateProjectRun, type ProjectRun, type RunOperation } from '../../api/projectRuns';
import type { CreateSessionRequest } from '../../api/client';
import type { ImplementationContextRequest } from '../../store/types';
import { useProjectAccess } from '../../hooks/useProjectAccess';
import { useProjectDraftScope } from '../../hooks/useProjectDraft';
import { useCockpit } from '../../store/store';
import { useAuth } from '../auth/AuthProvider';
import { useSetup } from '../setup/SetupProvider';
import { Button } from '../fluid/components/button';
import { WorkflowDialog } from '../common/WorkflowDialog';
import { SessionPicker } from '../topbar/SessionPicker';
import { ProjectSwarm } from './ProjectSwarm';
import { ORCHESTRATOR_OPEN, type OrchestratorOpenDetail } from '../orchestrator/OrchestratorPanel';
import './project-runs.css';

export interface ProjectRunDraft { implementationContext: ImplementationContextRequest; worktreeId: string }
type OpenCodingSessionTarget = { scope: string; projectId: string; sessionId: string; origin: Element | null };
type Dialog = 'join-check' | 'join-release' | 'accept' | 'cancel';
type ReleaseChoice = Pick<Release, 'id' | 'projectId' | 'commit' | 'status' | 'target'>;
const releaseKey = (release: ReleaseChoice) => JSON.stringify([release.target, release.id]);
const labels: Record<string, string> = {
  ready: 'Ready to code', coding: 'Coding', 'awaiting-checks': 'Awaiting verification',
  'awaiting-release': 'Awaiting release evidence', blocked: 'Needs attention', cancelled: 'Cancelled', complete: 'Accepted outcome',
};
const contextNeedsReview = (run?: ProjectRun) => !!run?.contextFreshness && run.contextFreshness.status !== 'current';
const codingBlocked = (run?: ProjectRun) => contextNeedsReview(run) || !!run?.codingBlocker;
const recordedCodingActivity = (run: ProjectRun) => run.operations.some(item => item.status === 'running') ? 'Coding recorded as running'
  : run.operations.some(item => ['dispatching', 'unknown'].includes(item.status)) ? 'Coding outcome unconfirmed'
    : run.operations.some(item => item.status === 'reserved') ? 'Coding reserved, not started' : '';
const runLabel = (run: ProjectRun) => {
  // Acceptance/cancellation are historical outcomes; current review does not
  // undo them or imply that an allocated coding child has stopped.
  if (run.state === 'complete' || run.state === 'cancelled') return labels[run.state] || run.state;
  if (!codingBlocked(run)) return runHasTaskReview(run) ? 'Awaiting task review' : labels[run.state] || run.state;
  const activity = recordedCodingActivity(run);
  if (activity) return activity;
  if (run.codingBlocker) return run.codingBlocker.kind === 'question'
    ? (run.state === 'ready' ? 'Decision needed before coding' : 'Decision needed before continuing') : 'Project decisions unavailable';
  return run.contextFreshness?.status === 'unavailable' ? 'Context unavailable' : 'Review current context';
};
const when = (value: string) => new Date(value).toLocaleString();
const scope = (values: unknown[]) => JSON.stringify(values);


type PlanComparisonRead = { status: 'reading' } | { status: 'error'; message: string } | { status: 'ready'; plan: ImplementationPlan };

function CurrentPlanComparison({ projectId, run }: { projectId: string; run: ProjectRun }) {
  const [read, setRead] = useState<PlanComparisonRead>({ status: 'reading' });
  const alive = useRef(true), generation = useRef(0);
  async function load() {
    const mine = ++generation.current;
    setRead({ status: 'reading' });
    try {
      const plan = await productApi.plan(projectId);
      if (!alive.current || mine !== generation.current) return;
      if (plan.projectId !== projectId || !Number.isSafeInteger(plan.revision) || plan.revision < 0 || !Array.isArray(plan.tasks) || plan.tasks.length > 200 ||
          plan.tasks.some(task => !task || typeof task.id !== 'string' || !task.id || typeof task.title !== 'string' || !task.title.trim() || task.title.length > 2000) || new Set(plan.tasks.map(task => task.id)).size !== plan.tasks.length) {
        throw new Error('The saved plan could not be verified for this project. Refresh the comparison.');
      }
      setRead({ status: 'ready', plan });
    } catch (cause) {
      if (alive.current && mine === generation.current) setRead({ status: 'error', message: errorText(cause) });
    }
  }
  useEffect(() => {
    alive.current = true; void load();
    return () => { alive.current = false; generation.current++; };
  }, [projectId, run.id, run.revision]);
  return <section aria-label="Selected task title comparison">
    <p>This build’s captured inputs and recorded checks are retained.</p>
    <p>This compares titles for the {run.input.implementation.taskIds.length} {run.input.implementation.taskIds.length === 1 ? 'task' : 'tasks'} selected for this build. It does not compare task status, unselected tasks, other context, or unsaved edits.</p>
    {read.status === 'reading' ? <p role="status">Reading current saved plan…</p>
      : read.status === 'error' ? <p role="alert">Current saved plan could not be compared. {read.message}</p>
        : <>
          <p>This build: plan {run.input.implementation.planRevision}. Current saved plan: {read.plan.revision}.</p>
          <ul>{run.input.implementation.taskIds.map((id, index) => {
            const captured = run.input.implementation.taskTitles[index];
            const current = read.plan.tasks.find(task => task.id === id);
            return <li key={id}>
              <p><strong>Captured task:</strong> {captured}</p>
              <p><strong>Current saved task:</strong> {current?.title ?? 'Removed from the current saved plan.'}</p>
              <p className="journey-muted">{!current ? 'Removed from saved plan.' : current.title === captured ? 'Title unchanged.' : 'Title changed.'}</p>
            </li>;
          })}</ul>
          <p className="journey-muted">These saved values are read when you open or refresh this comparison. Matching titles do not mean the full reviewed context still matches.</p>
        </>}
    <Button variant="secondary" disabled={read.status === 'reading'} onClick={() => void load()}>Refresh comparison</Button>
  </section>;
}

function PlanComparison({ projectId, run }: { projectId: string; run: ProjectRun }) {
  const [open, setOpen] = useState(false);
  // Closing, changing the selected run or losing read authority unmounts the
  // reader. Its completed values and pending replies never cross that boundary.
  return <details onToggle={event => setOpen(event.currentTarget.open)}><summary>Compare with current plan</summary>
    {open && <CurrentPlanComparison projectId={projectId} run={run} />}
  </details>;
}

export function ProjectRuns(props: { projectId: string; draft?: ProjectRunDraft; onDraftClose(): void; onReviewPlan(): void; onOpenCodingSession?(target: OpenCodingSessionTarget): void }) {
  const auth = useAuth(), setup = useSetup();
  const navigationScope = useProjectDraftScope(props.projectId);
  const identity = scope([productApi.baseUrl, auth?.profile.user.id, setup?.workspace, setup?.status?.access?.user, props.projectId, navigationScope]);
  const prepared = useRef({ draft: props.draft, identity });
  if (prepared.current.draft !== props.draft) prepared.current = { draft: props.draft, identity };
  // Discard transient requests, errors, selection and confirmations on account/workspace changes.
  return <RunWorkspace key={identity} {...props} navigationScope={navigationScope} draft={prepared.current.identity === identity ? props.draft : undefined} />;
}

function RunWorkspace({ projectId, draft, onDraftClose, onReviewPlan, onOpenCodingSession, navigationScope }: {
  projectId: string; draft?: ProjectRunDraft; onDraftClose(): void; onReviewPlan(): void; onOpenCodingSession?(target: OpenCodingSessionTarget): void; navigationScope: string | null;
}) {
  const access = useProjectAccess(projectId);
  const { canRead, canEdit, canExecute } = access;
  const mayDispatch = useRef(canExecute); mayDispatch.current = canExecute;
  const mayRead = useRef(canRead); mayRead.current = canRead;
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
  const run = selected ? runs.find(item => item.id === selected) : runs[0];
  const inspection = useCockpit(state => state.projectRunInspection);
  const region = useRef<HTMLElement>(null);
  const inspecting = useRef<typeof inspection>(null);
  const setLens = useCockpit(state => state.setLens);
  const allowed = (action: string) => canEdit && (!(run?.swarm && ['start-coding', 'continue-coding'].includes(action))) && (!['start-coding', 'continue-coding'].includes(action) || canExecute) && (!['start-coding', 'continue-coding', 'join-check', 'join-release', 'accept'].includes(action) || !codingBlocked(run)) && !!run?.allowedActions?.includes(action) && !(dispatchUncertain && (action === 'start-coding' || action === 'continue-coding'));
  const remember = (value: ProjectRun) => {
    if (!alive.current) return;
    if (value.projectId !== projectId || value.input.implementation.projectId !== projectId) throw new Error('The run belongs to another project. Refresh this project.');
    setRuns(current => [value, ...current.filter(item => item.id !== value.id)].sort((a, b) => b.createdAt.localeCompare(a.createdAt)));
    setSelected(value.id);
  };
  async function load() {
    if (!canRead) return;
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
  }, [projectId, canRead || runs.length > 0]);
  useEffect(() => { if (draft) { setOutcome('preview'); creation.current = null; } }, [draft]);
  useEffect(() => {
    if (!inspection || loading || busy || !canRead || inspecting.current === inspection) return;
    if (!navigationScope || inspection.scope !== navigationScope || inspection.projectId !== projectId) {
      useCockpit.setState(state => state.projectRunInspection === inspection ? { projectRunInspection: null } : {});
      return;
    }
    inspecting.current = inspection;
    setSelected(inspection.runId); setDialog(null); setNotice('');
    const finish = () => {
      if (!alive.current || useCockpit.getState().projectRunInspection !== inspection) return;
      useCockpit.setState({ projectRunInspection: null });
      requestAnimationFrame(() => {
        if (!alive.current) return;
        region.current?.scrollIntoView?.({ block: 'nearest' }); region.current?.focus();
      });
    };
    if (inspection.refresh) {
      const mine = ++request.current;
      setLoading(true);
      void projectRunsApi.get(projectId, inspection.runId).then(value => {
        if (!alive.current || mine !== request.current || useCockpit.getState().projectRunInspection !== inspection) return;
        if (value.id !== inspection.runId) throw new Error('The requested run is unavailable in this project.');
        remember(value); setError('');
      }).catch(cause => {
        if (alive.current && mine === request.current) setError(errorText(cause));
      }).finally(() => {
        if (alive.current && mine === request.current) { setLoading(false); finish(); }
      });
    } else {
      setError(runs.some(item => item.id === inspection.runId) ? '' : 'The requested run is unavailable in this project. Refresh history or choose another recorded run.');
      finish();
    }
  }, [inspection, loading, busy, canRead, navigationScope, projectId, runs]);

  async function operate(label: string, action: () => Promise<void>, changed = true) {
    if (working.current) return;
    working.current = true; request.current++; setLoading(false); setBusy(label); setError(''); setNotice('');
    try { await action(); if (changed && alive.current) notifyProjectContextChanged(navigationScope, projectId); }
    catch (cause) {
      if (alive.current) setError(`${errorText(cause)} Refresh history, then use Check run status before retrying an uncertain operation.`);
    } finally { working.current = false; if (alive.current) setBusy(''); }
  }
  async function create(selection: CreateSessionRequest) {
    if (!canEdit) return { ok: false as const, message: 'Editing access is required to prepare a project run.' };
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
      request.current++; setLoading(false); remember(value); notifyProjectContextChanged(navigationScope, projectId);
      setNotice(value.codingBlocker ? 'Run saved. Review the project decisions before starting more coding.' : contextNeedsReview(value) ? 'Run saved. Review its current context before starting more coding.' : 'Reviewed run saved. Start coding when ready; no session or deployment has started.');
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
    if (saved.codingBlocker) {
      setNotice('The coding reservation is retained. Review the project decisions before starting more coding.');
      return;
    }
    if (contextNeedsReview(saved)) {
      setNotice('The coding reservation is retained. Review the current plan before preparing more coding work.');
      return;
    }
    if (!mayDispatch.current) throw new Error('Execution access changed. The reserved operation remains available to an authorized project member.');
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
    if (reserved.length !== 1) throw new Error('The reserved operation is ambiguous. Use Check run status before continuing.');
    await dispatch(run, reserved[0]);
  }
  async function openSession(operation?: RunOperation) {
    if (!canRead) return;
    const child = operation?.childId || run?.observation?.childId || run?.operations[run.operations.length - 1]?.childId;
    if (!child || !run) return;
    const before = useCockpit.getState();
    if (before.pendingNavigation) return;
    const origin = document.activeElement;
    const session = await api.getSession(child);
    if (!alive.current || !mayRead.current) return;
    const currentNavigation = useCockpit.getState();
    if (currentNavigation.selection !== before.selection || currentNavigation.lens !== before.lens || currentNavigation.pendingNavigation) return;
    const bound = run.operations.find(item => item.id === session.projectRunOperationId && item.childId === session.id);
    if (session.id !== child || session.worktreeId !== (bound?.worktreeId || run.worktreeId) || session.projectId !== projectId || session.projectRunId !== run.id || !bound || operation && bound.id !== operation.id) throw new Error('This coding session does not belong to the selected run.');
    const state = useCockpit.getState();
    state.upsertSession(session); state.selectBranch(projectId, session.branchId);
    const current = useCockpit.getState();
    if (!current.pendingNavigation && current.lens === 'develop' && current.selection.projectId === projectId && current.selection.branchId === session.branchId) {
      current.selectWorktree(session.worktreeId); current.focusSession(session.id);
      const focused = useCockpit.getState();
      if (navigationScope && !focused.pendingNavigation && focused.lens === 'develop' && focused.selection.projectId === projectId && focused.selection.branchId === session.branchId && focused.selection.worktreeId === session.worktreeId && focused.selection.sessionId === session.id) {
        onOpenCodingSession?.({ scope: navigationScope, projectId, sessionId: session.id, origin });
      }
    }
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
  const activeCoding = !!run?.operations.some(item => ['running', 'dispatching', 'unknown'].includes(item.status));
  const reviewInputsFirst = !!run && !activeCoding && !['complete', 'cancelled'].includes(run.state);
  const primary = activeCoding && codingBlocked(run) && allowed('reconcile') ? 'reconcile'
    : reviewInputsFirst && run?.codingBlocker ? 'review-decisions'
    : reviewInputsFirst && contextNeedsReview(run) ? 'review-plan'
    : !run?.acceptance && allowed('accept') && run?.observation?.valid ? 'accept'
    : allowed('continue-coding') ? 'continue-coding'
      : run?.state === 'ready' && allowed('start-coding') ? 'start-coding'
      : run?.state === 'awaiting-checks' && allowed('join-check') ? 'verify'
        : run?.state === 'awaiting-release' && allowed('join-release') ? (run.checks.length && !run.releases.length ? 'deploy' : 'join-release')
          : allowed('reconcile') ? 'reconcile' : '';
  function action(kind: string) {
    if (!run) return;
    if (kind === 'review-plan') { onReviewPlan(); return; }
    if (kind === 'review-decisions') {
      if (navigationScope && run.codingBlocker) window.dispatchEvent(new CustomEvent<OrchestratorOpenDetail>(ORCHESTRATOR_OPEN, { detail: { projectId, scope: navigationScope, destination: 'decisions', turnId: run.codingBlocker.turnId } }));
      return;
    }
    if (kind === 'verify') {
      if (navigationScope && allowed('join-check')) useCockpit.setState({ projectRunVerification: { scope: navigationScope, projectId, runId: run.id } });
      return;
    }
    if (kind === 'deploy') {
      const check = run.checks[run.checks.length - 1];
      if (!navigationScope || !check || !allowed('join-release')) return;
      useCockpit.setState({ projectRunDeployment: { scope: navigationScope, projectId, runId: run.id, checkId: check.checkId, commit: check.commit, sourceDigest: check.sourceDigest, outcome: run.input.outcome } });
      setLens('deploy');
      return;
    }
    if (!allowed(kind)) return;
    if (kind === 'start-coding') void operate('Starting coding', startCoding);
    else if (kind === 'continue-coding') void operate('Continuing reserved coding', continueCoding);
    else if (kind === 'reconcile') void operate('Checking run status', async () => { remember(await projectRunsApi.reconcile(projectId, run.id, run.revision)); if (alive.current) setDispatchUncertain(false); });
    else if (kind === 'join-check' || kind === 'join-release') void operate('Reading available evidence', () => chooseEvidence(kind), false);
    else if (kind === 'accept' || kind === 'cancel') { setAccepted(false); setDialog(kind); }
  }
  const actionLabel: Record<string, string> = { 'review-decisions': run?.codingBlocker?.kind === 'question' ? 'Review blocking question' : 'Check project decisions', 'review-plan': 'Review current plan', verify: 'Verify this build', deploy: run?.input.outcome === 'publish' ? 'Prepare deployment' : 'Preview this build', 'start-coding': 'Start coding', 'continue-coding': 'Continue reserved coding', 'join-check': 'Select check evidence', 'join-release': 'Select release evidence', reconcile: 'Check run status', accept: 'Review and accept outcome', cancel: 'Cancel run' };
  if (!canRead) return <section className="project-runs" aria-label="Project runs"><p role="status">{access.message}</p></section>;
  return <section ref={region} tabIndex={-1} className="project-runs" aria-label="Project runs" aria-busy={!!busy || loading}>
    <div className="project-runs-heading"><h2>Project run</h2><Button variant="tertiary" disabled={!!busy || loading} onClick={() => void load()}>{loading ? 'Reading history…' : 'Refresh history'}</Button></div>
    {access.message && <p role="status" className="journey-muted">{access.message}</p>}
    {error && <p role="alert" className="journey-error">{error}</p>}
    {notice && <p role="status" className="journey-muted">{notice}</p>}
    {!run ? <><p className="journey-muted">Keep reviewed tasks, coding, checks and a verified release together. Creating a run saves its inputs; each next operation remains your choice.</p><Button variant="secondary" onClick={onReviewPlan}>Review implementation plan</Button></> : <>
      <div className="project-run-next"><strong>{runLabel(run)}</strong><span>{run.input.outcome === 'preview' ? 'Workspace preview' : 'Public deployment'}</span>{!codingBlocked(run) && <p>{run.reason}</p>}
        {!!recordedCodingActivity(run) && <p className="journey-muted">{recordedCodingActivity(run)}. Open the coding session or use Check run status for a fresh status update.</p>}
        {run.codingBlocker && <p className="journey-warning">{run.codingBlocker.reason}</p>}
        {run.contextFreshness && <p className={contextNeedsReview(run) ? 'journey-warning' : 'journey-muted'}>{contextNeedsReview(run) && <strong>{run.contextFreshness.status === 'unavailable' ? 'Reviewed context unavailable. ' : 'Reviewed context changed. '}</strong>}{run.contextFreshness.reason}</p>}
        {run.acceptance && <p>Accepted {when(run.acceptance.acceptedAt)}. This historical acceptance is retained separately from current validity.</p>}
        {run.observation ? <p className={run.observation.valid ? 'journey-muted' : 'journey-warning'}>{run.observation.valid ? 'Last run status check recorded' : 'Last run status check needs attention'} · {when(run.observation.observedAt)}.{codingBlocked(run) && <> This earlier status check does not clear the current review.</>}{!codingBlocked(run) && (!run.acceptance || !run.observation.valid) && run.observation.reason !== run.reason && <> {run.observation.reason}</>}</p> : <p className="journey-muted">No run status check recorded yet. Saved test and release evidence is shown separately below.</p>}
        <div className="journey-actions">
          {primary && <Button disabled={!!busy || loading || (['verify', 'deploy', 'review-decisions'].includes(primary) && !navigationScope)} onClick={() => action(primary)}>{busy || actionLabel[primary]}</Button>}
          {run.codingBlocker && primary !== 'review-decisions' && <Button variant="secondary" disabled={!!busy || loading || !navigationScope} onClick={() => action('review-decisions')}>{actionLabel['review-decisions']}</Button>}
          {contextNeedsReview(run) && primary !== 'review-plan' && <Button variant="secondary" disabled={!!busy || loading} onClick={onReviewPlan}>Review current plan</Button>}
          {!!run.operations.length && !run.swarm && <Button variant="secondary" disabled={!canRead || !!busy} onClick={() => void operate('Opening coding session', () => openSession(), false)}>Open coding session</Button>}
          {allowed('reconcile') && primary !== 'reconcile' && <Button variant="secondary" disabled={!!busy || loading} onClick={() => action('reconcile')}>Check run status</Button>}
          {run.state === 'awaiting-checks' && allowed('start-coding') && <Button variant="secondary" disabled={!!busy || loading} onClick={() => action('start-coding')}>Start another coding attempt</Button>}
          {allowed('join-release') && primary !== 'join-release' && <Button variant="tertiary" disabled={!!busy || loading} onClick={() => action('join-release')}>Select release evidence</Button>}
          {allowed('join-check') && <Button variant="tertiary" disabled={!!busy || loading} onClick={() => action('join-check')}>Select check evidence</Button>}
        </div>
        <p className="journey-muted">Refresh history reloads saved records. Check run status inspects existing work and records an updated status. It starts no coding session, project test run or deployment.</p>
        {run.state === 'awaiting-checks' && <p className="journey-muted">{codingBlocked(run) ? `This run retains its previous coding and evidence. Review ${run.codingBlocker ? 'the project decisions' : 'the current plan'} before continuing this run.` : runHasTaskReview(run) ? 'Review the exited task’s output and integrate accepted changes before verifying the combined build. A normal process exit does not establish correctness.' : 'Verify this build to review its saved criteria, run checks, and use matching results. If a check fails, inspect its output and start another coding attempt to repair the implementation.'} A CLI exit or task marked Done does not complete this run.</p>}
        {allowed('continue-coding') && <p className="journey-muted">A coding operation was reserved before dispatch. Continue uses that same child identity; it does not reserve another attempt. Use Check run status first if its result is uncertain.</p>}
        {run.state === 'cancelled' && <p className="journey-muted">Future dispatch is cancelled. Running children retain their separate stop controls.</p>}
      </div>
      <ProjectSwarm key={run.id} run={run} canEdit={canEdit} canExecute={canExecute} busy={!!busy || loading} onReviewPlan={onReviewPlan}
        onBusy={label => { working.current = !!label; setBusy(label); }}
        onUpdate={value => { request.current++; setLoading(false); remember(value); notifyProjectContextChanged(navigationScope, projectId); }}
        onOpenSession={openSession} />
      <PlanComparison key={`${run.id}:${run.revision}`} projectId={projectId} run={run} />
      <details className="project-run-receipts"><summary>Reviewed inputs and evidence</summary>
        {codingBlocked(run) && <>
          <p>Last recorded stage: {labels[run.state] || run.state}</p>
          <p>{run.state === 'ready' ? 'Recorded when prepared: ' : 'Last recorded guidance: '}{run.reason}</p>
          {run.observation && (!run.acceptance || !run.observation.valid) && run.observation.reason !== run.reason && <p>Last recorded status check: {run.observation.reason}</p>}
        </>}
        <p>{run.input.implementation.taskTitles.join('; ')}</p><p>Brief {run.input.implementation.briefRevision} · Plan {run.input.implementation.planRevision} · Definition {run.input.implementation.specificationRevision} · Design {run.input.implementation.designPlanRevision}</p>
        <p>{run.input.route.harness} · {run.input.route.model} · {run.input.route.connection.id || 'Direct agent settings'} · connection revision {run.input.route.connection.revision}</p>
        <p>Starting commit {run.input.implementation.sourceCommit || 'not committed'}{run.input.implementation.sourceDirty ? ' · included uncommitted changes' : ''}</p>
        {run.input.requirements.map(item => <p key={item.id}><strong>{item.id} · {item.title}</strong><br />{item.acceptance}</p>)}
        <h3>Coding attempts</h3>{!run.operations.length && <p>No coding operation reserved.</p>}
        <ul>{run.operations.map(item => <li key={item.id}>Attempt {item.attempt} · {item.status} · {item.childId}<p>{item.message}</p></li>)}</ul>
        {allowed('start-coding') && primary !== 'start-coding' && run.state !== 'awaiting-checks' && <Button variant="secondary" disabled={!!busy || loading} onClick={() => action('start-coding')}>Start another coding attempt</Button>}
        <h3>Check evidence</h3>{!run.checks.length && <p>No accepted check evidence.</p>}
        {run.checks.map(item => <div key={item.checkId}><p>{item.checkId} · {item.commit.slice(0, 12)} · {item.execution}</p>{!item.originalAvailable && <p className="journey-muted">Original check history is unavailable; the saved evidence snapshot is retained.</p>}<ul>{item.commands.map((command, index) => <li key={index}>{command.label} · {command.status} · exit {command.exitCode}</li>)}</ul></div>)}
        <h3>Release evidence</h3>{!run.releases.length && <p>No accepted release evidence.</p>}
        {run.releases.map(item => <p key={item.releaseId}>{item.releaseId} · {item.target} · {item.commit.slice(0, 12)}<br />Observed {when(item.observedAt)}. {item.limitations}</p>)}
        {allowed('cancel') && <Button variant="tertiary" disabled={!!busy || loading} onClick={() => action('cancel')}>Cancel run</Button>}
      </details>
    </>}
    {runs.length > 0 && <details className="project-run-history"><summary>Run history · {runs.length}</summary><ul>{runs.map(item => <li key={item.id}><Button variant={item.id === run?.id ? 'secondary' : 'tertiary'} disabled={!!busy} aria-pressed={item.id === run?.id} onClick={() => { setSelected(item.id); setDialog(null); setError(''); setNotice(''); }}>{runLabel(item)} · {when(item.createdAt)}</Button><span>{item.input.outcome === 'preview' ? 'Workspace preview' : 'Public deployment'}</span></li>)}</ul></details>}
    <SessionPicker open={!!draft && canEdit} worktreeId={draft?.worktreeId ?? null} implementationContext={draft?.implementationContext} onClose={onDraftClose} onSubmit={create} onCreated={onDraftClose} submitLabel="Create project run">
      <p className="pick-note">Save the reviewed inputs and model route. Coding and deployment start only through their separate controls.</p><label className="pick-field">Intended outcome<select value={outcome} onChange={event => setOutcome(event.target.value as typeof outcome)}><option value="preview">Workspace preview</option><option value="publish">Public deployment</option></select></label>
    </SessionPicker>
    {dialog && run && canEdit && <WorkflowDialog title={actionLabel[dialog]} busy={!!busy} onClose={() => setDialog(null)} description={dialog === 'cancel' ? 'Cancel future dispatch for this run. This does not stop an existing coding session, check or application; use its separate stop control.' : dialog === 'accept' ? 'Review the selected behavior yourself. Acceptance records that review only after the server revalidates matching technical evidence.' : 'Choose an existing result. The server validates its project, current reviews, requirements and exact source before joining it. This does not execute commands or deploy anything.'}>
      {error && <p role="alert" className="journey-error">{error}</p>}
      {evidenceNotice && <p role="status" className="journey-muted">{evidenceNotice}</p>}
      {dialog === 'accept' ? <label className="setup-check"><input type="checkbox" checked={accepted} onChange={event => setAccepted(event.target.checked)} />I reviewed the selected {run.input.outcome === 'preview' ? 'workspace preview' : 'public deployment'} and accept this run’s outcome.</label> : dialog === 'join-check' || dialog === 'join-release' ? <label className="luci-field">{dialog === 'join-check' ? 'Check result' : 'Release result'}<select value={choice} onChange={event => setChoice(event.target.value)}><option value="">Choose recorded evidence</option>{(dialog === 'join-check' ? checks : releases).map(item => <option key={'target' in item ? releaseKey(item) : item.id} value={'target' in item ? releaseKey(item) : item.id}>{item.id} · {item.status} · {'target' in item ? `${item.target} · ` : ''}{item.commit ? item.commit.slice(0, 12) : 'No commit'}</option>)}</select>{!(dialog === 'join-check' ? checks : releases).length && <small>No results recorded yet. Run checks or prepare a release through its existing controls first.</small>}</label> : null}
      <div className="workflow-actions"><Button variant="secondary" disabled={!!busy} onClick={() => setDialog(null)}>Keep reviewing</Button><Button disabled={!allowed(dialog) || !!busy || (dialog === 'accept' ? !accepted || !run.observation?.valid : dialog !== 'cancel' && !choice)} onClick={() => void operate(actionLabel[dialog], confirm)}>{busy || (dialog === 'accept' ? 'Accept reviewed outcome' : dialog === 'cancel' ? 'Cancel future dispatch' : 'Join selected evidence')}</Button></div>
    </WorkflowDialog>}
  </section>;
}
