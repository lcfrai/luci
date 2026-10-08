// Controlled helper contracts only. These tests never start an installed app,
// browser, Docker client, process, socket, or provider.
import assert from 'node:assert/strict';
import test from 'node:test';
import {routes,exactLease,exactOperation,assertHeld,assertReleased,reviewedIdentity,runWorkerRecoveryScenario} from './worker-recovery-scenario.mjs';

const copy=value=>structuredClone(value);
const target=Object.freeze({teamId:'team-fixture',projectId:'p-target',runId:'run-target',operationId:'op-target',childId:'s-target',taskId:'task-a',queueId:'queue-target',scope:'a'.repeat(64),engineId:'engine-fixture',instanceId:'instance-fixture',containerId:'b'.repeat(64),successorProjectId:'p-unrelated',actor:{userId:'user-fixture',teamId:'team-fixture',projectId:'p-target'}});
function held(){
 const lease={id:target.queueId,scope:target.scope,projectId:target.projectId,sessionId:target.childId,sandboxId:target.childId,state:'backend-settled',launchGuard:1,fenceProtocol:1,fenceEngineId:target.engineId,fenceInstanceId:target.instanceId,actor:{userId:'user-fixture',teamId:target.teamId,projectId:target.projectId},resources:{CPUMillis:1000,MemoryMiB:1024}};
 return {ledger:{version:1,leases:[lease,{...copy(lease),id:'queue-foreign-scope',scope:'c'.repeat(64),projectId:'p-foreign',sessionId:'s-foreign',sandboxId:'s-foreign'}],retirements:[{id:'retained-retirement'}],fences:[{id:'retained-fence'}],checks:[{id:'retained-check'}],checkFences:[]},
  run:{id:target.runId,projectId:target.projectId,revision:7,operations:[{id:target.operationId,childId:target.childId,taskId:target.taskId,allocation:'ready',status:'running'}]},
  session:{id:target.childId,projectId:target.projectId,status:'running',exitReason:''},
  queue:{items:[{id:target.queueId,projectId:target.projectId,runId:target.runId,childId:target.childId,operationId:target.operationId,taskId:target.taskId,state:'running',memoryMiB:1024,cpuMillis:1000}]},
  mutations:[{method:'POST',path:'/containers/create',ref:target.containerId},{method:'POST',path:'/containers/'+target.containerId+'/start'}],
  retained:{output:'WORKER_PROGRESS_RECORDED\n',checkout:{'result.txt':'retained partial output'},context:{digest:'d'.repeat(64)},history:[{id:'history-before-target',output:'prior output\n'}]},
  dispatches:{[target.childId]:1},createsForTarget:1,startsForTarget:1,fenceCreates:0,containerId:target.containerId,containerPID:41001,
  successor:{projectId:target.successorProjectId,state:'queued',childId:'',memoryMiB:0,cpuMillis:0}};
}
function released(before=held()){
 const after=copy(before);after.ledger.leases=after.ledger.leases.filter(row=>row.id!==target.queueId);
 after.session.status='exited';after.session.exitReason='killed';after.run.revision++;
 after.run.operations[0].status='failed';Object.assign(after.queue.items[0],{state:'failed',memoryMiB:0,cpuMillis:0});
 after.exactBackendAbsence={confirmed:true,childId:target.childId,immutableId:target.containerId};
 after.mutations.push({method:'DELETE',path:'/containers/'+target.containerId});return after;
}
const rejectsChange=(base,change,check)=>{const changed=copy(base);change(changed);assert.throws(()=>check(changed),assert.AssertionError);};

