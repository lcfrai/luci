import { StrictMode } from 'react';
import { act, fireEvent, render, screen, waitFor } from '@testing-library/react';
import { beforeEach, expect, it, vi } from 'vitest';
import { orchestrationContractsApi } from '../../api/orchestrationContracts';
import { contractPrepareFixture, contractViewFixture } from '../../test/orchestrationContractFixtures';
import { archiveContractReview, contractReviewKey, ExecutionContractReview, readContractReview, retainContractReview, type ExecutionContractReviewRequest } from './ExecutionContractReview';

const pending = (): ExecutionContractReviewRequest => ({ version: 1, projectId: 'p-1', actorId: 'owner', request: contractPrepareFixture(), runKey: 'save-run-request-123456', contractId: 'contract-1' });
beforeEach(() => { vi.restoreAllMocks(); localStorage.clear(); });
function props() { return { pending: pending(), initialView: contractViewFixture(), scope: 'owner/workspace/project', actorId: 'owner', canRead: true, canEdit: true,
  onPrepared: vi.fn(), onCreate: vi.fn().mockResolvedValue(undefined), onComplete: vi.fn(), onReviewInputs: vi.fn() }; }
function nativeProps() {
  const p = props(); p.pending.request.route.nativeDelegation = 'codex-native-helpers-2-v1'; p.pending.request.route.permissionMode = 'bypass'; p.pending.request.route.harness = 'codex';
  Object.assign(p.initialView.contract.input.route, {harness: 'codex', permissionMode: 'bypass', nativeDelegation: 'codex-native-helpers-2-v1', nativeDelegationImage: 'sha256:' + 'a'.repeat(64)});
  return p;
}
it('native helpers show exact captured image and honest parent-only observation before consent', () => {
  const p = nativeProps(); render(<ExecutionContractReview {...p} />);
  expect(screen.getByText('sha256:' + 'a'.repeat(64))).toBeTruthy();
  expect(screen.getByText(/individual helper activity is not fully tracked/)).toBeTruthy();
  expect(screen.getByText(/review or repair tasks receive no helper grant/)).toBeTruthy();
  expect(p.onCreate).not.toHaveBeenCalled();
});
it.each(['read-profile', 'read-image', 'approval-image'])('native helpers refuse changed captured binding at %s without creating', async boundary => {
  const p = nativeProps(); retainContractReview(p.scope, p.pending);
  const changed = structuredClone(p.initialView);
  if (boundary === 'read-profile') { delete changed.contract.input.route.nativeDelegation; delete changed.contract.input.route.nativeDelegationImage; }
  else changed.contract.input.route.nativeDelegationImage = 'sha256:' + 'b'.repeat(64);
  vi.spyOn(orchestrationContractsApi, 'get').mockResolvedValue(boundary === 'approval-image' ? p.initialView : changed);
  const approved = {...changed, contract: {...changed.contract, revision: 2, state: 'approved' as const, approval: {id: 'approval-1', actor: {id: 'owner', displayName: 'Owner'}, at: '2026-10-16T00:00:00Z', digest: changed.contract.digest}}};
  const approve = vi.spyOn(orchestrationContractsApi, 'approve').mockResolvedValue(approved);
  render(<ExecutionContractReview {...p} />);
  fireEvent.click(screen.getByRole('checkbox')); fireEvent.click(screen.getByRole('button', {name: 'Approve contract and save project run'}));
  await screen.findByRole('alert'); expect(p.onCreate).not.toHaveBeenCalled();
  if (boundary !== 'approval-image') expect(approve).not.toHaveBeenCalled();
  expect(readContractReview(p.scope, 'p-1', 'owner')?.request.route.nativeDelegation).toBe('codex-native-helpers-2-v1');
});
it('native helpers preserve explicit recovery intent and reject caller image or changed continuation profile', () => {
  const p = nativeProps(); retainContractReview(p.scope, p.pending);
  expect(readContractReview(p.scope, 'p-1', 'owner')?.request.route.nativeDelegation).toBe('codex-native-helpers-2-v1');
  const forged = structuredClone(p.pending); Object.assign(forged.request.route, {nativeDelegationImage: 'sha256:' + 'a'.repeat(64)});
  localStorage.setItem(contractReviewKey(p.scope), JSON.stringify(forged));
  expect(() => readContractReview(p.scope, 'p-1', 'owner')).toThrow(/could not be verified/);
  const mismatch = structuredClone(p.pending);
  mismatch.continuation = {idempotencyKey: mismatch.runKey, originTurnId: 'turn-1', expectedContextDigest: 'a'.repeat(64), expectedPolicyRevision: 1, maxParallel: 1,
    run: {worktreeId: mismatch.request.worktreeId, harness: 'codex', model: mismatch.request.route.model, permissionMode: 'bypass', outcome: mismatch.request.outcome, implementationContext: mismatch.request.implementationContext}};
  localStorage.setItem(contractReviewKey(p.scope), JSON.stringify(mismatch));
  expect(() => readContractReview(p.scope, 'p-1', 'owner')).toThrow(/could not be verified/);
});
it.each(['access', 'image'])('native helpers invalidate an awaited approval read across %s change and return', async boundary => {
  const p = nativeProps(); retainContractReview(p.scope, p.pending);
  let finish!: (value: typeof p.initialView) => void;
  vi.spyOn(orchestrationContractsApi, 'get').mockImplementationOnce(() => new Promise(resolve => { finish = resolve; }));
  const approve = vi.spyOn(orchestrationContractsApi, 'approve');
  const ui = render(<ExecutionContractReview {...p} />);
  fireEvent.click(screen.getByRole('checkbox')); fireEvent.click(screen.getByRole('button', {name: 'Approve contract and save project run'}));
  await waitFor(() => expect(finish).toBeDefined());
  const changed = structuredClone(p.initialView); changed.contract.input.route.nativeDelegationImage = 'sha256:' + 'b'.repeat(64);
  ui.rerender(<ExecutionContractReview {...p} {...(boundary === 'access' ? {canEdit: false} : {initialView: changed})} />);
  ui.rerender(<ExecutionContractReview {...p} />);
  await act(async () => finish(p.initialView));
  expect(approve).not.toHaveBeenCalled(); expect(p.onCreate).not.toHaveBeenCalled();
  expect(screen.getByRole('checkbox')).toHaveProperty('checked', false);
});
it.each([0, 1, 2])('shows exact captured repair budget%i before approval without consuming the first or final review', automaticRepairLimit => {
  const p = props(); p.initialView.contract.input.controls.automaticRepairLimit = automaticRepairLimit;
  render(<ExecutionContractReview {...p} />);
  const term = screen.getByText('Automatic repairs per coding task');
  expect(term.nextElementSibling?.textContent).toMatch(new RegExp('^' + automaticRepairLimit + ' ·'));
  expect(term.nextElementSibling?.textContent).toContain('final independent review');
  if (automaticRepairLimit === 0) expect(term.nextElementSibling?.textContent).toContain('Stop after the first failed independent review');
  expect(screen.getByRole('button', {name: 'Approve contract and save project run'})).toHaveProperty('disabled', true);
  expect(p.onCreate).not.toHaveBeenCalled();
});
it('displays legacy effective two without changing the retained contract', () => {
  const p = props(), bytes = JSON.stringify(p.initialView);
  render(<ExecutionContractReview {...p} />);
  expect(screen.getByText(/Retained contract uses the legacy default of two/)).toBeTruthy();
  expect(JSON.stringify(p.initialView)).toBe(bytes);
});

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

