import { productApi } from './product';
import type { PendingDiscoveryRequest } from './discoveryDocumentRecovery';

export type DiscoveryDocumentStatus = 'uploaded' | 'waiting-capacity' | 'extracting' | 'ready' | 'needs-review' | 'cancelled' | 'interrupted' | 'unsupported' | 'encrypted' | 'corrupt' | 'over-limit' | 'dependency-unavailable' | 'failed';
export interface DiscoveryActor {id: string; displayName: string}
export interface DiscoveryOriginal {digest: string; bytes: number; name: string; mimeType: string}
export interface DiscoveryLocation {x: number; y: number; width: number; height: number; firstLine: number; lastLine: number}
export interface DiscoverySegment {id: string; kind: 'text'|'ocr'|'correction'; text: string; textDigest: string; location: DiscoveryLocation; confidence: number|null; uncertainty: string}
export interface DiscoveryPage {page: number; width: number; height: number; segments: DiscoverySegment[]}
export interface DiscoveryExtraction {revision: number; attemptId: string; originalDigest: string; extractor: string; extractorVersion: string; status: DiscoveryDocumentStatus; pages: DiscoveryPage[]; issues: string[]; digest: string; correctsRevision?: number; at: string; actor: DiscoveryActor}
export interface DiscoveryAttempt {id: string; status: DiscoveryDocumentStatus; startedAt: string; expiresAt: string; finishedAt?: string; actor: DiscoveryActor}
export interface DiscoveryDocument {id: string; projectId: string; revision: number; original: DiscoveryOriginal; createdAt: string; createdBy: DiscoveryActor; attempts: DiscoveryAttempt[]; extractions: DiscoveryExtraction[]}
export interface DiscoverySourceLink {documentId: string; originalDigest: string; extractionRevision: number; page: number; segmentId: string; textDigest: string; targetKind: 'purpose'|'audience'|'constraint'|'requirement'|'glossary'|'decision'; targetId: string}
export interface DiscoveryCorrection {page: number; segmentId: string; expectedTextDigest: string; text: string; reason: string}
export const DISCOVERY_CORRECTION_REASON_BYTES = 2000-new TextEncoder().encode('Human correction: ').length;
export const DISCOVERY_LIMITS = {originalBytes:10*1024*1024, extractionBytes:1024*1024, pages:50, segments:10000} as const;
const path=(projectId:string)=>`/v1/product/projects/${encodeURIComponent(projectId)}/discovery/documents`;
const documentPath=(projectId:string,documentId:string)=>`${path(projectId)}/${encodeURIComponent(documentId)}`;
export const discoveryDocumentsApi = {
  list:(projectId:string,after='',signal?:AbortSignal)=>productApi.request<{documents:DiscoveryDocument[];nextCursor:string;omitted:boolean}>(`${path(projectId)}?${new URLSearchParams({after,limit:'25'})}`,undefined,'GET',30_000,signal),
  get:(projectId:string,documentId:string,signal?:AbortSignal)=>productApi.request<DiscoveryDocument>(documentPath(projectId,documentId),undefined,'GET',30_000,signal),
  upload:async(projectId:string,file:File,key:string,signal?:AbortSignal):Promise<DiscoveryDocument>=>{
    if(file.size<1 || file.size>DISCOVERY_LIMITS.originalBytes) throw new Error('Choose a document between 1 byte and 10 MiB.');
    const response=await productApi.requestBlob(path(projectId),{method:'POST',body:file,headers:{'Content-Type':file.type || 'application/octet-stream','X-Luci-Document-Name':encodeURIComponent(file.name),'Idempotency-Key':key}},60_000,signal);
    if(response.size>8*1024*1024) throw new Error('The private document response exceeded its bound. Inspect the retained upload before retrying.');
    const value:unknown=JSON.parse(await response.text()); if(!discoveryDocumentValid(value,projectId))throw new Error('The uploaded document receipt could not be verified. Inspect history before retrying.'); return value;
  },
  original:async(projectId:string,document:DiscoveryDocument,signal?:AbortSignal):Promise<Blob>=>{
    const response=await productApi.requestBlob(`${documentPath(projectId,document.id)}/original?digest=${document.original.digest}`,{},30_000,signal);
    if(response.size!==document.original.bytes || response.size>DISCOVERY_LIMITS.originalBytes || await discoveryDigest(new Uint8Array(await response.arrayBuffer()))!==document.original.digest)throw new Error('The original document no longer matches its private saved hash.'); return response;
  },
  start:(projectId:string,documentId:string,expectedRevision:number,idempotencyKey:string,signal?:AbortSignal)=>productApi.request<{document:DiscoveryDocument;attempt:DiscoveryAttempt}>(`${documentPath(projectId,documentId)}/extractions`,{expectedRevision,idempotencyKey},'POST',30_000,signal),
  cancel:(projectId:string,documentId:string,attemptId:string,expectedRevision:number,idempotencyKey:string,signal?:AbortSignal)=>productApi.request<DiscoveryDocument>(`${documentPath(projectId,documentId)}/extractions/${encodeURIComponent(attemptId)}`,{expectedRevision,idempotencyKey,cancel:true},'PUT',30_000,signal),
  correct:(projectId:string,documentId:string,expectedRevision:number,extractionRevision:number,idempotencyKey:string,changes:DiscoveryCorrection[],signal?:AbortSignal)=>productApi.request<DiscoveryDocument>(`${documentPath(projectId,documentId)}/corrections`,{expectedRevision,extractionRevision,idempotencyKey,changes},'POST',30_000,signal),
  discussion:(projectId:string,links:DiscoverySourceLink[],question:string,signal?:AbortSignal)=>productApi.request<{projectId:string;message:string;links:DiscoverySourceLink[]}>(`${path(projectId)}/discussion-context`,{links,question},'POST',30_000,signal),
  disposition:(projectId:string,request:PendingDiscoveryRequest,signal?:AbortSignal)=>productApi.request<{state:'recorded'|'rejected'|'unconfirmed';document:DiscoveryDocument|null;attemptId?:string;extractionRevision?:number}>(`${path(projectId)}/request-disposition`,request,'POST',30_000,signal),
};
export async function discoveryDigest(bytes:Uint8Array<ArrayBuffer>):Promise<string>{return Array.from(new Uint8Array(await crypto.subtle.digest('SHA-256',bytes))).map(value=>value.toString(16).padStart(2,'0')).join('');}
export function discoverySourceLinksValid(value:unknown):value is DiscoverySourceLink[] {
  return Array.isArray(value) && value.length<=200 && value.every(link=>!!link && typeof link==='object' && /^doc-[a-f0-9]{64}$/.test(link.documentId) && /^[a-f0-9]{64}$/.test(link.originalDigest) && Number.isSafeInteger(link.extractionRevision) && link.extractionRevision>=1 && link.extractionRevision<=128 && Number.isSafeInteger(link.page) && link.page>=1 && link.page<=50 && typeof link.segmentId==='string' && /^[A-Za-z0-9_-]{1,80}$/.test(link.segmentId) && /^[a-f0-9]{64}$/.test(link.textDigest) && ['purpose','audience','constraint','requirement','glossary','decision'].includes(link.targetKind) && typeof link.targetId==='string' && link.targetId.length<=100);
}
export function discoveryDocumentValid(value:unknown,projectId:string):value is DiscoveryDocument {
  if(!value || typeof value!=='object' || Array.isArray(value))return false;
  const item=value as DiscoveryDocument;
  const text=(value:unknown,max:number):value is string=>typeof value==='string' && new TextEncoder().encode(value).length<=max && !value.includes('\0');
  const integer=(value:unknown,min:number,max:number):value is number=>Number.isSafeInteger(value) && (value as number)>=min && (value as number)<=max;
  const actor=(value:DiscoveryActor|undefined)=>!!value && text(value.id,128) && !!value.id && text(value.displayName,512) && !!value.displayName;
  const status=(value:unknown)=>typeof value==='string' && ['extracting','ready','needs-review','cancelled','interrupted','unsupported','encrypted','corrupt','over-limit','dependency-unavailable','failed'].includes(value);
  if(!(item.projectId===projectId && /^doc-[a-f0-9]{64}$/.test(item.id) && integer(item.revision,1,Number.MAX_SAFE_INTEGER) && !!item.original && /^[a-f0-9]{64}$/.test(item.original.digest) && integer(item.original.bytes,1,DISCOVERY_LIMITS.originalBytes) && text(item.original.name,255) && text(item.original.mimeType,128) && actor(item.createdBy) && Array.isArray(item.attempts) && item.attempts.length<=128 && Array.isArray(item.extractions) && item.extractions.length<=128))return false;
  for(const attempt of item.attempts)if(!attempt || !text(attempt.id,80) || !status(attempt.status) || !actor(attempt.actor) || !text(attempt.startedAt,100) || !text(attempt.expiresAt,100))return false;
  for(const extraction of item.extractions){
    if(!extraction || !integer(extraction.revision,1,128) || extraction.originalDigest!==item.original.digest || !status(extraction.status) || !actor(extraction.actor) || !text(extraction.extractor,100) || !text(extraction.extractorVersion,100) || !Array.isArray(extraction.pages) || extraction.pages.length>DISCOVERY_LIMITS.pages || !Array.isArray(extraction.issues) || extraction.issues.length>30 || !extraction.issues.every(issue=>text(issue,2000)))return false;
    let segments=0;
    for(const page of extraction.pages){if(!page || !integer(page.page,1,50) || !Array.isArray(page.segments))return false;for(const segment of page.segments){
      if(++segments>DISCOVERY_LIMITS.segments || !segment || !text(segment.id,80) || !/^[a-f0-9]{64}$/.test(segment.textDigest) || !text(segment.text,64*1024) || !['text','ocr','correction'].includes(segment.kind) || segment.confidence!==null && !integer(segment.confidence,0,100) || !text(segment.uncertainty,2000) || !segment.location)return false;
      const box=segment.location;if(![box.x,box.y,box.width,box.height].every(value=>integer(value,0,10000)) || box.x+box.width>10000 || box.y+box.height>10000 || !integer(box.firstLine,0,1000000) || !integer(box.lastLine,box.firstLine,1000000))return false;
    }}
  }
  return true;
}
export function discoverySourceLink(document:DiscoveryDocument,extraction:DiscoveryExtraction,page:DiscoveryPage,segment:DiscoverySegment):DiscoverySourceLink {
  return {documentId:document.id,originalDigest:document.original.digest,extractionRevision:extraction.revision,page:page.page,segmentId:segment.id,textDigest:segment.textDigest,targetKind:'purpose',targetId:''};
}
