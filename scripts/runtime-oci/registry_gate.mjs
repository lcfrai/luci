/** Owned, serial Docker push gate for a pinned Distribution 3.0.0 child.
 *
 * The producer owns the capped container, loopback-only published port, E:
 * bind mount, exact engine/image probe, push, readback, and retirement. This
 * process does not infer that a stopped/absent upload settled an unknown push.
 */
import { createHash } from 'node:crypto';
import { createReadStream, existsSync, lstatSync, mkdirSync, openSync, readFileSync, renameSync, closeSync, fsyncSync, writeFileSync } from 'node:fs';
import { request as httpRequest, createServer } from 'node:http';
import { spawn } from 'node:child_process';
import { dirname, join, resolve } from 'node:path';
import { Transform } from 'node:stream';
import { pipeline } from 'node:stream/promises';
import { setTimeout as pause } from 'node:timers/promises';
import { fileURLToPath } from 'node:url';

export const RELEASE_ARCHIVE_SHA256 = '61c9a2c0d5981a78482025b6b69728521fbc78506d68b223d4a2eb825de5ca3d';
export const REGISTRY_BINARY_SHA256 = '7e293ee23c59f5f0e5ba162e483af1ce1bca074891d769302bd2cffefbe4363b';
export const REGISTRY_LICENSE_SHA256 = 'cb5e8e7e5f4a3988e1063c142c60dc2df75605f4c46515e776e3aca6df976e14';
const MAX_BODY = 8 * 1024 ** 3;
const MAX_RESPONSE = 1 << 20;
const SHA = /^sha256:[0-9a-f]{64}$/;
const NONCE = /^[0-9a-f]{32}$/;
const UUID = /^[a-f0-9-]{16,64}$/;
const HOP = new Set(['connection', 'keep-alive', 'proxy-authenticate', 'proxy-authorization', 'te', 'trailer', 'transfer-encoding', 'upgrade', 'host', 'forwarded', 'x-forwarded-host', 'x-forwarded-for', 'x-forwarded-proto', 'content-location', 'set-cookie']);

function port(value) {
  const number = Number(value);
  if (!Number.isInteger(number) || number < 1024 || number > 65535) throw new Error('invalid owned registry port');
  return number;
}

function repository(session) {
  if (!NONCE.test(session)) throw new Error('invalid owned registry session');
  return `luci/runtime-export-${session}`;
}

function strictFile(path) {
  const absolute = resolve(path);
  for (let entry = absolute; ; entry = dirname(entry)) {
    if (lstatSync(entry).isSymbolicLink()) throw new Error('linked registry input refused');
    if (entry === dirname(entry)) break;
  }
  if (!lstatSync(absolute).isFile()) throw new Error('registry input must be a regular file');
  return absolute;
}

function strictDir(path) {
  const absolute = resolve(path);
  for (let entry = absolute; ; entry = dirname(entry)) {
    if (lstatSync(entry).isSymbolicLink()) throw new Error('linked registry data directory refused');
    if (entry === dirname(entry)) break;
  }
  if (!lstatSync(absolute).isDirectory()) throw new Error('registry data mount missing');
  return absolute;
}

export async function verifyReleaseBinary(binary, license) {
  for (const [path, expected] of [[binary, REGISTRY_BINARY_SHA256], [license, REGISTRY_LICENSE_SHA256]]) {
    const digest = createHash('sha256');
    for await (const block of createReadStream(strictFile(path), { highWaterMark: 1 << 20 })) digest.update(block);
    if (digest.digest('hex') !== expected) throw new Error('pinned Distribution 3.0.0 bytes differ');
  }
}

function durable(path, value) {
  const raw = Buffer.from(JSON.stringify(value) + '\n');
  if (raw.length > MAX_RESPONSE) throw new Error('registry gate journal exceeds bound');
  const temporary = path + '.next';
  const fd = openSync(temporary, 'wx', 0o600);
  try { writeFileSync(fd, raw); fsyncSync(fd); } finally { closeSync(fd); }
  renameSync(temporary, path);
  const directory = openSync(dirname(path), 'r');
  try { fsyncSync(directory); } finally { closeSync(directory); }
}

