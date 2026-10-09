// Pure fake protocol callbacks only; no real browser, app, process signal or Docker.
import {test} from 'node:test';
import assert from 'node:assert/strict';
import {mkdtempSync,rmSync,readFileSync} from 'node:fs';
import {tmpdir} from 'node:os';
import {join} from 'node:path';
import {EventEmitter} from 'node:events';
import {createPrimaryProfiler,LIMITS} from './primary-profiler.mjs';

function fixture(t,overrides={}){
 const root=mkdtempSync(join(tmpdir(),'luci-profiler-pure-'));t.after(()=>rmSync(root,{recursive:true,force:true}));
 const directory=join(root,'profile'),calls=[],events=new EventEmitter();let detached=0;
 const session=Object.assign(events,{detach:async()=>{detached++;},send:async(method,params)=>{
  calls.push({method,params});if(overrides.send)return overrides.send(method,params,events);
  if(method==='Tracing.end')events.emit('Tracing.tracingComplete',{stream:'fixture-stream',dataLossOccurred:false});
  if(method==='Profiler.stop')return{profile:{nodes:[],startTime:1,endTime:2,samples:[],timeDeltas:[]}};
  if(method==='IO.read')return{data:'{"traceEvents":[]}',eof:true};return{};
 }});
 const page={context:()=>({newCDPSession:overrides.sessionFactory??(async()=>session)}),evaluate:async()=>({timeOrigin:1,now:2})};
 const controller=createPrimaryProfiler({page,directory,limits:{...LIMITS,commandMs:25,finishMs:120,captureMs:5000,...overrides.limits}});
 return{controller,directory,calls,events,session,get detached(){return detached;}};
}
test('complete fake primary capture preserves two private exact artifacts',async t=>{
 const f=fixture(t);await f.controller.start();const result=await f.controller.finish();
 assert.equal(result.complete,true);assert.equal(result.transportDetached,true);assert.equal(result.pendingCommands,0);assert.equal(f.detached,1);
 assert.deepEqual(Object.keys(result.artifacts).sort(),['browser-cpu.cpuprofile','browser-timeline.json']);
 assert.equal(JSON.parse(readFileSync(join(f.directory,'profiling-status.json'))).complete,true);
 assert.equal(f.calls.find(v=>v.method==='Profiler.setSamplingInterval').params.interval,5000);
 assert.equal(f.calls.find(v=>v.method==='Tracing.start').params.traceConfig.recordMode,'recordUntilFull');
});
test('cleanup is idempotent and does not claim completion for failed workflow',async t=>{
 const f=fixture(t);await f.controller.start();const a=f.controller.finish('workflow-finally'),b=f.controller.finish();assert.equal(a,b);
 assert.equal((await a).complete,false);assert.equal(f.calls.filter(v=>v.method==='Profiler.stop').length,1);
});
test('never-resolving session startup is bounded; a late handle is detached',async t=>{
 let resolve;const f=fixture(t,{sessionFactory:()=>new Promise(done=>{resolve=done;})});
 await assert.rejects(f.controller.start(),/session deadline/);await f.controller.finish('startup-failed');resolve(f.session);
 await new Promise(done=>setTimeout(done,1));assert.equal(f.detached,1);assert.equal(f.controller.status.complete,false);assert.equal(f.calls.length,0);
});
test('nonsettling end does not prevent bounded independent detach',async t=>{
 const f=fixture(t,{send:async(method)=>{
  if(method==='Tracing.end')return new Promise(()=>{});
  if(method==='Profiler.stop')return{profile:{nodes:[]}};return{};
 }});await f.controller.start();const start=performance.now(),r=await f.controller.finish();
 assert.equal(r.complete,false);assert.equal(r.pendingCommands,1);assert.equal(f.detached,1);assert(performance.now()-start<500);assert(r.failures.some(x=>x.stage==='trace-end'));
});
test('reported trace data loss refuses completeness',async t=>{
 const f=fixture(t,{send:async(method,_p,events)=>{
  if(method==='Tracing.end')events.emit('Tracing.tracingComplete',{stream:'fixture-stream',dataLossOccurred:true});
  if(method==='Profiler.stop')return{profile:{nodes:[]}};
  if(method==='IO.read')return{data:'{}',eof:true};return{};
 }});await f.controller.start();const r=await f.controller.finish();assert.equal(r.complete,false);assert.equal(r.traceDataLoss,true);
});
test('a full record-until-full buffer is not treated as a full cohort',async t=>{
 const f=fixture(t);await f.controller.start();f.events.emit('Tracing.bufferUsage',{percentFull:1});const r=await f.controller.finish();assert.equal(r.complete,false);assert.equal(r.traceBufferPeak,1);
});
test('oversize trace retains partial receipt and still closes transport',async t=>{
 const f=fixture(t,{limits:{traceBytes:3}});await f.controller.start();const r=await f.controller.finish();assert.equal(r.complete,false);assert.equal(r.artifacts['browser-timeline.json'].complete,false);assert.equal(r.artifacts['browser-timeline.json'].bytes,0);assert.equal(f.detached,1);
});
test('oversize CPU response is rejected independently of trace drain',async t=>{
 const f=fixture(t,{limits:{cpuBytes:3}});await f.controller.start();const r=await f.controller.finish();assert.equal(r.complete,false);assert.equal(r.artifacts['browser-timeline.json'].complete,true);assert(!r.artifacts['browser-cpu.cpuprofile']);assert.equal(f.detached,1);
});
test('empty never-ending trace chunks cannot grow unbounded command metadata',async t=>{
 const f=fixture(t,{limits:{maxReadChunks:2},send:async(method,_p,events)=>{
  if(method==='Tracing.end')events.emit('Tracing.tracingComplete',{stream:'fixture-stream',dataLossOccurred:false});
  if(method==='Profiler.stop')return{profile:{nodes:[]}};
  if(method==='IO.read')return{data:'',eof:false};return{};
 }});await f.controller.start();const r=await f.controller.finish();assert.equal(r.complete,false);assert.equal(f.calls.filter(v=>v.method==='IO.read').length,2);assert.equal(f.detached,1);
});
test('capture deadline settles diagnostics without claiming a successful cohort',async t=>{
 const f=fixture(t,{limits:{captureMs:1}});await f.controller.start();await new Promise(done=>setTimeout(done,10));const r=await f.controller.finish();assert.equal(r.captureTimerFired,true);assert.equal(r.complete,false);assert.equal(f.detached,1);
});
test('late unresolved commands remain uncertain rather than being counted closed',async t=>{
 const f=fixture(t,{send:async(method)=>{if(method==='Profiler.enable')return new Promise(()=>{});return{};}});
 await assert.rejects(f.controller.start(),/CDP deadline/);const r=await f.controller.finish('start-failed');assert.equal(r.pendingCommands,1);assert.equal(r.complete,false);assert.equal(f.detached,1);
});
