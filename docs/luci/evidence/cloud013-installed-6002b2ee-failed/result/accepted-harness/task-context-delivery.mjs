// Public session delivery and the private immutable reference are different
// contracts. Both must agree with the actual captured manifest and task source.
import assert from 'node:assert/strict';
import {createHash} from 'node:crypto';

export function verifyTaskContextDelivery({session,stored,run,operation,manifestBytes,version}) {
 assert(version===1||version===2);assert(Buffer.isBuffer(manifestBytes));
 const digest=createHash('sha256').update(manifestBytes).digest('hex'),manifest=JSON.parse(manifestBytes.toString('utf8'));
 assert.deepEqual(session.reviewedContextDelivery,{status:'captured',version,digest},'Public delivery must explicitly identify the exact captured manifest');
 assert.deepEqual(stored.reviewedContext,{version,digest},'Private captured reference must match the public delivery and actual bytes');
 assert.equal(manifest.version,version);
 for(const value of [session,stored]){
  assert.equal(value.id,operation.childId);assert.equal(value.projectId,run.projectId);assert.equal(value.projectRunId,run.id);assert.equal(value.projectRunOperationId,operation.id);assert.equal(value.worktreeId,operation.worktreeId);
  assert.deepEqual(value.implementationContext,operation.implementation,'Session must retain the exact approved task/source binding');
 }
 assert.deepEqual(operation.implementation.taskIds,[operation.taskId]);assert.equal(operation.implementation.projectId,run.projectId);assert.equal(operation.implementation.worktreeId,operation.worktreeId);
 assert.equal(run.swarm.tasks.find(task=>task.id===operation.taskId)?.operationId,operation.id);
 assert.deepEqual(manifest.binding,{...operation.implementation,sourceCommit:'',sourceDigest:'',sourceDirty:false},'Frozen context binding must match approved inputs independently of the source snapshot');
 return {delivery:session.reviewedContextDelivery,reference:stored.reviewedContext,manifestDigest:digest};
}
