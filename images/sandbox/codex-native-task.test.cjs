'use strict';
const { test } = require('node:test');
const assert = require('node:assert/strict');
const { EventEmitter } = require('node:events');
const { runNativeTask, PREFIX } = require('./codex-native-task.cjs');

function fixture() {
  const children = [], calls = [], kills = [], timers = [];
  const io = {
    spawn(command, args, options) {
      const child = new EventEmitter(); child.pid = 100 + children.length;
      child.stdout = new EventEmitter(); child.stderr = new EventEmitter();
      children.push(child); calls.push({ command, args, options }); return child;
    },
    kill: (...args) => kills.push(args),
    setTimeout: (fn, ms) => { timers.push({ fn, ms }); return timers.length; },
    clearTimeout: () => {},
  };
  const args = [...PREFIX, 'exec', '--color', 'never', '--dangerously-bypass-approvals-and-sandbox', '--', 'Implement only the reviewed task'];
  const settle = (child, text = 'codex-cli 0.160.0\n', code = 0, signal = null) => {
    child.stdout.emit('data', Buffer.from(text)); child.stdout.emit('end'); child.stderr.emit('end'); child.emit('close', code, signal);
  };
  return { io, args, children, calls, kills, timers, settle };
}

test('native helper compatibility waits for exact original close and both EOFs', async () => {
  const f = fixture(), result = runNativeTask(f.args, f.io), version = f.children[0];
  version.stdout.emit('data', Buffer.from('codex-cli 0.160.0\n'));
  version.emit('close', 0, null);
  await Promise.resolve(); assert.equal(f.calls.length, 1);
  version.stdout.emit('end'); await Promise.resolve(); assert.equal(f.calls.length, 1);
  version.stderr.emit('end'); await Promise.resolve();
  assert.equal(f.calls.length, 2); assert.equal(f.calls[0].command, '/usr/local/bin/codex');
  assert.deepEqual(f.calls[0].args, ['--version']); assert.equal(f.calls[0].options.detached, true);
  assert.deepEqual(f.calls[1].args, f.args); f.children[1].emit('close', 0, null);
  assert.equal(await result, 0);
});

test('native helper compatibility failures never invoke a model', async t => {
  for (const kind of ['wrong-version', 'missing-lf', 'stderr', 'overflow', 'timeout', 'nonzero', 'signal', 'spawn-error', 'stream-error']) {
    await t.test(kind, async () => {
      const f = fixture(), result = runNativeTask(f.args, f.io), child = f.children[0];
      if (kind === 'stderr') child.stderr.emit('data', Buffer.from('warning'));
      if (kind === 'overflow') child.stdout.emit('data', Buffer.alloc(129));
      if (kind === 'timeout') { assert.equal(f.timers[0].ms, 2000); f.timers[0].fn(); }
      if (kind === 'spawn-error') child.emit('error', new Error('synthetic'));
      if (kind === 'stream-error') child.stdout.emit('error', new Error('synthetic'));
      f.settle(child, kind === 'wrong-version' ? 'codex-cli 0.160.1\n' : kind === 'missing-lf' ? 'codex-cli 0.160.0' : 'codex-cli 0.160.0\n', kind === 'nonzero' ? 1 : 0, kind === 'signal' ? 'SIGTERM' : null);
      assert.equal(await result, 78); assert.equal(f.calls.length, 1);
      assert.deepEqual(f.kills, [[-100, 'SIGKILL']]);
    });
  }
});

test('native helper failure stays sticky through a late success-shaped return', async () => {
  const f = fixture(), result = runNativeTask(f.args, f.io), child = f.children[0];
  f.timers[0].fn(); child.emit('close', 0, null); await Promise.resolve();
  assert.equal(f.calls.length, 1);
  child.stdout.emit('data', Buffer.from('codex-cli 0.160.0\n')); child.stdout.emit('end'); child.stderr.emit('end');
  assert.equal(await result, 78); assert.equal(f.calls.length, 1);
});

test('native helper rejects altered policy and extra argv before compatibility spawn', async t => {
  for (const args of [[], [...PREFIX, 'exec', '--color', 'never', '--dangerously-bypass-approvals-and-sandbox', '-c', 'agents.enabled=false', '--', 'task'], ['-c', 'agents.enabled=true'], [...PREFIX, 'exec', '--color', 'never', '--dangerously-bypass-approvals-and-sandbox', '--', 'task', 'extra']]) {
    await t.test(String(args.length), async () => {
      const f = fixture(); assert.equal(await runNativeTask(args, f.io), 78); assert.equal(f.calls.length, 0);
    });
  }
});

test('native helper model error cannot become a successful wrapper exit', async () => {
  const f = fixture(), result = runNativeTask(f.args, f.io);
  f.settle(f.children[0]); await Promise.resolve();
  f.children[1].emit('error', new Error('synthetic')); f.children[1].emit('close', 0, null);
  assert.equal(await result, 78); assert.deepEqual(f.kills, [[-101, 'SIGKILL']]);
});
