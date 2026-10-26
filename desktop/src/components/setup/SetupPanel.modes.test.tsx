import { useEffect } from 'react';
import { act, cleanup, render, screen, waitFor } from '@testing-library/react';
import userEvent from '@testing-library/user-event';
import { afterEach, beforeEach, expect, it, vi } from 'vitest';
import { SetupPanel } from './SetupPanel';
import { createProductClient, type SetupChoice, type SetupStatus } from '../../api/product';
import { resetCockpit, useCockpit } from '../../store/store';

const mounts = vi.hoisted(() => ({ dependencies: vi.fn(), runtime: vi.fn(), storage: vi.fn() }));
vi.mock('./DependencySetup', () => ({ DependencySetup: (props: unknown) => { mounts.dependencies(props); return null; } }));
vi.mock('./LocalTools', () => ({ LocalTools: () => null }));
vi.mock('./DockerStorageConfirmation', () => ({ DockerStorageConfirmation: (props: unknown) => { mounts.storage(props); return null; } }));
vi.mock('./RuntimePreparation', () => ({ RuntimePreparation: (props:{onPendingChange(value:boolean):void;onStatus(value:SetupStatus,revision?:number):void;automatic?:boolean}) => { mounts.runtime(props); useEffect(()=>props.onPendingChange(false),[props.onPendingChange]); return <p>Explicit runtime controls</p>; } }));
vi.mock('./CloudManager', () => ({ CloudManager: () => <p>Existing protected connection tools</p> }));
const initial: SetupStatus = {product:'Luci',version:'0.5.0',platform:'test',configured:false,setupChoice:{revision:1,mode:''},config:{projectsDir:'C:\\IsolatedProjects',mode:'local',cloudProvider:'',region:'',domain:'',monthlyBudgetUsd:30},checks:[],cloudProviders:[]};
beforeEach(() => { resetCockpit(); vi.clearAllMocks(); });afterEach(cleanup);

function fixture(status=initial){
  let choice:SetupChoice=status.setupChoice??{revision:1,mode:''};
  const fetcher=vi.fn<typeof fetch>(async(input,init)=>{
    const path=new URL(String(input)).pathname;
    if(path==='/v1/setup/choice') { const req=JSON.parse(String(init?.body)); choice={revision:req.expectedRevision+1,mode:req.mode}; return new Response(JSON.stringify(choice)); }
    if(path==='/v1/setup/status')return new Response(JSON.stringify({...status,setupChoice:choice}));
    if(path==='/v1/setup/config')return new Response(JSON.stringify({...status,configured:true,setupChoice:choice}));
    throw new Error(`Unexpected mode-test request ${path}`);
  });
  const close=vi.fn();
  const props={initialStatus:status,client:createProductClient('http://localhost:19783',fetcher),presentation:'portal' as const,onStatus:vi.fn(),onProjectsChanged:vi.fn(),onClose:close};
  return {fetcher,close,props};
}

it('persists unavailable hosted preference without provisioning or blocking retained projects',async()=>{
  const f=fixture(); const view=render(<SetupPanel {...f.props}/>);
  await userEvent.click(screen.getByRole('button',{name:'Luci Cloud'}));
  await screen.findByRole('heading',{name:'Luci Cloud is not available yet'});
  expect(f.fetcher).toHaveBeenCalledTimes(1);
  expect(String(f.fetcher.mock.calls[0][0])).toMatch(/\/v1\/setup\/choice$/);
  expect(JSON.parse(String(f.fetcher.mock.calls[0][1]?.body))).toEqual({expectedRevision:1,mode:'luci-cloud'});
  expect(useCockpit.getState().projects).toEqual([]);
  await userEvent.click(screen.getByRole('button',{name:'Continue to projects'}));expect(f.close).toHaveBeenCalledOnce();
  view.unmount(); const returning=fixture({...initial,setupChoice:{revision:2,mode:'luci-cloud'}});render(<SetupPanel {...returning.props}/>);
  expect(screen.getByRole('heading',{name:'Choose your workspace'})).toBeTruthy();
  expect(screen.getByRole('button',{name:'Luci Cloud'}).getAttribute('aria-pressed')).toBe('true');
  expect(returning.fetcher).not.toHaveBeenCalled();
});

it('saves Local as a preference without preparing the folder or starting checks',async()=>{
  const f=fixture();render(<SetupPanel {...f.props}/>);
  await userEvent.click(screen.getByRole('button',{name:'Local'}));
  await screen.findByRole('heading',{name:'Set up this computer'});
  await waitFor(()=>expect(screen.getByRole('button',{name:'Continue to projects'})).toHaveProperty('disabled',false));
  expect(f.fetcher.mock.calls.map(c=>new URL(String(c[0])).pathname)).toEqual(['/v1/setup/choice']);
  expect(mounts.dependencies).not.toHaveBeenCalled();
  expect(mounts.runtime).not.toHaveBeenCalled();
  expect(mounts.storage).not.toHaveBeenCalled();
  expect(screen.getByText(/Use your Codex account, provider API credits or a local/)).toBeTruthy();
});

