import { runtimePrerequisitesApi } from '../../api/runtimePrerequisites';
import { act, cleanup, render, screen, waitFor } from '@testing-library/react';
import userEvent from '@testing-library/user-event';
import { afterEach, beforeEach, expect, it, vi } from 'vitest';
import { CoordinatorKubernetesSetup } from './CoordinatorKubernetesSetup';
import { coordinatorKubernetesApi,type CoordinatorKubernetesPlan } from '../../api/coordinatorKubernetes';
import { kubernetesDeploymentApi } from '../../api/kubernetesDeployment';
const access=vi.hoisted(()=>({canRead:true,canEdit:true,loading:false,error:'',permissions:{deploy:true,secrets:true}}));
vi.mock('../../hooks/useProjectAccess',()=>({useProjectAccess:()=>access}));
vi.mock('../../hooks/useUnsavedProjectDraft',()=>({useUnsavedProjectDraft:vi.fn()}));
const prepared=():CoordinatorKubernetesPlan=>({targetId:'kube-main',targetFingerprint:'a'.repeat(64),namespaceUid:'original-namespace',blueprintRevision:3,blueprintDigest:'b'.repeat(64),blueprintContextDigest:'c'.repeat(64),planDigest:'d'.repeat(64),serviceImages:[{serviceId:'app',image:'registry.example/app@sha256:'+'e'.repeat(64)}],monthlyBudgetUsd:50,acknowledgeCosts:false,acknowledgeImages:false,acknowledgeTelemetry:false});
beforeEach(()=>{vi.spyOn(runtimePrerequisitesApi,'list').mockResolvedValue([]);});
afterEach(()=>{cleanup();vi.restoreAllMocks();access.permissions.secrets=true;});
it('uses explicit namespace selection and full immutable-image review before exposing a delegation',async()=>{
 const read=vi.spyOn(kubernetesDeploymentApi,'targets').mockResolvedValue([{id:'kube-main',projectId:'p-one',name:'Development cluster',server:'https://cluster.example',namespace:'project-one',namespaceUid:'original-namespace',revision:1,fingerprint:'a'.repeat(64)}]);vi.spyOn(coordinatorKubernetesApi,'prepare').mockResolvedValue(prepared());const change=vi.fn(),user=userEvent.setup();
 render(<CoordinatorKubernetesSetup projectId="p-one" scope="alice/project" disabled={false} enabled recordedDraft="" onChange={change} onNavigateDeploy={vi.fn()}/>);expect(read).not.toHaveBeenCalled();
 await user.click(screen.getByText('Continue to an existing Kubernetes namespace'));await user.click(screen.getByRole('button',{name:'Read Kubernetes namespace targets'}));
 await user.selectOptions(screen.getByLabelText('Kubernetes namespace target'),'kube-main');await user.type(screen.getByLabelText(/Reviewed monthly workload budget/),'50');await user.click(screen.getByRole('button',{name:'Read current Kubernetes delegation'}));await screen.findByRole('region',{name:'Review Kubernetes continuation'});
 expect(change.mock.calls[change.mock.calls.length-1][0].plan).toBeNull();await user.click(screen.getByRole('checkbox',{name:/I reviewed these exact image references/}));await user.click(screen.getByRole('checkbox',{name:/I authorize the reviewed existing-namespace/}));await waitFor(()=>expect(change.mock.calls[change.mock.calls.length-1][0].plan?.targetId).toBe('kube-main'));
});
it('withdraws late namespace results and prepared delegation when Secrets access is removed',async()=>{
 let finish!:(value:Awaited<ReturnType<typeof kubernetesDeploymentApi.targets>>)=>void;vi.spyOn(kubernetesDeploymentApi,'targets').mockReturnValue(new Promise(resolve=>{finish=resolve;}));const change=vi.fn(),props={projectId:'p-one',scope:'alice/project',disabled:false,enabled:true,recordedDraft:'',onChange:change,onNavigateDeploy:vi.fn()},user=userEvent.setup();const view=render(<CoordinatorKubernetesSetup {...props}/>);
 await user.click(screen.getByText('Continue to an existing Kubernetes namespace'));await user.click(screen.getByRole('button',{name:'Read Kubernetes namespace targets'}));access.permissions.secrets=false;view.rerender(<CoordinatorKubernetesSetup {...props}/>);await act(async()=>{finish([{id:'kube-main',projectId:'p-one',name:'Late private target',server:'https://cluster.example',namespace:'project-one',namespaceUid:'original-namespace',revision:1,fingerprint:'a'.repeat(64)}]);});
 expect(screen.queryByRole('option',{name:/Late private target/})).toBeNull();expect(change.mock.calls[change.mock.calls.length-1][0].plan).toBeNull();
});
