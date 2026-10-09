import * as controllerLogFS from 'node:fs';
import {createHash as controllerLogHash} from 'node:crypto';
import {dirname as controllerLogDirname,resolve as controllerLogResolve} from 'node:path';

const controllerLogLimit=256*1024,controllerLogLineLimit=16*1024,controllerLogRelayLimit=16;
const controllerLogDigest=bytes=>controllerLogHash('sha256').update(bytes).digest('hex');
const controllerLogSame=(a,b)=>a.dev===b.dev&&a.ino===b.ino;

// Decode Go slog/strconv quoted values to bytes. In particular, \\x escapes are
// bytes, not JavaScript Unicode code points. Raw/error text never leaves this module.
function controllerLogQuoted(token){
 if(token[0]!=='"'||token.at(-1)!=='"')return null;
 const parts=[];let text='';const flush=()=>{if(text){parts.push(Buffer.from(text));text='';}};
 const simple={a:7,b:8,f:12,n:10,r:13,t:9,v:11,'\\':92,'"':34};
 for(let i=1;i<token.length-1;i++){
  const c=token[i];if(c!=='\\'){if(c==='"'||c.charCodeAt(0)<32)return null;text+=c;continue;}
  flush();const e=token[++i];if(e in simple){parts.push(Buffer.from([simple[e]]));continue;}
  const count=e==='x'?2:e==='u'?4:e==='U'?8:0;if(!count)return null;
  const value=token.slice(i+1,i+1+count);if(value.length!==count||!new RegExp('^[0-9a-fA-F]{'+count+'}$').test(value))return null;i+=count;
  const n=parseInt(value,16);if(e==='x')parts.push(Buffer.from([n]));else{if(n>0x10ffff||(n>=0xd800&&n<=0xdfff))return null;parts.push(Buffer.from(String.fromCodePoint(n)));}
 }
 flush();return Buffer.concat(parts);
}
function controllerLogFields(line){
 let text;try{text=new TextDecoder('utf-8',{fatal:true}).decode(line);}catch{return null;}
 const fields=new Map();let i=0;
 while(i<text.length){
  while(text[i]===' ')i++;if(i===text.length)break;
  const keyStart=i;while(i<text.length&&text[i]!=='='&&text[i]!==' ')i++;
  if(text[i]!=='=')return null;const key=text.slice(keyStart,i++);if(!/^[A-Za-z][A-Za-z0-9_.-]{0,63}$/.test(key)||fields.has(key)||fields.size>=64)return null;
  const start=i;let bytes;
  if(text[i]==='"'){
   i++;let closed=false;while(i<text.length){if(text[i]==='\\'){i+=2;continue;}if(text[i++]==='"'){closed=true;break;}}
   if(!closed)return null;bytes=controllerLogQuoted(text.slice(start,i));if(!bytes)return null;
  }else{while(i<text.length&&text[i]!==' ')i++;if(start===i)return null;bytes=Buffer.from(text.slice(start,i));}
  if(i<text.length&&text[i]!==' ')return null;fields.set(key,{bytes,token:Buffer.from(text.slice(start,i))});
 }
 return fields;
}
export function projectAttachRelayLine(line,expectedSession){
 if(!Buffer.isBuffer(line)||line.length>controllerLogLineLimit||!/^[s]-[a-f0-9]{8}$/.test(expectedSession??''))return null;
 const fields=controllerLogFields(line);if(!fields)return null;
 const value=k=>fields.get(k)?.bytes.toString('utf8');
 if(value('msg')!=='attach relay ended'||value('route')!=='attach'||value('session')!==expectedSession)return null;
 const time=value('time'),level=value('level'),error=fields.get('err');
 if(!/^\d{4}-\d{2}-\d{2}T\d{2}:\d{2}:\d{2}(?:\.\d{1,9})?(?:Z|[+-]\d{2}:\d{2})$/.test(time??'')||!Number.isFinite(Date.parse(time))||!['DEBUG','INFO','WARN','ERROR'].includes(level)||!error)return null;
 let errorText;try{errorText=new TextDecoder('utf-8',{fatal:true}).decode(error.bytes);}catch{errorText='';}
 // Conservative classification of fixed terminal forms only. Quoted close
 // reasons are not searched for context/EOF text or retained in the projection.
 const terminal=/^(?:[a-z][a-z -]{0,80}: ){0,8}(context canceled|context deadline exceeded|EOF|unexpected EOF)$/.exec(errorText);
 const statuses={StatusNormalClosure:1000,StatusGoingAway:1001,StatusProtocolError:1002,StatusUnsupportedData:1003,StatusNoStatusRcvd:1005,StatusAbnormalClosure:1006,StatusInvalidFramePayloadData:1007,StatusPolicyViolation:1008,StatusMessageTooBig:1009,StatusMandatoryExtension:1010,StatusInternalError:1011,StatusServiceRestart:1012,StatusTryAgainLater:1013,StatusBadGateway:1014,StatusTLSHandshake:1015};
 const close=/^(?:[a-z][a-z -]{0,80}: ){0,8}status = (Status[A-Za-z]+) and reason = ("(?:[^"\\]|\\.)*")$/.exec(errorText);
 const status=close&&Object.hasOwn(statuses,close[1])&&controllerLogQuoted(close[2])?statuses[close[1]]:null;
 return {kind:'attach-relay-ended',time,level,session:expectedSession,route:'attach',lineBytes:line.length,lineSHA256:controllerLogDigest(line),errorBytes:error.bytes.length,errorSHA256:controllerLogDigest(error.bytes),errorTokenBytes:error.token.length,errorTokenSHA256:controllerLogDigest(error.token),rawErrorRetained:false,classificationIsTextOnly:true,flags:{contextCanceled:terminal?.[1]==='context canceled',deadlineExceeded:terminal?.[1]==='context deadline exceeded',eof:terminal?.[1]==='EOF',unexpectedEOF:terminal?.[1]==='unexpected EOF',websocketStatus:status,unclassified:!terminal&&status===null}};
}