it.each([
  [false, 'local'], [true, 'local'],
  [false, 'bring-your-own-cloud'], [true, 'bring-your-own-cloud'],
  [false, 'luci-cloud'], [true, 'luci-cloud'],
] as const)('reviews saved mode without effects before continuing (configured=%s, mode=%s)',async(configured,mode)=>{
  const f=fixture({...initial,configured,setupChoice:{revision:4,mode}});
  render(<SetupPanel {...f.props}/>);
  expect(screen.getByRole('heading',{name:'Choose your workspace'})).toBeTruthy();
  const label=mode==='local'?'Local':mode==='bring-your-own-cloud'?'Bring your own cloud':'Luci Cloud';
  expect(screen.getByRole('button',{name:label}).getAttribute('aria-pressed')).toBe('true');
  expect(f.fetcher).not.toHaveBeenCalled();
  expect(mounts.dependencies).not.toHaveBeenCalled();
  expect(mounts.runtime).not.toHaveBeenCalled();
  expect(mounts.storage).not.toHaveBeenCalled();
  await userEvent.click(screen.getByRole('button',{name:'Continue to projects'}));
  expect(f.close).toHaveBeenCalledOnce();
  expect(f.fetcher).not.toHaveBeenCalled();
});

it('requires a fresh check action for another client and after choosing again',async()=>{
  const f=fixture({...initial,configured:true});
  const view=render(<SetupPanel {...f.props}/>);
  await userEvent.click(screen.getByRole('button',{name:'Local'}));
  screen.getByRole('button',{name:'Check local setup'}).focus();
  await userEvent.keyboard('{Enter}');
  await screen.findByText('Explicit runtime controls');
  expect(document.activeElement).toBe(screen.getByRole('heading',{name:'Set up this computer'}));
  expect(mounts.runtime.mock.lastCall?.[0].automatic).not.toBe(true);
  await userEvent.click(screen.getByRole('button',{name:'Choose another setup option'}));
  await userEvent.click(screen.getByRole('button',{name:'Local'}));
  expect(screen.queryByText('Explicit runtime controls')).toBeNull();
  expect(screen.getByRole('button',{name:'Continue to projects'})).toHaveProperty('disabled',false);
  await userEvent.click(screen.getByRole('button',{name:'Check local setup'}));
  await screen.findByText('Explicit runtime controls');
  const other=fixture({...initial,configured:true});
  view.rerender(<SetupPanel {...other.props}/>);
  expect(screen.queryByText('Explicit runtime controls')).toBeNull();
  expect(screen.getByRole('button',{name:'Check local setup'})).toBeTruthy();
  expect(other.fetcher).not.toHaveBeenCalled();
});

it.each(['account','workspace'] as const)('withdraws check activation immediately when the %s changes',async(change)=>{
  const f=fixture({...initial,configured:true});
  render(<SetupPanel {...f.props}/>);
  await userEvent.click(screen.getByRole('button',{name:'Local'}));
  await userEvent.click(screen.getByRole('button',{name:'Check local setup'}));
  await screen.findByText('Explicit runtime controls');
  const next:SetupStatus={...initial,configured:true,...(change==='account'
    ? {access:{user:{username:'another-account',role:'admin'},manageBacking:true,projectsDirManaged:true}}
    : {workspace:{mode:'cloud',connectionId:'another-workspace',host:'another.example.invalid',ready:true}})};
  const previous=mounts.runtime.mock.lastCall?.[0];
  act(()=>previous.onStatus(next,0));
  expect(screen.queryByText('Explicit runtime controls')).toBeNull();
  act(()=>{
    expect(previous.onStatus({...initial,configured:true},0)).toBe(false);
    previous.onPendingChange(true);
  });
  expect(screen.queryByText('Explicit runtime controls')).toBeNull();
  expect(screen.getByRole('button',{name:'Continue to projects'})).toHaveProperty('disabled',false);
  expect(f.fetcher.mock.calls.map(c=>new URL(String(c[0])).pathname)).toEqual(['/v1/setup/choice']);
});

