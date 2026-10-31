import type { PersonalRequest } from '../../api/personalLuci';
import { recoveryScope } from '../orchestrator/composerRecovery';

interface Recovery { message: string; pending: PersonalRequest | null }
const key = (scope: string) => 'luci.personal-composer.v1:' + encodeURIComponent(recoveryScope(scope));
export function readPersonalRecovery(scope: string): Recovery {
  const raw = localStorage.getItem(key(scope));
  if (!raw) return { message: '', pending: null };
  if (raw.length > 64 * 1024) throw new Error('Your saved message exceeds the browser recovery limit. Keep a copy before continuing.');
  const value = JSON.parse(raw) as Recovery;
  if (!value || typeof value.message !== 'string' || value.message.length > 16000 || value.pending &&
    (!/^[!-~]{16,128}$/.test(value.pending.idempotencyKey) || !/^[a-f0-9]{64}$/.test(value.pending.expectedContextDigest) || !Number.isSafeInteger(value.pending.expectedRevision) || value.pending.expectedRevision < 0 || value.pending.message !== value.message)) throw new Error('Your saved message could not be verified. Keep a copy before continuing.');
  return value;
}
export function savePersonalRecovery(scope: string, expected: Recovery, next: Recovery) {
  // Another tab's uncertain request is never overwritten or silently replayed.
  if (JSON.stringify(readPersonalRecovery(scope)) !== JSON.stringify(expected)) throw new Error('Another tab changed your message. Keep your draft and refresh before sending.');
  localStorage.setItem(key(scope), JSON.stringify(next));
}
