import { afterEach, describe, expect, it, vi } from 'vitest';
import { productApi } from './product';
import { approvedContractReference, orchestrationContractsApi, ownedOrchestrationContract } from './orchestrationContracts';
import { contractPrepareFixture, contractViewFixture } from '../test/orchestrationContractFixtures';
afterEach(() => vi.restoreAllMocks());
describe('orchestration contract public client', () => {
  it('uses exact prepare route, selected Ask mode and revision/digest approval', async () => {
    const request = vi.spyOn(productApi, 'request').mockResolvedValueOnce(contractViewFixture()).mockResolvedValueOnce(contractViewFixture(true));
    const input = contractPrepareFixture();
    const prepared = await orchestrationContractsApi.prepare('p-1', input);
    expect(request.mock.calls[0]).toEqual(['/v1/product/projects/p-1/orchestration-contracts/prepare', input]);
    await orchestrationContractsApi.approve('p-1', prepared);
    expect(request.mock.calls[1]).toEqual(['/v1/product/projects/p-1/orchestration-contracts/contract-1/approve', { expectedRevision: 1, expectedDigest: 'a'.repeat(64) }]);
    expect(input.route.permissionMode).toBe('ask');
  });
  it('refuses cross-project/identity and malformed approved receipts', () => {
    const wrong = contractViewFixture(); wrong.contract.projectId = 'other';
    expect(() => ownedOrchestrationContract('p-1', wrong)).toThrow();
    expect(() => ownedOrchestrationContract('p-1', contractViewFixture(), 'other')).toThrow();
    const forged = contractViewFixture(true); forged.contract.approval!.digest = 'b'.repeat(64);
    expect(() => ownedOrchestrationContract('p-1', forged)).toThrow();
  });
  it('never treats preparation or stale approval as a runnable contract reference', () => {
    expect(() => approvedContractReference(contractViewFixture())).toThrow();
    const stale = contractViewFixture(true); stale.freshness = { status: 'changed', reason: 'Design changed' };
    expect(() => approvedContractReference(stale)).toThrow('Design changed');
    expect(approvedContractReference(contractViewFixture(true))).toEqual({ id: 'contract-1', revision: 2, digest: 'a'.repeat(64), approvalId: 'approval-1' });
  });
});
