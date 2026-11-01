import { cleanup, render, screen, waitFor } from '@testing-library/react';
import userEvent from '@testing-library/user-event';
import { afterEach, beforeEach, expect, it, vi } from 'vitest';
import { PublicHTTPSCommissioning } from './PublicHTTPSCommissioning';
import { PublicRouteEditor } from './PublicRouteEditor';
import { publicHTTPSApi } from '../../api/publicHTTPS';
import { kubernetesDeploymentApi, type KubernetesStatus } from '../../api/kubernetesDeployment';
import { httpsFixture } from '../../test/fixtures/publicHTTPS';
import { writeHTTPSRecovery } from './publicHTTPSRecovery';
import { useUnsavedProjectDraft } from '../../hooks/useUnsavedProjectDraft';

const state=vi.hoisted(()=>({access:{canRead:true,canEdit:true,loading:false,error:'',permissions:{deploy:true,secrets:true}},team:{teamId:'team-one',revision:1,accessBlocked:false}}));
vi.mock('../../hooks/useProjectAccess',()=>({useProjectAccess:()=>state.access}));
vi.mock('../../hooks/useProjectDraft',()=>({useProjectDraftScope:(id:string)=>`owner/${id}`}));
vi.mock('../../hooks/useUnsavedProjectDraft',()=>({useUnsavedProjectDraft:vi.fn()}));
vi.mock('../teams/TeamWorkspaceProvider',()=>({useTeamWorkspace:()=>state.team}));
const recoveryKey='luci.public-https-recovery.v1:'+JSON.stringify(['owner/p-one','p-one','team-one']);
beforeEach(()=>{localStorage.clear();vi.mocked(useUnsavedProjectDraft).mockClear();state.access.canRead=true;state.access.canEdit=true;state.access.loading=false;state.access.error='';state.access.permissions={deploy:true,secrets:true};state.team.teamId='team-one';state.team.revision=1;state.team.accessBlocked=false;
 vi.spyOn(publicHTTPSApi,'list').mockResolvedValue([]);vi.spyOn(kubernetesDeploymentApi,'list').mockResolvedValue([]);
});
afterEach(()=>{cleanup();vi.restoreAllMocks();});
async function openSetup(){const user=userEvent.setup();await user.click(screen.getByRole('button',{name:'Load endpoints and services'}));await user.click(await screen.findByText('Connect a new public domain'));await user.click(screen.getByText('Azure DNS, certificate contact and storage'));return user;}

