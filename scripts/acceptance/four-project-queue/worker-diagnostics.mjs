// Installed-fixture evidence only. Never copy raw stderr, paths or error text.
import { constants, openSync, closeSync, readSync, fstatSync, lstatSync, realpathSync } from 'node:fs';
import { join, resolve } from 'node:path';

const maximumBytes = 64 * 1024;
const operations = new Set(['unclassified', 'native_state_prepare', 'native_conversation_find', 'session_resume_read', 'reviewed_context_capture', 'session_transcript_forget', 'reviewed_context_forget', 'session_record_forget', 'sandbox_create', 'sandbox_destroy', 'session_stop_record']);
const causes = new Set(['unclassified', 'not_exist', 'permission', 'no_space', 'context_canceled', 'deadline', 'hardlink', 'native_state_link', 'native_state_special', 'native_state_hardlink', 'native_state_limit', 'native_state_changed', 'native_state_io']);
const exactKeys = (value, keys) => value && typeof value === 'object' && !Array.isArray(value) && Object.keys(value).length === keys.length && keys.every(key => Object.hasOwn(value, key));
const same = (a, b) => a.dev === b.dev && a.ino === b.ino && a.size === b.size && a.mtimeMs === b.mtimeMs && a.nlink === b.nlink && a.mode === b.mode;
const privateFile = info => info.isFile() && !info.isSymbolicLink() && info.nlink === 1 && (info.mode & 0o077) === 0 && info.size <= maximumBytes;
const fixedTime = value => {
  if (typeof value !== 'string' || !/^\d{4}-\d{2}-\d{2}T\d{2}:\d{2}:\d{2}(?:\.\d{1,9})?Z$/.test(value)) return false;
  const date = new Date(value);
  return Number.isFinite(date.valueOf()) && date.toISOString().slice(0, 19) === value.slice(0, 19);
};

// No caller supplies a filename. The sole input is the owned fixture and its
// validated team identity, and collection happens only after all writers stop.
export function collectWorkerDiagnostics({ fixture, teamID, writersStopped }) {
  if (!writersStopped) return { status: 'not-collected', reason: 'writers-not-stopped' };
  if (teamID === undefined) return { status: 'not-collected', reason: 'team-not-created' };
  if (typeof teamID !== 'string' || !/^team-[A-Za-z0-9_-]{8,64}$/.test(teamID)) return { status: 'refused', reason: 'invalid-team-identity' };
  let fd;
  try {
    const root = resolve(fixture);
    if (root !== fixture || realpathSync(root) !== root) return { status: 'refused', reason: 'unsafe-fixture-root' };
    let path = root;
    for (const part of ['', 'state', 'control-plane', 'teams', teamID, 'runtime', 'state', 'diagnostics']) {
      if (part) path = join(path, part);
      const info = lstatSync(path);
      if (!info.isDirectory() || info.isSymbolicLink()) return { status: 'refused', reason: 'unsafe-diagnostics-directory' };
    }
    const file = join(path, 'api-failures.json'), before = lstatSync(file);
    if (!privateFile(before)) return { status: 'refused', reason: 'unsafe-diagnostics-file' };
    fd = openSync(file, constants.O_RDONLY | constants.O_NOFOLLOW);
    const opened = fstatSync(fd);
    if (!privateFile(opened) || !same(before, opened)) return { status: 'refused', reason: 'diagnostics-file-changed' };
    const bytes = Buffer.alloc(maximumBytes + 1);
    let count = 0;
    while (count < bytes.length) {
      const n = readSync(fd, bytes, count, bytes.length - count, null);
      if (!n) break;
      count += n;
    }
    if (count > maximumBytes || count !== opened.size || !same(opened, fstatSync(fd)) || !same(opened, lstatSync(file))) return { status: 'refused', reason: 'diagnostics-file-changed' };
    let value;
    try { value = JSON.parse(new TextDecoder('utf-8', { fatal: true }).decode(bytes.subarray(0, count))); }
    catch { return { status: 'refused', reason: 'invalid-diagnostics-json' }; }
    if (!exactKeys(value, ['version', 'events']) || value.version !== 1 || !Array.isArray(value.events) || value.events.length > 128) return { status: 'refused', reason: 'invalid-diagnostics-schema' };
    const events = [];
    for (const event of value.events) {
      if (!exactKeys(event, ['at', 'operation', 'cause']) || !fixedTime(event.at) || !operations.has(event.operation) || !causes.has(event.cause)) return { status: 'refused', reason: 'invalid-diagnostics-event' };
      // Reconstruct only the accepted fields; no original object is exported.
      events.push({ at: event.at, operation: event.operation, cause: event.cause });
    }
    return { status: 'captured', sourceBytes: count, document: { version: 1, events } };
  } catch (error) {
    // Error.message and stack may contain host paths or private input. Neither
    // is returned, even if the source file is malformed or replaced.
    return error.code === 'ENOENT' ? { status: 'absent' } : { status: 'refused', reason: 'diagnostics-read-failed' };
  } finally {
    if (fd !== undefined) { try { closeSync(fd); } catch { /* Read-only cleanup must not prevent owned runtime cleanup. */ } }
  }
}
