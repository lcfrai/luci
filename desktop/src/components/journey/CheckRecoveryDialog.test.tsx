import {useState} from 'react';
import {act,render,screen,waitFor} from '@testing-library/react';
import userEvent from '@testing-library/user-event';
import {beforeEach,expect,it,vi} from 'vitest';
import {productApi,type CheckRun} from '../../api/product';
import {ApiError,type ProblemCode} from '../../api/client';
import {checkRecoveryFixture,checkRunFixture} from '../../test/checkFixtures';
import {CheckRecoveryDialog} from './CheckRecoveryDialog';

const initial=checkRunFixture();
function Harness({canRead=true,canEdit=true}:{canRead?:boolean;canEdit?:boolean}){const[run,setRun]=useState(initial);return <CheckRecoveryDialog run={run} projectName="Alpha" canRead={canRead} canEdit={canEdit} onUpdate={setRun} onClose={vi.fn()} getCloseFocusTarget={()=>null}/>;}
beforeEach(()=>{vi.restoreAllMocks();vi.spyOn(productApi,'checkRun').mockResolvedValue(initial);vi.spyOn(productApi,'checkRecovery').mockResolvedValue(checkRecoveryFixture());});
it('requires exact check consent and makes one recovery request without starting commands',async()=>{
  const saved=checkRunFixture({revision:5,status:'failed',resourcesHeld:false}),start=vi.spyOn(productApi,'runChecks');
  const recover=vi.spyOn(productApi,'recoverCheck').mockImplementation(async()=>{vi.mocked(productApi.checkRun).mockResolvedValue(saved);vi.mocked(productApi.checkRecovery).mockResolvedValue(checkRecoveryFixture({revision:5,state:'complete',method:null,commandIndex:null,recoveryDigest:null}));return saved;});
  render(<Harness/>);const consent=await screen.findByRole('checkbox');const confirm=screen.getByRole('button',{name:'Recover this exact check'});
  expect(confirm).toHaveProperty('disabled',true);expect(recover).not.toHaveBeenCalled();
  await userEvent.click(consent);await userEvent.dblClick(confirm);
  await screen.findByText(/Recovery is recorded and this check no longer holds capacity/);
  expect(recover).toHaveBeenCalledExactlyOnceWith('p-1','check-a',checkRecoveryFixture());expect(start).not.toHaveBeenCalled();
});
it.each(['remove-exact','fence-launch'] as const)('keeps the %s effect and pending finalization explicit',async method=>{
  vi.mocked(productApi.checkRecovery).mockResolvedValue(checkRecoveryFixture({state:'pending',method}));render(<Harness/>);
  await screen.findByRole('checkbox');expect(screen.getByRole('button',{name:'Finish reviewed recovery'})).toHaveProperty('disabled',true);
  if(method==='fence-launch')expect(screen.getByText(/small permanent Docker recovery marker/)).toBeTruthy();else expect(screen.getByText(/only the exact recorded container/)).toBeTruthy();
});
it('keeps a null-method pending review read-only',async()=>{
  vi.mocked(productApi.checkRecovery).mockResolvedValue(checkRecoveryFixture({state:'pending',method:null,commandIndex:null,recoveryDigest:null}));render(<Harness/>);
  await screen.findByText(/Capacity remains held/);expect(screen.queryByRole('checkbox')).toBeNull();expect(screen.getByRole('button',{name:'Finish reviewed recovery'})).toHaveProperty('disabled',true);
});
it('refreshes after a lost recovery receipt but never resends or restores consent',async()=>{
  const saved=checkRunFixture({revision:5});const recover=vi.spyOn(productApi,'recoverCheck').mockImplementation(async()=>{vi.mocked(productApi.checkRun).mockResolvedValue(saved);vi.mocked(productApi.checkRecovery).mockResolvedValue(checkRecoveryFixture({revision:5,state:'pending'}));throw Error('Connection interrupted');});
  render(<Harness/>);await userEvent.click(await screen.findByRole('checkbox'));await userEvent.click(screen.getByRole('button',{name:'Recover this exact check'}));
  await screen.findByText(/No recovery request or command was repeated/);await waitFor(()=>expect(screen.getByRole('button',{name:'Finish reviewed recovery'})).toHaveProperty('disabled',true));
  expect(recover).toHaveBeenCalledOnce();expect(screen.getByRole('checkbox')).toHaveProperty('checked',false);
});
it('invalidates consent when the parent records a newer check revision',async()=>{
  const props={projectName:'Alpha',canRead:true,canEdit:true,onUpdate:vi.fn(),onClose:vi.fn(),getCloseFocusTarget:()=>null};
  const view=render(<CheckRecoveryDialog {...props} run={initial}/>);await userEvent.click(await screen.findByRole('checkbox'));
  view.rerender(<CheckRecoveryDialog {...props} run={{...initial,revision:5}}/>);
  expect(screen.getByText(/recorded check changed/)).toBeTruthy();expect(screen.getByRole('checkbox')).toHaveProperty('checked',false);expect(screen.getByRole('button',{name:'Recover this exact check'})).toHaveProperty('disabled',true);
});
it('discards a late recovery read after reading access is revoked',async()=>{
  let finish!:(run:CheckRun)=>void;vi.mocked(productApi.checkRun).mockReturnValue(new Promise(resolve=>{finish=resolve;}));const view=render(<Harness/>);
  view.rerender(<Harness canRead={false}/>);await act(async()=>finish(initial));
  expect(screen.queryByRole('checkbox')).toBeNull();expect(screen.queryByText('Unit tests')).toBeNull();expect(productApi.checkRecovery).not.toHaveBeenCalled();
});
it('allows inspection without Edit but does not expose recovery consent authority',async()=>{
  const recover=vi.spyOn(productApi,'recoverCheck');render(<Harness canEdit={false}/>);const consent=await screen.findByRole('checkbox');expect(consent).toHaveProperty('disabled',true);expect(screen.getByRole('button',{name:'Recover this exact check'})).toHaveProperty('disabled',true);expect(recover).not.toHaveBeenCalled();
});

