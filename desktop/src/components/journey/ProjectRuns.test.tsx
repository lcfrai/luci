import { act, render, screen, waitFor, within } from '@testing-library/react';
import userEvent from '@testing-library/user-event';
import { beforeEach, expect, it, vi } from 'vitest';
import { api } from '../../api';
import { projectRunsApi, type ProjectRun, type RunOperation } from '../../api/projectRuns';
import { productApi } from '../../api/product';
import { platformApi } from '../../api/platform';
import { resetCockpit, useCockpit } from '../../store/store';
import { aBranch, aHarness, aProject, aSession, aWorktree } from '../../test/fixtures';
import { ProjectRuns } from './ProjectRuns';
import { OrchestratorPanel } from '../orchestrator/OrchestratorPanel';
import { orchestratorApi } from '../../api/orchestrator';
import { context as orchestratorContext, view as orchestratorView } from '../orchestrator/fixtures';

const scope = vi.hoisted(() => ({ actor: 'owner', connection: '' }));
vi.mock('../auth/AuthProvider', () => ({ useAuth: () => ({ profile: { user: { id: scope.actor } } }) }));
vi.mock('../setup/SetupProvider', () => ({ useSetup: () => ({ workspace: { mode: scope.connection ? 'cloud' : 'local', connectionId: scope.connection, host: scope.connection }, status: { access: { user: { username: scope.actor } }, config: { projectsDir: '/projects' } } }) }));
const approvals = { specificationApprovalId: 'approval-spec', specificationRevision: 3, specificationDigest: 'spec', designApprovalId: 'approval-design', designRevision: 4, designDigest: 'design', briefDigest: 'brief', sketchDigest: 'sketch' };
const operation: RunOperation = { id: 'op-1', kind: 'session', childId: 's-bound', attempt: 1, status: 'reserved', requestDigest: 'request', createdAt: '2026-10-06T00:00:00Z', updatedAt: '2026-10-06T00:00:00Z' };
const makeRun = (patch: Partial<ProjectRun> = {}): ProjectRun => ({
  id: 'run-1', projectId: 'p-1', worktreeId: 'w-1', revision: 1, createdAt: '2026-10-06T00:00:00Z', updatedAt: '2026-10-06T00:00:00Z', state: 'ready', reason: 'Reviewed inputs saved.',
  input: { implementation: { projectId: 'p-1', worktreeId: 'w-1', briefRevision: 2, planRevision: 5, specificationRevision: 3, designPlanRevision: 4, taskIds: ['task-1'], taskTitles: ['Build journal'], requirementIds: ['req-1'], briefDigest: 'brief', planDigest: 'plan', specificationDigest: 'spec', designPlanDigest: 'design', sourceCommit: 'a'.repeat(40), sourceDigest: 'source', sourceDirty: false }, approvals, outcome: 'preview', requirements: [{ id: 'req-1', title: 'Keep notes', acceptance: 'Survive reload' }], route: { harness: 'claude-code', model: 'saved-model', permissionMode: 'ask', connection: { id: 'saved-connection', revision: 7, kind: 'compatible-api' } } },
  operations: [], checks: [], releases: [], allowedActions: ['start-coding', 'reconcile', 'cancel'], ...patch,
});
const props = () => ({ projectId: 'p-1', onDraftClose: vi.fn(), onReviewPlan: vi.fn() });
beforeEach(() => {
  vi.restoreAllMocks(); resetCockpit(); scope.actor = 'owner'; scope.connection = '';
  useCockpit.setState({ projects: [aProject({ id: 'p-1' })], branchesByProject: { 'p-1': [aBranch({ id: 'b-1', projectId: 'p-1' })] }, worktreesByBranch: { 'b-1': [aWorktree({ id: 'w-1', projectId: 'p-1', branchId: 'b-1', isPrimary: true })] }, selection: { projectId: 'p-1', branchId: 'b-1', worktreeId: 'w-1', sessionId: null }, harnesses: [aHarness({ id: 'claude-code', available: true, supportsOpeningPrompt: true })] });
  vi.spyOn(projectRunsApi, 'list').mockResolvedValue([makeRun()]);
  vi.spyOn(productApi, 'agents').mockResolvedValue({ providers: [] });
  vi.spyOn(productApi, 'agentConnections').mockResolvedValue({ revision: 0, activeConnectionId: '', connections: [] });
  vi.spyOn(platformApi, 'project').mockResolvedValue({ busy: false, project: { projectId: 'p-1', deployments: [], database: { engine: 'postgres', status: 'missing', message: '' }, backups: [] } });
});

it('only reads on mount and refresh, and retains ended or unknown coding as unverified', async () => {
  vi.mocked(projectRunsApi.list).mockResolvedValue([makeRun({ state: 'awaiting-checks', reason: 'CLI exited zero; application outcome unverified.', operations: [{ ...operation, status: 'succeeded' }], allowedActions: ['join-check', 'reconcile', 'cancel'] })]);
  const reconcile = vi.spyOn(projectRunsApi, 'reconcile');
  const create = vi.spyOn(api, 'createSession');
  render(<ProjectRuns {...props()} />);
  await screen.findByText('CLI exited zero; application outcome unverified.');
  await userEvent.click(screen.getByRole('button', { name: 'Refresh history' }));
  await waitFor(() => expect(projectRunsApi.list).toHaveBeenCalledTimes(2));
  expect(reconcile).not.toHaveBeenCalled(); expect(create).not.toHaveBeenCalled();
  expect(screen.queryByRole('button', { name: 'Review and accept outcome' })).toBeNull();
  expect(screen.getByText(/A CLI exit or task marked Done does not complete/)).not.toBeNull();
});

