// Actual helper selection/loop slices; inert GET and worker/context fixtures.
// No controller, browser, provider, Docker request or native workload starts.
import test from 'node:test';
import assert from 'node:assert/strict';
import {readFileSync} from 'node:fs';
import vm from 'node:vm';
const source=readFileSync(process.env.LUCI_SUSTAINED_TEST_SOURCE??new URL('./sustained.mjs',import.meta.url),'utf8');
const retained=JSON.parse(readFileSync(new URL('./fixtures/queue-order-retained.json',import.meta.url)));
function fixture(){return structuredClone(retained.observations).map(row=>{row.owner.queued=structuredClone(row.queue.items);return row;});}
function selectionContext(rows=fixture(),options={}){
 const reads=[],checkpoints=[],diagnosticReads=[];
 const ctx={assert,receipt:{},stage:'actual-work-build-a',workspace:'',evidence:'/controlled/evidence',join:(...parts)=>parts.join('/'),encodeURIComponent,
  setStage:value=>{ctx.stage=value;},writeFailureCheckpoint:(path,value)=>{assert(!checkpoints.some(entry=>entry.path===path));checkpoints.push({path,value:structuredClone(value)});options.checkpoint?.(path,value);},
  api:async(method,path,body,expected,timeout)=>{
   assert.equal(method,'GET','Selection must not mutate dispatch');reads.push(path);
   const row=rows.find(row=>path===row.owner.route+'/swarm-queue');if(row)return options.queue?options.queue(row):row.queue;
   assert.equal(body,undefined);assert.equal(expected,200);assert.equal(timeout,3000,'Diagnostic reads stay bounded');
   assert.equal(checkpoints.length,1,'Refusal must be durable before diagnostic awaits');diagnosticReads.push(path);
   if(options.read)return options.read(path);throw Error('Controlled diagnostic unavailable');
  },until:async(label,fn,budget)=>{assert.equal(budget,150000);for(let i=0;i<2;i++){const value=await fn();if(value)return value;}throw Error(label+' did not complete within its bounded prerequisite window');}};
 if(source.includes('async function allocatedQueuedOwner(')){
  const start=source.indexOf('async function failRequiredQueuedTarget(');
  vm.runInNewContext(source.slice(start<0?source.indexOf('async function allocatedQueuedOwner('):start,source.indexOf('async function queuedChild('))+'\nglobalThis.select=allocatedQueuedOwner;',ctx);
 }else{
  // Exact original C-only selector, ending before session/bundle inspection.
  const start=source.indexOf('async function queuedChild('),end=source.indexOf(' const session=await api(',start);assert(start>=0&&end>start);
  vm.runInNewContext(source.slice(start,end)+'return {owner,queued};}\nglobalThis.select=candidates=>queuedChild(Array.isArray(candidates)?candidates[0]:candidates);',ctx);
 }
 return {ctx,rows,reads,checkpoints,diagnosticReads,select:candidates=>ctx.select(candidates)};
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

test('high priority within B does not predict its cross-project turn',async()=>{
 const rows=fixture();const b=rows.find(row=>row.owner.task==='build-b');b.queue.items[0].state='queued';b.queue.items[0].childId=null;
 assert.equal(b.queue.items[0].priority,'high');const f=selectionContext(rows);
 const selected=await f.select(rows.filter(row=>row.owner.task!=='build-a').map(row=>row.owner));
 assert.equal(selected.owner.task,'build-d');assert.equal(b.queue.items[0].priority,'high');assert.equal(f.reads.length,3);
});

function terminalFixture(state='failed'){
 const rows=fixture(),b=rows.find(row=>row.owner.task==='build-b'),item=b.queue.items[0];
 Object.assign(item,{state,reasonCode:'cpu_pressure',operationId:'op-820720a86bb1fae84a2f7330',updatedAt:'2026-10-09T03:46:39.591940852Z',reason:'PRIVATE message',taskTitle:'PRIVATE title'});
 const failure={stage:'admission',reasonCode:'cpu_pressure',message:'PRIVATE message'};
 const run={id:b.owner.run.id,projectId:b.owner.owner.id,revision:6,state:'blocked',operations:[{id:item.operationId,taskId:item.taskId,childId:item.childId,status:'failed',startFailure:failure,implementation:'PRIVATE context'}],context:'PRIVATE context'};
 const session={id:item.childId,projectId:item.projectId,projectRunId:item.runId,projectRunOperationId:item.operationId,status:'exited',startFailure:failure,attachToken:'PRIVATE token',openingPrompt:'PRIVATE prompt'};
 const scheduler={revision:4,policy:{maxActive:1,maxPerProject:1,headroomPercent:25,minHeadroomMiB:2048,diskReserveMiB:1024},resources:{host:{observedAt:'2026-10-09T03:46:40Z',status:'ready',cpu:{utilizationPercent:12.5,effectiveMillis:4000},memory:{effectiveTotalBytes:8589934592,effectiveAvailableBytes:4294967296},disks:[{role:'state',availableBytes:6500000000,path:'PRIVATE path'}],processes:[{command:'PRIVATE command'}]},execution:{status:'ready',totalMemoryBytes:8589934592,cpuMillis:4000,containers:[{environment:'PRIVATE env'}]},reservations:{status:'ready',count:0,memoryMiB:0,cpuMillis:0}}};
 const read=async path=>{if(path===b.owner.route+'/runs/'+b.owner.run.id)return {run};if(path==='/v1/sessions/'+item.childId)return session;if(path==='/v1/product/swarm-scheduler')return scheduler;assert.fail('Unexpected diagnostic request');};
 return {rows,b,item,run,session,scheduler,read,candidates:rows.filter(row=>['build-b','build-c','build-d'].includes(row.owner.task)).map(row=>row.owner)};
}
test('required failed B is durably diagnosed before accepting running D, without replay or raw private fields',async()=>{
 const t=terminalFixture(),before=JSON.stringify(t.rows),f=selectionContext(t.rows,{read:t.read});
 await assert.rejects(f.select(t.candidates),error=>error.code==='required-queued-task-terminal'&&/build-b is failed \(cpu_pressure\)/.test(error.message));
 assert.equal(JSON.stringify(t.rows),before);assert.equal(f.reads.filter(path=>path.endsWith('/swarm-queue')).length,3,'One observation, no prerequisite polling');
 assert.equal(f.checkpoints.length,2);const [initial,complete]=f.checkpoints.map(entry=>entry.value);
 assert.equal(initial.diagnosticReads,'pending');assert.equal(initial.queue.id,t.item.id);assert.equal(initial.observedDuringStage,'actual-work-build-a');assert.equal(initial.runningPeers[0].taskId,'build-d');
 assert.equal(complete.run.id,t.run.id);assert.equal(complete.operation.id,t.item.operationId);assert.equal(complete.session.id,t.item.childId);assert.equal(complete.operation.startFailure.reasonCode,'cpu_pressure');
 assert.equal(complete.scheduler.policy.headroomPercent,25);assert.equal(complete.resources.host.cpu.utilizationPercent,12.5);assert.match(complete.resourceObservationBasis,/non-atomic/);assert.match(complete.resourceObservationBasis,/Exact admission operands are not retained/);
 assert.equal(complete.retryAttempted,false);assert.equal(f.ctx.stage,'required-queue-terminal-build-b');assert(!JSON.stringify(f.checkpoints).includes('PRIVATE'));
 assert.deepEqual(Object.values(complete.diagnosticReads),['observed','observed','observed']);
});
test('required cancelled target without a child fails promptly while deliberate extra cancellation stays ignored',async()=>{
 const t=terminalFixture('cancelled');delete t.item.childId;delete t.item.operationId;const f=selectionContext(t.rows,{read:t.read});
 await assert.rejects(f.select(t.candidates),/build-b is cancelled/);assert.equal(f.diagnosticReads.length,2);assert(!f.checkpoints[1].value.session);assert(!f.checkpoints[1].value.operation);
 const rows=fixture(),a=rows.find(row=>row.owner.task==='build-a');a.queue.items.find(item=>item.taskId==='cancel-a').state='cancelled';a.queue.items.find(item=>item.taskId==='build-a').state='queued';
 const ignored=selectionContext(rows);assert.equal((await ignored.select(rows.filter(row=>row.owner.task!=='build-b').map(row=>row.owner))).owner.task,'build-d');assert.equal(ignored.checkpoints.length,0);
});
test('required completion before receipt capture fails distinctly without accepting a verified result',async()=>{
 const t=terminalFixture('completed'),f=selectionContext(t.rows,{read:t.read});
 await assert.rejects(f.select(t.candidates),error=>error.code==='required-queued-task-completed-before-capture'&&/completed before receipt capture; no verified task result was accepted/.test(error.message));
 assert.equal(f.checkpoints[0].value.queue.state,'completed');assert.equal(f.checkpoints[1].value.retryAttempted,false);assert(!f.ctx.receipt.allocationOrder);assert(!f.ctx.receipt.taskReceiptSettlements);
});
test('terminal refusal remains the failure when run session and scheduler diagnostics are unavailable',async()=>{
 const t=terminalFixture(),f=selectionContext(t.rows);await assert.rejects(f.select(t.b.owner),error=>error.code==='required-queued-task-terminal');
 assert.equal(f.checkpoints.length,2);assert.deepEqual(Object.values(f.checkpoints[1].value.diagnosticReads),['unavailable','unavailable','unavailable']);assert.equal(f.reads.filter(path=>path.endsWith('/swarm-queue')).length,1);
});
test('blocked unknown and other nonterminal required targets retain conservative selection and waiting',async()=>{
 for(const state of ['blocked','unknown','queued','paused','reserved']){
  const t=terminalFixture(state),f=selectionContext(t.rows);assert.equal((await f.select(t.candidates)).owner.task,'build-d');assert.equal(f.checkpoints.length,0);
  await assert.rejects(f.select(t.b.owner),/bounded prerequisite window/);assert.equal(f.checkpoints.length,0);
 }
});
test('terminal target requires its exact project run enqueue and unique identity before diagnostic reads',async()=>{
 for(const mutate of [t=>{t.item.projectId='foreign';},t=>{t.item.runId='foreign';},t=>{t.item.id='foreign';},t=>{t.b.queue.items.push({...t.item});}]){
  const t=terminalFixture();mutate(t);const f=selectionContext(t.rows);await assert.rejects(f.select(t.candidates));assert.equal(f.checkpoints.length,0);assert.equal(f.diagnosticReads.length,0);
 }
});
test('terminal peer never bypasses running identity and one-worker guards',async()=>{
 for(const mutate of [t=>{t.rows.find(row=>row.owner.task==='build-d').queue.items[0].runId='foreign';},t=>{const c=t.rows.find(row=>row.owner.task==='build-c').queue.items[0];c.state='running';c.childId='controlled-second';}]){
  const t=terminalFixture();mutate(t);const f=selectionContext(t.rows);await assert.rejects(f.select(t.candidates));assert.equal(f.checkpoints.length,0);assert.equal(f.diagnosticReads.length,0);
 }
});
test('diagnostic run operation and session identities cannot attach another attempt to the refusal',async()=>{
 for(const mutate of [t=>{t.run.id='foreign';},t=>{t.run.projectId='foreign';},t=>{t.run.operations[0].id='foreign';},t=>{t.run.operations[0].taskId='foreign';},t=>{t.run.operations[0].childId='foreign';},t=>{t.run.operations.push({...t.run.operations[0]});}]){
  const t=terminalFixture();mutate(t);const f=selectionContext(t.rows,{read:t.read});await assert.rejects(f.select(t.candidates),error=>error.code==='required-queued-task-terminal');const diagnostic=f.checkpoints[1].value;
  assert.equal(diagnostic.diagnosticReads.run,'identity-or-shape-mismatch');assert(!diagnostic.operation);assert(!JSON.stringify(diagnostic).includes('foreign'));
 }
 for(const field of ['id','projectId','projectRunId','projectRunOperationId']){
  const t=terminalFixture();t.session[field]='foreign';const f=selectionContext(t.rows,{read:t.read});await assert.rejects(f.select(t.candidates),error=>error.code==='required-queued-task-terminal');
  assert.equal(f.checkpoints[1].value.diagnosticReads.session,'identity-or-shape-mismatch');assert(!f.checkpoints[1].value.session);
 }
});
test('failure to durably record the initial refusal cannot proceed to diagnostics or accept a worker',async()=>{
 const t=terminalFixture(),f=selectionContext(t.rows,{checkpoint:()=>{throw Error('controlled disk failure');}});
 await assert.rejects(f.select(t.candidates),/controlled disk failure/);assert.equal(f.diagnosticReads.length,0);
});

function actualEvidence(metric='host_cpu_utilization_percent'){
 return {metric,observedAt:'2020-01-02T03:04:00.000000001Z',decidedAt:'2020-01-02T03:04:05.000000001Z',observedPercent:metric==='host_cpu_utilization_percent'?81.25:0,maximumPercent:metric==='host_cpu_utilization_percent'?75:0,observedBytes:0,requiredBytes:metric==='host_cpu_utilization_percent'?0:3221225472};
}
async function captureAdmission(t){
 const before=JSON.stringify(t),f=selectionContext(t.rows,{read:t.read});
 await assert.rejects(f.select(t.candidates),error=>error.code==='required-queued-task-terminal');assert.equal(JSON.stringify(t),before);
 assert.equal(f.checkpoints.length,2);assert(!f.checkpoints[0].value.session);
 return f.checkpoints[1].value;
}
test('exact historical CPU admission operands remain distinct from later non-atomic scheduler observations',async()=>{
 const t=terminalFixture(),expected=actualEvidence();t.session.startAdmissionEvidence={...expected,command:'PRIVATE command',nested:{token:'PRIVATE token'}};
 const diagnostic=await captureAdmission(t);assert.deepEqual(diagnostic.session.startAdmissionEvidence,expected,'Exact actual decision operands were not captured');assert.equal(diagnostic.admissionDecision.status,'retained');assert.match(diagnostic.admissionDecision.basis,/Actual final admission operands/);assert.match(diagnostic.admissionDecision.basis,/decidedAt/);
 assert.equal(diagnostic.resources.host.cpu.utilizationPercent,12.5);assert.match(diagnostic.resourceObservationBasis,/non-atomic/);assert(!JSON.stringify(diagnostic).includes('PRIVATE'));assert.equal(diagnostic.retryAttempted,false);
});
for(const metric of ['host_available_memory','execution_total_memory'])test('exact '+metric+' admission preserves measured zero and the original required bytes',async()=>{
 const t=terminalFixture();t.item.reasonCode='memory_pressure';t.session.startFailure.reasonCode='memory_pressure';const expected=actualEvidence(metric);t.session.startAdmissionEvidence=expected;
 const diagnostic=await captureAdmission(t);assert.deepEqual(diagnostic.session.startAdmissionEvidence,expected,'Exact actual decision operands were not captured');assert.equal(diagnostic.admissionDecision.status,'retained');assert.equal(diagnostic.session.startAdmissionEvidence.observedBytes,0);
});
test('missing optional admission evidence and incomplete operands remain explicitly unavailable',async()=>{
 for(const evidence of [undefined,null,{},[],{...actualEvidence(),observedPercent:undefined},{...actualEvidence(),observedBytes:undefined}]){
  const t=terminalFixture();t.session.startAdmissionEvidence=evidence;const diagnostic=await captureAdmission(t);assert.equal(diagnostic.admissionDecision.status,'unavailable');assert(!diagnostic.session.startAdmissionEvidence);assert.equal(diagnostic.diagnosticReads.session,'observed');
 }
});
test('malformed admission enums and nonfinite out-of-range or unsafe numeric operands are omitted',async()=>{
 for(const changes of [{metric:'PRIVATE metric'},{observedPercent:NaN},{observedPercent:Infinity},{maximumPercent:-Infinity},{observedPercent:'81.25'},{maximumPercent:-1},{maximumPercent:101},{observedPercent:101},{observedPercent:75},{observedPercent:-1},{observedBytes:1},{requiredBytes:1},{observedBytes:false},{observedBytes:-1},{observedBytes:0.5}]){
  const t=terminalFixture();t.session.startAdmissionEvidence={...actualEvidence(),...changes};const diagnostic=await captureAdmission(t);assert.equal(diagnostic.admissionDecision.status,'unavailable');assert(!diagnostic.session.startAdmissionEvidence);
 }
 for(const changes of [{observedPercent:1},{maximumPercent:1},{requiredBytes:0},{observedBytes:3221225472},{observedBytes:Number.MAX_SAFE_INTEGER+1,requiredBytes:Number.MAX_SAFE_INTEGER+2},{requiredBytes:1.5}]){
  const t=terminalFixture();t.session.startFailure.reasonCode='memory_pressure';t.session.startAdmissionEvidence={...actualEvidence('host_available_memory'),...changes};const diagnostic=await captureAdmission(t);assert.equal(diagnostic.admissionDecision.status,'unavailable');assert(!diagnostic.session.startAdmissionEvidence);
 }
});
test('admission freshness uses precise original timestamps and rejects stale future or malformed samples',async()=>{
 for(const changes of [{observedAt:'2020-01-02T03:04:00.000000000Z'},{observedAt:'2020-01-02T03:04:05.000000002Z'},{observedAt:'2020-02-30T03:04:00Z'},{observedAt:'not-a-time'},{decidedAt:null},{decidedAt:'2020-01-02T03:04:05.000000001Z PRIVATE'},{observedAt:'0001-01-01T00:00:00Z',decidedAt:'0001-01-01T00:00:00Z'}]){
  const t=terminalFixture();t.session.startAdmissionEvidence={...actualEvidence(),...changes};const diagnostic=await captureAdmission(t);assert.equal(diagnostic.admissionDecision.status,'unavailable');assert(!diagnostic.session.startAdmissionEvidence);
 }
});
test('actual admission operands require the matching failure stage reason and exited session',async()=>{
 for(const alter of [t=>{t.session.startFailure.stage='run-revalidation';},t=>{t.session.startFailure.reasonCode='memory_pressure';},t=>{t.session.status='running';},t=>{delete t.session.startFailure;}]){
  const t=terminalFixture();t.session.startAdmissionEvidence=actualEvidence();alter(t);const diagnostic=await captureAdmission(t);assert.equal(diagnostic.admissionDecision.status,'unavailable');assert(!diagnostic.session.startAdmissionEvidence);
 }
});
test('actual admission evidence is never attached when exact session identity validation fails',async()=>{
 for(const key of ['id','projectId','projectRunId','projectRunOperationId']){
  const t=terminalFixture();t.session.startAdmissionEvidence=actualEvidence();t.session[key]='foreign';const diagnostic=await captureAdmission(t);assert.equal(diagnostic.admissionDecision.status,'unavailable');assert.equal(diagnostic.diagnosticReads.session,'identity-or-shape-mismatch');assert(!diagnostic.session);
 }
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

function permutations(items){return items.length?items.flatMap((item,index)=>permutations(items.filter((_,i)=>i!==index)).map(rest=>[item,...rest])):[[]];}
async function executeWorkflow(order,{canceledChild=false}={}){
 const rows=fixture();for(const row of rows)row.owner.run={id:row.owner.run.id};
 const events=[],completed=[],reviewed=new Set();let active=order[0];
 const f=selectionContext(rows,{queue:row=>({items:row.queue.items.map(item=>({...item,state:item.taskId==='cancel-a'?'paused':item.taskId===active?'running':completed.includes(item.taskId)?'completed':'queued',childId:item.taskId==='cancel-a'?(canceledChild?'forbidden-child':null):item.taskId===active?'controlled-'+item.taskId:undefined,operationId:undefined}))})});
 const state=task=>({task,session:{id:'controlled-'+task}});
 const ctx={assert,projectStates:rows.map(row=>row.owner),receipt:{},
  setStage:value=>events.push(value),exercise:async(worker,options)=>{assert.equal(worker.task,active);assert.equal(options.restart,completed.length===0);},
  completeNaturally:async(worker,owner)=>{assert.equal(worker.task,owner.task);assert(!completed.includes(owner.task));completed.push(owner.task);active=order[completed.length];events.push('completed '+owner.task);},
  queuedChild:async(candidates,ordinal)=>{assert.equal(ordinal,completed.length+1);const chosen=await f.select(candidates);events.push('observed '+chosen.owner.task);return state(chosen.owner.task);},attach:async()=>{},
  queueControls:async worker=>{assert.equal(worker.task,order[0]);events.push('controls during '+worker.task);},
  api:async(method,path)=>{assert.equal(method,'GET');const owner=rows.find(row=>path===row.owner.route+'/runs/'+row.owner.run.id)?.owner;assert(owner);return {run:{swarm:{tasks:[{id:owner.task,integration:{state:reviewed.has(owner.task)?'integrated':'pending'}}]}}};},
  reviewOwner:async owner=>{assert(!reviewed.has(owner.task));reviewed.add(owner.task);events.push('reviewed '+owner.task);},openProject:async()=>{},interactions:{warm:async()=>{}}};
 const start=source.indexOf(' let worker=await queuedChild('),end=source.indexOf(' phaseEnd(loaded);',start);assert(start>=0&&end>start);
 await vm.runInNewContext('(async()=>{'+source.slice(start,end)+'globalThis.completedScope=completedOwners;})()',ctx);
 assert.deepEqual(completed,order);assert.equal(reviewed.size,4);assert.equal(new Set(ctx.completedScope.map(owner=>owner.owner.id)).size,4);
 assert.equal(ctx.receipt.preIntegrationProgress.length,3);for(let i=0;i<3;i++)assert(events.indexOf('observed '+order[i+1])<events.indexOf('reviewed '+order[i]));
 assert.equal(events.filter(value=>value.startsWith('observed ')).length,4);assert(!events.some(value=>value.includes('cancel-a')));assert(f.reads.every(path=>path.endsWith('/swarm-queue')));
}
for(const order of permutations(['build-a','build-b','build-c','build-d']))test('actual startup and completion loop visits each project once with scheduler order '+order.join(' → '),()=>executeWorkflow(order));
test('actual initial discovery rejects the canceled child before exercise even when D starts first',async()=>{
 await assert.rejects(executeWorkflow(['build-d','build-a','build-c','build-b'],{canceledChild:true}),/Cancelled waiting fixture task/);
});

function controlsContext(active='build-d',options={}){
 const rows=fixture(),events=[],actions=[],observed=[];let opened=null,loaded=false;
 for(const row of rows)for(const item of row.queue.items){item.state='queued';item.childId=null;delete item.operationId;item.priority='normal';}
 const a=rows.find(row=>row.owner.task==='build-a'),cancel=a.queue.items.find(item=>item.taskId==='cancel-a');
 const locate=(method,name)=>({click:async()=>{assert.equal(method,'button');const task=opened.queue.items.find(item=>item.taskId==='cancel-a');assert(task);if(name==='Pause queued task'){assert.equal(task.state,'queued');task.state='paused';}else{assert.equal(name,'Cancel queued task');assert.equal(task.state,'paused');task.state='cancelled';}events.push(name);},selectOption:async value=>{assert.equal(method,'combobox');assert.equal(name,'Priority for '+opened.owner.task);assert.notEqual(opened.owner.task,active);const item=opened.queue.items.find(item=>item.taskId===opened.owner.task);assert(['queued','blocked'].includes(item.state));assert(!item.childId&&!item.operationId);assert.equal(value,'high');item.priority=value;events.push('priority '+opened.owner.task);}});
 const ctx={assert,projectStates:rows.map(row=>row.owner),receipt:{},page:{},health:()=>{},titles:new Map(rows.map(row=>[row.owner.task,row.owner.task])),setStage:stage=>events.push(stage),
  effects:()=>({create:loaded?1:0,start:loaded?1:0}),privateRun:()=>({operations:[]}),check:()=>{},
  api:async(method,path)=>{assert.equal(method,'GET');const row=rows.find(row=>path===row.owner.route+'/swarm-queue');assert(row);observed.push(row.owner.task);const result=structuredClone(row.queue);delete result.projectId;result.dispatchPaused=!loaded;return options.transform?options.transform(result,row):result;},
  openProject:async owner=>{opened=rows.find(row=>row.owner===owner);assert(opened);},
  openQueueControls:async()=>({locator:type=>{assert.equal(type,'li');return {filter:query=>{assert.equal(query.hasText,'Cancel this extra queued task');return {getByRole:(role,{name,exact})=>{assert(exact);return locate(role,name);}};}};},getByRole:(role,{name,exact})=>{assert(exact);return locate(role,name);}}),
  action:async(path,fn)=>{assert(opened);assert(path.startsWith(opened.owner.route+'/swarm-queue/'));const entry=opened.queue.items.find(item=>path.includes('/'+item.id+'/'));assert(entry);actions.push({path,task:entry.taskId});await fn();},
 };
 const start=source.indexOf('function exactQueueControlEntry('),end=source.indexOf('async function activeRestart(',start);assert(start>=0&&end>start,'Correction must expose actual guarded UI control functions');
 vm.runInNewContext(source.slice(start,end)+'globalThis.protect=protectCancellation;globalThis.controls=queueControls;',ctx);
 return {ctx,rows,a,cancel,events,actions,observed,protect:()=>ctx.protect(),controls:()=>{loaded=true;const entry=rows.find(row=>row.owner.task===active).queue.items.find(item=>item.taskId===active);entry.state='running';entry.childId='controlled-'+active;return ctx.controls({task:active});}};
}
for(const active of ['build-a','build-b','build-c','build-d'])test('actual queue controls protect exact A then cancel under load with '+active+' active',async()=>{
 const f=controlsContext(active);await f.protect();assert.equal(f.cancel.state,'paused');assert.equal(f.ctx.receipt.queueCancellationGuard.queueID,f.cancel.id);
 await f.controls();assert.equal(f.cancel.state,'cancelled');assert.equal(f.actions.length,3);assert.deepEqual(f.actions.map(value=>value.path.split('/').at(-1)),['pause','cancel','priority']);
 assert.deepEqual(f.actions.slice(0,2).map(value=>value.task),['cancel-a','cancel-a']);const priority=f.ctx.receipt.queueControls.prioritized;
 assert.notEqual(priority.task,active);assert.equal(priority.priority,'high');assert.equal(f.ctx.receipt.queueControls.activeTask,active);
 assert.equal(f.rows.flatMap(row=>row.queue.items).filter(item=>item.priority==='high').length,1);assert(!f.cancel.childId&&!f.cancel.operationId);
 const setupStart=source.indexOf(" for(const owner of projectStates){setStage('prepare-and-queue-'"),guard=source.indexOf('await protectCancellation();',setupStart),resume=source.indexOf('await setPaused(false);',setupStart);
 assert(setupStart>=0&&guard>setupStart&&resume>guard,'The exact cancellation pause must precede global resume');
});
test('queue control observations reject wrong scope duplicate identity and allocated targets before action',async()=>{
 for(const alter of [q=>({...q,items:q.items.map(i=>({...i,projectId:'other'}))}),q=>({...q,items:q.items.map(i=>i.taskId==='cancel-a'?{...i,runId:'other'}:i)}),q=>({...q,items:q.items.map(i=>i.taskId==='cancel-a'?{...i,childId:'unexpected'}:i)}),q=>({...q,items:q.items.map(i=>i.taskId==='cancel-a'?{...i,operationId:'unexpected'}:i)}),q=>({...q,items:[...q.items,...q.items.filter(i=>i.taskId==='cancel-a')]})]){
  const f=controlsContext('build-d',{transform:(q,row)=>row.owner.task==='build-a'?alter(q):q});await assert.rejects(f.protect());assert.equal(f.actions.length,0);
 }
});
test('queue priority requires another current waiting owner and exact high readback',async()=>{
 const none=controlsContext('build-d');await none.protect();for(const row of none.rows)for(const item of row.queue.items)if(item.taskId!=='cancel-a'&&item.taskId!=='build-d')item.state='failed';
 await assert.rejects(none.controls(),/different waiting priority/);assert.equal(none.actions.length,2);
 let prioritySent=false;const wrong=controlsContext('build-d',{transform:(q)=>prioritySent?({...q,items:q.items.map(i=>({...i,priority:'normal'}))}):q});await wrong.protect();const original=wrong.ctx.action;wrong.ctx.action=async(path,fn)=>{await original(path,fn);if(path.endsWith('/priority'))prioritySent=true;};await assert.rejects(wrong.controls());
});
