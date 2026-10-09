#!/usr/bin/env node
// Inert characterization: imports the frozen helpers unchanged. No browser,
// Docker, product runtime, private log, network request, or source rewrite.
import assert from 'node:assert/strict';
import {createHash} from 'node:crypto';
import {readFileSync, writeFileSync, statSync} from 'node:fs';
import {dirname, join} from 'node:path';
import {fileURLToPath, pathToFileURL} from 'node:url';

const output=dirname(fileURLToPath(import.meta.url));
const frozen='/workspace/.luci-tools/cloud072-installed-b94ad37e/evidence/cold-attempt2/driver/accepted-harness';
const paths={scenario:join(frozen,'scenario.mjs'),lifecycle:join(frozen,'launch-recovery/acceptance-lifecycle.mjs'),launcher:join(frozen,'launch.mjs')};
const digest=bytes=>createHash('sha256').update(bytes).digest('hex');
const identity=path=>{const bytes=readFileSync(path);return {path,bytes:bytes.length,sha256:digest(bytes)};};
const before=Object.fromEntries(Object.entries(paths).map(([key,path])=>[key,identity(path)]));
const {installTransportDiagnostics,installTerminalMonitor}=await import(pathToFileURL(paths.scenario));
const {boundedAttempt}=await import(pathToFileURL(paths.lifecycle));
const originalTimers={setTimeout:globalThis.setTimeout,clearTimeout:globalThis.clearTimeout};
const cases=[];
const unhandled=[];
const onUnhandled=()=>unhandled.push({kind:'unhandled-rejection'});
process.on('unhandledRejection',onUnhandled);
const flush=async()=>{for(let i=0;i<48;i++)await Promise.resolve();};
const deferred=()=>{let resolve,reject;const promise=new Promise((yes,no)=>{resolve=yes;reject=no;});return {promise,resolve,reject};};

class ManualClock {
 constructor(){this.now=0;this.next=0;this.timers=new Map();this.trace=[];}
 event(kind,detail={}){this.trace.push({kind,atMs:this.now,...detail});}
 setTimeout(fn,delay,...args){const id=++this.next;this.timers.set(id,{id,due:this.now+delay,fn,args});this.event('timer-registered',{id,delayMs:delay,dueMs:this.now+delay});return id;}
 clearTimeout(id){if(this.timers.delete(id))this.event('timer-cleared',{id});}
 elapseWithoutDispatch(ms){this.now+=ms;this.event('controlled-synchronous-elapse',{ms});}
 async advanceTo(target){assert(target>=this.now);for(;;){const timer=[...this.timers.values()].filter(x=>x.due<=target).sort((a,b)=>a.due-b.due||a.id-b.id)[0];if(!timer)break;this.timers.delete(timer.id);this.now=Math.max(this.now,timer.due);this.event('timer-fired',{id:timer.id,dueMs:timer.due});timer.fn(...timer.args);await flush();}this.now=Math.max(this.now,target);await flush();}
}

async function harness(kind,{pageOffDelay=0,detachDelay=0,detachThrows=false}={}){
 const clock=new ManualClock();
 globalThis.setTimeout=clock.setTimeout.bind(clock);
 globalThis.clearTimeout=clock.clearTimeout.bind(clock);
 const completion=deferred(),listeners=new Map(),pageListeners=new Map();
 const counts={detach:0,networkEnable:0,cdpOff:0,pageOff:0,privateErrorRecords:0};
 const client={
  on(name,fn){listeners.set(name,fn);},
  off(name,fn){assert.equal(listeners.get(name),fn);listeners.delete(name);counts.cdpOff++;clock.event('cdp-listener-removed');},
  send(name){assert.equal(name,'Network.enable');counts.networkEnable++;return Promise.resolve();},
  detach(){counts.detach++;clock.event('detach-invoked');if(detachDelay)clock.elapseWithoutDispatch(detachDelay);if(detachThrows)throw Error('controlled detach invocation failure');clock.event('detach-returned-promise');return completion.promise;},
 };
 const page={
  context(){return {newCDPSession:()=>Promise.resolve(client)};},
  addInitScript(){return Promise.resolve();},
  on(name,fn){pageListeners.set(name,fn);},
  off(name,fn){assert.equal(pageListeners.get(name),fn);pageListeners.delete(name);counts.pageOff++;clock.event('page-listener-removed');if(pageOffDelay)clock.elapseWithoutDispatch(pageOffDelay);},
  evaluate(){return Promise.resolve({version:1,failures:[],rows:[]});},
 };
 const options={origin:'ws://127.0.0.1:19876',base:'http://127.0.0.1:19876',attachPath:'/v1/sessions/s-1234abcd/attach',markers:{nonce:'controlled'},retainTransportError(){counts.privateErrorRecords++;throw Error('No private error traffic expected');}};
 const subject=kind==='monitor'?await installTerminalMonitor(page,options):await installTransportDiagnostics(page,options);
 assert.equal(counts.networkEnable,1);assert.equal(listeners.size,6);assert.equal(clock.timers.size,0);
 const installationTrace=clock.trace;clock.trace=[];
 let outcome={state:'pending'};
 const waiter=boundedAttempt(()=>subject.dispose(),2000,'transport-observer-dispose-timeout').then(
  ()=>{outcome={state:'fulfilled'};clock.event('outer-wait-fulfilled');},
  error=>{outcome={state:'rejected',name:error.name,code:error.code??null,message:error.message};clock.event('outer-wait-rejected',{code:error.code??null});},
 );
 await flush();
 return {clock,completion,counts,subject,installationTrace,waiter,get outcome(){return outcome;},async snapshot(){return await subject.snapshot();},restore(){globalThis.setTimeout=originalTimers.setTimeout;globalThis.clearTimeout=originalTimers.clearTimeout;}};
}

