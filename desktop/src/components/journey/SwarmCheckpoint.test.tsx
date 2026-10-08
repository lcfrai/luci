import {openTaskActions} from '../../test/swarmActions';
import {useState} from 'react';
import {act, render, screen, waitFor, within} from '@testing-library/react';
import userEvent from '@testing-library/user-event';
import {beforeEach, expect, it, vi} from 'vitest';
import {api} from '../../api';
import {projectRunsApi, type ProjectRun, type RunOperation} from '../../api/projectRuns';
import {projectSwarmApi} from '../../api/projectSwarm';
import {swarmCheckpointApi, type SwarmCheckpointReview} from '../../api/swarmCheckpoint';
import {swarmHarnessFixture, swarmRunFixture} from '../../test/swarmFixtures';
import {resetCockpit, useCockpit} from '../../store/store';
import {ProjectSwarm} from './ProjectSwarm';

vi.mock('./SwarmRunQueue', () => ({SwarmRunQueue: () => null}));
const action = 'Continue from retained changes', confirm = 'Continue from these reviewed changes';
function retained(): ProjectRun {
  const run = swarmRunFixture();
  const op: RunOperation = {id:'op-a', childId:'child-a', taskId:'a', worktreeId:'old-a', attempt:1, kind:'session', status:'failed', allocation:'ready', baseCommit:'a'.repeat(40), requestDigest:'request', createdAt:run.createdAt, updatedAt:run.updatedAt};
  return {...run, operations:[op], swarm:{...run.swarm!, tasks:run.swarm!.tasks.map(task => task.id === 'a' ? {...task, operationId:op.id} : task)}};
}
function review(run = retained()): SwarmCheckpointReview {
  return {projectId:run.projectId,runId:run.id,operationId:'op-a', childId:'child-a', taskId:'a', expectedRevision:run.revision, sourceCommit:'b'.repeat(40), targetCommit:'a'.repeat(40), reviewDigest:'d'.repeat(64), canContinue:true, reason:'Complete changes can seed a new attempt.', totalBytes:8, files:[{path:'src/a.ts', status:'modified', before:'old\n', after:'new\n', beforeMode:'100644', afterMode:'100644'}]};
}
function prepared(run = retained()) {
  const selected = review(run);
  const op: RunOperation = {...run.operations[0], id:'op-b', childId:'child-b', worktreeId:'new-a', attempt:2, status:'reserved', checkpoint:{sourceOperationId:'op-a', sourceChildId:'child-a', sourceCommit:selected.sourceCommit, sourceBaseCommit:'a'.repeat(40), targetCommit:selected.targetCommit, seedCommit:'c'.repeat(40), reviewDigest:selected.reviewDigest, contextDigest:'e'.repeat(64), createdAt:run.createdAt}};
  return {operation:op, run:{...run, revision:run.revision+1, operations:[...run.operations,op], swarm:{...run.swarm!, tasks:run.swarm!.tasks.map(task => task.id === 'a' ? {...task,operationId:op.id} : task)}}};
}
function Harness({initial=retained(), canEdit=true, canExecute=true}:{initial?:ProjectRun;canEdit?:boolean;canExecute?:boolean}) {
  const [run,setRun]=useState(initial),[busy,setBusy]=useState('');
  return <ProjectSwarm run={run} canEdit={canEdit} canExecute={canExecute} busy={!!busy} onBusy={setBusy} onUpdate={setRun} onOpenSession={vi.fn()} />;
}
beforeEach(()=>{vi.restoreAllMocks();resetCockpit();useCockpit.getState().setHarnesses([swarmHarnessFixture()]);});

