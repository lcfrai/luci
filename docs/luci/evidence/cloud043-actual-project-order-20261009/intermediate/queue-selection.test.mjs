// Actual helper selection/loop slices; inert GET and worker/context fixtures.
// No controller, browser, provider, Docker request or native workload starts.
import test from 'node:test';
import assert from 'node:assert/strict';
import {readFileSync} from 'node:fs';
import vm from 'node:vm';
const source=readFileSync(process.env.LUCI_SUSTAINED_TEST_SOURCE??new URL('./sustained.mjs',import.meta.url),'utf8');
const retained=JSON.parse(readFileSync(new URL('./fixtures/queue-order-retained.json',import.meta.url)));
function fixture(){return structuredClone(retained.observations).map(row=>{row.owner.queued=row.queue.items.filter(item=>item.taskId===row.owner.task);return row;});}
function selectionContext(rows=fixture(),options={}){
 const reads=[];
 const ctx={assert,api:async(method,path)=>{assert.equal(method,'GET','Selection must not mutate dispatch');reads.push(path);const row=rows.find(row=>path===row.owner.route+'/swarm-queue');assert(row,'Unknown project route');return options.queue?options.queue(row):row.queue;},until:async(label,fn,budget)=>{assert.equal(budget,150000);for(let i=0;i<2;i++){const value=await fn();if(value)return value;}throw Error(label+' did not complete within its bounded prerequisite window');}};
 if(source.includes('async function allocatedQueuedOwner(')){
  vm.runInNewContext(source.slice(source.indexOf('async function allocatedQueuedOwner('),source.indexOf('async function queuedChild('))+'\nglobalThis.select=allocatedQueuedOwner;',ctx);
 }else{
  // Exact original C-only selector, ending before session/bundle inspection.
  const start=source.indexOf('async function queuedChild('),end=source.indexOf(' const session=await api(',start);assert(start>=0&&end>start);
  vm.runInNewContext(source.slice(start,end)+'return {owner,queued};}\nglobalThis.select=candidates=>queuedChild(Array.isArray(candidates)?candidates[0]:candidates);',ctx);
 }
 return {ctx,rows,reads,select:candidates=>ctx.select(candidates)};
}
const remaining=rows=>rows.filter(row=>['build-c','build-d'].includes(row.owner.task)).map(row=>row.owner);

test('retained D-before-C allocation is selected without changing its queue or issuing work',async()=>{
 const f=selectionContext(),before=JSON.stringify(f.rows);const chosen=await f.select(remaining(f.rows));
 assert.equal(chosen.owner.task,'build-d');assert.equal(chosen.owner.owner.id,'p-f686c006');assert.equal(chosen.queued.childId,'s-58d19866');assert.equal(chosen.queued.id,'queue-e271747355890121c64156ef');
 assert.equal(JSON.stringify(f.rows),before);assert.equal(f.reads.length,2);assert(f.reads.every(path=>path.endsWith('/swarm-queue')));
});

test('empty and ambiguous active candidates refuse instead of choosing or launching a worker',async()=>{
 const none=selectionContext(fixture(),{queue:row=>({items:row.queue.items.map(item=>({...item,state:'queued',childId:undefined}))})});
 await assert.rejects(none.select(remaining(none.rows)),/bounded prerequisite window/);assert.equal(none.reads.length,4);
 const two=selectionContext(fixture(),{queue:row=>({items:row.queue.items.map(item=>({...item,state:'running',childId:item.childId??'controlled-second'}))})});
 await assert.rejects(two.select(remaining(two.rows)),/One worker ceiling/);
});

test('foreign task project run enqueue identity and missing child cannot establish an owned allocation',async()=>{
 for(const change of [item=>({...item,projectId:'other'}),item=>({...item,taskId:'unexpected'}),item=>({...item,runId:'other'}),item=>({...item,id:'other'}),item=>({...item,childId:''})]){
  const f=selectionContext(fixture(),{queue:row=>({items:row.queue.items.map(item=>item.state==='running'?change(item):item)})});
  await assert.rejects(f.select(remaining(f.rows)));
 }
});

