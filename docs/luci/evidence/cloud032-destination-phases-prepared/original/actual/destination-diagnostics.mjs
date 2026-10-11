// Additive numeric diagnostic. No selector retries are measured or inferred.
import assert from 'node:assert/strict';

// Self-contained: Playwright serializes this body into the existing evaluate RPC.
// The original connected/value predicate, immediate sample, rAF recursion,
// ten-second deadline and expected-DOM marker remain the completion oracle.
export function destinationValueWitness(el,{value,action,projectId}) {
 return new Promise((done,reject)=>{
  const until=performance.now()+10000;
  const observation=window.__luciDestinationPhase;
  const token=observation?.evaluationEnter(action,projectId,value,el);
  const sample=()=>{
   const match=el.isConnected&&el.value===value;
   observation?.evaluationSample(token,el,match);
   if(match){window.__luciAcceptanceLatency.markExpectedDOM('project-switch');observation?.evaluationWitness(token,el);return done();}
   if(performance.now()>=until)return reject(Error('Expected project context did not render'));
   requestAnimationFrame(sample);
  };
  sample();
 });
}

const snapshotKeys=['version','timeOrigin','armed','actions','destinations','reads','accepted','commits','evaluations','samples','witnesses','observed','dropped','errors','overlaps','mismatches','active','pendingReads','pendingEvaluations','rows'];
const rowKeys=['sequence','action','read','kind','at','connected','match','samples'];
const counters=['actions','destinations','reads','accepted','commits','evaluations','samples','witnesses','observed','dropped','errors','overlaps','mismatches'];
const awaitStages=['waitFor-start','waitFor-end','evaluate-start','evaluate-end','inputValue-start','inputValue-end'];

// Validate only retained observations; no extra page call, selector or waiting.
// The existing full100 Home validator and original action assertions still run.
export function validateDestinationCapture(receipt,expectedActions=100) {
 assert(Number.isSafeInteger(expectedActions)&&expectedActions>=1&&expectedActions<=100);
 const windows=receipt.browserEventWindows.map(row=>row.destinationPhase).filter(Boolean);
 assert(windows.length>0,'Missing destination observations');
 let previous=0,last=null,origin=null;const rows=[];
 for(const window of windows){
  assert.deepEqual(Object.keys(window).sort(),[...snapshotKeys].sort(),'Unexpected destination snapshot fields');
  assert.equal(window.version,1);assert(Number.isFinite(window.timeOrigin));
  assert([0,1].includes(window.armed));assert(Array.isArray(window.rows)&&window.rows.length<=2048);
  for(const name of snapshotKeys.filter(name=>!['version','timeOrigin','rows'].includes(name)))assert(Number.isSafeInteger(window[name])&&window[name]>=0,'Invalid numeric destination counter: '+name);
  if(origin===null&&window.armed)origin=window.timeOrigin;
  if(window.armed)assert.equal(window.timeOrigin,origin,'Measured destination document changed');
  if(last&&window.timeOrigin===last.timeOrigin)for(const name of counters)assert(window[name]>=last[name],'Destination counter regressed: '+name);
  if(!window.armed){assert.equal(window.rows.length,0);assert.equal(window.observed,0);continue;}
  for(const name of ['dropped','errors','overlaps','mismatches'])assert.equal(window[name],0,'Invalid destination diagnostic '+name);
  assert(window.actions<=expectedActions&&window.observed<=2048);
  for(const row of window.rows){
   assert(Object.keys(row).every(key=>rowKeys.includes(key)),'Unexpected private or unbounded destination row field');
   for(const name of ['sequence','action','read','kind','at'])assert(Object.hasOwn(row,name),'Missing destination row field');
   assert(Object.values(row).every(Number.isFinite));
   assert.equal(row.sequence,++previous,'Destination sequence gap/duplicate');
   assert(Number.isSafeInteger(row.action)&&row.action>=1&&row.action<=expectedActions);
   assert(Number.isSafeInteger(row.kind)&&row.kind>=1&&row.kind<=8);
   assert(Number.isSafeInteger(row.read)&&row.read>=0&&row.read<=expectedActions);
   if(Object.hasOwn(row,'connected'))assert([0,1].includes(row.connected));
   if(Object.hasOwn(row,'match'))assert([0,1].includes(row.match));
   if(Object.hasOwn(row,'samples'))assert(Number.isSafeInteger(row.samples)&&row.samples>=1);
   rows.push(row);
  }
  assert.equal(previous,window.observed,'Destination counter does not match retained rows');last=window;
 }
 assert(last&&last.armed===1);
 for(const name of ['actions','destinations','reads','accepted','commits','evaluations','witnesses'])assert.equal(last[name],expectedActions,'Incomplete destination '+name);
 for(const name of ['active','pendingReads','pendingEvaluations'])assert.equal(last[name],0,'Unsettled destination '+name);
 assert.equal(rows.length,expectedActions*8);
 let samples=0;
 for(let action=1;action<=expectedActions;action++){
  const phases=rows.filter(row=>row.action===action),byKind=new Map();
  for(let kind=1;kind<=8;kind++){
   const matches=phases.filter(row=>row.kind===kind);assert.equal(matches.length,1,'Exactly one destination phase per action');byKind.set(kind,matches[0]);
  }
  for(const [before,after] of [[1,2],[2,3],[3,4],[4,5],[2,6],[6,7],[7,8],[5,8]]){
   assert(byKind.get(before).sequence<byKind.get(after).sequence,'Invalid destination causal order');
   assert(byKind.get(before).at<=byKind.get(after).at,'Browser destination time regressed');
  }
  const read=byKind.get(3).read;assert(read>=1);
  for(const kind of [4,5,8])assert.equal(byKind.get(kind).read,read,'Destination read identity changed');
  assert.equal(byKind.get(7).samples,1);assert.equal(byKind.get(8).connected,1);assert.equal(byKind.get(8).match,1);
  samples+=byKind.get(8).samples;
 }
 assert.equal(last.samples,samples,'Destination sample total changed');
 const switches=receipt.interactions.filter(row=>row.kind==='project-switch');assert.equal(switches.length,expectedActions);
 for(const item of switches){
  assert.equal(item.success,true);
  const stages=item.diagnostics.nodeStages;
  const names=stages.map(row=>row.stage);
  let previousIndex=names.indexOf('verify-start');assert(previousIndex>=0);
  for(const name of [...awaitStages,'verify-end']){
   assert.equal(names.filter(value=>value===name).length,1,'Missing or duplicate per-await marker');
   const index=names.indexOf(name);assert(index>previousIndex);assert(stages[index].offsetMs>=stages[previousIndex].offsetMs);previousIndex=index;
  }
 }
 if(receipt.homePhaseCapture)assert.equal(receipt.homePhaseCapture.homes,expectedActions,'Home/destination ordinal count mismatch');
 receipt.destinationPhaseCapture={version:1,complete:true,actions:expectedActions,rows:rows.length,samples,identityJoin:'Sequential measured Home/navigation action, exact project/read token/accepted object/textarea matched only in memory; no private identifiers emitted',originalMetricUnchanged:true,diagnosticNotAcceptance:true,locatorPollAttemptsObserved:false,clockRule:'Node and browser spans remain separate; no mixed-clock subtraction, overhead subtraction or polling attribution'};
}