function load(path, session, publicPort) {
  const repo = repository(session);
  if (existsSync(path + '.next')) throw new Error('registry gate journal has an unsettled replacement');
  if (!existsSync(path)) {
    const initial = { version: 1, session, repository: repo, active: null, completed: [], manifest: null };
    durable(path, initial);
    return initial;
  }
  if (!lstatSync(path).isFile() || lstatSync(path).size > MAX_RESPONSE) throw new Error('registry gate journal is not bounded/regular');
  const state = JSON.parse(readFileSync(path, 'utf8'));
  if (state.version !== 1 || state.session !== session || state.repository !== repo ||
      !Array.isArray(state.completed) || state.completed.length > 4096 ||
      !['object', 'null'].includes(typeof state.active) ||
      !['object', 'null'].includes(typeof state.manifest)) throw new Error('registry gate journal identity/shape changed');
  if (state.active && (!['creating', 'active'].includes(state.active.phase) ||
      (state.active.phase === 'creating' ? state.active.uuid !== null : !UUID.test(state.active.uuid || '')))) throw new Error('registry gate active upload changed');
  if (state.manifest && (!['publishing', 'committed'].includes(state.manifest.phase) ||
      (state.manifest.phase === 'committed' && !SHA.test(state.manifest.digest || '')))) throw new Error('registry gate manifest state changed');
  if (state.completed.some(row => {
    if (!UUID.test(row.uuid || '') || !SHA.test(row.digest || '') || typeof row.location !== 'string' || row.location.length > 2048) return true;
    try { return committedLocation(row.location, publicPort, publicPort, repo, row.digest) !== row.location; }
    catch { return true; }
  })) throw new Error('registry gate completed upload changed');
  return state;
}

function route(raw, repo) {
  if (typeof raw !== 'string' || !raw.startsWith('/') || raw.includes('#') || raw.length > 2048) throw new Error('registry path refused');
  const rawPath = raw.split('?')[0];
  if (rawPath.includes('%') || rawPath.includes('..') || rawPath.includes('//') || rawPath.includes('\\')) throw new Error('registry path encoding refused');
  const url = new URL(raw, 'http://127.0.0.1');
  const path = url.pathname;
  if (path !== rawPath) throw new Error('registry path normalization refused');
  if (path === '/v2/' && !url.search) return { kind: 'ping', path: raw };
  const prefix = `/v2/${repo}/`;
  if (!path.startsWith(prefix)) throw new Error('foreign registry repository refused');
  const suffix = path.slice(prefix.length);
  if (suffix === 'blobs/uploads/' && !url.search) return { kind: 'create', path: raw };
  const upload = /^blobs\/uploads\/([a-f0-9-]{16,64})$/.exec(suffix);
  if (upload && UUID.test(upload[1])) {
    const keys = [...url.searchParams.keys()];
    if (new Set(keys).size !== keys.length || keys.some(key => !['_state', 'digest'].includes(key)) ||
        (url.searchParams.has('digest') && !SHA.test(url.searchParams.get('digest')))) throw new Error('upload query refused');
    return { kind: 'upload', uuid: upload[1], digest: url.searchParams.get('digest'), path: raw };
  }
  const blob = /^blobs\/(sha256:[0-9a-f]{64})$/.exec(suffix);
  if (blob && !url.search) return { kind: 'blob', digest: blob[1], path: raw };
  const manifest = /^manifests\/(payload|sha256:[0-9a-f]{64})$/.exec(suffix);
  if (manifest && !url.search) return { kind: 'manifest', reference: manifest[1], path: raw };
  throw new Error('registry request shape refused');
}

function uploadLocation(raw, backendPort, publicPort, repo) {
  if (typeof raw !== 'string' || raw.length > 2048) throw new Error('registry upload location missing');
  const url = new URL(raw, `http://127.0.0.1:${backendPort}`);
  if (url.protocol !== 'http:' || url.username || url.password || url.hash ||
      url.hostname !== '127.0.0.1' || ![backendPort, publicPort].includes(Number(url.port))) throw new Error('registry upload location escaped loopback');
  const parsed = route(url.pathname + url.search, repo);
  if (parsed.kind !== 'upload' || parsed.digest) throw new Error('registry upload location escaped session');
  return { uuid: parsed.uuid, publicURL: `http://127.0.0.1:${publicPort}${url.pathname}${url.search}` };
}

