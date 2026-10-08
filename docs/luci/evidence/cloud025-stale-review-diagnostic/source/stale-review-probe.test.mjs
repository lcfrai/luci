// Execute the exact DAG stale-review block and API/body parser with local fakes.
// No application, Git, Docker, browser, provider or network process is started.
import assert from 'node:assert/strict';
import {test} from 'node:test';
import {readFileSync, writeFileSync, mkdtempSync, rmSync} from 'node:fs';
import {join} from 'node:path';
import {tmpdir} from 'node:os';
import {createHash} from 'node:crypto';
import {probeStaleReview} from './stale-review-probe.mjs';

const source = readFileSync(new URL('./dag.mjs', import.meta.url), 'utf8');
function extract(start, end) {
  assert.equal(source.split(start).length, 2, 'One exact production seam');
  const begin = source.indexOf(start), finish = source.indexOf(end, begin);
  assert(finish > begin);
  return source.slice(begin, finish);
}
const apiSource = extract('async function api(', '\nasync function until(');
const bodySource = extract('async function boundedResponse(', '\nfunction privateJSON(');
const block = extract("setStage('stale-review-refusal')", " setStage('renewed-review-real-conflict')");
const AsyncFunction = Object.getPrototypeOf(async function(){}).constructor;
const sha = text => createHash('sha256').update(text).digest('hex');