async function run(name,kind,options,operation){
 let h;
 try{
  h=await harness(kind,options);
  assert.equal(h.counts.cdpOff,6);assert.equal(h.counts.detach,1);
  if(kind==='monitor')assert.equal(h.counts.pageOff,1);
  await operation(h);
  await flush();
  assert.equal(h.clock.timers.size,0,'Every controlled timer must settle or clear.');
  assert.equal(h.counts.privateErrorRecords,0);
  cases.push({name,passed:true,kind,controlledOptions:options??{},counts:h.counts,outcome:h.outcome,installationTrace:h.installationTrace,disposalTrace:h.clock.trace,snapshot:await h.snapshot()});
 }catch(error){cases.push({name,passed:false,error:{name:error.name,code:error.code??null,message:error.message},disposalTrace:h?.clock.trace??[]});}
 finally{h?.restore();}
}

await run('direct CDP detach resolves before both deadlines','transport',{},async h=>{
 assert.equal(h.outcome.state,'pending');await h.clock.advanceTo(250);h.completion.resolve();await flush();
 assert.deepEqual(h.outcome,{state:'fulfilled'});assert.deepEqual(h.subject.snapshot().failures,[]);
 const t=h.clock.trace;assert(t.findIndex(x=>x.kind==='detach-invoked')<t.findIndex(x=>x.kind==='timer-registered'&&x.delayMs===1500));
 assert.equal(t.find(x=>x.kind==='timer-registered'&&x.delayMs===2000).atMs,0);
});
await run('full terminal monitor removes page and CDP listeners then resolves','monitor',{},async h=>{
 h.completion.resolve();await flush();assert.deepEqual(h.outcome,{state:'fulfilled'});
 const t=h.clock.trace;assert(t.findIndex(x=>x.kind==='page-listener-removed')<t.findIndex(x=>x.kind==='detach-invoked'));
});
await run('asynchronous detach rejection preserves inner failure classification','transport',{},async h=>{
 h.completion.reject(Error('controlled asynchronous detach failure'));await flush();
 assert.equal(h.outcome.state,'rejected');assert.equal(h.outcome.code,null);assert.equal(h.outcome.message,'Passive transport detach unconfirmed');
 assert.deepEqual(h.subject.snapshot().failures,['cdp-detach-unconfirmed']);
});
await run('never-settling detach reaches inner 1500 before outer 2000','monitor',{},async h=>{
 await h.clock.advanceTo(1499);assert.equal(h.outcome.state,'pending');await h.clock.advanceTo(1500);
 assert.equal(h.outcome.code,null);assert.equal(h.outcome.message,'Passive transport detach unconfirmed');
 await h.clock.advanceTo(2000);assert.equal(h.clock.trace.filter(x=>x.kind==='timer-fired').length,1);
 const p=await h.snapshot();assert.deepEqual(p.protocol.failures,['cdp-detach-unconfirmed']);
});
await run('controlled entry delay lets outer deadline precede inner; late ack cannot erase outer failure','monitor',{pageOffDelay:700},async h=>{
 const t=h.clock.trace;assert.equal(t.find(x=>x.kind==='timer-registered'&&x.delayMs===2000).dueMs,2000);
 assert.equal(t.find(x=>x.kind==='timer-registered'&&x.delayMs===1500).dueMs,2200);
 await h.clock.advanceTo(2000);assert.equal(h.outcome.code,'transport-observer-dispose-timeout');
 await h.clock.advanceTo(2100);h.completion.resolve();await flush();await h.clock.advanceTo(2200);
 assert.equal(h.outcome.code,'transport-observer-dispose-timeout');assert.deepEqual((await h.snapshot()).protocol.failures,[]);
});
await run('eager detach invocation precedes timer; late rejection preserves outer timeout','transport',{detachDelay:700},async h=>{
 const t=h.clock.trace;assert.equal(t.find(x=>x.kind==='detach-invoked').atMs,0);
 assert.equal(t.find(x=>x.kind==='timer-registered'&&x.delayMs===1500).atMs,700);
 await h.clock.advanceTo(2000);assert.equal(h.outcome.code,'transport-observer-dispose-timeout');
 await h.clock.advanceTo(2100);h.completion.reject(Error('controlled late detach rejection'));await flush();await h.clock.advanceTo(2200);
 assert.equal(h.outcome.code,'transport-observer-dispose-timeout');assert.deepEqual(h.subject.snapshot().failures,['cdp-detach-unconfirmed']);
});
await run('outer then inner deadlines remain failures after eventual detach acknowledgement','transport',{detachDelay:700},async h=>{
 await h.clock.advanceTo(2000);assert.equal(h.outcome.code,'transport-observer-dispose-timeout');
 await h.clock.advanceTo(2200);assert.deepEqual(h.subject.snapshot().failures,['cdp-detach-unconfirmed']);
 await h.clock.advanceTo(2300);h.completion.resolve();await flush();assert.equal(h.outcome.code,'transport-observer-dispose-timeout');
 assert.deepEqual(h.subject.snapshot().failures,['cdp-detach-unconfirmed']);assert.equal(h.clock.trace.filter(x=>x.kind==='timer-fired').length,2);
});
await run('synchronous detach throw occurs before inner timer registration','transport',{detachThrows:true},async h=>{
 assert.equal(h.outcome.code,null);assert.equal(h.outcome.message,'Passive transport detach unconfirmed');
 assert.deepEqual(h.subject.snapshot().failures,['cdp-detach-unconfirmed']);
 assert.equal(h.clock.trace.filter(x=>x.kind==='timer-registered'&&x.delayMs===1500).length,0);
});

