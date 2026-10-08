// Preparation only: no top-level process, Docker, browser, timer, or network work.
// The ports must be backed by the fresh installed adapter; controlled tests are
// helper checks and never evidence that product Stop or settlement succeeded.
import assert from 'node:assert/strict';

export const pins=Object.freeze({product:'4df94ab9dad0e9fedd28a873bbadc56720e1402e',image:'sha256:333effcf40151e2411fa036b4581113be2af7f8d441bf9566f1cce6e7f208e98'});
export const bounds=Object.freeze({scenarioMs:600000,cleanupMs:180000,requestMs:120000,workerCreates:2,starts:2,fenceCreates:0,workerMemoryMiB:1024,workerCPUMillis:1000,minimumCreateFreeBytes:6*1024**3,runningReserveBytes:1024**3});

export function routes(target){
 for(const key of ['teamId','projectId','runId','operationId','childId','taskId','queueId'])assert.match(target[key]??'',/^[A-Za-z0-9_-]{1,200}$/);
 const workspace='/v1/teams/'+target.teamId+'/workspace',run=workspace+'/v1/product/projects/'+target.projectId+'/runs/'+target.runId;
 return {workspace,run,stop:workspace+'/v1/sessions/'+target.childId,queue:workspace+'/v1/product/projects/'+target.projectId+'/swarm-queue',preview:run+'/swarm/operations/'+target.operationId+'/recovery',reconcile:run+'/swarm/reconcile',abandon:run+'/swarm/tasks/'+target.taskId+'/abandon'};
}
export function exactLease(ledger,target){
 assert.equal(ledger.version,1);assert(Array.isArray(ledger.leases)&&ledger.leases.length<=4096);
 const matches=ledger.leases.filter(row=>row.id===target.queueId||row.sessionId===target.childId);
 assert.equal(matches.length,1,'Exact queue/child must resolve to one lease');const lease=matches[0];
 for(const [key,want] of Object.entries({id:target.queueId,scope:target.scope,projectId:target.projectId,sessionId:target.childId,sandboxId:target.childId,state:'backend-settled',launchGuard:1,fenceProtocol:1,fenceEngineId:target.engineId,fenceInstanceId:target.instanceId}))assert.equal(lease[key],want,key);
 assert(target.actor&&target.actor.userId&&target.actor.teamId===target.teamId&&target.actor.projectId===target.projectId,'Trusted fixture actor binding required');
 assert.deepEqual(lease.actor,target.actor);assert.deepEqual(lease.resources,{CPUMillis:1000,MemoryMiB:1024});return lease;
}
export function exactOperation(snapshot,target){
 assert.equal(snapshot.run.id,target.runId);assert.equal(snapshot.run.projectId,target.projectId);
 const rows=snapshot.run.operations.filter(row=>row.id===target.operationId||row.childId===target.childId);
 assert.equal(rows.length,1);const op=rows[0];
 for(const [key,want] of Object.entries({id:target.operationId,childId:target.childId,taskId:target.taskId,allocation:'ready'}))assert.equal(op[key],want);
 assert.equal(snapshot.session.id,target.childId);assert.equal(snapshot.session.projectId,target.projectId);
 const queued=snapshot.queue.items.filter(row=>row.id===target.queueId);assert.equal(queued.length,1);
 for(const [key,want] of Object.entries({childId:target.childId,operationId:target.operationId,taskId:target.taskId}))assert.equal(queued[0][key],want);
 return {op,queue:queued[0]};
}
const ledgerWithoutTarget=(ledger,target)=>({...ledger,leases:ledger.leases.filter(row=>row.id!==target.queueId)});
export function assertHeld(before,after,target){
 assert.deepEqual(exactLease(after.ledger,target),exactLease(before.ledger,target));
 assert.deepEqual(after.ledger,before.ledger,'Refusal cannot mutate any execution claim');
 assert.deepEqual(after.mutations,before.mutations,'Refusal cannot create, start, stop, or remove Docker work');
 assert.deepEqual(after.retained,before.retained,'Refusal cannot alter retained output, checkout, or context');
 assert.equal(after.dispatches[target.childId],1,'The first native command cannot replay');
 const q=exactOperation(after,target).queue;assert.equal(q.memoryMiB,1024);assert.equal(q.cpuMillis,1000);
}
export function assertReleased(before,after,target){
 const original=exactLease(before.ledger,target);const {op,queue}=exactOperation(after,target);
 assert.equal(after.session.status,'exited');assert.equal(after.exactBackendAbsence?.confirmed,true);assert.equal(after.exactBackendAbsence.childId,target.childId);assert.equal(after.exactBackendAbsence.immutableId,target.containerId);
 assert(!after.ledger.leases.some(row=>row.id===original.id||row.sessionId===target.childId));
 assert.deepEqual(ledgerWithoutTarget(after.ledger,target),ledgerWithoutTarget(before.ledger,target),'Stop can remove only the exact target charge');
 assert.equal(queue.memoryMiB??0,0);assert.equal(queue.cpuMillis??0,0);assert(['failed','cancelled','completed','unknown'].includes(queue.state));
 assert.notEqual(op.status,'running');assert.notEqual(op.status,'dispatching');assert.notEqual(op.status,'reserved');
 assert.deepEqual(after.retained,before.retained,'Recorded command output, task files, history, and context remain retained');
 assert.equal(after.dispatches[target.childId],1);assert.equal(after.createsForTarget,1);assert.equal(after.startsForTarget,1);assert.equal(after.fenceCreates,0);
 assert.equal(after.successor.childId??'',before.successor.childId??'');assert.equal(after.successor.state,'queued');assert.equal(after.successor.memoryMiB??0,0);assert.equal(after.successor.cpuMillis??0,0);
}
export function reviewedIdentity(snapshot,target){
 exactOperation(snapshot,target);assert(Number.isSafeInteger(snapshot.run.revision)&&snapshot.run.revision>0);
 return {expectedRevision:snapshot.run.revision,operationId:target.operationId,childId:target.childId};
}