it('refreshes a prepared run to changed context and opens its current plan without dispatch or rewriting history', async () => {
  const reason = 'The saved brief or implementation plan changed. Reload the plan before starting its task.';
  const original = makeRun({ contextFreshness: { status: 'current', reason: 'The saved reviewed context still matches. Source and model availability are checked separately.' } });
  // Keep an obsolete action in this response to exercise the UI's additional
  // context fence as well as the server's authoritative action projection.
  vi.mocked(projectRunsApi.list).mockResolvedValueOnce([original]).mockResolvedValue([{ ...original, contextFreshness: { status: 'changed', reason } }]);
  const initial = props(), reserve = vi.spyOn(projectRunsApi, 'reserve'), reconcile = vi.spyOn(projectRunsApi, 'reconcile'), create = vi.spyOn(api, 'createSession');
  render(<ProjectRuns {...initial} />);
  await screen.findByRole('button', { name: 'Start coding' });
  await userEvent.click(screen.getByRole('button', { name: 'Refresh history' }));
  const review = await screen.findByRole('button', { name: 'Review current plan' });
  expect(screen.getByText('Review current context')).not.toBeNull();
  expect(screen.getByText(reason, { exact: false })).not.toBeNull();
  expect(screen.queryByText('Ready to code')).toBeNull();
  expect(screen.queryByRole('button', { name: 'Start coding' })).toBeNull();
  await userEvent.click(review);
  expect(initial.onReviewPlan).toHaveBeenCalledTimes(1);
  expect(reserve).not.toHaveBeenCalled(); expect(reconcile).not.toHaveBeenCalled(); expect(create).not.toHaveBeenCalled();
  await userEvent.click(screen.getByText('Reviewed inputs and evidence'));
  expect(screen.getByText('Recorded when prepared: Reviewed inputs saved.')).not.toBeNull();
  expect(screen.getByText('Brief 2 · Plan 5 · Definition 3 · Design 4')).not.toBeNull();
  await userEvent.click(screen.getByText('Run history · 1'));
  expect(screen.getByRole('button', { name: /^Review current context ·/ })).not.toBeNull();
});

it.each(['changed', 'unavailable'] as const)('keeps active session access and reconciliation when reviewed context is %s', async status => {
  vi.mocked(projectRunsApi.list).mockResolvedValue([makeRun({ state: 'coding', operations: [operation], contextFreshness: { status, reason: 'Review the current saved context before more coding.' }, allowedActions: ['continue-coding', 'start-coding', 'reconcile', 'cancel'] })]);
  const create = vi.spyOn(api, 'createSession'), reserve = vi.spyOn(projectRunsApi, 'reserve');
  render(<ProjectRuns {...props()} />);
  await screen.findByText('Coding');
  expect(screen.getByRole('button', { name: 'Open coding session' })).not.toBeNull();
  expect(screen.getByRole('button', { name: 'Reconcile run' })).not.toBeNull();
  expect(screen.getByRole('button', { name: 'Review current plan' })).not.toBeNull();
  expect(screen.queryByRole('button', { name: 'Continue reserved coding' })).toBeNull();
  expect(screen.queryByRole('button', { name: 'Start another coding attempt' })).toBeNull();
  await userEvent.click(screen.getByText('Reviewed inputs and evidence'));
  expect(screen.getByText(/Attempt 1 · reserved · s-bound/)).not.toBeNull();
  expect(screen.getByRole('button', { name: 'Cancel run' })).not.toBeNull();
  expect(create).not.toHaveBeenCalled(); expect(reserve).not.toHaveBeenCalled();
});

it('retains the original Open trigger across a delayed session read so its destination can protect newer focus', async () => {
  vi.mocked(projectRunsApi.list).mockResolvedValue([makeRun({ state: 'coding', operations: [operation] })]);
  let resolve!: (session: ReturnType<typeof aSession>) => void;
  vi.spyOn(api, 'getSession').mockReturnValue(new Promise(done => { resolve = done; }));
  const opened = vi.fn();
  render(<><input aria-label="Other work" /><ProjectRuns {...props()} onOpenCodingSession={opened} /></>);
  const trigger = await screen.findByRole('button', { name: 'Open coding session' });
  await userEvent.click(trigger);
  const other = screen.getByRole('textbox', { name: 'Other work' }); await userEvent.click(other);
  await act(async () => resolve(aSession({ id: operation.childId, projectId: 'p-1', branchId: 'b-1', worktreeId: 'w-1', projectRunId: 'run-1', projectRunOperationId: operation.id })));
  await waitFor(() => expect(opened).toHaveBeenCalledTimes(1));
  expect(opened.mock.calls[0][0].projectId).toBe('p-1');
  expect(opened.mock.calls[0][0].sessionId).toBe(operation.childId);
  expect(opened.mock.calls[0][0].origin).toBe(trigger);
  expect(document.activeElement).toBe(other);
});

it('preserves accepted evidence and stopped-session status separately from current context freshness', async () => {
  const stopped = makeRun({ id: 'run-stopped', state: 'awaiting-checks', operations: [{ ...operation, status: 'cancelled' }], contextFreshness: { status: 'changed', reason: 'The implementation plan changed.' }, allowedActions: ['join-check', 'reconcile', 'start-coding'] });
  const accepted = makeRun({ state: 'complete', contextFreshness: { status: 'changed', reason: 'The design was reviewed again.' }, acceptance: { acceptedAt: '2026-10-06T01:00:00Z', checkId: 'check-1', releaseId: 'release-1' }, observation: { state: 'awaiting-release', valid: true, observedAt: '2026-10-06T00:59:00Z', reason: 'Matching evidence' }, allowedActions: ['reconcile'] });
  vi.mocked(projectRunsApi.list).mockResolvedValue([accepted, stopped]);
  render(<ProjectRuns {...props()} />);
  await screen.findByText('Accepted outcome');
  expect(screen.getByText(/historical acceptance is retained/)).not.toBeNull();
  expect(screen.getByText(/Last observation valid/)).not.toBeNull();
  expect(screen.queryByRole('button', { name: 'Review and accept outcome' })).toBeNull();
  await userEvent.click(screen.getByText('Run history · 2'));
  await userEvent.click(screen.getByRole('button', { name: /^Awaiting verification ·/ }));
  expect(screen.queryByRole('button', { name: 'Verify this build' })).toBeNull();
  expect(screen.queryByRole('button', { name: 'Select check evidence' })).toBeNull();
  expect(screen.queryByRole('button', { name: 'Start another coding attempt' })).toBeNull();
  expect(screen.getByText(/Review the current plan before continuing this run/)).not.toBeNull();
  expect(screen.getByRole('button', { name: 'Open coding session' })).not.toBeNull();
});

