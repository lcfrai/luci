import { useState } from 'react';
import { act, render, screen, waitFor, within } from '@testing-library/react';
import userEvent from '@testing-library/user-event';
import { afterEach, beforeEach, expect, it, vi } from 'vitest';
import { swarmQueueApi, type ProjectSwarmQueue } from '../../api/swarmQueue';
import { swarmHarnessFixture, swarmQueueFixture, swarmQueueItem, swarmRunFixture } from '../../test/swarmFixtures';
import { resetCockpit, useCockpit } from '../../store/store';
import { SwarmRunQueue } from './SwarmRunQueue';
const initial = swarmRunFixture();
async function openQueueControls() { await userEvent.click(screen.getByText(/^Queue status and controls/)); }
function Harness({run = initial, edit = true, execute = true, changed = vi.fn()} = {}) {
  const [busy, setBusy] = useState('');
  return <SwarmRunQueue readAccess={{canRead:true,loading:false,error:''}} run={run} canEdit={edit} canExecute={execute} busy={!!busy} onBusy={setBusy} onQueueChange={changed} />;
}
beforeEach(() => { resetCockpit(); useCockpit.getState().setHarnesses([swarmHarnessFixture()]); vi.spyOn(swarmQueueApi, 'get').mockResolvedValue(swarmQueueFixture()); });
afterEach(() => { vi.restoreAllMocks(); });

it('only reads on mount and requires explicit consent for independent and dependent tasks together', async () => {
  const tasks = initial.swarm!.tasks.map(task => swarmQueueItem({id: 'q-' + task.id, taskId: task.id, taskTitle: task.title}));
  const enqueue = vi.spyOn(swarmQueueApi, 'enqueueMany').mockResolvedValue({items: tasks, queue: swarmQueueFixture(tasks)});
  render(<Harness />);
  const button = await screen.findByRole('button', {name: 'Queue reviewed tasks (3)'});
  await waitFor(() => expect(button).toHaveProperty('disabled', false));
  expect(enqueue).not.toHaveBeenCalled();
  await userEvent.click(button);
  const dialog = screen.getByRole('dialog');
  expect(within(dialog).getByText(/Dependent tasks wait for reviewed integration/)).not.toBeNull();
  expect(within(dialog).getByText(/recorded-model/)).not.toBeNull();
  expect(within(dialog).getByText(/Saved permission: Bypass/)).not.toBeNull();
  expect(within(dialog).getByText(/queue completion does not accept or integrate changes/)).not.toBeNull();
  expect(within(dialog).getByText('Build c')).not.toBeNull();
  await userEvent.selectOptions(within(dialog).getByRole('combobox'), 'high');
  await userEvent.click(within(dialog).getByRole('button', {name: 'Authorize queued tasks'}));
  await screen.findByText(/3 tasks queued/);
  expect(enqueue).toHaveBeenCalledTimes(1);
  expect(enqueue.mock.calls[0]).toEqual(['p-1','run-1',['a','b','c'],1,expect.any(String),'high']);
  expect(screen.getByRole('button', {name: 'Queue reviewed tasks'})).toHaveProperty('disabled', true);
});

it.each(['ask', 'plan'] as const)('blocks queueing and resume for saved %s while retaining cancellation controls', async permissionMode => {
  const run = {...initial, input: {...initial.input, route: {...initial.input.route, permissionMode}}};
  const paused = swarmQueueItem({state: 'paused'});
  vi.mocked(swarmQueueApi.get).mockResolvedValue(swarmQueueFixture([paused]));
  const enqueue = vi.spyOn(swarmQueueApi, 'enqueueMany'), control = vi.spyOn(swarmQueueApi, 'control').mockResolvedValue(swarmQueueFixture([{...paused, state: 'cancelled', revision: 3}]));
  render(<Harness run={run} />);
  await openQueueControls();
  await screen.findByRole('button', {name: 'Resume queued task'});
  expect(screen.getByRole('button', {name: 'Resume queued task'})).toHaveProperty('disabled', true);
  expect(screen.getByRole('button', {name: 'Queue reviewed tasks (2)'})).toHaveProperty('disabled', true);
  expect(screen.getByRole('button', {name: 'Cancel queued task'})).toHaveProperty('disabled', false);
  await userEvent.click(screen.getByRole('button', {name: 'Cancel queued task'}));
  await waitFor(() => expect(control).toHaveBeenCalledWith('p-1', paused, 'cancel', undefined));
  expect(enqueue).not.toHaveBeenCalled(); expect(run.input.route.permissionMode).toBe(permissionMode);
});

