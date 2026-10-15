import { projectSwarmApi } from '../../api/projectSwarm';
import { StrictMode } from 'react';
import { act, render, screen, waitFor, within } from '@testing-library/react';
import userEvent from '@testing-library/user-event';
import { beforeEach, expect, it, vi } from 'vitest';
import { api } from '../../api';
import { projectRunsApi, type ProjectRun, type RunOperation } from '../../api/projectRuns';
import { productApi, type ImplementationPlan } from '../../api/product';
import { platformApi } from '../../api/platform';
import { resetCockpit, useCockpit } from '../../store/store';
import { aBranch, aHarness, aProject, aSession, aWorktree } from '../../test/fixtures';
import { ProjectRuns } from './ProjectRuns';
import { OrchestratorPanel } from '../orchestrator/OrchestratorPanel';
import { orchestratorApi } from '../../api/orchestrator';
import { context as orchestratorContext, view as orchestratorView } from '../orchestrator/fixtures';
import { swarmHarnessFixture, swarmQueueFixture, swarmRunFixture } from '../../test/swarmFixtures';
import { swarmQueueApi } from '../../api/swarmQueue';
import { orchestrationContractsApi } from '../../api/orchestrationContracts';
import { contractViewFixture } from '../../test/orchestrationContractFixtures';

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
  localStorage.clear();
  let approved = false;
  const contract = contractViewFixture();
  vi.spyOn(orchestrationContractsApi, 'prepare').mockImplementation(async (_project, request) => {
    contract.contract.outcome = request.outcome;
    contract.contract.route = { ...contract.contract.route, harness: request.route.harness, model: request.route.model, permissionMode: request.route.permissionMode };
    return contract;
  });
  vi.spyOn(orchestrationContractsApi, 'get').mockImplementation(async () => approved ? { ...contractViewFixture(true), contract: { ...contract.contract, ...contractViewFixture(true).contract, outcome: contract.contract.outcome, route: contract.contract.route } } : contract);
  vi.spyOn(orchestrationContractsApi, 'approve').mockImplementation(async () => {
    approved = true;
    return { ...contractViewFixture(true), contract: { ...contractViewFixture(true).contract, outcome: contract.contract.outcome, route: contract.contract.route } };
  });
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

it('prioritizes unintegrated task review over whole-build verification after a normal worker exit', async () => {
  const run = swarmRunFixture(); run.state = 'awaiting-checks';
  run.operations = [{...operation, id: 'task-op', taskId: 'a', status: 'review-required', allocation: 'ready'}];
  run.swarm!.tasks[0].operationId = 'task-op';
  useCockpit.getState().setHarnesses([swarmHarnessFixture()]);
  vi.mocked(projectRunsApi.list).mockResolvedValue([run]);
  vi.spyOn(swarmQueueApi, 'get').mockResolvedValue(swarmQueueFixture());
  const create = vi.spyOn(api, 'createSession'), review = vi.spyOn(projectSwarmApi, 'review');
  render(<ProjectRuns {...props()} />);
  await screen.findByText('Awaiting task review', {exact: true});
  expect(screen.getByText(/Review the exited task’s output and integrate accepted changes before verifying/)).not.toBeNull();
  expect(screen.getByRole('button', {name: 'Review task changes'})).toBeTruthy();
  await userEvent.click(screen.getByRole('button', {name: 'Review tasks'}));
  expect(document.activeElement).toBe(screen.getByRole('heading', {name:'Build a'}));
  expect(screen.queryByRole('dialog')).toBeNull(); expect(review).not.toHaveBeenCalled();
  expect(screen.queryByRole('button', {name: 'Review and accept outcome'})).toBeNull();
  expect(create).not.toHaveBeenCalled();
});