it('retains a lost review identity before HTTP without retaining the DNS secret or replaying it on reopen',async()=>{
 const row=httpsFixture();
 // The API layer has its own full response validation. This picker fixture
 // exposes only the already validated deployment projection used by the UI.
 vi.mocked(kubernetesDeploymentApi.list).mockResolvedValue([{review:{id:row.plan.deploymentId,target:{name:'Production',namespace:'app'},request:{blueprint:{services:[{id:'app',kind:'app',port:8080},{id:'model',kind:'inference',port:9000}]}}},operation:{}} as unknown as KubernetesStatus]);
 const review=vi.spyOn(publicHTTPSApi,'review').mockImplementation(async()=>{expect(localStorage.getItem(recoveryKey)).toContain('reviewKey');expect(localStorage.getItem(recoveryKey)).not.toContain('private-dns-password');throw new Error('Response lost');});
 const view=render(<PublicHTTPSCommissioning projectId="p-one" active/>),user=await openSetup();
 await user.type(screen.getByLabelText('Domain'),'app.example.com');await user.selectOptions(screen.getByLabelText('Original deployment'),row.plan.deploymentId);await user.selectOptions(screen.getByLabelText('Service'),'app');
 expect(screen.queryByRole('option',{name:'model · port 9000'})).toBeNull();
 for(const [label,value] of [['Reviewed monthly allowance (USD)','25'],['Azure DNS zone resource ID',row.plan.azureDnsZoneId],['Certificate registration email','owner@example.com'],['Azure tenant ID','tenant-id'],['DNS application client ID','dns-client'],['DNS application client secret','private-dns-password'],['Existing StorageClass','managed-csi']])await user.type(screen.getByLabelText(label),value);
 await user.click(screen.getByRole('checkbox',{name:/I authorize continuing Azure/}));await user.click(screen.getByRole('button',{name:'Review HTTPS setup'}));await screen.findByRole('alert');expect(review).toHaveBeenCalledTimes(1);
 const retained=localStorage.getItem(recoveryKey)!;expect(retained).not.toContain('private-dns-password');
 row.reviewKey=JSON.parse(retained).pending[0].reviewKey;vi.mocked(publicHTTPSApi.list).mockResolvedValueOnce([row]);await user.click(screen.getByRole('button',{name:'Read original review'}));
 await waitFor(()=>expect(screen.getByLabelText('Saved endpoint')).toHaveValue(row.id));expect(screen.getByLabelText('Domain')).toHaveValue('');expect(screen.getByLabelText('DNS application client secret')).toHaveValue('');expect(vi.mocked(useUnsavedProjectDraft).mock.calls.slice(-1)[0][1]).toBe(false);
 view.unmount();localStorage.setItem(recoveryKey,retained);
 render(<PublicHTTPSCommissioning projectId="p-one" active/>);await screen.findByText('Review response not yet confirmed');expect(review).toHaveBeenCalledTimes(1);expect(localStorage.getItem(recoveryKey)).toBe(retained);
});
it('preserves a private form through routine membership refresh but withdraws it on confirmed Secrets loss',async()=>{
 const view=render(<PublicHTTPSCommissioning projectId="p-one" active/>),user=await openSetup();
 await user.type(screen.getByLabelText('Domain'),'draft.example.com');await user.type(screen.getByLabelText('DNS application client secret'),'private-window-only');
 state.team.revision++;state.access.loading=true;state.access.permissions={deploy:false,secrets:false};view.rerender(<PublicHTTPSCommissioning projectId="p-one" active/>);
 expect(screen.queryByLabelText('Domain')).toBeNull();
 state.access.loading=false;state.access.permissions={deploy:true,secrets:true};view.rerender(<PublicHTTPSCommissioning projectId="p-one" active/>);
 expect(screen.getByLabelText('Domain')).toHaveValue('draft.example.com');expect(screen.getByLabelText('DNS application client secret')).toHaveValue('private-window-only');
 state.access.permissions.secrets=false;view.rerender(<PublicHTTPSCommissioning projectId="p-one" active/>);expect(screen.queryByLabelText('DNS application client secret')).toBeNull();
 state.access.permissions.secrets=true;view.rerender(<PublicHTTPSCommissioning projectId="p-one" active/>);await openSetup();expect(screen.getByLabelText('DNS application client secret')).toHaveValue('');
});
it('keeps an uncertain effect while observing separately and retries only its unchanged original identity',async()=>{
 const row=httpsFixture(),key='original_'+'a'.repeat(32);row.state='pending-address';row.revision=2;
 row.actions=[{requestKey:key,requestDigest:'b'.repeat(64),action:'apply',state:'unknown',startedRevision:1,settledRevision:0}];
 writeHTTPSRecovery(recoveryKey,null,{version:1,projectId:'p-one',teamId:'team-one',pending:[{kind:'action',reviewKey:row.reviewKey,id:row.id,digest:row.plan.digest,expectedRevision:1,requestKey:key,action:'apply'}]});
 vi.mocked(publicHTTPSApi.list).mockResolvedValue([row]);
 const observe=vi.spyOn(publicHTTPSApi,'action').mockImplementation(async(_project,original,action,requestKey)=>({...row,revision:4,actions:[...row.actions,{requestKey,requestDigest:'c'.repeat(64),action,state:'settled' as const,startedRevision:original.revision,settledRevision:3}]}));
 const retry=vi.spyOn(publicHTTPSApi,'originalAction').mockResolvedValue({...row,revision:6,actions:[{...row.actions[0],state:'settled',settledRevision:5}]});
 render(<PublicHTTPSCommissioning projectId="p-one" active/>);const user=userEvent.setup();await screen.findByText('Original apply request needs readback');await user.click(screen.getByRole('button',{name:'Load endpoints and services'}));await user.selectOptions(await screen.findByLabelText('Saved endpoint'),row.id);await user.click(screen.getByRole('button',{name:'Check endpoint'}));
 await waitFor(()=>expect(observe).toHaveBeenCalledTimes(1));expect(observe.mock.calls[0][2]).toBe('observe');expect(observe.mock.calls[0][3]).not.toBe(key);await waitFor(()=>expect(JSON.parse(localStorage.getItem(recoveryKey)!).pending).toHaveLength(1));
 expect(localStorage.getItem(recoveryKey)).toContain(key);await user.click(screen.getByRole('button',{name:'Retry this original request'}));
 await waitFor(()=>expect(retry).toHaveBeenCalledWith('p-one',row.id,row.plan.digest,row.reviewKey,1,'apply',key,expect.any(AbortSignal)));await waitFor(()=>expect(localStorage.getItem(recoveryKey)).toBeNull());
});
it('selects an exact verified managed endpoint only for the matching target namespace and Service port',async()=>{
 vi.spyOn(Date,'now').mockReturnValue(Date.parse('2026-11-02T00:00:00Z'));
 const row=httpsFixture();row.state='ready';row.httpsVerified=true;row.dnsObserved=true;row.address='203.0.113.8';row.observedAt='2030-01-01T00:05:00Z';row.certificateNotAfter='2030-03-01T00:00:00Z';row.certificateSha256='f'.repeat(64);row.url='https://app.example.com/';
 const expiredId='https-'+'e'.repeat(32);
 vi.mocked(publicHTTPSApi.list).mockResolvedValue([{...row,id:'https-'+'f'.repeat(32),plan:{...row.plan,destination:{...row.plan.destination,namespace:'foreign'}}},{...row,id:expiredId,plan:{...row.plan,id:expiredId},certificateNotAfter:'2000-01-01T00:00:00Z'},row]);
 const change=vi.fn();render(<PublicRouteEditor projectId="p-one" targetId={row.plan.destination.targetId} namespace="app" services={[{id:'app',kind:'app',port:8080,image:'',replicas:1,resources:{requestMilliCpu:1,limitMilliCpu:1,requestMemoryMiB:128,limitMemoryMiB:128}}]} value={{profile:'luci-managed-https',managedHTTPSId:'',serviceId:'',ingressClassName:'',tlsSecretName:'',hostname:'',path:'/',pathType:'Prefix'}} disabled={false} onChange={change}/>);
 const user=userEvent.setup();await user.click(screen.getByText('Public HTTPS route'));await user.click(screen.getByRole('button',{name:'Load verified endpoints for this target'}));await user.selectOptions(await screen.findByLabelText('Commissioned endpoint'),row.id);
 expect(screen.getByLabelText('Commissioned endpoint').querySelectorAll('option')).toHaveLength(2);
 expect(change).toHaveBeenCalledWith({profile:'luci-managed-https',managedHTTPSId:row.id,serviceId:'app',ingressClassName:'',tlsSecretName:'',hostname:'app.example.com',path:'/',pathType:'Prefix'});
});
it('keeps all128 indexed originals while inspecting an older review without displacing the oldest paused endpoint',async()=>{
 const values=Array.from({length:128},(_,index)=>{const row=httpsFixture(),id='https-'+index.toString(16).padStart(32,'0');return {...row,id,reviewKey:'review_'+index.toString(16).padStart(32,'0'),state:index===0?'paused' as const:'reviewed' as const,plan:{...row.plan,id,hostname:`app${index}.example.com`}};});
 const older=httpsFixture();older.id='https-'+'f'.repeat(32);older.plan={...older.plan,id:older.id,hostname:'older.example.com'};
 vi.mocked(publicHTTPSApi.list).mockImplementation(async(_project,_signal,key)=>key?[older]:values);
 render(<PublicHTTPSCommissioning projectId="p-one" active/>);const user=await openSetup();await user.type(screen.getByLabelText('Domain'),'draft.example.com');await user.type(screen.getByLabelText('DNS application client secret'),'window-private');
 expect(screen.getByLabelText('Saved endpoint').querySelectorAll('option')).toHaveLength(129);
 // Restore a separately retained metadata request through a real remount.
 cleanup();writeHTTPSRecovery(recoveryKey,null,{version:1,projectId:'p-one',teamId:'team-one',pending:[{kind:'review',reviewKey:older.reviewKey}]});
 render(<PublicHTTPSCommissioning projectId="p-one" active/>);await user.click(screen.getByRole('button',{name:'Load endpoints and services'}));await user.click(screen.getByRole('button',{name:'Read original review'}));
 await waitFor(()=>expect(screen.getByLabelText('Saved endpoint')).toHaveValue(older.id));expect(screen.getByLabelText('Saved endpoint').querySelectorAll('option')).toHaveLength(130);
 await user.selectOptions(screen.getByLabelText('Saved endpoint'),values[0].id);expect(screen.getByLabelText('Saved endpoint')).toHaveValue(values[0].id);expect(screen.getByLabelText('Saved endpoint').querySelectorAll('option')).toHaveLength(129);
});
