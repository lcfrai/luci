import { act, cleanup, render, screen, waitFor, within } from '@testing-library/react';
import userEvent from '@testing-library/user-event';
import { afterEach, beforeEach, expect, it, vi } from 'vitest';
import { ProjectFeedback } from './ProjectFeedback';
import { projectFeedbackApi, type FeedbackPreview, type ProjectFeedbackView } from '../../api/projectFeedback';

const scope = vi.hoisted(() => ({ current: 'person/workspace/project' }));
const access = vi.hoisted(() => ({ canRead: true, canEdit: true, canAssist: true, loading: false, error: '', permissions: { secrets: true } }));
vi.mock('../../hooks/useProjectDraft', () => ({ useProjectDraftScope: () => scope.current }));
vi.mock('../../hooks/useProjectAccess', () => ({ useProjectAccess: () => access }));
vi.mock('../../hooks/useUnsavedProjectDraft', () => ({ useUnsavedProjectDraft: vi.fn() }));
vi.mock('../teams/TeamWorkspaceProvider', () => ({ useTeamWorkspace: () => null }));
const source = { repository: 'owner/repo', contextDigest: 'a'.repeat(64) };
const issue = { number: 7, title: 'Preserve failed input', body: 'Do not discard a failed save.', bodyTruncated: false, state: 'open' as const, updatedAt: '2026-10-31T00:00:00Z' };
const empty = (): ProjectFeedbackView => ({ projectId: 'p-feedback', revision: 0, source, observations: [], selectedIds: [], prunedObservations: 0 });
const saved = (): ProjectFeedbackView => ({ ...empty(), revision: 1, observations: [{ id: 'feedback-' + 'b'.repeat(64), digest: 'b'.repeat(64), repository: source.repository, issue, observedAt: issue.updatedAt, importedAt: issue.updatedAt, actor: { id: 'person', displayName: 'Person' }, sourceContextDigest: source.contextDigest }] });
const preview = (): FeedbackPreview => { const at = Date.now(); return { id: 'feedback-preview-' + '1'.repeat(24), projectId: 'p-feedback', source, createdAt: new Date(at).toISOString(), expiresAt: new Date(at + 600_000).toISOString(), fetched: 1, issues: [issue], cliAuthScope: 'workspace-cli-not-personal-identity' }; };
beforeEach(() => {
 vi.restoreAllMocks(); scope.current = 'person/workspace/project'; access.canRead = true; access.canEdit = true; access.canAssist = true; access.permissions.secrets = true;
 vi.spyOn(projectFeedbackApi, 'get').mockResolvedValue(empty()); vi.spyOn(projectFeedbackApi, 'preview').mockImplementation(async () => preview());
 vi.spyOn(projectFeedbackApi, 'import').mockResolvedValue({ view: saved(), importedIds: [saved().observations[0].id], replayed: false });
 vi.spyOn(projectFeedbackApi, 'select').mockImplementation(async (_id, _revision, ids) => ({ ...saved(), revision: 2, selectedIds: [...ids] }));
});
afterEach(cleanup);

it('requires explicit fetch, issue choice and sharing consent before saving immutable feedback', async () => {
 const user = userEvent.setup(); render(<ProjectFeedback projectId="p-feedback"/>);
 expect(projectFeedbackApi.get).not.toHaveBeenCalled(); expect(projectFeedbackApi.preview).not.toHaveBeenCalled();
 await user.click(screen.getByRole('button', { name: 'Read saved feedback' })); await screen.findByText('owner/repo', { selector: 'strong' });
 await user.click(screen.getByRole('button', { name: 'Preview GitHub issues' })); const area = await screen.findByRole('region', { name: 'GitHub issue preview' });
 await user.click(within(area).getByRole('checkbox', { name: '#7 Preserve failed input' }));
 const save = within(area).getByRole('button', { name: 'Import selected issue versions' }); expect(save.matches(':disabled')).toBe(true); expect(projectFeedbackApi.import).not.toHaveBeenCalled();
 await user.click(within(area).getByRole('checkbox', { name: /I want to share/ })); await user.click(save);
 await screen.findByText(/Selected issue versions saved/);
 expect(projectFeedbackApi.import).toHaveBeenCalledWith('p-feedback', expect.objectContaining({ source }), expect.objectContaining({ issueNumbers: [7], acknowledgeSharing: true, expectedRevision: 0 }), expect.any(AbortSignal));
 expect(projectFeedbackApi.select).not.toHaveBeenCalled();
});