it('requires explicit readback after an uncertain save and does not replay a write',async()=>{
  const f=fixture();f.fetcher.mockRejectedValueOnce(new Error('Response lost'));
  render(<SetupPanel {...f.props}/>);
  await userEvent.click(screen.getByRole('button',{name:'Luci Cloud'}));
  await screen.findByRole('button',{name:'Refresh saved setup choice'});
  expect(screen.getByRole('button',{name:'Local'})).toHaveProperty('disabled',true);
  expect(f.fetcher).toHaveBeenCalledTimes(1);
  await userEvent.click(screen.getByRole('button',{name:'Refresh saved setup choice'}));
  await waitFor(()=>expect(screen.getByRole('button',{name:'Local'})).toHaveProperty('disabled',false));
  expect(f.fetcher).toHaveBeenCalledTimes(2);
});

it.each(['success','refusal'] as const)('discards a delayed %s mode-save response from another client',async(outcome)=>{
  let accept!:(value:Response)=>void, reject!:(cause:Error)=>void;
  const pending=new Promise<Response>((resolve,fail)=>{accept=resolve;reject=fail;});
  const f=fixture();f.fetcher.mockImplementationOnce(()=>pending);
  const view=render(<SetupPanel {...f.props}/>);
  await userEvent.click(screen.getByRole('button',{name:'Local'}));
  const other=fixture();view.rerender(<SetupPanel {...other.props}/>);
  await act(async()=>{
    if(outcome==='success')accept(new Response(JSON.stringify({revision:9,mode:'local'})));
    else reject(new Error('Old mode save failed'));
    await pending.catch(()=>{});
  });
  await waitFor(()=>expect(screen.getByRole('button',{name:'Local'})).toHaveProperty('disabled',false));
  expect(screen.getByRole('heading',{name:'Choose your workspace'})).toBeTruthy();
  expect(screen.queryByRole('button',{name:'Refresh saved setup choice'})).toBeNull();
  expect(other.props.onStatus).not.toHaveBeenCalled();
  expect(other.fetcher).not.toHaveBeenCalled();
});

it.each(['success','refusal'] as const)('discards a delayed %s choice-readback response from another client',async(outcome)=>{
  let accept!:(value:Response)=>void, reject!:(cause:Error)=>void;
  const pending=new Promise<Response>((resolve,fail)=>{accept=resolve;reject=fail;});
  const f=fixture();f.fetcher.mockRejectedValueOnce(new Error('Response lost'));
  const view=render(<SetupPanel {...f.props}/>);
  await userEvent.click(screen.getByRole('button',{name:'Local'}));
  const refresh=await screen.findByRole('button',{name:'Refresh saved setup choice'});
  f.fetcher.mockImplementationOnce(()=>pending);
  await userEvent.click(refresh);
  const other=fixture();view.rerender(<SetupPanel {...other.props}/>);
  await act(async()=>{
    if(outcome==='success')accept(new Response(JSON.stringify({...initial,setupChoice:{revision:9,mode:'local'}})));
    else reject(new Error('Old readback failed'));
    await pending.catch(()=>{});
  });
  await waitFor(()=>expect(screen.getByRole('button',{name:'Local'})).toHaveProperty('disabled',false));
  expect(screen.getByRole('button',{name:'Local'}).getAttribute('aria-pressed')).toBe('false');
  expect(screen.queryByRole('alert')).toBeNull();
  expect(other.props.onStatus).not.toHaveBeenCalled();
  expect(other.fetcher).not.toHaveBeenCalled();
});

it('keeps the three-choice review reachable for a returning configured workspace',async()=>{
  const f=fixture({...initial,configured:true,setupChoice:{revision:2,mode:'local'}});
  render(<SetupPanel {...f.props} presentation="dialog"/>);
  await userEvent.click(screen.getByRole('button',{name:'Workspace setup'}));
  expect(screen.getByRole('button',{name:'Local'}).getAttribute('aria-pressed')).toBe('true');
  await userEvent.click(screen.getByRole('button',{name:'Bring your own cloud'}));
  await screen.findByRole('heading',{name:'Bring your own cloud'});
  expect(f.fetcher).toHaveBeenCalledTimes(1);
  expect(screen.getByRole('region',{name:'Guided Azure setup'})).toBeTruthy();
  expect(screen.getByRole('button',{name:'Prepare Azure instructions'})).toBeTruthy();
  expect(screen.getByText(/Run setup, load its files, then connect your workspace/)).toBeTruthy();
  expect(screen.getByText(/Setup preference saved. Your active connection and existing projects are unchanged/)).toBeTruthy();
  expect(new URL(String(f.fetcher.mock.calls[0][0])).pathname).toBe('/v1/setup/choice');
  expect(JSON.parse(String(f.fetcher.mock.calls[0][1]?.body))).toEqual({expectedRevision:2,mode:'bring-your-own-cloud'});
  await userEvent.click(screen.getByRole('button',{name:'Workspace setup'}));
  expect(screen.getByRole('button',{name:'Bring your own cloud'}).getAttribute('aria-pressed')).toBe('true');
});
