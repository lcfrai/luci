// Synthetic upstream HTTP/socket tests only; this file never contacts Docker.
import test from 'node:test';
import assert from 'node:assert/strict';
import http from 'node:http';
import net from 'node:net';
import { mkdtemp, rm, unlink, writeFile, readFile } from 'node:fs/promises';
import { join } from 'node:path';
import { tmpdir } from 'node:os';
import { startFencedDockerProxy } from './fenced-docker-proxy.mjs';
import { createAcceptanceJournal } from './durable-acceptance-journal.mjs';
import { createWorkflowDeadline } from './acceptance-lifecycle.mjs';

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

test('new Start is fenced at forwarding after deadline; exact Stop/Delete and drain remain reachable',async t=>{
 let control;
 const f=await fixture(t,(_req,res,request)=>{if(request.path.includes('/create'))json(res,201,{Id:workerID});else{res.writeHead(204);res.end();}},
  {beforeStart:()=>control?.assertActive()});
 assert.equal((await f.request('POST',createPath('luci-task-one'),workerBody).promise).status,201);
 assert.equal((await f.request('POST',`/v1.47/containers/${workerID}/start`).promise).status,204);
 f.proxy.armGate({id:'start-after-deadline',phase:'before-forward',method:'POST',match:meta=>meta.path.endsWith('/start')});
 control=createWorkflowDeadline({timeoutMs:30});t.after(()=>control.close());
 const expired=assert.rejects(control.run(()=>new Promise(()=>{})),{code:'workflow-deadline-exceeded'});
 const late=f.request('POST',`/v1.47/containers/${workerID}/start`);await f.proxy.waitForGate('start-after-deadline');await expired;
 const result=await f.proxy.releaseGate('start-after-deadline');assert.equal(result.proxyStatus,503);assert.equal(result.forwardedAt,undefined);assert.equal((await late.promise).status,503);
 assert.equal(f.received.filter(row=>row.path.endsWith('/start')).length,1);
 assert.equal((await f.request('POST',`/v1.47/containers/${workerID}/stop?t=1`).promise).status,204);
 assert.equal((await f.request('DELETE',`/v1.47/containers/${workerID}`).promise).status,204);
 const closed=await f.proxy.close();assert.equal(closed.drained,true);assert.equal(closed.pendingRequests,0);assert.equal(closed.activeHandlers,0);
});

