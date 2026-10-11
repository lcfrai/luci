// Shell composition uses inert phase/conversation stand-ins; the actual conversation
// boundary, authority and recovered message tests live in OrchestratorPanel.test.tsx.
import { useEffect, useState } from 'react';
import { act, render, screen, waitFor } from '@testing-library/react';
import userEvent from '@testing-library/user-event';
import { beforeEach, expect, it, vi } from 'vitest';
import { AppShell } from './AppShell';
import { resetCockpit, useCockpit } from '../store/store';
import { aProject } from '../test/fixtures';
const { Phase } = vi.hoisted(() => ({ Phase: vi.fn() }));
vi.mock('./lens/DiscoverLens', () => ({ DiscoverLens: () => <Phase label="Discover" /> }));
vi.mock('./lens/DesignLens', () => ({ DesignLens: () => <Phase label="Design" /> }));
vi.mock('./develop/DevelopLens', () => ({ DevelopLens: () => <Phase label="Develop" /> }));
vi.mock('./lens/DeployLens', () => ({ DeployLens: () => <Phase label="Deploy" /> }));
vi.mock('./orchestrator/OrchestratorPanel', () => ({ ORCHESTRATOR_OPEN: 'luci:orchestrator-open', OrchestratorPanel: ({ projectId, primary }: { projectId: string; primary?: boolean }) => <section role={primary ? 'region' : 'complementary'} aria-label={primary ? 'Luci conversation' : 'Project understanding'} data-project={projectId} data-primary={String(primary)}>Conversation for {projectId}</section> }));
function PhaseEditor({ label }: { label: string }) {
  const [text, setText] = useState(''); const projectId = useCockpit(state => state.selection.projectId)!;
  useEffect(() => { const state = useCockpit.getState(); state.setUnsavedProjectDraft('test-editor', text ? { projectId, label: `${label} draft`, lens: label.toLowerCase() as 'discover', retained: true, discard: () => setText('') } : null); return () => state.setUnsavedProjectDraft('test-editor', null); }, [text, projectId, label]);
  return <textarea aria-label={`${label} editor`} value={text} onChange={event => setText(event.target.value)} />;
}
beforeEach(() => { vi.restoreAllMocks(); resetCockpit(); localStorage.clear(); Phase.mockImplementation(PhaseEditor); useCockpit.setState({ projects: [aProject({ id: 'p-1', name: 'Atlas' }), aProject({ id: 'p-2', name: 'Beacon' }), aProject({ id: 'p-3', name: 'Cedar' }), aProject({ id: 'p-4', name: 'Delta' })], selection: { projectId: 'p-1', branchId: null, worktreeId: null, sessionId: null }, lens: 'discover' }); });
it('defaults to the scoped main Luci conversation without mounting phase editors', () => {
  render(<AppShell />); expect(screen.getByRole('region', { name: 'Luci conversation' }).getAttribute('data-project')).toBe('p-1');
  expect(screen.getByRole('button', { name: 'Luci' }).getAttribute('aria-current')).toBe('page'); expect(screen.getByLabelText('Current project: Atlas')).not.toBeNull();
  expect(Phase).not.toHaveBeenCalled(); for (const name of ['Discover', 'Design', 'Develop', 'Deploy']) expect(screen.getByRole('button', { name }).getAttribute('aria-current')).toBeNull();
});
it('keeps the current editor mounted and its unsaved text intact through keyboard Luci and same-phase return', async () => {
  const ui = render(<AppShell />); await userEvent.click(screen.getByRole('button', { name: 'Discover' }));
  const editor = screen.getByLabelText('Discover editor'); await userEvent.type(editor, 'Keep this draft');
  const luci = screen.getByRole('button', { name: 'Luci' }); luci.focus(); await userEvent.keyboard('{Enter}');
  expect(useCockpit.getState().workspaceSurface).toBe('luci'); expect(editor.closest('[hidden][inert]')).not.toBeNull(); expect(editor).toHaveProperty('value', 'Keep this draft'); expect(screen.queryByRole('dialog')).toBeNull();
  await userEvent.click(screen.getByRole('button', { name: 'Discover' })); expect(editor).toBe(screen.getByLabelText('Discover editor')); expect(editor.closest('[hidden]')).toBeNull();
  expect(useCockpit.getState().unsavedProjectDrafts['test-editor']).toBeDefined(); expect(ui.container.querySelectorAll('[data-project="p-1"]')).toHaveLength(1);
});
it('retains the primary conversation when a dirty different-phase switch is cancelled', async () => {
  render(<AppShell />); await userEvent.click(screen.getByRole('button', { name: 'Discover' })); await userEvent.type(screen.getByLabelText('Discover editor'), 'Scoped edits');
  await userEvent.click(screen.getByRole('button', { name: 'Luci' })); await userEvent.click(screen.getByRole('button', { name: 'Design' }));
  await screen.findByRole('dialog', { name: 'Keep your changes?' }); await userEvent.click(screen.getByRole('button', { name: 'Keep editing' }));
  expect(useCockpit.getState().workspaceSurface).toBe('luci'); expect(useCockpit.getState().lens).toBe('discover'); expect(useCockpit.getState().unsavedProjectDrafts['test-editor']).toBeDefined();
  await userEvent.click(screen.getByRole('button', { name: 'Discover' })); expect(screen.getByLabelText('Discover editor')).toHaveProperty('value', 'Scoped edits');
});
it('keeps project selection and context bound through guarded switches in the primary view', async () => {
  render(<AppShell />); await userEvent.click(screen.getByRole('button', { name: 'Discover' })); await userEvent.type(screen.getByLabelText('Discover editor'), 'Atlas draft'); await userEvent.click(screen.getByRole('button', { name: 'Luci' }));
  await userEvent.click(screen.getByRole('treeitem', { name: /Beacon/ })); await screen.findByRole('dialog', { name: 'Keep your changes?' }); expect(screen.getByRole('region', { name: 'Luci conversation', hidden: true }).getAttribute('data-project')).toBe('p-1');
  await userEvent.click(screen.getByRole('button', { name: 'Keep drafts and continue' })); await waitFor(() => expect(useCockpit.getState().selection.projectId).toBe('p-2'));
  expect(useCockpit.getState().workspaceSurface).toBe('luci'); expect(screen.getByLabelText('Current project: Beacon')).not.toBeNull(); expect(screen.getByRole('region', { name: 'Luci conversation' }).getAttribute('data-project')).toBe('p-2'); expect(screen.queryByText('Conversation for p-1')).toBeNull();
});
it('opens the primary view only for the currently selected project event', async () => {
  useCockpit.setState({ workspaceSurface: 'tools' }); render(<AppShell />);
  act(() => window.dispatchEvent(new CustomEvent('luci:orchestrator-open', { detail: { projectId: 'p-2' } }))); expect(useCockpit.getState().workspaceSurface).toBe('tools');
  act(() => window.dispatchEvent(new CustomEvent('luci:orchestrator-open', { detail: { projectId: 'p-1' } }))); expect(useCockpit.getState().workspaceSurface).toBe('luci'); expect(screen.getByLabelText('Discover editor').closest('[hidden][inert]')).not.toBeNull();
});
it('keeps compact Projects navigation accessible, returns focus after Escape, and selects a scoped conversation', async () => {
  const media = vi.spyOn(window, 'matchMedia').mockImplementation(query => ({ matches: query === '(max-width: 760px)', media: query, onchange: null, addEventListener: vi.fn(), removeEventListener: vi.fn(), addListener: vi.fn(), removeListener: vi.fn(), dispatchEvent: () => false }));
  try { const ui = render(<AppShell />); const projects = screen.getByRole('button', { name: 'Projects' }); await userEvent.click(projects); expect(ui.container.querySelector('.shell-main')?.hasAttribute('inert')).toBe(true);
    await userEvent.keyboard('{Escape}'); expect(document.activeElement).toBe(projects); await userEvent.click(projects); await userEvent.click(screen.getByRole('treeitem', { name: /Beacon/ }));
    expect(projects.getAttribute('aria-expanded')).toBe('false'); expect(screen.getByRole('region', { name: 'Luci conversation' }).getAttribute('data-project')).toBe('p-2');
  } finally { media.mockRestore(); }
});
it('provides a Luci starting point without pretending an unselected project has shared history', async () => {
  useCockpit.setState({ selection: { projectId: null, branchId: null, worktreeId: null, sessionId: null } }); render(<AppShell />);
  expect(screen.getByRole('heading', { name: 'Luci' })).not.toBeNull(); expect(screen.getByText(/Choose a project to continue its conversation/)).not.toBeNull(); expect(screen.queryByRole('region', { name: 'Luci conversation' })).toBeNull();
  await userEvent.click(screen.getByRole('button', { name: 'Luci' })); await waitFor(() => expect(document.activeElement).toBe(screen.getByRole('heading', { name: 'Luci' })));
});

