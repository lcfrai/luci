import type { DiscoveryCorrection, DiscoveryOriginal } from './discoveryDocuments';
export type PendingDiscoveryRequest =
  | {action:'upload';idempotencyKey:string;documentId:string;expectedRevision:0;original:DiscoveryOriginal}
  | {action:'extract';idempotencyKey:string;documentId:string;expectedRevision:number}
  | {action:'cancel';idempotencyKey:string;documentId:string;expectedRevision:number;attemptId:string}
  | {action:'correct';idempotencyKey:string;documentId:string;expectedRevision:number;extractionRevision:number;changes:DiscoveryCorrection[]};
const key=(scope:string)=>`luci.discovery-document-request.v1:${encodeURIComponent(scope)}`;
const bytes=(value:string)=>new TextEncoder().encode(value).length;
export function discoveryPendingValid(value:unknown):value is PendingDiscoveryRequest {
  if(!value || typeof value!=='object' || Array.isArray(value))return false;
  const item=value as PendingDiscoveryRequest;
  if(!['upload','extract','cancel','correct'].includes(item.action) || typeof item.idempotencyKey!=='string' || !/^[A-Za-z0-9_-]{16,128}$/.test(item.idempotencyKey) || typeof item.documentId!=='string' || !Number.isSafeInteger(item.expectedRevision) || item.expectedRevision<0)return false;
  if(item.action==='upload')return item.expectedRevision===0 && !!item.original && typeof item.original.name==='string' && typeof item.original.mimeType==='string' && /^[a-f0-9]{64}$/.test(item.original.digest) && Number.isSafeInteger(item.original.bytes) && item.original.bytes>0 && item.original.bytes<=10*1024*1024;
  if(!/^doc-[a-f0-9]{64}$/.test(item.documentId) || item.expectedRevision<1)return false;
  if(item.action==='cancel')return typeof item.attemptId==='string' && /^extract-[a-f0-9]+$/.test(item.attemptId);
  if(item.action==='correct')return Number.isSafeInteger(item.extractionRevision) && item.extractionRevision>=1 && Array.isArray(item.changes) && item.changes.length>0 && item.changes.length<=200 && item.changes.every(change=>!!change && typeof change.segmentId==='string' && Number.isSafeInteger(change.page) && change.page>=1 && /^[a-f0-9]{64}$/.test(change.expectedTextDigest) && typeof change.text==='string' && bytes(change.text)<=64*1024 && typeof change.reason==='string' && bytes(change.reason)<=1982 && !!change.reason.trim());
  return true;
}
/** Exact pending authoring requests only; no credentials, raw uploads or provider consent. */
export function readDiscoveryRecovery(scope:string):PendingDiscoveryRequest|null {
  const raw=localStorage.getItem(key(scope));if(!raw)return null;
  if(bytes(raw)>1024*1024)throw new Error('The retained document request exceeds its recovery bound. Preserve it before replacing it.');
  const value:unknown=JSON.parse(raw);if(!discoveryPendingValid(value))throw new Error('The retained document request could not be read safely. Preserve it before continuing.');return value;
}
export function retainDiscoveryRecovery(scope:string,value:PendingDiscoveryRequest):void {
  if(!discoveryPendingValid(value))throw new Error('The document request cannot be retained safely.');const raw=JSON.stringify(value);
  if(bytes(raw)>1024*1024)throw new Error('The complete document request exceeds its recovery bound.');
  const previous=localStorage.getItem(key(scope));if(previous && previous!==raw)throw new Error('Another exact document request is awaiting readback in this project. Inspect it before continuing.');
  localStorage.setItem(key(scope),raw);
}
export function clearDiscoveryRecovery(scope:string,value:PendingDiscoveryRequest):void {
  const current=localStorage.getItem(key(scope));if(!current)return;if(current!==JSON.stringify(value))throw new Error('The retained request changed in another window. Reload its exact readback.');localStorage.removeItem(key(scope));
}
