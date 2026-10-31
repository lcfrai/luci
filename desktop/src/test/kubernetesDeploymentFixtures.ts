import type { KubernetesReview, KubernetesTarget } from '../api/kubernetesDeployment';

export const clusterTargetFixture=():KubernetesTarget=>({id:`cluster-${'a'.repeat(32)}`,projectId:'p-one',name:'Existing private cluster',server:'https://cluster.example',namespace:'apps',namespaceUid:'original-namespace',revision:1,fingerprint:'a'.repeat(64)});
export const clusterReviewFixture=():KubernetesReview=>{
 const target=clusterTargetFixture(),id=`workload-${'b'.repeat(32)}`;
 return {id,target,request:{identity:{projectId:'p-one',actorId:'alice',targetId:target.id,targetFingerprint:target.fingerprint,operationId:id,planDigest:'c'.repeat(64)},approvalReference:id,
  blueprint:{version:1,projectId:'p-one',source:{commit:'d'.repeat(40),configurationRevision:0},infrastructure:{id:'cluster',kind:'kubernetes',reference:target.id,namespace:target.namespace},services:[{id:'app',kind:'app',image:`registry/app@sha256:${'e'.repeat(64)}`,port:8080,replicas:1,resources:{requestMilliCpu:100,limitMilliCpu:200,requestMemoryMiB:128,limitMemoryMiB:256}}]},
  expected:[{kind:'Deployment',name:'app',uid:'',resourceVersion:''},{kind:'Service',name:'app',uid:'',resourceVersion:''}]},blueprintRevision:1,blueprintDigest:'f'.repeat(64),runId:`run-${'a'.repeat(24)}`,checkId:'check-one',runInputDigest:'1'.repeat(64),checkDigest:'2'.repeat(64),contextDigest:'3'.repeat(64),monthlyBudgetUsd:25,createdAt:'2026-10-31T00:00:00Z',expiresAt:'2026-10-31T00:10:00Z',applied:false};
};
