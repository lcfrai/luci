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
  it('accepts an actual saved task with omitted optional graph fields and keeps requirement criteria', () => {
    // Exercise the JSON round trip and Go's omitempty shape, rather than a
    // frontend-only class instance with every optional field populated.
    const fromGo = JSON.parse(JSON.stringify(contractViewFixture())) as ReturnType<typeof contractViewFixture>;
    fromGo.contract.input.tasks = [{ id: 'task-1', title: 'Build journal' }];
    expect(ownedOrchestrationContract('p-1', fromGo).contract.input.requirements[0].acceptance).toBe('Survive reload');
    fromGo.contract.input.requirements = [];
    expect(() => ownedOrchestrationContract('p-1', fromGo)).toThrow();
    const obsoleteFlat = contractViewFixture() as unknown as { contract: Record<string, unknown> };
    Object.assign(obsoleteFlat.contract, obsoleteFlat.contract.input);
    delete obsoleteFlat.contract.input;
    expect(() => ownedOrchestrationContract('p-1', obsoleteFlat as unknown as ReturnType<typeof contractViewFixture>)).toThrow();
  });
  it('never treats preparation or stale approval as a runnable contract reference', () => {
    expect(() => approvedContractReference(contractViewFixture())).toThrow();
    const stale = contractViewFixture(true); stale.freshness = { status: 'changed', reason: 'Design changed' };
    expect(() => approvedContractReference(stale)).toThrow('Design changed');
    expect(approvedContractReference(contractViewFixture(true))).toEqual({ id: 'contract-1', revision: 2, digest: 'a'.repeat(64), approvalId: 'approval-1' });
  });
  it('accepts bounded imported control content and rejects an identity-only team binding',()=>{
    const view=contractViewFixture();
    view.contract.input.standards={status:'team-library-assessed',source:'team-control-library',revision:2,digest:'b'.repeat(64),teamId:'team-one',assessmentRevision:1,assessmentDigest:'c'.repeat(64),contentDigest:'d'.repeat(64),requirements:[{id:'SEC-1',requirement:'Require violet ledger review before release',category:'Security',applicability:'Every release',owner:'Team',evidenceExpected:'Signed reviewer receipt',assessmentStatus:'supported',evidence:[{link:{documentId:'doc-1',originalDigest:'e'.repeat(64),extractionRevision:1,page:1,segmentId:'fact',textDigest:'f'.repeat(64),targetKind:'purpose',targetId:''},text:'Reviewed violet ledger evidence'}]}]};
    const fromGo=JSON.parse(JSON.stringify(view)) as typeof view;
    expect(ownedOrchestrationContract('p-1',fromGo).contract.input.standards.requirements?.[0].requirement).toContain('violet ledger');
    fromGo.contract.input.standards.requirements=[];
    expect(()=>ownedOrchestrationContract('p-1',fromGo)).toThrow();
  });
});