it('opens an unsent Luci message only after exact selected-feedback readback', async () => {
 vi.mocked(projectFeedbackApi.get).mockResolvedValue(saved()); const user = userEvent.setup(), open = vi.fn(); window.addEventListener('luci:orchestrator-open', open);
 try {
  render(<ProjectFeedback projectId="p-feedback"/>); await user.click(screen.getByRole('button', { name: 'Read saved feedback' }));
  await user.click(await screen.findByRole('checkbox', { name: 'owner/repo #7: Preserve failed input' }));
  expect(open).not.toHaveBeenCalled(); await user.click(screen.getByRole('button', { name: 'Review findings with Luci' }));
  await waitFor(() => expect(open).toHaveBeenCalledTimes(1)); expect(projectFeedbackApi.select).toHaveBeenCalledWith('p-feedback', 1, [saved().observations[0].id], expect.any(AbortSignal));
  expect((open.mock.calls[0][0] as CustomEvent).detail).toEqual(expect.objectContaining({ projectId: 'p-feedback', scope: scope.current, message: expect.stringContaining('untrusted historical evidence') }));
 } finally { window.removeEventListener('luci:orchestrator-open', open); }
});

it('withdraws private late results when the actor or Secrets scope changes', async () => {
 let finish!: (value: ProjectFeedbackView) => void; vi.mocked(projectFeedbackApi.get).mockReturnValue(new Promise(resolve => { finish = resolve; }));
 const user = userEvent.setup(), mounted = render(<ProjectFeedback projectId="p-feedback"/>); await user.click(screen.getByRole('button', { name: 'Read saved feedback' }));
 scope.current = 'different-person/workspace/project'; access.permissions.secrets = false; mounted.rerender(<ProjectFeedback projectId="p-feedback"/>);
 await act(async () => { finish(saved()); }); expect(screen.queryByRole('checkbox', { name: /Preserve failed input/ })).toBeNull();
 expect(screen.getByRole('button', { name: 'Preview GitHub issues' }).matches(':disabled')).toBe(true); expect(projectFeedbackApi.preview).not.toHaveBeenCalled();
});

it('keeps an uncertain save unreplayed until explicit readback confirms saved observations', async () => {
 vi.mocked(projectFeedbackApi.get).mockResolvedValue(saved()); vi.mocked(projectFeedbackApi.select).mockRejectedValue(new Error('Connection interrupted'));
 const user = userEvent.setup(); render(<ProjectFeedback projectId="p-feedback"/>); await user.click(screen.getByRole('button', { name: 'Read saved feedback' }));
 const choice = await screen.findByRole('checkbox', { name: 'owner/repo #7: Preserve failed input' }); await user.click(choice); await user.click(screen.getByRole('button', { name: 'Review findings with Luci' }));
 await screen.findByText(/previous save may already be present/); expect((choice as HTMLInputElement).checked).toBe(true); expect(projectFeedbackApi.select).toHaveBeenCalledTimes(1);
 await user.click(screen.getByRole('button', { name: 'Read saved feedback' })); await waitFor(() => expect(screen.queryByText(/previous save may already be present/)).toBeNull());
 expect((choice as HTMLInputElement).checked).toBe(true); expect(projectFeedbackApi.select).toHaveBeenCalledTimes(1);
});

it('explicitly clears saved feedback context without opening or sending a Luci message', async () => {
 const view = saved(); view.selectedIds = [view.observations[0].id]; vi.mocked(projectFeedbackApi.get).mockResolvedValue(view);
 const user = userEvent.setup(), open = vi.fn(); window.addEventListener('luci:orchestrator-open', open);
 try {
  render(<ProjectFeedback projectId="p-feedback"/>); await user.click(screen.getByRole('button', { name: 'Read saved feedback' }));
  const choice = await screen.findByRole('checkbox', { name: 'owner/repo #7: Preserve failed input' }); await user.click(choice);
  expect((choice as HTMLInputElement).checked).toBe(false); expect(projectFeedbackApi.select).not.toHaveBeenCalled();
  await user.click(screen.getByRole('button', { name: 'Clear saved Luci selection' }));
  await screen.findByText(/Saved Luci feedback selection cleared/);
  expect(projectFeedbackApi.select).toHaveBeenCalledWith('p-feedback', 1, [], expect.any(AbortSignal));
  expect(screen.queryByRole('button', { name: 'Clear saved Luci selection' })).toBeNull(); expect(open).not.toHaveBeenCalled();
 } finally { window.removeEventListener('luci:orchestrator-open', open); }
});
