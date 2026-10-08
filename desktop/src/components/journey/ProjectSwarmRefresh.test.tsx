import {StrictMode,useState} from 'react';
import {act,cleanup,fireEvent,render,screen,within} from '@testing-library/react';
import {afterEach,beforeEach,expect,it,vi} from 'vitest';
import {api} from '../../api';
import {ApiError} from '../../api/client';
import {projectRunsApi,type ProjectRun,type RunOperation} from '../../api/projectRuns';
import {projectSwarmApi} from '../../api/projectSwarm';
import {resetCockpit,useCockpit} from '../../store/store';
import {swarmHarnessFixture,swarmRunFixture} from '../../test/swarmFixtures';
import {ProjectSwarm} from './ProjectSwarm';

const access=vi.hoisted(()=>({canRead:true,canEdit:true,canExecute:true,loading:false,error:''}));
vi.mock('../../hooks/useProjectAccess',()=>({useProjectAccess:()=>access}));
vi.mock('./SwarmRunQueue',()=>({SwarmRunQueue:()=>null}));
function active():ProjectRun {
  const run=swarmRunFixture();
  const operation:RunOperation={id:'op-a',taskId:'a',childId:'child-a',worktreeId:'isolated-a',attempt:1,kind:'session',status:'running',allocation:'ready',requestDigest:'request',createdAt:run.createdAt,updatedAt:run.updatedAt,dependencyContext:{digest:'d'.repeat(64),count:0}};
  return {...run,operations:[operation],swarm:{...run.swarm!,tasks:run.swarm!.tasks.map(task=>task.id==='a'?{...task,operationId:operation.id}:task)}};
}
function finished(run=active(),status:'captured'|'missing'|'invalid'|'unavailable'='captured'):ProjectRun {
  return {...run,revision:run.revision+1,operations:run.operations.map(op=>({...op,status:'review-required',handoff:{status,digest:'a'.repeat(64),summary:'Worker claim',questions:status==='captured'?1:0,unanswered:status==='captured'?1:0,capturedAt:run.updatedAt}}))};
}
const props=()=>({canEdit:true,canExecute:true,busy:false,onBusy:vi.fn(),onUpdate:vi.fn(),onOpenSession:vi.fn()});
function Harness({initial=active()}:{initial?:ProjectRun}){const[run,setRun]=useState(initial);return <ProjectSwarm {...props()} run={run} onUpdate={setRun}/>;}
async function tick(ms=5000){await act(async()=>{await vi.advanceTimersByTimeAsync(ms);});}
beforeEach(()=>{vi.restoreAllMocks();vi.useFakeTimers();resetCockpit();useCockpit.getState().setHarnesses([swarmHarnessFixture()]);Object.assign(access,{canRead:true,canEdit:true,canExecute:true,loading:false,error:''});vi.spyOn(document,'visibilityState','get').mockReturnValue('visible');});
afterEach(()=>{cleanup();vi.useRealTimers();vi.restoreAllMocks();});

it('updates the visible manual task to review and questions without navigation, mutation or a focus jump',async()=>{
  const get=vi.spyOn(projectRunsApi,'get').mockResolvedValue(finished()),control=vi.spyOn(projectSwarmApi,'control'),reserve=vi.spyOn(projectSwarmApi,'reserve'),create=vi.spyOn(api,'createSession');
  render(<StrictMode><Harness/></StrictMode>);const focus=screen.getByRole('button',{name:'Check team status'});focus.focus();
  expect(get).not.toHaveBeenCalled();await tick();
  expect(screen.getByText('Awaiting review')).not.toBeNull();expect(screen.getByText(/1 agent-reported question needs an answer/)).not.toBeNull();
  expect(screen.getByRole('button',{name:'Review task changes'})).toHaveProperty('disabled',false);
  expect(screen.getByRole('button',{name:'Task result and questions'})).toHaveProperty('disabled',false);expect(document.activeElement).toBe(focus);
  await tick(30_000);expect(get).toHaveBeenCalledOnce();expect(control).not.toHaveBeenCalled();expect(reserve).not.toHaveBeenCalled();expect(create).not.toHaveBeenCalled();
});

it.each(['captured','missing','invalid','unavailable'] as const)('stops terminal refresh after the canonical %s report appears',async status=>{
  const initial=active();initial.operations[0].status='review-required';
  const get=vi.spyOn(projectRunsApi,'get').mockResolvedValueOnce(initial).mockResolvedValue(finished(initial,status));render(<Harness initial={initial}/>);
  await tick();await tick();await tick(30_000);expect(get).toHaveBeenCalledTimes(2);
});

