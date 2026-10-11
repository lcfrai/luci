import { act, cleanup, fireEvent, render, screen, waitFor } from '@testing-library/react';
import userEvent from '@testing-library/user-event';
import { afterEach, beforeEach, expect, it, vi } from 'vitest';
import { ExternalToolHandoff } from './ExternalToolHandoff';
import { externalHandoffsApi, type ExternalHandoff, type ExternalReturn, type ExternalReturnReview } from '../../api/externalHandoffs';
import { productApi, type Brief, type Design, type ImplementationPlan } from '../../api/product';
import { definitionApi, type ProjectSpecification } from '../../api/definition';
import { designPlanApi, type DesignPlan } from '../../api/designPlan';

const authority = vi.hoisted(() => ({ read: true, edit: true, loading: false, actor: 'owner-a' }));
vi.mock('../../hooks/useProjectAccess', () => ({ useProjectAccess: () => ({ canRead: authority.read, canEdit: authority.edit && !authority.loading, canExecute: false, loading: authority.loading, error: '', message: '', permissions: { read: authority.read, edit: authority.edit, execute: false } }) }));
vi.mock('../../hooks/useProjectDraft', async original => ({ ...await original<typeof import('../../hooks/useProjectDraft')>(), useProjectDraftScope: (project: string) => `${authority.actor}:local:${project}` }));
vi.mock('../../hooks/useUnsavedProjectDraft', () => ({ useUnsavedProjectDraft: vi.fn() }));
const sha = 'a'.repeat(64), reviewSHA = 'b'.repeat(64);
const handoff = (projectId = 'p-a'): ExternalHandoff => ({ id: `handoff-${projectId}`, projectId, createdAt: '2026-10-11T15:00:00Z', digest: sha, returnCount: 0,
  implementation: { projectId, worktreeId: 'primary', briefRevision: 1, specificationRevision: 2, designPlanRevision: 3, planRevision: 4, taskIds: ['task-a'], taskTitles: ['Build calculator'], requirementIds: ['AC-1'], briefDigest: sha, planDigest: sha, specificationDigest: sha, designPlanDigest: sha, sourceCommit: 'c'.repeat(40), sourceDigest: sha, sourceDirty: false },
  files: [{ path: 'HANDOFF.md', sha256: sha, bytes: 130 }], instructions: `Read HANDOFF.md for ${projectId}`, returns: [] });
const report = (projectId = 'p-a') => ({ projectId, handoffId: `handoff-${projectId}`, handoffDigest: sha, summary: 'Built calculator', checks: [{ command: 'npm test', outcome: 'passed', output: '3 assertions' }], notes: 'Keyboard checked manually' });
const review = (overrides: Partial<ExternalReturnReview> = {}): ExternalReturnReview => ({ handoffId: 'handoff-p-a', handoffDigest: sha, contextCurrent: true, contextMessage: 'Approved context is current.', source: { commit: 'd'.repeat(40), sourceDigest: sha, dirty: true }, changedFiles: [{ path: 'src/calculator.ts', status: 'modified' }], reviewDigest: reviewSHA, ...overrides });
const returned = (): ExternalReturn => ({ id: 'return-a', handoffId: 'handoff-p-a', handoffDigest: sha, createdAt: '2026-10-11T15:30:00Z', idempotencyKey: 'reported-request-key', evidenceStatus: 'unverified', source: review().source, changedFiles: review().changedFiles, report: report() });
function deferred<T>() { let resolve!: (value: T) => void, reject!: (error: Error) => void; const promise = new Promise<T>((yes, no) => { resolve = yes; reject = no; }); return { promise, resolve, reject }; }
async function open() { await userEvent.click(screen.getByRole('button', { name: 'Context export and returns' })); await screen.findByText('Tasks to hand over'); await waitFor(() => expect(screen.queryByText('Reading saved context…')).toBeNull()); }
async function choose() { await open(); await userEvent.selectOptions(screen.getByLabelText('Saved handoff'), 'handoff-p-a'); await screen.findByRole('button', { name: 'Download context ZIP' }); }
async function compare() { fireEvent.change(screen.getByLabelText('External report JSON'), { target: { value: JSON.stringify(report()) } }); await userEvent.click(screen.getByRole('button', { name: 'Compare returned work' })); await screen.findByText('src/calculator.ts'); }
async function consent() { await userEvent.click(screen.getByRole('checkbox', { name: /I reviewed these changes/ })); }
beforeEach(() => {
  authority.read = true; authority.edit = true; authority.loading = false; authority.actor = 'owner-a'; localStorage.clear();
  vi.spyOn(productApi, 'brief').mockImplementation(async projectId => ({ projectId, revision: 1, purpose: 'Calculator', criteria: [] }) as unknown as Brief);
  vi.spyOn(productApi, 'design').mockImplementation(async projectId => ({ projectId, revision: 1 }) as Design);
  vi.spyOn(definitionApi, 'get').mockImplementation(async projectId => ({ projectId, revision: 2, briefRevision: 1, status: 'approved' }) as ProjectSpecification);
  vi.spyOn(designPlanApi, 'get').mockImplementation(async projectId => ({ projectId, revision: 3, briefRevision: 1, specificationRevision: 2, sketchRevision: 1, status: 'approved' }) as DesignPlan);
  vi.spyOn(productApi, 'plan').mockImplementation(async projectId => ({ projectId, revision: 4, briefRevision: 1, specificationRevision: 2, tasks: [{ id: 'task-a', title: 'Build calculator', status: 'todo', acceptance: 'Correct arithmetic' }, { id: 'task-b', title: 'Add history', status: 'todo' }] }) as ImplementationPlan);
  vi.spyOn(externalHandoffsApi, 'list').mockImplementation(async project => [handoff(project)]);
  vi.spyOn(externalHandoffsApi, 'get').mockImplementation(async project => handoff(project));
  vi.spyOn(externalHandoffsApi, 'create').mockResolvedValue(handoff());
  vi.spyOn(externalHandoffsApi, 'bundle').mockResolvedValue(new Blob(['archive'], { type: 'application/zip' }));
  vi.spyOn(externalHandoffsApi, 'review').mockResolvedValue(review());
  vi.spyOn(externalHandoffsApi, 'record').mockResolvedValue(returned());
  vi.stubGlobal('URL', class extends URL { static createObjectURL = vi.fn(() => 'blob:fixture'); static revokeObjectURL = vi.fn(); });
  vi.spyOn(HTMLAnchorElement.prototype, 'click').mockImplementation(() => {});
});
afterEach(() => { cleanup(); vi.restoreAllMocks(); vi.unstubAllGlobals(); });

