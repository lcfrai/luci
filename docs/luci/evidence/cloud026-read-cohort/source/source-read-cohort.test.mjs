// Controlled event/body/UI seams only. No browser, Docker, network or product.
import assert from 'node:assert/strict';
import {test} from 'node:test';
import {EventEmitter} from 'node:events';
import {readFileSync,writeFileSync,mkdtempSync,rmSync} from 'node:fs';
import {tmpdir} from 'node:os';
import {join} from 'node:path';
import {observeSourceReadCohort,sourceReadSuffixes,sourceReadUI} from './source-read-cohort.mjs';

const origin='http://fixture.invalid', projectPath='/v1/product/projects/p-a123', mutationPath=projectPath+'/runs/run-alpha/swarm/tasks/task-a/integrate', commit='a'.repeat(40);
const readyUI={factsVisible:true,factsReady:true,factsCurrentCommit:true,factsAlert:false,understandingVisible:true,understandingReady:true,understandingAlert:false};
const understandingBody={projectId:'p-a123',context:{projectId:'p-a123',sourceCommit:commit,sourceKnown:true,sourceDirty:false}};
const tick=()=>new Promise(resolve=>setImmediate(resolve));
function deferred(){let resolve,reject;const promise=new Promise((yes,no)=>{resolve=yes;reject=no;});return {promise,resolve,reject};}
function fixture(t,{timeoutMs=1000}={}){
 const page=new EventEmitter(), dir=mkdtempSync(join(tmpdir(),'luci-read-cohort-unit-')), path=join(dir,'cohort.json'), saves=[];
 let stopped=false;
 const cohort=observeSourceReadCohort({page,origin,projectPath,mutationPath,timeoutMs,assertActive:()=>{if(stopped)throw Error('workflow stopped');},persist:value=>{writeFileSync(path,JSON.stringify(value));saves.push(value);}});
 t.after(()=>{cohort.cancel();rmSync(dir,{recursive:true,force:true});});
 const request=(suffix,{method='GET',status=200,body,bytes,bodyGate,finishGate,otherProject=false}={})=>{
  const route=suffix===mutationPath?suffix:projectPath+suffix;
  const value=body??(suffix==='/git'?{clean:true,log:[{hash:commit}]}:suffix==='/orchestrator'?understandingBody:{projectId:'p-a123'});
  const response={status:()=>status,finished:async()=>finishGate?await finishGate.promise:null,body:async()=>bodyGate?await bodyGate.promise:bytes??Buffer.from(JSON.stringify(value))};
  return {method:()=>method,url:()=>origin+(otherProject?route.replace('p-a123','p-b456'):route),response:async()=>response};
 };
 const emit=request=>{page.emit('request',request);page.emit('requestfinished',request);return request;};
 const mutation=(options={})=>emit(request(mutationPath,{method:'POST',...options}));
 const reads=(options={})=>sourceReadSuffixes.filter(path=>path!==options.omit).map(path=>emit(request(path,options.byPath?.[path]??{})));
 return {page,cohort,saves,path,request,emit,mutation,reads,stop:()=>{stopped=true;cohort.cancel(Error('workflow stopped'));},read:()=>JSON.parse(readFileSync(path)),finish:readUI=>cohort.finish({commit,readUI:readUI??(async()=>readyUI)})};
}
function detached(f){for(const event of ['request','requestfinished','requestfailed','close'])assert.equal(f.page.listenerCount(event),0,event);}

