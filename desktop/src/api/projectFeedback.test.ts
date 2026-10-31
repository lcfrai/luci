import { afterEach, expect, it, vi } from 'vitest';
import { productApi } from './product';
import { projectFeedbackApi, validCapturedFeedbackContext, validFeedbackPreview, validProjectFeedback, type FeedbackPreview, type ProjectFeedbackView } from './projectFeedback';

const source = { repository: 'owner/repo', contextDigest: 'a'.repeat(64) };
const issue = { number: 7, title: 'Keep failed saves', body: 'Preserve typed input.', bodyTruncated: false, state: 'open' as const, updatedAt: '2026-10-31T00:00:00Z' };
const preview = (): FeedbackPreview => ({ id: 'feedback-preview-' + '1'.repeat(24), projectId: 'p-feedback', source, createdAt: '2026-10-31T00:00:00Z', expiresAt: '2026-10-31T00:10:00Z', fetched: 1, issues: [{ ...issue }], cliAuthScope: 'workspace-cli-not-personal-identity' });
const saved = (): ProjectFeedbackView => ({ projectId: 'p-feedback', revision: 1, source, observations: [{ id: 'feedback-' + 'b'.repeat(64), digest: 'b'.repeat(64), repository: source.repository, issue: { ...issue }, observedAt: issue.updatedAt, importedAt: issue.updatedAt, actor: { id: 'person', displayName: 'Person' }, sourceContextDigest: source.contextDigest }], selectedIds: [], prunedObservations: 0 });
afterEach(() => vi.restoreAllMocks());

it('uses only fixed project routes and rejects a preview for a different attached source', async () => {
 const request = vi.spyOn(productApi, 'request').mockResolvedValue(preview());
 await projectFeedbackApi.preview('p-feedback', source);
 expect(request).toHaveBeenCalledWith('/v1/product/projects/p-feedback/feedback/preview', { source }, 'POST', 30_000, undefined);
 request.mockResolvedValue({ ...preview(), source: { ...source, repository: 'other/repo' } });
 await expect(projectFeedbackApi.preview('p-feedback', source)).rejects.toThrow('could not be confirmed');
 expect(validFeedbackPreview({ ...preview(), cli: { path: 'secret' } }, 'p-feedback')).toBe(false);
 expect(validFeedbackPreview({ ...preview(), issues: [issue, issue], fetched: 2 }, 'p-feedback')).toBe(false);
});

it('positively correlates every imported issue and exact selected readback without model calls', async () => {
 const view = saved(), ids = [view.observations[0].id];
 const request = vi.spyOn(productApi, 'request').mockResolvedValue({ view, importedIds: ids, replayed: false });
 const input = { previewId: preview().id, expectedRevision: 0, issueNumbers: [7], acknowledgeSharing: true, idempotencyKey: 'feedback-key' };
 expect((await projectFeedbackApi.import('p-feedback', preview(), input)).importedIds).toEqual(ids);
 request.mockResolvedValue({ view: { ...view, observations: [{ ...view.observations[0], issue: { ...issue, body: 'A different version' } }] }, importedIds: ids, replayed: true });
 await expect(projectFeedbackApi.import('p-feedback', preview(), input)).rejects.toThrow('could not be confirmed');
 request.mockResolvedValue({ ...view, revision: 2, selectedIds: ids });
 expect((await projectFeedbackApi.select('p-feedback', 1, ids)).selectedIds).toEqual(ids);
 request.mockResolvedValue({ ...view, revision: 2, selectedIds: [] });
 await expect(projectFeedbackApi.select('p-feedback', 1, ids)).rejects.toThrow('could not be confirmed');
 expect(request.mock.calls.every(call => String(call[0]).startsWith('/v1/product/projects/p-feedback/feedback'))).toBe(true);
});

it('rejects cross-project, dangling and duplicate retained evidence and preserves legacy context absence', () => {
 const view = saved();
 expect(validProjectFeedback(view, 'other')).toBe(false);
 expect(validProjectFeedback({ ...view, selectedIds: ['missing'] }, 'p-feedback')).toBe(false);
 expect(validProjectFeedback({ ...view, observations: [view.observations[0], view.observations[0]] }, 'p-feedback')).toBe(false);
 expect(validCapturedFeedbackContext(undefined, 'p-feedback')).toBe(true);
 expect(validCapturedFeedbackContext({ projectId: 'p-feedback', evidenceOnly: true, omittedBodyBytes: 0, items: [] }, 'p-feedback')).toBe(false);
});