function committedLocation(raw, backendPort, publicPort, repo, digest) {
  if (typeof raw !== 'string' || raw.length > 2048) throw new Error('registry committed blob location missing');
  const url = new URL(raw, `http://127.0.0.1:${backendPort}`);
  if (url.protocol !== 'http:' || url.username || url.password || url.hash || url.search ||
      url.hostname !== '127.0.0.1' || ![backendPort, publicPort].includes(Number(url.port))) throw new Error('registry committed blob location escaped loopback');
  const selected = route(url.pathname, repo);
  if (selected.kind !== 'blob' || selected.digest !== digest) throw new Error('registry committed blob identity changed');
  return `http://127.0.0.1:${publicPort}${url.pathname}`;
}

function responseHeaders(headers, location) {
  const result = {};
  for (const [key, value] of Object.entries(headers)) {
    if (HOP.has(key) || key === 'location' || value === undefined) continue;
    result[key] = value;
  }
  if (location) result.location = location;
  return result;
}

function refuse(res, code, message) {
  if (res.headersSent) { res.destroy(); return; }
  res.writeHead(code, { 'content-type': 'text/plain', 'cache-control': 'no-store' });
  res.end(message + '\n');
}

function framing(req, maxBody) {
  if (req.headers.upgrade || req.headers.expect || req.headers.authorization || req.headers['proxy-authorization'] || req.headers.trailer) throw new Error('registry upgrade/expect/authorization refused');
  const length = req.headers['content-length'];
  const transfer = req.headers['transfer-encoding'];
  if (length !== undefined && (!/^(0|[1-9][0-9]*)$/.test(length) || BigInt(length) > BigInt(maxBody))) throw new Error('registry request body bound refused');
  if (transfer !== undefined && (transfer.toLowerCase() !== 'chunked' || !maxBody)) throw new Error('registry transfer framing refused');
  if (!maxBody && length === undefined && !['GET', 'HEAD'].includes(req.method)) throw new Error('registry request body must be empty');
}

