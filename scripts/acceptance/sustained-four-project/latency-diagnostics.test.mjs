// Controlled VM / fake transport only. No browser, service, model, or Docker.
import test from 'node:test';
import assert from 'node:assert/strict';
import vm from 'node:vm';
import {browserLatencyInit,browserFrameWitness,nodeStageRecorder,createLatencyLedger} from './latency-diagnostics.mjs';
import {interactionRunner,timingSnapshot} from './interaction-metrics.mjs';

function browser({autoFrames=false,resource=true}={}) {
 let now=1,deliver;const listeners=new Map(),frames=[];
 const context=vm.createContext({URL,Promise,performance:{timeOrigin:1000,now:()=>now+=.01},location:{href:'http://fixture.invalid/',origin:'http://fixture.invalid'},document:{visibilityState:'visible'},PerformanceObserver:class{static supportedEntryTypes=resource?['resource']:[];constructor(cb){deliver=entries=>cb({getEntries:()=>entries});}observe(){}},requestAnimationFrame:fn=>{if(autoFrames)queueMicrotask(()=>{now+=16;fn();});else frames.push(fn);}});
 context.window=context;context.addEventListener=(name,fn)=>listeners.set(name,fn);
 vm.runInContext('('+browserLatencyInit.toString()+')()',context);
 return {context,emit:(type,extra={})=>listeners.get(type)({isTrusted:true,...extra}),flush(){while(frames.length){now+=16;frames.shift()();}},resources:entries=>deliver(entries),snapshot:()=>JSON.parse(JSON.stringify(context.__luciAcceptanceLatency.snapshot())),evaluate:(fn,arg)=>vm.runInContext('('+fn.toString()+')(__argument)',Object.assign(context,{__argument:arg}))};
}
const resource=name=>({name,startTime:1,duration:3,requestStart:1.5,responseStart:2,responseEnd:4,transferSize:500,encodedBodySize:300,decodedBodySize:350});

