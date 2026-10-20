import { act, fireEvent, render, screen, waitFor, within } from '@testing-library/react';
import userEvent from '@testing-library/user-event';
import { beforeEach, expect, it, vi } from 'vitest';
import { productApi, type Design, type GitState } from '../../api/product';
import { definitionApi } from '../../api/definition';
import { designPlanApi } from '../../api/designPlan';
import { notifyProjectContextChanged, requestProjectContextEdit } from '../../api/projectContextEvents';
import { resetCockpit, useCockpit } from '../../store/store';
import { context, view } from '../orchestrator/fixtures';
import { orchestratorApi } from '../../api/orchestrator';
import { OrchestratorPanel } from '../orchestrator/OrchestratorPanel';
import { DraftNavigationDialog } from '../common/DraftNavigationDialog';
import { ProjectWork } from './ProjectWork';

const identity = vi.hoisted(() => ({ scope: 'account-a/workspace-a', read: true, edit: true }));
const runActions = vi.hoisted(() => ({ open: undefined as undefined | ((target: { scope: string; projectId: string; sessionId: string; origin: Element | null }) => void) }));
vi.mock('../../hooks/useProjectDraft', async original => ({ ...await original<typeof import('../../hooks/useProjectDraft')>(), useProjectDraftScope: () => identity.scope }));
vi.mock('../../hooks/useProjectAccess', () => ({ useProjectAccess: () => ({
  canRead: identity.read, canEdit: identity.edit, canExecute: identity.edit, canAssist: identity.edit,
  permissions: { read: identity.read, edit: identity.edit, execute: identity.edit, secrets: identity.edit, deploy: identity.edit, admin: identity.edit },
  loading: false, error: '', message: identity.read ? '' : 'Project reading access is unavailable.',
}) }));
vi.mock('./ProjectRuns', () => ({ ProjectRuns: ({ onOpenCodingSession }: { onOpenCodingSession: typeof runActions.open }) => { runActions.open = onOpenCodingSession; return null; } }));
const saved = context();
const sketch: Design = { projectId: 'p-1', revision: 0, briefRevision: 0, frames: [], updatedAt: '' };
const handoff = { ...saved.designPlan, revision: 2, status: 'approved' as const, screens: [
  { id: 'journal', name: 'Journal', route: '/', purpose: 'Record a note', states: ['Empty', 'Saved', 'Save failed'], widths: [390, 1440], requirementIds: ['REQ-1'], accessibility: 'Labelled keyboard controls' },
  { id: 'history', name: 'History', route: '/history', purpose: 'Read saved notes', states: ['Loading', 'Loaded', 'Unavailable'], widths: [390, 1440], requirementIds: ['REQ-1'], accessibility: 'Readable list and retry control' },
] };
const git: GitState = { clean: true, branch: 'main', changes: [], log: [{ hash: 'a'.repeat(40), subject: 'Initial application' }], diff: '' };
function deferred<T>() { let resolve!: (value: T) => void; return { promise: new Promise<T>(done => { resolve = done; }), resolve }; }
beforeEach(() => {
  vi.restoreAllMocks(); localStorage.clear(); resetCockpit(); Object.assign(identity, { scope: 'account-a/workspace-a', read: true, edit: true });
  runActions.open = undefined;
  vi.spyOn(productApi, 'brief').mockImplementation(async id => ({ ...saved.brief, projectId: id }));
  vi.spyOn(productApi, 'design').mockImplementation(async id => ({ ...sketch, projectId: id }));
  vi.spyOn(productApi, 'git').mockResolvedValue(git);
  vi.spyOn(definitionApi, 'get').mockImplementation(async id => ({ ...saved.specification, projectId: id }));
  vi.spyOn(designPlanApi, 'get').mockImplementation(async id => ({ ...handoff, projectId: id }));
  vi.spyOn(productApi, 'plan').mockImplementation(async id => ({ ...saved.plan, projectId: id }));
  vi.spyOn(productApi, 'checks').mockResolvedValue({ runs: [], suggested: [] });
});

