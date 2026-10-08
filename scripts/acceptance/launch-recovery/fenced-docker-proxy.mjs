// Controlled acceptance transport, never a Docker emulator. A gate can delay a
// real request or lose a real response; it cannot manufacture daemon evidence.
import http from 'node:http';
import { chmod, link, lstat, mkdtemp, rmdir, unlink } from 'node:fs/promises';
import { dirname, isAbsolute, join, resolve } from 'node:path';

const CREATE = /^\/v\d+\.\d+\/containers\/create$/;
const OBJECT = /^\/v\d+\.\d+\/containers\/([a-f0-9]{64})(?:\/(start|stop|json|stats))?$/;
const INSPECT = /^\/v\d+\.\d+\/containers\/[^/]+\/json$/;
const ALLOWED_LABELS = new Set(['luci.managed', 'luci.instance', 'luci.sandbox-id', 'luci.session-id', 'luci.project-id', 'luci.user-id', 'luci.team-id', 'luci.launch-fence', 'luci.fence-intent', 'luci.fence-binding']);
const now = () => new Date().toISOString();
const problem = code => Object.assign(new Error(code), { code });
const deferred = () => { let resolvePromise; const promise = new Promise(resolve => { resolvePromise = resolve; }); return { promise, resolve: resolvePromise }; };
const cleanLabels = labels => Object.fromEntries(Object.entries(labels ?? {}).filter(([key, value]) => ALLOWED_LABELS.has(key) && typeof value === 'string' && /^[A-Za-z0-9_.:-]{1,200}$/.test(value)));
const cleanMeta = row => structuredClone(Object.fromEntries(Object.entries(row).filter(([key]) => key !== 'query')));

/**
 * identifyCreate(Buffer, metadata) must validate exact private ownership/config
 * and return 'worker' or 'fence'. Request bodies are never retained in receipts.
 * beforeCreate executes immediately before forwarding, even after a gate hold.
 * Gates are one-shot; async match receives sanitized metadata, never headers or
 * response/request bodies. waitForGate observes a captured gate; releaseGate
 * waits for its real daemon response to drain. An unforwarded cancellation is
 * explicit local failure, never Docker absence. close cancels unsent requests,
 * drains sent requests, and rejects on any uncertain forwarded mutation.
 */
