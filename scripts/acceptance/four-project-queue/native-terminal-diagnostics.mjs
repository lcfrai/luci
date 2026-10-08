// Fixture-only native startup observation. Does not send keystrokes or execute.
import { stripVTControlCharacters } from 'node:util';
import { constants, openSync, closeSync, readSync, lstatSync, fstatSync } from 'node:fs';

const maximumBufferBytes = 256 * 1024, maximumArtifactBytes = 64 * 1024;
export function classifyNativeTerminal(text) {
  return {
    directory_trust: text.includes('Trust this folder?') && text.includes('Trust and continue'),
    cwd_choice: text.includes('Working directory') && text.includes('Use session directory') && text.includes('Use current directory'),
    archived_conversation: text.includes('This conversation is archived'),
    hook_review: text.includes('Hooks need review'),
    model_migration: text.includes('Try new model') && text.includes('Use existing model'),
    fork_failure: text.includes('Failed to fork session from '),
    model_unavailable: text.includes('Thread model is unavailable. Wait for the thread to finish syncing or choose a model before sending input.'),
    phase_1_prompt: text.includes('LUCI_LIFECYCLE_PHASE_1'),
    phase_2_prompt: text.includes('LUCI_LIFECYCLE_PHASE_2'),
  };
}
export function fixedExitReason(value) {
  if (typeof value !== 'string') return { classification: 'unknown' };
  if (value === '') return { classification: 'none' };
  const status = /^exit status (\d{1,3})$/.exec(value);
  if (status) return { classification: 'exit-status', code: Number(status[1]) };
  if (['killed', 'signal: killed', 'signal: terminated', 'sandbox never became ready'].includes(value)) return { classification: value.replaceAll(' ', '_').replace(':', '') };
  return { classification: 'unknown' };
}
export function readNativeTranscriptForDiagnostics(path, limit, expected) {
  if (!Number.isInteger(limit) || limit < 0 || limit > 16 * 1024 ** 2) throw Error('invalid-transcript-limit');
  let fd;
  const identity = (a, b) => a.dev === b.dev && a.ino === b.ino;
  const valid = info => info.isFile() && !info.isSymbolicLink() && info.nlink === 1 && info.size <= limit;
  try {
    const before = lstatSync(path);
    if (!valid(before) || !identity(before, expected)) throw Error('unsafe-transcript-file');
    fd = openSync(path, constants.O_RDONLY | constants.O_NOFOLLOW);
    const opened = fstatSync(fd);
    if (!valid(opened) || !identity(before, opened)) throw Error('transcript-file-changed');
    const buffer = Buffer.alloc(Math.min(opened.size, limit) + 1);
    let count = 0;
    while (count < buffer.length) { const n = readSync(fd, buffer, count, buffer.length - count, null); if (!n) break; count += n; }
    const after = fstatSync(fd), named = lstatSync(path);
    if (count > limit || !valid(after) || !valid(named) || !identity(opened, after) || !identity(opened, named)) throw Error('transcript-file-changed');
    return { data: buffer.subarray(0, count), stable: opened.size === count && after.size === count && named.size === count && before.size === count && before.mtimeMs === after.mtimeMs && after.mtimeMs === named.mtimeMs };
  } finally { if (fd !== undefined) { try { closeSync(fd); } catch { /* Read-only diagnostic cleanup. */ } } }
}
function boundedText(bytes, truncated) {
  let text = stripVTControlCharacters(bytes.toString('utf8')).replace(/[\x00-\x08\x0b\x0c\x0e-\x1f\x7f]/g, '');
  // Remove incomplete oldest line when bounded storage discarded its prefix.
  if (truncated) text = text.includes('\n') ? text.slice(text.indexOf('\n') + 1) : '';
  return text;
}
export function createNativeTerminalProbe() {
  let retained = Buffer.alloc(0), dropped = false;
  const metadata = { frames: 0, stdoutFrames: 0, stdoutBytes: 0, otherFrames: 0, exitFrames: 0, closed: false, socketError: false, trustEnterSent: false, firstOutputAt: null, lastOutputAt: null, exitObservedAt: null, exit: { classification: 'not-observed' } };
  const seen = classifyNativeTerminal('');
  return {
    frame(bytes) {
      metadata.frames++;
      if (bytes[0] === 3) { metadata.exitFrames++; metadata.exitObservedAt ??= new Date().toISOString(); metadata.exit = fixedExitReason(bytes.subarray(1).toString('utf8')); return; }
      if (bytes[0] !== 0) { metadata.otherFrames++; return; }
      metadata.stdoutFrames++; metadata.stdoutBytes += bytes.length - 1;
      metadata.firstOutputAt ??= new Date().toISOString(); metadata.lastOutputAt = new Date().toISOString();
      const payload = bytes.subarray(1);
      if (payload.length >= maximumBufferBytes) {
        dropped ||= retained.length > 0 || payload.length > maximumBufferBytes;
        retained = Buffer.from(payload.subarray(payload.length - maximumBufferBytes));
      } else {
        const remaining = maximumBufferBytes - payload.length;
        if (retained.length > remaining) { retained = retained.subarray(retained.length - remaining); dropped = true; }
        retained = Buffer.concat([retained, payload], retained.length + payload.length);
      }
      const current = classifyNativeTerminal(boundedText(retained, dropped));
      for (const [name, value] of Object.entries(current)) seen[name] ||= value;
    },
    close(code, clean) { metadata.closed = true; metadata.closeCode = Number.isInteger(code) ? code : null; metadata.closeWasClean = !!clean; },
    error() { metadata.socketError = true; },
    trustSent() { metadata.trustEnterSent = true; },
    snapshot() { return { ...metadata, exit: { ...metadata.exit }, retainedBufferBytes: retained.length, retainedBackingBytes: retained.buffer.byteLength, bufferTruncated: dropped, observed: { ...seen }, retainedTail: classifyNativeTerminal(boundedText(retained, dropped)) }; },
    privateText(secrets, privateRoot) {
      let text = boundedText(retained, dropped);
      for (const secret of secrets) if (typeof secret === 'string' && secret.length) text = text.replaceAll(secret, '[redacted]');
      if (privateRoot) text = text.replaceAll(privateRoot, '[private-fixture]');
      // Also mask common credential assignments should an upstream error echo
      // its environment; never retain arbitrary auth headers or JWTs.
      text = text.replace(/\b(?:Bearer|Basic)\s+[^\s"',;]+/gi, '[redacted-authorization]');
      text = text.replace(/["']?\b(?:OPENAI_API_KEY|CODEX_API_KEY|api[_-]?key|access_token|refresh_token|authorization|proxy-authorization|password|auth_token|token)["']?\s*[=:]\s*(?:"[^"\r\n]*"|'[^'\r\n]*'|[^\s,;]+)/gi, '[redacted-credential]');
      text = text.replace(/\beyJ[A-Za-z0-9_-]{12,}\.[A-Za-z0-9_-]+\.[A-Za-z0-9_-]+\b/g, '[redacted-jwt]');
      let bytes = Buffer.from(text);
      if (bytes.length > maximumArtifactBytes) {
        bytes = bytes.subarray(bytes.length - maximumArtifactBytes);
        const newline = bytes.indexOf(10); bytes = newline >= 0 ? bytes.subarray(newline + 1) : Buffer.alloc(0);
      }
      return bytes;
    },
  };
}
