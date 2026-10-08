// PREPARATION ONLY. No top-level work, daemon client, service, browser or timer.
// The installed bounded adapter described in adapter-contract.md is required.
import assert from 'node:assert/strict';
import {createHash} from 'node:crypto';

export const pins = Object.freeze({
  product: '4df94ab9dad0e9fedd28a873bbadc56720e1402e',
  helper: '5d306b6dae3cc9d0a4d9cbf95f8142c37958964f',
  image: 'sha256:333effcf40151e2411fa036b4581113be2af7f8d441bf9566f1cce6e7f208e98',
});
export const bounds = Object.freeze({
  scenarioMs: 600000, cleanupMs: 180000, requestMs: 120000,
  creates: 4, checkCreates: 3, fenceCreates: 1, starts: 2,
  activeCheckMemoryMiB: 2048, activeCheckCPUMillis: 2000,
  retainedFenceMemoryBytes: 6 * 1024 ** 2, retainedFenceNanoCPUs: 100000000,
});
export const command = Object.freeze({label: 'node', executable: 'node', args: ['verify.mjs', 'final']});
const digest = value => createHash('sha256').update(JSON.stringify(value)).digest('hex');
const creates = rows => rows.filter(r => r.method === 'POST' && /\/containers\/create$/.test(r.path));
const starts = rows => rows.filter(r => r.method === 'POST' && /\/start$/.test(r.path));
const active = ledger => (ledger.checks ?? []).filter(r => r.state !== 'finished');
const mutationIntents = proxy => proxy.observations.requests.filter(r => !['GET', 'HEAD'].includes(r.method))
  .map(r => ({method: r.method, path: r.path, requestSequence: r.requestSequence}));

export function exactLease(ledger, row) {
  assert.equal(ledger.version, 1);
  const matches = active(ledger).filter(r => r.identity.checkId === row.id && r.identity.projectId === row.projectId);
  assert.equal(matches.length, 1, 'One product-created active allocation is required');
  const lease = matches[0];
  assert.equal(row.commands.length, 1);
  assert.equal(lease.id, row.commands[0].allocationId);
  assert.equal(lease.identity.allocationId, lease.id);
  assert.equal(lease.identity.commandIndex, 0);
  assert.equal(lease.scope, lease.identity.scopeDigest);
  assert.deepEqual(lease.resources, {CPUMillis: 1000, MemoryMiB: 1024});
  return lease;
}

export function assertUnknown(row, lease) {
  assert.equal(row.status, 'unknown');
  assert.equal(row.reasonCode, 'check_create_unconfirmed');
  assert.equal(row.resourcesHeld, true);
  assert.equal(row.evidenceStatus, 'unverified');
  assert.equal(row.commands[0].cleanupConfirmed, false);
  assert.equal(row.commands[0].status, 'unknown');
  assert.equal(row.commands[0].exitCode ?? null, null);
  assert.equal(row.commands[0].output, '', 'Unstarted command must not claim output');
  assert.equal(lease.state, 'creating');
  assert.equal(lease.ref ?? '', '', 'Daemon ACK must not have become product ACK');
}

export function assertReview(view, row) {
  assert.deepEqual(Object.keys(view).sort(), ['projectId', 'checkId', 'revision', 'state', 'method',
    'commandIndex', 'recoveryDigest', 'reasonCode', 'message'].sort());
  assert.equal(view.projectId, row.projectId); assert.equal(view.checkId, row.id);
  assert.equal(view.revision, row.revision); assert.equal(view.state, 'available');
  assert.equal(view.method, 'fence-launch'); assert.equal(view.commandIndex, 0);
  assert.match(view.recoveryDigest, /^[a-f0-9]{64}$/);
}