it('does not poll an unstarted reservation or legacy terminal output without a modern pending report',async()=>{
  const initial=active();initial.operations[0].status='reserved';const get=vi.spyOn(projectRunsApi,'get');const view=render(<ProjectSwarm {...props()} run={initial}/>);
  await tick(30_000);const legacy=active();legacy.operations[0].status='review-required';delete legacy.operations[0].dependencyContext;
  view.rerender(<ProjectSwarm {...props()} run={legacy}/>);await tick(30_000);expect(get).not.toHaveBeenCalled();
});

it.each(['unstarted','absence-retired','launch-retired'] as const)('does not wait indefinitely for a report from a cancelled %s attempt',async kind=>{
  const initial=active();initial.operations[0].status='cancelled';
  if(kind==='absence-retired')initial.operations[0].absenceRetirement={childId:'child-a',leaseId:'lease-a',backend:'docker',observedAt:initial.updatedAt,inventoryObservedAt:initial.updatedAt,finalizedAt:initial.updatedAt};
  if(kind==='launch-retired')initial.operations[0].recovery={state:'finalized',requestedAt:initial.updatedAt,provedAt:initial.updatedAt,finalizedAt:initial.updatedAt};
  const get=vi.spyOn(projectRunsApi,'get');render(<Harness initial={initial}/>);
  await tick(60_000);expect(get).not.toHaveBeenCalled();expect(screen.getByRole('button',{name:'Check team status'})).toHaveProperty('disabled',false);
});

it('keeps one read in flight, cancels on hidden, and recovers on the first visible interval',async()=>{
  let finish!:(run:ProjectRun)=>void;const get=vi.spyOn(projectRunsApi,'get').mockReturnValueOnce(new Promise(resolve=>{finish=resolve;})).mockResolvedValue(finished());
  render(<Harness/>);await tick();const signal=get.mock.calls[0][2]!;await tick(30_000);expect(get).toHaveBeenCalledOnce();
  vi.spyOn(document,'visibilityState','get').mockReturnValue('hidden');fireEvent(document,new Event('visibilitychange'));expect(signal.aborted).toBe(true);
  await act(async()=>finish(finished()));await tick(30_000);expect(screen.getByText('Recorded as running')).not.toBeNull();
  vi.spyOn(document,'visibilityState','get').mockReturnValue('visible');fireEvent(document,new Event('visibilitychange'));await tick(4999);expect(get).toHaveBeenCalledOnce();
  await tick(1);expect(get).toHaveBeenCalledTimes(2);expect(screen.getByText('Awaiting review')).not.toBeNull();
});

it('aborts pending observation while exact integration consent is open and resumes after close',async()=>{
  const initial=active();initial.operations[0].status='review-required';let finish!:(run:ProjectRun)=>void;
  const get=vi.spyOn(projectRunsApi,'get').mockReturnValueOnce(new Promise(resolve=>{finish=resolve;})).mockResolvedValue(finished(initial));
  vi.spyOn(projectSwarmApi,'review').mockResolvedValue({taskId:'a',operationId:'op-a',sourceCommit:'b'.repeat(40),targetCommit:'a'.repeat(40),reviewDigest:'d'.repeat(64),paths:[],patch:'',canIntegrate:true,reason:'Review fixture'});
  render(<Harness initial={initial}/>);await tick();
  await act(async()=>fireEvent.click(screen.getByRole('button',{name:'Review task changes'})));
  const dialog=screen.getByRole('dialog',{name:'Review task changes'}),checkbox=within(dialog).getByRole('checkbox');fireEvent.click(checkbox);
  expect(get.mock.calls[0][2]!.aborted).toBe(true);await act(async()=>finish(finished(initial)));await tick(30_000);
  expect(checkbox).toHaveProperty('checked',true);expect(screen.getByRole('dialog',{name:'Review task changes'})).toBe(dialog);expect(get).toHaveBeenCalledOnce();
  fireEvent.click(within(dialog).getByRole('button',{name:'Keep reviewing'}));await tick();expect(get).toHaveBeenCalledTimes(2);expect(screen.getByText(/1 agent-reported question needs an answer/)).not.toBeNull();
});

