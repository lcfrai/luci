// Controlled acceptance transport, never a Docker emulator. A gate can delay a
// real request or lose a real response; it cannot manufacture daemon evidence.
import http from 'node:http';
import net from 'node:net';
import { chmod, link, lstat, mkdtemp, rmdir, unlink } from 'node:fs/promises';
import { dirname, isAbsolute, join, resolve } from 'node:path';

const CREATE = /^\/v\d+\.\d+\/containers\/create$/;
const OBJECT = /^\/v\d+\.\d+\/containers\/([a-f0-9]{64})(?:\/(start|stop|json|stats))?$/;
const INSPECT = /^\/v\d+\.\d+\/containers\/[^/]+\/json$/;
const CHECK_OBJECT = /^\/v\d+\.\d+\/containers\/([a-f0-9]{64}|luci-check-check-[a-f0-9]{24}-0)(?:\/(start|stop|attach|wait))?$/;
const SDK_CHECK_OBJECT = /^\/v\d+\.\d+\/containers\/([a-f0-9]{64}|luci-check-checkexec-[a-f0-9]{24})(?:\/(start|stop|attach|wait|json|logs|stats))?$/;
const SDK_CHECK_LABELS = ['luci.check','luci.check-protocol','luci.instance','luci.check-engine','luci.check-allocation','luci.check-id','luci.check-command-index','luci.check-scope','luci.check-project','luci.check-command-digest','luci.check-source-digest','luci.check-image'];
const ALLOWED_LABELS = new Set(['luci.managed', 'luci.instance', 'luci.sandbox-id', 'luci.session-id', 'luci.project-id', 'luci.user-id', 'luci.team-id', 'luci.launch-fence', 'luci.fence-intent', 'luci.fence-binding']);
const now = () => new Date().toISOString();
const problem = code => Object.assign(new Error(code), { code });
const deferred = () => { let resolvePromise; const promise = new Promise(resolve => { resolvePromise = resolve; }); return { promise, resolve: resolvePromise }; };
const cleanLabels = labels => Object.fromEntries(Object.entries(labels ?? {}).filter(([key, value]) => ALLOWED_LABELS.has(key) && typeof value === 'string' && /^[A-Za-z0-9_.:-]{1,200}$/.test(value)));
const cleanMeta = row => structuredClone(Object.fromEntries(Object.entries(row).filter(([key]) => key !== 'query')));

