// Opt-in metadata-only observer for one isolated OpenCode -> local Ollama run.
// It is deliberately not a general proxy or a production model transport.
import {createServer, request as httpRequest} from 'node:http';
import {openSync, writeSync, closeSync, lstatSync} from 'node:fs';
import {dirname, resolve} from 'node:path';
import {pathToFileURL} from 'node:url';
import {timingSafeEqual} from 'node:crypto';

const MAX_REQUEST = 1024 * 1024;
const MAX_RESPONSE = 8 * 1024 * 1024;
const MAX_EVENT = 64 * 1024;
const MAX_CALLS = 64;
const MAX_ACTIVE = 4;
const MAX_RUN_MS = 6 * 60 * 1000;
const MAX_CALL_MS = 180 * 1000;
const MAX_BODY_MS = 10 * 1000;

function equalToken(actual, expected) {
  if (typeof actual !== 'string' || typeof expected !== 'string') return false;
  const a = Buffer.from(actual), b = Buffer.from(expected);
  return a.length === b.length && timingSafeEqual(a, b);
}

function safeToolNames(tools) {
  if (!Array.isArray(tools) || tools.length > 64) throw new Error('invalid_tools');
  const names = tools.map(t => t?.type === 'function' ? t.function?.name : undefined);
  if (names.some(n => typeof n !== 'string' || !/^[A-Za-z_][A-Za-z0-9_.-]{0,63}$/.test(n))) throw new Error('invalid_tools');
  return [...new Set(names)].sort();
}

function safeToolChoice(choice, offered) {
  if (['auto','required','none'].includes(choice)) return choice;
  if (choice?.type === 'function' && typeof choice.function?.name === 'string')
    return offered.includes(choice.function.name) ? `function:${choice.function.name}` : 'function:unoffered';
  return 'unspecified';
}

export function createMetadataExtractor(stream, offered) {
  let bytes = 0, incomplete = false, line = '', json = '';
  const calls = new Map();
  let finishReason = null, textBytes = 0, done = false;
  const name = n => offered.includes(n) ? n : 'unoffered';
  const observe = (choice, incremental) => {
    if (!choice || typeof choice !== 'object') return;
    if (typeof choice.finish_reason === 'string' && choice.finish_reason.length <= 64) finishReason = choice.finish_reason;
    const msg = incremental ? choice.delta : choice.message;
    if (!msg || typeof msg !== 'object') return;
    if (typeof msg.content === 'string') textBytes += Buffer.byteLength(msg.content);
    if (!Array.isArray(msg.tool_calls)) return;
    for (const tc of msg.tool_calls.slice(0, MAX_CALLS)) {
      const id = incremental ? tc?.index : (tc?.id ?? calls.size);
      if ((typeof id !== 'number' && typeof id !== 'string') || calls.size >= MAX_CALLS && !calls.has(id)) { incomplete = true; continue; }
      const n = tc?.function?.name;
      if (typeof n === 'string' && n.length <= 64) calls.set(id, name(n));
      else if (!calls.has(id)) calls.set(id, 'name_unavailable');
    }
  };
  const event = raw => {
    if (raw === '[DONE]') { done = true; return; }
    try {
      const parsed = JSON.parse(raw);
      for (const c of (Array.isArray(parsed.choices) ? parsed.choices : []).slice(0, 2)) observe(c, true);
    } catch { incomplete = true; }
  };
  return {
    feed(chunk) {
      bytes += chunk.length;
      if (bytes > MAX_RESPONSE) throw new Error('response_limit');
      if (!stream) {
        if (!incomplete && Buffer.byteLength(json) + chunk.length <= MAX_RESPONSE) json += chunk.toString('utf8');
        else incomplete = true;
        return;
      }
      line += chunk.toString('utf8');
      let pos;
      while ((pos = line.indexOf('\n')) >= 0) {
        const row = line.slice(0, pos).trimEnd(); line = line.slice(pos + 1);
        if (Buffer.byteLength(row) > MAX_EVENT) { incomplete = true; continue; }
        if (row.startsWith('data:')) event(row.slice(5).trimStart());
      }
      if (Buffer.byteLength(line) > MAX_EVENT) { incomplete = true; line = ''; }
    },
    finish() {
      if (!stream && !incomplete) {
        try {
          const parsed = JSON.parse(json);
          for (const c of (Array.isArray(parsed.choices) ? parsed.choices : []).slice(0, 2)) observe(c, false);
        } catch { incomplete = true; }
      }
      if (stream && line.trim()) incomplete = true;
      return {nativeToolCallCount: calls.size, nativeToolNames: [...calls.values()].sort(), finishReason,
        textPresent: textBytes > 0, textBytes, streamDone: stream ? done : null, metadataIncomplete: incomplete || (stream && !done)};
    },
  };
}