await new Promise(resolve=>setImmediate(resolve));
process.off('unhandledRejection',onUnhandled);
const after=Object.fromEntries(Object.entries(paths).map(([key,path])=>[key,identity(path)]));
assert.deepEqual(after,before,'Frozen helper sources changed during characterization.');
assert.equal(unhandled.length,0);
const report={version:1,mode:'inert-controlled-CDP-characterization',passed:cases.every(x=>x.passed),cases,unhandledRejections:unhandled,
 scope:'Exact exported frozen helpers and boundedAttempt; fake page/CDP and manual timers. Synthetic 700ms clock advancement models delayed timer registration, not measured runtime delay. No product/browser/Docker run and no claim about actual timeout cause.'};
const bindings={sourceBefore:before,sourceAfter:after,allUnchanged:true,testSource:identity(fileURLToPath(import.meta.url)),
 importedExports:['installTransportDiagnostics','installTerminalMonitor','boundedAttempt'],sourceRewritten:false,
 scope:'Imports reference accepted cold-attempt2 files directly. No helper edits, extraction, source rewriting, or browser/CDP runtime.'};
const analysis={version:1,actualAttempt:'CLOUD-072 installed b94 cold-attempt2',actualRetained:{cleanupCode:'transport-observer-dispose-timeout',outerBudgetMs:2000,innerDetachBudgetMs:1500,
 disposalEntryTimestampRetained:false,detachInvocationTimestampRetained:false,innerTimerRegistrationTimestampRetained:false,detachSettlementTimestampRetained:false,postDisposeProtocolSnapshotRetained:false},
 conclusions:['With prompt entry and a pending detach promise, the exact inner 1500ms race rejects before the 2000ms outer waiter.',
 'The outer timer is registered before monitor execution. CDP detach is invoked eagerly before its inner timer is registered.',
 'Controlled delayed entry or eager-call delay can produce the outer code while both configured budgets remain unchanged; this is a mechanism characterization, not attribution of the real attempt.',
 'Neither timeout cancels detach. Late acknowledgement or rejection does not erase the already-observed outer failure.',
 'The actual receipts lack the disposal timings and final transport observation needed to distinguish causes. No budget increase is justified by these results.'],
 limits:['All CDP/page behavior is controlled in-process; no actual Chromium latency or event-loop scheduling was measured.',
 'The separate workflow-closing first callback has no retained original request method; no GET claim.',
 'No helper or product fix was applied. A future diagnostic should retain bounded state/timing for disposal entry, detach invocation, inner timer registration, timeout/settlement, and the outer wait while preserving both deadlines.']};
const write=(name,value)=>writeFileSync(join(output,name),JSON.stringify(value,null,2)+'\n',{mode:0o600,flag:'wx'});
write('report.json',report);write('source-bindings.json',bindings);write('analysis.json',analysis);
const members=['characterize.mjs','report.json','source-bindings.json','analysis.json'];
writeFileSync(join(output,'SHA256SUMS'),members.map(name=>digest(readFileSync(join(output,name)))+'  '+name+'\n').join(''),{mode:0o600,flag:'wx'});
console.log(JSON.stringify({passed:report.passed,cases:cases.length,passedCases:cases.filter(x=>x.passed).length,sourceUnchanged:true,sourceRewritten:false,output,reportSHA256:digest(readFileSync(join(output,'report.json')))}));
if(!report.passed)process.exitCode=1;
