import assert from 'node:assert/strict';
import { createServer, request as httpRequest } from 'node:http';
import { mkdtempSync, readFileSync, rmSync } from 'node:fs';
import { tmpdir } from 'node:os';
import { join } from 'node:path';
import { afterEach, test } from 'node:test';
import { setTimeout as pause } from 'node:timers/promises';
import { createGate, registryChildEnv } from './registry_gate.mjs';

const session = 'a'.repeat(32);
const repo = `luci/runtime-export-${session}`;
const uuid = 'b'.repeat(32);
const digest = `sha256:${'c'.repeat(64)}`;
const openServers = [];
const roots = [];

test('pinned registry child has bounded Go environment without host inheritance', () => {
  const before = process.env.LUCI_REGISTRY_ENV_SENTINEL;
  process.env.LUCI_REGISTRY_ENV_SENTINEL = 'host-secret';
  try {
    assert.deepEqual(registryChildEnv(), {
      PATH: '/usr/bin:/bin',
      GOMEMLIMIT: '192MiB',
      GOMAXPROCS: '1',
      REGISTRY_STORAGE_DELETE_ENABLED: 'false',
      REGISTRY_STORAGE_MAINTENANCE_UPLOADPURGING_ENABLED: 'false',
    });
  } finally {
    if (before === undefined) delete process.env.LUCI_REGISTRY_ENV_SENTINEL;
    else process.env.LUCI_REGISTRY_ENV_SENTINEL = before;
  }
});

async function listen(server) {
  await new Promise((resolve, reject) => server.once('error', reject).listen(0, '127.0.0.1', resolve));
  openServers.push(server);
  return server.address().port;
}

async function freePort() {
  const server = createServer();
  const value = await listen(server);
  await new Promise(resolve => server.close(resolve));
  openServers.splice(openServers.indexOf(server), 1);
  return value;
}

function call(port, path, method = 'GET', body = '', host = `127.0.0.1:${port}`, headers = {}) {
  return new Promise((resolve, reject) => {
    const req = httpRequest({ hostname: '127.0.0.1', port, method, path,
      headers: { host, ...(method === 'GET' || method === 'HEAD' ? {} : { 'content-length': Buffer.byteLength(body) }), ...headers } }, res => {
      const chunks = [];
      res.on('data', chunk => chunks.push(chunk));
      res.on('end', () => resolve({ status: res.statusCode, headers: res.headers, body: Buffer.concat(chunks).toString() }));
    });
    req.on('error', reject);
    req.end(body);
  });
}

async function setup({ losePut = false } = {}) {
  const root = mkdtempSync(join(tmpdir(), 'luci-serial-registry-'));
  roots.push(root);
  const counts = { post: 0, patch: 0, put: 0, del: 0 };
  const backend = createServer((req, res) => {
    if (req.url === '/v2/') { res.writeHead(200); res.end(); return; }
    if (req.method === 'POST') {
      counts.post++;
      res.writeHead(202, { location: `/v2/${repo}/blobs/uploads/${counts.post === 1 ? uuid : 'd'.repeat(32)}?_state=token` });
      res.end();
    } else if (req.method === 'PATCH') {
      counts.patch++;
      req.resume(); req.on('end', () => { res.writeHead(202, { location: `/v2/${repo}/blobs/uploads/${uuid}?_state=token` }); res.end(); });
    } else if (req.method === 'PUT' && req.url.includes('/blobs/uploads/')) {
      counts.put++;
      req.resume(); req.on('end', () => {
        if (losePut) { req.socket.destroy(); return; }
        res.writeHead(201, { 'docker-content-digest': digest, location: `/v2/${repo}/blobs/${digest}` });
        res.end();
      });
    } else if (req.method === 'PUT' && req.url.includes('/manifests/')) {
      req.resume(); req.on('end', () => {
        res.writeHead(201, { 'docker-content-digest': digest });
        res.end();
      });
    } else if (req.method === 'DELETE') {
      counts.del++; res.writeHead(204); res.end();
    } else { res.writeHead(404); res.end(); }
  });
  const backendPort = await listen(backend);
  const frontPort = await freePort();
  const gate = createGate({ session, dataRoot: root, backendPort, frontPort, publicPort: frontPort, timeoutMs: 100 });
  await new Promise((resolve, reject) => gate.server.once('error', reject).listen(frontPort, '127.0.0.1', resolve));
  openServers.push(gate.server);
  return { gate, counts, frontPort, backendPort, root };
}

afterEach(async () => {
  for (const server of openServers.splice(0)) await new Promise(resolve => server.close(resolve));
  for (const root of roots.splice(0)) rmSync(root, { recursive: true, force: true });
});

