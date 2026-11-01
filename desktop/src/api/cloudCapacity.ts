import { productApi } from './product';

export type CapacityHarness = 'codex' | 'claude-code';
export interface CapacityActor {userId:string;projectId:string;teamId?:string}
export interface CloudCapacityPolicy {
  id:string;actor:CapacityActor;harness:CapacityHarness;revision:number;
  profileDigest:string;catalogueDigest:string;maxHosts:number;hostSeconds:number;
  idleSeconds:number;budgetMicros:number;expiresAt:string;enabled:boolean;
}
export interface CloudCapacityOperator {
  parentHostId:string;profileDigest:string;maxHosts:number;minHostSeconds:number;bootstrapSeconds:number;
  hostCostMicrosPerHour:number;maxHostSeconds:number;budgetMicros:number;
}
export interface CloudCapacityHost {
  id:string;state:string;estimateMicros:number;createdAt:string;expiresAt:string;retired:boolean;
}
export interface CloudCapacityView {
  configured:boolean;reason?:string;operator?:CloudCapacityOperator;policy?:CloudCapacityPolicy;hosts:CloudCapacityHost[];
  spend?:{policyUsedMicros:number;policyRemainingMicros:number;operatorUsedMicros:number;operatorRemainingMicros:number;unknown:boolean;cumulative:true};
}
export interface CloudCapacityInput {
  harness:CapacityHarness;expectedRevision:number;maxHosts:number;hostSeconds:number;idleSeconds:number;
  budgetMicros:number;expiresAt:string;acknowledgeCost:boolean;reviewDigest?:string;reviewId?:string;
}
export interface CloudCapacityReview {policy:CloudCapacityPolicy;digest:string;reviewId:string}
const object=(v:unknown):v is Record<string,unknown>=>!!v&&typeof v==='object'&&!Array.isArray(v);
const text=(v:unknown,max=256):v is string=>typeof v==='string'&&v.length>0&&v.length<=max&&!/[\p{Cc}\p{Cf}]/u.test(v);
const digest=(v:unknown):v is string=>typeof v==='string'&&/^[a-f0-9]{64}$/.test(v);
const integer=(v:unknown,min=1):v is number=>Number.isSafeInteger(v)&&(v as number)>=min;
const timestamp=(v:unknown):v is string=>typeof v==='string'&&v.length<=40&&Number.isFinite(Date.parse(v));
const harness=(v:unknown):v is CapacityHarness=>v==='codex'||v==='claude-code';
export function capacityPolicyValid(v:unknown,projectId:string,actorId:string,teamId:string):v is CloudCapacityPolicy {
  return object(v)&&text(v.id)&&object(v.actor)&&v.actor.projectId===projectId&&v.actor.userId===actorId&&
    (v.actor.teamId??'')===teamId&&harness(v.harness)&&integer(v.revision)&&digest(v.profileDigest)&&digest(v.catalogueDigest)&&
    integer(v.maxHosts)&&integer(v.hostSeconds)&&integer(v.idleSeconds)&&v.idleSeconds<=v.hostSeconds&&integer(v.budgetMicros)&&timestamp(v.expiresAt)&&typeof v.enabled==='boolean';
}
export function capacityInputValid(v:unknown):v is CloudCapacityInput {
  return object(v)&&harness(v.harness)&&integer(v.expectedRevision,0)&&integer(v.maxHosts)&&integer(v.hostSeconds)&&
    integer(v.idleSeconds)&&v.idleSeconds<=v.hostSeconds&&integer(v.budgetMicros)&&timestamp(v.expiresAt)&&
    typeof v.acknowledgeCost==='boolean'&&(v.reviewDigest===undefined||digest(v.reviewDigest))&&(v.reviewId===undefined||text(v.reviewId));
}
export function capacityReviewValid(v:unknown,projectId:string,actorId:string,teamId:string):v is CloudCapacityReview {
  return object(v)&&capacityPolicyValid(v.policy,projectId,actorId,teamId)&&digest(v.digest)&&text(v.reviewId);
}
export function capacityViewValid(v:unknown,projectId:string,actorId:string,teamId:string):v is CloudCapacityView {
  if(!object(v)||typeof v.configured!=='boolean'||(v.reason!==undefined&&!text(v.reason,2048))||!Array.isArray(v.hosts)||
    !v.hosts.every(h=>object(h)&&text(h.id)&&text(h.state,60)&&integer(h.estimateMicros,0)&&timestamp(h.createdAt)&&timestamp(h.expiresAt)&&typeof h.retired==='boolean')||
    new Set(v.hosts.map(h=>h.id)).size!==v.hosts.length||v.policy!==undefined&&!capacityPolicyValid(v.policy,projectId,actorId,teamId))return false;
  const s=v.spend;
  if(s!==undefined&&(!object(s)||!integer(s.policyUsedMicros,0)||!integer(s.policyRemainingMicros,0)||!integer(s.operatorUsedMicros,0)||!integer(s.operatorRemainingMicros,0)||typeof s.unknown!=='boolean'||s.cumulative!==true))return false;
  if(!v.configured)return v.operator===undefined;
  const o=v.operator;
  return object(o)&&text(o.parentHostId)&&digest(o.profileDigest)&&integer(o.maxHosts)&&integer(o.minHostSeconds)&&integer(o.bootstrapSeconds)&&o.minHostSeconds>o.bootstrapSeconds&&
    integer(o.maxHostSeconds)&&o.maxHostSeconds>=o.minHostSeconds&&integer(o.hostCostMicrosPerHour)&&integer(o.budgetMicros);
}
export function sameCapacityPolicy(a:CloudCapacityPolicy,b:CloudCapacityPolicy):boolean {
  return a.id===b.id&&a.actor.userId===b.actor.userId&&a.actor.projectId===b.actor.projectId&&(a.actor.teamId??'')===(b.actor.teamId??'')&&
    a.harness===b.harness&&a.revision===b.revision&&a.profileDigest===b.profileDigest&&a.catalogueDigest===b.catalogueDigest&&
    a.maxHosts===b.maxHosts&&a.hostSeconds===b.hostSeconds&&a.idleSeconds===b.idleSeconds&&a.budgetMicros===b.budgetMicros&&
    Date.parse(a.expiresAt)===Date.parse(b.expiresAt)&&a.enabled===b.enabled;
}
export function reviewMatchesInput(r:CloudCapacityReview,input:CloudCapacityInput):boolean {
  const p=r.policy;
  return p.revision===input.expectedRevision+1&&p.harness===input.harness&&p.maxHosts===input.maxHosts&&
    p.hostSeconds===input.hostSeconds&&p.idleSeconds===input.idleSeconds&&p.budgetMicros===input.budgetMicros&&
    Date.parse(p.expiresAt)===Date.parse(input.expiresAt)&&p.enabled;
}
export function capacityMoney(value:string):number|null {
  if(!/^(?:0|[1-9]\d*)(?:\.\d{1,6})?$/.test(value))return null;
  const [whole,fraction='']=value.split('.');
  const amount=Number(whole)*1_000_000+Number(fraction.padEnd(6,'0'));
  return Number.isSafeInteger(amount)&&amount>0?amount:null;
}
export function capacityMoneyInput(micros:number):string {
  const whole=Math.floor(micros/1_000_000),fraction=String(micros%1_000_000).padStart(6,'0').replace(/0+$/,'');
  return String(whole)+(fraction?'.'+fraction:'');
}
const root=(projectId:string)=>'/v1/product/projects/'+encodeURIComponent(projectId)+'/cloud-capacity/automation';
export const cloudCapacityApi={
  async read(projectId:string,actorId:string,signal:AbortSignal,teamId=''):Promise<CloudCapacityView>{
    const v=await productApi.request<unknown>(root(projectId),undefined,'GET',20_000,signal);
    if(!capacityViewValid(v,projectId,actorId,teamId))throw new Error('Cloud capacity could not be read completely. Refresh before changing its limits.');
    return v;
  },
  async review(projectId:string,actorId:string,input:CloudCapacityInput,signal:AbortSignal,teamId=''):Promise<CloudCapacityReview>{
    if(!capacityInputValid(input)||input.acknowledgeCost)throw new Error('Review valid capacity limits before enabling them.');
    const v=await productApi.request<unknown>(root(projectId),input,'POST',20_000,signal);
    if(!capacityReviewValid(v,projectId,actorId,teamId)||!reviewMatchesInput(v,input))throw new Error('The capacity review did not match your requested limits.');
    return v;
  },
  async enable(projectId:string,actorId:string,input:CloudCapacityInput,expected:CloudCapacityReview,signal:AbortSignal,teamId=''):Promise<CloudCapacityReview>{
    if(!capacityReviewValid(expected,projectId,actorId,teamId)||!capacityInputValid(input)||!input.acknowledgeCost||input.reviewDigest!==expected.digest||input.reviewId!==expected.reviewId||!reviewMatchesInput(expected,input))throw new Error('The reviewed capacity request changed.');
    const v=await productApi.request<unknown>(root(projectId),input,'PUT',20_000,signal);
    if(!capacityReviewValid(v,projectId,actorId,teamId)||v.reviewId!==expected.reviewId||v.digest!==expected.digest||!sameCapacityPolicy(v.policy,expected.policy))throw new Error('The enable response could not be confirmed. Read the saved policy before continuing.');
    return v;
  },
  async disable(projectId:string,revision:number,harness:CapacityHarness,signal:AbortSignal):Promise<void>{
    const v=await productApi.request<unknown>(root(projectId)+'/disable',{expectedRevision:revision,harness},'POST',20_000,signal);
    if(!object(v)||v.disabled!==true)throw new Error('The stop response could not be confirmed. Refresh the saved policy.');
  },
};
