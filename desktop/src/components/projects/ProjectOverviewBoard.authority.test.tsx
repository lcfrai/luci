import {act, cleanup, render, screen, waitFor, within} from '@testing-library/react';
import userEvent from '@testing-library/user-event';
import {afterEach, beforeEach, expect, it, vi} from 'vitest';
import {ProjectOverviewBoard} from './ProjectOverviewBoard';
import {projectOverviewApi, type ProjectOverview} from '../../api/projectOverview';
import {swarmQueueApi, type ProjectSwarmQueue, type SwarmQueueItem} from '../../api/swarmQueue';
import {ApiError, type ProblemCode} from '../../api/client';
import {personalPermissions, teamsApi, type ProjectAccess} from '../../api/teams';
import {resetCockpit, useCockpit} from '../../store/store';

// These tests exercise the actual board, bulk owner and Radix WorkflowDialog.
// Workspace membership and HTTP permission responses are controlled; no real
// provider, Docker, browser process or queue mutation is executed.
const environment = vi.hoisted(() => ({scope: 'bulk-account-one', teamId: 'team-one', revision: 0}));
vi.mock('../../hooks/useProjectDraft', () => ({useProjectDraftScope: (projectId: string) => `${environment.scope}:${projectId}`}));
vi.mock('../teams/TeamWorkspaceProvider', () => ({
  useTeamWorkspace: () => ({teamId: environment.teamId, revision: environment.revision, selected: null}),
  useTeamProjectPermissions: () => ({permissions: {read: true, edit: true, execute: true}, loading: false, error: ''}),
}));
vi.mock('../auth/AuthProvider', () => ({useAuth: () => ({profile: {user: {role: 'member'}}})}));
const projects = Array.from({length: 7}, (_, i) => ({id: `scope-p${i + 1}`, name: `Private project ${i + 1}`, path: `/unused/private-${i + 1}`}));
function overview(projectId: string): ProjectOverview {
  return {projectId, observedAt: new Date().toISOString(), status: 'ready', goal: {text: `Outcome ${projectId}`, revision: 1, updatedAt: '', truncated: false}, phase: {id: 'develop', label: 'Develop'}, context: {briefRevision: 1, specificationRevision: 1, definitionStatus: 'approved', designRevision: 1, designStatus: 'approved', planRevision: 1, tasks: {total: 1, recordedDone: 0}}, questions: {status: 'recorded', count: 0, items: [], omitted: 0}, execution: {scope: 'swarm-queue', status: 'recorded', observedAt: '', running: 0, queued: 1, paused: 0, reserved: 0, blocked: 0, unknown: 0, failed: 0, dispatchPaused: false}, nextAction: {id: 'review-plan', label: 'Review the plan', lens: 'develop'}, limitations: []};
}
function queue(projectId: string, extra = false): ProjectSwarmQueue {
  const item: SwarmQueueItem = {id: `q-${projectId}`, revision: 3, projectId, runId: `run-${projectId}`, taskId: `task-${projectId}`, taskTitle: `Private task ${projectId}`, state: 'queued', priority: 'normal', queuedAt: '', updatedAt: ''};
  return {revision: 2, observedAt: '', dispatchPaused: false, hasMore: false, counts: {queued: extra ? 2 : 1, paused: 0, reserved: 0, running: 0, blocked: 0, unknown: 0, completed: 0, failed: 0, cancelled: 0}, items: extra ? [item, {...item, id: `${item.id}-extra`, taskId: `${item.taskId}-extra`, taskTitle: `${item.taskTitle} extra`}] : [item]};
}
function permitted(teamId: string, projectId: string): ProjectAccess {
  return {team: {id: teamId, name: 'Controlled team', revision: 1, createdAt: '', updatedAt: ''}, projectId, permissions: {...personalPermissions}, grants: []};
}
type Held = {teamId: string; projectId: string; signal?: AbortSignal; resolve(value: ProjectAccess): void; reject(cause: Error): void; settled: boolean};
function holdPermissions() {
  const requests: Held[] = [];
  vi.mocked(teamsApi.projectAccess).mockImplementation((teamId, projectId, signal) => new Promise((resolve, reject) => {
    const held: Held = {teamId, projectId, signal, settled: false, resolve(value) {held.settled = true; resolve(value);}, reject(cause) {held.settled = true; reject(cause);}};
    requests.push(held);
  }));
  return requests;
}
function privateListVisible() {
  return [...document.querySelectorAll<HTMLElement>('.overview-cancel-list')].some(list => {
    for (let node: HTMLElement | null = list; node; node = node.parentElement) {
      const style = window.getComputedStyle(node);
      if (node.hidden || style.display === 'none' || style.visibility === 'hidden' || style.visibility === 'collapse') return false;
    }
    return true;
  });
}
function expectPrivateReviewHidden() {
  expect(screen.queryByRole('dialog', {name: 'Cancel selected waiting tasks?'})).toBeNull();
  expect(privateListVisible()).toBe(false);
  expect(screen.queryByRole('button', {name: /^Cancel \d+ waiting tasks?$/})).toBeNull();
  expect(document.body.style.pointerEvents).not.toBe('none');
}
function expectNoWrites() {expect(swarmQueueApi.control).not.toHaveBeenCalled(); expect(swarmQueueApi.enqueueMany).not.toHaveBeenCalled();}
async function choose(selected: typeof projects, shown: typeof projects = selected) {
  const user = userEvent.setup();
  const view = render(<ProjectOverviewBoard projects={shown} />);
  for (const project of selected) {
    const heading = await screen.findByRole('heading', {name: project.name});
    const card = heading.closest('li')!;
    await user.click(within(card).getByRole('button', {name: 'Manage queue'}));
    await user.click(await screen.findByRole('checkbox', {name: `Select Private task ${project.id} in ${project.name}`}));
  }
  return {user, view};
}
async function openReview(user: ReturnType<typeof userEvent.setup>) {await user.click(screen.getByRole('button', {name: 'Cancel waiting…'}));}
beforeEach(() => {
  vi.restoreAllMocks(); resetCockpit(); useCockpit.getState().setProjects(projects);
  environment.scope = 'bulk-account-one'; environment.teamId = 'team-one'; environment.revision = 0;
  vi.spyOn(projectOverviewApi, 'get').mockImplementation(async id => overview(id));
  vi.spyOn(swarmQueueApi, 'get').mockImplementation(async id => queue(id));
  vi.spyOn(swarmQueueApi, 'control').mockImplementation(async id => queue(id));
  vi.spyOn(swarmQueueApi, 'enqueueMany').mockRejectedValue(new Error('No enqueue is authorized by these display tests.'));
  vi.spyOn(teamsApi, 'projectAccess').mockImplementation(async (teamId, projectId) => permitted(teamId, projectId));
});
afterEach(cleanup);

