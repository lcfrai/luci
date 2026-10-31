import { act, cleanup, render, screen, waitFor } from '@testing-library/react';
import userEvent from '@testing-library/user-event';
import { afterEach, beforeEach, expect, it, vi } from 'vitest';
import { KubernetesReferencePicker } from './KubernetesReferencePicker';
import { kubernetesDeploymentApi, type KubernetesReferenceView } from '../../api/kubernetesDeployment';
import { clusterTargetFixture } from '../../test/kubernetesDeploymentFixtures';

const access=vi.hoisted(()=>({canRead:true,canEdit:true,loading:false,error:'',permissions:{deploy:true,secrets:true}}));
const scope=vi.hoisted(()=>({actor:'alice'}));
vi.mock('../../hooks/useProjectAccess',()=>({useProjectAccess:()=>access}));
vi.mock('../../hooks/useProjectDraft',()=>({useProjectDraftScope:()=>`actor:${scope.actor}:workspace`}));
const target=clusterTargetFixture();
const view=(detail=false):KubernetesReferenceView=>({projectId:'p-one',targetId:target.id,targetFingerprint:target.fingerprint,namespaceUid:target.namespaceUid,namespace:'apps',items:[{kind:'secret',name:'model-access',namespace:'apps',uid:'secret-one',resourceVersion:'5',...(detail?{keys:['token']}:{}),keysRead:detail,partial:false}],partial:false});
beforeEach(()=>{access.permissions.secrets=true;access.permissions.deploy=true;scope.actor='alice';});
afterEach(()=>{cleanup();vi.restoreAllMocks();});

it('reads names only on request and requires explicit resource and key selection before adoption',async()=>{
 const user=userEvent.setup(),choose=vi.fn(),read=vi.spyOn(kubernetesDeploymentApi,'references').mockResolvedValueOnce(view()).mockResolvedValueOnce(view(true));
 render(<KubernetesReferencePicker projectId="p-one" targetId={target.id} namespace="apps" kind="secret" active onChoose={choose}/>);
 expect(read).not.toHaveBeenCalled();await user.click(screen.getByText('Choose from existing Secrets'));
 await user.click(screen.getByRole('button',{name:'Read existing Secrets'}));
 await user.selectOptions(await screen.findByRole('combobox',{name:'Existing Secret'}),'model-access');
 const keys=await screen.findByRole('combobox',{name:'Existing key in model-access'});
 expect((screen.getByRole('button',{name:'Use selected resource key'}) as HTMLButtonElement).disabled).toBe(true);
 await user.selectOptions(keys,'token');expect(choose).not.toHaveBeenCalled();await user.click(screen.getByRole('button',{name:'Use selected resource key'}));
 expect(choose).toHaveBeenCalledExactlyOnceWith(view(true).items[0],'token');expect(read.mock.calls.map(row=>row[3])).toEqual(['','model-access']);
});

it('withdraws a late metadata read when Secrets or actor scope changes',async()=>{
 const user=userEvent.setup(),choose=vi.fn();let finish!:(v:KubernetesReferenceView)=>void;
 const read=vi.spyOn(kubernetesDeploymentApi,'references').mockImplementationOnce(()=>new Promise(resolve=>{finish=resolve;}));
 const props={projectId:'p-one',targetId:target.id,namespace:'apps',kind:'secret' as const,active:true,onChoose:choose};
 const rendered=render(<KubernetesReferencePicker {...props}/>);await user.click(screen.getByText('Choose from existing Secrets'));await user.click(screen.getByRole('button',{name:'Read existing Secrets'}));
 access.permissions.secrets=false;scope.actor='bob';rendered.rerender(<KubernetesReferencePicker {...props}/>);await act(async()=>finish(view()));
 await user.click(screen.getByText('Choose from existing Secrets'));
 expect(screen.queryByRole('option',{name:'model-access'})).toBeNull();expect((screen.getByRole('button',{name:'Read existing Secrets'}) as HTMLButtonElement).disabled).toBe(true);
 expect(read).toHaveBeenCalledTimes(1);expect(choose).not.toHaveBeenCalled();expect(read.mock.calls[0][4]?.aborted).toBe(true);
});

it('discloses incomplete inventories and refuses a pending block-mode model volume',async()=>{
 const user=userEvent.setup(),choose=vi.fn();
 const list:KubernetesReferenceView={...view(),partial:true,items:[{kind:'persistent-volume-claim',name:'models',namespace:'apps',uid:'pvc-one',resourceVersion:'1',keysRead:false,partial:false}]};
 const detail:KubernetesReferenceView={...list,partial:false,items:[{...list.items[0],phase:'Pending',volumeMode:'Block',accessModes:['ReadWriteOncePod']}]};
 vi.spyOn(kubernetesDeploymentApi,'references').mockResolvedValueOnce(list).mockResolvedValueOnce(detail);
 render(<KubernetesReferencePicker projectId="p-one" targetId={target.id} namespace="apps" kind="persistent-volume-claim" active onChoose={choose}/>);
 await user.click(screen.getByText('Choose from existing model volumes'));await user.click(screen.getByRole('button',{name:'Read existing model volumes'}));
 expect(await screen.findByText(/not a complete namespace inventory/)).not.toBeNull();await user.selectOptions(screen.getByRole('combobox',{name:'Existing model volume'}),'models');
 await waitFor(()=>expect(screen.getByText(/Pending · Block/)).not.toBeNull());expect((screen.getByRole('button',{name:'Use selected model volume'}) as HTMLButtonElement).disabled).toBe(true);expect(choose).not.toHaveBeenCalled();
});
