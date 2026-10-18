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
  const paused = swarmQueueItem({state: 'paused', reasonCode: 'operator_paused'});
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
  const control = vi.spyOn(swarmQueueApi, 'control').mockResolvedValue(swarmQueueFixture([{...queued,state:'paused',reasonCode:'operator_paused',revision:3}]));
  render(<Harness />);
  await openQueueControls();
  await screen.findByText('Build a');
  expect(screen.queryByText('Another run')).toBeNull();
  expect(screen.getAllByRole('button',{name:'Cancel queued task'})).toHaveLength(1);
  await userEvent.click(screen.getByRole('button',{name:'Pause queued task'}));
  await screen.findByRole('button',{name:'Resume queued task'});
  expect(control).toHaveBeenCalledWith('p-1', queued, 'pause', undefined);
});

it('pauses repair continuation explicitly and resumes only to the retained checkpoint wait', async () => {
  const pending = swarmQueueItem({state:'paused', reasonCode:'repair_checkpoint_pending'});
  const paused = {...pending, revision:pending.revision + 1, reasonCode:'operator_paused', reason:'Future starts are paused. No running process was stopped.'};
  const restored = {...pending, revision:paused.revision + 1};
  const initialQueue = swarmQueueFixture([pending]), pausedQueue = swarmQueueFixture([paused]), restoredQueue = swarmQueueFixture([restored]);
  vi.mocked(swarmQueueApi.get).mockResolvedValue(initialQueue);
  let settlePause!: (queue: ProjectSwarmQueue) => void, settleResume!: (queue: ProjectSwarmQueue) => void;
  const control = vi.spyOn(swarmQueueApi, 'control')
    .mockReturnValueOnce(new Promise<ProjectSwarmQueue>(resolve => {settlePause = resolve;}))
    .mockReturnValueOnce(new Promise<ProjectSwarmQueue>(resolve => {settleResume = resolve;}));
  const enqueue = vi.spyOn(swarmQueueApi, 'enqueueMany'), changed = vi.fn();
  render(<Harness changed={changed} />);
  await openQueueControls();
  expect(await screen.findByText(/Repair continuation is waiting for a protected checkpoint/)).toBeTruthy();
  expect(screen.getByText(/No repair agent has started for this continuation/)).toBeTruthy();
  expect(screen.queryByRole('button', {name:'Resume queued task'})).toBeNull();
  expect(control).not.toHaveBeenCalled();
  await userEvent.click(screen.getByRole('button', {name:'Pause repair continuation'}));
  expect(control).toHaveBeenCalledTimes(1);
  expect(control).toHaveBeenNthCalledWith(1, 'p-1', pending, 'pause', undefined);
  expect(screen.getByRole('button', {name:'Pause repair continuation'})).toHaveProperty('disabled', true);
  expect(screen.getByRole('button', {name:'Cancel queued task'})).toHaveProperty('disabled', true);
  await userEvent.click(screen.getByRole('button', {name:'Pause repair continuation'}));
  expect(control).toHaveBeenCalledTimes(1);
  await act(async () => settlePause(pausedQueue));
  await waitFor(() => expect(screen.getByRole('button', {name:'Resume queued task'})).toHaveProperty('disabled', false));
  expect(screen.queryByRole('button', {name:'Pause repair continuation'})).toBeNull();
  expect(screen.queryByText(/Repair continuation is waiting for a protected checkpoint/)).toBeNull();
  expect(changed).toHaveBeenLastCalledWith(pausedQueue);
  await userEvent.click(screen.getByRole('button', {name:'Resume queued task'}));
  expect(control).toHaveBeenNthCalledWith(2, 'p-1', paused, 'resume', undefined);
  expect(screen.getByRole('button', {name:'Resume queued task'})).toHaveProperty('disabled', true);
  await userEvent.click(screen.getByRole('button', {name:'Resume queued task'}));
  expect(control).toHaveBeenCalledTimes(2);
  await act(async () => settleResume(restoredQueue));
  await waitFor(() => expect(screen.getByRole('button', {name:'Pause repair continuation'})).toHaveProperty('disabled', false));
  expect(screen.getByText(/Repair continuation is waiting for a protected checkpoint/)).toBeTruthy();
  expect(screen.getByText(/No repair agent has started for this continuation/)).toBeTruthy();
  expect(screen.queryByRole('button', {name:'Resume queued task'})).toBeNull();
  expect(changed.mock.calls.map(([queue]) => queue)).toEqual([initialQueue, pausedQueue, restoredQueue]);
  expect(control).toHaveBeenCalledTimes(2); expect(enqueue).not.toHaveBeenCalled();
});