export function createControllerLogCapture({path,expectedSession,onFailure=()=>{},onRelay=()=>{}}){
 if(typeof expectedSession!=='function'||typeof onFailure!=='function'||typeof onRelay!=='function')throw Error('Invalid private controller log callbacks.');
 let fd,parentIdentity,fileIdentity;
 const directory=typeof path==='string'?controllerLogDirname(path):'';
 const privateParent=()=>{const p=controllerLogFS.lstatSync(directory);if(!p.isDirectory()||p.isSymbolicLink()||p.uid!==process.getuid()||(p.mode&0o077)!==0||controllerLogFS.realpathSync(directory)!==directory)throw Error('Private controller log parent refused.');return p;};
 const fileOK=stat=>stat.isFile()&&stat.uid===process.getuid()&&stat.nlink===1&&(stat.mode&0o777)===0o600;
 try{
  if(typeof process.getuid!=='function'||typeof path!=='string'||path.length>4096||path.includes('\0')||controllerLogResolve(path)!==path)throw Error('Invalid private path.');
  parentIdentity=privateParent();const c=controllerLogFS.constants;fd=controllerLogFS.openSync(path,c.O_WRONLY|c.O_CREAT|c.O_EXCL|c.O_NOFOLLOW|c.O_APPEND|c.O_NONBLOCK,0o600);fileIdentity=controllerLogFS.fstatSync(fd);
  if(!fileOK(fileIdentity)||!controllerLogSame(fileIdentity,controllerLogFS.lstatSync(path))||!controllerLogSame(parentIdentity,privateParent()))throw Error('Private log identity changed.');
 }catch{if(fd!==undefined)try{controllerLogFS.closeSync(fd);}catch{}throw Error('Private controller log initialization failed.');}
 const observedHash=controllerLogHash('sha256'),retainedHash=controllerLogHash('sha256');
 const state={attached:false,endObserved:false,closeObserved:false,errorObserved:false,settlementTimedOut:false,hashCoverageComplete:true,observedBytes:0,retainedBytes:0,truncated:false,retentionClosedEarly:false,closed:false,fsyncSucceeded:null,identityVerified:null,completeLines:0,trailingPartialLine:false,oversizedLines:0,unprojectedLines:0,matchingLines:0,omittedRelayRows:0,relayRows:[],failures:[]};
 let finishResolve;const finished=new Promise(resolve=>{finishResolve=resolve;});let lineParts=[],lineBytes=0,discardLine=false,settlePromise;
 const failure=code=>{if(!state.failures.includes(code)){state.failures.push(code);try{onFailure({kind:'controller-log-capture-failure',code});}catch{if(!state.failures.includes('failure_callback_failed'))state.failures.push('failure_callback_failed');}}};
 function closeFile(){
  if(state.closed)return;
  try{const stat=controllerLogFS.fstatSync(fd),named=controllerLogFS.lstatSync(path),parent=privateParent();if(!fileOK(stat)||!fileOK(named)||!controllerLogSame(fileIdentity,stat)||!controllerLogSame(stat,named)||!controllerLogSame(parentIdentity,parent)||stat.size!==state.retainedBytes)throw Error('Identity mismatch');state.identityVerified=true;}catch{state.identityVerified=false;failure('private_file_identity_changed');}
  try{controllerLogFS.fsyncSync(fd);state.fsyncSucceeded=true;}catch{state.fsyncSucceeded=false;failure('private_file_sync_failed');}
  try{controllerLogFS.closeSync(fd);}catch{failure('private_file_close_failed');}state.closed=true;
  // File bytes are synced; directory entry durability is checked separately.
  let dirFD;try{dirFD=controllerLogFS.openSync(directory,controllerLogFS.constants.O_RDONLY|controllerLogFS.constants.O_DIRECTORY|controllerLogFS.constants.O_NOFOLLOW);if(!controllerLogSame(parentIdentity,controllerLogFS.fstatSync(dirFD)))throw Error('Parent changed');controllerLogFS.fsyncSync(dirFD);}catch{failure('private_parent_sync_failed');}finally{if(dirFD!==undefined)try{controllerLogFS.closeSync(dirFD);}catch{failure('private_parent_close_failed');}}
 }
 function emitLine(line){
  let id;try{id=expectedSession();}catch{failure('expected_session_unavailable');state.unprojectedLines++;return;}
  const projection=projectAttachRelayLine(line,id);if(!projection){state.unprojectedLines++;return;}
  state.matchingLines++;if(state.relayRows.length>=controllerLogRelayLimit){state.omittedRelayRows++;failure('relay_row_limit');return;}
  state.relayRows.push(projection);try{onRelay(structuredClone(projection));}catch{failure('relay_callback_failed');}
 }
 function processLines(chunk){
  let start=0;
  while(start<chunk.length){const newline=chunk.indexOf(10,start),end=newline===-1?chunk.length:newline,piece=chunk.subarray(start,end);
   if(!discardLine){if(lineBytes+piece.length>controllerLogLineLimit){discardLine=true;lineParts=[];lineBytes=0;state.oversizedLines++;failure('line_byte_limit');}else if(piece.length){lineParts.push(Buffer.from(piece));lineBytes+=piece.length;}}
   if(newline===-1)break;state.completeLines++;if(!discardLine)emitLine(Buffer.concat(lineParts,lineBytes));lineParts=[];lineBytes=0;discardLine=false;start=newline+1;
  }
 }
 function data(chunk){
  if(!Buffer.isBuffer(chunk)&&!(chunk instanceof Uint8Array)){state.hashCoverageComplete=false;failure('non_byte_stream_chunk');return;}
  const bytes=Buffer.isBuffer(chunk)?chunk:Buffer.from(chunk.buffer,chunk.byteOffset,chunk.byteLength);observedHash.update(bytes);state.observedBytes+=bytes.length;
  if(!state.closed){const amount=Math.min(bytes.length,controllerLogLimit-state.retainedBytes);let offset=0;try{while(offset<amount){const n=controllerLogFS.writeSync(fd,bytes,offset,amount-offset);if(n<=0)throw Error('No progress');retainedHash.update(bytes.subarray(offset,offset+n));state.retainedBytes+=n;offset+=n;}}catch{failure('private_file_write_failed');closeFile();}}
  if(state.observedBytes>state.retainedBytes){state.truncated=true;failure(state.retainedBytes===controllerLogLimit?'private_log_byte_limit':'private_log_prefix_incomplete');}
  try{processLines(bytes);}catch{failure('line_observer_failed');}
 }
 function end(){state.endObserved=true;if(lineBytes||discardLine){state.trailingPartialLine=true;lineParts=[];lineBytes=0;discardLine=false;failure('unterminated_log_line');}closeFile();finishResolve();}
 function streamError(){state.errorObserved=true;failure('stderr_stream_error');closeFile();finishResolve();}
 function streamClose(){state.closeObserved=true;if(!state.endObserved){failure('stderr_closed_without_end');closeFile();finishResolve();}}
 function snapshot(){
  const complete=state.endObserved&&!state.errorObserved&&state.hashCoverageComplete;
  return {version:1,scope:'Attached controller stderr only; private exact bounded prefix; hashes cover observed bytes, not unobserved process output',...structuredClone(state),observedSHA256:observedHash.copy().digest('hex'),completeStreamSHA256:complete?observedHash.copy().digest('hex'):null,retainedPrefixSHA256:retainedHash.copy().digest('hex'),maxRetainedBytes:controllerLogLimit,maxLineBytes:controllerLogLineLimit,maxRelayRows:controllerLogRelayLimit,passed:complete&&state.closed&&state.identityVerified===true&&state.fsyncSucceeded===true&&state.failures.length===0};
 }
 return {attach(stream){if(state.attached||state.closed)throw Error('Controller log capture already attached or closed.');if(!stream||typeof stream.on!=='function'||typeof stream.listenerCount!=='function'||stream.readableEnded||stream.destroyed||stream.readableFlowing===true||stream.listenerCount('data')){failure('stderr_attachment_unavailable');closeFile();finishResolve();throw Error('Controller stderr attachment unavailable.');}state.attached=true;stream.on('data',data);stream.once('end',end);stream.on('error',streamError);stream.once('close',streamClose);},snapshot,settle(ms=2000){if(!Number.isInteger(ms)||ms<1||ms>3000)throw Error('Controller log settlement budget must be1..3000ms.');if(settlePromise)return settlePromise;settlePromise=(async()=>{if(!state.attached){failure('stderr_not_attached');closeFile();finishResolve();}let timer;const timeout=new Promise(resolve=>{timer=setTimeout(()=>{state.settlementTimedOut=true;state.retentionClosedEarly=!state.endObserved;failure('stderr_settlement_timeout');closeFile();resolve();},ms);});try{await Promise.race([finished,timeout]);}finally{clearTimeout(timer);}return snapshot();})();return settlePromise;}};
}
