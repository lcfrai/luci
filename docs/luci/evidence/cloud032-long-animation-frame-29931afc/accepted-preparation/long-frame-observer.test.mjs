import test from 'node:test';
import assert from 'node:assert/strict';
import vm from 'node:vm';
import {browserLongFrameInit,browserLimits,createLongFrameLedger,installLongFrameTiming} from './long-frame-observer.mjs';
const bundle={path:'/assets/app-123.js',sha256:'a'.repeat(64)};
const script=(changes={})=>({startTime:110,duration:20,executionStart:110,forcedStyleAndLayoutDuration:2,pauseDuration:0,sourceURL:'http://fixture.invalid/assets/app-123.js',sourceCharPosition:19,sourceFunctionName:'PRIVATE function',invoker:'PRIVATE element selector',invokerType:'event-listener',windowAttribution:'self',...changes});
const frame=(changes={})=>({entryType:'long-animation-frame',startTime:100,duration:90,blockingDuration:30,renderStart:140,styleAndLayoutStart:150,firstUIEventTimestamp:105,scripts:[script()],...changes});
function fixture({supported=true,lateWire=false,limits=browserLimits,throwObserve=false}={}){
  let observer,now=200,originalCalls=0;const listeners={},pending=[];
  const window={addEventListener:(name,callback)=>{listeners[name]=callback;}};
  const document={visibilityState:'visible',addEventListener:(name,callback)=>{listeners[name]=callback;}};
  const latency={snapshot(){originalCalls++;return {version:1,timeOrigin:1000,rows:[{kind:'original'}]};}};
  if(!lateWire)window.__luciAcceptanceLatency=latency;
  class Observer{
    static supportedEntryTypes=supported?['long-animation-frame']:[];
    constructor(callback){this.callback=callback;this.disconnected=0;observer=this;}
    observe(options){assert.equal(options.type,'long-animation-frame');assert.equal(options.buffered,true);if(throwObserve)throw Error('PRIVATE observer error');}
    disconnect(){this.disconnected++;}
    takeRecords(){return pending.splice(0);}
  }
  const context=vm.createContext({window,document,location:{href:'http://fixture.invalid/',origin:'http://fixture.invalid'},performance:{timeOrigin:1000,now:()=>now++},PerformanceObserver:Observer,URL,config:JSON.parse(JSON.stringify({limits,bundles:[bundle]}))});
  vm.runInContext(`(${browserLongFrameInit.toString()})(config)`,context);
  return {emit(rows){observer.callback({getEntries:()=>JSON.parse(JSON.stringify(rows))});},queue(row){pending.push(JSON.parse(JSON.stringify(row)));},wire(){window.__luciAcceptanceLatency=latency;listeners.DOMContentLoaded();},hide(){listeners.pagehide();},snapshot(){return JSON.parse(JSON.stringify(latency.snapshot()));},get observer(){return observer;},get originalCalls(){return originalCalls;}};
}
const capture=f=>f.snapshot().longAnimationFrames;
test('captures exact safe frame/script scalars while preserving original snapshot and privacy',()=>{
  const f=fixture();f.emit([frame()]);const snapshot=f.snapshot(),v=snapshot.longAnimationFrames;
  assert.deepEqual(snapshot.rows,[{kind:'original'}]);assert.equal(f.originalCalls,1);
  assert.equal(v.rows.length,1);assert.equal(v.rows[0].scripts[0].bundleSHA256,bundle.sha256);assert.equal(v.rows[0].scripts[0].sourceCharPosition,19);
  assert(!/PRIVATE|fixture.invalid|app-123|sourceURL|sourceFunctionName/.test(JSON.stringify(v)));
  const receipt={},ledger=createLongFrameLedger(receipt);ledger.append(v,1000);assert.equal(receipt.longAnimationFrameCapture.valid,true);
});
test('drains pending records once and keeps continuous sequence across snapshots',()=>{
  const f=fixture(),receipt={},ledger=createLongFrameLedger(receipt);f.queue(frame());ledger.append(capture(f),1000);f.emit([frame({startTime:300})]);ledger.append(capture(f),1000);ledger.append(capture(f),1000);
  assert.deepEqual(receipt.longAnimationFrameCapture.windows.map(v=>v.rows.map(r=>r.sequence)),[[1],[2],[]]);assert.equal(receipt.longAnimationFrameCapture.frames,2);
});
test('init-script ordering can attach once at DOMContentLoaded without timers',()=>{
  const f=fixture({lateWire:true});f.emit([frame()]);f.wire();assert.equal(capture(f).wired,true);assert.equal(f.originalCalls,1);
});
test('unsupported and failed-install observations are retained before explicit refusal',()=>{
  for(const options of [{supported:false},{throwObserve:true}]){const f=fixture(options),receipt={},ledger=createLongFrameLedger(receipt);assert.throws(()=>ledger.append(capture(f),1000),/unsupported/);assert.equal(receipt.longAnimationFrameCapture.valid,false);assert.equal(receipt.longAnimationFrameCapture.windows.length,1);}
});
test('pagehide flushes existing records and late callbacks cannot add old document data',()=>{
  const f=fixture();f.queue(frame());f.hide();f.emit([frame()]);const v=capture(f);assert.equal(v.closed,true);assert.equal(v.rows.length,1);assert.equal(v.lateCallbacks,1);assert(f.observer.disconnected>0);
  const receipt={},ledger=createLongFrameLedger(receipt);ledger.append(v,1000);const late={...v,rows:[{...v.rows[0],sequence:2}],observed:2,retained:2,scripts:2};assert.throws(()=>ledger.append(late,1000),/closed-document-data/);
});
test('browser frame bound preserves retained prefix and exposes dropped observation',()=>{
  const f=fixture({limits:{...browserLimits,maxFrames:1}});f.emit([frame(),frame()]);const v=capture(f);assert.equal(v.rows.length,1);assert.equal(v.status,'invalid');assert.equal(v.failureCode,'frame-bound');assert.equal(v.dropped,1);
});
test('script/byte bounds and malformed timing refuse capture without exposing raw text',()=>{
  for(const [options,row,code] of [[{limits:{...browserLimits,maxScriptsPerFrame:0}},frame(),'script-bound'],[{limits:{...browserLimits,maxBytes:1}},frame(),'byte-bound'],[{},frame({duration:-1}),'invalid-frame-time'],[{},frame({scripts:[script({pauseDuration:-1})]}),'invalid-script-time']]){
    const f=fixture(options);f.emit([row]);const v=capture(f);assert.equal(v.failureCode,code);assert.equal(v.rows.length,0);assert.equal(v.status,'invalid');assert(!JSON.stringify(v).includes('PRIVATE'));
  }
});
test('unknown, cross-origin and queried source identities never inherit app bundle authority',()=>{
  const f=fixture();f.emit([frame({scripts:[script({sourceURL:'https://other.invalid/assets/app-123.js'}),script({sourceURL:'http://fixture.invalid/assets/app-123.js?PRIVATE=token'}),script({sourceURL:'data:PRIVATE'})]})]);
  for(const s of capture(f).rows[0].scripts){assert.equal(s.sourceClass,'unattributed');assert.equal(s.bundleSHA256,null);assert.equal(s.sourceCharPosition,null);}
});
test('ledger rejects missing, wrong-document, duplicate, gap and counter-regression windows',()=>{
  assert.throws(()=>createLongFrameLedger({}).append(undefined,1000),/invalid-window/);
  const f=fixture();f.emit([frame()]);const v=capture(f);
  assert.throws(()=>createLongFrameLedger({}).append(v,1001),/invalid-window/);
  for(const bad of [{...v,rows:[{...v.rows[0],sequence:2}]},{...v,observed:2},{...v,rows:[{...v.rows[0],duration:-2}]}])assert.throws(()=>createLongFrameLedger({}).append(bad,1000));
  const ledger=createLongFrameLedger({});ledger.append(v,1000);assert.throws(()=>ledger.append(v,1000),/invalid-frame-sequence/);
  const ledger2=createLongFrameLedger({});ledger2.append(v,1000);assert.throws(()=>ledger2.append({...capture(f),snapshotAt:0},1000),/counter-regression/);
});
test('ledger fails closed on bounded storage and keeps original failure window',()=>{
  const f=fixture();f.emit([frame()]);const v=capture(f),receipt={};assert.throws(()=>createLongFrameLedger(receipt,{maxFrames:0,maxScripts:0,maxBytes:1,maxDocuments:1,maxWindows:1}).append(v,1000),/ledger-bound/);assert.equal(receipt.longAnimationFrameCapture.valid,false);
  const bad={...v,status:'invalid',failureCode:'frame-bound',dropped:1,observed:2},failed={};assert.throws(()=>createLongFrameLedger(failed).append(bad,1000),/frame-bound/);assert.equal(failed.longAnimationFrameCapture.windows.length,1);
});
test('setup adds one init script and reuses existing snapshot/append without action RPCs',async()=>{
  const receipt={},calls=[],context={async addInitScript(fn,arg){calls.push({fn,arg});}},timingLedger={append(value,reason){calls.push({value,reason});}};
  await installLongFrameTiming(context,{receipt,timingLedger,bundles:[bundle]});assert.equal(calls.length,1);assert.equal(calls[0].fn,browserLongFrameInit);
  const f=fixture();f.emit([frame()]);const value={timeOrigin:1000,latency:{longAnimationFrames:capture(f)}};timingLedger.append(value,'original-reason');assert.equal(calls.length,2);assert.equal(calls[1].value,value);assert.equal(calls[1].reason,'original-reason');assert.equal(receipt.longAnimationFrameCapture.frames,1);
  await assert.rejects(installLongFrameTiming(context,{receipt,timingLedger,bundles:[bundle]}),/One long-frame/);
});
test('setup rejects unbound asset identity and preserves an original ledger exception',async()=>{
  const context={async addInitScript(){}};await assert.rejects(installLongFrameTiming(context,{receipt:{},timingLedger:{append(){}},bundles:[{path:'/private?token=SECRET',sha256:bundle.sha256}]}));
  const original=Error('original failure'),receipt={},timingLedger={append(){throw original;}};await installLongFrameTiming(context,{receipt,timingLedger,bundles:[bundle]});assert.throws(()=>timingLedger.append({},'reason'),e=>e===original);assert.equal(receipt.longAnimationFrameCapture.windows.length,0);
});
