// Additive acceptance diagnostics only. No product state, values, keys or URLs.
import assert from 'node:assert/strict';

// Self-contained because Playwright serializes this function into each document.
export function browserLatencyInit() {
 const bound=2048;
 const data={version:1,timeOrigin:performance.timeOrigin,rows:[],observed:0,dropped:0,overflow:false,resourceUnsupported:false,callbacks:0,callbackMs:0};
 let lastInput=null,lastPointer=null,lastExpected=null;
 const timed=fn=>{const start=performance.now();try{return fn();}finally{data.callbacks++;data.callbackMs+=performance.now()-start;}};
 const emit=row=>{const sequence=++data.observed;if(data.rows.length>=bound){data.dropped++;data.overflow=true;return null;}const value={sequence,...row};data.rows.push(value);return value;};
 const eventRef=row=>row?{sequence:row.sequence,at:row.at}:null;
 const input=event=>{if(!event.isTrusted)return;timed(()=>{
  const row=emit({kind:'input-dispatch',at:performance.now()});lastInput=eventRef(row);
  if(!row)return;
  requestAnimationFrame(()=>timed(()=>{emit({kind:'input-frame-1',inputSequence:row.sequence,at:performance.now()});requestAnimationFrame(()=>timed(()=>emit({kind:'input-frame-2',inputSequence:row.sequence,at:performance.now()})));}));
 });};
 const pointer=event=>{if(event.isTrusted)timed(()=>{lastPointer=eventRef(emit({kind:'pointer-dispatch',at:performance.now()}));});};
 window.addEventListener('input',input,{capture:true,passive:true});
 window.addEventListener('pointerdown',pointer,{capture:true,passive:true});
 const markExpectedDOM=kind=>timed(()=>{
  if(!['input','project-switch'].includes(kind))throw Error('Unknown diagnostic DOM witness');
  const ref=eventRef(emit({kind:'expected-dom',actionKind:kind,at:performance.now(),input:lastInput,pointer:lastPointer}));lastExpected=ref?{...ref,actionKind:kind}:null;
 });
 const snapshot=()=>({...data,rows:data.rows.splice(0),visibility:document.visibilityState});
 const references=()=>({timeOrigin:data.timeOrigin,input:lastInput,pointer:lastPointer,expectedDOM:lastExpected});
 Object.defineProperty(window,'__luciAcceptanceLatency',{value:{snapshot,references,markExpectedDOM},configurable:false});
 // Only same-origin, known API route CLASSES are retained. No raw paths, names,
 // queries, identifiers, headers, body bytes, key strings or input values.
 const classify=name=>{
  let url;try{url=new URL(name,location.href);}catch{return null;}
  if(url.origin!==location.origin)return null;
  const path=url.pathname.replace(/^\/v1\/teams\/team-[A-Za-z0-9_-]{22}\/workspace(?=\/v1\/)/,'');
  const project='[A-Za-z0-9_-]+';
  for(const [label,pattern] of [
   ['project-overview',new RegExp('^/v1/product/projects/'+project+'/overview$')],
   ['project-specification',new RegExp('^/v1/product/projects/'+project+'/specification$')],
   ['project-orchestrator',new RegExp('^/v1/product/projects/'+project+'/orchestrator$')],
   ['project-swarm-queue',new RegExp('^/v1/product/projects/'+project+'/swarm-queue$')],
   ['project-runs',new RegExp('^/v1/product/projects/'+project+'/runs(?:/[A-Za-z0-9_-]+)?$')],
   ['swarm-scheduler',/^\/v1\/product\/swarm-scheduler$/],
  ])if(pattern.test(path))return label;
  return null;
 };
 try{
  if(!PerformanceObserver.supportedEntryTypes.includes('resource')){data.resourceUnsupported=true;return;}
  const observer=new PerformanceObserver(list=>timed(()=>{for(const entry of list.getEntries()){
   const route=classify(entry.name);if(!route)continue;
   emit({kind:'resource',route,at:entry.startTime,duration:entry.duration,responseStatus:Number.isInteger(entry.responseStatus)&&entry.responseStatus>=100&&entry.responseStatus<=599?entry.responseStatus:null,requestStart:entry.requestStart,responseStart:entry.responseStart,responseEnd:entry.responseEnd,transferSize:entry.transferSize,encodedBodySize:entry.encodedBodySize,decodedBodySize:entry.decodedBodySize});
  }}));observer.observe({type:'resource',buffered:true});
 }catch{data.resourceUnsupported=true;}
}

