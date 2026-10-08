// Synthetic upstream HTTP/socket tests only; this file never contacts Docker.
import test from 'node:test';
import assert from 'node:assert/strict';
import http from 'node:http';
import { mkdtemp, rm, unlink, writeFile, readFile } from 'node:fs/promises';
import { join } from 'node:path';
import { tmpdir } from 'node:os';
import { startFencedDockerProxy } from './fenced-docker-proxy.mjs';

const workerID = 'a'.repeat(64), fenceID = 'b'.repeat(64), secondFenceID = 'c'.repeat(64);
const createPath = name => `/v1.47/containers/create?name=${name}`;
const inspectPath = id => `/v1.47/containers/${id}/json`;
const workerBody = { Image: 'fixture-image', Env: ['DO_NOT_LOG=this-private-value'], Labels: { 'luci.session-id': 'task-one', 'luci.managed': 'true', 'not-a-public-label': 'also-private' } };
const fenceBody = { Cmd: ['/'], NetworkDisabled: true, Labels: { 'luci.launch-fence': '1', 'luci.instance': 'fixture-instance', 'luci.sandbox-id': 'task-one', 'luci.session-id': 'task-one', 'luci.fence-intent': 'd'.repeat(32), 'luci.fence-binding': 'e'.repeat(64) } };
const send = (socketPath, method, path, body) => {
  let request;
  const promise = new Promise(resolve => {
    request = http.request({ socketPath, method, path, agent: false }, response => {
      const chunks = [];
      response.on('data', chunk => chunks.push(chunk));
      response.once('end', () => resolve({ status: response.statusCode, body: Buffer.concat(chunks).toString() }));
      response.once('error', cause => resolve({ error: cause.code }));
    });
    request.once('error', cause => resolve({ error: cause.code }));
    request.end(body === undefined ? undefined : typeof body === 'string' ? body : JSON.stringify(body));
  });
  return { promise, destroy: () => request.destroy() };
};
const json = (res, status, data) => { res.writeHead(status, { 'content-type': 'application/json' }); res.end(JSON.stringify(data)); };
const until = async predicate => {
  const deadline = Date.now() + 1000;
  while (!predicate()) {
    if (Date.now() > deadline) throw Error('Timed out waiting for the actual transport event');
    await new Promise(resolve => setImmediate(resolve));
  }
};

async function fixture(t, respond, overrides = {}) {
  const directory = await mkdtemp(join(tmpdir(), 'lfp-'));
  const upstreamSocketPath = join(directory, 'up.sock'), socketPath = join(directory, 'p.sock');
  const received = [], sockets = new Set();
  const server = http.createServer((req, res) => {
    const parts = [];
    req.on('data', chunk => parts.push(chunk));
    req.on('end', () => {
      const entry = { method: req.method, path: req.url, body: Buffer.concat(parts).toString() };
      received.push(entry); respond(req, res, entry);
    });
  });
  server.on('connection', socket => { sockets.add(socket); socket.once('close', () => sockets.delete(socket)); });
  await new Promise(resolve => server.listen(upstreamSocketPath, resolve));
  const proxy = await startFencedDockerProxy({
    socketPath, upstreamSocketPath,
    identifyCreate: body => JSON.parse(body).Labels?.['luci.launch-fence'] === '1' ? 'fence' : 'worker',
    beforeCreate: () => {}, upstreamTimeoutMs: 1000, holdTimeoutMs: 1000,
    ...overrides,
  });
  t.after(async () => {
    await proxy.close().catch(() => {});
    for (const socket of sockets) socket.destroy();
    await new Promise(resolve => server.close(resolve));
    await rm(directory, { recursive: true, force: true });
  });
  return { proxy, received, socketPath, directory, server, upstreamSocketPath, request: (method, path, body) => send(socketPath, method, path, body) };
}

