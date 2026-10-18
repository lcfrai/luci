'use strict';

// Fixed image entry for one explicitly reviewed native-helper task. This check
// establishes CLI compatibility, not image provenance or transitive retirement.
const { spawn } = require('node:child_process');
const CODEX = '/usr/local/bin/codex';
const VERSION = Buffer.from('codex-cli 0.160.0\n', 'ascii');
const PREFIX = ['-c', 'cli_auth_credentials_store="file"', '-c', 'agents.enabled=true',
  '-c', 'agents.max_concurrent_threads_per_session=2', '-c', 'features.multi_agent_v2=false'];
const FAILURE = 78;

function validArguments(args) {
  if (!Array.isArray(args) || args.length < PREFIX.length + 5 || args.length > 24 ||
      args.some(value => typeof value !== 'string' || Buffer.byteLength(value) > 512 * 1024 || value.includes('\0')) ||
      PREFIX.some((value, index) => args[index] !== value)) return false;
  let i = PREFIX.length;
  if (args[i] === '-c') {
    if (!args[i + 1]?.startsWith('developer_instructions=')) return false;
    i += 2;
  }
  if (args[i++] !== 'exec' || args[i++] !== '--color' || args[i++] !== 'never' ||
      args[i++] !== '--dangerously-bypass-approvals-and-sandbox') return false;
  if (args[i] === '--model') {
    if (!args[i + 1] || args[i + 1].length > 512) return false;
    i += 2;
  }
  return args[i++] === '--' && i + 1 === args.length && args[i] !== '';
}

function compatibility(io) {
  return new Promise(resolve => {
    let child, timer, failed = false, closed = false, outEnd = false, errEnd = false;
    let output = Buffer.alloc(0), errorBytes = 0, killed = false;
    const kill = () => {
      if (killed) return;
      killed = true;
      if (Number.isSafeInteger(child?.pid) && child.pid > 0) {
        try { io.kill(-child.pid, 'SIGKILL'); } catch { /* Not proof of retirement. */ }
      }
    };
    const finish = () => {
      // Even failure retains the original child/streams until actual close/EOF.
      // If native IO cannot settle, independent parent retirement is decisive.
      if (!closed || !outEnd || !errEnd) return;
      io.clearTimeout(timer);
      resolve(!failed && output.equals(VERSION) && errorBytes === 0);
    };
    const refuse = () => { failed = true; kill(); finish(); };
    try { child = io.spawn(CODEX, ['--version'], { detached: true, stdio: ['ignore', 'pipe', 'pipe'] }); }
    catch { resolve(false); return; }
    if (!child || !child.stdout || !child.stderr) { refuse(); return; }
    child.stdout.on('data', bytes => {
      if (failed) return;
      if (!Buffer.isBuffer(bytes) || output.length + bytes.length > 128) { refuse(); return; }
      output = Buffer.concat([output, bytes]);
    });
    child.stderr.on('data', bytes => {
      if (failed) return;
      if (!Buffer.isBuffer(bytes) || errorBytes + bytes.length > 128) { refuse(); return; }
      errorBytes += bytes.length;
      if (bytes.length) refuse();
    });
    child.stdout.on('end', () => { outEnd = true; finish(); });
    child.stderr.on('end', () => { errEnd = true; finish(); });
    child.stdout.on('error', refuse);
    child.stderr.on('error', refuse);
    child.on('error', refuse);
    child.on('close', (code, signal) => {
      closed = true;
      if (code !== 0 || signal != null || !output.equals(VERSION) || errorBytes !== 0) refuse();
      finish();
    });
    timer = io.setTimeout(refuse, 2000);
  });
}

async function runNativeTask(args, io) {
  if (!validArguments(args) || !(await compatibility(io))) return FAILURE;
  return new Promise(resolve => {
    let child, failed = false;
    try { child = io.spawn(CODEX, args, { detached: true, stdio: 'inherit' }); }
    catch { resolve(FAILURE); return; }
    if (!child) { resolve(FAILURE); return; }
    child.on('error', () => {
      failed = true;
      if (Number.isSafeInteger(child.pid) && child.pid > 0) {
        try { io.kill(-child.pid, 'SIGKILL'); } catch { /* Parent still owns retirement. */ }
      }
    });
    child.on('close', (code, signal) => resolve(!failed && signal == null && Number.isInteger(code) && code >= 0 && code <= 255 ? code : FAILURE));
    if (io.onModel) io.onModel(child);
  });
}

if (require.main === module) {
  let model;
  const io = { spawn, kill: process.kill, setTimeout, clearTimeout, onModel: child => { model = child; } };
  for (const signal of ['SIGTERM', 'SIGINT']) process.on(signal, () => {
    if (Number.isSafeInteger(model?.pid) && model.pid > 0) {
      try { process.kill(-model.pid, signal); } catch { /* No transitive closure claim. */ }
    }
  });
  runNativeTask(process.argv.slice(2), io).then(code => {
    if (code === FAILURE) process.stderr.write('The configured Codex native-helper task profile is incompatible or could not settle its compatibility check. No compatibility failure starts model work.\n');
    process.exitCode = code;
  }, () => { process.exitCode = FAILURE; });
}

module.exports = { runNativeTask, validArguments, PREFIX };
