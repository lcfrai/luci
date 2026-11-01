import { validHTTPSID, validHTTPSKey, type HTTPSAction } from '../../api/publicHTTPS';

export type HTTPSPending = {kind:'review';reviewKey:string}|{kind:'action';reviewKey:string;id:string;digest:string;expectedRevision:number;requestKey:string;action:HTTPSAction};
export interface HTTPSRecovery {version:1;projectId:string;teamId:string;pending:HTTPSPending[]}
const object=(v:unknown):v is Record<string,unknown>=>!!v&&typeof v==='object'&&!Array.isArray(v);
export function validHTTPSRecovery(v:unknown,projectId:string,teamId:string):v is HTTPSRecovery {
  return object(v)&&Object.keys(v).every(k=>['version','projectId','teamId','pending'].includes(k))&&v.version===1&&v.projectId===projectId&&v.teamId===teamId&&Array.isArray(v.pending)&&v.pending.length>=1&&v.pending.length<=8&&v.pending.every(validPending)&&new Set(v.pending.map(p=>pendingKey(p as HTTPSPending))).size===v.pending.length;
}
function validPending(p:unknown):p is HTTPSPending {
  if(!object(p)||!validHTTPSKey(p.reviewKey))return false;
  if(p.kind==='review')return Object.keys(p).every(k=>['kind','reviewKey'].includes(k));
  return p.kind==='action'&&Object.keys(p).every(k=>['kind','reviewKey','id','digest','expectedRevision','requestKey','action'].includes(k))&&validHTTPSID(p.id)&&typeof p.digest==='string'&&/^[a-f0-9]{64}$/.test(p.digest)&&Number.isSafeInteger(p.expectedRevision)&&(p.expectedRevision as number)>0&&(p.expectedRevision as number)<=1e9&&validHTTPSKey(p.requestKey)&&typeof p.action==='string'&&['apply','observe','pause','resume','retire'].includes(p.action);
}
export function pendingKey(p:HTTPSPending):string{return p.kind==='review'?p.reviewKey:p.requestKey;}
export function readHTTPSRecovery(key:string,projectId:string,teamId:string):{raw:string|null;value:HTTPSRecovery|null} {
  const raw=localStorage.getItem(key);if(raw===null)return {raw,value:null};
  if(raw.length>8192)throw new Error('This browser’s HTTPS recovery record cannot be read. Keep the original record; do not resend a resource action.');
  const value:unknown=JSON.parse(raw);if(!validHTTPSRecovery(value,projectId,teamId))throw new Error('This browser’s HTTPS recovery record belongs to another scope or is invalid.');
  return {raw,value};
}
// Compare-and-read-back protects another tab's unresolved original. No expiry
// can turn an unknown resource write into permission to send another one.
export function writeHTTPSRecovery(key:string,expected:string|null,value:HTTPSRecovery|null):string|null {
  if(localStorage.getItem(key)!==expected)throw new Error('Another window changed HTTPS recovery. Reopen this project and read the saved original before continuing.');
  const next=value===null?null:JSON.stringify(value);
  if(next!==null&&(!validHTTPSRecovery(value,value!.projectId,value!.teamId)||next.length>8192))throw new Error('HTTPS recovery is full or invalid. Read original actions before starting another.');
  if(next===null)localStorage.removeItem(key);else localStorage.setItem(key,next);
  if(localStorage.getItem(key)!==next)throw new Error('The browser recovery change could not be confirmed. Reopen this project and read its original HTTPS record before continuing.');
  return next;
}
