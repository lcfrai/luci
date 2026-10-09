import { closeSync, constants, fsyncSync, openSync, writeSync } from 'node:fs';
import { dirname } from 'node:path';
import {setTimeout as delay} from 'node:timers/promises';

const problem = code => Object.assign(new Error(code), {code});

// Promise.race does not cancel its losing operation. Call assertActive at every
// mutation boundary as well as refusing browser routes and proxy Creates.
export function createWorkflowDeadline({timeoutMs, onFailure = () => {}}) {
  let failure, rejectFailure, timer;
  const failed = new Promise((_, reject) => {rejectFailure = reject;});
  // A failure may happen just before run attaches its race handler.
  void failed.catch(() => {});
  function fail(error) {
    if (failure) return;
    failure = error;
    rejectFailure(error);
  }
  timer = setTimeout(() => {
    const error = problem('workflow-deadline-exceeded');
    fail(error);
    onFailure(error);
  }, timeoutMs);
  return {
    fail,
    assertActive() {if (failure) throw failure;},
    get failed() {return !!failure;},
    async run(operation) {
      try {return await Promise.race([Promise.resolve().then(operation), failed]);}
      finally {clearTimeout(timer);}
    },
    close() {clearTimeout(timer);},
  };
}

export async function boundedAttempt(operation, timeoutMs, code) {
  if (!(Number.isFinite(timeoutMs) && timeoutMs > 0)) throw problem(code);
  let timer;
  try {
    return await Promise.race([Promise.resolve().then(operation), new Promise((_, reject) => {timer = setTimeout(() => reject(problem(code)), timeoutMs);})]);
  } finally {clearTimeout(timer);}
}

// launch() may have started profile-owned processes before returning a handle.
// Always reach the same identity-checked fallback, including that partial case.
export async function settleOwnedBrowser({browser, listOwned, stillOwned, forceTerminate, onFailure, closeTimeoutMs = 5000, settleTimeoutMs = 5000, pollIntervalMs = 100}) {
  if (browser) {
    try {await boundedAttempt(() => browser.close(), closeTimeoutMs, 'browser-close-deadline-exceeded');}
    catch (error) {onFailure(error);}
  }
  const end = Date.now() + settleTimeoutMs;
  while (Date.now() < end && listOwned().length) await delay(Math.min(pollIntervalMs, Math.max(1, end - Date.now())));
  for (const identity of listOwned()) {
    onFailure(problem('owned-browser-required-forced-cleanup'));
    if (stillOwned(identity)) await forceTerminate(identity);
  }
}

// Synchronous and durable before any potentially stuck cleanup await. Callers
// provide an already sanitized object; never pass raw exception or environment.
export function writeFailureCheckpoint(path, value) {
  const bytes = Buffer.from(JSON.stringify(value, null, 2) + '\n');
  if (bytes.length > 64 * 1024) throw problem('failure-checkpoint-budget-exceeded');
  const fd = openSync(path, constants.O_WRONLY | constants.O_CREAT | constants.O_EXCL | constants.O_NOFOLLOW, 0o600);
  try {
    for (let offset = 0; offset < bytes.length;) {const count = writeSync(fd, bytes, offset, bytes.length - offset); if (count <= 0) throw problem('failure-checkpoint-short-write'); offset += count;}
    fsyncSync(fd);
  } finally {closeSync(fd);}
  const parent = openSync(dirname(path), constants.O_RDONLY | constants.O_DIRECTORY | constants.O_NOFOLLOW);
  try {fsyncSync(parent);} finally {closeSync(parent);}
}

// One failed or stuck phase cannot skip subsequent independent cleanup phases.
// A timed-out operation remains unsettled; callers must retain that uncertainty.
export async function runBoundedCleanup({checkpoint, phases, onFailure = () => {}, onPhase = () => {}}) {
  try {checkpoint();} catch (error) {onFailure(error);}
  const results = [];
  for (const {name, timeoutMs, run} of phases) {
    try {onPhase(name, 'started');} catch (error) {onFailure(error);}
    try {results.push({name, status: 'completed', result: await boundedAttempt(run, timeoutMs, name + '-deadline-exceeded')});}
    catch (error) {results.push({name, status: 'failed-or-unsettled', code: error.code ?? 'cleanup-operation-failed'}); onFailure(error);}
    try {onPhase(name, results.at(-1).status);} catch (error) {onFailure(error);}
  }
  return results;
}
