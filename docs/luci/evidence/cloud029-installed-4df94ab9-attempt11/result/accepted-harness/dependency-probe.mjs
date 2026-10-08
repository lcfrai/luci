// Bounded diagnostic evidence. A recorded response is not a passed dependency gate.
import assert from 'node:assert/strict';

export function recordDependencyProbe(receipt, {response, ...observations}) {
  const fields = {}, truncatedFields = [];
  for (const [name, limit] of [['code',128],['message',2048],['error',2048]]) {
    if (typeof response?.[name] !== 'string') continue;
    fields[name] = response[name].slice(0,limit);
    if (response[name].length > limit) truncatedFields.push(name);
  }
  const probe = structuredClone({...observations, response:fields, truncatedFields,
    at:new Date().toISOString(), classification:'unclassified-response'});
  (receipt.dependencyProbeResponses ??= []).push(probe);
  return probe;
}

export function assertDependencyRefusal(probe) {
  assert.equal(probe.status,409,'Dependency reservation must be refused');
  assert.deepEqual(probe.truncatedFields,[],'Truncated errors cannot establish the dependency refusal');
  assert.equal(probe.response.error,'project_run_conflict');
  assert.equal(probe.response.message,"Integrate this task's dependencies before starting it.",
    'A source lock, stale revision or other denial is not a dependency refusal; see the retained probe');
}