it('retries an unavailable context through read-only history refresh without losing the prepared run', async () => {
  const run = makeRun({ contextFreshness: { status: 'unavailable', reason: 'The reviewed context could not be read. Check context storage and refresh history.' } });
  vi.mocked(projectRunsApi.list).mockResolvedValueOnce([run]).mockResolvedValue([{ ...run, contextFreshness: { status: 'current', reason: 'The saved reviewed context still matches. Source, model availability and running operations are checked separately.' } }]);
  const reconcile = vi.spyOn(projectRunsApi, 'reconcile'), create = vi.spyOn(api, 'createSession');
  render(<ProjectRuns {...props()} />);
  await screen.findByText('Context unavailable');
  expect(screen.queryByRole('button', { name: 'Start coding' })).toBeNull();
  await userEvent.click(screen.getByRole('button', { name: 'Refresh history' }));
  await screen.findByRole('button', { name: 'Start coding' });
  expect(screen.getByText(/Source, model availability and running operations are checked separately/)).not.toBeNull();
  expect(reconcile).not.toHaveBeenCalled(); expect(create).not.toHaveBeenCalled();
});

it.each(['awaiting-checks', 'awaiting-release'] as const)('routes a %s run with changed inputs to plan review instead of rejected evidence actions', async state => {
  vi.mocked(projectRunsApi.list).mockResolvedValue([makeRun({ state, contextFreshness: { status: 'changed', reason: 'The saved implementation plan changed.' }, observation: { state, valid: true, observedAt: '2026-10-06T00:59:00Z', reason: 'Previous evidence matched.' }, allowedActions: ['start-coding', 'join-check', 'join-release', 'accept', 'reconcile', 'cancel'] })]);
  const initial = props();
  render(<ProjectRuns {...initial} />);
  await userEvent.click(await screen.findByRole('button', { name: 'Review current plan' }));
  expect(initial.onReviewPlan).toHaveBeenCalledTimes(1);
  for (const name of ['Verify this build', 'Preview this build', 'Select check evidence', 'Select release evidence', 'Review and accept outcome', 'Start another coding attempt']) expect(screen.queryByRole('button', { name })).toBeNull();
  expect(screen.getByRole('button', { name: 'Reconcile run' })).not.toBeNull();
});

it('reveals the exact blocking question from a run whose reviewed documents still match without preparing a reply', async () => {
  vi.mocked(projectRunsApi.list).mockResolvedValue([makeRun({ contextFreshness: { status: 'current', reason: 'Saved reviewed context matches.' }, codingBlocker: { kind: 'question', turnId: 'turn-blocking', reason: 'Resolve the access decision before more coding.' }, allowedActions: ['start-coding', 'join-check', 'accept', 'reconcile', 'cancel'] })]);
  vi.spyOn(orchestratorApi, 'get').mockResolvedValue(orchestratorView({ blockingQuestions: [{ turnId: 'turn-other', question: 'A different decision?', rationale: '', options: [] }, { turnId: 'turn-blocking', question: 'Are notes shared?', rationale: 'Access rules affect implementation.', options: [] }] }));
  const create = vi.spyOn(api, 'createSession'), send = vi.spyOn(orchestratorApi, 'send');
  render(<><ProjectRuns {...props()} /><OrchestratorPanel projectId="p-1" /></>);
  await userEvent.click(await screen.findByRole('button', { name: 'Review blocking question' }));
  const group = await screen.findByRole('group', { name: 'Are notes shared?' });
  await waitFor(() => expect(document.activeElement).toBe(within(group).getByRole('button', { name: 'Answer this decision' })));
  expect(screen.getByText('Decision needed before coding')).not.toBeNull();
  expect(screen.queryByRole('button', { name: 'Start coding' })).toBeNull();
  expect(screen.queryByRole('button', { name: 'Select check evidence' })).toBeNull();
  expect(screen.queryByText(/Replying to:/)).toBeNull();
  expect(create).not.toHaveBeenCalled(); expect(send).not.toHaveBeenCalled();
});

it('offers a read-only decisions remedy when its ledger is unavailable and restores actions only after refreshed guidance', async () => {
  vi.mocked(projectRunsApi.list).mockResolvedValueOnce([makeRun({ codingBlocker: { kind: 'unavailable', reason: 'Project decisions could not be read.' } })]).mockResolvedValue([makeRun()]);
  const reserve = vi.spyOn(projectRunsApi, 'reserve'), reconcile = vi.spyOn(projectRunsApi, 'reconcile');
  render(<ProjectRuns {...props()} />);
  await screen.findByText('Project decisions unavailable');
  expect(screen.getByRole('button', { name: 'Check project decisions' })).not.toBeNull();
  expect(screen.queryByRole('button', { name: 'Start coding' })).toBeNull();
  await userEvent.click(screen.getByRole('button', { name: 'Refresh history' }));
  await screen.findByRole('button', { name: 'Start coding' });
  expect(reserve).not.toHaveBeenCalled(); expect(reconcile).not.toHaveBeenCalled();
});

