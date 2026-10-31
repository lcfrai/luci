import { afterEach, expect, it, vi } from 'vitest';
import { productApi } from './product';
import { coordinatorKubernetesApi,sameCoordinatorKubernetesBinding,validCoordinatorKubernetesPlan,validCoordinatorKubernetesAction,type CoordinatorKubernetesPlan } from './coordinatorKubernetes';

const plan=():CoordinatorKubernetesPlan=>({targetId:'kube-main',targetFingerprint:'a'.repeat(64),namespaceUid:'original-namespace',blueprintRevision:3,blueprintDigest:'b'.repeat(64),blueprintContextDigest:'c'.repeat(64),planDigest:'d'.repeat(64),serviceImages:[{serviceId:'app',image:'registry.example/app@sha256:'+'e'.repeat(64)}],monthlyBudgetUsd:50,acknowledgeCosts:false,acknowledgeImages:false,acknowledgeTelemetry:false});
afterEach(()=>vi.restoreAllMocks());
it('requires separate exact-image and cost delegation and correlates current preparation',async()=>{
 const before=plan(),saved={...before,acknowledgeCosts:true,acknowledgeImages:true};expect(validCoordinatorKubernetesPlan(before)).toBe(false);expect(validCoordinatorKubernetesPlan(saved)).toBe(true);expect(sameCoordinatorKubernetesBinding(before,saved)).toBe(true);
 expect(sameCoordinatorKubernetesBinding({...before,namespaceUid:'replacement'},saved)).toBe(false);expect(validCoordinatorKubernetesPlan({...saved,requiresTelemetry:true})).toBe(false);
 const request=vi.spyOn(productApi,'request').mockResolvedValue(before);const signal=new AbortController().signal;
 await coordinatorKubernetesApi.prepare('p-one',{targetId:'kube-main',monthlyBudgetUsd:50},signal);
 expect(request).toHaveBeenCalledWith('/v1/product/projects/p-one/orchestrator/kubernetes-plan',{targetId:'kube-main',monthlyBudgetUsd:50},'POST',30000,signal);
 request.mockResolvedValue({...before,targetId:'other'});await expect(coordinatorKubernetesApi.prepare('p-one',{targetId:'kube-main',monthlyBudgetUsd:50})).rejects.toThrow('could not be confirmed');
});
it('keeps exact original workload readiness separate from application acceptance',()=>{
 const action={actionId:'continue-one-kubernetes',reviewId:'workload-'+'a'.repeat(32),bindingDigest:'b'.repeat(64),applyAttempted:true,status:'ready',message:'Original namespace workloads ready.',startedAt:'2026-10-31T00:00:00Z',updatedAt:'2026-10-31T00:01:00Z',operationRevision:3,resources:[{kind:'Deployment',name:'app-original',uid:'uid-original'}],applicationHealthVerified:false};
 expect(validCoordinatorKubernetesAction(action,'continue-one')).toBe(true);expect(validCoordinatorKubernetesAction(action,'continue-other')).toBe(false);expect(validCoordinatorKubernetesAction({...action,applicationHealthVerified:true},'continue-one')).toBe(false);expect(validCoordinatorKubernetesAction({...action,status:['ready']},'continue-one')).toBe(false);
});
