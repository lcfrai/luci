// External diagnostics only. No product, Docker, provider or application mutations.
import {mkdirSync,writeFileSync,openSync,writeSync,closeSync} from 'node:fs';
import {join} from 'node:path';
import {createHash} from 'node:crypto';

export const LIMITS=Object.freeze({captureMs:150000,commandMs:2000,finishMs:9000,traceBytes:32*1024**2,cpuBytes:12*1024**2,traceBufferKiB:16384,readChunk:256*1024,maxReadChunks:256,cpuIntervalUs:5000});
export async function bounded(operation,ms,label){
 let timer;try{return await Promise.race([Promise.resolve().then(operation),new Promise((_,reject)=>{timer=setTimeout(()=>reject(Error(label)),ms);})]);}finally{clearTimeout(timer);}
}

// One controller per fresh, privately owned run directory. The caller MUST also
// retain its existing independent bounded browser/process cleanup. A protocol
// timeout is uncertainty; it never establishes that the browser stopped.
export function createPrimaryProfiler({page,directory,assertActive=()=>{},limits=LIMITS}){
 mkdirSync(directory,{mode:0o700});
 const status={version:1,scope:'Instrumented PRIMARY400 diagnostic; not acceptance, speedup or causal proof',mode:'CDP only; Playwright tracing disabled',limits,complete:false,phase:'prepared',failures:[],steps:[],artifacts:{},captureTimerFired:false,traceDataLoss:null,traceBufferPeak:0,pendingCommands:0,transportDetached:false,clockPolicy:'Node and browser clocks remain separate. Join exact browser marker names to primary frame witnesses; browser user-timing marks bridge browser performance and trace timestamps. CPU profile is whole-window only until its clock alignment is independently checked.'};
 const file=join(directory,'profiling-status.json');
 const save=()=>writeFileSync(file,JSON.stringify(status,null,2)+'\n',{mode:0o600});
 const error=(stage,e)=>{status.failures.push({stage,error:String(e?.message??e).slice(0,240)});save();};
 let session,timer,finishPromise,closed=false,traceRunning=false,cpuRunning=false,traceStream,traceEventResolve,traceComplete;
 const step=async(method,params={},ms=limits.commandMs)=>{
  status.pendingCommands++;const row={method,startedNodeMs:performance.now(),settled:false};status.steps.push(row);
  const pending=Promise.resolve().then(()=>session.send(method,params)).then(value=>{row.settled=true;row.resolved=true;return value;},e=>{row.settled=true;row.resolved=false;throw e;}).finally(()=>{status.pendingCommands--;});
  try{return await bounded(()=>pending,ms,'CDP deadline: '+method);}finally{row.returnedNodeMs=performance.now();save();}
 };
 const adopt=async()=>{
  const pending=page.context().newCDPSession(page);
  void pending.then(value=>{if(closed){void bounded(()=>value.detach(),limits.commandMs,'late CDP detach').catch(e=>error('late-session-detach',e));}else session=value;},e=>error('create-session',e));
  const value=await bounded(()=>pending,limits.commandMs,'CDP session deadline');
  if(closed)throw Error('Profiling closed during session creation');session=value;
 };
 const writeJSON=(name,value,cap)=>{
  const bytes=Buffer.from(JSON.stringify(value));if(bytes.length>cap)throw Error(name+' exceeds declared bound');
  writeFileSync(join(directory,name),bytes,{mode:0o600,flag:'wx'});status.artifacts[name]={bytes:bytes.length,sha256:createHash('sha256').update(bytes).digest('hex'),private:true};save();
 };
 async function start(){
  status.phase='starting';save();
  try{
   assertActive();await adopt();assertActive();if(closed)throw Error('Profiling closed');
   traceComplete=new Promise(resolve=>{traceEventResolve=resolve;});
   session.on('Tracing.tracingComplete',event=>{traceStream=event.stream;status.traceDataLoss=event.dataLossOccurred===true;traceEventResolve(event);save();});
   session.on('Tracing.bufferUsage',event=>{status.traceBufferPeak=Math.max(status.traceBufferPeak,Number(event.percentFull??0));});
   await step('Profiler.enable');assertActive();if(closed)throw Error('Profiling closed');
   await step('Profiler.setSamplingInterval',{interval:limits.cpuIntervalUs});assertActive();if(closed)throw Error('Profiling closed');
   // Buffer fills stop recording; no silent ring-buffer replacement of early cohorts.
   // No screenshots, network payload, DOM snapshot or JS source category is requested.
   traceRunning=true;
   await step('Tracing.start',{transferMode:'ReturnAsStream',streamFormat:'json',streamCompression:'none',bufferUsageReportingInterval:1000,traceConfig:{recordMode:'recordUntilFull',traceBufferSizeInKb:limits.traceBufferKiB,includedCategories:['devtools.timeline','v8','blink.user_timing','disabled-by-default-v8.gc']}});
   assertActive();if(closed)throw Error('Profiling closed');cpuRunning=true;await step('Profiler.start');
   assertActive();if(closed)throw Error('Profiling closed');
   // Setup-only RPC before the primary measured phase. Later marks are emitted
   // inside existing passive callbacks and the existing two-frame RPC.
   status.browserStart=await bounded(()=>page.evaluate(()=>{window.__luciPrimaryProfileActive=true;performance.mark('luci-primary-window:start');return{timeOrigin:performance.timeOrigin,now:performance.now()};}),limits.commandMs,'browser profiling start');
   assertActive();if(closed)throw Error('Profiling closed');status.phase='recording';status.recordingStartNodeMs=performance.now();save();
   timer=setTimeout(()=>{status.captureTimerFired=true;error('capture','Declared primary profiling window expired');void finish('capture-deadline');},limits.captureMs);
  }catch(e){error('start',e);throw e;}
 }
 async function drainTrace(ms){
  const done=await bounded(()=>traceComplete,ms,'Tracing completion deadline');
  if(!done.stream)throw Error('Tracing completion omitted stream');
  const path=join(directory,'browser-timeline.json'),fd=openSync(path,'wx',0o600),hash=createHash('sha256');let bytes=0,eof=false,chunks=0;
  try{
   while(!eof){
    const left=status.finishDeadlineNodeMs-performance.now();if(left<=0)throw Error('Trace drain deadline');
    if(++chunks>limits.maxReadChunks)throw Error('Trace stream chunk count exceeded declared bound');
    const part=await step('IO.read',{handle:done.stream,size:limits.readChunk},Math.min(limits.commandMs,left));
    const buf=Buffer.from(part.data??'',part.base64Encoded?'base64':'utf8');
    if(bytes+buf.length>limits.traceBytes)throw Error('Trace stream exceeds declared bound');
    writeSync(fd,buf);hash.update(buf);bytes+=buf.length;eof=part.eof===true;
   }
  }finally{closeSync(fd);status.artifacts['browser-timeline.json']={bytes,sha256:hash.digest('hex'),complete:eof,private:true};save();}
  if(status.traceDataLoss||status.traceBufferPeak>=1)throw Error('Trace data loss or full recording buffer');
 }
 function finish(reason='primary-complete'){
  if(finishPromise)return finishPromise;
  closed=true;clearTimeout(timer);status.phase='stopping';status.finishReason=reason;status.finishDeadlineNodeMs=performance.now()+limits.finishMs;save();
  finishPromise=(async()=>{
   const left=()=>Math.max(1,Math.min(limits.commandMs,status.finishDeadlineNodeMs-performance.now()));
   // Issuing end commands before draining bounds storage and avoids serial trace
   // writes keeping CPU sampling alive. Failed/late commands remain explicit.
   if(session){
    if(traceRunning){try{await step('Tracing.end',{},left());}catch(e){error('trace-end',e);}}
    if(cpuRunning){try{const value=await step('Profiler.stop',{},left());writeJSON('browser-cpu.cpuprofile',value.profile,limits.cpuBytes);}catch(e){error('cpu-stop',e);}}
    if(traceRunning){try{await drainTrace(Math.max(1,status.finishDeadlineNodeMs-performance.now()));}catch(e){error('trace-drain',e);}}
    if(traceStream){try{await step('IO.close',{handle:traceStream},left());}catch(e){error('trace-stream-close',e);}}
    try{await bounded(()=>session.detach(),left(),'CDP detach deadline');status.transportDetached=true;}catch(e){error('detach',e);}
   }
   status.phase='finished';status.complete=reason==='primary-complete'&&!status.captureTimerFired&&status.failures.length===0&&status.pendingCommands===0&&status.transportDetached&&status.artifacts['browser-timeline.json']?.complete===true&&!!status.artifacts['browser-cpu.cpuprofile'];save();return status;
  })();
  // A losing async tail may settle later. It cannot block the caller's resource
  // cleanup and it cannot grant acceptance success through this projection.
  return finishPromise;
 }
 save();return{start,finish,status,snapshot:()=>structuredClone(status)};
}