test('one upload holds the next POST until positive final PUT, with streamed PATCH and replayed receipt', async () => {
  const { gate, counts, frontPort, backendPort, root } = await setup();
  const createPath = `/v2/${repo}/blobs/uploads/`;
  const first = await call(frontPort, createPath, 'POST');
  assert.equal(first.status, 202);
  assert.equal(first.headers.location, `http://127.0.0.1:${frontPort}/v2/${repo}/blobs/uploads/${uuid}?_state=token`);
  const second = call(frontPort, createPath, 'POST');
  await pause(20);
  assert.equal(counts.post, 1);
  assert.equal(JSON.parse(readFileSync(gate.journal)).active.phase, 'active');
  const patch = await call(frontPort, `/v2/${repo}/blobs/uploads/${uuid}?_state=token`, 'PATCH', 'compressed-layer');
  assert.equal(patch.status, 202);
  assert.equal(counts.patch, 1);
  const finishPath = `/v2/${repo}/blobs/uploads/${uuid}?_state=token&digest=${digest}`;
  const finished = await call(frontPort, finishPath, 'PUT');
  assert.equal(finished.status, 201);
  assert.equal(finished.headers.location, `http://127.0.0.1:${frontPort}/v2/${repo}/blobs/${digest}`);
  assert.equal((await second).status, 202);
  assert.equal(counts.post, 2);
  const replay = await call(frontPort, finishPath, 'PUT');
  assert.equal(replay.status, 201);
  assert.equal(counts.put, 1);
  assert.equal((await call(frontPort, '/luci/status')).status, 200);
  assert.equal(gate.snapshot().completed.length, 1);
  assert.equal(gate.snapshot().active.phase, 'active');
  assert.equal(createGate({ session, dataRoot: root, backendPort, frontPort, publicPort: frontPort }).snapshot().completed[0].digest, digest);
  assert.equal((await call(frontPort, `/v2/${repo}/blobs/uploads/${'d'.repeat(32)}?_state=token`, 'DELETE')).status, 204);
  assert.equal(counts.del, 1);
});

test('lost final response retains the exact active upload across gate restart', async () => {
  const { gate, frontPort, backendPort, root } = await setup({ losePut: true });
  await call(frontPort, `/v2/${repo}/blobs/uploads/`, 'POST');
  const result = await call(frontPort, `/v2/${repo}/blobs/uploads/${uuid}?_state=token&digest=${digest}`, 'PUT');
  assert.equal(result.status, 502);
  assert.equal(gate.snapshot().active.uuid, uuid);
  assert.equal(JSON.parse(readFileSync(gate.journal)).active.uuid, uuid);
  const reloaded = createGate({ session, dataRoot: root, backendPort, frontPort: await freePort(), publicPort: await freePort(), timeoutMs: 10 });
  assert.equal(reloaded.snapshot().active.uuid, uuid);
  assert.equal((await call(frontPort, `/v2/${repo}/blobs/uploads/`, 'POST')).status, 409);
});

test('foreign repository, mount, method, host and upgrade refuse without forwarding', async () => {
  const { frontPort, counts } = await setup();
  const cases = [
    call(frontPort, '/v2/foreign/blobs/uploads/', 'POST'),
    call(frontPort, `/v2/${repo}/blobs/uploads/?mount=${digest}&from=foreign`, 'POST'),
    call(frontPort, `/v2/${repo}/blobs/uploads/`, 'PUT'),
    call(frontPort, '/v2/', 'GET', '', 'foreign.test:5000'),
    call(frontPort, `/v2/${repo}/blobs/uploads/`, 'POST', '', `127.0.0.1:${frontPort}`, { upgrade: 'websocket' })
  ];
  const rows = await Promise.all(cases);
  assert.deepEqual(rows.map(row => row.status), [409, 409, 409, 409, 409]);
  assert.equal(counts.post, 0);
});

test('manifest publication is journaled and blocks a second upload or publication', async () => {
  const { gate, frontPort } = await setup();
  const value = await call(frontPort, `/v2/${repo}/manifests/payload`, 'PUT', '{}');
  assert.equal(value.status, 201);
  assert.deepEqual(gate.snapshot().manifest, { phase: 'committed', digest });
  assert.equal((await call(frontPort, `/v2/${repo}/manifests/payload`, 'PUT', '{}')).status, 409);
  assert.equal((await call(frontPort, `/v2/${repo}/blobs/uploads/`, 'POST')).status, 409);
  assert.equal((await call(frontPort, '/luci/status')).body.includes('"ownershipSettled":false'), true);
});

test('oversized manifest and normalized path refuse before upstream effect', async () => {
  const { frontPort, counts } = await setup();
  assert.equal((await call(frontPort, `/v2/${repo}/manifests/payload`, 'PUT', 'x'.repeat((1 << 20) + 1))).status, 409);
  assert.equal((await call(frontPort, `/v2/${repo}/blobs/../blobs/uploads/`, 'POST')).status, 409);
  assert.equal(counts.post, 0);
});
