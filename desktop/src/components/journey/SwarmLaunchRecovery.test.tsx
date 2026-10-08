import { useState } from 'react';
import { act, render, screen, waitFor, within } from '@testing-library/react';
import userEvent from '@testing-library/user-event';
import { beforeEach, expect, it, vi } from 'vitest';
import { api } from '../../api';
import { projectRunsApi, type ProjectRun, type RunOperation } from '../../api/projectRuns';
import { projectSwarmApi, type SwarmRecoveryAvailability } from '../../api/projectSwarm';
import { swarmHarnessFixture, swarmRunFixture } from '../../test/swarmFixtures';
import { resetCockpit, useCockpit } from '../../store/store';
import { ProjectSwarm } from './ProjectSwarm';

vi.mock('./SwarmRunQueue', () => ({SwarmRunQueue: () => null}));
const stopLabel = 'Stop uncertain launch and retain output';
function retained(): ProjectRun {
  const run = swarmRunFixture();
  const operation: RunOperation = {id: 'op-a', childId: 'child-a', taskId: 'a', worktreeId: 'isolated-a', attempt: 2, kind: 'session', status: 'unknown', allocation: 'ready', requestDigest: 'request', createdAt: run.createdAt, updatedAt: run.updatedAt};
  return {...run, operations: [operation], swarm: {...run.swarm!, tasks: run.swarm!.tasks.map(task => task.id === 'a' ? {...task, operationId: operation.id} : task)}};
}
const available = (run: ProjectRun): SwarmRecoveryAvailability => ({expectedRevision: run.revision, operationId: 'op-a', childId: 'child-a', availability: {supported: true, reasonCode: 'supported', message: 'The saved launch supports explicit recovery.'}});
const recovered = (run: ProjectRun, state: 'pending' | 'fenced' | 'finalized'): ProjectRun => ({...run, revision: run.revision + 1, operations: [{...run.operations[0], status: state === 'pending' ? 'unknown' : 'cancelled', recovery: {state, requestedAt: run.updatedAt, ...(state === 'pending' ? {} : {provedAt: run.updatedAt}), ...(state === 'finalized' ? {finalizedAt: run.updatedAt} : {})}}]});
function Harness({initial = retained(), canEdit = true, canExecute = true, updated = vi.fn()}: {initial?: ProjectRun; canEdit?: boolean; canExecute?: boolean; updated?: (run: ProjectRun) => void}) {
  const [run, setRun] = useState(initial), [busy, setBusy] = useState('');
  return <ProjectSwarm run={run} canEdit={canEdit} canExecute={canExecute} busy={!!busy} onBusy={setBusy} onUpdate={value => {setRun(value); updated(value);}} onOpenSession={vi.fn()} />;
}
beforeEach(() => {vi.restoreAllMocks(); resetCockpit(); useCockpit.getState().setHarnesses([swarmHarnessFixture()]);});

it('requires separate stopping consent for the exact attempt, retains output and starts no replacement', async () => {
  const run = retained();
  const read = vi.spyOn(projectSwarmApi, 'recovery').mockResolvedValue(available(run));
  const abandon = vi.spyOn(projectSwarmApi, 'abandon').mockResolvedValue(recovered(run, 'finalized'));
  const create = vi.spyOn(api, 'createSession');
  render(<Harness initial={run} canExecute={false} />);
  expect(read).not.toHaveBeenCalled();
  await userEvent.click(screen.getByRole('button', {name: stopLabel}));
  const dialog = screen.getByRole('dialog', {name: stopLabel});
  expect(within(dialog).getByText('p-1')).not.toBeNull();
  expect(within(dialog).getByText('Build a')).not.toBeNull();
  expect(within(dialog).getByText('op-a')).not.toBeNull();
  expect(within(dialog).getByText(/small permanent recovery marker/)).not.toBeNull();
  expect(within(dialog).getByText(/may stop its original worker/)).not.toBeNull();
  expect(within(dialog).getByText(/never starts a replacement/)).not.toBeNull();
  const submit = within(dialog).getByRole('button', {name: stopLabel});
  await within(dialog).findByRole('checkbox');
  expect(submit).toHaveProperty('disabled', true); expect(abandon).not.toHaveBeenCalled();
  await userEvent.click(within(dialog).getByRole('checkbox'));
  await userEvent.click(submit);
  await waitFor(() => expect(abandon).toHaveBeenCalledWith('p-1', 'run-1', 'a', run.revision, {operationId: 'op-a', childId: 'child-a', stopUncertainLaunch: true}));
  await waitFor(() => expect(screen.queryByRole('dialog')).toBeNull());
  expect(screen.getByText('Attempt retired · output retained')).not.toBeNull();
  expect(screen.getByRole('button', {name: 'Retry in a new workspace'})).toHaveProperty('disabled', true);
  expect(screen.getByRole('button', {name: 'Inspect workspace output'})).not.toBeNull();
  expect(create).not.toHaveBeenCalled();
});

