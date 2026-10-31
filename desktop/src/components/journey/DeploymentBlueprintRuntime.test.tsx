import { cleanup, render, screen, waitFor } from '@testing-library/react';
import userEvent from '@testing-library/user-event';
import { afterEach, expect, it, vi } from 'vitest';
import { DeploymentBlueprint } from './DeploymentBlueprint';
import { deploymentBlueprintApi, type BlueprintProjectContext } from '../../api/deploymentBlueprint';

vi.mock('../../hooks/useProjectDraft',()=>({useProjectDraftScope:()=> 'alice:p-one'}));
vi.mock('../../hooks/useProjectAccess',()=>({useProjectAccess:()=>({canRead:true,canEdit:true,loading:false,error:'',permissions:{deploy:false,secrets:false}})}));
vi.mock('../../hooks/useUnsavedProjectDraft',()=>({useUnsavedProjectDraft:()=>undefined}));
vi.mock('../teams/TeamWorkspaceProvider',()=>({useTeamWorkspace:()=>null}));
vi.mock('./KubernetesDeployment',()=>({KubernetesDeployment:()=>null}));
vi.mock('./AzureFoundationHandoff',()=>({AzureFoundationHandoff:()=>null}));
afterEach(()=>{cleanup();vi.restoreAllMocks();});

it('retains explicitly entered resource names and keys omitted by bounded pickers without requesting values or deployment authority',async()=>{
 const context:BlueprintProjectContext={projectId:'p-one',head:'a'.repeat(40),clean:true,reviewDigest:'b'.repeat(64),definitionStatus:'draft',designStatus:'approved',source:{commit:'a'.repeat(40),configurationRevision:0}};
 vi.spyOn(deploymentBlueprintApi,'context').mockResolvedValue(context);
 vi.spyOn(deploymentBlueprintApi,'saved').mockResolvedValue({projectId:'p-one',saved:null,sourceVerified:false,effectsApplied:false});
 const save=vi.spyOn(deploymentBlueprintApi,'save').mockImplementation(async(_id,request)=>({projectId:'p-one',saved:{projectId:'p-one',revision:1,digest:'c'.repeat(64),planDigest:'d'.repeat(64),blueprint:request.blueprint,actor:{id:'alice',displayName:'Alice'},at:'2026-10-31T00:00:00Z'},sourceVerified:false,effectsApplied:false}));
 const user=userEvent.setup();render(<DeploymentBlueprint projectId="p-one" active onPublishing={vi.fn()}/>);
 await waitFor(()=>expect((screen.getByRole('button',{name:'Save blueprint'}) as HTMLButtonElement).disabled).toBe(false));
 await user.click(screen.getByText('Runtime environment references'));await user.click(screen.getByRole('button',{name:'Add runtime environment reference'}));
 await user.type(screen.getByRole('textbox',{name:'Environment name'}),'MODEL_TOKEN');await user.type(screen.getByRole('textbox',{name:'Existing resource name'}),'outside-first-page');await user.type(screen.getByRole('textbox',{name:'Existing resource key'}),'known-key');
 await user.click(screen.getByRole('button',{name:'Save blueprint'}));await waitFor(()=>expect(save).toHaveBeenCalledTimes(1));
 expect(save.mock.calls[0][1].blueprint.services[0].environment).toEqual([{name:'MODEL_TOKEN',kind:'secret',resourceName:'outside-first-page',key:'known-key'}]);
 expect(screen.queryByRole('textbox',{name:/secret value/i})).toBeNull();
});
