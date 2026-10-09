// Diagnostic only: delay response availability. No body capture, replacement, retry or replay.
import assert from 'node:assert/strict';

export function installLongTaskCollector() {
 const data={version:1,timeOrigin:performance.timeOrigin,rows:[],observed:0,dropped:0,unsupported:false};
 try {
  if(!PerformanceObserver.supportedEntryTypes.includes('longtask'))data.unsupported=true;
  else new PerformanceObserver(list=>{for(const entry of list.getEntries()){
   const sequence=++data.observed;
   if(data.rows.length===2048){data.dropped++;continue;}
   data.rows.push({sequence,startTime:entry.startTime,duration:entry.duration});
  }}).observe({type:'longtask',buffered:true});
 } catch {data.unsupported=true;}
 Object.defineProperty(window,'__luciHeaderDeliveryLongTasks',{value:{snapshot:()=>({...data,rows:data.rows.splice(0)})},configurable:false});
}

export async function createHeaderDelivery({page,base,projectIDs,receipt,onFailure,limits={},session}) {
 const caps={maxPaused:16,maxRequests:1000,maxMetadataBytes:4*1024**2,maxHeaderBytes:64*1024,maxPauseMs:5000,commandMs:1000,cancelWitnessMs:250,drainMs:6000,...limits};
 for(const number of Object.values(caps))assert(Number.isSafeInteger(number)&&number>0);
 assert.equal(new URL(base).origin,base);assert.equal(projectIDs.length,4);assert.equal(new Set(projectIDs).size,4);
 const paths=new Map(projectIDs.map((id,index)=>{assert(/^[A-Za-z0-9_-]+$/.test(id));return ['/v1/product/projects/'+id+'/overview',{route:'home-overview',projectIndex:index}];}));
 paths.set('/v1/product/swarm-scheduler',{route:'home-capacity'});
 const classify=request=>{try{const url=new URL(request.url);return request.method==='GET'&&url.origin===base&&url.search===''?paths.get(url.pathname):undefined;}catch{return undefined;}};
 const cdp=session??await page.context().newCDPSession(page),pending=new Map(),handlers=new Set(),network=new Map();
 const summary={version:2,diagnosticOnly:true,status:'installing',valid:true,limits:caps,requests:[],responses:[],events:[],longTaskWindows:[],metadataBytes:0,scope:'Response HEADERS availability/backpressure intervention. No body capture while paused or afterward. Upstream body completion and unchanged backend work are not established; frontend commit cost is not isolated.'};
 receipt.headerDelivery=summary;
 let active=null,closed=false,closing=false,failure,metadataBytes=0,sequence=0,longSequence=0,longDocument;
 const sleep=ms=>new Promise(resolve=>setTimeout(resolve,ms));
 const fail=label=>{if(!failure){failure=Error(label);summary.valid=false;summary.failure=label;onFailure(failure);}return failure;};
 const budget=value=>{metadataBytes+=Buffer.byteLength(JSON.stringify(value));summary.metadataBytes=metadataBytes;if(metadataBytes>caps.maxMetadataBytes)throw fail('Header diagnostic metadata cap exceeded');};
 const event=(kind,fields={})=>{const row={kind,nodeAt:performance.now(),...fields};budget(row);summary.events.push(row);};
 async function bounded(promise,ms,label){let timer;try{return await Promise.race([promise,new Promise((_,reject)=>{timer=setTimeout(()=>reject(fail(label)),ms);})]);}finally{clearTimeout(timer);}}
 const cancelled=row=>row.networkId&&network.get(row.networkId)?.cancelled===true;
 async function cancellationWitness(row){const until=performance.now()+caps.cancelWitnessMs;while(!cancelled(row)&&performance.now()<until)await sleep(Math.min(5,caps.cancelWitnessMs));return cancelled(row);}
 function release(row,reason){
  if(row.releasePromise)return row.releasePromise;
  clearTimeout(row.timer);
  row.releasePromise=(async()=>{
   try{await bounded(cdp.send('Fetch.continueResponse',{requestId:row.id}),caps.commandMs,'Original response continuation timed out');row.data.outcome='continued-original-headers';}
   catch(error){
    if(/Invalid InterceptionId/.test(String(error?.message))&&await cancellationWitness(row))row.data.outcome='original-network-cancelled-terminal-ID';
    else{row.data.outcome='unsettled-protocol-failure';throw fail('Response terminal identity could not be established');}
   }
   row.data.releaseReason=reason;row.data.releasedAt=performance.now();row.data.pauseMs=row.data.releasedAt-row.data.pausedAt;
   pending.delete(row.id);event('response-resolved',{sequence:row.data.sequence,outcome:row.data.outcome,reason,pauseMs:row.data.pauseMs});
  })();
  // Retain the rejected operation for drain/cleanup without an unhandled promise.
  void row.releasePromise.catch(()=>{});return row.releasePromise;
 }
 async function paused(value){
  const target=classify(value.request),request=network.get(value.networkId),round=active&&{...active};
  const data={sequence:++sequence,...target,status:value.responseStatusCode??null,requestRound:request?.round??null,requestCondition:request?.condition??null,responseRound:round?.round??null,responseCondition:round?.condition??null,pausedAt:performance.now()};
  budget(data);summary.responses.push(data);const row={id:value.requestId,networkId:value.networkId,data};pending.set(row.id,row);
  if(pending.size>caps.maxPaused||summary.responses.length>caps.maxRequests){fail('Header response count cap exceeded');await release(row,'count-cap-refusal');return;}
  const headerBytes=(value.responseHeaders??[]).reduce((n,h)=>n+Buffer.byteLength(h.name)+Buffer.byteLength(h.value),0);data.headerBytes=headerBytes;
  if(headerBytes>caps.maxHeaderBytes){fail('Header byte cap exceeded');await release(row,'header-cap-refusal');return;}
  // Non-200, non-GET, unknown project/path and inactive-phase reads are never held.
  if(!target||data.status!==200||!round||closed||failure){await release(row,'non-target-immediate');return;}
  if(!request){fail('Target response has no original network request witness');await release(row,'missing-network-witness');return;}
  if(request.round!==round.round||request.condition!==round.condition){await release(row,'request-from-another-round');return;}
  if(round.condition==='baseline'){await release(row,'baseline-immediate');return;}
  data.held=true;event('headers-held',{sequence:data.sequence,round:round.round,route:data.route});
  row.timer=setTimeout(()=>{fail('Header hold deadline exceeded');void release(row,'hold-timeout').catch(()=>{});},caps.maxPauseMs);
 }
 const onRequest=value=>{
  const target=classify(value.request);if(!target)return;
  if(network.size>=caps.maxRequests){fail('Header request count cap exceeded');return;}
  const row={sequence:summary.requests.length+1,...target,round:active?.round??null,condition:active?.condition??null,requestAt:value.timestamp,terminal:false};
  try{budget(row);network.set(value.requestId,row);summary.requests.push(row);}catch{ /* failure already latched */ }
 };
 const onFailed=value=>{const row=network.get(value.requestId);if(row){row.terminal=true;row.failedAt=value.timestamp;row.cancelled=value.canceled===true&&value.errorText==='net::ERR_ABORTED';row.outcome=row.cancelled?'natural-browser-abort':'network-failure';}};
 const onFinished=value=>{const row=network.get(value.requestId);if(row){row.terminal=true;row.finishedAt=value.timestamp;row.outcome='network-loading-finished';}};
 const onPaused=value=>{const work=paused(value).catch(()=>{fail('Header response handler failed');});handlers.add(work);void work.finally(()=>handlers.delete(work));};
 cdp.on('Network.requestWillBeSent',onRequest);cdp.on('Network.loadingFailed',onFailed);cdp.on('Network.loadingFinished',onFinished);cdp.on('Fetch.requestPaused',onPaused);
 await cdp.send('Network.enable');
 await cdp.send('Fetch.enable',{patterns:[{urlPattern:base+'/v1/product/projects/*/overview*',requestStage:'Response'},{urlPattern:base+'/v1/product/swarm-scheduler*',requestStage:'Response'}]});summary.status='ready';
 async function drain(reason,round){
  const until=performance.now()+caps.drainMs;
  const unfinished=()=>[...network.values()].some(row=>!row.terminal&&(round===undefined||row.round===round));
  while(pending.size||handlers.size||unfinished()){
   await Promise.allSettled([...pending.values()].map(row=>release(row,reason)));
   if(performance.now()>=until)throw fail('Header drain incomplete; owned browser closure required');
   await sleep(5);
  }
 }
 return {
  enterRound(row){assert(!closed&&!active&&!failure);assert.equal(pending.size,0);assert(['baseline','defer'].includes(row.condition));assert(Number.isSafeInteger(row.round)&&row.round>=0);active={condition:row.condition,round:row.round};event('round-start',active);},
  async endRound(){const previous=active;active=null;await drain('round-complete',previous?.round);event('round-end',previous??{});if(failure)throw failure;},
  async longTasks(page,reason){
   const value=await page.evaluate(()=>window.__luciHeaderDeliveryLongTasks?.snapshot());
   assert(value&&value.version===1&&!value.unsupported&&value.dropped===0&&Number.isFinite(value.timeOrigin)&&Array.isArray(value.rows)&&value.rows.length<=2048,'Invalid or unavailable long-task observation');
   assert(longDocument===undefined||longDocument===value.timeOrigin,'Unexpected document change during diagnostic');longDocument=value.timeOrigin;
   for(const row of value.rows){assert.equal(row.sequence,++longSequence);assert(Number.isFinite(row.startTime)&&row.startTime>=0&&Number.isFinite(row.duration)&&row.duration>=0);}
   assert.equal(longSequence,value.observed);assert(longSequence<=10000,'Long-task row cap exceeded');const window={reason,...value};budget(window);summary.longTaskWindows.push(window);
  },
  async close(){if(closing)return summary.status==='closed';closing=true;closed=true;active=null;try{await drain('finally-release');assert.equal(pending.size,0);assert.equal(handlers.size,0);await bounded(cdp.send('Fetch.disable'),caps.commandMs,'Disable settled header interception timed out');await cdp.detach();summary.status='closed';summary.pausedIDsAtClose=[];return true;}catch{summary.status='incomplete';summary.pausedIDsAtClose=[...pending.values()].map(row=>row.data.sequence);fail('Header interception cleanup incomplete');return false;}},
  stats(){return {pending:pending.size,handlers:handlers.size,valid:summary.valid,status:summary.status};},
 };
}
