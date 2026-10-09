import assert from 'node:assert/strict';
import test from 'node:test';
import {readFileSync} from 'node:fs';
import {createHash} from 'node:crypto';
import {runInNewContext} from 'node:vm';
import {pathToFileURL} from 'node:url';
import {resolve} from 'node:path';
import {setImmediate as turn} from 'node:timers/promises';
const root=resolve(process.env.C071_SOURCE_ROOT??new URL('.',import.meta.url).pathname),variant=process.env.C071_VARIANT??'warm';
const path=root+'/'+variant+'/launch.mjs',source=readFileSync(path,'utf8'),{problemProjection}=await import(pathToFileURL(path));
const start=source.indexOf("page.on('response',"),end=source.indexOf(';receipt.browserVersion=browser.version()',start);
assert(start>=0&&end>start,'Exact installed page response listener required');
const code=source.slice(start,end);
const sha=bytes=>createHash('sha256').update(bytes).digest('hex');
const sessionId='s-1234abcd',projectId='p-5678abcd',pathName='/v1/sessions/'+sessionId+'/restart',base='http://127.0.0.1:8730',generation='b'.repeat(64);
const success={id:sessionId,projectId,harness:'shell',status:'starting',executionGeneration:generation};
const refusal={error:'conflict',message:'Available memory cannot preserve the configured headroom for another worker.',details:{reasonCode:'memory_pressure',stage:'admission',secret:'never-publish-this-private-value'}};
const raw=v=>Buffer.from(JSON.stringify(v));
function fixture({status=200,bytes=raw(success),body,requestChange,recordError=false,timeout=false}={}){
 const request={method:()=> 'POST',url:()=>base+pathName};requestChange?.(request);
 const browserPending=new Map([[request,'browser-1']]),pendingMutations=new Map([['browser-1',{method:'POST',path:pathName}]]),records=[],errors=[];
 const session={id:sessionId},project={id:projectId},browserResponseObservations=new Set(),browserResponseRequests=new WeakSet();let callback,active=true,reads=0,expire;
 const response={request:()=>request,status:()=>status,body:async()=>{reads++;return body?body():bytes;}};
 const boundedAttempt=async(operation,budget,code)=>{assert([5000,5500].includes(budget));const pending=Promise.resolve().then(operation);if(!timeout)return pending;const deadline=new Promise((_,reject)=>{expire=()=>reject(Object.assign(Error('controlled deadline'),{code}));});return Promise.race([pending,deadline]);};
 const sandbox={assert,Buffer,URL,sha,problemProjection,closing:false,acceptRestartResponses:true,browserResponseObservations,browserResponseRequests,browserPending,pendingMutations,session,project,base,stage:'ui-restart',page:{on:(event,fn)=>{assert.equal(event,'response');callback=fn;}},health:()=>{assert(active,'Workflow closing');},remaining:ms=>ms,boundedAttempt,record:value=>{if(recordError)throw Error('controlled durable-record failure');records.push(value);},fail:error=>errors.push(error)};
 runInNewContext(code,sandbox,{filename:path});
 return {request,response,browserResponseObservations,browserPending,pendingMutations,records,errors,session,project,get reads(){return reads;},close:()=>{active=false;sandbox.closing=true;},failWorkflow:()=>{active=false;},seal:()=>{sandbox.acceptRestartResponses=false;},drain:async()=>{const start=source.indexOf('  acceptRestartResponses=false;'),end=source.indexOf('  closing=true;',start);assert(start>=0&&end>start,'Bounded drain precedes browser closure');sandbox.receipt={cleanup:[]};sandbox.safeError=()=>({code:'controlled-drain-failure'});await runInNewContext('(async()=>{'+source.slice(start,end)+'closing=true;})()',sandbox);},expire:()=>{assert(expire,'deadline operation started');expire();},fire:async(r=response)=>{callback(r);await turn();await turn();}};
}
function retained(f){assert.equal(f.pendingMutations.size,1);assert.equal(f.browserPending.size,1);assert(!f.records.some(r=>r.kind==='restart-http-response'));}
function diagnostic(f){return f.records.find(r=>r.kind==='http-problem-diagnostic')?.problem;}

