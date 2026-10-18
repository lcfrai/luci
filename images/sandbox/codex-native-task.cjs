'use strict';

// Compatibility is not image provenance or transitive process retirement.
const { spawn } = require('node:child_process');
const { constants } = require('node:os');
const CODEX = '/usr/local/bin/codex';
const VERSION = Buffer.from('codex-cli 0.160.0\n', 'ascii');
const PREFIX = ['-c', 'cli_auth_credentials_store="file"', '-c', 'agents.enabled=true',
  '-c', 'agents.max_concurrent_threads_per_session=2', '-c', 'features.multi_agent_v2=false'];
const FAILURE = 78;
const SIGNALS = constants.signals;

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

function createControl(io) {
  const control = { signal: null, active: null };
  control.signalLive = signal => {
    const original = control.active;
    // Never signal a reusable numeric group after this original child's close.
    if (!original || original.closed || !Number.isSafeInteger(original.child.pid) || original.child.pid <= 0) return;
    try { io.kill(-original.child.pid, signal); } catch { /* Not a closure proof. */ }
  };
  control.interrupt = signal => {
    if (signal !== 'SIGTERM' && signal !== 'SIGINT') return;
    if (control.signal == null) control.signal = signal;
    control.signalLive(control.signal);
    if (control.active?.refuse) control.active.refuse();
  };
  return control;
}

function compatibility(io, control) {
  if (control.signal) return Promise.resolve(false);
  return new Promise(resolve => {
    let child, timer, failed = false, finished = false, original;
    let output = Buffer.alloc(0), errorBytes = 0, code, signal;
    const streams = [{ ended: false, closed: false }, { ended: false, closed: false }];
    const finish = () => {
      if (finished || !original?.closed || streams.some(stream => !stream.closed)) return;
      finished = true; io.clearTimeout(timer);
      resolve(!failed && !control.signal && code === 0 && signal == null &&
        streams.every(stream => stream.ended) && output.equals(VERSION) && errorBytes === 0);
    };
    const refuse = () => {
      if (finished) return;
      if (!failed) {
        failed = true;
        if (original && !original.closed && control.active === original && Number.isSafeInteger(child.pid) && child.pid > 0) {
          try { io.kill(-child.pid, 'SIGKILL'); } catch { /* Exact original only; no closure proof. */ }
        }
      }
      finish();
    };
    try { child = io.spawn(CODEX, ['--version'], { detached: true, stdio: ['ignore', 'pipe', 'pipe'] }); }
    catch { resolve(false); return; }
    if (!child) { resolve(false); return; }
    original = { child, closed: false, refuse }; control.active = original;
    child.on('error', refuse);
    child.on('close', (status, terminatedBy) => {
      if (finished) return;
      original.closed = true; code = status; signal = terminatedBy;
      if (code !== 0 || signal != null || !output.equals(VERSION) || errorBytes !== 0) refuse();
      finish();
    });
    for (const [index, stream] of [child.stdout, child.stderr].entries()) {
      const state = streams[index];
      if (!stream) {
        // No JS stream backing was returned. Still retain the original child
        // through actual close, and retain any other stream through its close.
        state.closed = true; failed = true; continue;
      }
      stream.on('data', bytes => {
        if (failed || finished) return;
        if (!Buffer.isBuffer(bytes) || (index === 0 ? output.length : errorBytes) + bytes.length > 128) { refuse(); return; }
        if (index === 0) output = Buffer.concat([output, bytes]);
        else { errorBytes += bytes.length; if (bytes.length) refuse(); }
      });
      stream.on('end', () => { if (!finished) state.ended = true; });
      stream.on('error', refuse);
      stream.on('close', () => { if (finished) return; state.closed = true; if (!state.ended) refuse(); finish(); });
    }
    timer = io.setTimeout(refuse, 2000);
    if (failed || control.signal) { failed = false; refuse(); }
  });
}

async function runNativeTask(args, io, control = createControl(io)) {
  if (!validArguments(args) || !(await compatibility(io, control)) || control.signal) return { phase: 'compatibility', code: FAILURE, signal: null };
  return new Promise(resolve => {
    let child, failed = false;
    try { child = io.spawn(CODEX, args, { detached: true, stdio: 'inherit' }); }
    catch { resolve({ phase: 'model', code: FAILURE, signal: null }); return; }
    if (!child) { resolve({ phase: 'model', code: FAILURE, signal: null }); return; }
    const original = { child, closed: false }; control.active = original;
    child.on('error', () => { failed = true; control.signalLive('SIGKILL'); });
    child.on('close', (code, signal) => {
      original.closed = true;
      const effectiveSignal = signal || control.signal;
      if (typeof effectiveSignal === 'string' && Object.hasOwn(SIGNALS, effectiveSignal) && Number.isSafeInteger(SIGNALS[effectiveSignal]) && SIGNALS[effectiveSignal] > 0 && SIGNALS[effectiveSignal] < 128) resolve({ phase: 'model', code: 128 + SIGNALS[effectiveSignal], signal: effectiveSignal });
      else resolve({ phase: 'model', code: !failed && effectiveSignal == null && Number.isInteger(code) && code >= 0 && code <= 255 ? code : FAILURE, signal: null });
    });
    // A signal or partial return during spawn is latched before classification.
    if (control.signal) control.signalLive(control.signal);
  });
}

if (require.main === module) {
  const io = { spawn, kill: process.kill, setTimeout, clearTimeout };
  const control = createControl(io);
  const handlers = new Map();
  for (const signal of ['SIGTERM', 'SIGINT']) {
    const handler = () => control.interrupt(signal); handlers.set(signal, handler); process.on(signal, handler);
  }
  runNativeTask(process.argv.slice(2), io, control).then(result => {
    if (result.phase === 'compatibility') process.stderr.write('The configured Codex native-helper task profile is incompatible or interrupted. No model work was started.\n');
    process.exitCode = result.code;
    // Preserve runtime-supported Unix signal termination after actual close.
    // 128+signal is the bounded fallback if self-delivery is unavailable.
    if (result.signal) {
      for (const [signal, handler] of handlers) process.removeListener(signal, handler);
      try { process.kill(process.pid, result.signal); } catch { /* exitCode above */ }
    }
  }, () => { process.exitCode = FAILURE; });
}

module.exports = { runNativeTask, createControl, validArguments, PREFIX };
