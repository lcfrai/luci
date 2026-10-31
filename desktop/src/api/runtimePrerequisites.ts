import { validRuntimeResourcePlan,validRuntimeResourceOperation,resourceSelectionMatches,type RuntimeResourcePlan,type RuntimeResourceOperation,type RuntimeResourceSelection } from './runtimeResources';
import { productApi, type ImplementationTask } from './product';

export interface RuntimeTaskArtifact { path:string;content:string;sha256:string }
export interface TaskRuntimePrerequisite {preparationId:string;preparationDigest:string;serviceId:string;kind:'instrumentation'|'inference-client';artifacts:RuntimeTaskArtifact[];dependencies?:Record<string,string>}
export interface RuntimeRegistry {id:string;projectId:string;name:string;repository:string;revision:number;fingerprint:string}
export interface RuntimeBuildSelection {serviceId:string;dockerfile:string;packageJson?:string;artifactPaths?:Record<string,string>;integrationPaths?:string[];memoryMiB:number;cpuMillis:number}
export interface RuntimePreparationSummary {id:string;digest:string;blueprintRevision:number;services:string[];createdAt:string;finalized:boolean}
export interface RuntimeBuildReceipt {serviceId:string;runId:string;checkId:string;commit:string;sourceDigest:string;runInputDigest:string;checkDigest:string;attempted:true;runtime?:{phase:string;immutableImage?:string;resourcesHeld:boolean;cleanupConfirmed:boolean}}
export interface RuntimePreparationExecution {runId:string;checkId:string;commit:string;sourceDigest:string;runInputDigest:string;checkDigest:string;startedAt:string;finishedAt?:string;state:'running'|'attention'|'complete'}
export interface RuntimePreparation {
 execution?:RuntimePreparationExecution;
 plan:{resources?:RuntimeResourcePlan;id:string;projectId:string;actor:{userId:string;teamId?:string;projectId:string};blueprintRevision:number;blueprintDigest:string;contextDigest:string;registry:RuntimeRegistry;builds:{selection:RuntimeBuildSelection;port:number;artifacts:RuntimeTaskArtifact[];dependencies?:Record<string,string>}[];monthlyBudgetUsd:number;createdAt:string;expiresAt:string;digest:string};
 resources?:RuntimeResourceOperation;revision:number;builds:RuntimeBuildReceipt[];suggestedTasks:ImplementationTask[];savedBlueprintRevision?:number;savedBlueprintDigest?:string;
}
export interface RuntimePreparationInput {resources?:RuntimeResourceSelection;requestKey:string;blueprintRevision:number;blueprintDigest:string;registryId:string;registryRevision:number;builds:RuntimeBuildSelection[];monthlyBudgetUsd:number;acknowledgeBuildAndPush:boolean}
const object=(v:unknown):v is Record<string,unknown>=>!!v&&typeof v==='object'&&!Array.isArray(v);
const text=(v:unknown,max=240):v is string=>typeof v==='string'&&v.length>0&&v.length<=max&&!/[\0\r\n]/.test(v);
const hash=(v:unknown):v is string=>typeof v==='string'&&/^[a-f0-9]{64}$/.test(v);
const positive=(v:unknown):v is number=>Number.isSafeInteger(v)&&(v as number)>0&&(v as number)<=1e9;
const id=(v:unknown):v is string=>typeof v==='string'&&/^prep-[a-f0-9]{32}$/.test(v);
function registry(v:unknown,projectId:string):v is RuntimeRegistry{return object(v)&&typeof v.id==='string'&&/^registry-[a-f0-9]{32}$/.test(v.id)&&v.projectId===projectId&&text(v.name,120)&&text(v.repository)&&positive(v.revision)&&hash(v.fingerprint)&&!('password' in v)&&!('username' in v);}
function summary(v:unknown):v is RuntimePreparationSummary{return object(v)&&id(v.id)&&hash(v.digest)&&positive(v.blueprintRevision)&&Array.isArray(v.services)&&v.services.length>0&&v.services.length<=32&&v.services.every(x=>text(x,128))&&text(v.createdAt,64)&&Number.isFinite(Date.parse(v.createdAt))&&typeof v.finalized==='boolean';}
function artifact(v:unknown):v is RuntimeTaskArtifact{return object(v)&&text(v.path)&&typeof v.content==='string'&&v.content.length>0&&v.content.length<=16384&&hash(v.sha256);}
function selection(v:unknown):v is RuntimeBuildSelection{return object(v)&&text(v.serviceId,128)&&text(v.dockerfile)&&positive(v.memoryMiB)&&positive(v.cpuMillis)&&(v.packageJson===undefined||text(v.packageJson))&&(v.artifactPaths===undefined||object(v.artifactPaths)&&Object.keys(v.artifactPaths).length<=4&&Object.values(v.artifactPaths).every(x=>text(x)))&&(v.integrationPaths===undefined||Array.isArray(v.integrationPaths)&&v.integrationPaths.length<=16&&v.integrationPaths.every(x=>text(x)));}
function preparedBuild(v:unknown):v is RuntimePreparation['plan']['builds'][number]{return object(v)&&selection(v.selection)&&positive(v.port)&&Array.isArray(v.artifacts)&&v.artifacts.length<=4&&v.artifacts.every(artifact)&&(v.dependencies===undefined||object(v.dependencies)&&Object.keys(v.dependencies).length<=16&&Object.values(v.dependencies).every(x=>text(x,80)));}
function buildReceipt(v:unknown):v is RuntimeBuildReceipt{
 if(!object(v)||!text(v.serviceId,128)||!text(v.runId,128)||!text(v.checkId,128)||typeof v.commit!=='string'||!/^[a-f0-9]{40}(?:[a-f0-9]{24})?$/.test(v.commit)||!hash(v.sourceDigest)||!hash(v.runInputDigest)||!hash(v.checkDigest)||v.attempted!==true)return false;
 if(v.runtime===undefined)return true;const r=v.runtime;
 return object(r)&&typeof r.phase==='string'&&['blocked','reserved','building','built','publishing','published','unknown','failed','finished'].includes(r.phase)&&typeof r.resourcesHeld==='boolean'&&typeof r.cleanupConfirmed==='boolean'&&(r.immutableImage===undefined||typeof r.immutableImage==='string'&&/^[^\s@]+@sha256:[a-f0-9]{64}$/.test(r.immutableImage));
}
export function validRuntimePreparationExecution(value:unknown,row:RuntimePreparation):value is RuntimePreparationExecution{
 if(!object(value)||!text(value.runId,128)||!text(value.checkId,128)||typeof value.commit!=='string'||!/^[a-f0-9]{40}(?:[a-f0-9]{24})?$/.test(value.commit)||!hash(value.sourceDigest)||!hash(value.runInputDigest)||!hash(value.checkDigest)||!text(value.startedAt,64)||!Number.isFinite(Date.parse(value.startedAt))||!['running','attention','complete'].includes(value.state as string))return false;
 if(value.state==='running'&&value.finishedAt!==undefined||value.state!=='running'&&(!text(value.finishedAt,64)||!Number.isFinite(Date.parse(value.finishedAt))||Date.parse(value.finishedAt)<Date.parse(value.startedAt)))return false;
 if(value.state==='complete'&&!(Number.isSafeInteger(row.savedBlueprintRevision)&&row.savedBlueprintRevision!>row.plan.blueprintRevision))return false;
 return row.builds.length===row.plan.builds.length&&row.plan.builds.every(plan=>row.builds.some(b=>b.serviceId===plan.selection.serviceId&&b.runId===value.runId&&b.checkId===value.checkId&&b.commit===value.commit&&b.sourceDigest===value.sourceDigest&&b.runInputDigest===value.runInputDigest&&b.checkDigest===value.checkDigest));
}
export function validRuntimePreparation(v:unknown,projectId:string):v is RuntimePreparation{
 if(!object(v)||!object(v.plan)||!id(v.plan.id)||v.plan.projectId!==projectId||!object(v.plan.actor)||!text(v.plan.actor.userId,200)||v.plan.actor.projectId!==projectId||!positive(v.revision)||!positive(v.plan.blueprintRevision)||!hash(v.plan.blueprintDigest)||!hash(v.plan.contextDigest)||!hash(v.plan.digest)||!registry(v.plan.registry,projectId)||typeof v.plan.monthlyBudgetUsd!=='number'||!Number.isFinite(v.plan.monthlyBudgetUsd)||v.plan.monthlyBudgetUsd<=0||!text(v.plan.createdAt,64)||!text(v.plan.expiresAt,64)||!Number.isFinite(Date.parse(v.plan.createdAt))||Date.parse(v.plan.expiresAt)-Date.parse(v.plan.createdAt)!==86400000||!Array.isArray(v.plan.builds)||v.plan.builds.length===0&&v.plan.resources===undefined||v.plan.builds.length>8||!v.plan.builds.every(preparedBuild)||!Array.isArray(v.builds)||v.builds.length>8||!v.builds.every(buildReceipt)||!Array.isArray(v.suggestedTasks)||v.suggestedTasks.length>16)return false;
 if(v.plan.resources!==undefined&&!validRuntimeResourcePlan(v.plan.resources)||v.resources!==undefined&&(!validRuntimeResourcePlan(v.plan.resources)||!validRuntimeResourceOperation(v.resources,v.plan.resources,v.plan.id)))return false;
 if(v.execution!==undefined&&!validRuntimePreparationExecution(v.execution,v as unknown as RuntimePreparation))return false;
 if(v.execution!==undefined&&v.resources!==undefined){const execution=v.execution as RuntimePreparationExecution,source=(v.resources as RuntimeResourceOperation).source;if(source.runId!==execution.runId||source.checkId!==execution.checkId||source.commit!==execution.commit||source.sourceDigest!==execution.sourceDigest||source.runInputDigest!==execution.runInputDigest||source.checkDigest!==execution.checkDigest)return false;}
 const plan=v.plan,builds=plan.builds as RuntimePreparation['plan']['builds'];
 if(new Set(builds.map(b=>b.selection.serviceId)).size!==builds.length)return false;
 return v.suggestedTasks.every(t=>{
  if(!object(t)||!text(t.id,128)||!text(t.title,2000)||t.status!=='todo'||typeof t.acceptance!=='string'||!Array.isArray(t.writePaths??[]))return false;
  if(t.role==='reviewer')return text(t.reviewOfTaskId,128)&&!(t.writePaths as unknown[]|undefined)?.length;
  if(t.role!=='coding')return false;
  if(t.runtimePrerequisites===undefined)return builds.some(b=>Array.isArray(t.writePaths)&&t.writePaths.includes(b.selection.dockerfile));
  if(!Array.isArray(t.runtimePrerequisites)||t.runtimePrerequisites.length!==1)return false;
  return t.runtimePrerequisites.every(p=>object(p)&&p.preparationId===plan.id&&p.preparationDigest===plan.digest&&p.kind==='instrumentation'&&builds.some(b=>b.selection.serviceId===p.serviceId&&canonical(b.artifacts)===canonical(p.artifacts)&&canonical(b.dependencies??{})===canonical(p.dependencies??{})));
 });
}
async function result(v:unknown,projectId:string):Promise<RuntimePreparation>{
 if(!validRuntimePreparation(v,projectId))throw new Error('The exact runtime preparation could not be confirmed. Keep your draft and read current preparation.');
 for(const b of v.plan.builds)for(const a of b.artifacts){const bytes=new TextEncoder().encode(a.content);if(bytes.length>16384)throw new Error('A generated file exceeds its review limit.');if(!globalThis.crypto?.subtle)throw new Error('File hash verification is unavailable.');const got=await crypto.subtle.digest('SHA-256',bytes);if(Array.from(new Uint8Array(got),x=>x.toString(16).padStart(2,'0')).join('')!==a.sha256)throw new Error('A generated task file did not match its original hash.');}
 return v;
}
const canonical=(v:unknown):string=>JSON.stringify(v,(_,x:unknown)=>object(x)?Object.fromEntries(Object.keys(x).sort().map(k=>[k,x[k]])):x);
const path=(projectId:string,suffix:string)=>`/v1/product/projects/${encodeURIComponent(projectId)}/${suffix}`;
export const runtimePrerequisitesApi={
 async start(projectId:string,preparationId:string,input:{expectedRevision:number;preparationDigest:string;runId:string;checkId:string},signal?:AbortSignal):Promise<RuntimePreparation>{
  if(!id(preparationId)||!positive(input.expectedRevision)||!hash(input.preparationDigest)||!text(input.runId,128)||!text(input.checkId,128))throw new Error('Choose the reviewed preparation and checked project run.');
  const value=await result(await productApi.request(path(projectId,'runtime-preparations/'+encodeURIComponent(preparationId)+'/build'),input,'POST',30000,signal),projectId);
  if(value.plan.id!==preparationId||value.plan.digest!==input.preparationDigest||value.execution?.runId!==input.runId||value.execution?.checkId!==input.checkId)throw new Error('The build start was not confirmed. Read the original preparation before continuing.');
  return value;
 },
 async observe(projectId:string,preparationId:string,signal?:AbortSignal):Promise<RuntimePreparation>{
  if(!id(preparationId))throw new Error('Choose an original preparation.');
  const value=await result(await productApi.request(path(projectId,'runtime-preparations/'+encodeURIComponent(preparationId)+'/observe'),{},'POST',30000,signal),projectId);
  if(value.plan.id!==preparationId)throw new Error('The original preparation could not be confirmed.');
  return value;
 },

 async cleanup(projectId:string,preparation:RuntimePreparation,signal?:AbortSignal):Promise<RuntimePreparation>{const row=await result(await productApi.request(path(projectId,`runtime-preparations/${encodeURIComponent(preparation.plan.id)}/cleanup`),{preparationDigest:preparation.plan.digest},'POST',150000,signal),projectId);if(row.plan.id!==preparation.plan.id||row.plan.digest!==preparation.plan.digest)throw new Error('Original cleanup result did not match. Observe the retained preparation.');return row;},
 async registries(projectId:string,signal?:AbortSignal):Promise<RuntimeRegistry[]>{const v:unknown=await productApi.request(path(projectId,'runtime-registries'),undefined,'GET',30000,signal);if(!object(v)||v.projectId!==projectId||!Array.isArray(v.registries)||v.registries.length>32||!v.registries.every(r=>registry(r,projectId)))throw new Error('Registry setup could not be confirmed.');return v.registries;},
 async saveRegistry(projectId:string,input:{id?:string;expectedRevision:number;name:string;repository:string;username:string;password:string},signal?:AbortSignal):Promise<RuntimeRegistry>{const v:unknown=await productApi.request(path(projectId,'runtime-registries'),input,'POST',30000,signal);if(!registry(v,projectId)||v.repository!==input.repository||v.name!==input.name.trim()||v.revision!==input.expectedRevision+1)throw new Error('Registry setup was not confirmed. Read setup before trying again.');return v;},
 async list(projectId:string,signal?:AbortSignal):Promise<RuntimePreparationSummary[]>{const v:unknown=await productApi.request(path(projectId,'runtime-preparations'),undefined,'GET',30000,signal);if(!object(v)||v.projectId!==projectId||!Array.isArray(v.preparations)||v.preparations.length>128||!v.preparations.every(summary)||new Set(v.preparations.map(r=>r.id)).size!==v.preparations.length)throw new Error('Preparation history could not be confirmed.');return v.preparations;},
 async get(projectId:string,preparationId:string,signal?:AbortSignal):Promise<RuntimePreparation>{if(!id(preparationId))throw new Error('Choose a recorded preparation.');const value=await result(await productApi.request(path(projectId,`runtime-preparations/${encodeURIComponent(preparationId)}`),undefined,'GET',30000,signal),projectId);if(value.plan.id!==preparationId)throw new Error('The response did not match the original preparation.');return value;},
 async review(projectId:string,input:RuntimePreparationInput,signal?:AbortSignal):Promise<RuntimePreparation>{const v=await result(await productApi.request(path(projectId,'runtime-preparations'),input,'POST',30000,signal),projectId);if(v.plan.blueprintRevision!==input.blueprintRevision||v.plan.blueprintDigest!==input.blueprintDigest||v.plan.registry.id!==input.registryId||v.plan.registry.revision!==input.registryRevision||v.plan.monthlyBudgetUsd!==input.monthlyBudgetUsd||canonical(v.plan.builds.map(b=>b.selection))!==canonical(input.builds)||!resourceSelectionMatches(v.plan.resources,input.resources))throw new Error('The preparation did not match your exact selected inputs.');return v;},
};