it('selects four separate primary conversation destinations without merging their scopes', async () => {
  render(<AppShell />);
  for (const [id, name] of [['p-2', 'Beacon'], ['p-3', 'Cedar'], ['p-4', 'Delta'], ['p-1', 'Atlas']]) {
    await userEvent.click(screen.getByRole('treeitem', { name: new RegExp(name) }));
    expect(screen.getByRole('region', { name: 'Luci conversation' }).getAttribute('data-project')).toBe(id);
    expect(screen.getByLabelText(`Current project: ${name}`)).not.toBeNull(); expect(useCockpit.getState().workspaceSurface).toBe('luci');
  }
});
it('keeps Luci and its selected project through desktop to compact and back without leaving an inert main pane', async () => {
  let compact = false; const listeners = new Set<() => void>();
  const media = vi.spyOn(window, 'matchMedia').mockImplementation(query => ({ get matches() { return compact && query === '(max-width: 760px)'; }, media: query, onchange: null, addEventListener: (_name: string, listener: EventListenerOrEventListenerObject) => { if (query === '(max-width: 760px)') listeners.add(listener as () => void); }, removeEventListener: (_name: string, listener: EventListenerOrEventListenerObject) => listeners.delete(listener as () => void), addListener: vi.fn(), removeListener: vi.fn(), dispatchEvent: () => false }));
  try { const ui = render(<AppShell />); act(() => { compact = true; listeners.forEach(listener => listener()); });
    const projects = screen.getByRole('button', { name: 'Projects' }); await userEvent.click(projects); expect(projects.getAttribute('aria-expanded')).toBe('true');
    act(() => { compact = false; listeners.forEach(listener => listener()); });
    expect(ui.container.querySelector('.shell-main')?.hasAttribute('inert')).toBe(false); expect(screen.getByRole('region', { name: 'Luci conversation' }).getAttribute('data-project')).toBe('p-1');
    expect(useCockpit.getState().workspaceSurface).toBe('luci'); expect(document.activeElement).toBe(projects);
  } finally { media.mockRestore(); }
});
it('does not remount a previously visited project editor invisibly after switching away and back', async () => {
  render(<AppShell />); await userEvent.click(screen.getByRole('button', { name: 'Discover' })); expect(screen.getByLabelText('Discover editor')).not.toBeNull();
  await userEvent.click(screen.getByRole('button', { name: 'Luci' })); await userEvent.click(screen.getByRole('treeitem', { name: /Beacon/ })); await userEvent.click(screen.getByRole('treeitem', { name: /Atlas/ }));
  expect(screen.getByRole('region', { name: 'Luci conversation' }).getAttribute('data-project')).toBe('p-1'); expect(screen.queryByLabelText('Discover editor')).toBeNull();
});
