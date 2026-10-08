import {act, render, screen, waitFor, within} from '@testing-library/react';
import userEvent from '@testing-library/user-event';
import {beforeEach, expect, it, vi} from 'vitest';
import {createProductClient, type SetupStatus} from '../../api/product';
import type {ExecutionStorage} from '../../api/executionStorage';
import {resetCockpit} from '../../store/store';
import {SetupPanel} from './SetupPanel';

vi.mock('../auth/AuthProvider', () => ({useAuth: () => ({profile: {user: {id: 'owner', role: 'super-admin'}, permissions: {manageBacking: true}, policy: {allowCloudProvisioning: false}}})}));
vi.mock('../teams/TeamWorkspaceProvider', () => ({useTeamWorkspace: () => null}));
// Installation has separate consent tests; the real preparation/storage components remain mounted.
vi.mock('./DependencySetup', () => ({DependencySetup: () => null}));
const ready = (): SetupStatus => ({product:'Luci',version:'fixture',platform:'windows',configured:true,config:{projectsDir:'C:\\Projects',mode:'local',cloudProvider:'',region:'',domain:'',monthlyBudgetUsd:0},checks:[
  {id:'git',label:'Git',status:'ready',message:'Recorded Git ready.',action:''},
  {id:'docker',label:'Docker',status:'ready',message:'Recorded Docker ready.',action:''},
  {id:'sandbox-image',label:'Runtime',status:'ready',message:'Recorded runtime ready.',action:''},
  {id:'docker-backing-storage',label:'Docker storage readiness',status:'ready',message:'Recorded storage readiness.',action:''},
],cloudProviders:[]});
const storage = (patch: Partial<ExecutionStorage> = {}): ExecutionStorage => ({platform:'windows',scope:'local-docker-desktop',required:true,state:'confirmed',revision:1,reasonCode:'confirmed',message:'Current recorded storage observation.',basis:'Controlled storage fixture.',hostAvailableBytes:12*1024**3,guestAvailableBytes:24*1024**3,...patch});
function fixture(value = storage(), portal = false) {
  let observed = value, checks = 0, reads = 0, confirmations = 0;
  let checkReply: SetupStatus | undefined;
  const fetcher = vi.fn<typeof fetch>().mockImplementation(async (url, init) => {
    const path = new URL(String(url)).pathname;
    if (path === '/v1/admin/execution-storage') {
      if (init?.method === 'POST') {confirmations++; observed=storage({state:'pending-observation',revision:2,message:'Saved confirmation; waiting for measurement.'});}
      else reads++;
      return new Response(JSON.stringify(observed));
    }
    if (path === '/v1/setup/check') {checks++;if (checkReply) return new Response(JSON.stringify(checkReply));if (portal && checks===1) return new Response(JSON.stringify(ready())); return new Promise(() => {});}
    if (path === '/v1/setup/runtime/install') return new Response(JSON.stringify({status:'ready',message:'Recorded runtime ready.'}));
    if (path === '/v1/setup/git/identity') return new Response(JSON.stringify({name:'Fixture owner',email:'owner@example.invalid'}));
    throw new Error(`Unexpected fixture request ${path}`);
  });
  return {client:createProductClient('http://localhost:8730',fetcher),fetcher,checkCount:()=>checks,readCount:()=>reads,confirmationCount:()=>confirmations,observe:(value:ExecutionStorage)=>{observed=value;},allowReadiness:(value:SetupStatus)=>{checkReply=value;}};
}
beforeEach(()=>{vi.restoreAllMocks();resetCockpit();});

it('refreshes cached storage without running a full readiness check or locking Settings navigation', async () => {
  const f=fixture(), close=vi.fn();
  render(<SetupPanel initialStatus={ready()} client={f.client} onStatus={vi.fn()} onClose={close} onProjectsChanged={vi.fn()}/>);
  await screen.findByText(/Current recorded storage observation/);
  await userEvent.click(screen.getByRole('button',{name:'Refresh Docker storage status'}));
  await waitFor(()=>expect(screen.getByRole('button',{name:'Refresh Docker storage status'})).toHaveProperty('disabled',false));
  expect(f.readCount()).toBe(2);
  expect.soft(f.checkCount()).toBe(0);
  for(const name of ['Close settings','Back to project','Appearance','New project']) expect.soft(screen.getByRole('button',{name})).toHaveProperty('disabled',false);
  await userEvent.click(screen.getByRole('button',{name:'Appearance'}));
  expect(screen.queryByRole('heading',{name:'Make room for your way of working.'})).not.toBeNull();
  await userEvent.click(screen.getByRole('button',{name:'Close settings'}));
  expect(close).toHaveBeenCalledOnce();
});

it('keeps portal continuation usable without restarting preparation when storage is refreshed', async () => {
  const f=fixture(storage(),true), close=vi.fn();
  render(<SetupPanel initialStatus={ready()} presentation="portal" client={f.client} onStatus={vi.fn()} onClose={close} onProjectsChanged={vi.fn()}/>);
  await userEvent.click(screen.getByRole('button',{name:'On this computer'}));
  await waitFor(()=>expect(screen.getByRole('button',{name:'Continue to projects'})).toHaveProperty('disabled',false));
  expect(f.checkCount()).toBe(1);
  await userEvent.click(screen.getByRole('button',{name:'Refresh Docker storage status'}));
  await waitFor(()=>expect(screen.getByRole('button',{name:'Refresh Docker storage status'})).toHaveProperty('disabled',false));
  expect.soft(f.checkCount()).toBe(1);
  expect.soft(screen.getByRole('button',{name:'Continue to projects'})).toHaveProperty('disabled',false);
  await userEvent.click(screen.getByRole('button',{name:'Continue to projects'}));
  expect(close).toHaveBeenCalledOnce();
});