test('one alpha mutation and all six full responses precede ready UI; preserves bounded request facts',async t=>{
 const f=fixture(t);f.mutation();f.reads();const result=await f.finish();
 assert.equal(result.phase,'ready');assert.equal(result.requests.length,6);assert.equal(result.mutation.method,'POST');
 for(const row of result.requests){assert.equal(row.method,'GET');assert.equal(row.status,200);assert.equal(row.state,'finished');assert(row.startedAt&&row.finishedAt);assert.match(row.bodySHA256,/^[a-f0-9]{64}$/);assert.equal(row.body,undefined);}
 assert.equal(result.stableUIBoundary.commit,commit);assert.deepEqual(result,f.read());detached(f);
});
test('response headers do not complete the cohort; delayed sixth body keeps UI unopened',async t=>{
 const f=fixture(t),gate=deferred();f.mutation();f.reads({byPath:{'/orchestrator':{bodyGate:gate}}});let inspected=0;
 const pending=f.finish(async()=>{inspected++;return readyUI;});await tick();assert.equal(inspected,0);assert.notEqual(f.read().phase,'ready');
 gate.resolve(Buffer.from(JSON.stringify(understandingBody)));await pending;assert.equal(inspected,1);detached(f);
});
test('requestfinished still requires successful response.finished before reading body',async t=>{
 const f=fixture(t),gate=deferred();f.mutation();f.reads({byPath:{'/orchestrator':{finishGate:gate}}});const pending=f.finish();await tick();
 assert.notEqual(f.read().phase,'ready');gate.resolve(Error('connection closed'));await assert.rejects(pending,/response-unfinished/);detached(f);
});
for(const [name,options] of [['busy409',{status:409}],['denied403',{status:403}],['wrong project body',{body:{projectId:'p-b456'}}],['invalid JSON',{bytes:Buffer.from('{')}],['oversized body',{bytes:Buffer.alloc(2*1024**2+1)}]]){
 test('failed cohort read remains failed: '+name,async t=>{
  const f=fixture(t);f.mutation();f.reads({byPath:{'/orchestrator':options}});await assert.rejects(f.finish());
  assert.equal(f.read().phase,'failed');assert.equal(f.read().stableUIBoundary,null);assert.equal(f.read().mutation.method,'POST');detached(f);
 });
}
test('a failed request is durable and cannot be supplied by another response',async t=>{
 const f=fixture(t);f.mutation();f.reads({omit:'/git'});const request=f.request('/git');f.page.emit('request',request);f.page.emit('requestfailed',request);
 await assert.rejects(f.finish(),/request-failed/);assert.equal(f.read().requests.find(row=>row.path.endsWith('/git')).state,'failed');detached(f);
});
for(const wrong of [false,true])test(wrong?'wrong-project request cannot fill missing sixth read':'missing sixth read fails within deadline',async t=>{
 const f=fixture(t,{timeoutMs:25});f.mutation();f.reads({omit:'/orchestrator'});if(wrong)f.emit(f.request('/orchestrator',{otherProject:true}));
 await assert.rejects(f.finish(),/deadline-exceeded/);assert.equal(f.read().requests.length,5);detached(f);
});
test('preexisting request completion and pre-mutation reads cannot satisfy current cohort',async t=>{
 const f=fixture(t,{timeoutMs:25}),old=f.request('/orchestrator');f.page.emit('request',old);f.emit(f.request('/brief'));f.mutation();f.reads({omit:'/orchestrator'});f.page.emit('requestfinished',old);
 await assert.rejects(f.finish(),/deadline-exceeded/);assert.equal(f.read().requests.length,5);detached(f);
});
test('duplicate same-project request fails rather than hiding an overlapping reader',async t=>{
 const f=fixture(t);f.mutation();f.reads();f.emit(f.request('/git'));await assert.rejects(f.finish(),/duplicate-request/);assert.equal(f.read().phase,'failed');detached(f);
});
test('mutation status failure cannot pass merely because all reads succeed',async t=>{
 const f=fixture(t);f.mutation({status:409});f.reads();await assert.rejects(f.finish(),/status-409/);assert.equal(f.read().mutation.status,409);detached(f);
});
test('changed Git commit cannot establish alpha completion',async t=>{
 const f=fixture(t);f.mutation();f.reads({byPath:{'/git':{body:{clean:true,log:[{hash:'b'.repeat(40)}]}}}});await assert.rejects(f.finish());assert.equal(f.read().phase,'failed');detached(f);
});
test('stale orchestrator source cannot establish current project understanding',async t=>{
 const f=fixture(t);f.mutation();f.reads({byPath:{'/orchestrator':{body:{...understandingBody,context:{...understandingBody.context,sourceCommit:'b'.repeat(40)}}}}});
 await assert.rejects(f.finish());assert.equal(f.read().stableUIBoundary,null);detached(f);
});
for(const ui of [{factsReady:false},{factsAlert:true},{factsVisible:false},{factsCurrentCommit:false},{understandingReady:false},{understandingAlert:true},{understandingVisible:false}]){
 test('source read-state remains required: '+JSON.stringify(ui),async t=>{
  const f=fixture(t);f.mutation();f.reads();await assert.rejects(f.finish(async()=>({...readyUI,...ui})));assert.equal(f.read().stableUIBoundary,null);detached(f);
 });
}
test('pending UI does not authorize stale probe before readiness',async t=>{
 const f=fixture(t),gate=deferred();f.mutation();f.reads();let downstream=0;
 const pending=f.finish(()=>gate.promise).then(()=>downstream++);await tick();assert.equal(f.read().phase,'waiting-for-ready-ui');assert.equal(downstream,0);
 gate.resolve(readyUI);await pending;assert.equal(downstream,1);detached(f);
});
test('cancelled late body rejection is consumed and cannot rewrite durable failure or reach UI',async t=>{
 const f=fixture(t),gate=deferred();f.mutation();f.reads({byPath:{'/orchestrator':{bodyGate:gate}}});let inspected=0;
 const pending=f.finish(async()=>{inspected++;return readyUI;});await tick();f.stop();await assert.rejects(pending,/workflow stopped/);const before=readFileSync(f.path,'utf8');
 gate.reject(Error('late body rejection'));await tick();assert.equal(inspected,0);assert.equal(readFileSync(f.path,'utf8'),before);detached(f);
});
test('cancelled late UI completion cannot start downstream mutation',async t=>{
 const f=fixture(t),gate=deferred();f.mutation();f.reads();let downstream=0;
 const pending=f.finish(()=>gate.promise).then(()=>downstream++);await tick();f.stop();await assert.rejects(pending,/workflow stopped/);const before=readFileSync(f.path,'utf8');
 gate.resolve(readyUI);await tick();assert.equal(downstream,0);assert.equal(readFileSync(f.path,'utf8'),before);detached(f);
});
test('page close cancels pending observer and removes every listener',async t=>{
 const f=fixture(t);f.mutation();const pending=f.finish();f.page.emit('close');await assert.rejects(pending,/page-closed/);detached(f);
});

