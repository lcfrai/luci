import { beforeEach, expect, it, vi } from 'vitest';
import { act, render, waitFor } from '@testing-library/react';
import { ProjectHomeProvider, useProjectHome } from './ProjectHomeProvider';
import { projectHomeApi } from '../../api/projectHome';
import { resetCockpit, useCockpit } from '../../store/store';

const identity = vi.hoisted(() => ({scope: 'actor:workspace'}));
vi.mock('../../hooks/useProjectDraft', () => ({ useProjectDraftScope: () => identity.scope }));
const stop = { id: `stop-${'a'.repeat(64)}`, at: '2026-10-15T10:00:00Z', kind: 'stopped' as const };
const home = { version: 1 as const, revision: 3, pinnedProjectIds: ['p-a'], projects: [
  { id: 'p-a', name: 'A', working: true, needsInput: true, queued: false, unknown: false, stops: [] },
  { id: 'p-b', name: 'B', working: false, needsInput: false, queued: false, unknown: false, stops: [stop], newestObservedStop: stop.id },
] };
let view: ReturnType<typeof useProjectHome> = null;
function Probe() { view = useProjectHome(); return <div>{view?.home?.projects.length ?? 'loading'}</div>; }
beforeEach(() => { resetCockpit(); identity.scope = 'actor:workspace'; useCockpit.getState().selectProject('p-a'); vi.restoreAllMocks(); });

it('retains concurrent input and work; acknowledges the exact stop only after guarded navigation', async () => {
  vi.spyOn(projectHomeApi, 'getAll').mockResolvedValue(home);
  const ack = vi.spyOn(projectHomeApi, 'acknowledge').mockResolvedValue({ revision: 4, acknowledgedStopIds: [stop.id] });
  render(<ProjectHomeProvider><Probe /></ProjectHomeProvider>);
  await waitFor(() => expect(view?.home?.projects).toHaveLength(2));
  expect(view?.home?.projects[0]).toMatchObject({ working: true, needsInput: true });
  useCockpit.getState().setUnsavedProjectDraft('editor', { projectId: 'p-a', label: 'Definition', lens: 'discover', retained: true, discard: vi.fn() });
  act(() => view!.open('p-b'));
  expect(useCockpit.getState().pendingNavigation).toEqual({ kind: 'project', projectId: 'p-b' });
  expect(ack).not.toHaveBeenCalled();
  act(() => useCockpit.getState().discardDraftsAndNavigate());
  await waitFor(() => expect(ack).toHaveBeenCalledWith('p-b', [stop.id], 3));
  expect(useCockpit.getState().selection.projectId).toBe('p-b');
});

it('does not acknowledge a stop when the dirty navigation is cancelled', async () => {
  vi.spyOn(projectHomeApi, 'getAll').mockResolvedValue(home);
  const ack = vi.spyOn(projectHomeApi, 'acknowledge').mockResolvedValue({ revision: 4, acknowledgedStopIds: [stop.id] });
  render(<ProjectHomeProvider><Probe /></ProjectHomeProvider>);
  await waitFor(() => expect(view?.home?.projects).toHaveLength(2));
  useCockpit.getState().setUnsavedProjectDraft('editor', { projectId: 'p-a', label: 'Definition', lens: 'discover', retained: true, discard: vi.fn() });
  act(() => { view!.open('p-b'); useCockpit.getState().cancelNavigation(); });
  expect(useCockpit.getState().selection.projectId).toBe('p-a');
  expect(ack).not.toHaveBeenCalled();
});

it('hides the old actor workspace projection on a scope change before a new response arrives', async () => {
  let resolveNext!: (value: typeof home) => void;
  vi.spyOn(projectHomeApi, 'getAll').mockResolvedValueOnce(home).mockImplementationOnce(() => new Promise(resolve => { resolveNext = resolve; }));
  const rendered = render(<ProjectHomeProvider><Probe /></ProjectHomeProvider>);
  await waitFor(() => expect(view?.home?.projects).toHaveLength(2));
  identity.scope = 'other:workspace';
  rendered.rerender(<ProjectHomeProvider><Probe /></ProjectHomeProvider>);
  expect(view?.home).toBeNull();
  act(() => resolveNext({...home, projects: []}));
  await waitFor(() => expect(view?.home?.projects).toHaveLength(0));
});
