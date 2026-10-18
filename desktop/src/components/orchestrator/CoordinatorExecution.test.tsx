import { act, fireEvent, render, screen, waitFor } from '@testing-library/react';
import userEvent from '@testing-library/user-event';
import { beforeEach, expect, it, vi } from 'vitest';
import type { ReactNode } from 'react';
import { CoordinatorExecution } from './CoordinatorExecution';
import { coordinatorContinuationsApi as api, type CoordinatorCloudCapacity, type CoordinatorContinuation, type CoordinatorSavedRoute } from '../../api/coordinatorContinuations';
import { coordinatorDeploymentApi, type CoordinatorDeploymentPlan } from '../../api/coordinatorDeployment';
import { coordinatorPolicyApi, type CoordinatorPolicy } from '../../api/coordinatorPolicy';
import { ApiError } from '../../api/client';
import type { CreateSessionRequest } from '../../api/client';
import { productApi, ProductApiError } from '../../api/product';
import { resetCockpit, useCockpit } from '../../store/store';
import { aWorktree } from '../../test/fixtures';
import { context, proposalTurn, view } from './fixtures';
import { orchestrationContractsApi, type OrchestrationContractView, type PrepareOrchestrationContract } from '../../api/orchestrationContracts';
import { contractViewFixture } from '../../test/orchestrationContractFixtures';
import { archiveContractReview, readContractReview } from '../develop/ExecutionContractReview';

// Tests the continuation boundary, not the separately tested picker keyboard UI.
vi.mock('../topbar/SessionPicker', () => ({ SessionPicker: ({ open, implementationContext, onSubmit, onClose, children, progressLabel }: { open: boolean; implementationContext: CreateSessionRequest['implementationContext']; onSubmit(value: CreateSessionRequest): Promise<unknown>; onClose(): void; children: ReactNode; progressLabel: string }) => open ? <div role="dialog" aria-label="Existing upstream route picker">{children}<p>{progressLabel}</p><button onClick={() => void onSubmit({ worktreeId: 'w-1', harness: 'codex', model: 'model', permissionMode: 'bypass', agentConnectionId: 'configured', implementationContext })}>Submit reviewed route</button><button onClick={onClose}>Close picker</button></div> : null }));
const row = (patch: Partial<CoordinatorContinuation> = {}): CoordinatorContinuation => ({ id: 'continue-one', projectId: 'p-1', actor: { id: 'alice', displayName: 'Alice' }, idempotencyKey: 'request-one', originTurnId: 'turn-1', contextDigest: context().digest, policyRevision: 2, revision: 4, status: 'queued', message: 'Tasks queued', runId: 'run-one', queueIds: ['queue-one'], actions: [], createdAt: '2026-10-12T00:00:00Z', updatedAt: '2026-10-12T00:00:01Z', ...patch });
const policy = (patch: Partial<CoordinatorPolicy> = {}): CoordinatorPolicy => ({ projectId: 'p-1', revision: 2, mode: 'review', reviewMode: 'manual', allowedTargets: ['specification'], executionMode: 'reviewed-tasks', deploymentMode: 'manual', actor: { id: 'alice', displayName: 'Alice' }, updatedAt: '2026-10-12T00:00:00Z', ...patch });
const currentView = () => view({ context: { ...context(), designPlan: { ...context().designPlan, status: 'approved' }, plan: { ...context().plan, specificationRevision: 2 } }, turns: [{ ...proposalTurn(), appliedRevision: 3 }] });
const props = () => ({ projectId: 'p-1', scope: 'local/alice/p-1', actorId: 'alice', view: currentView(), canRead: true, canEdit: true, canExecute: true, blocked: '', runOn: 'This computer · Docker', onInspect: vi.fn(), onNavigate: vi.fn() });
const records = () => Object.keys(localStorage).filter(key => key.startsWith('luci.coordinator-execution.v1:'));
const configuredCloud = (): CoordinatorCloudCapacity => ({ provider: 'e2b-embed', region: 'us-east-1', configuredGlobalSlots: 8, configuredProjectSlots: 6, configuredAvailableSlots: 2, estimatedTaskCostMicros: 150_000, estimatedMonthMicros: 250_000, monthlyBudgetMicros: 1_000_000, maxTaskSeconds: 600, observedAt: '2026-10-13T12:00:00Z', limitation: 'Configured limits only; actual admission is checked later.', retainedProjectVMs: 0, moreRetainedAttempts: 0, retainedAttempts: [] });
function deferred<T>() { let resolve!: (value: T) => void, reject!: (error: unknown) => void; const promise = new Promise<T>((yes, no) => { resolve = yes; reject = no; }); return { promise, resolve, reject }; }
async function configure() { const button = await screen.findByRole('button', { name: 'Configure continued work' }); await waitFor(() => expect(button).toHaveProperty('disabled', false)); await userEvent.click(button); }
async function submitReviewedRoute() {
  await userEvent.click(screen.getByRole('button', { name: 'Submit reviewed route' }));
  await userEvent.click(await screen.findByRole('checkbox', { name: /I reviewed these exact scopes/ }));
  await userEvent.click(screen.getByRole('button', { name: 'Approve contract and continue reviewed work' }));
}
beforeEach(() => {
  vi.restoreAllMocks(); resetCockpit(); localStorage.clear();
  useCockpit.setState({ worktreesByBranch: { b: [aWorktree({ id: 'w-1', projectId: 'p-1', branchId: 'b', isPrimary: true })] }, selection: { projectId: 'p-1', branchId: 'b', worktreeId: 'w-1', sessionId: null } });
  vi.spyOn(coordinatorPolicyApi, 'get').mockResolvedValue(policy()); vi.spyOn(api, 'listState').mockResolvedValue({ continuations: [], savedRoute: null });
  vi.spyOn(api, 'create').mockImplementation(async (_project, _actor, input) => row({ idempotencyKey: input.idempotencyKey }));
  vi.spyOn(api, 'advance').mockResolvedValue(row());
  let contract: OrchestrationContractView = contractViewFixture();
  vi.spyOn(orchestrationContractsApi, 'prepare').mockImplementation(async (_project: string, request: PrepareOrchestrationContract) => {
    contract = contractViewFixture(); contract.contract.actor.id = 'alice'; contract.contract.input.implementation.projectId = request.implementationContext.projectId;
    contract.contract.input.implementation.worktreeId = request.worktreeId; contract.contract.input.implementation.taskIds = [...request.implementationContext.taskIds];
    contract.contract.input.route.harness = request.route.harness; contract.contract.input.route.model = request.route.model;
    contract.contract.input.route.connection = {id: 'configured', revision: 1, kind: 'compatible-api', authMode: 'api-key', protocol: request.route.harness === 'codex' ? 'responses' : request.route.harness === 'claude-code' ? 'anthropic-messages' : 'chat-completions', model: request.route.model, endpoint: 'https://example.invalid/v1'};
    contract.contract.input.route.permissionMode = request.route.permissionMode; contract.contract.input.outcome = request.outcome;
    return contract;
  });
  vi.spyOn(orchestrationContractsApi, 'get').mockImplementation(async () => contract);
  vi.spyOn(orchestrationContractsApi, 'approve').mockImplementation(async () => {
    contract = { ...contract, contract: { ...contract.contract, revision: 2, state: 'approved', approval: { id: 'approval-1', actor: { id: 'alice', displayName: 'Alice' }, at: '2026-10-16T00:00:00Z', digest: contract.contract.digest } } };
    return contract;
  });
  vi.spyOn(coordinatorPolicyApi, 'save').mockResolvedValue(policy({ revision: 3 }));
});

it('route parallel policy starts an untouched coordinator draft at one without allocating work', async () => {
  const input = props(); input.view.context.plan.tasks.push({...input.view.context.plan.tasks[0], id: 'T-2', title: 'Second independent task'});
  render(<CoordinatorExecution {...input} />); await configure();
  expect(screen.getByLabelText('Maximum parallel tasks')).toHaveProperty('value', '1');
  expect(api.create).not.toHaveBeenCalled(); expect(orchestrationContractsApi.prepare).not.toHaveBeenCalled();
});

it.each(['managed', 'unknown'])('route parallel policy retains requested two for a prepared %s route and leaves review recovery usable', async mode => {
  const input = props(); input.view.context.plan.tasks.push({...input.view.context.plan.tasks[0], id: 'T-2', title: 'Second independent task'});
  vi.mocked(orchestrationContractsApi.prepare).mockImplementation(async (_project, request) => {
    const value = contractViewFixture(); value.contract.actor.id = 'alice';
    value.contract.input.route = {...value.contract.input.route, harness: 'codex', model: 'model', permissionMode: 'bypass', connection: {id: 'configured', revision: 1, kind: mode === 'managed' ? 'codex-subscription' : 'legacy', authMode: mode === 'managed' ? 'chatgpt' : undefined}};
    value.contract.input.implementation = {...value.contract.input.implementation, ...request.implementationContext, worktreeId: request.worktreeId};
    return value;
  });
  render(<CoordinatorExecution {...input} />); await configure();
  const limit = screen.getByLabelText('Maximum parallel tasks'); await userEvent.clear(limit); await userEvent.type(limit, '2');
  await userEvent.click(screen.getByRole('button', {name: 'Submit reviewed route'}));
  await screen.findByRole('alert');
  expect(screen.getByRole('button', {name: 'Approve contract and continue reviewed work'})).toHaveProperty('disabled', true);
  expect(screen.getByRole('button', {name: 'Refresh contract status'})).toHaveProperty('disabled', false);
  // Contract recovery is separately retained; inspect its exact continuation.
  const reviewKey = Object.keys(localStorage).find(key => key.startsWith('luci.execution-contract-review.') && !key.includes(':archive:'));
  expect(reviewKey).toBeDefined();
  expect(JSON.parse(localStorage.getItem(reviewKey!)!).continuation.maxParallel).toBe(2);
  await userEvent.click(screen.getByRole('button', {name: 'Keep original request and review updated plan'}));
  expect(api.create).not.toHaveBeenCalled(); expect(orchestrationContractsApi.approve).not.toHaveBeenCalled();
});