it('discards queue consent when current task capability or saved route changes', async () => {
  const enqueue = vi.spyOn(swarmQueueApi, 'enqueueMany');
  const view = render(<Harness />);
  await waitFor(() => expect(screen.getByRole('button', {name: 'Queue reviewed tasks (3)'})).toHaveProperty('disabled', false));
  await userEvent.click(screen.getByRole('button', {name: 'Queue reviewed tasks (3)'}));
  act(() => useCockpit.getState().setHarnesses([{...swarmHarnessFixture(), autonomousTask: false}]));
  expect(screen.queryByRole('dialog')).toBeNull();
  expect(screen.getByRole('button', {name: 'Queue reviewed tasks (3)'})).toHaveProperty('disabled', true);
  act(() => useCockpit.getState().setHarnesses([swarmHarnessFixture()]));
  expect(screen.queryByRole('dialog')).toBeNull();
  await userEvent.click(screen.getByRole('button', {name: 'Queue reviewed tasks (3)'}));
  view.rerender(<Harness run={{...initial, input: {...initial.input, route: {...initial.input.route, permissionMode: 'ask'}}}} />);
  expect(screen.queryByRole('dialog')).toBeNull(); expect(enqueue).not.toHaveBeenCalled();
});

it('refreshes after a lost receipt without duplicating already accepted tasks', async () => {
  const enqueue = vi.spyOn(swarmQueueApi, 'enqueueMany').mockRejectedValue(new Error('Connection interrupted'));
  render(<Harness />);
  await waitFor(() => expect(screen.getByRole('button', {name: 'Queue reviewed tasks (3)'})).toHaveProperty('disabled', false));
  await userEvent.click(screen.getByRole('button', {name: 'Queue reviewed tasks (3)'}));
  await userEvent.click(screen.getByRole('button', {name: 'Authorize queued tasks'}));
  await screen.findByText(/queue result is unconfirmed/);
  expect(screen.getByRole('button', {name: 'Queue reviewed tasks (3)'})).toHaveProperty('disabled', true);
  vi.mocked(swarmQueueApi.get).mockResolvedValue(swarmQueueFixture(initial.swarm!.tasks.map(task => swarmQueueItem({id: task.id, taskId: task.id}))));
  await openQueueControls();
  await userEvent.click(screen.getByRole('button', {name: 'Refresh queue'}));
  await waitFor(() => expect(screen.getByRole('button', {name: 'Queue reviewed tasks'})).toHaveProperty('disabled', true));
  expect(enqueue).toHaveBeenCalledTimes(1);
});

it('closes consent when reviewed input or execution permission changes', async () => {
  const enqueue = vi.spyOn(swarmQueueApi, 'enqueueMany');
  const view = render(<Harness />);
  await waitFor(() => expect(screen.getByRole('button', {name: 'Queue reviewed tasks (3)'})).toHaveProperty('disabled', false));
  await userEvent.click(screen.getByRole('button', {name: 'Queue reviewed tasks (3)'}));
  view.rerender(<Harness run={{...initial, revision: 2}} />);
  expect(screen.queryByRole('dialog')).toBeNull();
  await userEvent.click(screen.getByRole('button', {name: 'Queue reviewed tasks (3)'}));
  view.rerender(<Harness run={{...initial, revision: 2}} execute={false} />);
  expect(screen.queryByRole('dialog')).toBeNull();
  expect(screen.getByRole('button', {name: 'Queue reviewed tasks (3)'})).toHaveProperty('disabled', true);
  expect(enqueue).not.toHaveBeenCalled();
});

it('keeps waiting-task control separate from active-session stops and hides other runs', async () => {
  const queued = swarmQueueItem();
  vi.mocked(swarmQueueApi.get).mockResolvedValue(swarmQueueFixture([queued, swarmQueueItem({id:'b',taskId:'b',taskTitle:'Build b',state:'running'}), swarmQueueItem({id:'c',taskId:'c',taskTitle:'Another run',runId:'other'})]));
  const control = vi.spyOn(swarmQueueApi, 'control').mockResolvedValue(swarmQueueFixture([{...queued,state:'paused',revision:3}]));
  render(<Harness />);
  await openQueueControls();
  await screen.findByText('Build a');
  expect(screen.queryByText('Another run')).toBeNull();
  expect(screen.getAllByRole('button',{name:'Cancel queued task'})).toHaveLength(1);
  await userEvent.click(screen.getByRole('button',{name:'Pause queued task'}));
  await screen.findByRole('button',{name:'Resume queued task'});
  expect(control).toHaveBeenCalledWith('p-1', queued, 'pause', undefined);
});

