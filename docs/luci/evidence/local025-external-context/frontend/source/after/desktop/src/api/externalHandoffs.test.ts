import { afterEach, expect, it, vi } from 'vitest';
import { externalHandoffsApi, parseExternalReport, type ExternalHandoff } from './externalHandoffs';
import { productApi } from './product';
const handoff = { id: 'handoff-a', projectId: 'p-a', digest: 'a'.repeat(64), implementation: { projectId: 'p-a' }, files: [], instructions: 'Read HANDOFF.md', returns: [] } as unknown as ExternalHandoff;
const report = { projectId: 'p-a', handoffId: 'handoff-a', handoffDigest: handoff.digest, summary: 'Built calculator', checks: [{ command: 'npm test', outcome: 'passed', output: '3 tests passed' }], notes: '' };
afterEach(() => vi.restoreAllMocks());
it('encodes project routes and records exact reviewed provenance with an idempotency key', async () => {
  const request = vi.spyOn(productApi, 'request').mockResolvedValue({ id: 'report-1', idempotencyKey: 'durable-request-key', handoffId: handoff.id, handoffDigest: handoff.digest, evidenceStatus: 'unverified' });
  await externalHandoffsApi.record('p/a', handoff, 'review-sha', report, 'durable-request-key');
  expect(request).toHaveBeenCalledExactlyOnceWith('/v1/product/projects/p%2Fa/external-handoffs/handoff-a/returns', { expectedReviewDigest: 'review-sha', report, idempotencyKey: 'durable-request-key' });
});
it.each(['project', 'handoff', 'digest', 'verified'])('refuses a mismatched %s response before presenting it', async kind => {
  const value = { id: 'report-1', idempotencyKey: 'durable-request-key', handoffId: handoff.id, handoffDigest: handoff.digest, evidenceStatus: 'unverified', ...(kind === 'handoff' ? { handoffId: 'foreign' } : kind === 'digest' ? { handoffDigest: 'b'.repeat(64) } : kind === 'verified' ? { evidenceStatus: 'verified' } : {}) };
  vi.spyOn(productApi, 'request').mockResolvedValue(kind === 'project' ? { ...handoff, projectId: 'p-b' } : value);
  if (kind === 'project') await expect(externalHandoffsApi.get('p-a', handoff.id)).rejects.toThrow('selected project');
  else await expect(externalHandoffsApi.record('p-a', handoff, 'review', report, 'key')).rejects.toThrow('selected handoff');
});
it('refuses another handoff review and malformed list before return consent', async () => {
  const request = vi.spyOn(productApi, 'request').mockResolvedValue({ handoffId: 'other', handoffDigest: handoff.digest, reviewDigest: 'b'.repeat(64), contextCurrent: true, changedFiles: [] });
  await expect(externalHandoffsApi.review('p-a', handoff)).rejects.toThrow('selected handoff');
  request.mockResolvedValue({ handoffs: [{ ...handoff, implementation: { projectId: 'p-b' } }] });
  await expect(externalHandoffsApi.list('p-a')).rejects.toThrow('selected project');
});
it('refuses a valid handoff receipt from a different submitted request', async () => {
  vi.spyOn(productApi, 'request').mockResolvedValue({ id: 'report-1', idempotencyKey: 'other-request-key', handoffId: handoff.id, handoffDigest: handoff.digest, evidenceStatus: 'unverified' });
  await expect(externalHandoffsApi.record('p-a', handoff, 'review', report, 'expected-request-key')).rejects.toThrow('submitted request');
});
it('parses only bounded identity-bound report fields and strips executable-looking extras', () => {
  const value = parseExternalReport(JSON.stringify({ ...report, execute: 'rm -rf anything', approved: true }), handoff);
  expect(value).toEqual(report);
  expect(Object.keys(value)).not.toContain('execute');
});
it.each(['projectId', 'handoffId', 'handoffDigest'])('rejects cross-context report %s', field => {
  expect(() => parseExternalReport(JSON.stringify({ ...report, [field]: 'foreign' }), handoff)).toThrow('does not match');
});
it('rejects truncated, oversized and malformed reports before any API request', () => {
  expect(() => parseExternalReport('{', handoff)).toThrow('valid JSON');
  expect(() => parseExternalReport('x'.repeat(128 * 1024 + 1), handoff)).toThrow('128 KiB');
  expect(() => parseExternalReport(JSON.stringify({ ...report, checks: [{ command: 'npm test', outcome: 'passed' }] }), handoff)).toThrow('output');
  expect(() => parseExternalReport(JSON.stringify({ ...report, checks: Array(51).fill(report.checks[0]) }), handoff)).toThrow('50');
});