export async function startFencedDockerProxy({
  socketPath, upstreamSocketPath, identifyCreate, beforeCreate,
  maxCreates = 4, maxCreatesByKind = { worker: 2, fence: 2 }, maxStarts = 2,
  maxObservations = 20_000, maxBodyBytes = 16 * 1024 * 1024,
  maxBufferedResponseBytes = 1024 * 1024, maxResponseBytes = 64 * 1024 * 1024,
  maxConcurrentRequests = 16, maxPendingBodyBytes = 32 * 1024 * 1024,
  upstreamTimeoutMs = 120_000, holdTimeoutMs = 120_000,
} = {}) {
  if (!isAbsolute(socketPath ?? '') || !isAbsolute(upstreamSocketPath ?? '') || resolve(socketPath) === resolve(upstreamSocketPath)) throw problem('distinct-absolute-sockets-required');
  if (Buffer.byteLength(socketPath) >= 108 || Buffer.byteLength(join(dirname(socketPath), '.luci-fence-proxy-XXXXXX', 'docker.sock')) >= 108) throw problem('socket-path-too-long');
  if (typeof identifyCreate !== 'function' || typeof beforeCreate !== 'function') throw problem('explicit-create-guards-required');
  for (const [name, value] of Object.entries({ maxCreates, maxStarts, maxObservations, maxBodyBytes, maxBufferedResponseBytes, maxResponseBytes, maxConcurrentRequests, maxPendingBodyBytes, upstreamTimeoutMs, holdTimeoutMs, worker: maxCreatesByKind?.worker, fence: maxCreatesByKind?.fence })) {
    if (!Number.isSafeInteger(value) || value < 1) throw problem(`invalid-${name}`);
  }
  try { await lstat(socketPath); throw problem('socket-already-exists'); } catch (cause) { if (cause.code !== 'ENOENT') throw cause; }
  if (!(await lstat(upstreamSocketPath)).isSocket()) throw problem('upstream-must-be-a-socket');

  const observations = { counts: { total: 0, create: 0, worker: 0, fence: 0, start: 0 }, requests: [], gates: [], dropped: 0, failures: [] };
  const gates = new Map(), knownContainers = new Map(), pending = new Set(), handlers = new Set(), downstreamSockets = new Set(), upstreamSockets = new Set();
  let closing = false, closePromise, uncertainMutation = false, socketIdentity, pendingBodyBytes = 0;
  const privateDirectory = await mkdtemp(join(dirname(socketPath), '.luci-fence-proxy-'));
  const privateSocket = join(privateDirectory, 'docker.sock');
  const server = http.createServer();
  server.requestTimeout = upstreamTimeoutMs;
  server.headersTimeout = Math.min(upstreamTimeoutMs, 30_000);
  server.maxConnections = 64;
  server.on('drop', () => failure('connection-limit'));
  server.on('connection', socket => { downstreamSockets.add(socket); socket.once('close', () => downstreamSockets.delete(socket)); socket.on('error', () => {}); });
  server.on('clientError', (_cause, socket) => socket.destroy());

  function failure(code, row) {
    if (!observations.failures.includes(code)) observations.failures.push(code);
    if (row) row.failure = code;
  }
  function localFailure(res, row, code, status = 503) {
    row.proxyStatus = status; row.completedAt ??= now(); failure(code, row);
    if (res.destroyed) return;
    if (res.headersSent) { res.destroy(); return; }
    const body = Buffer.from(JSON.stringify({ message: code }));
    res.writeHead(status, { 'content-type': 'application/json', 'content-length': body.length, connection: 'close' }); res.end(body);
  }
  function record(req) {
    const row = { method: req.method ?? 'GET', path: (req.url ?? '/').split('?')[0].replace(/[\x00-\x20\x7f]/g, '_').slice(0, 2048), requestedAt: now() };
    observations.counts.total++;
    if (observations.requests.length >= maxObservations) { observations.dropped++; failure('observation-limit'); return row; }
    observations.requests.push(row); return row;
  }
  function bounded(operation, code, duration = upstreamTimeoutMs) {
    let timer;
    return Promise.race([operation, new Promise((_resolve, reject) => { timer = setTimeout(() => reject(problem(code)), duration); })]).finally(() => clearTimeout(timer));
  }
  function collect(req, allocation) {
    return new Promise((resolveBody, reject) => {
      const chunks = []; let bytes = 0, settled = false;
      const timer = setTimeout(() => finish(problem('request-timeout')), upstreamTimeoutMs);
      function finish(cause) {
        if (settled) return; settled = true; clearTimeout(timer);
        req.off('data', data); req.off('end', end); req.off('aborted', aborted);
        if (cause) { chunks.length = 0; req.resume(); reject(cause); }
        else { const body = Buffer.concat(chunks, bytes); chunks.length = 0; resolveBody(body); }
      }
      function data(chunk) {
        bytes += chunk.length;
        if (bytes > maxBodyBytes) { finish(problem('request-body-limit')); return; }
        if (pendingBodyBytes + chunk.length > maxPendingBodyBytes) { finish(problem('pending-body-budget')); return; }
        allocation.bytes += chunk.length; pendingBodyBytes += chunk.length; chunks.push(chunk);
      }
      function end() { finish(req.complete ? null : problem('request-incomplete')); }
      function aborted() { finish(problem('request-incomplete')); }
      req.on('data', data); req.once('end', end); req.once('aborted', aborted); req.on('error', () => finish(problem('request-incomplete')));
    });
  }
  function armGate({ id, phase, method, pathPattern, match = () => true, decision = 'hold' }) {
    if (closing || !/^[a-zA-Z0-9_-]{1,80}$/.test(id ?? '') || gates.has(id) || gates.size >= 8) throw problem('invalid-or-duplicate-gate');
    if (!['before-forward', 'after-response'].includes(phase) || !['GET', 'HEAD', 'POST', 'DELETE'].includes(method) || typeof match !== 'function' || (pathPattern !== undefined && !(pathPattern instanceof RegExp))) throw problem('invalid-gate-matcher');
    if (!['hold', 'lose-response'].includes(decision) || (decision === 'lose-response' && phase !== 'after-response')) throw problem('invalid-gate-decision');
    const row = { id, phase, method, decision, state: 'armed', armedAt: now() };
    const gate = { row, pathPattern, match, reached: deferred(), continued: deferred(), drained: deferred() };
    gates.set(id, gate); observations.gates.push(row); return id;
  }
  async function matchGate(phase, row) {
    for (const gate of gates.values()) {
      if (gate.row.state !== 'armed' || gate.row.phase !== phase || gate.row.method !== row.method) continue;
      if (gate.pathPattern) { gate.pathPattern.lastIndex = 0; if (!gate.pathPattern.test(row.path)) continue; }
      const matches = await bounded(Promise.resolve().then(() => gate.match(cleanMeta(row))), 'gate-match-timeout', Math.min(upstreamTimeoutMs, 5000));
      if (closing) return null;
      if (!matches || gate.row.state !== 'armed') continue;
      gate.row.state = gate.row.decision === 'lose-response' ? 'lost' : 'holding';
      gate.row.capturedAt = now(); gate.row.request = cleanMeta(row); row.gateId = gate.row.id;
      gate.reached.resolve(cleanMeta(gate.row));
      if (gate.row.decision === 'lose-response') { gate.continued.resolve('lose-response'); return gate; }
      gate.timer = setTimeout(() => {
        if (gate.row.state !== 'holding') return;
        gate.row.state = 'timed-out'; gate.row.cancelledAt = now(); failure('gate-hold-timeout', row); gate.continued.resolve('cancel');
      }, holdTimeoutMs);
      return gate;
    }
    return null;
  }
  async function waitForGate(id, { timeoutMs = upstreamTimeoutMs } = {}) {
    const gate = gates.get(id); if (!gate || !Number.isSafeInteger(timeoutMs) || timeoutMs < 1) throw problem('invalid-gate-wait');
    const result = await bounded(gate.reached.promise, 'gate-wait-timeout', timeoutMs);
    if (result.error) throw problem(result.error); return structuredClone(result);
  }
  async function releaseGate(id) {
    const gate = gates.get(id); if (!gate || gate.row.state !== 'holding') throw problem('gate-not-held');
    clearTimeout(gate.timer); gate.row.state = 'released'; gate.row.releasedAt = now(); gate.continued.resolve('release');
    return structuredClone(await gate.drained.promise);
  }
  async function cancelGate(id) {
    const gate = gates.get(id); if (!gate || !['armed', 'holding'].includes(gate.row.state)) throw problem('gate-not-cancellable');
    const wasArmed = gate.row.state === 'armed';
    clearTimeout(gate.timer); gate.row.state = 'cancelled'; gate.row.cancelledAt = now(); gate.continued.resolve('cancel');
    if (wasArmed) { gate.reached.resolve({ error: 'gate-cancelled-before-match' }); gate.drained.resolve({ forwarded: false, cancelled: true }); }
    return structuredClone(await gate.drained.promise);
  }
  function mutationShape(row) {
    if (row.method === 'GET' || row.method === 'HEAD') return;
    if (row.method === 'POST' && CREATE.test(row.path)) return;
    const found = OBJECT.exec(row.path), identity = found && knownContainers.get(found[1]);
    if (!identity || !((row.method === 'POST' && ['start', 'stop'].includes(found[2])) || (row.method === 'DELETE' && found[2] === undefined))) throw problem('unknown-mutation-shape-or-identity');
    if (identity.kind === 'fence' && row.method !== 'DELETE') throw problem('fence-start-or-stop-forbidden');
    row.ref = found[1]; row.kind = identity.kind; row.labels = identity.labels; row.name = identity.name;
    if (found[2] === 'start' && ++observations.counts.start > maxStarts) throw problem('start-budget-exceeded');
  }
  async function forward(req, res, body, row) {
    const completion = deferred(); pending.add(completion.promise);
    const mutation = !['GET', 'HEAD'].includes(row.method), create = row.method === 'POST' && CREATE.test(row.path);
    let done = false, upstream;
    const timer = setTimeout(() => fail('upstream-timeout'), upstreamTimeoutMs);
    function finish() { if (done) return; done = true; clearTimeout(timer); pending.delete(completion.promise); completion.resolve(cleanMeta(row)); }
    function fail(code) {
      if (done) return; if (mutation) uncertainMutation = true;
      row.uncertainMutation = mutation; localFailure(res, row, code, 502); upstream?.destroy(); finish();
    }
    // Never connect a downstream disconnect to this request's cancellation.
    // Its uploaded mutation has an independent lifetime until fully drained.
    row.forwardedAt = now();
    upstream = http.request({ socketPath: upstreamSocketPath, path: req.url, method: req.method, headers: req.rawHeaders, agent: false }, reply => {
      row.status = reply.statusCode; row.respondedAt = now();
      const bufferResponse = create || INSPECT.test(row.path);
      const parts = []; let bytes = 0;
      if (!bufferResponse && !res.destroyed) { res.writeHead(reply.statusCode, reply.statusMessage, reply.rawHeaders); res.flushHeaders(); }
      reply.on('data', chunk => {
        if (done) return;
        bytes += chunk.length;
        if (bytes > (bufferResponse ? maxBufferedResponseBytes : maxResponseBytes)) { reply.destroy(); fail('response-body-limit'); return; }
        if (bufferResponse) parts.push(chunk);
        else if (!res.destroyed && !res.write(chunk)) { reply.pause(); res.once('drain', () => reply.resume()); }
      });
      res.once('close', () => reply.resume());
      reply.once('aborted', () => fail('upstream-response-incomplete'));
      reply.once('error', () => fail('upstream-response-error'));
      reply.once('close', () => { if (!reply.complete) fail('upstream-response-incomplete'); });
      reply.once('end', () => { void (async () => {
        if (done) return;
        if (!reply.complete) { fail('upstream-response-incomplete'); return; }
        row.completedAt = now(); row.responseBytes = bytes;
        const returned = bufferResponse ? Buffer.concat(parts, bytes) : null;
        if (create && reply.statusCode >= 200 && reply.statusCode < 300 && reply.statusCode !== 201) { fail('unexpected-create-success-status'); return; }
        if (create && reply.statusCode === 201) {
          let decoded; try { decoded = JSON.parse(returned.toString('utf8')); } catch { fail('invalid-create-identity'); return; }
          if (!/^[a-f0-9]{64}$/.test(decoded?.Id ?? '')) { fail('invalid-create-identity'); return; }
          if (knownContainers.has(decoded.Id)) { fail('duplicate-create-identity'); return; }
          row.id = decoded.Id; row.createResponseId = decoded.Id;
          knownContainers.set(row.id, { kind: row.kind, name: row.name, labels: row.labels });
        }
        // Upstream has finished. A deliberate held response is no longer an
        // uncertain daemon request and uses the separate bounded gate timer.
        clearTimeout(timer);
        let gate;
        try { gate = bufferResponse ? await matchGate('after-response', row) : null; }
        catch { failure('gate-match-failed', row); res.destroy(); finish(); return; }
        const choice = gate ? await gate.continued.promise : 'release';
        if (choice !== 'release' || closing || res.destroyed) { row.discardedAt = now(); res.destroy(); }
        else if (bufferResponse) {
          res.writeHead(reply.statusCode, reply.statusMessage, reply.rawHeaders);
          if (Object.keys(reply.trailers).length) res.addTrailers(reply.trailers);
          res.end(returned); row.deliveredAt = now();
        } else { if (!res.destroyed) { if (Object.keys(reply.trailers).length) res.addTrailers(reply.trailers); res.end(); } }
        gate?.drained.resolve(cleanMeta(row)); finish();
      })().catch(() => { failure('response-handler-failed', row); res.destroy(); finish(); }); });
    });
    upstream.once('socket', socket => { upstreamSockets.add(socket); socket.once('close', () => upstreamSockets.delete(socket)); socket.on('error', () => {}); });
    upstream.once('error', () => fail('upstream-request-error'));
    upstream.end(body); return completion.promise;
  }
  async function handle(req, res, row) {
    res.on('error', () => {}); res.once('close', () => { if (!res.writableFinished) row.clientDisconnectedAt = now(); });
    let gate; const allocation = { bytes: 0 };
    try {
      if (closing || observations.dropped > 0) throw problem(closing ? 'proxy-closing' : 'observation-limit');
      const body = await collect(req, allocation);
      if (closing) throw problem('proxy-closing');
      mutationShape(row);
      const create = row.method === 'POST' && CREATE.test(row.path);
      if (create) {
        row.ordinal = ++observations.counts.create;
        if (row.ordinal > maxCreates) throw problem('create-budget-exceeded');
        let input; try { input = JSON.parse(body.toString('utf8')); } catch { throw problem('invalid-create-json'); }
        row.name = new URL(req.url, 'http://fixture.invalid').searchParams.get('name') ?? '';
        if (!/^[A-Za-z0-9_.-]{1,200}$/.test(row.name)) throw problem('invalid-create-name');
        row.labels = cleanLabels(input?.Labels);
        row.kind = await bounded(Promise.resolve().then(() => identifyCreate(body, cleanMeta(row))), 'identify-create-timeout');
        if (!['worker', 'fence'].includes(row.kind)) throw problem('create-shape-rejected');
        if (++observations.counts[row.kind] > maxCreatesByKind[row.kind]) throw problem('create-kind-budget-exceeded');
      } else {
        const object = OBJECT.exec(row.path), known = object && knownContainers.get(object[1]);
        if (object) row.ref = object[1];
        if (known) { row.kind = known.kind; row.name = known.name; row.labels = known.labels; }
      }
      gate = await matchGate('before-forward', row);
      if (gate && await gate.continued.promise !== 'release') { row.cancelledBeforeForwardAt = now(); throw problem('gate-cancelled-before-forward'); }
      if (closing) throw problem('proxy-closing');
      if (create) await bounded(Promise.resolve().then(() => beforeCreate({ ...cleanMeta(row), body })), 'create-guard-timeout');
      if (closing) throw problem('proxy-closing');
      await forward(req, res, body, row);
    } catch (cause) {
      const code = typeof cause?.code === 'string' && /^[a-z-]{1,80}$/.test(cause.code) ? cause.code : 'request-guard-failed';
      localFailure(res, row, code, cause?.code === 'request-body-limit' ? 413 : 503);
    }
    finally { pendingBodyBytes -= allocation.bytes; gate?.drained.resolve(cleanMeta(row)); }
  }
  server.on('request', (req, res) => {
    const row = record(req);
    if (handlers.size >= maxConcurrentRequests) { req.resume(); localFailure(res, row, 'concurrent-request-limit'); return; }
    const task = handle(req, res, row);
    handlers.add(task); void task.finally(() => handlers.delete(task));
  });
  // This harness uses sandboxd's own HTTP/WebSocket endpoint. Unexpected raw
  // Docker upgrades must not bypass the request/identity/effect guard.
  server.on('upgrade', (req, socket) => { const row = record(req); failure('unexpected-upgrade', row); socket.destroy(); });

  async function unlinkOwned(path) {
    try { const info = await lstat(path); if (socketIdentity && info.isSocket() && info.dev === socketIdentity.dev && info.ino === socketIdentity.ino) await unlink(path); }
    catch (cause) { if (cause.code !== 'ENOENT') throw cause; }
  }
  try {
    await new Promise((resolveListen, reject) => { server.once('error', reject); server.listen(privateSocket, () => { server.off('error', reject); resolveListen(); }); });
    await chmod(privateSocket, 0o600); socketIdentity = await lstat(privateSocket); await link(privateSocket, socketPath);
  } catch {
    await new Promise(done => server.close(() => done()));
    await unlinkOwned(socketPath); await unlinkOwned(privateSocket); await rmdir(privateDirectory).catch(() => {});
    throw problem('private-proxy-socket-failed');
  }
  server.on('error', () => failure('proxy-listener-failed'));

  function close() {
    if (closePromise) return closePromise;
    closing = true;
    closePromise = (async () => {
      const closed = new Promise(done => server.close(() => done()));
      for (const gate of gates.values()) {
        if (['armed', 'holding'].includes(gate.row.state)) {
          clearTimeout(gate.timer); gate.row.state = 'cancelled'; gate.row.cancelledAt = now();
          gate.reached.resolve({ error: 'proxy-closed-before-gate' }); gate.continued.resolve('cancel');
        }
      }
      for (const socket of downstreamSockets) socket.destroy();
      await Promise.allSettled([...handlers]);
      await Promise.allSettled([...pending]);
      for (const socket of upstreamSockets) socket.destroy();
      await closed; await unlinkOwned(socketPath); await unlinkOwned(privateSocket); await rmdir(privateDirectory);
      observations.cleanup = { closedAt: now(), pendingRequests: pending.size, activeHandlers: handlers.size, pendingBodyBytes, uncertainMutation, drained: !uncertainMutation };
      if (uncertainMutation) throw problem('forwarded-mutation-outcome-uncertain');
      return structuredClone(observations.cleanup);
    })(); return closePromise;
  }
  return { observations, armGate, waitForGate, releaseGate, cancelGate, close };
}
