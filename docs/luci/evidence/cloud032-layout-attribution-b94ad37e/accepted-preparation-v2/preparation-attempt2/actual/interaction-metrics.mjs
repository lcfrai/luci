// Passive browser observation plus explicit real-user action measurements.
// These helpers never change product state through injected JavaScript.
import assert from 'node:assert/strict';
import {navigationContextWitness,navigationWitnessSlot} from './navigation-verifier.mjs';
import {browserLatencyInit,browserFrameWitness,nodeStageRecorder,createLatencyLedger} from './latency-diagnostics.mjs';
export const nearestRank = (values, q) => values.length ? [...values].sort((a,b)=>a-b)[Math.max(0,Math.ceil(values.length*q)-1)] : null;
export function summarizeInteractions(rows) {
 const groups={};for(const row of rows){const key=row.phase+':'+row.kind;(groups[key]??=[]).push(row);}
 return Object.fromEntries(Object.entries(groups).map(([key,items])=>{
  const successful=items.filter(row=>row.success),durations=items.map(row=>row.elapsedMs);
  return [key,{count:items.length,successful:successful.length,failed:items.length-successful.length,p50Ms:nearestRank(durations,.5),p95Ms:nearestRank(durations,.95),p99Ms:nearestRank(durations,.99),maxMs:Math.max(...durations),basis:'Automation command through expected DOM content at two animation frames; includes transport and any failed/timed-out action duration, not physical display latency.',timeouts:items.filter(row=>!row.success).map(row=>({at:row.at,elapsedMs:row.elapsedMs,error:row.error}))}];
 }));
}
export async function installBrowserTiming(context) {
 // Separate setup-only RPC, before any primary interaction measurement.
 await context.addInitScript(browserLatencyInit);
 await context.addInitScript(()=>{
  const data={version:2,timeOrigin:performance.timeOrigin,events:[],observedEvents:0,droppedEvents:0,overflow:false,unsupported:false,visibility:document.visibilityState};
  Object.defineProperty(window,'__luciAcceptanceTiming',{value:data,configurable:false});
  try {if(!PerformanceObserver.supportedEntryTypes.includes('event')){data.unsupported=true;return;}const observer=new PerformanceObserver(list=>{for(const event of list.getEntries()){
   const sequence=++data.observedEvents;
   if(data.events.length>=10000){data.overflow=true;data.droppedEvents++;continue;}
   data.events.push({sequence,name:event.name,startTime:event.startTime,duration:event.duration,processingStart:event.processingStart,processingEnd:event.processingEnd,interactionId:event.interactionId??0});
  }});observer.observe({type:'event',buffered:true,durationThreshold:16});}catch{data.unsupported=true;}
 });
}
export async function timingSnapshot(page) {
 // Drain only this fixture's passive observation buffer, never product state.
 // Sequence and overflow counters remain monotonic across every snapshot.
 return page.evaluate(()=>{const v=window.__luciAcceptanceTiming;if(!v)return null;const latency=window.__luciAcceptanceLatency?.snapshot();if(!latency)throw Error('Missing browser latency collector');return {...v,events:v.events.splice(0),visibilityNow:document.visibilityState,latency};});
}

export function createTimingLedger(receipt,limits={maxEvents:200000,maxBytes:32*1024**2,maxWindows:2000,maxDocuments:128}) {
 const documents=new Map();let count=0,bytes=0;const latencyLedger=createLatencyLedger(receipt);
 receipt.browserEventWindows??=[];
 receipt.browserTimingLedger={limits:{...limits},events:0,bytes:0,documents:0,sequenceVerified:true,scope:'Incrementally drained passive Event Timing; unsupported documents stay explicit. Counts and sequence checks do not establish physical input latency.'};
 return {
  append(value,reason){
   if(!value)return;
   const row={reason,at:new Date().toISOString(),...value};
   const fail=message=>{receipt.browserTimingLedger.sequenceVerified=false;receipt.browserTimingFailure={message,reason,timeOrigin:value.timeOrigin,observedEvents:value.observedEvents,droppedEvents:value.droppedEvents,receivedEvents:value.events?.length??null};throw Error(message);};
   if(value.version!==2||!Number.isFinite(value.timeOrigin)||!Array.isArray(value.events)||value.events.length>10000||!Number.isSafeInteger(value.observedEvents)||value.observedEvents<0||!Number.isSafeInteger(value.droppedEvents)||value.droppedEvents<0)fail('Invalid passive browser observation shape');
   const size=Buffer.byteLength(JSON.stringify(row));
   if(count+value.events.length>limits.maxEvents||bytes+size>limits.maxBytes||receipt.browserEventWindows.length>=limits.maxWindows||!documents.has(value.timeOrigin)&&documents.size>=limits.maxDocuments)fail('Passive browser observation ledger exceeded its declared bound');
   // Preserve an in-budget overflow/gap window before refusing acceptance.
   receipt.browserEventWindows.push(row);count+=value.events.length;bytes+=size;
   if(Object.hasOwn(value,'latency'))latencyLedger.append(value.latency);
   Object.assign(receipt.browserTimingLedger,{events:count,bytes,documents:documents.size+(documents.has(value.timeOrigin)?0:1)});
   let previous=documents.get(value.timeOrigin)??0;
   for(const event of value.events){if(!Number.isSafeInteger(event.sequence)||event.sequence!==previous+1)fail('Passive browser event sequence contains a gap or duplicate');previous=event.sequence;}
   if(value.overflow||value.droppedEvents!==0)fail('Browser observation overflow invalidates event metrics');
   if(previous!==value.observedEvents)fail('Passive browser observation counter does not match its retained sequence');
   documents.set(value.timeOrigin,previous);
  },
 };
}

