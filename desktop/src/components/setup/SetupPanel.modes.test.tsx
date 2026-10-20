import { useEffect } from 'react';
import { cleanup, render, screen, waitFor } from '@testing-library/react';
import userEvent from '@testing-library/user-event';
import { afterEach, beforeEach, expect, it, vi } from 'vitest';
import { SetupPanel } from './SetupPanel';
import { createProductClient, type SetupChoice, type SetupStatus } from '../../api/product';
import { resetCockpit, useCockpit } from '../../store/store';

vi.mock('./DependencySetup', () => ({ DependencySetup: () => null }));
vi.mock('./LocalTools', () => ({ LocalTools: () => null }));
vi.mock('./DockerStorageConfirmation', () => ({ DockerStorageConfirmation: () => null }));
vi.mock('./RuntimePreparation', () => ({ RuntimePreparation: ({onPendingChange}:{onPendingChange(value:boolean):void}) => { useEffect(()=>onPendingChange(false),[onPendingChange]); return null; } }));
vi.mock('./CloudManager', () => ({ CloudManager: () => <p>Existing protected connection tools</p> }));
const initial: SetupStatus = {product:'Luci',version:'0.5.0',platform:'test',configured:false,setupChoice:{revision:1,mode:''},config:{projectsDir:'C:\\IsolatedProjects',mode:'local',cloudProvider:'',region:'',domain:'',monthlyBudgetUsd:30},checks:[],cloudProviders:[]};
beforeEach(resetCockpit);afterEach(cleanup);

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
  expect(screen.getByRole('heading',{name:'Luci Cloud is not available yet'})).toBeTruthy();
  expect(returning.fetcher).not.toHaveBeenCalled();
});

it('saves Local before its existing folder preparation and keeps model setup separate',async()=>{
  const f=fixture();render(<SetupPanel {...f.props}/>);
  await userEvent.click(screen.getByRole('button',{name:'Local'}));
  await screen.findByRole('heading',{name:'Set up this computer'});
  await waitFor(()=>expect(screen.getByRole('button',{name:'Continue to projects'})).toHaveProperty('disabled',false));
  expect(f.fetcher.mock.calls.map(c=>new URL(String(c[0])).pathname)).toEqual(['/v1/setup/choice','/v1/setup/config']);
  expect(JSON.parse(String(f.fetcher.mock.calls[1][1]?.body))).toEqual(initial.config);
  expect(screen.getByText(/Use your Codex account, provider API credits or a local/)).toBeTruthy();
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