// Replaces the existing two-rAF RPC body, not the RPC or primary span. These are
// callback opportunities in the browser clock, not proof of physical painting.
export function browserFrameWitness() {
 const requestedAt=performance.now();
 return new Promise(resolve=>requestAnimationFrame(()=>{
  const firstFrameAt=performance.now();
  requestAnimationFrame(()=>resolve({requestedAt,firstFrameAt,secondFrameAt:performance.now(),references:window.__luciAcceptanceLatency?.references()??null}));
 }));
}

// Called synchronously at existing await boundaries: no added browser RPC.
export function nodeStageRecorder(item,start,clock=()=>performance.now()) {
 const stages=[];let closed=false;
 item.diagnostics={version:1,nodeStages:stages,nodeClockReads:0,browserFrames:null,limitations:['Node subspans include command transport and scheduler waits. Browser clock values are not subtracted from Node clock values. Two rAF callbacks are not physical paint or React commit proof. Instrumentation changes overhead; callback accounting excludes observer, clock and serialization overhead.']};
 return {
  mark(stage){if(closed)return;assert(['action-start','action-end','verify-start','verify-end','frames-start','frames-end','all-projects-click-end','overview-heading-ready','open-project-click-end','waitFor-start','waitFor-end','evaluate-start','evaluate-end','inputValue-start','inputValue-end'].includes(stage));assert(stages.length<18);item.diagnostics.nodeClockReads++;stages.push({stage,offsetMs:clock()-start});},
  frames(value){if(!closed)item.diagnostics.browserFrames=value;},
  close(){closed=true;},
 };
}

export function createLatencyLedger(receipt,limits={maxRows:50000,maxBytes:8*1024**2,maxWindows:2000,maxDocuments:128}) {
 const documents=new Map();let rows=0,bytes=0,windows=0;
 receipt.latencyDiagnostics={version:1,limits:{...limits},rows:0,bytes:0,windows:0,documents:0,status:'unobserved',valid:true,scope:'Secondary observations only; original action spans and 100/300ms thresholds remain unchanged. No keys, values, raw URLs or response content. Resource timing is browser network timing, not backend function timing. Callback self-time is partial instrumentation accounting; total overhead and physical paint remain unknown.'};
 return {append(value){
  const summary=receipt.latencyDiagnostics;
  const fail=message=>{summary.valid=false;summary.failure=message;throw Error(message);};
  if(!value)fail('Missing browser latency diagnostic snapshot');
  if(value.version!==1||!Number.isFinite(value.timeOrigin)||!Array.isArray(value.rows)||value.rows.length>2048||!Number.isSafeInteger(value.observed)||value.observed<0||!Number.isSafeInteger(value.dropped)||value.dropped<0)fail('Invalid browser latency diagnostic snapshot');
  const size=Buffer.byteLength(JSON.stringify(value));
  if(rows+value.rows.length>limits.maxRows||bytes+size>limits.maxBytes||windows>=limits.maxWindows||!documents.has(value.timeOrigin)&&documents.size>=limits.maxDocuments)fail('Browser latency diagnostics exceeded declared bound');
  rows+=value.rows.length;bytes+=size;windows++;
  Object.assign(summary,{status:'observed',rows,bytes,windows,documents:documents.size+(documents.has(value.timeOrigin)?0:1)});
  let previous=documents.get(value.timeOrigin)??0;
  for(const row of value.rows){if(row.sequence!==previous+1)fail('Browser latency diagnostic sequence gap or duplicate');previous=row.sequence;}
  if(value.overflow||value.dropped!==0)fail('Browser latency diagnostic overflow');
  if(previous!==value.observed)fail('Browser latency diagnostic counter mismatch');
  documents.set(value.timeOrigin,previous);
 }};
}