export function interactionRunner({page,owners,receipt,assertHealthy,phase,observe=async()=>{}}) {
 const values=new Map(owners.map(owner=>[owner.owner.id,owner.goal]));
 receipt.interactions??=[];
 let current;
 const frames=()=>page.evaluate(browserFrameWitness);
 async function measured(kind,owner,action,verify){
  assert(receipt.interactions.length<6000,'Bounded interaction measurement count');
  const item={phase:phase(),kind,projectID:owner.owner.id,at:new Date().toISOString(),success:false};
  const start=performance.now(),diagnostic=nodeStageRecorder(item,start);
  const navigationSlot=navigationWitnessSlot();
  let timeout;try {await Promise.race([(async()=>{diagnostic.mark('action-start');await action(diagnostic.mark);diagnostic.mark('action-end');diagnostic.mark('verify-start');await verify(diagnostic.mark,navigationSlot.accept);diagnostic.mark('verify-end');diagnostic.mark('frames-start');diagnostic.frames(await frames());diagnostic.mark('frames-end');})(),new Promise((_,reject)=>{timeout=setTimeout(()=>reject(Error('Interaction deadline exceeded after15seconds')),15000);})]);item.success=true;}
  catch(error){item.error=String(error.message).slice(0,300);throw error;}
  finally{clearTimeout(timeout);item.elapsedMs=performance.now()-start;diagnostic.close();const navigationWitness=navigationSlot.close();if(kind==='project-switch')item.navigationVerification={status:navigationWitness===undefined?'unavailable':'returned',reason:navigationWitness===undefined?'original-evaluate-not-returned-before-action-closed':null,witness:navigationWitness??null};receipt.interactions.push(item);}
 }
 async function switchTo(owner){
  assertHealthy();
  await measured('project-switch',owner,async mark=>{await page.getByRole('button',{name:'All projects',exact:true}).click();mark('all-projects-click-end');await page.getByRole('heading',{name:'Across your projects',exact:true}).waitFor();mark('overview-heading-ready');mark('open-project-click-start');await page.getByRole('button',{name:'Open project '+owner.owner.name,exact:true}).click();mark('open-project-click-end');},async (mark,retainNavigation)=>{
   // The calibrated lens is Define. Its actual loaded value is the witness,
   // rather than a title that could render before stale content is replaced.
   const field=page.getByRole('textbox',{name:'The idea',exact:true});await field.waitFor();mark('idea-field-wait-end');
   const navigationWitness=await field.evaluate(navigationContextWitness,values.get(owner.owner.id));mark('idea-context-evaluate-end');retainNavigation(navigationWitness);
   assert.equal(await field.inputValue(),values.get(owner.owner.id));
  });current=owner;
 }
 async function typeRound(){
  const owner=current;assert(owner);assertHealthy();
  const field=page.getByRole('textbox',{name:'The idea',exact:true}),expected=values.get(owner.owner.id);
  await field.focus();await field.press('End');
  await measured('input',owner,()=>field.press('x'),async()=>{await page.waitForFunction(value=>{if(document.activeElement?.value!==value)return false;window.__luciAcceptanceLatency.markExpectedDOM('input');return true;},expected+'x',{timeout:10000});});
  await measured('input',owner,()=>field.press('Backspace'),async()=>{await page.waitForFunction(value=>{if(document.activeElement?.value!==value)return false;window.__luciAcceptanceLatency.markExpectedDOM('input');return true;},expected,{timeout:10000});});
  // The product deliberately keeps dirty=true even if text returns to baseline.
  // Reset through its real discard flow; record this extra action separately.
  await measured('draft-reset',owner,async()=>{await page.getByRole('button',{name:'Design',exact:true}).click();await page.getByRole('dialog',{name:'Keep your changes?',exact:true}).getByRole('button',{name:'Discard edits and continue',exact:true}).click();await page.getByRole('button',{name:'Define',exact:true}).click();},async()=>{assert.equal(await field.inputValue(),expected);});
 }
 return {
  async calibrate(rounds=100){for(let index=0;index<rounds;index++){await switchTo(owners[index%owners.length]);await typeRound();await observe('after-calibration-round');}},
  async round(index){await switchTo(owners[index%owners.length]);await typeRound();await observe('after-loaded-round');},
  expected(owner,value){values.set(owner.owner.id,value);},
  async warm(){for(const owner of owners){await page.getByRole('button',{name:'All projects',exact:true}).click();await page.getByRole('button',{name:'Open project '+owner.owner.name,exact:true}).click();await page.getByRole('button',{name:'Define',exact:true}).click();assert.equal(await page.getByRole('textbox',{name:'The idea',exact:true}).inputValue(),values.get(owner.owner.id));}current=owners.at(-1);},
 };
}