test('routes bind all operations to the selected team, project, run and child',()=>{
 const p='/v1/teams/team-fixture/workspace';assert.deepEqual(routes(target),{workspace:p,run:p+'/v1/product/projects/p-target/runs/run-target',stop:p+'/v1/sessions/s-target',queue:p+'/v1/product/projects/p-target/swarm-queue',preview:p+'/v1/product/projects/p-target/runs/run-target/swarm/operations/op-target/recovery',reconcile:p+'/v1/product/projects/p-target/runs/run-target/swarm/reconcile',abandon:p+'/v1/product/projects/p-target/runs/run-target/swarm/tasks/task-a/abandon'});
 assert(!routes(target).stop.includes('purge'));
});
test('routes reject missing, traversing, encoded and oversized identity selectors',()=>{
 for(const key of ['teamId','projectId','runId','operationId','childId','taskId','queueId'])for(const value of ['',undefined,'../foreign','%2Fforeign','foreign?purge=true','x'.repeat(201)])assert.throws(()=>routes({...target,[key]:value}),assert.AssertionError,key);
});
test('exactLease locates the settled target among unrelated scoped claims',()=>{
 const value=held();assert.equal(exactLease(value.ledger,target),value.ledger.leases[0]);
});
test('exactLease refuses wrong queue, scope, project, sandbox, protocol or engine identity',()=>{
 for(const key of ['id','scope','projectId','sandboxId','fenceProtocol','fenceEngineId','fenceInstanceId'])rejectsChange(held(),s=>{s.ledger.leases[0][key]=key==='fenceProtocol'?0:'foreign';},s=>exactLease(s.ledger,target));
 rejectsChange(held(),s=>{s.ledger.leases[0].sessionId='s-other';},s=>exactLease(s.ledger,target));
});
test('exactLease refuses missing, duplicate and cross-scope identity collisions',()=>{
 const value=held();assert.throws(()=>exactLease({version:1,leases:[]},target),assert.AssertionError);
 for(const variant of [copy(value.ledger.leases[0]),{...copy(value.ledger.leases[0]),id:'queue-another'},{...copy(value.ledger.leases[0]),scope:'e'.repeat(64)}])assert.throws(()=>exactLease({...value.ledger,leases:[...value.ledger.leases,variant]},target),assert.AssertionError);
});
test('exactLease refuses a changed actor despite otherwise matching allocation identities',()=>{
 for(const key of ['userId','teamId','projectId'])rejectsChange(held(),s=>{s.ledger.leases[0].actor[key]='foreign';},s=>exactLease(s.ledger,target));
 rejectsChange(held(),s=>{delete s.ledger.leases[0].actor;},s=>exactLease(s.ledger,target));
});
test('exactLease requires settled launchGuard1 and the approved hard resource limits',()=>{
 for(const state of ['reserved','allocating','backend-dispatching','fencing','unknown'])rejectsChange(held(),s=>{s.ledger.leases[0].state=state;},s=>exactLease(s.ledger,target));
 for(const launchGuard of [0,undefined])rejectsChange(held(),s=>{s.ledger.leases[0].launchGuard=launchGuard;},s=>exactLease(s.ledger,target));
 for(const resources of [{CPUMillis:2000,MemoryMiB:1024},{CPUMillis:1000,MemoryMiB:2048},{}])rejectsChange(held(),s=>{s.ledger.leases[0].resources=resources;},s=>exactLease(s.ledger,target));
});
test('reviewedIdentity derives fresh CAS selection and rejects invalid revisions',()=>{
 const snapshot=held();assert.deepEqual(reviewedIdentity(snapshot,target),{expectedRevision:7,operationId:target.operationId,childId:target.childId});
 for(const revision of [0,-1,1.5,NaN,Number.MAX_SAFE_INTEGER+1,undefined])rejectsChange(snapshot,s=>{s.run.revision=revision;},s=>reviewedIdentity(s,target));
});
test('operation review rejects foreign run/session, ambiguous operations and wrong queue child',()=>{
 const cases=[s=>{s.run.id='run-foreign';},s=>{s.run.projectId='p-foreign';},s=>{s.session.id='s-foreign';},s=>{s.session.projectId='p-foreign';},s=>{s.run.operations.push(copy(s.run.operations[0]));},s=>{s.run.operations[0].taskId='task-foreign';},s=>{s.queue.items[0].childId='s-foreign';},s=>{s.queue.items.push(copy(s.queue.items[0]));}];
 for(const change of cases)rejectsChange(held(),change,s=>exactOperation(s,target));
});
test('held refusal accepts refreshed run metadata while preserving the exact charge and artifacts',()=>{
 const before=held(),after=copy(before);after.run.revision++;assert.doesNotThrow(()=>assertHeld(before,after,target));
});
test('held refusal rejects charge changes, unrelated ledger changes, Docker mutation and replay',()=>{
 const before=held();for(const change of [s=>{s.ledger.leases[0].resources.MemoryMiB=0;},s=>{s.ledger.leases[1].state='allocating';},s=>{s.ledger.checks=[];},s=>{s.mutations.push({method:'DELETE',ref:target.containerId});},s=>{s.dispatches[target.childId]=2;},s=>{s.queue.items[0].memoryMiB=0;},s=>{s.retained.output='';}])rejectsChange(before,change,s=>assertHeld(before,s,target));
});
test('released contract removes exactly the target and preserves unrelated private histories',()=>{
 const before=held();assert.doesNotThrow(()=>assertReleased(before,released(before),target));
});
test('release accepts product JSON omitting zero queue counters only after exact lease removal',()=>{
 const before=held(),after=released(before);delete after.queue.items[0].memoryMiB;delete after.queue.items[0].cpuMillis;
 delete after.successor.memoryMiB;delete after.successor.cpuMillis;
 assert.doesNotThrow(()=>assertReleased(before,after,target));
 after.ledger.leases.push(copy(before.ledger.leases[0]));assert.throws(()=>assertReleased(before,after,target),assert.AssertionError);
});
test('release refuses stopped-but-present, unknown and wrong-identity absence claims',()=>{
 const before=held();for(const proof of [undefined,{confirmed:false,state:'stopped',childId:target.childId,immutableId:target.containerId},{confirmed:false,state:'unknown',childId:target.childId,immutableId:target.containerId},{confirmed:true,childId:'s-other',immutableId:target.containerId},{confirmed:true,childId:target.childId,immutableId:'f'.repeat(64)}])rejectsChange(released(before),s=>{s.exactBackendAbsence=proof;},s=>assertReleased(before,s,target));
});
test('release refuses a retained target, changed foreign lease, retired history or check claim',()=>{
 const before=held();for(const change of [s=>{s.ledger.leases.push(copy(before.ledger.leases[0]));},s=>{s.ledger.leases=[];},s=>{s.ledger.leases[0].resources.MemoryMiB=512;},s=>{s.ledger.retirements=[];},s=>{s.ledger.fences=[];},s=>{s.ledger.checks=[];}])rejectsChange(released(before),change,s=>assertReleased(before,s,target));
});
test('release refuses queue capacity, active target and premature unrelated dispatch',()=>{
 const before=held();for(const change of [s=>{s.queue.items[0].memoryMiB=1024;},s=>{s.queue.items[0].cpuMillis=1000;},s=>{s.session.status='running';},s=>{s.run.operations[0].status='running';},s=>{s.successor.state='running';},s=>{s.successor.childId='s-successor';},s=>{s.successor.memoryMiB=1024;}])rejectsChange(released(before),change,s=>assertReleased(before,s,target));
});
test('release refuses output loss, context/history mutation, replay and unnecessary fence creation',()=>{
 const before=held();for(const change of [s=>{s.retained.output='';},s=>{s.retained.checkout['result.txt']='replaced';},s=>{s.retained.context.digest='foreign';},s=>{s.retained.history=[];},s=>{s.dispatches[target.childId]=2;},s=>{s.createsForTarget=2;},s=>{s.startsForTarget=2;},s=>{s.fenceCreates=1;}])rejectsChange(released(before),change,s=>assertReleased(before,s,target));
});