it.each(['pause', 'resume'] as const)('retains the repair queue receipt after a %s conflict without retrying', async action => {
  const item = swarmQueueItem({state:'paused', reasonCode:action === 'pause' ? 'repair_checkpoint_pending' : 'operator_paused'});
  const queue = swarmQueueFixture([item]);
  vi.mocked(swarmQueueApi.get).mockResolvedValue(queue);
  const control = vi.spyOn(swarmQueueApi, 'control').mockRejectedValue(new Error('The queue revision changed. Refresh queue status before continuing.'));
  const enqueue = vi.spyOn(swarmQueueApi, 'enqueueMany'), changed = vi.fn();
  render(<Harness changed={changed} />);
  await openQueueControls();
  const name = action === 'pause' ? 'Pause repair continuation' : 'Resume queued task';
  await userEvent.click(await screen.findByRole('button', {name}));
  expect(await screen.findByText(/Queue update unavailable: The queue revision changed/)).toBeTruthy();
  expect(control).toHaveBeenCalledTimes(1);
  expect(control).toHaveBeenCalledWith('p-1', item, action, undefined);
  expect(changed).toHaveBeenCalledTimes(1);
  expect(changed).toHaveBeenCalledWith(queue);
  await userEvent.click(screen.getByRole('button', {name:'Refresh queue'}));
  await waitFor(() => expect(changed).toHaveBeenCalledTimes(2));
  expect(changed).toHaveBeenLastCalledWith(queue);
  expect(screen.getByRole('button', {name})).toHaveProperty('disabled', false);
  expect(control).toHaveBeenCalledTimes(1); expect(enqueue).not.toHaveBeenCalled();
});

it.each([
  {label:'legacy paused reason', state:'paused' as const, reasonCode:undefined},
  {label:'unrecognized paused reason', state:'paused' as const, reasonCode:'repair_checkpoint_unconfirmed'},
  {label:'unknown checkpoint ownership', state:'unknown' as const, reasonCode:'repair_checkpoint_pending'},
  {label:'unknown operator pause ownership', state:'unknown' as const, reasonCode:'operator_paused'},
])('does not infer repair pause or resume authority from $label', async ({state, reasonCode}) => {
  const item = swarmQueueItem({state, reasonCode, operationId:'op-repair', childId:'child-repair', memoryMiB:1024});
  vi.mocked(swarmQueueApi.get).mockResolvedValue(swarmQueueFixture([item]));
  const control = vi.spyOn(swarmQueueApi, 'control'), enqueue = vi.spyOn(swarmQueueApi, 'enqueueMany');
  render(<Harness />); await openQueueControls(); await screen.findByText(item.taskTitle);
  for (const name of ['Pause repair continuation', 'Pause queued task', 'Resume queued task']) expect(screen.queryByRole('button', {name})).toBeNull();
  expect(screen.queryByText(/Repair continuation is waiting for a protected checkpoint/)).toBeNull();
  if (state === 'unknown') {
    expect(screen.getByText(/Capacity still reserved: 1024 MiB memory/)).toBeTruthy();
    expect(screen.getByRole('button', {name:'Stop active task'})).toHaveProperty('disabled', false);
    expect(screen.queryByRole('button', {name:'Cancel queued task'})).toBeNull();
  } else expect(screen.getByRole('button', {name:'Cancel queued task'})).toHaveProperty('disabled', false);
  expect(control).not.toHaveBeenCalled(); expect(enqueue).not.toHaveBeenCalled();
});

