import { afterEach, beforeEach, expect, it, vi } from 'vitest';
import { act, cleanup, fireEvent, render, screen, waitFor } from '@testing-library/react';
import userEvent from '@testing-library/user-event';
import { ProjectSkills } from './ProjectSkills';
import { projectSkillsApi, type ProjectSkillRegistry } from '../../api/projectSkills';
import { productApi } from '../../api/product';
import { resetCockpit } from '../../store/store';

const state = vi.hoisted(() => ({ scope: 'skills-owner-workspace', access: { canRead: true, canEdit: true, loading: false, error: '', message: '' } }));
vi.mock('../../hooks/useProjectAccess', () => ({ useProjectAccess: () => state.access }));
vi.mock('../../hooks/useProjectDraft', () => ({ useProjectDraftScope: () => state.scope }));
const fixture = (): ProjectSkillRegistry => ({ projectId: 'p-skills', revision: 2, versions: [{ id: 'review', revision: 1, title: 'Review saves', provenance: 'Project team', roles: ['coding', 'reviewer'], instructions: 'Check failed saves preserve input.', digest: 'a'.repeat(64) }], assignments: [] });
beforeEach(() => {
  resetCockpit(); state.scope = 'skills-owner-workspace'; state.access = { canRead: true, canEdit: true, loading: false, error: '', message: '' };
  vi.spyOn(projectSkillsApi, 'get').mockResolvedValue(fixture());
  vi.spyOn(productApi, 'plan').mockResolvedValue({ projectId: 'p-skills', revision: 1, briefRevision: 1, updatedAt: '', tasks: [{ id: 'build', title: 'Build request form', status: 'todo', role: 'coding' }] });
});
afterEach(() => { cleanup(); vi.restoreAllMocks(); });
async function openSkills() {
  const summary = screen.getByText('Project skills');
  await userEvent.click(summary);
  fireEvent.toggle(summary.closest('details')!, { target: { open: true } });
  await screen.findByText('Registry revision 2 · 1 immutable versions');
}

it('reads only on reveal and saves an exact task skill revision without starting work', async () => {
  const save = vi.spyOn(projectSkillsApi, 'save').mockImplementation(async (_id, input) => ({ ...fixture(), revision: 3, assignments: input.assignments }));
  render(<ProjectSkills projectId="p-skills" />);
  expect(projectSkillsApi.get).not.toHaveBeenCalled(); expect(save).not.toHaveBeenCalled();
  await openSkills();
  await userEvent.click(screen.getByRole('button', { name: 'Add assignment' }));
  await userEvent.selectOptions(screen.getByLabelText('Task'), 'build');
  expect(save).not.toHaveBeenCalled();
  await userEvent.click(screen.getByRole('button', { name: 'Save assignments' }));
  await waitFor(() => expect(save).toHaveBeenCalledOnce());
  expect(save).toHaveBeenCalledWith('p-skills', { expectedRevision: 2, assignments: [{ role: 'coding', taskId: 'build', skillId: 'review', skillRevision: 1 }] });
  await screen.findByText('Registry revision 3 · 1 immutable versions');
});

it('keeps authored instructions on a save conflict and requires an explicit discard', async () => {
  vi.spyOn(projectSkillsApi, 'save').mockRejectedValue(new Error('The skill registry changed. Reload before saving.'));
  render(<ProjectSkills projectId="p-skills" />); await openSkills();
  await userEvent.type(screen.getByLabelText('Skill ID'), 'safety');
  await userEvent.type(screen.getByLabelText('Title'), 'Keep drafts');
  await userEvent.type(screen.getByLabelText('Declared provenance'), 'Project team');
  await userEvent.type(screen.getByLabelText('Instructions'), 'Keep input when saving fails.');
  await userEvent.click(screen.getByRole('button', { name: 'Save new immutable version' }));
  await screen.findByRole('alert');
  expect((screen.getByLabelText('Instructions') as HTMLTextAreaElement).value).toBe('Keep input when saving fails.');
  expect((screen.getByRole('button', { name: 'Reload saved skills' }) as HTMLButtonElement).disabled).toBe(true);
  await userEvent.click(screen.getByRole('button', { name: 'Discard unsaved skill edits' }));
  expect((screen.getByLabelText('Instructions') as HTMLTextAreaElement).value).toBe('');
});

it('keeps all mutations disabled for read-only viewers', async () => {
  state.access.canEdit = false;
  const save = vi.spyOn(projectSkillsApi, 'save');
  render(<ProjectSkills projectId="p-skills" />); await openSkills();
  expect(screen.getByLabelText('Instructions').matches(':disabled')).toBe(true);
  expect(screen.getByRole('button', { name: 'Add assignment' }).matches(':disabled')).toBe(true);
  expect(save).not.toHaveBeenCalled();
});

it('ignores a late old-scope response and hides content immediately when read authority is withdrawn', async () => {
  let resolve!: (value: ProjectSkillRegistry) => void;
  vi.mocked(projectSkillsApi.get).mockImplementationOnce(() => new Promise(done => { resolve = done; }));
  const view = render(<ProjectSkills projectId="p-skills" />);
  const summary = screen.getByText('Project skills'); await userEvent.click(summary); fireEvent.toggle(summary.closest('details')!, { target: { open: true } });
  await waitFor(() => expect(projectSkillsApi.get).toHaveBeenCalledOnce());
  state.scope = 'different-owner-workspace'; view.rerender(<ProjectSkills projectId="p-skills" />);
  await screen.findByText('Registry revision 2 · 1 immutable versions');
  await act(async () => { resolve({ ...fixture(), revision: 99 }); });
  expect(screen.queryByText('Registry revision 99 · 1 immutable versions')).toBeNull();
  state.access.canRead = false; state.access.message = 'Read access removed.'; view.rerender(<ProjectSkills projectId="p-skills" />);
  expect(screen.queryByLabelText('Instructions')).toBeNull(); expect(screen.getByRole('status').textContent).toBe('Read access removed.');
});