export function createGate({ session, dataRoot, backendPort, frontPort, publicPort, timeoutMs = 30 * 60 * 1000 }) {
  const repo = repository(session);
  backendPort = port(backendPort); frontPort = port(frontPort); publicPort = port(publicPort);
  if (backendPort === frontPort) throw new Error('registry backend/frontend port collision');
  const root = strictDir(dataRoot);
  const journal = join(root, 'luci-serial-upload-v1.json');
  let state = load(journal, session, publicPort);
  const waiters = [];
  const commit = change => {
    const next = structuredClone(state);
    change(next);
    durable(journal, next);
    state = next;
  };
  const release = () => { const next = waiters.shift(); if (next) next(); };

  async function acquire(req) {
    while (true) {
      if (state.manifest !== null) throw new Error('registry manifest already committed or uncertain');
      if (state.active === null) break;
      if (waiters.length >= 16) throw new Error('registry upload backlog full');
      await new Promise((resolveWait, reject) => {
        const close = () => { if (waiters.includes(wake)) { clearTimeout(timer); waiters.splice(waiters.indexOf(wake), 1); reject(new Error('queued registry client closed')); } };
        const timer = setTimeout(() => { req.socket.off('close', close); waiters.splice(waiters.indexOf(wake), 1); reject(new Error('registry upload predecessor remains unsettled')); }, timeoutMs);
        const wake = () => { clearTimeout(timer); req.socket.off('close', close); resolveWait(); };
        waiters.push(wake);
        req.socket.once('close', close);
      });
    }
    commit(next => { next.active = { phase: 'creating', uuid: null }; }); // Before forwarding POST.
  }

  async function handle(req, res) {
    let selected;
    try {
      if (req.headers.host !== `127.0.0.1:${publicPort}`) throw new Error('registry host is not daemon loopback');
      if (req.method === 'GET' && req.url === '/luci/status') {
        framing(req, 0);
        const body = Buffer.from(JSON.stringify({ version: 1, session, repository: repo,
          upload: state.active ? state.active.phase : 'idle', activeUpload: state.active?.uuid || null,
          completedUploads: state.completed.length, manifest: state.manifest?.phase || 'none',
          manifestDigest: state.manifest?.digest || null, childReadyAtStart: true, ownershipSettled: false }) + '\n');
        res.writeHead(200, { 'content-type': 'application/json', 'content-length': body.length, 'cache-control': 'no-store' });
        res.end(body);
        return;
      }
      selected = route(req.url, repo);
      const method = req.method;
      const read = ['GET', 'HEAD'].includes(method) && ['ping', 'blob', 'manifest'].includes(selected.kind);
      const create = method === 'POST' && selected.kind === 'create';
      const part = method === 'PATCH' && selected.kind === 'upload';
      const finish = method === 'PUT' && selected.kind === 'upload' && selected.digest !== null;
      const cancel = method === 'DELETE' && selected.kind === 'upload';
      const publish = method === 'PUT' && selected.kind === 'manifest' && selected.reference === 'payload';
      if (!read && !create && !part && !finish && !cancel && !publish) throw new Error('registry method/shape refused');
      const maxBody = publish ? MAX_RESPONSE : part || finish ? MAX_BODY : 0;
      framing(req, maxBody);
      if (create) await acquire(req);
      if ((part || finish || cancel) && (!state.active || state.active.uuid !== selected.uuid)) {
        const done = finish && state.completed.find(row => row.uuid === selected.uuid && row.digest === selected.digest);
        if (done && req.headers['content-length'] === '0') {
          res.writeHead(201, { 'docker-content-digest': done.digest, location: done.location, 'content-length': '0' });
          res.end();
          return;
        }
        throw new Error('registry upload is not the exact active generation');
      }
      if (publish) {
        if (state.active || state.manifest) throw new Error('registry manifest outcome needs reconciliation');
        commit(next => { next.manifest = { phase: 'publishing', reference: 'payload' }; });
      }
      const headers = {};
      for (const [key, value] of Object.entries(req.headers)) if (!HOP.has(key) && value !== undefined) headers[key] = value;
      headers.host = `127.0.0.1:${backendPort}`;
      const upstream = httpRequest({ hostname: '127.0.0.1', port: backendPort, method, path: selected.path, headers, timeout: timeoutMs });
      let sent = false;
      upstream.on('timeout', () => upstream.destroy(new Error('registry backend timeout')));
      upstream.on('error', () => { if (!sent) refuse(res, 502, 'registry backend outcome uncertain'); });
      upstream.on('response', response => {
        try {
          let location;
          if ((create || part) && response.statusCode === 202) {
            const parsed = uploadLocation(response.headers.location, backendPort, publicPort, repo);
            if (part && parsed.uuid !== selected.uuid) throw new Error('registry upload identity changed');
            location = parsed.publicURL;
            if (create) commit(next => { next.active = { phase: 'active', uuid: parsed.uuid }; });
          } else if (create) throw new Error('registry creation was not positively acknowledged');
          if (finish && response.statusCode === 201) {
            if (response.headers['docker-content-digest'] !== selected.digest) throw new Error('registry committed different digest');
            location = committedLocation(response.headers.location, backendPort, publicPort, repo, selected.digest);
            commit(next => {
              next.completed.push({ uuid: selected.uuid, digest: selected.digest, location });
              if (next.completed.length > 4096) throw new Error('registry completed inventory exceeds bound');
              next.active = null;
            });
            release();
          }
          if (cancel && [202, 204].includes(response.statusCode)) {
            commit(next => { next.active = null; }); release();
          }
          if (publish && response.statusCode === 201) {
            const digest = response.headers['docker-content-digest'];
            if (!SHA.test(digest || '')) throw new Error('registry manifest digest absent');
            commit(next => { next.manifest = { phase: 'committed', digest }; });
          }
          if ((create || part || finish || cancel || publish) && response.statusCode >= 300) throw new Error('registry write outcome needs reconciliation');
          const reply = responseHeaders(response.headers, location);
          sent = true;
          res.writeHead(response.statusCode, reply);
          let bytes = 0;
          response.on('data', chunk => { bytes += chunk.length; if (bytes > (read && selected.kind === 'blob' ? MAX_BODY : MAX_RESPONSE)) response.destroy(new Error('registry response exceeds bound')); });
          response.on('error', () => res.destroy());
          response.pipe(res);
        } catch { response.destroy(); refuse(res, 502, 'registry outcome uncertain; owned slot retained'); }
      });
      req.on('aborted', () => upstream.destroy(new Error('registry client aborted')));
      if (create || part || finish || publish) {
        let bytes = 0;
        const limiter = new Transform({ transform(chunk, _encoding, callback) {
          bytes += chunk.length;
          callback(bytes > maxBody ? new Error('registry request exceeds bound') : null, chunk);
        }});
        pipeline(req, limiter, upstream).catch(() => { upstream.destroy(); if (!sent) refuse(res, 502, 'registry stream outcome uncertain'); });
      } else upstream.end();
    } catch (error) { refuse(res, 409, error.message); }
  }

  const server = createServer({ maxHeaderSize: 16384 }, (req, res) => { void handle(req, res); });
  server.on('checkContinue', (_req, res) => refuse(res, 417, 'registry Expect refused'));
  server.on('checkExpectation', (_req, res) => refuse(res, 417, 'registry Expect refused'));
  server.headersTimeout = 30000;
  server.requestTimeout = 0; // Upload deadline is the backend timeout, not a body-buffer timer.
  return { server, journal, repository: repo, snapshot: () => structuredClone(state) };
}

