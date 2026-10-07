import { act, render, screen, waitFor } from '@testing-library/react';
import userEvent from '@testing-library/user-event';
import { beforeEach, expect, it, vi } from 'vitest';
import { productApi, type Brief, type ImplementationPlan, type RequirementCoverage } from '../../api/product';
import { resetCockpit, useCockpit } from '../../store/store';
import { aBranch, aProject, aWorktree } from '../../test/fixtures';
import { ProjectPlan } from './ProjectPlan';

const identity = vi.hoisted(() => ({ scope: 'owner-workspace' }));
vi.mock('../../hooks/useProjectDraft', async original => ({ ...await original<typeof import('../../hooks/useProjectDraft')>(), useProjectDraftScope: () => identity.scope }));
const { createSession, listWorktrees } = vi.hoisted(() => ({ createSession: vi.fn(), listWorktrees: vi.fn() }));
vi.mock('../../api', () => ({ api: { createSession, listWorktrees } }));
vi.mock('../topbar/SessionPicker', () => ({ SessionPicker: () => null }));
const brief: Brief = { projectId: 'p-1', revision: 2, purpose: 'Journal', audience: '', constraints: '', criteria: [], updatedAt: '' };
const plan: ImplementationPlan = { projectId: 'p-1', revision: 3, briefRevision: 2, specificationRevision: 4, tasks: [{ id: 't-1', title: 'Build journal', status: 'todo', requirementIds: ['R-1'] }], updatedAt: '' };
const coverage: RequirementCoverage = { projectId: 'p-1', specification: { revision: 4, digest: 'spec', status: 'approved' }, design: { revision: 5, digest: 'design', status: 'approved' }, source: { commit: '', sourceDigest: 'unborn-source', dirty: true, available: true }, rows: [{ requirementId: 'R-1', title: 'Keep notes', acceptance: 'Notes survive reload.', taskIds: ['t-1'], doneTaskIds: [], evidenceRunIds: [], evidenceStatus: 'unverified' }] };
beforeEach(() => {
  vi.restoreAllMocks(); resetCockpit(); localStorage.clear(); identity.scope = 'owner-workspace'; createSession.mockClear(); listWorktrees.mockReset(); listWorktrees.mockResolvedValue([]);
  useCockpit.setState({ projects: [aProject({ id: 'p-1' })], branchesByProject: { 'p-1': [aBranch({ id: 'b-main', projectId: 'p-1', name: 'main', isHead: true }), aBranch({ id: 'b-linked', projectId: 'p-1', name: 'experiment' })] }, worktreesByBranch: { 'b-main': [aWorktree({ id: 'w-main', projectId: 'p-1', branchId: 'b-main', name: 'Journal', isPrimary: true })], 'b-linked': [aWorktree({ id: 'w-linked', projectId: 'p-1', branchId: 'b-linked', name: 'Experiment', isPrimary: false })] }, selection: { projectId: 'p-1', branchId: null, worktreeId: null, sessionId: null } });
  vi.spyOn(productApi, 'plan').mockResolvedValue(plan);
  vi.spyOn(productApi, 'checks').mockResolvedValue({ runs: [], suggested: [], coverage });
});
const ready = () => screen.findByText('Choose a build location before preparing the run.');

it('lets a fresh project explicitly choose its actual checkout at preparation without requiring the rail or starting coding', async () => {
  const prepare = vi.fn(); render(<ProjectPlan projectId="p-1" brief={brief} onPrepareRun={prepare} />); await ready();
  const location = screen.getByRole('combobox', { name: 'Build location' });
  expect((location as HTMLSelectElement).value).toBe('');
  expect((screen.getByRole('button', { name: 'Prepare project run' }) as HTMLButtonElement).disabled).toBe(true);
  expect((screen.getByRole('option', { name: /experiment.*linked checkout/ }) as HTMLOptionElement).disabled).toBe(true);
  await userEvent.selectOptions(location, 'w-main');
  await waitFor(() => expect(useCockpit.getState().selection).toEqual({ projectId: 'p-1', branchId: 'b-main', worktreeId: 'w-main', sessionId: null }));
  expect((location as HTMLSelectElement).selectedOptions[0].textContent).toBe('main · Journal · main checkout');
  await userEvent.click(screen.getByRole('button', { name: 'Prepare project run' }));
  expect(prepare).toHaveBeenCalledExactlyOnceWith({ worktreeId: 'w-main', implementationContext: { projectId: 'p-1', briefRevision: 2, planRevision: 3, specificationRevision: 4, designPlanRevision: 5, taskIds: ['t-1'] } });
  expect(createSession).not.toHaveBeenCalled();
});

