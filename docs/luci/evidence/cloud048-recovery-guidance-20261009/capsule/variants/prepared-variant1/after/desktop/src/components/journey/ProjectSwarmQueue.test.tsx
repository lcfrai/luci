import {openTaskActions} from '../../test/swarmActions';
import { useState } from 'react';
import { act, cleanup, fireEvent, render, screen, waitFor, within } from '@testing-library/react';
import userEvent from '@testing-library/user-event';
import { afterEach, beforeEach, expect, it, vi } from 'vitest';
import { projectRunsApi, type ProjectRun } from '../../api/projectRuns';
import { projectSwarmApi } from '../../api/projectSwarm';
import { api } from '../../api';
import { swarmQueueApi } from '../../api/swarmQueue';
import { swarmHarnessFixture, swarmQueueFixture, swarmQueueItem, swarmRunFixture } from '../../test/swarmFixtures';
import { resetCockpit, useCockpit } from '../../store/store';
import { ProjectSwarm } from './ProjectSwarm';

afterEach(() => vi.restoreAllMocks());
beforeEach(() => { resetCockpit(); useCockpit.getState().setHarnesses([swarmHarnessFixture()]); });
const run = swarmRunFixture();

it('disables a manual start already in the durable queue', async () => {
  vi.spyOn(swarmQueueApi, 'get').mockResolvedValue(swarmQueueFixture([swarmQueueItem()]));
  const reserve = vi.spyOn(projectSwarmApi, 'reserve');
  render(<ProjectSwarm run={run} canEdit canExecute busy={false} onBusy={vi.fn()} onUpdate={vi.fn()} onOpenSession={vi.fn()} />);
  await userEvent.click(screen.getByText(/^Queue status and controls/));
  await screen.findByRole('button', {name: 'Pause queued task'});
  expect(screen.getAllByRole('button', {name: 'Start task'})[0]).toHaveProperty('disabled', true);
  expect(screen.getByRole('button', {name: 'Start ready tasks (1)'})).toHaveProperty('disabled', false);
  expect(reserve).not.toHaveBeenCalled();
});

it('refreshes a background dispatch after an intervening UI operation and retries a failed read', async () => {
  vi.spyOn(swarmQueueApi, 'get').mockResolvedValue(swarmQueueFixture([swarmQueueItem({state: 'running', operationId: 'op-a', childId: 'child-a'})]));
  const updated = vi.fn(), get = vi.spyOn(projectRunsApi, 'get').mockRejectedValueOnce(new Error('temporary offline')).mockResolvedValue({...run,revision:2});
  const view = render(<ProjectSwarm run={run} canEdit canExecute busy onBusy={vi.fn()} onUpdate={updated} onOpenSession={vi.fn()} />);
  await userEvent.click(screen.getByText(/^Queue status and controls/));
  await screen.findByText('running');
  expect(get).not.toHaveBeenCalled();
  view.rerender(<ProjectSwarm run={run} canEdit canExecute busy={false} onBusy={vi.fn()} onUpdate={updated} onOpenSession={vi.fn()} />);
  await userEvent.click(screen.getByRole('button',{name:'Refresh queue'}));
  await waitFor(() => expect(get).toHaveBeenCalledTimes(1));
  expect(updated).not.toHaveBeenCalled();
  await userEvent.click(screen.getByRole('button',{name:'Refresh queue'}));
  await waitFor(() => expect(updated).toHaveBeenCalledWith({...run,revision:2}));
  expect(get).toHaveBeenCalledTimes(2);
});

it('offers retirement after a cancelled queue leaves an unstarted reserved workspace', async () => {
  const op = {id:'op-a',kind:'session',taskId:'a',childId:'child-a',attempt:1,status:'reserved',allocation:'ready' as const,requestDigest:'digest',createdAt:run.createdAt,updatedAt:run.updatedAt};
  const reserved = {...run, operations:[op], swarm: {...run.swarm!, tasks:run.swarm!.tasks.map(task => task.id === 'a' ? {...task,operationId:op.id} : task)}};
  vi.spyOn(swarmQueueApi, 'get').mockResolvedValue(swarmQueueFixture([swarmQueueItem({state:'cancelled',operationId:op.id,childId:op.childId})]));
  vi.spyOn(projectRunsApi, 'get').mockResolvedValue(reserved);
  const abandon = vi.spyOn(projectSwarmApi, 'abandon').mockResolvedValue({...reserved,revision:2,operations:[{...op,status:'cancelled',allocation:'abandoned'}]});
  render(<ProjectSwarm run={reserved} canEdit canExecute busy={false} onBusy={vi.fn()} onUpdate={vi.fn()} onOpenSession={vi.fn()} />);
  await openTaskActions();
  await screen.findByText(/This reservation belongs to the background queue/);
  expect(screen.getByRole('button',{name:'Continue reserved task'})).toHaveProperty('disabled',true);
  await userEvent.click(screen.getByRole('button',{name:'Retain changes and retry'}));
  await userEvent.click(screen.getByRole('button',{name:'Retain output and allow retry'}));
  expect(abandon).toHaveBeenCalledWith('p-1','run-1','a',1);
});

