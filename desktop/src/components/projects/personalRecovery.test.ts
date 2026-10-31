import { beforeEach, expect, it } from 'vitest';
import { readPersonalRecovery, savePersonalRecovery } from './personalRecovery';
beforeEach(() => localStorage.clear());
it('retains an original uncertain request and never substitutes another tab or actor', () => {
  const initial = readPersonalRecovery('alice');
  const pending = { expectedRevision: 2, expectedContextDigest: 'a'.repeat(64), idempotencyKey: 'original-private-request-123456', message: 'Keep the original' };
  savePersonalRecovery('alice', initial, { message: pending.message, pending });
  expect(readPersonalRecovery('alice').pending).toEqual(pending); expect(readPersonalRecovery('bob').pending).toBeNull();
  expect(() => savePersonalRecovery('alice', initial, { message: 'A different draft', pending: null })).toThrow(/Another tab/);
  expect(readPersonalRecovery('alice').pending).toEqual(pending);
});
