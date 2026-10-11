import { act, fireEvent, render, screen, waitFor } from '@testing-library/react';
import userEvent from '@testing-library/user-event';
import { beforeEach, expect, it, vi } from 'vitest';
import type { ReactNode } from 'react';
import { CoordinatorExecution } from './CoordinatorExecution';
import { coordinatorContinuationsApi as api, type CoordinatorContinuation, type CoordinatorSavedRoute } from '../../api/coordinatorContinuations';
import { coordinatorPolicyApi, type CoordinatorPolicy } from '../../api/coordinatorPolicy';
import { ApiError } from '../../api/client';
import type { CreateSessionRequest } from '../../api/client';
import { ProductApiError } from '../../api/product';
import { resetCockpit, useCockpit } from '../../store/store';
import { aWorktree } from '../../test/fixtures';
import { context, proposalTurn, view } from './fixtures';

// Tests the continuation boundary, not the separately tested picker keyboard UI.
vi.mock('../topbar/SessionPicker', () => ({ SessionPicker: ({ open, implementationContext, onSubmit, onClose, children, progressLabel }: { open: boolean; implementationContext: CreateSessionRequest['implementationContext']; onSubmit(value: CreateSessionRequest): Promise<unknown>; onClose(): void; children: ReactNode; progressLabel: string }) => open ? <div role="dialog" aria-label="Existing upstream route picker">{children}<p>{progressLabel}</p><button onClick={() => void onSubmit({ worktreeId: 'w-1', harness: 'codex', model: 'model', permissionMode: 'bypass', agentConnectionId: 'configured', implementationContext })}>Submit reviewed route</button><button onClick={onClose}>Close picker</button></div> : null }));
const row = (patch: Partial<CoordinatorContinuation> = {}): CoordinatorContinuation => ({ id: 'continue-one', projectId: 'p-1', actor: { id: 'alice', displayName: 'Alice' }, idempotencyKey: 'request-one', originTurnId: 'turn-1', contextDigest: context().digest, policyRevision: 2, revision: 4, status: 'queued', message: 'Tasks queued', runId: 'run-one', queueIds: ['queue-one'], actions: [], createdAt: '2026-10-12T00:00:00Z', updatedAt: '2026-10-12T00:00:01Z', ...patch });
const policy = (patch: Partial<CoordinatorPolicy> = {}): CoordinatorPolicy => ({ projectId: 'p-1', revision: 2, mode: 'review', allowedTargets: ['specification'], executionMode: 'reviewed-tasks', deploymentMode: 'manual', actor: { id: 'alice', displayName: 'Alice' }, updatedAt: '2026-10-12T00:00:00Z', ...patch });
const currentView = () => view({ context: { ...context(), designPlan: { ...context().designPlan, status: 'approved' }, plan: { ...context().plan, specificationRevision: 2 } }, turns: [{ ...proposalTurn(), appliedRevision: 3 }] });
const props = () => ({ projectId: 'p-1', scope: 'local/alice/p-1', actorId: 'alice', view: currentView(), canRead: true, canEdit: true, canExecute: true, blocked: '', runOn: 'This computer · Docker', onInspect: vi.fn(), onNavigate: vi.fn() });
const records = () => Object.keys(localStorage).filter(key => key.startsWith('luci.coordinator-execution.v1:'));
function deferred<T>() { let resolve!: (value: T) => void, reject!: (error: unknown) => void; const promise = new Promise<T>((yes, no) => { resolve = yes; reject = no; }); return { promise, resolve, reject }; }
async function configure() { const button = await screen.findByRole('button', { name: 'Configure continued work' }); await waitFor(() => expect(button).toHaveProperty('disabled', false)); await userEvent.click(button); }
beforeEach(() => {
  vi.restoreAllMocks(); resetCockpit(); localStorage.clear();
  useCockpit.setState({ worktreesByBranch: { b: [aWorktree({ id: 'w-1', projectId: 'p-1', branchId: 'b', isPrimary: true })] }, selection: { projectId: 'p-1', branchId: 'b', worktreeId: 'w-1', sessionId: null } });
  vi.spyOn(coordinatorPolicyApi, 'get').mockResolvedValue(policy()); vi.spyOn(api, 'listState').mockResolvedValue({ continuations: [], savedRoute: null });
  vi.spyOn(api, 'create').mockImplementation(async (_project, _actor, input) => row({ idempotencyKey: input.idempotencyKey }));
  vi.spyOn(api, 'advance').mockResolvedValue(row());
  vi.spyOn(coordinatorPolicyApi, 'save').mockResolvedValue(policy({ revision: 3 }));
});
it('reads on mount without launch and uses current approved tasks in the reused explicit route picker', async () => {
  render(<CoordinatorExecution {...props()} />); await configure();
  expect(api.create).not.toHaveBeenCalled(); expect(screen.getByText(/Resources may be allocated/)).not.toBeNull();
  await userEvent.click(screen.getByRole('button', { name: 'Submit reviewed route' }));
  await screen.findByText(/continuation is retained/);
  const request = vi.mocked(api.create).mock.calls[0][2];
  expect(request).toMatchObject({ originTurnId: 'turn-1', expectedContextDigest: context().digest, expectedPolicyRevision: 2, maxParallel: 1, run: { harness: 'codex', model: 'model', permissionMode: 'bypass', agentConnectionId: 'configured', outcome: 'preview', implementationContext: { projectId: 'p-1', taskIds: ['T-1'], specificationRevision: 2, designPlanRevision: 1 } } });
  expect(api.create).toHaveBeenCalledTimes(1); expect(records()).toHaveLength(0);
  expect(screen.getByText(/queued task reservations/)).not.toBeNull(); expect(screen.getByRole('button', { name: 'Configure continued work' })).toHaveProperty('disabled', true);
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
  await userEvent.click(screen.getByRole('button', { name: 'Submit reviewed route' })); await screen.findByText(/Reply lost/);
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
  const input = props(), ui = render(<CoordinatorExecution {...input} />); await configure(); await userEvent.click(screen.getByRole('button', { name: 'Submit reviewed route' }));
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
});
it('does not treat a client-generated scope 409 as a settled refusal', async () => {
  vi.mocked(api.create).mockRejectedValue(new ApiError('conflict', 'Workspace changed', 409)); render(<CoordinatorExecution {...props()} />); await configure();
  await userEvent.click(screen.getByRole('button', { name: 'Submit reviewed route' })); await screen.findByText(/Workspace changed/); await userEvent.click(screen.getByRole('button', { name: 'Close picker' }));
  await userEvent.click(screen.getByRole('button', { name: 'Refresh continued work' })); await waitFor(() => expect(api.listState).toHaveBeenCalledTimes(2));
  expect(records()).toHaveLength(1); expect(api.create).toHaveBeenCalledTimes(1);
});
it('observes a definite refusal before allowing a freshly configured new request', async () => {
  vi.mocked(api.create).mockRejectedValue(new ProductApiError('conflict', 'Approval changed', 409, { error: 'stale_context' })); render(<CoordinatorExecution {...props()} />); await configure();
  await userEvent.click(screen.getByRole('button', { name: 'Submit reviewed route' })); await screen.findByText(/Approval changed/); expect(records()).toHaveLength(1);
  await userEvent.click(screen.getByRole('button', { name: 'Close picker' })); await userEvent.click(screen.getByRole('button', { name: 'Refresh continued work' }));
  await waitFor(() => expect(records()).toHaveLength(0)); expect(api.create).toHaveBeenCalledTimes(1);
});
it('closes route selection on changed context instead of substituting a newer approval snapshot', async () => {
  const input = props(), ui = render(<CoordinatorExecution {...input} />); await configure();
  ui.rerender(<CoordinatorExecution {...input} view={{ ...input.view, context: { ...input.view.context, digest: 'changed' } }} />);
  expect(screen.queryByRole('dialog')).toBeNull(); expect(api.create).not.toHaveBeenCalled();
});
it('refuses launch when durable browser request storage cannot be written', async () => {
  render(<CoordinatorExecution {...props()} />); await configure(); vi.spyOn(Storage.prototype, 'setItem').mockImplementation(() => { throw new Error('Disk unavailable'); });
  await userEvent.click(screen.getByRole('button', { name: 'Submit reviewed route' })); await screen.findByText('Disk unavailable'); expect(api.create).not.toHaveBeenCalled();
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
  await userEvent.click(screen.getByRole('button', { name: 'Submit reviewed route' }));
  await waitFor(() => expect(api.create).toHaveBeenCalledTimes(1));
  expect(vi.mocked(api.create).mock.calls[0][2].maxParallel).toBe(5);
});
it('suppresses a late create response after switching to another project and preserves the original request', async () => {
  const held = deferred<CoordinatorContinuation>(); vi.mocked(api.create).mockReturnValue(held.promise);
  const first = props(), ui = render(<CoordinatorExecution key={first.scope} {...first} />); await configure();
  await userEvent.click(screen.getByRole('button', { name: 'Submit reviewed route' }));
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
it('automatically continues one applied approved amendment using the exact delegated route, without reopening route controls', async () => {
  const saved = savedRoute(); vi.mocked(api.listState).mockResolvedValue({ continuations: [], savedRoute: saved });
  const input = withRequirements(), ui = render(<CoordinatorExecution {...input} />);
  await waitFor(() => expect(api.create).toHaveBeenCalledTimes(1));
  expect(vi.mocked(api.create).mock.calls[0][2]).toMatchObject({ expectedRouteRevision: 5, expectedPolicyRevision: 2, checkPlan: saved.checkPlan, maxParallel: 4, run: { ...saved.route, implementationContext: { projectId: 'p-1', specificationRevision: 2, taskIds: ['T-1'] } } });
  expect(screen.queryByRole('dialog')).toBeNull(); ui.rerender(<CoordinatorExecution {...input} />);
  expect(api.create).toHaveBeenCalledTimes(1);
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
it('retains automatic-start uncertainty across refresh and remount without retrying a POST', async () => {
  const saved = savedRoute(); vi.mocked(api.listState).mockResolvedValue({ continuations: [], savedRoute: saved }); vi.mocked(api.create).mockRejectedValue(new Error('Reply lost'));
  const input = withRequirements(), ui = render(<CoordinatorExecution {...input} />); await screen.findByText(/Reply lost/);
  await userEvent.click(screen.getByRole('button', { name: 'Refresh continued work' })); ui.unmount(); render(<CoordinatorExecution {...input} />);
  await screen.findByText(/New requests are paused/); expect(api.create).toHaveBeenCalledTimes(1);
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
  await userEvent.click(screen.getByRole('checkbox', { name: /Reuse this upstream route/ })); await userEvent.click(screen.getByRole('button', { name: 'Submit reviewed route' }));
  await screen.findByText(/continuation is retained/); expect(vi.mocked(api.create).mock.calls[0][2]).toMatchObject({ rememberRoute: true, checkPlan: savedRoute().checkPlan });
  await waitFor(() => expect(screen.queryByRole('button', { name: 'Remove npm' })).toBeNull()); expect(api.create).toHaveBeenCalledTimes(1);
});
it('requires current new amendment approvals, then continues with fresh task IDs under unchanged saved coverage', async () => {
  const input = withRequirements(); vi.mocked(api.listState).mockResolvedValue({ continuations: [row()], savedRoute: savedRoute() });
  const ui = render(<CoordinatorExecution {...input} />); await screen.findByRole('article'); expect(api.create).not.toHaveBeenCalled();
  const next = structuredClone(input.view); next.turns.push({ ...proposalTurn(), id: 'turn-2', appliedRevision: 4 }); next.context.digest = 'b'.repeat(64); next.context.plan.tasks[0].id = 'T-2'; next.context.specification.status = 'draft';
  ui.rerender(<CoordinatorExecution {...input} view={next} />); await screen.findByText(/Review and approve/); expect(api.create).not.toHaveBeenCalled();
  const approved = structuredClone(next); approved.context.specification.status = 'approved'; approved.context.digest = 'c'.repeat(64);
  vi.mocked(api.create).mockImplementation(async (_p, _a, request) => row({ id: 'continue-two', originTurnId: request.originTurnId, contextDigest: request.expectedContextDigest, idempotencyKey: request.idempotencyKey }));
  ui.rerender(<CoordinatorExecution {...input} view={approved} />); await waitFor(() => expect(api.create).toHaveBeenCalledTimes(1));
  expect(vi.mocked(api.create).mock.calls[0][2]).toMatchObject({ originTurnId: 'turn-2', expectedContextDigest: approved.context.digest, expectedRouteRevision: 5, run: { implementationContext: { taskIds: ['T-2'] } } });
});
it('does not silently drop a newer saved delivery delegation when reusing a route', async () => {
  const saved = { ...savedRoute(), previewPlan: { dockerfile: 'Dockerfile', port: 3000 } };
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
