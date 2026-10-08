import assert from 'node:assert/strict';
import {test} from 'node:test';
import {recordDependencyProbe,assertDependencyRefusal} from './dependency-probe.mjs';

function observation(message="Integrate this task's dependencies before starting it.") {
 return {label:'both-processes-exited',status:409,response:{error:'project_run_conflict',message},
  requestedRevision:12,publicRevisionBefore:12,privateRevisionBefore:12,privateRevisionAfter:14,
  before:{effects:{create:2,start:2,destroy:2},primaryCommit:'a'.repeat(40),joinOperationIds:[]},
  after:{effects:{create:2,start:2,destroy:2},primaryCommit:'a'.repeat(40),joinOperationIds:[]}};
}
test('records exact dependency denial and revisions separately from passed gates',()=>{
 const receipt={},input=observation(),probe=recordDependencyProbe(receipt,input);
 assertDependencyRefusal(probe);assert.equal(receipt.dependencyGates,undefined);
 assert.equal(probe.classification,'unclassified-response');assert.equal(probe.requestedRevision,12);
 assert.equal(probe.privateRevisionAfter,14);assert.deepEqual(probe.before,probe.after);
 input.before.effects.create=99;assert.equal(probe.before.effects.create,2);
});
for(const message of ['project run changed; reload before continuing','Project source is busy. Wait before reserving this task.','This task has an integration receipt or an unresolved integration attempt. Inspect it before continuing.']) {
 test('retains denial before rejecting unrelated reason: '+message,()=>{
  const receipt={},probe=recordDependencyProbe(receipt,observation(message));
  assert.throws(()=>assertDependencyRefusal(probe));
  assert.equal(receipt.dependencyProbeResponses.length,1);assert.equal(probe.response.message,message);
  assert.equal(receipt.dependencyGates,undefined);
 });
}
test('preserves status and bounded known fields without adopting a large or unknown body',()=>{
 const receipt={},input=observation();input.status=500;
 input.response={code:'x'.repeat(129),message:'y'.repeat(2049),error:'z'.repeat(2049),privateToken:'never-copy'};
 const probe=recordDependencyProbe(receipt,input);
 assert.equal(probe.status,500);assert.equal(probe.response.code.length,128);
 assert.equal(probe.response.message.length,2048);assert.equal(probe.response.error.length,2048);
 assert.equal(probe.response.privateToken,undefined);assert.deepEqual(probe.truncatedFields,['code','message','error']);
 assert.throws(()=>assertDependencyRefusal(probe));
});