it.each([
  ['legacy_launch', 'This older launch has no recovery protocol. Preserve its reservation for host inspection.'],
  ['unsupported_backend', 'This backend cannot install a recovery marker.'],
  ['fence_quota', 'The permanent recovery marker limit is reached. Existing proofs must be retained.'],
])('preserves the server’s %s refusal without enabling a destructive fallback', async (reasonCode, message) => {
  const run = retained();
  vi.spyOn(projectSwarmApi, 'recovery').mockResolvedValue({...available(run), availability: {supported: false, reasonCode, message}});
  const abandon = vi.spyOn(projectSwarmApi, 'abandon');
  render(<Harness initial={run} />);
  await userEvent.click(screen.getByRole('button', {name: stopLabel}));
  const dialog = screen.getByRole('dialog', {name: stopLabel});
  await within(dialog).findByText(message);
  expect(within(dialog).queryByRole('checkbox')).toBeNull();
  expect(within(dialog).getByRole('button', {name: stopLabel})).toHaveProperty('disabled', true);
  await userEvent.click(within(dialog).getByRole('button', {name: stopLabel}));
  expect(abandon).not.toHaveBeenCalled();
});

it.each(['pending', 'fenced'] as const)('keeps %s recovery capacity held and requires deliberate continuation', async state => {
  const run = recovered(retained(), state);
  vi.spyOn(projectSwarmApi, 'recovery').mockResolvedValue(available(run));
  const abandon = vi.spyOn(projectSwarmApi, 'abandon');
  render(<Harness initial={run} />);
  expect(screen.getByText('New starts enabled · 1 of 2 agent slots occupied')).not.toBeNull();
  expect(screen.queryByRole('button', {name: 'Retry in a new workspace'})).toBeNull();
  expect(screen.queryByRole('button', {name: 'Open task session'})).toBeNull();
  expect(screen.queryByRole('button', {name: 'Finish recovery'})).toBeNull();
  await userEvent.click(screen.getByRole('button', {name: 'Continue launch recovery'}));
  const dialog = screen.getByRole('dialog', {name: 'Continue launch recovery'});
  await within(dialog).findByRole('checkbox');
  expect(within(dialog).getByRole('button', {name: 'Continue reviewed recovery'})).toHaveProperty('disabled', true);
  expect(abandon).not.toHaveBeenCalled();
});

it('reads back a lost recovery response once without replay and shows its retained pending state', async () => {
  const run = retained(), pending = recovered(run, 'pending');
  vi.spyOn(projectSwarmApi, 'recovery').mockResolvedValue(available(run));
  const abandon = vi.spyOn(projectSwarmApi, 'abandon').mockRejectedValue(new Error('Response interrupted; recovery may have advanced.'));
  const get = vi.spyOn(projectRunsApi, 'get').mockResolvedValue(pending);
  const create = vi.spyOn(api, 'createSession');
  render(<Harness initial={run} />);
  await userEvent.click(screen.getByRole('button', {name: stopLabel}));
  const dialog = screen.getByRole('dialog', {name: stopLabel});
  await userEvent.click(await within(dialog).findByRole('checkbox'));
  await userEvent.click(within(dialog).getByRole('button', {name: stopLabel}));
  await screen.findByText('Launch recovery pending · capacity held');
  expect(get).toHaveBeenCalledTimes(1); expect(abandon).toHaveBeenCalledTimes(1);
  expect(screen.getByRole('button', {name: 'Continue launch recovery'})).toHaveProperty('disabled', false);
  expect(screen.queryByRole('button', {name: 'Retry in a new workspace'})).toBeNull();
  expect(create).not.toHaveBeenCalled();
});

