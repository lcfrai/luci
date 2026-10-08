import assert from 'node:assert/strict';
import {test} from 'node:test';
import {selectAuthorizedTask,taskFailureDiagnostic,assertExactTaskFailure} from './queued-failure.mjs';

function fixture(){
 const authorization={id:'queue-new',projectId:'p-project',runId:'run-build',taskId:'task-b'};
 const startFailure={stage:'admission',reasonCode:'memory_pressure',message:'The saved fixed public refusal. No allocation was attempted.'};
 const queueItem={...authorization,revision:4,state:'failed',childId:'s-newchild',operationId:'op-new',reasonCode:'memory_pressure',reason:startFailure.message};
 const operation={id:'op-new',taskId:'task-b',childId:'s-newchild',worktreeId:'w-new',attempt:3,status:'failed',allocation:'ready',startFailure};
 return {variant:'b2',expectedAttempt:3,authorization,queueItem,session:{id:'s-newchild',projectId:'p-project',projectRunId:'run-build',projectRunOperationId:'op-new',worktreeId:'w-new',status:'exited',startFailure},run:{id:'run-build',projectId:'p-project',revision:20,state:'blocked',operations:[operation],swarm:{tasks:[{id:'task-b',operationId:'op-new'}]}}};
}
test('old failed B1 cannot fail a freshly authorized B2 or hide its running child',()=>{
 const f=fixture(),old={...f.queueItem,id:'queue-old',childId:'s-old',operationId:'op-old'};
 assert.equal(selectAuthorizedTask([old,{...f.queueItem,state:'queued',childId:undefined,operationId:undefined}],f.authorization,['s-old']),undefined);
 assert.equal(selectAuthorizedTask([old,f.queueItem],f.authorization,['s-old']),f.queueItem);
 const running={...f.queueItem,state:'running'};assert.equal(selectAuthorizedTask([old,running],f.authorization,['s-old']),running);
 assert.equal(selectAuthorizedTask([old,running],f.authorization,['s-old','s-newchild']),undefined);
});
test('waiting, unknown, historical completed and foreign queue items never count as terminal failed',()=>{
 const f=fixture();for(const state of ['queued','reserved','paused','blocked','unknown','completed','cancelled'])assert.equal(selectAuthorizedTask([{...f.queueItem,state}],f.authorization,[]),undefined);
 assert.equal(selectAuthorizedTask([{...f.queueItem,id:'foreign'}],f.authorization,[]),undefined);
 assert.throws(()=>selectAuthorizedTask([{...f.queueItem,runId:'other-run'}],f.authorization,[]));
});
test('binds public terminal failure to exact current run/task/operation/child and preserves typed cause',()=>{
 const f=fixture(),d=taskFailureDiagnostic(f);assertExactTaskFailure(d);
 assert.equal(d.session.startFailure.reasonCode,'memory_pressure');assert.equal(d.operation.attempt,3);
 assert.equal(d.classification,'unverified-terminal-queue-response');
});
test('initial bounded queue-only diagnostic survives a later failed detail lookup',()=>{
 const f=fixture(),d=taskFailureDiagnostic({...f,run:null,session:null});
 assert.equal(d.queue.reasonCode,'memory_pressure');assert.equal(d.session,null);assert.equal(d.operation,null);
 assert.throws(()=>assertExactTaskFailure(d));
});
test('refuses mismatched current operation, attempt, session, worktree or typed cause',()=>{
 for(const mutate of [f=>f.run.swarm.tasks[0].operationId='op-old',f=>f.run.operations[0].attempt=2,f=>f.session.id='s-old',f=>f.session.worktreeId='w-other',f=>f.session.projectRunId='run-other',f=>f.queueItem.reasonCode='cpu_pressure',f=>f.session.startFailure={...f.session.startFailure,reasonCode:'cpu_pressure'}]){
  const f=fixture();mutate(f);assert.throws(()=>assertExactTaskFailure(taskFailureDiagnostic(f)));
 }
});
test('acknowledged abnormal exit stays distinct from pre-allocation or unknown launch failure',()=>{
 const f=fixture();delete f.session.startFailure;delete f.run.operations[0].startFailure;f.queueItem.reasonCode='worker_failed';assertExactTaskFailure(taskFailureDiagnostic(f));
 f.queueItem.state='unknown';assert.throws(()=>assertExactTaskFailure(taskFailureDiagnostic(f)));
});
test('only bounded public diagnostic fields are retained; truncated evidence cannot pass binding',()=>{
 const f=fixture();f.session.attachToken='never-copy';f.session.env={secret:'never-copy'};f.queueItem.reason='x'.repeat(2049);
 const d=taskFailureDiagnostic(f);assert.equal(d.queue.reason.length,2048);assert.deepEqual(d.truncatedFields,['queue.reason']);assert.equal(d.session.attachToken,undefined);assert.equal(d.session.env,undefined);assert.throws(()=>assertExactTaskFailure(d));
});
