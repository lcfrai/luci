// Diagnostics for one stale-review POST. Recording a response never passes a gate.
import assert from 'node:assert/strict';

const fieldLimits = {code:128, message:2048, error:2048};
function boundedFields(value, scrub) {
  const fields = {}, truncatedFields = [];
  for (const [key, limit] of Object.entries(fieldLimits)) {
    if (typeof value?.[key] !== 'string') continue;
    const text = scrub(value[key]);
    fields[key] = text.slice(0, limit);
    if (text.length > limit) truncatedFields.push(key);
  }
  return {fields, truncatedFields};
}
function failure(error, scrub) {
  const {fields, truncatedFields} = boundedFields({code:error?.code, message:error?.message ?? String(error)}, scrub);
  return {...fields, truncatedFields};
}
const readers = {
  effects: value => {
    const fields = {};
    for (const name of ['create', 'start', 'destroy']) {
      assert(Number.isSafeInteger(value?.[name]) && value[name] >= 0, 'Invalid effect count');
      fields[name] = value[name];
    }
    return fields;
  },
  primaryCommit: value => { assert.match(value, /^[a-f0-9]{40,64}$/); return value; },
  primaryStatusSHA256: value => { assert.match(value, /^[a-f0-9]{64}$/); return value; },
  privateRevision: value => { assert(Number.isSafeInteger(value) && value >= 0); return value; },
  integrationCheckoutExists: value => { assert.equal(typeof value, 'boolean'); return value; },
  publicRevision: value => { assert(Number.isSafeInteger(value) && value >= 0); return value; },
};
async function snapshot(read, phase, scrub) {
  const result = {startedAt:new Date().toISOString(), errors:{}};
  // Each production reader is bounded (local Git/file reads or a timed GET).
  // Continue independent reads if one fails; never turn missing state into zero.
  for (const [key, validate] of Object.entries(readers)) {
    try { result[key] = validate(await read[key](phase)); }
    catch (error) { result.errors[key] = failure(error, scrub); }
  }
  result.finishedAt = new Date().toISOString();
  return result;
}

export async function probeStaleReview({receipt, requestedRevision, request, read, responseStatus, persist, scrub = String}) {
  const probe = {classification:'unclassified-response', requestedRevision, status:null,
    response:{}, truncatedFields:[], responseObserved:false, requestFailure:null, before:null, after:null};
  receipt.staleReviewDiagnostic = probe;
  let first = true;
  const save = phase => { probe.phase = phase; persist(probe, first); first = false; };
  probe.before = await snapshot(read, 'before', scrub);
  save('before-request');
  assert.deepEqual(probe.before.errors, {}, 'Stale-review preconditions are unavailable; no request sent');
  assert.equal(probe.before.integrationCheckoutExists, false);
  assert.equal(probe.before.publicRevision, requestedRevision);
  let requestError;
  try {
    // Exactly one mutation. The API observer runs before its status assertion.
    await request((response, status) => {
      const bounded = boundedFields(response, scrub);
      probe.response = bounded.fields;
      probe.truncatedFields = bounded.truncatedFields;
      probe.status = status;
      probe.responseObserved = true;
      save('response-received');
    });
  } catch (error) {
    requestError = error;
    probe.requestFailure = failure(error, scrub);
    // A malformed/oversized body may fail before the API observer. Preserve the
    // status already recorded by api(), without inventing a returned reason.
    const status = responseStatus();
    if (probe.status === null && Number.isInteger(status)) probe.status = status;
    save('request-failed');
  }
  probe.after = await snapshot(read, 'after', scrub);
  save('after-request');
  if (requestError) throw requestError;
  assert.equal(probe.status, 409, 'Stale-review integration must return 409');
  assert.equal(probe.responseObserved, true, 'Missing response cannot establish stale-review refusal');
  assert.deepEqual(probe.truncatedFields, [], 'Truncated fields cannot establish stale-review refusal');
  assert(/review/i.test(probe.response.message ?? probe.response.error ?? ''),
    'Expected a review refusal; see the retained stale-review diagnostic');
  assert.deepEqual(probe.after.errors, {}, 'Stale-review after-state is unavailable');
  assert.deepEqual(probe.after.effects, probe.before.effects);
  assert.equal(probe.after.primaryCommit, probe.before.primaryCommit);
  assert.equal(probe.after.primaryStatusSHA256, probe.before.primaryStatusSHA256);
  assert.equal(probe.after.integrationCheckoutExists, false);
  assert.equal(probe.after.publicRevision, requestedRevision);
  return probe;
}