it.each(['context', 'policy', 'scope', 'project', 'read-grant'])('route parallel policy invalidates the final approved read across a %s change and return without persisting a continuation', async boundary => {
  const input = props(), held = deferred<OrchestrationContractView>();
  const read = vi.mocked(orchestrationContractsApi.get).getMockImplementation()!;
  let reads = 0, approved!: OrchestrationContractView;
  vi.mocked(orchestrationContractsApi.get).mockImplementation(async (project, id) => {
    const value = await read(project, id);
    if (++reads === 2) { approved = value; return held.promise; }
    return value;
  });
  const ui = render(<CoordinatorExecution {...input} policyEpoch={0} />); await configure();
  await submitReviewedRoute(); await waitFor(() => expect(reads).toBe(2));
  expect(approved.contract.state).toBe('approved');
  const writes = vi.spyOn(Storage.prototype, 'setItem');
  const changed = {...input,
    ...(boundary === 'context' ? {view: {...input.view, context: {...input.view.context, digest: 'b'.repeat(64)}}} : {}),
    ...(boundary === 'scope' ? {scope: input.scope + '/changed'} : {}),
    ...(boundary === 'project' ? {projectId: 'p-2', scope: 'local/alice/p-2', view: view({projectId: 'p-2', context: context('p-2')})} : {}),
    ...(boundary === 'read-grant' ? {canRead: false} : {}),
  };
  ui.rerender(<CoordinatorExecution {...changed} policyEpoch={boundary === 'policy' ? 1 : 0} />);
  ui.rerender(<CoordinatorExecution {...input} policyEpoch={0} />);
  await act(async () => held.resolve(approved));
  expect(api.create).not.toHaveBeenCalled();
  expect(records()).toEqual([]);
  expect(writes.mock.calls.filter(([key]) => String(key).startsWith('luci.coordinator-execution.v1:'))).toEqual([]);
});
it('saves explicit zero, reloads it and preserves zero through unrelated permission saves', async () => {
  vi.mocked(coordinatorPolicyApi.save).mockResolvedValueOnce(policy({revision: 3, automaticRepairLimit: 0}));
  const first = render(<CoordinatorExecution {...props()} />);
  await userEvent.click(await screen.findByText('Automatic repairs'));
  const selector = await screen.findByRole('combobox', {name: 'Automatic repairs per coding task'});
  await waitFor(() => expect(selector).toHaveProperty('value', '2'));
  await userEvent.selectOptions(selector, '0');
  await userEvent.click(screen.getByRole('button', {name: 'Save automatic repair limit'}));
  await screen.findByText(/automatic repair limit is saved/);
  expect(coordinatorPolicyApi.save).toHaveBeenLastCalledWith('p-1', {expectedRevision: 2, mode: 'review', allowedTargets: ['specification'], automaticRepairLimit: 0});
  expect(selector).toHaveProperty('value', '0');
  first.unmount();
  vi.mocked(coordinatorPolicyApi.get).mockResolvedValue(policy({revision: 3, automaticRepairLimit: 0}));
  vi.mocked(coordinatorPolicyApi.save).mockResolvedValueOnce(policy({revision: 4, executionMode: 'manual', automaticRepairLimit: 0}));
  render(<CoordinatorExecution {...props()} />);
  await userEvent.click(await screen.findByText('Automatic repairs'));
  await waitFor(() => expect(screen.getByRole('combobox', {name: 'Automatic repairs per coding task'})).toHaveProperty('value', '0'));
  await userEvent.click(screen.getByRole('button', {name: 'Pause new continuations'}));
  await screen.findByText(/New automatic task continuations are paused/);
  const calls = vi.mocked(coordinatorPolicyApi.save).mock.calls;
  expect(calls[calls.length - 1][1]).not.toHaveProperty('automaticRepairLimit');
  expect(screen.getByRole('combobox', {name: 'Automatic repairs per coding task'})).toHaveProperty('value', '0');
  expect(api.create).not.toHaveBeenCalled();
});
it.each([0, 1])('reads a lost zero-budget save without resend; current value%i cannot silently replace its request', async currentLimit => {
  vi.mocked(coordinatorPolicyApi.save).mockRejectedValueOnce(new Error('response interrupted'));
  const first = render(<CoordinatorExecution {...props()} />);
  await userEvent.click(await screen.findByText('Automatic repairs'));
  const selector = await screen.findByRole('combobox', {name: 'Automatic repairs per coding task'});
  await userEvent.selectOptions(selector, '0');
  await userEvent.click(screen.getByRole('button', {name: 'Save automatic repair limit'}));
  await screen.findByText(/response interrupted/);
  expect(JSON.parse(localStorage.getItem('luci.coordinator-execution.v1:local/alice/p-1')!).automaticRepairLimit).toBe(0);
  first.unmount();
  vi.mocked(coordinatorPolicyApi.get).mockResolvedValue(policy({revision: 3, automaticRepairLimit: currentLimit}));
  render(<CoordinatorExecution {...props()} />);
  await userEvent.click(await screen.findByText('Automatic repairs'));
  await screen.findByRole('combobox', {name: 'Automatic repairs per coding task'});
  await waitFor(() => expect(screen.getByRole('button', {name: 'Refresh continued work'})).toHaveProperty('disabled', false));
  await userEvent.click(screen.getByRole('button', {name: 'Refresh continued work'}));
  if (currentLimit === 0) await waitFor(() => expect(records()).toHaveLength(0));
  else { await screen.findByText(/current saved repair limit differs/); expect(records()).toHaveLength(1); }
  expect(coordinatorPolicyApi.save).toHaveBeenCalledTimes(1);
  expect(api.create).not.toHaveBeenCalled();
});
it('keeps repair policy read-only without execution authority', async () => {
  render(<CoordinatorExecution {...props()} canExecute={false} />);
  await userEvent.click(await screen.findByText('Automatic repairs'));
  expect(await screen.findByRole('combobox', {name: 'Automatic repairs per coding task'})).toHaveProperty('disabled', true);
  expect(screen.getByRole('button', {name: 'Save automatic repair limit'})).toHaveProperty('disabled', true);
  expect(coordinatorPolicyApi.save).not.toHaveBeenCalled();
});
it('keeps a retained zero request when current policy is unavailable', async () => {
  localStorage.setItem('luci.coordinator-execution.v1:local/alice/p-1', JSON.stringify({kind: 'policy', actorId: 'alice', projectId: 'p-1', revision: 2, automaticRepairLimit: 0}));
  vi.mocked(coordinatorPolicyApi.get).mockRejectedValue(new Error('current policy unavailable'));
  render(<CoordinatorExecution {...props()} />);
  await screen.findByText(/current policy unavailable/);
  expect(JSON.parse(localStorage.getItem('luci.coordinator-execution.v1:local/alice/p-1')!).automaticRepairLimit).toBe(0);
  expect(coordinatorPolicyApi.save).not.toHaveBeenCalled(); expect(api.create).not.toHaveBeenCalled();
});
it('reads on mount without launch and uses current approved tasks in the reused explicit route picker', async () => {
  render(<CoordinatorExecution {...props()} />); await configure();
  expect(api.create).not.toHaveBeenCalled(); expect(screen.getByText(/No coding resource is allocated/)).not.toBeNull();
  await userEvent.click(screen.getByRole('button', { name: 'Submit reviewed route' }));
  await screen.findByRole('region', { name: 'Execution contract review' });
  expect(api.create).not.toHaveBeenCalled();
  await userEvent.click(screen.getByRole('checkbox', { name: /I reviewed these exact scopes/ }));
  await userEvent.click(screen.getByRole('button', { name: 'Approve contract and continue reviewed work' }));
  await screen.findByText(/continuation is retained/);
  const request = vi.mocked(api.create).mock.calls[0][2];
  expect(request).toMatchObject({ originTurnId: 'turn-1', expectedContextDigest: context().digest, expectedPolicyRevision: 2, maxParallel: 1, run: { harness: 'codex', model: 'model', permissionMode: 'bypass', agentConnectionId: 'configured', outcome: 'preview', implementationContext: { projectId: 'p-1', taskIds: ['T-1'], specificationRevision: 2, designPlanRevision: 1 } } });
  expect(api.create).toHaveBeenCalledTimes(1); expect(records()).toHaveLength(0);
  expect(screen.getByText(/queued task reservations/)).not.toBeNull(); expect(screen.getByRole('button', { name: 'Configure continued work' })).toHaveProperty('disabled', true);
});
it('retains a lost contract preparation and never creates work until the same review is approved', async () => {
  const pendingPreparation = deferred<OrchestrationContractView>();
  vi.mocked(orchestrationContractsApi.prepare).mockReturnValueOnce(pendingPreparation.promise);
  const first = render(<CoordinatorExecution {...props()} />); await configure();
  await userEvent.click(screen.getByRole('button', { name: 'Submit reviewed route' }));
  await screen.findByRole('region', { name: 'Execution contract review' });
  const retained = localStorage.getItem('luci.execution-contract-review.v1:' + encodeURIComponent('local/alice/p-1/continued-work'));
  expect(retained).not.toBeNull(); expect(api.create).not.toHaveBeenCalled();
  first.unmount(); render(<CoordinatorExecution {...props()} />);
  expect(screen.getByRole('button', { name: 'Configure continued work' })).toHaveProperty('disabled', true);
  expect(api.create).not.toHaveBeenCalled();
  await userEvent.click(screen.getByRole('button', { name: 'Refresh contract status' }));
  await screen.findByRole('checkbox', { name: /I reviewed these exact scopes/ });
  expect(vi.mocked(orchestrationContractsApi.prepare).mock.calls[1][1].idempotencyKey).toBe(JSON.parse(retained!).request.idempotencyKey);
  expect(api.create).not.toHaveBeenCalled();
});
it('records explicit per-task cloud microVM placement without changing local defaults or delivery authority', async () => {
  vi.mocked(api.listState).mockResolvedValue({ continuations: [], savedRoute: null, cloudCapacity: configuredCloud() });
  render(<CoordinatorExecution {...props()} />); await configure();
  await userEvent.selectOptions(screen.getByRole('combobox', { name: 'Develop execution location' }), 'cloud-microvm');
  expect(screen.getByText(/one isolated microVM per task/)).not.toBeNull();
  await submitReviewedRoute();
  await screen.findByText(/continuation is retained/);
  expect(vi.mocked(api.create).mock.calls[0][2]).toMatchObject({ run: { executionLocation: 'cloud-microvm', harness: 'codex', permissionMode: 'bypass', agentConnectionId: 'configured', outcome: 'preview' } });
  expect(vi.mocked(api.create).mock.calls[0][2].deploymentPlan).toBeUndefined();
});
it('activates the existing cloud task continuation from the primary surface with one durable request', async () => {
  vi.mocked(api.listState).mockResolvedValue({ continuations: [], savedRoute: null, cloudCapacity: configuredCloud() });
  render(<CoordinatorExecution {...props()} />);
  const action = await screen.findByRole('button', { name: 'Review cloud swarm activation' });
  await waitFor(() => expect(action).toHaveProperty('disabled', false));
  expect(api.create).not.toHaveBeenCalled();
  await userEvent.click(action);
  expect(screen.getByRole('combobox', { name: 'Develop execution location' })).toHaveProperty('value', 'cloud-microvm');
  await submitReviewedRoute();
  await screen.findByText(/continuation is retained/);
  expect(api.create).toHaveBeenCalledTimes(1);
  expect(vi.mocked(api.create).mock.calls[0][2]).toMatchObject({ maxParallel: 1, run: { executionLocation: 'cloud-microvm', implementationContext: { taskIds: ['T-1'] } } });
  expect(records()).toHaveLength(0);
});
it('names missing cloud setup and keeps activation disabled without touching a saved message or making a request', async () => {
  vi.mocked(api.listState).mockResolvedValue({ continuations: [], savedRoute: null, cloudCapacity: { ...configuredCloud(), provider: '', setupNeeds: ['Cloud task execution is not connected. Open Bring your own cloud in Setup to prepare an Azure workspace.'] } });
  render(<CoordinatorExecution {...props()} />);
  expect(await screen.findByText(/Bring your own cloud in Setup/)).toBeTruthy();
  expect(screen.getByRole('button', { name: 'Review cloud swarm activation' })).toHaveProperty('disabled', true);
  expect(api.create).not.toHaveBeenCalled();
});
it('allows the configured Azure provider into review while retaining exact effect-time admission', async () => {
  vi.mocked(api.listState).mockResolvedValue({ continuations: [], savedRoute: null, cloudCapacity: { ...configuredCloud(), provider: 'azure-e2b-embed', region: 'configured-region' } });
  render(<CoordinatorExecution {...props()} />);
  const action = await screen.findByRole('button', { name: 'Review cloud swarm activation' });
  await waitFor(() => expect(action).toHaveProperty('disabled', false));
  expect(screen.getByText(/Azure microVM · configured-region/)).toBeTruthy();
  expect(api.create).not.toHaveBeenCalled();
});
it('keeps exact recovery controls reachable when a retained VM charge blocks fresh activation', async () => {
  vi.mocked(api.listState).mockResolvedValue({ continuations: [], savedRoute: null, cloudCapacity: { ...configuredCloud(), setupNeeds: ['Reconcile retained charges before activation.'], retainedProjectVMs: 1, retainedAttempts: [{ runId: 'run-held', taskId: 'task-held', attemptId: 'attempt-held', operationId: 'op-held', state: 'unknown', recoveryPending: true, resourcesHeld: true, estimatedCostMicros: 150_000, updatedAt: '2026-10-13T12:00:00Z' }] } });
  const input = props(); render(<CoordinatorExecution {...input} />);
  expect(await screen.findByText('Reconcile retained charges before activation.')).toBeTruthy();
  expect(screen.getByRole('button', { name: 'Review cloud swarm activation' })).toHaveProperty('disabled', true);
  await userEvent.click(screen.getByText('1 retained task microVM in this project'));
  expect(screen.getByText(/restored identity readback pending; no replacement launch/)).toBeTruthy();
  await userEvent.click(screen.getByRole('button', { name: 'Open exact task controls' }));
  expect(input.onInspect).toHaveBeenCalledWith('run-held');
  expect(api.create).not.toHaveBeenCalled();
});
it('keeps the exact cloud activation identity across a lost response and restart', async () => {
  vi.mocked(api.listState).mockResolvedValue({ continuations: [], savedRoute: null, cloudCapacity: configuredCloud() });
  vi.mocked(api.create).mockRejectedValue(new Error('Cloud reply lost'));
  const input = props(), first = render(<CoordinatorExecution {...input} />);
  const action = await screen.findByRole('button', { name: 'Review cloud swarm activation' });
  await waitFor(() => expect(action).toHaveProperty('disabled', false));
  await userEvent.click(action);
  await submitReviewedRoute();
  await screen.findByText(/Cloud reply lost/);
  const sent = vi.mocked(api.create).mock.calls[0][2];
  expect(sent.run.executionLocation).toBe('cloud-microvm'); expect(records()).toHaveLength(1);
  first.unmount(); render(<CoordinatorExecution {...input} />);
  await screen.findByText(/New requests are paused/);
  expect(screen.getByRole('button', { name: 'Review cloud swarm activation' })).toHaveProperty('disabled', true);
  vi.mocked(api.listState).mockResolvedValue({ continuations: [row({ idempotencyKey: sent.idempotencyKey })], savedRoute: null, cloudCapacity: configuredCloud() });
  await userEvent.click(screen.getByRole('button', { name: 'Refresh continued work' }));
  await screen.findByText(/exact retained request was observed/);
  expect(api.create).toHaveBeenCalledTimes(1); expect(records()).toHaveLength(0);
});
it('withdraws cloud activation when current execution authority is revoked', async () => {
  vi.mocked(api.listState).mockResolvedValue({ continuations: [], savedRoute: null, cloudCapacity: configuredCloud() });
  const input = props(), ui = render(<CoordinatorExecution {...input} />);
  const action = await screen.findByRole('button', { name: 'Review cloud swarm activation' });
  await waitFor(() => expect(action).toHaveProperty('disabled', false));
  ui.rerender(<CoordinatorExecution {...input} canExecute={false} />);
  expect(action).toHaveProperty('disabled', true); expect(api.create).not.toHaveBeenCalled();
});
it('shows configured per-task VM and current-month estimate without treating it as admission', async () => {
  vi.mocked(api.listState).mockResolvedValue({ continuations: [], savedRoute: null, cloudCapacity: {
    provider: 'e2b-embed', region: 'us-east-1', configuredGlobalSlots: 8, configuredProjectSlots: 6,
    configuredAvailableSlots: 2, estimatedTaskCostMicros: 150_000, estimatedMonthMicros: 250_000,
    monthlyBudgetMicros: 1_000_000, maxTaskSeconds: 600, retiredBillingUnconfirmed: 2, observedAt: '2026-10-13T12:00:00Z',
    limitation: 'Configured limits only; actual admission is checked later.', retainedProjectVMs: 1, moreRetainedAttempts: 0,
    retainedAttempts: [{ runId: 'run-one', taskId: 'task-one', attemptId: 'attempt-one', operationId: 'op-one', state: 'unknown', resourcesHeld: true, estimatedCostMicros: 150_000, updatedAt: '2026-10-13T12:00:00Z' }]
  } });
  const input = props(); render(<CoordinatorExecution {...input} />); await configure();
  await userEvent.selectOptions(screen.getByRole('combobox', { name: 'Develop execution location' }), 'cloud-microvm');
  expect(screen.getByText(/2 task VM slots presently available/)).not.toBeNull();
  expect(screen.getAllByText(/\$0\.25 of \$1\.00 USD/)).toHaveLength(2);
  expect(screen.getByText(/2 retired microVMs this month have confirmed Luci allocation release but no independent provider billing settlement/)).not.toBeNull();
  await userEvent.click(screen.getByText('1 retained task microVM in this project'));
  expect(screen.getByText(/task-one · run run-one · attempt attempt-one · unknown/)).not.toBeNull();
  await userEvent.click(screen.getByRole('button', { name: 'Open exact task controls' }));
  expect(input.onInspect).toHaveBeenCalledWith('run-one');
  expect(api.create).not.toHaveBeenCalled();
});
it.each(['unapproved', 'unsaved', 'question', 'foreign-turn', 'unapplied', 'no-execute', 'no-edit', 'plan-base', 'wrong-checkout'])('does not create work when %s', async reason => {
  const input = props();
  if (reason === 'unapproved') input.view.context.designPlan.status = 'draft'; if (reason === 'unsaved') input.blocked = 'Unsaved context';
  if (reason === 'question') input.view.blockingQuestions = [{ turnId: 'q', question: 'Who pays?', options: [], rationale: '' }];
  if (reason === 'foreign-turn') input.actorId = 'bob'; if (reason === 'unapplied') delete input.view.turns[0].appliedRevision;
  if (reason === 'no-execute') input.canExecute = false; if (reason === 'no-edit') input.canEdit = false;
  if (reason === 'plan-base') input.view.context.plan.specificationRevision = 1;
  if (reason === 'wrong-checkout') useCockpit.setState({ selection: { projectId: 'p-1', branchId: 'b', worktreeId: 'foreign', sessionId: null } });
  render(<CoordinatorExecution {...input} />); await waitFor(() => expect(api.listState).toHaveBeenCalledTimes(1));
  expect(screen.getByRole('button', { name: 'Configure continued work' })).toHaveProperty('disabled', true); expect(api.create).not.toHaveBeenCalled();
});
it('requires explicit execution consent and preserves existing draft policy rather than treating credentials as authority', async () => {
  vi.mocked(coordinatorPolicyApi.get).mockResolvedValue(policy({ executionMode: 'manual' }));
  render(<CoordinatorExecution {...props()} />); await userEvent.click(await screen.findByText('Implementation permission'));
  expect(screen.getByRole('button', { name: 'Enable reviewed-task continuation' })).toHaveProperty('disabled', true);
  await userEvent.click(screen.getByRole('checkbox', { name: 'Allow reviewed-task continuation in this project.' }));
  await userEvent.click(screen.getByRole('button', { name: 'Enable reviewed-task continuation' }));
  await waitFor(() => expect(coordinatorPolicyApi.save).toHaveBeenCalledExactlyOnceWith('p-1', { expectedRevision: 2, mode: 'review', allowedTargets: ['specification'], executionMode: 'reviewed-tasks' }));
  expect(api.create).not.toHaveBeenCalled();
});
it('retains a lost create through remount and requires exact key/actor/project readback without re-POST', async () => {
  vi.mocked(api.create).mockRejectedValue(new Error('Reply lost'));
  const input = props(), first = render(<CoordinatorExecution {...input} />); await configure();
  await submitReviewedRoute(); await screen.findByText(/Reply lost/);
  const sent = vi.mocked(api.create).mock.calls[0][2]; expect(records()).toHaveLength(1); first.unmount();
  render(<CoordinatorExecution {...input} />); await screen.findByText(/New requests are paused/);
  vi.mocked(api.listState).mockResolvedValue({ continuations: [row({ idempotencyKey: 'different-request' })], savedRoute: null });
  await userEvent.click(screen.getByRole('button', { name: 'Refresh continued work' })); await waitFor(() => expect(api.listState).toHaveBeenCalledTimes(3));
  expect(records()).toHaveLength(1);
  vi.mocked(api.listState).mockResolvedValue({ continuations: [row({ idempotencyKey: sent.idempotencyKey })], savedRoute: null });
  await userEvent.click(screen.getByRole('button', { name: 'Refresh continued work' })); await screen.findByText(/exact retained request was observed/);
  expect(records()).toHaveLength(0); expect(api.create).toHaveBeenCalledTimes(1);
});
it('preserves an uncertain create after permission churn even if its successful reply arrives later', async () => {
  const held = deferred<CoordinatorContinuation>(); vi.mocked(api.create).mockReturnValue(held.promise);
  const input = props(), ui = render(<CoordinatorExecution {...input} />); await configure(); await submitReviewedRoute();
  ui.rerender(<CoordinatorExecution {...input} canEdit={false} />); ui.rerender(<CoordinatorExecution {...input} />);
  await act(async () => held.resolve(row())); expect(records()).toHaveLength(1); expect(api.create).toHaveBeenCalledTimes(1);
  expect(screen.queryByText(/continuation is retained/)).toBeNull();
});
it('hides cached private work on actual read denial while retaining unknown request bytes', async () => {
  vi.mocked(api.listState).mockResolvedValue({ continuations: [row()], savedRoute: null }); render(<CoordinatorExecution {...props()} />);
  await screen.findByRole('article', { name: 'Selected continuation' });
  vi.mocked(api.listState).mockRejectedValue(new ApiError('conflict', 'Read revoked', 403));
  await userEvent.click(screen.getByRole('button', { name: 'Refresh continued work' })); await screen.findByText(/Current access was refused/);
  expect(screen.queryByRole('article')).toBeNull(); expect(screen.queryByRole('button', { name: 'Inspect this build' })).toBeNull();
  expect(screen.queryByRole('region', { name: 'Cloud task swarm activation' })).toBeNull();
});
it('does not treat a client-generated scope 409 as a settled refusal', async () => {
  vi.mocked(api.create).mockRejectedValue(new ApiError('conflict', 'Workspace changed', 409)); render(<CoordinatorExecution {...props()} />); await configure();
  await submitReviewedRoute(); await screen.findByText(/Workspace changed/);
  await userEvent.click(screen.getByRole('button', { name: 'Refresh continued work' })); await waitFor(() => expect(api.listState).toHaveBeenCalledTimes(2));
  expect(records()).toHaveLength(1); expect(api.create).toHaveBeenCalledTimes(1);
});
it('observes a definite refusal before allowing a freshly configured new request', async () => {
  vi.mocked(api.create).mockRejectedValue(new ProductApiError('conflict', 'Approval changed', 409, { error: 'stale_context' })); render(<CoordinatorExecution {...props()} />); await configure();
  await submitReviewedRoute(); await screen.findByText(/Approval changed/); expect(records()).toHaveLength(1);
  await userEvent.click(screen.getByRole('button', { name: 'Refresh continued work' }));
  await waitFor(() => expect(records()).toHaveLength(0)); expect(api.create).toHaveBeenCalledTimes(1);
});
it('closes route selection on changed context instead of substituting a newer approval snapshot', async () => {
  const input = props(), ui = render(<CoordinatorExecution {...input} />); await configure();
  ui.rerender(<CoordinatorExecution {...input} view={{ ...input.view, context: { ...input.view.context, digest: 'changed' } }} />);
  expect(screen.queryByRole('dialog')).toBeNull(); expect(api.create).not.toHaveBeenCalled();
});
it('refuses launch when durable browser request storage cannot be written', async () => {
  render(<CoordinatorExecution {...props()} />); await configure(); vi.spyOn(Storage.prototype, 'setItem').mockImplementation(() => { throw new Error('Disk unavailable'); });
  await userEvent.click(screen.getByRole('button', { name: 'Submit reviewed route' }));
  expect(api.create).not.toHaveBeenCalled(); expect(orchestrationContractsApi.prepare).not.toHaveBeenCalled();
});
it('explicitly advances only the retained exact revision and never announces queued as successful delivery', async () => {
  vi.mocked(api.listState).mockResolvedValue({ continuations: [row({ status: 'unknown', revision: 3, message: 'Check exact action settlement' })], savedRoute: null });
  const input = props(); render(<CoordinatorExecution {...input} />); await screen.findByRole('article');
  await userEvent.click(screen.getByRole('button', { name: 'Continue retained work' }));
  await waitFor(() => expect(api.advance).toHaveBeenCalledExactlyOnceWith('p-1', 'alice', 'continue-one', 3));
  expect(screen.getByText(/Completion, independent checks and release evidence remain separate/)).not.toBeNull();
  await userEvent.click(screen.getByRole('button', { name: 'Inspect this build' })); expect(input.onInspect).toHaveBeenCalledWith('run-one');
});