test('an asynchronous Start guard is refused before upstream mutation',async t=>{
 const f=await fixture(t,(_req,res)=>json(res,201,{Id:workerID}),{beforeStart:async()=>{}});
 assert.equal((await f.request('POST',createPath('luci-task-one'),workerBody).promise).status,201);
 assert.equal((await f.request('POST',`/v1.47/containers/${workerID}/start`).promise).status,503);
 assert.equal(f.received.length,1);assert.equal(f.proxy.observations.requests.at(-1).forwardedAt,undefined);
 assert.equal((await f.proxy.close()).drained,true);
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

const checkID = 'd'.repeat(64), checkRunID = 'check-' + 'e'.repeat(24), checkName = 'luci-check-' + checkRunID + '-0';
const checkBody = { Image: 'fixture-check-image', Labels: { 'luci.check': 'true', 'luci.check-id': checkRunID } };
const checkOptions = { allowCheckTraffic: true, maxCreatesByKind: { worker: 4, fence: 1, check: 1 }, maxCreates: 5, maxStarts: 5, identifyCreate: body => JSON.parse(body).Labels?.['luci.check'] === 'true' ? 'check' : 'worker' };
function upgradeRequest(socketPath, path, extraHead = '') {
  let socket;
  const promise = new Promise(resolveReply => {
    const chunks = [];
    socket = net.createConnection({ path: socketPath }, () => socket.write(`POST ${path} HTTP/1.1\r\nHost: fixture\r\nConnection: Upgrade\r\nUpgrade: tcp\r\nContent-Length: 0\r\n\r\n${extraHead}`));
    socket.on('data', data => chunks.push(data));
    socket.on('error', () => {});
    socket.once('close', () => resolveReply(Buffer.concat(chunks)));
  });
  return { promise, destroy: () => socket.destroy() };
}
const attachPath = id => `/v1.47/containers/${id}/attach?stream=1&stdout=1&stderr=1&stdin=0`;

test('checks and upgrades are denied by default without forwarding', async t => {
  const f = await fixture(t, (_req, res) => json(res, 201, { Id: checkID }), { identifyCreate: () => 'check' });
  assert.equal((await f.request('POST', createPath(checkName), checkBody).promise).status, 503);
  assert.equal((await upgradeRequest(f.socketPath, attachPath(checkID)).promise).length, 0);
  assert.equal(f.received.length, 0);
  assert.equal(f.proxy.observations.counts.check, undefined);
});

test('one explicitly enabled check forwards actual start/wait and unchanged named cleanup after exact inspect', async t => {
  const f = await fixture(t, (_req, res, request) => {
    if (request.path.includes('/create')) json(res, 201, { Id: checkID });
    else if (request.path.endsWith('/json')) json(res, 200, { Id: checkID, Name: '/' + checkName, Config: { Labels: checkBody.Labels } });
    else if (request.path.includes('/wait')) json(res, 200, { StatusCode: 0 });
    else { res.writeHead(204); res.end(); }
  }, checkOptions);
  assert.equal((await f.request('POST', createPath(checkName), checkBody).promise).status, 201);
  assert.equal((await f.request('POST', `/v1.47/containers/${checkID}/start`).promise).status, 204);
  assert.deepEqual(JSON.parse((await f.request('POST', `/v1.47/containers/${checkID}/wait?condition=removed`).promise).body), { StatusCode: 0 });
  const deletion = `/v1.47/containers/${checkName}?force=1`;
  assert.equal((await f.request('DELETE', deletion).promise).status, 204);
  assert.equal(f.received.at(-1).path, deletion);
  assert.equal(f.received.at(-2).path, `/v1.47/containers/${checkName}/json`);
  assert.equal(f.proxy.observations.requests.find(row => row.internalObservation)?.identity, 'exact');
  assert.equal(f.proxy.observations.counts.check, 1); assert.equal(f.proxy.observations.counts.worker, 0);
  assert.equal((await f.proxy.close()).drained, true);
});

test('named check cleanup preserves real404 while a swapped or uncertain name cannot delete anything', async t => {
  for (const mode of ['absent', 'swapped', 'invalid', 'unavailable']) {
    const f = await fixture(t, (_req, res, request) => {
      if (request.path.includes('/create')) json(res, 201, { Id: checkID });
      else if (request.path.endsWith('/json')) {
        if (mode === 'absent') json(res, 404, { message: 'real fake-upstream absence' });
        else if (mode === 'unavailable') json(res, 500, { message: 'unavailable' });
        else if (mode === 'invalid') { res.writeHead(200); res.end('not json'); }
        else json(res, 200, { Id: workerID, Name: '/' + checkName, Config: { Labels: checkBody.Labels } });
      } else json(res, 404, { message: 'real fake-upstream remove absence' });
    }, checkOptions);
    await f.request('POST', createPath(checkName), checkBody).promise;
    const reply = await f.request('DELETE', `/v1.47/containers/${checkName}?force=1`).promise;
    assert.equal(reply.status, mode === 'absent' ? 404 : 503);
    assert.equal(f.received.filter(row => row.method === 'DELETE').length, mode === 'absent' ? 1 : 0);
  }
});

test('check identity, budgets, query and lifecycle bodies fail closed', async t => {
  const f = await fixture(t, (_req, res) => json(res, 201, { Id: checkID }), checkOptions);
  assert.equal((await f.request('DELETE', `/v1.47/containers/${checkName}?force=1`).promise).status, 503);
  await f.request('POST', createPath(checkName), checkBody).promise;
  for (const [method, path, body] of [
    ['POST', `/v1.47/containers/${checkID}/stop`],
    ['POST', `/v1.47/containers/${checkName}/start`],
    ['POST', `/v1.47/containers/${checkID}/wait?condition=unsafe`],
    ['DELETE', `/v1.47/containers/${checkID}?force=1&v=1`],
    ['POST', `/v1.47/containers/${checkID}/start`, '{}'],
    ['POST', createPath(checkName), checkBody],
    ['POST', `/v1.47/containers/${checkID}/exec`, '{}'],
  ]) assert.equal((await f.request(method, path, body).promise).status, 503);
  assert.equal(f.received.length, 1);
});

test('known check attach preserves real upgrade header and output bytes with bounded drain', async t => {
  const f = await fixture(t, (_req, res) => json(res, 201, { Id: checkID }), checkOptions);
  const output = Buffer.from([1,0,0,0,0,0,0,6,111,107,10,226,156,147]);
  const headers = 'HTTP/1.1 101 UPGRADED\r\nConnection: Upgrade\r\nUpgrade: tcp\r\n\r\n';
  let seen;
  f.server.on('upgrade', (req, socket, head) => {
    seen = { path: req.url, head: head.length };
    socket.write(headers.slice(0, 19));
    setImmediate(() => { socket.write(headers.slice(19)); socket.end(output); });
  });
  await f.request('POST', createPath(checkName), checkBody).promise;
  const reply = await upgradeRequest(f.socketPath, attachPath(checkID)).promise;
  assert.deepEqual(reply, Buffer.concat([Buffer.from(headers), output]));
  assert.deepEqual(seen, { path: attachPath(checkID), head: 0 });
  const observed = f.proxy.observations.requests.find(row => row.path.endsWith('/attach'));
  assert.equal(observed.kind, 'check'); assert.equal(observed.status, 101); assert.equal(observed.responseBytes, reply.length);
  assert.equal((await f.proxy.close()).drained, true); assert.deepEqual(f.proxy.observations.failures, []);
});

test('attach rejects workers, unknown checks, stdin and uploaded bytes before the upstream', async t => {
  const f = await fixture(t, (_req, res) => json(res, 201, { Id: checkID }), checkOptions);
  let upgraded = 0; f.server.on('upgrade', (_req, socket) => { upgraded++; socket.destroy(); });
  await f.request('POST', createPath(checkName), checkBody).promise;
  for (const [path, head] of [[attachPath(workerID), ''], [attachPath(checkID).replace('stdin=0','stdin=1'), ''], [attachPath(checkID), 'forbidden-input']]) {
    assert.equal((await upgradeRequest(f.socketPath, path, head).promise).length, 0);
  }
  assert.equal(upgraded, 0);
});

test('attach output/time caps are explicit fixture failures and close releases all sockets', async t => {
  for (const mode of ['bytes', 'time']) {
    const f = await fixture(t, (_req, res) => json(res, 201, { Id: checkID }), { ...checkOptions, maxResponseBytes: 128, upstreamTimeoutMs: 80 });
    f.server.on('upgrade', (_req, socket) => {
      socket.write('HTTP/1.1 101 UPGRADED\r\nConnection: Upgrade\r\nUpgrade: tcp\r\n\r\n');
      if (mode === 'bytes') socket.write(Buffer.alloc(129));
    });
    await f.request('POST', createPath(checkName), checkBody).promise;
    await upgradeRequest(f.socketPath, attachPath(checkID)).promise;
    assert(f.proxy.observations.failures.includes(mode === 'bytes' ? 'check-attach-byte-limit' : 'check-attach-timeout'));
    const closed = await f.proxy.close(); assert.equal(closed.activeHandlers, 0); assert.equal(closed.pendingRequests, 0);
  }
});

test('durable real Create ACK exists before held response delivery',async t=>{
 const dir=await mkdtemp(join(tmpdir(),'proxy-journal-')),path=join(dir,'journal.jsonl');
 const journal=createAcceptanceJournal(path);
 t.after(async()=>{journal.close();await rm(dir,{recursive:true,force:true});});
 const f=await fixture(t,(_req,res)=>json(res,201,{Id:workerID}),{journal});
 f.proxy.armGate({id:'hold-journal-ack',phase:'after-response',method:'POST'});
 const request=f.request('POST',createPath('luci-task-one'),workerBody);
 await f.proxy.waitForGate('hold-journal-ack');
 const rows=(await readFile(path,'utf8')).trim().split('\n').map(JSON.parse);
 assert.equal(rows.filter(row=>row.event==='create-ack').length,1);
 const ack=rows.find(row=>row.event==='create-ack');assert.equal(ack.status,201);assert.equal(ack.id,workerID);assert(ack.sequence>rows.find(row=>row.event==='mutation-forward-intent').sequence);
 assert(!rows.some(row=>row.event==='response-end-issued'));
 assert(!(await readFile(path,'utf8')).includes('this-private-value'));
 await f.proxy.releaseGate('hold-journal-ack');assert.equal((await request.promise).status,201);
 await f.proxy.close();
});

test('durable journal failure refuses forwarding a new mutation',async t=>{
 const f=await fixture(t,(_req,res)=>json(res,201,{Id:workerID}),{journal:{record(event){if(event==='mutation-forward-intent')throw Error('disk unavailable');}}});
 const response=await f.request('POST',createPath('luci-task-one'),workerBody).promise;
 assert.equal(response.status,503);assert.equal(f.received.length,0);assert(f.proxy.observations.failures.includes('journal-write-failed'));
});

test('an incomplete daemon response cannot create a journal ACK',async t=>{
 const events=[];
 const f=await fixture(t,(_req,res)=>{res.writeHead(201,{'content-type':'application/json','content-length':1000});res.write(JSON.stringify({Id:workerID}));res.socket.destroy();},{journal:{record(event,row){events.push({event,...row});}}});
 await f.request('POST',createPath('luci-task-one'),workerBody).promise;
 assert(!events.some(row=>row.event==='create-ack'));
 await assert.rejects(f.proxy.close(),/forwarded-mutation-outcome-uncertain/);
});

const sdkAllocation='checkexec-'+'e'.repeat(24),sdkName='luci-check-'+sdkAllocation;
const sdkLabels={
 'luci.check':'true','luci.check-protocol':'1','luci.instance':'fixture-instance',
 'luci.check-engine':'fixture-engine','luci.check-allocation':sdkAllocation,
 'luci.check-id':'check-'+'f'.repeat(24),'luci.check-command-index':'0',
 'luci.check-scope':'1'.repeat(64),'luci.check-project':'p-aabbccdd',
 'luci.check-command-digest':'2'.repeat(64),'luci.check-source-digest':'3'.repeat(64),
 'luci.check-image':'sha256:'+'4'.repeat(64),
};
const sdkBody={Labels:sdkLabels};
const sdkOptions={allowCheckTraffic:true,checkTrafficProtocol:'sdk-ledger-v1',maxCreatesByKind:{worker:2,fence:1,check:1},identifyCreate:()=> 'check'};

test('SDK opt-in retains exact lease labels and uses immutable inspect/logs/start/remove',async t=>{
 const dir=await mkdtemp(join(tmpdir(),'sdk-journal-')),journalPath=join(dir,'journal.jsonl'),journal=createAcceptanceJournal(journalPath);
 t.after(async()=>{journal.close();await rm(dir,{recursive:true,force:true});});
 const f=await fixture(t,(req,res)=>{
  if(req.url.startsWith('/v1.47/containers/create'))json(res,201,{Id:workerID});
  else if(req.url.endsWith('/json'))json(res,200,{Id:workerID});
  else if(req.url.includes('/logs?')){res.writeHead(200);res.end('DAG_CHECK_OK final\n');}
  else{res.writeHead(204);res.end();}
 },{...sdkOptions,journal});
 assert.equal((await f.request('POST',createPath(sdkName),sdkBody).promise).status,201);
 assert.deepEqual(f.proxy.observations.requests.find(row=>row.createResponseId)?.labels,sdkLabels);
 assert.equal((await f.request('GET',`/v1.47/containers/${workerID}/json`).promise).status,200);
 assert.equal((await f.request('POST',`/v1.47/containers/${workerID}/start`).promise).status,204);
 const logs=await f.request('GET',`/v1.47/containers/${workerID}/logs?stderr=1&stdout=1&tail=all`).promise;
 assert.equal(logs.status,200);assert.equal(logs.body,'DAG_CHECK_OK final\n');
 assert.equal((await f.request('DELETE',`/v1.47/containers/${workerID}?v=1`).promise).status,204);
 assert.equal(f.proxy.observations.counts.start,1);
 const ack=(await readFile(journalPath,'utf8')).trim().split('\n').map(JSON.parse).find(row=>row.event==='create-ack');assert.deepEqual(ack.labels,sdkLabels);
 await f.proxy.close();
});

test('SDK named reads/mutations, attach/wait, streaming logs, and unscoped removal are refused',async t=>{
 const f=await fixture(t,(_req,res)=>json(res,201,{Id:workerID}),sdkOptions);
 assert.equal((await f.request('POST',createPath(sdkName),sdkBody).promise).status,201);
 const requests=[
  ['GET',`/v1.47/containers/${sdkName}/json`],['DELETE',`/v1.47/containers/${sdkName}?v=1`],
  ['POST',`/v1.47/containers/${workerID}/attach?stream=1&stdout=1&stderr=1`],['POST',`/v1.47/containers/${workerID}/wait`],
  ['GET',`/v1.47/containers/${workerID}/json?size=1`],
  ['GET',`/v1.47/containers/${workerID}/logs?stdout=1&stderr=1&tail=all&follow=1`],
  ['GET',`/v1.47/containers/${workerID}/logs?stdout=1&stderr=1`],
  ['GET',`/v1.47/containers/${'9'.repeat(64)}/logs?stdout=1&stderr=1&tail=all`],
  ['DELETE',`/v1.47/containers/${workerID}`],['DELETE',`/v1.47/containers/${workerID}?v=1&link=1`],
 ];
 for(const [method,path] of requests)assert.equal((await f.request(method,path).promise).status,503,method+' '+path);
 let upgraded=0;f.server.on('upgrade',(_req,socket)=>{upgraded++;socket.destroy();});
 assert.equal((await upgradeRequest(f.socketPath,`/v1.47/containers/${workerID}/attach?stream=1&stdout=1&stderr=1`).promise).length,0);
 assert.equal(upgraded,0);assert(f.proxy.observations.failures.includes('sdk-check-upgrade-forbidden'));
 assert.equal(f.received.length,1);
});

test('SDK protocol rejects extra/missing labels, wrong allocation/name, and CLI names',async t=>{
 for(const [name,labels] of [
  [sdkName,{...sdkLabels,unscoped:'extra'}],
  [sdkName,Object.fromEntries(Object.entries(sdkLabels).filter(([key])=>key!=='luci.check-source-digest'))],
  [sdkName,{...sdkLabels,'luci.check-allocation':'checkexec-'+'a'.repeat(24)}],
  ['luci-check-check-'+'f'.repeat(24)+'-0',sdkLabels],
 ]){
  const f=await fixture(t,(_req,res)=>json(res,201,{Id:workerID}),sdkOptions);
  assert.equal((await f.request('POST',createPath(name),{Labels:labels}).promise).status,503);assert.equal(f.received.length,0);
 }
});

test('SDK exact-ID cancellation removal accepts only explicit v=1 and optional force=1',async t=>{
 const f=await fixture(t,(req,res)=>{if(req.method==='POST')json(res,201,{Id:workerID});else{res.writeHead(204);res.end();}},sdkOptions);
 assert.equal((await f.request('POST',createPath(sdkName),sdkBody).promise).status,201);
 assert.equal((await f.request('DELETE',`/v1.47/containers/${workerID}?force=1&v=1`).promise).status,204);
 const recorded=f.received.at(-1);assert.equal(recorded.path,`/v1.47/containers/${workerID}?force=1&v=1`);
});

test('SDK Start obeys the same active fence while exact cancellation removal remains available',async t=>{
 const f=await fixture(t,(_req,res,request)=>{if(request.path.includes('/create'))json(res,201,{Id:workerID});else{res.writeHead(204);res.end();}},
  {...sdkOptions,beforeStart:()=>{throw Object.assign(Error('cancelled'),{code:'workflow-deadline-exceeded'});}});
 assert.equal((await f.request('POST',createPath(sdkName),sdkBody).promise).status,201);
 assert.equal((await f.request('POST',`/v1.47/containers/${workerID}/start`).promise).status,503);
 assert.equal(f.received.filter(row=>row.path.endsWith('/start')).length,0);
 assert.equal((await f.request('DELETE',`/v1.47/containers/${workerID}?force=1&v=1`).promise).status,204);
 assert.equal((await f.proxy.close()).drained,true);
});