it('does not silently select the first checkout even when its branch was selected elsewhere', async () => {
  useCockpit.setState({ selection: { projectId: 'p-1', branchId: 'b-main', worktreeId: null, sessionId: null } });
  render(<ProjectPlan projectId="p-1" brief={brief} onPrepareRun={vi.fn()} />); await ready();
  expect((screen.getByRole('combobox', { name: 'Build location' }) as HTMLSelectElement).value).toBe('');
  expect(useCockpit.getState().selection.worktreeId).toBeNull();
  expect((screen.getByRole('button', { name: 'Prepare project run' }) as HTMLButtonElement).disabled).toBe(true);
});

it('keeps the exact requested checkout behind the shared dirty guard and discards it when navigation is cancelled', async () => {
  const discard = vi.fn();
  useCockpit.setState({ unsavedProjectDrafts: { existing: { projectId: 'p-1', label: 'Unsaved context', lens: 'develop', discard } } });
  render(<ProjectPlan projectId="p-1" brief={brief} onPrepareRun={vi.fn()} />); await ready();
  const location = screen.getByRole('combobox', { name: 'Build location' });
  await userEvent.selectOptions(location, 'w-main');
  expect(useCockpit.getState().pendingNavigation).toEqual({ kind: 'branch', projectId: 'p-1', branchId: 'b-main', worktreeId: 'w-main' });
  expect(useCockpit.getState().selection.worktreeId).toBeNull();
  act(() => useCockpit.getState().cancelNavigation());
  expect(useCockpit.getState().selection.branchId).toBeNull(); expect(discard).not.toHaveBeenCalled();
  await userEvent.selectOptions(location, 'w-main');
  act(() => useCockpit.getState().discardDraftsAndNavigate());
  await waitFor(() => expect(useCockpit.getState().selection.worktreeId).toBe('w-main'));
  expect(discard).toHaveBeenCalledOnce(); expect(createSession).not.toHaveBeenCalled();
});

it('does not carry a pending checkout choice into a different workspace with reused project IDs', async () => {
  useCockpit.setState({ unsavedProjectDrafts: { existing: { projectId: 'p-1', label: 'Unsaved context', lens: 'develop' } } });
  const view = render(<ProjectPlan projectId="p-1" brief={brief} onPrepareRun={vi.fn()} />); await ready();
  await userEvent.selectOptions(screen.getByRole('combobox', { name: 'Build location' }), 'w-main');
  identity.scope = 'different-workspace';
  view.rerender(<ProjectPlan projectId="p-1" brief={brief} onPrepareRun={vi.fn()} />);
  act(() => useCockpit.setState({ pendingNavigation: null, selection: { projectId: 'p-1', branchId: 'b-main', worktreeId: null, sessionId: null } }));
  await ready();
  expect(useCockpit.getState().selection.worktreeId).toBeNull();
  expect((screen.getByRole('combobox', { name: 'Build location' }) as HTMLSelectElement).value).toBe('');
});

it('does not apply a checkout removed while the navigation guard was open', async () => {
  useCockpit.setState({ unsavedProjectDrafts: { existing: { projectId: 'p-1', label: 'Unsaved context', lens: 'develop' } } });
  render(<ProjectPlan projectId="p-1" brief={brief} onPrepareRun={vi.fn()} />); await ready();
  await userEvent.selectOptions(screen.getByRole('combobox', { name: 'Build location' }), 'w-main');
  act(() => useCockpit.setState({ worktreesByBranch: {} }));
  act(() => useCockpit.getState().discardDraftsAndNavigate());
  expect(useCockpit.getState().toasts.some(toast => toast.message.includes('That checkout is no longer available'))).toBe(true);
  expect(useCockpit.getState().unsavedProjectDrafts.existing).toBeDefined();
  expect(useCockpit.getState().selection.worktreeId).toBeNull();
  expect((screen.getByRole('button', { name: 'Prepare project run' }) as HTMLButtonElement).disabled).toBe(true);
});