test('held uploaded Create survives requester death; released bytes get actual name conflict', async t => {
  let fenceCreated = false;
  const f = await fixture(t, (_req, res, request) => {
    const body = JSON.parse(request.body);
    if (body.Labels['luci.launch-fence']) { fenceCreated = true; json(res, 201, { Id: fenceID }); }
    else json(res, fenceCreated ? 409 : 201, fenceCreated ? { message: 'actual fake-upstream name conflict' } : { Id: workerID });
  }, { maxCreates: 3, maxCreatesByKind: { worker: 2, fence: 1 } });
  f.proxy.armGate({ id: 'delayed-create', phase: 'before-forward', method: 'POST', match: meta => meta.kind === 'worker' });
  const original = f.request('POST', createPath('luci-task-one'), workerBody);
  const gate = await f.proxy.waitForGate('delayed-create');
  assert.equal(gate.request.name, 'luci-task-one'); assert.equal(f.received.length, 0);
  original.destroy(); assert.ok((await original.promise).error);
  assert.equal((await f.request('POST', createPath('luci-task-one'), fenceBody).promise).status, 201);
  const drained = await f.proxy.releaseGate('delayed-create');
  assert.equal(drained.status, 409); assert.ok(drained.forwardedAt); assert.ok(drained.completedAt);
  await assert.rejects(f.proxy.releaseGate('delayed-create'), /gate-not-held/);
  assert.equal(f.received.length, 2); assert.equal(f.received[1].body, JSON.stringify(workerBody));
  assert.equal(f.proxy.observations.counts.start, 0);
  const receipt = JSON.stringify(f.proxy.observations);
  assert.equal(receipt.includes('this-private-value'), false); assert.equal(receipt.includes('also-private'), false);
  assert.equal((await f.proxy.close()).drained, true);
});

test('delayed immutable Start receives actual 404 after remove and never retargets the fence', async t => {
  let originalPresent = true;
  const f = await fixture(t, (_req, res, request) => {
    if (request.path.includes('/create')) json(res, 201, { Id: JSON.parse(request.body).Labels['luci.launch-fence'] ? fenceID : workerID });
    else if (request.method === 'DELETE') { originalPresent = false; res.writeHead(204); res.end(); }
    else json(res, originalPresent ? 204 : 404, { message: 'immutable ID absent' });
  });
  assert.equal((await f.request('POST', createPath('luci-task-one'), workerBody).promise).status, 201);
  f.proxy.armGate({ id: 'delayed-start', phase: 'before-forward', method: 'POST', match: meta => meta.ref === workerID && meta.path.endsWith('/start') });
  const delayed = f.request('POST', `/v1.47/containers/${workerID}/start`);
  await f.proxy.waitForGate('delayed-start'); delayed.destroy(); await delayed.promise;
  assert.equal((await f.request('DELETE', `/v1.47/containers/${workerID}?force=1`).promise).status, 204);
  assert.equal((await f.request('POST', createPath('luci-task-one'), fenceBody).promise).status, 201);
  assert.equal((await f.proxy.releaseGate('delayed-start')).status, 404);
  assert.equal(f.received.at(-1).path, `/v1.47/containers/${workerID}/start`);
  const startsBefore = f.received.length;
  assert.equal((await f.request('POST', `/v1.47/containers/${fenceID}/start`).promise).status, 503);
  assert.equal(f.received.length, startsBefore);
});

test('lost real fence ACK retains ID; async proof predicate holds only the final inspect response', async t => {
  let creates = 0, durableProof = false;
  const f = await fixture(t, (_req, res, request) => {
    if (request.path.includes('/create')) json(res, 201, { Id: ++creates === 1 ? fenceID : secondFenceID });
    else if (request.method === 'DELETE') { res.writeHead(204); res.end(); }
    else json(res, 200, { Id: secondFenceID, Config: { Env: ['PRIVATE_RESPONSE'] }, State: { Status: 'created' } });
  });
  f.proxy.armGate({ id: 'lost-ack', phase: 'after-response', method: 'POST', decision: 'lose-response', match: meta => meta.kind === 'fence' && meta.status === 201 });
  const lost = f.request('POST', createPath('luci-task-one'), fenceBody);
  const actual = await f.proxy.waitForGate('lost-ack');
  assert.equal(actual.request.createResponseId, fenceID); assert.equal(actual.request.status, 201);
  assert.ok((await lost.promise).error);
  assert.equal((await f.request('DELETE', `/v1.47/containers/${fenceID}`).promise).status, 204);
  assert.equal((await f.request('POST', createPath('luci-task-one'), fenceBody).promise).status, 201);
  f.proxy.armGate({ id: 'proof-published', phase: 'after-response', method: 'GET', pathPattern: /\/json$/, match: async meta => durableProof && meta.ref === secondFenceID && meta.status === 200 });
  assert.equal((await f.request('GET', inspectPath(secondFenceID)).promise).status, 200);
  durableProof = true;
  const inspect = f.request('GET', inspectPath(secondFenceID));
  const proofGate = await f.proxy.waitForGate('proof-published');
  assert.equal(proofGate.request.ref, secondFenceID); assert.equal(proofGate.request.status, 200);
  assert.equal(JSON.stringify(proofGate).includes('PRIVATE_RESPONSE'), false);
  inspect.destroy(); await inspect.promise;
  await until(() => f.proxy.observations.requests.at(-1).clientDisconnectedAt);
  const released = await f.proxy.releaseGate('proof-published');
  assert.equal(released.status, 200); assert.ok(released.discardedAt);
  assert.equal((await f.proxy.close()).drained, true);
});