it('reserves once before coding and sends the recorded route rather than current defaults', async () => {
  let resolve!: (value: { run: ProjectRun; operation: RunOperation }) => void;
  const reserve = vi.spyOn(projectRunsApi, 'reserve').mockReturnValue(new Promise(done => { resolve = done; }));
  const create = vi.spyOn(api, 'createSession').mockResolvedValue(aSession({ id: 's-bound', projectId: 'p-1', projectRunId: 'run-1', projectRunOperationId: 'op-1' }));
  vi.spyOn(projectRunsApi, 'get').mockResolvedValue(makeRun({ state: 'coding', operations: [operation], allowedActions: ['reconcile', 'cancel'] }));
  render(<ProjectRuns {...props()} />);
  const start = await screen.findByRole('button', { name: 'Start coding' });
  await userEvent.dblClick(start);
  expect(reserve).toHaveBeenCalledTimes(1); expect(create).not.toHaveBeenCalled();
  await act(async () => resolve({ run: makeRun({ revision: 2, operations: [operation] }), operation }));
  await screen.findByText(/Coding session started/);
  expect(create).toHaveBeenCalledTimes(1);
  expect(create.mock.calls[0][0]).toMatchObject({ harness: 'claude-code', model: 'saved-model', agentConnectionId: 'saved-connection', worktreeId: 'w-1', projectRunId: 'run-1', projectRunOperationId: 'op-1', implementationContext: { taskIds: ['task-1'], planRevision: 5 } });
});

it.each(['changed', 'unavailable'] as const)('retains a reservation without dispatch when its response reports %s context', async status => {
  vi.spyOn(projectRunsApi, 'reserve').mockResolvedValue({ run: makeRun({ revision: 2, operations: [operation], contextFreshness: { status, reason: 'Review the current context before more coding.' }, allowedActions: ['continue-coding', 'reconcile', 'cancel'] }), operation });
  const create = vi.spyOn(api, 'createSession');
  render(<ProjectRuns {...props()} />);
  await userEvent.click(await screen.findByRole('button', { name: 'Start coding' }));
  await screen.findByText(/The coding reservation is retained/);
  expect(create).not.toHaveBeenCalled();
  expect(screen.getByRole('button', { name: 'Review current plan' })).not.toBeNull();
  expect(screen.queryByRole('button', { name: 'Continue reserved coding' })).toBeNull();
  await userEvent.click(screen.getByText('Reviewed inputs and evidence'));
  expect(screen.getByText(/Attempt 1 · reserved · s-bound/)).not.toBeNull();
});

it('retains a reserved operation when a newly reported material question blocks dispatch', async () => {
  vi.spyOn(projectRunsApi, 'reserve').mockResolvedValue({ run: makeRun({ revision: 2, operations: [operation], codingBlocker: { kind: 'question', turnId: 'turn-blocking', reason: 'Resolve access before implementation.' }, allowedActions: ['continue-coding', 'reconcile', 'cancel'] }), operation });
  const create = vi.spyOn(api, 'createSession');
  render(<ProjectRuns {...props()} />);
  await userEvent.click(await screen.findByRole('button', { name: 'Start coding' }));
  await screen.findByText('The coding reservation is retained. Review the project decisions before starting more coding.');
  expect(screen.getByRole('button', { name: 'Review blocking question' })).not.toBeNull();
  expect(screen.queryByRole('button', { name: 'Continue reserved coding' })).toBeNull();
  expect(create).not.toHaveBeenCalled();
});

it('does not allocate a replacement or infer success after an ambiguous coding response', async () => {
  vi.spyOn(projectRunsApi, 'reserve').mockResolvedValue({ run: makeRun({ state: 'coding', revision: 2, operations: [operation], allowedActions: ['reconcile', 'cancel'] }), operation });
  const create = vi.spyOn(api, 'createSession').mockRejectedValue(new Error('Connection interrupted'));
  render(<ProjectRuns {...props()} />);
  await userEvent.click(await screen.findByRole('button', { name: 'Start coding' }));
  await screen.findByText(/Connection interrupted.*reconcile/);
  expect(create).toHaveBeenCalledTimes(1);
  expect(screen.queryByRole('button', { name: 'Start coding' })).toBeNull();
  expect(screen.getByRole('button', { name: 'Reconcile run' })).not.toBeNull();
});

it('continues the same reserved child after reload only through its explicit action', async () => {
  vi.mocked(projectRunsApi.list).mockResolvedValue([makeRun({ state: 'coding', revision: 2, operations: [operation], allowedActions: ['continue-coding', 'reconcile', 'cancel'] })]);
  const reserve = vi.spyOn(projectRunsApi, 'reserve');
  const create = vi.spyOn(api, 'createSession').mockResolvedValue(aSession({ id: operation.childId, projectId: 'p-1', projectRunId: 'run-1', projectRunOperationId: operation.id }));
  vi.spyOn(projectRunsApi, 'get').mockResolvedValue(makeRun({ state: 'coding', operations: [{ ...operation, status: 'running' }], allowedActions: ['reconcile', 'cancel'] }));
  render(<ProjectRuns {...props()} />);
  const button = await screen.findByRole('button', { name: 'Continue reserved coding' });
  expect(create).not.toHaveBeenCalled();
  await userEvent.click(button);
  await screen.findByText(/Coding session started/);
  expect(reserve).not.toHaveBeenCalled();
  expect(create.mock.calls[0][0]).toMatchObject({ projectRunId: 'run-1', projectRunOperationId: operation.id });
  expect(screen.queryByRole('button', { name: 'Continue reserved coding' })).toBeNull();
});

it('creates a saved run through the existing model picker without allocating a coding child', async () => {
  const create = vi.spyOn(projectRunsApi, 'create').mockResolvedValue(makeRun());
  const child = vi.spyOn(api, 'createSession');
  const initial = props(), implementationContext = makeRun().input.implementation;
  render(<ProjectRuns {...initial} draft={{ worktreeId: 'w-1', implementationContext }} />);
  const dialog = await screen.findByRole('dialog', { name: 'Create project run' });
  await userEvent.selectOptions(within(dialog).getByLabelText('Intended outcome'), 'publish');
  await userEvent.click(within(dialog).getByRole('button', { name: 'Create project run' }));
  await waitFor(() => expect(create).toHaveBeenCalledTimes(1));
  expect(create.mock.calls[0][1]).toMatchObject({ outcome: 'publish', implementationContext: { projectId: 'p-1', taskIds: ['task-1'] } });
  expect(create.mock.calls[0][1].idempotencyKey).toHaveLength(36);
  expect(child).not.toHaveBeenCalled();
});