export function assertRetired({before, after, targetBefore, targetAfter, witnessBefore, witnessAfter, historyBefore, historyAfter}) {
  const original = exactLease(before, targetBefore), witness = witnessBefore ? exactLease(before, witnessBefore) : null;
  assertUnknown(targetBefore, original);
  if (witness) {
    assert.deepEqual(exactLease(after, witnessAfter), witness, 'Unrelated held allocation must not change');
    assert.deepEqual(witnessAfter, witnessBefore, 'Unforwarded unrelated check must retain its exact receipt');
  }
  assert.deepEqual(historyAfter, historyBefore, 'Recorded output/history must remain byte-for-byte semantically unchanged');
  assert.equal(active(before).length, witness ? 2 : 1); assert.equal(active(after).length, witness ? 1 : 0);
  assert(!active(after).some(r => r.id === original.id));
  assert.deepEqual(after.leases ?? [], before.leases ?? [], 'Agent claims are not recovery targets');
  assert.equal(targetAfter.id, targetBefore.id); assert.equal(targetAfter.commit, targetBefore.commit);
  assert.equal(targetAfter.projectId, targetBefore.projectId);
  assert.equal(targetAfter.sourceDigest, targetBefore.sourceDigest);
  assert.equal(targetAfter.status, 'cancelled'); assert.equal(targetAfter.reasonCode, 'check_stopped_by_review');
  assert.equal(targetAfter.resourcesHeld, false); assert.equal(targetAfter.evidenceStatus, 'unverified');
  assert.equal(targetAfter.commands.length, 1);
  assert.equal(targetAfter.commands[0].status, 'cancelled');
  assert.equal(targetAfter.commands[0].exitCode ?? null, null);
  assert.equal(targetAfter.commands[0].cleanupConfirmed, true);
  for (const key of ['allocationId', 'label', 'executable', 'args', 'output', 'truncated'])
    assert.deepEqual(targetAfter.commands[0][key], targetBefore.commands[0][key]);
  const fences = (after.checkFences ?? []).filter(r => r.identity.allocationId === original.id);
  assert.equal(fences.length, 1); const fence = fences[0];
  assert.equal(fence.state, 'retired'); assert.deepEqual(fence.identity, original.identity);
  assert.equal(fence.scope, original.scope);
  assert.deepEqual(fence.proof.identity, original.identity);
  assert.equal(fence.proof.intentId, fence.intentId);
  assert.match(fence.proof.fenceRef, /^[a-f0-9]{64}$/);
  assert.match(fence.proof.configDigest, /^[a-f0-9]{64}$/);
  const ack = Date.parse(fence.proof.acknowledgedAt), seen = Date.parse(fence.proof.observedAt);
  assert(Number.isFinite(ack) && Number.isFinite(seen) && seen >= ack && ack >= Date.parse(fence.createdAt) && seen <= Date.now());
  assert.deepEqual(fence.actor, original.actor);
  const rest = ledger => ({...ledger,
    checks: (ledger.checks ?? []).filter(r => r.id !== original.id),
    checkFences: (ledger.checkFences ?? []).filter(r => r.identity.allocationId !== original.id)});
  assert.deepEqual(rest(after), rest(before), 'Every unrelated ledger field and record must remain unchanged');
  const remainingTarget = (after.checks ?? []).filter(r => r.id === original.id);
  assert(remainingTarget.length <= 1);
  for (const remaining of remainingTarget) {
    assert.equal(remaining.state, 'finished'); assert.deepEqual(remaining.identity, original.identity);
    assert.deepEqual(remaining.actor, original.actor); assert.deepEqual(remaining.resources, original.resources);
  }
  return {original, witness, fence};
}

/**
 * ctx is a reviewed installed adapter, never a controlled product API fixture.
 * See adapter-contract.md. It owns deadlines, API body bounds, authority,
 * private-state confinement, resource admission and shutdown on all failures.
 */