// Exercise the exact browser predicate with minimal DOM interfaces. Its real
// source is passed unchanged to Playwright; these are not rendered UI fixtures.
test('actual readiness predicate rejects hidden, busy, stale and failed views',()=>{
 const oldDocument=globalThis.document, oldStyle=globalThis.getComputedStyle;
 const state={hidden:false,busy:false,stale:false,alert:false};
 const refresh=text=>({textContent:text,get disabled(){return state.busy;},getClientRects:()=>state.hidden?[]:[{}]});
 const factsButton=refresh('Refresh project facts'),understandingButton=refresh('Refresh project understanding');
 const facts={querySelectorAll:selector=>selector==='button'?[factsButton]:selector==='.project-work-facts code'?[{textContent:commit.slice(0,8)}]:[],querySelector:()=>state.alert?{}:null};
 const understanding={querySelectorAll:selector=>selector==='button'?[understandingButton]:state.stale?[{textContent:'Showing last recorded context. Refresh project understanding.'}]:[],querySelector:selector=>selector.startsWith('.understanding-next')?{}:null};
 globalThis.document={querySelector:selector=>selector.startsWith('details')?facts:understanding};globalThis.getComputedStyle=()=>({visibility:'visible'});
 try{assert.deepEqual(sourceReadUI({commit}),readyUI);for(const key of ['hidden','busy','stale']){state[key]=true;assert.equal(sourceReadUI({commit}),false);state[key]=false;}state.alert=true;assert.throws(()=>sourceReadUI({commit}),/ui-read-failed/);}
 finally{globalThis.document=oldDocument;globalThis.getComputedStyle=oldStyle;}
});

test('exact DAG arms after alpha review, fences mutation, waits cohort, and retains strict stale probe',()=>{
 const dag=readFileSync(new URL('./dag.mjs',import.meta.url),'utf8');
 const integrate=dag.slice(dag.indexOf('async function integrate('),dag.indexOf('\nasync function boundedResponse('));
 assert(integrate.indexOf('await readReview(key)')<integrate.indexOf('observeSourceReadCohort'));
 assert(integrate.indexOf('observeSourceReadCohort')<integrate.indexOf('await action(path'));
 assert(integrate.includes("if(key==='a')"));assert(integrate.includes('sourceReadCohort.finish'));assert(integrate.includes('page.waitForFunction(sourceReadUI'));
 assert(integrate.includes('assertCurrent();await facts.locator'));assert(integrate.includes('finally{sourceReadCohort=null;}'));
 const preProbe=dag.slice(dag.indexOf("const oldB=await readReview('b1')"),dag.indexOf("setStage('renewed-review-real-conflict')"));
 assert(preProbe.indexOf("alphaSourceReadCohort?.phase,'ready'")<preProbe.indexOf('await probeStaleReview'));
 assert.equal(preProbe.split("request:observe=>api('POST',stalePath").length,2);
 assert(preProbe.includes('409,30_000,observe'));assert(dag.includes('function stopOnFailure(error) { sourceReadCohort?.cancel(error);'));
});
