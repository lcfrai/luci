// Inert source regression: VM-evaluate the exact Go-embedded server against a
// fake HTTP module. No socket, child process, Docker, application or credential.
import assert from 'node:assert/strict';
import { readFileSync } from 'node:fs';
import { createHash } from 'node:crypto';
import { runInNewContext } from 'node:vm';

const source = readFileSync(new URL('./reviewed_source_preview.go', import.meta.url), 'utf8');
const program = source.match(/const sourcePreviewServer = `([\s\S]*?)`/)[1];
const digest = raw => createHash('sha256').update(raw).digest('hex');
const files = [
  { name: 'index.html', type: 'text/html; charset=utf-8', data: Buffer.from('<h1>reviewed</h1>').toString('base64') },
  { name: 'assets/app.mjs', type: 'text/javascript; charset=utf-8', data: Buffer.from('globalThis.projectCodeExecuted = true;').toString('base64') },
  { name: 'empty.txt', type: 'text/plain; charset=utf-8', data: '' },
];
const raw = JSON.stringify(files), sourceDigest = 'a'.repeat(64), snapshotDigest = 'b'.repeat(64);
function fixture(payloadDigest = digest(raw)) {
  let handler, listened = false;
  const server = { listen(port, host) { assert.equal(port, 3000); assert.equal(host, '0.0.0.0'); listened = true; }, close() {}, closeAllConnections() {} };
  const sandbox = { Buffer, require(name) { if (name === 'node:crypto') return { createHash }; if (name === 'node:http') return { createServer(options, fn) { assert.equal(options.maxHeaderSize, 8192); handler = fn; return server; } }; throw new Error(`Unexpected import ${name}`); }, process: { argv: ['node', '3000', payloadDigest, sourceDigest, snapshotDigest, raw.slice(0, 32), raw.slice(32)], on() {}, exit(code) { throw new Error(`exit ${code}`); } } };
  runInNewContext(program, sandbox, { timeout: 1000 });
  assert.equal(listened, true); assert.equal(sandbox.projectCodeExecuted, undefined);
  assert.equal(server.maxConnections, 32); assert.equal(server.requestTimeout, 5000); assert.equal(server.headersTimeout, 5000); assert.equal(server.keepAliveTimeout, 1000);
  return (url, method = 'GET') => { const response = { writeHead(status, headers) { this.status = status; this.headers = headers; }, end(body) { this.body = body; } }; handler({ url, method }, response); return response; };
}
const request = fixture(); let cases = 0;
for (const [url, status] of [['/', 200], ['/index.html', 200], ['/assets/app.mjs?v=1', 200], ['/empty.txt', 200], ['/missing.js', 404], ['/../index.html', 400], ['/assets/../../index.html', 400], ['/%69ndex.html', 400], ['/assets/%2e%2e/index.html', 400], ['//index.html', 400], ['/assets\\app.mjs', 400], ['/.private/data.json', 400], ['http://elsewhere/index.html', 400], ['/index.html#fragment', 400], ['/' + 'x'.repeat(4096), 400]]) { assert.equal(request(url).status, status, url); cases++; }
const get = request('/'); assert.equal(get.body.toString(), '<h1>reviewed</h1>'); assert.equal(get.headers['Content-Type'], 'text/html; charset=utf-8'); assert.equal(get.headers['X-Luci-Source-Digest'], sourceDigest); assert.equal(get.headers['X-Luci-Snapshot-Digest'], snapshotDigest); assert.equal(get.headers['Cache-Control'], 'no-store'); assert.equal(get.headers['X-Content-Type-Options'], 'nosniff'); cases++;
assert.equal(request('/', 'HEAD').body, undefined); assert.equal(request('/', 'HEAD').headers['Content-Length'], get.body.length); cases++;
assert.equal(request('/', 'POST').status, 405); assert.equal(request('/', 'POST').headers.Allow, 'GET, HEAD'); cases++;
assert.equal(request('/assets/app.mjs').headers['Content-Type'], 'text/javascript; charset=utf-8'); cases++;
assert.throws(() => fixture('0'.repeat(64)), /exit 2/); cases++;
console.log(`PASS ${cases} inert static-server cases (fake HTTP; zero sockets/children/Docker)`);