export async function startObserver({listenPort, upstreamPort = 11434, model, token, receiptPath, now = Date.now, maxCallMs = MAX_CALL_MS}) {
  if (!Number.isInteger(listenPort) || listenPort < 0 || listenPort > 65535 ||
      !Number.isInteger(upstreamPort) || upstreamPort < 1 || upstreamPort > 65535 ||
      typeof model !== 'string' || !/^[A-Za-z0-9][A-Za-z0-9_.:/-]{0,199}$/.test(model) ||
      typeof token !== 'string' || token.length < 32 || token.length > 256 ||
      !Number.isInteger(maxCallMs) || maxCallMs < 10 || maxCallMs > MAX_CALL_MS) throw new Error('invalid_configuration');
  const dir = lstatSync(dirname(receiptPath));
  if (!dir.isDirectory() || dir.isSymbolicLink()) throw new Error('unsafe_receipt_directory');
  const receipt = openSync(receiptPath, 'wx', 0o600);
  let sequence = 0, closed = false, retired = false;
  const active = new Set();
  const started = now();
  const write = row => { writeSync(receipt, JSON.stringify(row) + '\n'); };
  const server = createServer((req, res) => {
    const reject = (status, code) => { res.writeHead(status, {'content-type':'application/json'}); res.end(JSON.stringify({error: code})); };
    if (closed || now() - started >= MAX_RUN_MS) return reject(503, 'observer_expired');
    if (req.method === 'GET' && req.url === '/health') {
      if (!equalToken(req.headers.authorization, `Bearer ${token}`)) return reject(401, 'unauthorized');
      res.writeHead(200, {'content-type':'application/json','cache-control':'no-store'});
      return res.end('{"state":"ready"}');
    }
    if (req.method !== 'POST' || req.url !== '/v1/chat/completions') return reject(404, 'route_unavailable');
    if (!equalToken(req.headers.authorization, `Bearer ${token}`)) return reject(401, 'unauthorized');
    if (sequence + active.size >= MAX_CALLS || active.size >= MAX_ACTIVE) return reject(429, 'observer_busy');
    const length = req.headers['content-length'];
    if (length === undefined || !/^\d+$/.test(length) || Number(length) > MAX_REQUEST || Number(length) === 0 ||
        req.headers['content-encoding'] || req.headers['transfer-encoding']) return reject(413, 'request_bound');
    const claim = {upstream: null, finish: null, response: res, request: req};
    active.add(claim);
    const parts = []; let size = 0, bodyEnded = false;
    const bodyTimer = setTimeout(() => { if (!bodyEnded && active.has(claim)) { reject(408, 'request_timeout'); active.delete(claim); req.destroy(); } }, MAX_BODY_MS);
    req.on('data', chunk => {
      size += chunk.length;
      if (size > MAX_REQUEST && active.has(claim)) { clearTimeout(bodyTimer); reject(413, 'request_bound'); active.delete(claim); req.destroy(); return; }
      parts.push(chunk);
    });
    req.on('end', () => {
      clearTimeout(bodyTimer); bodyEnded = true;
      if (!active.has(claim)) return;
      if (size !== Number(length)) { active.delete(claim); return reject(400, 'request_length'); }
      const body = Buffer.concat(parts, size);
      let parsed, offered;
      try {
        parsed = JSON.parse(body.toString('utf8'));
        offered = safeToolNames(parsed.tools ?? []);
        if (parsed.model !== model || typeof parsed.stream !== 'boolean') throw new Error('model_or_stream');
      } catch { active.delete(claim); return reject(400, 'request_shape'); }
      const seq = ++sequence;
      const row = {sequence: seq, atUTC: new Date(now()).toISOString(), stream: parsed.stream, offeredToolNames: offered,
        offeredToolCount: offered.length, toolChoice: safeToolChoice(parsed.tool_choice, offered),
        responseReceived: false, upstreamCompleted: false, outcome: 'dispatch_uncertain'};
      const extractor = createMetadataExtractor(parsed.stream, offered);
      let finished = false, timer;
      const finish = outcome => {
        if (finished) return;
        finished = true; clearTimeout(timer);
        row.outcome = outcome; Object.assign(row, extractor.finish()); write(row);
        active.delete(claim);
      };
      claim.finish = finish;
      const upstream = httpRequest({hostname:'127.0.0.1', port:upstreamPort, path:'/v1/chat/completions', method:'POST',
        headers:{'content-type':'application/json', 'content-length':body.length}, agent:false}, upstreamRes => {
        row.responseReceived = true;
        row.upstreamStatus = upstreamRes.statusCode ?? 502;
        res.writeHead(upstreamRes.statusCode ?? 502, {'content-type': upstreamRes.headers['content-type'] ?? 'application/json',
          'cache-control':'no-store'});
        upstreamRes.on('data', chunk => {
          try { extractor.feed(chunk); } catch {
            finish('response_limit_uncertain'); upstream.destroy(); res.destroy(); return;
          }
          if (!res.write(chunk)) upstreamRes.pause();
        });
        res.on('drain', () => upstreamRes.resume());
        upstreamRes.on('end', () => { row.upstreamCompleted = true; res.end(); });
        upstreamRes.on('error', () => { finish('upstream_stream_uncertain'); res.destroy(); });
      });
      claim.upstream = upstream;
      timer = setTimeout(() => { finish('deadline_uncertain'); upstream.destroy(); res.destroy(); }, maxCallMs);
      upstream.on('error', () => { finish('upstream_error_uncertain'); if (!res.headersSent) reject(502, 'upstream_unavailable'); else res.destroy(); });
      res.on('finish', () => finish('response_flushed'));
      res.on('close', () => { if (!finished) { finish('client_disconnected_uncertain'); upstream.destroy(); } });
      upstream.end(body);
    });
    req.on('error', () => { clearTimeout(bodyTimer); if (!claim.finish) active.delete(claim); });
    req.on('aborted', () => { clearTimeout(bodyTimer); if (!claim.finish) active.delete(claim); });
  });
  server.maxConnections = 8;
  server.maxHeadersCount = 32;
  server.headersTimeout = 5_000;
  server.requestTimeout = 12_000;
  server.keepAliveTimeout = 1_000;
  server.on('clientError', (_err, socket) => socket.destroy());
  await new Promise((resolve, reject) => { server.once('error', reject); server.listen(listenPort, '127.0.0.1', resolve); });
  let expire;
  const retire = async reason => {
    if (retired) return;
    retired = true; closed = true; clearTimeout(expire);
    for (const held of [...active]) {
      held.finish?.(reason);
      held.upstream?.destroy();
      held.response.destroy();
      held.request.destroy();
      active.delete(held);
    }
    await new Promise(resolve => server.close(resolve)); closeSync(receipt);
  };
  expire = setTimeout(() => { void retire('observer_expired_uncertain'); }, MAX_RUN_MS);
  return {port: server.address().port, close: () => retire('observer_retired_uncertain')};
}

if (process.argv[1] && import.meta.url === pathToFileURL(resolve(process.argv[1])).href) {
  const words = process.argv.slice(2);
  if (words.length !== 6 || words.some((v,i) => i % 2 === 0 && !['--listen-port','--model','--receipt'].includes(v)))
    throw new Error('expected --listen-port N --model ID --receipt PRIVATE_PATH');
  const args = Object.fromEntries([[words[0],words[1]],[words[2],words[3]],[words[4],words[5]]]);
  if (Object.keys(args).length !== 3) throw new Error('duplicate_argument');
  const port = Number(args['--listen-port']);
  if (!Number.isInteger(port) || port < 1024 || port > 65535) throw new Error('invalid_listen_port');
  const observer = await startObserver({listenPort:port, model:args['--model'], receiptPath:args['--receipt'], token:process.env.LUCI_OLLAMA_OBSERVER_TOKEN});
  const retire = () => observer.close().then(() => process.exit(0));
  process.on('SIGTERM', retire); process.on('SIGINT', retire);
  process.stdout.write(`observer listening on 127.0.0.1:${observer.port}\n`);
}