test('trusted input and frames retain timestamps but no key, value, target or text',()=>{
 const b=browser();b.emit('input',{key:'private-key',data:'private-value',target:{value:'secret'}});b.flush();b.context.__luciAcceptanceLatency.markExpectedDOM('input');
 const first=b.snapshot();assert.deepEqual(first.rows.map(r=>r.kind),['input-dispatch','input-frame-1','input-frame-2','expected-dom']);assert.equal(first.rows[1].inputSequence,1);assert(!JSON.stringify(first).includes('private'));assert(!JSON.stringify(first).includes('secret'));assert(first.callbacks>=4);assert(first.callbackMs>0);
 assert.equal(b.snapshot().rows.length,0);assert.equal(b.snapshot().observed,4);
});
test('synthetic DOM events are excluded and pointer capture is text-free',()=>{
 const b=browser();b.emit('input',{isTrusted:false,data:'secret'});b.emit('pointerdown',{target:{textContent:'private'}});assert.deepEqual(b.snapshot().rows.map(r=>r.kind),['pointer-dispatch']);
});
test('route timing retains only exact same-origin classes and strips identities/query',()=>{
 const b=browser();b.resources([
  resource('http://fixture.invalid/v1/product/projects/private-project/specification?token=secret'),
  resource('http://fixture.invalid/v1/teams/team-abcdefghijklmnopqrstuv/workspace/v1/product/projects/private-project/overview?private=secret'),
  {...resource('http://fixture.invalid/v1/product/swarm-scheduler'),responseStatus:200},
  resource('http://fixture.invalid/v1/product/projects/private-project/swarm-queue'),
  resource('http://foreign.invalid/v1/product/projects/private-project/overview'),
  resource('http://fixture.invalid/v1/private/secret'),
 ]);const result=b.snapshot();assert.deepEqual(result.rows.map(r=>r.route),['project-specification','project-overview','swarm-scheduler','project-swarm-queue']);assert(!/secret|private-project|\/v1\/|http:/.test(JSON.stringify(result)));assert.equal(result.rows[0].transferSize,500);assert.equal(result.rows[0].responseStatus,null);assert.equal(result.rows[2].responseStatus,200);
});
test('unsupported resource observation stays explicit while input still works',()=>{
 const b=browser({resource:false});b.emit('input');b.flush();const s=b.snapshot();assert.equal(s.resourceUnsupported,true);assert.equal(s.rows.length,3);
});
test('2048 browser row cap is sticky across drains; no overflowing input frame scheduled',()=>{
 const b=browser();for(let i=0;i<2049;i++)b.emit('pointerdown');const first=b.snapshot();assert.equal(first.rows.length,2048);assert.equal(first.observed,2049);assert.equal(first.dropped,1);assert.equal(first.overflow,true);const next=b.snapshot();assert.equal(next.rows.length,0);assert.equal(next.overflow,true);assert.equal(next.dropped,1);
});
test('bounded ledger verifies monotonic per-document sequence and accounts bytes',()=>{
 const b=browser(),receipt={},ledger=createLatencyLedger(receipt);b.emit('input');ledger.append(b.snapshot());b.flush();ledger.append(b.snapshot());ledger.append(b.snapshot());assert.equal(receipt.latencyDiagnostics.rows,3);assert.equal(receipt.latencyDiagnostics.windows,3);assert.equal(receipt.latencyDiagnostics.status,'observed');assert(receipt.latencyDiagnostics.bytes>0);
 const c=browser();c.context.performance.timeOrigin=2000; // collector origin is fixed at installation
 const other=c.snapshot();other.timeOrigin=2000;ledger.append(other);assert.equal(receipt.latencyDiagnostics.documents,2);
});
for(const [label,mutate] of [['gap',v=>v.rows[0].sequence=2],['counter',v=>v.observed++],['overflow',v=>{v.overflow=true;v.dropped=1;}]])test('ledger refuses '+label,()=>{
 const b=browser(),receipt={},ledger=createLatencyLedger(receipt);b.emit('pointerdown');const row=b.snapshot();mutate(row);assert.throws(()=>ledger.append(row));assert.equal(receipt.latencyDiagnostics.valid,false);
});
for(const [label,limit] of [['rows',{maxRows:0}],['bytes',{maxBytes:1}],['windows',{maxWindows:0}],['documents',{maxDocuments:0}]])test('ledger refuses '+label+' bound',()=>{
 const b=browser(),receipt={},ledger=createLatencyLedger(receipt,{maxRows:50000,maxBytes:8*1024**2,maxWindows:2000,maxDocuments:128,...limit});b.emit('pointerdown');assert.throws(()=>ledger.append(b.snapshot()),/declared bound/);assert.equal(receipt.latencyDiagnostics.rows,0);
});
test('Node stage recorder freezes on timeout/close and never changes primary elapsed',()=>{
 let clock=10;const item={elapsedMs:42};const d=nodeStageRecorder(item,10,()=>++clock);d.mark('action-start');d.mark('action-end');d.close();d.mark('frames-end');d.frames({late:true});assert.equal(item.elapsedMs,42);assert.equal(item.diagnostics.nodeStages.length,2);assert.equal(item.diagnostics.browserFrames,null);assert.deepEqual(item.diagnostics.nodeStages.map(v=>v.offsetMs),[1,2]);
});
test('existing frame RPC returns two chronological callbacks in the browser clock',async()=>{
 const b=browser({autoFrames:true});b.emit('pointerdown');const value=await b.evaluate(browserFrameWitness);assert(value.requestedAt<value.firstFrameAt);assert(value.firstFrameAt<value.secondFrameAt);assert.equal(value.references.timeOrigin,1000);assert.equal(value.references.pointer.sequence,1);
});
test('one full round preserves action order and existing RPC count',async()=>{
 const b=browser({autoFrames:true}),calls=[],field={value:'Expected goal',isConnected:true};b.context.document.activeElement=field;
 const record=(name)=>calls.push(name);
 const textbox={waitFor:async()=>record('field-wait'),evaluate:async(fn,value)=>{record('field-evaluate');b.context.__field=field;b.context.__value=value;return vm.runInContext('('+fn.toString()+')(__field,__value)',b.context);},inputValue:async()=>{record('input-value');return field.value;},focus:async()=>record('focus'),press:async key=>{record('press-'+key);if(key==='x')field.value+='x';if(key==='Backspace')field.value=field.value.slice(0,-1);if(key!=='End')b.emit('input');}};
 const page={getByRole(role,{name}){if(role==='textbox')return textbox;if(role==='dialog')return {getByRole:()=>({click:async()=>record('discard')})};return {click:async()=>record('click-'+name),waitFor:async()=>record('heading')};},evaluate:async(fn)=>{record('frames');return b.evaluate(fn);},waitForFunction:async(fn,value)=>{record('wait-value');assert(await b.evaluate(fn,value));}};
 const receipt={},runner=interactionRunner({page,owners:[{owner:{id:'p1',name:'One'},goal:'Expected goal'}],receipt,assertHealthy(){},phase:()=> 'controlled'});await runner.round(0);
 assert.deepEqual(calls,['click-All projects','heading','click-Open project One','field-wait','field-evaluate','input-value','frames','focus','press-End','press-x','wait-value','frames','press-Backspace','wait-value','frames','click-Design','discard','click-Discover','input-value','frames']);
 assert.deepEqual(receipt.interactions.map(r=>r.kind),['project-switch','input','input','draft-reset']);assert(receipt.interactions.every(r=>r.success&&r.elapsedMs>=0&&r.diagnostics.browserFrames));
 assert.deepEqual(receipt.interactions[0].diagnostics.nodeStages.map(r=>r.stage),['action-start','all-projects-click-end','overview-heading-ready','open-project-click-end','action-end','verify-start','verify-end','frames-start','frames-end']);
 assert.equal(field.value,'Expected goal');assert.equal(b.context.__luciAcceptanceLatency.references().expectedDOM.actionKind,'input');
});
test('snapshot uses one existing RPC and keeps passive timing plus diagnostics',async()=>{
 const b=browser();b.context.__luciAcceptanceTiming={version:2,events:[{sequence:1}],observedEvents:1};b.emit('pointerdown');let calls=0;const snapshot=await timingSnapshot({evaluate:fn=>{calls++;return b.evaluate(fn);}});assert.equal(calls,1);assert.equal(snapshot.events.length,1);assert.equal(snapshot.latency.rows.length,1);assert.equal(b.context.__luciAcceptanceTiming.events.length,0);
});
