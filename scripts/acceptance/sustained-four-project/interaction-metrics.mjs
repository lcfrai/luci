// Passive browser observation plus explicit real-user action measurements.
// These helpers never change product state through injected JavaScript.
import assert from 'node:assert/strict';
export const nearestRank = (values, q) => values.length ? [...values].sort((a,b)=>a-b)[Math.max(0,Math.ceil(values.length*q)-1)] : null;
export function summarizeInteractions(rows) {
 const groups={};for(const row of rows){const key=row.phase+':'+row.kind;(groups[key]??=[]).push(row);}
 return Object.fromEntries(Object.entries(groups).map(([key,items])=>{
  const successful=items.filter(row=>row.success),durations=items.map(row=>row.elapsedMs);
  return [key,{count:items.length,successful:successful.length,failed:items.length-successful.length,p50Ms:nearestRank(durations,.5),p95Ms:nearestRank(durations,.95),p99Ms:nearestRank(durations,.99),maxMs:Math.max(...durations),basis:'Automation command through expected DOM content at two animation frames; includes transport and any failed/timed-out action duration, not physical display latency.',timeouts:items.filter(row=>!row.success).map(row=>({at:row.at,elapsedMs:row.elapsedMs,error:row.error}))}];
 }));
}
export async function installBrowserTiming(context) {
 await context.addInitScript(()=>{
  const data={version:1,timeOrigin:performance.timeOrigin,events:[],overflow:false,unsupported:false,visibility:document.visibilityState,frameOpportunities:[]};
  Object.defineProperty(window,'__luciAcceptanceTiming',{value:data,configurable:false});
  try {if(!PerformanceObserver.supportedEntryTypes.includes('event')){data.unsupported=true;return;}const observer=new PerformanceObserver(list=>{for(const event of list.getEntries()){
   if(data.events.length>=10000){data.overflow=true;continue;}
   data.events.push({name:event.name,startTime:event.startTime,duration:event.duration,processingStart:event.processingStart,processingEnd:event.processingEnd,interactionId:event.interactionId??0});
  }});observer.observe({type:'event',buffered:true,durationThreshold:16});}catch{data.unsupported=true;}
 });
}
export async function timingSnapshot(page) {
 return page.evaluate(()=>{const v=window.__luciAcceptanceTiming;return v?{...v,events:[...v.events],visibilityNow:document.visibilityState}:null;});
}
export function interactionRunner({page,owners,receipt,assertHealthy,phase}) {
 const values=new Map(owners.map(owner=>[owner.owner.id,owner.goal]));
 receipt.interactions??=[];
 let current;
 const frames=()=>page.evaluate(()=>new Promise(resolve=>requestAnimationFrame(()=>requestAnimationFrame(()=>resolve(performance.now())))));
 async function measured(kind,owner,action,verify){
  assert(receipt.interactions.length<6000,'Bounded interaction measurement count');
  const item={phase:phase(),kind,projectID:owner.owner.id,at:new Date().toISOString(),success:false};
  const start=performance.now();
  let timeout;try {await Promise.race([(async()=>{await action();await verify();await frames();})(),new Promise((_,reject)=>{timeout=setTimeout(()=>reject(Error('Interaction deadline exceeded after15seconds')),15000);})]);item.success=true;}
  catch(error){item.error=String(error.message).slice(0,300);throw error;}
  finally{clearTimeout(timeout);item.elapsedMs=performance.now()-start;receipt.interactions.push(item);}
 }
 async function switchTo(owner){
  assertHealthy();
  await measured('project-switch',owner,async()=>{await page.getByRole('button',{name:'All projects',exact:true}).click();await page.getByRole('heading',{name:'Across your projects',exact:true}).waitFor();await page.getByRole('button',{name:'Open project '+owner.owner.name,exact:true}).click();},async()=>{
   // The calibrated lens is Define. Its actual loaded value is the witness,
   // rather than a title that could render before stale content is replaced.
   const field=page.getByLabel('The idea',{exact:true});await field.waitFor();
   await field.evaluate((el,value)=>new Promise((done,reject)=>{const until=performance.now()+10000;const sample=()=>{if(el.isConnected&&el.value===value)return done();if(performance.now()>=until)return reject(Error('Expected project context did not render'));requestAnimationFrame(sample);};sample();}),values.get(owner.owner.id));
   assert.equal(await field.inputValue(),values.get(owner.owner.id));
  });current=owner;
 }
 async function typeRound(){
  const owner=current;assert(owner);assertHealthy();
  const field=page.getByLabel('The idea',{exact:true}),expected=values.get(owner.owner.id);
  await field.focus();await field.press('End');
  await measured('input',owner,()=>field.press('x'),async()=>{await page.waitForFunction(value=>document.activeElement?.value===value,expected+'x',{timeout:10000});});
  await measured('input',owner,()=>field.press('Backspace'),async()=>{await page.waitForFunction(value=>document.activeElement?.value===value,expected,{timeout:10000});});
  // The product deliberately keeps dirty=true even if text returns to baseline.
  // Reset through its real discard flow; record this extra action separately.
  await measured('draft-reset',owner,async()=>{await page.getByRole('button',{name:'Design',exact:true}).click();await page.getByRole('dialog',{name:'Keep your changes?',exact:true}).getByRole('button',{name:'Discard edits and continue',exact:true}).click();await page.getByRole('button',{name:'Define',exact:true}).click();},async()=>{assert.equal(await field.inputValue(),expected);});
 }
 return {
  async calibrate(rounds=100){for(let index=0;index<rounds;index++){await switchTo(owners[index%owners.length]);await typeRound();}},
  async round(index){await switchTo(owners[index%owners.length]);await typeRound();},
  expected(owner,value){values.set(owner.owner.id,value);},
  async warm(){for(const owner of owners){await page.getByRole('button',{name:'All projects',exact:true}).click();await page.getByRole('button',{name:'Open project '+owner.owner.name,exact:true}).click();await page.getByRole('button',{name:'Define',exact:true}).click();assert.equal(await page.getByLabel('The idea',{exact:true}).inputValue(),values.get(owner.owner.id));}current=owners.at(-1);},
 };
}
