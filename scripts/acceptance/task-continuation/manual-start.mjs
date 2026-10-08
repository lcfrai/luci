// Authoritative production reconciliation only. A manual HTTP 201 does not itself
// change the durable run operation from dispatching to observed running.
import assert from 'node:assert/strict';
function exactOperation(run,session,selection){
 assert.equal(run.projectId,selection.projectId);assert.equal(run.id,selection.runId);
 assert.equal(session.projectId,selection.projectId);assert.equal(session.projectRunId,selection.runId);
 const matches=run.operations.filter(op=>op.taskId===selection.taskId&&op.attempt===selection.attempt);assert.equal(matches.length,1);
 const op=matches[0];assert.equal(op.id,session.projectRunOperationId);assert.equal(op.childId,session.id);assert.equal(op.worktreeId,session.worktreeId);assert.equal(op.allocation,'ready');assert.equal(run.swarm.tasks.find(task=>task.id===selection.taskId)?.operationId,op.id);assert.equal(run.operations.filter(other=>other.childId===session.id).length,1);
 assert.deepEqual(session.implementationContext.taskIds,[selection.taskId]);assert.deepEqual(op.implementation.taskIds,[selection.taskId]);assert.equal(op.implementation.projectId,selection.projectId);assert.equal(op.implementation.worktreeId,session.worktreeId);return op;
}
export async function reconcileManualStart({session,selection,readRun,reconcileRun}){
 const before=await readRun(),original=structuredClone(exactOperation(before,session,selection));
 assert(['dispatching','running'].includes(original.status),'Acknowledged manual start still needs a valid current operation');
 // The callback supplies the actual automatic post-start reconciliation
 // response. A GET, local status rewrite, or dispatching flag is not proof.
 const run=await reconcileRun();assert(run.revision>=before.revision);const operation=exactOperation(run,session,selection);
 assert.equal(operation.status,'running','Explicit production observation must confirm the exact task as running');
 for(const field of ['id','childId','taskId','attempt','worktreeId','baseCommit','requestDigest','implementation','checkpoint'])assert.deepEqual(operation[field],original[field],'Reconciliation substituted '+field);
 return {run,operation,beforeStatus:original.status,beforeRevision:before.revision};
}