test('close conclusively cancels an unsent gate and preserves a replacement public path', async t => {
  const f = await fixture(t, (_req, res) => json(res, 201, { Id: workerID }));
  f.proxy.armGate({ id: 'unsent', phase: 'before-forward', method: 'POST', match: meta => meta.kind === 'worker' });
  const original = f.request('POST', createPath('luci-task-one'), workerBody);
  await f.proxy.waitForGate('unsent');
  await unlink(f.socketPath); await writeFile(f.socketPath, 'replacement-preserved');
  const cleanup = await f.proxy.close(); await original.promise;
  assert.equal(cleanup.drained, true); assert.equal(f.received.length, 0);
  assert.equal(await readFile(f.socketPath, 'utf8'), 'replacement-preserved');
  assert.ok(f.proxy.observations.requests[0].cancelledBeforeForwardAt);
});

test('close drains a forwarded mutation despite downstream death; transport truncation remains uncertain', async t => {
  let reply, seen;
  const observed = new Promise(resolve => { seen = resolve; });
  const f = await fixture(t, (_req, res) => { reply = res; seen(); });
  const original = f.request('POST', createPath('luci-task-one'), workerBody);
  await observed; original.destroy(); await original.promise;
  const closing = f.proxy.close();
  json(reply, 201, { Id: workerID });
  assert.equal((await closing).drained, true);
  assert.equal(f.proxy.observations.requests[0].id, workerID);

  const broken = await fixture(t, (_req, res) => { res.writeHead(201, { 'content-length': 200 }); res.write('{"Id":'); res.socket.destroy(); });
  await broken.request('POST', createPath('luci-task-one'), workerBody).promise;
  await assert.rejects(broken.proxy.close(), /forwarded-mutation-outcome-uncertain/);
  assert.equal(broken.proxy.observations.cleanup.drained, false);
});

test('unknown mutation, kind budget, body limit and failed allocation guard never forward', async t => {
  const f = await fixture(t, (_req, res) => json(res, 201, { Id: workerID }), { maxCreates: 4, maxCreatesByKind: { worker: 1, fence: 1 }, maxBodyBytes: 1024 });
  for (const [method, path] of [['POST', '/v1.47/images/create'], ['DELETE', '/v1.47/containers/luci-task-one'], ['POST', `/v1.47/containers/${'f'.repeat(64)}/start`]]) {
    assert.equal((await f.request(method, path).promise).status, 503);
  }
  assert.equal(f.received.length, 0);
  assert.equal((await f.request('POST', createPath('luci-task-one'), workerBody).promise).status, 201);
  assert.equal((await f.request('POST', createPath('luci-task-two'), workerBody).promise).status, 503);
  assert.equal((await f.request('POST', createPath('luci-task-three'), 'x'.repeat(1025)).promise).status, 413);
  assert.equal(f.received.length, 1);
  const rejected = await fixture(t, (_req, res) => json(res, 201, { Id: workerID }), { beforeCreate: () => { throw Error('capacity unavailable'); } });
  assert.equal((await rejected.request('POST', createPath('luci-task-one'), workerBody).promise).status, 503);
  assert.equal(rejected.received.length, 0);
});

