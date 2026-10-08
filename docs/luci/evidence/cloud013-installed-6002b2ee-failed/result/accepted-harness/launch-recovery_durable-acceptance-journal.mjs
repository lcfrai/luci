// Acceptance diagnostics only. Never persist headers, bodies, environments,
// credentials, free-form error messages, private paths, or provider payloads.
import { closeSync, constants, fstatSync, fsyncSync, openSync, readFileSync, realpathSync, writeSync } from 'node:fs';
import { dirname, isAbsolute, resolve } from 'node:path';

const problem = code => Object.assign(new Error(code), { code });
const events = new Set(['driver-start', 'driver-stage', 'driver-background-failure', 'driver-caught-failure', 'driver-fatal', 'driver-signal', 'driver-exit', 'driver-cleanup-start', 'driver-receipt-prepared', 'controller-start', 'controller-exit', 'proxy-start', 'mutation-forward-intent', 'mutation-response-drained', 'create-ack', 'response-end-issued', 'request-failed', 'proxy-close-start', 'proxy-drained']);
const durableEvents = new Set(events);
const labels = new Set(['luci.managed', 'luci.instance', 'luci.sandbox-id', 'luci.session-id', 'luci.project-id', 'luci.user-id', 'luci.team-id', 'luci.launch-fence', 'luci.fence-intent', 'luci.fence-binding', 'luci.check', 'luci.check-id', 'luci.check-protocol', 'luci.check-engine', 'luci.check-allocation', 'luci.check-command-index', 'luci.check-scope', 'luci.check-project', 'luci.check-command-digest', 'luci.check-source-digest', 'luci.check-image']);
const numeric = ['requestSequence', 'ordinal', 'status', 'proxyStatus', 'responseBytes', 'pid', 'parentPid', 'startTicks', 'group', 'session', 'deadlineMs', 'pendingRequests', 'activeHandlers', 'pendingBodyBytes'];
const boolean = ['uncertainMutation', 'drained', 'passed', 'clientDestroyed'];
const strings = ['stage', 'kind', 'code', 'errorName', 'errorCode', 'signal', 'origin'];
const short = value => typeof value === 'string' && /^[A-Za-z0-9_.:-]{1,160}$/.test(value);

export function diagnosticError(error) {
  const value = {};
  if (['Error', 'AssertionError', 'TypeError', 'RangeError', 'AbortError', 'SyntaxError'].includes(error?.name)) value.errorName = error.name;
  if (short(error?.code)) value.errorCode = error.code;
  const locations = [...String(error?.stack ?? '').matchAll(/(?:^|[/\\])(dag\.mjs|sustained\.mjs|acceptance-lifecycle\.mjs|fenced-docker-proxy\.mjs|project-check-guard\.mjs|durable-acceptance-journal\.mjs):(\d{1,7}):(\d{1,7})/g)];
  value.locations = locations.slice(0, 8).map(([, file, line, column]) => ({file, line: Number(line), column: Number(column)}));
  return value;
}

export function journalProcessIdentity(pid = process.pid) {
  const raw = readFileSync(`/proc/${pid}/stat`, 'utf8');
  const fields = raw.slice(raw.lastIndexOf(')') + 2).trim().split(/\s+/);
  return {pid, parentPid: Number(fields[1]), group: Number(fields[2]), session: Number(fields[3]), startTicks: Number(fields[19])};
}

