import assert from 'node:assert/strict';
import { createHash } from 'node:crypto';
import { copyFileSync, existsSync, mkdirSync, readFileSync, writeFileSync } from 'node:fs';
const digest = path => createHash('sha256').update(readFileSync(path)).digest('hex');
const manifest = JSON.parse(readFileSync('fixture-manifest.json', 'utf8'));
assert.match(manifest.nonce, /^[a-f0-9-]{36}$/);
for (const [name, hash] of Object.entries(manifest.sources)) assert.equal(digest(name), hash, name);
assert.equal(existsSync('dist'), false, 'Build output must not arrive in source context');
const tap = readFileSync('test-output.tap', 'utf8');
assert.ok(Buffer.byteLength(tap) < 128 * 1024);
assert.match(tap, /\n# tests 10\n/);
assert.match(tap, /\n# pass 10\n/);
assert.match(tap, /\n# fail 0\n/);
assert.match(tap, /\n# skipped 0\n/);
assert.match(tap, /\n# cancelled 0\n/);
assert.doesNotMatch(tap, /^not ok /m);
const files = ['/proc/self/cgroup', '/sys/fs/cgroup/memory.max', '/sys/fs/cgroup/memory.swap.max', '/sys/fs/cgroup/cpu.max', '/sys/fs/cgroup/pids.max', '/sys/fs/cgroup/memory/memory.limit_in_bytes', '/sys/fs/cgroup/memory/memory.memsw.limit_in_bytes', '/sys/fs/cgroup/cpu/cpu.cfs_quota_us', '/sys/fs/cgroup/cpu/cpu.cfs_period_us'];
const cgroup = {};
for (const path of files) {
  try { const text = readFileSync(path, 'utf8'); assert.ok(Buffer.byteLength(text) < 8192); cgroup[path] = {status: 'read', text}; }
  catch (error) { cgroup[path] = {status: 'unavailable', code: error.code ?? error.name}; }
}
const proof = {schema: 1, scope: 'Handwritten warm-base build fixture; no model or whole-product pass', nonce: manifest.nonce, node: process.version, architecture: process.arch, platform: process.platform, uid: process.getuid(), sources: manifest.sources, tap, tapSHA256: digest('test-output.tap'), testProcessExit: 0, cgroup};
mkdirSync('dist');
for (const name of ['server.mjs', 'calculator.mjs']) copyFileSync(name, `dist/${name}`);
writeFileSync('dist/build-proof.json', JSON.stringify(proof));
process.stdout.write(`LUCI_WARM_PREVIEW_BUILD_OK ${manifest.nonce} ${proof.tapSHA256}\n`);
