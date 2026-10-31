import { productApi } from './product';

export interface CoordinatorKubernetesPlan {
 requiresPublicRoute?:boolean;acknowledgePublicRoute?:boolean;
 preparation?:{id:string;digest:string};
 targetId:string;targetFingerprint:string;namespaceUid:string;blueprintRevision:number;blueprintDigest:string;
 blueprintContextDigest:string;planDigest:string;serviceImages:{serviceId:string;image:string}[];requiresTelemetry?:boolean;
 monthlyBudgetUsd:number;acknowledgeCosts:boolean;acknowledgeImages:boolean;acknowledgeTelemetry:boolean;
}
export interface CoordinatorKubernetesAction {
 actionId:string;reviewId:string;bindingDigest:string;applyAttempted:boolean;status:'pending'|'running'|'ready'|'unknown'|'blocked';
 message:string;startedAt:string;updatedAt:string;operationRevision:number;resources:{kind:'Deployment'|'Service'|'Ingress'|'ConfigMap';name:string;uid:string}[];applicationHealthVerified:false;
}
const object=(v:unknown):v is Record<string,unknown>=>!!v&&typeof v==='object'&&!Array.isArray(v);
const token=(v:unknown,max=128):v is string=>typeof v==='string'&&v.length>0&&v.length<=max&&!/[\r\n\0]/.test(v);
const hash=(v:unknown):v is string=>typeof v==='string'&&/^[a-f0-9]{64}$/.test(v);
const positive=(v:unknown):v is number=>Number.isSafeInteger(v)&&(v as number)>0&&(v as number)<1e9;
const keys=['requiresPublicRoute','acknowledgePublicRoute','preparation','targetId','targetFingerprint','namespaceUid','blueprintRevision','blueprintDigest','blueprintContextDigest','planDigest','serviceImages','requiresTelemetry','monthlyBudgetUsd','acknowledgeCosts','acknowledgeImages','acknowledgeTelemetry'];
export function validCoordinatorKubernetesPlan(v:unknown,prepared=false):v is CoordinatorKubernetesPlan{
 if(!object(v)||Object.keys(v).some(key=>!keys.includes(key))||!token(v.targetId)||!hash(v.targetFingerprint)||!token(v.namespaceUid)||!positive(v.blueprintRevision)||!hash(v.blueprintDigest)||!hash(v.blueprintContextDigest)||!hash(v.planDigest)||!Array.isArray(v.serviceImages)||v.serviceImages.length<1||v.serviceImages.length>32||typeof v.monthlyBudgetUsd!=='number'||!Number.isFinite(v.monthlyBudgetUsd)||v.monthlyBudgetUsd<=0||v.requiresTelemetry!==undefined&&typeof v.requiresTelemetry!=='boolean'||v.requiresPublicRoute!==undefined&&typeof v.requiresPublicRoute!=='boolean'||v.acknowledgePublicRoute!==undefined&&typeof v.acknowledgePublicRoute!=='boolean'||typeof v.acknowledgeCosts!=='boolean'||typeof v.acknowledgeImages!=='boolean'||typeof v.acknowledgeTelemetry!=='boolean')return false;
 if(v.preparation!==undefined&&(!object(v.preparation)||Object.keys(v.preparation).some(k=>!['id','digest'].includes(k))||typeof v.preparation.id!=='string'||!/^prep-[a-f0-9]{32}$/.test(v.preparation.id)||!hash(v.preparation.digest)))return false;
 if(prepared?(v.acknowledgeCosts||v.acknowledgeImages||v.acknowledgeTelemetry||v.acknowledgePublicRoute):(!v.acknowledgeCosts||!v.acknowledgeImages||v.requiresTelemetry&&!v.acknowledgeTelemetry||v.requiresPublicRoute&&!v.acknowledgePublicRoute))return false;
 return new Set(v.serviceImages.map(row=>object(row)?row.serviceId:null)).size===v.serviceImages.length&&v.serviceImages.every(row=>object(row)&&Object.keys(row).every(key=>['serviceId','image'].includes(key))&&token(row.serviceId)&&((token(row.image,1024)&&/^[^\s@]+@sha256:[a-f0-9]{64}$/.test(row.image))||(row.image===''&&v.preparation!==undefined)));
}
export function sameCoordinatorKubernetesBinding(prepared:CoordinatorKubernetesPlan,saved:CoordinatorKubernetesPlan):boolean{
 return validCoordinatorKubernetesPlan(prepared,true)&&validCoordinatorKubernetesPlan(saved)&&keys.filter(key=>!key.startsWith('acknowledge')).every(key=>JSON.stringify(prepared[key as keyof CoordinatorKubernetesPlan]??false)===JSON.stringify(saved[key as keyof CoordinatorKubernetesPlan]??false));
}
export function validCoordinatorKubernetesAction(value:unknown,continuationId:string):value is CoordinatorKubernetesAction{
 if(!object(value)||value.actionId!==continuationId+'-kubernetes'||!token(value.reviewId,41)||!/^workload-[a-f0-9]{32}$/.test(value.reviewId)||!hash(value.bindingDigest)||typeof value.applyAttempted!=='boolean'||typeof value.status!=='string'||!['pending','running','ready','unknown','blocked'].includes(value.status)||!token(value.message,4000)||!token(value.startedAt,64)||!Number.isFinite(Date.parse(value.startedAt))||!token(value.updatedAt,64)||!Number.isFinite(Date.parse(value.updatedAt))||!Number.isSafeInteger(value.operationRevision)||(value.operationRevision as number)<0||!Array.isArray(value.resources)||value.resources.length>68||value.applicationHealthVerified!==false)return false;
 if(value.status==='ready'&&(!value.applyAttempted||!positive(value.operationRevision)||value.resources.length===0))return false;
 return value.resources.every(row=>object(row)&&(row.kind==='Deployment'||row.kind==='Service'||row.kind==='Ingress'||row.kind==='ConfigMap')&&token(row.name,253)&&token(row.uid));
}
export const coordinatorKubernetesApi={
 prepare:async(projectId:string,input:{targetId:string;monthlyBudgetUsd:number;preparationId?:string},signal?:AbortSignal):Promise<CoordinatorKubernetesPlan>=>{
  if(input.preparationId!==undefined&&!/^prep-[a-f0-9]{32}$/.test(input.preparationId))throw new Error('Choose a saved reviewed preparation.');
  if(!token(input.targetId)||!Number.isFinite(input.monthlyBudgetUsd)||input.monthlyBudgetUsd<=0)throw new Error('Select the saved namespace target and a positive monthly budget.');
  const value:unknown=await productApi.request(`/v1/product/projects/${encodeURIComponent(projectId)}/orchestrator/kubernetes-plan`,input,'POST',30000,signal);
  if(!validCoordinatorKubernetesPlan(value,true)||value.targetId!==input.targetId||value.monthlyBudgetUsd!==input.monthlyBudgetUsd||value.preparation?.id!==input.preparationId)throw new Error('The current saved Kubernetes delegation could not be confirmed. Read and review it again; no delegation was saved.');
  return value;
 },
};