function sanitize(input) {
  const row = {};
  for (const key of numeric) if (Number.isSafeInteger(input[key]) && input[key] >= 0) row[key] = input[key];
  for (const key of ['exitCode', 'signalNumber']) if (Number.isSafeInteger(input[key]) && Math.abs(input[key]) < 65536) row[key] = input[key];
  for (const key of boolean) if (typeof input[key] === 'boolean') row[key] = input[key];
  for (const key of strings) if (short(input[key])) row[key] = input[key];
  for (const key of ['id', 'ref', 'createResponseId']) if (/^[a-f0-9]{64}$/.test(input[key] ?? '')) row[key] = input[key];
  if (/^[A-Za-z0-9_.-]{1,200}$/.test(input.name ?? '')) row.name = input.name;
  if (['GET', 'HEAD', 'POST', 'DELETE'].includes(input.method)) row.method = input.method;
  if (/^\/v\d+\.\d+\/containers\/(?:create|[a-f0-9]{64}(?:\/(?:start|stop|json|stats|attach|wait|logs))?|luci-check-(?:check-[a-f0-9]{24}-0|checkexec-[a-f0-9]{24})(?:\/(?:start|stop|json|attach|wait))?)$/.test(input.path ?? '')) row.path = input.path;
  if (input.labels && typeof input.labels === 'object') row.labels = Object.fromEntries(Object.entries(input.labels).filter(([key, value]) => labels.has(key) && short(value)));
  if (Array.isArray(input.locations)) row.locations = input.locations.slice(0, 8).filter(item => ['dag.mjs', 'sustained.mjs', 'acceptance-lifecycle.mjs', 'fenced-docker-proxy.mjs', 'project-check-guard.mjs', 'durable-acceptance-journal.mjs'].includes(item?.file) && Number.isSafeInteger(item.line) && item.line > 0 && Number.isSafeInteger(item.column) && item.column > 0).map(({file, line, column}) => ({file, line, column}));
  return row;
}

export function createAcceptanceJournal(path, {maxEvents = 20_000, maxBytes = 4 * 1024 * 1024, maxLineBytes = 8192} = {}) {
  if (!isAbsolute(path) || realpathSync(dirname(path)) !== resolve(dirname(path))) throw problem('journal-path-refused');
  if (![maxEvents, maxBytes, maxLineBytes].every(value => Number.isSafeInteger(value) && value > 0) || maxEvents > 20_000 || maxBytes > 4 * 1024 * 1024 || maxLineBytes > 8192) throw problem('journal-budget-refused');
  const fd = openSync(path, constants.O_WRONLY | constants.O_CREAT | constants.O_EXCL | constants.O_NOFOLLOW, 0o600);
  const info = fstatSync(fd);
  if (!info.isFile() || info.nlink !== 1 || (info.mode & 0o777) !== 0o600) {closeSync(fd); throw problem('journal-file-refused');}
  // Persist the directory entry as well as records. No overwrite/append reuse.
  const parent = openSync(dirname(path), constants.O_RDONLY | constants.O_DIRECTORY | constants.O_NOFOLLOW);
  try {fsyncSync(parent);} finally {closeSync(parent);}
  let sequence = 0, bytes = 0, failed = false, closed = false;
  const started = process.hrtime.bigint();
  function record(event, input = {}) {
    if (closed || failed) throw problem('journal-unavailable');
    if (!events.has(event)) throw problem('journal-event-refused');
    const row = {version: 1, sequence: sequence + 1, event, at: new Date().toISOString(), elapsedMs: Number(process.hrtime.bigint() - started) / 1e6, ...sanitize(input)};
    const data = Buffer.from(JSON.stringify(row) + '\n');
    if (sequence >= maxEvents || data.length > maxLineBytes || bytes + data.length > maxBytes) {failed = true; throw problem('journal-budget-exceeded');}
    try {
      for (let offset = 0; offset < data.length;) {const count = writeSync(fd, data, offset, data.length - offset); if (count <= 0) throw problem('journal-short-write'); offset += count;}
      if (durableEvents.has(event)) fsyncSync(fd);
      sequence++; bytes += data.length;
    } catch {failed = true; throw problem('journal-write-failed');}
    return sequence;
  }
  function close() {if (closed) return; closed = true; try {fsyncSync(fd);} finally {closeSync(fd);}}
  return {record, close, get failed() {return failed;}, get counts() {return {events: sequence, bytes};}};
}