it('CLOUD-060 bulk: one pending exact peer hides every selected private row until both owners are admitted', async () => {
  const requests = holdPermissions(); const {user} = await choose(projects.slice(0, 2), projects.slice(0, 3));
  useCockpit.getState().selectProject(projects[2].id); // Not a selected task owner.
  await openReview(user);
  expectPrivateReviewHidden();
  await waitFor(() => expect(requests).toHaveLength(2));
  expect(requests.map(row => row.projectId).sort()).toEqual(projects.slice(0, 2).map(row => row.id));
  await act(async () => requests[0].resolve(permitted(requests[0].teamId, requests[0].projectId)));
  expectPrivateReviewHidden(); expect(screen.getByText('2 waiting tasks selected')).toBeTruthy();
  await act(async () => requests[1].resolve(permitted(requests[1].teamId, requests[1].projectId)));
  const dialog = await screen.findByRole('dialog', {name: 'Cancel selected waiting tasks?'});
  for (const project of projects.slice(0, 2)) expect(within(dialog).getByText(`${project.name} · Private task ${project.id}`)).toBeTruthy();
  expect(within(dialog).queryByText(projects[2].name)).toBeNull(); expectNoWrites();
});

it('CLOUD-060 bulk: a selected peer revoked on revision refresh suspends the whole private review without clearing selection', async () => {
  const {user, view} = await choose(projects.slice(0, 2)); await openReview(user);
  await screen.findByRole('dialog', {name: 'Cancel selected waiting tasks?'});
  const requests = holdPermissions(); environment.revision++;
  view.rerender(<ProjectOverviewBoard projects={projects.slice(0, 2)} />);
  expectPrivateReviewHidden();
  await waitFor(() => expect(requests).toHaveLength(2));
  await act(async () => {
    requests[0].resolve(permitted(requests[0].teamId, requests[0].projectId));
    requests[1].reject(new ApiError('permission_denied' as ProblemCode, 'Peer access revoked.', 403));
  });
  expectPrivateReviewHidden(); expect(screen.getByText('2 waiting tasks selected')).toBeTruthy();
  for (const project of projects.slice(0, 2)) expect(screen.getByRole('checkbox', {name: `Select Private task ${project.id} in ${project.name}`})).toHaveProperty('checked', true);
  expectNoWrites();
});