export async function runWorkerRecoveryScenario(port){
 const record=port.record;await port.prepareFresh();await port.pause(true);await port.queueBoth();
 await port.pause(false);const target=await port.waitTargetActive();routes(target);
 await port.waitRecordedOutput(target);await port.pause(true);
 const before=await port.snapshot(target);exactLease(before.ledger,target);exactOperation(before,target);
 assert.equal(before.session.status,'running');assert.equal(before.successor.state,'queued');assert.equal(before.successor.childId??'','');assert.equal(before.dispatches[target.childId],1);
 record({phase:'acknowledged-worker-held',target:port.publicIdentity(target),leaseState:'backend-settled',memoryMiB:1024,cpuMillis:1000});

 // Ordinary restart is sufficient for this settled allocation; it must preserve
 // the real container and its process while the scripted final response waits.
 await port.restartControllers();const restarted=await port.snapshot(target);assertHeld(before,restarted,target);
 assert.equal(restarted.containerId,before.containerId);assert.equal(restarted.containerPID,before.containerPID);
 record({phase:'controller-restart-preserved',sameContainer:true,samePID:true,commandDispatches:1});

 const preview=await port.preview(target);assert.equal(preview.operationId,target.operationId);assert.equal(preview.childId,target.childId);assert.equal(preview.availability.supported,false);
 assert.equal(preview.availability.reasonCode,'recovery_launch_unconfirmed');
 record({phase:'settled-fence-ineligible',availability:preview.availability});

 // Refresh/revoke the same member's project grant via the owner account. Never
 // use the owner/admin token as the denied actor: it bypasses project narrowing.
 for(const permission of ['edit','execute']){
  await port.setMemberGrant(permission==='edit'?{role:'viewer',execute:true}:{role:'editor',execute:false});
  const readable=await port.readAsMember(target);assert.equal(readable.status,200);assert.equal(readable.body.id,target.childId);
  const denied=await port.stopAsMember(target);assert.equal(denied.status,403);assert.equal(denied.body.error,'team_permission_denied');assertHeld(before,await port.snapshot(target),target);
  record({phase:'current-'+permission+'-refused',status:403,error:denied.body.error});
 }
 await port.setMemberGrant({role:'editor',execute:true});
 const foreign=await port.foreignProjectProbe(target);assert.equal(foreign.status,403);assert.equal(foreign.body.error,'team_access_denied');assertHeld(before,await port.snapshot(target),target);
 record({phase:'foreign-project-refused',status:403,error:foreign.body.error});

 const current=await port.snapshot(target),identity=reviewedIdentity(current,target);
 for(const altered of [{...identity,expectedRevision:identity.expectedRevision-1},{...identity,childId:'s-00000000'},{...identity,operationId:'op-foreign-fixture'}]){
  const denied=await port.abandon(target,altered);assert.equal(denied.status,409);assertHeld(before,await port.snapshot(target),target);
 }
 record({phase:'revision-and-identity-refused',requests:3,status:409});

 // The adapter loses real exact Inspect replies through a bounded gate window.
 // Transport failure is not daemon absence and cannot release a lease.
 const unknown=await port.stopWithUnavailableInspect(target);assert.equal(unknown.status,503);assertHeld(before,await port.snapshot(target),target);
 record({phase:'unknown-backend-refused',status:503,exactLeaseRetained:true});

 const stopped=await port.stopAsMember(target);assert.equal(stopped.status,200);assert.equal(stopped.body.id,target.childId);assert.equal(stopped.body.status,'exited');
 const released=await port.waitReleased(target);assertReleased(before,released,target);
 record({phase:'ordinary-stop-settled',exitReason:released.session.exitReason,operationStatus:exactOperation(released,target).op.status,queueState:exactOperation(released,target).queue.state,capacityReleased:true,fenceCreated:false});
 // Unknown outcomes remain unknown: no forced abandon or retry is needed to
 // establish resource release and independent queued project progress.
 await port.pause(false);const successor=await port.completeSuccessor();
 assert.equal(successor.projectId,target.successorProjectId);assert.equal(successor.nativeExitCode,0);assert.equal(successor.commandDispatches,1);assert.equal(successor.queueState,'completed');assert.equal(successor.leaseAbsent,true);
 const final=await port.snapshot(target);assert.equal(final.createsForTarget,1);assert.equal(final.startsForTarget,1);assert.equal(final.dispatches[target.childId],1);assert.deepEqual(final.retained,before.retained);assert.equal(final.fenceCreates,0);
 record({phase:'independent-project-progress',...successor});
 return {passed:true,scope:'Installed ordinary Stop of acknowledged worker after controller restart, with separate lease/queue release and retained output. Scripted provider; no model-quality or original-fixture diagnosis.',target:port.publicIdentity(target)};
}