it('offers bounded concurrency beyond four when the reviewed task selection permits it', async () => {
  const input = props(); input.view.context.plan.tasks = Array.from({ length: 12 }, (_, index) => ({ id: `T-${index}`, title: `Task ${index}`, status: 'todo' }));
  render(<CoordinatorExecution {...input} />); await configure();
  const limit = screen.getByLabelText('Maximum parallel tasks');
  expect(limit).toHaveProperty('max', '12'); await userEvent.clear(limit); await userEvent.type(limit, '5');
  await submitReviewedRoute();
  await waitFor(() => expect(api.create).toHaveBeenCalledTimes(1));
  expect(vi.mocked(api.create).mock.calls[0][2].maxParallel).toBe(5);
});
it('suppresses a late create response after switching to another project and preserves the original request', async () => {
  const held = deferred<CoordinatorContinuation>(); vi.mocked(api.create).mockReturnValue(held.promise);
  const first = props(), ui = render(<CoordinatorExecution key={first.scope} {...first} />); await configure();
  await submitReviewedRoute();
  const second = { ...props(), projectId: 'p-2', scope: 'local/alice/p-2', view: view({ projectId: 'p-2', context: context('p-2') }) };
  vi.mocked(coordinatorPolicyApi.get).mockResolvedValue(policy({ projectId: 'p-2' }));
  ui.rerender(<CoordinatorExecution key={second.scope} {...second} />);
  await act(async () => held.resolve(row()));
  expect(records()).toEqual(['luci.coordinator-execution.v1:local/alice/p-1']);
  expect(screen.queryByRole('article')).toBeNull(); expect(api.create).toHaveBeenCalledTimes(1);
});
it('revalidates through the server after ordinary source changes instead of permanently freezing the captured continuation', async () => {
  vi.mocked(api.listState).mockResolvedValue({ continuations: [row()], savedRoute: null }); const input = props(); input.view.context.digest = 'after-code-change';
  render(<CoordinatorExecution {...input} />); await screen.findByText(/current context observation differs/);
  await userEvent.click(screen.getByRole('button', { name: 'Continue retained work' }));
  await waitFor(() => expect(api.advance).toHaveBeenCalledExactlyOnceWith('p-1', 'alice', 'continue-one', 4));
});
it('reports independent checks as separate from preview or successful deployment', async () => {
  vi.mocked(api.listState).mockResolvedValue({ continuations: [row({ status: 'checks-verified', delivery: { actionId: 'continue-one-checks', checkId: 'check-one', status: 'verified', commit: 'a'.repeat(40), sourceDigest: 'b'.repeat(64), integrationDigest: 'c'.repeat(64), message: 'The exact independent check passed', startedAt: '2026-10-12T00:00:00Z', updatedAt: '2026-10-12T00:00:01Z' } })], savedRoute: null });
  render(<CoordinatorExecution {...props()} />); await screen.findByRole('region', { name: 'Independent check evidence' });
  expect(screen.getByText(/Preview, release and deployment remain separate/)).not.toBeNull(); expect(api.advance).not.toHaveBeenCalled();
});
it('refreshes retained policy when the adjacent draft policy changes, clearing old consent', async () => {
  vi.mocked(coordinatorPolicyApi.get).mockResolvedValue(policy({ executionMode: 'manual' }));
  const input = props(), ui = render(<CoordinatorExecution {...input} policyEpoch={0} />);
  await userEvent.click(await screen.findByText('Implementation permission')); await userEvent.click(screen.getByRole('checkbox', { name: 'Allow reviewed-task continuation in this project.' }));
  vi.mocked(coordinatorPolicyApi.get).mockResolvedValue(policy({ revision: 3, executionMode: 'manual', mode: 'auto-drafts' }));
  ui.rerender(<CoordinatorExecution {...input} policyEpoch={1} />);
  await waitFor(() => expect(coordinatorPolicyApi.get).toHaveBeenCalledTimes(2));
  expect(screen.getByRole('checkbox', { name: 'Allow reviewed-task continuation in this project.' })).toHaveProperty('checked', false);
  expect(coordinatorPolicyApi.save).not.toHaveBeenCalled();
});
it('aborts explicit policy/history reads when the current project surface unmounts', async () => {
  const input = props(), ui = render(<CoordinatorExecution {...input} />); await configure(); await userEvent.click(screen.getByRole('button', { name: 'Close picker' }));
  const held = deferred<{ continuations: CoordinatorContinuation[]; savedRoute: null }>(); vi.mocked(api.listState).mockReturnValue(held.promise);
  await userEvent.click(screen.getByRole('button', { name: 'Refresh continued work' }));
  const signal = vi.mocked(api.listState).mock.calls[1][2]!;
  expect(signal.aborted).toBe(false); expect(vi.mocked(coordinatorPolicyApi.get).mock.calls[1][1]).toBe(signal);
  ui.unmount(); expect(signal.aborted).toBe(true);
  await act(async () => held.resolve({ continuations: [row()], savedRoute: null })); expect(api.create).not.toHaveBeenCalled();
});

