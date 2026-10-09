// Pure VM/browser stubs; no Chromium, app, filesystem fixture or Docker.
import {test} from 'node:test';
import assert from 'node:assert/strict';
import {runInNewContext} from 'node:vm';
import {browserLatencyInit,browserFrameWitness} from './candidate/latency-diagnostics.mjs';
import {readFileSync} from 'node:fs';

function fake(supported=['resource','longtask','long-animation-frame']){
 const callbacks={},marks=[];const window={addEventListener(){},__luciPrimaryProfileStartedAt:100,__luciPrimaryProfileActive:false};let clock=150;
 class Observer{static supportedEntryTypes=supported;constructor(cb){this.cb=cb;}observe({type}){callbacks[type]=this.cb;}}
 const ctx={window,PerformanceObserver:Observer,performance:{timeOrigin:1,now:()=>clock++,mark:(name,options)=>marks.push({name,detail:options?.detail}),clearMarks(){}},document:{visibilityState:'visible'},location:{origin:'http://fixture.invalid',href:'http://fixture.invalid/'},URL,requestAnimationFrame:cb=>cb()};
 runInNewContext('('+browserLatencyInit.toString()+')()',ctx);
 return{ctx,window,marks,emit:(type,entries)=>callbacks[type]({getEntries:()=>entries}),snapshot:()=>JSON.parse(JSON.stringify(window.__luciAcceptanceLatency.snapshot()))};
}
test('unsupported long observers are explicit',()=>{const f=fake(['resource']);const d=f.snapshot();assert.equal(d.longTaskUnsupported,true);assert.equal(d.longAnimationFrameUnsupported,true);});
test('pre-primary and pre-start buffered long tasks are omitted',()=>{const f=fake();f.emit('longtask',[{startTime:120,duration:70}]);assert.equal(f.snapshot().rows.length,0);f.window.__luciPrimaryProfileActive=true;f.emit('longtask',[{startTime:99,duration:80},{startTime:120,duration:70,attribution:[{secret:'private'}]}]);assert.deepEqual(f.snapshot().rows.map(({sequence,...x})=>x),[{kind:'long-task',at:120,duration:70}]);});
test('long frame retains numeric summaries only and exposes script cap',()=>{const f=fake();f.window.__luciPrimaryProfileActive=true;f.emit('long-animation-frame',[{startTime:110,duration:80,blockingDuration:30,renderStart:160,styleAndLayoutStart:165,firstUIEventTimestamp:112,scripts:Array.from({length:70},()=>({duration:1,forcedStyleAndLayoutDuration:2,pauseDuration:3,sourceURL:'PRIVATE_URL',sourceFunctionName:'PRIVATE_NAME'}))}]);const d=f.snapshot();assert(!JSON.stringify(d).includes('PRIVATE'));assert.equal(d.rows[0].observedScripts,64);assert.equal(d.rows[0].scriptCount,70);assert.equal(d.rows[0].scriptObservationTruncated,true);assert.equal(d.rows[0].forcedStyleAndLayoutDurationMs,128);});
test('existing passive bound still refuses silent overflow',()=>{const f=fake();f.window.__luciPrimaryProfileActive=true;f.emit('longtask',Array.from({length:2049},()=>({startTime:120,duration:70})));const d=f.snapshot();assert.equal(d.rows.length,2048);assert.equal(d.overflow,true);assert.equal(d.dropped,1);});
test('existing frame RPC retains exact ordinal markers without another RPC',async()=>{const f=fake();f.window.__luciPrimaryProfileActive=true;const value=await runInNewContext('('+browserFrameWitness.toString()+')(57)',f.ctx);assert.equal(value.primaryProfile.actionID,57);assert.deepEqual(f.marks.map(x=>x.name),['luci-primary:57:requested','luci-primary:57:frame-1','luci-primary:57:frame-2']);});
test('narrow trace categories and caps are explicit',()=>{const source=readFileSync(new URL('./candidate/primary-profiler.mjs',import.meta.url),'utf8');assert(source.includes("includedCategories:['blink.user_timing']"));assert(!source.includes("'devtools.timeline'"));assert(!source.includes("'disabled-by-default-v8.gc'"));assert(source.includes('traceBufferKiB:16384'));assert(source.includes('traceBytes:32*1024**2'));});