it('lazily reads approved context and captures only explicitly selected saved tasks without execute authority', async () => {
  render(<ExternalToolHandoff projectId="p-a" onReviewPlan={vi.fn()} />);
  expect(externalHandoffsApi.list).not.toHaveBeenCalled(); await open();
  expect((screen.getByRole('button', { name: 'Capture selected context' }) as HTMLButtonElement).disabled).toBe(true);
  await userEvent.click(screen.getByRole('checkbox', { name: /Build calculator/ }));
  await userEvent.click(screen.getByRole('button', { name: 'Capture selected context' }));
  await screen.findByText(/Context captured/);
  expect(externalHandoffsApi.create).toHaveBeenCalledExactlyOnceWith('p-a', { projectId: 'p-a', briefRevision: 1, specificationRevision: 2, designPlanRevision: 3, planRevision: 4, taskIds: ['task-a'] }, expect.any(String));
  expect(externalHandoffsApi.bundle).not.toHaveBeenCalled();
});
it('keeps capture blocked with an actionable current-definition review', async () => {
  vi.mocked(definitionApi.get).mockResolvedValue({ projectId: 'p-a', revision: 2, briefRevision: 1, status: 'draft' } as ProjectSpecification);
  render(<ExternalToolHandoff projectId="p-a" onReviewPlan={vi.fn()} />); await open();
  await userEvent.click(screen.getByRole('checkbox', { name: /Build calculator/ }));
  expect((screen.getByRole('button', { name: 'Capture selected context' }) as HTMLButtonElement).disabled).toBe(true);
  expect(screen.getByRole('button', { name: 'Review definition' })).not.toBeNull(); expect(externalHandoffsApi.create).not.toHaveBeenCalled();
});
it('separates observed changed source from reported check claims and requires explicit consent', async () => {
  render(<ExternalToolHandoff projectId="p-a" onReviewPlan={vi.fn()} />); await choose(); await compare();
  expect(externalHandoffsApi.record).not.toHaveBeenCalled();
  expect(screen.getByText(/Luci has not run these commands/)).not.toBeNull();
  expect((screen.getByRole('button', { name: 'Record reviewed report' }) as HTMLButtonElement).disabled).toBe(true);
  await consent(); await userEvent.click(screen.getByRole('button', { name: 'Record reviewed report' }));
  await screen.findByText(/Report recorded as unverified/);
  expect(externalHandoffsApi.record).toHaveBeenCalledExactlyOnceWith('p-a', handoff(), reviewSHA, report(), expect.any(String));
});
it('rejects a report for another project before source review or storage', async () => {
  render(<ExternalToolHandoff projectId="p-a" onReviewPlan={vi.fn()} />); await choose();
  fireEvent.change(screen.getByLabelText('External report JSON'), { target: { value: JSON.stringify(report('p-b')) } });
  await userEvent.click(screen.getByRole('button', { name: 'Compare returned work' }));
  expect((await screen.findByRole('alert')).textContent).toContain('does not match'); expect(externalHandoffsApi.review).not.toHaveBeenCalled(); expect(externalHandoffsApi.record).not.toHaveBeenCalled();
});
it('shows amendment staleness and refuses a return against older approved context', async () => {
  vi.mocked(externalHandoffsApi.review).mockResolvedValue(review({ contextCurrent: false, contextMessage: 'The design was amended.' }));
  render(<ExternalToolHandoff projectId="p-a" onReviewPlan={vi.fn()} />); await choose(); await compare();
  expect(screen.getByText('The design was amended.')).not.toBeNull();
  expect((screen.getByRole('checkbox', { name: /I reviewed these changes/ }) as HTMLInputElement).disabled).toBe(true);
  expect((screen.getByRole('button', { name: 'Record reviewed report' }) as HTMLButtonElement).disabled).toBe(true);
});
it('clears review consent whenever the report changes', async () => {
  render(<ExternalToolHandoff projectId="p-a" onReviewPlan={vi.fn()} />); await choose(); await compare(); await consent();
  fireEvent.change(screen.getByLabelText('External report JSON'), { target: { value: JSON.stringify({ ...report(), summary: 'New claims' }) } });
  expect(screen.queryByRole('button', { name: 'Record reviewed report' })).toBeNull(); expect(externalHandoffsApi.record).not.toHaveBeenCalled();
});
it('retains the report and same request identity after a lost response until an explicit new review', async () => {
  vi.mocked(externalHandoffsApi.record).mockRejectedValueOnce(new Error('Response lost'));
  render(<ExternalToolHandoff projectId="p-a" onReviewPlan={vi.fn()} />); await choose(); await compare(); await consent();
  await userEvent.click(screen.getByRole('button', { name: 'Record reviewed report' })); await screen.findByText('Response lost');
  const key = vi.mocked(externalHandoffsApi.record).mock.calls[0][4];
  expect((screen.getByRole('button', { name: 'Record reviewed report' }) as HTMLButtonElement).disabled).toBe(true);
  vi.mocked(externalHandoffsApi.review).mockResolvedValue(review({ reviewDigest: 'c'.repeat(64) }));
  await userEvent.click(screen.getByRole('button', { name: 'Compare returned work' })); await consent();
  await userEvent.click(screen.getByRole('button', { name: 'Record reviewed report' })); await screen.findByText(/Report recorded as unverified/);
  expect(vi.mocked(externalHandoffsApi.record).mock.calls[1][4]).toBe(key);
  expect(vi.mocked(externalHandoffsApi.record).mock.calls[1][2]).toBe(reviewSHA);
});
it('reconciles an exact already-recorded request from refreshed history without appending or replaying it', async () => {
  vi.mocked(externalHandoffsApi.record).mockRejectedValueOnce(new Error('Response lost'));
  render(<ExternalToolHandoff projectId="p-a" onReviewPlan={vi.fn()} />); await choose(); await compare(); await consent();
  await userEvent.click(screen.getByRole('button', { name: 'Record reviewed report' })); await screen.findByText('Response lost');
  const key = vi.mocked(externalHandoffsApi.record).mock.calls[0][4];
  vi.mocked(externalHandoffsApi.get).mockResolvedValue({ ...handoff(), returns: [{ ...returned(), idempotencyKey: key }], returnCount: 1 });
  await userEvent.click(screen.getByRole('button', { name: 'Refresh saved handoffs' }));
  await screen.findByText(/original report was already recorded/);
  expect(screen.getByText('Retained reports · 1')).not.toBeNull();
  expect((screen.getByLabelText('External report JSON') as HTMLTextAreaElement).value).not.toContain('Built calculator');
  expect(externalHandoffsApi.record).toHaveBeenCalledTimes(1);
});
it('does not clear an uncertain mutation before the selected-handoff refresh settles', async () => {
  vi.mocked(externalHandoffsApi.record).mockRejectedValueOnce(new Error('Response lost'));
  render(<ExternalToolHandoff projectId="p-a" onReviewPlan={vi.fn()} />); await choose(); await compare(); await consent();
  await userEvent.click(screen.getByRole('button', { name: 'Record reviewed report' })); await screen.findByText('Response lost');
  const pending = deferred<ExternalHandoff>(); vi.mocked(externalHandoffsApi.get).mockReturnValueOnce(pending.promise);
  await userEvent.click(screen.getByRole('button', { name: 'Refresh saved handoffs' }));
  await waitFor(() => expect(externalHandoffsApi.get).toHaveBeenCalledTimes(2));
  expect(screen.getByText(/last action needs reconciliation/)).not.toBeNull();
  await act(async () => pending.reject(new Error('History unavailable')));
  expect(screen.getByText(/last action needs reconciliation/)).not.toBeNull();
});
it('suppresses an old successful mutation after edit revocation and restoration', async () => {
  const pending = deferred<ExternalReturn>(); vi.mocked(externalHandoffsApi.record).mockReturnValueOnce(pending.promise);
  const mounted = render(<ExternalToolHandoff projectId="p-a" onReviewPlan={vi.fn()} />); await choose(); await compare(); await consent();
  await userEvent.click(screen.getByRole('button', { name: 'Record reviewed report' }));
  authority.edit = false; mounted.rerender(<ExternalToolHandoff projectId="p-a" onReviewPlan={vi.fn()} />);
  authority.edit = true; mounted.rerender(<ExternalToolHandoff projectId="p-a" onReviewPlan={vi.fn()} />);
  await act(async () => pending.resolve(returned()));
  expect(screen.queryByText(/Report recorded as unverified/)).toBeNull(); expect(screen.getByText(/last action needs reconciliation/)).not.toBeNull();
});
it('fences a pending download after project change and keeps its draft isolated', async () => {
  const pending = deferred<Blob>(); vi.mocked(externalHandoffsApi.bundle).mockReturnValueOnce(pending.promise);
  const mounted = render(<ExternalToolHandoff projectId="p-a" onReviewPlan={vi.fn()} />); await choose();
  fireEvent.change(screen.getByLabelText('External report JSON'), { target: { value: JSON.stringify(report()) } });
  await userEvent.click(screen.getByRole('button', { name: 'Download context ZIP' }));
  mounted.rerender(<ExternalToolHandoff projectId="p-b" onReviewPlan={vi.fn()} />); await act(async () => pending.resolve(new Blob(['old'])));
  expect(HTMLAnchorElement.prototype.click).not.toHaveBeenCalled(); await open();
  expect(screen.queryByText(/report draft is saved/)).toBeNull();
});
it('prevents download and clipboard access while current project authority is refreshing', async () => {
  const mounted = render(<ExternalToolHandoff projectId="p-a" onReviewPlan={vi.fn()} />); await choose();
  authority.loading = true; mounted.rerender(<ExternalToolHandoff projectId="p-a" onReviewPlan={vi.fn()} />);
  await userEvent.click(screen.getByRole('button', { name: 'Download context ZIP' }));
  expect(externalHandoffsApi.bundle).not.toHaveBeenCalled(); expect(HTMLAnchorElement.prototype.click).not.toHaveBeenCalled();
});
it('restores a durable report under its original handoff without granting return consent', async () => {
  const mounted = render(<ExternalToolHandoff projectId="p-a" onReviewPlan={vi.fn()} />); await choose();
  fireEvent.change(screen.getByLabelText('External report JSON'), { target: { value: JSON.stringify(report()) } });
  await screen.findByText(/Report draft saved in this browser/); mounted.unmount();
  render(<ExternalToolHandoff projectId="p-a" onReviewPlan={vi.fn()} />); await open();
  await userEvent.click(screen.getByRole('button', { name: 'Restore report draft' }));
  await waitFor(() => expect((screen.getByLabelText('External report JSON') as HTMLTextAreaElement).value).toBe(JSON.stringify(report())));
  expect(screen.queryByRole('button', { name: 'Record reviewed report' })).toBeNull(); expect(externalHandoffsApi.record).not.toHaveBeenCalled();
});
it('allows read-only history and download while forbidding capture and record actions', async () => {
  authority.edit = false; render(<ExternalToolHandoff projectId="p-a" onReviewPlan={vi.fn()} />); await choose();
  expect((screen.getByRole('button', { name: 'Capture selected context' }) as HTMLButtonElement).disabled).toBe(true);
  expect((screen.getByLabelText('External report JSON') as HTMLTextAreaElement).disabled).toBe(true);
  await userEvent.click(screen.getByRole('button', { name: 'Download context ZIP' }));
  expect(externalHandoffsApi.bundle).toHaveBeenCalledExactlyOnceWith('p-a', 'handoff-p-a'); expect(HTMLAnchorElement.prototype.click).toHaveBeenCalledOnce();
});