it('does not invite coding when an idempotent create response recovers a run with changed context', async () => {
  vi.spyOn(projectRunsApi, 'create').mockResolvedValue(makeRun({ contextFreshness: { status: 'changed', reason: 'The design handoff changed after this run was prepared.' }, allowedActions: ['reconcile', 'cancel'] }));
  const child = vi.spyOn(api, 'createSession');
  render(<ProjectRuns {...props()} draft={{ worktreeId: 'w-1', implementationContext: makeRun().input.implementation }} />);
  const dialog = await screen.findByRole('dialog', { name: 'Create project run' });
  await userEvent.click(within(dialog).getByRole('button', { name: 'Create project run' }));
  await screen.findByText('Run saved. Review its current context before starting more coding.');
  expect(screen.queryByText(/Start coding when ready/)).toBeNull();
  expect(screen.queryByRole('button', { name: 'Start coding' })).toBeNull();
  expect(child).not.toHaveBeenCalled();
});

it('joins only a selected existing check ID and preserves a server rejection', async () => {
  vi.mocked(projectRunsApi.list).mockResolvedValue([makeRun({ state: 'awaiting-checks', allowedActions: ['join-check', 'reconcile'] })]);
  vi.spyOn(productApi, 'checks').mockResolvedValue({ suggested: [], runs: [{ id: 'check-failed', projectId: 'p-1', status: 'failed', commands: [], commit: 'b'.repeat(40), dirty: false, createdAt: '', updatedAt: '', message: 'Tests failed' }] });
  const join = vi.spyOn(projectRunsApi, 'joinCheck').mockRejectedValue(new Error('Checks failed or source changed'));
  render(<ProjectRuns {...props()} />);
  await userEvent.click(await screen.findByRole('button', { name: 'Select check evidence' }));
  const dialog = await screen.findByRole('dialog', { name: 'Select check evidence' });
  await userEvent.selectOptions(within(dialog).getByLabelText('Check result'), 'check-failed');
  await userEvent.click(within(dialog).getByRole('button', { name: 'Join selected evidence' }));
  await waitFor(() => expect(join).toHaveBeenCalledExactlyOnceWith('p-1', 'run-1', 1, 'check-failed'));
  expect((await within(dialog).findByRole('alert')).textContent).toContain('Checks failed or source changed');
  expect(screen.queryByText('Accepted outcome')).toBeNull();
});

it('shows accepted history and the valid observation time without repeating the prior acceptance instruction', async () => {
  const observationReason = 'The checked release matches. Review the application and accept the selected outcome.';
  vi.mocked(projectRunsApi.list).mockResolvedValue([makeRun({ state: 'complete', reason: 'The reviewed outcome was accepted.', acceptance: { acceptedAt: '2026-10-06T01:00:00Z', checkId: 'check-1', releaseId: 'release-1' }, observation: { state: 'awaiting-release', valid: true, observedAt: '2026-10-06T00:59:00Z', reason: observationReason }, allowedActions: ['accept', 'reconcile'] })]);
  const accept = vi.spyOn(projectRunsApi, 'accept');
  render(<ProjectRuns {...props()} />);
  await screen.findByText(/historical acceptance is retained/);
  expect(screen.getByText(/Last observation valid/).textContent).toContain(new Date('2026-10-06T00:59:00Z').toLocaleString());
  expect(screen.queryByText(/Review the application and accept the selected outcome/)).toBeNull();
  expect(screen.queryByRole('button', { name: 'Review and accept outcome' })).toBeNull();
  expect(screen.getByRole('button', { name: 'Reconcile run' })).not.toBeNull();
  expect(accept).not.toHaveBeenCalled();
});

it('retains historical acceptance while displaying current invalidity without offering acceptance again', async () => {
  vi.mocked(projectRunsApi.list).mockResolvedValue([makeRun({ state: 'complete', acceptance: { acceptedAt: '2026-10-06T01:00:00Z', checkId: 'check-1', releaseId: 'release-1' }, observation: { state: 'blocked', valid: false, observedAt: '2026-10-06T02:00:00Z', reason: 'Design approval changed.' }, allowedActions: ['reconcile'] })]);
  render(<ProjectRuns {...props()} />);
  await screen.findByText(/historical acceptance is retained/);
  expect(screen.getByText(/Current evidence needs attention.*Design approval changed/)).not.toBeNull();
  expect(screen.queryByRole('button', { name: 'Review and accept outcome' })).toBeNull();
});

it('retains pruned check provenance and joins a selected release with its exact target', async () => {
  vi.mocked(projectRunsApi.list).mockResolvedValue([makeRun({ state: 'awaiting-release', checks: [{ checkId: 'check-pruned', projectId: 'p-1', commit: 'a'.repeat(40), sourceDigest: 'source', approvals, requirements: [], commands: [{ label: 'Tests', executable: 'npm', args: ['test'], status: 'passed', exitCode: 0 }], execution: 'docker', createdAt: '', joinedAt: '', originalAvailable: false }], allowedActions: ['join-release', 'reconcile'] })]);
  vi.spyOn(productApi, 'releases').mockResolvedValue({ releases: [{ id: 'release-pages', projectId: 'p-1', briefRevision: 2, commit: 'a'.repeat(40), target: 'pages', status: 'ready', url: 'https://example.com', message: 'Observed', createdAt: '' }] });
  const join = vi.spyOn(projectRunsApi, 'joinRelease').mockRejectedValue(new Error('Release commit does not match checked source'));
  render(<ProjectRuns {...props()} />);
  await screen.findByText('Reviewed inputs saved.');
  await userEvent.click(screen.getByText('Reviewed inputs and evidence'));
  expect(screen.getByText(/Original check history is unavailable/)).not.toBeNull();
  expect(screen.getByText('Tests · passed · exit 0')).not.toBeNull();
  await userEvent.click(screen.getByRole('button', { name: 'Select release evidence' }));
  const dialog = await screen.findByRole('dialog', { name: 'Select release evidence' });
  await userEvent.selectOptions(within(dialog).getByLabelText('Release result'), JSON.stringify(['pages', 'release-pages']));
  await userEvent.click(within(dialog).getByRole('button', { name: 'Join selected evidence' }));
  await waitFor(() => expect(join).toHaveBeenCalledExactlyOnceWith('p-1', 'run-1', 1, 'release-pages', 'pages'));
  expect((await within(dialog).findByRole('alert')).textContent).toContain('Release commit does not match');
});