const savedRoute = (): CoordinatorSavedRoute => ({ revision: 5, route: { worktreeId: 'w-1', harness: 'codex', model: 'model', permissionMode: 'bypass', agentConnectionId: 'configured', outcome: 'preview' }, maxParallel: 4, updatedAt: '2026-10-12T00:00:00Z', checkPlan: { commands: [{ label: 'npm', executable: 'npm', args: ['test'] }], requirementIds: ['REQ-1'], requirementSnapshots: [{ id: 'REQ-1', title: 'Book', acceptance: 'A visitor can book' }], acknowledgeProjectCommands: true } });
const withRequirements = () => { const input = props(); input.view.context.plan.tasks[0].requirementIds = ['REQ-1']; return input; };
it('leaves accepted-event admission to the durable service while showing the saved route', async () => {
  const saved = savedRoute(); vi.mocked(api.listState).mockResolvedValue({ continuations: [], savedRoute: saved });
  const input = withRequirements(), ui = render(<CoordinatorExecution {...input} />);
  await screen.findByText(/Saved upstream route/);
  expect(api.create).not.toHaveBeenCalled();
  expect(screen.queryByRole('dialog')).toBeNull(); ui.rerender(<CoordinatorExecution {...input} />);
  expect(api.create).not.toHaveBeenCalled();
});
it('shows reconciliation first, then reviews the exact accepted selection without creating a second run', async () => {
  const saved = savedRoute(), selection = { projectId: 'p-1', briefRevision: 1, planRevision: 1, specificationRevision: 2, designPlanRevision: 1, taskIds: ['T-1'] };
  vi.mocked(api.listState).mockResolvedValueOnce({ continuations: [row()], savedRoute: saved, acceptedReview: { eventId: 'coord-event-changed', status: 'waiting', reason: 'The accepted context commit has an unconfirmed outcome.' } })
    .mockResolvedValueOnce({ continuations: [row()], savedRoute: saved, acceptedReview: { eventId: 'coord-event-changed', status: 'review-required', reason: 'Review and approve its new execution contract.', selection } })
    .mockResolvedValue({ continuations: [row()], savedRoute: saved, acceptedReview: { eventId: 'coord-event-changed', status: 'approved', reason: 'Current approval observed.', selection } });
  render(<CoordinatorExecution {...withRequirements()} />);
  await screen.findByText('The accepted context commit has an unconfirmed outcome.');
  expect(screen.queryByRole('button', { name: 'Review changed execution contract' })).toBeNull();
  await userEvent.click(screen.getByRole('button', { name: 'Refresh continued work' }));
  await screen.findByRole('button', { name: 'Review changed execution contract' });
  await userEvent.click(screen.getByRole('button', { name: 'Review changed execution contract' }));
  await screen.findByRole('region', { name: 'Execution contract review' });
  const calls = vi.mocked(orchestrationContractsApi.prepare).mock.calls;
  const prepared = calls[calls.length - 1]?.[1];
  expect(prepared?.implementationContext).toEqual(selection);
  expect(prepared?.route).toMatchObject({ harness: 'codex', model: 'model', permissionMode: 'bypass', agentConnectionId: 'configured' });
  expect(api.create).not.toHaveBeenCalled();
  await userEvent.click(screen.getByRole('checkbox', { name: /I reviewed these exact scopes/ }));
  await userEvent.click(screen.getByRole('button', { name: 'Approve changed execution contract' }));
  await screen.findByText(/changed execution contract is approved/);
  expect(api.create).not.toHaveBeenCalled();
});
it('does not resurrect an archived accepted-event review when its prepare response arrives late', async () => {
  const scope = 'local/alice/p-1/continued-work';
  const selection = { projectId: 'p-1', briefRevision: 1, planRevision: 1, specificationRevision: 2, designPlanRevision: 1, taskIds: ['T-1'] };
  vi.mocked(api.listState).mockResolvedValue({ continuations: [row()], savedRoute: savedRoute(), acceptedReview: { eventId: 'coord-event-changed', status: 'review-required', reason: 'Review changed plan.', selection } });
  const held = deferred<OrchestrationContractView>();
  vi.mocked(orchestrationContractsApi.prepare).mockReturnValue(held.promise);
  render(<CoordinatorExecution {...withRequirements()} />);
  await userEvent.click(await screen.findByRole('button', { name: 'Review changed execution contract' }));
  await screen.findByRole('region', { name: 'Execution contract review' });
  expect(screen.getByRole('button', { name: 'Keep original request and review updated plan' })).toHaveProperty('disabled', true);
  const original = readContractReview(scope, 'p-1', 'alice')!;
  archiveContractReview(scope, original); // A second tab archived the original while preparation was in flight.
  const value = contractViewFixture(); value.contract.actor.id = 'alice';
  await act(async () => held.resolve(value));
  await waitFor(() => expect(screen.queryByRole('region', { name: 'Execution contract review' })).toBeNull());
  expect(readContractReview(scope, 'p-1', 'alice')).toBeNull();
  expect(localStorage.getItem('luci.execution-contract-review.v1:' + encodeURIComponent(scope) + ':archive:' + original.request.idempotencyKey)).not.toBeNull();
  expect(api.create).not.toHaveBeenCalled();
});
it.each(['policy epoch', 'read grant'])('keeps the original uncertain review after a %s change during prepare', async change => {
  const scope = 'local/alice/p-1/continued-work';
  const selection = { projectId: 'p-1', briefRevision: 1, planRevision: 1, specificationRevision: 2, designPlanRevision: 1, taskIds: ['T-1'] };
  vi.mocked(api.listState).mockResolvedValue({ continuations: [row()], savedRoute: savedRoute(), acceptedReview: { eventId: 'coord-event-changed', status: 'review-required', reason: 'Review changed plan.', selection } });
  const held = deferred<OrchestrationContractView>();
  vi.mocked(orchestrationContractsApi.prepare).mockReturnValue(held.promise);
  const input = withRequirements(), ui = render(<CoordinatorExecution {...input} />);
  await userEvent.click(await screen.findByRole('button', { name: 'Review changed execution contract' }));
  const retained = readContractReview(scope, 'p-1', 'alice')!;
  ui.rerender(<CoordinatorExecution {...input} {...(change === 'policy epoch' ? { policyEpoch: 1 } : { canRead: false })} />);
  const value = contractViewFixture(); value.contract.actor.id = 'alice';
  await act(async () => held.resolve(value));
  expect(readContractReview(scope, 'p-1', 'alice')).toEqual(retained);
  expect(screen.queryByText('Current context and policy')).toBeNull();
  expect(api.create).not.toHaveBeenCalled();
});
it.each(['acceptance', 'coverage', 'no-plan', 'manual-policy', 'unapplied', 'other-person', 'existing', 'draft', 'authority', 'checkout'])('does not automatically reuse a saved route when %s changed', async condition => {
  const input = withRequirements(), saved = savedRoute(); let rows: CoordinatorContinuation[] = [];
  if (condition === 'acceptance') input.view.context.specification.requirements[0].acceptance = 'Changed acceptance';
  if (condition === 'coverage') saved.checkPlan!.requirementSnapshots[0].id = 'OLD';
  if (condition === 'no-plan') delete saved.checkPlan;
  if (condition === 'manual-policy') vi.mocked(coordinatorPolicyApi.get).mockResolvedValue(policy({ executionMode: 'manual' }));
  if (condition === 'unapplied') delete input.view.turns[0].appliedRevision;
  if (condition === 'other-person') input.actorId = 'bob'; if (condition === 'existing') rows = [row()];
  if (condition === 'draft') input.blocked = 'Unsaved definition'; if (condition === 'authority') input.canExecute = false;
  if (condition === 'checkout') saved.route.worktreeId = 'foreign';
  vi.mocked(api.listState).mockResolvedValue({ continuations: rows, savedRoute: saved }); render(<CoordinatorExecution {...input} />);
  await screen.findByText(/Saved upstream route/); expect(api.create).not.toHaveBeenCalled();
});
it('does not run automatically before resolving a retained browser command draft', async () => {
  const input = withRequirements();
  const key = 'luci.project-draft.v1:' + JSON.stringify([JSON.stringify([input.scope, 'coordinator-checks']), 'verification-setup']);
  localStorage.setItem(key, JSON.stringify({ version: 1, base: 'old', value: { commands: [], executable: 'go', args: '[]', requirementIds: [], buildId: '' }, updatedAt: Date.now() }));
  vi.mocked(api.listState).mockResolvedValue({ continuations: [], savedRoute: savedRoute() }); render(<CoordinatorExecution {...input} />);
  await userEvent.click(screen.getByText(/Independent checks for continued work/)); await screen.findByRole('region', { name: 'Continued-work checks draft recovery' }); await screen.findByText(/Saved upstream route/); expect(api.create).not.toHaveBeenCalled();
});