/**
 * identifyCreate(Buffer, metadata) must validate exact private ownership/config
 * and return 'worker' or 'fence' (or 'check' only with explicit check opt-in).
 * Request bodies are never retained in receipts.
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
  allowCheckTraffic = false, journal = null, checkTrafficProtocol = 'cli-v1',
} = {}) {
  if (!['cli-v1','sdk-ledger-v1'].includes(checkTrafficProtocol) || (checkTrafficProtocol !== 'cli-v1' && !allowCheckTraffic)) throw problem('explicit-check-protocol-required');
  const sdkChecks = checkTrafficProtocol === 'sdk-ledger-v1';
  if (journal !== null && typeof journal?.record !== 'function') throw problem('invalid-journal');
  if (!isAbsolute(socketPath ?? '') || !isAbsolute(upstreamSocketPath ?? '') || resolve(socketPath) === resolve(upstreamSocketPath)) throw problem('distinct-absolute-sockets-required');
  if (Buffer.byteLength(socketPath) >= 108 || Buffer.byteLength(join(dirname(socketPath), '.luci-fence-proxy-XXXXXX', 'docker.sock')) >= 108) throw problem('socket-path-too-long');
  if (typeof identifyCreate !== 'function' || typeof beforeCreate !== 'function') throw problem('explicit-create-guards-required');
  for (const [name, value] of Object.entries({ maxCreates, maxStarts, maxObservations, maxBodyBytes, maxBufferedResponseBytes, maxResponseBytes, maxConcurrentRequests, maxPendingBodyBytes, upstreamTimeoutMs, holdTimeoutMs, worker: maxCreatesByKind?.worker, fence: maxCreatesByKind?.fence })) {
    if (!Number.isSafeInteger(value) || value < 1) throw problem(`invalid-${name}`);
  }
  if (typeof allowCheckTraffic !== 'boolean' || (allowCheckTraffic && maxCreatesByKind?.check !== 1) || (!allowCheckTraffic && maxCreatesByKind?.check !== undefined)) throw problem('explicit-single-check-budget-required');
  try { await lstat(socketPath); throw problem('socket-already-exists'); } catch (cause) { if (cause.code !== 'ENOENT') throw cause; }
  if (!(await lstat(upstreamSocketPath)).isSocket()) throw problem('upstream-must-be-a-socket');

  const observations = { counts: { total: 0, create: 0, worker: 0, fence: 0, start: 0 }, requests: [], gates: [], dropped: 0, failures: [] };
  if (allowCheckTraffic) observations.counts.check = 0;
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

  let journalFailed = false;
  function persist(event, row = {}, required = false) {
    if (!journal) return;
    try {journal.record(event, row);}
    catch {journalFailed = true; failure('journal-write-failed', row); if (required) throw problem('journal-write-failed');}
  }
  function failure(code, row) {
    if (!observations.failures.includes(code)) observations.failures.push(code);
    if (row) row.failure = code;
  }
  function localFailure(res, row, code, status = 503) {
    row.proxyStatus = status; row.completedAt ??= now(); failure(code, row);
    if (!['GET', 'HEAD'].includes(row.method)) persist('request-failed', {...row, code});
    if (res.destroyed) return;
    if (res.headersSent) { res.destroy(); return; }
    const body = Buffer.from(JSON.stringify({ message: code }));
    res.writeHead(status, { 'content-type': 'application/json', 'content-length': body.length, connection: 'close' }); res.end(body);
  }
  function record(req) {
    const row = { method: req.method ?? 'GET', path: (req.url ?? '/').split('?')[0].replace(/[\x00-\x20\x7f]/g, '_').slice(0, 2048), requestedAt: now() };
    observations.counts.total++; row.requestSequence = observations.counts.total;
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
  function checkObject(row) {
    if (!allowCheckTraffic) return null;
    const found = (sdkChecks ? SDK_CHECK_OBJECT : CHECK_OBJECT).exec(row.path);
    if (!found) return null;
    let id = found[1], identity = knownContainers.get(id);
    if (!identity && id.startsWith('luci-check-')) {
      const matched = [...knownContainers].find(([, value]) => value.kind === 'check' && value.name === id);
      if (matched) [id, identity] = matched;
    }
    return identity?.kind === 'check' ? { id, identity, supplied: found[1], action: found[2] } : null;
  }
  function queryShape(req, allowed, required = {}) {
    const query = new URL(req.url, 'http://fixture.invalid').searchParams;
    const keys = [...query.keys()];
    if (new Set(keys).size !== keys.length || keys.some(key => !allowed[key]?.includes(query.get(key))) || Object.entries(required).some(([key, values]) => !values.includes(query.get(key)))) throw problem('check-query-shape-refused');
  }
  function checkTraffic(req, row, upgrade = false) {
    const check = checkObject(row);
    if (!check) throw problem('unknown-check-identity');
    row.ref = check.id; row.kind = 'check'; row.name = check.identity.name; row.labels = check.identity.labels;
    if (sdkChecks) {
      if (upgrade || check.supplied !== check.id) throw problem('sdk-check-immutable-id-required');
      if (row.method === 'GET' && check.action === 'json') queryShape(req, {});
      else if (row.method === 'GET' && check.action === 'logs') queryShape(req, {stdout:['1'],stderr:['1'],tail:['all']}, {stdout:['1'],stderr:['1'],tail:['all']});
      else if (row.method === 'POST' && check.action === 'start') {
        queryShape(req, {});
        if (++check.identity.startCount > 1 || ++observations.counts.start > maxStarts) throw problem('start-budget-exceeded');
      } else if (row.method === 'DELETE' && !check.action) queryShape(req, {v:['1'],force:['1']}, {v:['1']});
      else throw problem('sdk-check-request-shape-refused');
      return check;
    }
    if (row.method === 'POST' && check.action === 'attach') {
      if (!upgrade || check.supplied !== check.id || ++check.identity.attachCount > 1) throw problem('check-attach-shape-refused');
      queryShape(req, { logs: ['0','1','false','true'], stream: ['1','true'], stdin: ['0','false'], stdout: ['1','true'], stderr: ['1','true'] }, { stream: ['1','true'], stdout: ['1','true'], stderr: ['1','true'] });
    } else if (!upgrade && row.method === 'POST' && check.action === 'wait') {
      if (check.supplied !== check.id || ++check.identity.waitCount > 1) throw problem('check-wait-shape-refused');
      queryShape(req, { condition: ['not-running','next-exit','removed'] });
    } else if (!upgrade && row.method === 'DELETE' && !check.action) {
      queryShape(req, { force: ['1','true'], v: ['0','false'], link: ['0','false'] });
    } else if (!upgrade && row.method === 'POST' && check.action === 'start' && check.supplied === check.id) {
      queryShape(req, {});
      if (++check.identity.startCount > 1 || ++observations.counts.start > maxStarts) throw problem('start-budget-exceeded');
    } else throw problem('check-mutation-shape-refused');
    return check;
  }
  function mutationShape(req, row) {
    if (sdkChecks) {
      if (checkObject(row)) {checkTraffic(req,row);return;}
      const sdkObject=SDK_CHECK_OBJECT.exec(row.path), knownWorker=sdkObject&&knownContainers.get(sdkObject[1])?.kind==='worker';
      if (/^\/v\d+\.\d+\/containers\/luci-check-/.test(row.path) || (!knownWorker && /^\/v\d+\.\d+\/containers\/[^/]+\/logs$/.test(row.path))) throw problem('unknown-sdk-check-identity');
    }
    if (row.method === 'GET' || row.method === 'HEAD') return;
    if (row.method === 'POST' && CREATE.test(row.path)) return;
    if (checkObject(row)) { checkTraffic(req, row); return; }
    const found = OBJECT.exec(row.path), identity = found && knownContainers.get(found[1]);
    if (!identity || !((row.method === 'POST' && ['start', 'stop'].includes(found[2])) || (row.method === 'DELETE' && found[2] === undefined))) throw problem('unknown-mutation-shape-or-identity');
    if (identity.kind === 'fence' && row.method !== 'DELETE') throw problem('fence-start-or-stop-forbidden');
    row.ref = found[1]; row.kind = identity.kind; row.labels = identity.labels; row.name = identity.name;
    if (found[2] === 'start' && ++observations.counts.start > maxStarts) throw problem('start-budget-exceeded');
  }
  async function forward(req, res, body, row) {
    const mutation = !['GET', 'HEAD'].includes(row.method), create = row.method === 'POST' && CREATE.test(row.path);
    // Intent is not proof of sending or receipt. Durability must precede send.
    if (mutation) {if (journalFailed) throw problem('journal-write-failed'); persist('mutation-forward-intent', row, true);}
    const completion = deferred(); pending.add(completion.promise);
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
          knownContainers.set(row.id, { kind: row.kind, name: row.name, labels: row.labels, attachCount: 0, waitCount: 0, startCount: 0 });
          // Only the fully drained real 201 body establishes this ACK.
          persist('create-ack', row, true);
        }
        if (mutation && !(create && reply.statusCode === 201)) persist('mutation-response-drained', row, true);
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
          // This records local end(), not downstream receipt acknowledgement.
          if (mutation) persist('response-end-issued', row);
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
      if (journalFailed && !['GET', 'HEAD'].includes(row.method)) throw problem('journal-write-failed');
      if (closing || observations.dropped > 0) throw problem(closing ? 'proxy-closing' : 'observation-limit');
      const body = await collect(req, allocation);
      if (closing) throw problem('proxy-closing');
      mutationShape(req, row);
      if (row.kind === 'check' && body.length !== 0) throw problem('check-lifecycle-body-refused');
      const create = row.method === 'POST' && CREATE.test(row.path);
      if (create) {
        row.ordinal = ++observations.counts.create;
        if (row.ordinal > maxCreates) throw problem('create-budget-exceeded');
        let input; try { input = JSON.parse(body.toString('utf8')); } catch { throw problem('invalid-create-json'); }
        row.name = new URL(req.url, 'http://fixture.invalid').searchParams.get('name') ?? '';
        if (!/^[A-Za-z0-9_.-]{1,200}$/.test(row.name)) throw problem('invalid-create-name');
        row.labels = cleanLabels(input?.Labels);
        row.kind = await bounded(Promise.resolve().then(() => identifyCreate(body, cleanMeta(row))), 'identify-create-timeout');
        if (!['worker', 'fence', ...(allowCheckTraffic ? ['check'] : [])].includes(row.kind)) throw problem('create-shape-rejected');
        if (row.kind === 'check') {
          if (sdkChecks) {
            const labels=input.Labels??{};
            if (Object.keys(labels).length!==SDK_CHECK_LABELS.length || SDK_CHECK_LABELS.some(key=>typeof labels[key]!=='string') || labels['luci.check']!=='true' || labels['luci.check-protocol']!=='1' || labels['luci.check-command-index']!=='0' || !/^checkexec-[a-f0-9]{24}$/.test(labels['luci.check-allocation']??'') || !/^check-[a-f0-9]{24}$/.test(labels['luci.check-id']??'') || row.name!==`luci-check-${labels['luci.check-allocation']}` || [...knownContainers.values()].some(value=>value.name===row.name)) throw problem('sdk-check-create-identity-refused');
            if (!['luci.check-scope','luci.check-command-digest','luci.check-source-digest'].every(key=>/^[a-f0-9]{64}$/.test(labels[key])) || !/^sha256:[a-f0-9]{64}$/.test(labels['luci.check-image']) || !['luci.instance','luci.check-engine','luci.check-project'].every(key=>/^[A-Za-z0-9_.:-]{1,200}$/.test(labels[key]))) throw problem('sdk-check-create-labels-refused');
            row.labels=Object.fromEntries(SDK_CHECK_LABELS.map(key=>[key,labels[key]]));
          } else {
            if (!/^luci-check-check-[a-f0-9]{24}-0$/.test(row.name) || input?.Labels?.['luci.check'] !== 'true' || row.name !== `luci-check-${input?.Labels?.['luci.check-id']}-0` || [...knownContainers.values()].some(value => value.name === row.name)) throw problem('check-create-identity-refused');
            row.labels = { 'luci.check': 'true', 'luci.check-id': input.Labels['luci.check-id'] };
          }
        }
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
      if (row.kind === 'check' && row.method === 'DELETE' && checkObject(row)?.supplied !== row.ref) await verifyNamedCheck(row);
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
  // The optional check uses Docker CLI's read-only output attach. Agent/fence
  // upgrades remain refused; no command/exec/stdin tunnel is admitted.
  server.on('upgrade', (req, socket, head) => {
    const row = record(req);
    if (!allowCheckTraffic || sdkChecks) { failure(sdkChecks ? 'sdk-check-upgrade-forbidden' : 'unexpected-upgrade', row); socket.destroy(); return; }
    const task = checkUpgrade(req, socket, head, row);
    handlers.add(task); void task.finally(() => handlers.delete(task));
  });

  async function verifyNamedCheck(row) {
    // Preserve Docker CLI's original name request. This read rejects a swapped
    // name; it is not an atomic name lock. The acceptance window permits no
    // external Docker allocator, and this proxy permits only one check Create.
    const path = row.path + '/json', observed = record({ method: 'GET', url: path });
    observed.internalObservation = 'check-name-identity'; observed.ref = row.ref;
    if (observations.dropped) throw problem('observation-limit');
    await new Promise((resolveProof, reject) => {
      let settled = false, request;
      const timer = setTimeout(() => finish(problem('check-name-inspect-timeout')), upstreamTimeoutMs);
      function finish(cause) { if (settled) return; settled = true; clearTimeout(timer); if (cause) { request?.destroy(); reject(cause); } else resolveProof(); }
      observed.forwardedAt = now();
      request = http.request({ socketPath: upstreamSocketPath, path, method: 'GET', agent: false }, reply => {
        observed.status = reply.statusCode; observed.respondedAt = now();
        const chunks = []; let bytes = 0;
        reply.on('data', chunk => { bytes += chunk.length; if (bytes > maxBufferedResponseBytes) { reply.destroy(); finish(problem('check-name-inspect-body-limit')); } else chunks.push(chunk); });
        reply.once('error', () => finish(problem('check-name-inspect-error')));
        reply.once('aborted', () => finish(problem('check-name-inspect-truncated')));
        reply.once('end', () => {
          observed.completedAt = now(); observed.responseBytes = bytes;
          if (!reply.complete) { finish(problem('check-name-inspect-truncated')); return; }
          if (reply.statusCode === 404) { observed.identity = 'absent'; finish(); return; }
          let value; try { value = JSON.parse(Buffer.concat(chunks).toString('utf8')); } catch { finish(problem('check-name-inspect-invalid')); return; }
          if (reply.statusCode !== 200 || value.Id !== row.ref || value.Name !== '/' + row.name || value.Config?.Labels?.['luci.check'] !== 'true' || value.Config?.Labels?.['luci.check-id'] !== row.labels['luci.check-id']) { finish(problem('check-name-identity-changed')); return; }
          observed.identity = 'exact'; finish();
        });
      });
      request.once('socket', socket => { upstreamSockets.add(socket); socket.once('close', () => upstreamSockets.delete(socket)); socket.on('error', () => {}); });
      request.once('error', () => finish(problem('check-name-inspect-error'))); request.end();
    });
  }

  async function checkUpgrade(req, downstream, head, row) {
    try {
      if (closing || observations.dropped || handlers.size >= maxConcurrentRequests) throw problem('check-upgrade-unavailable');
      const length = Number(req.headers['content-length'] ?? 0);
      if (length !== 0 || head.length || req.headers['transfer-encoding'] || req.headers.upgrade?.toLowerCase() !== 'tcp' || !String(req.headers.connection ?? '').toLowerCase().split(',').map(value => value.trim()).includes('upgrade')) throw problem('check-upgrade-shape-refused');
      checkTraffic(req, row, true);
      await new Promise(resolveDrain => {
        let finished = false, header = Buffer.alloc(0), headerComplete = false, bytes = 0, upstreamEnded = false;
        const upstream = net.createConnection({ path: upstreamSocketPath, allowHalfOpen: true });
        upstreamSockets.add(upstream); upstream.once('close', () => upstreamSockets.delete(upstream));
        const timer = setTimeout(() => finish('check-attach-timeout'), upstreamTimeoutMs);
        function finish(code) {
          if (finished) return; finished = true; clearTimeout(timer);
          if (code) failure(code, row);
          row.responseBytes = bytes; row.completedAt = now(); upstream.destroy();
          if (code) downstream.destroy(); else if (!downstream.destroyed) downstream.end();
          resolveDrain();
        }
        downstream.on('data', () => finish('check-attach-stdin-refused'));
        downstream.once('close', () => { row.clientDisconnectedAt = now(); upstream.resume(); });
        upstream.once('connect', () => {
          if (closing) { finish('proxy-closing'); return; }
          row.forwardedAt = now();
          const headers = req.rawHeaders.reduce((lines, value, i, all) => i % 2 ? lines : lines + `${value}: ${all[i + 1]}\r\n`, '');
          upstream.write(`${req.method} ${req.url} HTTP/${req.httpVersion}\r\n${headers}\r\n`);
        });
        upstream.on('data', chunk => {
          bytes += chunk.length;
          if (bytes > maxResponseBytes) { finish('check-attach-byte-limit'); return; }
          if (!headerComplete) {
            header = Buffer.concat([header, chunk]); const end = header.indexOf('\r\n\r\n');
            if ((end < 0 && header.length > 64 * 1024) || end > 64 * 1024) { finish('check-attach-header-limit'); return; }
            if (end >= 0) {
              const status = /^HTTP\/1\.[01] (\d{3})(?: |\r)/.exec(header.toString('latin1'));
              if (!status) { finish('check-attach-invalid-response'); return; }
              row.status = Number(status[1]); row.respondedAt = now(); headerComplete = true; header = Buffer.alloc(0);
              if (row.status !== 101) failure('check-attach-not-upgraded', row);
            }
          }
          if (!downstream.destroyed && !downstream.write(chunk)) { upstream.pause(); downstream.once('drain', () => upstream.resume()); }
        });
        upstream.once('end', () => { upstreamEnded = true; finish(headerComplete ? undefined : 'check-attach-incomplete-header'); });
        upstream.once('error', () => finish('check-attach-upstream-error'));
        upstream.once('close', () => { if (!finished) finish(upstreamEnded ? undefined : 'check-attach-upstream-truncated'); });
      });
    } catch (cause) { failure(typeof cause?.code === 'string' ? cause.code : 'check-upgrade-guard-failed', row); downstream.destroy(); }
  }

  async function unlinkOwned(path) {
    try { const info = await lstat(path); if (socketIdentity && info.isSocket() && info.dev === socketIdentity.dev && info.ino === socketIdentity.ino) await unlink(path); }
    catch (cause) { if (cause.code !== 'ENOENT') throw cause; }
  }
  try {
    await new Promise((resolveListen, reject) => { server.once('error', reject); server.listen(privateSocket, () => { server.off('error', reject); resolveListen(); }); });
    await chmod(privateSocket, 0o600); socketIdentity = await lstat(privateSocket); await link(privateSocket, socketPath);
    persist('proxy-start', {}, true);
  } catch {
    await new Promise(done => server.close(() => done()));
    await unlinkOwned(socketPath); await unlinkOwned(privateSocket); await rmdir(privateDirectory).catch(() => {});
    throw problem('private-proxy-socket-failed');
  }
  server.on('error', () => failure('proxy-listener-failed'));

  function close() {
    if (closePromise) return closePromise;
    closing = true; persist('proxy-close-start');
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
      persist('proxy-drained', observations.cleanup, true);
      if (uncertainMutation) throw problem('forwarded-mutation-outcome-uncertain');
      return structuredClone(observations.cleanup);
    })(); return closePromise;
  }
  return { observations, armGate, waitForGate, releaseGate, cancelGate, close };
}
