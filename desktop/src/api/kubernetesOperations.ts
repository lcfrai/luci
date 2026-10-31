import { productApi } from './product';
import type { KubernetesIdentity, KubernetesReview,StoredTelemetrySource } from './kubernetesDeployment';

export interface OperationalPod { componentId:string;name:string;uid:string;resourceVersion:string;deploymentUid:string;serviceUid:string;replicaSetUid:string;phase:'pending'|'running'|'succeeded'|'failed'|'unknown';ready:boolean }
export interface OperationalPods {identity:KubernetesIdentity;componentId:string;namespaceUid:string;observedAt:string;pods:OperationalPod[];omitted:boolean;evidenceDigest:string}
export interface LogQuery {componentId:string;podUid:string;tailLines:number;sinceSeconds:number;previous:boolean}
export interface OperationalLogs {identity:KubernetesIdentity;namespaceUid:string;pod:OperationalPod;query:LogQuery;observedAt:string;state:'recorded'|'empty'|'partial';text:string;textSha256:string;lineCount:number;truncated:boolean;evidenceDigest:string}
export interface TraceQuery {componentId:string;podUid:string;lookbackSeconds:number;ownedBackend?:boolean}
export interface TraceSummary {traceId:string;spanCount:number;errorSpans:number;firstSpanAt:string;lastSpanAt:string;matchingSpanWindowNanos:string}
export interface OperationalTraces {storage?:StoredTelemetrySource;identity:KubernetesIdentity;namespaceUid:string;pod:OperationalPod;query:TraceQuery;startAt:string;endAt:string;observedAt:string;state:'recorded'|'partial'|'no-data';traces:TraceSummary[];partial:boolean;evidenceDigest:string}
export interface TempoView {targetId:string;targetFingerprint:string;revision:number;configured:boolean;name:string;endpoint:string;updatedAt:string}
export interface TempoSave {expectedRevision:number;name:string;connection:{endpoint:string;caPem:string;token:string;tenant:string}}
export interface OperationalEvidence {
 version:1;id:string;revision:number;projectId:string;actor:{userId:string;projectId:string;teamId?:string;local?:false};reviewId:string;kind:'logs'|'traces';
 planDigest:string;sourceCommit:string;targetId:string;targetFingerprint:string;namespaceUid:string;componentId:string;podUid:string;deploymentUid:string;serviceUid:string;
 observedAt:string;state:'recorded'|'empty'|'partial'|'no-data';payloadDigest:string;observationDigest:string;lineCount:number;traceCount:number;spanCount:number;errorSpans:number;
 ownedBackend?:StoredTelemetrySource;partial:boolean;backendRevision:number;backendName:string;startAt:string;endAt:string;logQuery?:LogQuery;selected:boolean;selectedAt:string;
}
export interface LogResult {observation:OperationalLogs;evidence:OperationalEvidence}
export interface TraceResult {observation:OperationalTraces;evidence:OperationalEvidence;backend:TempoView}
export interface OperationalSelection {projectId:string;digest:string;historical:true;rows:{id:string;revision:number;kind:'logs'|'traces';componentId:string;observedAt:string}[]}
const object=(v:unknown):v is Record<string,unknown>=>!!v&&typeof v==='object'&&!Array.isArray(v);
const text=(v:unknown,max:number):v is string=>typeof v==='string'&&new TextEncoder().encode(v).length<=max;
const id=(v:unknown,max=128):v is string=>text(v,max)&&!!v&&!/[\x00-\x1f\x7f]/.test(v);
const count=(v:unknown,max=1e9):v is number=>Number.isSafeInteger(v)&&(v as number)>=0&&(v as number)<=max;
const hash=(v:unknown):v is string=>typeof v==='string'&&/^[a-f0-9]{64}$/.test(v);
const date=(v:unknown):v is string=>text(v,64)&&Number.isFinite(Date.parse(v));
const same=(a:unknown,b:unknown)=>JSON.stringify(a)===JSON.stringify(b);
const bad=()=>new Error('This operational observation could not be confirmed for the original workload. Refresh its saved deployment before continuing.');
function identity(v:unknown,review:KubernetesReview):v is KubernetesIdentity {return object(v)&&same(v,review.request.identity);}
export function validOperationalPod(v:unknown,review:KubernetesReview):v is OperationalPod {
 return object(v)&&id(v.componentId,63)&&review.request.blueprint.services.some(row=>row.id===v.componentId)&&id(v.name,253)&&id(v.uid)&&id(v.resourceVersion)&&id(v.deploymentUid)&&text(v.serviceUid,128)&&id(v.replicaSetUid)&&typeof v.phase==='string'&&['pending','running','succeeded','failed','unknown'].includes(v.phase)&&typeof v.ready==='boolean';
}
export function validOperationalEvidence(v:unknown,review:KubernetesReview):v is OperationalEvidence {
 if(object(v)&&v.ownedBackend!==undefined&&(!validOwnedTraceSource(v.ownedBackend,review)||v.kind!=='traces'||v.backendRevision!==1||v.backendName!=='Owned Tempo '+v.ownedBackend.serviceId))return false;
 if(!object(v)||v.version!==1||!id(v.id,80)||!/^ops-evidence-[a-f0-9]{64}$/.test(v.id)||!count(v.revision)||v.revision<1||v.projectId!==review.request.identity.projectId||!object(v.actor)||v.actor.userId!==review.request.identity.actorId||v.actor.projectId!==v.projectId||(v.actor.teamId??'')!==(review.request.identity.teamId??'')||v.actor.local!==undefined&&v.actor.local!==false||v.reviewId!==review.id||v.kind!=='logs'&&v.kind!=='traces'||v.planDigest!==review.request.identity.planDigest||v.sourceCommit!==review.request.blueprint.source.commit||v.targetId!==review.target.id||v.targetFingerprint!==review.target.fingerprint||v.namespaceUid!==review.target.namespaceUid||!id(v.componentId,63)||!review.request.blueprint.services.some(row=>row.id===v.componentId)||!id(v.podUid)||!id(v.deploymentUid)||!text(v.serviceUid,128)||!date(v.observedAt)||typeof v.state!=='string'||!['recorded','empty','partial','no-data'].includes(v.state)||!hash(v.payloadDigest)||!hash(v.observationDigest)||!count(v.lineCount,201)||!count(v.traceCount,4)||!count(v.spanCount,1024)||!count(v.errorSpans,1024)||v.errorSpans>v.spanCount||typeof v.partial!=='boolean'||!count(v.backendRevision)||!text(v.backendName,120)||!text(v.startAt,64)||!text(v.endAt,64)||typeof v.selected!=='boolean'||!text(v.selectedAt,64))return false;
 if(v.selected?!date(v.selectedAt):v.selectedAt!=='')return false;
 return v.kind==='logs'?v.traceCount===0&&v.spanCount===0&&v.errorSpans===0&&v.backendRevision===0&&v.backendName===''&&v.startAt===''&&v.endAt===''&&v.partial===true&&object(v.logQuery)&&v.logQuery.componentId===v.componentId&&v.logQuery.podUid===v.podUid&&count(v.logQuery.tailLines,200)&&v.logQuery.tailLines>0&&count(v.logQuery.sinceSeconds,3600)&&v.logQuery.sinceSeconds>0&&typeof v.logQuery.previous==='boolean':v.lineCount===0&&v.backendRevision>0&&!!v.backendName&&v.logQuery===undefined&&date(v.startAt)&&date(v.endAt)&&Date.parse(v.endAt)>=Date.parse(v.startAt);
}
function validOwnedTraceSource(v:unknown,review:KubernetesReview):v is StoredTelemetrySource{
 if(!object(v)||Object.keys(v).some(key=>!['serviceId','profile','deploymentUid','serviceUid','podUid','configUid','configVersion'].includes(key))||v.profile!=='tempo-2.10.10-azure'||!['serviceId','deploymentUid','serviceUid','podUid','configUid','configVersion'].every(key=>id(v[key])))return false;
 return review.request.blueprint.services.some(s=>s.id===v.serviceId&&s.telemetryStorage?.profile===v.profile)&&review.request.blueprint.services.some(s=>s.collector?.ownedDestination?.tracesServiceId===v.serviceId);
}
function validTempo(v:unknown,targetId:string):v is TempoView {
 return object(v)&&Object.keys(v).every(key=>['targetId','targetFingerprint','revision','configured','name','endpoint','updatedAt'].includes(key))&&v.targetId===targetId&&hash(v.targetFingerprint)&&count(v.revision)&&typeof v.configured==='boolean'&&text(v.name,120)&&text(v.endpoint,2048)&&text(v.updatedAt,64)&&(v.configured?v.revision>0&&!!v.name&&date(v.updatedAt):v.revision===0&&v.name===''&&v.endpoint===''&&v.updatedAt==='');
}
async function textDigest(text:string):Promise<string> {return [...new Uint8Array(await crypto.subtle.digest('SHA-256',new TextEncoder().encode(text)))].map(x=>x.toString(16).padStart(2,'0')).join('');}
const root=(projectId:string)=>`/v1/product/projects/${encodeURIComponent(projectId)}/kubernetes`;
const operation=(review:KubernetesReview)=>`${root(review.request.identity.projectId)}/deployments/${encodeURIComponent(review.id)}/operations`;
function selection(v:unknown,projectId:string):v is OperationalSelection {
 return object(v)&&v.projectId===projectId&&hash(v.digest)&&v.historical===true&&Array.isArray(v.rows)&&v.rows.length<=16&&v.rows.every(row=>object(row)&&typeof row.id==='string'&&/^ops-evidence-[a-f0-9]{64}$/.test(row.id)&&count(row.revision)&&row.revision>0&&(row.kind==='logs'||row.kind==='traces')&&id(row.componentId,63)&&date(row.observedAt))&&new Set(v.rows.map(row=>row.id)).size===v.rows.length;
}
export const kubernetesOperationsApi={
 selection:async(projectId:string,signal?:AbortSignal):Promise<OperationalSelection>=>{const v:unknown=await productApi.request(`${root(projectId)}/operations/evidence/selected`,undefined,'GET',30_000,signal);if(!selection(v,projectId))throw bad();return v;},
 clearSelection:async(projectId:string,expected:OperationalSelection,signal?:AbortSignal):Promise<OperationalSelection>=>{const v:unknown=await productApi.request(`${root(projectId)}/operations/evidence/selection/clear`,{expectedDigest:expected.digest},'POST',30_000,signal);if(!selection(v,projectId)||v.rows.length!==0)throw bad();return v;},
 pods:async(review:KubernetesReview,componentId:string,signal?:AbortSignal):Promise<OperationalPods>=>{
  const v:unknown=await productApi.request(`${operation(review)}/pods/read`,{componentId},'POST',95_000,signal);
  if(!object(v)||!identity(v.identity,review)||v.componentId!==componentId||v.namespaceUid!==review.target.namespaceUid||!date(v.observedAt)||!Array.isArray(v.pods)||v.pods.length>128||!v.pods.every(row=>validOperationalPod(row,review)&&row.componentId===componentId)||new Set(v.pods.map(row=>row.uid)).size!==v.pods.length||typeof v.omitted!=='boolean'||!hash(v.evidenceDigest))throw bad();return v as unknown as OperationalPods;
 },
 logs:async(review:KubernetesReview,query:LogQuery,acknowledged:boolean,signal?:AbortSignal):Promise<LogResult>=>{
  const v:unknown=await productApi.request(`${operation(review)}/logs/read`,{query,acknowledgeSensitiveLogs:acknowledged},'POST',95_000,signal);
  if(!object(v)||!object(v.observation)||!validOperationalEvidence(v.evidence,review))throw bad();const o=v.observation;
  if(!identity(o.identity,review)||o.namespaceUid!==review.target.namespaceUid||!validOperationalPod(o.pod,review)||o.pod.uid!==query.podUid||o.pod.componentId!==query.componentId||!object(o.query)||o.query.componentId!==query.componentId||o.query.podUid!==query.podUid||o.query.tailLines!==query.tailLines||o.query.sinceSeconds!==query.sinceSeconds||o.query.previous!==query.previous||!date(o.observedAt)||!['recorded','empty','partial'].includes(typeof o.state==='string'?o.state:'')||!text(o.text,64<<10)||!hash(o.textSha256)||!count(o.lineCount,201)||typeof o.truncated!=='boolean'||!hash(o.evidenceDigest)||v.evidence.kind!=='logs'||v.evidence.observationDigest!==o.evidenceDigest||v.evidence.payloadDigest!==o.textSha256||v.evidence.podUid!==o.pod.uid||v.evidence.lineCount!==o.lineCount||await textDigest(o.text)!==o.textSha256)throw bad();return v as unknown as LogResult;
 },
 traces:async(review:KubernetesReview,query:TraceQuery,signal?:AbortSignal):Promise<TraceResult>=>{
  const v:unknown=await productApi.request(`${operation(review)}/traces/read`,query,'POST',95_000,signal);
  if(!object(v)||!object(v.observation)||!validOperationalEvidence(v.evidence,review)||!validTempo(v.backend,review.target.id))throw bad();const o=v.observation;
  if(query.ownedBackend? !validOwnedTraceSource(o.storage,review)||!same(o.storage,v.evidence.ownedBackend)||v.backend.name!=='Owned Tempo '+o.storage.serviceId :o.storage!==undefined||v.evidence.ownedBackend!==undefined)throw bad();
  if(!identity(o.identity,review)||o.namespaceUid!==review.target.namespaceUid||!validOperationalPod(o.pod,review)||o.pod.uid!==query.podUid||o.pod.componentId!==query.componentId||!object(o.query)||o.query.componentId!==query.componentId||o.query.podUid!==query.podUid||o.query.lookbackSeconds!==query.lookbackSeconds||(o.query.ownedBackend??false)!==(query.ownedBackend??false)||!date(o.observedAt)||!date(o.startAt)||!date(o.endAt)||!['recorded','partial','no-data'].includes(typeof o.state==='string'?o.state:'')||!Array.isArray(o.traces)||o.traces.length>4||typeof o.partial!=='boolean'||!hash(o.evidenceDigest)||!o.traces.every(row=>object(row)&&typeof row.traceId==='string'&&/^[a-f0-9]{32}$/.test(row.traceId)&&count(row.spanCount,256)&&row.spanCount>0&&count(row.errorSpans,256)&&row.errorSpans<=row.spanCount&&date(row.firstSpanAt)&&date(row.lastSpanAt)&&typeof row.matchingSpanWindowNanos==='string'&&/^[0-9]{1,20}$/.test(row.matchingSpanWindowNanos))||new Set(o.traces.map(row=>(row as TraceSummary).traceId)).size!==o.traces.length||v.evidence.kind!=='traces'||v.evidence.observationDigest!==o.evidenceDigest||v.evidence.podUid!==o.pod.uid||v.evidence.traceCount!==o.traces.length||v.evidence.backendRevision!==v.backend.revision)throw bad();return v as unknown as TraceResult;
 },
 tempo:async(projectId:string,targetId:string,signal?:AbortSignal):Promise<TempoView>=>{const v:unknown=await productApi.request(`${root(projectId)}/targets/${encodeURIComponent(targetId)}/tempo-query`,undefined,'GET',30_000,signal);if(!validTempo(v,targetId))throw bad();return v;},
 saveTempo:async(projectId:string,targetId:string,input:TempoSave,signal?:AbortSignal):Promise<TempoView>=>{const v:unknown=await productApi.request(`${root(projectId)}/targets/${encodeURIComponent(targetId)}/tempo-query`,input,'PUT',30_000,signal);if(!validTempo(v,targetId)||!v.configured||v.revision!==input.expectedRevision+1||v.name!==input.name||v.endpoint!==input.connection.endpoint)throw bad();return v;},
 select:async(review:KubernetesReview,row:OperationalEvidence,selected:boolean,signal?:AbortSignal):Promise<OperationalEvidence>=>{
  const v:unknown=await productApi.request(`${root(row.projectId)}/operations/evidence/${encodeURIComponent(row.id)}/selection`,{expectedRevision:row.revision,selected},'POST',30_000,signal);
  if(!validOperationalEvidence(v,review)||v.id!==row.id||v.revision!==row.revision+1||v.selected!==selected||v.observationDigest!==row.observationDigest||v.payloadDigest!==row.payloadDigest||v.podUid!==row.podUid)throw bad();return v;
 },
};