it('leaves a viewer read-only and aborts an unfinished read on unmount', async () => {
  let resolve!: (q: ProjectSwarmQueue) => void;
  vi.mocked(swarmQueueApi.get).mockReturnValue(new Promise(done => {resolve = done;}));
  const changed = vi.fn(), view = render(<Harness edit={false} changed={changed} />);
  expect(screen.getByRole('button', {name: 'Queue reviewed tasks (3)'})).toHaveProperty('disabled', true);
  const signal = vi.mocked(swarmQueueApi.get).mock.calls[0][1]!;
  view.unmount();
  expect(signal.aborted).toBe(true);
  await act(async () => resolve(swarmQueueFixture([swarmQueueItem()])));
  expect(changed).not.toHaveBeenCalled();
});

it('offers an explicitly retired attempt for fresh queueing and keeps uncertain starts blocked', async () => {
  const run = {...initial, operations: [
    {id:'old-a',kind:'session',childId:'old-child',attempt:1,status:'cancelled',allocation:'abandoned' as const,requestDigest:'digest',createdAt:initial.createdAt,updatedAt:initial.updatedAt},
    {id:'old-b',kind:'session',childId:'uncertain-child',attempt:1,status:'unknown',allocation:'unknown' as const,requestDigest:'digest',createdAt:initial.createdAt,updatedAt:initial.updatedAt},
  ], swarm: {...initial.swarm!,tasks: initial.swarm!.tasks.map(task => ({...task,...(task.id === 'a' ? {operationId:'old-a'} : task.id === 'b' ? {operationId:'old-b'} : {})}))}};
  vi.mocked(swarmQueueApi.get).mockResolvedValue(swarmQueueFixture([swarmQueueItem({state:'cancelled'})]));
  render(<Harness run={run} />);
  await waitFor(() => expect(screen.getByRole('button',{name:'Queue reviewed tasks (2)'})).toHaveProperty('disabled', false));
  await userEvent.click(screen.getByRole('button',{name:'Queue reviewed tasks (2)'}));
  expect(within(screen.getByRole('dialog')).queryByText('Build b')).toBeNull();
  expect(within(screen.getByRole('dialog')).getByText('Build a')).not.toBeNull();
});

it('waits for durable retirement finalization before offering the task for a new queue attempt', async () => {
  const operation = {id:'old-a',kind:'session',childId:'old-child',attempt:1,status:'cancelled',allocation:'ready' as const,requestDigest:'digest',createdAt:initial.createdAt,updatedAt:initial.updatedAt,
    absenceRetirement:{childId:'old-child',leaseId:'lease-a',backend:'docker',observedAt:initial.updatedAt,inventoryObservedAt:initial.updatedAt}};
  const run = {...initial, operations:[operation],swarm:{...initial.swarm!,tasks:initial.swarm!.tasks.map(task => task.id === 'a' ? {...task,operationId:operation.id} : task)}};
  vi.mocked(swarmQueueApi.get).mockResolvedValue(swarmQueueFixture([swarmQueueItem({state:'cancelled'})]));
  const view = render(<Harness run={run} />);
  await waitFor(() => expect(screen.getByRole('button',{name:'Queue reviewed tasks (2)'})).toHaveProperty('disabled', false));
  await userEvent.click(screen.getByRole('button',{name:'Queue reviewed tasks (2)'}));
  expect(within(screen.getByRole('dialog')).queryByText('Build a')).toBeNull();
  view.rerender(<Harness run={{...run,revision:run.revision+1,operations:[{...operation,absenceRetirement:{...operation.absenceRetirement,finalizedAt:initial.updatedAt}}]}} />);
  await waitFor(() => expect(screen.queryByRole('dialog')).toBeNull());
  expect(screen.getByRole('button',{name:'Queue reviewed tasks (3)'})).toHaveProperty('disabled', false);
});


