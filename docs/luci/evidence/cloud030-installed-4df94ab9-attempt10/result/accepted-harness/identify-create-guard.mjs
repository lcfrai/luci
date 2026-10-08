// Acceptance-helper evidence only. Never persist request bodies or error text.
import assert from 'node:assert/strict';
import {createHash} from 'node:crypto';

// management.opaqueID("user-") takes 22 RawURLEncoding characters. This shape
// check is not authorization: assertCheckLease still binds the exact bootstrap
// owner and project to the one active private check lease.
export function assertBootstrapOwnerID(value) {
  assert.match(value ?? '', /^user-[A-Za-z0-9_-]{22}$/);
}

function failure(error) {
  const result = {code:/^ERR_[A-Z_]{1,64}$/.test(error?.code ?? '') ? error.code : 'identify-guard-failed'};
  if (['Error','AssertionError','TypeError','RangeError','SyntaxError','AbortError'].includes(error?.name)) result.name = error.name;
  result.locations = [...String(error?.stack ?? '').matchAll(/(?:^|[/\\])(dag\.mjs|project-check-guard\.mjs|identify-create-guard\.mjs):(\d{1,7}):(\d{1,7})/g)]
    .slice(0,6).map(([,file,line,column]) => ({file,line:Number(line),column:Number(column)}));
  return result;
}

export function diagnoseCreateGuard({bytes, stage, phase, requestSequence, ordinal, identify, persist}) {
  assert(Buffer.isBuffer(bytes));
  assert.match(stage, /^[a-z0-9-]{1,80}$/);
  assert(['identify-create','before-create'].includes(phase));
  const diagnostic = {version:1, at:new Date().toISOString(), stage, phase,
    bodyBytes:bytes.length, bodySHA256:createHash('sha256').update(bytes).digest('hex'),
    outcome:'pending', forwarded:false};
  for (const [name,value] of Object.entries({requestSequence,ordinal})) {
    if (Number.isSafeInteger(value) && value >= 0) diagnostic[name] = value;
  }
  let kind;
  try {
    kind = identify();
    assert(['worker','check'].includes(kind), 'Only a validated worker or check may continue');
  } catch (error) {
    diagnostic.outcome = 'refused'; diagnostic.failure = failure(error);
    persist(diagnostic); // Synchronous durable checkpoint, before rethrow/forward.
    throw error;
  }
  diagnostic.outcome = 'identified'; diagnostic.kind = kind;
  persist(diagnostic); // This records guard completion, never Docker creation.
  return kind;
}
