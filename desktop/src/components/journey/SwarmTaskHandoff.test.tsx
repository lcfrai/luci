import {useState} from 'react';
import {act, fireEvent, render, screen, waitFor, within} from '@testing-library/react';
import userEvent from '@testing-library/user-event';
import {beforeEach, expect, it, vi} from 'vitest';
import {api} from '../../api';
import {ApiError, type ProblemCode} from '../../api/client';
import {projectRunsApi, type ProjectRun} from '../../api/projectRuns';
import {projectSwarmApi} from '../../api/projectSwarm';
import {swarmHandoffApi, type TaskHandoffMutation, type TaskHandoffView} from '../../api/swarmHandoff';
import {resetCockpit, useCockpit} from '../../store/store';
import {handoffAnswerResult, handoffFixture} from '../../test/handoffFixtures';
import {swarmHarnessFixture} from '../../test/swarmFixtures';
import {SwarmTaskHandoff} from './SwarmTaskHandoff';
import {ProjectSwarm} from './ProjectSwarm';
import {RecordedRunAttention} from '../common/RecordedRunAttention';

const access=vi.hoisted(()=>({scope:'account-A/workspace-A',canRead:true,canEdit:true,canExecute:false,loading:false,error:''}));
vi.mock('../../hooks/useProjectDraft',()=>({useProjectDraftScope:()=>access.scope}));
vi.mock('../../hooks/useProjectAccess',()=>({useProjectAccess:()=>({...access,message:'Checking project permissions…'})}));
vi.mock('./SwarmRunQueue',()=>({SwarmRunQueue:()=>null}));
function pending<T>() {let resolve!:(value:T)=>void;return {promise:new Promise<T>(done=>{resolve=done;}),resolve};}
const props=()=>{const f=handoffFixture();return {projectId:f.run.projectId,projectName:'North star',taskTitle:'Shared settings',run:f.run,operation:f.operation,canRead:true,canEdit:true,onRun:vi.fn(),onClose:vi.fn()};};
function Harness({initial=handoffFixture().run,onRun=vi.fn(),canRead=true,canEdit=true}:{initial?:ProjectRun;onRun?:(run:ProjectRun)=>void;canRead?:boolean;canEdit?:boolean}) {
  const [run,setRun]=useState(initial);
  return <SwarmTaskHandoff {...props()} run={run} operation={run.operations[0]} canRead={canRead} canEdit={canEdit} onRun={next=>{setRun(next);onRun(next);}}/>;
}
beforeEach(()=>{
  vi.restoreAllMocks();resetCockpit();Object.assign(access,{scope:'account-A/workspace-A',canRead:true,canEdit:true,canExecute:false,loading:false,error:''});
  const f=handoffFixture();
  vi.spyOn(swarmHandoffApi,'get').mockResolvedValue(f.handoff);
  vi.spyOn(swarmHandoffApi,'collect').mockResolvedValue({run:{...f.run,revision:2},handoff:f.handoff});
  vi.spyOn(swarmHandoffApi,'answer').mockImplementation(async(run,_op,_digest,_question,answer)=>handoffAnswerResult(run,f.handoff,answer));
  vi.spyOn(projectRunsApi,'get').mockResolvedValue(f.run);
});
async function draft(answer='Exclude archived projects.') {
  await userEvent.click(await screen.findByRole('button',{name:'Answer question'}));
  await userEvent.type(screen.getByRole('textbox',{name:'Your answer'}),answer);
}

