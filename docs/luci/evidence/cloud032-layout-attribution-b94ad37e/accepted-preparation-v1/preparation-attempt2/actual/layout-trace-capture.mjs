import assert from 'node:assert/strict';
import {createHash} from 'node:crypto';
import {openSync,writeSync,fsyncSync,closeSync} from 'node:fs';
import {join} from 'node:path';

export const LIMITS=Object.freeze({rounds:8,traceMs:30000,commandMs:5000,completeMs:5000,drainMs:10000,closeMs:2000,chunkBytes:65536,maxBytes:64*1024*1024,bufferKiB:32768,stopFraction:.75,maxBufferEvents:256,maxJournalEvents:4096,maxJournalBytes:1024*1024});
export const CATEGORIES=Object.freeze(['devtools.timeline','disabled-by-default-devtools.timeline','disabled-by-default-devtools.timeline.stack','disabled-by-default-devtools.timeline.invalidationTracking','blink.user_timing']);
const writeAll=(fd,bytes)=>{let offset=0;while(offset<bytes.length){const wrote=writeSync(fd,bytes,offset,bytes.length-offset);assert(wrote>0,'Trace sink write made no progress');offset+=wrote;}};
const refusal=code=>Object.assign(Error(code),{code});
const safeError=e=>({code:/^[a-z][a-z0-9-]{1,80}$/.test(e?.code??'')?e.code:'trace-operation-failed'});
export function createTraceSink(directory,{limits=LIMITS}={}){
 const journal=openSync(join(directory,'layout-trace-journal.jsonl'),'wx',0o600);let raw;try{raw=openSync(join(directory,'layout-trace.raw.json'),'wx',0o600);}catch(error){closeSync(journal);throw error;}
 let closed=false,events=0,journalBytes=0,bytes=0;const hash=createHash('sha256');
 return {
  record(row){assert(!closed);const line=Buffer.from(JSON.stringify({sequence:events+1,...row})+'\n');assert(events<limits.maxJournalEvents&&journalBytes+line.length<=limits.maxJournalBytes,'Bounded trace journal');writeAll(journal,line);fsyncSync(journal);events++;journalBytes+=line.length;},
  append(chunk){assert(!closed);assert(Buffer.isBuffer(chunk)&&bytes+chunk.length<=limits.maxBytes,'Bounded raw trace');writeAll(raw,chunk);fsyncSync(raw);hash.update(chunk);bytes+=chunk.length;},
  snapshot(){return {bytes,sha256:hash.copy().digest('hex'),journalEvents:events,journalBytes,private:true};},
  close(){if(closed)return;closed=true;try{fsyncSync(raw);}finally{try{closeSync(raw);}finally{closeSync(journal);}}},
 };
}