it('records one explicit route and exact command delegation, then clears only the authored command draft after confirmation', async () => {
  const input = withRequirements(); render(<CoordinatorExecution {...input} />); await waitFor(() => expect(screen.getByRole('button', { name: 'Configure continued work' })).toHaveProperty('disabled', false));
  await userEvent.click(screen.getByText('Independent checks for continued work')); await screen.findByLabelText('Executable'); await userEvent.type(screen.getByLabelText('Executable'), 'npm'); fireEvent.change(screen.getByLabelText('Arguments · JSON array'), { target: { value: '["test"]' } });
  await userEvent.click(screen.getByRole('button', { name: 'Add check command' }));
  expect(screen.getByRole('button', { name: 'Configure continued work' })).toHaveProperty('disabled', true);
  await userEvent.click(screen.getByRole('checkbox', { name: /REQ-1/ })); await userEvent.click(screen.getByRole('checkbox', { name: /I delegate these exact/ })); await configure();
  await userEvent.click(screen.getByRole('checkbox', { name: /Reuse this upstream route/ })); await submitReviewedRoute();
  await screen.findByText(/continuation is retained/); expect(vi.mocked(api.create).mock.calls[0][2]).toMatchObject({ rememberRoute: true, checkPlan: savedRoute().checkPlan });
  await waitFor(() => expect(screen.queryByRole('button', { name: 'Remove npm' })).toBeNull()); expect(api.create).toHaveBeenCalledTimes(1);
});
it('requires current new amendment approvals, then continues with fresh task IDs under unchanged saved coverage', async () => {
  const input = withRequirements(); vi.mocked(api.listState).mockResolvedValue({ continuations: [row()], savedRoute: savedRoute() });
  const ui = render(<CoordinatorExecution {...input} />); await screen.findByRole('article'); expect(api.create).not.toHaveBeenCalled();
  const next = structuredClone(input.view); next.turns.push({ ...proposalTurn(), id: 'turn-2', appliedRevision: 4 }); next.context.digest = 'b'.repeat(64); next.context.plan.tasks[0].id = 'T-2'; next.context.specification.status = 'draft';
  ui.rerender(<CoordinatorExecution {...input} view={next} />); await screen.findByText(/Review and approve/); expect(api.create).not.toHaveBeenCalled();
  const approved = structuredClone(next); approved.context.specification.status = 'approved'; approved.context.digest = 'c'.repeat(64);
  ui.rerender(<CoordinatorExecution {...input} view={approved} />);
  await screen.findByText(/Saved upstream route/);
  expect(api.create).not.toHaveBeenCalled(); // The durable server selects and reserves affected work.
});
it('does not silently drop a newer saved delivery delegation when reusing a route', async () => {
  const saved = { ...savedRoute(), futureDelivery: { unsupported: true } };
  vi.mocked(api.listState).mockResolvedValue({ continuations: [], savedRoute: saved }); render(<CoordinatorExecution {...withRequirements()} />);
  await screen.findByText(/newer delivery delegation/); expect(api.create).not.toHaveBeenCalled();
});