it('invalidates only storage advice after confirmation and waits for explicit readiness without discarding an unsaved folder', async () => {
  const f=fixture(storage({state:'confirmation-required',message:'Confirm this fixture location.'})), received=vi.fn();
  render(<SetupPanel initialStatus={ready()} client={f.client} onStatus={received} onClose={vi.fn()} onProjectsChanged={vi.fn()}/>);
  const folder=screen.getByRole('textbox',{name:/Projects folder/});
  await userEvent.clear(folder);await userEvent.type(folder,'D:\\Unsaved projects');
  await userEvent.type(await screen.findByLabelText('Active Docker VHD file'),String.raw`D:\Docker\disk.vhdx`);
  await userEvent.click(screen.getByRole('checkbox',{name:/I checked that this is Docker Desktop/}));
  await userEvent.click(screen.getByRole('button',{name:'Review storage location'}));
  await userEvent.click(screen.getByRole('button',{name:'Confirm Docker storage location'}));
  await screen.findByText(/Saved confirmation; waiting for measurement/);
  expect(f.confirmationCount()).toBe(1);expect(f.checkCount()).toBe(0);
  expect(folder).toHaveProperty('value','D:\\Unsaved projects');
  expect(received.mock.lastCall?.[0].checks.find((check:{id:string})=>check.id==='docker-backing-storage').status).toBe('unknown');
  expect(received.mock.lastCall?.[0].checks.find((check:{id:string})=>check.id==='docker').status).toBe('ready');
  expect(screen.getByRole('button',{name:'Back to project'})).toHaveProperty('disabled',false);
  f.observe(storage({revision:2,message:'Fresh recorded location confirmed.'}));
  await userEvent.click(screen.getByRole('button',{name:'Refresh Docker storage status'}));
  await screen.findByText(/Fresh recorded location confirmed/);
  expect(f.checkCount()).toBe(0);
  expect(received.mock.lastCall?.[0].checks.find((check:{id:string})=>check.id==='docker-backing-storage').status).toBe('unknown');
  f.allowReadiness(ready());
  await userEvent.click(screen.getByRole('button',{name:'Check readiness'}));
  await screen.findByText('Readiness check finished.');
  expect(f.checkCount()).toBe(1);
  expect(received.mock.lastCall?.[0].checks.find((check:{id:string})=>check.id==='docker-backing-storage').status).toBe('ready');
});

it('keeps navigation usable while a cached refresh is pending and ignores its result after leaving storage', async () => {
  const f=fixture(), received=vi.fn();let finish!: (response:Response)=>void;
  render(<SetupPanel initialStatus={ready()} client={f.client} onStatus={received} onClose={vi.fn()} onProjectsChanged={vi.fn()}/>);
  await screen.findByText(/Current recorded storage observation/);
  f.fetcher.mockImplementationOnce(()=>new Promise(resolve=>{finish=resolve;}));
  await userEvent.click(screen.getByRole('button',{name:'Refresh Docker storage status'}));
  expect(screen.getByRole('button',{name:'Refresh Docker storage status'})).toHaveProperty('disabled',true);
  expect(screen.getByRole('button',{name:'Close settings'})).toHaveProperty('disabled',false);
  expect(screen.getByRole('button',{name:'Back to project'})).toHaveProperty('disabled',false);
  await userEvent.click(screen.getByRole('button',{name:'Appearance'}));
  await act(async()=>finish(new Response(JSON.stringify(storage({state:'invalidated',revision:99,message:'Old view result must not invalidate this screen.'})))));
  expect(received).not.toHaveBeenCalled();expect(f.checkCount()).toBe(0);
  expect(screen.queryByText(/Old view result/)).toBeNull();
});

it('shows stale storage in portal preparation without probing again or blocking continuing to projects', async () => {
  const f=fixture(storage(),true);
  render(<SetupPanel initialStatus={ready()} presentation="portal" client={f.client} onStatus={vi.fn()} onClose={vi.fn()} onProjectsChanged={vi.fn()}/>);
  await userEvent.click(screen.getByRole('button',{name:'On this computer'}));
  await waitFor(()=>expect(screen.getByRole('button',{name:'Continue to projects'})).toHaveProperty('disabled',false));
  f.observe(storage({revision:2,state:'invalidated',message:'The storage settings changed.'}));
  await userEvent.click(screen.getByRole('button',{name:'Refresh Docker storage status'}));
  await screen.findByText(/The storage settings changed/);
  const preparation=screen.getByRole('region',{name:'Prepare your workspace'});
  expect(within(preparation).getByText('Docker storage changed. Choose Check readiness to refresh workspace readiness.')).not.toBeNull();
  expect(f.checkCount()).toBe(1);
  expect(screen.getByRole('button',{name:'Continue to projects'})).toHaveProperty('disabled',false);
  expect(screen.getByRole('button',{name:'Choose another setup option'})).toHaveProperty('disabled',false);
});