it('keeps exact queue consent open while a manual task completes, then refreshes after close', async () => {
  vi.useFakeTimers();
  try {
    vi.spyOn(document,'visibilityState','get').mockReturnValue('visible');
    vi.spyOn(swarmQueueApi,'get').mockResolvedValue(swarmQueueFixture());
    const enqueue=vi.spyOn(swarmQueueApi,'enqueueMany');
    const op={id:'op-a',kind:'session',taskId:'a',childId:'child-a',worktreeId:'isolated-a',attempt:1,status:'running',allocation:'ready' as const,requestDigest:'digest',createdAt:run.createdAt,updatedAt:run.updatedAt,dependencyContext:{digest:'d'.repeat(64),count:0}};
    const active={...run,operations:[op],swarm:{...run.swarm!,tasks:run.swarm!.tasks.map(task=>task.id==='a'?{...task,operationId:op.id}:task)}};
    const finished:ProjectRun={...active,revision:2,operations:[{...op,status:'review-required',handoff:{digest:'a'.repeat(64),status:'missing',summary:'',questions:0,unanswered:0,capturedAt:run.updatedAt}}]};
    let resolve!:(value:ProjectRun)=>void;
    const get=vi.spyOn(projectRunsApi,'get').mockReturnValueOnce(new Promise(done=>{resolve=done;})).mockResolvedValue(finished);
    function Selected(){const[value,setValue]=useState<ProjectRun>(active);return <ProjectSwarm run={value} canEdit canExecute busy={false} onBusy={vi.fn()} onUpdate={setValue} onOpenSession={vi.fn()}/>;}
    render(<Selected/>);await act(async()=>{});await act(async()=>vi.advanceTimersByTimeAsync(5000));
    expect(get).toHaveBeenCalledOnce();
    fireEvent.click(screen.getByRole('button',{name:'Queue reviewed tasks (2)'}));
    const dialog=screen.getByRole('dialog',{name:'Queue reviewed work'});
    fireEvent.change(within(dialog).getByRole('combobox'),{target:{value:'high'}});
    expect(get.mock.calls[0][2]!.aborted).toBe(true);
    await act(async()=>resolve(finished));await act(async()=>vi.advanceTimersByTimeAsync(30_000));
    expect(screen.getByRole('dialog',{name:'Queue reviewed work'})).toBe(dialog);
    expect(within(dialog).getByRole('combobox')).toHaveProperty('value','high');
    expect(get).toHaveBeenCalledOnce();expect(enqueue).not.toHaveBeenCalled();
    fireEvent.click(within(dialog).getByRole('button',{name:'Keep reviewing'}));
    await act(async()=>vi.advanceTimersByTimeAsync(5000));
    expect(get).toHaveBeenCalledTimes(2);expect(screen.getByText('Awaiting review')).not.toBeNull();
  } finally {cleanup();vi.useRealTimers();}
});

function refusedStart(): ProjectRun {
  const base = swarmRunFixture();
  const operation = {id:'op-refused',kind:'session',taskId:'a',childId:'child-refused',worktreeId:'isolated-refused',attempt:1,status:'failed',allocation:'ready' as const,requestDigest:'digest',createdAt:base.createdAt,updatedAt:base.updatedAt,
    startFailure:{stage:'admission' as const,reasonCode:'cpu_pressure',message:'Measured CPU pressure prevented final admission. No sandbox allocation was attempted. Inspect this retained attempt before explicitly retrying.'}};
  return {...base,state:'blocked',operations:[operation],input:{...base.input,implementation:{...base.input.implementation,taskIds:['a'],taskTitles:['Build a']}},swarm:{...base.swarm!,tasks:[{...base.swarm!.tasks[0],operationId:operation.id}]}};
}