it('keeps automatic continuation paused when discarding a retained command draft cannot remove its bytes', async () => {
  const input = withRequirements(), requirements = savedRoute().checkPlan!.requirementSnapshots;
  const key = 'luci.project-draft.v1:' + JSON.stringify([JSON.stringify([input.scope, 'coordinator-checks']), 'verification-setup']);
  const base = JSON.stringify([input.view.context.specification.revision, input.view.context.designPlan.revision, input.view.context.plan.revision, requirements]);
  localStorage.setItem(key, JSON.stringify({ version: 1, base, value: { commands: [{ label: 'npm', executable: 'npm', args: ['test'] }], executable: '', args: '[]', requirementIds: [], buildId: '' }, updatedAt: Date.now() }));
  vi.mocked(api.listState).mockResolvedValue({ continuations: [], savedRoute: savedRoute() }); render(<CoordinatorExecution {...input} />);
  await screen.findByText(/Saved upstream route/); await userEvent.click(screen.getByText(/Independent checks for continued work/));
  await userEvent.click(await screen.findByRole('button', { name: 'Restore draft' })); expect(api.create).not.toHaveBeenCalled();
  const remove = Storage.prototype.removeItem; vi.spyOn(Storage.prototype, 'removeItem').mockImplementation(function (this: Storage, target: string) { if (target === key) throw new Error('Storage locked'); return remove.call(this, target); });
  await userEvent.click(screen.getByRole('button', { name: 'Discard continued-work check setup' }));
  await screen.findAllByText(/This draft could not be removed/); await act(async () => { await Promise.resolve(); });
  expect(localStorage.getItem(key)).not.toBeNull(); expect(api.create).not.toHaveBeenCalled();
  expect(screen.getByRole('button', { name: 'Configure continued work' })).toHaveProperty('disabled', true);
});

it('pauses new automatic continuations with exact policy CAS without cancelling retained work', async () => {
  vi.mocked(api.listState).mockResolvedValue({ continuations: [row()], savedRoute: null }); vi.mocked(coordinatorPolicyApi.save).mockResolvedValue(policy({ revision: 3, executionMode: 'manual' }));
  render(<CoordinatorExecution {...props()} />); await userEvent.click(await screen.findByRole('button', { name: 'Pause new continuations' }));
  await waitFor(() => expect(coordinatorPolicyApi.save).toHaveBeenCalledExactlyOnceWith('p-1', { expectedRevision: 2, mode: 'review', allowedTargets: ['specification'], executionMode: 'manual' }));
  expect(api.advance).not.toHaveBeenCalled(); expect(api.create).not.toHaveBeenCalled(); await screen.findByText(/they have not been cancelled/);
});
it('requires separate Deploy authority and explicit preview permission consent', async () => {
  const input = props(), ui = render(<CoordinatorExecution {...input} />); await screen.findByText('Checked-source preview permission');
  expect(screen.getByRole('button', { name: 'Enable checked-source previews' })).toHaveProperty('disabled', true);
  ui.rerender(<CoordinatorExecution {...input} canDeploy />); await waitFor(() => expect(screen.getByRole('checkbox', { name: /Allow checked-source local previews/ })).toHaveProperty('disabled', false));
  vi.mocked(coordinatorPolicyApi.save).mockResolvedValue(policy({ revision: 3, previewMode: 'checked-source' }));
  await userEvent.click(screen.getByRole('checkbox', { name: /Allow checked-source local previews/ })); await userEvent.click(screen.getByRole('button', { name: 'Enable checked-source previews' }));
  await waitFor(() => expect(coordinatorPolicyApi.save).toHaveBeenCalledExactlyOnceWith('p-1', { expectedRevision: 2, mode: 'review', allowedTargets: ['specification'], previewMode: 'checked-source' }));
  expect(api.create).not.toHaveBeenCalled();
});
it('saves a separate reviewed-source policy without reinterpreting a Dockerfile approval', async () => {
  const input = props(); render(<CoordinatorExecution {...input} canDeploy />);
  await waitFor(() => expect(screen.getByRole('button', { name: 'Enable static reviewed-source previews' })).toHaveProperty('disabled', true));
  vi.mocked(coordinatorPolicyApi.save).mockResolvedValue(policy({ revision: 3, previewMode: 'reviewed-source' }));
  await userEvent.click(screen.getByRole('checkbox', { name: /Allow checked-source local previews/ }));
  await userEvent.click(screen.getByRole('button', { name: 'Enable static reviewed-source previews' }));
  await waitFor(() => expect(coordinatorPolicyApi.save).toHaveBeenCalledExactlyOnceWith('p-1', { expectedRevision: 2, mode: 'review', allowedTargets: ['specification'], previewMode: 'reviewed-source' }));
  expect(api.create).not.toHaveBeenCalled();
});
it.each(['matched', 'changed', 'revoked'])('reuses an exact saved preview only with %s current configuration and access', async condition => {
  const saved = { ...savedRoute(), previewPlan: { dockerfile: 'Dockerfile', port: 3000, expectedConfigurationRevision: 3, acknowledgePreviewBuild: true as const } };
  vi.mocked(api.listState).mockResolvedValue({ continuations: [], savedRoute: saved }); vi.mocked(coordinatorPolicyApi.get).mockResolvedValue(policy({ previewMode: 'checked-source' }));
  const metadata = vi.spyOn(productApi, 'request').mockResolvedValue({ revision: condition === 'changed' ? 4 : 3, variables: [], updatedAt: '' });
  render(<CoordinatorExecution {...withRequirements()} canDeploy={condition !== 'revoked'} />); await screen.findByText(/Saved upstream route/);
  expect(api.create).not.toHaveBeenCalled();
  expect(metadata).toHaveBeenCalledTimes(condition === 'revoked' ? 0 : 1);
});