it('describes the approved structured handoff and its behavior even when no sketch exists', async () => {
  render(<ProjectWork projectId="p-1" />);
  await screen.findByText('Design handoff revision 2 · Approved.');
  expect(screen.getByText('Definition revision 2 · Approved.')).not.toBeNull();
  expect(screen.getByText('2 planned screens · 6 documented states · based on definition 2.')).not.toBeNull();
  expect(screen.getByText('No optional sketch saved.')).not.toBeNull();
  expect(screen.queryByText(/No saved design yet/)).toBeNull();
  expect(definitionApi.get).toHaveBeenCalledExactlyOnceWith('p-1');
  expect(designPlanApi.get).toHaveBeenCalledExactlyOnceWith('p-1');
});

it.each(['outdated status', 'new definition', 'new brief', 'new sketch', 'unapproved definition'])('shows a handoff needing review after %s instead of claiming its earlier approval is current', async changed => {
  if (changed === 'outdated status') vi.mocked(designPlanApi.get).mockResolvedValue({ ...handoff, status: 'outdated' });
  if (changed === 'new definition') vi.mocked(definitionApi.get).mockResolvedValue({ ...saved.specification, revision: 3 });
  if (changed === 'new brief') vi.mocked(productApi.brief).mockResolvedValue({ ...saved.brief, revision: 2 });
  if (changed === 'new sketch') vi.mocked(productApi.design).mockResolvedValue({ ...sketch, revision: 1 });
  if (changed === 'unapproved definition') vi.mocked(definitionApi.get).mockResolvedValue({ ...saved.specification, status: 'draft' });
  render(<ProjectWork projectId="p-1" />);
  await screen.findByText('Design handoff revision 2 · Needs review.');
  expect(screen.getByText(/Review and save it in Design, then approve the current context/)).not.toBeNull();
  expect(screen.queryByText('Design handoff revision 2 · Approved.')).toBeNull();
});

it('distinguishes an unapproved handoff from a sketch without inventing a review', async () => {
  vi.mocked(designPlanApi.get).mockResolvedValue({ ...handoff, status: 'draft' });
  const mounted = render(<ProjectWork projectId="p-1" />);
  await screen.findByText('Design handoff revision 2 · Draft.');
  expect(screen.getByText('Review and approve this design handoff before preparing a build.')).not.toBeNull();
  mounted.unmount(); vi.mocked(designPlanApi.get).mockResolvedValue({ ...handoff, revision: 0, screens: [] });
  vi.mocked(productApi.design).mockResolvedValue({ ...sketch, revision: 5, briefRevision: 1 });
  render(<ProjectWork projectId="p-1" />);
  await screen.findByText(/No structured design handoff saved yet/);
  expect(screen.getByText('Optional sketch revision 5, based on brief 1.')).not.toBeNull();
});

it('refreshes only matching scoped facts while retaining the same plan, check and child draft controls', async () => {
  render(<ProjectWork projectId="p-1"><input aria-label="Additional draft" defaultValue="Keep me" /></ProjectWork>);
  const task = await screen.findByLabelText('Task 1') as HTMLInputElement;
  fireEvent.change(task, { target: { value: 'My unfinished task' } });
  const args = screen.getByLabelText('Arguments · JSON array') as HTMLInputElement;
  fireEvent.change(args, { target: { value: '["unfinished' } });
  const child = screen.getByLabelText('Additional draft') as HTMLInputElement;
  fireEvent.change(child, { target: { value: 'Keep my other work' } });
  act(() => { notifyProjectContextChanged('other-account', 'p-1'); notifyProjectContextChanged(identity.scope, 'other-project'); });
  expect(designPlanApi.get).toHaveBeenCalledTimes(1);
  vi.mocked(productApi.brief).mockResolvedValue({ ...saved.brief, revision: 2, purpose: 'Amended booking service' });
  vi.mocked(definitionApi.get).mockResolvedValue({ ...saved.specification, revision: 3, briefRevision: 2 });
  vi.mocked(designPlanApi.get).mockResolvedValue({ ...handoff, status: 'outdated' });
  act(() => notifyProjectContextChanged(identity.scope, 'p-1'));
  await screen.findByText('Amended booking service');
  expect(screen.getByText('Design handoff revision 2 · Needs review.')).not.toBeNull();
  expect(screen.getByLabelText('Task 1')).toBe(task); expect(task.value).toBe('My unfinished task');
  expect(screen.getByLabelText('Arguments · JSON array')).toBe(args); expect(args.value).toBe('["unfinished');
  expect(screen.getByLabelText('Additional draft')).toBe(child); expect(child.value).toBe('Keep my other work');
  expect(productApi.plan).toHaveBeenCalledTimes(1); expect(productApi.checks).toHaveBeenCalledTimes(2); // Plan + Checks initial reads only.
});

