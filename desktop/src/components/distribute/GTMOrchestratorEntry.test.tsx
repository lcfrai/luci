import { act, cleanup, render, screen } from '@testing-library/react';
import userEvent from '@testing-library/user-event';
import { afterEach, beforeEach, expect, it, vi } from 'vitest';
import { GTMWorkspace } from './GTMWorkspace';
import { gtmWorkspaceApi, type GTMView } from '../../api/gtmWorkspace';
import { PROJECT_CONTEXT_CHANGED } from '../../api/projectContextEvents';
import { resetCockpit } from '../../store/store';

const scope = 'gtm-owner/workspace/p-one';
vi.mock('../../hooks/useProjectDraft', async original => ({ ...await original<typeof import('../../hooks/useProjectDraft')>(), useProjectDraftScope: () => 'gtm-owner/workspace/p-one' }));
vi.mock('../teams/TeamWorkspaceProvider', () => ({ useTeamWorkspace: () => null }));
function baseline(): GTMView {
  return { projectId: 'p-one', workspace: null, freshness: 'none', history: [], requirements: [], sourceDigest: 'a'.repeat(64), source: { briefRevision: 1, briefDigest: 'a'.repeat(64), specificationRevision: 1, specificationDigest: 'b'.repeat(64), designRevision: 0, designDigest: 'c'.repeat(64), planRevision: 0, planDigest: 'd'.repeat(64), checks: 'not-selected', release: 'not-selected' } };
}
beforeEach(() => { vi.restoreAllMocks(); localStorage.clear(); resetCockpit(); vi.spyOn(gtmWorkspaceApi, 'get').mockResolvedValue(baseline()); vi.spyOn(gtmWorkspaceApi, 'save').mockRejectedValue(new Error('Unexpected save')); });
afterEach(cleanup);
it('opens a review-only Luci draft request without sending or saving it', async () => {
  const receive = vi.fn(); window.addEventListener('luci:orchestrator-open', receive);
  try {
    render(<GTMWorkspace projectId="p-one" canRead canEdit />);
    await screen.findByLabelText('Positioning');
    await userEvent.click(screen.getByRole('button', { name: 'Draft or amend with Luci' }));
    expect(receive).toHaveBeenCalledOnce();
    const detail = (receive.mock.calls[0][0] as CustomEvent).detail;
    expect(detail.projectId).toBe('p-one'); expect(detail.message).toContain('explicit review');
    expect(gtmWorkspaceApi.save).not.toHaveBeenCalled();
  } finally { window.removeEventListener('luci:orchestrator-open', receive); }
});
it('keeps an unsaved strategy and requires review after a project-context notification', async () => {
  render(<GTMWorkspace projectId="p-one" canRead canEdit />);
  const positioning = await screen.findByLabelText('Positioning');
  await userEvent.type(positioning, 'Keep this unsaved strategy');
  expect(screen.getByRole('button', { name: 'Draft or amend with Luci' })).toHaveProperty('disabled', true);
  act(() => window.dispatchEvent(new CustomEvent(PROJECT_CONTEXT_CHANGED, { detail: { projectId: 'p-one', scope } })));
  expect(positioning).toHaveProperty('value', 'Keep this unsaved strategy');
  expect(screen.getByRole('button', { name: 'Save GTM revision' })).toHaveProperty('disabled', true);
  expect(gtmWorkspaceApi.save).not.toHaveBeenCalled();
});