export async function runInstalledRecoveryScenario(ctx) {
  assert.equal(ctx.windowGranted, true, 'Root must explicitly release the runtime window');
  assert.match(ctx.pins.product,/^[a-f0-9]{40}$/);
  assert.equal(ctx.pins.product,ctx.expectedSource??pins.product,'Actual package must match the explicitly selected product source');
  assert.deepEqual({...ctx.pins,product:pins.product},pins); assert.equal(ctx.freshFixture, true);
  assert.equal(ctx.externalModelCalls, 0); assert.equal(ctx.projects.length, 2);
  const [targetProject, witnessProject] = ctx.projects;
  const historyProject = witnessProject, chargedWitness = ctx.chargedWitness === true;
  assert.notEqual(targetProject.id, witnessProject.id);
  assert.equal(ctx.proxy.observations.counts.create, 0);
  assert.equal(ctx.proxy.observations.counts.start, 0);
  const artifacts = [], record = (phase, details) => {
    const row = {phase, at: new Date().toISOString(), ...details}; artifacts.push(row); ctx.record(row);
  };

  // A real successful historical check gives preservation a nonempty witness.
  // It is a separate check, so every command remains index 0.
  const historyStarted = await ctx.startCheckInBrowser(historyProject, command);
  const history = await ctx.waitCheckTerminal(historyProject, historyStarted.id);
  assert.equal(history.status, 'passed'); assert.equal(history.resourcesHeld, false);
  assert.equal(history.commands[0].cleanupConfirmed, true);
  assert.equal(history.commands[0].output, ctx.expectedOutput);
  assert(ctx.expectedOutput.length > 0);
  const historySnapshot = structuredClone(history);
  record('real-history-created', {checkId: history.id, outputSHA256: digest(history.commands[0].output)});

  // The proxy records/drains an actual daemon 201, then drops the downstream
  // response. It never synthesizes a daemon success or a product unknown row.
  ctx.proxy.armGate({id: 'target-create-ack-lost', phase: 'after-response', method: 'POST',
    pathPattern: /\/containers\/create$/, decision: 'lose-response',
    match: row => row.kind === 'check' && row.labels['luci.check-project'] === targetProject.id});
  const targetStarted = await ctx.startCheckInBrowser(targetProject, command);
  const lost = await ctx.proxy.waitForGate('target-create-ack-lost', {timeoutMs: 120000});
  assert.equal(lost.request.status, 201); assert.match(lost.request.id, /^[a-f0-9]{64}$/);
  let target = await ctx.waitCheckTerminal(targetProject, targetStarted.id);
  const targetLease = exactLease(await ctx.readLedger(), target); assertUnknown(target, targetLease);
  assert.equal(lost.request.labels['luci.check-allocation'], targetLease.id);
  await ctx.assertRealOriginalCreatedAndNeverStarted(lost.request, targetLease);
  record('real-create-ack-lost', {checkId: target.id, allocationId: targetLease.id,
    containerId: lost.request.id, status: target.status, reasonCode: target.reasonCode});

  // A distinct real check reserves capacity but its Create is held upstream.
  // No ledger file is edited and no extra ACK is lost.
  let witness = null;
  if (chargedWitness) {
    await ctx.admitAdditionalChargedWitness();
    ctx.proxy.armGate({id: 'unrelated-create-held', phase: 'before-forward', method: 'POST',
      pathPattern: /\/containers\/create$/, decision: 'hold',
      match: row => row.kind === 'check' && row.labels['luci.check-project'] === witnessProject.id});
    const witnessStarted = await ctx.startCheckInBrowser(witnessProject, command);
    await ctx.proxy.waitForGate('unrelated-create-held', {timeoutMs: 120000});
    witness = await ctx.readCheck(witnessProject, witnessStarted.id);
  }
  target = await ctx.readCheck(targetProject, target.id); // settle post-failure observation before exact review
  const before = await ctx.readLedger(), witnessLease = witness ? exactLease(before, witness) : null;
  if (witness) { assert.equal(witnessLease.state, 'creating'); assert.equal(witness.resourcesHeld, true); }
  assertUnknown(target, exactLease(before, target)); assert.equal(active(before).length, chargedWitness ? 2 : 1);
  const sourceBefore = await ctx.snapshotProjectFiles();
  await ctx.assertCurrentReservationTotals({count: chargedWitness ? 2 : 1, memoryMiB: chargedWitness ? 2048 : 1024, cpuMillis: chargedWitness ? 2000 : 1000});
  const beforeEffects = mutationIntents(ctx.proxy);
  const preservedReceipts = {target: structuredClone(target), witness: structuredClone(witness), history: historySnapshot};
  async function assertNoRecoveryEffects(label) {
    assert.deepEqual(await ctx.readLedger(), before, label + ': ledger changed');
    assert.deepEqual(await ctx.readCheck(targetProject, target.id), preservedReceipts.target, label + ': target changed');
    if (witness) assert.deepEqual(await ctx.readCheck(witnessProject, witness.id), preservedReceipts.witness, label + ': witness changed');
    assert.deepEqual(await ctx.readCheck(historyProject, history.id), preservedReceipts.history, label + ': history changed');
    assert.deepEqual(await ctx.snapshotProjectFiles(), sourceBefore, label + ': source changed');
    assert.deepEqual(mutationIntents(ctx.proxy), beforeEffects, label + ': Docker mutation attempted');
  }
  const review = await ctx.readRecovery(targetProject, target.id); assertReview(review, target);
  await ctx.openAndDismissReviewInBrowser(targetProject, target.id, review);
  await assertNoRecoveryEffects('Read and dismiss');

  // Both probes use current installed API responses and explicit fixture
  // identities. An unchanged review after a refusal must still be reviewed.
  await ctx.probeNoEditAuthority({targetProject, target, review, expectedStatus: 403});
  await assertNoRecoveryEffects('No current Edit authority');
  await ctx.probeStaleReview({targetProject, target, review, expectedStatus: 409});
  await assertNoRecoveryEffects('Stale reviewed revision');
  const current = await ctx.readCheck(targetProject, target.id);
  const refreshed = await ctx.readRecovery(targetProject, target.id); assertReview(refreshed, current);
  const posted = await ctx.recoverInBrowser(targetProject, target.id, refreshed);
  assert.equal(posted.httpStatus, 200);
  assert.deepEqual(posted.request, {expectedRevision: refreshed.revision,
    recoveryDigest: refreshed.recoveryDigest, acknowledgeStop: true});
  assert.equal(posted.postCount, 1, 'Exactly one successful explicit browser recovery POST');

  const targetAfter = await ctx.readCheck(targetProject, target.id), after = await ctx.readLedger();
  const evidence = assertRetired({before, after, targetBefore: target, targetAfter,
    witnessBefore: witness, witnessAfter: witness ? await ctx.readCheck(witnessProject, witness.id) : null,
    historyBefore: historySnapshot, historyAfter: await ctx.readCheck(historyProject, history.id)});
  await ctx.assertAcknowledgedPermanentFence({originalCreate: lost.request, ...evidence});
  await ctx.assertCurrentReservationTotals({count: chargedWitness ? 1 : 0, memoryMiB: chargedWitness ? 1024 : 0, cpuMillis: chargedWitness ? 1000 : 0});
  assert.deepEqual(await ctx.snapshotProjectFiles(), sourceBefore);
  const rows = ctx.proxy.observations.requests;
  const targetCreates = creates(rows).filter(r => r.kind === 'check' && r.labels['luci.check-id'] === target.id);
  assert.equal(targetCreates.length, 1, 'Recovery must not replay target Create');
  assert.equal(starts(rows).filter(r => r.ref === lost.request.id).length, 0);
  assert.equal(creates(rows).filter(r => r.kind === 'fence').length, 1);
  record('exact-target-charge-retired', {checkId: target.id, allocationId: targetLease.id,
    resourcesHeld: false, cleanupConfirmed: true, witnessAllocationId: witnessLease?.id ?? null,
    fenceId: evidence.fence.proof.fenceRef, fenceState: evidence.fence.state,
    beforeCharge: {memoryMiB: chargedWitness ? 2048 : 1024, cpuMillis: chargedWitness ? 2000 : 1000},
    afterCharge: {memoryMiB: chargedWitness ? 1024 : 0, cpuMillis: chargedWitness ? 1000 : 0},
    concurrentChargedWitness: chargedWitness ? 'verified' : 'not-exercised-single-allocation-scope'});

  // The unrelated original request is then released once, with a fresh guard.
  let witnessAfter = null;
  if (chargedWitness) {
    await ctx.proxy.releaseGate('unrelated-create-held');
    witnessAfter = await ctx.waitCheckTerminal(witnessProject, witness.id);
    assert.equal(witnessAfter.status, 'passed'); assert.equal(witnessAfter.resourcesHeld, false);
    assert.equal(witnessAfter.commands[0].output, ctx.expectedOutput);
    assert.equal(witnessAfter.commands[0].cleanupConfirmed, true);
  }
  assert.deepEqual(await ctx.readCheck(targetProject, target.id), targetAfter);
  assert.deepEqual(await ctx.readCheck(historyProject, history.id), historySnapshot);
  assert.deepEqual(await ctx.snapshotProjectFiles(), sourceBefore);
  assert.equal(active(await ctx.readLedger()).length, 0);
  await ctx.assertCurrentReservationTotals({count: 0, memoryMiB: 0, cpuMillis: 0});
  await ctx.assertFenceStillPresent(evidence.fence);
  const completed = await ctx.readRecovery(targetProject, target.id); assert.equal(completed.state, 'complete');
  assert.equal(completed.method, null); assert.equal(completed.recoveryDigest, null);
  assert.equal(ctx.proxy.observations.counts.check, chargedWitness ? 3 : 2);
  assert.equal(ctx.proxy.observations.counts.worker, 0);
  assert.equal(ctx.proxy.observations.counts.fence, 1);
  assert.equal(ctx.proxy.observations.counts.create, chargedWitness ? 4 : 3);
  assert.equal(ctx.proxy.observations.counts.start, chargedWitness ? 2 : 1);
  assert.equal(ctx.proxy.observations.dropped, 0);
  assert.deepEqual(ctx.proxy.observations.failures, []);
  record('scenario-complete-marker-retained', {witnessCheckId: witness?.id ?? null, retainedHistoryCheckId: history.id,
    activeCheckLeases: 0, permanentFenceId: evidence.fence.proof.fenceRef,
    scope: 'Installed explicit check recovery only; recovery is not passing target-check evidence.'});
  return {artifacts, target: targetAfter, witness: witnessAfter,
    retainedMarker: {id: evidence.fence.proof.fenceRef, allocationId: targetLease.id},
    cleanupPending: true, passed: false}; // Adapter must positively close controllers/proxy/browser first.
}
