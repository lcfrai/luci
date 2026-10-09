// Diagnostic-only response delivery intervention. Never fetches/replays/fulfills a request.
import assert from 'node:assert/strict';
import {createHash} from 'node:crypto';

export function installLongTaskCollector() {
 const state={version:1,timeOrigin:performance.timeOrigin,rows:[],observed:0,dropped:0,unsupported:false};
 try {
  if(!PerformanceObserver.supportedEntryTypes.includes('longtask'))state.unsupported=true;
  else new PerformanceObserver(list=>{for(const entry of list.getEntries()){
   const sequence=++state.observed;
   if(state.rows.length===2048){state.dropped++;continue;}
   state.rows.push({sequence,startTime:entry.startTime,duration:entry.duration});
  }}).observe({type:'longtask',buffered:true});
 } catch {state.unsupported=true;}
 Object.defineProperty(window,'__luciSummaryDeliveryLongTasks',{value:{snapshot:()=>({...state,rows:state.rows.splice(0)})},configurable:false});
}

export async function createSummaryDelivery({page,base,projectIDs,receipt,onFailure,limits={},session}) {
 const caps={maxPaused:16,maxBodyBytes:2*1024**2,maxRetainedBytes:8*1024**2,maxRequests:1000,maxMetadataBytes:4*1024**2,bodyTimeoutMs:5000,holdTimeoutMs:5000,cancelWitnessMs:500,drainTimeoutMs:6000,...limits};
 assert.equal(new URL(base).origin,base);assert.equal(projectIDs.length,4);assert.equal(new Set(projectIDs).size,4);
 const paths=new Map(projectIDs.map((id,index)=>{assert(/^[A-Za-z0-9_-]+$/.test(id));return ['/v1/product/projects/'+id+'/overview',{route:'home-overview',projectIndex:index}];}));
 paths.set('/v1/product/swarm-scheduler',{route:'home-capacity'});
 const classify=request=>{try{const url=new URL(request.url);return request.method==='GET'&&url.origin===base&&url.search===''?paths.get(url.pathname):undefined;}catch{return undefined;}};
 const cdp=session??await page.context().newCDPSession(page),pending=new Map(),handlers=new Set(),network=new Map();
 const summary={version:1,diagnosticOnly:true,limits:caps,status:'installing',valid:true,requests:[],pauses:[],longTaskWindows:[],events:[],metadataBytes:0,retainedBodyBytes:0,maximumRetainedBodyBytes:0,scope:'Original authenticated requests and response bytes; both arms use the same response capture. No replay/substitution. A delivery effect includes parsing, callbacks, rendering and polling timing; no natural-performance or React-only claim.'};
 receipt.summaryDelivery=summary;
 let active=null,closed=false,sequence=0,retained=0,metadataBytes=0,failure,closeStarted=false,lastLongSequence=0,longDocument;
 const sleep=ms=>new Promise(resolve=>setTimeout(resolve,ms));
 const fail=error=>{if(!failure){failure=error instanceof Error?error:Error(String(error));summary.valid=false;summary.failure=failure.message;onFailure(failure);}return failure;};
 const record=(kind,fields={})=>{const row={kind,nodeAt:performance.now(),...fields};metadataBytes+=Buffer.byteLength(JSON.stringify(row));summary.metadataBytes=metadataBytes;if(metadataBytes>caps.maxMetadataBytes){fail(Error('Summary delivery metadata cap exceeded'));return;}summary.events.push(row);};
 async function bounded(promise,ms,label){let timer;try{return await Promise.race([promise,new Promise((_,reject)=>{timer=setTimeout(()=>reject(Error(label+' deadline exceeded')),ms);})]);}finally{clearTimeout(timer);}}
 const cancellation=row=>row.networkId&&network.get(row.networkId)?.cancelled===true;
 async function cancelled(row){const until=performance.now()+caps.cancelWitnessMs;while(!cancellation(row)&&performance.now()<until)await sleep(Math.min(10,caps.cancelWitnessMs));return cancellation(row);}
 async function release(row,reason){
  if(!pending.has(row.id)||row.releasing)return;
  assert(!row.bodyInFlight,'Never continue a response while getResponseBody is in flight');row.releasing=true;clearTimeout(row.holdTimer);
  try{await bounded(cdp.send('Fetch.continueResponse',{requestId:row.id}),caps.bodyTimeoutMs,'Continue original response');row.outcome='continued-unchanged';}
  catch(error){if(await cancelled(row)){row.outcome=row.bodyComplete?'naturally-aborted-after-body':'naturally-aborted-before-body';}else{row.outcome='unclassified-protocol-error';fail(error);}}
  finally{retained-=row.retainedBytes??0;row.retainedBytes=0;summary.retainedBodyBytes=retained;pending.delete(row.id);row.releasedAt=performance.now();row.releaseReason=reason;record('release',{sequence:row.sequence,outcome:row.outcome,reason,heldMs:row.bodyCompleteAt===undefined?null:row.releasedAt-row.bodyCompleteAt});}
 }
 async function paused(value){
  const target=classify(value.request),round=active&&{...active};
  const row={id:value.requestId,networkId:value.networkId,sequence:++sequence,...target,status:value.responseStatusCode,condition:round?.condition??null,round:round?.round??null,pausedAt:performance.now(),bodyInFlight:false};
  pending.set(row.id,row);summary.pauses.push(row);record('response-paused',{sequence:row.sequence,...target,status:row.status,condition:row.condition,round:row.round});
  if(pending.size>caps.maxPaused||summary.pauses.length>caps.maxRequests){fail(Error('Summary response count cap exceeded'));await release(row,'cap-refusal');return;}
  if(!target||row.status!==200||!round||closed||failure){await release(row,'non-target-immediate');return;}
  const length=value.responseHeaders?.find(h=>h.name.toLowerCase()==='content-length')?.value;
  if(length!==undefined&&(!/^\d+$/.test(length)||Number(length)>caps.maxBodyBytes)){fail(Error('Declared response body exceeds diagnostic cap'));await release(row,'declared-size-refusal');return;}
  row.bodyInFlight=true;
  // Watchdogs invalidate the run but never issue another Fetch command while body capture is pending.
  const watchdog=setTimeout(()=>fail(Error('Summary response body capture deadline exceeded')),caps.bodyTimeoutMs);
  let body;
  try{body=await cdp.send('Fetch.getResponseBody',{requestId:row.id});}
  catch(error){row.bodyInFlight=false;clearTimeout(watchdog);if(await cancelled(row)){row.outcome='naturally-aborted-before-body';record('body-cancelled',{sequence:row.sequence});await release(row,'natural-prebody-cancellation');return;}fail(error);await release(row,'body-capture-failure');return;}
  finally{row.bodyInFlight=false;clearTimeout(watchdog);}
  const bytes=Buffer.from(body.body,body.base64Encoded?'base64':'utf8');row.bodyBytes=bytes.length;row.bodySHA256=createHash('sha256').update(bytes).digest('hex');row.bodyComplete=true;row.bodyCompleteAt=performance.now();
  // CDP returns a complete body; this is a retained-copy cap, not a peak Chromium allocation claim.
  if(bytes.length>caps.maxBodyBytes||retained+bytes.length>caps.maxRetainedBytes){fail(Error('Captured response body exceeds diagnostic cap'));await release(row,'captured-size-refusal');return;}
  row.retainedBytes=bytes.length;retained+=bytes.length;summary.retainedBodyBytes=retained;summary.maximumRetainedBodyBytes=Math.max(summary.maximumRetainedBodyBytes,retained);record('body-complete',{sequence:row.sequence,bytes:bytes.length,sha256:row.bodySHA256});
  if(round.condition==='baseline'||active?.round!==round.round||closed||failure){await release(row,round.condition==='baseline'?'baseline-immediate':'round-already-finished');return;}
  row.held=true;record('held',{sequence:row.sequence});
  row.holdTimer=setTimeout(()=>{fail(Error('Summary response hold deadline exceeded'));void release(row,'hold-timeout');},caps.holdTimeoutMs);
 }
 const onRequest=value=>{const target=classify(value.request);if(!target)return;if(network.size>=caps.maxRequests){fail(Error('Summary request metadata cap exceeded'));return;}const row={sequence:summary.requests.length+1,...target,condition:active?.condition??null,round:active?.round??null,requestAt:value.timestamp};network.set(value.requestId,row);summary.requests.push(row);};
 const onFailed=value=>{const row=network.get(value.requestId);if(row){row.failedAt=value.timestamp;row.cancelled=value.canceled===true;row.failureClass=row.cancelled?'browser-cancelled':'network-failure';record('network-failed',{sequence:row.sequence,cancelled:row.cancelled});}};
 const onPaused=value=>{const work=paused(value).catch(fail);handlers.add(work);void work.finally(()=>handlers.delete(work));};
 cdp.on('Network.requestWillBeSent',onRequest);cdp.on('Network.loadingFailed',onFailed);cdp.on('Fetch.requestPaused',onPaused);
 await cdp.send('Network.enable');
 await cdp.send('Fetch.enable',{patterns:[{urlPattern:base+'/v1/product/projects/*/overview*',requestStage:'Response'},{urlPattern:base+'/v1/product/swarm-scheduler*',requestStage:'Response'}]});summary.status='ready';
 async function drain(reason){
  const until=performance.now()+caps.drainTimeoutMs;
  while(pending.size||handlers.size){
   for(const row of [...pending.values()])if(!row.bodyInFlight&&!row.releasing)await release(row,reason);
   if(performance.now()>=until)throw fail(Error('Summary response drain incomplete; owned browser closure required'));
   await sleep(5);
  }
 }
 return {
  enterRound(row){assert(!closed&&!active&&!failure);assert.equal(pending.size,0);assert(['baseline','defer'].includes(row.condition));assert(Number.isSafeInteger(row.round)&&row.round>=0);active={condition:row.condition,round:row.round};record('round-start',active);},
  async endRound(){const previous=active;active=null;await drain('round-complete');record('round-end',previous??{});if(failure)throw failure;},
  async longTasks(page,reason){
   const value=await page.evaluate(()=>window.__luciSummaryDeliveryLongTasks?.snapshot());
   assert(value&&value.version===1&&!value.unsupported&&value.dropped===0&&Number.isFinite(value.timeOrigin)&&Array.isArray(value.rows)&&value.rows.length<=2048,'Invalid or unavailable long-task observation');
   assert(longDocument===undefined||longDocument===value.timeOrigin,'Unexpected document change during diagnostic');longDocument=value.timeOrigin;
   for(const row of value.rows){assert.equal(row.sequence,++lastLongSequence);assert(Number.isFinite(row.startTime)&&row.startTime>=0&&Number.isFinite(row.duration)&&row.duration>=0);}
   assert.equal(lastLongSequence,value.observed);assert(lastLongSequence<=10000,'Long-task row cap exceeded');
   const window={reason,...value};metadataBytes+=Buffer.byteLength(JSON.stringify(window));assert(metadataBytes<=caps.maxMetadataBytes,'Long-task metadata cap exceeded');summary.metadataBytes=metadataBytes;summary.longTaskWindows.push(window);
  },
  async close(){if(closeStarted)return summary.status==='closed';closeStarted=true;closed=true;active=null;try{await drain('finally-release');assert.equal(pending.size,0);assert.equal(handlers.size,0);await bounded(cdp.send('Fetch.disable'),caps.bodyTimeoutMs,'Disable settled response interception');await cdp.detach();summary.status='closed';summary.pausedIDsAtClose=[];return true;}catch(error){summary.status='incomplete';summary.pausedIDsAtClose=[...pending.values()].map(row=>({sequence:row.sequence,bodyInFlight:row.bodyInFlight}));fail(error);return false;}},
  stats(){return {pending:pending.size,handlers:handlers.size,valid:summary.valid,status:summary.status};},
 };
}