test('cancelled waiting fixture cannot allocate and duplicate candidate sets remain bounded',async()=>{
 const rows=fixture(),a=rows.find(row=>row.owner.task==='build-a');a.queue.items.find(item=>item.taskId==='cancel-a').childId='must-not-exist';
 const f=selectionContext(rows);await assert.rejects(f.select(a.owner),/Cancelled waiting fixture task/);
 await assert.rejects(f.select([]),/Bounded remaining/);await assert.rejects(f.select(Array(5).fill(a.owner)),/Bounded remaining/);
 await assert.rejects(f.select([a.owner,a.owner]),/Duplicate candidate project/);
 const duplicate={...a.owner,owner:{id:'different'}};await assert.rejects(f.select([a.owner,duplicate]),/Duplicate candidate task/);
});

test('a known high-priority successor is still required before equal-priority peers',async()=>{
 const f=selectionContext();const b=f.rows.find(row=>row.owner.task==='build-b').owner;
 await assert.rejects(f.select([b]),/bounded prerequisite window/);
 assert.equal(f.reads.length,2);assert(f.reads.every(path=>path===b.route+'/swarm-queue'));
});

function workerContext(overrides={}){
 const f=selectionContext(),row=f.rows.find(row=>row.owner.task==='build-d'),owner=row.owner,queued=row.queue.items[0];
 owner.owner.path='/controlled/primary-d';owner.canonicalLuci=Buffer.from('Exact project D brief');
 const session={id:queued.childId,projectId:owner.owner.id,implementationContext:{taskIds:[owner.task]}};
 const operation={id:'op-13e7db55e7a454367ad14551',taskId:owner.task,childId:session.id,worktreeId:'w-d'};
 const run={id:owner.run.id,projectId:owner.owner.id,operations:[operation]};const bundle={};const checks=[];
 const originalApi=f.ctx.api;
 Object.assign(f.ctx,{api:async(method,path)=>{
  assert.equal(method,'GET');if(path.endsWith('/swarm-queue'))return originalApi(method,path);
  if(path==='/v1/sessions/'+session.id)return session;
  if(path===owner.route+'/runs/'+owner.run.id)return {run};
  if(path==='/v1/projects/'+owner.owner.id+'/worktrees')return {worktrees:[{id:'w-d',path:'/controlled/isolated-d'}]};
  assert.fail('Unknown request '+path);
 },workspace:'',ownedSessions:new Set(),taskData:new Map(),initialBundles:new Map(),bundles:new Map([[session.id,'/controlled/context-d']]),receipt:{},taskContextVersion:1,names:[],canonicalNames:[],
  live:id=>{assert.equal(id,session.id);assert.equal(f.ctx.taskData.get(owner.task).checkout,'/controlled/isolated-d');return {id:'immutable-d',pid:17};},bundleSnapshot:id=>{assert.equal(id,session.id);return bundle;},
  verifyTaskContext:value=>{checks.push(value);assert.equal(value.session.projectId,owner.owner.id);assert.equal(value.operation,operation);assert.equal(value.run,run);},
  row:()=>session,readFileSync:path=>path.endsWith('/LUCI.md')?owner.canonicalLuci:JSON.stringify({projectId:owner.owner.id}),join:(...parts)=>parts.join('/'),workerContainers:()=>['immutable-d'],Buffer,...overrides});
 const start=source.indexOf('async function queuedChild('),end=source.indexOf('async function setPaused(',start);
 vm.runInNewContext(source.slice(start,end)+'\nglobalThis.child=queuedChild;',f.ctx);
 return {...f,owner,session,checks,child:()=>f.ctx.child([owner],3)};
}