it.each(['repair_checkpoint_pending', 'operator_paused'])('keeps %s repair controls read-only for a viewer', async reasonCode => {
  vi.mocked(swarmQueueApi.get).mockResolvedValue(swarmQueueFixture([swarmQueueItem({state:'paused', reasonCode})]));
  const control = vi.spyOn(swarmQueueApi, 'control');
  render(<Harness edit={false} />); await openQueueControls();
  const name = reasonCode === 'repair_checkpoint_pending' ? 'Pause repair continuation' : 'Resume queued task';
  const button = await screen.findByRole('button', {name});
  expect(button).toHaveProperty('disabled', true);
  expect(screen.getByRole('button', {name:'Cancel queued task'})).toHaveProperty('disabled', true);
  await userEvent.click(button);
  expect(control).not.toHaveBeenCalled();
});

it.each(['execution permission', 'source state', 'task capability'])('allows repair pause but keeps resume blocked by %s', async guard => {
  const pending = swarmQueueItem({state:'paused', reasonCode:'repair_checkpoint_pending'});
  const paused = {...pending, revision:pending.revision + 1, reasonCode:'operator_paused'};
  vi.mocked(swarmQueueApi.get).mockResolvedValue(swarmQueueFixture([pending]));
  const control = vi.spyOn(swarmQueueApi, 'control').mockResolvedValue(swarmQueueFixture([paused]));
  const run = guard === 'source state' ? {...initial, state:'cancelled' as const} : initial;
  if (guard === 'task capability') useCockpit.getState().setHarnesses([{...swarmHarnessFixture(), autonomousTask:false}]);
  render(<Harness run={run} execute={guard !== 'execution permission'} />); await openQueueControls();
  const pause = await screen.findByRole('button', {name:'Pause repair continuation'});
  expect(pause).toHaveProperty('disabled', false);
  await userEvent.click(pause);
  const resume = await screen.findByRole('button', {name:'Resume queued task'});
  expect(resume).toHaveProperty('disabled', true);
  expect(screen.getByRole('button', {name:'Cancel queued task'})).toHaveProperty('disabled', false);
  await userEvent.click(resume);
  expect(control).toHaveBeenCalledTimes(1);
  expect(control).toHaveBeenCalledWith('p-1', pending, 'pause', undefined);
});

it('shows restored cloud ownership as pending without offering a replacement start', async () => {
  const run = {...initial, input: {...initial.input, route: {...initial.input.route, executionLocation: 'cloud-microvm' as const}}};
  vi.mocked(swarmQueueApi.get).mockResolvedValue(swarmQueueFixture([swarmQueueItem({state: 'unknown', reasonCode: 'cloud_restore_pending', operationId: 'op-a', childId: 'vm-a', memoryMiB: 2048, cpuMillis: 2000})]));
  render(<Harness run={run} />);
  await openQueueControls();
  expect(await screen.findByText(/checking the exact restored microVM identities/)).not.toBeNull();
  expect(screen.getByText(/Capacity still reserved: 2048 MiB memory and 2000m CPU/)).not.toBeNull();
  expect(screen.getByRole('button', {name: 'Stop active task'})).not.toBeNull();
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
  expect(screen.getByText(/Capacity still reserved: 1024 MiB memory and 0m CPU/)).toBeTruthy();
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

it('lets a current editor retry exact cleanup after execution permission is withdrawn', async () => {
  const pending = swarmQueueItem({state:'unknown', operationId:'op-one', childId:'vm-one', memoryMiB:1024, reasonCode:'operator_stop_pending'});
  vi.mocked(swarmQueueApi.get).mockResolvedValue(swarmQueueFixture([pending]));
  const stop = vi.spyOn(swarmQueueApi, 'control').mockResolvedValue(swarmQueueFixture([{...pending, state:'cancelled', memoryMiB:0}]));
  render(<Harness execute={false} />);
  await openQueueControls();
  const retry = await screen.findByRole('button', {name:'Retry exact stop'});
  expect(retry).toHaveProperty('disabled', false);
  await userEvent.click(retry);
  expect(stop).toHaveBeenCalledWith('p-1', pending, 'stop', undefined);
  expect(screen.getByRole('button', {name:'Queue reviewed tasks (3)'})).toHaveProperty('disabled', true);
});