it('retains a preview-policy request after Deploy permission churn until authoritative readback', async () => {
  const held = deferred<CoordinatorPolicy>(); vi.mocked(coordinatorPolicyApi.save).mockReturnValue(held.promise);
  const input = props(), ui = render(<CoordinatorExecution {...input} canDeploy />);
  await waitFor(() => expect(screen.getByRole('checkbox', { name: /Allow checked-source local previews/ })).toHaveProperty('disabled', false));
  await userEvent.click(screen.getByRole('checkbox', { name: /Allow checked-source local previews/ })); await userEvent.click(screen.getByRole('button', { name: 'Enable checked-source previews' }));
  ui.rerender(<CoordinatorExecution {...input} canDeploy={false} />); ui.rerender(<CoordinatorExecution {...input} canDeploy />);
  await act(async () => held.resolve(policy({ revision: 3, previewMode: 'checked-source' })));
  expect(records()).toHaveLength(1); expect(screen.queryByText(/preview permission is saved/)).toBeNull(); expect(coordinatorPolicyApi.save).toHaveBeenCalledTimes(1);
  vi.mocked(coordinatorPolicyApi.get).mockResolvedValue(policy({ revision: 3, previewMode: 'checked-source' }));
  await userEvent.click(screen.getByRole('button', { name: 'Refresh continued work' })); await screen.findByText(/Current authoritative state was read/);
  expect(records()).toHaveLength(0); expect(coordinatorPolicyApi.save).toHaveBeenCalledTimes(1); expect(api.create).not.toHaveBeenCalled();
});
it('records the exact delegated preview with checks and does not retain preview consent after confirmation', async () => {
  const input = withRequirements(); vi.mocked(coordinatorPolicyApi.get).mockResolvedValue(policy({ previewMode: 'checked-source' }));
  vi.spyOn(productApi, 'request').mockResolvedValue({ revision: 3, variables: [], updatedAt: '' });
  render(<CoordinatorExecution {...input} canDeploy />); await waitFor(() => expect(screen.getByRole('button', { name: 'Configure continued work' })).toHaveProperty('disabled', false));
  await userEvent.click(screen.getByText('Independent checks for continued work')); await screen.findByLabelText('Executable'); await userEvent.type(screen.getByLabelText('Executable'), 'npm'); fireEvent.change(screen.getByLabelText('Arguments · JSON array'), { target: { value: '["test"]' } });
  await userEvent.click(screen.getByRole('button', { name: 'Add check command' })); await userEvent.click(screen.getByRole('checkbox', { name: /REQ-1/ })); await userEvent.click(screen.getByRole('checkbox', { name: /I delegate these exact/ }));
  await userEvent.click(screen.getByText(/Preview after independent checks/)); await userEvent.click(await screen.findByRole('button', { name: 'Read preview configuration' })); await screen.findByText(/configuration revision 3/);
  await userEvent.click(screen.getByRole('checkbox', { name: /I delegate this exact Dockerfile/ })); await configure();
  await userEvent.click(screen.getByRole('checkbox', { name: /Reuse this upstream route/ })); await submitReviewedRoute();
  await screen.findByText(/continuation is retained/); expect(api.create).toHaveBeenCalledTimes(1);
  expect(vi.mocked(api.create).mock.calls[0][2]).toMatchObject({ rememberRoute: true, checkPlan: savedRoute().checkPlan, previewPlan: { dockerfile: 'Dockerfile', port: 3000, expectedConfigurationRevision: 3, acknowledgePreviewBuild: true } });
  await waitFor(() => expect(screen.getByRole('checkbox', { name: /I delegate this exact Dockerfile/ })).toHaveProperty('checked', false));
});
it('labels a ready preview as local and opens Deploy without claiming exact release inspection or public delivery', async () => {
  const input = props(); vi.mocked(api.listState).mockResolvedValue({ continuations: [row({ status: 'preview-ready', delivery: { actionId: 'continue-one-checks', checkId: 'check-one', status: 'verified', commit: 'a'.repeat(40), sourceDigest: 'b'.repeat(64), integrationDigest: 'c'.repeat(64), message: 'Checks verified', startedAt: '2026-10-12T00:00:00Z', updatedAt: '2026-10-12T00:00:01Z', preview: { actionId: 'continue-one-preview', releaseId: 'rel-one', status: 'ready', message: 'Observed local preview', startedAt: '2026-10-12T00:00:01Z', updatedAt: '2026-10-12T00:00:02Z' } } })], savedRoute: null });
  render(<CoordinatorExecution {...input} />); await screen.findByRole('region', { name: 'Checked-source preview' });
  expect(screen.getByText(/This is a local preview, not public deployment or semantic acceptance/)).not.toBeNull();
  await userEvent.click(screen.getByRole('button', { name: 'Open Deploy' })); expect(input.onNavigate).toHaveBeenCalledExactlyOnceWith('deploy'); expect(api.advance).not.toHaveBeenCalled(); expect(api.create).not.toHaveBeenCalled();
});

const deploymentPlan = (): CoordinatorDeploymentPlan => ({ targetId: 'target-one', targetFingerprint: 'd'.repeat(64), serverUuid: 'server-one', repository: 'https://github.com/test/app', branch: 'main', domain: 'https://app.example.com', port: 3000, healthPath: '/', dockerfile: 'Dockerfile', source: 'public', dataClientFramework: 'none', expectedConfigurationRevision: 3, expectedDataDomainRevision: 0, expectedDataBindingFingerprint: '', acknowledgeRepositoryPush: true, acknowledgePublic: true, acknowledgeDeploymentCosts: true });
const deploymentRoute = (): CoordinatorSavedRoute => ({ ...savedRoute(), route: { ...savedRoute().route, outcome: 'publish' }, previewPlan: { dockerfile: 'Dockerfile', port: 3000, expectedConfigurationRevision: 3, acknowledgePreviewBuild: true }, deploymentPlan: deploymentPlan() });
it('pauses a saved automatic model preference by exact route revision without starting a worker', async () => {
  const saved = { ...savedRoute(), autoDiscussFindings: true };
  vi.mocked(api.listState).mockResolvedValue({ continuations: [], savedRoute: saved });
  const update = vi.spyOn(api, 'setFindingPreference').mockResolvedValue({ ...saved, revision: saved.revision + 1, autoDiscussFindings: false });
  render(<CoordinatorExecution {...props()} />);
  await userEvent.click(await screen.findByRole('button', { name: 'Pause automatic finding discussions' }));
  expect(update).toHaveBeenCalledWith('p-1', saved.revision, false);
  expect(await screen.findByRole('button', { name: 'Enable automatic finding discussions' })).not.toBeNull();
  expect(api.create).not.toHaveBeenCalled();
});
it('requires separate existing-target permission and does not mutate other policy dimensions', async () => {
  const input = props(), ui = render(<CoordinatorExecution {...input} />);
  await screen.findByText('Existing-target deployment permission');
  expect(screen.getByRole('button', { name: 'Enable existing-target deployment' })).toHaveProperty('disabled', true);
  ui.rerender(<CoordinatorExecution {...input} canDeploy />);
  const consent = screen.getByRole('checkbox', { name: /Allow existing-target deployment/ }); await waitFor(() => expect(consent).toHaveProperty('disabled', false));
  vi.mocked(coordinatorPolicyApi.save).mockResolvedValue(policy({ revision: 3, deploymentMode: 'existing-target' }));
  await userEvent.click(consent); await userEvent.click(screen.getByRole('button', { name: 'Enable existing-target deployment' }));
  await waitFor(() => expect(coordinatorPolicyApi.save).toHaveBeenCalledExactlyOnceWith('p-1', { expectedRevision: 2, mode: 'review', allowedTargets: ['specification'], deploymentMode: 'existing-target' }));
  expect(api.create).not.toHaveBeenCalled();
});
it.each(['matched', 'target', 'config', 'data', 'repository', 'revoked', 'manual'])('reuses deployment only with %s exact current binding and policy', async condition => {
  const route = deploymentRoute(); vi.mocked(api.listState).mockResolvedValue({ continuations: [], savedRoute: route });
  vi.mocked(coordinatorPolicyApi.get).mockResolvedValue(policy({ previewMode: 'checked-source', deploymentMode: condition === 'manual' ? 'manual' : 'existing-target' }));
  vi.spyOn(productApi, 'request').mockResolvedValue({ revision: 3, variables: [], updatedAt: '' });
  const prepared = { ...deploymentPlan(), acknowledgeRepositoryPush: false, acknowledgePublic: false, acknowledgeDeploymentCosts: false };
  if (condition === 'target') prepared.targetFingerprint = 'e'.repeat(64);
  if (condition === 'config') prepared.expectedConfigurationRevision = 4;
  if (condition === 'data') { prepared.expectedDataDomainRevision = 2; prepared.expectedDataBindingFingerprint = 'e'.repeat(64); }
  if (condition === 'repository') prepared.repository = 'https://github.com/other/app';
  const prepare = vi.spyOn(coordinatorDeploymentApi, 'prepare').mockResolvedValue(prepared);
  render(<CoordinatorExecution {...withRequirements()} canDeploy={condition !== 'revoked'} />);
  await screen.findByText(/Saved upstream route/);
  if (condition === 'matched') {
    expect(prepare).toHaveBeenCalledTimes(1);
  }
  expect(api.create).not.toHaveBeenCalled();
  if (condition === 'revoked' || condition === 'manual') expect(prepare).not.toHaveBeenCalled();
});
it('retains deployment-policy uncertainty across permission churn and pauses without stopping existing releases', async () => {
  const held = deferred<CoordinatorPolicy>(); vi.mocked(coordinatorPolicyApi.save).mockReturnValue(held.promise);
  const input = props(), ui = render(<CoordinatorExecution {...input} canDeploy />);
  await waitFor(() => expect(screen.getByRole('checkbox', { name: /Allow existing-target deployment/ })).toHaveProperty('disabled', false));
  await userEvent.click(screen.getByRole('checkbox', { name: /Allow existing-target deployment/ })); await userEvent.click(screen.getByRole('button', { name: 'Enable existing-target deployment' }));
  ui.rerender(<CoordinatorExecution {...input} canDeploy={false} />); ui.rerender(<CoordinatorExecution {...input} canDeploy />);
  await act(async () => held.resolve(policy({ revision: 3, deploymentMode: 'existing-target' })));
  expect(records()).toHaveLength(1); expect(screen.queryByText(/deployment permission is saved/)).toBeNull();
  vi.mocked(coordinatorPolicyApi.get).mockResolvedValue(policy({ revision: 3, deploymentMode: 'existing-target' }));
  await userEvent.click(screen.getByRole('button', { name: 'Refresh continued work' })); await screen.findByText(/Current authoritative state was read/); expect(records()).toHaveLength(0);
  vi.mocked(coordinatorPolicyApi.save).mockResolvedValue(policy({ revision: 4, deploymentMode: 'manual' }));
  await userEvent.click(screen.getByRole('button', { name: 'Pause delegated deployments' })); await screen.findByText(/Existing releases and their costs have not been stopped/);
  expect(vi.mocked(coordinatorPolicyApi.save).mock.calls[1][1]).toEqual({ expectedRevision: 3, mode: 'review', allowedTargets: ['specification'], deploymentMode: 'manual' });
  expect(api.advance).not.toHaveBeenCalled(); expect(api.create).not.toHaveBeenCalled();
});