it('retains drafts and labels facts as last-read when refreshing them fails', async () => {
  render(<ProjectWork projectId="p-1" />); const task = await screen.findByLabelText('Task 1') as HTMLInputElement;
  fireEvent.change(task, { target: { value: 'Keep on network failure' } });
  vi.mocked(designPlanApi.get).mockRejectedValue(new Error('Connection interrupted'));
  await userEvent.click(screen.getByRole('button', { name: 'Refresh project facts' }));
  await screen.findByText(/facts below are from the last successful read/);
  expect(screen.getByLabelText('Task 1')).toBe(task); expect(task.value).toBe('Keep on network failure');
});

it('ignores older facts after a newer scoped refresh has completed', async () => {
  render(<ProjectWork projectId="p-1" />); await screen.findByText('Design handoff revision 2 · Approved.');
  const older = deferred<typeof handoff>();
  vi.mocked(designPlanApi.get).mockReturnValueOnce(older.promise).mockResolvedValue({ ...handoff, revision: 4 });
  act(() => notifyProjectContextChanged(identity.scope, 'p-1'));
  act(() => notifyProjectContextChanged(identity.scope, 'p-1'));
  await screen.findByText('Design handoff revision 4 · Approved.');
  await act(async () => older.resolve({ ...handoff, revision: 3 }));
  expect(screen.queryByText('Design handoff revision 3 · Approved.')).toBeNull();
});

it.each(['account', 'workspace', 'project'])('fences private facts and late reads across a changed %s', async changed => {
  const earlier = deferred<typeof handoff>();
  vi.mocked(designPlanApi.get).mockReturnValueOnce(earlier.promise);
  const mounted = render(<ProjectWork projectId="p-1" />);
  const projectId = changed === 'project' ? 'p-2' : 'p-1';
  identity.scope = `new-${changed}`;
  vi.mocked(designPlanApi.get).mockResolvedValue({ ...handoff, projectId, revision: 8 });
  mounted.rerender(<ProjectWork projectId={projectId} />);
  await screen.findByText('Design handoff revision 8 · Approved.');
  await act(async () => earlier.resolve({ ...handoff, revision: 7 }));
  expect(screen.queryByText('Design handoff revision 7 · Approved.')).toBeNull();
});

it('refuses a foreign structured context response and hides facts after confirmed read denial', async () => {
  vi.mocked(designPlanApi.get).mockResolvedValue({ ...handoff, projectId: 'foreign' });
  const mounted = render(<ProjectWork projectId="p-1" />);
  await screen.findByText(/saved context belongs to another project/);
  expect(screen.queryByText(/Design handoff revision/)).toBeNull();
  vi.mocked(designPlanApi.get).mockResolvedValue(handoff);
  await userEvent.click(screen.getByRole('button', { name: 'Refresh project facts' }));
  await screen.findByText('Design handoff revision 2 · Approved.');
  identity.read = false; mounted.rerender(<ProjectWork projectId="p-1" />);
  expect(screen.queryByText(/Design handoff revision/)).toBeNull();
  expect(screen.getByText('Project reading access is unavailable.')).not.toBeNull();
});

it('does not restore private facts when a pending refresh completes after read access is revoked', async () => {
  const mounted = render(<ProjectWork projectId="p-1" />); await screen.findByText('Design handoff revision 2 · Approved.');
  const pending = deferred<typeof handoff>(); vi.mocked(designPlanApi.get).mockReturnValueOnce(pending.promise);
  await userEvent.click(screen.getByRole('button', { name: 'Refresh project facts' }));
  identity.read = false; mounted.rerender(<ProjectWork projectId="p-1" />);
  await act(async () => pending.resolve({ ...handoff, revision: 9 }));
  expect(screen.queryByText(/Design handoff revision/)).toBeNull();
  expect(screen.queryByRole('region', { name: 'Implementation plan' })).toBeNull();
});