test('exact known 409 retains bounded allowlisted refusal before failing without clearing pending',async()=>{
 const bytes=raw(refusal),f=fixture({status:409,bytes});await f.fire();const value=diagnostic(f);
 assert(value,'known browser refusal lost its diagnostic before success assertion');
 assert.equal(f.reads,1);assert.equal(f.errors.length,1);assert.equal(value.status,409);assert.equal(value.error,'conflict');assert.equal(value.details.reasonCode,'memory_pressure');assert.equal(value.message,refusal.message);
 assert.equal(value.originalBodySHA256,sha(bytes));assert.equal(value.originalBodyBytes,bytes.length);assert.equal(value.originalBodyRetained,false);assert.equal(value.parsedJSON,true);assert(!JSON.stringify(f.records).includes(refusal.details.secret));assert(Buffer.byteLength(JSON.stringify(value))<=4096);retained(f);
});
for(const state of ['starting','running'])test('exact 200 '+state+' acknowledgment retains generation and settles only known request',async()=>{
 const f=fixture({bytes:raw({...success,status:state})});await f.fire();assert.equal(f.errors.length,0);assert.equal(f.pendingMutations.size,0);assert.equal(f.browserPending.size,0);assert.equal(f.records.length,1);assert.equal(f.records[0].kind,'restart-http-response');assert.equal(f.records[0].executionGeneration,generation);
});
for(const [name,change] of [['session',v=>v.id='s-ffffffff'],['project',v=>v.projectId='p-ffffffff'],['harness',v=>v.harness='codex'],['status',v=>v.status='exited'],['generation',v=>v.executionGeneration='not-an-exact-generation']])test('200 wrong '+name+' retains unresolved mutation',async()=>{
 const value={...success};change(value);const f=fixture({bytes:raw(value)});await f.fire();assert.equal(f.errors.length,1);retained(f);
});
for(const status of [200,409])test('malformed '+status+' retains hash-only bounded diagnosis and pending',async()=>{
 const bytes=Buffer.from('private malformed body which must never be published {'),f=fixture({status,bytes});await f.fire();const d=diagnostic(f);assert(d);assert.equal(d.parsedJSON,false);assert.equal(d.originalBodySHA256,sha(bytes));assert(!JSON.stringify(f.records).includes(bytes.toString()));assert(!String(f.errors[0]?.message).includes(bytes.toString()));retained(f);
});
test('empty refusal body retains an exact empty hash and unparsed diagnosis',async()=>{const f=fixture({status:409,bytes:Buffer.alloc(0)});await f.fire();assert.equal(diagnostic(f)?.originalBodySHA256,sha(Buffer.alloc(0)));assert.equal(diagnostic(f)?.parsedJSON,false);retained(f);});
test('oversized response records only bounded prefix hash and never parses or settles',async()=>{
 const bytes=Buffer.alloc(2*1024**2+1,65),f=fixture({status:409,bytes});await f.fire();const r=f.records.find(r=>r.bodyState==='oversized');assert(r);assert.equal(r.observedBodyBytes,bytes.length);assert.equal(r.hashedPrefixBytes,2*1024**2);assert.equal(r.hashedPrefixSHA256,sha(bytes.subarray(0,2*1024**2)));assert.equal(r.originalBodyRetained,false);assert(!('originalBodySHA256' in r));assert.equal(diagnostic(f),undefined);retained(f);
});
test('body transport rejection retains safe unavailability and pending without raw error',async()=>{
 const secret='private-response-error-secret',f=fixture({status:409,body:()=>Promise.reject(Error(secret))});await f.fire();assert.equal(f.records[0]?.bodyState,'unavailable');assert.equal(f.records[0]?.status,409);assert(!JSON.stringify(f.records).includes(secret));assert(!String(f.errors[0]?.message).includes(secret));retained(f);
});
test('bounded body timeout cannot later record or clear a successful late body',async()=>{
 let release;const f=fixture({timeout:true,body:()=>new Promise(resolve=>{release=resolve;})});await f.fire();assert.equal(f.reads,1);f.expire();await turn();await turn();assert.equal(f.errors.length,1);assert.equal(f.records[0]?.bodyState,'unavailable');const before=JSON.stringify(f.records);release(raw(success));await turn();await turn();assert.equal(JSON.stringify(f.records),before);retained(f);
});
test('late body rejection after timeout is consumed without a second failure or raw publication',async()=>{
 let reject;const f=fixture({timeout:true,body:()=>new Promise((_,r)=>{reject=r;})});await f.fire();f.expire();await turn();await turn();reject(Error('late-private-error'));await turn();await turn();assert.equal(f.errors.length,1);assert(!JSON.stringify(f.records).includes('late-private-error'));retained(f);
});
test('response arriving after workflow close does not read or settle',async()=>{const f=fixture();f.close();await f.fire();assert.equal(f.reads,0);assert.equal(f.records.length,0);retained(f);});
test('body resolving after workflow close does not record or settle',async()=>{let release;const f=fixture({body:()=>new Promise(r=>{release=r;})});await f.fire();f.close();release(raw(success));await turn();await turn();assert.equal(f.records.length,0);retained(f);});
for(const field of ['request map','pending path','session','project'])test('changed '+field+' during body observation refuses stale acknowledgment',async()=>{
 let release;const f=fixture({body:()=>new Promise(r=>{release=r;})});await f.fire();if(field==='request map')f.browserPending.set(f.request,'different-key');if(field==='pending path')f.pendingMutations.get('browser-1').path='/v1/sessions/s-other/restart';if(field==='session')f.session.id='s-other';if(field==='project')f.project.id='p-other';release(raw(success));await turn();await turn();assert.equal(f.errors.length,1);assert.equal(f.pendingMutations.size,1);assert(!f.records.some(x=>x.kind==='restart-http-response'));
});
for(const [name,change] of [['method',r=>r.method=()=> 'DELETE'],['origin',r=>r.url=()=> 'http://example.invalid'+pathName],['path',r=>r.url=()=>base+'/v1/sessions/s-other/restart'],['query',r=>r.url=()=>base+pathName+'?unexpected=1']])test('known object with wrong '+name+' refuses before body read',async()=>{const f=fixture({requestChange:change});await f.fire();assert.equal(f.reads,0);assert.equal(f.errors.length,1);retained(f);});
test('unknown response is ignored without reading or mutating known pending',async()=>{const f=fixture();await f.fire({...f.response,request:()=>({})});assert.equal(f.reads,0);assert.equal(f.errors.length,0);retained(f);});
test('durable acknowledgment record failure retains pending instead of certifying cleanup',async()=>{const f=fixture({recordError:true});await f.fire();assert.equal(f.errors.length,1);retained(f);});
test('handler has no request, replay, Docker, cleanup, or pending-clear refusal path',()=>{assert(!/fetch\(|execFile|spawn\(|docker\(|\.continue\(|\.abort\(/.test(code));assert(code.indexOf('problemProjection(')<code.indexOf("assert.equal(status,200"));assert(code.indexOf("assert.equal(status,200")<code.indexOf('pendingMutations.delete(key)'));});

test('accepted refusal survives concurrent scenario failure and drains before browser closes',async()=>{
 let release;const f=fixture({status:409,body:()=>new Promise(r=>{release=r;})});await f.fire();assert.equal(f.browserResponseObservations.size,1);f.failWorkflow();const drained=f.drain();release(raw(refusal));await drained;await turn();assert.equal(diagnostic(f)?.details.reasonCode,'memory_pressure');assert.equal(f.browserResponseObservations.size,0);retained(f);
});
test('accepted success after scenario failure cannot certify mutation settlement',async()=>{let release;const f=fixture({body:()=>new Promise(r=>{release=r;})});await f.fire();f.failWorkflow();const drained=f.drain();release(raw(success));await drained;assert.equal(f.errors.length,1);retained(f);});
test('duplicate response event reads and records exactly once',async()=>{let release;const f=fixture({status:409,body:()=>new Promise(r=>{release=r;})});await f.fire();await f.fire();assert.equal(f.reads,1);release(raw(refusal));await turn();await turn();assert.equal(f.records.filter(r=>r.kind==='http-problem-diagnostic').length,1);assert.equal(f.errors.length,1);retained(f);});
test('sealed response gate ignores new events without body observation',async()=>{const f=fixture();f.seal();await f.fire();assert.equal(f.reads,0);assert.equal(f.errors.length,0);assert.equal(f.records.length,0);retained(f);});
test('private problem message and details stay omitted while original bounded bytes remain hash-bound',async()=>{const secret='unknown-private-problem-text',bytes=raw({error:'conflict',message:secret,details:{reasonCode:'unknown-private-reason',opaque:secret}}),f=fixture({status:409,bytes});await f.fire();const d=diagnostic(f);assert.equal(d.originalBodySHA256,sha(bytes));assert.equal(d.message,null);assert.equal(d.messageOmitted,true);assert.equal(Object.keys(d.details).length,0);assert(!JSON.stringify(f.records).includes(secret));retained(f);});
test('malicious 200 identity cannot leak raw values through assertion diagnostics',async()=>{const secret='unknown-private-identity-value',f=fixture({bytes:raw({...success,id:secret})});await f.fire();assert.equal(f.errors.length,1);assert(!String(f.errors[0]?.message).includes(secret));assert(!JSON.stringify(f.records).includes(secret));retained(f);});

test('known refusal event after scenario failure still records diagnosis before the gate seals',async()=>{const f=fixture({status:409,bytes:raw(refusal)});f.failWorkflow();await f.fire();assert.equal(diagnostic(f)?.details.reasonCode,'memory_pressure');assert.equal(f.errors.length,1);retained(f);});
