// Harness transport only. This proxy forwards a real Docker daemon; it never
// invents a successful create, starts a container, or logs request/response bodies.
import http from 'node:http';
import net from 'node:net';
import { chmod, link, lstat, mkdtemp, rmdir, unlink } from 'node:fs/promises';
import { dirname, isAbsolute, join, resolve } from 'node:path';

const CREATE = /^\/v\d+\.\d+\/containers\/create$/;
const START = /^\/v\d+\.\d+\/containers\/[^/]+\/start$/;
const LABELS = new Set(['luci.session-id', 'luci.project-id', 'luci.release-id', 'luci.user-id', 'luci.team-id']);
const now = () => new Date().toISOString();
const pathOnly = value => (value ?? '/').split('?')[0].replace(/[\x00-\x20\x7f]/g, '_').slice(0, 2048);
const error = message => new Error(message);

/**
 * Hold the first completed, successful versioned Docker container-create reply.
 * `onHeldCreate` and `waitForHeldCreate()` receive { id, labels }; labels contain
 * only bounded Luci identity labels, never environment/configuration values.
 * `releaseHeld()` is idempotent and permanently enables ordinary passthrough.
 * `close()` releases/discards the reply, drains already-forwarded creates within
 * their timeout, and removes only this proxy's own socket links.
 *
 * Optional limits exist for fake-upstream tests and explicitly bounded fixtures.
 * Observations contain no query strings, headers, bodies, labels or raw errors.
 */