it('keeps queue observation mounted while controls are collapsed and consent independently visible', async () => {
  const control = vi.spyOn(swarmQueueApi, 'control'), enqueue = vi.spyOn(swarmQueueApi, 'enqueueMany');
  render(<Harness />);
  await waitFor(() => expect(screen.getByRole('button', {name: 'Queue reviewed tasks (3)'})).toHaveProperty('disabled', false));
  expect(swarmQueueApi.get).toHaveBeenCalledOnce();
  const disclosure = screen.getByText(/^Queue status and controls/);
  expect(disclosure.closest('details')).toHaveProperty('open', false);
  await userEvent.click(disclosure);
  expect(disclosure.closest('details')).toHaveProperty('open', true);
  expect(screen.getByRole('button', {name: 'Refresh queue'})).toBeTruthy();
  await userEvent.click(disclosure);
  await userEvent.click(screen.getByRole('button', {name: 'Queue reviewed tasks (3)'}));
  expect(screen.getByRole('dialog', {name: 'Queue reviewed work'}).closest('details')).toBeNull();
  expect(disclosure.closest('details')).toHaveProperty('open', false);
  await userEvent.click(screen.getByRole('button', {name: 'Keep reviewing'}));
  expect(control).not.toHaveBeenCalled(); expect(enqueue).not.toHaveBeenCalled();
  expect(swarmQueueApi.get).toHaveBeenCalledOnce();
});

it('shows failed queue work separately and offers no queue replay or pending-item controls', async () => {
  const failed = swarmQueueItem({state:'failed', reasonCode:'provider_reservation_refused', reason:'The provider reservation was refused.'});
  vi.mocked(swarmQueueApi.get).mockResolvedValue(swarmQueueFixture([failed]));
  const control = vi.spyOn(swarmQueueApi, 'control'), enqueue = vi.spyOn(swarmQueueApi, 'enqueueMany');
  render(<Harness />); await screen.findByText(/Queued work needs attention/);
  await openQueueControls();
  expect(screen.getByText('The provider reservation was refused.')).toBeTruthy();
  for (const name of ['Pause queued task','Resume queued task','Cancel queued task']) expect(screen.queryByRole('button',{name})).toBeNull();
  expect(screen.queryByRole('combobox')).toBeNull();
  expect(control).not.toHaveBeenCalled(); expect(enqueue).not.toHaveBeenCalled();
});

it('explains automatic old-context stop without offering a duplicate queue or cancel action', async () => {
  const pending = swarmQueueItem({state:'unknown', reasonCode:'context_superseded_stop_pending', reason:'Exact stop is pending.', memoryMiB:1024});
  vi.mocked(swarmQueueApi.get).mockResolvedValue(swarmQueueFixture([pending]));
  render(<Harness />);
  await openQueueControls();
  expect(await screen.findByText(/reservation remains held until absence and release are confirmed/)).toBeTruthy();
  expect(screen.queryByRole('button', {name:'Cancel queued task'})).toBeNull();
  expect(screen.queryByRole('button', {name:'Resume queued task'})).toBeNull();
});

it('names the selected per-task cloud boundary in queue consent', async () => {
  const run = {...initial, input:{...initial.input, route:{...initial.input.route, executionLocation:'cloud-microvm' as const}}};
  render(<Harness run={run} />);
  await waitFor(() => expect(screen.getByRole('button', {name:'Queue reviewed tasks (3)'})).toHaveProperty('disabled', false));
  await userEvent.click(screen.getByRole('button', {name:'Queue reviewed tasks (3)'}));
  expect(within(screen.getByRole('dialog')).getByText(/separate owned cloud microVMs/)).toBeTruthy();
});

it('requests one exact active cloud stop and retains pending charge after an unconfirmed result', async () => {
  const item = swarmQueueItem({state:'running', operationId:'op-one', childId:'vm-one', memoryMiB:1024});
  vi.mocked(swarmQueueApi.get).mockResolvedValue(swarmQueueFixture([item]));
  const pending = {...item, state:'unknown' as const, revision:item.revision+1, reasonCode:'operator_stop_pending'};
  const stop = vi.spyOn(swarmQueueApi, 'control').mockResolvedValue(swarmQueueFixture([pending]));
  render(<Harness />);
  await openQueueControls();
  await userEvent.click(await screen.findByRole('button', {name:'Stop active task'}));
  expect(stop).toHaveBeenCalledWith('p-1', item, 'stop', undefined);
  expect(screen.getByText(/child and reservation are confirmed settled/)).toBeTruthy();
  expect(screen.queryByRole('button', {name:'Stop active task'})).toBeNull();
  expect(screen.getByRole('button', {name:'Retry exact stop'})).toBeTruthy();
});