it('keeps the create idempotency key after an uncertain response without starting a child', async () => {
  const create = vi.spyOn(projectRunsApi, 'create').mockRejectedValueOnce(new Error('Response interrupted')).mockResolvedValue(makeRun());
  const child = vi.spyOn(api, 'createSession');
  render(<ProjectRuns {...props()} draft={{ worktreeId: 'w-1', implementationContext: makeRun().input.implementation }} />);
  const dialog = await screen.findByRole('dialog', { name: 'Create project run' });
  await userEvent.click(within(dialog).getByRole('button', { name: 'Create project run' }));
  await within(dialog).findByText(/Response interrupted/);
  await userEvent.click(within(dialog).getByRole('button', { name: 'Create project run' }));
  await waitFor(() => expect(create).toHaveBeenCalledTimes(2));
  expect(create.mock.calls[0][1].idempotencyKey).toBe(create.mock.calls[1][1].idempotencyKey);
  expect(child).not.toHaveBeenCalled();
});

it('offers native deployment evidence even when the standard release history is unavailable', async () => {
  vi.mocked(projectRunsApi.list).mockResolvedValue([makeRun({ state: 'awaiting-release', allowedActions: ['join-release', 'reconcile'] })]);
  vi.spyOn(productApi, 'releases').mockRejectedValue(new Error('Product release history unavailable'));
  vi.mocked(platformApi.project).mockResolvedValue({ busy: false, project: { projectId: 'p-1', database: { engine: 'postgres', status: 'missing', message: '' }, backups: [], deployments: [{ id: 'native-1', projectId: 'p-1', provider: 'docker', commit: 'a'.repeat(40), configurationRevision: 0, status: 'ready', message: 'Healthy', createdAt: '', updatedAt: '' }] } });
  const join = vi.spyOn(projectRunsApi, 'joinRelease').mockResolvedValue(makeRun({ allowedActions: ['accept'] }));
  render(<ProjectRuns {...props()} />);
  await userEvent.click(await screen.findByRole('button', { name: 'Select release evidence' }));
  const dialog = await screen.findByRole('dialog', { name: 'Select release evidence' });
  expect(within(dialog).getByText(/Some deployment history could not be loaded/)).not.toBeNull();
  await userEvent.selectOptions(within(dialog).getByLabelText('Release result'), JSON.stringify(['native-docker', 'native-1']));
  await userEvent.click(within(dialog).getByRole('button', { name: 'Join selected evidence' }));
  await waitFor(() => expect(join).toHaveBeenCalledExactlyOnceWith('p-1', 'run-1', 1, 'native-1', 'native-docker'));
});

it('requires explicit behavior acceptance and sends only expected revision', async () => {
  vi.mocked(projectRunsApi.list).mockResolvedValue([makeRun({ state: 'awaiting-release', observation: { state: 'awaiting-release', reason: 'Matching evidence', valid: true, observedAt: '2026-10-06T00:00:00Z' }, allowedActions: ['accept', 'reconcile'] })]);
  const accept = vi.spyOn(projectRunsApi, 'accept').mockResolvedValue(makeRun({ state: 'complete', allowedActions: ['reconcile'] }));
  render(<ProjectRuns {...props()} />);
  await userEvent.click(await screen.findByRole('button', { name: 'Review and accept outcome' }));
  const dialog = screen.getByRole('dialog', { name: 'Review and accept outcome' });
  const button = within(dialog).getByRole('button', { name: 'Accept reviewed outcome' });
  expect((button as HTMLButtonElement).disabled).toBe(true); expect(accept).not.toHaveBeenCalled();
  await userEvent.click(within(dialog).getByRole('checkbox'));
  await userEvent.click(button);
  await waitFor(() => expect(accept).toHaveBeenCalledExactlyOnceWith('p-1', 'run-1', 1));
});

it('drops late project history and does not dispatch a reservation after switching account or workspace', async () => {
  let resolve!: (value: { run: ProjectRun; operation: RunOperation }) => void;
  vi.spyOn(projectRunsApi, 'reserve').mockReturnValue(new Promise(done => { resolve = done; }));
  const create = vi.spyOn(api, 'createSession');
  const initial = props();
  const rendered = render(<ProjectRuns {...initial} />);
  await userEvent.click(await screen.findByRole('button', { name: 'Start coding' }));
  scope.actor = 'member'; scope.connection = 'remote-workspace';
  vi.mocked(projectRunsApi.list).mockResolvedValue([]);
  rendered.rerender(<ProjectRuns {...initial} />);
  await screen.findByRole('button', { name: 'Review implementation plan' });
  await act(async () => resolve({ run: makeRun({ operations: [operation] }), operation }));
  expect(create).not.toHaveBeenCalled();
  expect(screen.queryByText('Reviewed inputs saved.')).toBeNull();
});

