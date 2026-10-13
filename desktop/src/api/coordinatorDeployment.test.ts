import { afterEach, expect, it, vi } from 'vitest';
import { productApi } from './product';
import { coordinatorDeploymentApi, coordinatorDeploymentPlanValid, sameCoordinatorDeploymentBinding, type CoordinatorDeploymentInput, type CoordinatorDeploymentPlan } from './coordinatorDeployment';
const input = (): CoordinatorDeploymentInput => ({ targetId: 'target-1', repository: 'https://github.com/team/app', branch: 'main', domain: 'https://app.example.com', port: 3000, healthPath: '/health', dockerfile: 'Dockerfile', source: 'public', dataClientFramework: 'none' });
const prepared = (): CoordinatorDeploymentPlan => ({ ...input(), targetFingerprint: 'a'.repeat(64), serverUuid: 'server-1', expectedConfigurationRevision: 3, expectedDataDomainRevision: 0, expectedDataBindingFingerprint: '', acknowledgeRepositoryPush: false, acknowledgePublic: false, acknowledgeDeploymentCosts: false });
const approved = (): CoordinatorDeploymentPlan => ({ ...prepared(), acknowledgeRepositoryPush: true, acknowledgePublic: true, acknowledgeDeploymentCosts: true });
afterEach(() => vi.restoreAllMocks());
it('prepares an exact flat plan with all three consents false, preserving cancellation and performing no provider mutation', async () => {
  const request = vi.spyOn(productApi, 'request').mockResolvedValue(prepared()), signal = new AbortController().signal;
  expect(await coordinatorDeploymentApi.prepare('p/one', approved(), signal)).toEqual(prepared());
  expect(request).toHaveBeenCalledExactlyOnceWith('/v1/product/projects/p%2Fone/orchestrator/deployment-plan', { ...approved(), acknowledgeRepositoryPush: false, acknowledgePublic: false, acknowledgeDeploymentCosts: false }, 'POST', 30000, signal);
});
it.each([
  { targetId: 'other' }, { repository: 'https://github.com/other/app' }, { branch: 'release' }, { domain: 'https://other.example.com' },
  { port: 4000 }, { healthPath: '/other' }, { dockerfile: 'Otherfile' }, { source: 'private', privateKeyUuid: 'key-1' }, { dataClientFramework: 'vite' },
  { acknowledgeRepositoryPush: true }, { acknowledgePublic: true }, { acknowledgeDeploymentCosts: true }, { acknowledgePublic: undefined },
  { targetFingerprint: 'unbound' }, { serverUuid: '' }, { expectedConfigurationRevision: -1 }, { expectedConfigurationRevision: 0.5 },
  { expectedConfigurationRevision: 1_000_000_000 }, { expectedDataDomainRevision: 0, expectedDataBindingFingerprint: 'b'.repeat(64) },
  { expectedDataDomainRevision: 1, expectedDataBindingFingerprint: '' }, { expectedDataDomainRevision: -1 }, { accessToken: 'unexpected' },
  { privateKeyUuid: null }, { privateKeyUuid: false }, { privateKeyUuid: 0 },
])('rejects changed or untrusted preparation fields %j', async patch => {
  vi.spyOn(productApi, 'request').mockResolvedValue({ ...prepared(), ...patch });
  await expect(coordinatorDeploymentApi.prepare('p-1', input())).rejects.toThrow(/did not match/);
});
it('accepts an exact private key reference and current nonempty Data binding without granting consent', async () => {
  const value = { ...prepared(), source: 'private' as const, privateKeyUuid: 'key-1', expectedDataDomainRevision: 4, expectedDataBindingFingerprint: 'b'.repeat(64) };
  vi.spyOn(productApi, 'request').mockResolvedValue(value);
  expect(await coordinatorDeploymentApi.prepare('p-1', { ...input(), source: 'private', privateKeyUuid: 'key-1' })).toEqual(value);
  expect(coordinatorDeploymentPlanValid(value)).toBe(false);
  expect(coordinatorDeploymentPlanValid({ ...value, acknowledgeRepositoryPush: true, acknowledgePublic: true, acknowledgeDeploymentCosts: true })).toBe(true);
});
it.each([{ repository: 'https://user:secret@github.com/team/app' }, { repository: 'https://github.com/team/app?token=value' }, { domain: 'http://app.example.com' }, { healthPath: '//other/health' }, { healthPath: '/health?token=value' }, { dockerfile: '../Dockerfile' }, { dockerfile: '.ssh/id_rsa' }, { dockerfile: 'C:\\Dockerfile' }, { source: 'private', privateKeyUuid: '' }, { privateKeyUuid: null }, { port: 65536 }, { unknown: true }])('refuses invalid inputs before a request %j', async patch => {
  const request = vi.spyOn(productApi, 'request');
  await expect(coordinatorDeploymentApi.prepare('p-1', { ...input(), ...patch } as CoordinatorDeploymentInput)).rejects.toThrow(/Choose a valid/);
  expect(request).not.toHaveBeenCalled();
});
it('does not start an already-cancelled read or return a late cancelled response', async () => {
  const request = vi.spyOn(productApi, 'request'), controller = new AbortController(); controller.abort();
  await expect(coordinatorDeploymentApi.prepare('p-1', input(), controller.signal)).rejects.toHaveProperty('name', 'AbortError');
  expect(request).not.toHaveBeenCalled();
  const active = new AbortController(); request.mockImplementation(async () => { active.abort(); return prepared(); });
  await expect(coordinatorDeploymentApi.prepare('p-1', input(), active.signal)).rejects.toHaveProperty('name', 'AbortError');
});
it.each(['targetId', 'repository', 'branch', 'domain', 'port', 'healthPath', 'dockerfile', 'source', 'privateKeyUuid', 'dataClientFramework', 'targetFingerprint', 'serverUuid', 'expectedConfigurationRevision', 'expectedDataDomainRevision', 'expectedDataBindingFingerprint'] as const)('binds saved delegation to current %s', field => {
  const current = prepared(), saved = approved();
  expect(sameCoordinatorDeploymentBinding(current, saved)).toBe(true);
  const changed = { ...current, [field]: typeof current[field] === 'number' ? Number(current[field]) + 1 : `${current[field] ?? ''}changed` };
  expect(sameCoordinatorDeploymentBinding(changed, saved)).toBe(false);
});
it('refuses unacknowledged or extra-field saved plans rather than silently dropping fields', () => {
  expect(coordinatorDeploymentPlanValid(prepared())).toBe(false);
  expect(coordinatorDeploymentPlanValid(approved())).toBe(true);
  expect(coordinatorDeploymentPlanValid({ ...approved(), extra: true })).toBe(false);
  expect(sameCoordinatorDeploymentBinding(prepared(), { ...approved(), extra: true } as CoordinatorDeploymentPlan)).toBe(false);
});
it('reads connected targets explicitly with the signal and rejects duplicate or malformed identities', async () => {
  const target = { id: 'target-1', name: 'Existing server', baseUrl: 'https://deploy.example.com', serverUuid: 'server-1' }, signal = new AbortController().signal;
  const request = vi.spyOn(productApi, 'request').mockResolvedValue({ targets: [target] });
  expect(await coordinatorDeploymentApi.targets(signal)).toEqual([target]);
  expect(request).toHaveBeenCalledExactlyOnceWith('/v1/setup/targets', undefined, 'GET', 15000, signal);
  request.mockResolvedValue({ targets: [target, target] }); await expect(coordinatorDeploymentApi.targets()).rejects.toThrow(/could not be verified/);
  request.mockResolvedValue({ targets: [{ ...target, secret: 'unexpected' }] }); await expect(coordinatorDeploymentApi.targets()).rejects.toThrow(/could not be verified/);
});