it('preserves the currently selected checkout when a same-branch location change is cancelled', async () => {
  const linked = aWorktree({ id: 'w-same-branch', projectId: 'p-1', branchId: 'b-main', name: 'Other checkout', isPrimary: false });
  useCockpit.setState(state => ({ worktreesByBranch: { ...state.worktreesByBranch, 'b-main': [...state.worktreesByBranch['b-main'], linked] }, selection: { projectId: 'p-1', branchId: 'b-main', worktreeId: linked.id, sessionId: null }, unsavedProjectDrafts: { existing: { projectId: 'p-1', label: 'Unsaved context', lens: 'develop' } } }));
  render(<ProjectPlan projectId="p-1" brief={brief} onPrepareRun={vi.fn()} />);
  await screen.findByRole('combobox', { name: 'Build location' });
  await userEvent.selectOptions(screen.getByRole('combobox', { name: 'Build location' }), 'w-main');
  expect(useCockpit.getState().pendingNavigation).toEqual({ kind: 'branch', projectId: 'p-1', branchId: 'b-main', worktreeId: 'w-main' });
  expect(useCockpit.getState().selection.worktreeId).toBe(linked.id);
  act(() => useCockpit.getState().cancelNavigation());
  expect(useCockpit.getState().selection.worktreeId).toBe(linked.id);
  expect((screen.getByRole('combobox', { name: 'Build location' }) as HTMLSelectElement).value).toBe(linked.id);
  expect(createSession).not.toHaveBeenCalled();
});


it('reads the actual main checkout when the project rail has never been expanded, without selecting a branch', async () => {
  const checkout = aWorktree({ id: 'w-fresh', projectId: 'p-1', branchId: 'b-main', name: 'Fresh journal', isPrimary: true });
  useCockpit.setState({ worktreesByBranch: {} });
  listWorktrees.mockResolvedValue([checkout]);
  const prepare = vi.fn(); render(<ProjectPlan projectId="p-1" brief={brief} onPrepareRun={prepare} />);
  await screen.findByRole('option', { name: 'main · Fresh journal · main checkout' });
  expect(listWorktrees).toHaveBeenCalledExactlyOnceWith('p-1', 'b-main');
  expect(useCockpit.getState().selection).toEqual({ projectId: 'p-1', branchId: null, worktreeId: null, sessionId: null });
  expect((screen.getByRole('button', { name: 'Prepare project run' }) as HTMLButtonElement).disabled).toBe(true);
  await userEvent.selectOptions(screen.getByRole('combobox', { name: 'Build location' }), 'w-fresh');
  await userEvent.click(screen.getByRole('button', { name: 'Prepare project run' }));
  expect(prepare.mock.calls[0][0].worktreeId).toBe('w-fresh');
  expect(createSession).not.toHaveBeenCalled();
});

it('ignores a late checkout response after the workspace changes', async () => {
  useCockpit.setState({ worktreesByBranch: {} });
  let resolve!: (value: ReturnType<typeof aWorktree>[]) => void;
  listWorktrees.mockReturnValueOnce(new Promise(done => { resolve = done; }));
  const view = render(<ProjectPlan projectId="p-1" brief={brief} onPrepareRun={vi.fn()} />);
  await waitFor(() => expect(listWorktrees).toHaveBeenCalledTimes(1));
  identity.scope = 'different-workspace';
  view.rerender(<ProjectPlan projectId="p-1" brief={brief} onPrepareRun={vi.fn()} />);
  await act(async () => resolve([aWorktree({ id: 'w-old', projectId: 'p-1', branchId: 'b-main', name: 'Private old checkout', isPrimary: true })]));
  expect(screen.queryByRole('option', { name: /Private old checkout/ })).toBeNull();
  expect(useCockpit.getState().worktreesByBranch['b-main']).toEqual([]);
  expect(useCockpit.getState().selection.worktreeId).toBeNull();
});

it('keeps drafts when the selected project disappears before the guarded checkout choice is confirmed', async () => {
  useCockpit.setState({ unsavedProjectDrafts: { existing: { projectId: 'p-1', label: 'Unsaved context', lens: 'develop' } } });
  render(<ProjectPlan projectId="p-1" brief={brief} onPrepareRun={vi.fn()} />); await ready();
  await userEvent.selectOptions(screen.getByRole('combobox', { name: 'Build location' }), 'w-main');
  act(() => useCockpit.getState().setProjects([]));
  act(() => useCockpit.getState().discardDraftsAndNavigate());
  expect(useCockpit.getState().selection.projectId).toBeNull();
  expect(useCockpit.getState().selection.worktreeId).toBeNull();
  expect(useCockpit.getState().unsavedProjectDrafts.existing).toBeDefined();
});