export async function startDockerCreateProxy({
  socketPath,
  upstreamSocketPath,
  onHeldCreate,
  beforeCreate,
  maxBodyBytes = 16 * 1024 * 1024,
  maxCreateResponseBytes = 1024 * 1024,
  maxResponseBytes = 64 * 1024 * 1024,
  upstreamTimeoutMs = 30_000,
  maxObservations = 1000,
  maxCreates = 2,
  maxStarts = 2,
} = {}) {
  if (!isAbsolute(socketPath ?? '') || !isAbsolute(upstreamSocketPath ?? '') || resolve(socketPath) === resolve(upstreamSocketPath)) {
    throw error('Use distinct absolute proxy and upstream socket paths.');
  }
  // Linux sockaddr_un includes a terminating NUL. Refuse before Node can
  // silently truncate the private listener path; no request has been forwarded.
  if (Buffer.byteLength(socketPath) >= 108 || Buffer.byteLength(join(dirname(socketPath), '.luci-docker-proxy-XXXXXX', 'docker.sock')) >= 108) {
    throw error('The proxy socket path is too long; use a shorter private fixture root.');
  }
  if (beforeCreate !== undefined && typeof beforeCreate !== 'function') throw error('beforeCreate must be a function.');
  if (onHeldCreate !== undefined && typeof onHeldCreate !== 'function') throw error('onHeldCreate must be a function.');
  for (const [name, value] of Object.entries({ maxBodyBytes, maxCreateResponseBytes, maxResponseBytes, upstreamTimeoutMs, maxObservations, maxCreates, maxStarts })) {
    if (!Number.isSafeInteger(value) || value < 1) throw error(`Invalid ${name}.`);
  }
  try { await lstat(socketPath); throw error('The requested proxy socket path already exists.'); }
  catch (cause) { if (cause.code !== 'ENOENT') throw cause; }

  const observations = { counts: { create: 0, start: 0, total: 0 }, requests: [], dropped: 0 };
  let released = false, held = null, heldIdentity = null, holdFailure = null, callbackFailure = null, closing = false, closePromise;
  const waiters = new Set(), incomingSockets = new Set(), outgoingSockets = new Set(), pending = new Set(), callbacks = new Set();
  let uncertainCreate = false, guardedCreateInFlight = false;
  const privateDirectory = await mkdtemp(join(dirname(socketPath), '.luci-docker-proxy-'));
  const privateSocket = join(privateDirectory, 'docker.sock');
  let socketIdentity;
  const server = http.createServer();
  server.requestTimeout = upstreamTimeoutMs;
  server.headersTimeout = upstreamTimeoutMs;
  server.on('connection', socket => {
    incomingSockets.add(socket);
    socket.on('close', () => incomingSockets.delete(socket));
    socket.on('error', () => {});
  });
  server.on('clientError', (_cause, socket) => socket.destroy());

  function record(req) {
    const row = { method: req.method ?? 'GET', path: pathOnly(req.url), requestedAt: now() };
    observations.counts.total++;
    if (row.method === 'POST' && CREATE.test(row.path)) observations.counts.create++;
    if (row.method === 'POST' && START.test(row.path)) observations.counts.start++;
    if (observations.requests.length < maxObservations) observations.requests.push(row);
    else observations.dropped++;
    return row;
  }
  function settleWaiters() {
    for (const waiter of waiters) {
      clearTimeout(waiter.timer);
      if (heldIdentity) waiter.resolve(structuredClone(heldIdentity));
      else waiter.reject(holdFailure ?? error('Proxy closed or released before a successful create was held.'));
    }
    waiters.clear();
  }
  function failHold(message) {
    if (!heldIdentity && !holdFailure && !released) {
      holdFailure = error(message);
      // Once a waiter fails, never trap a later real response without an
      // observer. A new proxy is required to arm another interruption attempt.
      released = true;
      settleWaiters();
    }
  }
  function waitForHeldCreate({ timeoutMs = upstreamTimeoutMs + 1000 } = {}) {
    if (!Number.isSafeInteger(timeoutMs) || timeoutMs < 1) return Promise.reject(error('Invalid wait timeout.'));
    if (heldIdentity) return Promise.resolve(structuredClone(heldIdentity));
    if (holdFailure || closing || released) return Promise.reject(holdFailure ?? error('No create response remains eligible to be held.'));
    return new Promise((resolveWaiter, reject) => {
      const waiter = { resolve: resolveWaiter, reject, timer: null };
      waiter.timer = setTimeout(() => { waiters.delete(waiter); reject(error('Timed out waiting for a completed successful create.')); }, timeoutMs);
      waiters.add(waiter);
    });
  }
  function deliverHeld() {
    if (!held) return;
    const { response, status, statusMessage, headers, trailers, body, row } = held;
    held = null;
    if (response.destroyed || !response.writable || closing) {
      row.discardedAt = now();
      response.destroy();
      return;
    }
    response.writeHead(status, statusMessage, headers);
    if (Object.keys(trailers).length) response.addTrailers(trailers);
    response.end(body);
    row.releasedAt = now();
  }
  function releaseHeld() {
    released = true;
    deliverHeld();
    if (!heldIdentity) settleWaiters();
  }
  function localFailure(res, row, status, code) {
    row.proxyStatus = status;
    row.completedAt ??= now();
    if (res.destroyed) return;
    if (res.headersSent) { res.destroy(); return; }
    const body = Buffer.from(JSON.stringify({ message: code }));
    res.writeHead(status, { 'content-type': 'application/json', 'content-length': body.length, connection: 'close' });
    res.end(body);
  }
  function identityFrom(body, returned) {
    let input, output;
    try { input = JSON.parse(body.toString('utf8')); output = JSON.parse(returned.toString('utf8')); }
    catch { throw error('A successful create did not return a valid identity.'); }
    if (!output || typeof output.Id !== 'string' || !/^[a-f0-9]{12,64}$/i.test(output.Id)) throw error('A successful create did not return a bounded container ID.');
    const labels = {};
    if (input?.Labels && typeof input.Labels === 'object' && !Array.isArray(input.Labels)) {
      for (const name of LABELS) {
        const value = input.Labels[name];
        if (typeof value === 'string' && /^[A-Za-z0-9_.:-]{1,200}$/.test(value)) labels[name] = value;
      }
    }
    return { id: output.Id, labels };
  }
  function notifyIdentity(identity, row) {
    if (!onHeldCreate) return;
    let timer;
    const callback = Promise.race([
      Promise.resolve().then(() => onHeldCreate(structuredClone(identity))),
      new Promise((_resolve, reject) => { timer = setTimeout(() => reject(error('callback-timeout')), upstreamTimeoutMs); }),
    ]).catch(() => {
      callbackFailure = error('onHeldCreate failed; the actual container identity remains recorded.');
      row.callbackFailedAt = now();
    }).finally(() => { clearTimeout(timer); callbacks.delete(callback); });
    callbacks.add(callback);
  }

  function collect(req) {
    return new Promise((resolveBody, reject) => {
      const parts = []; let size = 0, settled = false;
      function finish(cause) {
        if (settled) return;
        settled = true;
        req.off('data', data); req.off('end', end); req.off('aborted', aborted);
        if (cause) { parts.length = 0; req.resume(); reject(cause); }
        else resolveBody(Buffer.concat(parts, size));
      }
      function data(chunk) { size += chunk.length; if (size > maxBodyBytes) finish(error('request-body-limit')); else parts.push(chunk); }
      function end() { finish(req.complete ? null : error('incomplete-request')); }
      function aborted() { finish(error('incomplete-request')); }
      req.on('data', data); req.once('end', end); req.once('aborted', aborted);
      // Retain an error listener after an oversized request is rejected.
      req.on('error', () => finish(error('incomplete-request')));
    });
  }
  function trackSocket(socket) {
    outgoingSockets.add(socket);
    socket.on('close', () => outgoingSockets.delete(socket));
    socket.on('error', () => {});
  }
  function forward(req, res, requestBody, row) {
    let finish;
    const finished = new Promise(resolveDone => { finish = resolveDone; });
    pending.add(finished);
    let done = false, upstream;
    const create = req.method === 'POST' && CREATE.test(row.path);
    const timer = setTimeout(() => {
      upstream?.destroy(error('upstream-timeout'));
      failed('upstream-timeout', 504);
    }, upstreamTimeoutMs);
    function complete() {
      if (done) return;
      done = true; clearTimeout(timer); pending.delete(finished); finish();
    }
    function failed(code, status = 502) {
      if (done) return;
      if (create) {
        uncertainCreate = true;
        failHold('The forwarded create failed before a complete successful response could be held.');
      }
      localFailure(res, row, status, code);
      upstream?.destroy();
      complete();
    }
    // Deliberately do not wire downstream close/abort to upstream.destroy(). A
    // fully uploaded create must finish even after its requesting worker dies.
    upstream = http.request({ socketPath: upstreamSocketPath, method: req.method, path: req.url, headers: req.rawHeaders, agent: false }, reply => {
      row.status = reply.statusCode;
      row.respondedAt = now();
      const shouldBuffer = create && reply.statusCode >= 200 && reply.statusCode < 300;
      const parts = []; let size = 0;
      if (!shouldBuffer && !res.destroyed) {
        res.writeHead(reply.statusCode, reply.statusMessage, reply.rawHeaders);
        res.flushHeaders();
      }
      reply.on('data', chunk => {
        if (done) return;
        size += chunk.length;
        if (size > (shouldBuffer ? maxCreateResponseBytes : maxResponseBytes)) { reply.destroy(); failed('response-body-limit'); return; }
        if (shouldBuffer) parts.push(chunk);
        else if (!res.destroyed && !res.write(chunk)) { reply.pause(); res.once('drain', () => reply.resume()); }
      });
      // A disconnected downstream must not leave a paused upstream unread.
      res.once('close', () => reply.resume());
      reply.once('aborted', () => failed('upstream-response-incomplete'));
      reply.once('error', () => failed('upstream-response-error'));
      reply.once('end', () => {
        if (done) return;
        if (!reply.complete) { failed('upstream-response-incomplete'); return; }
        row.completedAt = now();
        if (shouldBuffer) {
          const responseBody = Buffer.concat(parts, size);
          let identity;
          try { identity = identityFrom(requestBody, responseBody); }
          catch {
            // Preserve the real daemon response even if its identity is unusable.
            uncertainCreate = true;
            failHold('A completed successful create had an unusable identity.');
          }
          if (identity) row.id = identity.id;
          const firstIdentity = identity && !heldIdentity && !holdFailure;
          if (firstIdentity) {
            heldIdentity = identity;
            notifyIdentity(identity, row);
          }
          if (firstIdentity && !released && !closing) {
            held = { response: res, status: reply.statusCode, statusMessage: reply.statusMessage, headers: reply.rawHeaders, trailers: reply.trailers, body: responseBody, row };
            row.heldAt = now();
            settleWaiters();
          } else if (!res.destroyed && !closing) {
            res.writeHead(reply.statusCode, reply.statusMessage, reply.rawHeaders);
            if (Object.keys(reply.trailers).length) res.addTrailers(reply.trailers);
            res.end(responseBody);
          } else row.discardedAt = now();
        } else if (!res.destroyed) {
          if (Object.keys(reply.trailers).length) res.addTrailers(reply.trailers);
          res.end();
        }
        complete();
      });
      reply.once('close', () => { if (!reply.complete) failed('upstream-response-incomplete'); });
    });
    upstream.once('socket', trackSocket);
    upstream.once('error', () => failed('upstream-request-error'));
    upstream.end(requestBody);
    return finished;
  }
  server.on('request', async (req, res) => {
    const row = record(req);
    res.on('error', () => {});
    res.once('close', () => { if (!res.writableFinished) row.clientDisconnectedAt = now(); });
    if (closing) { localFailure(res, row, 503, 'proxy-closing'); return; }
    // Lifecycle acceptance hard budget: refuse excess requests before the daemon.
    // This is an explicit harness failure, never an invented successful response.
    if (req.method === 'POST' && ((CREATE.test(row.path) && observations.counts.create > maxCreates) || (START.test(row.path) && observations.counts.start > maxStarts))) {
      row.lifecycleBudgetRejected = true; req.resume(); localFailure(res, row, 503, 'lifecycle-allocation-budget'); return;
    }
    try {
      const body = await collect(req);
      if (closing) { localFailure(res, row, 503, 'proxy-closing'); return; }
      if (req.method === 'POST' && CREATE.test(row.path) && beforeCreate) {
        if (guardedCreateInFlight) { row.allocationGuardRejected = true; localFailure(res, row, 503, 'one-create-at-a-time'); return; }
        guardedCreateInFlight = true;
        try {
          try { await beforeCreate({ ordinal: observations.counts.create, body }); }
          catch { row.allocationGuardRejected = true; localFailure(res, row, 503, 'allocation-guard-refused'); return; }
          if (closing) { localFailure(res, row, 503, 'proxy-closing'); return; }
          await forward(req, res, body, row);
        } finally { guardedCreateInFlight = false; }
      } else await forward(req, res, body, row);
    }
    catch (cause) {
      localFailure(res, row, cause.message === 'request-body-limit' ? 413 : 400, 'request-body-unavailable');
      if (req.method === 'POST' && CREATE.test(row.path)) failHold('The create request was not forwarded because its upload was incomplete or oversized.');
    }
  });

  // Docker attach/exec may switch to a raw TCP stream. Preserve the actual
  // upgrade, including any bytes already read by either HTTP parser.
  server.on('upgrade', (req, downstream, head) => {
    const row = record(req);
    const length = Number(req.headers['content-length'] ?? 0);
    if (closing || head.length > maxBodyBytes || !Number.isSafeInteger(length) || length < 0 || length > maxBodyBytes || req.headers['transfer-encoding']) {
      downstream.destroy(); return;
    }
    // A body-bearing exec-start must reach Docker before Docker can upgrade.
    // Tunnel its original HTTP bytes immediately, then preserve the raw stream.
    const upstream = net.createConnection({ path: upstreamSocketPath });
    trackSocket(upstream);
    const timer = setTimeout(() => { upstream.destroy(); downstream.destroy(); }, upstreamTimeoutMs);
    let responseHeaders = Buffer.alloc(0), observed = false;
    upstream.on('data', bytes => {
      if (observed) return;
      const end = bytes.indexOf('\r\n\r\n');
      responseHeaders = Buffer.concat([responseHeaders, end >= 0 ? bytes.subarray(0, end + 4) : bytes]);
      if (responseHeaders.length > 64 * 1024) { upstream.destroy(); downstream.destroy(); return; }
      if (responseHeaders.includes('\r\n\r\n')) {
        const status = /^HTTP\/\d\.\d (\d{3})(?: |\r)/.exec(responseHeaders.toString('latin1'));
        if (status) { row.status = Number(status[1]); row.respondedAt = now(); }
        observed = true; responseHeaders = Buffer.alloc(0); clearTimeout(timer);
      }
    });
    upstream.once('connect', () => {
      const headers = req.rawHeaders.reduce((lines, value, i, all) => i % 2 ? lines : lines + `${value}: ${all[i + 1]}\r\n`, '');
      upstream.write(`${req.method} ${req.url} HTTP/${req.httpVersion}\r\n${headers}\r\n`);
      if (head.length) upstream.write(head);
      downstream.pipe(upstream); upstream.pipe(downstream);
    });
    upstream.once('error', () => downstream.destroy());
    upstream.once('close', () => { clearTimeout(timer); row.completedAt = now(); downstream.destroy(); });
    downstream.once('close', () => { row.clientDisconnectedAt = now(); upstream.destroy(); });
  });

  async function unlinkIfOwned(path) {
    try {
      const info = await lstat(path);
      if (socketIdentity && info.isSocket() && info.dev === socketIdentity.dev && info.ino === socketIdentity.ino) await unlink(path);
    } catch (cause) { if (cause.code !== 'ENOENT') throw cause; }
  }
  try {
    await new Promise((resolveListen, reject) => {
      server.once('error', reject);
      server.listen(privateSocket, () => { server.off('error', reject); resolveListen(); });
    });
    await chmod(privateSocket, 0o600);
    socketIdentity = await lstat(privateSocket);
    // Node unlinks its listen path on close. Bind a private path and hard-link
    // the public socket so Node can never unlink a replacement public path.
    await link(privateSocket, socketPath);
  } catch (cause) {
    await new Promise(done => server.close(() => done()));
    await unlinkIfOwned(socketPath);
    await unlinkIfOwned(privateSocket);
    await rmdir(privateDirectory).catch(() => {});
    throw error('Could not create the private proxy socket.');
  }
  server.on('error', () => failHold('The proxy listener failed.'));

  function close() {
    if (closePromise) return closePromise;
    closing = true;
    releaseHeld();
    closePromise = (async () => {
      const closed = new Promise(done => server.close(() => done()));
      for (const socket of incomingSockets) socket.destroy();
      // A daemon create may already exist even if its worker disappeared. Wait
      // for each already-forwarded real request before closing upstream sockets.
      await Promise.allSettled([...pending]);
      await Promise.allSettled([...callbacks]);
      for (const socket of outgoingSockets) socket.destroy();
      await closed;
      await unlinkIfOwned(socketPath);
      await unlinkIfOwned(privateSocket);
      await rmdir(privateDirectory);
      if (callbackFailure) throw callbackFailure;
      if (uncertainCreate) throw error('A forwarded create outcome is uncertain. Inspect its owned daemon resources before declaring cleanup complete.');
    })();
    return closePromise;
  }
  return { close, releaseHeld, observations, waitForHeldCreate };
}