it('guides a confirmed retired refusal to queue review without starting work or bypassing separate consent', async () => {
  const initial = refusedStart(), diagnosis = initial.operations[0].startFailure;
  let current: ProjectRun = initial;
  const failed = swarmQueueItem({state:'failed',operationId:'op-refused',childId:'child-refused',reasonCode:'cpu_pressure',reason:diagnosis!.message});
  vi.spyOn(swarmQueueApi,'get').mockResolvedValue(swarmQueueFixture([failed]));
  vi.spyOn(projectRunsApi,'get').mockImplementation(async () => current);
  const abandon = vi.spyOn(projectSwarmApi,'abandon').mockImplementation(async () => {
    current = {...initial,revision:2,operations:[{...initial.operations[0],status:'cancelled'}]};return current;
  });
  const queued = swarmQueueItem({id:'q-retry'}), enqueue = vi.spyOn(swarmQueueApi,'enqueueMany').mockResolvedValue({items:[queued],queue:swarmQueueFixture([failed,queued])});
  const reserve = vi.spyOn(projectSwarmApi,'reserve'), create = vi.spyOn(api,'createSession');
  function Selected() { const [value,setValue] = useState(initial); return <ProjectSwarm run={value} canEdit canExecute busy={false} onBusy={vi.fn()} onUpdate={setValue} onOpenSession={vi.fn()} />; }
  render(<Selected />);
  expect(screen.queryByRole('button',{name:'Go to queue review'})).toBeNull();
  await openTaskActions();
  await userEvent.click(screen.getByRole('button',{name:'Retain changes and retry'}));
  await userEvent.click(screen.getByRole('button',{name:'Retain output and allow retry'}));
  const next = await screen.findByRole('button',{name:'Go to queue review'});
  const review = screen.getByRole('button',{name:'Queue reviewed tasks (1)'});
  await waitFor(() => expect(review).toHaveProperty('disabled',false));
  expect(screen.getByText('Previous start: '+diagnosis!.message)).not.toBeNull();
  expect(screen.getByText(/Retiring this attempt did not start another agent/)).not.toBeNull();
  expect(screen.queryByRole('button',{name:'Retry in a new workspace'})).toBeNull();
  await userEvent.click(next);
  expect(document.activeElement).toBe(review);expect(screen.queryByRole('dialog')).toBeNull();
  expect(enqueue).not.toHaveBeenCalled();expect(reserve).not.toHaveBeenCalled();expect(create).not.toHaveBeenCalled();
  await userEvent.click(review);
  const consent = screen.getByRole('dialog',{name:'Queue reviewed work'});
  expect(within(consent).getByText('Build a')).not.toBeNull();expect(enqueue).not.toHaveBeenCalled();
  await userEvent.click(within(consent).getByRole('button',{name:'Authorize queued tasks'}));
  await waitFor(() => expect(enqueue).toHaveBeenCalledOnce());
  expect(enqueue).toHaveBeenCalledWith('p-1','run-1',['a'],2,expect.any(String),'normal');
  expect(abandon).toHaveBeenCalledOnce();expect(abandon).toHaveBeenCalledWith('p-1','run-1','a',1);
  expect(current.operations[0].startFailure).toEqual(diagnosis);expect(current.operations).toHaveLength(1);
  expect(reserve).not.toHaveBeenCalled();expect(create).not.toHaveBeenCalled();
});

it.each(['queued','execute-revoked'] as const)('keeps retired-refusal navigation informative when queue authorization is disabled by %s', async reason => {
  const initial = refusedStart(), retired = {...initial,revision:2,operations:[{...initial.operations[0],status:'cancelled'}]};
  vi.spyOn(swarmQueueApi,'get').mockResolvedValue(swarmQueueFixture(reason==='queued'?[swarmQueueItem({id:'q-new-attempt'})]:[]));
  const enqueue = vi.spyOn(swarmQueueApi,'enqueueMany'), reserve = vi.spyOn(projectSwarmApi,'reserve'), create = vi.spyOn(api,'createSession');
  render(<ProjectSwarm run={retired} canEdit canExecute={reason!=='execute-revoked'} busy={false} onBusy={vi.fn()} onUpdate={vi.fn()} onOpenSession={vi.fn()} />);
  const review = await screen.findByRole('button',{name:/^Queue reviewed tasks/});
  if (reason==='queued') await waitFor(() => expect(screen.getByRole('button',{name:'Queue reviewed tasks'})).toBe(review));
  await waitFor(() => expect(review).toHaveProperty('disabled',true));
  await userEvent.click(screen.getByRole('button',{name:'Go to queue review'}));
  expect(document.activeElement).toBe(screen.getByRole('region',{name:'Background queue review'}));
  expect(screen.queryByRole('dialog')).toBeNull();expect(enqueue).not.toHaveBeenCalled();expect(reserve).not.toHaveBeenCalled();expect(create).not.toHaveBeenCalled();
  expect(screen.getByText('Previous start: '+initial.operations[0].startFailure!.message)).not.toBeNull();
});

it('drops retired-refusal guidance on a project switch and never carries its navigation consent into the new project', async () => {
  const initial = refusedStart(), retired = {...initial,revision:2,operations:[{...initial.operations[0],status:'cancelled'}]};
  const next = {...swarmRunFixture(),id:'run-next',projectId:'p-next'};
  next.input = {...next.input,implementation:{...next.input.implementation,projectId:next.projectId}};
  vi.spyOn(swarmQueueApi,'get').mockResolvedValue(swarmQueueFixture());
  const enqueue = vi.spyOn(swarmQueueApi,'enqueueMany');
  const props = {canEdit:true,canExecute:true,busy:false,onBusy:vi.fn(),onUpdate:vi.fn(),onOpenSession:vi.fn()};
  const view = render(<ProjectSwarm {...props} run={retired} />);
  const oldAction = screen.getByRole('button',{name:'Go to queue review'});
  view.rerender(<ProjectSwarm {...props} run={next} />);
  expect(screen.queryByRole('button',{name:'Go to queue review'})).toBeNull();
  expect(screen.queryByText('Previous start: '+initial.operations[0].startFailure!.message)).toBeNull();
  fireEvent.click(oldAction);
  expect(screen.queryByRole('dialog')).toBeNull();expect(enqueue).not.toHaveBeenCalled();
  expect(document.activeElement).not.toBe(screen.getByRole('region',{name:'Background queue review'}));
});