it('rejects foreign project receipts and only requests explicit cancellation of future dispatch', async () => {
  vi.mocked(projectRunsApi.list).mockResolvedValue([makeRun({ projectId: 'p-other' })]);
  const rendered = render(<ProjectRuns {...props()} />);
  await screen.findByText(/history belongs to another project/);
  expect(screen.queryByRole('button', { name: 'Start coding' })).toBeNull();
  rendered.unmount();
  vi.mocked(projectRunsApi.list).mockResolvedValue([makeRun()]);
  const cancel = vi.spyOn(projectRunsApi, 'cancel').mockResolvedValue(makeRun({ state: 'cancelled', allowedActions: [] }));
  render(<ProjectRuns {...props()} />);
  await screen.findByText('Reviewed inputs saved.');
  await userEvent.click(screen.getByText('Reviewed inputs and evidence'));
  await userEvent.click(screen.getByRole('button', { name: 'Cancel run' }));
  const dialog = screen.getByRole('dialog', { name: 'Cancel run' });
  expect(within(dialog).getByText(/does not stop an existing coding session/)).not.toBeNull();
  await userEvent.click(within(dialog).getByRole('button', { name: 'Cancel future dispatch' }));
  await waitFor(() => expect(cancel).toHaveBeenCalledExactlyOnceWith('p-1', 'run-1', 1));
});

const teamAccess = vi.hoisted(() => ({ loading: false, error: "", permissions: { read: true, edit: true, execute: true, secrets: true, deploy: true, admin: true } }));
vi.mock("../teams/TeamWorkspaceProvider", () => ({ useTeamProjectPermissions: () => teamAccess, useTeamWorkspace: () => null }));
beforeEach(() => { teamAccess.loading = false; teamAccess.error = ""; Object.assign(teamAccess.permissions, { read: true, edit: true, execute: true, secrets: true, deploy: true, admin: true }); });
it('exposes run history to viewers and reconciliation to editors without granting execution', async () => {
  teamAccess.permissions.edit = false; teamAccess.permissions.execute = false;
  const reserve = vi.spyOn(projectRunsApi, 'reserve'), reconcile = vi.spyOn(projectRunsApi, 'reconcile').mockResolvedValue(makeRun());
  const view = render(<ProjectRuns {...props()} />); await screen.findByText('Reviewed inputs saved.');
  expect(screen.queryByRole('button', { name: 'Start coding' })).toBeNull(); expect(screen.queryByRole('button', { name: 'Reconcile run' })).toBeNull();
  teamAccess.permissions.edit = true; view.rerender(<ProjectRuns {...props()} />);
  await userEvent.click(screen.getByRole('button', { name: 'Reconcile run' }));
  await waitFor(() => expect(reconcile).toHaveBeenCalledOnce()); expect(reserve).not.toHaveBeenCalled();
});
it('does not dispatch a reserved child when execution permission changes during reservation', async () => {
  let finish!: (value: { run: ProjectRun; operation: RunOperation }) => void;
  vi.spyOn(projectRunsApi, 'reserve').mockImplementation(() => new Promise(resolve => { finish = resolve; }));
  const create = vi.spyOn(api, 'createSession');
  const view = render(<ProjectRuns {...props()} />);
  await userEvent.click(await screen.findByRole('button', { name: 'Start coding' }));
  teamAccess.permissions.execute = false; view.rerender(<ProjectRuns {...props()} />);
  await act(async () => finish({ run: makeRun({ state: 'coding', operations: [operation], allowedActions: ['continue-coding', 'reconcile', 'cancel'] }), operation }));
  await screen.findByText(/Execution access changed/);
  expect(create).not.toHaveBeenCalled();
});

function InspectionJourney() {
  const lens = useCockpit(state => state.lens);
  return <><OrchestratorPanel projectId="p-1" />{lens === 'develop' && <ProjectRuns {...props()} />}</>;
}
function inspectionOverview() {
  const snapshot = orchestratorContext();
  snapshot.currentRun = { id: 'run-new', state: 'ready', reason: 'New scope prepared', valid: false, tasks: [{ id: 'new-task', title: 'New calendar export', status: 'selected' }] };
  snapshot.activeRuns = [{ id: 'run-older', state: 'cancelled', reason: 'Future dispatch cancelled', valid: false, activity: 'running', dispatchCancelled: true,
    operations: [{ id: 'op-older', childId: 'session-older', status: 'running', updatedAt: '2026-10-06T00:00:00Z' }], tasks: [{ id: 'older-task', title: 'Original private booking', status: 'selected' }] }];
  vi.spyOn(orchestratorApi, 'get').mockResolvedValue(orchestratorView({ context: snapshot, nextAction: { id: 'inspect-run', label: 'Inspect active builds', reason: 'Inspect the existing activity.', lens: 'develop', runId: 'run-older' } }));
  useCockpit.setState({ lens: 'design' });
}
it.each(['build card', 'primary next step'])('opens the exact older build through the %s after Develop mounts and history arrives, without dispatch or reconciliation', async source => {
  inspectionOverview();
  let finish!: (runs: ProjectRun[]) => void;
  vi.mocked(projectRunsApi.list).mockImplementation(() => new Promise(resolve => { finish = resolve; }));
  const reconcile = vi.spyOn(projectRunsApi, 'reconcile'), reserve = vi.spyOn(projectRunsApi, 'reserve'), create = vi.spyOn(api, 'createSession');
  render(<InspectionJourney />);
  const active = await screen.findByRole('region', { name: 'Active and unconfirmed work' });
  expect(screen.queryByRole('region', { name: 'Project runs' })).toBeNull();
  await userEvent.click(source === 'build card' ? within(active).getByRole('button', { name: 'Inspect build and checks' }) : screen.getByRole('button', { name: 'Inspect active builds' }));
  expect(useCockpit.getState().lens).toBe('develop');
  expect(useCockpit.getState().projectRunInspection?.runId).toBe('run-older');
  const older = makeRun({ id: 'run-older', state: 'cancelled', reason: 'Older run remains active', allowedActions: ['reconcile'] });
  older.input.implementation.taskTitles = ['Original private booking'];
  await act(async () => finish([makeRun({ id: 'run-new', reason: 'New run is only prepared' }), older]));
  const runs = screen.getByRole('region', { name: 'Project runs' });
  expect(await within(runs).findByText('Older run remains active')).not.toBeNull();
  expect(within(runs).queryByText('New run is only prepared')).toBeNull();
  await userEvent.click(within(runs).getByText('Reviewed inputs and evidence'));
  expect(within(runs).getByText('Original private booking')).not.toBeNull();
  expect(useCockpit.getState().projectRunInspection).toBeNull();
  expect(reconcile).not.toHaveBeenCalled(); expect(reserve).not.toHaveBeenCalled(); expect(create).not.toHaveBeenCalled();
});

