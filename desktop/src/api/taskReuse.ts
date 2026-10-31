import { productApi } from './product';
export interface TaskReuseCitation { url: string; digest: string; observedAt: string }
export interface TaskReuseSource { id: string; name: string; url: string; description: string; framework: string; licence: string; licenceStatus: 'known'|'restricted'|'unknown'; licenceScope: string; licenceBasis: string; observedAt: string; catalogueHash: string; catalogueGeneratedAt: string; responseDigest: string; capturedAt: string; citations: TaskReuseCitation[]; citationCount: number; digest: string }
export interface TaskReuseDecision { source: TaskReuseSource; decision: 'investigate'|'use'|'reject'; rationale: string; licenceReview: string; compatibilityReview: string }
export interface ReuseCandidate { id: string; name: string; url: string; description: string; framework: string|null; license: { spdx: string|null; status: 'known'|'restricted'|'unknown'; scope: string; basis: string }; observedAt: string }
export interface ReuseSearch { projectId: string; catalogueHash: string; generatedAt: string; query: string; total: number; items: ReuseCandidate[] }
const object = (v: unknown): v is Record<string, unknown> => !!v && typeof v==='object' && !Array.isArray(v);
const text = (v: unknown, max: number): v is string => typeof v==='string' && new TextEncoder().encode(v).length<=max && !/[\u0000-\u0008\u000b\u000c\u000e-\u001f]/.test(v);
const hash = (v: unknown): v is string => typeof v==='string' && /^[a-f0-9]{64}$/.test(v);
const id = (v: unknown): v is string => typeof v==='string' && /^[a-f0-9]{20}$/.test(v);
const date = (v: unknown): v is string => typeof v==='string' && v.length<=40 && Number.isFinite(Date.parse(v));
export function reusePublicURL(value: unknown): value is string {
 if (!text(value,4000)) return false;
 try { const u=new URL(value), h=u.hostname.toLowerCase(); return u.href===value && u.protocol==='https:' && !u.username && !u.password && !u.port && !u.search && !u.hash && h.includes('.') && !h.endsWith('.') && !h.includes(':') && !/^\d+(?:\.\d+){3}$/.test(h) && !/(^|\.)(localhost|local|internal|test|invalid)$/.test(h); } catch { return false; }
}
export function validTaskReuseSource(v: unknown): v is TaskReuseSource {
 return object(v) && id(v.id) && text(v.name,200) && !!v.name.trim() && reusePublicURL(v.url) && text(v.description,1200) && text(v.framework,100) && text(v.licence,100) && ['known','restricted','unknown'].includes(v.licenceStatus as string) && (v.licence==='')===(v.licenceStatus==='unknown') && ['repository','registry-provider','registry-item','source-artifact','public-page'].includes(v.licenceScope as string) && ['upstream-metadata','source-declaration'].includes(v.licenceBasis as string) && date(v.observedAt) && date(v.catalogueGeneratedAt) && date(v.capturedAt) && hash(v.catalogueHash) && hash(v.responseDigest) && hash(v.digest) && Array.isArray(v.citations) && v.citations.length>=1 && v.citations.length<=3 && Number.isInteger(v.citationCount) && (v.citationCount as number)>=v.citations.length && (v.citationCount as number)<=21 && v.citations.every(c=>object(c)&&reusePublicURL(c.url)&&hash(c.digest)&&date(c.observedAt));
}
export function validTaskReuse(value: unknown, editing=false): value is TaskReuseDecision[]|undefined {
 if(value===undefined)return true;
 return Array.isArray(value)&&value.length<=4&&new Set(value.map(r=>r?.source?.id)).size===value.length&&value.every(r=>object(r)&&validTaskReuseSource(r.source)&&['investigate','use','reject'].includes(r.decision as string)&&text(r.rationale,2000)&&text(r.licenceReview,2000)&&text(r.compatibilityReview,2000)&&(editing||r.decision!=='use'||!!r.rationale.trim()&&!!r.licenceReview.trim()&&!!r.compatibilityReview.trim()));
}
function candidate(v: unknown): v is ReuseCandidate {return object(v)&&id(v.id)&&text(v.name,200)&&!!v.name.trim()&&reusePublicURL(v.url)&&text(v.description,1200)&&(v.framework===null||text(v.framework,100))&&date(v.observedAt)&&object(v.license)&&['known','restricted','unknown'].includes(v.license.status as string)&&(v.license.spdx===null||text(v.license.spdx,100));}
const route=(projectId:string)=>'/v1/product/projects/'+encodeURIComponent(projectId)+'/reuse/catalogue';
const invalid=()=>new Error('The catalogue response does not match this project and snapshot. Search again; no source was added.');
export const taskReuseApi={
 async search(projectId:string,query:string,signal?:AbortSignal):Promise<ReuseSearch>{
  const v:unknown=await productApi.request(route(projectId),{action:'search',query},'POST',12_000,signal);
  if(!object(v)||v.projectId!==projectId||!hash(v.catalogueHash)||!date(v.generatedAt)||v.query!==query||!Array.isArray(v.items)||v.items.length>12||!v.items.every(candidate)||new Set(v.items.map(x=>x.id)).size!==v.items.length||!Number.isInteger(v.total)||(v.total as number)<v.items.length||(v.total as number)>100000)throw invalid();return v as unknown as ReuseSearch;
 },
 async inspect(projectId:string,search:ReuseSearch,candidateId:string,signal?:AbortSignal):Promise<TaskReuseSource>{
  const expected=search.items.find(c=>c.id===candidateId);if(search.projectId!==projectId||!expected)throw invalid();
  const v:unknown=await productApi.request(route(projectId),{action:'inspect',id:candidateId,catalogueHash:search.catalogueHash},'POST',12_000,signal);
  if(!object(v)||v.projectId!==projectId||!validTaskReuseSource(v.source)||v.source.id!==candidateId||v.source.url!==expected.url||v.source.catalogueHash!==search.catalogueHash||v.source.catalogueGeneratedAt!==search.generatedAt)throw invalid();return v.source;
 }
};