it('opens and focuses the exact implementation plan for a read-only scoped request without resetting drafts', async () => {
  const mounted = render(<ProjectWork projectId="p-1" expanded={false} />);
  await waitFor(() => expect(mounted.container.querySelector('.project-plan')).not.toBeNull());
  const plan = screen.getByRole('region', { name: 'Implementation plan', hidden: true });
  identity.edit = false; mounted.rerender(<ProjectWork projectId="p-1" expanded={false} />);
  const cancelForeign = requestProjectContextEdit('foreign-scope', 'p-1', 'plan');
  expect(mounted.container.querySelector<HTMLDetailsElement>('.project-work')?.open).toBe(false); cancelForeign();
  act(() => { requestProjectContextEdit(identity.scope, 'p-1', 'plan'); });
  await waitFor(() => expect(document.activeElement).toBe(plan));
  expect(mounted.container.querySelector<HTMLDetailsElement>('.project-work')?.open).toBe(true);
  expect(screen.getByRole('region', { name: 'Implementation plan' })).toBe(plan);
  expect(definitionApi.get).toHaveBeenCalledTimes(1); expect(productApi.plan).toHaveBeenCalledTimes(1);
});

it('waits for delayed facts to reveal the plan, and does not take focus from newer work', async () => {
  const delayed = deferred<typeof handoff>(); vi.mocked(designPlanApi.get).mockReturnValueOnce(delayed.promise);
  render(<><input aria-label="Other work" /><ProjectWork projectId="p-1" expanded={false} /></>);
  act(() => { requestProjectContextEdit(identity.scope, 'p-1', 'plan'); });
  const other = screen.getByRole('textbox', { name: 'Other work' }); await userEvent.click(other);
  await act(async () => delayed.resolve(handoff));
  await screen.findByRole('region', { name: 'Implementation plan' });
  expect(document.activeElement).toBe(other);
});

it('reveals a retained plan recovery without restoring or overwriting it automatically', async () => {
  const mounted = render(<ProjectWork projectId="p-1" />);
  const task = await screen.findByLabelText('Task 1'); fireEvent.change(task, { target: { value: 'Unfinished recovered task' } });
  mounted.unmount(); render(<ProjectWork projectId="p-1" expanded={false} />);
  act(() => { requestProjectContextEdit(identity.scope, 'p-1', 'plan'); });
  const region = await screen.findByRole('region', { name: 'Implementation plan' });
  await waitFor(() => expect(document.activeElement).toBe(region));
  await within(region).findByRole('region', { name: 'Implementation plan draft recovery' });
  expect((within(region).getByLabelText('Task 1') as HTMLInputElement).value).toBe('Build booking');
  expect(within(region).getByRole('button', { name: 'Restore draft' })).not.toBeNull();
});

it('finishes a guarded overview plan navigation with focus in the plan after the draft dialog restores its trigger', async () => {
  vi.spyOn(orchestratorApi, 'get').mockResolvedValue(view({ nextAction: { id: 'plan', label: 'Review implementation tasks', reason: 'Review the saved tasks.', lens: 'develop' } }));
  vi.spyOn(productApi, 'agentConnections').mockResolvedValue({ revision: 0, activeConnectionId: '', connections: [] });
  useCockpit.setState(state => ({ selection: { ...state.selection, projectId: 'p-1' }, lens: 'design' }));
  const discard = vi.fn();
  useCockpit.getState().setUnsavedProjectDraft('sketch', { projectId: 'p-1', label: 'Sketch', lens: 'design', retained: true, discard });
  render(<><OrchestratorPanel projectId="p-1" /><DraftNavigationDialog /><ProjectWork projectId="p-1" expanded={false} /></>);
  await screen.findByText('Design handoff revision 2 · Approved.');
  await userEvent.click(await screen.findByRole('button', { name: 'Review implementation tasks' }));
  const dialog = await screen.findByRole('dialog', { name: 'Keep your changes?' });
  await userEvent.click(within(dialog).getByRole('button', { name: 'Keep drafts and continue' }));
  const region = await screen.findByRole('region', { name: 'Implementation plan' });
  await waitFor(() => expect(document.activeElement).toBe(region));
  expect(useCockpit.getState().lens).toBe('develop'); expect(discard).not.toHaveBeenCalled();
  expect(screen.queryByRole('dialog', { name: 'Keep your changes?' })).toBeNull();
});

