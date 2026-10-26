import { beforeEach, expect, it, vi } from 'vitest';
import { act, render, waitFor } from '@testing-library/react';
import { ProjectHomeProvider, useProjectHome } from './ProjectHomeProvider';
import { projectHomeApi } from '../../api/projectHome';
import { resetCockpit, useCockpit } from '../../store/store';
import { ApiError } from '../../api/client';
import { aProject } from '../../test/fixtures';

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

it.each(['actor workspace', 'catalogue epoch', 'catalogue membership'])('never restores a retained outage projection across a changed %s', async boundary => {
  vi.spyOn(projectHomeApi, 'getAll').mockResolvedValueOnce(home)
    .mockRejectedValueOnce(new ApiError('network_unavailable', 'Connection interrupted.', 0))
    .mockImplementation(() => new Promise<typeof home>(() => {}));
  const rendered = render(<ProjectHomeProvider><Probe /></ProjectHomeProvider>);
  await waitFor(() => expect(view?.activityAvailable).toBe(true));
  act(() => view!.refresh());
  await waitFor(() => expect(view?.error).toBe('Connection interrupted.'));
  expect(view?.home?.projects).toHaveLength(2);
  expect(view?.activityAvailable).toBe(false);
  if (boundary === 'actor workspace') {
    identity.scope = 'other:workspace';
    rendered.rerender(<ProjectHomeProvider><Probe /></ProjectHomeProvider>);
  } else if (boundary === 'catalogue epoch') {
    act(() => useCockpit.setState(state => ({ projectCatalogueEpoch: state.projectCatalogueEpoch + 1 })));
  } else {
    act(() => useCockpit.getState().setProjects([aProject({id: 'p-new'})]));
  }
  expect(view?.home).toBeNull();
  expect(view?.activityAvailable).toBe(false);
  if (boundary === 'actor workspace') {
    identity.scope = 'actor:workspace';
    rendered.rerender(<ProjectHomeProvider><Probe /></ProjectHomeProvider>);
    expect(view?.home).toBeNull();
  }
});

it('rechecks Home immediately after a created project enters the catalogue without treating the old empty projection as current', async () => {
  const empty = {...home, pinnedProjectIds: [], projects: []};
  const created = {...home, pinnedProjectIds: [], projects: [{...home.projects[0], id: 'p-new', name: 'New project', working: false, needsInput: false}]};
  let finish!: (value: typeof home) => void;
  const read = vi.spyOn(projectHomeApi, 'getAll').mockResolvedValueOnce(empty)
    .mockImplementationOnce(() => new Promise(resolve => { finish = resolve; }));
  const pin = vi.spyOn(projectHomeApi, 'pin');
  render(<ProjectHomeProvider><Probe /></ProjectHomeProvider>);
  await waitFor(() => expect(view?.home?.projects).toHaveLength(0));
  act(() => {
    useCockpit.getState().setProjects([aProject({id: 'p-new', name: 'New project'})]);
    useCockpit.getState().selectProject('p-new');
  });
  expect(view?.home).toBeNull(); expect(view?.activityAvailable).toBe(false);
  await waitFor(() => expect(read).toHaveBeenCalledTimes(2));
  expect(read.mock.calls[0][0]).toHaveProperty('aborted', true);
  await act(async () => { await view!.pin('p-new', true); });
  expect(pin).not.toHaveBeenCalled();
  await act(async () => finish(created));
  await waitFor(() => expect(view?.home?.projects[0].id).toBe('p-new'));
  expect(view?.activityAvailable).toBe(true); expect(read).toHaveBeenCalledTimes(2);
});

it('keeps the verified projection and poll when only catalogue order, names or selection change', async () => {
  const a = aProject({id: 'p-a'}), b = aProject({id: 'p-b'});
  useCockpit.getState().setProjects([a, b]);
  const read = vi.spyOn(projectHomeApi, 'getAll').mockResolvedValue(home);
  render(<ProjectHomeProvider><Probe /></ProjectHomeProvider>);
  await waitFor(() => expect(view?.activityAvailable).toBe(true));
  const verified = view?.home;
  act(() => {
    useCockpit.getState().setProjects([{...b, name: 'Renamed B'}, a]);
    useCockpit.getState().selectProject('p-b');
  });
  expect(view?.home).toBe(verified); expect(view?.activityAvailable).toBe(true);
  expect(read).toHaveBeenCalledOnce(); expect(read.mock.calls[0][0]).toHaveProperty('aborted', false);
});

it('never revives an old projection or late ignored-abort read after catalogue membership changes away and back', async () => {
  const a = aProject({id: 'p-a'}), b = aProject({id: 'p-b'});
  useCockpit.getState().setProjects([a]);
  let finishB!: (value: typeof home) => void, finishA!: (value: typeof home) => void;
  const read = vi.spyOn(projectHomeApi, 'getAll').mockResolvedValueOnce({...home, projects: [home.projects[0]]})
    .mockImplementationOnce(() => new Promise(resolve => { finishB = resolve; }))
    .mockImplementationOnce(() => new Promise(resolve => { finishA = resolve; }));
  render(<ProjectHomeProvider><Probe /></ProjectHomeProvider>);
  await waitFor(() => expect(view?.home?.projects[0].id).toBe('p-a'));
  act(() => useCockpit.getState().setProjects([b]));
  expect(view?.home).toBeNull();
  await waitFor(() => expect(read).toHaveBeenCalledTimes(2));
  act(() => useCockpit.getState().setProjects([a]));
  expect(view?.home).toBeNull();
  await waitFor(() => expect(read).toHaveBeenCalledTimes(3));
  expect(read.mock.calls[1][0]).toHaveProperty('aborted', true);
  await act(async () => finishA({...home, revision: 5, projects: [home.projects[0]]}));
  await waitFor(() => expect(view?.home?.revision).toBe(5));
  await act(async () => finishB({...home, revision: 4, projects: [home.projects[1]]}));
  expect(view?.home?.revision).toBe(5); expect(view?.home?.projects[0].id).toBe('p-a');
  expect(read).toHaveBeenCalledTimes(3);
});