it('shows source publication separately from a recorded healthy deployment and preserves unconfirmed status', async () => {
  const delivery: NonNullable<CoordinatorContinuation['delivery']> = { actionId: 'continue-one-checks', checkId: 'check-one', status: 'verified', commit: 'a'.repeat(40), sourceDigest: 'b'.repeat(64), integrationDigest: 'c'.repeat(64), message: 'Independent checks observed', startedAt: '2026-10-12T00:00:00Z', updatedAt: '2026-10-12T00:00:01Z', gitPublication: { actionId: 'continue-one-git', status: 'verified', repository: 'https://github.com/test/app', branch: 'main', commit: 'a'.repeat(40), startedAt: '2026-10-12T00:00:00Z', updatedAt: '2026-10-12T00:00:01Z', message: 'Exact remote commit' }, deployment: { actionId: 'continue-one-deploy', releaseId: 'cloud-rel-one', status: 'ready', message: 'Provider identity and HTTPS observed', startedAt: '2026-10-12T00:00:00Z', updatedAt: '2026-10-12T00:00:01Z', evidence: { projectId: 'p-1', releaseId: 'cloud-rel-one', checkId: 'check-one', target: 'target-one', outcome: 'publish', commit: 'a'.repeat(40), sourceDigest: 'b'.repeat(64), observedAt: '2026-10-12T00:00:01Z', url: 'https://app.example.com', limitations: 'Behavior acceptance remains separate', joinedAt: '', approvals: { briefDigest: 'd'.repeat(64), specificationApprovalId: 'a', specificationRevision: 1, specificationDigest: 'e'.repeat(64), sketchDigest: 'f'.repeat(64), designApprovalId: 'b', designRevision: 1, designDigest: 'g'.repeat(64) } } } };
  vi.mocked(api.listState).mockResolvedValue({ continuations: [row({ status: 'deployed', delivery })], savedRoute: null });
  render(<CoordinatorExecution {...props()} />);
  expect((await screen.findByRole('link', { name: 'Open deployed application' })).getAttribute('href')).toBe('https://app.example.com');
  expect(screen.getByRole('region', { name: 'Source publication' }).textContent).toContain('does not establish application deployment');
  expect(screen.getByRole('region', { name: 'Application deployment' }).textContent).toContain('do not prove all product behavior');
  const uncertain = structuredClone(delivery); uncertain.deployment!.status = 'unknown'; delete uncertain.deployment!.evidence;
  vi.mocked(api.listState).mockResolvedValue({ continuations: [row({ status: 'unknown', delivery: uncertain })], savedRoute: null });
  await userEvent.click(screen.getByRole('button', { name: 'Refresh continued work' })); await screen.findByText('Outcome unconfirmed');
  expect(screen.queryByRole('link', { name: 'Open deployed application' })).toBeNull(); expect(api.advance).not.toHaveBeenCalled();
});
it('does not start from a deployment binding read that completes after Deploy authority is revoked', async () => {
  const route = deploymentRoute(), held = deferred<CoordinatorDeploymentPlan>();
  vi.mocked(api.listState).mockResolvedValue({ continuations: [], savedRoute: route });
  vi.mocked(coordinatorPolicyApi.get).mockResolvedValue(policy({ previewMode: 'checked-source', deploymentMode: 'existing-target' }));
  vi.spyOn(productApi, 'request').mockResolvedValue({ revision: 3, variables: [] });
  const prepare = vi.spyOn(coordinatorDeploymentApi, 'prepare').mockReturnValue(held.promise);
  const input = withRequirements(), ui = render(<CoordinatorExecution {...input} canDeploy />);
  await waitFor(() => expect(prepare).toHaveBeenCalledTimes(1)); const signal = prepare.mock.calls[0][2]!;
  ui.rerender(<CoordinatorExecution {...input} canDeploy={false} />);
  expect(signal.aborted).toBe(true);
  await act(async () => held.resolve({ ...deploymentPlan(), acknowledgeRepositoryPush: false, acknowledgePublic: false, acknowledgeDeploymentCosts: false }));
  await screen.findByText(/saved deployment requires current Deploy access/); expect(api.create).not.toHaveBeenCalled();
});
it('records one exact check-preview-deployment delegation through the existing route picker', async () => {
  const input = withRequirements(); vi.mocked(coordinatorPolicyApi.get).mockResolvedValue(policy({ previewMode: 'checked-source', deploymentMode: 'existing-target' }));
  vi.spyOn(productApi, 'request').mockResolvedValue({ revision: 3, variables: [], updatedAt: '' });
  vi.spyOn(coordinatorDeploymentApi, 'targets').mockResolvedValue([{ id: 'target-one', name: 'Connected server', serverUuid: 'server-one', baseUrl: 'https://control.example.com' }]);
  vi.spyOn(coordinatorDeploymentApi, 'prepare').mockResolvedValue({ ...deploymentPlan(), acknowledgeRepositoryPush: false, acknowledgePublic: false, acknowledgeDeploymentCosts: false });
  render(<CoordinatorExecution {...input} canDeploy />); await waitFor(() => expect(screen.getByRole('button', { name: 'Configure continued work' })).toHaveProperty('disabled', false));
  await userEvent.click(screen.getByText('Independent checks for continued work')); await screen.findByLabelText('Executable'); await userEvent.type(screen.getByLabelText('Executable'), 'npm'); fireEvent.change(screen.getByLabelText('Arguments · JSON array'), { target: { value: '["test"]' } });
  await userEvent.click(screen.getByRole('button', { name: 'Add check command' })); await userEvent.click(screen.getByRole('checkbox', { name: /REQ-1/ })); await userEvent.click(screen.getByRole('checkbox', { name: /I delegate these exact/ }));
  await userEvent.click(screen.getByText(/Preview after independent checks/)); await userEvent.click(await screen.findByRole('button', { name: 'Read preview configuration' })); await screen.findByText(/configuration revision 3/); await userEvent.click(screen.getByRole('checkbox', { name: /I delegate this exact Dockerfile/ }));
  await userEvent.click(screen.getByText(/Deploy after checks and local preview/)); await userEvent.click(await screen.findByRole('button', { name: 'Read connected deployment servers' }));
  await screen.findByRole('option', { name: 'Connected server' }); await userEvent.selectOptions(screen.getByLabelText('Deployment server'), 'target-one');
  await userEvent.selectOptions(screen.getByLabelText('Repository access'), 'public'); fireEvent.change(screen.getByLabelText('Repository URL · HTTPS'), { target: { value: 'https://github.com/test/app' } }); fireEvent.change(screen.getByLabelText('Application domain'), { target: { value: 'https://app.example.com' } });
  await userEvent.click(screen.getByRole('button', { name: 'Read current deployment plan' })); await screen.findByRole('region', { name: 'Review current deployment plan' });
  await userEvent.click(screen.getByRole('checkbox', { name: /authorize pushing/ })); await userEvent.click(screen.getByRole('checkbox', { name: /authorize public deployment/ }));
  expect(screen.getByRole('button', { name: 'Configure continued work' })).toHaveProperty('disabled', true);
  await userEvent.click(screen.getByRole('checkbox', { name: /accept ongoing deployment/ })); await configure();
  expect(screen.getByLabelText('Intended delivery outcome')).toHaveProperty('value', 'publish');
  await userEvent.click(screen.getByRole('checkbox', { name: /Reuse this upstream route/ })); await submitReviewedRoute(); await screen.findByText(/continuation is retained/);
  expect(api.create).toHaveBeenCalledTimes(1); expect(vi.mocked(api.create).mock.calls[0][2]).toMatchObject({ rememberRoute: true, deploymentPlan: deploymentPlan(), checkPlan: savedRoute().checkPlan, previewPlan: deploymentRoute().previewPlan, run: { outcome: 'publish' } });
  await waitFor(() => expect(screen.getByRole('checkbox', { name: /authorize pushing/ })).toHaveProperty('checked', false));
});
it('keeps readable continuation history and reconfiguration usable after saved deployment preparation is refused', async () => {
  const saved = deploymentRoute(); vi.mocked(api.listState).mockResolvedValue({ continuations: [row()], savedRoute: saved });
  vi.mocked(coordinatorPolicyApi.get).mockResolvedValue(policy({ previewMode: 'checked-source', deploymentMode: 'existing-target' }));
  vi.spyOn(productApi, 'request').mockResolvedValue({ revision: 3, variables: [] });
  vi.spyOn(coordinatorDeploymentApi, 'prepare').mockRejectedValue(new ProductApiError('conflict', 'Target was removed', 409, { error: 'stale_target' }));
  render(<CoordinatorExecution {...withRequirements()} canDeploy />);
  expect(await screen.findByRole('article', { name: 'Selected continuation' })).not.toBeNull();
  expect(screen.getByRole('button', { name: 'Pause delegated deployments' })).toHaveProperty('disabled', false);
  expect(screen.getByRole('button', { name: 'Inspect this build' })).toHaveProperty('disabled', false);
  expect(api.create).not.toHaveBeenCalled();
});
it('keeps readable history when Edit is revoked and does not perform Edit-only deployment preparation', async () => {
  vi.mocked(api.listState).mockResolvedValue({ continuations: [row()], savedRoute: deploymentRoute() });
  vi.mocked(coordinatorPolicyApi.get).mockResolvedValue(policy({ previewMode: 'checked-source', deploymentMode: 'existing-target' }));
  vi.spyOn(productApi, 'request').mockResolvedValue({ revision: 3, variables: [] });
  const prepare = vi.spyOn(coordinatorDeploymentApi, 'prepare').mockRejectedValue(new ApiError('conflict', 'Edit revoked', 403));
  render(<CoordinatorExecution {...withRequirements()} canDeploy canEdit={false} />);
  expect(await screen.findByRole('article', { name: 'Selected continuation' })).not.toBeNull();
  expect(prepare).not.toHaveBeenCalled(); expect(api.create).not.toHaveBeenCalled();
});
