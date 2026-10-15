import { StrictMode } from 'react';
import { fireEvent, render, screen, waitFor } from '@testing-library/react';
import { beforeEach, expect, it, vi } from 'vitest';
import { orchestrationContractsApi } from '../../api/orchestrationContracts';
import { contractPrepareFixture, contractViewFixture } from '../../test/orchestrationContractFixtures';
import { archiveContractReview, contractReviewKey, ExecutionContractReview, readContractReview, retainContractReview, type ExecutionContractReviewRequest } from './ExecutionContractReview';

const pending = (): ExecutionContractReviewRequest => ({ version: 1, projectId: 'p-1', actorId: 'owner', request: contractPrepareFixture(), runKey: 'save-run-request-123456', contractId: 'contract-1' });
beforeEach(() => { vi.restoreAllMocks(); localStorage.clear(); });
function props() { return { pending: pending(), initialView: contractViewFixture(), scope: 'owner/workspace/project', actorId: 'owner', canRead: true, canEdit: true,
  onPrepared: vi.fn(), onCreate: vi.fn().mockResolvedValue(undefined), onComplete: vi.fn(), onReviewInputs: vi.fn() }; }

it('requires explicit review and saves one approved reference without starting coding', async () => {
  const p = props(); retainContractReview(p.scope, p.pending);
  vi.spyOn(orchestrationContractsApi, 'get').mockResolvedValue(contractViewFixture());
  const approve = vi.spyOn(orchestrationContractsApi, 'approve').mockResolvedValue(contractViewFixture(true));
  render(<ExecutionContractReview {...p} />);
  const button = screen.getByRole('button', { name: 'Approve contract and save project run' }) as HTMLButtonElement;
  expect(button.disabled).toBe(true);
  expect(screen.getByText(/not been assessed against a structured control library/)).toBeTruthy();
  fireEvent.click(screen.getByRole('checkbox')); fireEvent.click(button);
  await waitFor(() => expect(p.onCreate).toHaveBeenCalledTimes(1));
  expect(approve).toHaveBeenCalledTimes(1);
  expect(p.onCreate.mock.calls[0]).toEqual([p.pending.request, { id: 'contract-1', revision: 2, digest: 'a'.repeat(64), approvalId: 'approval-1' }, p.pending.runKey]);
  await waitFor(() => expect(p.onComplete).toHaveBeenCalledTimes(1));
});
it('shows exact imported requirements and marks document quotes untrusted',()=>{
  const p=props();
  p.initialView.contract.input.standards={status:'team-library-assessed',source:'team-control-library',revision:1,digest:'b'.repeat(64),teamId:'team-one',assessmentRevision:1,assessmentDigest:'c'.repeat(64),contentDigest:'d'.repeat(64),requirements:[{id:'SEC-1',requirement:'Require violet ledger review',category:'Security',applicability:'Every release',owner:'Team',evidenceExpected:'Reviewer receipt',assessmentStatus:'supported',evidence:[{link:{documentId:'doc-one',originalDigest:'e'.repeat(64),extractionRevision:1,page:1,segmentId:'fact',textDigest:'f'.repeat(64),targetKind:'purpose',targetId:''},text:'Exact reviewed evidence'}]}]};
  render(<ExecutionContractReview {...p}/>);
  expect(screen.getByText(/Team library revision 1, assessment revision 1/)).toBeTruthy();
  fireEvent.click(screen.getByText('Imported control requirements and evidence · 1'));
  expect(screen.getByText(/Require violet ledger review/)).toBeTruthy();
  expect(screen.getByText(/Untrusted source quote: Exact reviewed evidence/)).toBeTruthy();
});
it('blocks a changed Design discovered at the actual approval boundary', async () => {
  const p = props(), stale = contractViewFixture(); stale.freshness = { status: 'changed', reason: 'Design changed' };
  vi.spyOn(orchestrationContractsApi, 'get').mockResolvedValue(stale);
  const approve = vi.spyOn(orchestrationContractsApi, 'approve');
  render(<ExecutionContractReview {...p} />); fireEvent.click(screen.getByRole('checkbox'));
  fireEvent.click(screen.getByRole('button', { name: 'Approve contract and save project run' }));
  await screen.findByRole('alert'); expect(approve).not.toHaveBeenCalled(); expect(p.onCreate).not.toHaveBeenCalled();
});
it('recovers a known approval by read only after reload/StrictMode, without automatically creating a run', async () => {
  const p = props();
  const get = vi.spyOn(orchestrationContractsApi, 'get').mockResolvedValue(contractViewFixture(true));
  const approve = vi.spyOn(orchestrationContractsApi, 'approve');
  render(<StrictMode><ExecutionContractReview {...p} initialView={null} /></StrictMode>);
  await screen.findByRole('button', { name: 'Save reviewed project run' });
  expect(get).toHaveBeenCalledTimes(1); expect(approve).not.toHaveBeenCalled(); expect(p.onCreate).not.toHaveBeenCalled();
});
it('keeps the exact create key after an interrupted response and never fabricates success', async () => {
  const p = props(); p.initialView = contractViewFixture(true); p.onCreate.mockRejectedValue(new Error('Response interrupted'));
  vi.spyOn(orchestrationContractsApi, 'get').mockResolvedValue(contractViewFixture(true));
  render(<ExecutionContractReview {...p} />); fireEvent.click(screen.getByRole('checkbox'));
  fireEvent.click(screen.getByRole('button', { name: 'Save reviewed project run' }));
  await screen.findByRole('alert'); expect(p.onComplete).not.toHaveBeenCalled();
  expect(readContractReview(p.scope, 'p-1', 'owner')?.runKey).toBe(p.pending.runKey);
});
it('does not replace an unresolved request from another tab or expose another actor recovery', () => {
  const original = pending(); retainContractReview('scope', original);
  expect(() => retainContractReview('scope', { ...original, runKey: 'another-save-key-1234' })).toThrow();
  expect(() => readContractReview('scope', 'p-1', 'other-person')).toThrow();
});
it('requires current edit authority and successful recovery storage before approval', async () => {
  const p = props(); const approve = vi.spyOn(orchestrationContractsApi, 'approve');
  const view = render(<ExecutionContractReview {...p} canEdit={false} />);
  expect((screen.getByRole('checkbox') as HTMLInputElement).disabled).toBe(true);
  view.rerender(<ExecutionContractReview {...p} />);
  vi.spyOn(Storage.prototype, 'setItem').mockImplementation(() => { throw new Error('storage denied'); });
  fireEvent.click(screen.getByRole('checkbox')); fireEvent.click(screen.getByRole('button', { name: 'Approve contract and save project run' }));
  await screen.findByRole('alert'); expect(approve).not.toHaveBeenCalled(); expect(p.onCreate).not.toHaveBeenCalled();
});
it('preserves an original uncertain request before allowing genuinely updated plan input', () => {
  const original = pending(); retainContractReview('scope', original);
  const raw = localStorage.getItem(contractReviewKey('scope'));
  archiveContractReview('scope', original);
  expect(readContractReview('scope', 'p-1', 'owner')).toBeNull();
  expect(localStorage.getItem(contractReviewKey('scope') + ':archive:' + original.request.idempotencyKey)).toBe(raw);
});