function fixture(t, options = {}) {
  const evidence = mkdtempSync(join(tmpdir(), 'luci-stale-review-unit-'));
  t.after(() => rmSync(evidence, {recursive:true, force:true}));
  const receipt = {staleReview:{captured:{reviewDigest:'d'.repeat(64)}}};
  const owner = {route:'/fixture/project', owner:{path:'/fixture/primary'}, run:{id:'run-fixture', revision:13}};
  const b1 = {checkout:'/fixture/beta'}, oldB = {review:{reviewDigest:'d'.repeat(64)}};
  const calls = [], saved = [], reads = [];
  let after = false;
  const scrub = text => String(text).replaceAll('fixture-secret', '[redacted]');
  const fetch = async (url, init) => {
    calls.push({url, method:init.method, body:init.body, signal:init.signal});
    if (init.method === 'GET') {
      assert.equal(url, 'http://fixture.invalid/fixture/project/runs/run-fixture');
      if (after && options.publicReadError) throw Error('Public read unavailable');
      return new Response(JSON.stringify({run:{id:'run-fixture', revision:after ? (options.revision ?? 13) : 13}}));
    }
    assert.equal(init.method, 'POST');
    assert.equal(url, 'http://fixture.invalid/fixture/project/runs/run-fixture/swarm/tasks/task-b/integrate');
    assert.deepEqual(JSON.parse(init.body), {expectedRevision:13, reviewDigest:'d'.repeat(64)});
    // The precondition checkpoint must already exist before the only mutation.
    assert.equal(JSON.parse(readFileSync(join(evidence, 'stale-review-probe.json'))).phase, 'before-request');
    after = true;
    if (options.requestError) throw Error('Synthetic lost response');
    return new Response(options.raw ?? JSON.stringify(options.response ?? {error:'project_run_conflict', message:'Task review changed; review the current changes.'}), {status:options.status ?? 409});
  };
  const boundedResponse = new Function('Buffer', `return (${bodySource});`)(Buffer);
  const api = new Function('fetch', 'receipt', 'assert', 'scrub', 'boundedResponse', 'health',
    `const closing=false,base='http://fixture.invalid',token='',stage='stale-review-refusal',remainingBudget=value=>value;return (${apiSource});`)
    (fetch, receipt, assert, scrub, boundedResponse, () => {});
  const refreshRun = async () => { owner.run = (await api('GET', owner.route+'/runs/'+owner.run.id)).run; };
  const effects = () => { reads.push(['effects', after]); return {create:2+(after ? (options.createDelta ?? 0) : 0), start:2, destroy:2}; };
  const gitAt = (path, command) => {
    assert.equal(path, owner.owner.path); reads.push([command, after]);
    if (after && options.sourceReadError) throw Error('Source read unavailable');
    return command === 'rev-parse' ? (after && options.changedHead ? 'b'.repeat(40) : 'a'.repeat(40)) : (after ? (options.dirty ?? '') : '');
  };
  const privateRun = () => { reads.push(['private', after]); return {revision:after ? 18 : 17}; };
  const existsSync = path => { assert.equal(path, b1.checkout+'-integration'); reads.push(['checkout', after]); return after && !!options.integrationCheckout; };
  const persist = (path, bytes, flags) => {
    writeFileSync(path, bytes, flags);
    const parsed = JSON.parse(readFileSync(path));
    saved.push(parsed);
  };
  const execute = new AsyncFunction('setStage','refreshRun','owner','receipt','probeStaleReview','scrub','api','effects','gitAt','sha','privateRun','existsSync','b1','writeFileSync','join','evidence','oldB',
    'let run;\n'+block);
  return {receipt, calls, saved, reads, evidence,
    execute:() => execute(() => {}, refreshRun, owner, receipt, probeStaleReview, scrub, api, effects, gitAt, sha, privateRun, existsSync, b1, persist, join, evidence, oldB),
    diagnostic:() => JSON.parse(readFileSync(join(evidence, 'stale-review-probe.json')))};
}
function retained(f) {
  const probe = f.diagnostic();
  assert.equal(probe.phase, 'after-request');
  assert.equal(probe.classification, 'unclassified-response');
  assert.deepEqual(probe, f.receipt.staleReviewDiagnostic);
  assert.equal(probe.requestedRevision, 13);
  assert.equal(probe.before.privateRevision, 17);
  assert.equal(probe.after.privateRevision, 18);
  assert.deepEqual(probe.before.effects, {create:2,start:2,destroy:2});
  assert(f.reads.some(([name, after]) => name === 'checkout' && after));
  assert.equal(f.calls.filter(call => call.method === 'POST').length, 1, 'No automatic mutation retry');
  return probe;
}
test('actual stale-review path records diagnostics before passing every original gate', async t => {
  const f = fixture(t); await f.execute(); const probe = retained(f);
  assert.equal(probe.status, 409); assert.equal(probe.responseObserved, true);
  assert.deepEqual(f.saved.map(value => value.phase), ['before-request','response-received','after-request']);
  assert.equal(f.saved[1].after, null, 'Response was written before after-state reads');
  assert.equal(f.receipt.staleReview.usedCurrentRevision, 13);
  assert.equal(f.receipt.staleReview.mainUnchanged, true);
});
for (const message of ['Project source is busy. Wait before integrating.', 'Project run changed; reload before continuing.']) {
  test('unrelated 409 remains failed after durable reason and state: '+message, async t => {
    const f = fixture(t, {response:{code:'conflict', error:'project_run_conflict', message}});
    await assert.rejects(f.execute(), /Expected a review refusal/);
    const probe = retained(f); assert.equal(probe.response.message, message); assert.equal(probe.status, 409);
    assert.equal(f.receipt.staleReview.mainUnchanged, undefined);
  });
}
test('wrong HTTP status retains response and after-state before the existing API assertion escapes', async t => {
  const f = fixture(t, {status:503}); await assert.rejects(f.execute(), /503/);
  const probe = retained(f); assert.equal(probe.status, 503); assert.match(probe.response.message, /review/);
  assert(probe.requestFailure); assert.equal(f.receipt.staleReview.mainUnchanged, undefined);
});
test('truncated diagnostic fields remain bounded and cannot satisfy the reason gate', async t => {
  const f = fixture(t, {response:{code:'c'.repeat(129), message:'review '+ 'x'.repeat(2200), error:'e'.repeat(2200), privateToken:'never-copy'}});
  await assert.rejects(f.execute(), /Truncated fields/);
  const probe = retained(f); assert.deepEqual(probe.truncatedFields, ['code','message','error']);
  assert.deepEqual(Object.values(probe.response).map(value => value.length), [128,2048,2048]);
  assert.equal(probe.response.privateToken, undefined); assert.equal(f.receipt.staleReview.mainUnchanged, undefined);
});
for (const [name, raw, expected] of [['truncated JSON','{"message":"review',/Invalid JSON/], ['over-bound body','x'.repeat(2*1024**2+1),/HTTP response bound/]]) {
  test(name+' retains recorded HTTP status without inventing a returned reason', async t => {
    const f = fixture(t, {raw}); await assert.rejects(f.execute(), expected);
    const probe = retained(f); assert.equal(probe.status, 409); assert.equal(probe.responseObserved, false);
    assert.deepEqual(probe.response, {}); assert(probe.requestFailure); assert.equal(f.receipt.staleReview.mainUnchanged, undefined);
  });
}
for (const options of [{createDelta:1}, {changedHead:true}, {dirty:' M shared.txt'}, {integrationCheckout:true}, {revision:14}]) {
  test('original no-effect/source/checkout/revision gate remains strict: '+JSON.stringify(options), async t => {
    const f = fixture(t, options); await assert.rejects(f.execute()); retained(f);
    assert.equal(f.receipt.staleReview.mainUnchanged, undefined);
  });
}
test('failed after-state reads do not suppress independent observations or accept missing state', async t => {
  const f = fixture(t, {sourceReadError:true, publicReadError:true}); await assert.rejects(f.execute(), /after-state is unavailable/);
  const probe = retained(f); assert(probe.after.errors.primaryCommit); assert(probe.after.errors.primaryStatusSHA256); assert(probe.after.errors.publicRevision);
  assert.equal(probe.after.integrationCheckoutExists, false); assert.equal(probe.after.privateRevision, 18);
});
test('lost response preserves after-state, status remains unknown, and mutation is never replayed', async t => {
  const f = fixture(t, {requestError:true}); await assert.rejects(f.execute(), /Synthetic lost response/);
  const probe = retained(f); assert.equal(probe.status, null); assert.equal(probe.responseObserved, false);
});
test('only bounded known fields are retained and caller secrets are scrubbed before disk writes', async t => {
  const f = fixture(t, {response:{message:'fixture-secret busy', error:'project_run_conflict', code:'fixture-secret', privateToken:'never-copy'}});
  await assert.rejects(f.execute()); const probe = retained(f);
  assert.equal(probe.response.message, '[redacted] busy'); assert.equal(probe.response.code, '[redacted]');
  for (const value of f.saved) { assert(!JSON.stringify(value).includes('fixture-secret')); assert(!JSON.stringify(value).includes('never-copy')); }
});