async function main() {
  const session = process.env.LUCI_REGISTRY_SESSION;
  const dataRoot = process.env.LUCI_REGISTRY_DATA;
  const backendPort = port(process.env.LUCI_REGISTRY_BACKEND_PORT);
  const frontPort = port(process.env.LUCI_REGISTRY_FRONT_PORT);
  const publicPort = port(process.env.LUCI_REGISTRY_PUBLIC_PORT);
  const binary = strictFile(process.env.LUCI_REGISTRY_BINARY);
  const license = strictFile(process.env.LUCI_REGISTRY_LICENSE);
  await verifyReleaseBinary(binary, license);
  strictDir(dataRoot);
  const config = join(dataRoot, 'luci-distribution-v3.yml');
  if (existsSync(config)) throw new Error('registry config already exists; reconcile prior generation');
  writeFileSync(config, `version: 0.1\nlog:\n  level: warn\nstorage:\n  filesystem:\n    rootdirectory: ${join(dataRoot, 'blobs')}\nhttp:\n  addr: 127.0.0.1:${backendPort}\n`, { flag: 'wx', mode: 0o600 });
  mkdirSync(join(dataRoot, 'blobs'), { mode: 0o700 });
  const child = spawn(binary, ['serve', config], { stdio: ['ignore', 'inherit', 'inherit'],
    env: { PATH: '/usr/bin:/bin', REGISTRY_STORAGE_DELETE_ENABLED: 'false', REGISTRY_STORAGE_MAINTENANCE_UPLOADPURGING_ENABLED: 'false' } });
  child.on('error', error => { process.stderr.write(`pinned registry child failed: ${error.message}\n`); process.exitCode = 1; });
  let ready = false;
  for (let attempt = 0; attempt < 50 && child.exitCode === null; attempt++) {
    try {
      await new Promise((resolve, reject) => {
        const request = httpRequest({ hostname: '127.0.0.1', port: backendPort, path: '/v2/', timeout: 500 }, response => {
          response.resume(); response.on('end', () => response.statusCode === 200 ? resolve() : reject(new Error('registry readiness refused')));
        });
        request.on('error', reject); request.end();
      });
      ready = true; break;
    } catch { await pause(100); }
  }
  if (!ready) throw new Error('pinned registry child did not become ready');
  const gate = createGate({ session, dataRoot, backendPort, frontPort, publicPort });
  await new Promise((resolveListen, reject) => gate.server.once('error', reject).listen(frontPort, '0.0.0.0', resolveListen));
  let stopping = false;
  let socketsClosed = false;
  let childResult = null;
  function terminal() {
    if (!stopping || !socketsClosed || !childResult) return;
    const state = gate.snapshot();
    if (state.active === null && state.manifest?.phase !== 'publishing') {
      durable(join(dataRoot, 'luci-serial-gate-terminal-v1.json'), { version: 1, session,
        repository: gate.repository, child: childResult, socketsClosed: true,
        upload: 'idle', manifest: state.manifest, verifiedExport: false, engineRetired: false });
    }
  }
  process.on('SIGTERM', () => {
    if (stopping) return;
    stopping = true;
    gate.server.close(() => { socketsClosed = true; terminal(); });
    child.kill('SIGTERM');
  });
  child.on('exit', (code, signal) => {
    childResult = { exitCode: code, signal };
    if (!stopping) { gate.server.close(); process.exitCode = 1; }
    else terminal();
  });
}

if (process.argv[1] && resolve(process.argv[1]) === resolve(fileURLToPath(import.meta.url))) {
  main().catch(error => { process.stderr.write(`registry gate startup refused: ${error.message}\n`); process.exitCode = 1; });
}