it('route approval issue preserves refresh and updated-input recovery on a current contract', async () => {
  const p = props(); retainContractReview(p.scope, p.pending);
  vi.spyOn(orchestrationContractsApi, 'get').mockResolvedValue(p.initialView);
  const approve = vi.spyOn(orchestrationContractsApi, 'approve');
  render(<ExecutionContractReview {...p} approvalIssue="Shared managed credentials require one slot." />);
  expect(screen.getByRole('checkbox')).toHaveProperty('disabled', true);
  expect(screen.getByRole('button', {name: 'Approve contract and save project run'})).toHaveProperty('disabled', true);
  await fireEvent.click(screen.getByRole('button', {name: 'Refresh contract status'}));
  await waitFor(() => expect(p.onPrepared).toHaveBeenCalledTimes(1));
  fireEvent.click(screen.getByRole('button', {name: 'Keep original request and review updated plan'}));
  expect(p.onReviewInputs).toHaveBeenCalledTimes(1); expect(approve).not.toHaveBeenCalled(); expect(p.onCreate).not.toHaveBeenCalled();
  expect(localStorage.getItem(contractReviewKey(p.scope) + ':archive:' + p.pending.request.idempotencyKey)).not.toBeNull();
});

it.each(['read', 'approval'])('route approval issue arriving during awaited %s prevents the next write and retains exact recovery', async boundary => {
  const p = props(); retainContractReview(p.scope, p.pending);
  let settle!: (value: ReturnType<typeof contractViewFixture>) => void;
  const held = new Promise<ReturnType<typeof contractViewFixture>>(resolve => { settle = resolve; });
  vi.spyOn(orchestrationContractsApi, 'get').mockImplementation(() => boundary === 'read' ? held : Promise.resolve(p.initialView));
  const approve = vi.spyOn(orchestrationContractsApi, 'approve').mockImplementation(() => held);
  const ui = render(<ExecutionContractReview {...p} />);
  fireEvent.click(screen.getByRole('checkbox')); fireEvent.click(screen.getByRole('button', {name: 'Approve contract and save project run'}));
  if (boundary === 'approval') await waitFor(() => expect(approve).toHaveBeenCalledTimes(1));
  ui.rerender(<ExecutionContractReview {...p} approvalIssue="Route no longer supports this concurrency." />);
  settle(contractViewFixture(boundary === 'approval'));
  await waitFor(() => expect(screen.getByRole('button', {name: 'Refresh contract status'})).toHaveProperty('disabled', false));
  if (boundary === 'read') expect(approve).not.toHaveBeenCalled();
  expect(p.onCreate).not.toHaveBeenCalled(); expect(p.onComplete).not.toHaveBeenCalled();
  expect(readContractReview(p.scope, p.pending.projectId, p.pending.actorId)?.runKey).toBe(p.pending.runKey);
});