it('clears support and consent on a failed capability refresh without automatically confirming', async () => {
  const run = retained();
  vi.spyOn(projectSwarmApi, 'recovery').mockResolvedValueOnce(available(run)).mockRejectedValueOnce(new Error('Current control permission was denied.'));
  const abandon = vi.spyOn(projectSwarmApi, 'abandon');
  render(<Harness initial={run} />);
  await userEvent.click(screen.getByRole('button', {name: stopLabel}));
  const dialog = screen.getByRole('dialog', {name: stopLabel});
  await userEvent.click(await within(dialog).findByRole('checkbox'));
  await userEvent.click(within(dialog).getByRole('button', {name: 'Refresh recovery availability'}));
  await within(dialog).findByRole('alert');
  expect(within(dialog).queryByRole('checkbox')).toBeNull();
  expect(within(dialog).getByRole('button', {name: stopLabel})).toHaveProperty('disabled', true);
  expect(abandon).not.toHaveBeenCalled();
});

it('cannot revive late availability or consent after edit authority is revoked and restored', async () => {
  const run = retained(); let resolve!: (value: SwarmRecoveryAvailability) => void;
  const read = vi.spyOn(projectSwarmApi, 'recovery').mockReturnValueOnce(new Promise(done => {resolve = done;})).mockResolvedValue(available(run));
  const abandon = vi.spyOn(projectSwarmApi, 'abandon');
  const view = render(<Harness initial={run} />);
  await userEvent.click(screen.getByRole('button', {name: stopLabel}));
  view.rerender(<Harness initial={run} canEdit={false} />);
  view.rerender(<Harness initial={run} canEdit />);
  await act(async () => resolve(available(run)));
  expect(screen.queryByRole('dialog')).toBeNull();
  await userEvent.click(screen.getByRole('button', {name: stopLabel}));
  const dialog = screen.getByRole('dialog', {name: stopLabel});
  expect(await within(dialog).findByRole('checkbox')).toHaveProperty('checked', false);
  expect(read).toHaveBeenCalledTimes(2); expect(abandon).not.toHaveBeenCalled();
});

it('ignores a delayed mutation result after the user leaves the project or loses control', async () => {
  const run = retained(); let resolve!: (value: ProjectRun) => void;
  vi.spyOn(projectSwarmApi, 'recovery').mockResolvedValue(available(run));
  vi.spyOn(projectSwarmApi, 'abandon').mockReturnValue(new Promise(done => {resolve = done;}));
  const updated = vi.fn();
  const view = render(<Harness initial={run} updated={updated} />);
  await userEvent.click(screen.getByRole('button', {name: stopLabel}));
  const dialog = screen.getByRole('dialog', {name: stopLabel});
  await userEvent.click(await within(dialog).findByRole('checkbox'));
  await userEvent.click(within(dialog).getByRole('button', {name: stopLabel}));
  view.rerender(<Harness initial={run} updated={updated} canEdit={false} />);
  view.rerender(<Harness initial={run} updated={updated} canEdit />);
  await act(async () => resolve(recovered(run, 'finalized')));
  expect(updated).not.toHaveBeenCalled(); expect(screen.queryByRole('dialog')).toBeNull();
});

it('drops the review when the run revision or project changes before confirmation', async () => {
  const run = retained(); vi.spyOn(projectSwarmApi, 'recovery').mockResolvedValue(available(run));
  const abandon = vi.spyOn(projectSwarmApi, 'abandon');
  const props = {canEdit: true, canExecute: true, busy: false, onBusy: vi.fn(), onUpdate: vi.fn(), onOpenSession: vi.fn()};
  const view = render(<ProjectSwarm {...props} run={run} />);
  await userEvent.click(screen.getByRole('button', {name: stopLabel}));
  await userEvent.click(await within(screen.getByRole('dialog')).findByRole('checkbox'));
  view.rerender(<ProjectSwarm {...props} run={{...run, revision: run.revision + 1}} />);
  expect(screen.queryByRole('dialog')).toBeNull();
  view.rerender(<ProjectSwarm {...props} run={{...run, projectId: 'another-project'}} />);
  expect(screen.queryByRole('dialog')).toBeNull(); expect(abandon).not.toHaveBeenCalled();
});

it.each(['integrating', 'unknown', 'integrated'] as const)('keeps a retained %s integration out of launch recovery', state => {
  const run = retained();
  run.swarm!.tasks[0].integration = {state, sourceCommit: 'a'.repeat(40), targetCommit: 'b'.repeat(40), reviewDigest: 'review'};
  const read = vi.spyOn(projectSwarmApi, 'recovery');
  render(<Harness initial={run} />);
  expect(screen.queryByRole('button', {name: stopLabel})).toBeNull();
  expect(screen.getByRole('button', {name: 'Inspect workspace output'})).not.toBeNull();
  expect(read).not.toHaveBeenCalled();
});