it('CLOUD-060 bulk: Close review while a peer read is pending aborts that exact batch, retains selection and never cancels work', async () => {
  const requests = holdPermissions(); const {user} = await choose(projects.slice(0, 2)); await openReview(user);
  await waitFor(() => expect(requests).toHaveLength(2));
  expectPrivateReviewHidden();
  await user.click(screen.getByRole('button', {name: 'Close review'}));
  expect(requests.every(row => row.signal?.aborted)).toBe(true);
  await act(async () => requests.forEach(row => row.resolve(permitted(row.teamId, row.projectId))));
  expectPrivateReviewHidden(); expect(screen.queryByRole('region', {name: 'Review access'})).toBeNull();
  expect(screen.getByText('2 waiting tasks selected')).toBeTruthy(); expectNoWrites();
});

it('CLOUD-060 bulk: an aborted older revision cannot reveal names while the current exact owner remains pending or denied', async () => {
  const requests = holdPermissions(); const {user, view} = await choose(projects.slice(0, 2)); await openReview(user);
  await waitFor(() => expect(requests).toHaveLength(2));
  const older = requests.slice(); environment.revision++;
  view.rerender(<ProjectOverviewBoard projects={projects.slice(0, 2)} />);
  await waitFor(() => expect(requests).toHaveLength(4));
  expect(older.every(row => row.signal?.aborted)).toBe(true);
  await act(async () => older.forEach(row => row.resolve(permitted(row.teamId, row.projectId))));
  expectPrivateReviewHidden();
  await act(async () => {
    requests[2].resolve(permitted(requests[2].teamId, requests[2].projectId));
    requests[3].resolve({...permitted(requests[3].teamId, requests[3].projectId), permissions: {...personalPermissions, read: false}});
  });
  expectPrivateReviewHidden(); expectNoWrites();
});

it('CLOUD-060 bulk: changing account ownership unmounts the review and rejects its late positive reads', async () => {
  const requests = holdPermissions(); const {user, view} = await choose(projects.slice(0, 2)); await openReview(user);
  await waitFor(() => expect(requests).toHaveLength(2));
  environment.scope = 'bulk-account-two'; environment.teamId = 'team-two';
  view.rerender(<ProjectOverviewBoard projects={projects.slice(2, 3)} />);
  expect(requests.every(row => row.signal?.aborted)).toBe(true);
  await act(async () => requests.forEach(row => row.resolve(permitted(row.teamId, row.projectId))));
  expectPrivateReviewHidden(); expect(screen.queryByText('2 waiting tasks selected')).toBeNull();
  expect(requests).toHaveLength(2); expectNoWrites();
});

it('CLOUD-060 bulk: deduplicates exact selected owners and bounds permission admission to four concurrent reads', async () => {
  vi.mocked(swarmQueueApi.get).mockImplementation(async id => queue(id, id === projects[0].id));
  const requests = holdPermissions(); const {user} = await choose(projects.slice(0, 6), projects);
  await user.click(screen.getByRole('checkbox', {name: `Select Private task ${projects[0].id} extra in ${projects[0].name}`}));
  await openReview(user); expectPrivateReviewHidden();
  await waitFor(() => expect(requests).toHaveLength(4));
  expect(requests.filter(row => !row.settled && !row.signal?.aborted)).toHaveLength(4);
  await act(async () => requests[0].resolve(permitted(requests[0].teamId, requests[0].projectId)));
  await waitFor(() => expect(requests).toHaveLength(5));
  expect(requests.filter(row => !row.settled && !row.signal?.aborted)).toHaveLength(4);
  await act(async () => requests[1].resolve(permitted(requests[1].teamId, requests[1].projectId)));
  await waitFor(() => expect(requests).toHaveLength(6));
  expect(requests.filter(row => !row.settled && !row.signal?.aborted)).toHaveLength(4);
  expect(requests.map(row => row.projectId).sort()).toEqual(projects.slice(0, 6).map(row => row.id));
  expect(new Set(requests.map(row => row.projectId)).size).toBe(6);
  expect(requests.every(row => row.teamId === 'team-one')).toBe(true);
  await act(async () => requests.filter(row => !row.settled).forEach(row => row.resolve(permitted(row.teamId, row.projectId))));
  const dialog = await screen.findByRole('dialog', {name: 'Cancel selected waiting tasks?'});
  expect(within(dialog).getAllByRole('listitem')).toHaveLength(7); expectNoWrites();
});

it.each(['project', 'team'] as const)('CLOUD-060 bulk: refuses a positive permission response with the wrong %s binding', async kind => {
  vi.mocked(teamsApi.projectAccess).mockImplementation(async (teamId, projectId) => permitted(kind === 'team' ? 'foreign-team' : teamId, kind === 'project' ? 'foreign-project' : projectId));
  const {user} = await choose(projects.slice(0, 1)); await openReview(user);
  await screen.findByText('This review is unavailable with the current project access. Restore access before continuing.');
  expectPrivateReviewHidden(); expect(screen.getByText('1 waiting task selected')).toBeTruthy(); expectNoWrites();
});
