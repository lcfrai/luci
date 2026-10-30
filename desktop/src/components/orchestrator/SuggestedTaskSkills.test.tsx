import { act, cleanup, render, screen } from '@testing-library/react';
import userEvent from '@testing-library/user-event';
import { afterEach, beforeEach, expect, it, vi } from 'vitest';
import { context, proposalTurn } from './fixtures';
import { SuggestedTaskSkills } from './SuggestedTaskSkills';
import { AmendmentReview } from './AmendmentReview';
import { productApi } from '../../api/product';
import { projectSkillsApi, type ProjectSkillRegistry } from '../../api/projectSkills';
import type { OrchestratorTurn } from '../../api/orchestrator';
import { resetCockpit, useCockpit } from '../../store/store';
import type { ReactNode } from 'react';

vi.mock('../common/WorkflowDialog', () => ({ WorkflowDialog: ({ children }: { children: ReactNode }) => <section>{children}</section> }));
function fixture() {
  const registry: ProjectSkillRegistry = { projectId: 'p-1', revision: 3, versions: [{ id: 'persistence', revision: 1, digest: 'b'.repeat(64), title: 'Persistence', provenance: 'Project instructions', roles: ['coding'], instructions: 'Check save and reload.' }], assignments: [{ role: 'coding', taskId: 'other', skillId: 'persistence', skillRevision: 1 }] };
  const base = context(), tasks = [...base.plan.tasks, { id: 'other', title: 'Unrelated work', status: 'todo' as const }];
  const turn: OrchestratorTurn = { ...proposalTurn(), appliedRevision: 4, context: { ...base, skills: { projectId: 'p-1', revision: 3, registryDigest: 'c'.repeat(64), versions: registry.versions.map(v => ({ ...v, instructionExcerpt: v.instructions })), omittedVersions: 2, assignments: registry.assignments } } };
  turn.answer = { ...turn.answer!, proposal: { target: 'plan', summary: 'A narrow implementation', specification: null, designPlan: null, plan: { ...base.plan, tasks }, suggestedSkillAssignments: [{ taskId: 'T-1', role: 'coding', skillId: 'persistence', skillRevision: 1, skillDigest: 'b'.repeat(64), reason: 'Verify save and reload behavior.' }] } };
  const plan = { ...base.plan, revision: 4, tasks };
  return { turn, registry, plan };
}
beforeEach(() => { vi.restoreAllMocks(); resetCockpit(); });
afterEach(cleanup);
function mounted() {
  const f = fixture();
  vi.spyOn(productApi, 'plan').mockResolvedValue(f.plan);
  vi.spyOn(projectSkillsApi, 'get').mockResolvedValue(f.registry);
  const save = vi.spyOn(projectSkillsApi, 'save').mockImplementation(async (_id, input) => ({ ...f.registry, revision: 4, assignments: input.assignments }));
  const props = { turn: f.turn, scope: 'alice:workspace:p-1', canRead: true, canEdit: true, blocked: false };
  return { ...f, props, save, ui: render(<SuggestedTaskSkills {...props}/>) };
}
it('shows full immutable references before separate explicit review and save while preserving unrelated selections', async () => {
  const f = mounted();
  expect(screen.getByText('b'.repeat(64))).not.toBeNull();
  expect(screen.getByText(/2 saved skill versions were omitted/)).not.toBeNull();
  expect(projectSkillsApi.get).not.toHaveBeenCalled(); expect(f.save).not.toHaveBeenCalled();
  await userEvent.click(screen.getByRole('button', { name: 'Review current plan and skills' }));
  await screen.findByText(/Saved plan revision 4/);
  const button = screen.getByRole('button', { name: 'Use suggested task skills' });
  await userEvent.click(button); expect(f.save).not.toHaveBeenCalled();
  await userEvent.click(screen.getByRole('checkbox')); await userEvent.click(button);
  await screen.findByText('Task skill assignments saved. No work was started.');
  expect(f.save).toHaveBeenCalledExactlyOnceWith('p-1', { expectedRevision: 3, expectedPlanRevision: 4, assignments: [...f.registry.assignments, { taskId: 'T-1', role: 'coding', skillId: 'persistence', skillRevision: 1 }] });
});
it('refuses changed saved plan contents before offering a save', async () => {
  const f = mounted(); vi.mocked(productApi.plan).mockResolvedValue({ ...f.plan, revision: 5 });
  await userEvent.click(screen.getByRole('button', { name: 'Review current plan and skills' }));
  await screen.findByRole('alert'); expect(f.save).not.toHaveBeenCalled();
  expect(screen.queryByRole('button', { name: 'Use suggested task skills' })).toBeNull();
});
it('rechecks the registry immediately before POST and refuses drift without replacing assignments', async () => {
  const f = mounted();
  await userEvent.click(screen.getByRole('button', { name: 'Review current plan and skills' })); await screen.findByText(/Saved plan revision 4/);
  vi.mocked(projectSkillsApi.get).mockResolvedValue({ ...f.registry, revision: 4 });
  await userEvent.click(screen.getByRole('checkbox')); await userEvent.click(screen.getByRole('button', { name: 'Use suggested task skills' }));
  await screen.findByRole('alert'); expect(f.save).not.toHaveBeenCalled();
});
it('withdraws a held read after permission loss and does not publish it on restoration', async () => {
  const f = mounted(); let resolve!: (r: ProjectSkillRegistry) => void;
  vi.mocked(projectSkillsApi.get).mockReturnValue(new Promise(r => { resolve = r; }));
  await userEvent.click(screen.getByRole('button', { name: 'Review current plan and skills' }));
  f.ui.rerender(<SuggestedTaskSkills {...f.props} canRead={false}/>);
  f.ui.rerender(<SuggestedTaskSkills {...f.props}/>);
  await act(async () => resolve(f.registry));
  expect(screen.queryByText(/Saved plan revision 4/)).toBeNull(); expect(f.save).not.toHaveBeenCalled();
});
it('blocks live unsaved project drafts and never automatically retries an uncertain save', async () => {
  const f = mounted();
  await userEvent.click(screen.getByRole('button', { name: 'Review current plan and skills' })); await screen.findByText(/Saved plan revision 4/);
  await userEvent.click(screen.getByRole('checkbox'));
  act(() => useCockpit.getState().setUnsavedProjectDraft('draft', { projectId: 'p-1', label: 'Task edits', lens: 'develop', retained: true }));
  await userEvent.click(screen.getByRole('button', { name: 'Use suggested task skills' })); expect(f.save).not.toHaveBeenCalled();
  act(() => useCockpit.getState().setUnsavedProjectDraft('draft', null));
  f.save.mockRejectedValue(new Error('Reply lost'));
  await userEvent.click(screen.getByRole('button', { name: 'Use suggested task skills' }));
  await screen.findByText(/Nothing will be resent automatically/);
  await act(async () => {}); expect(f.save).toHaveBeenCalledTimes(1);
  expect(screen.queryByRole('button', { name: 'Use suggested task skills' })).toBeNull();
});
it('keeps plan Apply separate from the visible skill recommendations', async () => {
  const f = fixture(), save = vi.spyOn(projectSkillsApi, 'save'), apply = vi.fn(); delete f.turn.appliedRevision;
  const { view } = await import('./fixtures');
  render(<AmendmentReview turn={f.turn} view={view({ context: f.turn.context })} readAccess={{ canRead: true, loading: false, error: '' }} busy={false} blocked="" error="" onClose={vi.fn()} onApply={apply}/>);
  expect(screen.getByRole('region', { name: 'Suggested task skills' })).not.toBeNull();
  await userEvent.click(screen.getByRole('checkbox')); await userEvent.click(screen.getByRole('button', { name: 'Apply reviewed amendment' }));
  expect(apply).toHaveBeenCalledTimes(1); expect(save).not.toHaveBeenCalled();
});
it('does not confirm a returned assignment whose immutable skill digest differs', async () => {
  const f = mounted(); f.save.mockImplementation(async (_id, input) => ({ ...f.registry, revision: 4, assignments: input.assignments, versions: f.registry.versions.map(v => ({ ...v, digest: 'd'.repeat(64) })) }));
  await userEvent.click(screen.getByRole('button', { name: 'Review current plan and skills' })); await screen.findByText(/Saved plan revision 4/);
  await userEvent.click(screen.getByRole('checkbox')); await userEvent.click(screen.getByRole('button', { name: 'Use suggested task skills' }));
  await screen.findByText(/returned skill version did not match/);
  expect(screen.queryByText('Task skill assignments saved. No work was started.')).toBeNull();
  expect(f.save).toHaveBeenCalledTimes(1);
});