function controlledPort(overrides={}){
 let state=held(),grant={role:'editor',execute:true},phase='held';const calls=[],records=[];
 const call=(name,value)=>calls.push({name,...(value===undefined?{}:{value:copy(value)})});
 const port={record:row=>records.push(copy(row)),
  async prepareFresh(){call('prepareFresh');},async pause(value){call('pause',value);},async queueBoth(){call('queueBoth');},
  async waitTargetActive(){call('waitTargetActive');return copy(target);},async waitRecordedOutput(){call('waitRecordedOutput');assert(state.retained.output);},
  async snapshot(){call('snapshot',phase);return copy(state);},publicIdentity:value=>({projectId:value.projectId,childId:value.childId}),
  async restartControllers(){call('restartControllers');},async preview(){call('preview');return {expectedRevision:state.run.revision,operationId:target.operationId,childId:target.childId,availability:{supported:false,reasonCode:'recovery_launch_unconfirmed'}};},
  async setMemberGrant(value){grant=copy(value);call('grant',value);},async readAsMember(){call('readAsMember');return {status:200,body:copy(state.session)};},
  async stopAsMember(){call('stopAsMember',grant);if(grant.role!=='editor'||!grant.execute)return {status:403,body:{error:'team_permission_denied'}};state=released(state);phase='released';return {status:200,body:copy(state.session)};},
  async foreignProjectProbe(){call('foreignProjectProbe');return {status:403,body:{error:'team_access_denied'}};},
  async abandon(_selected,identity){call('abandon',identity);assert(identity.expectedRevision!==state.run.revision||identity.operationId!==target.operationId||identity.childId!==target.childId);return {status:409,body:{error:'project_run_conflict'}};},
  async stopWithUnavailableInspect(){call('unavailableInspect');return {status:503,body:{error:'backend_unavailable'}};},
  async waitReleased(){call('waitReleased');assert.equal(phase,'released');return copy(state);},
  async completeSuccessor(){call('completeSuccessor');assert.equal(phase,'released');phase='successor-completed';return {projectId:target.successorProjectId,nativeExitCode:0,commandDispatches:1,queueState:'completed',leaseAbsent:true};},
 };
 Object.assign(port,overrides);return {port,calls,records,getState:()=>copy(state),setState:value=>{state=copy(value);}};
}
test('controlled ports exercise restart, authority denials, reviewed conflicts and release before successor consent',async()=>{
 const c=controlledPort();const result=await runWorkerRecoveryScenario(c.port);assert.equal(result.passed,true);
 assert.deepEqual(c.records.map(r=>r.phase),['acknowledged-worker-held','controller-restart-preserved','settled-fence-ineligible','current-edit-refused','current-execute-refused','foreign-project-refused','revision-and-identity-refused','unknown-backend-refused','ordinary-stop-settled','independent-project-progress']);
 assert.deepEqual(c.calls.filter(r=>r.name==='pause').map(r=>r.value),[true,false,true,false]);
 assert.deepEqual(c.calls.filter(r=>r.name==='grant').map(r=>r.value),[{role:'viewer',execute:true},{role:'editor',execute:false},{role:'editor',execute:true}]);
 assert.equal(c.calls.filter(r=>r.name==='stopAsMember').length,3);assert.equal(c.calls.filter(r=>r.name==='abandon').length,3);
 const releasedIndex=c.calls.findIndex(r=>r.name==='waitReleased');assert(releasedIndex>=0);assert(c.calls.findIndex(r=>r.name==='completeSuccessor')>releasedIndex);
 assert.deepEqual(c.getState().retained,held().retained);
});
test('controlled scenario rejects missing Edit/Execute authority accepted by the adapter',async()=>{
 for(const permission of ['edit','execute']){
  const c=controlledPort();let grant;
  c.port.setMemberGrant=async value=>{grant=value;};c.port.stopAsMember=async()=>grant?.role==='viewer'&&permission==='edit'||grant?.execute===false&&permission==='execute'?{status:200,body:{id:target.childId,status:'exited'}}:{status:403,body:{error:'team_permission_denied'}};
  await assert.rejects(()=>runWorkerRecoveryScenario(c.port),assert.AssertionError);assert(!c.records.some(r=>r.phase==='ordinary-stop-settled'));
 }
});
test('controlled scenario rejects wrong authority error codes and foreign-project acceptance',async()=>{
 for(const overrides of [{stopAsMember:async()=>({status:401,body:{error:'sign_in_required'}})},{stopAsMember:async()=>({status:403,body:{error:'wrong_error'}})},{foreignProjectProbe:async()=>({status:200,body:{id:target.childId}})}]){
  const c=controlledPort(overrides);await assert.rejects(()=>runWorkerRecoveryScenario(c.port),assert.AssertionError);assert(!c.records.some(r=>r.phase==='ordinary-stop-settled'));
 }
});
test('controlled scenario rejects silently replayed restart, unsupported fence claims and unknown backend success',async()=>{
 for(const mode of ['restart-replay','fence-supported','unknown-success']){
  const c=controlledPort();if(mode==='restart-replay')c.port.restartControllers=async()=>{const s=c.getState();s.dispatches[target.childId]=2;c.setState(s);};
  if(mode==='fence-supported')c.port.preview=async()=>({operationId:target.operationId,childId:target.childId,availability:{supported:true,reasonCode:'recovery_supported'}});
  if(mode==='unknown-success')c.port.stopWithUnavailableInspect=async()=>({status:200,body:{id:target.childId,status:'exited'}});
  await assert.rejects(()=>runWorkerRecoveryScenario(c.port),assert.AssertionError);assert(!c.records.some(r=>r.phase==='ordinary-stop-settled'));
 }
});
test('controlled scenario rejects output loss after independent project completion',async()=>{
 const c=controlledPort(),finish=c.port.completeSuccessor;c.port.completeSuccessor=async()=>{const value=await finish();const s=c.getState();s.retained.output='';c.setState(s);return value;};
 await assert.rejects(()=>runWorkerRecoveryScenario(c.port),assert.AssertionError);assert(!c.records.some(r=>r.phase==='independent-project-progress'));
});