it('keeps an unavailable inspection target explicit instead of selecting or starting the newest run', async () => {
  inspectionOverview();
  vi.mocked(projectRunsApi.list).mockResolvedValue([makeRun({ id: 'run-new' })]);
  const create = vi.spyOn(api, 'createSession');
  render(<InspectionJourney />);
  await userEvent.click(within(await screen.findByRole('region', { name: 'Active and unconfirmed work' })).getByRole('button', { name: 'Inspect build and checks' }));
  const runs = await screen.findByRole('region', { name: 'Project runs' });
  expect(await within(runs).findByRole('alert')).toHaveProperty('textContent', expect.stringContaining('The requested run is unavailable'));
  expect(within(runs).queryByRole('button', { name: 'Start coding' })).toBeNull();
  expect(within(runs).queryByText('Reviewed inputs saved.')).toBeNull();
  expect(create).not.toHaveBeenCalled();
});

it('discards an inspection request when the account changes before its history reply', async () => {
  inspectionOverview();
  let finish!: (runs: ProjectRun[]) => void;
  vi.mocked(projectRunsApi.list).mockImplementationOnce(() => new Promise(resolve => { finish = resolve; })).mockResolvedValue([makeRun({ id: 'other-account', reason: 'Other account history' })]);
  const mounted = render(<InspectionJourney />);
  await userEvent.click(within(await screen.findByRole('region', { name: 'Active and unconfirmed work' })).getByRole('button', { name: 'Inspect build and checks' }));
  scope.actor = 'other-owner';
  mounted.rerender(<InspectionJourney />);
  const runs = await screen.findByRole('region', { name: 'Project runs' });
  await within(runs).findByText('Other account history');
  await act(async () => finish([makeRun({ id: 'run-older', reason: 'Old account reply' })]));
  expect(within(runs).queryByText('Old account reply')).toBeNull();
  expect(within(runs).queryByRole('alert')).toBeNull();
  expect(useCockpit.getState().projectRunInspection).toBeNull();
});

it('hands verification the selected reviewed run without executing or asking for an evidence ID', async () => {
  vi.mocked(projectRunsApi.list).mockResolvedValue([makeRun({ state: 'awaiting-checks', allowedActions: ['join-check', 'reconcile', 'start-coding'] })]);
  const checks = vi.spyOn(productApi, 'checks');
  const create = vi.spyOn(api, 'createSession');
  render(<ProjectRuns {...props()} />);
  await userEvent.click(await screen.findByRole('button', { name: 'Verify this build' }));
  expect(useCockpit.getState().projectRunVerification).toMatchObject({ projectId: 'p-1', runId: 'run-1' });
  expect(useCockpit.getState().projectRunVerification?.scope).toContain('owner');
  expect(checks).not.toHaveBeenCalled(); expect(create).not.toHaveBeenCalled();
  expect(screen.queryByRole('dialog')).toBeNull();
  expect(screen.getByRole('button', { name: 'Start another coding attempt' })).not.toBeNull();
});

it('carries the last accepted check and exact source into deployment while retaining the dirty navigation guard', async () => {
  const check: ProjectRun['checks'][number] = { checkId: 'checked-version', projectId: 'p-1', commit: 'a'.repeat(40), sourceDigest: 'reviewed-source', approvals, requirements: makeRun().input.requirements, commands: [], execution: 'docker', createdAt: '', joinedAt: '', originalAvailable: true };
  vi.mocked(projectRunsApi.list).mockResolvedValue([makeRun({ state: 'awaiting-release', checks: [{ ...check, checkId: 'older-check' }, check], allowedActions: ['join-release', 'reconcile'] })]);
  useCockpit.setState({ unsavedProjectDrafts: { draft: { projectId: 'p-1', label: 'Plan', lens: 'develop' } } });
  render(<ProjectRuns {...props()} />);
  await userEvent.click(await screen.findByRole('button', { name: 'Preview this build' }));
  expect(useCockpit.getState().projectRunDeployment).toMatchObject({ projectId: 'p-1', runId: 'run-1', checkId: 'checked-version', commit: check.commit, sourceDigest: check.sourceDigest, outcome: 'preview' });
  expect(useCockpit.getState().lens).toBe('develop');
  expect(useCockpit.getState().pendingNavigation).toEqual({ kind: 'lens', lens: 'deploy' });
  act(() => useCockpit.getState().cancelNavigation());
  expect(useCockpit.getState().projectRunDeployment).toBeNull();
});

it('refreshes the exact inspection target after evidence is joined instead of continuing with its cached revision', async () => {
  render(<ProjectRuns {...props()} />);
  await screen.findByRole('button', { name: 'Start coding' });
  await userEvent.click(screen.getByRole('button', { name: 'Refresh history' }));
  await waitFor(() => expect(projectRunsApi.list).toHaveBeenCalledTimes(2));
  const get = vi.spyOn(projectRunsApi, 'get').mockResolvedValue(makeRun({ revision: 9, state: 'awaiting-release', reason: 'Fresh joined evidence', allowedActions: ['join-release', 'reconcile'] }));
  const matchingScope = JSON.stringify([productApi.baseUrl, 'owner', 'local', '', '', 'owner', '/projects', 'p-1']);
  act(() => useCockpit.setState({ projectRunInspection: { scope: matchingScope, projectId: 'p-1', runId: 'run-1', refresh: true } }));
  await screen.findByText('Fresh joined evidence');
  expect(get).toHaveBeenCalledWith('p-1', 'run-1');
  expect(useCockpit.getState().projectRunInspection).toBeNull();
  expect(screen.queryByRole('button', { name: 'Start coding' })).toBeNull();
});