it('reviews complete retained changes, requires separate consent and prepares without starting or integrating',async()=>{
  const run=retained(), selected=review(run);
  const read=vi.spyOn(swarmCheckpointApi,'review').mockResolvedValue(selected);
  const reserve=vi.spyOn(swarmCheckpointApi,'reserve').mockResolvedValue(prepared(run));
  const create=vi.spyOn(api,'createSession'), integrate=vi.spyOn(projectSwarmApi,'integrate');
  render(<Harness initial={run}/>);
  await openTaskActions();expect(read).not.toHaveBeenCalled();
  await userEvent.click(screen.getByRole('button',{name:action}));
  const dialog=screen.getByRole('dialog',{name:action});
  const consent=await within(dialog).findByRole('checkbox');
  expect(within(dialog).getByText(/task stays incomplete/)).not.toBeNull();
  await userEvent.click(within(dialog).getByText('src/a.ts'));
  expect(within(dialog).getByLabelText('Before src/a.ts').textContent).toBe('old\n');
  expect(within(dialog).getByLabelText('After src/a.ts').textContent).toBe('new\n');
  expect(within(dialog).getByRole('button',{name:confirm})).toHaveProperty('disabled',true);
  await userEvent.click(consent);await userEvent.click(within(dialog).getByRole('button',{name:confirm}));
  await waitFor(()=>expect(reserve).toHaveBeenCalledTimes(1));
  expect(reserve.mock.calls[0][0].id).toBe(run.id);expect(reserve.mock.calls[0][1].id).toBe('op-a');expect(reserve.mock.calls[0][2]).toEqual(selected);
  expect(reserve.mock.calls[0][3]).toMatch(/^[a-f0-9-]{36}$/);
  await waitFor(()=>expect(screen.queryByRole('dialog')).toBeNull());
  expect(screen.getByText(/new attempt is prepared with the reviewed retained changes/)).not.toBeNull();
  await waitFor(()=>expect(document.activeElement).toBe(screen.getByRole('button',{name:'Continue reserved task'})));
  expect(create).not.toHaveBeenCalled();expect(integrate).not.toHaveBeenCalled();
});
it('returns keyboard focus to the connected opener when keeping the retained attempt',async()=>{
  vi.spyOn(swarmCheckpointApi,'review').mockResolvedValue(review());
  const reserve=vi.spyOn(swarmCheckpointApi,'reserve');render(<Harness/>);
  await openTaskActions();const opener=screen.getByRole('button',{name:action});opener.focus();
  await userEvent.keyboard('{Enter}');const dialog=screen.getByRole('dialog',{name:action});
  await within(dialog).findByRole('checkbox');await userEvent.keyboard('{Escape}');
  await waitFor(()=>expect(screen.queryByRole('dialog')).toBeNull());
  await waitFor(()=>expect(document.activeElement).toBe(opener));expect(reserve).not.toHaveBeenCalled();
});
it('shows a complete-review refusal without offering consent or allocating',async()=>{
  vi.spyOn(swarmCheckpointApi,'review').mockResolvedValue({...review(),canContinue:false,reason:'A retained file overlaps a newer primary change.',files:[],totalBytes:0});
  const reserve=vi.spyOn(swarmCheckpointApi,'reserve');render(<Harness/>);
  await openTaskActions();
  await userEvent.click(screen.getByRole('button',{name:action}));
  const dialog=screen.getByRole('dialog',{name:action});await within(dialog).findByText(/overlaps a newer primary/);
  expect(within(dialog).queryByRole('checkbox')).toBeNull();expect(within(dialog).getByRole('button',{name:confirm})).toHaveProperty('disabled',true);expect(reserve).not.toHaveBeenCalled();
});
it('clears consent on refresh failure and rejects late review after permission revoke/regrant',async()=>{
  let done!:(v:SwarmCheckpointReview)=>void;
  vi.spyOn(swarmCheckpointApi,'review').mockReturnValueOnce(new Promise(resolve=>{done=resolve;})).mockResolvedValueOnce(review()).mockRejectedValueOnce(new Error('Retained source changed.'));
  const reserve=vi.spyOn(swarmCheckpointApi,'reserve');const view=render(<Harness/>);
  await openTaskActions();
  await userEvent.click(screen.getByRole('button',{name:action}));view.rerender(<Harness canEdit={false}/>);view.rerender(<Harness/>);
  await act(async()=>done(review()));expect(screen.queryByRole('dialog')).toBeNull();
  await userEvent.click(screen.getByRole('button',{name:action}));const dialog=screen.getByRole('dialog',{name:action});
  await userEvent.click(await within(dialog).findByRole('checkbox'));
  await userEvent.click(within(dialog).getByRole('button',{name:'Refresh retained-change review'}));
  await within(dialog).findByRole('alert');expect(within(dialog).queryByRole('checkbox')).toBeNull();expect(reserve).not.toHaveBeenCalled();
});
it('reads back a lost reservation once without automatic allocation retry or agent start',async()=>{
  vi.spyOn(swarmCheckpointApi,'review').mockResolvedValue(review());
  const reserve=vi.spyOn(swarmCheckpointApi,'reserve').mockRejectedValue(new Error('Response lost.'));
  const get=vi.spyOn(projectRunsApi,'get').mockResolvedValue(prepared().run), create=vi.spyOn(api,'createSession');
  render(<Harness/>);
  await openTaskActions();await userEvent.click(screen.getByRole('button',{name:action}));const dialog=screen.getByRole('dialog',{name:action});
  await userEvent.click(await within(dialog).findByRole('checkbox'));await userEvent.click(within(dialog).getByRole('button',{name:confirm}));
  await waitFor(()=>expect(screen.queryByRole('dialog')).toBeNull());
  await waitFor(()=>expect(document.activeElement).toBe(screen.getByRole('button',{name:'Continue reserved task'})));
  expect(reserve).toHaveBeenCalledTimes(1);expect(get).toHaveBeenCalledTimes(1);expect(create).not.toHaveBeenCalled();
});
it('focuses the exact task heading when the recorded continuation cannot yet start',async()=>{
  const result=prepared();result.operation.allocation='unknown';
  vi.spyOn(swarmCheckpointApi,'review').mockResolvedValue(review());
  vi.spyOn(swarmCheckpointApi,'reserve').mockResolvedValue(result);
  render(<Harness/>);await openTaskActions();await userEvent.click(screen.getByRole('button',{name:action}));
  const dialog=screen.getByRole('dialog',{name:action});await userEvent.click(await within(dialog).findByRole('checkbox'));
  await userEvent.click(within(dialog).getByRole('button',{name:confirm}));
  await waitFor(()=>expect(screen.queryByRole('dialog')).toBeNull());
  await waitFor(()=>expect(document.activeElement).toBe(screen.getByRole('heading',{name:result.run.swarm!.tasks[0].title})));
  expect(screen.queryByRole('button',{name:'Continue reserved task'})).toBeNull();
});
it('does not restore an old task target or commit a late reservation after switching build scope',async()=>{
  let finish!:(value:ReturnType<typeof prepared>)=>void;
  vi.spyOn(swarmCheckpointApi,'review').mockResolvedValue(review());
  vi.spyOn(swarmCheckpointApi,'reserve').mockReturnValue(new Promise(resolve=>{finish=resolve;}));
  const original=retained(),update=vi.fn(),props={canEdit:true,canExecute:true,busy:false,onBusy:vi.fn(),onUpdate:update,onOpenSession:vi.fn()};
  const view=render(<ProjectSwarm {...props} run={original}/>);await openTaskActions();
  await userEvent.click(screen.getByRole('button',{name:action}));const dialog=screen.getByRole('dialog',{name:action});
  await userEvent.click(await within(dialog).findByRole('checkbox'));await userEvent.click(within(dialog).getByRole('button',{name:confirm}));
  await waitFor(()=>expect(swarmCheckpointApi.reserve).toHaveBeenCalledOnce());
  view.rerender(<ProjectSwarm {...props} run={{...prepared(original).run,id:'another-build',projectId:'another-project'}}/>);
  const destination=screen.getByRole('button',{name:'Pause new starts'});destination.focus();
  await act(async()=>{finish(prepared(original));});
  await waitFor(()=>expect(screen.queryByRole('dialog')).toBeNull());
  expect(document.activeElement).toBe(destination);expect(update).not.toHaveBeenCalled();
});
it.each([{canEdit:false},{canExecute:false}])('does not review or reserve without both permissions: %j',async permission=>{
  const read=vi.spyOn(swarmCheckpointApi,'review'),reserve=vi.spyOn(swarmCheckpointApi,'reserve');render(<Harness {...permission}/>);
  await openTaskActions();
  expect(screen.getByRole('button',{name:action})).toHaveProperty('disabled',true);await userEvent.click(screen.getByRole('button',{name:action}));expect(read).not.toHaveBeenCalled();expect(reserve).not.toHaveBeenCalled();
});
it('reports an HTTP-successful retained integration conflict without claiming integration or dependency release',async()=>{
  const run=retained();run.operations[0].status='review-required';
  vi.spyOn(projectSwarmApi,'review').mockResolvedValue({taskId:'a',operationId:'op-a',sourceCommit:'b'.repeat(40),targetCommit:'a'.repeat(40),reviewDigest:'d'.repeat(64),paths:['src/a.ts'],patch:'+new',canIntegrate:true,reason:'Ready for reviewed integration.'});
  const changed={...run,revision:run.revision+1,state:'blocked' as const,swarm:{...run.swarm!,tasks:run.swarm!.tasks.map(task=>task.id==='a'?{...task,integration:{state:'unknown' as const,sourceCommit:'b'.repeat(40),targetCommit:'a'.repeat(40),reviewDigest:'d'.repeat(64)}}:task)}};
  vi.spyOn(projectSwarmApi,'integrate').mockResolvedValue(changed);
  render(<Harness initial={run}/>);await userEvent.click(screen.getByRole('button',{name:'Review task changes'}));const dialog=await screen.findByRole('dialog',{name:'Review task changes'});
  await userEvent.click(within(dialog).getByRole('checkbox'));await userEvent.click(within(dialog).getByRole('button',{name:'Integrate reviewed changes'}));
  await screen.findByText(/Integration did not complete/);expect(screen.queryByText(/Reviewed changes integrated/)).toBeNull();expect(screen.getByText(/Dependent tasks still wait/)).not.toBeNull();
});