test('actual child helper retains owned context and single-container checks after peer selection',async()=>{
 const f=workerContext();const child=await f.child();assert.equal(child.task,'build-d');assert.equal(child.session.id,'s-58d19866');assert.equal(f.checks.length,1);assert.equal(f.ctx.ownedSessions.size,1);assert.equal(f.ctx.initialBundles.size,1);
 assert.equal(f.ctx.receipt.allocationOrder[0].ordinal,3);assert.equal(f.ctx.receipt.allocationOrder[0].queueID,'queue-e271747355890121c64156ef');
 const excess=workerContext({workerContainers:()=>['one','two']});await assert.rejects(excess.child(),/One worker ceiling/);
 const foreign=workerContext({row:()=>({projectId:'foreign',implementationContext:{taskIds:['build-d']}})});await assert.rejects(foreign.child());
 for(const taskIds of [[],['build-d','build-d']]){const invalid=workerContext();invalid.session.implementationContext.taskIds=taskIds;await assert.rejects(invalid.child());}
 const missing=workerContext();delete missing.session.implementationContext;await assert.rejects(missing.child());
 const wrongDocument=workerContext({readFileSync:path=>path.endsWith('/LUCI.md')?Buffer.from('Exact project D brief'):JSON.stringify({projectId:'foreign'})});await assert.rejects(wrongDocument.child(),/Mixed-project context/);
});

for(const order of [['build-a','build-b','build-d','build-c'],['build-a','build-b','build-c','build-d']])test('actual completion loop visits each project once with scheduler order '+order.join(' → '),async()=>{
 const rows=fixture();for(const row of rows)row.owner.run={id:row.owner.run.id};
 const events=[],completed=[],reviewed=new Set();let active=order[0];
 const f=selectionContext(rows,{queue:row=>({items:row.queue.items.map(item=>({...item,state:item.taskId==='cancel-a'?'cancelled':item.taskId===active?'running':completed.includes(item.taskId)?'completed':'queued',childId:item.taskId==='cancel-a'?null:item.taskId===active?'controlled-'+item.taskId:undefined}))})});
 const state=task=>({task,session:{id:'controlled-'+task}});
 const ctx={assert,projectStates:rows.map(row=>row.owner),worker:state(active),completedOwners:[],receipt:{},
  setStage:value=>events.push(value),exercise:async(worker,options)=>{assert.equal(worker.task,active);assert.equal(options.restart,completed.length===0);},
  completeNaturally:async(worker,owner)=>{assert.equal(worker.task,owner.task);assert(!completed.includes(owner.task));completed.push(owner.task);active=order[completed.length];events.push('completed '+owner.task);},
  queuedChild:async(candidates,ordinal)=>{assert.equal(ordinal,completed.length+1);const chosen=await f.select(candidates);events.push('observed '+chosen.owner.task);return state(chosen.owner.task);},attach:async()=>{},
  api:async(method,path)=>{assert.equal(method,'GET');const owner=rows.find(row=>path===row.owner.route+'/runs/'+row.owner.run.id)?.owner;assert(owner);return {run:{swarm:{tasks:[{id:owner.task,integration:{state:reviewed.has(owner.task)?'integrated':'pending'}}]}}};},
  reviewOwner:async owner=>{assert(!reviewed.has(owner.task));reviewed.add(owner.task);events.push('reviewed '+owner.task);},openProject:async()=>{},interactions:{warm:async()=>{}}};
 const marker=source.indexOf(' const completedOwners=[];'),start=source.indexOf(' for(let index=0;index<4;index++){',marker),end=source.indexOf(' phaseEnd(loaded);',start);assert(marker>=0&&start>marker&&end>start);
 await vm.runInNewContext('(async()=>{'+source.slice(start,end)+'})()',ctx);
 assert.deepEqual(completed,order);assert.equal(reviewed.size,4);assert.equal(new Set(ctx.completedOwners.map(owner=>owner.owner.id)).size,4);
 assert.equal(ctx.receipt.preIntegrationProgress.length,3);for(let i=0;i<3;i++)assert(events.indexOf('observed '+order[i+1])<events.indexOf('reviewed '+order[i]));
 assert(!events.some(value=>value.includes('cancel-a')));assert(f.reads.every(path=>path.endsWith('/swarm-queue')));
});
