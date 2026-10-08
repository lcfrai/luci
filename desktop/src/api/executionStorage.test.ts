import sharedHTTP from '../../../control-plane/internal/management/testdata/execution-storage-http.json?raw';
import { afterEach, expect, it, vi } from 'vitest';
import { createExecutionStorageClient, type ExecutionStorage } from './executionStorage';
import { productApi } from './product';
const value = (): ExecutionStorage => ({ platform: 'windows', scope: 'local-docker-desktop', required: true, state: 'confirmation-required', revision: 4, reasonCode: 'confirm', message: 'Review the active file.', basis: 'Operator-confirmed and measured.' });
afterEach(() => vi.restoreAllMocks());
it('reads with cancellation and sends exactly the reviewed host path/revision and affirmative confirmation', async () => {
  const request = vi.spyOn(productApi, 'request').mockResolvedValue(value()), api = createExecutionStorageClient();
  const signal = new AbortController().signal; await api.get(signal);
  expect(request.mock.lastCall).toEqual(['/v1/admin/execution-storage', undefined, 'GET', 30_000, signal]);
  const path = String.raw`D:\Docker\docker_data.vhdx`; await api.confirm(value(), path);
  expect(request.mock.lastCall).toEqual(['/v1/admin/execution-storage', { expectedRevision: 4, vhdPath: path, confirmActiveAndPendingLocal: true }, 'POST']);
  expect(request).toHaveBeenCalledTimes(2);
});
it('accepts measured zero and absent observations while rejecting malformed read and write receipts', async () => {
  const request = vi.spyOn(productApi, 'request'), api = createExecutionStorageClient();
  request.mockResolvedValue({ ...value(), hostAvailableBytes: 0 });
  expect((await api.get()).hostAvailableBytes).toBe(0);
  for (const patch of [{ revision: -1 }, { hostAvailableBytes: -1 }, { guestAvailableBytes: NaN }, { state: 'ready' }, { vhdPath: 4 }]) {
    request.mockResolvedValue({ ...value(), ...patch });
    await expect(api.get()).rejects.toThrow('could not be read');
    await expect(api.confirm(value(), 'path')).rejects.toThrow('could not be read');
  }
});
it('never retries a confirmation after an interrupted response', async () => {
  const request = vi.spyOn(productApi, 'request').mockRejectedValue(new Error('Network lost.'));
  await expect(createExecutionStorageClient().confirm(value(), 'path')).rejects.toThrow('Network lost');
  expect(request).toHaveBeenCalledOnce();
});

it('requires an actual ready Windows storage check while leaving other hosts unchanged', async () => {
  const { storagePreparationBlock } = await import('./executionStorage');
  expect(storagePreparationBlock({ platform: 'linux', checks: [] })).toBe('');
  expect(storagePreparationBlock({ platform: 'windows', workspace: { mode: 'cloud', connectionId: 'remote', host: 'linux-host', ready: true }, checks: [] })).toBe('');
  expect(storagePreparationBlock({ platform: 'windows', checks: [] })).toContain('Check Docker storage readiness');
  expect(storagePreparationBlock({ platform: 'windows', checks: [{ id: 'docker-backing-storage', label: 'Storage', status: 'ready', message: '', action: '' }] })).toBe('');
  expect(storagePreparationBlock({ platform: 'windows', checks: [{ id: 'docker-backing-storage', label: 'Storage', status: 'missing', message: 'Free physical disk space first.', action: '' }] })).toBe('Free physical disk space first.');
});

it('decodes the same complete GET/POST bodies emitted by the management HTTP handler contract test', async () => {
  const bodies = JSON.parse(sharedHTTP) as Record<'notRequired' | 'pendingObservation' | 'confirmed', ExecutionStorage>;
  const request = vi.spyOn(productApi, 'request'), api = createExecutionStorageClient();
  request.mockResolvedValue(bodies.notRequired);
  expect(await api.get()).toEqual(bodies.notRequired);
  request.mockResolvedValue(bodies.pendingObservation);
  expect(await api.confirm({ ...bodies.pendingObservation, revision: 0 }, bodies.pendingObservation.vhdPath!)).toEqual(bodies.pendingObservation);
  request.mockResolvedValue(bodies.confirmed);
  const confirmed = await api.get();
  expect(confirmed).toEqual(bodies.confirmed);
  expect(confirmed.hostAvailableBytes).toBe(0);
  expect(confirmed.guestAvailableBytes).toBeGreaterThan(0);
});
