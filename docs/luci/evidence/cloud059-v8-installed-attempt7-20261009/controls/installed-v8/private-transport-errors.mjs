import * as transportPrivateFS from 'node:fs';
import {createHash as transportPrivateHash} from 'node:crypto';
import {dirname as transportPrivateDirname,resolve as transportPrivateResolve} from 'node:path';

// Private JSONL contains reversible JSON escaping of original error strings.
// Public callers receive only record ordinal/length/hash, never the raw text/path.
export function createPrivateTransportErrors({path,onFailure=()=>{}}){
 const limit=128*1024,maxTextBytes=2048,maxRows=32,parent=transportPrivateDirname(path),f=transportPrivateFS,c=f.constants;
 const same=(a,b)=>a.dev===b.dev&&a.ino===b.ino;let fd,fileIdentity,parentIdentity,closed=false,failed=false,rows=0,bytes=0;
 const digest=transportPrivateHash('sha256'),failures=[];
 const fail=code=>{failed=true;if(!failures.includes(code)){failures.push(code);try{onFailure({kind:'private-transport-error-capture-failure',code});}catch{failures.push('failure_callback_failed');}}};
 const validParent=()=>{const s=f.lstatSync(parent);if(!s.isDirectory()||s.isSymbolicLink()||s.uid!==process.getuid()||(s.mode&0o077)||f.realpathSync(parent)!==parent)throw Error('Private parent refused');return s;};
 const validFile=s=>s.isFile()&&s.nlink===1&&s.uid===process.getuid()&&(s.mode&0o777)===0o600;
 try{if(typeof process.getuid!=='function'||typeof path!=='string'||path.length>4096||path.includes('\0')||transportPrivateResolve(path)!==path)throw Error('Invalid private path');parentIdentity=validParent();fd=f.openSync(path,c.O_WRONLY|c.O_CREAT|c.O_EXCL|c.O_NOFOLLOW|c.O_APPEND|c.O_NONBLOCK,0o600);fileIdentity=f.fstatSync(fd);if(!validFile(fileIdentity)||!same(fileIdentity,f.lstatSync(path)))throw Error('Invalid private file');}catch{if(fd!==undefined)try{f.closeSync(fd);}catch{}throw Error('Private transport error file initialization failed');}
 const state={fsyncSucceeded:null,identityVerified:null,closeAcknowledged:false};
 function snapshot(){return {version:1,scope:'Private browser error text records only; no URLs, headers, stdout or credentials deliberately collected; arbitrary error strings may themselves contain secrets and remain private.',rows,bytes,sha256:digest.copy().digest('hex'),maxBytes:limit,maxTextBytes,maxRows,closed,failed,failures:[...failures],...state,passed:closed&&!failed&&state.identityVerified===true&&state.fsyncSucceeded===true&&state.closeAcknowledged};}
 return {record({channel,ordinal,error}){
  if(closed||failed){fail('private_capture_not_writable');throw Error('Private transport capture unavailable');}
  if(!['playwright','cdp'].includes(channel)||!Number.isInteger(ordinal)||ordinal<1||ordinal>8||typeof error!=='string'){fail('invalid_error_observation');throw Error('Private transport error observation invalid');}
  const raw=Buffer.from(error);if(raw.length>maxTextBytes||rows>=maxRows){fail(raw.length>maxTextBytes?'private_error_text_limit':'private_error_row_limit');throw Error('Private transport error capture limit');}
  const row={record:rows+1,channel,ordinal,wall:new Date().toISOString(),monotonicMs:performance.now(),errorBytes:raw.length,errorSHA256:transportPrivateHash('sha256').update(raw).digest('hex'),error};
  const data=Buffer.from(JSON.stringify(row)+'\n');if(bytes+data.length>limit){fail('private_error_file_limit');throw Error('Private transport error file limit');}
  let written=0;try{while(written<data.length){const n=f.writeSync(fd,data,written,data.length-written);if(n<=0)throw Error('No progress');digest.update(data.subarray(written,written+n));bytes+=n;written+=n;}f.fsyncSync(fd);}catch{fail('private_error_write_unconfirmed');throw Error('Private transport error write unconfirmed');}
  rows++;return {record:row.record,errorBytes:row.errorBytes,errorSHA256:row.errorSHA256,retainedPrivately:true};
 },snapshot,close(){if(closed)return snapshot();try{const named=f.lstatSync(path),current=f.fstatSync(fd);if(!validFile(named)||!validFile(current)||!same(fileIdentity,current)||!same(current,named)||current.size!==bytes||!same(parentIdentity,validParent()))throw Error('Identity changed');state.identityVerified=true;}catch{state.identityVerified=false;fail('private_error_identity_changed');}
  try{f.fsyncSync(fd);state.fsyncSucceeded=true;}catch{state.fsyncSucceeded=false;fail('private_error_sync_failed');}try{f.closeSync(fd);state.closeAcknowledged=true;}catch{fail('private_error_close_failed');}closed=true;let parentFD;try{parentFD=f.openSync(parent,c.O_RDONLY|c.O_DIRECTORY|c.O_NOFOLLOW);if(!same(parentIdentity,f.fstatSync(parentFD)))throw Error('Parent changed');f.fsyncSync(parentFD);}catch{fail('private_error_parent_sync_failed');}finally{if(parentFD!==undefined)try{f.closeSync(parentFD);}catch{fail('private_error_parent_close_failed');}}return snapshot();}};
}