it.each(['scope','read','unmount'] as const)('cancels and discards a late private response after %s changes',async change=>{
  const initial=active(),p=props();let finish!:(run:ProjectRun)=>void;const get=vi.spyOn(projectRunsApi,'get').mockReturnValue(new Promise(resolve=>{finish=resolve;}));
  const view=render(<ProjectSwarm {...p} run={initial}/>);await tick();const signal=get.mock.calls[0][2]!;
  if(change==='read'){access.canRead=false;view.rerender(<ProjectSwarm {...p} run={initial}/>);}
  else if(change==='scope')view.rerender(<ProjectSwarm {...p} run={{...initial,id:'another-run',projectId:'another-project'}}/>);
  else view.unmount();
  expect(signal.aborted).toBe(true);await act(async()=>finish(finished(initial)));expect(p.onUpdate).not.toHaveBeenCalled();
});

it('never replaces a newer visible revision with a late recorded read',async()=>{
  const initial=active(),p=props();let finish!:(run:ProjectRun)=>void;vi.spyOn(projectRunsApi,'get').mockReturnValue(new Promise(resolve=>{finish=resolve;}));
  const view=render(<ProjectSwarm {...p} run={initial}/>);await tick();view.rerender(<ProjectSwarm {...p} run={{...initial,revision:10}}/>);
  await act(async()=>finish(finished(initial)));expect(p.onUpdate).not.toHaveBeenCalled();
});

it('backs off between transient failures and stops repeating a confirmed permission refusal',async()=>{
  const get=vi.spyOn(projectRunsApi,'get').mockRejectedValueOnce(new Error('Offline')).mockRejectedValue(new ApiError('conflict','Access refused',403));
  render(<Harness/>);await tick();expect(screen.getByText(/Automatic status refresh is unavailable/)).not.toBeNull();await tick(4999);expect(get).toHaveBeenCalledOnce();
  await tick(1);expect(screen.getByText(/Automatic status refresh was refused/)).not.toBeNull();await tick(30_000);expect(get).toHaveBeenCalledTimes(2);
});

it('does not carry a previous run refresh warning into another selected run',async()=>{
  const initial=active(),p=props();vi.spyOn(projectRunsApi,'get').mockRejectedValue(new Error('Offline'));
  const view=render(<ProjectSwarm {...p} run={initial}/>);await tick();expect(screen.getByText(/Automatic status refresh is unavailable/)).not.toBeNull();
  view.rerender(<ProjectSwarm {...p} run={{...initial,id:'another-run',projectId:'another-project'}}/>);
  expect(screen.queryByText(/Automatic status refresh is unavailable/)).toBeNull();
});


it('does not wait for a report from a positively recorded pre-launch failure and preserves explicit retirement',async()=>{
  const initial=active();initial.operations[0].status='failed';initial.operations[0].startFailure={stage:'admission',reasonCode:'capacity_unavailable',message:'Admission refused the start before worker creation.'};
  const get=vi.spyOn(projectRunsApi,'get'),reserve=vi.spyOn(projectSwarmApi,'reserve'),create=vi.spyOn(api,'createSession');
  render(<Harness initial={initial}/>);await tick(60_000);
  expect(screen.getByText('Start blocked')).toBeTruthy();expect(screen.getByText(/Admission refused/)).toBeTruthy();
  expect(screen.queryByRole('button',{name:'Review task changes'})).toBeNull();expect(screen.queryByRole('button',{name:'Task result and questions'})).toBeNull();
  const task=screen.getByRole('heading',{name:'Build a'}).closest('li')!;
  fireEvent.click(within(task).getByText('More task actions'));
  expect(within(task).getByRole('button',{name:'Inspect workspace output'})).toBeTruthy();
  expect(within(task).getByRole('button',{name:'Retain changes and retry'})).toBeTruthy();
  expect(within(task).queryByRole('button',{name:'Retry in a new workspace'})).toBeNull();
  expect(get).not.toHaveBeenCalled();expect(reserve).not.toHaveBeenCalled();expect(create).not.toHaveBeenCalled();
});

it('keeps a launched worker failure reviewable and waits for its canonical output report',async()=>{
  const initial=active();initial.operations[0].status='failed';
  const result=finished(initial,'missing');result.operations[0].status='failed';
  const get=vi.spyOn(projectRunsApi,'get').mockResolvedValue(result);
  render(<Harness initial={initial}/>);
  expect(screen.getByRole('button',{name:'Review task changes'})).toBeTruthy();expect(screen.getByRole('button',{name:'Task result and questions'})).toBeTruthy();
  await tick();await tick(30_000);expect(get).toHaveBeenCalledOnce();
});