it('reads inert worker claims separately from recorded evidence without capture, execution or integration',async()=>{
  const f=handoffFixture(),hostile='<img src="https://invalid.example/private" onerror="alert(1)">';f.handoff.receipt!.reported!.summary=hostile;
  vi.mocked(swarmHandoffApi.get).mockResolvedValue(f.handoff);
  const create=vi.spyOn(api,'createSession'),integrate=vi.spyOn(projectSwarmApi,'integrate');
  render(<Harness/>);await screen.findByText(hostile);
  expect(within(screen.getByRole('region',{name:'Agent-reported checks'})).getByText('Reported passed')).not.toBeNull();
  expect(within(screen.getByRole('region',{name:'Recorded verification evidence'})).getByText('No recorded checks match this task’s captured source.')).not.toBeNull();
  expect(screen.getByRole('dialog').querySelector('img,iframe')).toBeNull();
  expect(screen.getByText(/Agent flagged this as blocking/)).not.toBeNull();
  expect(swarmHandoffApi.collect).not.toHaveBeenCalled();expect(swarmHandoffApi.answer).not.toHaveBeenCalled();expect(create).not.toHaveBeenCalled();expect(integrate).not.toHaveBeenCalled();
});
it.each(['not-recorded','missing','invalid','unavailable'] as const)('explains %s without inventing output or questions',async status=>{
  const f=handoffFixture();vi.mocked(swarmHandoffApi.get).mockResolvedValue({...f.handoff,status,receipt:status==='not-recorded'?null:{...f.handoff.receipt!,status,reported:null},answers:[],questionIds:[]});
  render(<Harness/>);await screen.findByText('No recorded checks match this task’s captured source.');
  expect(screen.queryByRole('region',{name:'Agent questions'})).toBeNull();expect(screen.queryByText('Reported passed')).toBeNull();expect(swarmHandoffApi.collect).not.toHaveBeenCalled();
});
it('requires explicit capture, updates the run receipt, and never treats capture as a passing check',async()=>{
  const changed=vi.fn();render(<Harness onRun={changed}/>);
  await userEvent.click(await screen.findByRole('button',{name:'Capture report again'}));
  await screen.findByText(/Task report capture recorded/);
  expect(swarmHandoffApi.collect).toHaveBeenCalledExactlyOnceWith(handoffFixture().run,handoffFixture().operation);
  expect(changed).toHaveBeenCalledWith(expect.objectContaining({revision:2}));
  expect(screen.getByText('No recorded checks match this task’s captured source.')).not.toBeNull();
});
it('allows an editor without Execute to answer once, preserve the draft, and amend the saved answer',async()=>{
  render(<Harness/>);await draft();
  expect(screen.getByRole('button',{name:'Answer question'})).toHaveProperty('disabled',true);
  expect(Object.values(useCockpit.getState().unsavedProjectDrafts).map(item=>item.label)).toContain('Task answer');
  await userEvent.click(screen.getByRole('button',{name:'Answer question'}));
  expect(screen.getByRole('textbox',{name:'Your answer'})).toHaveProperty('value','Exclude archived projects.');
  await userEvent.click(screen.getByRole('button',{name:'Save answer'}));
  await screen.findByText('Recorded answer');expect(swarmHandoffApi.answer).toHaveBeenCalledTimes(1);
  expect(screen.getByText('Originally flagged as blocking by the agent.').className).toBe('handoff-muted');
  expect(screen.queryByText('Agent flagged this as blocking.')).toBeNull();
  const f=handoffFixture();expect(swarmHandoffApi.answer).toHaveBeenLastCalledWith(f.run,f.operation,f.handoff.digest,f.handoff.questionIds[0],'Exclude archived projects.');
  expect(Object.values(useCockpit.getState().unsavedProjectDrafts)).toHaveLength(0);
  await userEvent.click(screen.getByRole('button',{name:'Amend recorded answer'}));
  expect(screen.getByRole('textbox',{name:'Your answer'})).toHaveProperty('value','Exclude archived projects.');
  await userEvent.clear(screen.getByRole('textbox',{name:'Your answer'}));await userEvent.type(screen.getByRole('textbox',{name:'Your answer'}),'Include archived projects after all.');
  await userEvent.click(screen.getByRole('button',{name:'Save answer'}));await screen.findByText('Include archived projects after all.');
  expect(swarmHandoffApi.answer).toHaveBeenCalledTimes(2);expect(vi.mocked(swarmHandoffApi.answer).mock.calls[1][0].revision).toBe(2);
});
it('prevents accidental close/navigation loss of a changed answer and requires explicit discard',async()=>{
  const p=props(),view=render(<SwarmTaskHandoff {...p}/>);await draft('Keep this decision.');
  await userEvent.click(screen.getByRole('button',{name:'Close task result'}));
  expect(p.onClose).not.toHaveBeenCalled();await screen.findByText('Discard this unsaved answer?');
  await userEvent.click(screen.getByRole('button',{name:'Keep editing'}));expect(screen.getByRole('textbox',{name:'Your answer'})).toHaveProperty('value','Keep this decision.');
  await userEvent.click(screen.getByRole('button',{name:'Close task result'}));await userEvent.click(screen.getByRole('button',{name:'Discard draft and close'}));
  expect(p.onClose).toHaveBeenCalledOnce();view.unmount();expect(Object.values(useCockpit.getState().unsavedProjectDrafts)).toHaveLength(0);
});
it('blocks oversized UTF-8 answers rather than relying on the browser character limit',async()=>{
  render(<Harness/>);await draft('a');fireEvent.change(screen.getByRole('textbox',{name:'Your answer'}),{target:{value:'🙂'.repeat(1001)}});
  expect(screen.getByRole('button',{name:'Save answer'})).toHaveProperty('disabled',true);expect(screen.getByText(/4,004 \/ 4,000 UTF-8 bytes/)).not.toBeNull();expect(swarmHandoffApi.answer).not.toHaveBeenCalled();
});
it('holds a lost answer response for explicit fresh review, preserves text and obtains the next CAS revision',async()=>{
  const f=handoffFixture();vi.mocked(swarmHandoffApi.answer).mockRejectedValueOnce(new Error('Connection lost.'));
  render(<Harness/>);await draft();await userEvent.click(screen.getByRole('button',{name:'Save answer'}));
  await screen.findByText(/save outcome is unconfirmed/);expect(swarmHandoffApi.answer).toHaveBeenCalledOnce();expect(swarmHandoffApi.get).toHaveBeenCalledOnce();expect(projectRunsApi.get).not.toHaveBeenCalled();
  expect(screen.getByRole('textbox',{name:'Your answer'})).toHaveProperty('value','Exclude archived projects.');expect(screen.getByRole('button',{name:'Save answer'})).toHaveProperty('disabled',true);
  vi.mocked(projectRunsApi.get).mockResolvedValue({...f.run,revision:5});
  await userEvent.click(screen.getByRole('button',{name:'Refresh task result'}));
  await waitFor(()=>expect(screen.getByRole('button',{name:'Save answer'})).toHaveProperty('disabled',false));
  expect(swarmHandoffApi.answer).toHaveBeenCalledOnce();await userEvent.click(screen.getByRole('button',{name:'Save answer'}));await screen.findByText('Recorded answer');
  expect(vi.mocked(swarmHandoffApi.answer).mock.calls[1][0].revision).toBe(5);
});
it('keeps an outdated draft after report replacement but never saves it against another question digest',async()=>{
  const f=handoffFixture();render(<Harness/>);await draft('My unsent decision.');
  const newer={...f.handoff,digest:'8'.repeat(64),questionIds:['9'.repeat(64)]};vi.mocked(swarmHandoffApi.get).mockResolvedValue(newer);vi.mocked(projectRunsApi.get).mockResolvedValue({...f.run,revision:2});
  await userEvent.click(screen.getByRole('button',{name:'Refresh task result'}));await screen.findByText(/build revision 2/);
  expect(screen.getByRole('textbox',{name:'Your answer'})).toHaveProperty('value','My unsent decision.');expect(screen.getByRole('button',{name:'Save answer'})).toHaveProperty('disabled',true);expect(swarmHandoffApi.answer).not.toHaveBeenCalled();
});
it('requires deliberate amendment when another editor changes the answer and preserves both versions',async()=>{
  const f=handoffFixture();render(<Harness/>);await draft('My preferred scope.');
  const newer=handoffAnswerResult(f.run,f.handoff,'Another editor chose a different scope.');
  vi.mocked(swarmHandoffApi.get).mockResolvedValue(newer.handoff);vi.mocked(projectRunsApi.get).mockResolvedValue(newer.run);
  await userEvent.click(screen.getByRole('button',{name:'Refresh task result'}));await screen.findByText('Another editor chose a different scope.');
  expect(screen.getByRole('textbox',{name:'Your answer'})).toHaveProperty('value','My preferred scope.');expect(screen.getByRole('button',{name:'Save answer'})).toHaveProperty('disabled',true);
  await userEvent.click(screen.getByRole('button',{name:'Use draft to amend current answer'}));
  expect(screen.getByRole('button',{name:'Save answer'})).toHaveProperty('disabled',false);expect(swarmHandoffApi.answer).not.toHaveBeenCalled();
  await userEvent.click(screen.getByRole('button',{name:'Save answer'}));await screen.findByText('My preferred scope.');expect(vi.mocked(swarmHandoffApi.answer).mock.calls[0][0].revision).toBe(newer.run.revision);
});
it('recognizes an answer saved before its response was lost without sending it again',async()=>{
  const f=handoffFixture();vi.mocked(swarmHandoffApi.answer).mockRejectedValue(new Error('Response lost.'));
  render(<Harness/>);await draft();await userEvent.click(screen.getByRole('button',{name:'Save answer'}));await screen.findByText(/save outcome is unconfirmed/);
  const saved=handoffAnswerResult(f.run,f.handoff);vi.mocked(swarmHandoffApi.get).mockResolvedValue(saved.handoff);vi.mocked(projectRunsApi.get).mockResolvedValue(saved.run);
  await userEvent.click(screen.getByRole('button',{name:'Refresh task result'}));await screen.findByText('Your draft matches the answer now recorded. No further save is needed.');
  await userEvent.click(screen.getByRole('button',{name:'Dismiss already recorded draft'}));expect(screen.queryByRole('textbox',{name:'Your answer'})).toBeNull();expect(swarmHandoffApi.answer).toHaveBeenCalledOnce();
});
it.each(['scope','read','parent-read','unmount'] as const)('fences late private reads when %s changes',async change=>{
  const delayed=pending<TaskHandoffView>(),next=pending<TaskHandoffView>();vi.mocked(swarmHandoffApi.get).mockReturnValueOnce(delayed.promise).mockReturnValue(next.promise);
  const p=props(),view=render(<SwarmTaskHandoff {...p}/>);await waitFor(()=>expect(swarmHandoffApi.get).toHaveBeenCalledOnce());
  const signal=vi.mocked(swarmHandoffApi.get).mock.calls[0][3]!;
  if(change==='scope')access.scope='another-account';if(change==='read')access.canRead=false;
  if(change==='unmount')view.unmount();else view.rerender(<SwarmTaskHandoff {...p} canRead={change!=='parent-read'}/>);
  expect(signal.aborted).toBe(true);await act(async()=>delayed.resolve(handoffFixture().handoff));
  expect(screen.queryByText('Added the shared settings view.')).toBeNull();
  expect(swarmHandoffApi.collect).not.toHaveBeenCalled();expect(swarmHandoffApi.answer).not.toHaveBeenCalled();
});
it.each([401,403])('clears prior private report and draft after a confirmed read denial %s',async status=>{
  render(<Harness/>);await draft('Private draft.');vi.mocked(swarmHandoffApi.get).mockRejectedValue(new ApiError('permission_denied' as ProblemCode,'Access refused.',status));
  await userEvent.click(screen.getByRole('button',{name:'Refresh task result'}));await screen.findByText('Access refused.');
  expect(screen.queryByText('Added the shared settings view.')).toBeNull();expect(screen.queryByRole('textbox',{name:'Your answer'})).toBeNull();expect(Object.values(useCockpit.getState().unsavedProjectDrafts)).toHaveLength(0);
});
it('clears prior private report after a confirmed answer denial without automatic retry',async()=>{
  vi.mocked(swarmHandoffApi.answer).mockRejectedValue(new ApiError('permission_denied' as ProblemCode,'Access refused.',403));
  render(<Harness/>);await draft();await userEvent.click(screen.getByRole('button',{name:'Save answer'}));await screen.findByText(/Project access was refused/);
  expect(screen.queryByText('Added the shared settings view.')).toBeNull();expect(screen.queryByRole('textbox',{name:'Your answer'})).toBeNull();expect(swarmHandoffApi.answer).toHaveBeenCalledOnce();
});
it('ignores a late mutation after Edit is revoked and regranted, keeps the draft and requires fresh review',async()=>{
  const delayed=pending<TaskHandoffMutation>(),changed=vi.fn();vi.mocked(swarmHandoffApi.answer).mockReturnValue(delayed.promise);
  const view=render(<Harness onRun={changed}/>);await draft();await userEvent.click(screen.getByRole('button',{name:'Save answer'}));
  access.canEdit=false;view.rerender(<Harness onRun={changed}/>);access.canEdit=true;view.rerender(<Harness onRun={changed}/>);
  const f=handoffFixture();await act(async()=>delayed.resolve(handoffAnswerResult(f.run,f.handoff,'Old authority answer.')));
  await screen.findByText(/Project access or the build changed while saving/);
  expect(changed).not.toHaveBeenCalled();expect(screen.queryByText('Old authority answer.')).toBeNull();expect(screen.getByRole('textbox',{name:'Your answer'})).toHaveProperty('value','Exclude archived projects.');expect(screen.getByRole('button',{name:'Save answer'})).toHaveProperty('disabled',true);expect(swarmHandoffApi.answer).toHaveBeenCalledOnce();
});
it('cannot double-submit while pending and clears no unsent answer before success',async()=>{
  const delayed=pending<TaskHandoffMutation>();vi.mocked(swarmHandoffApi.answer).mockReturnValue(delayed.promise);
  render(<Harness/>);await draft();const save=screen.getByRole('button',{name:'Save answer'});fireEvent.click(save);fireEvent.click(save);
  expect(swarmHandoffApi.answer).toHaveBeenCalledOnce();expect(screen.getByRole('button',{name:'Close task result'})).toHaveProperty('disabled',true);
  const f=handoffFixture();await act(async()=>delayed.resolve(handoffAnswerResult(f.run,f.handoff)));await screen.findByText('Recorded answer');
});
it.each(['active','integrated','recovery','read-only'] as const)('blocks capture for %s without blocking authorized integrated answers',async condition=>{
  const f=handoffFixture();if(condition==='active')f.operation.status='running';if(condition==='recovery')f.operation.recovery={state:'pending',requestedAt:f.run.createdAt};if(condition==='read-only')access.canEdit=false;
  if(condition==='integrated')f.handoff.integration={state:'integrated',sourceCommit:'a'.repeat(40),targetCommit:'b'.repeat(40),reviewDigest:'c'.repeat(64)};
  vi.mocked(swarmHandoffApi.get).mockResolvedValue(f.handoff);render(<Harness initial={f.run}/>);
  const capture=await screen.findByRole('button',{name:'Capture report again'});expect(capture).toHaveProperty('disabled',true);await userEvent.click(capture);expect(swarmHandoffApi.collect).not.toHaveBeenCalled();
  expect(screen.getByRole('button',{name:'Answer question'})).toHaveProperty('disabled',condition==='read-only');
});
it('opens the exact parent attempt only on deliberate review, with Edit but no Execute, and closes on lost Read',async()=>{
  const f=handoffFixture();useCockpit.getState().setHarnesses([swarmHarnessFixture()]);
  const p={run:f.run,canEdit:true,canExecute:false,busy:false,onBusy:vi.fn(),onUpdate:vi.fn(),onOpenSession:vi.fn()};
  const view=render(<ProjectSwarm {...p}/>);expect(swarmHandoffApi.get).not.toHaveBeenCalled();
  await userEvent.click(screen.getByRole('button',{name:'Task result and questions'}));await screen.findByText('Added the shared settings view.');
  expect(swarmHandoffApi.get).toHaveBeenCalledWith(f.run.projectId,f.run.id,f.operation,expect.any(AbortSignal));
  expect(screen.getByRole('button',{name:'Answer question'})).toHaveProperty('disabled',false);
  access.canRead=false;view.rerender(<ProjectSwarm {...p}/>);expect(screen.queryByRole('dialog')).toBeNull();expect(screen.queryByText('Added the shared settings view.')).toBeNull();
});
it('follows Home review work to visible earlier questions and opens the retained report by its exact identity',async()=>{
  const f=handoffFixture();f.operation.attempt=2;
  f.operation.handoff={status:'captured',digest:f.handoff.digest!,summary:'Worker claim',questions:1,unanswered:1,capturedAt:f.run.createdAt};
  const previous={...f.operation,id:'previous-op',childId:'previous-child',worktreeId:'previous-tree',attempt:1,status:'cancelled'};
  f.run.operations.unshift(previous);useCockpit.getState().setHarnesses([swarmHarnessFixture()]);
  vi.mocked(swarmHandoffApi.get).mockImplementation(async(_p,_r,op)=>({...f.handoff,operationId:op.id,receipt:{...f.handoff.receipt!,operationId:op.id,childId:op.childId,worktreeId:op.worktreeId!,reported:{...f.handoff.receipt!.reported!,summary:'Earlier attempt report'}}}));
  function HomeReview() {
    const [selected,setSelected]=useState('');
    return selected===f.run.id
      ? <ProjectSwarm run={f.run} canEdit canExecute={false} busy={false} onBusy={vi.fn()} onUpdate={vi.fn()} onOpenSession={vi.fn()}/>
      : <RecordedRunAttention compact initialLimit={1} items={[{id:f.run.id,state:'cancelled',kind:'task-question',contextStatus:'current'}]} onInspect={setSelected}/>;
  }
  render(<HomeReview/>);
  await userEvent.click(screen.getByRole('button',{name:`Inspect ${f.run.id}: Review reported task questions`}));
  expect(screen.getByText(/1 agent-reported question needs an answer/)).not.toBeNull();
  const cue=screen.getByText(/1 agent-reported question remains unanswered in earlier attempts/);
  expect(cue.closest('details')).toBeNull();
  const summary=screen.getAllByText('Task scope and handoff')[0];
  expect(summary.closest('details')).toHaveProperty('open',false);
  expect(swarmHandoffApi.get).not.toHaveBeenCalled();
  await userEvent.click(screen.getByRole('button',{name:'View earlier attempts'}));
  expect(summary.closest('details')).toHaveProperty('open',true);expect(document.activeElement).toBe(summary);
  expect(within(summary.closest('details')!).getByText(/1 agent-reported question remains unanswered/)).not.toBeNull();
  await userEvent.click(screen.getByRole('button',{name:'Result and questions for attempt 1'}));await screen.findByText('Earlier attempt report');
  expect(swarmHandoffApi.get).toHaveBeenCalledExactlyOnceWith(f.run.projectId,f.run.id,previous,expect.any(AbortSignal));expect(swarmHandoffApi.collect).not.toHaveBeenCalled();expect(swarmHandoffApi.answer).not.toHaveBeenCalled();
});
