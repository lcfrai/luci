import assert from 'node:assert/strict';
import {test} from 'node:test';
import {readFile} from 'node:fs/promises';
import {interactionRunner as originalRunner} from './original/interaction-metrics.mjs';
import {interactionRunner as candidateRunner} from './candidate/interaction-metrics.mjs';
import {nodeStageRecorder} from './candidate/latency-diagnostics.mjs';
import {projectBoundaryRows,SWITCH_STAGES} from './boundaries.mjs';
const newMarks=['open-project-click-start','idea-field-wait-end','idea-context-evaluate-end'];
const owner={owner:{id:'synthetic-id',name:'Synthetic project'},goal:'Synthetic saved value'};
async function run(factory,fail=null){
 const before=Object.getOwnPropertyDescriptor(globalThis,'performance');let now=1000;const calls=[],receipt={};
 Object.defineProperty(globalThis,'performance',{configurable:true,value:{now:()=>now}});
 const step=async(name,cost,value)=>{calls.push(name);now+=cost;if(name===fail)throw Error('Synthetic bounded failure: '+name);return value;};
 const locator=(role,name)=>({
  getByRole:(r,o)=>locator(r,o.name),
  click:()=>step('click:'+name,name.startsWith('Open project ')?119:11),
  waitFor:()=>step('wait:'+name,role==='textbox'?73:7),
  evaluate:()=>step('evaluate:'+name,141),
  inputValue:()=>step('inputValue:'+name,52,owner.goal),
  focus:()=>step('focus:'+name,2),press:key=>step('press:'+key,3),
 });
 const page={getByRole:(role,options)=>locator(role,options.name),evaluate:()=>step('frames',13,{requestedAt:1,firstFrameAt:2,secondFrameAt:3,references:null}),waitForFunction:()=>step('input-verify',4)};
 let error=null;try{await factory({page,owners:[owner],receipt,assertHealthy:()=>{},phase:()=> 'synthetic-boundary-contract'}).round(0);}catch(e){error=e.message;}finally{Object.defineProperty(globalThis,'performance',before);}
 return {calls,rows:receipt.interactions,error};
}
test('same original method order/count, actions, outcomes and full spans; 12 marks delimit all three awaits',async()=>{
 const a=await run(originalRunner),b=await run(candidateRunner);assert.deepEqual(a.calls,b.calls);assert.equal(b.error,null);assert.equal(b.rows.length,4);
 assert.deepEqual(b.rows.map(x=>[x.kind,x.success,x.elapsedMs]),a.rows.map(x=>[x.kind,x.success,x.elapsedMs]));
 const projected=projectBoundaryRows(b.rows);assert.deepEqual(projected.map(x=>x.issues),[[],[],[],[]]);
 assert.deepEqual(b.rows[0].diagnostics.nodeStages.map(x=>x.stage),SWITCH_STAGES);assert.deepEqual(projected[0].segments,{openCommand:119,fieldWait:73,contextEvaluate:141,inputValueAndAssert:52});
 assert.deepEqual(b.rows.map(x=>x.diagnostics.nodeClockReads),[12,6,6,6]);assert.deepEqual(a.rows.map(x=>x.diagnostics.nodeClockReads),[9,6,6,6]);
 assert.deepEqual(b.rows[0].diagnostics.nodeStages.filter(x=>!newMarks.includes(x.stage)),a.rows[0].diagnostics.nodeStages);
 // Fake clock counts only controlled commands: equal spans prove no command additions,
 // not zero real instrumentation overhead or real app/browser readiness.
});
for(const fail of ['click:Open project Synthetic project','wait:The idea','evaluate:The idea','inputValue:The idea'])test('same failure and completed prefix: '+fail,async()=>{
 const a=await run(originalRunner,fail),b=await run(candidateRunner,fail);assert.deepEqual(a.calls,b.calls);assert.equal(a.error,b.error);assert.equal(b.rows.length,1);assert.equal(b.rows[0].success,false);assert.equal(b.rows[0].elapsedMs,a.rows[0].elapsedMs);assert.deepEqual(projectBoundaryRows(b.rows)[0].issues,[]);
 assert.deepEqual(b.rows[0].diagnostics.nodeStages.filter(x=>!newMarks.includes(x.stage)),a.rows[0].diagnostics.nodeStages);
});
test('original overflow, unknown label and closed callback guards remain strict',()=>{
 const item={},rec=nodeStageRecorder(item,0,()=>1);for(const name of SWITCH_STAGES)rec.mark(name);assert.equal(item.diagnostics.nodeClockReads,12);assert.throws(()=>rec.mark('action-start'));assert.throws(()=>rec.mark('unknown'));rec.close();rec.mark('action-start');assert.equal(item.diagnostics.nodeClockReads,12);
});
test('projection rejects excess rows/stages and refuses duplicate/missing/reordered/nonfinite evidence without context leakage',async()=>{
 const {rows}=await run(candidateRunner);const valid=rows[0];
 assert.throws(()=>projectBoundaryRows(Array(6001).fill(valid)));
 const excess=structuredClone(valid);excess.diagnostics.nodeStages.push({stage:'frames-end',offsetMs:1});assert.throws(()=>projectBoundaryRows([excess]));
 for(const mutate of [r=>r.diagnostics.nodeStages.splice(7,1),r=>r.diagnostics.nodeStages[7].stage='verify-start',r=>r.diagnostics.nodeStages[7].offsetMs=NaN,r=>r.diagnostics.nodeClockReads=11]){const v=structuredClone(valid);mutate(v);const p=projectBoundaryRows([v])[0];assert(p.issues.length);assert.deepEqual(p.segments,{});}
 const privateRow={...valid,projectID:'PRIVATE_ID',privateContext:'PRIVATE_CONTEXT',error:'PRIVATE_ERROR'};assert(!JSON.stringify(projectBoundaryRows([privateRow])).includes('PRIVATE_'));
});
test('candidate differs from frozen modules only by explicit mark plumbing and allowlist; no bounds/actions changed',async()=>{
 let source=await readFile(new URL('./candidate/interaction-metrics.mjs',import.meta.url),'utf8');
 source=source.replace('await verify(diagnostic.mark);','await verify();').replace("mark('open-project-click-end');},async mark=>{","mark('open-project-click-end');},async()=>{");
 for(const mark of newMarks)source=source.replace(`mark('${mark}');`,'');
 assert.equal(source,await readFile(new URL('./original/interaction-metrics.mjs',import.meta.url),'utf8'));
 const diag=(await readFile(new URL('./candidate/latency-diagnostics.mjs',import.meta.url),'utf8')).replace(",'open-project-click-start','idea-field-wait-end','idea-context-evaluate-end'",'');
 assert.equal(diag,await readFile(new URL('./original/latency-diagnostics.mjs',import.meta.url),'utf8'));
});
