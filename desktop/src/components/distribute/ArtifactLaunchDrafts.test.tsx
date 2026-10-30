import { cleanup, render, screen, waitFor, within } from '@testing-library/react';
import userEvent from '@testing-library/user-event';
import { afterEach, beforeEach, expect, it, vi } from 'vitest';
import { ArtifactWorkspace } from './ArtifactWorkspace';
import { distributeArtifactsApi, type DistributeArtifact } from '../../api/distributeArtifacts';
import { resetCockpit } from '../../store/store';

vi.mock('../../hooks/useProjectDraft', () => ({ useProjectDraftScope: () => 'launch-owner/workspace/p-one' }));
const hash = 'a'.repeat(64), gtmDigest = 'b'.repeat(64);
function fixture(): DistributeArtifact {
  return { id: 'artifact-launch', projectId: 'p-one', revision: 1, kind: 'landing-copy', source: { briefRevision: 1, briefDigest: hash, specificationRevision: 2, specificationDigest: hash, designRevision: 1, designDigest: hash, planRevision: 3, planDigest: hash, gtmRevision: 2, gtmDigest, checks: 'not-selected', release: 'not-selected' }, sourceDigest: 'c'.repeat(64), title: 'Landing-page copy', sections: [{ id: 'launch-headline', heading: 'Headline draft', body: 'Keep project context together', bullets: [], sourceRefs: [`gtm:2:${gtmDigest}`] }], gaps: ['Choose an audience before external use.'], assumptions: ['Demand is a hypothesis.'], state: 'draft', digest: hash, at: '2026-10-30T00:00:00Z' };
}
beforeEach(() => { vi.restoreAllMocks(); localStorage.clear(); resetCockpit(); vi.spyOn(distributeArtifactsApi, 'list').mockResolvedValue({ projectId: 'p-one', artifacts: [] }); });
afterEach(cleanup);

it('previews exact GTM-linked copy before explicit private save and retains source through editing', async () => {
  const row = fixture(), user = userEvent.setup();
  const prepare = vi.spyOn(distributeArtifactsApi, 'prepare').mockResolvedValue({ draft: row, prepareDigest: hash });
  const save = vi.spyOn(distributeArtifactsApi, 'save').mockResolvedValue({ artifact: row, freshness: 'current' });
  const revise = vi.spyOn(distributeArtifactsApi, 'revise').mockResolvedValue({ artifact: { ...row, revision: 2, sections: [{ ...row.sections[0], body: 'A refined headline' }] }, freshness: 'current' });
  render(<ArtifactWorkspace projectId="p-one" canRead canEdit />);
  await user.selectOptions(screen.getByLabelText('Artifact kind'), 'landing-copy');
  await user.click(screen.getByRole('button', { name: 'Prepare source draft' }));
  const preview = await screen.findByLabelText('Prepared launch copy');
  expect(within(preview).getByText('Keep project context together')).not.toBeNull();
  expect(prepare).toHaveBeenCalledWith('p-one', 'landing-copy', ''); expect(save).not.toHaveBeenCalled();
  expect(screen.getByText(new RegExp(`GTM 2 · ${gtmDigest}`))).not.toBeNull();
  await user.click(screen.getByRole('button', { name: 'Save private draft' }));
  await screen.findByText(/Private draft saved/);
  const body = screen.getByLabelText('Claim or explanation');
  await user.clear(body); await user.type(body, 'A refined headline');
  await user.click(screen.getByRole('button', { name: 'Save artifact edits' }));
  await screen.findByText(/New private draft revision saved/);
  expect(revise).toHaveBeenCalledWith('p-one', row, row.title, [expect.objectContaining({ body: 'A refined headline', sourceRefs: [`gtm:2:${gtmDigest}`] })], row.gaps, row.assumptions, expect.any(String));
  expect(screen.queryByRole('button', { name: /Publish|Send campaign|Send email/ })).toBeNull();
});

it('requires a saved current GTM strategy and never saves after a refused prepare', async () => {
  const save = vi.spyOn(distributeArtifactsApi, 'save').mockRejectedValue(new Error('Unexpected save'));
  vi.spyOn(distributeArtifactsApi, 'prepare').mockRejectedValue(new Error('Save a GTM revision linked to the current project context before preparing launch copy.'));
  render(<ArtifactWorkspace projectId="p-one" canRead canEdit />);
  await userEvent.selectOptions(screen.getByLabelText('Artifact kind'), 'channel-copy');
  await userEvent.click(screen.getByRole('button', { name: 'Prepare source draft' }));
  expect(await screen.findByRole('alert')).toHaveProperty('textContent', 'Save a GTM revision linked to the current project context before preparing launch copy.');
  expect(screen.queryByRole('button', { name: 'Save private draft' })).toBeNull(); expect(save).not.toHaveBeenCalled();
});

it('keeps an older launch source readable while blocking review after GTM changes', async () => {
  const row = fixture();
  vi.mocked(distributeArtifactsApi.list).mockResolvedValue({ projectId: 'p-one', artifacts: [{ id: row.id, title: row.title, kind: row.kind, revision: 1, state: 'draft', freshness: 'requires-readback', digest: row.digest }] });
  vi.spyOn(distributeArtifactsApi, 'get').mockResolvedValue({ artifact: row, freshness: 'outdated' });
  const review = vi.spyOn(distributeArtifactsApi, 'review').mockRejectedValue(new Error('Unexpected review'));
  render(<ArtifactWorkspace projectId="p-one" canRead canEdit />);
  await userEvent.click(await screen.findByRole('button', { name: /Landing-page copy · revision 1/ }));
  await waitFor(() => expect(screen.getByLabelText('Title')).toHaveProperty('value', row.title));
  expect(screen.getByText(new RegExp(`GTM source: revision 2 · ${gtmDigest}`))).not.toBeNull();
  expect(screen.getByRole('button', { name: 'Mark reviewed' })).toHaveProperty('disabled', true);
  expect(review).not.toHaveBeenCalled();
});