function sessionWorkspace() {
  useCockpit.setState(state => ({ selection: { ...state.selection, projectId: 'p-1', sessionId: 's-1' } }));
  return render(<div className="develop-workspace"><button>Other work</button><ProjectWork projectId="p-1" /><section className="pane" data-session-id="s-1"><div className="pane-body"><textarea aria-label="Session input" /></div></section></div>);
}
function openCurrentSession(origin = document.activeElement) {
  act(() => runActions.open?.({ scope: identity.scope, projectId: 'p-1', sessionId: 's-1', origin }));
}

it('explicitly reveals and focuses an already-selected coding session again without recreating its input', async () => {
  const mounted = sessionWorkspace(); await screen.findByText('Design handoff revision 2 · Approved.');
  const input = screen.getByRole('textbox', { name: 'Session input' });
  const pane = mounted.container.querySelector<HTMLElement>('.pane')!;
  const scroll = vi.spyOn(pane, 'scrollIntoView');
  openCurrentSession(); await waitFor(() => expect(document.activeElement).toBe(input));
  expect(scroll).toHaveBeenCalledWith({ block: 'start', inline: 'nearest' });
  expect(mounted.container.querySelector<HTMLDetailsElement>('.project-work')?.open).toBe(false);
  await userEvent.click(screen.getByRole('button', { name: 'Other work' }));
  openCurrentSession(); await waitFor(() => expect(document.activeElement).toBe(input));
  expect(scroll).toHaveBeenCalledTimes(2);
  expect(screen.getByRole('textbox', { name: 'Session input' })).toBe(input);
  act(() => notifyProjectContextChanged(identity.scope, 'p-1'));
  await waitFor(() => expect(designPlanApi.get).toHaveBeenCalledTimes(2));
  expect(scroll).toHaveBeenCalledTimes(2);
});

it.each(['new focus before response', 'new focus', 'new selection', 'pending navigation', 'read denial'])('does not reveal a session over %s before its scheduled focus transfer', async changed => {
  const mounted = sessionWorkspace(); await screen.findByText('Design handoff revision 2 · Approved.');
  const scroll = vi.spyOn(mounted.container.querySelector<HTMLElement>('.pane')!, 'scrollIntoView');
  const frames: FrameRequestCallback[] = []; vi.spyOn(window, 'requestAnimationFrame').mockImplementation(callback => { frames.push(callback); return frames.length; });
  const other = screen.getByRole('button', { name: 'Other work' });
  const origin = document.activeElement;
  if (changed === 'new focus before response') other.focus();
  openCurrentSession(origin);
  if (changed === 'new focus') other.focus();
  if (changed === 'new selection') act(() => useCockpit.setState(state => ({ selection: { ...state.selection, sessionId: 's-2' } })));
  if (changed === 'pending navigation') act(() => {
    useCockpit.getState().setUnsavedProjectDraft('draft', { projectId: 'p-1', lens: 'develop', label: 'Draft', retained: true, discard: vi.fn() });
    useCockpit.getState().setLens('design');
  });
  if (changed === 'read denial') { identity.read = false; mounted.rerender(<div className="develop-workspace"><ProjectWork projectId="p-1" /></div>); }
  act(() => frames.forEach(callback => callback(0)));
  expect(scroll).not.toHaveBeenCalled();
  if (changed.startsWith('new focus')) expect(document.activeElement).toBe(other);
  if (changed === 'new focus before response') expect(mounted.container.querySelector<HTMLDetailsElement>('.project-work')?.open).toBe(true);
});

it('ignores coding-session destinations from another context scope or project', async () => {
  const mounted = sessionWorkspace(); await screen.findByText('Design handoff revision 2 · Approved.');
  const scroll = vi.spyOn(mounted.container.querySelector<HTMLElement>('.pane')!, 'scrollIntoView');
  act(() => {
    runActions.open?.({ scope: 'other-account', projectId: 'p-1', sessionId: 's-1', origin: document.activeElement });
    runActions.open?.({ scope: identity.scope, projectId: 'p-2', sessionId: 's-1', origin: document.activeElement });
  });
  expect(mounted.container.querySelector<HTMLDetailsElement>('.project-work')?.open).toBe(true);
  expect(scroll).not.toHaveBeenCalled();
});