it('routes unsupported autonomous permissions back to explicit plan review while retaining interactive coding', async () => {
  useCockpit.getState().setHarnesses([aHarness({id: 'claude-code', name: 'Claude Code', supportsOpeningPrompt: true, autonomousTask: true, taskPermissionModes: ['bypass']})]);
  const initial = props(), create = vi.spyOn(api, 'createSession');
  render(<ProjectRuns {...initial} />);
  const review = await screen.findByRole('button', {name: 'Review plan and create a new build'});
  expect(screen.getByRole('button', {name: 'Prepare agent team'})).toHaveProperty('disabled', true);
  expect(screen.getByRole('button', {name: 'Start coding'})).toHaveProperty('disabled', false);
  await userEvent.click(review);
  expect(initial.onReviewPlan).toHaveBeenCalledOnce(); expect(create).not.toHaveBeenCalled();
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
  await screen.findByText('Coding reserved, not started');
  expect(screen.getByRole('button', { name: 'Open coding session' })).not.toBeNull();
  expect(screen.getByRole('button', { name: 'Check run status' })).not.toBeNull();
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
  expect(screen.getByText(/Last run status check recorded/)).not.toBeNull();
  expect(screen.queryByRole('button', { name: 'Review and accept outcome' })).toBeNull();
  await userEvent.click(screen.getByText('Run history · 2'));
  await userEvent.click(screen.getByRole('button', { name: /^Review current context ·/ }));
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
  expect(screen.getByRole('button', { name: 'Check run status' })).not.toBeNull();
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
  await screen.findByText(/Connection interrupted.*Check run status/);
  expect(create).toHaveBeenCalledTimes(1);
  expect(screen.queryByRole('button', { name: 'Start coding' })).toBeNull();
  expect(screen.getByRole('button', { name: 'Check run status' })).not.toBeNull();
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

it('captures the approved build contract through the existing picker without allocating a coding child', async () => {
  const create = vi.spyOn(projectRunsApi, 'create').mockResolvedValue(makeRun());
  const child = vi.spyOn(api, 'createSession');
  const initial = props(), implementationContext = makeRun().input.implementation;
  render(<ProjectRuns {...initial} draft={{ worktreeId: 'w-1', implementationContext }} />);
  const dialog = await screen.findByRole('dialog', { name: 'Review build contract' });
  await userEvent.selectOptions(within(dialog).getByLabelText('Intended outcome'), 'publish');
  await userEvent.click(within(dialog).getByRole('button', { name: 'Review build contract' }));
  await screen.findByRole('region', { name: 'Execution contract review' });
  expect(create).not.toHaveBeenCalled();
  await userEvent.click(screen.getByRole('checkbox', { name: /I reviewed these exact scopes/ }));
  await userEvent.click(screen.getByRole('button', { name: 'Approve contract and save project run' }));
  await waitFor(() => expect(create).toHaveBeenCalledTimes(1));
  expect(create.mock.calls[0][1]).toMatchObject({ outcome: 'publish', permissionMode: 'ask', implementationContext: { projectId: 'p-1', taskIds: ['task-1'] }, orchestrationContract: { id: 'contract-1', revision: 2, digest: 'a'.repeat(64), approvalId: 'approval-1' } });
  expect(create.mock.calls[0][1].idempotencyKey).toHaveLength(36);
  expect(child).not.toHaveBeenCalled();
});

it('does not invite coding when an idempotent create response recovers a run with changed context', async () => {
  vi.spyOn(projectRunsApi, 'create').mockResolvedValue(makeRun({ contextFreshness: { status: 'changed', reason: 'The design handoff changed after this run was prepared.' }, allowedActions: ['reconcile', 'cancel'] }));
  const child = vi.spyOn(api, 'createSession');
  render(<ProjectRuns {...props()} draft={{ worktreeId: 'w-1', implementationContext: makeRun().input.implementation }} />);
  const dialog = await screen.findByRole('dialog', { name: 'Review build contract' });
  await userEvent.click(within(dialog).getByRole('button', { name: 'Review build contract' }));
  await screen.findByRole('region', { name: 'Execution contract review' });
  await userEvent.click(screen.getByRole('checkbox', { name: /I reviewed these exact scopes/ }));
  await userEvent.click(screen.getByRole('button', { name: 'Approve contract and save project run' }));
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
  expect(screen.getByText(/Last run status check recorded/).textContent).toContain(new Date('2026-10-06T00:59:00Z').toLocaleString());
  expect(screen.queryByText(/Review the application and accept the selected outcome/)).toBeNull();
  expect(screen.queryByRole('button', { name: 'Review and accept outcome' })).toBeNull();
  expect(screen.getByRole('button', { name: 'Check run status' })).not.toBeNull();
  expect(accept).not.toHaveBeenCalled();
});

it('retains historical acceptance while displaying current invalidity without offering acceptance again', async () => {
  vi.mocked(projectRunsApi.list).mockResolvedValue([makeRun({ state: 'complete', acceptance: { acceptedAt: '2026-10-06T01:00:00Z', checkId: 'check-1', releaseId: 'release-1' }, observation: { state: 'blocked', valid: false, observedAt: '2026-10-06T02:00:00Z', reason: 'Design approval changed.' }, allowedActions: ['reconcile'] })]);
  render(<ProjectRuns {...props()} />);
  await screen.findByText(/historical acceptance is retained/);
  expect(screen.getByText(/Last run status check needs attention.*Design approval changed/)).not.toBeNull();
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
  const dialog = await screen.findByRole('dialog', { name: 'Review build contract' });
  await userEvent.click(within(dialog).getByRole('button', { name: 'Review build contract' }));
  await screen.findByRole('region', { name: 'Execution contract review' });
  await userEvent.click(screen.getByRole('checkbox', { name: /I reviewed these exact scopes/ }));
  await userEvent.click(screen.getByRole('button', { name: 'Approve contract and save project run' }));
  await screen.findByText(/Response interrupted/);
  await userEvent.click(screen.getByRole('checkbox', { name: /I reviewed these exact scopes/ }));
  await userEvent.click(screen.getByRole('button', { name: 'Save reviewed project run' }));
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
  const create = vi.spyOn(api, 'createSession'), checks = vi.spyOn(productApi, 'runChecks'), release = vi.spyOn(productApi, 'release');
  const view = render(<ProjectRuns {...props()} />); await screen.findByText('Reviewed inputs saved.');
  expect(screen.queryByRole('button', { name: 'Start coding' })).toBeNull(); expect(screen.queryByRole('button', { name: 'Check run status' })).toBeNull();
  teamAccess.permissions.edit = true; view.rerender(<ProjectRuns {...props()} />);
  await userEvent.click(screen.getByRole('button', { name: 'Check run status' }));
  await waitFor(() => expect(reconcile).toHaveBeenCalledExactlyOnceWith('p-1', 'run-1', 1));
  for (const operation of [reserve, create, checks, release]) expect(operation).not.toHaveBeenCalled();
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
  const read = vi.spyOn(projectRunsApi, 'get').mockResolvedValue(older);
  await act(async () => finish([makeRun({ id: 'run-new', reason: 'New run is only prepared' }), older]));
  const runs = screen.getByRole('region', { name: 'Project runs' });
  expect(await within(runs).findByText('Older run remains active')).not.toBeNull();
  expect(within(runs).queryByText('New run is only prepared')).toBeNull();
  await userEvent.click(within(runs).getByText('Reviewed inputs and evidence'));
  expect(within(runs).getByText('Original private booking')).not.toBeNull();
  expect(useCockpit.getState().projectRunInspection).toBeNull();
  expect(read).toHaveBeenCalledExactlyOnceWith('p-1', 'run-older');
  expect(reconcile).not.toHaveBeenCalled(); expect(reserve).not.toHaveBeenCalled(); expect(create).not.toHaveBeenCalled();
});

it('keeps an unavailable inspection target explicit instead of selecting or starting the newest run', async () => {
  inspectionOverview();
  vi.mocked(projectRunsApi.list).mockResolvedValue([makeRun({ id: 'run-new' })]);
  vi.spyOn(projectRunsApi, 'get').mockRejectedValue(new Error('The requested run is unavailable in this project. Refresh history or choose another recorded run.'));
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
  const read = vi.spyOn(projectRunsApi, 'get');
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
  expect(read).not.toHaveBeenCalled();
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

it('refreshes a cached run from the overview before carrying its newly joined check into deployment', async () => {
  const cached = makeRun({ state: 'awaiting-checks', reason: 'Checks have not been joined yet.', allowedActions: ['join-check', 'reconcile'] });
  vi.mocked(projectRunsApi.list).mockResolvedValue([cached]);
  const snapshot = orchestratorContext();
  snapshot.currentRun = { id: cached.id, state: cached.state, valid: false, reason: cached.reason, outcome: 'preview' };
  const overview = vi.spyOn(orchestratorApi, 'get').mockResolvedValue(orchestratorView({ context: snapshot, nextAction: { id: 'check', label: 'Verify the implementation', reason: 'Verify this run.', lens: 'develop', runId: cached.id } }));
  let finish!: (value: ProjectRun) => void;
  const read = vi.spyOn(projectRunsApi, 'get').mockImplementation(() => new Promise(resolve => { finish = resolve; }));
  const create = vi.spyOn(api, 'createSession'), reserve = vi.spyOn(projectRunsApi, 'reserve'), reconcile = vi.spyOn(projectRunsApi, 'reconcile');
  useCockpit.setState({ lens: 'develop' });
  render(<InspectionJourney />);
  await screen.findByRole('button', { name: 'Verify this build' });
  // A teammate joins the check while this workspace retains its older history.
  overview.mockResolvedValue(orchestratorView({ context: { ...snapshot, currentRun: { ...snapshot.currentRun, state: 'awaiting-release', valid: true } }, nextAction: { id: 'deploy', label: 'Review verified build', reason: 'Inspect the exact checked build before previewing it.', lens: 'deploy', runId: cached.id } }));
  await userEvent.click(screen.getByRole('button', { name: 'Refresh project understanding' }));
  await userEvent.click(await screen.findByRole('button', { name: 'Review verified build' }));
  await waitFor(() => expect(read).toHaveBeenCalledExactlyOnceWith('p-1', cached.id));
  expect(useCockpit.getState().lens).toBe('develop');
  expect(useCockpit.getState().projectRunDeployment).toBeNull();
  expect(screen.queryByRole('button', { name: 'Preview this build' })).toBeNull();
  const check: ProjectRun['checks'][number] = { checkId: 'newly-joined-check', projectId: 'p-1', commit: 'b'.repeat(40), sourceDigest: 'newly-checked-source', approvals, requirements: cached.input.requirements, commands: [], execution: 'docker', createdAt: '', joinedAt: '', originalAvailable: true };
  await act(async () => finish({ ...cached, revision: 9, state: 'awaiting-release', reason: 'Matching independent checks joined.', checks: [check], allowedActions: ['join-release', 'reconcile'] }));
  expect(screen.queryByRole('button', { name: 'Verify this build' })).toBeNull();
  await userEvent.click(await screen.findByRole('button', { name: 'Preview this build' }));
  expect(useCockpit.getState().projectRunDeployment).toMatchObject({ projectId: 'p-1', runId: cached.id, checkId: check.checkId, commit: check.commit, sourceDigest: check.sourceDigest, outcome: 'preview' });
  expect(useCockpit.getState().lens).toBe('deploy');
  expect(projectRunsApi.list).toHaveBeenCalledTimes(1);
  expect(create).not.toHaveBeenCalled(); expect(reserve).not.toHaveBeenCalled(); expect(reconcile).not.toHaveBeenCalled();
});


it.each(['awaiting-checks', 'awaiting-release'] as const)('reframes recorded %s guidance after a saved plan amendment instead of advertising it as the next step', async state => {
  const reason = state === 'awaiting-release' ? 'Checks are linked to the reviewed requirements and source. Join a matching observed release next.' : 'Coding finished. Join independent check evidence next.';
  const saved = makeRun({ state, reason, contextFreshness: { status: 'current', reason: 'The saved reviewed context matches.' }, observation: { state, reason, valid: true, observedAt: '2026-10-06T00:59:00Z' }, allowedActions: ['join-check', 'join-release', 'reconcile', 'accept', 'cancel'] });
  const amended = { ...saved, contextFreshness: { status: 'changed' as const, reason: 'The saved implementation plan changed. Review the current plan.' } };
  const original = structuredClone(saved);
  vi.mocked(projectRunsApi.list).mockResolvedValueOnce([saved]).mockResolvedValue([amended]);
  const initial = props(), reconcile = vi.spyOn(projectRunsApi, 'reconcile'), joinCheck = vi.spyOn(projectRunsApi, 'joinCheck'), joinRelease = vi.spyOn(projectRunsApi, 'joinRelease'), accept = vi.spyOn(projectRunsApi, 'accept'), create = vi.spyOn(api, 'createSession');
  render(<ProjectRuns {...initial} />);
  await screen.findByText(reason);
  await userEvent.click(screen.getByRole('button', { name: 'Refresh history' }));
  const review = await screen.findByRole('button', { name: 'Review current plan' });
  const workspace = screen.getByRole('region', { name: 'Project runs' });
  const next = workspace.querySelector('.project-run-next')!;
  expect(within(next as HTMLElement).getByText('Review current context')).not.toBeNull();
  expect(next.textContent).not.toContain('Join a matching observed release next');
  expect(next.textContent).not.toContain('Join independent check evidence next');
  expect(next.textContent).not.toContain(state === 'awaiting-release' ? 'Awaiting release evidence' : 'Awaiting verification');
  await userEvent.click(review);
  expect(initial.onReviewPlan).toHaveBeenCalledTimes(1);
  await userEvent.click(screen.getByText('Reviewed inputs and evidence'));
  const receipts = screen.getByText('Reviewed inputs and evidence').closest('details')!;
  expect(within(receipts).getByText(`Last recorded stage: ${state === 'awaiting-release' ? 'Awaiting release evidence' : 'Awaiting verification'}`)).not.toBeNull();
  expect(within(receipts).getByText(`Last recorded guidance: ${reason}`)).not.toBeNull();
  expect(within(receipts).getByText('Build journal')).not.toBeNull();
  expect(within(receipts).getByText('Brief 2 · Plan 5 · Definition 3 · Design 4')).not.toBeNull();
  expect(saved).toEqual(original);
  for (const mutation of [reconcile, joinCheck, joinRelease, accept, create]) expect(mutation).not.toHaveBeenCalled();
});


it.each(['changed', 'unavailable'] as const)('keeps prior release guidance in its receipts when context is %s', async status => {
  const run = makeRun({ state: 'awaiting-release', reason: 'Join a matching observed release next.', contextFreshness: { status, reason: 'Review the current inputs before more work.' }, observation: { state: 'awaiting-release', reason: 'Review the application and accept its outcome.', valid: true, observedAt: '2026-10-06T00:59:00Z' }, allowedActions: ['join-release', 'accept', 'reconcile'] });
  vi.mocked(projectRunsApi.list).mockResolvedValue([run]);
  render(<ProjectRuns {...props()} />);
  await screen.findByText(status === 'changed' ? 'Review current context' : 'Context unavailable');
  const next = screen.getByRole('region', { name: 'Project runs' }).querySelector('.project-run-next')!;
  expect(next.textContent).not.toMatch(/Join a matching|accept its outcome|Awaiting release evidence/);
  expect(next.textContent).toContain('This earlier status check does not clear the current review.');
  await userEvent.click(screen.getByText('Reviewed inputs and evidence'));
  expect(screen.getByText('Last recorded guidance: Join a matching observed release next.')).not.toBeNull();
  expect(screen.getByText('Last recorded status check: Review the application and accept its outcome.')).not.toBeNull();
});

it.each(['running', 'dispatching', 'unknown'] as const)('keeps recorded %s coding visible ahead of stale review guidance without dispatch', async status => {
  const run = makeRun({ state: 'awaiting-release', reason: 'Join a matching observed release next.', operations: [{ ...operation, status }], contextFreshness: { status: 'changed', reason: 'The implementation plan changed.' }, allowedActions: ['join-release', 'continue-coding', 'start-coding', 'reconcile', 'cancel'] });
  vi.mocked(projectRunsApi.list).mockResolvedValue([run]);
  const create = vi.spyOn(api, 'createSession'), reconcile = vi.spyOn(projectRunsApi, 'reconcile'), initial = props();
  render(<ProjectRuns {...initial} />);
  await screen.findByText(status === 'running' ? 'Coding recorded as running' : 'Coding outcome unconfirmed');
  const next = screen.getByRole('region', { name: 'Project runs' }).querySelector('.project-run-next')!;
  expect(next.textContent).not.toContain('Awaiting release evidence');
  expect(next.textContent).not.toContain('Join a matching observed release next');
  expect(within(next as HTMLElement).getAllByRole('button')[0].textContent).toBe('Check run status');
  expect(screen.getByRole('button', { name: 'Open coding session' })).not.toBeNull();
  await userEvent.click(screen.getByRole('button', { name: 'Review current plan' }));
  expect(initial.onReviewPlan).toHaveBeenCalledTimes(1);
  expect(screen.queryByRole('button', { name: 'Continue reserved coding' })).toBeNull();
  expect(screen.queryByRole('button', { name: 'Select release evidence' })).toBeNull();
  expect(create).not.toHaveBeenCalled(); expect(reconcile).not.toHaveBeenCalled();
});

it('retains cancellation and unconfirmed child activity when the plan has changed', async () => {
  vi.mocked(projectRunsApi.list).mockResolvedValue([makeRun({ state: 'cancelled', reason: 'Future dispatch was cancelled.', operations: [{ ...operation, status: 'unknown' }], contextFreshness: { status: 'changed', reason: 'The current plan differs.' }, allowedActions: ['reconcile'] })]);
  render(<ProjectRuns {...props()} />);
  await screen.findByText('Cancelled');
  expect(screen.getByText(/Coding outcome unconfirmed. Open the coding session/)).not.toBeNull();
  expect(screen.getByText('Future dispatch is cancelled. Running children retain their separate stop controls.')).not.toBeNull();
  expect(screen.getByRole('button', { name: 'Open coding session' })).not.toBeNull();
  expect(screen.getByRole('button', { name: 'Check run status' })).not.toBeNull();
  expect(screen.getByRole('button', { name: 'Review current plan' })).not.toBeNull();
});

it('labels a release-stage decision as current while keeping its former release instruction historical', async () => {
  vi.mocked(projectRunsApi.list).mockResolvedValue([makeRun({ state: 'awaiting-release', reason: 'Join a matching observed release next.', contextFreshness: { status: 'current', reason: 'The reviewed documents still match.' }, codingBlocker: { kind: 'question', reason: 'Resolve who may access this release.', turnId: 'access-question' }, allowedActions: ['join-release', 'accept', 'reconcile'] })]);
  render(<ProjectRuns {...props()} />);
  await screen.findByText('Decision needed before continuing');
  const next = screen.getByRole('region', { name: 'Project runs' }).querySelector('.project-run-next')!;
  expect(next.textContent).not.toContain('Join a matching observed release next');
  expect(screen.getByRole('button', { name: 'Review blocking question' })).not.toBeNull();
  expect(screen.queryByRole('button', { name: 'Select release evidence' })).toBeNull();
});


function savedComparisonPlan(patch: Partial<ImplementationPlan> = {}): ImplementationPlan {
  return { projectId: 'p-1', revision: 6, briefRevision: 2, updatedAt: '2026-10-07T10:00:00Z', tasks: [{ id: 'task-1', title: 'Build a journal with a visible revision trail', status: 'todo' }], ...patch };
}
function pendingComparisonPlan() {
  let resolve!: (plan: ImplementationPlan) => void;
  const promise = new Promise<ImplementationPlan>(done => { resolve = done; });
  return { promise, resolve };
}

it('lazily compares the captured selected task titles with saved changed, removed and unchanged titles for a viewer', async () => {
  teamAccess.permissions.edit = false; teamAccess.permissions.execute = false;
  const saved = makeRun();
  saved.input.implementation.taskIds = ['task-1', 'removed', 'unchanged'];
  saved.input.implementation.taskTitles = ['Build journal', 'Export journal entries', 'Private notes'];
  const original = structuredClone(saved);
  const longTitle = 'Build a journal with a visible revision trail that shows the author, saved time and earlier version before restoring an entry';
  const plan = vi.spyOn(productApi, 'plan').mockResolvedValue(savedComparisonPlan({ tasks: [{ id: 'task-1', title: longTitle, status: 'doing' }, { id: 'unchanged', title: 'Private notes', status: 'done' }, { id: 'unselected', title: 'Unselected billing work', status: 'todo' }] }));
  vi.mocked(projectRunsApi.list).mockResolvedValue([saved]);
  const save = vi.spyOn(productApi, 'savePlan'), reserve = vi.spyOn(projectRunsApi, 'reserve'), create = vi.spyOn(api, 'createSession');
  render(<ProjectRuns {...props()} />);
  const trigger = await screen.findByText('Compare with current plan');
  expect(plan).not.toHaveBeenCalled();
  expect(screen.queryByRole('region', { name: 'Selected task title comparison' })).toBeNull();
  await userEvent.click(trigger);
  const comparison = await screen.findByRole('region', { name: 'Selected task title comparison' });
  await within(comparison).findByText('This build: plan 5. Current saved plan: 6.');
  expect(plan).toHaveBeenCalledExactlyOnceWith('p-1');
  expect(within(comparison).getByText(longTitle).tagName).toBe('P');
  expect(within(comparison).getByText('Build journal')).not.toBeNull();
  expect(within(comparison).getByText('Export journal entries')).not.toBeNull();
  expect(within(comparison).getByText('Removed from the current saved plan.')).not.toBeNull();
  expect(within(comparison).getByText('Title changed.')).not.toBeNull();
  expect(within(comparison).getByText('Removed from saved plan.')).not.toBeNull();
  expect(within(comparison).getByText('Title unchanged.')).not.toBeNull();
  expect(within(comparison).queryByText('Unselected billing work')).toBeNull();
  expect(comparison.textContent).toContain('This build’s captured inputs and recorded checks are retained.');
  expect(comparison.textContent).toContain('It does not compare task status, unselected tasks, other context, or unsaved edits.');
  expect(comparison.textContent).toContain('Matching titles do not mean the full reviewed context still matches.');
  expect(saved).toEqual(original);
  for (const mutation of [save, reserve, create]) expect(mutation).not.toHaveBeenCalled();
});

it('clears an old comparison immediately during refresh, failure and explicit recovery', async () => {
  const pending = pendingComparisonPlan();
  const plan = vi.spyOn(productApi, 'plan').mockResolvedValueOnce(savedComparisonPlan()).mockRejectedValueOnce(new Error('Saved plan is temporarily unavailable')).mockReturnValueOnce(pending.promise);
  render(<ProjectRuns {...props()} />);
  await userEvent.click(await screen.findByText('Compare with current plan'));
  await screen.findByText('This build: plan 5. Current saved plan: 6.');
  expect(screen.getByText(/This compares titles for the 1 task selected for this build/)).not.toBeNull();
  await userEvent.click(screen.getByRole('button', { name: 'Refresh comparison' }));
  expect(await screen.findByRole('alert')).toHaveProperty('textContent', expect.stringContaining('Saved plan is temporarily unavailable'));
  expect(screen.queryByText('This build: plan 5. Current saved plan: 6.')).toBeNull();
  expect(screen.queryByText('Build a journal with a visible revision trail')).toBeNull();
  await userEvent.click(screen.getByRole('button', { name: 'Refresh comparison' }));
  await screen.findByText('Reading current saved plan…');
  expect(screen.queryByRole('alert')).toBeNull();
  await act(async () => pending.resolve(savedComparisonPlan({ revision: 7 })));
  await screen.findByText('This build: plan 5. Current saved plan: 7.');
  expect(plan).toHaveBeenCalledTimes(3);
});

it('discards a closed comparison request before reopening with a new saved snapshot', async () => {
  const pending = pendingComparisonPlan();
  const plan = vi.spyOn(productApi, 'plan').mockReturnValueOnce(pending.promise).mockResolvedValueOnce(savedComparisonPlan({ revision: 8 }));
  render(<ProjectRuns {...props()} />);
  const trigger = await screen.findByText('Compare with current plan');
  await userEvent.click(trigger);
  await waitFor(() => expect(plan).toHaveBeenCalledTimes(1));
  await userEvent.click(trigger);
  await waitFor(() => expect(screen.queryByRole('region', { name: 'Selected task title comparison' })).toBeNull());
  await userEvent.click(trigger);
  await screen.findByText('This build: plan 5. Current saved plan: 8.');
  await act(async () => pending.resolve(savedComparisonPlan({ revision: 99, tasks: [{ id: 'task-1', title: 'Late closed comparison', status: 'todo' }] })));
  expect(screen.queryByText('Late closed comparison')).toBeNull();
  expect(screen.getByText('This build: plan 5. Current saved plan: 8.')).not.toBeNull();
});

it('retires the previous selected run comparison and requires a new explicit read for its replacement', async () => {
  const pending = pendingComparisonPlan();
  const first = makeRun(), second = makeRun({ id: 'run-second', state: 'awaiting-checks', reason: 'Second captured run', allowedActions: ['reconcile'] });
  second.input.implementation.planRevision = 9; second.input.implementation.taskTitles = ['Capture a second plan'];
  vi.mocked(projectRunsApi.list).mockResolvedValue([first, second]);
  const plan = vi.spyOn(productApi, 'plan').mockReturnValueOnce(pending.promise).mockResolvedValueOnce(savedComparisonPlan({ revision: 10 }));
  render(<ProjectRuns {...props()} />);
  await userEvent.click(await screen.findByText('Compare with current plan'));
  await waitFor(() => expect(plan).toHaveBeenCalledTimes(1));
  await userEvent.click(screen.getByText('Run history · 2'));
  await userEvent.click(screen.getByRole('button', { name: /^Awaiting verification ·/ }));
  await screen.findByText('Second captured run');
  expect(screen.queryByRole('region', { name: 'Selected task title comparison' })).toBeNull();
  await act(async () => pending.resolve(savedComparisonPlan({ revision: 99 })));
  expect(screen.queryByText(/Current saved plan: 99/)).toBeNull();
  await userEvent.click(screen.getByText('Compare with current plan'));
  await screen.findByText('This build: plan 9. Current saved plan: 10.');
  const comparison = screen.getByRole('region', { name: 'Selected task title comparison' });
  expect(within(comparison).getByText('Capture a second plan')).not.toBeNull();
  expect(within(comparison).queryByText('Build journal')).toBeNull();
});

it.each(['account', 'workspace', 'project'] as const)('does not carry a saved-plan comparison across a %s switch', async change => {
  const pending = pendingComparisonPlan();
  vi.mocked(projectRunsApi.list).mockImplementation(async projectId => {
    const run = makeRun({ projectId }); run.input.implementation.projectId = projectId; return [run];
  });
  const plan = vi.spyOn(productApi, 'plan').mockReturnValueOnce(pending.promise).mockImplementation(async projectId => savedComparisonPlan({ projectId, revision: 8 }));
  const initial = props();
  const mounted = render(<ProjectRuns {...initial} />);
  await userEvent.click(await screen.findByText('Compare with current plan'));
  await waitFor(() => expect(plan).toHaveBeenCalledTimes(1));
  if (change === 'account') scope.actor = 'another-owner';
  if (change === 'workspace') scope.connection = 'another-workspace';
  const projectId = change === 'project' ? 'p-2' : 'p-1';
  mounted.rerender(<ProjectRuns {...initial} projectId={projectId} />);
  await screen.findByText('Compare with current plan');
  expect(screen.queryByRole('region', { name: 'Selected task title comparison' })).toBeNull();
  await act(async () => pending.resolve(savedComparisonPlan({ revision: 99 })));
  await userEvent.click(screen.getByText('Compare with current plan'));
  await screen.findByText('This build: plan 5. Current saved plan: 8.');
  expect(screen.queryByText(/Current saved plan: 99/)).toBeNull();
  expect(plan).toHaveBeenLastCalledWith(projectId);
});

it('clears and retires comparisons after confirmed loss of project reading access', async () => {
  const pending = pendingComparisonPlan();
  const plan = vi.spyOn(productApi, 'plan').mockReturnValueOnce(pending.promise).mockResolvedValueOnce(savedComparisonPlan({ revision: 8 }));
  const initial = props(), mounted = render(<ProjectRuns {...initial} />);
  await userEvent.click(await screen.findByText('Compare with current plan'));
  await waitFor(() => expect(plan).toHaveBeenCalledTimes(1));
  teamAccess.permissions.read = false;
  mounted.rerender(<ProjectRuns {...initial} />);
  expect(screen.queryByRole('region', { name: 'Selected task title comparison' })).toBeNull();
  expect(screen.queryByText('Compare with current plan')).toBeNull();
  teamAccess.permissions.read = true; teamAccess.loading = false; teamAccess.error = '';
  mounted.rerender(<ProjectRuns {...initial} />);
  await screen.findByText('Compare with current plan');
  await act(async () => pending.resolve(savedComparisonPlan({ revision: 99 })));
  expect(screen.queryByRole('region', { name: 'Selected task title comparison' })).toBeNull();
  await userEvent.click(screen.getByText('Compare with current plan'));
  await screen.findByText('This build: plan 5. Current saved plan: 8.');
  expect(screen.queryByText(/Current saved plan: 99/)).toBeNull();
});

it.each(['foreign', 'duplicate-task'] as const)('refuses an unverifiable %s saved plan without displaying its contents', async kind => {
  const plan = savedComparisonPlan({ projectId: kind === 'foreign' ? 'foreign-project' : 'p-1', tasks: [{ id: 'task-1', title: 'Unverified plan contents', status: 'todo' }] });
  if (kind === 'duplicate-task') plan.tasks.push({ id: 'task-1', title: 'Ambiguous second title', status: 'todo' });
  vi.spyOn(productApi, 'plan').mockResolvedValue(plan);
  render(<ProjectRuns {...props()} />);
  await userEvent.click(await screen.findByText('Compare with current plan'));
  expect(await screen.findByRole('alert')).toHaveProperty('textContent', expect.stringContaining('The saved plan could not be verified for this project'));
  expect(screen.queryByText('Unverified plan contents')).toBeNull();
  expect(screen.queryByText(/Current saved plan:/)).toBeNull();
});

it('keeps the latest saved-plan read when StrictMode retires an earlier effect', async () => {
  const pending = pendingComparisonPlan();
  const plan = vi.spyOn(productApi, 'plan').mockReturnValueOnce(pending.promise).mockResolvedValueOnce(savedComparisonPlan({ revision: 8 }));
  render(<StrictMode><ProjectRuns {...props()} /></StrictMode>);
  await userEvent.click(await screen.findByText('Compare with current plan'));
  await screen.findByText('This build: plan 5. Current saved plan: 8.');
  expect(plan).toHaveBeenCalledTimes(2);
  await act(async () => pending.resolve(savedComparisonPlan({ revision: 99 })));
  expect(screen.queryByText(/Current saved plan: 99/)).toBeNull();
  expect(screen.getByText('This build: plan 5. Current saved plan: 8.')).not.toBeNull();
});


it.each(['loading', 'error'] as const)('retains an open saved-plan comparison through a %s permission refresh with reading access', async refreshState => {
  const pending = pendingComparisonPlan();
  const plan = vi.spyOn(productApi, 'plan').mockResolvedValueOnce(savedComparisonPlan()).mockReturnValueOnce(pending.promise);
  const initial = props(), mounted = render(<ProjectRuns {...initial} />);
  await userEvent.click(await screen.findByText('Compare with current plan'));
  await screen.findByText('This build: plan 5. Current saved plan: 6.');
  const comparison = screen.getByRole('region', { name: 'Selected task title comparison' });
  const disclosure = comparison.closest('details')!;
  const refresh = within(comparison).getByRole('button', { name: 'Refresh comparison' });
  refresh.focus();
  teamAccess.loading = refreshState === 'loading';
  teamAccess.error = refreshState === 'error' ? 'Permission service temporarily unavailable' : '';
  mounted.rerender(<ProjectRuns {...initial} />);
  expect(screen.getByRole('region', { name: 'Selected task title comparison' })).toBe(comparison);
  expect(disclosure.open).toBe(true);
  expect(document.activeElement).toBe(refresh);
  expect(within(comparison).getByText('This build: plan 5. Current saved plan: 6.')).not.toBeNull();
  expect(plan).toHaveBeenCalledTimes(1);
  // Retained reading access permits an explicit read. The server still owns
  // authorization; the permission refresh neither closes nor replays it.
  await userEvent.click(refresh);
  await within(comparison).findByText('Reading current saved plan…');
  await act(async () => pending.resolve(savedComparisonPlan({ revision: 8 })));
  await within(comparison).findByText('This build: plan 5. Current saved plan: 8.');
  teamAccess.loading = false; teamAccess.error = '';
  mounted.rerender(<ProjectRuns {...initial} />);
  expect(screen.getByRole('region', { name: 'Selected task title comparison' })).toBe(comparison);
  expect(disclosure.open).toBe(true);
  expect(plan).toHaveBeenCalledTimes(2);
  // A later confirmed denial is different from a transient refresh. Clear
  // the visible snapshot and require a new deliberate read after regrant.
  teamAccess.permissions.read = false;
  mounted.rerender(<ProjectRuns {...initial} />);
  expect(screen.queryByRole('region', { name: 'Selected task title comparison' })).toBeNull();
  expect(comparison.isConnected).toBe(false);
  teamAccess.permissions.read = true;
  mounted.rerender(<ProjectRuns {...initial} />);
  await screen.findByText('Compare with current plan');
  expect(screen.queryByRole('region', { name: 'Selected task title comparison' })).toBeNull();
  expect(screen.queryByText('This build: plan 5. Current saved plan: 8.')).toBeNull();
  expect(plan).toHaveBeenCalledTimes(2);
});