// One CDP trace lifetime. Every outstanding race has an immediate rejection fence;
// a late response cannot resume rounds, certify completion, or append after close.
export function createLayoutTraceCapture({session,sink,onFailure=()=>{},limits=LIMITS,now=()=>Date.now()}){
 const receipt={version:1,diagnosticOnly:true,acceptance:false,limits:{...limits},state:'prepared',startRequested:false,startAcknowledged:false,endRequests:0,completedRounds:0,attemptedRounds:0,markers:[],bufferUsage:[],cleanup:{end:'not-requested',streamClose:'not-opened',detach:'not-attempted'},failure:null,complete:false,dataLossOccurred:null,rawPrivate:true};
 let failure,closed=false,stopping=false,traceTimer,stream,finishPromise,lateResponses=0;
 let rejectFailure,resolveComplete;const failed=new Promise((_,reject)=>{rejectFailure=reject;});void failed.catch(()=>{});
 const completed=new Promise(resolve=>{resolveComplete=resolve;});let completion;
 function record(event,fields={}){if(closed)return;sink.record({event,atMs:now(),...fields});}
 function fail(error){if(!failure){failure=refusal(safeError(error).code);receipt.failure=safeError(failure);record('refusal',receipt.failure);rejectFailure(failure);try{onFailure(failure);}catch{}}return failure;}
 function assertActive(){if(failure)throw failure;if(closed||stopping)throw refusal('trace-lifetime-closed');}
 async function bounded(fn,ms,code,{ignoreFailure=false}={}){
  if(!ignoreFailure)assertActive();let timer,settled=false;
  const operation=Promise.resolve().then(fn);void operation.catch(()=>{});operation.then(()=>{if(settled)lateResponses++;},()=>{if(settled)lateResponses++;});
  try{return await Promise.race([operation,new Promise((_,reject)=>{timer=setTimeout(()=>reject(refusal(code)),ms);}),...(ignoreFailure?[]:[failed])]);}
  finally{settled=true;clearTimeout(timer);}
 }
 const onBuffer=value=>{
  if(closed||stopping)return;
  const fraction=value?.percentFull??value?.value;
  if(!Number.isFinite(fraction)||fraction<0||fraction>1){fail(refusal('trace-buffer-usage-invalid'));return;}
  if(receipt.bufferUsage.length>=limits.maxBufferEvents){fail(refusal('trace-buffer-event-bound'));return;}
  receipt.bufferUsage.push({fraction,atMs:now()});record('buffer-usage',{fraction});
  if(fraction>=limits.stopFraction)fail(refusal('trace-buffer-limit'));
 };
 const onComplete=value=>{
  if(closed)return;
  if(completion){fail(refusal('trace-duplicate-completion'));return;}
  completion=value;resolveComplete(value);record('tracing-complete',{dataLossOccurred:value?.dataLossOccurred===true,hasStream:typeof value?.stream==='string'});
  if(!stopping)fail(refusal('trace-unexpected-completion'));
 };
 session.on('Tracing.bufferUsage',onBuffer);session.on('Tracing.tracingComplete',onComplete);
 record('prepared',{limits:receipt.limits,requiredCategories:CATEGORIES});
 async function start(){
  assertActive();assert.equal(receipt.state,'prepared');receipt.state='starting';
  try{
   record('categories-requested');const reply=await bounded(()=>session.send('Tracing.getCategories'),limits.commandMs,'trace-categories-timeout');assertActive();
   if(!Array.isArray(reply?.categories)||reply.categories.length>4096||reply.categories.some(v=>typeof v!=='string'||v.length>256))throw refusal('trace-categories-invalid');
   receipt.categories={availableCount:reply.categories.length,availableSHA256:createHash('sha256').update(JSON.stringify(reply.categories)).digest('hex'),selected:[...CATEGORIES]};
   record('categories-returned',receipt.categories);
   if(!CATEGORIES.every(v=>reply.categories.includes(v)))throw refusal('trace-categories-unavailable');
   const config={transferMode:'ReturnAsStream',streamFormat:'json',streamCompression:'none',bufferUsageReportingInterval:500,traceConfig:{recordMode:'recordUntilFull',traceBufferSizeInKb:limits.bufferKiB,includedCategories:[...CATEGORIES],enableSampling:false}};
   receipt.startRequested=true;receipt.config=config;record('start-requested',{config});
   traceTimer=setTimeout(()=>fail(refusal('trace-duration-limit')),limits.traceMs);
   await bounded(()=>session.send('Tracing.start',config),limits.commandMs,'trace-start-timeout');assertActive();receipt.startAcknowledged=true;receipt.state='recording';record('start-acknowledged');
  }catch(error){throw fail(error);}
 }
 async function runRounds({mark,round,assertHealthy=()=>{}}){
  assertActive();assert.equal(receipt.state,'recording');
  try{
   for(let index=0;index<limits.rounds;index++){
    assertActive();assertHealthy();receipt.attemptedRounds++;record('round-requested',{round:index});
    for(const edge of ['start','end']){
     if(edge==='end'){await bounded(()=>round(index),limits.traceMs,'trace-round-timeout');assertActive();assertHealthy();}
     const name=`luci-layout-boundary-${index}-${edge}`;
     const marker=await bounded(()=>mark({name,round:index,edge}),limits.commandMs,'trace-marker-timeout');assertActive();assertHealthy();
     if(marker?.name!==name||marker.round!==index||marker.edge!==edge||!Number.isFinite(marker.timeOrigin)||marker.timeOrigin<=0||!Number.isFinite(marker.startTime)||marker.startTime<0)throw refusal('trace-marker-invalid');
     receipt.markers.push({name,round:index,edge,timeOrigin:marker.timeOrigin,startTime:marker.startTime});record('round-marker',receipt.markers.at(-1));
    }
    receipt.completedRounds++;record('round-completed',{round:index});
   }
  }catch(error){throw fail(error);}
 }
 async function finish(reason='completed'){
  if(finishPromise)return finishPromise;
  finishPromise=(async()=>{
   stopping=true;clearTimeout(traceTimer);receipt.state='stopping';record('stop-requested',{reason:reason==='completed'?'completed':'failed',completedRounds:receipt.completedRounds});
   try{
    if(receipt.startRequested){
     receipt.endRequests++;receipt.cleanup.end='requested';record('end-requested');
     try{await bounded(()=>session.send('Tracing.end'),limits.commandMs,'trace-end-timeout',{ignoreFailure:true});receipt.cleanup.end='acknowledged';record('end-acknowledged');}catch(error){fail(error);receipt.cleanup.end='failed';}
     try{
      const done=completion??await bounded(()=>completed,limits.completeMs,'trace-completion-timeout',{ignoreFailure:true});
      receipt.dataLossOccurred=done?.dataLossOccurred===true;
      if(typeof done?.dataLossOccurred!=='boolean')fail(refusal('trace-loss-status-missing'));
      if(done?.dataLossOccurred===true)fail(refusal('trace-data-loss'));
      if(typeof done?.stream!=='string'||done.stream.length===0||done.stream.length>512)throw refusal('trace-stream-missing');
      stream=done.stream;receipt.cleanup.streamClose='pending';record('stream-opened');
      const deadline=now()+limits.drainMs;let eof=false,chunks=0;
      while(!eof){
       const left=deadline-now();if(left<=0)throw refusal('trace-drain-timeout');
       const remainingBytes=limits.maxBytes-sink.snapshot().bytes;if(remainingBytes<=0)throw refusal('trace-stream-byte-limit');
       const requestedBytes=Math.min(limits.chunkBytes,remainingBytes);
       const chunk=await bounded(()=>session.send('IO.read',{handle:stream,size:requestedBytes}),left,'trace-read-timeout',{ignoreFailure:true});
       if(typeof chunk?.data!=='string'||typeof chunk?.eof!=='boolean'||(chunk.base64Encoded!==undefined&&typeof chunk.base64Encoded!=='boolean'))throw refusal('trace-chunk-invalid');
       if(chunk.data.length>limits.chunkBytes*2)throw refusal('trace-chunk-limit');
       if(chunk.base64Encoded&&!/^(?:[A-Za-z0-9+/]{4})*(?:[A-Za-z0-9+/]{2}==|[A-Za-z0-9+/]{3}=)?$/.test(chunk.data))throw refusal('trace-base64-invalid');
       const bytes=Buffer.from(chunk.data,chunk.base64Encoded?'base64':'utf8');
       record('stream-chunk-returned',{bytes:bytes.length,sha256:createHash('sha256').update(bytes).digest('hex'),eof:chunk.eof});
       if(bytes.length>limits.chunkBytes||sink.snapshot().bytes+bytes.length>limits.maxBytes)throw refusal('trace-stream-byte-limit');
       if(bytes.length===0&&!chunk.eof)throw refusal('trace-stream-no-progress');
       sink.append(bytes);chunks++;record('stream-chunk',{chunk:chunks,bytes:bytes.length,totalBytes:sink.snapshot().bytes,eof:chunk.eof});eof=chunk.eof;
      }
      receipt.streamEOF=true;record('stream-eof',{chunks,...sink.snapshot()});
     }catch(error){fail(error);}
    }
   }finally{
    if(stream){try{await bounded(()=>session.send('IO.close',{handle:stream}),limits.closeMs,'trace-stream-close-timeout',{ignoreFailure:true});receipt.cleanup.streamClose='closed';record('stream-closed');}catch(error){fail(error);receipt.cleanup.streamClose='failed';}}
    try{await bounded(()=>session.detach(),limits.closeMs,'trace-detach-timeout',{ignoreFailure:true});receipt.cleanup.detach='detached';record('session-detached');}catch(error){fail(error);receipt.cleanup.detach='failed';}
    session.off('Tracing.bufferUsage',onBuffer);session.off('Tracing.tracingComplete',onComplete);
    receipt.complete=!failure&&receipt.startAcknowledged&&receipt.completedRounds===limits.rounds&&receipt.streamEOF===true&&receipt.dataLossOccurred===false&&receipt.cleanup.end==='acknowledged'&&receipt.cleanup.streamClose==='closed'&&receipt.cleanup.detach==='detached';
    if(!receipt.complete&&!failure)fail(refusal('trace-capture-incomplete'));
    receipt.state=receipt.complete?'captured':'refused';receipt.lateResponsesObserved=lateResponses;receipt.raw=sink.snapshot();record('closed',{state:receipt.state,complete:receipt.complete,failure:receipt.failure,cleanup:receipt.cleanup,raw:receipt.raw});closed=true;sink.close();
   }
   return structuredClone(receipt);
  })();void finishPromise.catch(()=>{});return finishPromise;
 }
 return {start,runRounds,finish,assertActive,cancel:()=>closed?undefined:fail(refusal('trace-cancelled')),get receipt(){return structuredClone(receipt);}};
}