it('allows closing a slow read and aborts it without a second read or recovery write',async()=>{
  let finish!:(run:CheckRun)=>void;
  vi.mocked(productApi.checkRun).mockReturnValue(new Promise(resolve=>{finish=resolve;}));
  const recover=vi.spyOn(productApi,'recoverCheck');
  function ClosingView(){const[open,setOpen]=useState(true);return open?<CheckRecoveryDialog run={initial} projectName="Alpha" canRead canEdit onUpdate={vi.fn()} onClose={()=>setOpen(false)} getCloseFocusTarget={()=>null}/>:<p>Check history</p>;}
  render(<ClosingView/>);
  await waitFor(()=>expect(productApi.checkRun).toHaveBeenCalledOnce());
  const signal=vi.mocked(productApi.checkRun).mock.calls[0][2];
  await userEvent.click(screen.getByRole('button',{name:'Keep inspecting'}));
  expect(screen.queryByRole('dialog')).toBeNull();expect(signal?.aborted).toBe(true);
  await act(async()=>finish(initial));
  expect(productApi.checkRecovery).not.toHaveBeenCalled();expect(recover).not.toHaveBeenCalled();
});

it('does not interpret an Edit-only POST refusal as loss of reading access',async()=>{
  const denied=vi.fn();vi.spyOn(productApi,'recoverCheck').mockRejectedValue(new ApiError('permission_denied' as ProblemCode,'Editing access changed.',403));
  render(<CheckRecoveryDialog run={initial} projectName="Alpha" canRead canEdit onUpdate={vi.fn()} onReadDenied={denied} onClose={vi.fn()} getCloseFocusTarget={()=>null}/>);
  await userEvent.click(await screen.findByRole('checkbox'));await userEvent.click(screen.getByRole('button',{name:'Recover this exact check'}));
  await screen.findByText('Editing access changed.');expect(denied).not.toHaveBeenCalled();
  expect(productApi.checkRun).toHaveBeenCalledOnce();expect(productApi.checkRecovery).toHaveBeenCalledOnce();expect(productApi.recoverCheck).toHaveBeenCalledOnce();
});

it('keeps pending recovery details keyboard reachable without enabling dismissal or a second mutation',async()=>{
  let finish!:(run:CheckRun)=>void;
  const recover=vi.spyOn(productApi,'recoverCheck').mockReturnValue(new Promise(resolve=>{finish=resolve;}));
  const close=vi.fn();
  render(<CheckRecoveryDialog run={initial} projectName="Alpha" canRead canEdit onUpdate={vi.fn()} onClose={close} getCloseFocusTarget={()=>null}/>);
  await userEvent.click(await screen.findByRole('checkbox'));
  await userEvent.click(screen.getByRole('button',{name:'Recover this exact check'}));
  const details=await screen.findByRole('region',{name:'Check recovery details'});
  // JSDOM retains focus on the newly disabled submit button. Start from the
  // dialog fallback used by the browser when that control becomes disabled.
  screen.getByRole('dialog').focus();
  await userEvent.tab();expect(document.activeElement).toBe(details);
  expect(screen.getByRole('button',{name:'Keep inspecting'})).toHaveProperty('disabled',true);
  expect(screen.getByRole('button',{name:'Refresh check recovery'})).toHaveProperty('disabled',true);
  expect(screen.getByRole('button',{name:'Recover this exact check'})).toHaveProperty('disabled',true);
  await userEvent.keyboard('{Escape}');expect(close).not.toHaveBeenCalled();expect(recover).toHaveBeenCalledOnce();
  const saved=checkRunFixture({revision:5,status:'failed',resourcesHeld:false});
  vi.mocked(productApi.checkRun).mockResolvedValue(saved);vi.mocked(productApi.checkRecovery).mockResolvedValue(checkRecoveryFixture({revision:5,state:'complete',method:null,commandIndex:null,recoveryDigest:null}));
  await act(async()=>finish(saved));
  await screen.findByText(/Recovery is complete/);
  expect(screen.queryByRole('region',{name:'Check recovery details'})).toBeNull();
  expect(screen.getByRole('button',{name:'Keep inspecting'})).toHaveProperty('disabled',false);expect(recover).toHaveBeenCalledOnce();
});
