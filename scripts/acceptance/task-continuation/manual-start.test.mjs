import assert from 'node:assert/strict';
import {test} from 'node:test';
import {reconcileManualStart} from './manual-start.mjs';
function fixture(){
 const selection={projectId:'project',runId:'run',taskId:'task-work',attempt:1},implementation={projectId:'project',worktreeId:'work',taskIds:['task-work']};
 const session={id:'s-12345678',projectId:'project',projectRunId:'run',projectRunOperationId:'op',worktreeId:'work',implementationContext:implementation};
 const run={id:'run',projectId:'project',revision:5,operations:[{id:'op',childId:session.id,taskId:'task-work',attempt:1,worktreeId:'work',baseCommit:'a'.repeat(40),requestDigest:'b'.repeat(64),implementation,allocation:'ready',status:'dispatching'}],swarm:{tasks:[{id:'task-work',operationId:'op'}]}};
 return {selection,session,run};
}
test('manual dispatching advances only through actual post-start reconciliation before native inspection',async()=>{
 const f=fixture(),calls=[];const observed=structuredClone(f.run);observed.revision++;observed.operations[0].status='running';
 const value=await reconcileManualStart({...f,readRun:async()=>{calls.push('read');return f.run;},reconcileRun:async()=>{calls.push('production-reconcile');return observed;}});
 calls.push('native-inspection-can-begin');assert.deepEqual(calls,['read','production-reconcile','native-inspection-can-begin']);assert.equal(value.beforeStatus,'dispatching');assert.equal(value.operation.status,'running');assert.equal(f.run.operations[0].status,'dispatching','Helper must never rewrite the original read');
});
test('read-only polling or unknown/terminal observation cannot stand in for reconciliation',async()=>{
 for(const status of ['dispatching','unknown','reserved','failed','review-required']){const f=fixture();const observed=structuredClone(f.run);observed.operations[0].status=status;await assert.rejects(reconcileManualStart({...f,readRun:async()=>f.run,reconcileRun:async()=>observed}));}
});
test('wrong acknowledged scope, child or attempt fails before any reconciliation request',async()=>{
 for(const mutate of [f=>{f.session.projectId='other'},f=>{f.session.projectRunId='other'},f=>{f.session.id='s-ffffffff'},f=>{f.session.worktreeId='other'},f=>{f.session.projectRunOperationId='other'},f=>{f.selection.attempt=2},f=>{f.run.operations.push({...f.run.operations[0]})}]){const f=fixture();mutate(f);let calls=0;await assert.rejects(reconcileManualStart({...f,readRun:async()=>f.run,reconcileRun:async()=>{calls++;return f.run;}}));assert.equal(calls,0);}
});
test('reconciliation cannot substitute operation, source binding, current task or revision',async()=>{
 for(const mutate of [r=>{r.operations[0].requestDigest='c'.repeat(64)},r=>{r.operations[0].baseCommit='d'.repeat(40)},r=>{r.operations[0].childId='s-ffffffff'},r=>{r.swarm.tasks[0].operationId='other'},r=>{r.revision--},r=>{r.operations[0].implementation.worktreeId='other'}]){const f=fixture(),observed=structuredClone(f.run);observed.operations[0].status='running';mutate(observed);await assert.rejects(reconcileManualStart({...f,readRun:async()=>f.run,reconcileRun:async()=>observed}));}
});
test('production refusal remains a refusal; no local fallback accepts dispatching',async()=>{const f=fixture();await assert.rejects(reconcileManualStart({...f,readRun:async()=>f.run,reconcileRun:async()=>{throw Error('fresh permission refused');}}),/fresh permission refused/);});

test('already-observed read still requires the actual automatic response, not a manual fallback',async()=>{const f=fixture();f.run.operations[0].status='running';let calls=0;const confirmed=await reconcileManualStart({...f,readRun:async()=>f.run,reconcileRun:async()=>{calls++;return structuredClone(f.run);}});assert.equal(calls,1);assert.equal(confirmed.operation.status,'running');});