test('gate expiry is explicit, one-shot release cannot replay, and arm cancellation resolves waiters', async t => {
  const f = await fixture(t, (_req, res) => json(res, 201, { Id: workerID }), { holdTimeoutMs: 25 });
  f.proxy.armGate({ id: 'expires', phase: 'before-forward', method: 'POST' });
  const original = f.request('POST', createPath('luci-task-one'), workerBody);
  await f.proxy.waitForGate('expires');
  assert.equal((await original.promise).status, 503); assert.equal(f.received.length, 0);
  await assert.rejects(f.proxy.releaseGate('expires'), /gate-not-held/);
  f.proxy.armGate({ id: 'unused', phase: 'after-response', method: 'GET' });
  await f.proxy.cancelGate('unused');
  await assert.rejects(f.proxy.waitForGate('unused'), /gate-cancelled-before-match/);
});

test('observation overflow rejects later work instead of silently losing evidence', async t => {
  const f = await fixture(t, (_req, res) => json(res, 200, {}), { maxObservations: 1 });
  assert.equal((await f.request('GET', '/v1.47/info').promise).status, 200);
  assert.equal((await f.request('GET', '/v1.47/info').promise).status, 503);
  assert.equal(f.received.length, 1); assert.equal(f.proxy.observations.dropped, 1);
  assert.equal(f.proxy.observations.requests.length, 1);
});

test('aggregate body and concurrent request bounds do not leak retained buffers on cancel', async t => {
  const f = await fixture(t, (_req, res) => json(res, 201, { Id: workerID }), { maxPendingBodyBytes: Buffer.byteLength(JSON.stringify(workerBody)) + 8, maxConcurrentRequests: 2 });
  f.proxy.armGate({ id: 'body-owner', phase: 'before-forward', method: 'POST' });
  const first = f.request('POST', createPath('luci-task-one'), workerBody);
  await f.proxy.waitForGate('body-owner');
  assert.equal((await f.request('POST', createPath('luci-task-two'), workerBody).promise).status, 503);
  assert.equal(f.proxy.observations.requests.at(-1).failure, 'pending-body-budget');
  await f.proxy.cancelGate('body-owner'); await first.promise;
  assert.equal(f.received.length, 0);
  assert.equal((await f.proxy.close()).pendingBodyBytes, 0);

  const limited = await fixture(t, (_req, res) => json(res, 201, { Id: workerID }), { maxConcurrentRequests: 1 });
  limited.proxy.armGate({ id: 'only-slot', phase: 'before-forward', method: 'POST' });
  const held = limited.request('POST', createPath('luci-task-one'), workerBody);
  await limited.proxy.waitForGate('only-slot');
  assert.equal((await limited.request('GET', '/v1.47/info').promise).status, 503);
  assert.equal(limited.proxy.observations.requests.at(-1).failure, 'concurrent-request-limit');
  await limited.proxy.cancelGate('only-slot'); await held.promise;
});

test('overlong socket paths fail before allocation', async () => {
  await assert.rejects(startFencedDockerProxy({ socketPath: '/workspace/' + 'x'.repeat(85) + '/p.sock', upstreamSocketPath: '/run/docker.sock', identifyCreate() {}, beforeCreate() {} }), /socket-path-too-long/);
});

test('a malformed or unexpected success is never accepted as a known container identity', async t => {
  for (const [status, body] of [[201, { Id: 'short' }], [200, { Id: workerID }]]) {
    const f = await fixture(t, (_req, res) => json(res, status, body));
    assert.equal((await f.request('POST', createPath('luci-task-one'), workerBody).promise).status, 502);
    await assert.rejects(f.proxy.close(), /forwarded-mutation-outcome-uncertain/);
    assert.equal(f.proxy.observations.cleanup.drained, false);
  }
});

test('a forwarded request that never finishes makes close fail after its bounded drain', async t => {
  let entered;
  const ready = new Promise(resolve => { entered = resolve; });
  const f = await fixture(t, () => entered(), { upstreamTimeoutMs: 35 });
  const request = f.request('POST', createPath('luci-task-one'), workerBody);
  await ready;
  await assert.rejects(f.proxy.close(), /forwarded-mutation-outcome-uncertain/);
  await request.promise;
  assert.equal(f.proxy.observations.cleanup.pendingRequests, 0);
  assert.equal(f.proxy.observations.cleanup.uncertainMutation, true);
});
